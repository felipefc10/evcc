package supercharge

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
	"strings"
)

// The learner measures how fast each loadpoint+vehicle actually obeys. It is
// conservative on both sides: slow quantile for rates, high quantile for
// latency, a margin on top, and silent until it has evidence.

const (
	learnMargin       = 0.7
	learnMoveA        = 0.4
	learnArriveA      = 0.6
	learnGiveUpS      = 90.0
	learnMinStepA     = 3.0
	learnMinSamples   = 4
	maxEntryOffsetA   = 2.0
	maxEntryResidualA = 1.25
	learnMaxSourceOhm = 1.5
	minSourceStepA    = 5.0
	entryDeadbandA    = 0.5
	LearnSchema       = 9
	learnKeep         = 40
	slowestCredible   = 4.0
	latestCredible    = 3.0
	latestCredibleUp  = 6.0
	learnKeySeparator = "|"
)

// Behaviour is what has been learned about one loadpoint+vehicle
type Behaviour struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Charger      string  `json:"charger"`
	Vehicle      string  `json:"vehicle"`
	RampUp       float64 `json:"rampUp"`
	RampDown     float64 `json:"rampDown"`
	LatencyDown  float64 `json:"latencyDown"`
	LatencyUp    float64 `json:"latencyUp"`
	EntryOffsetA float64 `json:"entryOffsetA"`
	Ups          int     `json:"ups"`
	Downs        int     `json:"downs"`
	LatsUp       int     `json:"latsUp"`
	LatsDown     int     `json:"latsDown"`
	Entries      int     `json:"entries"`
}

type pending struct {
	tCmd    float64
	startA  float64
	targetA float64
	tMove   *float64
}

type bag struct {
	Label    string    `json:"label"`
	Ups      []float64 `json:"ups"`
	Downs    []float64 `json:"downs"`
	LatsUp   []float64 `json:"lats_up"`
	LatsDown []float64 `json:"lats_down"`
	Entries  []float64 `json:"entries"`
	pending  *pending
}

func push(s []float64, v float64) []float64 {
	s = append(s, v)
	if len(s) > learnKeep {
		s = s[len(s)-learnKeep:]
	}
	return s
}

func slowQ(values []float64, margin float64) float64 {
	if len(values) < learnMinSamples {
		return 0
	}
	o := slices.Clone(values)
	sort.Float64s(o)
	q := o[int(float64(len(o))*0.2)]
	return math.Max(q*margin, 0.1)
}

func timidQ(values []float64, margin float64) float64 {
	if len(values) < learnMinSamples {
		return 0
	}
	o := slices.Clone(values)
	sort.Float64s(o)
	q := o[int(float64(len(o))*0.2)]
	if q > 0 {
		q *= margin
	}
	return math.Max(math.Min(q, maxEntryOffsetA), -maxEntryOffsetA)
}

func lateQ(values []float64, margin float64) float64 {
	if len(values) < learnMinSamples {
		return 0
	}
	o := slices.Clone(values)
	sort.Float64s(o)
	q := o[min(len(o)-1, int(float64(len(o))*0.8))]
	return q / math.Max(margin, 0.05)
}

// Learner holds learned behaviour per loadpoint+vehicle key
type Learner struct {
	Margin  float64
	bags    map[string]*bag
	ohms    []float64
	retired bool
	Dirty   bool
	held    map[string]float64
	moaned  map[string]bool
	// Warn is called once per implausible learned value
	Warn func(format string, args ...any)
}

// NewLearner creates a learner
func NewLearner() *Learner {
	return &Learner{Margin: learnMargin, bags: map[string]*bag{}, held: map[string]float64{}, moaned: map[string]bool{}}
}

// LearnKey builds the learner key of a loadpoint and vehicle
func LearnKey(lp, vehicle string) string {
	if vehicle == "" {
		vehicle = "?"
	}
	return lp + learnKeySeparator + vehicle
}

func (l *Learner) bag(key, label string) *bag {
	b, ok := l.bags[key]
	if !ok {
		b = &bag{}
		l.bags[key] = b
	}
	if label != "" {
		b.Label = label
	}
	return b
}

// Command records a setpoint issued as the start of a labelled step response
func (l *Learner) Command(key string, t, measuredA, targetA float64, label string) {
	b := l.bag(key, label)
	if math.Abs(targetA-measuredA) < learnMinStepA {
		b.pending = nil
		return
	}
	b.pending = &pending{tCmd: t, startA: measuredA, targetA: targetA}
}

