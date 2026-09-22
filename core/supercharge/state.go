package supercharge

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// LpView is one loadpoint as published to the UI
type LpView struct {
	Index            int        `json:"index"`
	Name             string     `json:"name"`
	Title            string     `json:"title"`
	Vehicle          string     `json:"vehicle"`
	Mode             string     `json:"mode"`
	Priority         int        `json:"priority"`
	Connected        bool       `json:"connected"`
	Charging         bool       `json:"charging"`
	Wants            bool       `json:"wants"`
	DemandA          float64    `json:"demandA"`
	MinA             float64    `json:"minA"`
	MaxA             float64    `json:"maxA"`
	Phases           int        `json:"phases"`
	SetpointA        int        `json:"setpointA"`
	AllocatedA       int        `json:"allocatedA"`
	Paused           bool       `json:"paused"`
	Ops              int        `json:"ops"`
	WaitS            float64    `json:"waitS"`
	MeasuredA        float64    `json:"measuredA"`
	MeasuredRawA     float64    `json:"measuredRawA"`
	MeasuredSrc      string     `json:"measuredSrc"`
	Clamped          bool       `json:"clamped"`
	Supercharge      bool       `json:"supercharge"`
	SuperchargeUntil *time.Time `json:"superchargeUntil"`
	Fast             bool       `json:"fast"`
	StoodOffS        float64    `json:"stoodOffS"`
	Soc              float64    `json:"soc"`
	LimitSoc         int        `json:"limitSoc"`
	Forecast         Forecast   `json:"forecast"`
	FeedAgeS         *float64   `json:"feedAgeS"`
}

// MeterView is the meter as published
type MeterView struct {
	OK        bool    `json:"ok"`
	KVA       float64 `json:"kva"`
	Watts     float64 `json:"watts"`
	Vars      float64 `json:"vars"`
	Volts     float64 `json:"volts"`
	LatencyMs int64   `json:"latencyMs"`
	Error     string  `json:"error,omitempty"`
}

// State is the published state
type State struct {
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	Armed      bool   `json:"armed"`
	Running    bool   `json:"running"`
	Status     string `json:"status"`
	Phase      Phase  `json:"phase"`
	PollMode   string `json:"pollMode"`
	LastError  string `json:"lastError"`

	VaKVA              float64 `json:"vaKva"`
	HouseKVA           float64 `json:"houseKva"`
	BudgetA            float64 `json:"budgetA"`
	TrimA              float64 `json:"trimA"`
	TiS                float64 `json:"tiS"`
	Closeness          float64 `json:"closeness"`
	PeakCloseness      float64 `json:"peakCloseness"`
	PeakClosenessPhase string  `json:"peakClosenessPhase"`
	PeakClosenessBlind bool    `json:"peakClosenessBlind"`
	SetpointA          int     `json:"setpointA"`
	Bursts             int     `json:"bursts"`
	BurstCmds          int     `json:"burstCmds"`
	ContactorOps       int     `json:"contactorOps"`
	Writes             int     `json:"writes"`
	WritesPerH         float64 `json:"writesPerH"`
	Blind              int     `json:"blind"`

	ThresholdKVA     float64 `json:"thresholdKva"`
	ThresholdA       float64 `json:"thresholdA"`
	BaseKVA          float64 `json:"baseKva"`
	BurstKVA         float64 `json:"burstKva"`
	BurstWindowS     float64 `json:"burstWindowS"`
	CanBurst         bool    `json:"canBurst"`
	BurstArmed       bool    `json:"burstArmed"`
	BurstStoodDown   string  `json:"burstStoodDown"`
	StillbornLimit   int     `json:"stillbornLimit"`
	ExpectedKVA      float64 `json:"expectedKva"`
	ExpectedGain     float64 `json:"expectedGain"`
	PlannedCloseness float64 `json:"plannedCloseness"`
	PlannedLeadS     float64 `json:"plannedLeadS"`
	ExitCloseness    float64 `json:"exitCloseness"`
	ExitLeadS        float64 `json:"exitLeadS"`

	TempC           *float64 `json:"tempC"`
	TempBlock       bool     `json:"tempBlock"`
	MaxTempC        float64  `json:"maxTempC"`
	EntryOffsetA    float64  `json:"entryOffsetA"`
	EntryOffsetHeld bool     `json:"entryOffsetHeld"`
	SourceOhm       float64  `json:"sourceOhm"`
	VoltsAtGoal     float64  `json:"voltsAtGoal"`

	AvgKVA     float64 `json:"avgKva"`
	KVAh       float64 `json:"kvah"`
	GainVsLine float64 `json:"gainVsLine"`
	ElapsedH   float64 `json:"elapsedH"`

	Meter         MeterView `json:"meter"`
	FloorKVA      float64   `json:"floorKva"`
	IdleHouseKVA  float64   `json:"idleHouseKva"`
	ClaimKVA      float64   `json:"claimKva"`
	CreditKVA     float64   `json:"creditKva"`
	Disagreements int       `json:"disagreements"`

	Loadpoints []LpView    `json:"loadpoints"`
	Learned    []Behaviour `json:"learned"`
	Checks     []Check     `json:"checks"`
	CheckedAt  *time.Time  `json:"checkedAt"`
}

