package core

import (
	"math"
	"sort"
	"time"

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
func (site *Site) distributePower(loadpoints []loadpoint.API, sitePower float64, consumption, feedin api.Rates) map[loadpoint.API]float64 {
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
			gridBudget = maxP - site.gridPower
		}
	}

	if site.log != nil {
		site.log.DEBUG.Printf("prioritizer: budgets before add-back: solar %.0fW, grid %.0fW, site %.0fW", solarBudget, gridBudget, sitePower)
	}

	// Add current consumption back to budgets
	for _, lp := range loadpoints {
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
			// Smart Cost or Feedin Check
			if lp.GetSmartCostLimit() != nil || lp.GetSmartFeedInPriorityLimit() != nil {
				// We don't have access to lp.smartLimitActive logic easily without duplication.
				// However, Loadpoint has public methods to check.
				// But site.go calls lp.Update with rates later.
				// We need to check here.
				// Duplicating check logic:
				smartCost := lp.GetSmartCostLimit() != nil && consumption != nil && func() bool {
					rate, err := consumption.At(time.Now())
					return err == nil && rate.Value <= *lp.GetSmartCostLimit()
				}()

				smartFeedin := lp.GetSmartFeedInPriorityLimit() != nil && feedin != nil && func() bool {
					rate, err := feedin.At(time.Now())
					return err == nil && rate.Value >= *lp.GetSmartFeedInPriorityLimit()
				}()

				if smartCost || smartFeedin {
					node.basePower = maxP // Treat as Mode Now
					node.allowGrid = true
				} else {
					node.basePower = minP
					node.minPower = minP
					node.allowGrid = true
				}
			} else {
				node.basePower = minP
				node.minPower = minP
				node.allowGrid = true
			}
		} else {
			// PV: Needs Min to start. Uses Solar.
			// Smart Cost or Feedin Check
			smartCost := lp.GetSmartCostLimit() != nil && consumption != nil && func() bool {
				rate, err := consumption.At(time.Now())
				return err == nil && rate.Value <= *lp.GetSmartCostLimit()
			}()

			smartFeedin := lp.GetSmartFeedInPriorityLimit() != nil && feedin != nil && func() bool {
				rate, err := feedin.At(time.Now())
				return err == nil && rate.Value >= *lp.GetSmartFeedInPriorityLimit()
			}()

			if smartCost || smartFeedin {
				node.basePower = maxP // Treat as Mode Now
				node.minPower = minP
				node.allowGrid = true
			} else {
				node.basePower = 0
				node.minPower = minP
				node.allowGrid = false
			}
		}

		nodes = append(nodes, node)
	}

	if site.log != nil {
		site.log.DEBUG.Printf("prioritizer: budgets after add-back: solar %.0fW, grid %.0fW", solarBudget, gridBudget)
	}

	// 3. Group and Sort
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].priority > nodes[j].priority
	})

	// 4. Distribute
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

			// Step A: Base Power (Solar)
			// Tries to fulfill base requirement from Solar.
			distributeBudget(group, &solarBudget, &gridBudget, func(n *loadpointNode) float64 {
				return n.basePower - n.allocation
			}, func(n *loadpointNode) bool {
				return true
			})

			// Step B: Base Power (Grid)
			// Tries to fulfill remaining base requirement from Grid.
			distributeBudget(group, &gridBudget, nil, func(n *loadpointNode) float64 {
				return n.basePower - n.allocation
			}, func(n *loadpointNode) bool {
				return n.allowGrid
			})

			// Step C: Optional Power (Solar)
			// Tries to fulfill remaining max requirement from Solar.
			distributeBudget(group, &solarBudget, &gridBudget, func(n *loadpointNode) float64 {
				return n.maxPower - n.allocation
			}, func(n *loadpointNode) bool {
				// Everyone (who hasn't reached max) can take solar surplus
				// Usually strict filtering isn't needed here as base logic handled mode constraints.
				// But we should double check if pure Grid mode should take solar?
				// Mode Now (Grid) has basePower=maxPower, so allocation already full, demand=0.
				// Mode MinPV (Grid+Solar) has base=min. Optional=Max. demand>0. Takes Solar.
				// Mode PV (Solar) has base=0. Optional=Max. demand>0. Takes Solar.
				// So `true` is fine.
				return true
			})
		}
	}

	// 5. Check Min Power Thresholds for Pure PV
	// If a PV node didn't get enough to start, revoke allocation.
	for _, n := range nodes {
		if !n.allowGrid && n.allocation < n.minPower {
			if n.allocation > 0 {
				if site.log != nil {
					site.log.DEBUG.Printf("prioritizer: lp %s (prio %d) revoked %.0fW allocation (below min %.0fW)", n.lp.GetTitle(), n.priority, n.allocation, n.minPower)
				}
			}
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

// distributeBudget distributes a specific budget to a group of nodes equally/proportionally
func distributeBudget(group []*loadpointNode, budget *float64, secondaryBudget *float64, demandFunc func(*loadpointNode) float64, filterFunc func(*loadpointNode) bool) {
	available := math.Max(0, *budget)
	if available <= 0.1 {
		return
	}

	// Iterative Water Filling
	for available > 0.1 {
		// Find active consumers
		var active []*loadpointNode
		totalDemand := 0.0

		for _, n := range group {
			if !filterFunc(n) {
				continue
			}
			demand := demandFunc(n)
			if demand > 0.1 {
				active = append(active, n)
				totalDemand += demand
			}
		}

		if len(active) == 0 {
			break
		}

		// Distribute share
		// Each gets min(demand, share)
		// Share = available / count
		share := available / float64(len(active))
		usedThisRound := 0.0

		for _, n := range active {
			demand := demandFunc(n)
			give := math.Min(demand, share)
			n.allocation += give
			usedThisRound += give
		}

		available -= usedThisRound

		// If we gave nothing (shouldn't happen with >0 checks), break to avoid infinite loop
		if usedThisRound < 0.001 {
			break
		}
	}

	used := math.Max(0, *budget) - available
	*budget = available

	// Reduce secondary budget if linked (e.g. Solar usage consumes Grid Capacity)
	if secondaryBudget != nil {
		*secondaryBudget -= used
	}
}
