package supercharge

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// The control law. Pure state machine: samples in, commands out. No I/O.
//
//	BASE   cars held so total VA stays at or below k*SC - base margin
//	BURST  single clean step up to the burst target, held until the deadline
//	       or until something says stop. Never a gradual ramp.

const (
	baselineTTLS          = 1800.0 // how long a measured idle-house reading stays evidence
	motionA               = 0.4    // a reading moving more than this between samples is moving
	motionSettleS         = 3.0    // a loadpoint is assumed moving this long after a command
	warmupToFloors        = true   // the first allocation of a run is held to the floors
	burstExitClearanceKVA = 0.05   // how far under the line a burst exit must land
	burstExitMaxTrimA     = 2      // amps the exit may give up chasing that
	maxSourceOhm          = 1.5    // largest supply resistance acted on
	cadenceMaxS           = 120.0  // no cadence row may ask for longer

	houseFloorKVA      = 0.15 // absolute floor under house load
	houseBaselineFrac  = 0.7  // share of the measured idle house held back
	CarRampAPerS       = 2.0  // fallback ramp until the learner measures one
	CmdLatencyS        = 2.0  // fallback command latency, likewise
	floorSlackKVA      = 0.8  // how far over the line a floor hold may sit
	restartHysteresisA = 1.0  // spare amps required before resuming a car
	StillbornLimit     = 5    // failed bursts before supercharging stands down

	NoUptakeS     = 45.0  // how long a commanded car may draw nothing
	standOffMinS  = 60.0  // first stand-down
	standOffMaxS  = 900.0 // longest stand-down
	drawingA      = 0.5   // a car is still drawing above this
	restopS       = 10.0  // re-issue cadence of an ignored stop
	reissueS      = 20.0  // re-issue cadence of an ignored setpoint
	reissueMax    = 3     // times an ignored setpoint is re-issued
	overdrawA     = 2.0   // above its setpoint by this is a command that did not land
	entrySettleS  = 8.0   // before this a burst's own meter reading means nothing
	houseStableKV = 0.25  // how far the house may move for a burst to still testify
	tempHystC     = 2.0   // how far the charger must cool before a block lets go

	never = -1e9
)

// Phase is the controller phase
type Phase string

const (
	PhaseIdle  Phase = "idle"
	PhaseBase  Phase = "base"
	PhaseBurst Phase = "burst"
)

// Act is a per-loadpoint action
type Act string

const (
	ActNone Act = "none"
	ActSet  Act = "set"
	ActStop Act = "stop"
)

// LpSample is one loadpoint as the control law sees it
type LpSample struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Priority     int     `json:"priority"`
	Connected    bool    `json:"connected"`
	Wants        bool    `json:"wants"`
	Running      bool    `json:"running"`
	MeasuredA    float64 `json:"measured_a"`
	Stale        bool    `json:"stale"`
	Estimated    bool    `json:"estimated"`
	MinA         float64 `json:"min_a"`
	MaxA         float64 `json:"max_a"`
	Fast         bool    `json:"fast"`
	Direct       bool    `json:"direct"`
	Supercharge  bool    `json:"supercharge"`
	Commandable  bool    `json:"commandable"`
	Finished     bool    `json:"finished"`
	Src          string  `json:"src"`
	RampAPerS    float64 `json:"ramp_a_per_s"`
	LatencyS     float64 `json:"latency_s"`
	LatencyUpS   float64 `json:"latency_up_s"`
	EntryOffsetA float64 `json:"entry_offset_a"`
}

// Sample is one control step's input
type Sample struct {
	T               float64    `json:"t"`
	OK              bool       `json:"ok"`
	VaKVA           float64    `json:"va_kva"`
	Watts           float64    `json:"watts"`
	Var             float64    `json:"var"`
	Volt            float64    `json:"volt"`
	Lps             []LpSample `json:"lps"`
	BurstWindowOpen bool       `json:"burst_window_open"`
	BlindWhy        string     `json:"blind_why"`
	Alarm           bool       `json:"alarm"`
	TempC           *float64   `json:"temp_c"`
	SourceOhm       float64    `json:"source_ohm"`
	// PvKVA is the solar production evcc measures. Solar lets the house net below zero,
	// but never further than the panels deliver, so a car overclaiming cannot invent headroom.
	PvKVA float64 `json:"pv_kva"`
}

// Volts is the line voltage, nominal when implausible
func (sm *Sample) Volts() float64 {
	return UsableVolts(sm.Volt)
}

// NetKVA is the apparent power signed like the real power: negative while exporting
func (sm *Sample) NetKVA() float64 {
	if sm.Watts < 0 {
		return -sm.VaKVA
	}
	return sm.VaKVA
}

// LpCommand is one loadpoint's command
type LpCommand struct {
	Key    string `json:"key"`
	Act    Act    `json:"act"`
	Amps   int    `json:"amps"`
	Reason string `json:"reason"`
	Repeat bool   `json:"repeat"`
}

// Command is what to do this step
type Command struct {
	Act    Act         `json:"act"`
	Amps   int         `json:"amps"`
	Reason string      `json:"reason"`
	Lps    []LpCommand `json:"lps"`
}

// LpTelemetry is one loadpoint's telemetry
type LpTelemetry struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Priority     int     `json:"priority"`
	Connected    bool    `json:"connected"`
	Wants        bool    `json:"wants"`
	SetpointA    int     `json:"setpointA"`
	AllocatedA   int     `json:"allocatedA"`
	MinA         float64 `json:"minA"`
	MaxA         float64 `json:"maxA"`
	Paused       bool    `json:"paused"`
	Ops          int     `json:"ops"`
	Supercharge  bool    `json:"supercharge"`
	WaitS        float64 `json:"waitS"`
	Commandable  bool    `json:"commandable"`
	Direct       bool    `json:"direct"`
	MeasuredA    float64 `json:"measuredA"`
	MeasuredRawA float64 `json:"measuredRawA"`
	MeasuredSrc  string  `json:"measuredSrc"`
	Clamped      bool    `json:"clamped"`
}

// Telemetry is the controller's published state
type Telemetry struct {
	Phase              Phase            `json:"phase"`
	VaKVA              float64          `json:"vaKva"`
	HouseKVA           float64          `json:"houseKva"`
	BudgetA            float64          `json:"budgetA"`
	TrimA              float64          `json:"trimA"`
	TiS                float64          `json:"tiS"`
	Closeness          float64          `json:"closeness"`
	PeakClosenessSeen  float64          `json:"peakCloseness"`
	PeakClosenessPhase string           `json:"peakClosenessPhase"`
	PeakClosenessKVA   float64          `json:"peakClosenessKva"`
	PeakClosenessBlind bool             `json:"peakClosenessBlind"`
	SetpointA          int              `json:"setpointA"`
	Bursts             int              `json:"bursts"`
	BurstCmds          int              `json:"burstCmds"`
	ContactorOps       int              `json:"contactorOps"`
	Writes             int              `json:"writes"`
	AvgKVA             float64          `json:"avgKva"`
	KVAh               float64          `json:"kvah"`
	TempC              float64          `json:"tempC"`
	TempBlock          bool             `json:"tempBlock"`
	EntryOffsetA       float64          `json:"entryOffsetA"`
	EntryOffsetHeld    bool             `json:"entryOffsetHeld"`
	SourceOhm          float64          `json:"sourceOhm"`
	VoltsAtGoal        float64          `json:"voltsAtGoal"`
	ExitCloseness      float64          `json:"exitCloseness"`
	ExitLeadS          float64          `json:"exitLeadS"`
	Blind              int              `json:"blind"`
	MeterDisagreements int              `json:"meterDisagreements"`
	CarClaimKVA        float64          `json:"carClaimKva"`
	CarCreditKVA       float64          `json:"carCreditKva"`
	StoodOff           map[string]int64 `json:"-"`
	Note               string           `json:"note"`
	Lps                []LpTelemetry    `json:"lps"`
}

