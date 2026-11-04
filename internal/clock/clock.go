// Package clock is the engine's one clock service (ADR-0004 as annotated at S3.1-1): the per-app stream registry,
// written by ingest or the playback driver, and the per-rule construct clock, shared by the rule's keys on one
// shard, with each key's release buffer and timers. RuleClock.Advance walks pending work forward in event time —
// the only place that order exists. All times are int64 milliseconds; no time.Time and no clock read appear here.
// Design: docs/sprints/s3-1-1-design.md (v2).
//
// STUB (S3.1-1, before G1): the API QA's tests are written against; every body panics until the build.
package clock

import "github.com/vorsar/cep/internal/event"

const notBuilt = "clock: S3.1-1 not built yet"

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

// StreamRegistry holds every stream's admission watermark, floors and contribution.
type StreamRegistry struct{}

// NewStreamRegistry registers the streams in declaration order; start is the deployment's start (wall ms).
func NewStreamRegistry(specs []StreamSpec, start int64) *StreamRegistry { panic(notBuilt) }

// ObserveEvent judges an event's stream lateness and moves its stream's watermark; arrival is its intake stamp.
func (r *StreamRegistry) ObserveEvent(sid int, ts, arrival int64) (streamLate bool, advance WMAdvance) {
	panic(notBuilt)
}

// ObserveTick moves every quiet stream's floor to its own last event time plus the wall time elapsed (live).
func (r *StreamRegistry) ObserveTick(now int64, buf []WMAdvance) []WMAdvance { panic(notBuilt) }

// ObserveVirtual raises every source stream's virtual floor to now − d_s (playback).
func (r *StreamRegistry) ObserveVirtual(now int64, buf []WMAdvance) []WMAdvance { panic(notBuilt) }

// ObservePunctuation records a derived stream's punctuation.
func (r *StreamRegistry) ObservePunctuation(sid int, w int64) WMAdvance { panic(notBuilt) }

// EndOfInput raises every contribution to +∞.
func (r *StreamRegistry) EndOfInput(buf []WMAdvance) []WMAdvance { panic(notBuilt) }

// Contribution is a stream's current contribution.
func (r *StreamRegistry) Contribution(sid int) int64 { panic(notBuilt) }

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

// Timer is one timer that fired.
type Timer struct {
	Due  int64
	Kind TimerKind
	Ref  uint64
}

// Callbacks receive the walk's items in order.
type Callbacks struct {
	OnEvent   func(in *Instance, e event.Event)
	OnTimers  func(in *Instance, fires []Timer)
	OnDiscard func(in *Instance, refs []uint64)
}

// RuleClock is one rule's construct clock on one shard, shared by its keys' instances.
type RuleClock struct{}

// NewRuleClock makes a rule clock reading the given streams.
func NewRuleClock(inputs []int) *RuleClock { panic(notBuilt) }

// ApplyAdvance takes a stream's new contribution; the clock never retreats.
func (rc *RuleClock) ApplyAdvance(a WMAdvance) (clock int64, advanced bool) { panic(notBuilt) }

// Clock is the construct clock.
func (rc *RuleClock) Clock() int64 { panic(notBuilt) }

// NewInstance makes a key's instance; ordinals count up from 0.
func (rc *RuleClock) NewInstance() *Instance { panic(notBuilt) }

// Advance walks every pending item up to the clock, forward in event time.
func (rc *RuleClock) Advance(cb Callbacks) { panic(notBuilt) }

// Free drops a key's instance; the instance must be Empty.
func (rc *RuleClock) Free(in *Instance) { panic(notBuilt) }

// Instance is one key's release buffer and timers under its rule clock.
type Instance struct{}

// Ordinal is the key's birth order on this rule clock.
func (in *Instance) Ordinal() uint64 { panic(notBuilt) }

// Buffer holds an admitted event until the walk releases it, or reports it construct-late.
func (in *Instance) Buffer(e event.Event) (constructLate bool) { panic(notBuilt) }

// Schedule adds a timer; a second Schedule of the same (kind, ref) replaces it.
func (in *Instance) Schedule(due int64, k TimerKind, ref uint64) { panic(notBuilt) }

// Cancel removes a pending timer; false when there was none.
func (in *Instance) Cancel(k TimerKind, ref uint64) bool { panic(notBuilt) }

// CancelAll removes every pending timer of the key (the TTL purge's reset).
func (in *Instance) CancelAll() { panic(notBuilt) }

// Empty says the key has no buffered event and no timer.
func (in *Instance) Empty() bool { panic(notBuilt) }
