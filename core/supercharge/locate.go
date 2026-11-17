package supercharge

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
)

// Car detection: which car is on which charger.
//
// A dumb charger (evcc's vehicle-api charger) has no data and is driven through one car's own API.
// Plain AC chargers report that a car is connected, never which one. A car that can sit on either
// charger is placed by physical proof: the car reports the pilot current it sees, and only a car
// plugged into a smart charger follows that charger's commanded current. Without proof the car is
// assumed on its dumb charger: a car wrongly believed elsewhere would charge there uncontrolled.

const (
	locHistoryS   = 240.0 // commanded current history kept per loadpoint
	locFollowS    = 120.0 // a car's reading may lag the command it follows by this much
	locEchoS      = 120.0 // a reading equal to a current the dumb charger commanded the car is no evidence
	locTol        = 0.5   // A: pilots are whole amps
	locMisses     = 3     // consecutive readings that follow nothing revoke a proof
	locQuietS     = 60.0  // a probe is due when the smart charger's command has not moved this long
	locProbeS     = 60.0  // a probe holds its reduction this long
	locProbeStepA = 2.0
	locBackoffS   = 120.0
	locBackoffMax = 1800.0
	locNegatives  = 2    // failed probes after which a smart charger's session is left alone
	locReleaseS   = 30.0 // release and rest are sent again this long later: a command still in flight may land after the first
)

// LocLp is a loadpoint as car detection sees it, read every second from cached state
type LocLp struct {
	Name           string // config name
	Title          string
	Connected      bool
	Enabled        bool
	OfferedA       float64
	MinA           float64
	MaxA           float64
	Vehicle        string // name of the vehicle it holds, "" for none
	DefaultVehicle string // name of its default vehicle, "" for none
}

// LocCar is a car's last reading. Pilot is only read while a smart charger has a car.
type LocCar struct {
	Title    string
	StatusOK bool    // the status could be read
	Plugged  bool    // status B or C
	CanProve bool    // the car reports the pilot it sees
	PilotOK  bool    // Pilot was sampled on this step
	Pilot    float64 // A, the pilot the car sees
}

// LocAssign gives a loadpoint a vehicle ("" takes it away)
type LocAssign struct {
	Loadpoint string
	Vehicle   string
}

// CarLocation is what the UI shows about a car configured on a dumb charger
type CarLocation struct {
	Loadpoint      string  `json:"loadpoint"`      // the dumb charger's loadpoint
	LoadpointTitle string  `json:"loadpointTitle"` //
	Vehicle        string  `json:"vehicle"`        //
	VehicleTitle   string  `json:"vehicleTitle"`   //
	At             string  `json:"at"`             // loadpoint holding the car, "" for none
	AtTitle        string  `json:"atTitle"`        //
	Plugged        bool    `json:"plugged"`        //
	Proven         bool    `json:"proven"`         // proven on a smart charger
	Manual         bool    `json:"manual"`         // put elsewhere by the owner or the charger, respected
	CanProve       bool    `json:"canProve"`       //
	Probing        bool    `json:"probing"`        //
	ProbeOnTitle   string  `json:"probeOnTitle"`   // smart charger being probed
	Pilot          float64 `json:"pilot"`          // last pilot reading, 0 when none
	Evidence       string  `json:"evidence"`       // why it is where it is
	Error          string  `json:"error"`          // why the pair cannot work: missing, duplicate, homeDefault, default
	ErrorOn        string  `json:"errorOn"`        // the loadpoint the error names
	Probes         int     `json:"probes"`         //
	Revokes        int     `json:"revokes"`        //
}

// LocDecision is what the site applies after a step
type LocDecision struct {
	Assign   []LocAssign        // in order: releases first
	Release  map[string]float64 // vehicle -> current: on a smart charger, allow the car to charge and lift its own current setting to this
	Rest     map[string]float64 // vehicle -> its dumb charger's minimum: on leaving a smart charger, set the car's own current to the safe rest value
	Probes   map[string]float64 // loadpoint -> current cap while probing
	Cars     []CarLocation
	Excluded map[string]bool // vehicles that status guessing must never assign
}

type locPoint struct {
	t       float64
	offered float64 // 0 while disabled
}

