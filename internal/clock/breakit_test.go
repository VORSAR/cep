package clock_test

// Break-it tests for S3.1-1's clock (QA, after the build). Red tests are findings: each names the spec line it
// violates. Green tests are properties that held and guard against regression.

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// bkRand is a tiny deterministic generator (xorshift64*), so a failing seed reproduces exactly.
type bkRand struct{ s uint64 }

func (r *bkRand) next() uint64 {
	r.s ^= r.s >> 12
	r.s ^= r.s << 25
	r.s ^= r.s >> 27
	return r.s * 2685821657736338717
}

func (r *bkRand) intn(n int) int { return int(r.next() % uint64(n)) }

// bkMix hashes a few numbers into one, so the test-double operator's choices are a pure function of what it saw.
func bkMix(xs ...uint64) uint64 {
	h := uint64(1469598103934665603)
	for _, x := range xs {
		h ^= x
		h *= 1099511628211
		h ^= h >> 29
	}
	return h
}

const (
	bkPhaseEvent = iota
	bkPhaseInclusive
	bkPhaseStrict
)

// bkOp is a deterministic test-double operator over one rule clock. Its callbacks schedule, re-arm, cancel and
// purge as a pattern operator would, as a pure function of (key ordinal, item), never of the clock. It records the
// callback sequence and checks the walk's invariants as it goes.
type bkOp struct {
	t       *testing.T
	rc      *clock.RuleClock
	log     []string
	pos     [2]int64 // (instant, phase) of the last item: non-decreasing when no callback schedules into the past
	posSet  bool
	nEvents int
	seen    map[evKey]int
}

func (o *bkOp) at(instant int64, phase int, what string) {
	o.t.Helper()
	p := [2]int64{instant, int64(phase)}
	if o.posSet && (p[0] < o.pos[0] || (p[0] == o.pos[0] && p[1] < o.pos[1])) {
		o.t.Errorf("walk went backwards: %s at (instant %d, phase %d) after (instant %d, phase %d)",
			what, instant, phase, o.pos[0], o.pos[1])
	}
	o.pos, o.posSet = p, true
}

func (o *bkOp) callbacks() clock.Callbacks {
	return clock.Callbacks{
		OnEvent: func(in *clock.Instance, e event.Event) {
			if e.Ts >= o.rc.Clock() && o.rc.Clock() != math.MaxInt64 { // owner D1 (2026-11-05): only once past it
				o.t.Errorf("event %+v released with Ts ≥ clock %d", keyOf(e), o.rc.Clock())
			}
			o.at(e.Ts, bkPhaseEvent, "event")
			o.seen[keyOf(e)]++
			o.nEvents++
			o.log = append(o.log, fmt.Sprintf("E k%d s%d t%d q%d", in.Ordinal(), e.StreamID, e.Ts, e.Seq))
			h := bkMix(in.Ordinal(), uint64(e.Ts), e.Seq)
			// re-arm the TTL at release (ADR-0004 §5 as annotated); its ref varies on purpose
			in.Schedule(e.Ts+int64(h%4), clock.TTLPurge, h%5)
			// an inclusive timer of a varying kind, at or after the event
			kinds := []clock.TimerKind{clock.Completion, clock.CronClose, clock.DelayDue, clock.BatchClose}
			in.Schedule(e.Ts+int64((h>>8)%3), kinds[(h>>16)%4], (h>>20)%2)
			if (h>>24)%2 == 0 {
				in.Schedule(e.Ts+int64((h>>28)%3), clock.Discard, e.Seq)
			}
			if (h>>32)%3 == 0 {
				in.Cancel(clock.Discard, e.Seq-1)
			}
		},
		OnTimers: func(in *clock.Instance, fires []clock.Timer) {
			if len(fires) == 0 {
				o.t.Errorf("empty OnTimers batch")
			}
			for _, tm := range fires {
				ph := bkPhaseInclusive
				if tm.Kind == clock.TTLPurge {
					ph = bkPhaseStrict
				}
				if tm.Due >= o.rc.Clock() && o.rc.Clock() != math.MaxInt64 { // owner D1: every kind needs clock > due
					o.t.Errorf("%+v fired with clock %d (needs clock > due, or +∞)", tm, o.rc.Clock())
				}
				if tm.Kind == clock.Discard {
					o.t.Errorf("Discard through OnTimers: %+v", tm)
				}
				o.at(tm.Due, ph, "timer")
				o.log = append(o.log, fmt.Sprintf("T k%d d%d K%d r%d", in.Ordinal(), tm.Due, tm.Kind, tm.Ref))
			}
			for _, tm := range fires {
				switch {
				case tm.Kind == clock.TTLPurge:
					in.CancelAll() // the purge's reset (D1); buffered events stay
				case tm.Kind == clock.Completion && tm.Ref == 0:
					in.Schedule(tm.Due+1, clock.CronClose, 7) // a follow-up that joins the walk in order
				case tm.Kind == clock.CronClose && tm.Ref == 1:
					in.Cancel(clock.Discard, uint64(tm.Due)) // cancel from inside a group
				}
			}
		},
		OnDiscard: func(in *clock.Instance, refs []uint64) {
			if len(refs) == 0 {
				o.t.Errorf("empty OnDiscard batch")
			}
			o.log = append(o.log, fmt.Sprintf("D k%d", in.Ordinal()))
			for _, r := range refs {
				o.log = append(o.log, fmt.Sprintf("  r%d", r))
			}
		},
	}
}

