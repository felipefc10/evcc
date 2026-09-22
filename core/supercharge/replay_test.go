package supercharge

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestReplay replays controller traces recorded from the reference Python
// implementation's simulator and requires the Go controller to issue the same
// commands at every step. Set SC_REPLAY_DIR to the directory of *.jsonl.gz files.
func TestReplay(t *testing.T) {
	dir := os.Getenv("SC_REPLAY_DIR")
	if dir == "" {
		t.Skip("SC_REPLAY_DIR not set")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl.gz"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no replay files in %s", dir)
	}
	sort.Strings(files)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			replayFile(t, f)
		})
	}
}

type pySettings map[string]any

func (p pySettings) f(k string) float64 {
	v, _ := p[k].(float64)
	return v
}

func (p pySettings) s(k string) string {
	v, _ := p[k].(string)
	return v
}

func settingsFromPy(p pySettings) Settings {
	return Settings{
		BurstKVA:           p.f("burst_kva"),
		BumpKVA:            p.f("bump_kva"),
		MarginS:            p.f("margin_s"),
		ResetS:             p.f("reset_s"),
		MaxCloseness:       p.f("max_closeness"),
		ExitLeadFrac:       p.f("exit_lead_frac"),
		BaseMarginKVA:      p.f("base_margin_kva"),
		AmpMin:             int(p.f("amp_min")),
		AmpMax:             int(p.f("amp_max")),
		RaiseTable:         p.s("raise_table"),
		ReduceTable:        p.s("reduce_table"),
		FloorDwellS:        p.f("floor_dwell_s"),
		RestartDwellS:      p.f("restart_dwell_s"),
		BlindHoldPolls:     int(p.f("blind_hold_polls")),
		BlindPolls:         int(p.f("blind_polls")),
		BlindAction:        p.s("blind_action"),
		BlindHoldA:         int(p.f("blind_hold_a")),
		BaseAbortCloseness: p.f("base_abort_closeness"),
		BaseStopCloseness:  p.f("base_stop_closeness"),
		BaseTiAbortS:       p.f("base_ti_abort_s"),
		BaseTiStopS:        p.f("base_ti_stop_s"),
		TrimMaxA:           p.f("trim_max_a"),
		MaxTempC:           p.f("max_temp_c"),
		Supercharge:        map[string]int64{},
	}
}

func scopeInto(s *Settings, scope string, sm *Sample) {
	s.Supercharge = map[string]int64{}
	for part := range strings.SplitSeq(scope, ",") {
		k := strings.TrimSpace(part)
		if k == "" {
			continue
		}
		if k == "all" {
			for _, lp := range sm.Lps {
				s.Supercharge[lp.Key] = 0
			}
			continue
		}
		s.Supercharge[k] = 0
	}
}

type recLine struct {
	Op       string     `json:"op"`
	Curve    *struct{ Q, K, Sc float64 } `json:"curve"`
	Settings pySettings `json:"settings"`
	Frac     *float64   `json:"frac"`
	Now      float64    `json:"now"`
	Keep     bool       `json:"keep"`
	Sample   *Sample    `json:"sample"`
	Cmd      *struct {
		Act    Act         `json:"act"`
		Amps   int         `json:"amps"`
		Reason string      `json:"reason"`
		Lps    []LpCommand `json:"lps"`
	} `json:"cmd"`
	Phase   Phase             `json:"phase"`
	Ti      float64           `json:"ti"`
	Trim    float64           `json:"trim"`
	Budget  float64           `json:"budget"`
	House   float64           `json:"house"`
	Ops     int               `json:"ops"`
	Writes  int               `json:"writes"`
	Records []json.RawMessage `json:"records"`
}

