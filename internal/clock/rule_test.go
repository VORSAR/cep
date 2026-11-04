package clock_test

// The per-rule clock (owner decisions D1, D2, D7; design v2 §2–§3; decisions review D1-1, D1-2, D2-2).

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// D2: one clock per rule, shared by its keys; a key born after the clock passed t starts at the rule's current
// time, so its event before the clock is construct-late. Ordinals count up from 0 in birth order.
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
		t.Fatalf("event at 500 = clock: construct-late, want kept")
	}
	if got, want := r.advance(rc), []item{E(1, 0, 500, 2)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
}

// D7 (sealed instant): once a key's timers at instant t have fired, a later Buffer of that key's event at t is
// construct-late; before they fired it is kept and released ahead of them; another key, or an instant where no
// timer of the key fired, is not sealed.
func TestRuleClock_D7_SealedInstant(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	r := &recorder{t: t}
	k0.Schedule(100, clock.Completion, 1)
	to(rc, 100, 0)
	// Before the walk: K0's event at 100 is kept, and released before K0's timer at 100.
	if k0.Buffer(ev(0, 100, 1)) {
		t.Fatalf("K0@100 before its timers fired: construct-late, want kept")
	}
	if got, want := r.advance(rc), []item{E(0, 0, 100, 1), T(0, 100, clock.Completion, 1)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	// K0's timers at 100 have fired: 100 ≤ sealed 100 → construct-late (100 < clock 100 is false: only the seal).
	if !k0.Buffer(ev(0, 100, 2)) {
		t.Fatalf("K0@100 after its timers at 100 fired: kept, want construct-late (sealed)")
	}
	// K1 had no timer at 100: not sealed.
	if k1.Buffer(ev(0, 100, 3)) {
		t.Fatalf("K1@100: construct-late, want kept (another key is not sealed)")
	}
	if got, want := r.advance(rc), []item{E(1, 0, 100, 3)}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	// Releasing K1's event fired no timer of K1: K1 is still not sealed at 100.
	if k1.Buffer(ev(0, 100, 4)) {
		t.Fatalf("K1@100 again: construct-late, want kept (no timer of K1 fired)")
	}
	r.advance(rc)
	// Clock 150, no K0 timer at 150: K0@150 is kept.
	to(rc, 150, 0)
	r.advance(rc)
	if k0.Buffer(ev(0, 150, 5)) {
		t.Fatalf("K0@150 with no K0 timer fired at 150: construct-late, want kept")
	}
}

// D1-1: a TTL purge due inside a jump fires before a later event of the same key; the operator's purge calls
// CancelAll, which leaves the key's later buffered event in place, released after it (and it starts the key fresh).
// TTL = 100; the key's last released activity is 0 → purge due 100 (strict).
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
	r.advance(rc)                        // releases 0: TTL due 100
	k.Schedule(300, clock.Completion, 9) // pending work the purge must cancel
	k.Buffer(ev(0, 150, 2))
	to(rc, 200, 0)                                                // one jump over the purge instant 100 and the event at 150
	want := []item{T(0, 100, clock.TTLPurge, 0), E(0, 0, 150, 2)} // 200 > 100: the purge, then the event (TTL → 250)
	if got := r.advance(rc); !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	to(rc, 400, 0) // the Completion at 300 was cancelled by the purge; the re-armed TTL 250 fires (400 > 250)
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
