package clock_test

// The per-rule clock (owner decisions D1, D2, D7; design v2 §2–§3; decisions review D1-1, D1-2, D2-2).

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// D2: one clock per rule, shared by its keys; a key born after the clock passed t starts at the rule's current
// time, so its event before the clock is construct-late. Ordinals count up from 0 in birth order. Owner D1
// (2026-11-05): its event AT the clock is kept and released only once the clock is past it.
func TestRuleClock_KeyBornLateStartsAtTheRuleClock(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k0 := rc.NewInstance()
	r := &recorder{t: t}
	to(rc, 500, 0)
	r.advance(rc)
	k1, k2 := rc.NewInstance(), rc.NewInstance()
	if k0.Ordinal() != 0 || k1.Ordinal() != 1 || k2.Ordinal() != 2 {
		t.Fatalf("ordinals %d %d %d, want 0 1 2", k0.Ordinal(), k1.Ordinal(), k2.Ordinal())
	}
	if !k1.Buffer(ev(0, 400, 1)) {
		t.Fatalf("a key born at clock 500: its event at 400 is not construct-late (it got its own clock from −∞)")
	}
	if k1.Buffer(ev(0, 500, 2)) {
		t.Fatalf("event at 500 = clock: construct-late, want kept (500 < 500 is false)")
	}
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("clock 500 delivered %v; D1: the event at 500 waits for the clock to pass 500", got)
	}
	to(rc, 501, 0)
	if got, want := r.advance(rc), []item{E(1, 0, 500, 2)}; !slices.Equal(got, want) {
		t.Fatalf("clock 501 delivered %v, want %v", got, want)
	}
}

// TestRuleClock_D1_NoSealedInstant (was TestRuleClock_D7_SealedInstant): owner D1 (2026-11-05) retires S3.1-1's D7
// seal. Construct-late is Ts < clock, nothing else: with the clock ON instant 100 and K0's Completion due 100
// pending, K0's events at 100 — before and after a walk at clock 100 — and K1's are all kept; once the clock passes 100 they are released in event.Compare order, BEFORE K0's timer at 100.
// Only then is an event at 100 construct-late, for every key alike (100 < 101).
func TestRuleClock_D1_NoSealedInstant(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	r := &recorder{t: t}
	k0.Schedule(100, clock.Completion, 1)
	to(rc, 100, 0)
	if k0.Buffer(ev(0, 100, 1)) {
		t.Fatalf("K0@100 at clock 100: construct-late, want kept")
	}
	// The clock is on 100: nothing at 100 is final — no event released, the Completion due 100 waits.
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("clock 100 delivered %v; D1: the instant 100 waits for the clock to pass it", got)
	}
	// After that walk K0's later event at 100 is still kept: no seal (100 < 100 is false).
	if k0.Buffer(ev(0, 100, 2)) {
		t.Fatalf("K0@100 after a walk at clock 100: construct-late, want kept (the seal is retired)")
	}
	if k1.Buffer(ev(0, 100, 3)) {
		t.Fatalf("K1@100 at clock 100: construct-late, want kept")
	}
	// Clock 101: the instant 100 — its events (K0 seq 1, K0 seq 2, K1 seq 3), then K0's inclusive timer.
	to(rc, 101, 0)
	want := []item{E(0, 0, 100, 1), E(0, 0, 100, 2), E(1, 0, 100, 3), T(0, 100, clock.Completion, 1)}
	if got := r.advance(rc); !slices.Equal(got, want) {
		t.Fatalf("clock 101 delivered %v, want %v", got, want)
	}
	// 100 < 101: construct-late for both keys, by the clock alone; 101 = clock is kept for both.
	if !k0.Buffer(ev(0, 100, 4)) || !k1.Buffer(ev(0, 100, 5)) {
		t.Fatalf("an event at 100 with the clock at 101 kept, want construct-late")
	}
	if k0.Buffer(ev(0, 101, 6)) || k1.Buffer(ev(0, 101, 7)) {
		t.Fatalf("an event at 101 = clock construct-late, want kept")
	}
}