func replayFile(t *testing.T, path string) {
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	gz, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 1<<24)

	var (
		ctl   *Controller
		set   *Settings
		scope string
		line  int
		steps int
	)
	for sc.Scan() {
		line++
		var r recLine
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("line %d: %v", line, err)
		}
		switch r.Op {
		case "new":
			s := settingsFromPy(r.Settings)
			set = &s
			scope = r.Settings.s("supercharge_scope")
			ctl = NewController(Curve{Q: r.Curve.Q, K: r.Curve.K, SCkVA: r.Curve.Sc}, set)
			if r.Frac != nil {
				ctl.HouseBaselineFrac = *r.Frac
			}
		case "start":
			ctl.Start(r.Now, r.Keep)
		case "stop":
			ctl.Stop()
		case "step":
			steps++
			if r.Settings != nil {
				*set = settingsFromPy(r.Settings)
				scope = r.Settings.s("supercharge_scope")
			}
			if r.Frac != nil {
				ctl.HouseBaselineFrac = *r.Frac
			}
			sm := r.Sample
			scopeInto(set, scope, sm)
			cmd := ctl.Step(sm)

			fail := func(what string, got, want any) {
				t.Fatalf("line %d (t=%.2f): %s: got %v want %v\ngo cmd: %+v\npy cmd: %+v", line, sm.T, what, got, want, cmd, *r.Cmd)
			}
			if cmd.Act != r.Cmd.Act {
				fail("act", cmd.Act, r.Cmd.Act)
			}
			if cmd.Amps != r.Cmd.Amps {
				fail("amps", cmd.Amps, r.Cmd.Amps)
			}
			if cmd.Reason != r.Cmd.Reason {
				fail("reason", cmd.Reason, r.Cmd.Reason)
			}
			if len(cmd.Lps) != len(r.Cmd.Lps) {
				fail("lps len", len(cmd.Lps), len(r.Cmd.Lps))
			}
			for i, want := range r.Cmd.Lps {
				got := cmd.Lps[i]
				if got.Key != want.Key || got.Act != want.Act || got.Amps != want.Amps || got.Repeat != want.Repeat || got.Reason != want.Reason {
					fail("lp cmd "+want.Key, got, want)
				}
			}
			if ctl.Phase != r.Phase {
				fail("phase", ctl.Phase, r.Phase)
			}
			if math.Abs(ctl.ti-r.Ti) > 1e-9 {
				fail("ti", ctl.ti, r.Ti)
			}
			if math.Abs(ctl.trimA-r.Trim) > 1e-9 {
				fail("trim", ctl.trimA, r.Trim)
			}
			if math.Abs(ctl.Tel.BudgetA-r.Budget) > 0.051 {
				fail("budget", ctl.Tel.BudgetA, r.Budget)
			}
			if math.Abs(ctl.Tel.HouseKVA-r.House) > 0.0011 {
				fail("house", ctl.Tel.HouseKVA, r.House)
			}
			if ctl.Tel.ContactorOps != r.Ops {
				fail("ops", ctl.Tel.ContactorOps, r.Ops)
			}
			if ctl.Tel.Writes != r.Writes {
				fail("writes", ctl.Tel.Writes, r.Writes)
			}
			if len(r.Records) > 0 {
				got := ctl.Records[len(ctl.Records)-len(r.Records):]
				for i, raw := range r.Records {
					var want map[string]any
					_ = json.Unmarshal(raw, &want)
					g := got[i]
					check := map[string]float64{
						"lasted_s": g.LastedS, "window_s": g.WindowS, "peak_va_kva": g.PeakVaKVA,
						"peak_closeness": g.PeakCloseness, "settled_va_kva": g.SettledVaKVA,
						"target_a": float64(g.TargetA), "cmds": float64(g.Cmds), "wall_s": g.WallS,
					}
					for k, v := range check {
						if w, _ := want[k].(float64); math.Abs(w-v) > 0.0011 {
							fail("record "+k, v, w)
						}
					}
					if want["exit"] != g.Exit {
						fail("record exit", g.Exit, want["exit"])
					}
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d steps identical", steps)
}