func newTelemetry(phase Phase) Telemetry {
	return Telemetry{Phase: phase, StoodOff: map[string]int64{}}
}

// BurstRecord is one line per burst
type BurstRecord struct {
	StartedAt      float64 `json:"startedAt"`
	TargetKVA      float64 `json:"targetKva"`
	TargetA        int     `json:"targetA"`
	LastedS        float64 `json:"lastedS"`
	WindowS        float64 `json:"windowS"`
	PeakVaKVA      float64 `json:"peakKva"`
	PeakCloseness  float64 `json:"peakCloseness"`
	Exit           string  `json:"exit"`
	UsedContactor  bool    `json:"usedContactor"`
	Loadpoint      string  `json:"loadpoint"`
	WallS          float64 `json:"wallS"`
	SettledVaKVA   float64 `json:"settledKva"`
	SettledVolts   float64 `json:"settledVolts"`
	BaseVaKVA      float64 `json:"baseKva"`
	BaseVolts      float64 `json:"baseVolts"`
	PeakVolts      float64 `json:"peakVolts"`
	EntryOffsetA   float64 `json:"entryOffsetA"`
	Cmds           int     `json:"cmds"`
	At             string  `json:"at,omitempty"`
	LoadpointTitle string  `json:"loadpointTitle,omitempty"`
}

type lpState struct {
	setpoint     int
	paused       bool
	lastCmdT     float64
	lastStopT    float64
	stopBeganT   float64
	floorSince   *float64
	pendingSince *float64
	pendingDir   int
	ops          int
	owesWrite    bool
	unanswered   int
}

func newLpState() *lpState {
	return &lpState{paused: true, lastCmdT: never, lastStopT: never, stopBeganT: never}
}

func (st *lpState) believedRunning() bool {
	return !st.paused && st.setpoint > 0
}

func (st *lpState) stoppedByUs() bool {
	return st.paused && st.lastStopT > never
}

type split struct {
	house   float64
	share   map[string]float64
	raw     map[string]float64
	clamped map[string]bool
}

// Controller is the control law state machine
type Controller struct {
	curve Curve
	s     *Settings

	Phase   Phase
	Tel     Telemetry
	Records []BurstRecord

	// HouseBaselineFrac is the share of the measured idle house held back from the cars
	HouseBaselineFrac float64

	lp    map[string]*lpState
	order []string

	baseT0         float64
	burstT0        float64
	burstWindow    float64
	burstPeakVA    float64
	burstPeakClose float64
	settledVA      float64
	settledVolts   float64
	houseLo        float64
	houseHi        float64
	houseAtEntry   float64
	tooHot         bool
	offsetApplied  float64
	burstOffsetA   float64
	burstTargetA   int
	burstKeys      []string
	baseVA         float64
	baseVolts      float64
	burstPeakVolts float64
	inBurst        bool
	burstCmds      int
	blind          int
	ti             float64
	baseTi         float64
	lastT          *float64
	trimA          float64
	trimT          float64
	kvah           float64
	elapsedS       float64
	splitT         *float64
	splitVal       *split
	disagreements  int
	idleHouseKVA   float64
	idleHouseT     float64
	noUptakeSince  map[string]float64
	standOffUntil  map[string]float64
	standOffS      map[string]float64
	lastAlloc      map[string]int
	blindHold      string
	prevDraw       map[string]float64
	moved          map[string]float64
	raiseRows      [][2]float64
	reduceRows     [][2]float64
	cadenceSrc     [2]string
	cadenceParsed  bool
	waits          map[string]float64
	warmed         bool
}

// NewController creates a controller
func NewController(curve Curve, s *Settings) *Controller {
	return &Controller{
		curve:             curve,
		s:                 s,
		Phase:             PhaseIdle,
		Tel:               newTelemetry(PhaseIdle),
		HouseBaselineFrac: houseBaselineFrac,
		lp:                map[string]*lpState{},
		houseLo:           inf,
		trimT:             never,
		idleHouseT:        never,
		noUptakeSince:     map[string]float64{},
		standOffUntil:     map[string]float64{},
		standOffS:         map[string]float64{},
		lastAlloc:         map[string]int{},
		prevDraw:          map[string]float64{},
		moved:             map[string]float64{},
		waits:             map[string]float64{},
	}
}

// SetCurve replaces the meter curve
func (c *Controller) SetCurve(curve Curve) {
	c.curve = curve
}

func (c *Controller) state(key string) *lpState {
	st, ok := c.lp[key]
	if !ok {
		st = newLpState()
		c.lp[key] = st
		c.order = append(c.order, key)
	}
	return st
}

// HouseFloor returns the floor now and the last measured idle house, both kVA
func (c *Controller) HouseFloor() (float64, float64) {
	learned := c.idleHouseKVA * c.HouseBaselineFrac
	return max(houseFloorKVA, learned, 0.0), c.idleHouseKVA
}

// SetpointOf returns the commanded current, 0 when paused
func (c *Controller) SetpointOf(key string) int {
	st, ok := c.lp[key]
	if !ok || st.paused {
		return 0
	}
	return st.setpoint
}

// Paused reports whether the controller holds this loadpoint stopped
func (c *Controller) Paused(key string) bool {
	st, ok := c.lp[key]
	return !ok || st.paused
}

// StoodOff returns the seconds of stand-down left for this loadpoint
func (c *Controller) StoodOff(key string, now *float64) float64 {
	t := c.lastT
	if now != nil {
		t = now
	}
	if t == nil {
		return 0
	}
	until, ok := c.standOffUntil[key]
	if !ok {
		until = never
	}
	return math.Max(until-*t, 0.0)
}

func (c *Controller) stoodOffAt(key string, t float64) float64 {
	return c.StoodOff(key, &t)
}

func takingUp(lp *LpSample) bool {
	return !lp.Estimated && lp.MeasuredA > 0.5
}

func (c *Controller) watchUptake(sm *Sample) {
	rivals := map[string]bool{}
	for _, lp := range sm.Lps {
		if lp.Connected && lp.Wants && c.stoodOffAt(lp.Key, sm.T) == 0 {
			rivals[lp.Key] = true
		}
	}
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		key := lp.Key
		if takingUp(lp) {
			delete(c.noUptakeSince, key)
			delete(c.standOffUntil, key)
			delete(c.standOffS, key)
			continue
		}
		st, ok := c.lp[key]
		offered := ok && !st.paused && st.setpoint >= c.floorA(lp)
		reserved := !c.reachable(lp) && c.lastAlloc[key] > 0
		if !(lp.Connected && lp.Wants && (offered || reserved)) {
			delete(c.noUptakeSince, key)
			continue
		}
		since, ok := c.noUptakeSince[key]
		if !ok {
			since = sm.T
			c.noUptakeSince[key] = since
		}
		if sm.T-since < NoUptakeS {
			continue
		}
		others := false
		for k := range rivals {
			if k != key {
				others = true
			}
		}
		if !others {
			continue
		}
		wait := math.Min(math.Max(c.standOffS[key]*2.0, standOffMinS), standOffMaxS)
		c.standOffS[key] = wait
		c.standOffUntil[key] = sm.T + wait
		delete(c.noUptakeSince, key)
		c.Tel.StoodOff[key] = int64(math.Round(sm.T + wait))
	}
}

func (c *Controller) floorA(lp *LpSample) int {
	if lp.MinA != 0 {
		return int(lp.MinA)
	}
	return c.s.AmpMin
}

func (c *Controller) ceilingA(lp *LpSample) int {
	if lp.MaxA != 0 {
		return int(lp.MaxA)
	}
	return c.s.AmpMax
}

func (c *Controller) draw(lp *LpSample) float64 {
	if lp.Estimated {
		return 0.0
	}
	if !lp.Stale {
		return math.Max(lp.MeasuredA, 0.0)
	}
	return math.Max(math.Min(lp.MeasuredA, float64(c.SetpointOf(lp.Key))), 0.0)
}

