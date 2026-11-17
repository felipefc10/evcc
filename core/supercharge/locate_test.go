package supercharge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// locWorld is a scripted house: a smart charger "x" (Wallbox) and a dumb charger "l" for car "v"
type locWorld struct {
	t      *testing.T
	loc    *Locator
	now    float64
	xOn    bool    // a car is plugged into x
	xA     float64 // x's commanded current
	xMin   float64
	lA     float64 // what the dumb charger commands the car
	owner  map[string]string
	defVeh map[string]string
	plug   bool
	away   bool                              // the dumb charger's geofence reports no car
	pilot  func(now float64) (float64, bool) // what the car reports
	probes map[string]float64
	last   LocDecision
	assign []LocAssign
}

func newWorld(t *testing.T) *locWorld {
	return &locWorld{t: t, loc: NewLocator(), xOn: true, xA: 16, xMin: 6, owner: map[string]string{}, defVeh: map[string]string{}, plug: true}
}

func (w *locWorld) step() LocDecision {
	w.now++
	x := w.xA
	if p, ok := w.probes["x"]; ok {
		x = min(x, p) // the probe lowers what x offers
	}
	lps := []LocLp{
		{Name: "x", Title: "Wallbox", Connected: w.xOn, Enabled: w.xOn, OfferedA: x, MinA: w.xMin, MaxA: 32, Vehicle: w.owner["x"], DefaultVehicle: w.defVeh["x"]},
		{Name: "l", Title: "Dumb", Connected: w.owner["l"] == "v" && !w.away, Enabled: w.lA > 0, OfferedA: w.lA, MinA: 6, MaxA: 32, Vehicle: w.owner["l"], DefaultVehicle: w.defVeh["l"]},
	}
	car := LocCar{Title: "Tesla", StatusOK: true, Plugged: w.plug, CanProve: w.pilot != nil}
	if w.pilot != nil && int(w.now)%5 == 0 {
		car.Pilot, car.PilotOK = w.pilot(w.now)
	}
	d := w.loc.Step(w.now, map[string]string{"l": "v"}, lps, map[string]LocCar{"v": car})
	for _, a := range d.Assign {
		if a.Vehicle == "" {
			delete(w.owner, a.Loadpoint)
		} else {
			for k, v := range w.owner {
				require.NotEqual(w.t, a.Vehicle, v, "%s still holds %s while %s takes it", k, v, a.Loadpoint)
			}
			w.owner[a.Loadpoint] = a.Vehicle
		}
		w.assign = append(w.assign, a)
	}
	w.probes = d.Probes
	w.last = d
	return d
}

func (w *locWorld) run(secs int) {
	for range secs {
		w.step()
	}
}

func (w *locWorld) car() CarLocation {
	require.Len(w.t, w.last.Cars, 1)
	return w.last.Cars[0]
}

// followsX: the car is on x and reports x's pilot after 18 s, like the Tesla on the Wallbox
func (w *locWorld) followsX() func(float64) (float64, bool) {
	hist := map[int]float64{}
	return func(now float64) (float64, bool) {
		x := w.xA
		if p, ok := w.probes["x"]; ok {
			x = min(x, p)
		}
		hist[int(now)] = x
		if v, ok := hist[int(now)-18]; ok {
			return v, true
		}
		return x, true
	}
}

func TestLocateDumbCarStaysHome(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 32, true } // fixed dumb pilot
	for i := range 20 {
		w.xA = []float64{16, 14, 12, 16}[i%4]
		w.run(30)
	}
	c := w.car()
	assert.Equal(t, "l", c.At)
	assert.False(t, c.Proven)
	assert.Equal(t, "v", w.owner["l"])
}

func TestLocateFollowsSmartCharger(t *testing.T) {
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	assert.Equal(t, "l", w.car().At, "unproven: assumed on its dumb charger")
	w.xA = 14
	w.run(40)
	c := w.car()
	assert.True(t, c.Proven)
	assert.Equal(t, "x", c.At)
	assert.Equal(t, "v", w.owner["x"])
	assert.Empty(t, w.owner["l"])
	assert.Contains(t, c.Evidence, "16 to 14")
}

func TestLocateReleasesTwice(t *testing.T) {
	// once on proof, once more after a disable the dumb charger had in flight could have landed
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	w.xA = 14
	var at []float64
	for range 120 {
		if a, ok := w.step().Release["v"]; ok {
			at = append(at, w.now)
			assert.Equal(t, 14.0, a, "the pilot the car sees: some apis refuse more")
		}
	}
	require.Len(t, at, 2)
	assert.InDelta(t, locReleaseS, at[1]-at[0], 1)
}