type bkScenario struct {
	keys   int
	events []struct {
		key int
		e   event.Event
	}
	timers []struct {
		key  int
		due  int64
		kind clock.TimerKind
		ref  uint64
	}
}

func bkGen(seed uint64) bkScenario {
	r := &bkRand{s: seed*0x9E3779B97F4A7C15 + 1}
	var sc bkScenario
	sc.keys = 1 + r.intn(4)
	n := r.intn(14)
	seq := map[int]uint64{}
	for i := 0; i < n; i++ {
		sid := r.intn(3)
		seq[sid]++
		sc.events = append(sc.events, struct {
			key int
			e   event.Event
		}{r.intn(sc.keys), ev(sid, int64(r.intn(12)), seq[sid])})
	}
	m := r.intn(8)
	for i := 0; i < m; i++ {
		sc.timers = append(sc.timers, struct {
			key  int
			due  int64
			kind clock.TimerKind
			ref  uint64
		}{r.intn(sc.keys), int64(r.intn(15)), clock.TimerKind(1 + r.intn(6)), uint64(r.intn(3))})
	}
	return sc
}

// bkRun buffers the scenario into a fresh rule clock (input stream 0), then walks it to each target in turn.
// It returns the callback log and the instances.
func bkRun(t *testing.T, sc bkScenario, targets []int64) ([]string, *bkOp, []*clock.Instance) {
	t.Helper()
	rc := clock.NewRuleClock([]int{0})
	ins := make([]*clock.Instance, sc.keys)
	for i := range ins {
		ins[i] = rc.NewInstance()
	}
	op := &bkOp{t: t, rc: rc, seen: map[evKey]int{}}
	for _, b := range sc.events {
		if ins[b.key].Buffer(b.e) {
			t.Fatalf("event %+v construct-late on a fresh clock at −∞", keyOf(b.e))
		}
	}
	for _, tm := range sc.timers {
		ins[tm.key].Schedule(tm.due, tm.kind, tm.ref)
	}
	for _, x := range targets {
		to(rc, x, 0)
		if rc.Clock() < x {
			t.Fatalf("clock %d below target %d", rc.Clock(), x)
		}
		rc.Advance(op.callbacks()) // the position check runs across advances too: the walk only moves forward
	}
	return op.log, op, ins
}

