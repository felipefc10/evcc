package supercharge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
)

const (
	pollS          = 1.0              // control loop period while active
	idlePollS      = 10.0             // meter poll period while nothing is charging
	activeHoldS    = 60.0             // stay on the fast cadence this long after activity
	autoStopS      = 300.0            // stand down this long after the last charge
	armPollS       = 3.0              // standby check period
	stillbornS     = 5.0              // a burst shorter than this died on entry
	heartbeatS     = 25.0             // a loop silent this long is treated as dead by the loadpoints
	measureStaleS  = 300.0            // an on-change current feed is held this long
	tempFreshS     = 120.0            // a temperature older than this is unknown
	forecastWinS   = 600.0            // window of the delivered-power average
	forecastBucket = 10.0             // bucket width of that window
	burstLogKeep   = 200              // bursts kept
	traceKeep      = 180              // samples kept for diagnostics
	learnSaveEvery = 10 * time.Minute // persist the learner this often
	retryS         = 2.0              // retry pace of a failed actuation
	slowReadS      = 2.0              // live current read period of a slow charger
	liveFreshS     = 4.0              // a live current reading older than this is not used
)

// LpState is what the manager reads from one loadpoint
type LpState struct {
	Title          string
	Vehicle        string
	Mode           string
	Priority       int
	Connected      bool
	Charging       bool
	Enabled        bool
	DemandA        float64 // what the loadpoint's own mode logic asks for, 0 = not charging
	MinA           float64
	MaxA           float64
	Phases         int
	OfferedA       float64
	ChargePowerW   float64
	ChargeCurrents []float64
	Soc            float64
	LimitSoc       int
	RemainingWh    float64
	ChargedWh      float64
}

// Loadpoint is the loadpoint side of the integration
type Loadpoint interface {
	SuperchargeState() LpState
	// SuperchargeCurrents reads the phase currents live from the charger
	SuperchargeCurrents() ([]float64, error)
	// SuperchargeApply commands the charger, 0 disables it
	SuperchargeApply(amps int) error
}

// LpConfig configures one loadpoint
type LpConfig struct {
	// Fast means the charger actuates within about a second and its current may be read live every second
	Fast bool `json:"fast"`
	// MeasureTopic is an MQTT topic publishing this car's measured current (or power)
	MeasureTopic string `json:"measureTopic"`
	// MeasureUnit is A or W
	MeasureUnit string `json:"measureUnit"`
	// TempTopic is an MQTT topic publishing the charger temperature in °C
	TempTopic string `json:"tempTopic"`
	// MaxTopic is an MQTT topic publishing the most current the car accepts right now, in A.
	// Commands above it are refused by some cars' integrations, so the loop never plans above it.
	MaxTopic string `json:"maxTopic"`
}

// Config is the persisted configuration, settings included
type Config struct {
	Enabled bool `json:"enabled"`
	// FailsafeA limits every loadpoint while load management is switched off, 0 for no limit
	FailsafeA float64 `json:"failsafeA"`
	// MeterURI is a Shelly EM read directly, empty uses evcc's grid meter
	MeterURI    string              `json:"meterUri"`
	Q           float64             `json:"q"`
	K           float64             `json:"k"`
	ContractKVA float64             `json:"contractKva"`
	Loadpoints  map[string]LpConfig `json:"loadpoints"`
	Settings    Settings            `json:"settings"`
}

// DefaultConfig returns the configuration used before anything was saved
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		FailsafeA:   6,
		Q:           50,
		K:           1.2,
		ContractKVA: 3.45,
		Loadpoints:  map[string]LpConfig{},
		Settings:    DefaultSettings(),
	}
}

// Curve returns the configured meter curve
func (c Config) Curve() Curve {
	return Curve{Q: c.Q, K: c.K, SCkVA: c.ContractKVA}
}

func (c *Config) sanitize() {
	c.Q = math.Min(math.Max(c.Q, 1), 200)
	c.K = math.Min(math.Max(c.K, 1), 2)
	c.ContractKVA = math.Min(math.Max(c.ContractKVA, 1), 20)
	c.MeterURI = strings.TrimSpace(c.MeterURI)
	c.FailsafeA = math.Min(math.Max(c.FailsafeA, 0), 32)
	if c.Loadpoints == nil {
		c.Loadpoints = map[string]LpConfig{}
	}
	for k, v := range c.Loadpoints {
		v.MeasureTopic = strings.TrimSpace(v.MeasureTopic)
		v.TempTopic = strings.TrimSpace(v.TempTopic)
		v.MaxTopic = strings.TrimSpace(v.MaxTopic)
		if v.MeasureUnit != "W" {
			v.MeasureUnit = "A"
		}
		c.Loadpoints[k] = v
	}
	c.Settings.Sanitize()
}

// Store persists configuration and learned state
type Store interface {
	Load(key string, v any) error
	Save(key string, v any) error
}

// Subscriber subscribes to an MQTT topic
type Subscriber func(topic string, cb func(string)) error

// Plant is what the manager reads from the site
type Plant struct {
	Grid api.Meter      // evcc's grid meter, nil when none
	PV   func() float64 // solar production in W, nil when no PV meter
}

type managedLp struct {
	index int
	name  string
	lp    Loadpoint

	mu        sync.Mutex
	want      *int // latest setpoint to apply, nil when nothing is owed
	applied   *int
	failedAt  time.Time
	failures  int
	wake      chan struct{}
	commanded bool // the controller has commanded it this run

	live   float64 // last live current reading, A
	liveAt time.Time
	liveOK bool

	delivered [2]float64 // Wh, s
	lastT     float64
	recent    [][3]float64 // slot, Wh, s
	standSaid int64
}

type feedValue struct {
	v  float64
	at time.Time
}