func TestLocateLiftsWithThePilot(t *testing.T) {
	// a supercharge burst raises the Wallbox: the car's own setting follows each new highest pilot,
	// never down, never above the maximum
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	w.xA = 14
	w.run(120)
	require.True(t, w.car().Proven)
	var lifts []float64
	for _, a := range []float64{12, 20, 16, 40} {
		w.xA = a
		for range 60 {
			if v, ok := w.step().Release["v"]; ok {
				lifts = append(lifts, v)
			}
		}
	}
	assert.Equal(t, []float64{20, 32}, lifts)
}

func TestLocateRestsOnLeaving(t *testing.T) {
	// proven on the Wallbox, released to the full current; unplugged: back to the rest current at
	// once, and again later only while the car is still unplugged
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	w.xA = 14
	w.run(120)
	require.True(t, w.car().Proven)
	w.xOn, w.plug = false, false
	var at []float64
	for range 60 {
		if a, ok := w.step().Rest["v"]; ok {
			at = append(at, w.now)
			assert.Equal(t, 6.0, a, "the dumb charger's minimum, raised to the fail-safe by the site")
		}
	}
	require.Len(t, at, 2)
	assert.InDelta(t, locReleaseS, at[1]-at[0], 1)
}

func TestLocateRestNotResentOnceBackHome(t *testing.T) {
	// plugged into the dumb charger within the wait: it sets the current itself, no second rest
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	w.xA = 14
	w.run(120)
	w.xOn, w.plug = false, false
	n := 0
	for i := range 60 {
		if i == 5 {
			w.plug = true
			w.pilot = func(float64) (float64, bool) { return 32, true }
		}
		if _, ok := w.step().Rest["v"]; ok {
			n++
		}
	}
	assert.Equal(t, 1, n)
	assert.Equal(t, "v", w.owner["l"])
}

func TestLocateNoRestWithoutProof(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 32, true }
	w.run(60)
	w.plug = false
	for range 60 {
		assert.Empty(t, w.step().Rest)
	}
}

func TestLocateNoProbeWhileAway(t *testing.T) {
	// plugged in at a public charger: the dumb charger's geofence reports no car, nothing to probe
	w := newWorld(t)
	w.away = true
	w.pilot = func(float64) (float64, bool) { return 32, true }
	w.run(600)
	assert.Zero(t, w.car().Probes)
}

func TestLocateEchoIsNoProof(t *testing.T) {
	// the car reports the current it was told (its own setting), and the dumb charger tells it
	// exactly what the smart charger happens to command
	w := newWorld(t)
	w.lA = 16
	w.pilot = func(float64) (float64, bool) { return w.lA, true }
	w.run(30)
	w.xA, w.lA = 14, 14
	w.run(60)
	w.xA, w.lA = 12, 12
	w.run(60)
	assert.False(t, w.car().Proven)
	assert.Equal(t, "l", w.car().At)
}

func TestLocateZeroIsNoProof(t *testing.T) {
	// the car reads 0 when stopped, just as the smart charger switches off
	w := newWorld(t)
	w.pilot = func(now float64) (float64, bool) {
		if w.xA == 0 {
			return 0, true
		}
		return 32, true
	}
	w.run(30)
	w.xA = 0
	w.run(40)
	assert.False(t, w.car().Proven)
}

func TestLocateFrozenReadingIsNoProof(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 16, true } // asleep since a Wallbox session at 16 A
	for i := range 10 {
		w.xA = []float64{16, 14}[i%2]
		w.run(60)
	}
	assert.False(t, w.car().Proven)
}

func TestLocateSingleMatchIsNoProof(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(now float64) (float64, bool) {
		if now < 40 {
			return 32, true
		}
		return 16, true // one coincidental reading equal to x
	}
	w.run(200)
	assert.False(t, w.car().Proven)
}

func TestLocateRevokes(t *testing.T) {
	w := newWorld(t)
	moved := false
	follow := w.followsX()
	w.pilot = func(now float64) (float64, bool) {
		if moved {
			return 32, true // moved to the dumb charger, a guest took x without x disconnecting
		}
		return follow(now)
	}
	w.run(30)
	w.xA = 14
	w.run(40)
	require.True(t, w.car().Proven)
	moved = true
	w.run(30)
	c := w.car()
	assert.False(t, c.Proven)
	assert.Equal(t, 1, c.Revokes)
	assert.Equal(t, "l", c.At)
	assert.Equal(t, "v", w.owner["l"])
	assert.Empty(t, w.owner["x"])
}

func TestLocateUnplugOnSmartCharger(t *testing.T) {
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	w.xA = 14
	w.run(40)
	require.True(t, w.car().Proven)
	w.plug, w.xOn = false, false
	w.run(10)
	assert.False(t, w.car().Proven)
	assert.Empty(t, w.owner["l"], "an unplugged car goes nowhere")
}

