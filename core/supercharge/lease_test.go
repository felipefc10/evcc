package supercharge

import (
	"math"
	"testing"

	"github.com/evcc-io/evcc/util"
)

func leaseSettings(leaseS float64) Settings {
	s := DefaultSettings()
	s.LeaseS = leaseS
	return s
}

func TestLeaseHoldUnderTheLineIsTheSetting(t *testing.T) {
	cv := DefaultConfig().Curve()
	for _, ph := range []Phase{PhaseIdle, PhaseBase} {
		if got := leaseHoldS(Telemetry{Phase: ph, VaKVA: 4.0}, cv, leaseSettings(60)); got != 60 {
			t.Errorf("%s under the line: hold %v, want the setting 60", ph, got)
		}
	}
}

func TestLeaseHoldBalancingOverTheLine(t *testing.T) {
	cv := DefaultConfig().Curve()
	s := leaseSettings(60)
	// a house surge holds the base over the line: never longer than the base backstop allows
	tel := Telemetry{Phase: PhaseBase, VaKVA: 6.5, TiS: 10}
	want := s.BaseStopCloseness*cv.Tdisp(6.5) - 10 + leaseSlackS
	if got := leaseHoldS(tel, cv, s); math.Abs(got-want) > 1e-9 || got >= 60 {
		t.Fatalf("hold %v, want %v (< 60)", got, want)
	}
}

func TestLeaseHoldInBurstFollowsThePlannedExit(t *testing.T) {
	cv := DefaultConfig().Curve()
	s := leaseSettings(60)
	exitAt := math.Min(cv.PlannedCloseness(s.BurstKVA, s.BumpKVA, s.MarginS), s.MaxCloseness)
	planned := exitAt * cv.Tdisp(s.BurstKVA)

	// burst entry: the current is still ramping and the house is under the line. The hold is
	// the planned burst window, not "forever" (the bug the simulator found)
	entry := Telemetry{Phase: PhaseBurst, VaKVA: 4.0}
	if got := leaseHoldS(entry, cv, s); math.Abs(got-(planned+leaseSlackS)) > 1e-9 || got > 20 {
		t.Fatalf("entry hold %v, want the planned window %v + slack", got, planned)
	}

	// banked meter time and the shed lead bring the deadline in
	mid := Telemetry{Phase: PhaseBurst, VaKVA: s.BurstKVA, TiS: 4, ExitLeadS: 0.5}
	if got := leaseHoldS(mid, cv, s); math.Abs(got-(planned-4.5+leaseSlackS)) > 1e-9 {
		t.Fatalf("mid-burst hold %v, want %v", got, planned-4.5+leaseSlackS)
	}

	// the house load rises above the target: the meter is less patient, the deadline moves in
	high := mid
	high.VaKVA = s.BurstKVA + 1
	if leaseHoldS(high, cv, s) >= leaseHoldS(mid, cv, s) {
		t.Fatal("a higher load must shorten the hold")
	}

	// past the planned exit: only the slack
	past := Telemetry{Phase: PhaseBurst, VaKVA: s.BurstKVA, TiS: 60}
	if got := leaseHoldS(past, cv, s); got != leaseSlackS {
		t.Fatalf("past exit: hold %v, want %v", got, leaseSlackS)
	}

	// never longer than the setting
	if got := leaseHoldS(entry, cv, leaseSettings(15)); got > 15 {
		t.Fatalf("hold %v above the setting", got)
	}
}

func TestLeaseFallback(t *testing.T) {
	for _, c := range []struct {
		failsafe, minA float64
		want           int
	}{
		{7, 6, 7},
		{7.9, 6, 7}, // never rounds up
		{3, 6, 6},   // never below the charger's minimum
		{0, 6, 6},   // no limit while off: the minimum
		{0, 0, 0},
	} {
		if got := leaseFallbackA(c.failsafe, c.minA); got != c.want {
			t.Errorf("leaseFallbackA(%v, %v) = %d, want %d", c.failsafe, c.minA, got, c.want)
		}
	}
}