// TestBreakit_D1_GranularityRandom: SEMANTICS §6.4 as annotated (D1) — "Results no longer depend on how far one
// advance jumps." Over 3000 random scenarios (several keys, equal timestamps, equal dues, every kind, callbacks that
// schedule, re-arm, cancel and purge), one advance, random splits and one-ms steps give the same callback log; the
// same input twice gives the same log; every buffered event is released exactly once; every instance ends Empty.
func TestBreakit_D1_GranularityRandom(t *testing.T) {
	const final = 40
	for seed := uint64(0); seed < 3000; seed++ {
		sc := bkGen(seed)
		one, op, ins := bkRun(t, sc, []int64{final})
		again, _, _ := bkRun(t, sc, []int64{final})
		if !reflect.DeepEqual(one, again) {
			t.Fatalf("seed %d: same input twice, different logs:\n%v\n%v", seed, one, again)
		}
		// splits
		r := &bkRand{s: seed + 77}
		var splits []int64
		for x := int64(-2); x < final; x += int64(1 + r.intn(6)) {
			splits = append(splits, x)
		}
		splits = append(splits, final)
		split, _, _ := bkRun(t, sc, splits)
		var steps []int64
		for x := int64(-1); x <= final; x++ {
			steps = append(steps, x)
		}
		step, _, _ := bkRun(t, sc, steps)
		if !reflect.DeepEqual(one, split) {
			t.Fatalf("seed %d: one advance vs splits %v differ:\none:   %v\nsplit: %v", seed, splits, one, split)
		}
		if !reflect.DeepEqual(one, step) {
			t.Fatalf("seed %d: one advance vs one-ms steps differ:\none:  %v\nstep: %v", seed, one, step)
		}
		if op.nEvents != len(sc.events) {
			t.Fatalf("seed %d: %d events released, %d buffered", seed, op.nEvents, len(sc.events))
		}
		for k, n := range op.seen {
			if n != 1 {
				t.Fatalf("seed %d: event %+v released %d times", seed, k, n)
			}
		}
		// everything left is a timer due at or before 40+4; walk to +∞ and every key must be Empty
		to(op.rc, posInf, 0)
		op.rc.Advance(op.callbacks())
		for i, in := range ins {
			if !in.Empty() {
				t.Fatalf("seed %d: key %d not Empty after the clock reached +∞", seed, i)
			}
			op.rc.Free(in)
		}
	}
}

// TestBreakit_PartialAdvanceHoldsTheRest: an advance to x releases exactly the events with Ts < x (owner D1,
// 2026-11-05: an instant is finalized only once the clock is past it) and leaves the rest buffered (Empty false
// while one is held), whatever x.
func TestBreakit_PartialAdvanceHoldsTheRest(t *testing.T) {
	for seed := uint64(0); seed < 500; seed++ {
		sc := bkGen(seed)
		for x := int64(-1); x <= 12; x++ {
			_, op, ins := bkRun(t, sc, []int64{x})
			want := 0
			held := make([]int, sc.keys)
			for _, b := range sc.events {
				if b.e.Ts < x {
					want++
				} else {
					held[b.key]++
				}
			}
			if op.nEvents != want {
				t.Fatalf("seed %d x %d: released %d, want %d", seed, x, op.nEvents, want)
			}
			for k, n := range held {
				if n > 0 && ins[k].Empty() {
					t.Fatalf("seed %d x %d: key %d holds %d events but reports Empty", seed, x, k, n)
				}
			}
		}
	}
}

