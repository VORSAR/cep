// Package clock is the engine's one clock service (ADR-0004): the per-stream watermark registry, written by ingest
// or the playback driver, and the per-instance construct clock with its release buffer and timers, used by the
// owning shard. Advance is the only place the order events, then timers, then discards exists (SEMANTICS §6.4).
// All times are int64 milliseconds, UTC; no time.Time appears here. Design: docs/sprints/s3-1-1-design.md.
//
// STUB (S3.1-1, before G1): the API QA's tests are written against; every body panics until the build.
package clock

import "github.com/vorsar/cep/internal/event"

const notBuilt = "clock: S3.1-1 not built yet"

// StreamSpec describes one stream to the registry; its id is its index (SPEC §6.4's declaration order).
type StreamSpec struct {
	Name    string
	Delay   int64 // d_s, ms: the watermark bound (0 for ascending and arrival streams)
	Derived bool  // a rule's output: punctuations only (v0.1 routes none; ADR-0004 §6)
}

// WMAdvance is one stream's new contribution to the construct clocks that read it. The zero value is no advance.
type WMAdvance struct {
	StreamID     int
	Contribution int64
	Moved        bool
}

// StreamRegistry holds every stream's admission watermark and contribution.
type StreamRegistry struct{}

// NewStreamRegistry registers the streams in declaration order.
func NewStreamRegistry(specs []StreamSpec) *StreamRegistry { panic(notBuilt) }

// ObserveEvent judges an event's stream lateness and moves its stream's watermark.
func (r *StreamRegistry) ObserveEvent(sid int, ts int64) (streamLate bool, advance WMAdvance) {
	panic(notBuilt)
}

// ObserveIdle marks a stream idle (live): it leaves every min until its next event.
func (r *StreamRegistry) ObserveIdle(sid int) WMAdvance { panic(notBuilt) }

// ObservePunctuation records a derived stream's punctuation.
func (r *StreamRegistry) ObservePunctuation(sid int, w int64) WMAdvance { panic(notBuilt) }

// ObserveVirtual raises every source stream's virtual floor to now − d_s (playback).
func (r *StreamRegistry) ObserveVirtual(now int64) []WMAdvance { panic(notBuilt) }

// Contribution is a stream's current contribution.
func (r *StreamRegistry) Contribution(sid int) int64 { panic(notBuilt) }

// TimerKind names a timer; the order of the constants is (b)'s order for timers due at one instant.
type TimerKind uint8

// The timer kinds.
const (
	Completion TimerKind = iota + 1
	AbsenceResolution
	CronClose
	DelayDue
	TTLPurge
	Discard // a partial's within deadline: OnDiscard in (c), never OnTimers
)

// Timer is one timer that fired.
type Timer struct {
	Due  int64
	Kind TimerKind
	Ref  uint64
}

// AdvanceCallbacks receive one Advance's three steps, in order.
type AdvanceCallbacks struct {
	OnEvents  func(batch []event.Event)
	OnTimers  func(fires []Timer)
	OnDiscard func(overrun []uint64)
}

// Instance is one rule × key's construct clock, release buffer and timers.
type Instance struct{}

// NewInstance makes an instance reading the given streams.
func NewInstance(inputs []int) *Instance { panic(notBuilt) }

// Clock is the construct clock.
func (in *Instance) Clock() int64 { panic(notBuilt) }

// Buffer holds an admitted event until its release, or reports it construct-late.
func (in *Instance) Buffer(e event.Event) (constructLate bool) { panic(notBuilt) }

// ApplyAdvance takes a stream's new contribution; the clock never retreats.
func (in *Instance) ApplyAdvance(a WMAdvance) (newClock int64, advanced bool) { panic(notBuilt) }

// Advance runs the one ordered sequence at the current clock.
func (in *Instance) Advance(cb AdvanceCallbacks) { panic(notBuilt) }

// Schedule adds a timer; a TTLPurge replaces the instance's one TTL timer.
func (in *Instance) Schedule(due int64, k TimerKind, ref uint64) { panic(notBuilt) }

// Cancel removes a pending timer; false when there was none.
func (in *Instance) Cancel(k TimerKind, ref uint64) bool { panic(notBuilt) }