// Manager runs the control law against the plant
type Manager struct {
	log   *util.Logger
	store Store
	sub   Subscriber
	pub   func(string, any)
	plant Plant

	mu      sync.Mutex
	cfg     Config
	ctl     *Controller
	learner *Learner
	meter   Meter
	lps     []*managedLp
	start   time.Time

	armed        bool
	running      bool
	armedWhy     string
	unsupported  string // why the installation cannot be balanced, load management then acts as off
	lastCharging time.Time
	activeUntil  float64
	pollMode     string
	heartbeat    time.Time
	lastReading  Reading
	meterW       float64
	meterVar     float64
	meterVolts   float64
	status       string
	lastError    string
	startedAt    time.Time
	stillborn    int
	stoodDown    string
	bursts       []BurstRecord
	trace        []map[string]any
	lastPhase    Phase
	warnedClaim  bool
	lastSave     time.Time
	checks       []Check
	lastConfig   string
	checkedAt    time.Time

	feeds   map[string]feedValue
	feedsMu sync.Mutex
	subbed  map[string]bool

	wake chan struct{}
}

// Check is one self-test row
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Info   bool   `json:"info,omitempty"` // passes, but is a limitation worth knowing
}

// NewManager creates the manager. lps are in site order with their config names.
func NewManager(log *util.Logger, store Store, sub Subscriber, pub func(string, any), plant Plant) *Manager {
	m := &Manager{
		log:      log,
		store:    store,
		sub:      sub,
		pub:      pub,
		plant:    plant,
		cfg:      DefaultConfig(),
		learner:  NewLearner(),
		feeds:    map[string]feedValue{},
		subbed:   map[string]bool{},
		wake:     make(chan struct{}, 1),
		start:    time.Now(),
		pollMode: "standby",
		status:   "standby",
	}
	m.learner.Warn = log.WARN.Printf

	// saved values are merged over the defaults, so a document missing a field keeps its default
	cfg := DefaultConfig()
	if store != nil {
		if err := store.Load(keyConfig, &cfg); err == nil {
			m.cfg = cfg
		}
	}
	m.cfg.sanitize()

	var raw json.RawMessage
	if store != nil && store.Load(keyLearned, &raw) == nil && len(raw) > 0 {
		if n, err := m.learner.Load(raw); err != nil {
			log.WARN.Printf("learned behaviour: %v", err)
		} else {
			log.INFO.Printf("learned behaviour for %d loadpoint/vehicle pairs", n)
		}
	}
	var bursts []BurstRecord
	if store != nil && store.Load(keyBursts, &bursts) == nil {
		m.bursts = bursts
	}

	m.ctl = NewController(m.cfg.Curve(), &m.cfg.Settings)
	m.meter = m.meterFor(m.cfg)
	return m
}

// meterFor returns the reader of the configured meter
func (m *Manager) meterFor(cfg Config) Meter {
	switch {
	case cfg.MeterURI != "":
		return NewShellyMeter(cfg.MeterURI)
	case m.plant.Grid != nil:
		return NewGridMeter(m.plant.Grid)
	default:
		return noMeter{}
	}
}

const (
	keyConfig  = "supercharging"
	keyLearned = "supercharging.learned"
	keyBursts  = "supercharging.bursts"
)

// AddLoadpoint registers a loadpoint, in site order
func (m *Manager) AddLoadpoint(index int, name string, lp Loadpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mlp := &managedLp{index: index, name: name, lp: lp, wake: make(chan struct{}, 1)}
	m.lps = append(m.lps, mlp)
}

func (m *Manager) now() float64 {
	return time.Since(m.start).Seconds()
}

// Enabled reports whether load management is switched on
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.Enabled
}

// Owns reports whether the control loop is balancing right now and answering.
// While it does, it is the whole-house limit and evcc's own circuit limits step aside;
// otherwise they keep holding the house as they would without load management.
func (m *Manager) Owns() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.Enabled && m.armed && m.running && time.Since(m.heartbeat) <= heartbeatS*time.Second
}

// Clamp is called by a loadpoint for every current it is about to apply on its own cycle.
// It returns the current the loadpoint may actually apply.
func (m *Manager) Clamp(name string, current, minA float64, enabled bool, offered float64) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled || m.unsupported != "" {
		if m.cfg.FailsafeA > 0 {
			return math.Min(current, math.Max(m.cfg.FailsafeA, minA))
		}
		return current
	}

	// hold what it has: never start or raise a car the loop has not spoken for
	hold := func() float64 {
		if !enabled {
			return 0
		}
		if offered <= 0 {
			return math.Min(current, math.Max(minA, 0))
		}
		return math.Min(current, math.Max(offered, minA))
	}

	if !m.armed {
		if current > 0 {
			m.requestArm("a loadpoint wants to charge")
		}
		return hold()
	}
	if !m.running || time.Since(m.heartbeat) > heartbeatS*time.Second {
		// the loop is not (yet) answering: never more than the minimum
		return math.Min(hold(), math.Max(minA, 0))
	}
	for _, l := range m.lps {
		if l.name != name {
			continue
		}
		if !l.commanded {
			return hold()
		}
		return math.Min(current, float64(m.ctl.SetpointOf(name)))
	}
	return hold()
}

