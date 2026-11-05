// Package playback is the replay format layer (ADR-0005 §3): the log format and its reader, the virtual-clock
// driver, the canonical output encoder and the canonical trace. The drive loop through the runtime is S3.2b's.
// Design: docs/sprints/s3-2a-1-design.md §5.
package playback

import (
	"io"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/table"
)

// Stream is one declared stream; its index in App.Streams is its stream id (SEMANTICS §2.2's declaration order).
type Stream struct {
	Schema    event.Schema // name, attributes, time attribute (event.ArrivalTime for `time arrival`)
	Delay     int64        // d_s, ms
	IdleAfter int64        // ms; 0: the stream declares no idleness (a recording's ticks never move it)
	Derived   bool         // a rule's output: never in a log (v0.1)
}

// Table is one declared table; its index in App.Tables is its table id.
type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}

// App is what a log is read against.
type App struct {
	Streams []Stream
	Tables  []Table
}

// RecordKind is a log line's kind.
type RecordKind uint8

// The record kinds (design note §5.2).
const (
	RecEvent RecordKind = iota + 1
	RecTable
	RecBatchEnd
	RecHeader // a recording's first record: {"recording":{"start":…}}
	RecTick   // a recording's idle tick
	RecStop   // a recording's last record: the session ended by Stop
)

// TableOp is a table record's operation.
type TableOp uint8

// The table operations.
const (
	OpPut TableOp = iota + 1
	OpDelete
)

// Record is one log line, decoded. Every slice in it is fresh: the clock and later the operators keep events, so a
// Values slice is never reused (ADR-0010 §4).
type Record struct {
	Line    int
	Kind    RecordKind
	Event   event.Event // RecEvent: StreamID, Ts, Values, BatchID; Seq and Tables are the driver's
	Arrival int64       // RecEvent in a recording: the intake stamp (a `time arrival` stream: = Event.Ts)
	Table   int         // RecTable
	Op      TableOp
	Row     []event.Value // put: the row in attribute order; delete: the key in key order
	Stream  int           // RecBatchEnd
	Batch   string
	Time    int64 // RecHeader: the deployment's start; RecTick: the tick's wall time
}

// LoadError is the first fault in a log (owner D6 of S3.2a-1): the reader stops at its line; steps already delivered
// stand. A payload that fails its stream's schema is a fault too, with its cause in Decode.
type LoadError struct {
	Line   int
	Msg    string
	Decode *event.DecodeError // a payload fault's cause; nil otherwise
}

func (e *LoadError) Error() string {
	panic("playback: LoadError.Error: not implemented (S3.2a-1 stub)")
}

// Reader reads a log one record at a time.
type Reader struct{}

// NewReader reads r against app; an invalid App is an error.
func NewReader(r io.Reader, app *App) (*Reader, error) {
	panic("playback: NewReader: not implemented (S3.2a-1 stub)")
}

// Next is the next record; io.EOF after the last; a *LoadError at the first fault, after which it returns the same.
func (r *Reader) Next() (Record, error) {
	panic("playback: Reader.Next: not implemented (S3.2a-1 stub)")
}

// StepKind is a replay step's kind.
type StepKind uint8

// The step kinds (design note §5.3).
const (
	StepEvent      StepKind = iota + 1 // admitted: the step contract (design note §1) with Advances
	StepHeader                         // a recording's header: the driver switches to recording mode
	StepStreamLate                     // behind its stream's admission watermark: counted; nothing moves (not a fault)
	StepTable                          // a table record, applied
	StepBatchEnd                       // a batch-end record
	StepTick                           // a recording's tick: Advances from ObserveTick
	StepStop                           // a recording's stop: Advances from Drain
	StepEnd                            // a fixture: Advances raise every contribution to +∞; a recording: none
)

// Step is one replay step.
type Step struct {
	Kind     StepKind
	Line     int               // the record's line; StepEnd: the last line + 1
	Event    event.Event       // StepEvent, StepStreamLate: Seq and Tables set
	Advances []clock.WMAdvance // StepEvent, StepTick, StepStop, StepEnd; valid until the next call
	Tables   *table.Snapshot   // the snapshot current after this step: what this step's timers read
	Table    int               // StepTable
	Op       TableOp
	Stream   int // StepBatchEnd
	Batch    string
}