// D1-1: a TTL purge due inside a jump fires before a later event of the same key; the operator's purge calls
// CancelAll, which leaves the key's later buffered event in place, released after it (and it starts the key fresh).
// TTL = 100; the key's last released activity is 0 → purge due 100. Owner D1 (2026-11-05): the event at 0 is
// released only once the clock is past 0.
func TestRuleClock_D1_1_PurgeInsideAJumpKeepsTheBuffer(t *testing.T) {
	t.Parallel()
	const ttl = 100
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	r := &recorder{t: t}
	r.onEvent = func(in *clock.Instance, e event.Event) { in.Schedule(e.Ts+ttl, clock.TTLPurge, 0) } // re-arm at release
	r.onTimer = func(in *clock.Instance, tm clock.Timer) {
		if tm.Kind == clock.TTLPurge {
			in.CancelAll()
			if tm.Due == ttl && in.Empty() {
				t.Errorf("Empty() inside the purge at 100 with the event at 150 still buffered; the shard would free it")
			}
		}
	}
	k.Buffer(ev(0, 0, 1))
	to(rc, 0, 0)
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("clock 0 delivered %v; D1: the event at 0 waits for the clock to pass 0", got)
	}
	to(rc, 1, 0)
	if got, want := r.advance(rc), []item{E(0, 0, 0, 1)}; !slices.Equal(got, want) { // 0 < 1: released, TTL due 100
		t.Fatalf("clock 1 delivered %v, want %v", got, want)
	}
	k.Schedule(300, clock.Completion, 9) // pending work the purge must cancel
	k.Buffer(ev(0, 150, 2))
	to(rc, 200, 0)                                                // one jump over the purge instant 100 and the event at 150
	want := []item{T(0, 100, clock.TTLPurge, 0), E(0, 0, 150, 2)} // 100 < 200: the purge; 150 < 200: the event (TTL → 250)
	if got := r.advance(rc); !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	to(rc, 400, 0) // the Completion at 300 was cancelled by the purge; the re-armed TTL 250 fires (250 < 400)
	if got, want := r.advance(rc), []item{T(0, 250, clock.TTLPurge, 0)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	if !k.Empty() {
		t.Fatalf("not Empty after the last purge with nothing buffered")
	}
	rc.Free(k)
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("after Free delivered %v", got)
	}
}

// D1's point (design §6 determinism test): the same events and timers fed as one big advance or as many small ones
// give the same callback sequence. The operator schedules from callbacks too, so joined timers are covered.
func TestRuleClock_D1_GranularityDoesNotChangeTheWalk(t *testing.T) {
	t.Parallel()
	run := func(stops []int64) []item {
		rc := clock.NewRuleClock([]int{0, 1})
		k0, k1 := rc.NewInstance(), rc.NewInstance()
		r := &recorder{t: t}
		r.onEvent = func(in *clock.Instance, e event.Event) {
			in.Schedule(e.Ts+30, clock.Completion, e.Seq) // inclusive, joins later
			in.Schedule(e.Ts+20, clock.Discard, e.Seq)    // strict
			in.Schedule(e.Ts+50, clock.TTLPurge, 0)       // strict, re-armed
		}
		r.onTimer = func(in *clock.Instance, tm clock.Timer) {
			if tm.Kind == clock.Completion {
				in.Schedule(tm.Due+5, clock.DelayDue, tm.Ref)
			}
		}
		for _, b := range []struct {
			in *clock.Instance
			e  event.Event
		}{
			{k0, ev(0, 0, 1)}, {k1, ev(1, 10, 1)}, {k0, ev(0, 10, 2)}, {k1, ev(0, 25, 3)}, {k0, ev(1, 30, 2)},
			{k1, ev(0, 30, 4)}, {k0, ev(0, 60, 5)}, {k1, ev(1, 100, 3)},
		} {
			if b.in.Buffer(b.e) {
				t.Fatalf("Buffer(%v) construct-late at −∞", keyOf(b.e))
			}
		}
		k1.Schedule(55, clock.CronClose, 77)
		for _, s := range stops {
			to(rc, s, 0, 1)
			r.advance(rc)
		}
		return r.items
	}
	big := run([]int64{1000})
	if len(big) < 20 {
		t.Fatalf("the scenario delivered only %d items: %v", len(big), big)
	}
	var everyMs []int64
	for s := int64(-5); s <= 1000; s++ {
		everyMs = append(everyMs, s)
	}
	for name, stops := range map[string][]int64{
		"every ms":  everyMs,
		"irregular": {0, 10, 20, 29, 30, 31, 50, 55, 59, 60, 61, 99, 100, 101, 250, 1000},
	} {
		if got := run(stops); !slices.Equal(got, big) {
			t.Errorf("%s: small advances delivered\n  %v\none advance delivered\n  %v", name, got, big)
		}
	}
}

