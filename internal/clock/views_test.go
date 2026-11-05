package clock_test

// The clock's views and the replacement fix (design note s3-2a-1 §4): Instance.Timers in walk order (timer.before:
// due, phase, key birth, kind, ref, schedule order — within one key only due, phase, kind and ref can differ),
// Instance.Events in event.Compare order, StreamRegistry.Walk, and a replacing Schedule that allocates nothing.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// K0 (ordinal 0) and K1 (ordinal 1); K1 schedules first, so K1 is born first. K0's timers, scheduled scrambled:
//
//	(100 Discard 5) (100 Completion 9) (100 TTLPurge 0) (50 DelayDue 1) (100 Completion 3) (100 BatchClose 1)
//	(200 Completion 1)
//
// Walk order for K0: due 50 first; at 100 the inclusive phase by kind then ref — Completion 3, Completion 9,
// BatchClose 1 — then the strict phase by kind — TTLPurge (kind 5) before Discard (kind 6); then due 200.
// K1's: (75 Discard 4), (100 Completion 2). Timers appends after dst's contents.
func TestInstanceTimersInWalkOrder(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	k1.Schedule(100, clock.Completion, 2)
	k1.Schedule(75, clock.Discard, 4)
	k0.Schedule(100, clock.Discard, 5)
	k0.Schedule(100, clock.Completion, 9)
	k0.Schedule(100, clock.TTLPurge, 0)
	k0.Schedule(50, clock.DelayDue, 1)
	k0.Schedule(100, clock.Completion, 3)
	k0.Schedule(100, clock.BatchClose, 1)
	k0.Schedule(200, clock.Completion, 1)
	sentinel := clock.Timer{Due: -1, Kind: clock.Completion, Ref: 99}
	want0 := []clock.Timer{sentinel,
		{Due: 50, Kind: clock.DelayDue, Ref: 1},
		{Due: 100, Kind: clock.Completion, Ref: 3},
		{Due: 100, Kind: clock.Completion, Ref: 9},
		{Due: 100, Kind: clock.BatchClose, Ref: 1},
		{Due: 100, Kind: clock.TTLPurge, Ref: 0},
		{Due: 100, Kind: clock.Discard, Ref: 5},
		{Due: 200, Kind: clock.Completion, Ref: 1},
	}
	got0 := k0.Timers([]clock.Timer{sentinel})
	if !slices.Equal(got0, want0) {
		t.Fatalf("K0.Timers\n got %+v\nwant %+v", got0, want0)
	}
	want1 := []clock.Timer{{Due: 75, Kind: clock.Discard, Ref: 4}, {Due: 100, Kind: clock.Completion, Ref: 2}}
	if got1 := k1.Timers(nil); !slices.Equal(got1, want1) {
		t.Fatalf("K1.Timers %+v, want %+v", got1, want1)
	}
	// The view is the walk's order: walk everything (clock 300) and keep K0's items.
	r := &recorder{t: t}
	to(rc, 300, 0)
	var walked []clock.Timer
	for _, it := range r.advance(rc) {
		switch {
		case it.Key != 0:
		case it.Cb == "timer":
			walked = append(walked, it.Timer)
		case it.Cb == "discard":
			walked = append(walked, clock.Timer{Due: 100, Kind: clock.Discard, Ref: it.Ref}) // K0's one discard is due 100
		}
	}
	if !slices.Equal(walked, want0[1:]) {
		t.Errorf("K0's walk %+v, want its Timers view %+v", walked, want0[1:])
	}
	if got := k0.Timers(nil); len(got) != 0 {
		t.Errorf("K0.Timers after the walk %+v, want none", got)
	}
}

