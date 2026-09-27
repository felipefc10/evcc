// Package supercharge implements whole-house load management against the
// E-REDES ICP trip curve: balancing every loadpoint under the never-trip line
// by priority, and optional supercharging bursts above it.
//
// The trip law, per DEF-C44-506/N Ed.3 Anexo B §B1.1:
//
//	Tdisp = Q / ((Sinst / SC) - k)   valid only for Sinst > k*SC
//
// Sinst > k*SC starts a counter Ti, Ti > Tdisp opens the ICP and
// Sinst <= k*SC resets Ti to 0 instantly.
package supercharge

import "math"

var inf = math.Inf(1)

// UsableVolts returns the measured voltage, or nominal 230 V when the reading is implausible
func UsableVolts(volts float64) float64 {
	if volts > 100 {
		return volts
	}
	return 230.0
}

// Curve holds the meter parameters. Q and k are DSO-configurable.
type Curve struct {
	Q     float64 `json:"q"`
	K     float64 `json:"k"`
	SCkVA float64 `json:"scKva"`
}

// ThresholdKVA is the never-trip line
func (c Curve) ThresholdKVA() float64 {
	return c.K * c.SCkVA
}

// ThresholdAmpsAt230 is the never-trip line in amps at nominal voltage
func (c Curve) ThresholdAmpsAt230() float64 {
	return c.ThresholdKVA() * 1000.0 / 230.0
}

// Tdisp returns the seconds the meter tolerates at this apparent power, infinite at or below the line
func (c Curve) Tdisp(vaKVA float64) float64 {
	if vaKVA <= c.ThresholdKVA() {
		return inf
	}
	return c.Q / (vaKVA/c.SCkVA - c.K)
}

// BurstWindow is how long a burst may sit at burstKVA and still survive a
// bumpKVA household step at its end with marginS seconds to react.
// Zero means no safe burst exists.
func (c Curve) BurstWindow(burstKVA, bumpKVA, marginS float64) float64 {
	denom := (burstKVA+bumpKVA)/c.SCkVA - c.K
	if denom <= 0 {
		return 0
	}
	return math.Max(0, c.Q/denom-marginS)
}

// PlannedCloseness is the burst window expressed as a fraction of the meter's patience at the target
func (c Curve) PlannedCloseness(burstKVA, bumpKVA, marginS float64) float64 {
	t := c.Tdisp(burstKVA)
	if math.IsInf(t, 1) {
		return 0
	}
	return math.Max(0, c.BurstWindow(burstKVA, bumpKVA, marginS)/t)
}

// Closeness is Ti/Tdisp, the breaker opens at 1.0
func (c Curve) Closeness(vaKVA, tiS float64) float64 {
	t := c.Tdisp(vaKVA)
	if math.IsInf(t, 1) {
		return 0
	}
	return tiS / t
}

// ExpectedAverageKVA models the average power a burst duty cycle delivers
func ExpectedAverageKVA(curve Curve, burstKVA, baseKVA, windowS, resetS, houseKVA float64,
	ampMax, ampMin int, volts, carRamp, cmdLatency float64,
) float64 {
	if windowS < 5.0 || burstKVA <= baseKVA {
		return baseKVA
	}

	reachable := houseKVA + float64(ampMax)*UsableVolts(volts)/1000.0
	burst := math.Min(burstKVA, reachable)
	tUp := PlannedShedS(baseKVA, burst, houseKVA, ampMax, ampMin, volts, carRamp, cmdLatency)

	effective := math.Max(0.0, windowS-tUp*0.5)
	total := windowS + resetS
	return (burst*effective + baseKVA*(total-effective)) / total
}

// PlannedShedS is how long the car takes to get back under the line for the planned profile
func PlannedShedS(baseKVA, burstKVA, houseKVA float64, ampMax, ampMin int, volts, carRamp, cmdLatency float64) float64 {
	v := UsableVolts(volts)
	baseA := math.Max(float64(ampMin), (baseKVA-houseKVA)*1000.0/v)
	burstA := math.Min(float64(ampMax), (burstKVA-houseKVA)*1000.0/v)
	return cmdLatency + math.Max(0.0, burstA-baseA)/math.Max(carRamp, 0.1)
}

// ApparentKVA is S = sqrt(P² + Q²) in kVA
func ApparentKVA(powerW, reactiveVar float64) float64 {
	return math.Sqrt(powerW*powerW+reactiveVar*reactiveVar) / 1000.0
}