func (c *Controller) floorKVA(sm *Sample) float64 {
	fresh := sm.T-c.idleHouseT <= baselineTTLS
	learned := 0.0
	if fresh {
		learned = c.idleHouseKVA * c.HouseBaselineFrac
	}
	return max(houseFloorKVA, learned, 0.0)
}

func (c *Controller) computeSplit(sm *Sample) *split {
	v := sm.Volts() / 1000.0
	raw := map[string]float64{}
	share := map[string]float64{}
	var keys []string
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if _, ok := raw[lp.Key]; !ok {
			keys = append(keys, lp.Key)
		}
		raw[lp.Key] = c.draw(lp) * v
		share[lp.Key] = raw[lp.Key]
	}
	clamped := map[string]bool{}

	sum := func(m map[string]float64) float64 {
		var s float64
		for _, k := range keys {
			s += m[k]
		}
		return s
	}
	total := sum(share)
	avail := math.Max(sm.NetKVA()+sm.PvKVA-c.floorKVA(sm), 0.0)
	if total > avail+1e-9 {
		scale := 0.0
		if total > 0 {
			scale = avail / total
		}
		for _, k := range keys {
			if share[k] > 0.05 {
				clamped[k] = true
			}
			share[k] *= scale
		}
		total = sum(share)
	}
	return &split{house: math.Max(sm.NetKVA()-total, -sm.PvKVA), share: share, raw: raw, clamped: clamped}
}

func (c *Controller) splitNow(sm *Sample) *split {
	if c.splitT != nil && *c.splitT == sm.T && c.splitVal != nil {
		return c.splitVal
	}
	return c.computeSplit(sm)
}

func (c *Controller) resplit(sm *Sample) {
	moved := map[string]float64{}
	prev := map[string]float64{}
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		d := c.draw(lp)
		p, ok := c.prevDraw[lp.Key]
		if !ok {
			p = d
		}
		moved[lp.Key] = math.Abs(d - p)
	}
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		prev[lp.Key] = c.draw(lp)
	}
	c.moved = moved
	c.prevDraw = prev

	var measured float64
	anyRunning, anySetpoint := false, false
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		measured += math.Max(lp.MeasuredA, 0.0)
		anyRunning = anyRunning || lp.Running
		anySetpoint = anySetpoint || c.SetpointOf(lp.Key) != 0
	}
	if measured <= 0.25 && !anyRunning && !anySetpoint && !c.inMotion(sm) {
		c.idleHouseKVA = math.Max(sm.NetKVA()+sm.PvKVA, 0.0)
		c.idleHouseT = sm.T
	}

	t := sm.T
	c.splitT = &t
	c.splitVal = c.computeSplit(sm)
	sp := c.splitVal
	var claim, credit float64
	for i := range sm.Lps {
		k := sm.Lps[i].Key
		claim += sp.raw[k]
		credit += sp.share[k]
	}
	c.Tel.CarClaimKVA = round(claim, 3)
	c.Tel.CarCreditKVA = round(credit, 3)
	if len(sp.clamped) > 0 {
		c.disagreements++
	}
	c.Tel.MeterDisagreements = c.disagreements
}

func (c *Controller) houseKVA(sm *Sample) float64 {
	return c.splitNow(sm).house
}

func (c *Controller) voltsAt(goalKVA float64, sm *Sample) float64 {
	r := math.Max(math.Min(sm.SourceOhm, maxSourceOhm), 0.0)
	if r <= 0.0 || goalKVA <= 0.0 {
		return sm.Volts()
	}
	vOC := sm.Volts() + r*(sm.VaKVA*1000.0/sm.Volts())
	disc := vOC*vOC - 4.0*r*goalKVA*1000.0
	if disc <= 0.0 {
		return vOC / 2.0
	}
	return (vOC + math.Sqrt(disc)) / 2.0
}

func (c *Controller) carsA(sm *Sample, commanded bool) float64 {
	var total float64
	if !commanded {
		for i := range sm.Lps {
			total += c.draw(&sm.Lps[i])
		}
		return total
	}
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		sp := float64(c.SetpointOf(lp.Key))
		if c.state(lp.Key).paused {
			sp = 0.0
		}
		total += math.Max(c.draw(lp), sp)
	}
	roomA := math.Max(sm.NetKVA()+sm.PvKVA-c.floorKVA(sm), 0.0) * 1000.0 / sm.Volts()
	return math.Min(total, roomA)
}

func (c *Controller) headroomA(goalKVA float64, sm *Sample, commanded bool) float64 {
	carsA := c.carsA(sm, commanded)
	vGoal := c.voltsAt(goalKVA, sm)
	q, pMeter := sm.Var, sm.Watts
	if q == 0 || pMeter == 0 {
		var house float64
		if commanded {
			house = math.Max(sm.NetKVA()-carsA*sm.Volts()/1000.0, -sm.PvKVA)
		} else {
			house = c.houseKVA(sm)
		}
		return (goalKVA - house) * 1000.0 / vGoal
	}
	goalW := goalKVA * 1000.0
	if math.Abs(q) >= goalW {
		return 0.0
	}
	pGoal := math.Sqrt(goalW*goalW - q*q)
	pCars := carsA * sm.Volts()
	return (pGoal - math.Max(pMeter-pCars, -sm.PvKVA*1000.0)) / vGoal
}

func (c *Controller) budgetA(goalKVA float64, sm *Sample, clamp bool, extraA float64, commanded bool) float64 {
	base := c.headroomA(goalKVA, sm, commanded)
	want := math.Max(base+c.appliedTrimA(goalKVA)+extraA, 0.0)
	if clamp {
		return c.meterClamp(want, goalKVA, sm, extraA)
	}
	return want
}

func (c *Controller) offsetAllowed(extraA float64) float64 {
	if extraA <= 0 {
		c.offsetApplied = extraA
		c.Tel.EntryOffsetA = round(extraA, 2)
		c.Tel.EntryOffsetHeld = false
		return extraA
	}
	if c.tooHot {
		c.offsetApplied = 0.0
		c.Tel.EntryOffsetA = 0.0
		c.Tel.EntryOffsetHeld = true
		return 0.0
	}
	c.offsetApplied = extraA
	c.Tel.EntryOffsetA = round(extraA, 2)
	c.Tel.EntryOffsetHeld = false
	return extraA
}

func (c *Controller) anchorA(lp *LpSample, sm *Sample) float64 {
	st := c.state(lp.Key)
	if st.paused {
		return c.creditedA(lp, sm)
	}
	if st.lastCmdT > never/2 {
		return float64(c.SetpointOf(lp.Key))
	}
	return c.creditedA(lp, sm)
}

func (c *Controller) creditedA(lp *LpSample, sm *Sample) float64 {
	share := c.splitNow(sm).share
	return share[lp.Key] * 1000.0 / sm.Volts()
}

func (c *Controller) meterClamp(wantA, goalKVA float64, sm *Sample, extraA float64) float64 {
	if !sm.OK {
		return wantA
	}
	var nowA float64
	for i := range sm.Lps {
		nowA += c.anchorA(&sm.Lps[i], sm)
	}
	roomA := goalKVA*1000.0/c.voltsAt(goalKVA, sm) - sm.NetKVA()*1000.0/sm.Volts()
	if roomA > 0 {
		roomA += math.Max(extraA, 0.0)
	}
	lo, hi := nowA, nowA+roomA
	if hi < lo {
		lo, hi = hi, lo
	}
	return math.Max(math.Min(math.Max(wantA, lo), hi), 0.0)
}

func (c *Controller) appliedTrimA(goalKVA float64) float64 {
	s := c.s
	if s.TrimMaxA <= 0 {
		return 0.0
	}
	if c.Phase == PhaseBurst {
		return 0.0
	}
	if goalKVA > s.BaseKVA(c.curve)+1e-9 {
		return 0.0
	}
	return math.Min(math.Max(c.trimA, -s.TrimMaxA), s.TrimMaxA)
}