func (m *Manager) requestArm(why string) {
	if m.armedWhy == "" {
		m.armedWhy = why
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Run is the manager loop. It returns when ctx is done.
func (m *Manager) Run(ctx context.Context) {
	for _, l := range m.lps {
		go m.actuator(ctx, l)
	}
	m.subscribe()
	m.selftestAsync(8 * time.Second)

	saveTick := time.NewTicker(time.Minute)
	defer saveTick.Stop()
	for {
		m.mu.Lock()
		armed := m.armed
		enabled := m.cfg.Enabled
		m.mu.Unlock()

		if !enabled || !armed {
			m.standby(ctx)
		} else {
			m.runArmed(ctx)
		}

		select {
		case <-ctx.Done():
			m.saveLearned(true)
			return
		default:
		}
	}
}

// standby waits for something to charge, without polling the meter
func (m *Manager) standby(ctx context.Context) {
	m.publish()
	t := time.NewTimer(time.Duration(armPollS * float64(time.Second)))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return
	case <-m.wake:
	case <-t.C:
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.burstClockLocked()
	if !m.cfg.Enabled {
		m.armedWhy = ""
		m.status = "load management is switched off"
		return
	}
	states := make([]LpState, len(m.lps))
	for i, l := range m.lps {
		states[i] = l.lp.SuperchargeState()
	}
	if m.checkPhasesLocked(states) {
		m.armedWhy = ""
		m.status = m.unsupported
		return
	}
	why := m.armedWhy
	for _, st := range states {
		if st.Charging {
			why = "a loadpoint is charging"
			break
		}
		if st.Connected && st.DemandA > 0 && why == "" {
			why = "a loadpoint wants to charge"
		}
	}
	if why != "" {
		m.armLocked(why)
	}
	m.status = "standby"
}

func (m *Manager) armLocked(why string) {
	m.armed = true
	m.armedWhy = ""
	m.lastCharging = time.Now()
	m.stillborn = 0
	m.stoodDown = ""
	m.warnedClaim = false
	m.learner.ForgetWarnings()
	for _, l := range m.lps {
		l.delivered = [2]float64{}
		l.recent = nil
		l.standSaid = 0
	}
	s := m.cfg.Settings
	m.log.INFO.Printf("ARMED (%s): balancing all loadpoints, supercharge %s, burst %.2f kVA, bump %.2f, margin %.0f s",
		why, m.scopeText(), s.BurstKVA, s.BumpKVA, s.MarginS)
}

func (m *Manager) scopeText() string {
	if len(m.cfg.Settings.Supercharge) == 0 {
		return "off"
	}
	var names []string
	for _, l := range m.lps {
		if _, ok := m.cfg.Settings.Supercharge[l.name]; ok {
			names = append(names, l.lp.SuperchargeState().Title)
		}
	}
	return strings.Join(names, ", ")
}

// checkPhasesLocked refuses installations the single-phase trip model does not describe
func (m *Manager) checkPhasesLocked(states []LpState) bool {
	why := ""
	for _, st := range states {
		if st.Phases > 1 {
			why = fmt.Sprintf("%s charges on %d phases: load management balances single-phase installations only, every loadpoint is held to the fail-safe limit", st.Title, st.Phases)
			break
		}
	}
	if why != m.unsupported {
		if why != "" {
			m.log.WARN.Println(why)
		}
		m.unsupported = why
	}
	return why != ""
}

// runArmed runs the control loop until it stands down
func (m *Manager) runArmed(ctx context.Context) {
	m.mu.Lock()
	m.ctl.SetCurve(m.cfg.Curve())
	m.ctl.Start(m.now(), false)
	for _, l := range m.lps {
		l.commanded = false
	}
	m.running = true
	m.startedAt = time.Now()
	m.activeUntil = m.now() + activeHoldS
	m.heartbeat = time.Now()
	m.log.INFO.Printf("balancing: poll %.0f s (idle %.0f s), line %.2f kVA", pollS, idlePollS, m.cfg.Curve().ThresholdKVA())
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.ctl.Stop()
		for _, l := range m.lps {
			l.commanded = false
			l.mu.Lock()
			l.want = nil
			l.mu.Unlock()
		}
		m.status = "standby"
		m.mu.Unlock()
		m.saveLearned(true)
		// hand every loadpoint back to its own cycle
		for _, l := range m.lps {
			l.lp.SuperchargeApply(-1)
		}
	}()

	nextBeat := time.Now().Add(time.Minute)
	for {
		m.mu.Lock()
		stop := !m.armed || !m.cfg.Enabled
		active := m.now() <= m.activeUntil
		m.mu.Unlock()
		if stop {
			return
		}

		m.step(ctx)

		if time.Now().After(nextBeat) {
			nextBeat = time.Now().Add(time.Minute)
			m.heartbeatLog()
		}
		if time.Since(m.lastSave) > learnSaveEvery {
			m.saveLearned(false)
		}

		period := pollS
		if !active {
			period = idlePollS
		}
		t := time.NewTimer(time.Duration(period * float64(time.Second)))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		case <-m.wake:
			t.Stop()
		}
	}
}