type locTrack struct {
	provenOn  string
	placedOn  string  // loadpoint the locator itself gave the car to
	releaseAt float64 // when the release is sent again, 0 when done
	lifted    float64 // highest own current setting asked of the car since the proof
	rest      bool    // proof was lost: the rest current is due
	restAt    float64 // when the rest current is sent again, 0 when done
	evidence  string
	misses    int
	revokes   int

	lastPilot float64
	havePilot bool

	probeOn    string
	probeUntil float64
	nextProbe  float64
	probes     int
	failed     map[string]int  // per smart loadpoint session: probes the car did not follow
	overruled  map[string]bool // per smart loadpoint session: a manual pick the car did not follow
}

// Locator places cars configured on dumb chargers. It is pure: no I/O, the caller samples and applies.
type Locator struct {
	hist      map[string][]locPoint
	connected map[string]bool
	tracks    map[string]*locTrack // by vehicle
}

// NewLocator creates a locator
func NewLocator() *Locator {
	return &Locator{hist: map[string][]locPoint{}, connected: map[string]bool{}, tracks: map[string]*locTrack{}}
}

func near(a, b float64) bool { return math.Abs(a-b) <= locTol }

// commanded reports whether lp commanded about amps at some time in [from, to]
func (l *Locator) commanded(lp string, amps, from, to float64) bool {
	for _, p := range l.hist[lp] {
		if p.t >= from && p.t <= to && p.offered > 0 && near(p.offered, amps) {
			return true
		}
	}
	return false
}

// followed reports whether lp stepped its command from about a to about b, landing on b in [from, to].
// Zero is no evidence: a stopped car may read 0 wherever it is, as does a switched-off charger.
func (l *Locator) followed(lp string, a, b, from, to float64) bool {
	if math.Abs(a-b) < 1 || a <= 0 || b <= 0 {
		return false
	}
	h := l.hist[lp]
	for i, p := range h {
		if p.t < from || p.t > to || !near(p.offered, b) {
			continue
		}
		for j := i - 1; j >= 0 && h[j].t >= p.t-locFollowS; j-- {
			if h[j].offered > 0 && near(h[j].offered, a) {
				return true
			}
		}
	}
	return false
}

// lastStep returns when lp's command last changed
func (l *Locator) lastStep(lp string) float64 {
	h := l.hist[lp]
	for i := len(h) - 1; i > 0; i-- {
		if !near(h[i].offered, h[i-1].offered) {
			return h[i].t
		}
	}
	if len(h) > 0 {
		return h[0].t
	}
	return 0
}

