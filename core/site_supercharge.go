package core

import (
	"context"
	"errors"

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

// prepareSupercharge creates the whole-house load manager and attaches every loadpoint
func (site *Site) prepareSupercharge() {
	site.supercharge = supercharge.NewManager(util.NewLogger("supercharge"), superchargeStore{}, superchargeSubscribe, site.publish)

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
}
