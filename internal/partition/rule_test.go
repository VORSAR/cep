package partition_test

// The step contract and batch invariance (design v2 §1, owner D1), Apply (QA A7), keys (§3 Offer; QA A2; D7), the
// derivability invariant (§3 CheckTimers, ADR-0006 §2(c); QA A6), the tables a timer reads (D5, §2), order (§3:
// Ordinal order; exit 1) and the state registry (§3 Walk; ADR-0006 §2(b); QA A3).

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// The seal's replacement (design v2 §4.1). E1 of a at 0:00 (released at clock 0:00.001) schedules a Completion at
// 1:00, ref 7. The clock then reaches 1:00: 1:00 < 1:00 is false, so the Completion does not fire. E2 of a at
// exactly 1:00 arrives: admitted (1:00 < 1:00 is false). The advance to 1:00.001 finalizes 1:00 — events first, so
// E2 is released before the Completion fires. An event of a at 1:00 arriving after that (clock 1:00.001) is
// construct-late: 1:00 < 1:00.001.
func TestStepContract_EventAtPendingTimersInstantIsReleasedFirst(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		if e.Seq != 1 {
			return nil
		}
		return []clock.Timer{{Due: e.Ts + minute, Kind: clock.Completion, Ref: 7}}
	}}
	w := newWorld(t, 10*minute, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	w.to(1)
	w.to(minute)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0))
	if late := w.arrive(minute, str("a"), ev(0, minute, 2)); late {
		t.Fatal("E2 at its key's pending timer's instant, the clock at that instant: construct-late, want admitted")
	}
	wantLog(t, b, N("a", 0), Ev("a", 0, 0))
	w.to(minute + 1)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), Ev("a", 0, minute), Tm("a", 0, clock.Timer{Due: minute, Kind: clock.Completion, Ref: 7}))
	if late := w.arrive(minute+1, str("a"), ev(0, minute, 3)); !late {
		t.Error("an event at 1:00 with the clock at 1:00.001: want construct-late")
	}
	if s := w.r.Stats(); s.ConstructLate != 1 {
		t.Errorf("Stats.ConstructLate %d, want 1", s.ConstructLate)
	}
}

// Batch invariance (design v2 §1, D1; ADR-0010 §2): one fixed sequence of messages (two inputs, 40 keys, events
// ahead of and behind the clock, a Completion at e.Ts + 30 s per released event, TTL 2 min so keys purge and
// return) gives the same body callbacks walked after every message, every 7 messages, and once at the end. The
// ordinal is left out of the comparison: a key freed between walks returns on a new ordinal, one never freed keeps
// its own (gap B1); what the body sees — keys, events, fires, purges — is compared.
func TestStepContract_BatchInvariance(t *testing.T) {
	t.Parallel()
	type msg struct {
		c0, c1 int64
		has    bool
		key    string
		e      event.Event
	}
	rng := rand.New(rand.NewPCG(7, 11))
	var msgs []msg
	var base int64
	seq := [2]uint64{}
	for range 400 {
		base += rng.Int64N(15_000)
		m := msg{c0: base, c1: base - rng.Int64N(20_000)}
		if rng.IntN(5) != 0 {
			sid := rng.IntN(2)
			seq[sid]++
			m.has, m.key = true, "k"+strconv.Itoa(rng.IntN(40))
			m.e = ev(sid, base+rng.Int64N(90_000)-25_000, seq[sid])
		}
		msgs = append(msgs, m)
	}
	run := func(every int) ([]rec, partition.Stats) {
		b := &body{t: t, schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 30*second, Kind: clock.Completion, Ref: uint64(e.StreamID)<<32 | e.Seq}}
		}}
		w := newWorld(t, 2*minute, b, 0, 1)
		for i, m := range msgs {
			w.r.Apply(clock.WMAdvance{StreamID: 0, Contribution: m.c0, Moved: true})
			w.r.Apply(clock.WMAdvance{StreamID: 1, Contribution: m.c1, Moved: true})
			if m.has {
				w.r.Offer(str(m.key), m.e)
			}
			if every > 0 && (i+1)%every == 0 {
				w.advance()
			}
		}
		w.advance()
		w.to(posInf)
		for i := range b.log {
			b.log[i].Ord = 0
		}
		return b.log, w.r.Stats()
	}
	perMsg, s1 := run(1)
	for _, cb := range []string{"event", "timers", "purge"} {
		n := 0
		for _, r := range perMsg {
			if r.Cb == cb {
				n++
			}
		}
		if n == 0 {
			t.Fatalf("the sequence has no %q callback: the test proves nothing", cb)
		}
	}
	if s1.ConstructLate == 0 {
		t.Fatal("the sequence has no construct-late event: the test proves nothing")
	}
	for _, every := range []int{7, 0} {
		got, s := run(every)
		if s != s1 {
			t.Errorf("walk every %d: Stats %+v, want %+v (walked per message)", every, s, s1)
		}
		if !slices.EqualFunc(got, perMsg, recEqual) {
			i := 0
			for i < len(got) && i < len(perMsg) && recEqual(got[i], perMsg[i]) {
				i++
			}
			t.Errorf("walk every %d: callbacks differ from walked-per-message at %d of %d/%d", every, i, len(got), len(perMsg))
		}
	}
}

