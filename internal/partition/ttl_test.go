package partition_test

// Exit line 4 and the TTL purge, under owner D1 (design note s3-2a-1 v2 §4.1: a walk releases an event only when
// Ts < clock and fires a timer only when due < clock; +∞ is past everything). Sources: SEMANTICS §9 (as annotated:
// activity counts at release; a purge falls at its instant in the walk, before the key's later events, which are
// kept and start the key fresh); ADR-0004 §5 (one TTL timer per instance, re-armed on release); ADR-0006 §3 (as
// annotated: freed only when Empty); design v2 §3 (Offer, Advance, OnPurge's view, pending free once — review F10,
// every resident key purges at +∞ — QA A9) and §6.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// Exit 4: one key, 10,000 events 1 s apart (ts_i = i·1000, i = 0…9999), each arriving with the clock at ts_i.
// Under D1 walk i releases event i−1 (ts_(i−1) < ts_i) and not event i (ts_i < ts_i is false). So after walk 0 the
// key is unborn and holds no timer; after walk i ≥ 1 it holds exactly one, the TTLPurge at LastActivity + TTL =
// ts_(i−1) + 300,000 (the re-arm replaces the key's one TTL timer). A last advance to 10,000,000 releases event 9999:
// TTL at 9,999,000 + 300,000 = 10,299,000. The TTL never fires: ts_(i−1) + 300,000 < ts_i is false for every i.
func TestOneTTLTimerPerInstance(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	check := func(after string, last int64) {
		t.Helper()
		ins := instances(w.r)
		if len(ins) != 1 {
			t.Fatalf("%s: %d instances, want 1", after, len(ins))
		}
		in := ins[0]
		got := in.Clock.Timers(nil)
		if last == negInf {
			if in.Op != nil || len(got) != 0 {
				t.Fatalf("%s: want unborn with no timer; Op set %v, pending %+v", after, in.Op != nil, got)
			}
			return
		}
		if in.LastActivity != last {
			t.Fatalf("%s: LastActivity %d, want %d", after, in.LastActivity, last)
		}
		if want := []clock.Timer{ttl(last + ttlMs)}; !slices.Equal(got, want) {
			t.Fatalf("%s: pending %+v, want exactly %+v", after, got, want)
		}
	}
	for i := range int64(10_000) {
		ts := i * second
		if late := w.arrive(ts, str("a"), ev(0, ts, uint64(i+1))); late {
			t.Fatalf("event %d (ts %d) construct-late at clock %d", i, ts, ts)
		}
		last := negInf
		if i > 0 {
			last = ts - second
		}
		check("after walk "+keyName(event.Value{Kind: event.TInt, I: i}), last)
	}
	w.to(10_000 * second)
	check("after the last release", 9_999*second)
	if n := b.count("new"); n != 1 {
		t.Errorf("NewState called %d times, want 1 (one birth, no purge)", n)
	}
	if n := b.count("event"); n != 10_000 {
		t.Errorf("%d events released, want 10,000", n)
	}
	if s := w.r.Stats(); s.Purged != 0 || s.ConstructLate != 0 {
		t.Errorf("Stats %+v, want zero", s)
	}
}

// Exit 4, many keys: 100 keys k0…k99, key i's one event at i·1000. The advance to 100,000 releases them all (each
// i·1000 < 100,000) → 100 TTL timers, key i's at i·1000 + 300,000, none fired (all > 100,000).
func TestOneTTLTimerPerInstance_HundredKeys(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	w := newWorld(t, ttlMs, &body{}, 0)
	for i := range int64(100) {
		w.arrive(i*second, str("k"+keyName(event.Value{Kind: event.TInt, I: i})), ev(0, i*second, uint64(i+1)))
	}
	w.to(100 * second)
	if n := w.r.Len(); n != 100 {
		t.Fatalf("Len %d, want 100", n)
	}
	ttls := 0
	for _, in := range instances(w.r) {
		for _, tm := range in.Clock.Timers(nil) {
			if tm.Kind != clock.TTLPurge {
				t.Errorf("key %s: unexpected timer %+v", keyName(in.Key), tm)
				continue
			}
			ttls++
			if tm.Due != in.LastActivity+ttlMs {
				t.Errorf("key %s: TTL due %d, want LastActivity %d + %d", keyName(in.Key), tm.Due, in.LastActivity, ttlMs)
			}
		}
	}
	if ttls != 100 {
		t.Errorf("%d TTL timers, want 100 (one per instance)", ttls)
	}
}