// step runs one control step
func (m *Manager) step(ctx context.Context) {
	m.mu.Lock()
	meter := m.meter
	m.mu.Unlock()
	rctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	rd := meter.Read(rctx)
	cancel()

	var pvKVA float64
	if m.plant.PV != nil {
		pvKVA = math.Max(m.plant.PV(), 0) / 1000.0
	}

	states := make([]LpState, len(m.lps))
	for i, l := range m.lps {
		states[i] = l.lp.SuperchargeState()
	}
	// live charger currents, outside the lock: fast chargers every step, the others
	// every slowReadS, since a reading from the loadpoint's own cycle is too old to steer by
	m.mu.Lock()
	cfgs := make([]LpConfig, len(m.lps))
	for i, l := range m.lps {
		cfgs[i] = m.cfg.Loadpoints[l.name]
	}
	m.mu.Unlock()
	liveErr := make([]error, len(m.lps))
	for i, l := range m.lps {
		if cfgs[i].MeasureTopic != "" {
			continue
		}
		if !cfgs[i].Fast && time.Since(l.liveAt) < slowReadS*time.Second {
			continue
		}
		cur, err := l.lp.SuperchargeCurrents()
		if err != nil {
			liveErr[i] = err
			l.liveOK = false
			continue
		}
		l.live, l.liveAt, l.liveOK = sumCurrents(cur), time.Now(), true
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.checkPhasesLocked(states) {
		m.armed = false
		m.status = m.unsupported
		return
	}
	now := m.now()
	m.heartbeat = time.Now()
	m.lastReading = rd
	if rd.OK {
		m.meterW, m.meterVar, m.meterVolts = rd.Watts, rd.Var, UsableVolts(rd.Volts)
		m.lastError = ""
	} else if rd.Err != nil {
		m.lastError = rd.Err.Error()
	}

	m.burstClockLocked()

	volts := m.meterVolts
	if volts <= 0 {
		volts = 230
	}
	s := &m.cfg.Settings
	lps := make([]LpSample, 0, len(m.lps))
	blindCharger := false
	for i, l := range m.lps {
		st := states[i]
		c := cfgs[i]
		phases := max(st.Phases, 1)
		measured, stale, src := 0.0, true, "evcc"
		switch {
		case c.MeasureTopic != "":
			if v, ok := m.feed(c.MeasureTopic); ok {
				if c.MeasureUnit == "W" {
					v = v / volts
				}
				measured, src = math.Max(v, 0), "mqtt"
				stale = !m.feedFresh(c.MeasureTopic, measureStaleS)
			} else {
				measured, stale, src = lastCycleAmps(st, volts), true, "evcc"
			}
		case l.liveOK && time.Since(l.liveAt) <= liveFreshS*time.Second:
			measured, stale, src = l.live, false, "charger"
		default:
			measured, stale, src = lastCycleAmps(st, volts), true, "evcc"
			// a fast charger that stops answering while charging leaves the loop blind
			if c.Fast && liveErr[i] != nil && !errors.Is(liveErr[i], api.ErrNotAvailable) {
				if st.Charging || st.Enabled {
					blindCharger = true
				}
				m.lastError = fmt.Sprintf("%s current: %v", st.Title, liveErr[i])
			}
		}

		wants := st.Connected && st.DemandA > 0 && st.DemandA+1e-9 >= st.MinA
		minA := st.MinA * float64(phases)
		maxA := m.carMaxA(c, st) * float64(phases)
		if wants && st.DemandA > 0 {
			maxA = math.Min(maxA, math.Floor(st.DemandA+1e-9)*float64(phases))
		}
		key := LearnKey(l.name, st.Vehicle)
		lps = append(lps, LpSample{
			Key: l.name, Name: st.Title, Priority: st.Priority,
			Connected: st.Connected, Wants: wants, Running: st.Charging,
			MeasuredA: measured, Stale: stale, Src: src,
			MinA: minA, MaxA: maxA,
			Fast: c.Fast, Direct: !c.Fast,
			Supercharge: s.MaySupercharge(l.name),
			Commandable: true, Finished: !wants,
			RampAPerS:    m.learner.RampDown(key, CarRampAPerS),
			LatencyS:     m.learner.Latency(key, CmdLatencyS),
			LatencyUpS:   m.learner.LatencyUp(key, CmdLatencyS),
			EntryOffsetA: m.learner.EntryOffset(key),
		})
		m.noteDelivery(l, lps[len(lps)-1], volts, now)
	}

	var temp *float64
	for _, c := range cfgs {
		if c.TempTopic == "" || !m.feedFresh(c.TempTopic, tempFreshS) {
			continue
		}
		if v, ok := m.feed(c.TempTopic); ok && (temp == nil || v > *temp) {
			vv := v
			temp = &vv
		}
	}

	ok := rd.OK && !blindCharger
	blindWhy := ""
	if !rd.OK {
		blindWhy = "meter"
	} else if blindCharger {
		blindWhy = "wallbox"
	}
	sm := &Sample{
		T: now, OK: ok, Lps: lps, BlindWhy: blindWhy,
		BurstWindowOpen: s.AnySupercharge() && m.stoodDown == "",
		SourceOhm:       m.learner.SourceOhm(),
		TempC:           temp,
		Volt:            230,
		PvKVA:           pvKVA,
	}
	if ok {
		sm.VaKVA, sm.Volt, sm.Watts, sm.Var = rd.VaKVA, UsableVolts(rd.Volts), rd.Watts, rd.Var
	}

	for _, lp := range lps {
		m.ctl.External(lp.Key, sm.T, lp.Connected && lp.Wants, int(lp.MaxA))
	}

	cmd := m.ctl.Step(sm)
	tel := m.ctl.Tel

	// learner observations
	for i := range lps {
		st := states[i]
		m.learner.Observe(LearnKey(lps[i].Key, st.Vehicle), sm.T, lps[i].MeasuredA)
	}

	// actuation
	byKey := map[string]int{}
	for i, l := range m.lps {
		byKey[l.name] = i
	}
	for _, c := range cmd.Lps {
		if c.Act == ActNone {
			continue
		}
		i, ok := byKey[c.Key]
		if !ok {
			continue
		}
		l := m.lps[i]
		amps := 0
		if c.Act == ActSet {
			m.learner.Command(LearnKey(c.Key, states[i].Vehicle), sm.T, lps[i].MeasuredA, float64(c.Amps), states[i].Title)
			phases := max(states[i].Phases, 1)
			amps = c.Amps / phases
		}
		l.commanded = true
		m.log.DEBUG.Printf("%s: %s %d A (%s)", states[i].Title, c.Act, amps, c.Reason)
		l.setWant(amps, c.Repeat)
	}

	// stand-downs
	for _, l := range m.lps {
		if until, ok := tel.StoodOff[l.name]; ok && until != l.standSaid {
			l.standSaid = until
			m.log.INFO.Printf("%s was offered current and drew nothing for %.0f s, standing it down for %.0f s and giving its share to the other car (expected when a car reaches its own charge limit)",
				nameOf(sm, l.name), NoUptakeS, m.ctl.StoodOff(l.name, &sm.T))
		}
	}

	if tel.MeterDisagreements > 0 && !m.warnedClaim {
		m.warnedClaim = true
		m.log.WARN.Printf("meter disagreement: the cars claim %.2f kVA of a %.2f kVA house measurement, credited %.2f kVA. The meter is master.",
			tel.CarClaimKVA, sm.VaKVA, tel.CarCreditKVA)
	}

	// activity and stand-down
	carsPresent := false
	anyCharging := false
	for _, lp := range lps {
		if lp.Connected && lp.Wants {
			carsPresent = true
		}
		if lp.Running {
			anyCharging = true
		}
	}
	if carsPresent || anyCharging || m.ctl.Phase == PhaseBurst {
		m.activeUntil = now + activeHoldS
	}
	if now <= m.activeUntil {
		m.pollMode = "active"
	} else {
		m.pollMode = "idle"
	}
	if anyCharging || carsPresent {
		m.lastCharging = time.Now()
	} else if time.Since(m.lastCharging) >= autoStopS*time.Second {
		m.armed = false
		m.log.INFO.Printf("DISARMED (nothing charging for %.0f min): standby, the meter is not polled", autoStopS/60)
	}

	if tel.Phase != m.lastPhase {
		var parts []string
		for _, r := range tel.Lps {
			parts = append(parts, fmt.Sprintf("%s %d A", r.Name, r.SetpointA))
		}
		m.log.INFO.Printf("%s -> %s, %.2f kVA, %s", orStart(m.lastPhase), tel.Phase, sm.VaKVA, strings.Join(parts, ", "))
		m.lastPhase = tel.Phase
	}

	m.addTrace(sm, cmd)
	m.flushRecords(states)
	m.status = tel.Note
	m.publishLocked()
}

func orStart(p Phase) string {
	if p == "" {
		return "start"
	}
	return string(p)
}

func sumCurrents(c []float64) float64 {
	var s float64
	for _, v := range c {
		if v > 0 && !math.IsNaN(v) {
			s += v
		}
	}
	return s
}

// lastCycleAmps is the loadpoint's own last-cycle reading, old by construction
func lastCycleAmps(st LpState, volts float64) float64 {
	if len(st.ChargeCurrents) > 0 {
		return sumCurrents(st.ChargeCurrents)
	}
	if st.ChargePowerW > 0 {
		return st.ChargePowerW / UsableVolts(volts)
	}
	return 0
}

func (l *managedLp) setWant(amps int, repeat bool) {
	l.mu.Lock()
	if repeat || l.want == nil || *l.want != amps {
		a := amps
		l.want = &a
		l.failures = 0
		l.failedAt = time.Time{}
	}
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// actuator applies the latest setpoint to one loadpoint, off the control loop,
// newest wins, retried until it lands
func (m *Manager) actuator(ctx context.Context, l *managedLp) {
	for {
		l.mu.Lock()
		var want *int
		if l.want != nil {
			w := *l.want
			want = &w
		}
		wait := time.Duration(0)
		if want != nil && !l.failedAt.IsZero() {
			wait = time.Until(l.failedAt.Add(time.Duration(retryS * float64(time.Second))))
		}
		l.mu.Unlock()

		if want == nil || wait > 0 {
			d := time.Hour
			if wait > 0 {
				d = wait
			}
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-l.wake:
				t.Stop()
			case <-t.C:
			}
			continue
		}

		err := l.lp.SuperchargeApply(*want)

		l.mu.Lock()
		if err != nil {
			l.failures++
			l.failedAt = time.Now()
			if l.failures == 1 || l.failures%30 == 0 {
				m.log.WARN.Printf("loadpoint %d: applying %d A failed (%d times): %v", l.index+1, *want, l.failures, err)
			}
		} else if l.want != nil && *l.want == *want {
			a := *want
			l.applied = &a
			l.want = nil
			l.failures = 0
			l.failedAt = time.Time{}
		}
		l.mu.Unlock()
	}
}

func (m *Manager) noteDelivery(l *managedLp, lp LpSample, volts, now float64) {
	dt := 0.0
	if l.lastT > 0 {
		dt = now - l.lastT
	}
	l.lastT = now
	// a pause or a restart is not chargeable time
	if !(lp.Connected && lp.Wants) || dt <= 0 || dt > 60 {
		return
	}
	wh := math.Max(lp.MeasuredA, 0) * volts * dt / 3600.0
	l.delivered[0] += wh
	l.delivered[1] += dt
	slot := now - math.Mod(now, forecastBucket)
	if n := len(l.recent); n > 0 && l.recent[n-1][0] == slot {
		l.recent[n-1][1] += wh
		l.recent[n-1][2] += dt
	} else {
		l.recent = append(l.recent, [3]float64{slot, wh, dt})
	}
	for len(l.recent) > 0 && now-l.recent[0][0] > forecastWinS {
		l.recent = l.recent[1:]
	}
}

// Forecast is the delivered-power based charge forecast of one loadpoint
type Forecast struct {
	AvgKW        float64    `json:"avgKw"`
	DeliveredKWh float64    `json:"deliveredKwh"`
	RemainingKWh *float64   `json:"remainingKwh"`
	EtaS         *float64   `json:"etaS"`
	EtaAt        *time.Time `json:"etaAt"`
}

func (m *Manager) forecast(l *managedLp, st LpState) Forecast {
	now := m.now()
	var rWh, rS float64
	for _, b := range l.recent {
		if now-b[0] <= forecastWinS {
			rWh += b[1]
			rS += b[2]
		}
	}
	avgW := 0.0
	if rS > 60 {
		avgW = rWh * 3600.0 / rS
	}
	f := Forecast{AvgKW: round(avgW/1000.0, 2), DeliveredKWh: round(l.delivered[0]/1000.0, 2)}
	if st.RemainingWh > 0 {
		r := round(st.RemainingWh/1000.0, 2)
		f.RemainingKWh = &r
		if avgW > 200 {
			eta := math.Round(st.RemainingWh * 3600.0 / avgW)
			f.EtaS = &eta
			at := time.Now().Add(time.Duration(eta) * time.Second)
			f.EtaAt = &at
		}
	}
	return f
}

// burstClockLocked stands down every ticked loadpoint whose time has come
func (m *Manager) burstClockLocked() {
	s := &m.cfg.Settings
	now := time.Now().Unix()
	changed := false
	for key, until := range s.Supercharge {
		known := false
		for _, l := range m.lps {
			if l.name == key {
				known = true
			}
		}
		if len(m.lps) > 0 && !known {
			delete(s.Supercharge, key)
			changed = true
			continue
		}
		if until > 0 && until <= now {
			delete(s.Supercharge, key)
			changed = true
			m.log.INFO.Printf("SUPERCHARGING STOOD DOWN for %s: the time it was armed until has arrived; balancing carries on", m.titleOf(key))
		}
	}
	if changed {
		m.saveConfigLocked()
	}
}

func (m *Manager) titleOf(name string) string {
	for _, l := range m.lps {
		if l.name == name {
			return l.lp.SuperchargeState().Title
		}
	}
	return name
}

func (m *Manager) flushRecords(states []LpState) {
	for len(m.ctl.Records) > 0 {
		rec := m.ctl.Records[0]
		m.ctl.Records = m.ctl.Records[1:]
		rec.At = time.Now().Format(time.RFC3339)
		var titles []string
		for _, k := range strings.Split(rec.Loadpoint, ",") {
			if k != "" {
				titles = append(titles, m.titleOf(k))
			}
		}
		rec.LoadpointTitle = strings.Join(titles, ", ")
		m.bursts = append(m.bursts, rec)
		if len(m.bursts) > burstLogKeep {
			m.bursts = m.bursts[len(m.bursts)-burstLogKeep:]
		}
		plural := "s"
		if rec.Cmds == 1 {
			plural = ""
		}
		contactor := ""
		if rec.UsedContactor {
			contactor = ", PAUSED A LOADPOINT"
		}
		m.log.INFO.Printf("burst #%d ended: %s, %.1f s of %.1f s, peak %.2f kVA, closeness %.2f, %d current change%s%s",
			m.ctl.Tel.Bursts, rec.Exit, rec.LastedS, rec.WindowS, rec.PeakVaKVA, rec.PeakCloseness, rec.Cmds, plural, contactor)

		// stillborn stand-down
		if rec.WallS >= stillbornS {
			m.stillborn = 0
		} else if rec.Exit != "no draw" {
			m.stillborn++
			if StillbornLimit > 0 && m.stillborn >= StillbornLimit && m.stoodDown == "" {
				m.stoodDown = rec.Exit
				if m.stoodDown == "" {
					m.stoodDown = "it ended immediately"
				}
				m.log.WARN.Printf("SUPERCHARGING STOOD DOWN: %d bursts in a row ended within %.0f s (%s); baseline balancing continues", m.stillborn, stillbornS, m.stoodDown)
			}
		}

		// entry learning
		keys := []string{}
		for _, k := range strings.Split(rec.Loadpoint, ",") {
			if k != "" {
				keys = append(keys, k)
			}
		}
		if rec.SettledVaKVA > 0 && rec.SettledVolts > 0 && len(keys) == 1 {
			errA := rec.EntryOffsetA + (rec.TargetKVA-rec.SettledVaKVA)*1000.0/rec.SettledVolts
			m.learner.Entry(LearnKey(keys[0], m.vehicleOf(keys[0], states)), errA, m.titleOf(keys[0]))
		}
		hiV, hiS := rec.PeakVolts, rec.PeakVaKVA
		if rec.SettledVaKVA > 0 && rec.SettledVolts > 0 {
			hiV, hiS = rec.SettledVolts, rec.SettledVaKVA
		}
		m.learner.Supply(rec.BaseVolts, rec.BaseVaKVA, hiV, hiS)
		if m.store != nil {
			if err := m.store.Save(keyBursts, m.bursts); err != nil {
				m.log.WARN.Printf("persist bursts: %v", err)
			}
		}
	}
}

func (m *Manager) vehicleOf(name string, states []LpState) string {
	for i, l := range m.lps {
		if l.name == name && i < len(states) {
			return states[i].Vehicle
		}
	}
	return ""
}

func (m *Manager) addTrace(sm *Sample, cmd Command) {
	tel := m.ctl.Tel
	cars := map[string]float64{}
	sp := map[string]int{}
	for _, lp := range sm.Lps {
		cars[lp.Key] = round(lp.MeasuredA, 1)
	}
	for _, r := range tel.Lps {
		sp[r.Key] = r.SetpointA
	}
	m.trace = append(m.trace, map[string]any{
		"t": time.Now().Format("15:04:05"), "phase": tel.Phase, "ok": sm.OK,
		"va": round(sm.VaKVA, 3), "v": round(sm.Volt, 1), "house": tel.HouseKVA,
		"budget": tel.BudgetA, "trim": tel.TrimA, "cars": cars, "sp": sp,
		"ti": round(tel.TiS, 1), "close": round(tel.Closeness, 3),
		"act": cmd.Act, "why": cmd.Reason,
	})
	if len(m.trace) > traceKeep {
		m.trace = m.trace[len(m.trace)-traceKeep:]
	}
}

func (m *Manager) heartbeatLog() {
	m.mu.Lock()
	defer m.mu.Unlock()
	tel := m.ctl.Tel
	var parts []string
	for _, r := range tel.Lps {
		parts = append(parts, fmt.Sprintf("%s=%dA/p%d", r.Name, r.SetpointA, r.Priority))
	}
	blind := ""
	if m.lastReading.Err != nil {
		blind = ", BLIND"
	}
	m.log.INFO.Printf("%s, %.2f kVA (line %.2f), house %.2f, %s, close %.2f, %d bursts, %d pauses, meter %d ms%s",
		tel.Phase, tel.VaKVA, m.cfg.Curve().ThresholdKVA(), tel.HouseKVA, strings.Join(parts, " "),
		tel.Closeness, tel.Bursts, tel.ContactorOps, m.lastReading.Latency.Milliseconds(), blind)
}

func (m *Manager) saveLearned(force bool) {
	m.mu.Lock()
	dirty := m.learner.Dirty
	var raw []byte
	var err error
	if dirty || force {
		raw, err = json.Marshal(m.learner)
	}
	m.lastSave = time.Now()
	m.mu.Unlock()
	if err != nil || raw == nil || m.store == nil {
		return
	}
	if err := m.store.Save(keyLearned, json.RawMessage(raw)); err != nil {
		m.log.WARN.Printf("persist learned behaviour: %v", err)
		return
	}
	m.mu.Lock()
	m.learner.Dirty = false
	m.mu.Unlock()
}

func (m *Manager) saveConfigLocked() {
	if m.store == nil {
		return
	}
	if err := m.store.Save(keyConfig, m.cfg); err != nil {
		m.log.ERROR.Printf("persist load management settings: %v", err)
	}
}

// subscribe ensures every configured MQTT feed is subscribed
func (m *Manager) subscribe() {
	if m.sub == nil {
		return
	}
	m.mu.Lock()
	var topics []string
	for _, c := range m.cfg.Loadpoints {
		for _, t := range []string{c.MeasureTopic, c.TempTopic, c.MaxTopic} {
			if t != "" && !m.subbed[t] {
				topics = append(topics, t)
			}
		}
	}
	m.mu.Unlock()
	for _, topic := range topics {
		err := m.sub(topic, func(payload string) {
			v, err := strconv.ParseFloat(strings.TrimSpace(payload), 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return
			}
			m.feedsMu.Lock()
			m.feeds[topic] = feedValue{v: v, at: time.Now()}
			m.feedsMu.Unlock()
		})
		if err != nil {
			m.log.WARN.Printf("subscribe %s: %v", topic, err)
			continue
		}
		m.mu.Lock()
		m.subbed[topic] = true
		m.mu.Unlock()
	}
}

// carMaxA is the loadpoint maximum, lowered to what the car says it accepts right now.
// A ceiling below the minimum is ignored: it is a parked car's reading, not a limit.
func (m *Manager) carMaxA(c LpConfig, st LpState) float64 {
	if c.MaxTopic == "" {
		return st.MaxA
	}
	if v, ok := m.feed(c.MaxTopic); ok && v >= st.MinA && v < st.MaxA {
		return math.Floor(v + 1e-9)
	}
	return st.MaxA
}

func (m *Manager) feed(topic string) (float64, bool) {
	m.feedsMu.Lock()
	defer m.feedsMu.Unlock()
	f, ok := m.feeds[topic]
	return f.v, ok
}

func (m *Manager) feedFresh(topic string, maxAgeS float64) bool {
	m.feedsMu.Lock()
	defer m.feedsMu.Unlock()
	f, ok := m.feeds[topic]
	return ok && time.Since(f.at) <= time.Duration(maxAgeS*float64(time.Second))
}

func (m *Manager) feedAge(topic string) *float64 {
	m.feedsMu.Lock()
	defer m.feedsMu.Unlock()
	f, ok := m.feeds[topic]
	if !ok {
		return nil
	}
	a := math.Round(time.Since(f.at).Seconds())
	return &a
}

// ---------------------------------------------------------------- settings API

// Settings returns a copy of the configuration
func (m *Manager) Config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.cfg
	c.Settings = m.cfg.Settings.Clone()
	c.Loadpoints = make(map[string]LpConfig, len(m.cfg.Loadpoints))
	for k, v := range m.cfg.Loadpoints {
		c.Loadpoints[k] = v
	}
	return c
}

