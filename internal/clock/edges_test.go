package clock_test

import (
	"testing"

	"github.com/vorsar/cep/internal/clock"
)

// All inputs idle: the clock does not move (executor run_pattern_multi: `if active:`), never jumping to +∞.
func TestEdge_AllInputsIdleLeavesTheClock(t *testing.T) {
	t.Parallel()
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S0"}, {Name: "S1"}})
	in := clock.NewInstance([]int{0, 1})
	apply := func(a clock.WMAdvance) (int64, bool) { return in.ApplyAdvance(a) }

	_, a0 := reg.ObserveEvent(0, 100)
	apply(a0)
	_, a1 := reg.ObserveEvent(1, 50)
	if c, _ := apply(a1); c != 50 {
		t.Fatalf("Clock = %d, want min(100, 50) = 50", c)
	}
	if c, moved := apply(reg.ObserveIdle(1)); c != 100 || !moved {
		t.Fatalf("S1 idle: ApplyAdvance = (%d, %v), want (100, true): min over {S0}", c, moved)
	}
	if c, moved := apply(reg.ObserveIdle(0)); c != 100 || moved {
		t.Fatalf("all idle: ApplyAdvance = (%d, %v), want (100, false): the clock stays", c, moved)
	}
	if got := in.Clock(); got != 100 {
		t.Fatalf("all idle: Clock = %d, want 100", got)
	}
	// S0 resumes at 200 (idle cleared, contrib 200); S1 still idle → min over {S0} = 200.
	_, a0 = reg.ObserveEvent(0, 200)
	if c, moved := apply(a0); c != 200 || !moved {
		t.Fatalf("S0 resumes: ApplyAdvance = (%d, %v), want (200, true)", c, moved)
	}

	// A fresh instance whose only input goes idle stays at −∞.
	reg2 := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S0"}})
	in2 := clock.NewInstance([]int{0})
	if c, moved := in2.ApplyAdvance(reg2.ObserveIdle(0)); c != negInf || moved {
		t.Fatalf("sole input idle: ApplyAdvance = (%d, %v), want (−∞, false)", c, moved)
	}
}

// An advance for a stream the instance does not read changes nothing.
func TestEdge_AdvanceForUnreadStreamIsIgnored(t *testing.T) {
	t.Parallel()
	in := clock.NewInstance([]int{1})
	if c, moved := in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 500, Moved: true}); c != negInf || moved {
		t.Fatalf("unread stream 0: ApplyAdvance = (%d, %v), want (−∞, false)", c, moved)
	}
	if c, moved := in.ApplyAdvance(clock.WMAdvance{StreamID: 1, Contribution: 300, Moved: true}); c != 300 || !moved {
		t.Fatalf("read stream 1: ApplyAdvance = (%d, %v), want (300, true)", c, moved)
	}
	if c, moved := in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 900, Moved: true}); c != 300 || moved {
		t.Fatalf("unread stream 0 again: ApplyAdvance = (%d, %v), want (300, false)", c, moved)
	}
}

// An advance with Moved false changes nothing — including the zero value, which is stream 0 at time 0 (design A1).
func TestEdge_MovedFalseIsNoAdvance(t *testing.T) {
	t.Parallel()
	in := clock.NewInstance([]int{0})
	if c, moved := in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 500}); c != negInf || moved {
		t.Fatalf("Moved false: ApplyAdvance = (%d, %v), want (−∞, false)", c, moved)
	}
	if c, moved := in.ApplyAdvance(clock.WMAdvance{}); c != negInf || moved {
		t.Fatalf("zero WMAdvance: ApplyAdvance = (%d, %v), want (−∞, false)", c, moved)
	}
	if got := in.Clock(); got != negInf {
		t.Fatalf("Clock = %d, want −∞", got)
	}
}

// ts − d_s and now − d_s saturate at math.MinInt64; a wrap would make the watermark huge and every later event
// stream-late.
func TestEdge_SubtractionNearMinInt64DoesNotWrap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		delay int64
		ts    int64
	}{
		{"1 s below the floor", 1000, negInf + 10}, // MinInt64 + 10 − 1000 would wrap to MaxInt64 − 989
		{"MinInt64 − 1", 1, negInf},                // would wrap to MaxInt64
		{"delay MaxInt64", posInf, -2},             // −2 − MaxInt64 would wrap to MaxInt64
	}
	for _, c := range cases {
		reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S", Delay: c.delay}})
		if late, _ := reg.ObserveEvent(0, c.ts); late {
			t.Fatalf("%s: the first event is stream-late", c.name)
		}
		if got := reg.Contribution(0); got != negInf {
			t.Fatalf("%s: Contribution = %d, want math.MinInt64 (saturated)", c.name, got)
		}
		if late, _ := reg.ObserveEvent(0, c.ts); late {
			t.Fatalf("%s: an equal second event is stream-late (the watermark wrapped)", c.name)
		}
		// The virtual floor: now − d_s saturates too.
		reg2 := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S", Delay: c.delay}})
		reg2.ObserveVirtual(c.ts)
		if got := reg2.Contribution(0); got != negInf {
			t.Fatalf("%s: virtual floor Contribution = %d, want math.MinInt64 (saturated)", c.name, got)
		}
	}
}

// Advance with nothing due calls no callback: a fresh instance, and one whose event, timer and discard all lie
// 1 ms past the clock. Nil callbacks are safe when nothing is called.
func TestEdge_AdvanceWithNothingDueCallsNothing(t *testing.T) {
	t.Parallel()
	r := newRecorder(t, 0)
	if calls := r.advance(); len(calls) != 0 {
		t.Fatalf("fresh instance: Advance called %v", phasesOf(calls))
	}
	r.in.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 100, Moved: true})
	if r.in.Buffer(ev(0, 101, 1)) {
		t.Fatalf("101 construct-late at clock 100")
	}
	r.in.Schedule(101, clock.Completion, 1)
	r.in.Schedule(101, clock.Discard, 2)
	if calls := r.advance(); len(calls) != 0 {
		t.Fatalf("nothing due at 100: Advance called %v", phasesOf(calls))
	}
	r.in.Advance(clock.AdvanceCallbacks{}) // nothing due: no callback, so nil ones are never touched
}
