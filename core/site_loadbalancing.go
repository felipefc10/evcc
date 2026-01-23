package core

import (
	"sort"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/loadpoint"
)

type loadpointNode struct {
	lp       loadpoint.API
	priority int
	phases   int
	voltage  float64

	// Power requirements
	basePower float64 // Guaranteed power (e.g. MinPV min)
	minPower  float64 // Minimum power to receive ANY optional allocation
	maxPower  float64 // Maximum total power

	// Permissions
	allowGrid bool // Can this node use grid power?

	// Output
	allocation float64
}

func (site *Site) effectiveVoltage(lp loadpoint.API) float64 {
	phases := lp.ActivePhases()
	if phases == 0 {
		phases = lp.GetPhases()
	}
	if phases == 0 {
		phases = 3
	}

	// Capture grid voltages (fallback to site voltage)
	v := []float64{site.Voltage, site.Voltage, site.Voltage}
	if len(site.gridVoltages) == 3 {
		v = site.gridVoltages
	}

	// Calculate average voltage per phase (ignoring zeros)
	var sum float64
	var count int
	for _, val := range v {
		if val > 10 { // ignore noise/off
			sum += val
			count++
		}
	}

	var avgVoltage float64
	if count > 0 {
		avgVoltage = sum / float64(count)
	} else {
		avgVoltage = site.Voltage // fallback
	}

	// If 3-phase, use sum of all phases
	if phases == 3 {
		return v[0] + v[1] + v[2]
	}

	// For 1 or 2 phases, estimate using average
	return avgVoltage * float64(phases)
}