func (c *Controller) trimStepA() float64 {
	return math.Max(c.s.TrimMaxA/8.0, 0.05)
}

func (c *Controller) inMotion(sm *Sample) bool {
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if c.moved[lp.Key] > motionA {
			return true
		}
		if sm.T-c.state(lp.Key).lastCmdT < lp.LatencyUpS+motionSettleS {
			return true
		}
	}
	return false
}

func (c *Controller) updateTrim(sm *Sample, goalKVA float64) {
	s := c.s
	if s.TrimMaxA <= 0 {
		c.trimA = 0.0
		return
	}
	anySc := false
	for i := range sm.Lps {
		if s.MaySupercharge(sm.Lps[i].Key) {
			anySc = true
		}
	}
	if anySc && sm.BurstWindowOpen {
		c.trimA = 0.0
		return
	}
	if c.Phase == PhaseBurst {
		return
	}
	if !sm.OK {
		return
	}

	step := c.trimStepA()

	if c.inMotion(sm) {
		return
	}
	if sm.T-c.trimT < motionSettleS+CmdLatencyS {
		return
	}

	var live []*LpSample
	for i := range sm.Lps {
		if !c.state(sm.Lps[i].Key).paused {
			live = append(live, &sm.Lps[i])
		}
	}
	if len(live) == 0 {
		return
	}

	if c.ti > 0.0 {
		c.trimA = math.Max(c.trimA-step, -s.TrimMaxA)
		c.trimT = sm.T
		return
	}

	if sm.T-c.baseT0 < s.ResetS {
		return
	}

	residualA := (goalKVA - sm.VaKVA) * 1000.0 / sm.Volts()
	if residualA <= 0.0 {
		return
	}
	if (c.curve.ThresholdKVA()-sm.VaKVA)*1000.0/sm.Volts() < 1.0 {
		return
	}

	allAtCeiling := true
	for _, lp := range live {
		if c.SetpointOf(lp.Key) < c.ceilingA(lp) {
			allAtCeiling = false
		}
	}
	if allAtCeiling {
		return
	}

	for _, lp := range live {
		if !lp.Stale && c.SetpointOf(lp.Key) > 0 && c.draw(lp) < 0.5*float64(c.SetpointOf(lp.Key)) {
			return
		}
	}

	for _, lp := range live {
		if c.moved[lp.Key] > 0.5 {
			return
		}
	}
	c.trimA = math.Min(c.trimA+step, s.TrimMaxA)
	c.trimT = sm.T
}

func (c *Controller) allocLps(sm *Sample, caps map[string]float64) []AllocLp {
	out := make([]AllocLp, 0, len(sm.Lps))
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		st := c.state(lp.Key)
		minA, maxA := lp.MinA, lp.MaxA
		if minA == 0 {
			minA = float64(c.s.AmpMin)
		}
		if maxA == 0 {
			maxA = float64(c.s.AmpMax)
		}
		a := AllocLp{
			Key:      lp.Key,
			Priority: lp.Priority,
			MinA:     minA,
			MaxA:     maxA,
			Wants:    lp.Connected && lp.Wants && c.stoodOffAt(lp.Key, sm.T) == 0,
			Running:  !st.paused,
		}
		if caps != nil {
			if v, ok := caps[lp.Key]; ok {
				vv := v
				a.CapA = &vv
			}
		}
		out = append(out, a)
	}
	return out
}

func (c *Controller) allocate(goalKVA float64, sm *Sample, eligible []string, clamp, commanded bool) map[string]int {
	if !sm.OK {
		res := map[string]int{}
		for i := range sm.Lps {
			lp := &sm.Lps[i]
			if lp.Connected && lp.Wants {
				res[lp.Key] = c.floorA(lp)
			} else {
				res[lp.Key] = 0
			}
		}
		return res
	}
	atBase := c.allocLps(sm, nil)
	baseBudget := c.budgetA(c.s.BaseKVA(c.curve), sm, clamp && eligible == nil, 0.0, commanded)
	base := Allocate(baseBudget, atBase)
	held := c.heldByRogues(sm, base)
	heldSum := held.Sum()
	if len(held.Keys) > 0 {
		caps := map[string]float64{}
		for _, k := range held.Keys {
			caps[k] = base.Get(k)
		}
		atBase = c.allocLps(sm, caps)
		base = Allocate(baseBudget-heldSum, atBase)
	}
	if eligible == nil {
		return ToAmps(base, atBase)
	}

	caps := map[string]float64{}
	for i := range sm.Lps {
		k := sm.Lps[i].Key
		if !slices.Contains(eligible, k) {
			caps[k] = base.Get(k)
		}
	}
	lps := c.allocLps(sm, caps)
	extra := 0.0
	if goalKVA > c.s.BaseKVA(c.curve)+1e-9 {
		for i := range sm.Lps {
			if slices.Contains(eligible, sm.Lps[i].Key) {
				extra += sm.Lps[i].EntryOffsetA
			}
		}
	}
	extra = c.offsetAllowed(extra)
	return ToAmps(Allocate(c.budgetA(goalKVA, sm, clamp, extra, commanded)-heldSum, lps), lps)
}

type emitOpts struct {
	force, urgent bool
	dwellOK       bool
}

func (c *Controller) emitLp(lp *LpSample, want int, sm *Sample, o emitOpts) LpCommand {
	s := c.s
	st := c.state(lp.Key)
	if lp.MeasuredA > drawingA {
		st.unanswered = 0
	}
	floor := c.floorA(lp)
	ceiling := c.ceilingA(lp)

	if want <= 0 {
		rogue := c.rogue(lp)
		if st.paused && !rogue {
			st.floorSince = nil
			return LpCommand{Key: lp.Key, Act: ActNone}
		}
		if rogue || o.urgent || !o.dwellOK {
			return c.stopLp(lp, sm, "no budget")
		}
		if st.floorSince == nil {
			t := sm.T
			st.floorSince = &t
		}
		if sm.T-*st.floorSince >= s.FloorDwellS {
			return c.stopLp(lp, sm, "below its minimum")
		}
		return c.setLp(lp, floor, sm, "holding at its minimum", false)
	}

	st.floorSince = nil
	target := int(math.Min(math.Max(float64(want), float64(floor)), float64(ceiling)))

	if st.paused {
		if sm.T-st.lastStopT < s.RestartDwellS && !o.force {
			return LpCommand{Key: lp.Key, Act: ActNone, Reason: "waiting to resume"}
		}
		if float64(target) < float64(floor)+restartHysteresisA && !o.force {
			return LpCommand{Key: lp.Key, Act: ActNone, Reason: "not enough to resume"}
		}
		return c.setLp(lp, target, sm, "resuming", false)
	}

	switch c.deaf(lp, sm) {
	case "over":
		return c.setLp(lp, target, sm, "re-issuing a reduction", true)
	case "idle":
		if st.unanswered >= reissueMax {
			st.unanswered = 0
			st.paused = true
			st.setpoint = 0
			st.lastStopT = sm.T
			st.stopBeganT = sm.T
			return LpCommand{Key: lp.Key, Act: ActNone, Reason: "setpoint never taken up"}
		}
		st.unanswered++
		return c.setLp(lp, target, sm, "re-issuing the setpoint", true)
	}

	owed := st.owesWrite && c.reachable(lp)

	delta := target - st.setpoint
	if o.force || owed {
		return c.setLp(lp, target, sm, "setpoint", false)
	}

	if delta == 0 {
		st.pendingSince = nil
		st.pendingDir = 0
		return LpCommand{Key: lp.Key, Act: ActNone}
	}

	rows := c.raiseRows
	if delta < 0 {
		rows = c.reduceRows
	}
	firstStep := 0.0
	if len(rows) > 0 {
		firstStep = rows[0][0]
	}
	if math.Abs(float64(delta)) < firstStep {
		st.pendingSince = nil
		st.pendingDir = 0
		c.waits[lp.Key] = 0.0
		return LpCommand{Key: lp.Key, Act: ActNone}
	}
	direction := -1
	if delta > 0 {
		direction = 1
	}
	if st.pendingSince == nil || st.pendingDir != direction {
		t := sm.T
		st.pendingSince = &t
		st.pendingDir = direction
	}
	banked := sm.T - *st.pendingSince
	gap := math.Abs(float64(delta))
	if sm.OK && !lp.Stale {
		gap = math.Max(gap, math.Abs(float64(target)-lp.MeasuredA))
	}
	wait := CadenceWait(rows, gap) - banked
	if c.ti > 0.0 && delta < 0 {
		left := s.BaseTiAbortS - c.ti - motionSettleS
		wait = math.Min(wait, math.Max(left, 0.0))
	}
	c.waits[lp.Key] = math.Max(wait, 0.0)
	if wait <= 0.0 {
		return c.setLp(lp, target, sm, "setpoint", false)
	}
	return LpCommand{Key: lp.Key, Act: ActNone}
}