// Events view: two keys' events buffered interleaved (clock −∞: none construct-late). event.Compare is (Ts, StreamID,
// Seq):
//
//	K0 buffered (1,100,1) (0,100,3) (0,50,1) (0,100,2)  → (0,50,1) (0,100,2) (0,100,3) (1,100,1)
//	K1 buffered (0,75,4) (1,100,2) (0,20,5)             → (0,20,5) (0,75,4) (1,100,2)
//
// After a walk to 60 releases (0,20,5) and (0,50,1), each view holds the rest.
func TestInstanceEventsInCompareOrder(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0, 1})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	for _, b := range []struct {
		in *clock.Instance
		e  event.Event
	}{
		{k0, ev(1, 100, 1)}, {k1, ev(0, 75, 4)}, {k0, ev(0, 100, 3)}, {k1, ev(1, 100, 2)}, {k0, ev(0, 50, 1)},
		{k1, ev(0, 20, 5)}, {k0, ev(0, 100, 2)},
	} {
		if b.in.Buffer(b.e) {
			t.Fatalf("Buffer(%v) construct-late at clock −∞", keyOf(b.e))
		}
	}
	keys := func(es []event.Event) []evKey {
		out := make([]evKey, len(es))
		for i, e := range es {
			out[i] = keyOf(e)
		}
		return out
	}
	head := ev(9, -1, 9)
	got0 := k0.Events([]event.Event{head})
	want0 := []evKey{keyOf(head), {0, 50, 1}, {0, 100, 2}, {0, 100, 3}, {1, 100, 1}}
	if !slices.Equal(keys(got0), want0) {
		t.Fatalf("K0.Events %v, want %v", keys(got0), want0)
	}
	if got1, want1 := keys(k1.Events(nil)), []evKey{{0, 20, 5}, {0, 75, 4}, {1, 100, 2}}; !slices.Equal(got1, want1) {
		t.Fatalf("K1.Events %v, want %v", got1, want1)
	}
	r := &recorder{t: t}
	to(rc, 60, 0, 1)
	r.advance(rc)
	if got, want := keys(k0.Events(nil)), []evKey{{0, 100, 2}, {0, 100, 3}, {1, 100, 1}}; !slices.Equal(got, want) {
		t.Errorf("K0.Events after the walk to 60: %v, want %v", got, want)
	}
	if got, want := keys(k1.Events(nil)), []evKey{{0, 75, 4}, {1, 100, 2}}; !slices.Equal(got, want) {
		t.Errorf("K1.Events after the walk to 60: %v, want %v", got, want)
	}
}

// A replacing Schedule — the TTL re-arm on every released event (ADR-0004 §5 as annotated) — MUST NOT allocate
// (design §4; ADR-0010 §1). The heap stays right: after 1,001 re-arms the key holds one TTL timer at the last due.
func TestScheduleReplacementAllocatesNothing(t *testing.T) {
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	other := rc.NewInstance()
	other.Schedule(5_000, clock.Completion, 1) // a second timer in the heap, so the fix-up has work
	due := int64(1_000)
	k.Schedule(due, clock.TTLPurge, 0)
	k.Schedule(due, clock.Completion, 7)
	if got := testing.AllocsPerRun(1000, func() {
		due++
		k.Schedule(due, clock.TTLPurge, 0)
	}); got != 0 {
		t.Errorf("TTLPurge re-arm: %v allocations, want 0", got)
	}
	if got := testing.AllocsPerRun(1000, func() {
		due++
		k.Schedule(due, clock.Completion, 7)
	}); got != 0 {
		t.Errorf("Completion (ref 7) replacement: %v allocations, want 0", got)
	}
	// AllocsPerRun runs f 1 + 1000 times: the TTL's last due is 1000 + 1001 = 2001, the Completion's 2001 + 1001 = 3002.
	want := []clock.Timer{{Due: 2_001, Kind: clock.TTLPurge}, {Due: 3_002, Kind: clock.Completion, Ref: 7}}
	if got := k.Timers(nil); !slices.Equal(got, want) {
		t.Errorf("after the replacements: %+v, want %+v", got, want)
	}
}

// A replacement is ordered as a fresh schedule at its new due: Completion 1 at 100 and Completion 2 at 200, then
// Completion 1 moved to 300 → view [(200 C 2), (300 C 1)]; the walk to 150 fires nothing (the old due is gone), the
// walk to 300 fires C 2 then C 1.
func TestScheduleReplacementKeepsWalkOrder(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	k.Schedule(100, clock.Completion, 1)
	k.Schedule(200, clock.Completion, 2)
	k.Schedule(300, clock.Completion, 1)
	want := []clock.Timer{{Due: 200, Kind: clock.Completion, Ref: 2}, {Due: 300, Kind: clock.Completion, Ref: 1}}
	if got := k.Timers(nil); !slices.Equal(got, want) {
		t.Fatalf("Timers %+v, want %+v", got, want)
	}
	r := &recorder{t: t}
	to(rc, 150, 0)
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("walk to 150 fired %v: the replaced due 100 must be gone", got)
	}
	to(rc, 300, 0)
	if got, w := r.advance(rc), []item{T(0, 200, clock.Completion, 2), T(0, 300, clock.Completion, 1)}; !slices.Equal(got, w) {
		t.Errorf("walk to 300: %v, want %v", got, w)
	}
}

// StreamRegistry.Walk yields each stream as ("stream", its name), in declaration order (design §4).
func TestStreamRegistryWalk(t *testing.T) {
	t.Parallel()
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "Zeta"}, {Name: "Alpha", Delay: minute}, {Name: "Mid"}}, 0)
	var got []string
	reg.Walk(func(component, key string, _ any) { got = append(got, component+"/"+key) })
	if want := []string{"stream/Zeta", "stream/Alpha", "stream/Mid"}; !slices.Equal(got, want) {
		t.Errorf("Walk %v, want %v", got, want)
	}
}