// Observe feeds a measurement to a pending step response
func (l *Learner) Observe(key string, t, measuredA float64) {
	b, ok := l.bags[key]
	if !ok || b.pending == nil {
		return
	}
	p := b.pending
	up := p.targetA > p.startA
	moved := p.startA - measuredA
	if up {
		moved = measuredA - p.startA
	}
	if t-p.tCmd > learnGiveUpS {
		b.pending = nil
		return
	}
	if p.tMove == nil {
		if moved >= learnMoveA {
			tt := t
			p.tMove = &tt
			if up {
				b.LatsUp = push(b.LatsUp, math.Max(t-p.tCmd, 0))
			} else {
				b.LatsDown = push(b.LatsDown, math.Max(t-p.tCmd, 0))
			}
			l.Dirty = true
		}
		return
	}
	if math.Abs(measuredA-p.targetA) <= learnArriveA {
		dt := t - *p.tMove
		travel := math.Abs(measuredA - p.startA)
		if dt > 0.2 && travel >= learnMoveA {
			if up {
				b.Ups = push(b.Ups, travel/dt)
			} else {
				b.Downs = push(b.Downs, travel/dt)
			}
			l.Dirty = true
		}
		b.pending = nil
	}
}

// Supply learns the supply resistance from two settled states
func (l *Learner) Supply(vLo, sLo, vHi, sHi float64) {
	if math.Min(vLo, vHi) < 100 || sHi <= sLo {
		return
	}
	iLo, iHi := sLo*1000.0/vLo, sHi*1000.0/vHi
	if iHi-iLo < minSourceStepA {
		return
	}
	ohm := (vLo - vHi) / (iHi - iLo)
	if !(0.0 <= ohm && ohm <= learnMaxSourceOhm*2) {
		return
	}
	was := l.SourceOhm()
	l.ohms = push(l.ohms, ohm)
	l.Dirty = true
	if !l.retired && was <= 0.0 && l.SourceOhm() > 0.0 {
		for _, b := range l.bags {
			b.Entries = nil
		}
		l.retired = true
	}
}

// SourceOhm is the learned supply resistance, 0 until known
func (l *Learner) SourceOhm() float64 {
	if len(l.ohms) < learnMinSamples {
		return 0
	}
	o := slices.Clone(l.ohms)
	sort.Float64s(o)
	q := o[int(float64(len(o))*0.2)]
	return math.Max(math.Min(q, learnMaxSourceOhm), 0.0)
}

// Entry records one burst entry shortfall observation
func (l *Learner) Entry(key string, errA float64, label string) {
	b := l.bag(key, label)
	b.Entries = push(b.Entries, errA)
	l.Dirty = true
}

func splitKey(key string) (string, string) {
	i := strings.LastIndex(key, learnKeySeparator)
	if i < 0 {
		return key, ""
	}
	veh := key[i+1:]
	if veh == "?" {
		veh = ""
	}
	return key[:i], veh
}

// Behaviour returns what has been learned about key
func (l *Learner) Behaviour(key string) Behaviour {
	charger, vehicle := splitKey(key)
	b, ok := l.bags[key]
	if !ok {
		return Behaviour{Key: key, Charger: charger, Vehicle: vehicle}
	}
	return Behaviour{
		Key: key, Label: b.Label, Charger: charger, Vehicle: vehicle,
		RampUp:       round(slowQ(b.Ups, l.Margin), 2),
		RampDown:     round(slowQ(b.Downs, l.Margin), 2),
		LatencyDown:  round(lateQ(b.LatsDown, l.Margin), 2),
		LatencyUp:    round(lateQ(b.LatsUp, l.Margin), 2),
		EntryOffsetA: round(timidQ(b.Entries, l.Margin), 2),
		Ups:          len(b.Ups), Downs: len(b.Downs),
		LatsUp: len(b.LatsUp), LatsDown: len(b.LatsDown), Entries: len(b.Entries),
	}
}

func (l *Learner) notCredible(key, what string, got, bound, fallback float64) {
	id := key + "/" + what
	if l.moaned[id] {
		return
	}
	l.moaned[id] = true
	if l.Warn != nil {
		l.Warn("learned %s for %s is %.4g, past the credible bound of %.4g against a configured %.4g, using the configured value", what, key, got, bound, fallback)
	}
}

