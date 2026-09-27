package supercharge

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

// CadenceDefault is the default cadence table: constant 10 A.s
const CadenceDefault = "1:10, 2:5, 3:3.3, 5:2, 10:1, 20:0.5"

// Settings holds everything tunable, read fresh each step
type Settings struct {
	// the burst, an emergency facility, off unless a loadpoint is ticked
	BurstKVA     float64 `json:"burstKva"`
	BumpKVA      float64 `json:"bumpKva"`
	MarginS      float64 `json:"marginS"`
	ResetS       float64 `json:"resetS"`
	MaxCloseness float64 `json:"maxCloseness"`
	ExitLeadFrac float64 `json:"exitLeadFrac"`

	// the baseline
	BaseMarginKVA float64 `json:"baseMarginKva"`

	// per-loadpoint fallbacks, used only when the loadpoint does not say
	AmpMin int `json:"ampMin"`
	AmpMax int `json:"ampMax"`

	// cadence tables, amps:seconds
	RaiseTable  string `json:"raiseTable"`
	ReduceTable string `json:"reduceTable"`

	// contactor thrift
	FloorDwellS   float64 `json:"floorDwellS"`
	RestartDwellS float64 `json:"restartDwellS"`

	// missing data
	BlindHoldPolls int    `json:"blindHoldPolls"`
	BlindPolls     int    `json:"blindPolls"`
	BlindAction    string `json:"blindAction"` // stop | hold
	BlindHoldA     int    `json:"blindHoldA"`

	// the baseline backstop
	BaseAbortCloseness float64 `json:"baseAbortCloseness"`
	BaseStopCloseness  float64 `json:"baseStopCloseness"`
	BaseTiAbortS       float64 `json:"baseTiAbortS"`
	BaseTiStopS        float64 `json:"baseTiStopS"`

	// the delivery trim bound, 0 disables
	TrimMaxA float64 `json:"trimMaxA"`

	// bursting stops above this charger temperature
	MaxTempC float64 `json:"maxTempC"`

	// Supercharge maps a loadpoint key to the epoch second it stands down at, 0 for indefinitely.
	// A key being present means that loadpoint is ticked for supercharging.
	Supercharge map[string]int64 `json:"supercharge"`
}

// DefaultSettings returns the control law defaults
func DefaultSettings() Settings {
	return Settings{
		BurstKVA:           6.50,
		BumpKVA:            2.0,
		MarginS:            30.0,
		ResetS:             10.0,
		MaxCloseness:       0.70,
		ExitLeadFrac:       0.5,
		BaseMarginKVA:      0.10,
		AmpMin:             6,
		AmpMax:             32,
		RaiseTable:         CadenceDefault,
		ReduceTable:        CadenceDefault,
		FloorDwellS:        6.0,
		RestartDwellS:      30.0,
		BlindHoldPolls:     2,
		BlindPolls:         8,
		BlindAction:        "stop",
		BlindHoldA:         10,
		BaseAbortCloseness: 0.40,
		BaseStopCloseness:  0.75,
		BaseTiAbortS:       30.0,
		BaseTiStopS:        90.0,
		TrimMaxA:           2.0,
		MaxTempC:           55.0,
		Supercharge:        map[string]int64{},
	}
}

// BaseKVA is where BASE sits: just under the never-trip line
func (s *Settings) BaseKVA(c Curve) float64 {
	return c.ThresholdKVA() - s.BaseMarginKVA
}

// MaySupercharge reports whether this loadpoint is ticked for supercharging
func (s *Settings) MaySupercharge(key string) bool {
	_, ok := s.Supercharge[key]
	return ok
}

// AnySupercharge reports whether any loadpoint is ticked
func (s *Settings) AnySupercharge() bool {
	return len(s.Supercharge) > 0
}

// Clone returns a deep copy
func (s Settings) Clone() Settings {
	res := s
	res.Supercharge = make(map[string]int64, len(s.Supercharge))
	maps.Copy(res.Supercharge, s.Supercharge)
	return res
}

// Limit is the allowed range and step of a numeric setting
type Limit struct {
	Min, Max, Step float64
}

// Limits are the bounds every numeric setting is clamped into
var Limits = map[string]Limit{
	"burstKva":           {4.2, 9.0, 0.05},
	"bumpKva":            {1.5, 4.0, 0.1},
	"marginS":            {10, 120, 1},
	"resetS":             {0.5, 60, 0.5},
	"baseMarginKva":      {0, 0.5, 0.01},
	"maxCloseness":       {0.3, 0.95, 0.05},
	"ampMin":             {2, 16, 1},
	"ampMax":             {10, 32, 1},
	"blindHoldPolls":     {1, 10, 1},
	"blindPolls":         {2, 30, 1},
	"blindHoldA":         {0, 32, 1},
	"maxTempC":           {30, 90, 1},
	"baseAbortCloseness": {0.1, 0.8, 0.05},
	"baseStopCloseness":  {0.2, 0.95, 0.05},
	"baseTiAbortS":       {5, 300, 5},
	"baseTiStopS":        {10, 600, 5},
	"trimMaxA":           {0, 4, 0.25},
	"exitLeadFrac":       {0, 1, 0.05},
	"floorDwellS":        {0, 60, 1},
	"restartDwellS":      {0, 300, 5},
}