// TestBreakit_CancelAllInsidePurgeDropsSameInstantDiscard: the purge precedes the discard at one instant (§3) and its
// CancelAll inside OnTimers removes the key's discard due at that very instant; another key's discard still fires.
func TestBreakit_CancelAllInsidePurgeDropsSameInstantDiscard(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a, b := rc.NewInstance(), rc.NewInstance()
	a.Schedule(5, clock.Discard, 0) // ref 0 < the TTL's ref: the kind still orders the purge first
	a.Schedule(5, clock.TTLPurge, 9)
	b.Schedule(5, clock.Discard, 1)
	rec := &recorder{t: t, onTimer: func(in *clock.Instance, tm clock.Timer) {
		if tm.Kind == clock.TTLPurge {
			in.CancelAll()
		}
	}}
	to(rc, 6, 0)
	got := rec.advance(rc)
	want := []item{T(0, 5, clock.TTLPurge, 9), D(1, 1)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !a.Empty() || !b.Empty() {
		t.Fatalf("Empty: a %v b %v", a.Empty(), b.Empty())
	}
}

// TestBreakit_ScheduleForAnotherKeyFromACallback: OnEvent of key 1 schedules an inclusive timer for key 0 at the
// same instant; it joins the walk in order. Keys go in BIRTH order (the oracle's, as the two findings below pin):
// key 1 was born first, by its own timer; key 0 is born only when the callback schedules for it. (Coordinator,
// 2026-11-04: this guard was written while the walk still ordered keys by creation; its expectation follows the
// findings' birth order.) Owner D1 (2026-11-05): the instant 5 goes once the clock is past it, at 6.
func TestBreakit_ScheduleForAnotherKeyFromACallback(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	k1.Buffer(ev(0, 5, 1))
	k1.Schedule(5, clock.Completion, 0)
	rec := &recorder{t: t, onEvent: func(in *clock.Instance, e event.Event) { k0.Schedule(5, clock.DelayDue, 3) }}
	to(rc, 5, 0)
	if got := rec.advance(rc); len(got) != 0 {
		t.Fatalf("clock 5 delivered %+v; D1: the instant 5 waits for the clock to pass it", got)
	}
	to(rc, 6, 0)
	got := rec.advance(rc)
	want := []item{E(1, 0, 5, 1), T(1, 5, clock.Completion, 0), T(0, 5, clock.DelayDue, 3)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestBreakit_D1_SealRetired_EveryKeyKeepsItsEventAtTheTimersInstant (was TestBreakit_D7_SealIsPerKeyAndOnlyAfter
// Inclusive): owner D1 (2026-11-05) retires S3.1-1's D7 seal. At clock 5 key c's Discard due 4 fires (4 < 5) and
// key a's Completion due 5 waits; then an event at 5 is kept for every key — a, whose timer is pending at 5, b,
// which has none, and c, whose strict timer just fired. At clock 6 the instant 5 goes: the three events in
// event.Compare order, then a's Completion. Then 5 < 6 makes an event at 5 construct-late for every key alike.
func TestBreakit_D1_SealRetired_EveryKeyKeepsItsEventAtTheTimersInstant(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a, b, c := rc.NewInstance(), rc.NewInstance(), rc.NewInstance()
	a.Schedule(5, clock.Completion, 0)
	c.Schedule(4, clock.Discard, 0)
	rec := &recorder{t: t}
	to(rc, 5, 0)
	if got, want := rec.advance(rc), []item{D(2, 0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clock 5: got %+v, want %+v (only the Discard due 4)", got, want)
	}
	for i, in := range []*clock.Instance{a, b, c} {
		if in.Buffer(ev(0, 5, uint64(i+1))) {
			t.Fatalf("key %d: event at 5 = clock construct-late; D1: kept (the seal is retired)", i)
		}
	}
	to(rc, 6, 0)
	want := []item{E(0, 0, 5, 1), E(1, 0, 5, 2), E(2, 0, 5, 3), T(0, 5, clock.Completion, 0)}
	if got := rec.advance(rc); !reflect.DeepEqual(got, want) {
		t.Fatalf("clock 6: got %+v, want %+v", got, want)
	}
	for i, in := range []*clock.Instance{a, b, c} {
		if !in.Buffer(ev(0, 5, uint64(i+4))) {
			t.Fatalf("key %d: event at 5 with the clock at 6 kept, want construct-late", i)
		}
	}
}

// TestBreakit_TTLOneTimerWhateverRef: a key has exactly one TTL timer whatever its ref (ADR-0004 §5); Cancel of the
// TTL with any ref cancels it.
func TestBreakit_TTLOneTimerWhateverRef(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a := rc.NewInstance()
	for ref := uint64(0); ref < 5; ref++ {
		a.Schedule(int64(10+ref), clock.TTLPurge, ref)
	}
	rec := &recorder{t: t}
	to(rc, 100, 0)
	got := rec.advance(rc)
	if want := []item{T(0, 14, clock.TTLPurge, 4)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	a.Schedule(200, clock.TTLPurge, 1)
	if !a.Cancel(clock.TTLPurge, 99) || !a.Empty() {
		t.Fatalf("Cancel(TTLPurge, other ref) must cancel the key's one TTL timer")
	}
}

// TestBreakit_StrictAtTheClockWaitsAndASameInstantEventGoesFirst: a strict timer due at the clock does not fire
// (§3 "only if clock > t"); the key is not sealed by it, so an event at that instant is kept, and on the next
// advance it is released before the strict timer (events, then inclusive, then strict at one instant).
func TestBreakit_StrictAtTheClockWaitsAndASameInstantEventGoesFirst(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a := rc.NewInstance()
	a.Schedule(5, clock.Discard, 1)
	a.Schedule(5, clock.TTLPurge, 0)
	rec := &recorder{t: t}
	to(rc, 5, 0)
	if got := rec.advance(rc); len(got) != 0 {
		t.Fatalf("strict timers due 5 fired at clock 5: %+v", got)
	}
	if a.Buffer(ev(0, 5, 1)) {
		t.Fatalf("event at 5 dropped; nothing inclusive fired, nothing is sealed")
	}
	to(rc, 6, 0)
	got := rec.advance(rc)
	want := []item{E(0, 0, 5, 1), T(0, 5, clock.TTLPurge, 0), D(0, 1)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// ---- findings (red) ----

// TestBreakit_D1_FreeingAnEmptyKeyLosesNothing (was TestBreakit_FreeingAnEmptyKeyForgetsItsSeal, a finding on the
// D7 seal, which owner D1 of 2026-11-05 retires). A key's Completion due 5 waits at clock 5 (the key is not Empty:
// its timer is pending), so its event at 5 is kept; at clock 6 the event, then the Completion, and the key is
// Empty. The shard frees it; a fresh instance for the same key rejects an event at 5 by the clock alone (5 < 6) and
// keeps one at 6 = clock — nothing the seal held is lost by freeing.
func TestBreakit_D1_FreeingAnEmptyKeyLosesNothing(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a := rc.NewInstance()
	a.Schedule(5, clock.Completion, 0)
	rec := &recorder{t: t}
	to(rc, 5, 0)
	if got := rec.advance(rc); len(got) != 0 {
		t.Fatalf("clock 5 delivered %+v; D1: the Completion due 5 waits", got)
	}
	if a.Empty() {
		t.Fatalf("Empty with its Completion due 5 pending")
	}
	if a.Buffer(ev(0, 5, 1)) {
		t.Fatalf("event at 5 = clock construct-late, want kept")
	}
	to(rc, 6, 0)
	if got, want := rec.advance(rc), []item{E(0, 0, 5, 1), T(0, 5, clock.Completion, 0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clock 6: got %+v, want %+v", got, want)
	}
	if !a.Empty() {
		t.Fatalf("not Empty after its event and timer went (D1: no seal to keep)")
	}
	rc.Free(a)
	reborn := rc.NewInstance()
	if !reborn.Buffer(ev(0, 5, 2)) {
		t.Fatalf("fresh instance: event at 5 with the clock at 6 kept, want construct-late")
	}
	if reborn.Buffer(ev(0, 6, 3)) {
		t.Fatalf("fresh instance: event at 6 = clock construct-late, want kept")
	}
}

// TestBreakit_KeysWalkInBirthOrder_BirthIsFirstRelease — FINDING. ADR-0004 as annotated: "RuleClock … walks its
// instances (keys in birth order)"; SEMANTICS §9: "an instance for key k is born when the first event routes to k";
// the oracle (executor.py `_next_timer`: `for order, k in enumerate(self.last_act)`, with last_act first written in
// `_process`, i.e. when the key's first event is RELEASED in §2.2 order) orders keys by first release. The Go API
// fixes the ordinal at NewInstance, which the shard must call before Buffer, i.e. at ARRIVAL. Key b's event (ts 10)
// arrives before key a's (ts 5); both keys' completions are due at 20. The oracle fires a first (born at release 5);
// the clock fires b first. Output order across keys at one instant differs from the oracle.
func TestBreakit_KeysWalkInBirthOrder_BirthIsFirstRelease(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	b := rc.NewInstance() // b's event arrives first: the shard needs b's instance to Buffer it
	b.Buffer(ev(0, 10, 1))
	a := rc.NewInstance()
	a.Buffer(ev(0, 5, 2))
	name := map[*clock.Instance]string{a: "a", b: "b"}
	var got []string
	cb := clock.Callbacks{
		OnEvent: func(in *clock.Instance, e event.Event) {
			got = append(got, "event "+name[in])
			in.Schedule(20, clock.Completion, 0)
		},
		OnTimers: func(in *clock.Instance, fires []clock.Timer) { got = append(got, "complete "+name[in]) },
	}
	to(rc, 30, 0)
	rc.Advance(cb)
	want := []string{"event a", "event b", "complete a", "complete b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (keys in birth order; the oracle's birth is the key's first released event)",
			got, want)
	}
}

// TestBreakit_PurgedKeyIsBornAgain — FINDING. SEMANTICS §9 as annotated (D1): "a purge falls at its instant in the
// walk, before the key's later events, which are kept and start the key fresh"; §9: "after a purge, a returning key
// starts a fresh instance"; executor.py `_walk` deletes last_act[k] on purge, so the key's next released event
// re-inserts it LAST in birth order. In the clock, the purged key keeps its buffered events (correctly) but also its
// old ordinal, so it still sorts before keys born after it. Key a (born first) is purged at 5 with an event at 8
// still buffered; key b was born at 6. Both complete at 20: the oracle fires b first (a was reborn at 8); the clock
// fires a first.
func TestBreakit_PurgedKeyIsBornAgain(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a := rc.NewInstance()
	a.Buffer(ev(0, 1, 1))
	b := rc.NewInstance()
	b.Buffer(ev(0, 6, 2))
	a.Buffer(ev(0, 8, 3))
	name := map[*clock.Instance]string{a: "a", b: "b"}
	var got []string
	cb := clock.Callbacks{
		OnEvent: func(in *clock.Instance, e event.Event) {
			got = append(got, fmt.Sprintf("event %s@%d", name[in], e.Ts))
			if in == a && e.Ts == 1 {
				in.Schedule(4, clock.TTLPurge, 0) // TTL 3 from the release at 1: purged once the clock passes 4
				return
			}
			in.Schedule(20, clock.Completion, 0)
		},
		OnTimers: func(in *clock.Instance, fires []clock.Timer) {
			for _, f := range fires {
				if f.Kind == clock.TTLPurge {
					got = append(got, "purge "+name[in])
					in.CancelAll()
				} else {
					got = append(got, "complete "+name[in])
				}
			}
		},
	}
	to(rc, 30, 0)
	rc.Advance(cb)
	want := []string{"event a@1", "purge a", "event b@6", "event a@8", "complete b", "complete a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (a purged key's kept events start a fresh instance, born after b)", got, want)
	}
}

// TestBreakit_EventAtMinInt64OnAFreshKey — FINDING (extreme value). Design §2: Buffer is construct-late "when
// e.Ts < clock, or when e.Ts ≤ sealed, where sealed is the latest instant at which this key's timers fired". A key
// none of whose timers fired has no sealed instant, but `sealed` starts at math.MinInt64, so an event at
// Ts = math.MinInt64 is construct-late on a fresh key while the rule clock is still at −∞.
func TestBreakit_EventAtMinInt64OnAFreshKey(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	a := rc.NewInstance()
	if a.Buffer(ev(0, math.MinInt64, 1)) {
		t.Errorf("event at Ts=MinInt64 judged construct-late on a fresh key at a fresh clock: no timer fired, " +
			"nothing is sealed (design §2, D7)")
	}
}

// TestBreakit_StrictTimerAtMaxInt64FiresAtEndOfInput — FINDING (extreme value). Design §1: "+∞ only after
// EndOfInput"; §3: strict timers fire "only if clock > t"; the executor drains to math.inf at end of input, where
// inf > every due. The clock represents +∞ as math.MaxInt64, so a strict timer whose due saturated to MaxInt64
// (t_last + TTL or t_first + within near the top of the range; design §1: "additions [saturate] at MaxInt64")
// never fires, and its key never becomes Empty.
func TestBreakit_StrictTimerAtMaxInt64FiresAtEndOfInput(t *testing.T) {
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "s", Delay: 0}}, 0)
	rc := clock.NewRuleClock([]int{0})
	a := rc.NewInstance()
	a.Schedule(math.MaxInt64, clock.Discard, 1)
	for _, adv := range reg.EndOfInput(nil) {
		rc.ApplyAdvance(adv)
	}
	got := (&recorder{t: t}).advance(rc)
	if want := []item{D(0, 1)}; !reflect.DeepEqual(got, want) || !a.Empty() {
		t.Errorf("after EndOfInput (clock +∞): got %+v, Empty %v; want %+v and Empty (the oracle's wm = inf > due)",
			got, a.Empty(), want)
	}
}

// ---- the registry against a direct model of design §1 ----

type bkStreamModel struct {
	spec                       clock.StreamSpec
	hasMax                     bool
	maxTs, lastTs, lastArrival int64
	floor, vfloor, punct       int64
	ended                      bool
}

func (m *bkStreamModel) contrib() int64 {
	if m.ended {
		return posInf
	}
	c := max(m.floor, m.vfloor, m.punct)
	if m.hasMax {
		c = max(c, m.maxTs-m.spec.Delay)
	}
	return c
}

// TestBreakit_RegistryMatchesTheModel: random events (out of order, stream-late), ticks, virtual time and
// punctuations over random specs; after every operation each contribution equals design §1's formulas
// (admission, quiet floor from the largest event time at wall speed, virtual floor, punctuation), never falls,
// Moved is set exactly when it rose, buffers are appended to and not reset; the rule clock is the clamped min.
func TestBreakit_RegistryMatchesTheModel(t *testing.T) {
	for seed := uint64(1); seed <= 2000; seed++ {
		r := &bkRand{s: seed * 0x2545F4914F6CDD1D}
		n := 1 + r.intn(3)
		start := int64(r.intn(50)) - 25
		specs := make([]clock.StreamSpec, n)
		ms := make([]*bkStreamModel, n)
		for i := range specs {
			specs[i] = clock.StreamSpec{Name: fmt.Sprint(i), Delay: int64(r.intn(4)), IdleAfter: int64(r.intn(4)),
				Derived: r.intn(6) == 0}
			ms[i] = &bkStreamModel{spec: specs[i], lastTs: start, lastArrival: start, floor: negInf, vfloor: negInf,
				punct: negInf}
		}
		reg := clock.NewStreamRegistry(specs, start)
		inputs := []int{}
		for i := 0; i < n; i++ {
			if r.intn(3) != 0 {
				inputs = append(inputs, i)
			}
		}
		if len(inputs) == 0 {
			inputs = []int{0}
		}
		rc := clock.NewRuleClock(inputs)
		prev := make([]int64, n)
		for i := range prev {
			prev[i] = negInf
		}
		wall := start
		clockPrev := negInf
		var trace []string
		check := func(advs []clock.WMAdvance) {
			for _, a := range advs {
				if !a.Moved {
					t.Fatalf("seed %d %v: an unmoved advance in buf: %+v", seed, trace, a)
				}
				rc.ApplyAdvance(a)
			}
			moved := map[int]bool{}
			for _, a := range advs {
				moved[a.StreamID] = true
				if a.Contribution != reg.Contribution(a.StreamID) {
					t.Fatalf("seed %d %v: advance %+v vs Contribution %d", seed, trace, a, reg.Contribution(a.StreamID))
				}
			}
			for i, m := range ms {
				got, want := reg.Contribution(i), m.contrib()
				if got != want {
					t.Fatalf("seed %d %v: stream %d contribution %d, model %d", seed, trace, i, got, want)
				}
				if got < prev[i] {
					t.Fatalf("seed %d %v: stream %d contribution fell %d → %d", seed, trace, i, prev[i], got)
				}
				if (got > prev[i]) != moved[i] {
					t.Fatalf("seed %d %v: stream %d rose %v but Moved %v", seed, trace, i, got > prev[i], moved[i])
				}
				prev[i] = got
			}
			want := posInf
			for _, sid := range inputs {
				want = min(want, ms[sid].contrib())
			}
			if rc.Clock() != max(clockPrev, want) {
				t.Fatalf("seed %d %v: rule clock %d, want max(previous %d, min over inputs %d)", seed, trace,
					rc.Clock(), clockPrev, want)
			}
			clockPrev = rc.Clock()
		}
		ops := 5 + r.intn(25)
		for k := 0; k < ops; k++ {
			wall += int64(r.intn(4))
			switch op := r.intn(10); {
			case op < 5:
				sid := r.intn(n)
				m := ms[sid]
				if m.spec.Derived {
					continue
				}
				ts := wall + int64(r.intn(9)) - 6
				trace = append(trace, fmt.Sprintf("ev(s%d,t%d,a%d)", sid, ts, wall))
				late, adv := reg.ObserveEvent(sid, ts, wall)
				m.lastArrival = wall
				wantLate := m.hasMax && ts < m.maxTs-m.spec.Delay
				if !wantLate {
					if !m.hasMax || ts > m.maxTs {
						m.maxTs = ts
					}
					m.hasMax = true
					m.lastTs = m.maxTs
				}
				if late != wantLate {
					t.Fatalf("seed %d %v: streamLate %v, want %v", seed, trace, late, wantLate)
				}
				var advs []clock.WMAdvance
				if adv.Moved {
					advs = append(advs, adv)
				}
				check(advs)
			case op < 8:
				trace = append(trace, fmt.Sprintf("tick(%d)", wall))
				for _, m := range ms {
					if !m.spec.Derived && m.spec.IdleAfter > 0 && wall-m.lastArrival >= m.spec.IdleAfter {
						m.floor = max(m.floor, m.lastTs+(wall-m.lastArrival)-m.spec.Delay)
					}
				}
				sentinel := clock.WMAdvance{StreamID: 99, Contribution: 1, Moved: true}
				buf := reg.ObserveTick(wall, []clock.WMAdvance{sentinel})
				if len(buf) == 0 || buf[0] != sentinel {
					t.Fatalf("seed %d: ObserveTick dropped the caller's buffer prefix", seed)
				}
				check(buf[1:])
			case op < 9:
				v := wall - int64(r.intn(10))
				trace = append(trace, fmt.Sprintf("virt(%d)", v))
				for _, m := range ms {
					if !m.spec.Derived {
						m.vfloor = max(m.vfloor, v-m.spec.Delay)
					}
				}
				check(reg.ObserveVirtual(v, nil))
			default:
				sid := r.intn(n)
				w := wall - int64(r.intn(10))
				trace = append(trace, fmt.Sprintf("punct(s%d,%d)", sid, w))
				ms[sid].punct = max(ms[sid].punct, w)
				adv := reg.ObservePunctuation(sid, w)
				var advs []clock.WMAdvance
				if adv.Moved {
					advs = append(advs, adv)
				}
				check(advs)
			}
		}
		trace = append(trace, "eoi")
		for _, m := range ms {
			m.ended = true
		}
		check(reg.EndOfInput(nil))
		if rc.Clock() != posInf {
			t.Fatalf("seed %d: rule clock %d after EndOfInput", seed, rc.Clock())
		}
		// more input after the end moves nothing
		if _, adv := reg.ObserveEvent(0, wall+1000, wall+1000); adv.Moved {
			t.Fatalf("seed %d: an event after EndOfInput moved a contribution", seed)
		}
	}
}

// TestBreakit_GranularityScenariosAreNonTrivial guards the random test against generating nothing: across its seeds
// the walk delivers events, every kind of timer, discards and purges, and same-instant ties across keys.
func TestBreakit_GranularityScenariosAreNonTrivial(t *testing.T) {
	counts := map[byte]int{}
	for seed := uint64(0); seed < 3000; seed++ {
		log, _, _ := bkRun(t, bkGen(seed), []int64{40})
		for _, l := range log {
			counts[l[0]]++
		}
	}
	if counts['E'] < 10000 || counts['T'] < 10000 || counts['D'] < 1000 {
		t.Fatalf("too little exercised: %v", counts)
	}
}
