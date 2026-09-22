package supercharge

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// TestLearnReplay replays a random operation sequence recorded from the reference
// Python learner. Set SC_LEARN_REPLAY to the recorded JSON file.
func TestLearnReplay(t *testing.T) {
	path := os.Getenv("SC_LEARN_REPLAY")
	if path == "" {
		t.Skip("SC_LEARN_REPLAY not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ops [][]json.RawMessage
	if err := json.Unmarshal(raw, &ops); err != nil {
		t.Fatal(err)
	}
	l := NewLearner()
	f := func(m json.RawMessage) float64 {
		var v float64
		_ = json.Unmarshal(m, &v)
		return v
	}
	s := func(m json.RawMessage) string {
		var v string
		_ = json.Unmarshal(m, &v)
		return v
	}
	queries := 0
	for i, op := range ops {
		switch s(op[0]) {
		case "c":
			l.Command(s(op[1]), f(op[2]), f(op[3]), f(op[4]), "")
		case "o":
			l.Observe(s(op[1]), f(op[2]), f(op[3]))
		case "s":
			l.Supply(f(op[1]), f(op[2]), f(op[3]), f(op[4]))
		case "e":
			l.Entry(s(op[1]), f(op[2]), "")
		case "q":
			queries++
			var out map[string][4]float64
			_ = json.Unmarshal(op[1], &out)
			var beh map[string]map[string]any
			_ = json.Unmarshal(op[2], &beh)
			if got, want := l.SourceOhm(), f(op[3]); math.Abs(got-want) > 1e-12 {
				t.Fatalf("op %d: source ohm %v want %v", i, got, want)
			}
			for k, want := range out {
				got := [4]float64{l.RampDown(k, 2.0), l.Latency(k, 2.0), l.LatencyUp(k, 2.0), l.EntryOffset(k)}
				for j := range got {
					if math.Abs(got[j]-want[j]) > 1e-9 {
						t.Fatalf("op %d key %s value %d: got %v want %v", i, k, j, got, want)
					}
				}
				b := l.Behaviour(k)
				for name, v := range map[string]float64{
					"ramp_up": b.RampUp, "ramp_down": b.RampDown, "latency_down": b.LatencyDown,
					"latency_up": b.LatencyUp, "entry_offset_a": b.EntryOffsetA, "ups": float64(b.Ups),
					"downs": float64(b.Downs), "entries": float64(b.Entries),
				} {
					if w, _ := beh[k][name].(float64); math.Abs(w-v) > 1e-9 {
						t.Fatalf("op %d key %s %s: got %v want %v", i, k, name, v, w)
					}
				}
			}
		}
	}
	t.Logf("%d ops, %d queries identical", len(ops), queries)
}
