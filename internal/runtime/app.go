package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vorsar/cep/internal/event"
)

type appState struct {
	id       AppID
	app      App
	ing      *Ingest          // the router's
	byStream [][]route        // per stream: the rules reading it
	sinks    [][]*sinkQ       // per stream: its bindings
	all      []*sinkQ         // every binding, in order
	draining atomic.Bool      // a caller's fast path (owner F5)
	drained  bool             // the router's, by intake order
	drainTok uint64           // the router's: the drain's token while its acks are due
	drainRes *reply           // the router's: the drain's result, once known; a retried Drain waits on it (owner)
	late     []int64          // per stream: stream_late (the router's)
	after    []int64          // per stream: refused after the drain (the router's)
	keyErr   []int64          // per rule: key_errors (the router's)
	dec      []*event.Decoder // per stream: SendPayload's decoder (safe for concurrent use)
	rec      *recQ            // nil: no recording (owner D4 of S3.2b-2's design: off unless turned on)

	// The per-app lifecycle (design s3-3-1 §2.2).
	phase     atomic.Int32       // appPhase: the caller-side fast path, read before every entry point
	started   atomic.Bool        // set by Activate; the router reads it (a drain of a never-activated app)
	start     int64              // the router's stamp when it installed the app: the Ingest's and the header's
	srcCtx    context.Context    // the source context, made at Add; Drain cancels it
	cancelSrc context.CancelFunc //
	runMu     sync.Mutex         // guards runs and waited
	runs      []*runState        // Run's goroutines
	waited    bool               // Drain's caller side: the runs were awaited once
	quiet     bool               // the router's: ticks skip the app (S3.1-1's D3)
	batches   []int64            // per stream: StreamStats.Batches (the router's)
	onRemoved func()             // App.OnRemoved
	removed   sync.Once          // onRemoved runs once
	activated bool               // the router's: the app was activated (iActivate); ticks reach it, its drain records
	removing  bool               // the router's: a removal was accepted; a later Activate or Remove is refused
}

// runState is one Run goroutine: err is written before done is closed.
type runState struct {
	done chan struct{}
	err  error
}

type route struct {
	rule  int
	attrs []int // the key's attributes in part order; nil: key-less
}

// newAppState builds one app's router-side state (design s3-3-1 §2.4): its ingest from start, its decoders, its
// routes, its sink queues and, with a Recorder, its recording queue with the header queued (written by the recorder
// goroutine, which Activate starts).
func (r *Runtime) newAppState(id AppID, app App, start int64) *appState {
	a := &appState{
		id: id, app: app, ing: NewIngest(app.Streams, start, app.Tables, false).withFlagSlots(app.Schemas),
		byStream: make([][]route, len(app.Streams)), sinks: make([][]*sinkQ, len(app.Streams)),
		late: make([]int64, len(app.Streams)), after: make([]int64, len(app.Streams)),
		batches: make([]int64, len(app.Streams)),
		keyErr:  make([]int64, len(app.Rules)),
		dec:     make([]*event.Decoder, len(app.Streams)),
		start:   start, onRemoved: app.OnRemoved,
	}
	a.srcCtx, a.cancelSrc = context.WithCancel(context.Background())
	for sid, sch := range app.Schemas {
		a.dec[sid] = event.NewDecoder(sch)
	}
	if app.Recorder != nil {
		a.rec = newRecQ(app.Recorder, recordQueueFor(r.cfg.RecordQueue, app.Loads))
		a.rec.put(recItem{kind: rHeader, arrival: start}) // the header first (design s3-2b-2 §7)
	}
	for _, b := range app.Sinks {
		q := &sinkQ{q: make(chan sinkItem, r.cfg.SinkQueue), sink: b.Sink, returned: make(chan struct{})}
		a.sinks[b.Stream] = append(a.sinks[b.Stream], q)
		a.all = append(a.all, q)
	}
	return a
}

// recordQueueFor is an app's recording queue: the configured size plus one slot per starting row, which is
// max(configured, 1 + rows + (configured - 1)). The header and every starting row are queued before the recorder
// starts, so they always fit and leave the room an app without starting rows has after its header; a full queue
// still ends the recording (owner D2 of S3.2b-2's design).
func recordQueueFor(configured int, loads []TableLoad) int {
	n := configured
	for _, l := range loads {
		n += len(l.Rows)
	}
	return n
}

// installRoutes makes the app reachable: per stream, the rules reading it (the router's, once: build for a built
// app, installAck for an added one).
func (a *appState) installRoutes() {
	for ri, rule := range a.app.Rules {
		for k, sid := range rule.Inputs {
			var attrs []int
			if rule.KeyAttrs != nil {
				attrs = rule.KeyAttrs[k]
			}
			a.byStream[sid] = append(a.byStream[sid], route{rule: ri, attrs: attrs})
		}
	}
}

// appList is the app list's current snapshot: nil slots are removed apps (design s3-3-1 §2.5 MUST 2).
func (r *Runtime) appList() []*appState {
	if p := r.apps.Load(); p != nil {
		return *p
	}
	return nil
}

// write applies one table write in intake order (owner F14), shared by the router and Sync.
func (a *appState) write(tableID int, op TableOp, rows [][]event.Value) error {
	if a.app.Tables == nil || tableID < 0 || tableID >= a.app.Tables.Len() {
		return fmt.Errorf("runtime: app %d: no table %d", a.id, tableID)
	}
	tb := a.app.Tables.Table(tableID)
	switch op {
	case TablePut:
		return tb.Upsert(rows)
	case TableDelete:
		return tb.Delete(rows)
	}
	return fmt.Errorf("runtime: app %d: an unknown table operation %d", a.id, op)
}
