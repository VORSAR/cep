package partition_test

// The step contract (design §1), keys (§3 Offer, §0.2, §7 O5), the derivability invariant (§3 CheckTimers,
// ADR-0006 §2(c)), order (§3 MUST: Ordinal order; exit 1) and the state registry (ADR-0006 §2(b)).

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// The step contract (§1, §0.5): E1 of a at 0:00 schedules a Completion at 1:00 (ref 7). E2 of a at exactly 1:00 is
// offered after the advance to 1:00 is applied and before Advance: Buffer sees clock 1:00 and no seal (nothing fired
// yet), so E2 is buffered; the walk at 1:00 releases events before inclusive timers, so E2 comes before the
// Completion. A third event of a at 1:00 offered after that walk IS construct-late: the Completion sealed 1:00 for a
// (owner D7) — the seal the contract keeps E2 clear of.
func TestStepContract_EventAtItsTimersInstantIsReleasedFirst(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		if e.Seq != 1 {
			return nil
		}
		return []clock.Timer{{Due: e.Ts + minute, Kind: clock.Completion, Ref: 7}}
	}}
	w := newWorld(t, 10*minute, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	if late := w.arrive(minute, str("a"), ev(0, minute, 2)); late {
		t.Fatal("E2 at its key's pending timer's instant was construct-late: the walk ran before Offer")
	}
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), Ev("a", 0, minute), Tm("a", 0, clock.Timer{Due: minute, Kind: clock.Completion, Ref: 7}))
	if late := w.arrive(minute, str("a"), ev(0, minute, 3)); !late {
		t.Error("E3 at the sealed instant 1:00: want construct-late (owner D7)")
	}
	if s := w.r.Stats(); s.ConstructLate != 1 {
		t.Errorf("Stats.ConstructLate %d, want 1", s.ConstructLate)
	}
}

// Two inputs: the rule's clock is the min of its inputs (SEMANTICS §2.1). Stream 0 at 1:00 alone does not move the
// clock while stream 1 is at −∞; the event at 1:00 waits buffered; stream 1's advance to 1:00 releases it.
func TestStepContract_ApplyDoesNotWalk(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 10*minute, b, 0, 1)
	if w.r.Apply(clock.WMAdvance{StreamID: 0, Contribution: minute, Moved: true}) {
		t.Error("Apply of one input of two: want not advanced (min(1:00, −∞) = −∞)")
	}
	if c := w.r.Clock(); c != negInf {
		t.Fatalf("clock %d after one of two inputs moved, want −∞", c)
	}
	if late := w.r.Offer(str("a"), ev(0, minute, 1)); late {
		t.Fatal("construct-late at clock −∞")
	}
	if !w.r.Apply(clock.WMAdvance{StreamID: 1, Contribution: minute, Moved: true}) {
		t.Error("Apply of the second input to 1:00: want advanced (min 1:00 > −∞)")
	}
	if len(b.log) != 0 {
		t.Fatalf("Apply walked: %+v (step 1 MUST NOT walk)", b.log)
	}
	w.advance()
	wantLog(t, b, N("a", 0), Ev("a", 0, minute))
	if c := w.r.Clock(); c != minute {
		t.Errorf("clock %d, want %d", c, minute)
	}
}

// -0.0 and +0.0 are one key (design §3: looked up by value, IEEE ==, as table's key). Instance.Key is the first
// event's key value, so the sign of -0 is kept.
func TestKeys_NegativeAndPositiveZeroAreOneKey(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorldKeyed(t, event.TFloat, 5*minute, b, 0)
	negZero := math.Copysign(0, -1)
	w.arrive(0, event.Value{Kind: event.TFloat, F: negZero}, ev(0, 0, 1))
	w.arrive(second, event.Value{Kind: event.TFloat, F: 0}, ev(0, second, 2))
	if w.r.Len() != 1 {
		t.Fatalf("Len %d, want 1: -0.0 and +0.0 are one key", w.r.Len())
	}
	wantLog(t, b, N("-0", 0), Ev("-0", 0, 0), Ev("-0", 0, second))
	if in := instances(w.r)[0]; !math.Signbit(in.Key.F) {
		t.Errorf("Instance.Key %v: want the first event's key (-0)", in.Key.F)
	}
}