// quiet is a body that allocates nothing: its one state is shared and implies no timer.
type quiet struct{ st quietState }

type quietState struct{}

func (quietState) Walk(func(component, key string, state any)) {}
func (quietState) Timers(dst []clock.Timer) []clock.Timer      { return dst }

func (q *quiet) NewState(*partition.Instance) partition.OpState   { return &q.st }
func (q *quiet) OnEvent(*partition.Instance, event.Event)         {}
func (q *quiet) OnTimers(*partition.Instance, []clock.Timer, any) {}
func (q *quiet) OnDiscard(*partition.Instance, []uint64)          {}
func (q *quiet) OnPurge(*partition.Instance)                      {}

// Exit 4, the budget (design §3 MUST, ADR-0010 §1): Apply, Offer and Advance on a resident, born key allocate
// nothing. Each run moves the clock 1 s, buffers one event and releases the previous one (D1), whose release
// re-arms the TTL in place (§4.3: a replacing Schedule MUST NOT allocate).
func TestOneTTLTimerPerInstance_ReArmAllocatesNothing(t *testing.T) {
	const ttlMs = 5 * minute
	r, err := partition.NewRule(partition.Config{Inputs: []int{0}, KeyType: event.TString, TTL: ttlMs, Body: &quiet{}})
	if err != nil {
		t.Fatalf("NewRule: %v", err)
	}
	key := str("a")
	var ts int64
	var seq uint64
	step := func() {
		ts += second
		seq++
		r.Apply(clock.WMAdvance{StreamID: 0, Contribution: ts, Moved: true})
		r.Offer(key, event.Event{StreamID: 0, Ts: ts, Seq: seq})
		r.Advance(nil)
	}
	for range 16 { // birth and warm-up: the instance, its map entries, the heaps' capacity
		step()
	}
	if got := testing.AllocsPerRun(1000, step); got != 0 {
		t.Errorf("re-arm of a resident key: %v allocations per step, want 0", got)
	}
	if err := r.CheckTimers(); err != nil {
		t.Errorf("CheckTimers: %v", err)
	}
	if r.Stats().Purged != 0 || r.Len() != 1 {
		t.Errorf("Purged %d, Len %d; want 0, 1", r.Stats().Purged, r.Len())
	}
}

// The purge falls at its instant in the walk, before the key's later buffered event, which starts the key fresh.
// TTL 5 min.
//   - E1 of a at 0:00 arrives at clock 0; the advance to 0:00.001 releases it (0 < 1) → birth, TTL due 5:00.
//   - E2 of a at 7:00 arrives at clock 0:00.001 (admitted: 7:00 ≥ the clock) and waits.
//   - The clock jumps to 10:00. The walk goes by instant: 5:00 the TTL (5:00 < 10:00 → purge), then 7:00 E2
//     (7:00 < 10:00), a second birth (NewState again) that re-arms the TTL at 7:00 + 5:00 = 12:00 > 10:00 (pending).
func TestTTLPurgeBeforeLaterBufferedEvents(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	w.to(1)
	if late := w.arrive(1, str("a"), ev(0, 7*minute, 2)); late {
		t.Fatal("E2 at 7:00 construct-late at clock 0:00.001")
	}
	wantLog(t, b, N("a", 0), Ev("a", 0, 0))
	w.to(10 * minute)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), P("a", 0, 0), N("a", 0), Ev("a", 0, 7*minute))
	in := find(w.r, "a")
	if in == nil || w.r.Len() != 1 {
		t.Fatalf("after the purge and rebirth: Len %d, a resident %v; want 1, true", w.r.Len(), in != nil)
	}
	if in.Op == nil || in.LastActivity != 7*minute {
		t.Errorf("reborn a: Op set %v, LastActivity %d; want true, %d", in.Op != nil, in.LastActivity, 7*minute)
	}
	if got, want := in.Clock.Timers(nil), []clock.Timer{ttl(12 * minute)}; !slices.Equal(got, want) {
		t.Errorf("reborn a: pending %+v, want %+v (a new TTL)", got, want)
	}
	if s := w.r.Stats(); s.Purged != 1 {
		t.Errorf("Stats.Purged %d, want 1", s.Purged)
	}
}

