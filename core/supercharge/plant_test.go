package supercharge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedMeter float64

func (m fixedMeter) CurrentPower() (float64, error) {
	return float64(m), nil
}

type slowMeter struct{ release chan struct{} }

func (m slowMeter) CurrentPower() (float64, error) {
	<-m.release
	return 100, nil
}

func TestShellyGen1AndGen2(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		kva, w     float64
		ok         bool
	}{
		{"gen1", `{"power":3000,"reactive":400,"voltage":231,"is_valid":true}`, 3.0265, 3000, true},
		{"gen1 invalid", `{"power":3000,"reactive":400,"voltage":231,"is_valid":false}`, 0, 0, false},
		{"gen2", `{"id":0,"act_power":-1200,"aprt_power":1300,"voltage":229}`, 1.3, -1200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			rd := NewShellyMeter(srv.URL).Read(context.Background())
			require.Equal(t, tc.ok, rd.OK, rd.Err)
			if tc.ok {
				assert.InDelta(t, tc.kva, rd.VaKVA, 1e-3)
				assert.Equal(t, tc.w, rd.Watts)
			}
		})
	}
}

func TestGridMeter(t *testing.T) {
	rd := NewGridMeter(fixedMeter(-2500)).Read(context.Background())
	require.True(t, rd.OK, rd.Err)
	assert.Equal(t, 2.5, rd.VaKVA)
	assert.Equal(t, -2500.0, rd.Watts)

	// a stuck meter is no reading, and is not asked again until it answers
	slow := slowMeter{release: make(chan struct{})}
	m := NewGridMeter(slow)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	assert.False(t, m.Read(ctx).OK)
	assert.ErrorContains(t, m.Read(context.Background()).Err, "pending")
	close(slow.release)
	require.Eventually(t, func() bool { return m.Read(context.Background()).OK }, time.Second, 5*time.Millisecond)
}

func TestSolarHeadroom(t *testing.T) {
	s := DefaultSettings()
	c := NewController(Curve{Q: 50, K: 1.2, SCkVA: 3.45}, &s)
	goal := 4.0
	export := &Sample{OK: true, VaKVA: 1.0, Watts: -1000, Volt: 230}

	// without a PV meter an export is not credited: the house nets at zero at most
	assert.InDelta(t, goal*1000/230, c.headroomA(goal, export, false), 1e-6)

	// with PV the export is headroom on top of the line
	export.PvKVA = 3.0
	assert.InDelta(t, (goal+1.0)*1000/230, c.headroomA(goal, export, false), 1e-6)

	// never more than the panels deliver
	export.VaKVA, export.Watts = 5.0, -5000
	assert.InDelta(t, (goal+3.0)*1000/230, c.headroomA(goal, export, false), 1e-6)
}

func TestSinglePhaseOnly(t *testing.T) {
	m := newTestManager(t)
	m.cfg.Enabled = true
	m.AddLoadpoint(0, "lp-1", &fakeLp{st: LpState{Title: "Garage", Phases: 3, Connected: true, DemandA: 16, MinA: 6, MaxA: 16}})

	m.mu.Lock()
	unsupported := m.checkPhasesLocked([]LpState{m.lps[0].lp.SuperchargeState()})
	m.mu.Unlock()
	require.True(t, unsupported)

	// acts as switched off: the fail-safe limit holds
	assert.Equal(t, 6.0, m.Clamp("lp-1", 16, 6, true, 0))
}

func TestExportImport(t *testing.T) {
	src := newTestManager(t)
	src.cfg.Enabled = true
	src.cfg.MeterURI = "http://meter/emeter/0"
	src.cfg.ContractKVA = 6.9
	src.cfg.Loadpoints = map[string]LpConfig{"lp-1": {Fast: true, MeasureUnit: "A"}}
	src.cfg.Settings.BurstKVA = 7.5
	src.learner.Command("lp-1:car", 0, 0, 10, "Garage")
	src.bursts = []BurstRecord{{Loadpoint: "lp-1", Exit: "done"}}

	b, err := src.Export()
	require.NoError(t, err)
	raw, err := json.Marshal(b)
	require.NoError(t, err)

	var in Backup
	require.NoError(t, json.Unmarshal(raw, &in))
	dst := NewManager(util.NewLogger("test"), memStore{}, nil, func(string, any) {}, Plant{})
	require.NoError(t, dst.Import(in))

	cfg := dst.Config()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "http://meter/emeter/0", cfg.MeterURI)
	assert.Equal(t, 6.9, cfg.ContractKVA)
	assert.Equal(t, 7.5, cfg.Settings.BurstKVA)
	assert.True(t, cfg.Loadpoints["lp-1"].Fast)
	assert.Len(t, dst.Bursts(), 1)
	assert.Equal(t, src.learner.All(), dst.learner.All())

	in.Version = 99
	assert.Error(t, dst.Import(in))
}

type fakeLp struct{ st LpState }

func (f *fakeLp) SuperchargeState() LpState               { return f.st }
func (f *fakeLp) SuperchargeCurrents() ([]float64, error) { return nil, nil }
func (f *fakeLp) SuperchargeApply(int) error              { return nil }