func TestLocateRespectsManualPick(t *testing.T) {
	w := newWorld(t)
	w.pilot = w.followsX()
	w.owner["x"] = "v" // the owner chose it on the Wallbox, where it is
	for i := range 10 {
		w.xA = []float64{16, 14}[i%2]
		w.run(60)
	}
	c := w.car()
	assert.True(t, c.Manual)
	assert.Equal(t, "x", c.At)
	assert.Empty(t, w.assign)
	assert.Empty(t, w.probes, "a manual pick is not probed")
}

func TestLocateOverrulesWrongManualPick(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 32, true } // really on the dumb charger
	w.owner["x"] = "v"                                          // but chosen on the Wallbox
	w.run(30)
	c := w.car()
	assert.False(t, c.Manual)
	assert.Equal(t, "l", c.At, "evidence beats a wrong pick: the car must not charge uncontrolled")
	assert.Equal(t, "v", w.owner["l"])
	assert.Contains(t, c.Evidence, "does not follow")
}

func TestLocateOverrulesWrongPickWhileSmartChargerOff(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 32, true } // really on the dumb charger
	w.owner["x"] = "v"
	w.xA = 0 // the Wallbox waits for the cheap window: no pilot at all
	w.run(200)
	assert.Equal(t, "l", w.car().At)
	assert.Equal(t, "v", w.owner["l"])
}

func TestLocateKeepsProofWhileSmartChargerOff(t *testing.T) {
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(30)
	w.xA = 14
	w.run(40)
	require.True(t, w.car().Proven)
	frozen := 14.0
	w.pilot = func(float64) (float64, bool) { return frozen, true } // the car keeps its last reading
	w.xA = 0
	w.run(600)
	assert.True(t, w.car().Proven, "a car on a switched-off charger stays proven there")
}

func TestLocateManualPickOnEmptyCharger(t *testing.T) {
	w := newWorld(t)
	w.xOn = false
	w.owner["x"] = "v"
	w.run(5)
	assert.Equal(t, "l", w.car().At)
	assert.Equal(t, "v", w.owner["l"])
}

func TestLocateRefusesDefaultVehicle(t *testing.T) {
	w := newWorld(t)
	w.pilot = w.followsX()
	w.defVeh["x"] = "v"
	w.run(30)
	assert.Equal(t, "default", w.car().Error)
	assert.Equal(t, "Wallbox", w.car().ErrorOn)
	assert.Empty(t, w.last.Excluded)
	assert.Empty(t, w.assign)

	w = newWorld(t)
	w.defVeh["l"] = "z"
	w.run(5)
	assert.Equal(t, "homeDefault", w.car().Error)
}

func TestLocateProbeProves(t *testing.T) {
	w := newWorld(t)
	w.pilot = w.followsX()
	w.run(60) // x quiet at 16 A
	d := w.step()
	require.Contains(t, d.Probes, "x")
	assert.Equal(t, 14.0, d.Probes["x"])
	assert.True(t, w.car().Probing)
	w.run(40)
	c := w.car()
	assert.True(t, c.Proven)
	assert.Empty(t, w.probes, "proof ends the probe")
	assert.Equal(t, 1, c.Probes)
}

func TestLocateProbeBacksOffAndGivesUp(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 32, true } // on the dumb charger
	w.run(3600)
	c := w.car()
	assert.False(t, c.Proven)
	assert.Equal(t, locNegatives, c.Probes, "a car that never follows stops costing the smart charger")
	assert.Empty(t, w.probes)

	// x's next session is a new question
	w.xOn = false
	w.run(5)
	w.xOn = true
	w.run(120)
	assert.Equal(t, locNegatives+1, w.car().Probes)
}

func TestLocateNoProbeAtMinimum(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 6, true }
	w.xA = 6
	w.run(300)
	assert.Zero(t, w.car().Probes)
}

func TestLocateNoProbeWhileXMoves(t *testing.T) {
	w := newWorld(t)
	w.pilot = func(float64) (float64, bool) { return 32, true }
	for i := range 30 {
		w.xA = []float64{16, 15}[i%2]
		w.run(20)
	}
	assert.Zero(t, w.car().Probes)
}

func TestLocateUnpluggedCarIsNotProbed(t *testing.T) {
	w := newWorld(t)
	w.plug = false
	w.pilot = func(float64) (float64, bool) { return 16, true }
	w.run(300)
	assert.Zero(t, w.car().Probes)
	assert.Empty(t, w.owner)
}

func TestLocateCarWithoutPilot(t *testing.T) {
	w := newWorld(t)
	w.run(300)
	c := w.car()
	assert.False(t, c.CanProve)
	assert.Equal(t, "l", c.At)
	assert.Zero(t, c.Probes)
}