// Strict >: SEMANTICS §9 purges when wm − t_last > TTL; under D1 the TTL due t_last + TTL fires once the clock is
// past it, which is the same thing. TTL 5 min.
//   - E1 at 0:00, released by the advance to 0:00.001 → TTL due 5:00.
//   - E2 at exactly 5:00 arrives with the clock at 5:00: admitted (5:00 < 5:00 is false), nothing fires (the TTL
//     needs 5:00 < clock).
//   - clock 5:00.001: instant 5:00 — events first: E2 is released and re-arms the TTL at 10:00, replacing the one
//     at 5:00 before the walk reaches 5:00's TTL phase. No purge, no second birth: the event at the deadline extends.
//   - clock 10:00: 10:00 < 10:00 is false → no purge. clock 10:00.001: purge; the key holds nothing → freed, Len 0.
func TestTTLEventAtExactlyTTLKeepsKey(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	w.to(1)
	if late := w.arrive(5*minute, str("a"), ev(0, 5*minute, 2)); late {
		t.Fatal("E2 at 5:00 with the clock at 5:00: construct-late, want admitted (D1)")
	}
	if w.r.Stats().Purged != 0 {
		t.Fatal("purged at clock 5:00 = LastActivity + TTL")
	}
	w.to(5*minute + 1)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), Ev("a", 0, 5*minute))
	if in := find(w.r, "a"); in == nil || !slices.Equal(in.Clock.Timers(nil), []clock.Timer{ttl(10 * minute)}) {
		t.Fatalf("a after E2's release: want resident with exactly the TTL at 10:00")
	}
	w.to(10 * minute)
	if s := w.r.Stats(); s.Purged != 0 || w.r.Len() != 1 {
		t.Fatalf("clock 10:00: Purged %d, Len %d; want 0, 1 (10:00 − 5:00 is not > 5:00)", s.Purged, w.r.Len())
	}
	w.to(10*minute + 1)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), Ev("a", 0, 5*minute), P("a", 0, 5*minute))
	if s := w.r.Stats(); s.Purged != 1 || w.r.Len() != 0 {
		t.Errorf("clock 10:00.001: Purged %d, Len %d; want 1, 0 (purged and freed)", s.Purged, w.r.Len())
	}
}