// Apply reports the rule clock moving (QA A7: the clamped min over the inputs) and never walks (step 1). Two
// inputs: E of a at 1:00 is offered at clock −∞. Stream 0 → 2:00 alone: min(2:00, −∞) = −∞, false. Stream 1 →
// 2:00: the min is 2:00, true. Stream 0 → 1:00 again (contributions never fall), false; an unread stream, false.
// Nothing is released until Advance, which releases E (1:00 < 2:00).
func TestApplyReportsTheClockAndDoesNotWalk(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 10*minute, b, 0, 1)
	if late := w.r.Offer(str("a"), ev(0, minute, 1)); late {
		t.Fatal("construct-late at clock −∞")
	}
	if w.r.Apply(clock.WMAdvance{StreamID: 0, Contribution: 2 * minute, Moved: true}) {
		t.Error("Apply of one input of two: want false (min(2:00, −∞) = −∞)")
	}
	if c := w.r.Clock(); c != negInf {
		t.Fatalf("clock %d after one of two inputs moved, want −∞", c)
	}
	if !w.r.Apply(clock.WMAdvance{StreamID: 1, Contribution: 2 * minute, Moved: true}) {
		t.Error("Apply of the second input to 2:00: want true (the min rose to 2:00)")
	}
	if w.r.Apply(clock.WMAdvance{StreamID: 0, Contribution: minute, Moved: true}) {
		t.Error("Apply of a lower contribution: want false")
	}
	if w.r.Apply(clock.WMAdvance{StreamID: 5, Contribution: 9 * minute, Moved: true}) {
		t.Error("Apply of a stream the rule does not read: want false")
	}
	if len(b.log) != 0 {
		t.Fatalf("Apply walked: %+v (step 1 MUST NOT walk)", b.log)
	}
	w.advance()
	wantLog(t, b, N("a", 0), Ev("a", 0, minute))
	if c := w.r.Clock(); c != 2*minute {
		t.Errorf("clock %d, want %d", c, 2*minute)
	}
}

// -0.0 and +0.0 are one key (design §3: looked up by value, IEEE ==, as table's key). Instance.Key is the first
// event's key value, so the sign of -0 is kept. Events at 0:00 (-0) and 0:01 (+0); clock 0:01.001 releases both.
func TestKeys_NegativeAndPositiveZeroAreOneKey(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorldKeyed(t, event.TFloat, 5*minute, b, 0)
	negZero := math.Copysign(0, -1)
	w.arrive(0, event.Value{Kind: event.TFloat, F: negZero}, ev(0, 0, 1))
	w.arrive(second, event.Value{Kind: event.TFloat, F: 0}, ev(0, second, 2))
	w.to(second + 1)
	if w.r.Len() != 1 {
		t.Fatalf("Len %d, want 1: -0.0 and +0.0 are one key", w.r.Len())
	}
	wantLog(t, b, N("-0", 0), Ev("-0", 0, 0), Ev("-0", 0, second))
	if in := instances(w.r)[0]; !math.Signbit(in.Key.F) {
		t.Errorf("Instance.Key %v: want the first event's key (-0)", in.Key.F)
	}
}