// UpdateConfig merges a partial JSON document into the configuration and applies it at once
func (m *Manager) UpdateConfig(patch []byte) (Config, error) {
	m.mu.Lock()
	cur := m.cfg
	cur.Settings = m.cfg.Settings.Clone()
	raw, err := json.Marshal(cur)
	if err != nil {
		m.mu.Unlock()
		return Config{}, err
	}
	var merged map[string]any
	_ = json.Unmarshal(raw, &merged)
	var p map[string]any
	if err := json.Unmarshal(patch, &p); err != nil {
		m.mu.Unlock()
		return Config{}, fmt.Errorf("invalid json: %w", err)
	}
	deepMerge(merged, p)
	raw, _ = json.Marshal(merged)
	var next Config
	if err := json.Unmarshal(raw, &next); err != nil {
		m.mu.Unlock()
		return Config{}, fmt.Errorf("invalid settings: %w", err)
	}
	// the supercharge ticks are owned by the per-loadpoint endpoint
	next.Settings.Supercharge = m.cfg.Settings.Supercharge
	if err := next.Settings.Validate(); err != nil {
		m.mu.Unlock()
		return Config{}, err
	}
	m.applyConfigLocked(next)
	res := m.cfg
	m.mu.Unlock()

	m.applied()
	return res, nil
}