// A purged key holding nothing is freed; a returning key gets a new instance (ordinals are never reused,
// clock.RuleClock.NewInstance). TTL 5 min.
//   - a's event at 0:00 (ordinal 0) arrives at clock 0; b's at 1:00 (ordinal 1) arrives at clock 1:00, whose walk
//     releases a's (0 < 1:00): a's TTL 5:00.
//   - clock 5:00.001: instant 1:00 b's event (b's TTL 6:00), instant 5:00 a's TTL (< 5:00.001) → a purged, freed.
//   - a returns at 6:40 (400,000), clock 6:40: Offer makes ordinal 2; the walk fires b's TTL at 6:00 (< 6:40) → b
//     purged, freed; a's event (6:40 < 6:40 is false) waits. Len 1.
//   - clock 6:40.001: a's event released, a new birth on ordinal 2. Purged 2.
func TestTTLPurgedKeyFreedAndReturningKeyIsNew(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	w.arrive(1*minute, str("b"), ev(0, 1*minute, 2))
	w.to(5*minute + 1)
	if w.r.Len() != 1 || find(w.r, "a") != nil || find(w.r, "b") == nil {
		t.Fatalf("clock 5:00.001: Len %d; want 1 with only b resident", w.r.Len())
	}
	w.arrive(400*second, str("a"), ev(0, 400*second, 3))
	if w.r.Len() != 1 || find(w.r, "b") != nil {
		t.Fatalf("clock 6:40: Len %d; want 1 (b purged and freed, a waiting)", w.r.Len())
	}
	w.to(400*second + 1)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), N("b", 1), Ev("b", 1, 1*minute), P("a", 0, 0), P("b", 1, 1*minute),
		N("a", 2), Ev("a", 2, 400*second))
	if in := find(w.r, "a"); in == nil || in.Clock.Ordinal() != 2 {
		t.Errorf("returning a: want a new instance, ordinal 2")
	}
	if s := w.r.Stats(); s.Purged != 2 {
		t.Errorf("Stats.Purged %d, want 2", s.Purged)
	}
}

// Review F10: a key purged several times in one walk is listed for freeing once, and Stats.Purged counts every
// purge. TTL 1 min; a's events at 0:00, 2:00 and 4:00 are all offered at clock 0 (admitted, waiting).
//
//	clock 4:00.001 → 0:00 E1 birth (TTL 1:00) · 1:00 purge 1 · 2:00 E2 rebirth (TTL 3:00) · 3:00 purge 2 ·
//	                 4:00 E3 rebirth (TTL 5:00 > 4:00.001, pending). Ends born: resident, Purged 2.
//	clock 5:00.001 → 5:00 purge 3; nothing left → freed, Len 0.
//
// Then b, whose one walk (to 10:00.001) holds two purges and ends unborn: freed once, Len 0; b then returns as a
// new instance that lives normally.
func TestTTLRepeatedPurgesInOneWalk(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, minute, b, 0)
	for i, at := range []int64{0, 2 * minute, 4 * minute} {
		w.arrive(0, str("a"), ev(0, at, uint64(i+1)))
	}
	w.to(4*minute + 1)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), P("a", 0, 0), N("a", 0), Ev("a", 0, 2*minute), P("a", 0, 2*minute),
		N("a", 0), Ev("a", 0, 4*minute))
	in := find(w.r, "a")
	if in == nil || in.Op == nil || w.r.Len() != 1 || w.r.Stats().Purged != 2 {
		t.Fatalf("clock 4:00.001: want a resident and born, Len 1, Purged 2; got Len %d, Purged %d", w.r.Len(), w.r.Stats().Purged)
	}
	w.to(5*minute + 1)
	if w.r.Len() != 0 || w.r.Stats().Purged != 3 {
		t.Fatalf("clock 5:00.001: Len %d, Purged %d; want 0, 3", w.r.Len(), w.r.Stats().Purged)
	}

	b.log = nil
	for i, at := range []int64{6 * minute, 8 * minute, 9 * minute} { // b is ordinal 1
		w.arrive(5*minute+1, str("b"), ev(0, at, uint64(10+i)))
	}
	// clock 10:00.001 → 6:00 E birth (TTL 7:00) · 7:00 purge · 8:00 E rebirth (TTL 9:00) · 9:00 events first: E at
	// 9:00 released, re-arms TTL at 10:00 (the 9:00 TTL is replaced before its phase) · 10:00 purge. 2 purges, unborn.
	w.to(10*minute + 1)
	wantLog(t, b, N("b", 1), Ev("b", 1, 6*minute), P("b", 1, 6*minute), N("b", 1), Ev("b", 1, 8*minute),
		Ev("b", 1, 9*minute), P("b", 1, 9*minute))
	if w.r.Len() != 0 || w.r.Stats().Purged != 5 {
		t.Fatalf("clock 10:00.001: Len %d, Purged %d; want 0 (freed once), 5", w.r.Len(), w.r.Stats().Purged)
	}
	w.arrive(11*minute, str("b"), ev(0, 11*minute, 20))
	w.to(11*minute + 1)
	if in := find(w.r, "b"); in == nil || in.Clock.Ordinal() != 2 || in.LastActivity != 11*minute || w.r.Len() != 1 {
		t.Errorf("returning b: want a new instance (ordinal 2), LastActivity 11:00, Len 1")
	}
}