// A null key and a key of another type panic (design §3: the router checks both first). One KeyType per rule, so an
// int 1 and a float 1.0 cannot both arrive: the int panics on a float rule.
func TestKeys_NullAndWrongTypePanic(t *testing.T) {
	t.Parallel()
	w := newWorldKeyed(t, event.TFloat, 5*minute, &body{}, 0)
	w.arrive(0, event.Value{Kind: event.TFloat, F: 1}, ev(0, 0, 1))
	mustPanic(t, "Offer of a null key", func() { w.r.Offer(event.Value{Kind: event.TFloat, Null: true}, ev(0, 0, 2)) })
	mustPanic(t, "Offer of an int key on a float rule", func() { w.r.Offer(event.Value{Kind: event.TInt, I: 1}, ev(0, 0, 3)) })
	mustPanic(t, "Offer of a string key on a float rule", func() { w.r.Offer(str("1"), ev(0, 0, 4)) })
	if w.r.Len() != 1 {
		t.Errorf("Len %d after the refused offers, want 1", w.r.Len())
	}
}

// NewRule refuses an empty Inputs, KeyType invalid or object (§0.2, §7 O5), a negative TTL and a nil Body.
func TestNewRuleRefuses(t *testing.T) {
	t.Parallel()
	ok := partition.Config{Inputs: []int{0}, KeyType: event.TString, TTL: minute, Body: &body{t: t}}
	if r, err := partition.NewRule(ok); err != nil || r == nil {
		t.Fatalf("NewRule of a valid config: %v", err)
	}
	for _, c := range []struct {
		name string
		edit func(*partition.Config)
	}{
		{"nil inputs", func(c *partition.Config) { c.Inputs = nil }},
		{"empty inputs", func(c *partition.Config) { c.Inputs = []int{} }},
		{"KeyType invalid", func(c *partition.Config) { c.KeyType = event.TInvalid }},
		{"KeyType object", func(c *partition.Config) { c.KeyType = event.TObject }},
		{"negative TTL", func(c *partition.Config) { c.TTL = -1 }},
		{"nil Body", func(c *partition.Config) { c.Body = nil }},
	} {
		cfg := ok
		c.edit(&cfg)
		if _, err := partition.NewRule(cfg); err == nil {
			t.Errorf("NewRule(%s): want an error", c.name)
		}
	}
	for _, kt := range []event.Type{event.TString, event.TInt, event.TFloat, event.TBool, event.TTimestamp, event.TDuration} {
		cfg := ok
		cfg.KeyType = kt
		if _, err := partition.NewRule(cfg); err != nil {
			t.Errorf("NewRule(KeyType %v): %v", kt, err)
		}
	}
}

// ordinalRe matches the ordinal as a whole number in CheckTimers' message (design §3: "naming the instance's
// ordinal").
func ordinalRe(n int) *regexp.Regexp { return regexp.MustCompile(`\b` + strconv.Itoa(n) + `\b`) }