func (c *Controller) setLp(lp *LpSample, amps int, sm *Sample, reason string, repeat bool) LpCommand {
	st := c.state(lp.Key)
	if !c.reachable(lp) && c.draw(lp) <= 0.5 && !st.believedRunning() {
		st.owesWrite = true
		return LpCommand{Key: lp.Key, Act: ActNone, Reason: "not ready for a current command"}
	}
	if c.inBurst && (st.paused || amps != st.setpoint) {
		c.burstCmds++
	}
	c.Tel.Writes++
	st.owesWrite = false
	st.setpoint = amps
	st.lastCmdT = sm.T
	st.paused = false
	st.pendingSince = nil
	st.pendingDir = 0
	return LpCommand{Key: lp.Key, Act: ActSet, Amps: amps, Reason: reason, Repeat: repeat}
}

func (c *Controller) reachable(lp *LpSample) bool {
	return lp.Commandable || (c.state(lp.Key).stoppedByUs() && !lp.Finished)
}

func (c *Controller) deaf(lp *LpSample, sm *Sample) string {
	st := c.state(lp.Key)
	if !sm.OK || !st.believedRunning() {
		return ""
	}
	gap := sm.T - st.lastCmdT
	if lp.MeasuredA <= drawingA {
		if gap >= math.Max(reissueS, 2.0*lp.LatencyS) {
			return "idle"
		}
		return ""
	}
	excess := lp.MeasuredA - float64(st.setpoint)
	if excess < overdrawA {
		return ""
	}
	needed := lp.LatencyS + excess/math.Max(lp.RampAPerS, 0.1) + 2.0
	if gap >= needed {
		return "over"
	}
	return ""
}

func (c *Controller) rogue(lp *LpSample) bool {
	return c.state(lp.Key).paused && lp.MeasuredA > drawingA
}

func (c *Controller) heldByRogues(sm *Sample, alloc Alloc) Alloc {
	held := newAlloc()
	if !sm.OK {
		return held
	}
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		st := c.state(lp.Key)
		if !(st.stoppedByUs() && c.rogue(lp)) {
			continue
		}
		if lp.Fast && sm.T-st.stopBeganT < restopS {
			continue
		}
		excess := c.creditedA(lp, sm) - alloc.Get(lp.Key)
		if excess > EPS {
			held.Set(lp.Key, excess)
		}
	}
	return held
}

func heldFromInts(sm *Sample, c *Controller, alloc map[string]int) Alloc {
	a := newAlloc()
	for i := range sm.Lps {
		k := sm.Lps[i].Key
		if v, ok := alloc[k]; ok {
			a.Set(k, float64(v))
		}
	}
	return c.heldByRogues(sm, a)
}

func (c *Controller) stopLp(lp *LpSample, sm *Sample, reason string) LpCommand {
	st := c.state(lp.Key)
	if st.paused && !c.rogue(lp) {
		return LpCommand{Key: lp.Key, Act: ActNone}
	}
	if st.paused && sm.T-st.lastStopT < restopS {
		return LpCommand{Key: lp.Key, Act: ActNone}
	}
	first := !st.paused || st.lastStopT <= never
	if c.inBurst {
		c.burstCmds++
	}
	st.setpoint = 0
	st.paused = true
	st.lastCmdT = sm.T
	st.lastStopT = sm.T
	st.floorSince = nil
	if first {
		st.stopBeganT = sm.T
		st.ops++
		c.Tel.ContactorOps++
	}
	return LpCommand{Key: lp.Key, Act: ActStop, Amps: 0, Reason: reason}
}

func (c *Controller) dwellAffordable(sm *Sample) bool {
	if c.Phase == PhaseBurst || !sm.OK {
		return false
	}
	var sum float64
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if !c.state(lp.Key).paused {
			sum += math.Max(c.draw(lp)-float64(c.floorA(lp)), 0.0)
		}
	}
	held := sm.NetKVA() - sum*sm.Volts()/1000.0
	return held <= c.curve.ThresholdKVA()+floorSlackKVA
}

func (c *Controller) bundle(cmds []LpCommand, reason string) Command {
	act := ActNone
	var live []LpCommand
	for _, cmd := range cmds {
		if cmd.Act != ActNone {
			live = append(live, cmd)
		}
	}
	for _, cmd := range live {
		if cmd.Act == ActStop {
			act = ActStop
		}
	}
	if act == ActNone && len(live) > 0 {
		act = ActSet
	}
	total := 0
	for _, k := range c.order {
		if st := c.lp[k]; !st.paused {
			total += st.setpoint
		}
	}
	why := reason
	if why == "" {
		for _, cmd := range live {
			if cmd.Reason != "" {
				why = cmd.Reason
				break
			}
		}
	}
	return Command{Act: act, Amps: total, Reason: why, Lps: cmds}
}

func (c *Controller) respond(sm *Sample, cmds []LpCommand, alloc map[string]int, reason string) Command {
	if alloc != nil {
		c.lastAlloc = make(map[string]int, len(alloc))
		for k, v := range alloc {
			c.lastAlloc[k] = v
		}
	}
	c.telemetry(sm, alloc)
	return c.bundle(cmds, reason)
}

// Start begins a run window. keepCounter keeps the meter counter across a reconnect.
func (c *Controller) Start(now float64, keepCounter bool) {
	c.Phase = PhaseBase
	c.baseT0 = now
	c.blind = 0
	for _, st := range c.lp {
		st.setpoint = 0
		st.paused = true
		st.lastCmdT = never
		st.lastStopT = never
		st.stopBeganT = never
		st.floorSince = nil
		st.pendingSince = nil
		st.pendingDir = 0
		st.owesWrite = false
		st.unanswered = 0
		st.ops = 0
	}
	c.warmed = false
	if !keepCounter {
		c.ti = 0.0
	}
	c.trimA = 0.0
	c.trimT = never
	c.noUptakeSince = map[string]float64{}
	c.standOffUntil = map[string]float64{}
	c.standOffS = map[string]float64{}
	c.lastAlloc = map[string]int{}
	c.lastT = nil
	c.kvah = 0.0
	c.elapsedS = 0.0
	c.Tel = newTelemetry(PhaseBase)
}

// External aligns the controller's picture with what the loadpoint's own logic did to the
// charger outside the control law: a loadpoint that no longer wants to charge has already
// been switched off (so holding it at its floor would be a phantom current), and one whose
// own demand dropped below the setpoint is already limited to that demand.
func (c *Controller) External(key string, t float64, wants bool, capA int) {
	st, ok := c.lp[key]
	if !ok || st.paused {
		return
	}
	if !wants {
		st.setpoint = 0
		st.paused = true
		st.lastCmdT = t
		st.lastStopT = never
		st.stopBeganT = never
		st.floorSince = nil
		st.pendingSince = nil
		st.pendingDir = 0
		st.owesWrite = false
		st.unanswered = 0
		return
	}
	if capA > 0 && st.setpoint > capA {
		st.setpoint = capA
	}
}

