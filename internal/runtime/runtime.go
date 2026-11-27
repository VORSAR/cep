// Package runtime is the engine's live core (ADR-0006 §4, §5; ADR-0012 §3, §6): one ingest/router goroutine, shards
// with bounded mailboxes that run each key's work serially, the non-blocking outbox handoff, sink queues with
// runtime-owned flushers, the drain barrier, and the ingest step it shares with the replay driver.
// Design: docs/sprints/s3-2a-2-design.md.
package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
)

// AppID is an app's id: dense, from 0, in Start's order.
type AppID int

// Emit is a body's output: an event on a derived stream (e.StreamID); it goes to that stream's sinks.
type Emit = func(e event.Event)

// Rule is one rule of an app: the planner's description (one description, design s3-2b-1 v2 §1, review F6).
type Rule = plan.Rule

// Sink receives one derived stream's outputs on its own flusher goroutine. An error is counted.
type Sink func(e event.Event) error

// SinkBinding binds a sink to a derived stream.
type SinkBinding struct {
	Stream int
	Sink   Sink
}

// App is one deployed app.
type App struct {
	Streams []clock.StreamSpec // declaration order; their index is the stream id
	Schemas []event.Schema     // aligned with Streams: Send and Start validate against them
	// Tables: nil, no tables. The router is their only live writer (owner F14), and Remove empties them, so the Set
	// is the app's own, never shared with another app.
	Tables *table.Set
	Rules  []Rule
	Sinks  []SinkBinding
	// Recorder writes the app's recording (S3.2a-1's D3; S3.2b-2's Q1, Q2) on its own goroutine; nil: none, the
	// default (owner D4 of S3.2b-2's design: recording is off unless the deployment turns it on).
	Recorder Recorder
	// OnRemoved, if set, is called once, on the caller's goroutine of Remove or Close, after the app's goroutines
	// were asked to stop: the facade cancels its sinks' context here (design s3-3-1 §2.1, §3.6). Never called twice.
	OnRemoved func()
	// Loads are the app's starting rows, put table by table in load order by the router's activation step
	// (Activate), before the app takes any input; Add checks each, so applying them cannot fail. The runtime keeps
	// the rows: the caller must not change them after Add. With a Recorder, the app's recording queue is
	// Config.RecordQueue plus one slot per starting row, so the header and the starting rows, queued before the
	// recorder starts, always fit and leave the configured room for later records. A built app (NewSync) takes none.
	Loads []TableLoad
}

// TableLoad is one table's starting rows: whole rows put into table Table of App.Tables at activation.
type TableLoad struct {
	Table int
	Rows  [][]event.Value
}

// Recorder writes one app's recording (design s3-2b-2 §7). The runtime calls it from the app's recorder goroutine —
// never the router's — in intake order: each record is queued before the step it records. A non-nil error ends the
// recording for good (owner Q2 of S3.2b-2): the router never calls
// it again, records the failure in Stats and goes on running the app.
type Recorder interface {
	Header(start int64) error                                           // once, before any other call
	Event(e *event.Event, arrival int64, payload []byte) error          // payload: SendPayload's bytes; nil for Send
	Rejected(stream int, arrival int64, fault *event.DecodeError) error // owner Q1 of S3.2b-2
	Tick(now int64) error
	Table(table int, op TableOp, rows [][]event.Value) error // only a write that succeeded
	Stop() error                                             // at the drain, once
	// a source's batch-close signal, in intake order (design s3-3-1 §2.1)
	BatchEnd(stream int, batch string) error
}

// Config is the runtime's configuration; zero values take the defaults.
type Config struct {
	Shards    int           // 0: GOMAXPROCS
	Mailbox   int           // per shard, in messages; 0: 1024
	Intake    int           // the router's intake queue; 0: 1024
	SinkQueue int           // per sink binding; 0: 1024
	TickEvery time.Duration // live idle ticks from an internal ticker; 0: none
	Now       func() int64  // wall ms; nil: time.Now
	// RecordQueue is each recording's queue, in records, plus one slot per starting row of the app (App.Loads); 0:
	// 65,536. At a full queue the router first gives up the processor, at most twice, and offers the record again; if
	// the queue is still full, the recording ends (owner D2 of S3.2b-2's design). The queue must be large enough to
	// cover the recorder's wake-up, and values far below the default can end a recording under bursts at any CPU count.
	// The queue is allocated at Add and released when the app is removed (Remove), once its recorder has returned.
	RecordQueue int
}

// The errors a caller sees.
var (
	ErrClosed   = errors.New("runtime: closed")
	ErrDrained  = errors.New("runtime: the app is drained: its input has ended")
	ErrInactive = errors.New("runtime: the app is not activated")          // design s3-3-1 §2.1
	ErrStarted  = errors.New("runtime: the app is already activated")      // design s3-3-1 §2.1
	ErrRemoved  = errors.New("runtime: the app is removed")                // design s3-3-1 §2.1
	ErrActive   = errors.New("runtime: the app is active: drain it first") // Remove of an active app
	// ErrDrainIncomplete is Remove of a draining app whose barrier has not run (a Drain stuck on its sources): drain
	// again.
	ErrDrainIncomplete = errors.New("runtime: the drain is not complete: drain it again")
)

