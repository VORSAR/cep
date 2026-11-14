package clock

import "testing"

// fireGroupWorld is one rule clock with a group of timers due at one instant on one key, kept so a run can put the
// same timers back after fireGroup takes them (design s1-1c-5 §7.5, DQ-S12: fireGroup is on the per-event walk,
// runtime shard.tryAdvance -> partition Rule.Advance -> RuleClock.Advance).
type fireGroupWorld struct {
	rc     *RuleClock
	timers []*timer
	cb     Callbacks
	fired  int
}

func newFireGroupWorld(n int) *fireGroupWorld {
	w := &fireGroupWorld{rc: NewRuleClock([]int{0})}
	in := w.rc.NewInstance()
	for ref := range uint64(n) {
		in.Schedule(100, Completion, ref)
	}
	for _, t := range in.timers {
		w.timers = append(w.timers, t)
	}
	w.cb = Callbacks{OnTimers: func(_ *Instance, fires []Timer) { w.fired += len(fires) }}
	return w
}

// refill puts every timer back on its key and in the heap without allocating.
func (w *fireGroupWorld) refill() {
	for _, t := range w.timers {
		t.in.timers[keyOf(t.kind, t.ref)] = t
		w.rc.pushTimer(t)
	}
}

func (w *fireGroupWorld) fire() {
	w.rc.fireGroup(w.rc.timers[0], w.cb) // the heap's head, as Advance hands it over
}

// Not parallel: testing.AllocsPerRun refuses to run in a parallel test.
func TestFireGroupAllocs(t *testing.T) {
	w := newFireGroupWorld(8)
	w.fire() // grows the reused fires buffer once
	if w.fired != 8 || len(w.rc.timers) != 0 {
		t.Fatalf("the first fire took %d timers and left %d, want 8 and 0", w.fired, len(w.rc.timers))
	}
	// Pinned at today's value: fireGroup reuses rc.fires and rc.refs, so a warm fire allocates nothing.
	if got := testing.AllocsPerRun(100, func() { w.refill(); w.fire() }); got != 0 {
		t.Fatalf("fireGroup allocated %v times per run, want 0", got)
	}
	if w.fired != 8*102 {
		t.Fatalf("fired %d timers, want %d", w.fired, 8*102)
	}
}

func BenchmarkRuleClockFireGroup(b *testing.B) {
	w := newFireGroupWorld(8)
	w.fire() // empties the heap; each run puts the group back first
	b.ReportAllocs()
	for b.Loop() {
		w.refill()
		w.fire()
	}
}
