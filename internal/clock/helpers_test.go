package clock_test

import (
	"math"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// All times are int64 ms, as in the harness (SEC, MIN); 10:00 is 0. In live tests the wall clock is on the same
// scale: an arrival stamp of 0 is wall 10:00.
const (
	second int64 = 1000
	minute       = 60 * second
	negInf int64 = math.MinInt64 // −∞: a contribution before anything moved it, a fresh rule clock
	posInf int64 = math.MaxInt64 // +∞: every contribution after EndOfInput
)

// evKey is an event's identity: event.Compare's three keys.
type evKey struct {
	StreamID int
	Ts       int64
	Seq      uint64
}

func ev(sid int, ts int64, seq uint64) event.Event {
	return event.Event{StreamID: sid, Ts: ts, Seq: seq}
}

func keyOf(e event.Event) evKey { return evKey{e.StreamID, e.Ts, e.Seq} }

// item is one thing the walk delivered: an event, a fired timer or a discarded ref, with its key's ordinal.
type item struct {
	Cb    string // "event" (OnEvent), "timer" (OnTimers) or "discard" (OnDiscard)
	Key   uint64
	Ev    evKey
	Timer clock.Timer
	Ref   uint64
}

func E(key uint64, sid int, ts int64, seq uint64) item {
	return item{Cb: "event", Key: key, Ev: evKey{sid, ts, seq}}
}

func T(key uint64, due int64, k clock.TimerKind, ref uint64) item {
	return item{Cb: "timer", Key: key, Timer: clock.Timer{Due: due, Kind: k, Ref: ref}}
}

func D(key uint64, ref uint64) item { return item{Cb: "discard", Key: key, Ref: ref} }

// call is one callback invocation, kept to check batching (one OnTimers call per key's inclusive group).
type call struct {
	Cb     string
	Key    uint64
	Timers []clock.Timer
}

// recorder is the test-double operator for one rule clock: it records every item in order (copying each batch)
// and lets a test schedule or cancel from inside a callback, as a real operator does.
type recorder struct {
	t         *testing.T
	items     []item
	calls     []call
	onEvent   func(in *clock.Instance, e event.Event)
	onTimer   func(in *clock.Instance, tm clock.Timer)
	onDiscard func(in *clock.Instance, ref uint64)
}

func (r *recorder) callbacks() clock.Callbacks {
	return clock.Callbacks{
		OnEvent: func(in *clock.Instance, e event.Event) {
			r.items = append(r.items, item{Cb: "event", Key: in.Ordinal(), Ev: keyOf(e)})
			r.calls = append(r.calls, call{Cb: "event", Key: in.Ordinal()})
			if r.onEvent != nil {
				r.onEvent(in, e)
			}
		},
		OnTimers: func(in *clock.Instance, fires []clock.Timer) {
			if len(fires) == 0 {
				r.t.Errorf("OnTimers called with an empty batch")
			}
			r.calls = append(r.calls, call{Cb: "timer", Key: in.Ordinal(), Timers: append([]clock.Timer(nil), fires...)})
			for _, tm := range fires {
				if tm.Kind == clock.Discard {
					r.t.Errorf("OnTimers received a Discard %+v; discards arrive only through OnDiscard", tm)
				}
				r.items = append(r.items, item{Cb: "timer", Key: in.Ordinal(), Timer: tm})
			}
			if r.onTimer != nil {
				for _, tm := range fires {
					r.onTimer(in, tm)
				}
			}
		},
		OnDiscard: func(in *clock.Instance, refs []uint64) {
			if len(refs) == 0 {
				r.t.Errorf("OnDiscard called with an empty batch")
			}
			r.calls = append(r.calls, call{Cb: "discard", Key: in.Ordinal()})
			for _, ref := range refs {
				r.items = append(r.items, item{Cb: "discard", Key: in.Ordinal(), Ref: ref})
			}
			if r.onDiscard != nil {
				for _, ref := range refs {
					r.onDiscard(in, ref)
				}
			}
		},
	}
}

// advance runs one walk and returns the items it delivered.
func (r *recorder) advance(rc *clock.RuleClock) []item {
	start := len(r.items)
	rc.Advance(r.callbacks())
	return append([]item(nil), r.items[start:]...)
}

// to moves a rule clock reading the given streams to exactly t (each stream's contribution set to t).
func to(rc *clock.RuleClock, t int64, sids ...int) {
	for _, s := range sids {
		rc.ApplyAdvance(clock.WMAdvance{StreamID: s, Contribution: t, Moved: true})
	}
}

// world drives a registry and one rule clock the way a runtime does (ADR-0004 §3 as annotated): every WMAdvance
// the registry produces is applied to the rule clock; an arriving event is buffered into its key's instance unless
// stream-late; then the rule clock walks. In playback the driver also raises the virtual clock to the event's time.
type world struct {
	t        *testing.T
	playback bool
	reg      *clock.StreamRegistry
	rc       *clock.RuleClock
	rec      *recorder
	buf      []clock.WMAdvance
}

func newWorld(t *testing.T, playback bool, specs []clock.StreamSpec, start int64, inputs ...int) *world {
	t.Helper()
	return &world{t: t, playback: playback, reg: clock.NewStreamRegistry(specs, start), rc: clock.NewRuleClock(inputs),
		rec: &recorder{t: t}}
}

func (w *world) apply(advs []clock.WMAdvance) {
	for _, a := range advs {
		w.rc.ApplyAdvance(a)
	}
}

// arrive is one arrival: ObserveEvent → apply → (playback) ObserveVirtual → apply → Buffer → Advance.
func (w *world) arrive(in *clock.Instance, e event.Event, arrival int64) (streamLate, constructLate bool, got []item) {
	w.t.Helper()
	streamLate, adv := w.reg.ObserveEvent(e.StreamID, e.Ts, arrival)
	w.rc.ApplyAdvance(adv)
	if w.playback {
		w.buf = w.reg.ObserveVirtual(e.Ts, w.buf[:0])
		w.apply(w.buf)
	}
	if !streamLate {
		constructLate = in.Buffer(e)
	}
	return streamLate, constructLate, w.rec.advance(w.rc)
}

// tick is a live wall tick from the intake: ObserveTick → apply → Advance.
func (w *world) tick(now int64) []item {
	w.t.Helper()
	w.buf = w.reg.ObserveTick(now, w.buf[:0])
	w.apply(w.buf)
	return w.rec.advance(w.rc)
}