// distributePower distributes the available power among loadpoints based on priority
func (site *Site) distributePower(sitePower float64) map[loadpoint.API]float64 {
	allocations := make(map[loadpoint.API]float64)
	var nodes []*loadpointNode

	// 1. Determine Budgets
	// Solar Budget: Available surplus (Export).
	// We want to distribute (Surplus + CurrentConsumption of controlled loadpoints).
	solarBudget := -sitePower

	// Grid Budget: Available grid capacity.
	// This requires knowing the Site's Max Power limit (if any) and current consumption.
	// If no limit is set, budget is effectively infinite (physically limited by fuse, handled by circuit but here we assume high).
	var gridBudget float64 = 1e6 // Default infinite

	if site.circuit != nil {
		if maxP := site.circuit.GetMaxPower(); maxP > 0 {
			// Remaining Grid = Max - CurrentUsage.
			// CurrentUsage includes Site consumption (which is SitePower if positive).
			// If SitePower is negative (Export), Site Consumption is covered by PV.
			// Actually, `site.circuit.GetChargePower()` returns total loadpoint power?
			// We need the *Grid Import*.
			// Grid Power (site.gridPower) is Import(+)/Export(-).
			// If Importing, Remaining = Max - Import.
			// If Exporting, Remaining = Max (full import capability available).

			// We need to add back the *current loadpoint consumption* to re-distribute it.
			// But careful: site.gridPower *already includes* loadpoint consumption.
			// So `Remaining = Max - site.gridPower`.

			// Example: Max 10kW. Import 2kW (EVs consuming 2kW).
			// Remaining = 10 - 2 = 8kW.
			// Total Grid Budget available for EVs = 8kW (new) + 2kW (existing) = 10kW.
			// Wait, if base load is 0kW.

			// Let's use `gridPower`.
			// `availableGrid` = `maxP - site.gridPower`.
			// We want to re-distribute.
			// So we add back current LP consumption *that is coming from grid*.
			// That's hard to distinguish.
			// Easier: `budget = available + usage`.
			gridBudget = maxP - site.gridPower
		}
	}

	// Add current consumption back to budgets
	for _, lp := range site.loadpoints {
		status := lp.GetStatus()
		mode := lp.GetMode()

		// Ignore disconnected or Off
		if status == api.StatusA || mode == api.ModeOff {
			allocations[lp] = 0
			continue
		}

		chargePower := lp.GetChargePower()
		solarBudget += chargePower

		// If limit is active, add charge power to grid budget too (as it frees up capacity if we stop charging)
		if gridBudget < 1e6 {
			gridBudget += chargePower
		}

		phases := lp.ActivePhases()
		if phases == 0 {
			phases = lp.GetPhases()
		}
		if phases == 0 {
			phases = 3
		}

		volts := site.effectiveVoltage(lp)

		minC := lp.GetMinCurrent()
		maxC := lp.GetMaxCurrent()

		minP := minC * volts
		maxP := maxC * volts

		node := &loadpointNode{
			lp:       lp,
			priority: lp.GetPriority(),
			phases:   phases,
			voltage:  volts,
			maxPower: maxP,
		}

		if mode == api.ModeNow {
			// Mode Now: Wants Max. Uses Grid.
			node.basePower = maxP // Aggressive base
			node.minPower = minP
			node.allowGrid = true
		} else if mode == api.ModeMinPV {
			// MinPV: Base is Min. Uses Grid for Base. Optional Uses Solar.
			node.basePower = minP
			node.minPower = minP
			node.allowGrid = true // For the base part
		} else {
			// PV: Needs Min to start. Uses Solar.
			node.basePower = 0
			node.minPower = minP
			node.allowGrid = false
		}

		nodes = append(nodes, node)
	}

	// 3. Group and Sort
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].priority > nodes[j].priority
	})

	// 4. Distribute

	// We need to distribute carefully.
	// Step A: Fulfill "Base Power" (Guaranteed).
	// Base Power can come from Solar OR Grid (if allowed).
	// Prioritize Solar for everyone to keep Grid usage low? Yes.

	for _, n := range nodes {
		needed := n.basePower
		if needed == 0 {
			continue
		}

		// Try Solar First
		if solarBudget >= needed {
			solarBudget -= needed
			n.allocation += needed
			// If we used solar, we effectively didn't use grid for this amount.
			// But GridBudget represents "Available Import". Solar usage doesn't reduce Import capacity.
			// So GridBudget remains same?
			// Correct.
		} else {
			// Take what we can from Solar
			taken := max(0, solarBudget)
			n.allocation += taken
			solarBudget = 0
			needed -= taken

			// Take rest from Grid (if allowed)
			if n.allowGrid {
				if gridBudget >= needed {
					gridBudget -= needed
					n.allocation += needed
				} else {
					// Grid saturated. Take what we can.
					taken := max(0, gridBudget)
					gridBudget = 0
					n.allocation += taken
				}
			}
		}
	}

	// Step B: Distribute Remaining Solar to Optional Demand (PV surplus)
	// Iterate Priority Groups
	if len(nodes) > 0 {
		i := 0
		for i < len(nodes) {
			prio := nodes[i].priority
			j := i
			for j < len(nodes) && nodes[j].priority == prio {
				j++
			}
			group := nodes[i:j]
			i = j

			// Distribute Solar Budget to Group
			distributeToGroup(group, &solarBudget, false)
		}
	}

	// 5. Check Min Power Thresholds for Pure PV
	// If a PV node didn't get enough to start, revoke allocation.
	for _, n := range nodes {
		if !n.allowGrid && n.allocation < n.minPower {
			// Return allocation to SolarBudget?
			// Implementing simple revocation for now.
			// Ideally, we loop until stable, but one pass is safer for convergence.
			n.allocation = 0
		}
	}

	// 6. Return Power (Watts)
	for _, n := range nodes {
		totalP := n.allocation
		allocations[n.lp] = totalP
	}

	return allocations
}

func distributeToGroup(group []*loadpointNode, budget *float64, useGrid bool) {
	remainingBudget := max(0, *budget)

	for remainingBudget > 0 {
		var active []*loadpointNode
		for _, n := range group {
			// If using grid, only allow grid-enabled nodes that haven't reached max
			// If not using grid (solar), allow anyone who hasn't reached max
			// Note: "Optional" demand for MinPV/Now is handled here too?
			// For "Now", basePower == maxPower, so it won't be active here.
			// For "MinPV", basePower = Min. It wants up to Max.
			// For "PV", basePower = 0. Wants up to Max.

			if n.allocation < n.maxPower {
				active = append(active, n)
			}
		}

		if len(active) == 0 {
			break
		}

		share := remainingBudget / float64(len(active))
		allocatedThisRound := 0.0

		for _, n := range active {
			wanted := n.maxPower - n.allocation
			give := min(wanted, share)
			n.allocation += give
			allocatedThisRound += give
		}

		remainingBudget -= allocatedThisRound
		if allocatedThisRound < 1.0 {
			break
		}
	}
	*budget = remainingBudget
}