// Step records the loadpoints and returns what to apply. pairs maps a dumb charger's loadpoint to its car.
func (l *Locator) Step(now float64, pairs map[string]string, lps []LocLp, cars map[string]LocCar) LocDecision {
	d := LocDecision{Release: map[string]float64{}, Rest: map[string]float64{}, Probes: map[string]float64{}, Excluded: map[string]bool{}}
	byName := map[string]LocLp{}
	for _, lp := range lps {
		byName[lp.Name] = lp
		offered := lp.OfferedA
		if !lp.Enabled || !lp.Connected {
			offered = 0
		}
		h := append(l.hist[lp.Name], locPoint{t: now, offered: offered})
		for len(h) > 0 && h[0].t < now-locHistoryS {
			h = h[1:]
		}
		l.hist[lp.Name] = h

		// a smart charger's new session starts its evidence afresh
		if lp.Connected != l.connected[lp.Name] {
			for _, t := range l.tracks {
				delete(t.failed, lp.Name)
				delete(t.overruled, lp.Name)
				if !lp.Connected && t.provenOn == lp.Name {
					t.provenOn, t.evidence, t.rest = "", lp.Title+" disconnected", true
				}
			}
			l.connected[lp.Name] = lp.Connected
		}
	}

	isDumb := func(lp string) bool {
		_, ok := pairs[lp]
		return ok
	}

	seen := map[string]bool{}
	for _, lpName := range slices.Sorted(maps.Keys(pairs)) {
		vName := pairs[lpName]
		home, ok := byName[lpName]
		car, okCar := cars[vName]
		loc := CarLocation{Loadpoint: lpName, LoadpointTitle: home.Title, Vehicle: vName, VehicleTitle: car.Title, CanProve: car.CanProve}
		if !ok || !okCar {
			loc.Error = "missing"
			d.Cars = append(d.Cars, loc)
			continue
		}
		if seen[vName] {
			loc.Error = "duplicate"
			d.Cars = append(d.Cars, loc)
			continue
		}
		seen[vName] = true
		if loc.Error, loc.ErrorOn = pairError(home, vName, lps); loc.Error != "" {
			d.Cars = append(d.Cars, loc)
			continue
		}
		d.Excluded[vName] = true

		t := l.tracks[vName]
		if t == nil {
			t = &locTrack{failed: map[string]int{}, overruled: map[string]bool{}}
			l.tracks[vName] = t
		}

		owner := ""
		for _, lp := range lps {
			if lp.Vehicle == vName {
				owner = lp.Name
				break
			}
		}

		if car.StatusOK && !car.Plugged && t.provenOn != "" {
			t.provenOn, t.evidence, t.rest = "", "unplugged", true
		}

		// put elsewhere by the owner or by that charger's own identification: respected while that
		// charger has a car and the evidence does not contradict it. A wrong pick would leave the car
		// on its dumb charger uncontrolled.
		manualOn := ""
		if owner != "" && owner != lpName && owner != t.placedOn && byName[owner].Connected && !t.overruled[owner] {
			manualOn = owner
		}

		// evidence from a new pilot reading
		if car.PilotOK && car.CanProve {
			r := car.Pilot
			changed := !t.havePilot || !near(r, t.lastPilot)
			if on := cmp.Or(t.provenOn, manualOn); on != "" {
				x := byName[on]
				// a proof is only checked while x offers current: leaving x means unplugging, which ends it.
				// A pick by hand is checked always: a car reading a pilot x is not offering, even 0 while
				// x is off, is not on x, and left there it would charge uncontrolled on its dumb charger.
				if x.Connected && r > 0 && (manualOn != "" || x.Enabled && x.OfferedA > 0) {
					if l.commanded(x.Name, r, now-locFollowS, now) {
						t.misses = 0
					} else if t.misses++; t.misses >= locMisses {
						t.evidence = fmt.Sprintf("does not follow %s (reads %.0f A)", x.Title, r)
						if t.provenOn != "" {
							t.provenOn = ""
						} else {
							t.overruled[on] = true
							manualOn = ""
						}
						t.misses = 0
						t.revokes++
					}
				}
			} else if changed && t.havePilot && car.Plugged {
				var hits []LocLp
				for _, x := range lps {
					if isDumb(x.Name) || !x.Connected {
						continue
					}
					// a reading the dumb charger itself commanded may be the car echoing its own setting
					if l.followed(x.Name, t.lastPilot, r, now-locFollowS, now) && !l.commanded(lpName, r, now-locEchoS, now) {
						hits = append(hits, x)
					}
				}
				if len(hits) == 1 {
					x := hits[0]
					t.provenOn, t.misses, t.releaseAt, t.rest, t.restAt = x.Name, 0, now+locReleaseS, false, 0
					t.evidence = fmt.Sprintf("pilot followed %s from %.0f to %.0f A", x.Title, t.lastPilot, r)
					t.lifted = 0
				}
			}
			// on a smart charger the car's own setting must never hold the pilot back (a supercharge
			// burst offers more than the line). Lifted to each new highest pilot rather than straight to
			// the maximum: some car apis refuse a setting above the pilot the car sees right now.
			if lift := min(r, home.MaxA); t.provenOn != "" && lift > t.lifted {
				t.lifted = lift
				d.Release[vName] = lift
			}
			t.lastPilot, t.havePilot = r, true
		}

		// where it should be
		want := owner
		switch {
		case t.provenOn != "":
			want = t.provenOn
			loc.Proven = true
		case manualOn != "":
			loc.Manual = true
			if t.evidence == "" || t.placedOn != "" {
				t.evidence = "chosen on " + byName[owner].Title
			}
		case car.StatusOK && car.Plugged:
			want = lpName
			if owner != lpName && (t.evidence == "" || t.placedOn != "") {
				t.evidence = "not proven elsewhere"
			}
		case car.StatusOK && !car.Plugged && owner != "" && owner == t.placedOn:
			want = ""
		}
		if want != owner {
			if owner != "" {
				d.Assign = append(d.Assign, LocAssign{Loadpoint: owner})
			}
			if want != "" {
				d.Assign = append(d.Assign, LocAssign{Loadpoint: want, Vehicle: vName})
			}
		}
		if want != "" && want != lpName && t.provenOn == want {
			t.placedOn = want
		} else if want == lpName || want == "" {
			t.placedOn = ""
		}

		if t.releaseAt > 0 && now >= t.releaseAt {
			if t.provenOn != "" && t.lifted > 0 {
				d.Release[vName] = t.lifted
			}
			t.releaseAt = 0
		}

		// left the smart charger: the car keeps its own current setting, which the release lifted to
		// the full current. Back to the safe rest value before it arrives at its dumb charger, where it
		// draws that setting until evcc takes it over. Sent again later only while still unplugged: once
		// the dumb charger has the car it sets the current itself.
		if t.rest {
			d.Rest[vName] = home.MinA
			t.rest, t.restAt = false, now+locReleaseS
		} else if t.restAt > 0 && now >= t.restAt {
			if car.StatusOK && !car.Plugged {
				d.Rest[vName] = home.MinA
			}
			t.restAt = 0
		}

		// probe: lower a smart charger briefly so a car on it shows itself. Only while the dumb
		// charger has the car connected: away from home (its geofence) nothing is ambiguous.
		if t.probeOn != "" && now >= t.probeUntil {
			if t.provenOn == "" {
				t.failed[t.probeOn]++
				back := math.Min(locBackoffS*math.Pow(2, float64(t.failed[t.probeOn]-1)), locBackoffMax)
				t.nextProbe = now + back
			}
			t.probeOn = ""
		}
		if t.probeOn == "" && t.provenOn == "" && !loc.Manual && car.CanProve && car.Plugged && home.Connected && now >= t.nextProbe {
			for _, x := range lps {
				if isDumb(x.Name) || !x.Connected || !x.Enabled || x.OfferedA <= 0 || t.failed[x.Name] >= locNegatives {
					continue
				}
				if x.OfferedA-locProbeStepA < x.MinA-1e-9 && x.OfferedA-x.MinA < 1 {
					continue // at its minimum: a probe would have to raise
				}
				if now-l.lastStep(x.Name) < locQuietS {
					continue // it moves anyway: watch, do not touch
				}
				t.probeOn, t.probeUntil = x.Name, now+locProbeS
				t.probes++
				break
			}
		}
		if t.probeOn != "" {
			x := byName[t.probeOn]
			if !x.Connected || t.provenOn != "" {
				t.probeOn = ""
			} else {
				d.Probes[x.Name] = math.Max(x.MinA, math.Floor(l.probeBase(x.Name, t.probeUntil-locProbeS)-locProbeStepA))
				loc.Probing, loc.ProbeOnTitle = true, x.Title
			}
		}

		loc.At = want
		loc.AtTitle = byName[want].Title
		loc.Plugged = car.Plugged
		loc.Pilot = t.lastPilot
		loc.Evidence = t.evidence
		loc.Probes, loc.Revokes = t.probes, t.revokes
		d.Cars = append(d.Cars, loc)
	}

	// forget cars no longer configured
	for v := range l.tracks {
		if !seen[v] {
			delete(l.tracks, v)
		}
	}
	return d
}