// BlindActions are the allowed blind actions
var BlindActions = []string{"stop", "hold"}

func clampLimit(name string, v float64) float64 {
	l, ok := Limits[name]
	if !ok {
		return v
	}
	return math.Min(math.Max(v, l.Min), l.Max)
}

// ParseCadence parses "1:20, 5:4" into rows sorted by amps with non-increasing seconds.
// Junk rows are dropped.
func ParseCadence(text string) [][2]float64 {
	var rows [][2]float64
	for part := range strings.SplitSeq(strings.ReplaceAll(text, ";", ","), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		amps, secs, _ := strings.Cut(part, ":")
		a, err1 := parsePyFloat(amps)
		s, err2 := parsePyFloat(secs)
		if err1 != nil || err2 != nil {
			continue
		}
		if a <= 0 || s < 0 {
			continue
		}
		rows = append(rows, [2]float64{a, math.Min(s, cadenceMaxS)})
	}
	slices.SortStableFunc(rows, func(x, y [2]float64) int {
		if x[0] != y[0] {
			if x[0] < y[0] {
				return -1
			}
			return 1
		}
		switch {
		case x[1] < y[1]:
			return -1
		case x[1] > y[1]:
			return 1
		}
		return 0
	})
	out := make([][2]float64, 0, len(rows))
	for _, r := range rows {
		if len(out) > 0 {
			r[1] = math.Min(r[1], out[len(out)-1][1])
		}
		out = append(out, r)
	}
	return out
}

// parsePyFloat mimics python float() on trimmed text
func parsePyFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// CadenceWait returns the minimum seconds since the correction appeared for a deltaA correction
func CadenceWait(rows [][2]float64, deltaA float64) float64 {
	if len(rows) == 0 {
		return 0
	}
	wait := rows[0][1]
	for _, r := range rows {
		if deltaA >= r[0] {
			wait = r[1]
		}
	}
	return wait
}

func formatG(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// SanitiseTable normalises a cadence table, keeping current if nothing usable remains
func SanitiseTable(text, current string) string {
	rows := ParseCadence(text)
	if len(rows) == 0 {
		return current
	}
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, formatG(r[0])+":"+formatG(r[1]))
	}
	return strings.Join(parts, ", ")
}

// Sanitize clamps every field and enforces the cross-field invariants
func (s *Settings) Sanitize() {
	c := func(name string, v *float64) {
		if math.IsNaN(*v) || math.IsInf(*v, 0) {
			*v = 0
		}
		*v = clampLimit(name, *v)
	}
	ci := func(name string, v *int) {
		*v = int(clampLimit(name, float64(*v)))
	}
	c("burstKva", &s.BurstKVA)
	c("bumpKva", &s.BumpKVA)
	c("marginS", &s.MarginS)
	c("resetS", &s.ResetS)
	c("baseMarginKva", &s.BaseMarginKVA)
	c("maxCloseness", &s.MaxCloseness)
	ci("ampMin", &s.AmpMin)
	ci("ampMax", &s.AmpMax)
	ci("blindHoldPolls", &s.BlindHoldPolls)
	ci("blindPolls", &s.BlindPolls)
	ci("blindHoldA", &s.BlindHoldA)
	c("maxTempC", &s.MaxTempC)
	c("baseAbortCloseness", &s.BaseAbortCloseness)
	c("baseStopCloseness", &s.BaseStopCloseness)
	c("baseTiAbortS", &s.BaseTiAbortS)
	c("baseTiStopS", &s.BaseTiStopS)
	c("trimMaxA", &s.TrimMaxA)
	c("exitLeadFrac", &s.ExitLeadFrac)
	c("floorDwellS", &s.FloorDwellS)
	c("restartDwellS", &s.RestartDwellS)

	s.RaiseTable = SanitiseTable(s.RaiseTable, CadenceDefault)
	s.ReduceTable = SanitiseTable(s.ReduceTable, CadenceDefault)

	if !slices.Contains(BlindActions, s.BlindAction) {
		s.BlindAction = "stop"
	}
	if s.AmpMin > s.AmpMax {
		s.AmpMin = s.AmpMax
	}
	if s.BaseStopCloseness <= s.BaseAbortCloseness {
		s.BaseStopCloseness = math.Min(0.95, s.BaseAbortCloseness+0.05)
	}
	if s.BaseTiStopS <= s.BaseTiAbortS {
		s.BaseTiStopS = s.BaseTiAbortS + 30.0
	}
	if s.Supercharge == nil {
		s.Supercharge = map[string]int64{}
	}
	for k, v := range s.Supercharge {
		if k == "" {
			delete(s.Supercharge, k)
		} else if v < 0 {
			s.Supercharge[k] = 0
		}
	}
}

// Validate returns an error describing the first unusable value, used by the API
func (s *Settings) Validate() error {
	if s.BlindAction != "" && !slices.Contains(BlindActions, s.BlindAction) {
		return fmt.Errorf("invalid blind action: %s", s.BlindAction)
	}
	return nil
}
