package clock_test

// The clock's views, the replacement fix and the stop drain (design note s3-2a-1 v2 §4.2, §4.3), under owner D1
// (§4.1: a walk releases an event only when Ts < clock and fires a timer only when due < clock). Instance.Timers in
// walk order (timer.before: due, then within one instant the inclusive kinds, TTLPurge, Discard; within one key only
// due, kind and ref can differ); Instance.Events in event.Compare order; Instance.Birth; RuleClock.State;
// StreamRegistry.Walk and Drain; a replacing Schedule that allocates nothing.
//
// This file keeps its own small helpers (prefixed v) and uses none of the package's other test files.

import (
	"math"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

const vNegInf int64 = math.MinInt64

type vKey struct {
	StreamID int
	Ts       int64
	Seq      uint64
}

func vEv(sid int, ts int64, seq uint64) event.Event {
	return event.Event{StreamID: sid, Ts: ts, Seq: seq}
}

func vKeys(es []event.Event) []vKey {
	out := make([]vKey, len(es))
	for i, e := range es {
		out[i] = vKey{e.StreamID, e.Ts, e.Seq}
	}
	return out
}

// vFired is one timer or discard the walk delivered, with its key's ordinal.
type vFired struct {
	Key   uint64
	Timer clock.Timer
}

// vWalk moves a rule clock reading the given streams to c and walks it, returning the timers and discards it
// delivered (a discard as a Discard timer due 0: OnDiscard carries no due) and the events it released.
func vWalk(rc *clock.RuleClock, c int64, sids ...int) (fired []vFired, released []vKey) {
	for _, s := range sids {
		rc.ApplyAdvance(clock.WMAdvance{StreamID: s, Contribution: c, Moved: true})
	}
	rc.Advance(clock.Callbacks{
		OnEvent: func(in *clock.Instance, e event.Event) { released = append(released, vKey{e.StreamID, e.Ts, e.Seq}) },
		OnTimers: func(in *clock.Instance, fs []clock.Timer) {
			for _, f := range fs {
				fired = append(fired, vFired{in.Ordinal(), f})
			}
		},
		OnDiscard: func(in *clock.Instance, refs []uint64) {
			for _, r := range refs {
				fired = append(fired, vFired{in.Ordinal(), clock.Timer{Kind: clock.Discard, Ref: r}})
			}
		},
	})
	return fired, released
}

// K0 (ordinal 0) and K1 (ordinal 1); K1 schedules first. K0's timers, scheduled scrambled:
//
//	(100 Discard 5) (100 Completion 9) (100 TTLPurge 0) (50 DelayDue 1) (100 Completion 3) (100 BatchClose 1)
//	(200 Completion 1)
//
// Walk order for K0: due 50; at 100 the inclusive kinds by kind then ref — Completion 3, Completion 9, BatchClose 1
// — then TTLPurge, then Discard; then due 200. K1's: (75 Discard 4), (100 Completion 2). Timers appends after dst.
// A walk to 300 fires all (each due < 300, D1), and K0's share of it is its view, in order.
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
	fired, _ := vWalk(rc, 300, 0)
	var walked []clock.Timer
	for _, f := range fired {
		if f.Key == 0 {
			if f.Timer.Kind == clock.Discard {
				f.Timer.Due = 100 // K0's one discard is due 100
			}
			walked = append(walked, f.Timer)
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
// A walk to 60 releases what is below 60 (D1): (0,20,5) and (0,50,1); each view holds the rest.
func TestInstanceEventsInCompareOrder(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0, 1})
	k0, k1 := rc.NewInstance(), rc.NewInstance()
	for _, b := range []struct {
		in *clock.Instance
		e  event.Event
	}{
		{k0, vEv(1, 100, 1)}, {k1, vEv(0, 75, 4)}, {k0, vEv(0, 100, 3)}, {k1, vEv(1, 100, 2)}, {k0, vEv(0, 50, 1)},
		{k1, vEv(0, 20, 5)}, {k0, vEv(0, 100, 2)},
	} {
		if b.in.Buffer(b.e) {
			t.Fatalf("Buffer(%+v) construct-late at clock −∞", b.e)
		}
	}
	head := vEv(9, -1, 9)
	got0 := vKeys(k0.Events([]event.Event{head}))
	want0 := []vKey{{9, -1, 9}, {0, 50, 1}, {0, 100, 2}, {0, 100, 3}, {1, 100, 1}}
	if !slices.Equal(got0, want0) {
		t.Fatalf("K0.Events %v, want %v", got0, want0)
	}
	if got1, want1 := vKeys(k1.Events(nil)), []vKey{{0, 20, 5}, {0, 75, 4}, {1, 100, 2}}; !slices.Equal(got1, want1) {
		t.Fatalf("K1.Events %v, want %v", got1, want1)
	}
	_, rel := vWalk(rc, 60, 0, 1)
	if want := []vKey{{0, 20, 5}, {0, 50, 1}}; !slices.Equal(rel, want) {
		t.Fatalf("walk to 60 released %v, want %v", rel, want)
	}
	if got, want := vKeys(k0.Events(nil)), []vKey{{0, 100, 2}, {0, 100, 3}, {1, 100, 1}}; !slices.Equal(got, want) {
		t.Errorf("K0.Events after the walk to 60: %v, want %v", got, want)
	}
	if got, want := vKeys(k1.Events(nil)), []vKey{{0, 75, 4}, {1, 100, 2}}; !slices.Equal(got, want) {
		t.Errorf("K1.Events after the walk to 60: %v, want %v", got, want)
	}
}

// Birth (§4.2): unborn before the first released event or timer; a key's birth places it in the walk's order across
// keys; CancelAll unbears it, and its next timer bears it again, after every key born before.
//   - K0, K1 fresh: both unborn.
//   - K1 schedules first, then K0: K1 born before K0 (birth K1 < birth K0).
//   - K1.CancelAll: unborn. K1 schedules again: born, now after K0 (birth K1 > birth K0).
//   - K2's first event, released by a walk to 11 (10 < 11): born, after K1.
func TestInstanceBirth(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k0, k1, k2 := rc.NewInstance(), rc.NewInstance(), rc.NewInstance()
	for i, k := range []*clock.Instance{k0, k1, k2} {
		if _, born := k.Birth(); born {
			t.Fatalf("K%d born before anything", i)
		}
	}
	k1.Schedule(500, clock.Completion, 1)
	k0.Schedule(500, clock.Completion, 1)
	b0, born0 := k0.Birth()
	b1, born1 := k1.Birth()
	if !born0 || !born1 || !(b1 < b0) {
		t.Fatalf("after K1 then K0 scheduled: K0 (%d, %v), K1 (%d, %v); want both born, K1 first", b0, born0, b1, born1)
	}
	k1.CancelAll()
	if _, born := k1.Birth(); born {
		t.Fatal("K1 born after CancelAll")
	}
	k1.Schedule(600, clock.Completion, 2)
	if b1, born1 = k1.Birth(); !born1 || !(b1 > b0) {
		t.Fatalf("K1 reborn: (%d, %v); want born after K0 (%d)", b1, born1, b0)
	}
	if k2.Buffer(vEv(0, 10, 1)) {
		t.Fatal("K2's event construct-late at clock −∞")
	}
	if _, born := k2.Birth(); born {
		t.Fatal("K2 born by a buffered, unreleased event")
	}
	vWalk(rc, 11, 0)
	if b2, born2 := k2.Birth(); !born2 || !(b2 > b1) {
		t.Errorf("K2 after its release: (%d, %v); want born after K1 (%d)", b2, born2, b1)
	}
}

// RuleState (§4.2) is the rule clock's own state, copied. Inputs [2, 0]: contributions start at −∞, the clock at −∞.
// Stream 0 → 500: [−∞, 500], clock min = −∞. Stream 2 → 300: [300, 500], clock 300. Stream 2 → 200: unchanged (a
// contribution never falls). Changing the returned slices changes nothing.
func TestRuleClockState(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{2, 0})
	st := rc.State()
	if !slices.Equal(st.Inputs, []int{2, 0}) || !slices.Equal(st.Contributions, []int64{vNegInf, vNegInf}) || st.Clock != vNegInf {
		t.Fatalf("fresh State %+v, want Inputs [2 0], Contributions [−∞ −∞], Clock −∞", st)
	}
	rc.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 500, Moved: true})
	if st = rc.State(); !slices.Equal(st.Contributions, []int64{vNegInf, 500}) || st.Clock != vNegInf {
		t.Fatalf("after stream 0 → 500: %+v, want [−∞ 500], clock −∞", st)
	}
	rc.ApplyAdvance(clock.WMAdvance{StreamID: 2, Contribution: 300, Moved: true})
	rc.ApplyAdvance(clock.WMAdvance{StreamID: 2, Contribution: 200, Moved: true})
	st = rc.State()
	if !slices.Equal(st.Contributions, []int64{300, 500}) || st.Clock != 300 {
		t.Fatalf("after stream 2 → 300, → 200: %+v, want [300 500], clock 300", st)
	}
	st.Inputs[0], st.Contributions[0] = 99, 99
	if again := rc.State(); again.Inputs[0] != 2 || again.Contributions[0] != 300 {
		t.Errorf("State is not a copy: after changing the returned slices, State %+v", again)
	}
}