// A null key, a key of another type and a NaN float key panic (design v2 §3; QA A2), and leave no instance. One
// KeyType per rule, so an int 1 and a float 1.0 cannot both arrive: the int panics on a float rule.
func TestKeys_NullWrongTypeAndNaNPanic(t *testing.T) {
	t.Parallel()
	w := newWorldKeyed(t, event.TFloat, 5*minute, &body{}, 0)
	w.arrive(0, event.Value{Kind: event.TFloat, F: 1}, ev(0, 0, 1))
	mustPanic(t, "Offer of a null key", func() { w.r.Offer(event.Value{Kind: event.TFloat, Null: true}, ev(0, 0, 2)) })
	mustPanic(t, "Offer of an int key on a float rule", func() { w.r.Offer(event.Value{Kind: event.TInt, I: 1}, ev(0, 0, 3)) })
	mustPanic(t, "Offer of a string key on a float rule", func() { w.r.Offer(str("1"), ev(0, 0, 4)) })
	mustPanic(t, "Offer of a NaN key", func() { w.r.Offer(event.Value{Kind: event.TFloat, F: math.NaN()}, ev(0, 0, 5)) })
	mustPanic(t, "Offer of a second NaN key", func() { w.r.Offer(event.Value{Kind: event.TFloat, F: math.NaN()}, ev(0, 0, 6)) })
	if w.r.Len() != 1 {
		t.Errorf("Len %d after the refused offers, want 1 (a refused key leaves no instance)", w.r.Len())
	}
}

// NewRule refuses an empty Inputs, KeyType invalid or object (D7), a negative TTL and a nil Body; it accepts every
// other KeyType and TTL 0 (design v2 §3, QA A8).
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
		for _, ttlMs := range []int64{0, minute} {
			cfg := ok
			cfg.KeyType, cfg.TTL = kt, ttlMs
			if _, err := partition.NewRule(cfg); err != nil {
				t.Errorf("NewRule(KeyType %v, TTL %d): %v", kt, ttlMs, err)
			}
		}
	}
}

// The invariant, planted faults (design v2 §3 CheckTimers; QA A6). Keys a, b, c are honest; key d (ordinal 3)
// breaks it. d's event at 0:03 is released by the advance to 0:04 (D1); each fault is then found, and the error
// begins "partition: instance 3: ".
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
		// reports a DelayDue it never scheduled
		{"phantom timer", nil, func(st *state) { st.extra = []clock.Timer{{Due: 9 * minute, Kind: clock.DelayDue, Ref: 2}} }},
		// a body scheduling TTLPurge (MUST NOT), at the same due as the real one and reporting it
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
			for i, k := range []string{"a", "b", "c", "d"} {
				w.arrive(int64(i)*second, str(k), ev(0, int64(i)*second, uint64(i+1)))
			}
			w.noCheck = true
			w.to(4 * second)
			if find(w.r, "d") == nil || find(w.r, "d").Op == nil {
				t.Fatal("d not born at clock 0:04")
			}
			err := w.r.CheckTimers()
			if err == nil {
				t.Fatal("CheckTimers: nil, want an error naming instance 3")
			}
			if !strings.HasPrefix(err.Error(), "partition: instance 3: ") {
				t.Errorf("CheckTimers: %q, want the prefix %q", err, "partition: instance 3: ")
			}
		})
	}
}

// D5 (design v2 §2): Body.OnTimers receives Advance's tables — the snapshot of the message whose walk fired the
// timers, not the anchor event's. a's E1 at 0:00 (released at clock 0:00.001, tables "s1") schedules Completions at
// 1:00 (ref 1) and 2:00 (ref 2). The advance to 1:00.001 with "s2" fires ref 1 with "s2"; the advance to +∞ with
// "s3" fires ref 2 with "s3".
func TestOnTimersReceivesAdvanceTables(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + minute, Kind: clock.Completion, Ref: 1}, {Due: e.Ts + 2*minute, Kind: clock.Completion, Ref: 2}}
	}}
	w := newWorld(t, 0, b, 0)
	w.tables = "s1"
	w.arrive(0, str(""), ev(0, 0, 1))
	w.to(1)
	w.tables = "s2"
	w.to(minute + 1)
	w.tables = "s3"
	w.to(posInf)
	t1 := Tm("", 0, clock.Timer{Due: minute, Kind: clock.Completion, Ref: 1})
	t1.Tables = "s2"
	t2 := Tm("", 0, clock.Timer{Due: 2 * minute, Kind: clock.Completion, Ref: 2})
	t2.Tables = "s3"
	wantLog(t, b, N("", 0), Ev("", 0, 0), t1, t2)
}