// SEMANTICS §9's worked example (TTL vs absence), at the partition, and OnPurge's view (design v2 §3, QA A4): TTL
// 3 min, the body waits 5 min (a Completion at e.Ts + 5:00, ref 1). Read at 0:00 (released at clock 0:00.001), then
// silence. Clock 3:00: 3:00 < 3:00 is false, nothing. Clock 3:00.001: the purge. Inside OnPurge, Op is still set,
// LastActivity is still 0, the TTL timer is gone (it fired) and the Completion is still pending. The reset then
// drops every timer, so at +∞ the Completion never fires: "attack missed" — the configuration the deploy check
// (§11.3) rejects, which partition does not repeat.
func TestTTLPurgeComesBeforeResetAndDropsTimers(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + 5*minute, Kind: clock.Completion, Ref: 1}}
	}}
	w := newWorld(t, 3*minute, b, 0)
	w.arrive(0, str("k"), ev(0, 0, 1))
	w.to(1)
	in := find(w.r, "k")
	if in == nil {
		t.Fatal("k not resident")
	}
	// walk order (clock.timer.before): due 3:00 the TTL, due 5:00 the Completion
	if got, want := in.Clock.Timers(nil), []clock.Timer{ttl(3 * minute), {Due: 5 * minute, Kind: clock.Completion, Ref: 1}}; !slices.Equal(got, want) {
		t.Fatalf("k pending %+v, want %+v", got, want)
	}
	w.to(3 * minute)
	if w.r.Stats().Purged != 0 {
		t.Fatal("purged at clock = LastActivity + TTL; the purge needs the clock past it")
	}
	w.to(3*minute + 1)
	w.to(posInf)
	wantLog(t, b, N("k", 0), Ev("k", 0, 0), P("k", 0, 0, clock.Timer{Due: 5 * minute, Kind: clock.Completion, Ref: 1}))
	if s := w.r.Stats(); s.Purged != 1 || w.r.Len() != 0 {
		t.Errorf("Purged %d, Len %d; want 1, 0", s.Purged, w.r.Len())
	}
}

// At the end of input (+∞) every resident key purges (design v2 §3, QA A9; SEMANTICS §2.1: all open state expires).
// TTL 5 min; a, b, c's events at 0:00, 0:01, 0:02 wait at clock 0. Clock +∞: the events by instant, then the TTLs
// at 5:00, 5:01, 5:02 by instant → three purges in key order, all freed.
func TestEndOfInputPurgesEveryResidentKey(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 5*minute, b, 0)
	for i, k := range []string{"a", "b", "c"} {
		w.arrive(0, str(k), ev(0, int64(i)*second, uint64(i+1)))
	}
	w.to(posInf)
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), N("b", 1), Ev("b", 1, second), N("c", 2), Ev("c", 2, 2*second),
		P("a", 0, 0), P("b", 1, second), P("c", 2, 2*second))
	if s := w.r.Stats(); s.Purged != 3 || w.r.Len() != 0 {
		t.Errorf("Purged %d, Len %d; want 3, 0", s.Purged, w.r.Len())
	}
}

