package supercharge

import (
	"cmp"
	"math"
	"slices"
)

// EPS is the amps below which a difference is not worth an actuation
const EPS = 0.05

// AllocLp is one loadpoint as the allocator sees it
type AllocLp struct {
	Key      string
	Priority int
	MinA     float64
	MaxA     float64
	// Wants is connected and evcc is willing to charge it
	Wants bool
	// Running breaks ties toward not operating a contactor that is already closed
	Running bool
	// CapA is a hard ceiling imposed by the caller for this step, nil for none
	CapA *float64
}

// Ceiling is the allocation ceiling
func (lp AllocLp) Ceiling() float64 {
	c := lp.MaxA
	if lp.CapA != nil {
		c = math.Min(lp.MaxA, *lp.CapA)
	}
	return math.Max(c, 0.0)
}

// Floor is the allocation floor, never above the ceiling
func (lp AllocLp) Floor() float64 {
	return math.Max(math.Min(lp.MinA, lp.Ceiling()), 0.0)
}

// Alloc is an ordered key -> amps mapping
type Alloc struct {
	Keys []string
	Amps map[string]float64
}

func newAlloc() Alloc {
	return Alloc{Amps: make(map[string]float64)}
}

// Set assigns amps to key, keeping first-insertion order
func (a *Alloc) Set(key string, amps float64) {
	if _, ok := a.Amps[key]; !ok {
		a.Keys = append(a.Keys, key)
	}
	a.Amps[key] = amps
}

// Get returns amps for key, 0 if absent
func (a Alloc) Get(key string) float64 {
	return a.Amps[key]
}

// Sum adds up the values in insertion order
func (a Alloc) Sum() float64 {
	var s float64
	for _, k := range a.Keys {
		s += a.Amps[k]
	}
	return s
}

// fill splits budget across group so every member ends at the same current except where
// its own floor or ceiling stops it (water-filling by bisection)
func fill(budget float64, group []AllocLp) Alloc {
	taken := func(level float64) Alloc {
		res := newAlloc()
		for _, lp := range group {
			res.Set(lp.Key, math.Min(math.Max(level, lp.Floor()), lp.Ceiling()))
		}
		return res
	}

	var ceilSum float64
	for _, lp := range group {
		ceilSum += lp.Ceiling()
	}
	if budget >= ceilSum-EPS {
		res := newAlloc()
		for _, lp := range group {
			res.Set(lp.Key, lp.Ceiling())
		}
		return res
	}

	lo, hi := 0.0, 0.0
	for i, lp := range group {
		if i == 0 || lp.Ceiling() > hi {
			hi = lp.Ceiling()
		}
	}
	for range 60 {
		mid := (lo + hi) / 2
		if taken(mid).Sum() < budget {
			lo = mid
		} else {
			hi = mid
		}
	}
	return taken(lo)
}

// serve serves as much of one priority group as budget allows. Returns the allocation,
// what is left and whether anybody was dropped for want of its floor.
func serve(budget float64, group []AllocLp) (Alloc, float64, bool) {
	keep := slices.Clone(group)
	slices.SortStableFunc(keep, func(a, b AllocLp) int {
		if a.Running != b.Running {
			if !a.Running {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(-a.Floor(), -b.Floor()); c != 0 {
			return c
		}
		return cmp.Compare(a.Key, b.Key)
	})

	var dropped []AllocLp
	held := 0.0
	floors := func() float64 {
		var s float64
		for _, lp := range keep {
			s += lp.Floor()
		}
		return s
	}
	for len(keep) > 0 && floors() > budget-held+EPS {
		gone := keep[0]
		keep = keep[1:]
		dropped = append(dropped, gone)
		if gone.Running {
			held += gone.Floor()
		}
	}

	avail := math.Max(budget-held, 0.0)
	out := newAlloc()
	for _, lp := range dropped {
		out.Set(lp.Key, 0.0)
	}
	if len(keep) == 0 {
		return out, avail, len(dropped) > 0
	}
	filled := fill(avail, keep)
	for _, k := range filled.Keys {
		out.Set(k, filled.Amps[k])
	}
	return out, avail - out.Sum(), len(dropped) > 0
}

// Allocate returns amps per loadpoint key, never more than budgetA in total.
// Higher priority is served first up to its ceiling; equal priorities split equally.
func Allocate(budgetA float64, lps []AllocLp) Alloc {
	out := newAlloc()
	for _, lp := range lps {
		out.Set(lp.Key, 0.0)
	}

	var live []AllocLp
	for _, lp := range lps {
		if lp.Wants && lp.Ceiling() >= EPS {
			live = append(live, lp)
		}
	}
	left := math.Max(budgetA, 0.0)

	var prios []int
	for _, lp := range live {
		if !slices.Contains(prios, lp.Priority) {
			prios = append(prios, lp.Priority)
		}
	}
	slices.Sort(prios)
	slices.Reverse(prios)

	for _, prio := range prios {
		var group []AllocLp
		for _, lp := range live {
			if lp.Priority == prio {
				group = append(group, lp)
			}
		}
		served, rest, starved := serve(left, group)
		left = rest
		for _, k := range served.Keys {
			out.Set(k, served.Amps[k])
		}
		if left <= EPS {
			left = 0.0
		}
		// a higher priority that could not be seated keeps the budget
		if starved {
			left = 0.0
		}
	}
	return out
}

// ToAmps rounds down to whole amps. A share below its floor becomes 0 (a pause).
func ToAmps(alloc Alloc, lps []AllocLp) map[string]int {
	byKey := make(map[string]AllocLp, len(lps))
	for _, lp := range lps {
		byKey[lp.Key] = lp
	}
	out := make(map[string]int)
	for _, key := range alloc.Keys {
		lp, ok := byKey[key]
		if !ok {
			continue
		}
		whole := int(math.Trunc(alloc.Amps[key] + EPS))
		if float64(whole) >= lp.Floor()-EPS {
			out[key] = whole
		} else {
			out[key] = 0
		}
	}
	return out
}