// The invariant, planted faults (design §3 CheckTimers). Keys a, b, c are honest; key d (ordinal 3) breaks it. Each
// fault is found and the message names ordinal 3.
func TestCheckTimers_PlantedFaults(t *testing.T) {
	t.Parallel()
	completion := func(in *partition.Instance, e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + minute, Kind: clock.Completion, Ref: 1}}
	}
	for _, c := range []struct {
		name     string
		schedule func(in *partition.Instance, e event.Event) []clock.Timer
		plant    func(st *state)
	}{
		// scheduled a Completion its state does not report
		{"hidden timer", completion, func(st *state) { st.hide = true }},
		// reports a Completion it never scheduled
		{"phantom timer", nil, func(st *state) { st.extra = []clock.Timer{{Due: 9 * minute, Kind: clock.DelayDue, Ref: 2}} }},
		// a body scheduling TTLPurge (design §3 MUST NOT), at the same due as the real one and reporting it
		{"body TTLPurge, reported", func(in *partition.Instance, e event.Event) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 5*minute, Kind: clock.TTLPurge}}
		}, nil},
		// a body re-scheduling TTLPurge at another due, unreported: the TTL is no longer LastActivity + TTL
		{"body TTLPurge, moved", func(in *partition.Instance, e event.Event) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 9*minute, Kind: clock.TTLPurge}}
		}, func(st *state) { st.hide = true }},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := &body{}
			b.schedule = func(in *partition.Instance, e event.Event) []clock.Timer {
				if keyName(in.Key) != "d" || c.schedule == nil {
					return nil
				}
				return c.schedule(in, e)
			}
			b.plant = func(key string, st *state) {
				if key == "d" && c.plant != nil {
					c.plant(st)
				}
			}
			w := newWorld(t, 5*minute, b, 0)
			for i, k := range []string{"a", "b", "c"} {
				w.arrive(int64(i)*second, str(k), ev(0, int64(i)*second, uint64(i+1)))
			}
			w.noCheck = true
			w.arrive(3*second, str("d"), ev(0, 3*second, 4))
			err := w.r.CheckTimers()
			if err == nil {
				t.Fatal("CheckTimers: nil, want an error naming instance 3")
			}
			if !ordinalRe(3).MatchString(err.Error()) {
				t.Errorf("CheckTimers: %q does not name ordinal 3", err)
			}
		})
	}
}

// Instances and Walk go in Ordinal order whatever the keys' values (§3 MUST; exit 1: a Go map's order never
// reaches any output). 300 int keys v_i = (i·919 mod 1000) − 500, distinct since gcd(919, 1000) = 1, so neither
// value order nor a map's order is ordinal order.
func TestInstancesInOrdinalOrder(t *testing.T) {
	t.Parallel()
	w := newWorldKeyed(t, event.TInt, 10*minute, &body{}, 0)
	val := func(i int64) int64 { return i*919%1000 - 500 }
	for i := range int64(300) {
		w.arrive(i, event.Value{Kind: event.TInt, I: val(i)}, ev(0, i, uint64(i+1)))
	}
	for run := range 3 {
		ins := instances(w.r)
		if len(ins) != 300 {
			t.Fatalf("%d instances, want 300", len(ins))
		}
		for i, in := range ins {
			if in.Clock.Ordinal() != uint64(i) || in.Key.I != val(int64(i)) {
				t.Fatalf("run %d: position %d is ordinal %d key %d; want ordinal %d key %d", run, i,
					in.Clock.Ordinal(), in.Key.I, i, val(int64(i)))
			}
		}
		var keys []string
		w.r.Walk(func(component, key string, _ any) {
			if component == "instance" {
				keys = append(keys, key)
			}
		})
		for i, k := range keys {
			if k != strconv.Itoa(i) {
				t.Fatalf("run %d: Walk's instance %d has key %q, want %q (ordinal in decimal)", run, i, k, strconv.Itoa(i))
			}
		}
		if len(keys) != 300 {
			t.Fatalf("Walk yielded %d instances, want 300", len(keys))
		}
	}
}

// instanceState accepts InstanceState by value or by pointer (the design does not say which; gap A3).
func instanceState(t *testing.T, s any) partition.InstanceState {
	t.Helper()
	switch v := s.(type) {
	case partition.InstanceState:
		return v
	case *partition.InstanceState:
		return *v
	}
	t.Fatalf("Walk's instance state is %T, want partition.InstanceState", s)
	return partition.InstanceState{}
}