// Stop ends the run window
func (c *Controller) Stop() {
	c.Phase = PhaseIdle
	c.Tel.Phase = PhaseIdle
}

func (c *Controller) mirrorMeter(sm *Sample) {
	dt := 0.0
	if c.lastT != nil {
		dt = math.Max(0.0, sm.T-*c.lastT)
	}
	t := sm.T
	c.lastT = &t

	if sm.OK && sm.VaKVA <= c.curve.ThresholdKVA() {
		c.ti = 0.0
		c.baseTi = 0.0
	} else {
		c.ti += dt
		if c.Phase != PhaseBurst {
			c.baseTi += dt
		}
	}

	if sm.OK {
		c.kvah += sm.VaKVA * dt / 3600.0
		c.elapsedS += dt
	}
}

func (c *Controller) telemetry(sm *Sample, alloc map[string]int) {
	c.Tel.TrimA = round(c.appliedTrimA(c.s.BaseKVA(c.curve)), 2)
	v := sm.Volts() / 1000.0
	sp := c.splitNow(sm)
	rows := make([]LpTelemetry, 0, len(sm.Lps))
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		st := c.state(lp.Key)
		name := lp.Name
		if name == "" {
			name = lp.Key
		}
		setpoint := st.setpoint
		if st.paused {
			setpoint = 0
		}
		measured := 0.0
		if v != 0 {
			measured = round(sp.share[lp.Key]/v, 1)
		}
		rows = append(rows, LpTelemetry{
			Key: lp.Key, Name: name, Priority: lp.Priority,
			Connected: lp.Connected, Wants: lp.Wants,
			SetpointA:    setpoint,
			MeasuredA:    measured,
			MeasuredRawA: round(math.Max(lp.MeasuredA, 0.0), 1),
			MeasuredSrc:  lp.Src,
			Clamped:      sp.clamped[lp.Key],
			AllocatedA:   alloc[lp.Key],
			MinA:         lp.MinA, MaxA: lp.MaxA, Paused: st.paused, Ops: st.ops,
			Supercharge: c.s.MaySupercharge(lp.Key),
			WaitS:       round(c.waits[lp.Key], 1),
			Commandable: c.reachable(lp), Direct: lp.Direct,
		})
	}
	c.Tel.Lps = rows
	total := 0
	for _, r := range rows {
		total += r.SetpointA
	}
	c.Tel.SetpointA = total
}

func (c *Controller) refreshCadence() {
	src := [2]string{c.s.RaiseTable, c.s.ReduceTable}
	if !c.cadenceParsed || src != c.cadenceSrc {
		c.cadenceParsed = true
		c.cadenceSrc = src
		c.raiseRows = ParseCadence(src[0])
		c.reduceRows = ParseCadence(src[1])
	}
}

// Step runs one control step
func (c *Controller) Step(sm *Sample) Command {
	s := c.s
	c.refreshCadence()
	c.mirrorMeter(sm)
	c.waits = map[string]float64{}
	if sm.OK {
		c.blind = 0
		c.blindHold = ""
	} else {
		c.blind++
	}
	c.Tel.Blind = c.blind
	if sm.OK {
		c.Tel.VaKVA = sm.VaKVA
		c.resplit(sm)
		c.watchUptake(sm)
		c.Tel.HouseKVA = round(c.houseKVA(sm), 3)
	}
	c.Tel.TiS = c.ti
	va := sm.VaKVA
	if !sm.OK {
		va = s.BurstKVA + s.BumpKVA
	}
	c.Tel.Closeness = c.curve.Closeness(va, c.ti)
	if c.Tel.Closeness > c.Tel.PeakClosenessSeen {
		c.Tel.PeakClosenessSeen = round(c.Tel.Closeness, 3)
		c.Tel.PeakClosenessPhase = string(c.Phase)
		if sm.OK {
			c.Tel.PeakClosenessKVA = round(sm.VaKVA, 3)
		} else {
			c.Tel.PeakClosenessKVA = 0
		}
		c.Tel.PeakClosenessBlind = !sm.OK
	}
	if c.elapsedS > 0 {
		c.Tel.AvgKVA = c.kvah * 3600.0 / c.elapsedS
	} else {
		c.Tel.AvgKVA = 0
	}
	c.Tel.KVAh = c.kvah

	switch c.Phase {
	case PhaseIdle:
		c.telemetry(sm, map[string]int{})
		return Command{Act: ActNone}
	case PhaseBase:
		return c.stepBase(sm)
	default:
		return c.stepBurst(sm)
	}
}

func (c *Controller) stepBase(sm *Sample) Command {
	cv, s := c.curve, c.s
	c.Tel.Phase = PhaseBase

	if !(sm.OK && sm.VaKVA <= cv.ThresholdKVA()) {
		c.baseT0 = sm.T
	}

	if !sm.OK {
		return c.blindBase(sm)
	}

	baseClose := cv.Closeness(sm.VaKVA, c.baseTi)
	if baseClose >= s.BaseStopCloseness || c.baseTi >= s.BaseTiStopS {
		c.Tel.Note = fmt.Sprintf("BASELINE OVERSHOOT %.2f (%.0f s banked in this phase), stopping every loadpoint", baseClose, c.baseTi)
		cmds := make([]LpCommand, 0, len(sm.Lps))
		for i := range sm.Lps {
			cmds = append(cmds, c.stopLp(&sm.Lps[i], sm, "baseline overshoot"))
		}
		return c.respond(sm, cmds, nil, "baseline overshoot")
	}
	if baseClose >= s.BaseAbortCloseness || c.baseTi >= s.BaseTiAbortS {
		c.Tel.Note = fmt.Sprintf("baseline overshoot %.2f (%.0f s banked in this phase), holding every loadpoint at its minimum", baseClose, c.baseTi)
		cmds := make([]LpCommand, 0, len(sm.Lps))
		for i := range sm.Lps {
			lp := &sm.Lps[i]
			switch {
			case c.rogue(lp):
				cmds = append(cmds, c.stopLp(lp, sm, "baseline overshoot"))
			case !c.state(lp.Key).paused && c.SetpointOf(lp.Key) > c.floorA(lp):
				cmds = append(cmds, c.emitLp(lp, c.floorA(lp), sm, emitOpts{force: true, dwellOK: false}))
			default:
				cmds = append(cmds, LpCommand{Key: lp.Key, Act: ActNone})
			}
		}
		return c.respond(sm, cmds, nil, "baseline overshoot")
	}

	goal := s.BaseKVA(cv)
	c.updateTrim(sm, goal)

	alloc := c.allocate(goal, sm, nil, true, false)
	warming := false
	if warmupToFloors && !c.warmed {
		if sm.OK && sm.T-c.baseT0 >= s.ResetS {
			c.warmed = true
		} else {
			floors := map[string]int{}
			for i := range sm.Lps {
				lp := &sm.Lps[i]
				if alloc[lp.Key] > 0 {
					floors[lp.Key] = c.floorA(lp)
				} else {
					floors[lp.Key] = 0
				}
			}
			alloc = floors
			warming = true
		}
	}
	heldA := heldFromInts(sm, c, alloc).Sum()
	c.Tel.BudgetA = round(math.Max(c.budgetA(goal, sm, true, 0, false)-heldA, 0.0), 1)
	c.Tel.SourceOhm = round(sm.SourceOhm, 3)
	c.Tel.VoltsAtGoal = round(c.voltsAt(s.BurstKVA, sm), 1)
	if warming {
		c.Tel.Note = "warming up, holding at the minimums"
	} else {
		c.Tel.Note = fmt.Sprintf("balancing %.2f kVA · %.0f A to share", sm.VaKVA, c.Tel.BudgetA)
	}

	window := cv.BurstWindow(s.BurstKVA, s.BumpKVA, s.MarginS)

	tooHot := c.hot(sm.TempC, s.MaxTempC)
	if sm.TempC != nil {
		c.Tel.TempC = *sm.TempC
	} else {
		c.Tel.TempC = 0
	}
	c.Tel.TempBlock = tooHot

	held := sm.T-c.baseT0 >= s.ResetS
	var eligible []string
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if s.MaySupercharge(lp.Key) && lp.Connected && lp.Wants && alloc[lp.Key] > 0 && !c.state(lp.Key).paused {
			eligible = append(eligible, lp.Key)
		}
	}
	if held && len(eligible) > 0 && sm.BurstWindowOpen && !sm.Alarm && !tooHot && window >= 5.0 {
		burst := c.allocate(s.BurstKVA, sm, eligible, true, false)
		for _, k := range eligible {
			if burst[k] > alloc[k] {
				return c.enterBurst(sm, burst, eligible, window)
			}
		}
	}

	dwellOK := c.dwellAffordable(sm)
	cmds := make([]LpCommand, 0, len(sm.Lps))
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		cmds = append(cmds, c.emitLp(lp, alloc[lp.Key], sm, emitOpts{dwellOK: dwellOK}))
	}

	if held && s.AnySupercharge() {
		switch {
		case !sm.BurstWindowOpen:
			c.Tel.Note = "baseline only, supercharging stood itself down after bursts died on entry"
		case tooHot:
			c.Tel.Note = fmt.Sprintf("baseline only, charger at %.0f °C, limit %.0f °C", *sm.TempC, s.MaxTempC)
		case window < 5.0:
			c.Tel.Note = "no safe burst window at these settings"
		case len(eligible) == 0:
			c.Tel.Note = "baseline only, nothing eligible to supercharge"
		}
	}
	return c.respond(sm, cmds, alloc, "")
}

