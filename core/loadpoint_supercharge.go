package core

import (
	"math"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/supercharge"
)

var _ supercharge.Loadpoint = (*Loadpoint)(nil)

// superchargeRegister attaches the load manager to this loadpoint
func (lp *Loadpoint) superchargeRegister(m *supercharge.Manager, name string) {
	lp.limitMu.Lock()
	defer lp.limitMu.Unlock()
	lp.sc = m
	lp.scName = name
}

// SuperchargeState returns what the load manager needs to know about this loadpoint.
// Everything here is cached state: no device I/O.
func (lp *Loadpoint) SuperchargeState() supercharge.LpState {
	prio := lp.EffectivePriority()
	limitSoc := lp.EffectiveLimitSoc()
	soc := lp.GetSoc()
	charged := lp.GetChargedEnergy()

	var vehicle string
	if v := lp.GetVehicle(); v != nil {
		vehicle = v.GetTitle()
	}

	lp.RLock()
	defer lp.RUnlock()

	minA, maxA := lp.scMinA, lp.scMaxA
	if minA <= 0 {
		minA = lp.minCurrent
	}
	if maxA <= 0 {
		maxA = lp.maxCurrent
	}
	// a changed loadpoint maximum applies at once, before the next cycle recomputes the effective one
	maxA = math.Min(maxA, lp.maxCurrent)
	if lp.minCurrent > minA && lp.scMinA == lp.scLpMin {
		minA = lp.minCurrent
	}

	phases := lp.scPhases
	if phases <= 0 {
		phases = 1
	}

	st := supercharge.LpState{
		Title:        lp.title,
		Vehicle:      vehicle,
		Mode:         string(lp.mode),
		Priority:     prio,
		Connected:    lp.status == api.StatusB || lp.status == api.StatusC,
		Charging:     lp.status == api.StatusC,
		Enabled:      lp.enabled,
		DemandA:      lp.scDemand,
		MinA:         minA,
		MaxA:         maxA,
		Phases:       phases,
		OfferedA:     lp.offeredCurrent,
		ChargePowerW: lp.chargePower,
		Soc:          soc,
		LimitSoc:     limitSoc,
		RemainingWh:  lp.chargeRemainingEnergy * 1e3,
		ChargedWh:    charged,
	}
	if lp.chargeCurrents != nil {
		st.ChargeCurrents = append([]float64(nil), lp.chargeCurrents...)
	}
	return st
}

// SuperchargeCurrents reads the phase currents live from the charge meter
func (lp *Loadpoint) SuperchargeCurrents() ([]float64, error) {
	pc, ok := api.Cap[api.PhaseCurrents](lp.chargeMeter)
	if !ok {
		return nil, api.ErrNotAvailable
	}
	i1, i2, i3, err := pc.Currents()
	if err != nil {
		return nil, err
	}
	return []float64{i1, i2, i3}, nil
}

// SuperchargeApply commands the charger from the load manager, off the loadpoint's own cycle.
// A negative value hands the loadpoint back to its own cycle.
func (lp *Loadpoint) SuperchargeApply(amps int) error {
	if amps < 0 {
		lp.requestUpdate()
		return nil
	}

	lp.limitMu.Lock()
	defer lp.limitMu.Unlock()

	lp.RLock()
	demand, minA := lp.scDemand, lp.scMinA
	connected := lp.status == api.StatusB || lp.status == api.StatusC
	lp.RUnlock()

	current := float64(amps)
	switch {
	case !connected, demand <= 0, demand+1e-9 < minA:
		// never start a car the loadpoint's own mode logic does not want charging
		current = 0
	default:
		current = math.Min(current, demand)
	}

	return lp.applyLimit(current)
}

// superchargeClamp records what the mode logic asks for and returns what the load manager allows
func (lp *Loadpoint) superchargeClamp(current float64) float64 {
	if lp.sc == nil {
		return current
	}

	minC, maxC := lp.effectiveMinCurrent(), lp.effectiveMaxCurrent()
	phases := lp.ActivePhases()

	lp.Lock()
	lp.scDemand = current
	lp.scMinA, lp.scMaxA = minC, maxC
	lp.scLpMin = lp.minCurrent
	lp.scPhases = phases
	enabled, offered := lp.enabled, lp.offeredCurrent
	lp.Unlock()

	return lp.sc.Clamp(lp.scName, current, minC, enabled, offered)
}
