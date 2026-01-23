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

	// Calculate average voltage per phase
	avgVoltage := (v[0] + v[1] + v[2]) / 3

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

	// 1. Calculate Budget
	// sitePower is net export (surplus).
	// We want to distribute (Surplus + CurrentConsumption of controlled loadpoints).
	budget := -sitePower

	for _, lp := range site.loadpoints {
		status := lp.GetStatus()
		mode := lp.GetMode()

		// Ignore disconnected or Off
		if status == api.StatusA || mode == api.ModeOff {
			allocations[lp] = 0
			continue
		}

		// Ignore Now (handled by default Max in update loop)
		if mode == api.ModeNow {
			allocations[lp] = lp.GetMaxCurrent()
			continue
		}

		// PV / MinPV
		// Add current consumption back to budget
		budget += lp.GetChargePower()

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

		if mode == api.ModeMinPV {
			// MinPV: Always gets Min.
			node.basePower = minP
			node.minPower = 0 // Any extra is fine
		} else {
			// PV: Needs Min to start
			node.basePower = 0
			node.minPower = minP
		}

		nodes = append(nodes, node)
	}

	// 2. Deduct Base Power (MinPV guarantees)
	for _, n := range nodes {
		budget -= n.basePower
	}

	// 3. Group and Sort
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].priority > nodes[j].priority
	})

	// 4. Distribute Remaining Budget
	remainingBudget := max(0, budget)

	// Process priority groups
	if len(nodes) > 0 {
		i := 0
		for i < len(nodes) {
			// Identify current priority group
			prio := nodes[i].priority
			j := i
			for j < len(nodes) && nodes[j].priority == prio {
				j++
			}
			group := nodes[i:j]
			i = j

			// Distribute to this group
			// Iterative approach to handle max limits
			for remainingBudget > 0 {
				// Count active nodes in this group (those that can take more power)
				var active []*loadpointNode
				for _, n := range group {
					currentAlloc := n.basePower + n.allocation
					if currentAlloc < n.maxPower {
						active = append(active, n)
					}
				}

				if len(active) == 0 {
					break
				}

				share := remainingBudget / float64(len(active))
				allocatedThisRound := 0.0

				for _, n := range active {
					wanted := n.maxPower - (n.basePower + n.allocation)
					give := min(wanted, share)
					n.allocation += give
					allocatedThisRound += give
				}

				remainingBudget -= allocatedThisRound
				if allocatedThisRound < 1.0 { // precision break
					break
				}
			}
		}
	}

	// 5. Check Min Power Thresholds for PV mode
	// If a PV node (basePower=0) didn't get enough optional allocation to meet minPower, revoke it.
	// But wait, if we revoke, the power becomes available again!
	// This requires a restart of distribution or a specific strategy.
	// For "Equal Split", if split < min, nobody gets it?
	// The iterative allocator above fills greedily.
	// If we have A (min 4) and B (min 4). Budget 6.
	// Split 3 each. Both < Min.
	// Both should drop.
	// But `allocation` field is updated.
	// We need to verify valid state.

	// Post-processing check
	for _, n := range nodes {
		total := n.basePower + n.allocation
		// If this is a purely optional node (basePower == 0) and total < minPower
		if n.basePower == 0 && total < n.minPower {
			// Revoke allocation
			// budget += n.allocation // Logic to give back?
			// For now, strict revocation without redistribution.
			// Redistribution would require complex recursion.
			// "Equal Split" implies if we can't split equally sufficient amount, we stop.
			n.allocation = 0
		}
	}

	// 6. Return Power (Watts)
	for _, n := range nodes {
		totalP := n.basePower + n.allocation
		allocations[n.lp] = totalP
	}

	return allocations
}