func blindLabel(sm *Sample) string {
	if sm.BlindWhy == "wallbox" {
		return "charger current feed blind"
	}
	return "meter blind"
}

func nameOf(sm *Sample, key string) string {
	for i := range sm.Lps {
		if sm.Lps[i].Key == key {
			if sm.Lps[i].Name != "" {
				return sm.Lps[i].Name
			}
			return key
		}
	}
	return key
}

func (c *Controller) blindKeeper(sm *Sample) string {
	var eligible []*LpSample
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if lp.Connected && lp.Wants && (c.SetpointOf(lp.Key) >= c.floorA(lp) || lp.MeasuredA > drawingA || lp.Running) {
			eligible = append(eligible, lp)
		}
	}
	if c.blindHold != "" {
		for _, lp := range eligible {
			if lp.Key == c.blindHold {
				return c.blindHold
			}
		}
		c.blindHold = ""
		return ""
	}
	if len(eligible) == 0 {
		return ""
	}
	best := eligible[0]
	for _, lp := range eligible[1:] {
		if lp.Priority > best.Priority ||
			(lp.Priority == best.Priority && (lp.MeasuredA > best.MeasuredA ||
				(lp.MeasuredA == best.MeasuredA && lp.Key > best.Key))) {
			best = lp
		}
	}
	c.blindHold = best.Key
	return best.Key
}

func (c *Controller) blindBase(sm *Sample) Command {
	s := c.s
	why := blindLabel(sm)
	if s.BlindAction == "hold" && c.blind >= s.BlindHoldPolls {
		keep := c.blindKeeper(sm)
		if keep != "" {
			c.Tel.Note = fmt.Sprintf("%s, holding %s at %d A, everything else off", why, nameOf(sm, keep), s.BlindHoldA)
		} else {
			c.Tel.Note = why + ", nothing was charging, so nothing is held"
		}
		cmds := make([]LpCommand, 0, len(sm.Lps))
		for i := range sm.Lps {
			lp := &sm.Lps[i]
			want := max(min(s.BlindHoldA, c.ceilingA(lp)), c.floorA(lp))
			switch {
			case !(lp.Connected && lp.Wants) || lp.Key != keep:
				cmds = append(cmds, c.stopLp(lp, sm, why))
			case c.SetpointOf(lp.Key) != want:
				cmds = append(cmds, c.emitLp(lp, want, sm, emitOpts{force: true, dwellOK: false}))
			default:
				cmds = append(cmds, LpCommand{Key: lp.Key, Act: ActNone})
			}
		}
		return c.respond(sm, cmds, nil, why)
	}
	if c.blind >= s.BlindPolls {
		c.Tel.Note = fmt.Sprintf("%s for %d polls, stopping", why, c.blind)
		cmds := make([]LpCommand, 0, len(sm.Lps))
		for i := range sm.Lps {
			cmds = append(cmds, c.stopLp(&sm.Lps[i], sm, why))
		}
		return c.respond(sm, cmds, nil, why)
	}
	if c.blind >= s.BlindHoldPolls {
		c.Tel.Note = why + ", holding at the minimums"
		cmds := make([]LpCommand, 0, len(sm.Lps))
		for i := range sm.Lps {
			lp := &sm.Lps[i]
			if c.SetpointOf(lp.Key) > c.floorA(lp) {
				cmds = append(cmds, c.emitLp(lp, c.floorA(lp), sm, emitOpts{force: true, dwellOK: false}))
			} else {
				cmds = append(cmds, LpCommand{Key: lp.Key, Act: ActNone})
			}
		}
		return c.respond(sm, cmds, nil, why)
	}
	c.Tel.Note = why
	c.telemetry(sm, map[string]int{})
	return Command{Act: ActNone}
}

func (c *Controller) enterBurst(sm *Sample, alloc map[string]int, eligible []string, window float64) Command {
	c.Phase = PhaseBurst
	c.burstT0 = sm.T
	c.burstWindow = window
	c.burstKeys = slices.Clone(eligible)
	c.burstTargetA = 0
	for _, k := range eligible {
		c.burstTargetA += alloc[k]
	}
	c.burstPeakVA = sm.VaKVA
	c.burstPeakVolts = 0
	if sm.OK {
		c.burstPeakVolts = sm.Volts()
	}
	c.burstPeakClose = 0.0
	c.settledVA = 0.0
	c.settledVolts = 0.0
	c.houseLo = inf
	c.houseHi = 0.0
	c.houseAtEntry = c.houseKVA(sm)
	c.baseVA, c.baseVolts = 0, 0
	if sm.OK {
		c.baseVA = sm.VaKVA
		c.baseVolts = sm.Volts()
	}
	c.burstOffsetA = c.offsetApplied
	c.inBurst = true
	c.burstCmds = 0
	c.Tel.Bursts++
	c.Tel.Phase = PhaseBurst
	cmds := make([]LpCommand, 0, len(sm.Lps))
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if !c.state(lp.Key).paused {
			cmds = append(cmds, c.emitLp(lp, alloc[lp.Key], sm, emitOpts{force: true, dwellOK: true}))
		} else {
			cmds = append(cmds, LpCommand{Key: lp.Key, Act: ActNone})
		}
	}
	return c.respond(sm, cmds, alloc, "burst")
}

