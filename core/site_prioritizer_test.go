package core

import (
	"testing"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/circuit"
	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
)

func TestDistributePower(t *testing.T) {
	// Setup Loadpoints
	lp1 := &Loadpoint{
		status:     api.StatusC,
		mode:       api.ModePV,
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     3,
	}
	lp2 := &Loadpoint{
		status:     api.StatusC,
		mode:       api.ModePV,
		priority:   2,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	site := &Site{
		loadpoints:   []*Loadpoint{lp1, lp2},
		gridVoltages: []float64{230, 230, 230},
		Voltage:      230,
	}

	// Case 1: Enough for both (15kW)
	// LP1 (3p) needs ~4.1kW min.
	// LP2 (1p) needs ~1.4kW min.
	// Surplus 15kW.
	// Budget = 15kW + 0 (charge).
	// Priority 2 (LP2) > Priority 1 (LP1).
	// LP2 gets Max (16A * 230 = 3680W).
	// LP1 gets rest (11320W). Max (16A*230*3 = 11040W).
	// LP1 clamped to Max.
	// LP1 gets 11040W.
	allocs := site.distributePower(-15000, nil, nil)

	// Verify LP2 (High Prio)
	// 3680W / 230V = 16A.
	assert.InDelta(t, 3680.0, allocs[lp2], 1.0, "LP2 should get max")

	// Verify LP1 (Low Prio)
	// 11040W / 690V = 16A.
	assert.InDelta(t, 11040.0, allocs[lp1], 1.0, "LP1 should get max")

	// Case 2: Not enough for both. Prio works.
	// Surplus 4kW.
	// LP2 (Prio 2, 1p) needs 1.4kW.
	// LP1 (Prio 1, 3p) needs 4.1kW.
	// LP2 gets Max (3.68kW).
	// Remaining: 320W.
	// LP1 needs 4.1kW. 320 < 4100.
	// LP1 gets 0.
	allocs = site.distributePower(-4000, nil, nil)
	assert.InDelta(t, 3680.0, allocs[lp2], 1.0, "LP2 should get max")
	assert.InDelta(t, 0.0, allocs[lp1], 1.0, "LP1 should get 0")

	// Case 3: Equal Priority. Split.
	lp2.priority = 1
	// Surplus 5kW.
	// Both Prio 1.
	// LP1 (3p) Needs 4.1kW. Max 11kW.
	// LP2 (1p) Needs 1.4kW. Max 3.68kW.
	// Split: 2.5kW each.
	// LP2: 2.5kW > 1.4kW. Alloc = 2.5kW.
	// LP1: 2.5kW < 4.1kW. Alloc = 0?
	// With my logic:
	// Group Demand check? No, iterative split.
	// Share 2.5kW.
	// LP2 takes 2.5kW.
	// LP1 takes 2.5kW? No, < Min.
	// Post-check: LP1 < Min -> 0.
	// Result: LP2=2.5kW, LP1=0.
	allocs = site.distributePower(-5000, nil, nil)
	assert.InDelta(t, 2500.0, allocs[lp2], 1.0, "LP2 gets split share")
	assert.InDelta(t, 0.0, allocs[lp1], 1.0, "LP1 gets 0")

	// Case 4: Min Start.
	// Surplus 4.2kW.
	// LP1 (3p) needs 4.1kW.
	// LP2 (1p) needs 1.4kW.
	// Split 2.1kW.
	// LP2 > 1.4 -> 2.1kW.
	// LP1 < 4.1 -> 0.
	// Result: LP2=2.1kW. LP1=0.
	allocs = site.distributePower(-4200, nil, nil)
	assert.InDelta(t, 2100.0, allocs[lp2], 1.0, "LP2 gets split")
	assert.InDelta(t, 0.0, allocs[lp1], 1.0, "LP1 gets 0")

	// Case 5: MinPV.
	lp1.mode = api.ModeMinPV // 3p. Min 4.1kW.
	// Surplus 2kW.
	// LP1 MinPV: Base 4.1kW reserved.
	// Budget = 2kW - 4.1kW = -2.1kW.
	// Distribute 0.
	// LP1 gets Base (4.1kW).
	// LP2 (PV) gets 0.
	allocs = site.distributePower(-2000, nil, nil)
	assert.InDelta(t, 4140.0, allocs[lp1], 1.0, "LP1 MinPV gets min (6A)") // 6*230*3 = 4140
	assert.InDelta(t, 0.0, allocs[lp2], 1.0, "LP2 gets 0")
}

func TestDistributePower_1pGrid(t *testing.T) {
	// Setup Loadpoints
	lp1 := &Loadpoint{
		status:     api.StatusC,
		mode:       api.ModePV,
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	site := &Site{
		loadpoints:   []*Loadpoint{lp1},
		gridVoltages: []float64{230, 0, 0}, // 1-phase grid
		Voltage:      230,
	}

	// Surplus 3kW.
	// LP1 (1p) needs 6A * 230V = 1380W.
	// With correct logic (ignoring zeros), voltage = 230V.
	// Max power = 16 * 230 = 3680W.
	// Alloc = 3000W.

	allocs := site.distributePower(-3000, nil, nil)

	// We expect full utilization of 3000W
	assert.InDelta(t, 3000.0, allocs[lp1], 1.0, "Should allocate full 3000W on 1p grid")
}

func TestDistributePower_GridLimit(t *testing.T) {
	// Setup Loadpoints
	lp1 := &Loadpoint{
		title:      "Now",
		status:     api.StatusC,
		mode:       api.ModeNow, // Wants Max (3.6kW) from Grid
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	lp2 := &Loadpoint{
		title:      "MinPV",
		status:     api.StatusC,
		mode:       api.ModeMinPV, // Wants Min (3.6kW) from Grid
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	// Site with Circuit Limit
	circ, _ := circuit.New(util.NewLogger("foo"), "main", 0, 5000, nil, 0) // Max Power 5kW
	site := &Site{
		loadpoints:   []*Loadpoint{lp1, lp2},
		gridVoltages: []float64{230, 230, 230},
		Voltage:      230,
		circuit:      circ,
	}

	allocs := site.distributePower(0, nil, nil) // 0 Solar Surplus

	// Total should be ~5000
	assert.InDelta(t, 5000.0, allocs[lp1] + allocs[lp2], 100.0, "Total should equal grid limit")
}

func TestDistributePower_GridLimit_Priority(t *testing.T) {
	// Setup Loadpoints
	lp1 := &Loadpoint{
		title:      "Now Low Prio",
		status:     api.StatusC,
		mode:       api.ModeNow, // Wants Max (3.68kW)
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	lp2 := &Loadpoint{
		title:      "Now High Prio",
		status:     api.StatusC,
		mode:       api.ModeNow, // Wants Max (3.68kW)
		priority:   2,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	// Site with Circuit Limit 5kW
	// Both want 3.68kW. Total 7.36kW.
	// 5kW available.
	// LP2 (Prio 2) should get 3.68kW.
	// LP1 (Prio 1) should get remaining 1.32kW.

	circ, _ := circuit.New(util.NewLogger("foo"), "main", 0, 5000, nil, 0)
	site := &Site{
		loadpoints:   []*Loadpoint{lp1, lp2},
		gridVoltages: []float64{230, 230, 230},
		Voltage:      230,
		circuit:      circ,
	}

	allocs := site.distributePower(0, nil, nil)

	assert.InDelta(t, 3680.0, allocs[lp2], 1.0, "High Prio LP2 should get full demand")
	assert.InDelta(t, 1320.0, allocs[lp1], 1.0, "Low Prio LP1 should get remaining")
}

func TestDistributePower_DoubleCounting(t *testing.T) {
	// Setup Loadpoints
	lp1 := &Loadpoint{
		title:      "Now",
		status:     api.StatusC,
		mode:       api.ModeNow, // Wants Max (3.68kW)
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	lp2 := &Loadpoint{
		title:      "Now 2",
		status:     api.StatusC,
		mode:       api.ModeNow, // Wants Max (3.68kW)
		priority:   1,
		minCurrent: 6,
		maxCurrent: 16,
		phases:     1,
	}

	// Scenario: Max Import = 5000W. PV = 5000W. Net Grid = -5000W (Export).
	// GridBudget (Capacity) = Max - Grid = 5000 - (-5000) = 10000W.
	// SolarBudget = 5000W.

	// Total Available Capacity = 10000W (5kW from PV + 5kW from Grid).
	// If we don't fix double counting, we might think we have 10kW Grid + 5kW Solar = 15kW?
	// No, GridBudget implicitly includes Solar Surplus capacity.

	// Demand: LP1 (3.68kW) + LP2 (3.68kW) = 7.36kW.
	// This fits easily in 10kW.

	// Let's constrain it. Max Import = 1000W. PV = 5000W. Grid = -5000W.
	// GridBudget = 1000 - (-5000) = 6000W.
	// SolarBudget = 5000W.
	// Total Capacity = 6000W.

	// Demand: 7.36kW.
	// Expectation: Capped at 6000W.

	circ, _ := circuit.New(util.NewLogger("foo"), "main", 0, 1000, nil, 0) // Max Power 1kW
	site := &Site{
		loadpoints:   []*Loadpoint{lp1, lp2},
		gridVoltages: []float64{230, 230, 230},
		Voltage:      230,
		circuit:      circ,
		gridPower:    -5000, // Exporting 5kW
	}

	allocs := site.distributePower(-5000, nil, nil) // Solar Surplus 5kW

	total := allocs[lp1] + allocs[lp2]
	assert.InDelta(t, 6000.0, total, 1.0, "Total should be capped at Max Import + PV")
}

func TestDistributePower_SmartCost(t *testing.T) {
	// Scenario: Mode PV. Solar = 0. Grid Price Cheap.
	// Expectation: Should use Grid Power.

	limit := 0.20
	lp1 := &Loadpoint{
		status:         api.StatusC,
		mode:           api.ModePV,
		priority:       1,
		minCurrent:     6,
		maxCurrent:     16,
		phases:         1,
		smartCostLimit: &limit, // Limit 0.20
	}

	rates := api.Rates{
		{Start: time.Now(), End: time.Now().Add(time.Hour), Value: 0.10}, // Cheap
	}

	site := &Site{
		loadpoints:   []*Loadpoint{lp1},
		gridVoltages: []float64{230, 230, 230},
		Voltage:      230,
	}

	// Pass rates. Logic should detect rate <= limit and enable grid.
	allocs := site.distributePower(0, rates, nil)

	// Expectation: LP1 treated as Mode Now -> Base Power = Max.
	// Max Power = 16A * 230V = 3680W.
	assert.InDelta(t, 3680.0, allocs[lp1], 1.0, "Should charge at max power due to cheap grid")
}