// applyConfigLocked replaces the configuration and applies it at once
func (m *Manager) applyConfigLocked(next Config) {
	next.sanitize()
	wasEnabled := m.cfg.Enabled
	// Settings is shared with the controller by pointer: copy in place
	settings := next.Settings
	next.Settings = Settings{}
	m.cfg = next
	m.cfg.Settings = settings
	m.meter = m.meterFor(m.cfg)
	m.ctl.SetCurve(m.cfg.Curve())
	m.saveConfigLocked()
	if wasEnabled != m.cfg.Enabled {
		if m.cfg.Enabled {
			m.log.INFO.Println("load management switched ON")
		} else {
			m.log.WARN.Println("load management switched OFF: nothing is holding the house under the never-trip line")
			m.armed = false
		}
	}
}

// applied lets the loop and the UI pick up a new configuration
func (m *Manager) applied() {
	m.subscribe()
	m.kick()
	m.publish()
}

const backupVersion = 1

// Backup is everything load management keeps: installation, tuning, learned behaviour and bursts
type Backup struct {
	Version int             `json:"version"`
	Config  Config          `json:"config"`
	Learned json.RawMessage `json:"learned"`
	Bursts  []BurstRecord   `json:"bursts"`
}

// Export returns everything load management keeps, for moving it to another installation
func (m *Manager) Export() (Backup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	learned, err := json.Marshal(m.learner)
	if err != nil {
		return Backup{}, err
	}
	cfg := m.cfg
	cfg.Settings = m.cfg.Settings.Clone()
	return Backup{Version: backupVersion, Config: cfg, Learned: learned, Bursts: slices.Clone(m.bursts)}, nil
}