// Owner D1 (2026-11-05), "when the walk runs never changes a result" (ADR-0010 §2; S3.2a-2's shard may walk per
// message or per chunk): one sequence of advances and Buffer calls, walked after every step or once at the end,
// gives the same callbacks and the same construct-late verdicts. Two keys on streams 0 and 1:
//
//	step 1: K0 schedules Completion due 100, K1 Discard due 150; K0 buffers e(0, 50, 1); clock → 100
//	step 2: K0 buffers e(1, 100, 1) — at the clock's instant, the instant of K0's pending Completion
//	step 3: K1 buffers e(0, 100, 2) — the same instant, another key
//	step 4: clock → 150
//	step 5: K1 buffers e(1, 150, 2) — at the clock's instant, the instant of K1's pending Discard; clock → 200
//
// Walked per step: 1 → E(K0 50) (50 < 100; 100 waits); 2, 3 → nothing (100 < 100 is false: kept, not released);
// 4 → instant 100: events by (Ts, StreamID, Seq) — e(0, 100, 2) of K1 before e(1, 100, 1) of K0 — then K0's
// Completion; 5 → instant 150: K1's event, then its Discard. Walked once: the same five instants in one walk. No
// Buffer call is construct-late in either run (each Ts ≥ the clock at its call).
func TestRuleClock_D1_WalkTimingNeverChangesAResult(t *testing.T) {
	t.Parallel()
	run := func(walkEachStep bool) ([]item, []bool) {
		rc := clock.NewRuleClock([]int{0, 1})
		k0, k1 := rc.NewInstance(), rc.NewInstance()
		r := &recorder{t: t}
		var late []bool
		walk := func() {
			if walkEachStep {
				r.advance(rc)
			}
		}
		k0.Schedule(100, clock.Completion, 1)
		k1.Schedule(150, clock.Discard, 7)
		late = append(late, k0.Buffer(ev(0, 50, 1)))
		to(rc, 100, 0, 1)
		walk()
		late = append(late, k0.Buffer(ev(1, 100, 1)))
		walk()
		late = append(late, k1.Buffer(ev(0, 100, 2)))
		walk()
		to(rc, 150, 0, 1)
		walk()
		late = append(late, k1.Buffer(ev(1, 150, 2)))
		to(rc, 200, 0, 1)
		r.advance(rc) // the last walk runs in both
		return r.items, late
	}
	want := []item{
		E(0, 0, 50, 1),
		E(1, 0, 100, 2), E(0, 1, 100, 1), T(0, 100, clock.Completion, 1),
		E(1, 1, 150, 2), D(1, 7),
	}
	wantLate := []bool{false, false, false, false}
	for _, each := range []bool{true, false} {
		got, late := run(each)
		if !slices.Equal(got, want) || !slices.Equal(late, wantLate) {
			t.Errorf("walk after each step = %v: delivered %v late %v, want %v late %v", each, got, late, want, wantLate)
		}
	}
}