type walked struct {
	Component, Key string
	State          any
}

// Walk yields, per instance in Ordinal order, ("instance", ordinal, InstanceState) and then the OpState's parts; an
// unborn instance has no parts. TTL 5 min.
//   - a at 0:00 (ordinal 0): born; schedules Completion 1:00 ref 1; TTL 5:00.
//   - b at 0:10 offered at clock 0 (ordinal 1): buffered, unborn: LastActivity −∞, no timers.
//   - a at 0:20 offered at clock 0: buffered.
//
// a's InstanceState: Events [(0, 0:20, 3)]; Timers in walk order [Completion 1:00, TTL 5:00]; its part ("op", "a").
func TestWalkYieldsInstanceStateThenParts(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + minute, Kind: clock.Completion, Ref: 1}}
	}}
	w := newWorld(t, 5*minute, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	w.arrive(0, str("b"), ev(0, 10*second, 2))
	w.arrive(0, str("a"), ev(0, 20*second, 3))
	var got []walked
	w.r.Walk(func(component, key string, s any) { got = append(got, walked{component, key, s}) })
	if len(got) != 3 {
		t.Fatalf("Walk yielded %d items, want 3: %+v", len(got), got)
	}
	if got[0].Component != "instance" || got[0].Key != "0" || got[1].Component != "op" || got[1].Key != "a" ||
		got[2].Component != "instance" || got[2].Key != "1" {
		t.Fatalf("Walk order %+v, want instance 0, op a, instance 1", got)
	}
	a := instanceState(t, got[0].State)
	if keyName(a.Key) != "a" || a.LastActivity != 0 {
		t.Errorf("a: Key %v LastActivity %d, want a, 0", a.Key, a.LastActivity)
	}
	if len(a.Events) != 1 || a.Events[0].Ts != 20*second || a.Events[0].Seq != 3 {
		t.Errorf("a: Events %+v, want the one at 0:20 seq 3", a.Events)
	}
	if want := []clock.Timer{{Due: minute, Kind: clock.Completion, Ref: 1}, ttl(5 * minute)}; !slices.Equal(a.Timers, want) {
		t.Errorf("a: Timers %+v, want %+v", a.Timers, want)
	}
	bs := instanceState(t, got[2].State)
	if keyName(bs.Key) != "b" || bs.LastActivity != negInf || len(bs.Timers) != 0 {
		t.Errorf("b: %+v, want key b, LastActivity −∞, no timers", bs)
	}
	if len(bs.Events) != 1 || bs.Events[0].Ts != 10*second || bs.Events[0].Seq != 2 {
		t.Errorf("b: Events %+v, want the one at 0:10 seq 2", bs.Events)
	}
}

type holder struct{ parts []string }

func (h holder) Walk(fn func(component, key string, state any)) {
	for _, p := range h.parts {
		fn("c", p, nil)
	}
}

// The registry walks holders in registration order, not name order; a duplicate name panics. A Rule and the stream
// registry are holders (ADR-0006 §2(b)).
func TestRegistryOrderAndDuplicate(t *testing.T) {
	t.Parallel()
	var _ partition.Holder = (*partition.Rule)(nil)
	var _ partition.Holder = (*clock.StreamRegistry)(nil)
	g := &partition.Registry{}
	g.Register("zeta", holder{[]string{"z1", "z2"}})
	g.Register("alpha", holder{[]string{"a1"}})
	var got []string
	g.Walk(func(h, c, k string, _ any) { got = append(got, fmt.Sprintf("%s/%s/%s", h, c, k)) })
	if want := []string{"zeta/c/z1", "zeta/c/z2", "alpha/c/a1"}; !slices.Equal(got, want) {
		t.Errorf("Registry.Walk %v, want %v", got, want)
	}
	mustPanic(t, "a duplicate Register", func() { g.Register("zeta", holder{}) })
}