// Import replaces everything load management keeps with an export
func (m *Manager) Import(b Backup) error {
	if b.Version != backupVersion {
		return fmt.Errorf("unsupported export version %d", b.Version)
	}
	next := DefaultConfig()
	raw, err := json.Marshal(b.Config)
	if err != nil {
		return err
	}
	// merged over the defaults, like a saved configuration
	if err := json.Unmarshal(raw, &next); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}
	if next.Settings.Supercharge == nil {
		next.Settings.Supercharge = map[string]int64{}
	}
	if err := next.Settings.Validate(); err != nil {
		return err
	}
	learner := NewLearner()
	learner.Warn = m.log.WARN.Printf
	if len(b.Learned) > 0 {
		n, err := learner.Load(b.Learned)
		if err != nil {
			return fmt.Errorf("learned behaviour: %w", err)
		}
		if n == 0 && string(b.Learned) != "null" {
			var f learnFile
			if json.Unmarshal(b.Learned, &f) == nil && len(f.Keys) > 0 {
				return fmt.Errorf("learned behaviour: unsupported schema %d", f.Schema)
			}
		}
	}

	m.mu.Lock()
	m.applyConfigLocked(next)
	m.learner = learner
	m.learner.Dirty = true
	m.bursts = slices.Clone(b.Bursts)
	if m.store != nil {
		if err := m.store.Save(keyBursts, m.bursts); err != nil {
			m.log.WARN.Printf("persist bursts: %v", err)
		}
	}
	m.log.INFO.Printf("imported configuration, learned behaviour for %d loadpoint/vehicle pairs and %d bursts", len(m.learner.bags), len(m.bursts))
	m.mu.Unlock()

	m.saveLearned(true)
	m.applied()
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if sv, ok := v.(map[string]any); ok {
			if dv, ok := dst[k].(map[string]any); ok && k != "supercharge" {
				deepMerge(dv, sv)
				continue
			}
		}
		dst[k] = v
	}
}

