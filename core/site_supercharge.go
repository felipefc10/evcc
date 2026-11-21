package core

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/circuit"
	"github.com/evcc-io/evcc/core/coordinator"
	"github.com/evcc-io/evcc/core/supercharge"
	settingsdb "github.com/evcc-io/evcc/db/settings"
	"github.com/evcc-io/evcc/plugin/mqtt"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/config"
)

// superchargeStore persists the load manager in the settings database
type superchargeStore struct{}

func (superchargeStore) Load(key string, v any) error {
	return settingsdb.Json(key, v)
}

func (superchargeStore) Save(key string, v any) error {
	return settingsdb.SetJson(key, v)
}

// superchargeSubscribe subscribes to a topic on evcc's own MQTT connection
func superchargeSubscribe(topic string, cb func(string)) error {
	if mqtt.Instance == nil {
		return errors.New("mqtt not configured")
	}
	return mqtt.Instance.Listen(topic, cb)
}

// superchargePublish publishes a charger lease on evcc's own MQTT connection. Never
// retained: a stored lease would be applied, stale, by a bridge that restarts.
func superchargePublish(topic, payload string) {
	if mqtt.Instance != nil {
		mqtt.Instance.Publish(topic, false, payload)
	}
}

// prepareSupercharge creates the whole-house load manager and attaches every loadpoint
func (site *Site) prepareSupercharge() {
	var plant supercharge.Plant
	if site.gridMeter != nil {
		plant.Grid = site.gridMeter.Instance()
	}
	if len(site.pvMeters) > 0 {
		plant.PV = func() float64 { return site.state().pvPower }
	}

	site.supercharge = supercharge.NewManager(util.NewLogger("supercharge"), superchargeStore{}, superchargeSubscribe, site.publish, plant)
	site.supercharge.SetPublisher(superchargePublish)
	circuit.SetStandIn(site.supercharge.Owns)

	lpDevices := config.Loadpoints().Devices()
	for id, lp := range site.loadpoints {
		if lp == nil {
			continue
		}
		name := lp.GetTitle()
		if id < len(lpDevices) {
			name = lpDevices[id].Config().Name
		}
		site.supercharge.AddLoadpoint(id, name, lp)
		lp.superchargeRegister(site.supercharge, name)
	}
}

// Supercharge returns the whole-house load manager
func (site *Site) Supercharge() *supercharge.Manager {
	return site.supercharge
}

// runSupercharge runs the load manager until stopC closes
func (site *Site) runSupercharge(stopC chan struct{}) {
	if site.supercharge == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopC
		cancel()
	}()
	go site.supercharge.Run(ctx)

	// status guessing must leave the configured cars alone from the very first loadpoint cycle
	var excluded atomic.Pointer[map[api.Vehicle]bool]
	excluded.Store(site.dumbCarVehicles(site.supercharge.DumbCars()))
	coordinator.Excluded = func(v api.Vehicle) bool {
		return (*excluded.Load())[v]
	}
	go site.runCarDetection(ctx, &excluded)
}

// dumbCarVehicles returns the instances of the cars configured on dumb chargers
func (site *Site) dumbCarVehicles(pairs map[string]string) *map[api.Vehicle]bool {
	res := map[api.Vehicle]bool{}
	for _, dev := range config.Vehicles().Devices() {
		for _, name := range pairs {
			if dev.Config().Name == name {
				res[dev.Instance()] = true
			}
		}
	}
	return &res
}

// carSampleS is how often car detection reads each configured car's own api
const carSampleS = 5

