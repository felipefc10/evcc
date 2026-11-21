package supercharge

import (
	"fmt"
	"math"
	"time"
)

// The charger lease. A loadpoint whose charger bridge supports it (LpConfig.LeaseTopic)
// gets "<amps> <seconds> <fallback> <seq>" every step: the bridge sets <amps> and, if no
// new lease arrives within <seconds>, lowers the charger to <fallback> by itself. So a
// lost network, broker or evcc can never hold a current the meter cannot stand: on
// 2026-11-18 a router restart cut the Wallbox off mid-burst, it held 28 A and the ICP
// tripped. The lease is an extra output only: control decisions do not change.
//
//   - <seconds> is Settings.LeaseS, shortened to the meter's patience whenever the house
//     is over the line: in a burst the time left to the burst's planned exit, while
//     balancing the time left to the base backstop's stop closeness.
//   - <fallback> is the fail-safe current, at least the loadpoint's minimum.
//   - <seq> counts up with every message, releases included, from the clock at start:
//     evcc publishes asynchronously, and the bridge drops a message that arrives behind a
//     newer one, so an older, higher current never overtakes a lower one or a release.
//   - A setpoint at or below the fallback needs no lease: "0 <seq>" releases it, and the
//     bridge lowers to the fallback (it never raises).
//
// Leases go out non-retained (a stored lease would be applied, stale, by a restarting
// bridge) at evcc's MQTT QoS. A message queued for a connection that is lost is dropped by
// evcc's client, and one that still arrives late is older than the next one by <seq>.

// Publisher publishes a non-retained MQTT message
type Publisher func(topic, payload string)

// SetPublisher wires the MQTT publisher used for charger leases
func (m *Manager) SetPublisher(p Publisher) {
	m.mu.Lock()
	m.mqttPub = p
	m.mu.Unlock()
}

// leaseSlackS is added to every hold so a healthy renewal is never beaten by its own
// countdown: one loop period plus the longest meter read of a step (step's 2.5 s).
const leaseSlackS = pollS + 2.5

// leaseSeqStart seeds the counter from the clock, so a restarted evcc continues above the
// last run's numbers (the bridge would drop a lower one as out of order).
func leaseSeqStart() int64 {
	return time.Now().UnixMilli()
}

// leaseHoldS is how long the current setpoint may stand without a renewal.
func leaseHoldS(tel Telemetry, cv Curve, s Settings) float64 {
	hold := s.LeaseS
	switch {
	case tel.Phase == PhaseBurst:
		// The controller's own planned exit (stepBurst's "deadline": banked meter time Ti
		// reaching the exit closeness, less the shed lead). The meter's patience is taken
		// at the burst target, or at the measured load when that is higher: while the
		// current is still ramping the house is not yet over the line, and its patience
		// there would say "forever".
		tdisp := cv.Tdisp(math.Max(tel.VaKVA, s.BurstKVA))
		exitAt := math.Min(cv.PlannedCloseness(s.BurstKVA, s.BumpKVA, s.MarginS), s.MaxCloseness)
		hold = math.Min(hold, math.Max(exitAt*tdisp-tel.TiS-tel.ExitLeadS, 0)+leaseSlackS)
	case tel.VaKVA > cv.ThresholdKVA():
		// Balancing over the line (a house surge, a slow car): no longer than the base
		// backstop would allow before stopping cars.
		tdisp := cv.Tdisp(tel.VaKVA)
		hold = math.Min(hold, math.Max(s.BaseStopCloseness*tdisp-tel.TiS, 0)+leaseSlackS)
	}
	return hold
}

// leaseFallbackA is the current a leased charger falls back to: the fail-safe current,
// never below the loadpoint's minimum (a charger may refuse less, and the bridge would
// retry forever). A fail-safe of 0 (no limit while off) leaves the minimum.
func leaseFallbackA(failsafeA, minA float64) int {
	return int(math.Max(math.Floor(failsafeA), math.Ceil(minA)))
}

// leasePayload is the message for one loadpoint: a lease, or a release at or below the fallback
func leasePayload(amps, fallback int, holdS float64, seq int64) string {
	if amps <= fallback {
		return releasePayload(seq)
	}
	return fmt.Sprintf("%d %.1f %d %d", amps, holdS, fallback, seq)
}

func releasePayload(seq int64) string {
	return fmt.Sprintf("0 %d", seq)
}

// publishLeasesLocked renews every leased loadpoint's lease after a step.
// A release is sent once; renewals are sent every step, they are the heartbeat.
// Balancing runs single-phase only (checkPhasesLocked), so a setpoint is per phase.
func (m *Manager) publishLeasesLocked(tel Telemetry) {
	if m.mqttPub == nil {
		return
	}
	// A lease whose topic was changed or cleared, or whose loadpoint is gone, is released
	// on its old topic: left alone it would expire later and silently derate a car the
	// controller believes it set.
	for key, topic := range m.leased {
		if m.cfg.Loadpoints[key].LeaseTopic != topic {
			m.releaseLocked(key)
		}
	}
	hold := leaseHoldS(tel, m.cfg.Curve(), m.cfg.Settings)
	for _, row := range tel.Lps {
		topic := m.cfg.Loadpoints[row.Key].LeaseTopic
		if topic == "" {
			continue
		}
		fallback := leaseFallbackA(m.cfg.FailsafeA, row.MinA)
		if row.SetpointA > fallback {
			m.leaseSeq++
			m.mqttPub(topic, leasePayload(row.SetpointA, fallback, hold, m.leaseSeq))
			m.leased[row.Key] = topic
		} else if _, held := m.leased[row.Key]; held {
			m.releaseLocked(row.Key)
		}
	}
}

// releaseLeasesLocked ends every lease, when load management hands the loadpoints back
func (m *Manager) releaseLeasesLocked() {
	if m.mqttPub == nil {
		return
	}
	for key := range m.leased {
		m.releaseLocked(key)
	}
}

// releaseLocked ends a held lease on the topic it was sent to
func (m *Manager) releaseLocked(key string) {
	m.leaseSeq++
	m.mqttPub(m.leased[key], releasePayload(m.leaseSeq))
	delete(m.leased, key)
}
