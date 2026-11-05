package partition_test

// Exit line 4 and the TTL purge. Sources: SEMANTICS §9 (as annotated 2026-11-04, D1: activity counts at release; a
// purge falls at its instant in the walk, before the key's later events, which are kept and start the key fresh);
// ADR-0004 §5 (one TTL timer per instance, re-armed on release, strict); ADR-0006 §3 (as annotated: freed only when
// Empty); design note s3-2a-1 §3 (Offer, Advance, freeing) and §6 (Exit 4, TTL purge rows).

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// Exit 4: one key, 10,000 events 1 s apart (ts_i = i·1000, i = 0…9999). After every walk the key holds exactly one
// pending timer, the TTLPurge at LastActivity + TTL = ts_i + 300,000 (design §3: the re-arm replaces the key's one
// TTL timer). The TTL never fires: the clock after event i is ts_i, and ts_i < ts_(i−1) + 300,000.
func TestOneTTLTimerPerInstance(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	for i := range int64(10_000) {
		ts := i * second
		if late := w.arrive(ts, str("a"), ev(0, ts, uint64(i+1))); late {
			t.Fatalf("event %d (ts %d) construct-late at clock %d", i, ts, ts)
		}
		ins := instances(w.r)
		if len(ins) != 1 {
			t.Fatalf("after event %d: %d instances, want 1", i, len(ins))
		}
		in := ins[0]
		if in.LastActivity != ts {
			t.Fatalf("after event %d: LastActivity %d, want %d", i, in.LastActivity, ts)
		}
		got := in.Clock.Timers(nil)
		if want := []clock.Timer{ttl(ts + ttlMs)}; !slices.Equal(got, want) {
			t.Fatalf("after event %d: pending %+v, want exactly %+v", i, got, want)
		}
	}
	if n := b.count("new"); n != 1 {
		t.Errorf("NewState called %d times, want 1 (one birth, no purge)", n)
	}
	if s := w.r.Stats(); s.Purged != 0 || s.ConstructLate != 0 {
		t.Errorf("Stats %+v, want zero", s)
	}
}

// Exit 4, many keys: 100 keys k0…k99, key i's one event at i·1000 → 100 TTL timers, key i's at i·1000 + 300,000.
func TestOneTTLTimerPerInstance_HundredKeys(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	w := newWorld(t, ttlMs, &body{}, 0)
	for i := range int64(100) {
		w.arrive(i*second, str("k"+keyName(event.Value{Kind: event.TInt, I: i})), ev(0, i*second, uint64(i+1)))
	}
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

// Exit 4, the budget (design §3 MUST, ADR-0010 §1): re-arming a resident, born key — Apply, Offer, Advance of its
// next event — allocates nothing. Each run moves the clock 1 s and releases one event; its TTL is re-armed in place
// (design §4: a replacing Schedule MUST NOT allocate).
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
}