// RampDown is the shed rate, never faster than fallback
func (l *Learner) RampDown(key string, fallback float64) float64 {
	b := l.Behaviour(key)
	if b.RampDown <= 0 {
		return fallback
	}
	floor := fallback / slowestCredible
	if b.RampDown < floor {
		l.notCredible(key, "rampDown", b.RampDown, floor, fallback)
		return fallback
	}
	return math.Min(b.RampDown, fallback)
}

// Latency is the shed dead time, never shorter than fallback
func (l *Learner) Latency(key string, fallback float64) float64 {
	return l.latency(key, fallback, l.Behaviour(key).LatencyDown, "latencyDown", latestCredible)
}

// LatencyUp is the raise dead time, never shorter than fallback
func (l *Learner) LatencyUp(key string, fallback float64) float64 {
	return l.latency(key, fallback, l.Behaviour(key).LatencyUp, "latencyUp", latestCredibleUp)
}

func (l *Learner) latency(key string, fallback, got float64, what string, credible float64) float64 {
	if got <= 0 {
		return fallback
	}
	ceiling := fallback * credible
	if got > ceiling {
		l.notCredible(key, what, got, ceiling, fallback)
		return fallback
	}
	return math.Max(got, fallback)
}

func (l *Learner) entryBound() float64 {
	if l.SourceOhm() > 0.0 {
		return maxEntryResidualA
	}
	return maxEntryOffsetA
}

// EntryOffset is the burst entry correction, held against small downward moves
func (l *Learner) EntryOffset(key string) float64 {
	bound := l.entryBound()
	want := math.Max(math.Min(l.Behaviour(key).EntryOffsetA, bound), -bound)
	held, ok := l.held[key]
	if !ok || want < 0.0 || want > held || held-want >= entryDeadbandA {
		l.held[key] = want
		return want
	}
	return held
}

// ForgetWarnings re-arms the implausible value warnings
func (l *Learner) ForgetWarnings() {
	l.moaned = map[string]bool{}
}

// All returns every learned behaviour sorted by key
func (l *Learner) All() []Behaviour {
	keys := make([]string, 0, len(l.bags))
	for k := range l.bags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	res := make([]Behaviour, 0, len(keys))
	for _, k := range keys {
		res = append(res, l.Behaviour(k))
	}
	return res
}

type learnFile struct {
	Schema         int             `json:"schema"`
	Ohms           []float64       `json:"ohms"`
	EntriesRetired bool            `json:"entries_retired"`
	Keys           map[string]*bag `json:"keys"`
}

// MarshalJSON persists the raw observations
func (l *Learner) MarshalJSON() ([]byte, error) {
	return json.Marshal(learnFile{Schema: LearnSchema, Ohms: l.ohms, EntriesRetired: l.retired, Keys: l.bags})
}

// Load restores observations and returns the number of keys loaded
func (l *Learner) Load(data []byte) (int, error) {
	var f learnFile
	if err := json.Unmarshal(data, &f); err != nil {
		return 0, err
	}
	if f.Schema != LearnSchema {
		return 0, nil
	}
	dropEntries := false
	l.ohms = nil
	for _, v := range f.Ohms {
		l.ohms = push(l.ohms, v)
	}
	l.retired = f.EntriesRetired
	if !l.retired && l.SourceOhm() > 0.0 {
		dropEntries = true
		l.retired = true
	}
	l.bags = map[string]*bag{}
	for k, b := range f.Keys {
		if b == nil {
			continue
		}
		nb := &bag{Label: b.Label}
		for _, v := range b.Ups {
			nb.Ups = push(nb.Ups, v)
		}
		for _, v := range b.Downs {
			nb.Downs = push(nb.Downs, v)
		}
		for _, v := range b.LatsUp {
			nb.LatsUp = push(nb.LatsUp, v)
		}
		for _, v := range b.LatsDown {
			nb.LatsDown = push(nb.LatsDown, v)
		}
		if !dropEntries {
			for _, v := range b.Entries {
				nb.Entries = push(nb.Entries, v)
			}
		}
		l.bags[k] = nb
	}
	l.held = map[string]float64{}
	return len(l.bags), nil
}