// Driver replays a log in file order under the virtual clock (ADR-0005 §3, SEMANTICS §2.1).
type Driver struct{}

// NewDriver makes a driver with fresh, empty tables: a replay is a sandbox (SPEC §6.8).
func NewDriver(app *App) (*Driver, error) {
	panic("playback: NewDriver: not implemented (S3.2a-1 stub)")
}

// Step processes one record the Reader produced; it cannot fail (the Reader checks everything first).
func (d *Driver) Step(rec Record) Step {
	panic("playback: Driver.Step: not implemented (S3.2a-1 stub)")
}

// End is the log's end: in a fixture every contribution to +∞; in a recording nothing.
func (d *Driver) End() Step { panic("playback: Driver.End: not implemented (S3.2a-1 stub)") }

// Tables is the replay's own tables.
func (d *Driver) Tables() *table.Set {
	panic("playback: Driver.Tables: not implemented (S3.2a-1 stub)")
}

// Registry is the replay's stream registry.
func (d *Driver) Registry() *clock.StreamRegistry {
	panic("playback: Driver.Registry: not implemented (S3.2a-1 stub)")
}

// Walk yields the driver's Seq counters (component "seq", key the stream name): a state holder (ADR-0006 §2(b)).
func (d *Driver) Walk(fn func(component, key string, state any)) {
	panic("playback: Driver.Walk: not implemented (S3.2a-1 stub)")
}

// Replay streams r through d line by line, calling fn on each step, then End; it stops at the first error (a fault or
// fn's), and the steps already delivered stand (owner D6 of S3.2a-1).
func Replay(r *Reader, d *Driver, fn func(*Step) error) error {
	panic("playback: Replay: not implemented (S3.2a-1 stub)")
}

// AppendValue appends v's canonical JSON (design note §5.4). Error: a NaN or ±Inf float; a timestamp outside the
// years 0000–9999.
func AppendValue(dst []byte, v event.Value) ([]byte, error) {
	panic("playback: AppendValue: not implemented (S3.2a-1 stub)")
}

// AppendEvent appends e as one log line without a newline: {"attrs":{…},"batch":"…","stream":"…","ts":"…"}; batch
// only when set, ts only when s has no time attribute.
func AppendEvent(dst []byte, s event.Schema, e event.Event) ([]byte, error) {
	panic("playback: AppendEvent: not implemented (S3.2a-1 stub)")
}

// Trace records a replay's canonical trace: one canonical JSON line per entry, in the order recorded (design note
// §5.5).
type Trace struct{}

// NewTrace makes an empty trace for app.
func NewTrace(app *App) *Trace { panic("playback: NewTrace: not implemented (S3.2a-1 stub)") }

// Step records the driver's own entries: stream_late, table, batch_end, tick, stop, end.
func (t *Trace) Step(s *Step) { panic("playback: Trace.Step: not implemented (S3.2a-1 stub)") }

// Released records a released event.
func (t *Trace) Released(at int, rule string, key event.Value, e event.Event) {
	panic("playback: Trace.Released: not implemented (S3.2a-1 stub)")
}

// ConstructLate records an event the rule's clock judged late.
func (t *Trace) ConstructLate(at int, rule string, key event.Value, e event.Event) {
	panic("playback: Trace.ConstructLate: not implemented (S3.2a-1 stub)")
}

// Fired records a fired timer.
func (t *Trace) Fired(at int, rule string, key event.Value, tm clock.Timer) {
	panic("playback: Trace.Fired: not implemented (S3.2a-1 stub)")
}

// Discarded records a discard.
func (t *Trace) Discarded(at int, rule string, key event.Value, ref uint64) {
	panic("playback: Trace.Discarded: not implemented (S3.2a-1 stub)")
}

// Purged records a TTL purge.
func (t *Trace) Purged(at int, rule string, key event.Value, due int64) {
	panic("playback: Trace.Purged: not implemented (S3.2a-1 stub)")
}

// Note records a body's own observation.
func (t *Trace) Note(at int, rule string, key event.Value, name string, v event.Value) {
	panic("playback: Trace.Note: not implemented (S3.2a-1 stub)")
}

// Bytes is the trace so far, or the first encoding error.
func (t *Trace) Bytes() ([]byte, error) {
	panic("playback: Trace.Bytes: not implemented (S3.2a-1 stub)")
}