// The purge falls at its instant in the walk, before the key's later buffered event, which starts the key fresh.
// TTL 5 min. E1 of a at 0:00 → TTL due 5:00. E2 of a at 7:00 is offered while the clock is 0 (buffered). The clock
// jumps to 10:00: the walk goes by instant — 5:00 the strict TTL (10:00 > 5:00 → purge), then 7:00 E2, which is a
// second birth (NewState again) and re-arms the TTL at 7:00 + 5:00 = 12:00 > 10:00 (pending).
func TestTTLPurgeBeforeLaterBufferedEvents(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	if late := w.arrive(0, str("a"), ev(0, 7*minute, 2)); late {
		t.Fatal("E2 at 7:00 construct-late at clock 0")
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

// Strict >: SEMANTICS §9 purges when wm − t_last > TTL. TTL 5 min.
//   - E1 at 0:00 → TTL due 5:00.
//   - E2 at exactly 5:00 with the clock at 5:00: the walk releases events before timers at an instant, and the
//     strict TTL waits for clock > 5:00 anyway; E2 re-arms it at 10:00. No purge, no second birth.
//   - clock 10:00: 10:00 − 5:00 = 5:00, not > 5:00 → no purge.
//   - clock 10:00.001: 5:00.001 > 5:00 → purge; the key holds nothing (no event, no timer, no seal) → freed, Len 0.
func TestTTLEventAtExactlyTTLKeepsKey(t *testing.T) {
	t.Parallel()
	const ttlMs = 5 * minute
	b := &body{}
	w := newWorld(t, ttlMs, b, 0)
	w.arrive(0, str("a"), ev(0, 0, 1))
	if late := w.arrive(5*minute, str("a"), ev(0, 5*minute, 2)); late {
		t.Fatal("E2 at 5:00 construct-late")
	}
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), Ev("a", 0, 5*minute))
	if in := find(w.r, "a"); in == nil || !slices.Equal(in.Clock.Timers(nil), []clock.Timer{ttl(10 * minute)}) {
		t.Fatalf("a after E2: want resident with exactly the TTL at 10:00")
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
//   - a at 0:00 (ordinal 0, TTL 5:00); b at 1:00 (ordinal 1, TTL 6:00).
//   - clock 5:00.001: a purged (> 5:00) and freed; b kept (< 6:00). Len 1.
//   - a returns at 6:40 (400,000): Offer makes ordinal 2; the walk to 6:40 purges b at 6:00 first (6:40 > 6:00),
//     then releases a's event: a new birth on ordinal 2. Len 1, Purged 2.
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
	wantLog(t, b, N("a", 0), Ev("a", 0, 0), N("b", 1), Ev("b", 1, 1*minute), P("a", 0, 0), P("b", 1, 1*minute),
		N("a", 2), Ev("a", 2, 400*second))
	if w.r.Len() != 1 {
		t.Errorf("Len %d, want 1 (b freed, a new)", w.r.Len())
	}
	if in := find(w.r, "a"); in == nil || in.Clock.Ordinal() != 2 {
		t.Errorf("returning a: want a new instance, ordinal 2")
	}
	if s := w.r.Stats(); s.Purged != 2 {
		t.Errorf("Stats.Purged %d, want 2", s.Purged)
	}
}

// SEMANTICS §9's worked example (TTL vs absence), at the partition: TTL 3 min, the body waits 5 min (a Completion at
// 5:00, ref 1). Read at 0:00, then silence. Clock 3:00: 3:00 − 0 is not > 3:00, nothing. Clock 3:00.001: the purge.
// OnPurge comes before the reset: Op is still set, LastActivity is still 0, and the Completion is still pending (the
// TTL timer itself has left the heap by firing). Then the reset drops every timer, so at +∞ the Completion never
// fires: "attack missed" — the configuration the deploy check (§11.3) rejects, which partition does not repeat.
func TestTTLPurgeComesBeforeResetAndDropsTimers(t *testing.T) {
	t.Parallel()
	b := &body{schedule: func(in *partition.Instance, e event.Event) []clock.Timer {
		return []clock.Timer{{Due: e.Ts + 5*minute, Kind: clock.Completion, Ref: 1}}
	}}
	w := newWorld(t, 3*minute, b, 0)
	w.arrive(0, str("k"), ev(0, 0, 1))
	in := find(w.r, "k")
	if in == nil {
		t.Fatal("k not resident")
	}
	// walk order (clock.timer.before): due 3:00 the strict TTL, due 5:00 the Completion
	if got, want := in.Clock.Timers(nil), []clock.Timer{ttl(3 * minute), {Due: 5 * minute, Kind: clock.Completion, Ref: 1}}; !slices.Equal(got, want) {
		t.Fatalf("k pending %+v, want %+v", got, want)
	}
	w.to(3 * minute)
	if w.r.Stats().Purged != 0 {
		t.Fatal("purged at clock = LastActivity + TTL; the purge is strict")
	}
	w.to(3*minute + 1)
	w.to(posInf)
	wantLog(t, b, N("k", 0), Ev("k", 0, 0), P("k", 0, 0, clock.Timer{Due: 5 * minute, Kind: clock.Completion, Ref: 1}))
	if s := w.r.Stats(); s.Purged != 1 || w.r.Len() != 0 {
		t.Errorf("Purged %d, Len %d; want 1, 0", s.Purged, w.r.Len())
	}
}

// A construct-late first event leaves no instance (design §3, Offer: an instance created by this call is freed at
// once). Clock 1:00 after a's event; b's first event at 0:30 < 1:00 is construct-late (clock.Instance.Buffer) → Len
// stays 1, no birth for b. A construct-late event of the resident a does not free a. ConstructLate counts both.
func TestConstructLateFirstEventLeavesNoInstance(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 5*minute, b, 0)
	w.arrive(1*minute, str("a"), ev(0, 1*minute, 1))
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
	wantLog(t, b, N("a", 0), Ev("a", 0, 1*minute))
	if s := w.r.Stats(); s.ConstructLate != 2 || s.Purged != 0 {
		t.Errorf("Stats %+v, want ConstructLate 2, Purged 0", s)
	}
}

// TTL 0 (a key-less rule, design §3 "The pseudo-key"): no TTL timer is ever armed and the key is never purged, even
// at +∞. CheckTimers expects no TTL timer when TTL is 0.
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

// Saturation (design §3: e.Ts + TTL saturates at MaxInt64). An event at MaxInt64 − 10 with TTL 5 min arms the TTL
// at MaxInt64, not a wrapped negative due that would purge the key at once.
func TestTTLDueSaturates(t *testing.T) {
	t.Parallel()
	b := &body{}
	w := newWorld(t, 5*minute, b, 0)
	ts := posInf - 10
	w.arrive(ts, str("a"), ev(0, ts, 1))
	in := find(w.r, "a")
	if in == nil {
		t.Fatal("a purged at once: the TTL due wrapped")
	}
	if got, want := in.Clock.Timers(nil), []clock.Timer{ttl(posInf)}; !slices.Equal(got, want) {
		t.Errorf("pending %+v, want %+v", got, want)
	}
}
