// Package runtime is the engine's live core (ADR-0006 §4, §5; ADR-0012 §3, §6): one ingest/router goroutine, shards
// with bounded mailboxes that run each key's work serially, the non-blocking outbox handoff, sink queues with
// runtime-owned flushers, the drain barrier, and the ingest step it shares with the replay driver.
// Design: docs/sprints/s3-2a-2-design.md.
package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/table"
)

// AppID is an app's id: dense, from 0, in Start's order.
type AppID int

// Emit is a body's output: an event on a derived stream (e.StreamID); it goes to that stream's sinks.
type Emit func(e event.Event)

// Rule is one rule of an app, as S3.2b's planner will describe it.
type Rule struct {
	Name    string
	Inputs  []int      // the stream ids the rule reads
	KeyAttr []int      // per input, the key attribute's index in that stream; nil: key-less (the pseudo-key "")
	KeyType event.Type // the keys' type; TString for a key-less rule
	TTL     int64      // ms; 0 for a key-less rule
	NewBody func(emit Emit) partition.Body
}

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
	Tables  *table.Set         // nil: no tables; the router is their only live writer (owner F14)
	Rules   []Rule
	Sinks   []SinkBinding
}

// Config is the runtime's configuration; zero values take the defaults.
type Config struct {
	Shards    int           // 0: GOMAXPROCS
	Mailbox   int           // per shard, in messages; 0: 1024
	Intake    int           // the router's intake queue; 0: 1024
	SinkQueue int           // per sink binding; 0: 1024
	TickEvery time.Duration // live idle ticks from an internal ticker; 0: none
	Now       func() int64  // wall ms; nil: time.Now
}

// The errors a caller sees.
var (
	ErrClosed  = errors.New("runtime: closed")
	ErrDrained = errors.New("runtime: the app is drained: its input has ended (owner F5)")
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
	Pending int64 // body timers left unfired after the barrier, TTL timers excepted (drained_pending)
}

// RuleStats are one rule's counters, summed over the shards.
type RuleStats struct {
	ConstructLate, Purged, KeyErrors, Panics int64
}

// Stats is a consistent cut of the runtime's counters at one intake position.
type Stats struct {
	StreamLate     [][]int64     // [app][stream]
	AfterDrain     [][]int64     // [app][stream]: refused after the app's drain, by intake order
	Rules          [][]RuleStats // [app][rule]
	SinkErrors     [][]int64     // [app][binding]; read atomically, outside the cut
	PendingHigh    int           // the router's pending FIFO, its longest
	MailboxHigh    []int         // per shard, the longest mailbox seen after a send
	SaturatedParks []int64       // per shard: parks with a non-empty outbox while the mailbox was full
}

// Runtime is a running engine core.
type Runtime struct{}

// Start validates the apps, builds every shard's rules, and starts the router, the shards and the sink flushers.
func Start(cfg Config, apps []App) (*Runtime, error) {
	panic("runtime: Start: not implemented (S3.2a-2 stub)")
}

// Send hands one source event of app to the intake; it blocks for backpressure and respects ctx.
func (r *Runtime) Send(ctx context.Context, app AppID, e event.Event) error {
	panic("runtime: Send: not implemented (S3.2a-2 stub)")
}

// Tick hands a wall tick to the intake, behind every event already there.
func (r *Runtime) Tick(ctx context.Context, now int64) error {
	panic("runtime: Tick: not implemented (S3.2a-2 stub)")
}

// TableWrite applies rows to one of app's tables in intake order (owner F14) and returns once the router has applied
// them: put rows are whole rows; delete rows are key values, in key order.
func (r *Runtime) TableWrite(ctx context.Context, app AppID, tableID int, op TableOp, rows [][]event.Value) error {
	panic("runtime: TableWrite: not implemented (S3.2a-2 stub)")
}

// Drain runs the drain barrier for app (design §5).
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
	panic("runtime: Drain: not implemented (S3.2a-2 stub)")
}

// Stats is a consistent cut of the counters, taken through the intake (design §6).
func (r *Runtime) Stats(ctx context.Context) (Stats, error) {
	panic("runtime: Stats: not implemented (S3.2a-2 stub)")
}

// Close stops every goroutine and drops what is left; it fires no pending timer.
func (r *Runtime) Close(ctx context.Context) error {
	panic("runtime: Close: not implemented (S3.2a-2 stub)")
}
