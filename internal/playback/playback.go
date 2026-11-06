// Package playback is the replay format layer (ADR-0005 §3): the log format and its reader, the virtual-clock
// driver, the canonical output encoder and the canonical trace. The driver replays through the real runtime: it owns a
// runtime.Sync, one shard stepped on its caller's goroutine (ADR-0005 §3). Designs: docs/sprints/s3-2a-1-design.md §5,
// docs/sprints/s3-2b-1-design.md §4.
package playback

import (
	"fmt"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
)

// Stream is one declared stream; its index in App.Streams is its stream id (SEMANTICS §2.2's declaration order). It is
// the planner's: one description of an app (design s3-2b-1 v2 §1, review F6). A derived stream is never in a log
// (v0.1).
type Stream = plan.Stream

// Table is one declared table; its index in App.Tables is its table id.
type Table = plan.Table

// App is what a log is read against, and, with rules, what a plan driver replays.
type App = plan.App

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
	// RecRejected is a recording's rejected live event (owner Q1 of S3.2b-2; design s3-2b-2 §7):
	// {"arrival":…,"rejected":{"reason":…,"attr":…},"stream":…}. Stub: the reader does not produce it yet.
	RecRejected
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
	// Info is RecEvent's decode report: unknown payload keys and a payload's own text_invisibles, counted per stream
	// (design s3-2b-2 §6). Stub: not filled yet.
	Info event.Decoded
	// Reason and Attr are RecRejected's fault (Stream and Arrival hold its stream and stamp).
	Reason event.DecodeReason
	Attr   string
}

// LoadError is the first fault in a log (owner D6 of S3.2a-1): the reader stops at its line; steps already delivered
// stand. A payload that fails its stream's schema is a fault too, with its cause in Decode.
type LoadError struct {
	Line   int
	Msg    string
	Decode *event.DecodeError // a payload fault's cause; nil otherwise
}

func (e *LoadError) Error() string { return fmt.Sprintf("playback: line %d: %s", e.Line, e.Msg) }

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
	Time     int64 // StepHeader: the deployment's start; StepTick: the tick's wall time
}
