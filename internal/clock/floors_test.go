package clock_test

// The stream registry's floors (design v2 §1; owner decisions D3, D5, D6): a quiet stream's contribution runs from
// its own last event time at wall speed; a stream never seen runs from the deployment's start; contributions only
// rise; EndOfInput raises everything to +∞.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// D3 / D6 (fail-closed: "silence is silence, the alert fires on time"): every input quiet, the clock moves at wall
// speed and a due absence fires on time. Replaces v1's "all inputs idle, the clock stays".
// S0 (d 1 min, idle after 30 s) carries the anchor; S1 (d 0, idle after 30 s) never sends; start = 0. Owner D1
// (2026-11-05): the anchor is released, and the absence fires, only once the clock is past their instants.
func TestQuiet_AllInputsQuietClockMovesAtWallSpeedAbsenceFiresOnTime(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{
		{Name: "S0", Delay: 1 * minute, IdleAfter: 30 * second}, {Name: "S1", IdleAfter: 30 * second},
	}, 0, 0, 1)
	k := w.rc.NewInstance()
	w.rec.onEvent = func(in *clock.Instance, e event.Event) { in.Schedule(e.Ts+5*minute, clock.Completion, 1) }
	w.arrive(k, ev(0, 0, 1), 0) // adm S0 = −60000; S1 −∞ → clock −∞
	steps := []struct {
		now, s0, s1 int64 // contributions after the tick
		want        []item
	}{
		// 10000 − 0 < 30000: neither quiet; S0 = adm −60000, S1 −∞.
		{10 * second, -1 * minute, negInf, nil},
		// 30000 ≥ 30000: both quiet. S0 = 0 + 30000 − 60000 = −30000; S1 = 0 + 30000 − 0 = 30000. clock −30000.
		{30 * second, -30 * second, 30 * second, nil},
		// S0 = 60000 − 60000 = 0; S1 = 60000. clock 0 is ON the anchor's instant: not yet released (D1).
		{1 * minute, 0, 1 * minute, nil},
		// S0 = 60001 − 60000 = 1; S1 = 60001. clock 1 > 0: the anchor is released, Completion due 300000.
		{1*minute + 1, 1, 1*minute + 1, []item{E(0, 0, 0, 1)}},
		// S0 = 359999 − 60000 = 299999: short.
		{6*minute - 1, 5*minute - 1, 6*minute - 1, nil},
		// S0 = 360000 − 60000 = 300000: ON the deadline — an approval at 10:05 could still come (D1).
		{6 * minute, 5 * minute, 6 * minute, nil},
		// S0 = 360001 − 60000 = 300001 > 300000: fires on time — the first wall ms past deadline 10:05 + d.
		{6*minute + 1, 5*minute + 1, 6*minute + 1, []item{T(0, 5*minute, clock.Completion, 1)}},
	}
	for _, s := range steps {
		got := w.tick(s.now)
		if w.reg.Contribution(0) != s.s0 || w.reg.Contribution(1) != s.s1 {
			t.Fatalf("tick %d: contributions %d, %d, want %d, %d", s.now, w.reg.Contribution(0), w.reg.Contribution(1), s.s0, s.s1)
		}
		if w.rc.Clock() != min(s.s0, s.s1) { // both rise, so the clamp never holds it above the min
			t.Fatalf("tick %d: clock %d, want %d", s.now, w.rc.Clock(), min(s.s0, s.s1))
		}
		if !slices.Equal(got, s.want) {
			t.Fatalf("tick %d: delivered %v, want %v", s.now, got, s.want)
		}
	}
}

// D5: the floor runs from the stream's OWN last event time, not the wall minus d. A source that always delivers
// 2 s late (d = 0, idle after 1 min) goes quiet; after a tick at wall 10:10:02 an event stamped 10:10:00 arriving
// at 10:10:02 is NOT construct-late. (Under "wall − d" the floor would be 602000 and the event would be dropped.)
// Harness check "2.1 · quiet · a source that always delivers 2 s late"; owner D1 (2026-11-05): each release waits
// for the floor to pass the event's instant.
func TestQuiet_D5_LaggingSourceKeepsItsEvents(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S", IdleAfter: 1 * minute}}, 0, 0)
	k := w.rc.NewInstance()
	// ts 0 arrives at wall 2000: adm 0, clock 0 — ON the event's instant: kept, not released (D1).
	if sl, cl, got := w.arrive(k, ev(0, 0, 1), 2*second); sl || cl || len(got) != 0 {
		t.Fatalf("first event: late %v %v, delivered %v; want kept, not yet released", sl, cl, got)
	}
	// Tick at 602000: 602000 − 2000 = 600000 ≥ 60000 → quiet; floor = 0 + 600000 − 0 = 600000 (10:10:00).
	// 0 < 600000: the first event is released.
	if got, want := w.tick(10*minute+2*second), []item{E(0, 0, 0, 1)}; !slices.Equal(got, want) {
		t.Fatalf("tick 602000 delivered %v, want %v", got, want)
	}
	if got := w.reg.Contribution(0); got != 10*minute {
		t.Fatalf("quiet floor %d, want 600000 = last ts 0 + elapsed 600000 − d 0", got)
	}
	if got := w.rc.Clock(); got != 10*minute {
		t.Fatalf("clock %d, want 600000", got)
	}
	// ts 600000 arriving at 602000: adm before 0 → admitted; 600000 < 600000 is false → kept; the clock is ON
	// 600000 → not released yet (D1).
	sl, cl, got := w.arrive(k, ev(0, 10*minute, 2), 10*minute+2*second)
	if sl || cl {
		t.Fatalf("10:10:00 arriving at 10:10:02: streamLate=%v constructLate=%v, want neither", sl, cl)
	}
	if len(got) != 0 {
		t.Fatalf("clock 600000 delivered %v; D1: the event at 600000 waits for the floor to pass it", got)
	}
	// Quiet again: tick at 662000, 662000 − 602000 = 60000 ≥ 60000; floor = 600000 + 60000 − 0 = 660000 > 600000.
	if got, want := w.tick(11*minute+2*second), []item{E(0, 0, 10*minute, 2)}; !slices.Equal(got, want) {
		t.Fatalf("tick 662000 (clock %d) delivered %v, want %v", w.rc.Clock(), got, want)
	}
}