func TestLeasePayload(t *testing.T) {
	for _, c := range []struct {
		amps, fallback int
		hold           float64
		want           string
	}{
		{28, 7, 18.37, "28 18.4 7 5"},
		{16, 7, 60, "16 60.0 7 5"},
		{7, 7, 60, "0 5"}, // at the fallback: release
		{0, 7, 60, "0 5"}, // paused
	} {
		if got := leasePayload(c.amps, c.fallback, c.hold, 5); got != c.want {
			t.Errorf("leasePayload(%d, %d, %v) = %q, want %q", c.amps, c.fallback, c.hold, got, c.want)
		}
	}
}

type pubLog struct{ msgs [][2]string }

func (p *pubLog) pub(topic, payload string) { p.msgs = append(p.msgs, [2]string{topic, payload}) }

func leaseTestManager(t *testing.T) (*Manager, *pubLog) {
	t.Helper()
	m := newTestManager(t)
	if _, err := m.UpdateConfig([]byte(`{"failsafeA": 7, "loadpoints": {"wb": {"leaseTopic": "wallbox/lease/set"}}}`)); err != nil {
		t.Fatal(err)
	}
	p := &pubLog{}
	m.SetPublisher(p.pub)
	m.leaseSeq = 0 // readable numbers in the tests; the real start comes from the clock
	return m, p
}

func lpRow(key string, sp int) LpTelemetry {
	return LpTelemetry{Key: key, SetpointA: sp, MinA: 6}
}

func expectMsgs(t *testing.T, p *pubLog, want [][2]string) {
	t.Helper()
	if len(p.msgs) != len(want) {
		t.Fatalf("published %v, want %v", p.msgs, want)
	}
	for i := range want {
		if p.msgs[i] != want[i] {
			t.Fatalf("message %d = %v, want %v", i, p.msgs[i], want[i])
		}
	}
}

func TestLeasePublishRenewsEveryStepAndReleasesOnce(t *testing.T) {
	m, p := leaseTestManager(t)
	tel := func(sp int) Telemetry {
		return Telemetry{Phase: PhaseBase, VaKVA: 4.0, Lps: []LpTelemetry{lpRow("wb", sp), lpRow("other", 20)}}
	}
	m.mu.Lock()
	m.publishLeasesLocked(tel(6))  // never leased, at the fallback: nothing to say
	m.publishLeasesLocked(tel(16)) // lease
	m.publishLeasesLocked(tel(16)) // renewal: the heartbeat, sent again
	m.publishLeasesLocked(tel(6))  // at or below the fallback: release
	m.publishLeasesLocked(tel(0))  // still released: not repeated
	m.publishLeasesLocked(tel(20))
	m.mu.Unlock()
	expectMsgs(t, p, [][2]string{
		{"wallbox/lease/set", "16 60.0 7 1"},
		{"wallbox/lease/set", "16 60.0 7 2"},
		{"wallbox/lease/set", "0 3"}, // releases are numbered too: a late lease cannot overtake one
		{"wallbox/lease/set", "20 60.0 7 4"},
	})
}

func TestLeaseUsesTheSetting(t *testing.T) {
	m, p := leaseTestManager(t)
	if _, err := m.UpdateConfig([]byte(`{"settings": {"leaseS": 45}}`)); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.publishLeasesLocked(Telemetry{Phase: PhaseBase, VaKVA: 4.0, Lps: []LpTelemetry{lpRow("wb", 16)}})
	m.mu.Unlock()
	expectMsgs(t, p, [][2]string{{"wallbox/lease/set", "16 45.0 7 1"}})
}

func TestLeaseSettingIsClamped(t *testing.T) {
	m, _ := leaseTestManager(t)
	for in, want := range map[string]float64{"5": 15, "60": 60, "9999": 120} {
		if _, err := m.UpdateConfig([]byte(`{"settings": {"leaseS": ` + in + `}}`)); err != nil {
			t.Fatal(err)
		}
		if got := m.cfg.Settings.LeaseS; got != want {
			t.Errorf("leaseS %s -> %v, want %v", in, got, want)
		}
	}
}

