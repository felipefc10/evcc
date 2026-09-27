package supercharge

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/evcc-io/evcc/util"
)

type memStore map[string][]byte

func (s memStore) Load(key string, v any) error {
	b, ok := s[key]
	if !ok {
		return errors.New("not found")
	}
	return json.Unmarshal(b, v)
}

func (s memStore) Save(key string, v any) error {
	b, err := json.Marshal(v)
	s[key] = b
	return err
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(util.NewLogger("test"), memStore{}, nil, func(string, any) {}, Plant{})
}

func TestClampFailsafeWhileOff(t *testing.T) {
	m := newTestManager(t)

	if got := m.Clamp("lp-1", 32, 6, true, 0); got != 6 {
		t.Fatalf("default limit while off: got %v, want 6", got)
	}
	// a car with a higher minimum is never asked below it
	if got := m.Clamp("lp-1", 32, 8, true, 0); got != 8 {
		t.Fatalf("minimum wins: got %v, want 8", got)
	}
	if got := m.Clamp("lp-1", 0, 6, false, 0); got != 0 {
		t.Fatalf("off stays off: got %v", got)
	}

	if _, err := m.UpdateConfig([]byte(`{"failsafeA": 16}`)); err != nil {
		t.Fatal(err)
	}
	if got := m.Clamp("lp-1", 32, 6, true, 0); got != 16 {
		t.Fatalf("configured limit: got %v, want 16", got)
	}

	if _, err := m.UpdateConfig([]byte(`{"failsafeA": 0}`)); err != nil {
		t.Fatal(err)
	}
	if got := m.Clamp("lp-1", 32, 6, true, 0); got != 32 {
		t.Fatalf("0 means no limit: got %v, want 32", got)
	}

	if _, err := m.UpdateConfig([]byte(`{"failsafeA": 99}`)); err != nil {
		t.Fatal(err)
	}
	if m.cfg.FailsafeA != 32 {
		t.Fatalf("limit is clamped: got %v", m.cfg.FailsafeA)
	}
}

func TestClampUnarmedHoldsWhatItHas(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.UpdateConfig([]byte(`{"enabled": true}`)); err != nil {
		t.Fatal(err)
	}
	// not armed yet: nothing more than the minimum, and a stopped car stays stopped
	if got := m.Clamp("lp-1", 32, 6, true, 0); got != 6 {
		t.Fatalf("unarmed start: got %v, want 6", got)
	}
	if got := m.Clamp("lp-1", 32, 6, true, 10); got != 10 {
		t.Fatalf("unarmed keeps what it has: got %v, want 10", got)
	}
}
