// Package clock is the engine's one clock service (ADR-0004 as annotated at S3.1-1): the per-app stream registry,
// written by ingest or the playback driver, and the per-rule construct clock, shared by the rule's keys on one
// shard, with each key's release buffer and timers. RuleClock.Advance walks pending work forward in event time —
// the only place that order exists. All times are int64 milliseconds; no time.Time and no clock read appear here.
// Design: docs/sprints/s3-1-1-design.md (v2).
package clock

import (
	"math"

	"github.com/vorsar/cep/internal/event"
)

// StreamSpec describes one stream to the registry; its id is its index (SPEC §6.4's declaration order).
type StreamSpec struct {
	Name      string
	Delay     int64 // d_s, ms: the watermark bound
	IdleAfter int64 // ms; 0: the stream declares no idleness and never goes quiet
	Derived   bool  // a rule's output: punctuations only (v0.1 routes none)
}

// WMAdvance is one stream's new contribution to the rule clocks that read it. The zero value is no advance.
type WMAdvance struct {
	StreamID     int
	Contribution int64
	Moved        bool
}

// sub is a − b, saturating at the int64 bounds.
func sub(a, b int64) int64 {
	if b > 0 && a < math.MinInt64+b {
		return math.MinInt64
	}
	if b < 0 && a > math.MaxInt64+b {
		return math.MaxInt64
	}
	return a - b
}

// add is a + b, saturating at the int64 bounds.
func add(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

type stream struct {
	spec        StreamSpec
	maxTs       int64 // the largest event time admitted
	adm         int64 // admission watermark: maxTs − d
	lastTs      int64 // own last event time (the deployment's start before any event)
	lastArrival int64 // that event's intake stamp
	floor       int64 // live: the quiet floor
	vfloor      int64 // playback: the virtual floor
	punct       int64 // derived: the latest punctuation
	ended       bool
	contrib     int64
}

// StreamRegistry holds every stream's admission watermark, floors and contribution. Every quantity only rises, so a
// contribution never falls.
type StreamRegistry struct {
	streams []stream
}

// NewStreamRegistry registers the streams in declaration order; start is the deployment's start (wall ms).
func NewStreamRegistry(specs []StreamSpec, start int64) *StreamRegistry {
	r := &StreamRegistry{streams: make([]stream, len(specs))}
	for i, sp := range specs {
		r.streams[i] = stream{
			spec: sp, maxTs: math.MinInt64, adm: math.MinInt64, lastTs: start, lastArrival: start,
			floor: math.MinInt64, vfloor: math.MinInt64, punct: math.MinInt64, contrib: math.MinInt64,
		}
	}
	return r
}

// raise recomputes a stream's contribution and reports the advance when it rose.
func (r *StreamRegistry) raise(sid int) WMAdvance {
	s := &r.streams[sid]
	c := max(s.adm, s.floor, s.vfloor, s.punct)
	if s.ended {
		c = math.MaxInt64
	}
	if c <= s.contrib {
		return WMAdvance{}
	}
	s.contrib = c
	return WMAdvance{StreamID: sid, Contribution: c, Moved: true}
}

// ObserveEvent judges an event's stream lateness and moves its stream's watermark; arrival is its intake stamp.
// A stream-late event (ts < the admission watermark before it, strictly) moves nothing but its arrival time.
func (r *StreamRegistry) ObserveEvent(sid int, ts, arrival int64) (streamLate bool, advance WMAdvance) {
	s := &r.streams[sid]
	s.lastArrival = arrival
	if ts < s.adm {
		return true, WMAdvance{}
	}
	s.maxTs = max(s.maxTs, ts)
	s.lastTs = s.maxTs
	s.adm = max(s.adm, sub(s.maxTs, s.spec.Delay))
	return false, r.raise(sid)
}

// ObserveTick moves every quiet stream's floor to its own last event time plus the wall time since that event
// arrived, minus its bound (live; owner D3, D5). A stream is quiet once IdleAfter has elapsed since its last arrival.
func (r *StreamRegistry) ObserveTick(now int64, buf []WMAdvance) []WMAdvance {
	for sid := range r.streams {
		s := &r.streams[sid]
		if s.spec.Derived || s.spec.IdleAfter <= 0 {
			continue
		}
		elapsed := sub(now, s.lastArrival)
		if elapsed < s.spec.IdleAfter {
			continue
		}
		s.floor = max(s.floor, sub(add(s.lastTs, elapsed), s.spec.Delay))
		if a := r.raise(sid); a.Moved {
			buf = append(buf, a)
		}
	}
	return buf
}

// ObserveVirtual raises every source stream's virtual floor to now − d_s (playback; ADR-0004: every stream).
func (r *StreamRegistry) ObserveVirtual(now int64, buf []WMAdvance) []WMAdvance {
	for sid := range r.streams {
		s := &r.streams[sid]
		if s.spec.Derived {
			continue
		}
		s.vfloor = max(s.vfloor, sub(now, s.spec.Delay))
		if a := r.raise(sid); a.Moved {
			buf = append(buf, a)
		}
	}
	return buf
}

// ObservePunctuation records a derived stream's punctuation.
func (r *StreamRegistry) ObservePunctuation(sid int, w int64) WMAdvance {
	s := &r.streams[sid]
	s.punct = max(s.punct, w)
	return r.raise(sid)
}

// EndOfInput raises every contribution to +∞.
func (r *StreamRegistry) EndOfInput(buf []WMAdvance) []WMAdvance {
	for sid := range r.streams {
		r.streams[sid].ended = true
		if a := r.raise(sid); a.Moved {
			buf = append(buf, a)
		}
	}
	return buf
}

// Contribution is a stream's current contribution.
func (r *StreamRegistry) Contribution(sid int) int64 { return r.streams[sid].contrib }

// Drain raises every stream's contribution to one past the largest event time admitted on any of the app's streams
// (ADR-0012 §3's Stop under owner D1 of S3.2a-1: that instant is complete once the sources are stopped, so it
// finalizes; any deadline after it stays unfired). Nothing admitted: no advance.
func (r *StreamRegistry) Drain(buf []WMAdvance) []WMAdvance {
	panic("clock: StreamRegistry.Drain: not implemented (S3.2a-1 stub)")
}

// Walk yields each stream's state, component "stream" and key its name: the registry is a state holder
// (ADR-0006 §2(b); design note s3-2a-1 §4).
func (r *StreamRegistry) Walk(fn func(component, key string, state any)) {
	panic("clock: StreamRegistry.Walk: not implemented (S3.2a-1 stub)")
}

// TimerKind names a timer; for one key at one instant the inclusive kinds fire in this order.
type TimerKind uint8

// The timer kinds. TTLPurge and Discard are strict: they fire once the clock is past their due.
const (
	Completion TimerKind = iota + 1
	CronClose
	DelayDue
	BatchClose
	TTLPurge
	Discard
)

func (k TimerKind) strict() bool { return k == TTLPurge || k == Discard }

// Timer is one timer that fired.
type Timer struct {
	Due  int64
	Kind TimerKind
	Ref  uint64
}

// Callbacks receive the walk's items in order. A nil callback ignores its items.
type Callbacks struct {
	OnEvent   func(in *Instance, e event.Event)
	OnTimers  func(in *Instance, fires []Timer)
	OnDiscard func(in *Instance, refs []uint64)
}