// A construct-late first event leaves no instance (design §3, Offer: an instance created by this call is freed at
// once). Under D1 construct-late is Ts < clock: with the clock at 1:00, a's event at 1:00 is admitted; b's first
// event at 0:30 is construct-late → b leaves nothing, no birth; a's own event at 0:30 is construct-late and does not
// free a. ConstructLate counts both.
func TestConstructLateFirstEventLeavesNoInstance(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 5*minute, b, 0)
	if late := w.arrive(1*minute, str("a"), ev(0, 1*minute, 1)); late {
		t.Fatal("a at 1:00 with the clock at 1:00: want admitted (D1)")
	}
	if late := w.arrive(1*minute, str("b"), ev(0, 30*second, 2)); !late {
		t.Fatal("b at 0:30 with the clock at 1:00: want construct-late")
	}
	if w.r.Len() != 1 || find(w.r, "b") != nil {
		t.Errorf("after b's construct-late first event: Len %d; want 1, b not resident", w.r.Len())
	}
	if late := w.arrive(1*minute, str("a"), ev(0, 30*second, 3)); !late {
		t.Fatal("a at 0:30 with the clock at 1:00: want construct-late")
	}
	if w.r.Len() != 1 || find(w.r, "a") == nil {
		t.Errorf("a resident key's construct-late event freed it: Len %d", w.r.Len())
	}
	w.to(1*minute + 1)
	wantLog(t, b, N("a", 0), Ev("a", 0, 1*minute))
	if s := w.r.Stats(); s.ConstructLate != 2 || s.Purged != 0 {
		t.Errorf("Stats %+v, want ConstructLate 2, Purged 0", s)
	}
}

// TTL 0 (design v2 §3: accepted; the pseudo-key): no TTL timer is ever armed and the key is never purged, even at
// +∞. CheckTimers expects no TTL timer when TTL is 0. Events at 0:00 and 10:00; +∞ releases the second.
func TestTTLZeroNeverPurges(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 0, b, 0)
	w.arrive(0, str(""), ev(0, 0, 1))
	w.arrive(10*minute, str(""), ev(0, 10*minute, 2))
	w.to(posInf)
	in := find(w.r, "")
	if in == nil || w.r.Len() != 1 || in.Op == nil {
		t.Fatalf("the pseudo-key at +∞: want resident and born")
	}
	if got := in.Clock.Timers(nil); len(got) != 0 {
		t.Errorf("pseudo-key pending %+v, want none (TTL 0 arms nothing)", got)
	}
	if s := w.r.Stats(); s.Purged != 0 {
		t.Errorf("Stats.Purged %d, want 0", s.Purged)
	}
	wantLog(t, b, N("", 0), Ev("", 0, 0), Ev("", 0, 10*minute))
}

// Saturation (design §3: e.Ts + TTL saturates). An event at MaxInt64 − 10 (released at clock MaxInt64 − 9) with
// TTL 5 min arms the TTL at MaxInt64, not a wrapped negative due that would purge the key at once. At +∞ it fires
// (+∞ is past every instant): the key purges (A9).
func TestTTLDueSaturates(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 5*minute, b, 0)
	ts := posInf - 10
	w.arrive(ts, str("a"), ev(0, ts, 1))
	w.to(ts + 1)
	in := find(w.r, "a")
	if in == nil || in.Op == nil {
		t.Fatal("a not resident and born after its release: the TTL due wrapped")
	}
	if got, want := in.Clock.Timers(nil), []clock.Timer{ttl(posInf)}; !slices.Equal(got, want) {
		t.Errorf("pending %+v, want %+v", got, want)
	}
	w.to(posInf)
	if s := w.r.Stats(); s.Purged != 1 || w.r.Len() != 0 {
		t.Errorf("at +∞: Purged %d, Len %d; want 1, 0", s.Purged, w.r.Len())
	}
}