func TestLeaseReleasedWhenHandingBack(t *testing.T) {
	m, p := leaseTestManager(t)
	m.mu.Lock()
	m.publishLeasesLocked(Telemetry{Phase: PhaseBase, VaKVA: 4.0, Lps: []LpTelemetry{lpRow("wb", 16)}})
	m.releaseLeasesLocked()
	m.releaseLeasesLocked() // nothing left to release
	m.mu.Unlock()
	expectMsgs(t, p, [][2]string{{"wallbox/lease/set", "16 60.0 7 1"}, {"wallbox/lease/set", "0 2"}})
}

func TestLeaseTopicChangeReleasesTheOldTopic(t *testing.T) {
	m, p := leaseTestManager(t)
	tel := Telemetry{Phase: PhaseBase, VaKVA: 4.0, Lps: []LpTelemetry{lpRow("wb", 16)}}
	m.mu.Lock()
	m.publishLeasesLocked(tel)
	m.mu.Unlock()

	if _, err := m.UpdateConfig([]byte(`{"loadpoints": {"wb": {"leaseTopic": "new/lease/set"}}}`)); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.publishLeasesLocked(tel)
	m.mu.Unlock()

	if _, err := m.UpdateConfig([]byte(`{"loadpoints": {"wb": {"leaseTopic": ""}}}`)); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.publishLeasesLocked(tel)
	m.publishLeasesLocked(tel) // nothing more: no topic, nothing leased
	m.mu.Unlock()

	expectMsgs(t, p, [][2]string{
		{"wallbox/lease/set", "16 60.0 7 1"},
		{"wallbox/lease/set", "0 2"}, // the old topic is released, not left to expire
		{"new/lease/set", "16 60.0 7 3"},
		{"new/lease/set", "0 4"}, // cleared: released
	})
}

func TestLeaseNothingWithoutTopicOrPublisher(t *testing.T) {
	m := newTestManager(t) // no publisher
	m.mu.Lock()
	m.publishLeasesLocked(Telemetry{Lps: []LpTelemetry{lpRow("wb", 16)}})
	m.releaseLeasesLocked()
	m.mu.Unlock()

	m2, p := leaseTestManager(t)
	m2.mu.Lock()
	m2.publishLeasesLocked(Telemetry{Lps: []LpTelemetry{lpRow("no-topic", 16)}})
	m2.mu.Unlock()
	if len(p.msgs) != 0 {
		t.Fatalf("published %v for a loadpoint without a lease topic", p.msgs)
	}
}

func TestLeaseSeqStartsFromTheClock(t *testing.T) {
	m := newTestManager(t)
	if m.leaseSeq < 1_700_000_000_000 {
		t.Fatalf("leaseSeq starts at %d: a restarted evcc must count on above the last run", m.leaseSeq)
	}
}

func TestLeaseOldConfigKeepsDefault(t *testing.T) {
	// a saved configuration from before the lease existed has no leaseS: it keeps the default
	st := memStore{}
	if err := st.Save(keyConfig, map[string]any{"enabled": true, "settings": map[string]any{"marginS": 15}}); err != nil {
		t.Fatal(err)
	}
	m := NewManager(util.NewLogger("test"), st, nil, func(string, any) {}, Plant{})
	if got := m.cfg.Settings.LeaseS; got != 60 {
		t.Fatalf("leaseS from an old config: %v, want the default 60", got)
	}
}

func TestLeaseOldExportKeepsDefault(t *testing.T) {
	m := newTestManager(t)
	exp, err := m.Export()
	if err != nil {
		t.Fatal(err)
	}
	exp.Config.Settings.LeaseS = 0 // an export from before the lease
	if err := m.Import(exp); err != nil {
		t.Fatal(err)
	}
	if got := m.cfg.Settings.LeaseS; got != 60 {
		t.Fatalf("leaseS after importing an old export: %v, want the default 60", got)
	}
}