// runCarDetection places cars configured on dumb chargers, see supercharge/locate.go.
// It is the only caller of SetVehicle besides the user, so takeovers never cross.
func (site *Site) runCarDetection(ctx context.Context, excluded *atomic.Pointer[map[api.Vehicle]bool]) {
	log := util.NewLogger("cars")
	loc := supercharge.NewLocator()
	start := time.Now()
	cars := map[string]supercharge.LocCar{}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for n := 0; ; n++ {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}

		pairs := site.supercharge.DumbCars()
		if len(pairs) == 0 {
			if len(cars) > 0 || len(*excluded.Load()) > 0 {
				clear(cars)
				excluded.Store(&map[api.Vehicle]bool{})
				site.supercharge.SetLocation(nil, nil)
			}
			continue
		}

		vehicles := map[string]api.Vehicle{}
		names := map[api.Vehicle]string{}
		for _, dev := range config.Vehicles().Devices() {
			v := dev.Instance()
			vehicles[dev.Config().Name] = v
			names[v] = dev.Config().Name
		}

		lpDevices := config.Loadpoints().Devices()
		lps := make([]supercharge.LocLp, 0, len(site.loadpoints))
		byName := map[string]*Loadpoint{}
		smartCar := false // a car on a charger other than a dumb one: only then can a pilot tell anything
		for id, lp := range site.loadpoints {
			if lp == nil {
				continue
			}
			name := lp.GetTitle()
			if id < len(lpDevices) {
				name = lpDevices[id].Config().Name
			}
			st := lp.SuperchargeState()
			l := supercharge.LocLp{
				Name: name, Title: st.Title, Connected: st.Connected, Enabled: st.Enabled,
				OfferedA: st.OfferedA, MinA: st.MinA, MaxA: st.MaxA,
				DefaultVehicle: lp.GetDefaultVehicleRef(),
			}
			if v := lp.GetVehicle(); v != nil {
				l.Vehicle = names[v]
			}
			lps = append(lps, l)
			byName[name] = lp
			if _, dumb := pairs[name]; !dumb && st.Connected {
				smartCar = true
			}
		}

		// read each car's own api, sparingly: cloud apis are rate limited
		sample := n%carSampleS == 0
		for _, vName := range pairs {
			v, ok := vehicles[vName]
			if !ok {
				delete(cars, vName)
				continue
			}
			c := cars[vName]
			c.Title = v.GetTitle()
			c.PilotOK = false
			cg, canProve := api.Cap[api.CurrentGetter](v)
			c.CanProve = canProve
			if sample {
				if cs, ok := api.Cap[api.ChargeState](v); ok {
					status, err := cs.Status()
					c.StatusOK = err == nil
					c.Plugged = err == nil && (status == api.StatusB || status == api.StatusC)
				}
				if canProve && c.StatusOK && c.Plugged && smartCar {
					if a, err := cg.GetMaxCurrent(); err == nil {
						c.Pilot, c.PilotOK = a, true
					}
				}
			}
			cars[vName] = c
		}

		d := loc.Step(time.Since(start).Seconds(), pairs, lps, cars)

		ex := map[api.Vehicle]bool{}
		for vName := range d.Excluded {
			if v, ok := vehicles[vName]; ok {
				ex[v] = true
			}
		}
		excluded.Store(&ex)

		for _, a := range d.Assign {
			lp := byName[a.Loadpoint]
			if lp == nil {
				continue
			}
			if a.Vehicle == "" {
				log.INFO.Printf("%s: releasing %s", lp.GetTitle(), titleOf(lp.GetVehicle()))
				lp.SetVehicle(nil)
				continue
			}
			v := vehicles[a.Vehicle]
			log.INFO.Printf("%s: %s is here (%s)", lp.GetTitle(), v.GetTitle(), evidenceOf(d.Cars, a.Vehicle))
			lp.SetVehicle(v)
		}

		// the dumb charger may have stopped the car or lowered its own current setting through
		// the car's api: undo both, the smart charger's pilot and enable govern from now on
		for vName, amps := range d.Release {
			go func(v api.Vehicle) {
				if cc, ok := api.Cap[api.ChargeController](v); ok {
					if err := cc.ChargeEnable(true); err != nil {
						log.WARN.Printf("%s: allowing the car to charge: %v", v.GetTitle(), err)
					}
				}
				if cc, ok := api.Cap[api.CurrentController](v); ok && amps > 0 {
					if err := cc.MaxCurrent(int64(amps)); err != nil {
						log.WARN.Printf("%s: lifting the car's own current setting: %v", v.GetTitle(), err)
					}
				}
			}(vehicles[vName])
		}

		for vName, minA := range d.Rest {
			amps := site.supercharge.RestAmps(minA)
			go func(v api.Vehicle) {
				if cc, ok := api.Cap[api.CurrentController](v); ok {
					if err := cc.MaxCurrent(int64(amps)); err != nil {
						log.WARN.Printf("%s: setting the car's own current back to %.0f A: %v", v.GetTitle(), amps, err)
					}
				}
			}(vehicles[vName])
		}

		site.supercharge.SetLocation(d.Probes, d.Cars)
	}
}

func titleOf(v api.Vehicle) string {
	if v == nil {
		return "unknown"
	}
	return v.GetTitle()
}

func evidenceOf(cars []supercharge.CarLocation, vehicle string) string {
	for _, c := range cars {
		if c.Vehicle == vehicle {
			return c.Evidence
		}
	}
	return ""
}