// D5: a stream with no event yet runs from the deployment's start (lastTs = lastArrival = start); IdleAfter 0
// never goes quiet; a quiet floor is monotone and stays once the stream speaks again.
func TestQuiet_NeverSeenStreamStartsFromStart_IdleAfterZeroNeverQuiet(t *testing.T) {
	t.Parallel()
	const start = 1_000_000
	reg := clock.NewStreamRegistry([]clock.StreamSpec{
		{Name: "S0", Delay: 1 * minute, IdleAfter: 30 * second}, {Name: "S1", Delay: 1 * minute},
	}, start)
	var buf []clock.WMAdvance
	// 1010000 − 1000000 = 10000 < 30000: not quiet, still −∞.
	buf = reg.ObserveTick(start+10*second, buf[:0])
	if got := reg.Contribution(0); got != negInf {
		t.Fatalf("before IdleAfter: %d, want −∞", got)
	}
	// 30000 ≥ 30000: floor = 1000000 + 30000 − 60000 = 970000.
	buf = reg.ObserveTick(start+30*second, buf[:0])
	if got := reg.Contribution(0); got != start-30*second {
		t.Fatalf("quiet from start: %d, want 970000", got)
	}
	// floor = 1000000 + 100000 − 60000 = 1040000.
	buf = reg.ObserveTick(start+100*second, buf[:0])
	if got := reg.Contribution(0); got != start+40*second {
		t.Fatalf("quiet from start: %d, want 1040000", got)
	}
	if got := reg.Contribution(1); got != negInf {
		t.Fatalf("IdleAfter 0 stream moved by a tick: %d, want −∞", got)
	}
	// S0 speaks: ts 500000 (arr start+100000): adm = 440000 < floor 1040000 → contribution stays 1040000 (monotone).
	if late, adv := reg.ObserveEvent(0, 500*second, start+100*second); late || adv.Moved {
		t.Fatalf("event under the floor: (%v, %+v), want admitted, no advance", late, adv)
	}
	if got := reg.Contribution(0); got != start+40*second {
		t.Fatalf("contribution fell to %d, want 1040000 (only rises)", got)
	}
	// Not quiet any more at start+110000 (110000 − 100000 = 10000 < 30000): nothing moves.
	buf = reg.ObserveTick(start+110*second, buf[:0])
	if got := reg.Contribution(0); got != start+40*second {
		t.Fatalf("tick after the event moved S0 to %d, want 1040000", got)
	}
	// Quiet again at start+130000 (30000 ≥ 30000): floor = max(1040000, 500000 + 30000 − 60000 = 470000) = 1040000.
	buf = reg.ObserveTick(start+130*second, buf[:0])
	if got := reg.Contribution(0); got != start+40*second {
		t.Fatalf("quiet floor fell to %d, want 1040000", got)
	}
	_ = buf
}

// ObserveTick, ObserveVirtual and EndOfInput append to the caller's buffer and return it (design §1).
func TestRegistry_AppendsToCallerBuffer(t *testing.T) {
	t.Parallel()
	sentinel := clock.WMAdvance{StreamID: 99, Contribution: 42, Moved: true}
	reg := clock.NewStreamRegistry([]clock.StreamSpec{{Name: "S0", IdleAfter: 1}, {Name: "S1"}}, 0)
	for name, f := range map[string]func([]clock.WMAdvance) []clock.WMAdvance{
		"ObserveTick":    func(b []clock.WMAdvance) []clock.WMAdvance { return reg.ObserveTick(10, b) },
		"ObserveVirtual": func(b []clock.WMAdvance) []clock.WMAdvance { return reg.ObserveVirtual(20, b) },
		"EndOfInput":     func(b []clock.WMAdvance) []clock.WMAdvance { return reg.EndOfInput(b) },
	} {
		got := f([]clock.WMAdvance{sentinel})
		if len(got) == 0 || got[0] != sentinel {
			t.Errorf("%s did not append to the caller's buffer: %v", name, got)
		}
	}
}

// End of input (SEMANTICS §2.1: wm → +∞, all open state fires/expires): every contribution rises to +∞, and the
// walk delivers everything left, strict timers included, in order.
func TestEndOfInput_EverythingFires(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false, []clock.StreamSpec{{Name: "S0", Delay: 1 * minute}, {Name: "S1"}}, 0, 0, 1)
	k := w.rc.NewInstance()
	w.arrive(k, ev(0, 1000, 1), 1000)
	k.Schedule(5000, clock.Completion, 1)
	k.Schedule(6000, clock.Discard, 2)
	k.Schedule(7000, clock.TTLPurge, 0)
	w.buf = w.reg.EndOfInput(w.buf[:0])
	w.apply(w.buf)
	if w.reg.Contribution(0) != posInf || w.reg.Contribution(1) != posInf || w.rc.Clock() != posInf {
		t.Fatalf("after EndOfInput: %d %d clock %d, want all +∞", w.reg.Contribution(0), w.reg.Contribution(1), w.rc.Clock())
	}
	want := []item{E(0, 0, 1000, 1), T(0, 5000, clock.Completion, 1), D(0, 2), T(0, 7000, clock.TTLPurge, 0)}
	if got := w.rec.advance(w.rc); !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
}
