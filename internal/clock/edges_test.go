package clock_test

import (
	"testing"

	"github.com/vorsar/cep/internal/clock"
)

// An advance for a stream the rule does not read changes nothing (v1 note §2; v2 is silent — see the report).
func TestEdge_AdvanceForUnreadStreamIsIgnored(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{1})
	if c, moved := rc.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 500, Moved: true}); c != negInf || moved {
		t.Fatalf("unread stream 0: ApplyAdvance = (%d, %v), want (−∞, false)", c, moved)
	}
	if c, moved := rc.ApplyAdvance(clock.WMAdvance{StreamID: 1, Contribution: 300, Moved: true}); c != 300 || !moved {
		t.Fatalf("read stream 1: ApplyAdvance = (%d, %v), want (300, true)", c, moved)
	}
	if c, moved := rc.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 900, Moved: true}); c != 300 || moved {
		t.Fatalf("unread stream 0 again: ApplyAdvance = (%d, %v), want (300, false)", c, moved)
	}
}

// Moved false is no advance — including the zero value, which is stream 0 at time 0.
func TestEdge_MovedFalseIsNoAdvance(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	if c, moved := rc.ApplyAdvance(clock.WMAdvance{StreamID: 0, Contribution: 500}); c != negInf || moved {
		t.Fatalf("Moved false: (%d, %v), want (−∞, false)", c, moved)
	}
	if c, moved := rc.ApplyAdvance(clock.WMAdvance{}); c != negInf || moved {
		t.Fatalf("zero WMAdvance: (%d, %v), want (−∞, false)", c, moved)
	}
}

// The clock never retreats, and the min over inputs holds it at the slowest stream.
func TestEdge_ClampedMinNeverRetreats(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0, 1})
	to(rc, 100, 0)
	if c, _ := rc.ApplyAdvance(clock.WMAdvance{StreamID: 1, Contribution: 50, Moved: true}); c != 50 {
		t.Fatalf("clock %d, want min(100, 50) = 50", c)
	}
	if c, moved := rc.ApplyAdvance(clock.WMAdvance{StreamID: 1, Contribution: 40, Moved: true}); c != 50 || moved {
		t.Fatalf("a lower contribution: (%d, %v), want (50, false)", c, moved)
	}
}

// Subtractions saturate at math.MinInt64 and additions at math.MaxInt64 (design §1); a wrap would make every later
// event stream-late, or a quiet floor negative.
func TestEdge_SaturatingArithmetic(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		delay, ts int64
	}{
		{"1 s below the floor", 1000, negInf + 10}, // MinInt64 + 10 − 1000 would wrap to MaxInt64 − 989
		{"MinInt64 − 1", 1, negInf},
		{"delay MaxInt64", posInf, -2}, // −2 − MaxInt64 would wrap
	} {
		reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S", Delay: c.delay}}, 0)
		if late, _ := reg.ObserveEvent(0, c.ts, 0); late {
			t.Fatalf("%s: the first event is stream-late", c.name)
		}
		if got := reg.Contribution(0); got != negInf {
			t.Fatalf("%s: Contribution = %d, want math.MinInt64", c.name, got)
		}
		if late, _ := reg.ObserveEvent(0, c.ts, 0); late {
			t.Fatalf("%s: an equal second event is stream-late (wrapped)", c.name)
		}
		reg2 := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S", Delay: c.delay}}, 0)
		reg2.ObserveVirtual(c.ts, nil)
		if got := reg2.Contribution(0); got != negInf {
			t.Fatalf("%s: virtual floor = %d, want math.MinInt64", c.name, got)
		}
	}
	// A quiet floor near the top: lastTs MaxInt64 − 10 + elapsed 1000 − d 0 saturates at MaxInt64, never wraps.
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S", IdleAfter: 1}}, 0)
	reg.ObserveEvent(0, posInf-10, 0)
	reg.ObserveTick(1000, nil)
	if got := reg.Contribution(0); got != posInf {
		t.Fatalf("quiet floor near MaxInt64 = %d, want math.MaxInt64 (saturated)", got)
	}
}

// Advance with nothing due calls no callback; nil callbacks are then never touched.
func TestEdge_AdvanceWithNothingDueCallsNothing(t *testing.T) {
	t.Parallel()
	rc := clock.NewRuleClock([]int{0})
	k := rc.NewInstance()
	r := &recorder{t: t}
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("fresh rule clock delivered %v", got)
	}
	to(rc, 100, 0)
	k.Buffer(ev(0, 101, 1))
	k.Schedule(101, clock.Completion, 1)
	k.Schedule(100, clock.Discard, 2) // strict: due = clock waits
	if got := r.advance(rc); len(got) != 0 {
		t.Fatalf("nothing due at 100 delivered %v", got)
	}
	rc.Advance(clock.Callbacks{})
}