// State returns the current state
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateLocked()
}

func (m *Manager) stateLocked() State {
	c := m.cfg
	s := &m.cfg.Settings
	cv := c.Curve()
	tel := m.ctl.Tel
	running := m.running

	window := cv.BurstWindow(s.BurstKVA, s.BumpKVA, s.MarginS)
	house := tel.HouseKVA
	if !running || house <= 0 {
		house = 0.70
	}
	expected := ExpectedAverageKVA(cv, s.BurstKVA, s.BaseKVA(cv), window, s.ResetS, house,
		s.AmpMax, s.AmpMin, 230.0, CarRampAPerS, CmdLatencyS)
	plannedLead := PlannedShedS(s.BaseKVA(cv), s.BurstKVA, house, s.AmpMax, s.AmpMin, 230.0, CarRampAPerS, CmdLatencyS) * s.ExitLeadFrac

	st := State{
		Enabled:    c.Enabled,
		Configured: c.MeterURI != "",
		Armed:      m.armed,
		Running:    running,
		Status:     m.status,
		Phase:      PhaseIdle,
		PollMode:   "standby",
		LastError:  m.lastError,

		ThresholdKVA:     round(cv.ThresholdKVA(), 3),
		ThresholdA:       round(cv.ThresholdAmpsAt230(), 1),
		BaseKVA:          round(s.BaseKVA(cv), 3),
		BurstKVA:         s.BurstKVA,
		BurstWindowS:     round(window, 1),
		CanBurst:         window >= 5.0,
		BurstArmed:       s.AnySupercharge() && m.stoodDown == "",
		BurstStoodDown:   m.stoodDown,
		StillbornLimit:   StillbornLimit,
		ExpectedKVA:      round(expected, 3),
		ExpectedGain:     round(100*(expected/cv.ThresholdKVA()-1), 1),
		PlannedCloseness: round(math.Min(cv.PlannedCloseness(s.BurstKVA, s.BumpKVA, s.MarginS), s.MaxCloseness), 3),
		PlannedLeadS:     round(plannedLead, 1),
		MaxTempC:         s.MaxTempC,
		SourceOhm:        round(m.learner.SourceOhm(), 3),
		Learned:          m.learner.All(),
		Checks:           m.checks,
	}
	if !m.checkedAt.IsZero() {
		at := m.checkedAt
		st.CheckedAt = &at
	}
	if running {
		st.Phase = tel.Phase
		st.PollMode = m.pollMode
		st.VaKVA = round(tel.VaKVA, 3)
		st.HouseKVA = round(tel.HouseKVA, 3)
		st.BudgetA = tel.BudgetA
		st.TrimA = tel.TrimA
		st.TiS = round(tel.TiS, 1)
		st.Closeness = round(tel.Closeness, 3)
		st.PeakCloseness = tel.PeakClosenessSeen
		st.PeakClosenessPhase = tel.PeakClosenessPhase
		st.PeakClosenessBlind = tel.PeakClosenessBlind
		st.SetpointA = tel.SetpointA
		st.Bursts = tel.Bursts
		st.BurstCmds = tel.BurstCmds
		st.ContactorOps = tel.ContactorOps
		st.Writes = tel.Writes
		if h := time.Since(m.startedAt).Hours(); h > 0 {
			st.WritesPerH = round(float64(tel.Writes)/math.Max(h, 1.0/3600), 1)
			st.ElapsedH = round(h, 2)
		}
		st.Blind = tel.Blind
		st.ExitCloseness = round(tel.ExitCloseness, 3)
		st.ExitLeadS = round(tel.ExitLeadS, 1)
		st.TempBlock = tel.TempBlock
		st.EntryOffsetA = round(tel.EntryOffsetA, 2)
		st.EntryOffsetHeld = tel.EntryOffsetHeld
		st.VoltsAtGoal = tel.VoltsAtGoal
		st.AvgKVA = round(tel.AvgKVA, 3)
		st.KVAh = round(tel.KVAh, 3)
		if tel.AvgKVA > 0 {
			st.GainVsLine = round(100*(tel.AvgKVA/cv.ThresholdKVA()-1), 1)
		}
		floor, idle := m.ctl.HouseFloor()
		st.FloorKVA = round(floor, 3)
		st.IdleHouseKVA = round(idle, 3)
		st.ClaimKVA = tel.CarClaimKVA
		st.CreditKVA = tel.CarCreditKVA
		st.Disagreements = tel.MeterDisagreements
		st.Meter = MeterView{
			OK: m.lastReading.OK, KVA: round(m.lastReading.VaKVA, 3),
			Watts: math.Round(m.meterW), Vars: math.Round(m.meterVar), Volts: round(m.meterVolts, 1),
			LatencyMs: m.lastReading.Latency.Milliseconds(),
		}
		if m.lastReading.Err != nil {
			st.Meter.Error = m.lastReading.Err.Error()
		}
	}

	for _, cfg := range c.Loadpoints {
		if cfg.TempTopic != "" {
			if v, ok := m.feed(cfg.TempTopic); ok && m.feedFresh(cfg.TempTopic, tempFreshS) {
				if st.TempC == nil || v > *st.TempC {
					vv := v
					st.TempC = &vv
				}
			}
		}
	}
	if !running && st.TempC != nil {
		st.TempBlock = *st.TempC >= s.MaxTempC
	}

	rows := map[string]LpTelemetry{}
	if running {
		for _, r := range tel.Lps {
			rows[r.Key] = r
		}
	}
	for _, l := range m.lps {
		ls := l.lp.SuperchargeState()
		lc := c.Loadpoints[l.name]
		r, live := rows[l.name]
		v := LpView{
			Index: l.index, Name: l.name, Title: ls.Title, Vehicle: ls.Vehicle, Mode: ls.Mode,
			Priority: ls.Priority, Connected: ls.Connected, Charging: ls.Charging,
			Wants:   ls.Connected && ls.DemandA > 0 && ls.DemandA+1e-9 >= ls.MinA,
			DemandA: ls.DemandA, MinA: ls.MinA, MaxA: ls.MaxA, Phases: max(ls.Phases, 1),
			Paused: true, Fast: lc.Fast, Soc: ls.Soc, LimitSoc: ls.LimitSoc,
			Forecast: m.forecast(l, ls),
		}
		if until, ok := s.Supercharge[l.name]; ok {
			v.Supercharge = true
			if until > 0 {
				t := time.Unix(until, 0)
				v.SuperchargeUntil = &t
			}
		}
		if live {
			v.SetpointA = r.SetpointA
			v.AllocatedA = r.AllocatedA
			v.Paused = r.Paused
			v.Ops = r.Ops
			v.WaitS = r.WaitS
			v.MeasuredA = r.MeasuredA
			v.MeasuredRawA = r.MeasuredRawA
			v.MeasuredSrc = r.MeasuredSrc
			v.Clamped = r.Clamped
			now := m.now()
			v.StoodOffS = math.Round(m.ctl.StoodOff(l.name, &now))
		} else {
			v.MeasuredA = round(lastCycleAmps(ls, 230), 1)
			v.MeasuredRawA = v.MeasuredA
			v.MeasuredSrc = "evcc"
			v.Paused = !ls.Enabled
			v.SetpointA = int(ls.OfferedA)
		}
		if lc.MeasureTopic != "" {
			v.FeedAgeS = m.feedAge(lc.MeasureTopic)
		}
		st.Loadpoints = append(st.Loadpoints, v)
	}
	return st
}

