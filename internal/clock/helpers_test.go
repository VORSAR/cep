package clock_test

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// All times are int64 ms, as in the harness (SEC, MIN); 10:00 is 0.
const (
	second int64 = 1000
	minute       = 60 * second
	negInf int64 = math.MinInt64 // −∞: a contribution before any event, a fresh construct clock
	posInf int64 = math.MaxInt64 // +∞: an idle stream's contribution (design note §1)
)

// evKey is an event's identity for comparisons: event.Compare's three keys.
type evKey struct {
	StreamID int
	Ts       int64
	Seq      uint64
}

func ev(sid int, ts int64, seq uint64) event.Event {
	return event.Event{StreamID: sid, Ts: ts, Seq: seq}
}

func keyOf(e event.Event) evKey { return evKey{e.StreamID, e.Ts, e.Seq} }

// phase is one of Advance's three ordered steps (SEMANTICS §6.4; ADR-0004 §2).
type phase int

const (
	phaseEvents  phase = iota // (a) OnEvents
	phaseTimers               // (b) OnTimers
	phaseDiscard              // (c) OnDiscard
)

func (p phase) String() string {
	return [...]string{"(a) OnEvents", "(b) OnTimers", "(c) OnDiscard"}[p]
}

// call is one callback the test-double operator received.
type call struct {
	phase  phase
	events []evKey
	timers []clock.Timer
	refs   []uint64
}

// recorder is the test-double operator: it owns one instance, records every callback in order (copying each
// batch), and lets a test schedule or cancel from inside a callback, as a real operator does.
type recorder struct {
	t       *testing.T
	in      *clock.Instance
	calls   []call
	onEvent func(in *clock.Instance, e event.Event)
	onTimer func(in *clock.Instance, tm clock.Timer)
}

func newRecorder(t *testing.T, inputs ...int) *recorder {
	t.Helper()
	return &recorder{t: t, in: clock.NewInstance(inputs)}
}

func (r *recorder) callbacks() clock.AdvanceCallbacks {
	return clock.AdvanceCallbacks{
		OnEvents: func(batch []event.Event) {
			if len(batch) == 0 {
				r.t.Errorf("OnEvents called with an empty batch; a callback with nothing to receive is not called")
			}
			c := call{phase: phaseEvents}
			for _, e := range batch {
				c.events = append(c.events, keyOf(e))
			}
			r.calls = append(r.calls, c)
			if r.onEvent != nil {
				for _, e := range batch {
					r.onEvent(r.in, e)
				}
			}
		},
		OnTimers: func(fires []clock.Timer) {
			if len(fires) == 0 {
				r.t.Errorf("OnTimers called with an empty batch; a callback with nothing to receive is not called")
			}
			for _, tm := range fires {
				if tm.Kind == clock.Discard {
					r.t.Errorf("OnTimers received a Discard timer %+v; discards arrive only in OnDiscard (c)", tm)
				}
			}
			r.calls = append(r.calls, call{phase: phaseTimers, timers: append([]clock.Timer(nil), fires...)})
			if r.onTimer != nil {
				for _, tm := range fires {
					r.onTimer(r.in, tm)
				}
			}
		},
		OnDiscard: func(overrun []uint64) {
			if len(overrun) == 0 {
				r.t.Errorf("OnDiscard called with an empty batch; a callback with nothing to receive is not called")
			}
			r.calls = append(r.calls, call{phase: phaseDiscard, refs: append([]uint64(nil), overrun...)})
		},
	}
}

// advance runs one Advance and returns the calls it made, after checking the (a) → (b) → (c) order:
// phases never go backwards and (a) is at most one chunk (ADR-0004 §2: "as ONE chunk").
func (r *recorder) advance() []call {
	r.t.Helper()
	start := len(r.calls)
	r.in.Advance(r.callbacks())
	got := r.calls[start:]
	nEvents := 0
	for i, c := range got {
		if c.phase == phaseEvents {
			nEvents++
		}
		if i > 0 && c.phase < got[i-1].phase {
			r.t.Errorf("Advance called %v after %v; the order is (a) events, (b) timers, (c) discards: %v",
				c.phase, got[i-1].phase, phasesOf(got))
		}
	}
	if nEvents > 1 {
		r.t.Errorf("Advance called OnEvents %d times; (a) is one chunk", nEvents)
	}
	return got
}

func phasesOf(calls []call) []phase {
	var ps []phase
	for _, c := range calls {
		ps = append(ps, c.phase)
	}
	return ps
}

func eventsOf(calls []call) []evKey {
	var out []evKey
	for _, c := range calls {
		out = append(out, c.events...)
	}
	return out
}

func timersOf(calls []call) []clock.Timer {
	var out []clock.Timer
	for _, c := range calls {
		out = append(out, c.timers...)
	}
	return out
}

func discardsOf(calls []call) []uint64 {
	var out []uint64
	for _, c := range calls {
		out = append(out, c.refs...)
	}
	return out
}

// world drives a registry and instances the way a runtime does (ADR-0004 §3): every WMAdvance the registry
// produces is applied to EVERY instance (the broadcast; an instance ignores a stream it does not read), the
// arriving event is buffered into its owner instance unless stream-late, then every instance runs Advance.
// In playback the driver also raises the virtual clock to the event's time (run_pattern_multi's virtual_now).
type world struct {
	t        *testing.T
	playback bool
	reg      *clock.StreamRegistry
	recs     []*recorder
}

func newWorld(t *testing.T, playback bool, specs []clock.StreamSpec, inputs ...[]int) *world {
	t.Helper()
	w := &world{t: t, playback: playback, reg: clock.NewStreamRegistry(specs)}
	for _, in := range inputs {
		w.recs = append(w.recs, newRecorder(t, in...))
	}
	return w
}

func (w *world) broadcast(a clock.WMAdvance) {
	for _, r := range w.recs {
		r.in.ApplyAdvance(a)
	}
}

// arrive is one arrival: ObserveEvent → broadcast → (playback) ObserveVirtual → broadcast → Buffer → Advance.
func (w *world) arrive(owner int, e event.Event) (streamLate, constructLate bool, calls [][]call) {
	w.t.Helper()
	streamLate, adv := w.reg.ObserveEvent(e.StreamID, e.Ts)
	w.broadcast(adv)
	if w.playback {
		for _, a := range w.reg.ObserveVirtual(e.Ts) {
			w.broadcast(a)
		}
	}
	if !streamLate {
		constructLate = w.recs[owner].in.Buffer(e)
	}
	return streamLate, constructLate, w.advanceAll()
}

// idle is a live idle tick: the stream's `idle after` elapsed.
func (w *world) idle(sid int) [][]call {
	w.t.Helper()
	w.broadcast(w.reg.ObserveIdle(sid))
	return w.advanceAll()
}

func (w *world) advanceAll() [][]call {
	w.t.Helper()
	out := make([][]call, len(w.recs))
	for i, r := range w.recs {
		out[i] = r.advance()
	}
	return out
}