// probeBase is what lp was commanded when the probe started
func (l *Locator) probeBase(lp string, at float64) float64 {
	h := l.hist[lp]
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].t <= at {
			return h[i].offered
		}
	}
	if len(h) > 0 {
		return h[0].offered
	}
	return 0
}

// pairError explains why a car cannot be placed by detection, and on which loadpoint
func pairError(home LocLp, vName string, lps []LocLp) (string, string) {
	if home.DefaultVehicle != "" {
		return "homeDefault", home.Title
	}
	for _, lp := range lps {
		if lp.DefaultVehicle == vName {
			return "default", lp.Title
		}
	}
	return "", ""
}

// RestAmps is a car's own current setting while it is not proven on a smart charger: the fail-safe
// current (what any loadpoint gets while load management cannot answer), at least the minimum
func (m *Manager) RestAmps(minA float64) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return math.Max(m.cfg.FailsafeA, minA)
}

// DumbCars returns the configured dumb charger loadpoints and their cars
func (m *Manager) DumbCars() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.cfg.DumbCars)
}

// SetLocation takes car detection's probes and verdicts and publishes them when they changed
func (m *Manager) SetLocation(probes map[string]float64, cars []CarLocation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := !maps.Equal(probes, m.probes) || !slices.Equal(cars, m.cars)
	for lp, a := range probes {
		if _, ok := m.probes[lp]; !ok {
			m.log.DEBUG.Printf("car detection: probing %s at %.0f A", m.titleOf(lp), a)
		}
	}
	m.probes, m.cars = probes, cars
	if changed {
		m.publishLocked()
	}
}