// Instances and Walk go in Ordinal order whatever the keys' values (§3; exit 1: a Go map's order never reaches any
// output). 300 int keys v_i = (i·919 mod 1000) − 500, distinct since gcd(919, 1000) = 1, so neither value order
// nor a map's order is ordinal order.
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
		if len(keys) != 300 {
			t.Fatalf("Walk yielded %d instances, want 300", len(keys))
		}
		for i, k := range keys {
			if k != strconv.Itoa(i) {
				t.Fatalf("run %d: Walk's instance %d has key %q, want %q (ordinal in decimal)", run, i, k, strconv.Itoa(i))
			}
		}
	}
}

type walked struct {
	Component, Key string
	State          any
}

// Walk (design v2 §3) yields ("clock", "", clock.RuleState) first, then per instance in Ordinal order ("instance",
// ordinal, InstanceState as a value) and the OpState's parts; an unborn instance has no parts. TTL 5 min.
//   - a's E1 at 0:00 (ordinal 0), released at clock 0:00.001: born (birth 0); schedules Completion 1:00 ref 1;
//     TTL 5:00; LastActivity 0.
//   - b's event at 0:10 (ordinal 1) and a's E3 at 0:20 arrive at clock 0:00.001 and wait: b unborn, LastActivity −∞.
//
// The clock: Inputs [0], Contributions [1], Clock 1. a: Events [(0, 0:20, 3)], Timers in walk order [Completion
// 1:00, TTL 5:00], part ("op", "a"). b: Events [(0, 0:10, 2)], no timers, Born false.
func TestWalkYieldsClockThenInstanceStateThenParts(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + minute, Kind: clock.Completion, Ref: 1}}
	}}
	w := newWorld(t, 5*minute, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	w.to(1)
	w.arrive(1, str("b"), ev(0, 10*second, 2))
	w.arrive(1, str("a"), ev(0, 20*second, 3))
	var got []walked
	w.r.Walk(func(component, key string, s any) { got = append(got, walked{component, key, s}) })
	if len(got) != 4 {
		t.Fatalf("Walk yielded %d items, want 4: %+v", len(got), got)
	}
	var shape []string
	for _, g := range got {
		shape = append(shape, g.Component+"/"+g.Key)
	}
	if want := []string{"clock/", "instance/0", "op/a", "instance/1"}; !slices.Equal(shape, want) {
		t.Fatalf("Walk order %v, want %v", shape, want)
	}
	rs, ok := got[0].State.(clock.RuleState)
	if !ok {
		t.Fatalf("the clock's state is %T, want clock.RuleState (a value)", got[0].State)
	}
	if !slices.Equal(rs.Inputs, []int{0}) || !slices.Equal(rs.Contributions, []int64{1}) || rs.Clock != 1 {
		t.Errorf("RuleState %+v, want Inputs [0], Contributions [1], Clock 1", rs)
	}
	a, ok := got[1].State.(partition.InstanceState)
	if !ok {
		t.Fatalf("instance 0's state is %T, want partition.InstanceState (a value, QA A3)", got[1].State)
	}
	if keyName(a.Key) != "a" || a.LastActivity != 0 || !a.Born || a.Birth != 0 {
		t.Errorf("a: Key %v LastActivity %d Born %v Birth %d; want a, 0, true, 0", a.Key, a.LastActivity, a.Born, a.Birth)
	}
	if len(a.Events) != 1 || a.Events[0].Ts != 20*second || a.Events[0].Seq != 3 {
		t.Errorf("a: Events %+v, want the one at 0:20 seq 3", a.Events)
	}
	if want := []clock.Timer{{Due: minute, Kind: clock.Completion, Ref: 1}, ttl(5 * minute)}; !slices.Equal(a.Timers, want) {
		t.Errorf("a: Timers %+v, want %+v", a.Timers, want)
	}
	bs, ok := got[3].State.(partition.InstanceState)
	if !ok {
		t.Fatalf("instance 1's state is %T, want partition.InstanceState", got[3].State)
	}
	if keyName(bs.Key) != "b" || bs.LastActivity != negInf || bs.Born || len(bs.Timers) != 0 {
		t.Errorf("b: %+v, want key b, LastActivity −∞, not born, no timers", bs)
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