func (c *Controller) stepBurst(sm *Sample) Command {
	cv, s := c.curve, c.s

	va := sm.VaKVA
	if !sm.OK {
		va = s.BurstKVA + s.BumpKVA
	}
	elapsed := c.ti

	tdispNow := cv.Tdisp(va)
	tdispBumped := cv.Tdisp(va + s.BumpKVA)
	closeness := 0.0
	if !math.IsInf(tdispNow, 1) {
		closeness = elapsed / tdispNow
	}

	if va > c.burstPeakVA {
		c.burstPeakVA = va
		c.burstPeakVolts = 0
		if sm.OK {
			c.burstPeakVolts = sm.Volts()
		}
	}
	c.burstPeakClose = math.Max(c.burstPeakClose, closeness)
	c.noteSettled(sm)
	c.Tel.VaKVA = va

	window := math.Min(c.burstWindow, cv.BurstWindow(s.BurstKVA, s.BumpKVA, s.MarginS))
	c.burstWindow = window

	exitAt := math.Min(cv.PlannedCloseness(s.BurstKVA, s.BumpKVA, s.MarginS), s.MaxCloseness)
	c.Tel.ExitCloseness = exitAt

	base := c.allocate(s.BaseKVA(cv), sm, nil, false, true)
	tShed := c.shedS(sm, base)
	lead := 0.0
	if tdispNow < 1e8 {
		lead = tShed * s.ExitLeadFrac / tdispNow
	}
	c.Tel.ExitLeadS = tShed * s.ExitLeadFrac
	atDeadline := sm.OK && (closeness+lead) >= exitAt
	c.Tel.Note = fmt.Sprintf("burst %.0f s banked · %.0f%% of %.0f%% budget (+%.0f%% shed)", elapsed, closeness*100, exitAt*100, lead*100)

	if atDeadline {
		return c.exitBurst(sm, "deadline", elapsed, base, exitOpts{})
	}

	if !sm.OK {
		return c.exitBurst(sm, blindLabel(sm), elapsed, base, exitOpts{forceStop: c.blind >= s.BlindPolls, gentle: true})
	}

	if sm.T-c.burstT0 >= window+s.MarginS && elapsed <= 1.0 {
		return c.exitBurst(sm, "no draw", elapsed, base, exitOpts{gentle: true})
	}

	headroom := tdispNow - elapsed

	if sm.Alarm {
		return c.exitBurst(sm, "icp alarm", elapsed, base, exitOpts{headroom: headroom})
	}
	if closeness > s.MaxCloseness {
		return c.exitBurst(sm, "closeness", elapsed, base, exitOpts{headroom: headroom})
	}
	if elapsed >= tdispBumped-s.MarginS {
		return c.exitBurst(sm, "bump margin", elapsed, base, exitOpts{headroom: headroom})
	}

	alloc := c.allocate(s.BurstKVA, sm, c.burstKeys, true, false)
	cmds := make([]LpCommand, 0, len(sm.Lps))
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		cmds = append(cmds, c.emitLp(lp, alloc[lp.Key], sm, emitOpts{dwellOK: true}))
	}
	return c.respond(sm, cmds, alloc, "")
}

func (c *Controller) noteSettled(sm *Sample) {
	if !sm.OK || sm.T-c.burstT0 < entrySettleS {
		return
	}
	total := 0
	for _, k := range c.burstKeys {
		total += c.SetpointOf(k)
	}
	if total != c.burstTargetA {
		return
	}
	house := c.houseKVA(sm)
	c.houseLo = math.Min(c.houseLo, house)
	c.houseHi = math.Max(c.houseHi, house)
	c.settledVA = sm.VaKVA
	c.settledVolts = sm.Volts()
}

func (c *Controller) entryEvidence() (float64, float64) {
	lo := math.Min(c.houseLo, c.houseAtEntry)
	hi := math.Max(c.houseHi, c.houseAtEntry)
	if hi-lo > houseStableKV {
		return 0.0, 0.0
	}
	return c.settledVA, c.settledVolts
}

func (c *Controller) hot(tempC *float64, limit float64) bool {
	if tempC == nil {
		c.tooHot = false
		return false
	}
	var now bool
	if c.tooHot {
		now = *tempC >= limit-tempHystC
	} else {
		now = *tempC >= limit
	}
	c.tooHot = now
	return now
}

func (c *Controller) shedS(sm *Sample, base map[string]int) float64 {
	worst := 0.0
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		if c.state(lp.Key).paused {
			continue
		}
		drop := math.Max(0.0, c.draw(lp)-float64(base[lp.Key]))
		if drop <= 0 {
			continue
		}
		worst = math.Max(worst, lp.LatencyS+drop/math.Max(lp.RampAPerS, 0.1))
	}
	return worst
}

func (c *Controller) exitTarget(lp *LpSample, want int, base map[string]int, sm *Sample) int {
	if want <= 0 || !sm.OK {
		return want
	}
	fits := c.budgetA(c.curve.ThresholdKVA()-burstExitClearanceKVA, sm, false, 0.0, true)
	others := 0
	for i := range sm.Lps {
		k := sm.Lps[i].Key
		if a, ok := base[k]; ok && k != lp.Key && a > 0 {
			others += a
		}
	}
	allowed := int(math.Max(fits-float64(others), 0.0))
	if allowed >= want {
		return want
	}
	return max(allowed, want-burstExitMaxTrimA, c.floorA(lp))
}

type exitOpts struct {
	forceStop bool
	headroom  float64
	gentle    bool
}

func (c *Controller) exitBurst(sm *Sample, reason string, elapsed float64, base map[string]int, o exitOpts) Command {
	s := c.s
	c.Phase = PhaseBase
	c.baseT0 = sm.T
	c.baseTi = 0.0
	c.Tel.Phase = PhaseBase

	cmds := make([]LpCommand, 0, len(sm.Lps))
	usedContactor := false
	for i := range sm.Lps {
		lp := &sm.Lps[i]
		st := c.state(lp.Key)
		want := c.exitTarget(lp, base[lp.Key], base, sm)
		if st.paused {
			cmds = append(cmds, LpCommand{Key: lp.Key, Act: ActNone})
			continue
		}
		var cmd LpCommand
		switch {
		case o.forceStop:
			cmd = c.stopLp(lp, sm, reason)
		case o.gentle || reason == "deadline":
			cmd = c.emitLp(lp, want, sm, emitOpts{force: true, dwellOK: c.dwellAffordable(sm)})
		default:
			cmd = c.shedLp(lp, want, sm, o.headroom, reason)
		}
		usedContactor = usedContactor || cmd.Act == ActStop
		cmds = append(cmds, cmd)
	}

	settledVA, settledVolts := c.entryEvidence()
	c.Records = append(c.Records, BurstRecord{
		StartedAt:     c.burstT0,
		TargetKVA:     s.BurstKVA,
		TargetA:       c.burstTargetA,
		LastedS:       round(elapsed, 2),
		WindowS:       round(c.burstWindow, 2),
		WallS:         round(math.Max(sm.T-c.burstT0, 0.0), 2),
		PeakVaKVA:     round(c.burstPeakVA, 3),
		PeakCloseness: round(c.burstPeakClose, 3),
		Exit:          reason,
		UsedContactor: usedContactor,
		Loadpoint:     strings.Join(c.burstKeys, ","),
		SettledVaKVA:  round(settledVA, 3),
		SettledVolts:  round(settledVolts, 1),
		BaseVaKVA:     round(c.baseVA, 3),
		BaseVolts:     round(c.baseVolts, 1),
		PeakVolts:     round(c.burstPeakVolts, 1),
		EntryOffsetA:  round(c.burstOffsetA, 2),
		Cmds:          c.burstCmds,
	})
	c.inBurst = false
	c.Tel.BurstCmds = c.burstCmds
	c.Tel.Note = "burst ended: " + reason
	return c.respond(sm, cmds, base, reason)
}

func (c *Controller) shedLp(lp *LpSample, want int, sm *Sample, headroom float64, why string) LpCommand {
	st := c.state(lp.Key)
	if st.paused {
		return LpCommand{Key: lp.Key, Act: ActNone}
	}
	floor := c.floorA(lp)
	if want < floor {
		return c.stopLp(lp, sm, why+"; below its minimum")
	}
	tPilot := lp.LatencyS + math.Max(0.0, c.draw(lp)-float64(want))/math.Max(lp.RampAPerS, 0.1)
	if tPilot <= headroom-c.s.MarginS/2 {
		return c.emitLp(lp, want, sm, emitOpts{force: true, dwellOK: false})
	}
	return c.stopLp(lp, sm, why+"; pilot too slow")
}

// round mimics Python's round(x, n) closely enough for display values
func round(x float64, n int) float64 {
	p := math.Pow(10, float64(n))
	v := x * p
	r := math.RoundToEven(v)
	if math.Abs(v-math.Trunc(v)) != 0.5 {
		r = math.Round(v)
	}
	return r / p
}