// A replacing Schedule — the TTL re-arm on every released event (ADR-0004 §5 as annotated) — MUST NOT allocate
// (§4.3; ADR-0010 §1). The heap stays right: afterwards the key holds one timer per (kind, ref) at the last due.
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

// A replacement is ordered as a fresh schedule at its new due (QA A5): Completion 1 at 100, Completion 2 at 200, then
// Completion 1 moved to 300 → view [(200 C 2), (300 C 1)]. A walk to 150 fires nothing (the old due 100 would
// have: 100 < 150); a walk to 301 fires C 2 then C 1 (200, 300 < 301).
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
	if fired, _ := vWalk(rc, 150, 0); len(fired) != 0 {
		t.Fatalf("walk to 150 fired %v: the replaced due 100 must be gone", fired)
	}
	fired, _ := vWalk(rc, 301, 0)
	if w := []vFired{{0, want[0]}, {0, want[1]}}; !slices.Equal(fired, w) {
		t.Errorf("walk to 301: %v, want %v", fired, w)
	}
}

// StreamRegistry.Walk yields each stream as ("stream", its name), in declaration order (§4.2).
func TestStreamRegistryWalk(t *testing.T) {
	t.Parallel()
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "Zeta"}, {Name: "Alpha", Delay: 60_000}, {Name: "Mid"}}, 0)
	var got []string
	reg.Walk(func(component, key string, _ any) { got = append(got, component+"/"+key) })
	if want := []string{"stream/Zeta", "stream/Alpha", "stream/Mid"}; !slices.Equal(got, want) {
		t.Errorf("Walk %v, want %v", got, want)
	}
}