// TableOp is a table write's operation.
type TableOp uint8

// The table operations.
const (
	TablePut TableOp = iota + 1
	TableDelete
)

// DrainResult is what a drain barrier reports.
type DrainResult struct {
	Pending int64   // body timers left unfired after the barrier, TTL timers excepted (drained_pending)
	Runs    []error // per Run goroutine, in Run order: its return, with context.Canceled read as nil (design s3-3-1 §2.1)
}

// RuleStats are one rule's counters, summed over the shards.
type RuleStats struct {
	ConstructLate, Purged, KeyErrors, Panics int64
	Suppressed, Errored                      int64 // from each body's plan.Counted (design s3-2b-1 §4)
	// OutboxHigh is the most outputs the rule emitted from one message on one shard — one Advance's emissions, which
	// the tripwire's outbox ceiling bounds (ADR-0006 §5; design s3-2b-2 §8); the largest over the shards.
	OutboxHigh int64
}

// StreamStats are one source stream's counters (design s3-2b-2 §6).
type StreamStats struct {
	Late          int64 // stream-late events
	AfterDrain    int64 // refused after the app's drain
	Rejected      int64 // payloads that failed the schema: errored_events (SEMANTICS §10)
	Unknown       int64 // payload keys the schema does not declare, each occurrence
	FlagOverrides int64 // payloads carrying text_invisibles
	Batches       int64 // batch-close signals accepted (owner, 2026-11-07, S3.3-1 design 6)
}

// RecordingStats is one app's recording (owner Q2 of S3.2b-2): the records written before a failure, and its message.
type RecordingStats struct {
	Records int64
	Failed  bool
	Err     string
}

// Stats is a consistent cut of the runtime's counters at one intake position.
type Stats struct {
	Streams        [][]StreamStats  // [app][stream] (design s3-2b-2 §6)
	Recording      []RecordingStats // per app; read outside the cut, as SinkErrors (the recorder goroutine's own)
	SinkHigh       [][]int          // [app][binding]: the longest its queue has been (sink-depth; design s3-2b-2 §8)
	Rules          [][]RuleStats    // [app][rule]
	SinkErrors     [][]int64        // [app][binding]; read atomically, outside the cut
	PendingHigh    int              // the router's pending FIFO, its longest
	MailboxHigh    []int            // per shard, the longest mailbox seen after a send
	SaturatedParks []int64          // per shard: parks with a non-empty outbox while the mailbox was full
}

// Runtime is a running engine core (design §2–§6).
type Runtime struct {
	start int64 // the deployment's start: the stamps' floor (review F5)
	cfg   Config
	// copy-on-write by the router (iAdd, iRemove); callers Load a snapshot (design s3-3-1 §2.5 MUST 1)
	apps      atomic.Pointer[[]*appState]
	shards    []*shard
	intake    chan item
	handoff   chan handoff
	stopped   chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	inflight  inflightGate // Add and Activate run inside it; Close closes it first (design s3-3-2 §9.1)
	// removeFault is for the package's tests only: a non-nil error it returns is Remove's, before the router is asked
	// (the seam TestCloseReportsARemoveError needs: nothing else makes Remove fail without ctx inside Close).
	removeFault func(id AppID) error
	// addHold is for the package's tests only: the router calls it as it begins an install, so a test can hold an Add
	// in flight deterministically (TestCloseWaitsForAnAddInFlight). Set before the Add it holds.
	addHold func()
	// activateHold is for the package's tests only: Activate calls it between the router's answer and its tail, so a
	// test can hold an activation there while Close gives up at its deadline (the combined review's R-S3).
	activateHold func()
	// openHold is for the package's tests only: Activate's last step calls it under inflight.mu, between marking the
	// app's goroutines running (a.opened) and its move to active, so a test can run a Drain there.
	openHold func()
	// recordingWait is for the package's tests only: Drain calls it as it begins to wait for the recording, so a test
	// can release a held Recorder only once Drain waits for it (TestDrainWaitsForAStoppedRecordingsQueuedRecords).
	recordingWait func()
}

// Start is New, then Add and Activate for each app in order (design s3-3-1 §2.7): the apps validated, every shard's
// rules built, the router, the shards and the sink flushers started. Unchanged behaviour since S3.2a-2.
func Start(cfg Config, apps []App) (*Runtime, error) {
	r, err := New(cfg)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		id, err := r.Add(context.Background(), apps[i])
		if err != nil {
			_ = r.Close(context.Background())
			return nil, err
		}
		if err := r.Activate(context.Background(), id); err != nil {
			_ = r.Close(context.Background())
			return nil, err
		}
	}
	return r, nil
}
