package core

import (
	"testing"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/loadpoint"
	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestDistributePower_EqualPriority_GridSplit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mocks
	lp1 := loadpoint.NewMockAPI(ctrl)
	lp2 := loadpoint.NewMockAPI(ctrl)

	// Setup Site
	site := &Site{
		log: util.NewLogger("foo"),
	}
	lps := []loadpoint.API{lp1, lp2}

	// Mock Loadpoint behavior
	// Both Mode Now (Grid), Prio 0 (Default), Wants 11kW (3p 16A)

	// LP1
	lp1.EXPECT().GetStatus().Return(api.StatusC).AnyTimes() // Charging
	lp1.EXPECT().GetMode().Return(api.ModeNow).AnyTimes()
	lp1.EXPECT().GetChargePower().Return(0.0).AnyTimes() // Assume starting from 0 for calculation simplicity? Or running?
	// If running, budget add-back logic applies. Let's assume they are connected but 0 power (waiting).
	lp1.EXPECT().ActivePhases().Return(3).AnyTimes()
	lp1.EXPECT().GetPhases().Return(3).AnyTimes()
	lp1.EXPECT().GetMinCurrent().Return(6.0).AnyTimes()
	lp1.EXPECT().GetMaxCurrent().Return(16.0).AnyTimes() // 11kW
	lp1.EXPECT().GetPriority().Return(0).AnyTimes()
	lp1.EXPECT().GetTitle().Return("LP1").AnyTimes()

	// LP2 - Same
	lp2.EXPECT().GetStatus().Return(api.StatusC).AnyTimes()
	lp2.EXPECT().GetMode().Return(api.ModeNow).AnyTimes()
	lp2.EXPECT().GetChargePower().Return(0.0).AnyTimes()
	lp2.EXPECT().ActivePhases().Return(3).AnyTimes()
	lp2.EXPECT().GetPhases().Return(3).AnyTimes()
	lp2.EXPECT().GetMinCurrent().Return(6.0).AnyTimes()
	lp2.EXPECT().GetMaxCurrent().Return(16.0).AnyTimes()
	lp2.EXPECT().GetPriority().Return(0).AnyTimes()
	lp2.EXPECT().GetTitle().Return("LP2").AnyTimes()

	// Site Configuration
	site.Voltage = 230
	// Max Grid Power = 11kW (Limit)
	// We use a mock circuit for this? Or just set gridBudget logic in code?
	// The code uses `site.circuit.GetMaxPower()`.
	// We need to mock circuit.

	// `api.Circuit` is an interface.
	circuit := api.NewMockCircuit(ctrl)
	circuit.EXPECT().GetMaxPower().Return(11000.0).AnyTimes()
	site.circuit = circuit
	site.gridPower = 0 // No other consumption

	// Run Distribute
	// Site Power = 0 (Balanced)
	// Consumption/Feedin Rates = nil
	allocs := site.distributePower(lps, 0, nil, nil)

	// Expectation:
	// Total Available Grid = 11kW.
	// Both want 11kW.
	// Should split ~5.5kW each.
	// With current logic (sequential), LP1 likely gets 11kW, LP2 gets 0.

	t.Logf("LP1 Allocation: %.1f", allocs[lp1])
	t.Logf("LP2 Allocation: %.1f", allocs[lp2])

	assert.InDelta(t, 5500.0, allocs[lp1], 100.0, "LP1 should get half")
	assert.InDelta(t, 5500.0, allocs[lp2], 100.0, "LP2 should get half")
}

func TestDistributePower_Preemption(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lpHigh := loadpoint.NewMockAPI(ctrl)
	lpLow := loadpoint.NewMockAPI(ctrl)

	site := &Site{
		log: util.NewLogger("foo"),
	}
	lps := []loadpoint.API{lpHigh, lpLow}

	// High Prio: PV Mode. Needs Solar.
	lpHigh.EXPECT().GetStatus().Return(api.StatusC).AnyTimes()
	lpHigh.EXPECT().GetMode().Return(api.ModePV).AnyTimes()
	lpHigh.EXPECT().GetChargePower().Return(0.0).AnyTimes()
	lpHigh.EXPECT().ActivePhases().Return(1).AnyTimes()
	lpHigh.EXPECT().GetPhases().Return(1).AnyTimes()
	lpHigh.EXPECT().GetMinCurrent().Return(6.0).AnyTimes() // 1.4kW
	lpHigh.EXPECT().GetMaxCurrent().Return(16.0).AnyTimes()
	lpHigh.EXPECT().GetPriority().Return(10).AnyTimes() // High
	lpHigh.EXPECT().GetTitle().Return("High").AnyTimes()
	lpHigh.EXPECT().GetSmartCostLimit().Return(nil).AnyTimes()
	lpHigh.EXPECT().GetSmartFeedInPriorityLimit().Return(nil).AnyTimes()

	// Low Prio: Now Mode. Grid.
	lpLow.EXPECT().GetStatus().Return(api.StatusC).AnyTimes()
	lpLow.EXPECT().GetMode().Return(api.ModeNow).AnyTimes()
	lpLow.EXPECT().GetChargePower().Return(0.0).AnyTimes()
	lpLow.EXPECT().ActivePhases().Return(1).AnyTimes()
	lpLow.EXPECT().GetPhases().Return(1).AnyTimes()
	lpLow.EXPECT().GetMinCurrent().Return(6.0).AnyTimes()
	lpLow.EXPECT().GetMaxCurrent().Return(16.0).AnyTimes() // 3.6kW
	lpLow.EXPECT().GetPriority().Return(0).AnyTimes() // Low
	lpLow.EXPECT().GetTitle().Return("Low").AnyTimes()

	// Site Info
	site.Voltage = 230
	// Infinite Grid
	site.circuit = nil

	// Case 1: Surplus 2kW.
	// High Prio (PV) needs 1.4kW min. Should take it.
	// Low Prio (Now) needs 3.6kW. Should take Grid.
	// Solar Budget = 2000.

	allocs := site.distributePower(lps, -2000, nil, nil) // -2000 sitePower = 2000 Surplus

	// High Prio should get > 0
	// Low Prio should get Max (3.6kW) (from Grid)

	assert.Greater(t, allocs[lpHigh], 1000.0, "High Prio should get solar")
	assert.InDelta(t, 3680.0, allocs[lpLow], 100.0, "Low Prio should get max grid")

	// Case 2: Limited Grid. Max 4kW.
	// High Prio (PV) takes 2kW Solar.
	// Low Prio (Now) wants 3.6kW.
	// Grid Budget = 4kW - (SolarUsed? No, Solar Export doesn't consume Grid Import Capacity usually, BUT evcc logic treats gridBudget as 'fuse limit', so yes if import+export limited? No usually current based.)
	// The `gridBudget` logic in evcc is `maxP - gridPower`.
	// If Exporting 2kW. GridPower = -2kW.
	// MaxP = 4kW.
	// Budget = 4 - (-2) = 6kW?
	// Yes, if fuse allows.
	// So both should be happy.
}
