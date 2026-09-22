package supercharge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
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

// ShellyMeter reads a Shelly EM channel (/emeter/N) directly. It deliberately uses
// no keep-alive: a standing connection has rebooted a Gen1 Shelly on this plant.
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
	Power    *float64 `json:"power"`
	Reactive *float64 `json:"reactive"`
	Voltage  *float64 `json:"voltage"`
	IsValid  *bool    `json:"is_valid"`
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
	if r.IsValid == nil || !*r.IsValid {
		res.Err = errors.New("meter: reading not valid")
		return res
	}
	if r.Power == nil {
		res.Err = errors.New("meter: no power field")
		return res
	}
	res.Watts = *r.Power
	if r.Reactive != nil {
		res.Var = *r.Reactive
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