// Drain (§4.3): every stream's contribution rises to one past the largest event time admitted on any stream.
// Streams A (d 1 min), B (d 0), C (d 0, no event). A admits 10:00 (600,000): A's contribution 600,000 − 60,000 =
// 540,000. B admits 9:00 (540,000): 540,000. B's 8:00 (480,000) is stream-late (< 540,000): not admitted. The max
// admitted is 600,000 → target 600,001: A, B and C each advance to 600,001, in stream order, appended after buf.
// A second Drain moves nothing.
func TestStreamRegistryDrain(t *testing.T) {
	t.Parallel()
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "A", Delay: 60_000}, {Name: "B"}, {Name: "C"}}, 0)
	reg.ObserveEvent(0, 600_000, 0)
	reg.ObserveEvent(1, 540_000, 0)
	if late, _ := reg.ObserveEvent(1, 480_000, 0); !late {
		t.Fatal("B's 8:00 after its 9:00: want stream-late")
	}
	if a, b := reg.Contribution(0), reg.Contribution(1); a != 540_000 || b != 540_000 {
		t.Fatalf("contributions A %d, B %d; want 540000, 540000", a, b)
	}
	head := clock.WMAdvance{StreamID: 9, Contribution: 1, Moved: true}
	got := reg.Drain([]clock.WMAdvance{head})
	want := []clock.WMAdvance{head,
		{StreamID: 0, Contribution: 600_001, Moved: true},
		{StreamID: 1, Contribution: 600_001, Moved: true},
		{StreamID: 2, Contribution: 600_001, Moved: true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Drain %+v, want %+v", got, want)
	}
	for sid := range 3 {
		if c := reg.Contribution(sid); c != 600_001 {
			t.Errorf("stream %d contribution %d after Drain, want 600001", sid, c)
		}
	}
	if again := reg.Drain(nil); len(again) != 0 {
		t.Errorf("a second Drain %+v, want no advance", again)
	}
}

// Drain with nothing admitted: no advance, and every contribution stays −∞. A contribution already above the target
// is not lowered: A admits 100 (d 1 min → contribution −59,900); the virtual clock at 1,000 raises B (d 0) to 1,000
// and A to max(−59,900, 1,000 − 60,000) = −59,000. Drain's target is 101: A rises to 101; B stays at 1,000.
func TestStreamRegistryDrainNothingAdmittedAndNeverLowers(t *testing.T) {
	t.Parallel()
	empty := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "A"}, {Name: "B", Delay: 5}}, 0)
	if got := empty.Drain(nil); len(got) != 0 {
		t.Errorf("Drain with nothing admitted: %+v, want no advance", got)
	}
	if a, b := empty.Contribution(0), empty.Contribution(1); a != vNegInf || b != vNegInf {
		t.Errorf("contributions %d, %d after an empty Drain, want −∞", a, b)
	}
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "A", Delay: 60_000}, {Name: "B"}}, 0)
	reg.ObserveEvent(0, 100, 0)
	reg.ObserveVirtual(1_000, nil)
	if a, b := reg.Contribution(0), reg.Contribution(1); a != -59_000 || b != 1_000 {
		t.Fatalf("before Drain: A %d, B %d; want −59000, 1000", a, b)
	}
	if got, want := reg.Drain(nil), []clock.WMAdvance{{StreamID: 0, Contribution: 101, Moved: true}}; !slices.Equal(got, want) {
		t.Errorf("Drain %+v, want %+v", got, want)
	}
	if b := reg.Contribution(1); b != 1_000 {
		t.Errorf("B's contribution %d after Drain, want 1000 (never lowered)", b)
	}
}
