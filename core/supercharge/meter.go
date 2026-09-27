package supercharge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/evcc-io/evcc/api"
)

// Reading is one apparent power measurement at the meter
type Reading struct {
	OK      bool
	VaKVA   float64
	Watts   float64
	Var     float64
	Volts   float64
	Latency time.Duration
	Err     error
}

// Meter reads the grid connection
type Meter interface {
	Read(ctx context.Context) Reading
}

// ShellyMeter reads a Shelly EM channel directly: Gen1 /emeter/N or Gen2 /rpc/EM1.GetStatus?id=N.
// It deliberately uses no keep-alive: a standing connection has rebooted a Gen1 Shelly.
type ShellyMeter struct {
	URI    string
	client *http.Client
}

// NewShellyMeter creates a meter reader
func NewShellyMeter(uri string) *ShellyMeter {
	return &ShellyMeter{
		URI: uri,
		client: &http.Client{
			Timeout:   2 * time.Second,
			Transport: &http.Transport{DisableKeepAlives: true, Proxy: http.ProxyFromEnvironment},
		},
	}
}

type emeterResponse struct {
	// Gen1
	Power    *float64 `json:"power"`
	Reactive *float64 `json:"reactive"`
	IsValid  *bool    `json:"is_valid"`
	// Gen2
	ActPower  *float64 `json:"act_power"`
	AprtPower *float64 `json:"aprt_power"`
	// both
	Voltage *float64 `json:"voltage"`
}

// Read performs one reading. A reading the meter marks invalid is no reading.
func (m *ShellyMeter) Read(ctx context.Context) Reading {
	start := time.Now()
	res := Reading{}
	if m.URI == "" {
		res.Err = errors.New("meter not configured")
		return res
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URI, nil)
	if err != nil {
		res.Err = err
		return res
	}
	resp, err := m.client.Do(req)
	res.Latency = time.Since(start)
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		res.Err = err
		return res
	}
	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("meter: http %d", resp.StatusCode)
		return res
	}
	var r emeterResponse
	if err := json.Unmarshal(body, &r); err != nil {
		res.Err = fmt.Errorf("meter: %w", err)
		return res
	}
	switch {
	case r.ActPower != nil:
		res.Watts = *r.ActPower
		if r.AprtPower != nil {
			res.Var = math.Sqrt(math.Max(*r.AprtPower**r.AprtPower-res.Watts*res.Watts, 0))
		}
	case r.IsValid == nil || !*r.IsValid:
		res.Err = errors.New("meter: reading not valid")
		return res
	case r.Power == nil:
		res.Err = errors.New("meter: no power field")
		return res
	default:
		res.Watts = *r.Power
		if r.Reactive != nil {
			res.Var = *r.Reactive
		}
	}
	if r.Voltage != nil {
		res.Volts = *r.Voltage
	}
	for _, v := range []float64{res.Watts, res.Var, res.Volts} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			res.Err = errors.New("meter: implausible value")
			return res
		}
	}
	res.VaKVA = ApparentKVA(res.Watts, res.Var)
	res.OK = true
	return res
}

// GridMeter reads evcc's own grid meter on the control loop's cadence. evcc meters report
// real power only, so the apparent power the ICP trips on is taken as the real power.
type GridMeter struct {
	meter api.Meter
	busy  atomic.Bool
}

// NewGridMeter creates a reader of evcc's grid meter
func NewGridMeter(meter api.Meter) *GridMeter {
	return &GridMeter{meter: meter}
}

// Read performs one reading. A meter slower than ctx is no reading, and is not asked again
// until it has answered: a stuck device must not pile up requests.
func (m *GridMeter) Read(ctx context.Context) Reading {
	start := time.Now()
	if !m.busy.CompareAndSwap(false, true) {
		return Reading{Err: errors.New("meter: previous reading still pending")}
	}

	ch := make(chan Reading, 1)
	go func() {
		defer m.busy.Store(false)
		var res Reading
		res.Watts, res.Err = m.meter.CurrentPower()
		if res.Err == nil {
			if pv, ok := api.Cap[api.PhaseVoltages](m.meter); ok {
				if u, _, _, err := pv.Voltages(); err == nil {
					res.Volts = u
				}
			}
		}
		ch <- res
	}()

	var res Reading
	select {
	case res = <-ch:
	case <-ctx.Done():
		res.Err = ctx.Err()
	}
	res.Latency = time.Since(start)
	if res.Err == nil && (math.IsNaN(res.Watts) || math.IsInf(res.Watts, 0)) {
		res.Err = errors.New("implausible value")
	}
	if res.Err != nil {
		res.Err = fmt.Errorf("meter: %w", res.Err)
		return res
	}
	res.VaKVA = math.Abs(res.Watts) / 1000.0
	res.OK = true
	return res
}

// noMeter is the reading when neither a meter address nor an evcc grid meter is configured
type noMeter struct{}

func (noMeter) Read(context.Context) Reading {
	return Reading{Err: errors.New("meter not configured")}
}