func (m *Manager) kick() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// ResolveUntil turns "HH:MM", an ISO date-time, a number of hours or "" into an epoch second, 0 for indefinitely
func ResolveUntil(when string, now time.Time) (int64, error) {
	when = strings.TrimSpace(when)
	if when == "" {
		return 0, nil
	}
	if h, err := strconv.ParseFloat(when, 64); err == nil {
		if h <= 0 || h > 24*14 {
			return 0, fmt.Errorf("invalid duration: %s", when)
		}
		return now.Add(time.Duration(h * float64(time.Hour))).Unix(), nil
	}
	if t, err := time.ParseInLocation("15:04", when, now.Location()); err == nil {
		at := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if !at.After(now) {
			at = at.AddDate(0, 0, 1)
		}
		return at.Unix(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, when, now.Location()); err == nil {
			if !t.After(now) {
				return 0, fmt.Errorf("time is in the past: %s", when)
			}
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("invalid time: %s", when)
}

// SetSupercharge ticks or unticks one loadpoint (by 0-based index) for supercharging, until the given epoch
func (m *Manager) SetSupercharge(index int, on bool, until int64) error {
	m.mu.Lock()
	var name string
	for _, l := range m.lps {
		if l.index == index {
			name = l.name
		}
	}
	if name == "" {
		m.mu.Unlock()
		return fmt.Errorf("unknown loadpoint %d", index+1)
	}
	s := &m.cfg.Settings
	if on {
		s.Supercharge[name] = until
		m.stillborn = 0
		m.stoodDown = ""
		when := "indefinitely"
		if until > 0 {
			when = "until " + time.Unix(until, 0).Format("Mon 02 Jan 15:04")
		}
		m.log.INFO.Printf("supercharge ARMED for %s %s", m.titleOf(name), when)
	} else {
		delete(s.Supercharge, name)
		m.log.INFO.Printf("supercharge off for %s", m.titleOf(name))
	}
	m.saveConfigLocked()
	m.mu.Unlock()
	m.kick()
	m.publish()
	return nil
}

// Bursts returns the burst log, newest last
func (m *Manager) Bursts() []BurstRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.bursts)
}

// Diagnostics returns everything worth sending back after a run
func (m *Manager) Diagnostics() map[string]any {
	m.mu.Lock()
	cfg := m.cfg
	cfg.MeterURI = redactHost(cfg.MeterURI)
	res := map[string]any{
		"at":       time.Now().Format(time.RFC3339),
		"config":   cfg,
		"learned":  m.learner.All(),
		"bursts":   slices.Clone(m.bursts),
		"trace":    slices.Clone(m.trace),
		"selftest": slices.Clone(m.checks),
	}
	m.mu.Unlock()
	res["state"] = m.State()
	return res
}

func redactHost(uri string) string {
	if uri == "" {
		return ""
	}
	return "<address>"
}