func (m *Manager) publish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishLocked()
}

func (m *Manager) publishLocked() {
	if m.pub == nil {
		return
	}
	m.pub("supercharging", m.stateLocked())
	m.publishConfigLocked()
}

// publishConfigLocked publishes the configuration when it changed
func (m *Manager) publishConfigLocked() {
	if m.pub == nil {
		return
	}
	cfg := m.cfg
	cfg.Settings = m.cfg.Settings.Clone()
	raw, err := json.Marshal(cfg)
	if err != nil || string(raw) == m.lastConfig {
		return
	}
	m.lastConfig = string(raw)
	m.pub("superchargingConfig", cfg)
}

func (m *Manager) selftestAsync(delay time.Duration) {
	go func() {
		time.Sleep(delay)
		m.Selftest(context.Background())
	}()
}

// Selftest checks every external dependency and publishes the result
func (m *Manager) Selftest(ctx context.Context) []Check {
	m.mu.Lock()
	cfg := m.cfg
	cfg.Settings = m.cfg.Settings.Clone()
	lps := m.lps
	running := m.running
	m.mu.Unlock()

	var checks []Check
	add := func(name string, ok bool, format string, args ...any) {
		checks = append(checks, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, args...)})
	}

	add("Load management", cfg.Enabled, "%s", map[bool]string{true: "switched on", false: "switched off: nothing holds the house under the never-trip line"}[cfg.Enabled])

	if cfg.MeterURI == "" {
		add("Meter", false, "no meter configured")
	} else {
		rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		rd := NewShellyMeter(cfg.MeterURI).Read(rctx)
		cancel()
		if rd.OK {
			add("Meter", true, "%.2f kVA (%.0f W, %.0f var) at %.1f V", rd.VaKVA, rd.Watts, rd.Var, UsableVolts(rd.Volts))
			add("Meter latency", rd.Latency < 500*time.Millisecond, "%d ms", rd.Latency.Milliseconds())
			add("Reactive power", rd.Var != 0, "%s", map[bool]string{true: "reported, apparent power is computed properly", false: "not reported, apparent power equals real power"}[rd.Var != 0])
		} else {
			add("Meter", false, "%v", rd.Err)
		}
	}

	cv := cfg.Curve()
	add("Never-trip line", true, "%.2f kVA (%.2f × %.2f kVA contract, Q %.0f)", cv.ThresholdKVA(), cv.K, cv.SCkVA, cv.Q)

	add("Loadpoints", len(lps) > 0, "%d loadpoint(s)", len(lps))
	for _, l := range lps {
		st := l.lp.SuperchargeState()
		lc := cfg.Loadpoints[l.name]
		switch {
		case lc.MeasureTopic != "":
			if age := m.feedAge(lc.MeasureTopic); age != nil {
				add(st.Title+" current feed", true, "MQTT %s, last value %.0f s ago", lc.MeasureTopic, *age)
			} else {
				// on-change publishers are silent while a car is parked
				add(st.Title+" current feed", !st.Charging, "MQTT %s, nothing received yet", lc.MeasureTopic)
			}
		case lc.Fast:
			if cur, err := l.lp.SuperchargeCurrents(); err == nil {
				add(st.Title+" current", true, "read live from the charger: %.1f A", sumCurrents(cur))
			} else {
				add(st.Title+" current", false, "fast loadpoint, but the charger current cannot be read: %v", err)
			}
		default:
			add(st.Title+" current", true, "taken from the loadpoint's own cycle (treated as stale)")
		}
		add(st.Title+" range", st.MaxA >= st.MinA && st.MinA > 0, "%.0f-%.0f A, priority %d", st.MinA, st.MaxA, st.Priority)
		if lc.TempTopic != "" {
			if age := m.feedAge(lc.TempTopic); age != nil {
				v, _ := m.feed(lc.TempTopic)
				add(st.Title+" temperature", *age <= tempFreshS, "%.0f °C, %.0f s ago", v, *age)
			} else {
				add(st.Title+" temperature", true, "no reading yet: unknown is never treated as cool, bursting is not blocked by it")
			}
		}
	}

	s := cfg.Settings
	window := cv.BurstWindow(s.BurstKVA, s.BumpKVA, s.MarginS)
	if s.AnySupercharge() {
		add("Burst window", window >= 5.0, "%.1f s at %.2f kVA (bump %.2f kVA, margin %.0f s)", window, s.BurstKVA, s.BumpKVA, s.MarginS)
	}
	add("Baseline backstop", s.BaseStopCloseness > s.BaseAbortCloseness, "shed at %.2f, stop at %.2f closeness", s.BaseAbortCloseness, s.BaseStopCloseness)
	_ = running

	m.mu.Lock()
	m.checks = checks
	m.checkedAt = time.Now()
	failed := 0
	for _, c := range checks {
		if !c.OK {
			failed++
		}
	}
	m.mu.Unlock()
	if failed > 0 {
		m.log.WARN.Printf("self-test: %d of %d checks failed", failed, len(checks))
	} else {
		m.log.INFO.Printf("self-test passed (%d checks)", len(checks))
	}
	m.publish()
	return checks
}
