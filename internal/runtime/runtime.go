// Package runtime is the engine's live core (ADR-0006 §4, §5; ADR-0012 §3, §6): one ingest/router goroutine, shards
// with bounded mailboxes that run each key's work serially, the non-blocking outbox handoff, sink queues with
// runtime-owned flushers, the drain barrier, and the ingest step it shares with the replay driver.
// Design: docs/sprints/s3-2a-2-design.md.
package runtime

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	goruntime "runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
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
	Tables  *table.Set         // nil: no tables; the router is their only live writer (owner F14)
	Rules   []Rule
	Sinks   []SinkBinding
	// Recorder writes the app's recording (S3.2a-1's D3; S3.2b-2's Q1, Q2) on its own goroutine; nil: none, the
	// default (owner D4 of S3.2b-2's design: recording is off unless the deployment turns it on).
	Recorder Recorder
	// OnRemoved, if set, is called once, on the caller's goroutine of Remove or Close, after the app's goroutines
	// were asked to stop: the facade cancels its sinks' context here (design s3-3-1 §2.1, §3.6). Never called twice.
	OnRemoved func()
}

// Recorder writes one app's recording (design s3-2b-2 §7). The runtime calls it from the app's recorder goroutine —
// never the router's — in intake order: each record is queued before the step it records. A non-nil error ends the recording for good (owner Q2 of S3.2b-2): the router never calls
// it again, records the failure in Stats and goes on running the app.
type Recorder interface {
	Header(start int64) error                                           // once, before any other call
	Event(e *event.Event, arrival int64, payload []byte) error          // payload: SendPayload's bytes; nil for Send
	Rejected(stream int, arrival int64, fault *event.DecodeError) error // owner Q1 of S3.2b-2
	Tick(now int64) error
	Table(table int, op TableOp, rows [][]event.Value) error // only a write that succeeded
	Stop() error                                             // at the drain, once
	BatchEnd(stream int, batch string) error                 // a source's batch-close signal, in intake order (design s3-3-1 §2.1)
}

// Config is the runtime's configuration; zero values take the defaults.
type Config struct {
	Shards    int           // 0: GOMAXPROCS
	Mailbox   int           // per shard, in messages; 0: 1024
	Intake    int           // the router's intake queue; 0: 1024
	SinkQueue int           // per sink binding; 0: 1024
	TickEvery time.Duration // live idle ticks from an internal ticker; 0: none
	Now       func() int64  // wall ms; nil: time.Now
	// RecordQueue is each recording's queue, in records; 0: 65,536. A full queue ends the recording (owner D2 of
	// S3.2b-2's design).
	RecordQueue int
}

// The errors a caller sees.
var (
	ErrClosed   = errors.New("runtime: closed")
	ErrDrained  = errors.New("runtime: the app is drained: its input has ended (owner F5)")
	ErrInactive = errors.New("runtime: the app is not activated")          // design s3-3-1 §2.1
	ErrStarted  = errors.New("runtime: the app is already activated")      // design s3-3-1 §2.1
	ErrRemoved  = errors.New("runtime: the app is removed")                // design s3-3-1 §2.1
	ErrActive   = errors.New("runtime: the app is active: drain it first") // Remove of an active app
	// ErrDrainIncomplete is Remove of a draining app whose barrier has not run (a Drain stuck on its sources): drain again.
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
	start     int64 // the deployment's start: the stamps' floor (review F5)
	cfg       Config
	apps      atomic.Pointer[[]*appState] // copy-on-write by the router (iAdd, iRemove); callers Load a snapshot (design s3-3-1 §2.5 MUST 1)
	shards    []*shard
	intake    chan item
	handoff   chan handoff
	stopped   chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

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

type sinkQ struct {
	q        chan sinkItem
	sink     Sink
	errors   atomic.Int64
	high     int           // the router's: the longest q has been after a send
	returned chan struct{} // closed by the flusher when it returns (design s3-3-1 §2.4)
}

type sinkItem struct {
	ev     event.Event
	marker chan struct{} // non-nil: a drain's flush marker, closed on reaching it
	last   bool          // with marker: the flusher returns after closing it (Remove; design s3-3-1 §2.4)
}

type itemKind uint8

const (
	iEvent  itemKind = iota + 1
	iReject          // a payload that failed its schema or the format's limits (owner Q1 of S3.2b-2)
	iTick
	iWrite
	iDrain
	iStats
	iAdd      // design s3-3-1 §2.3: register an app, inactive
	iRemove   // unregister a drained or inactive app
	iQuiet    // the ticks stop for the app (S3.1-1's D3)
	iBatchEnd // a source's batch-close signal
	iActivate // design s3-3-1 §2.4 as fixed: the router decides the activation in intake order (review B1)
)

type item struct {
	kind    itemKind
	app     AppID
	ev      event.Event
	info    event.Decoded      // iEvent from SendPayload: its decode report
	payload []byte             // iEvent from SendPayload with a recording: the payload as sent, cleaned; the item's own
	stream  int                // iReject
	fault   *event.DecodeError // iReject
	now     int64
	table   int
	op      TableOp
	rows    [][]event.Value
	reply   chan reply // buffered 1: the router never blocks on it
	add     *App       // iAdd
	batch   string     // iBatchEnd
}

type reply struct {
	err       error
	id        AppID // iAdd's
	pending   int64
	markers   []chan struct{}
	recMarker chan struct{} // a drain's recording stop, waited for only until the drain's deadline (owner, 2026-11-06)
	stats     Stats
}

type msgKind uint8

const (
	mWalk msgKind = iota + 1 // advances, and an event's offers
	mDrain
	mStats
	mAdd    // design s3-3-1 §2.3: build the app's slot on this shard; acked
	mRemove // free it; acked
)

type message struct {
	kind      msgKind
	app       AppID
	adv       []clock.WMAdvance
	tables    *table.Snapshot
	ev        event.Event
	offers    []offer
	token     uint64
	admStream int   // an event message: its stream, whose admission watermark is adm (owner D1 of S3.2b-1); -1: none
	adm       int64 // what an on-arrival rule's clock takes, in place of the stream's advances
	final     bool  // a drain or the end: on-arrival rules take the advances too, as nothing arrives after them
	add       *App  // mAdd
}

type offer struct {
	rule int
	key  partition.Key // Rest fresh per event, read-only (design s3-2b-2 §2.6)
}

type outKind uint8

const (
	oEvent outKind = iota + 1
	oAck
	oStats
)

type outItem struct {
	kind    outKind
	app     AppID
	ev      event.Event
	token   uint64
	pending int64
	stats   *shardStats
	err     error // oAck of an mAdd: the shard's build error
}

type shardStats struct {
	outHigh [][]int64           // [app][rule]: the most outputs one message made the rule emit
	rules   [][]partition.Stats // [app][rule]
	emits   [][]plan.Stats      // [app][rule]: each body's plan.Counted, zero for a body that is not
	panics  [][]int64           // [app][rule]: the shard boundary's last-resort recoveries
	parks   int64
}

type handoff struct {
	shard int
	items []outItem
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

// build validates the apps and builds their state and every shard's rules; it starts nothing (Start and NewSync
// share it, design s3-2b-1 v2 §4).
func build(cfg Config, apps []App) (*Runtime, error) {
	if cfg.Shards <= 0 {
		cfg.Shards = goruntime.GOMAXPROCS(0)
	}
	if cfg.Mailbox <= 0 {
		cfg.Mailbox = 1024
	}
	if cfg.Intake <= 0 {
		cfg.Intake = 1024
	}
	if cfg.SinkQueue <= 0 {
		cfg.SinkQueue = 1024
	}
	if cfg.Now == nil {
		cfg.Now = func() int64 { return time.Now().UnixMilli() }
	}
	for i := range apps {
		if err := validateApp(i, &apps[i]); err != nil {
			return nil, err
		}
	}
	if cfg.RecordQueue <= 0 {
		cfg.RecordQueue = 65536
	}
	r := &Runtime{
		cfg: cfg, intake: make(chan item, cfg.Intake), handoff: make(chan handoff, cfg.Shards),
		stopped: make(chan struct{}),
	}
	// Config.Now is read once per app at its install (design s3-3-1 §2.4, MUST 10; S3.2a-2's §7.8: once at Start),
	// never for the empty runtime: the stamps' floor starts unset and the first install raises it.
	start := int64(math.MinInt64)
	if len(apps) > 0 {
		start = cfg.Now()
	}
	r.start = start
	var list []*appState
	for i := range apps {
		a := r.newAppState(AppID(i), apps[i], start)
		a.installRoutes()
		a.activated = true
		a.started.Store(true) // a built app is live from the start (NewSync)
		a.phase.Store(int32(phaseActive))
		list = append(list, a)
	}
	r.apps.Store(&list)
	for sid := range cfg.Shards {
		sh := newShard(sid, cfg.Mailbox)
		for _, a := range list {
			if err := sh.install(a.id, &a.app); err != nil {
				return nil, err
			}
		}
		r.shards = append(r.shards, sh)
	}
	return r, nil
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
		a.rec = newRecQ(app.Recorder, r.cfg.RecordQueue)
		a.rec.put(recItem{kind: rHeader, arrival: start}) // the header first (design s3-2b-2 §7)
	}
	for _, b := range app.Sinks {
		q := &sinkQ{q: make(chan sinkItem, r.cfg.SinkQueue), sink: b.Sink, returned: make(chan struct{})}
		a.sinks[b.Stream] = append(a.sinks[b.Stream], q)
		a.all = append(a.all, q)
	}
	return a
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

// validateApp checks an app against design §2 (review F9, F21; QA Q4).
func validateApp(i int, a *App) error {
	if len(a.Schemas) != len(a.Streams) {
		return fmt.Errorf("runtime: app %d: %d schemas for %d streams", i, len(a.Schemas), len(a.Streams))
	}
	for sid, sch := range a.Schemas {
		if sch.Stream() != a.Streams[sid].Name {
			return fmt.Errorf("runtime: app %d: schema %d describes stream %q, not %q", i, sid, sch.Stream(), a.Streams[sid].Name)
		}
	}
	for ri, rule := range a.Rules {
		bad := func(format string, args ...any) error {
			return fmt.Errorf("runtime: app %d rule %d (%q): %s", i, ri, rule.Name, fmt.Sprintf(format, args...))
		}
		switch {
		case len(rule.Inputs) == 0:
			return bad("reads no stream")
		case rule.NewBody == nil:
			return bad("has no body")
		case len(rule.KeyTypes) == 0:
			return bad("a key has at least one part (a key-less rule: one string part)")
		case rule.KeyAttrs != nil && len(rule.KeyAttrs) != len(rule.Inputs):
			return bad("%d key attribute lists for %d inputs", len(rule.KeyAttrs), len(rule.Inputs))
		case rule.KeyAttrs != nil && rule.TTL <= 0:
			return bad("a keyed rule needs a TTL above 0")
		case rule.KeyAttrs == nil && (rule.TTL != 0 || len(rule.KeyTypes) != 1 || rule.KeyTypes[0] != event.TString):
			return bad("a key-less rule has TTL 0 and the string pseudo-key")
		case slices.ContainsFunc(rule.KeyTypes, func(t event.Type) bool { return t < event.TString || t >= event.TObject }):
			return bad("key types %v: a part of these types cannot key a partition", rule.KeyTypes)
		}
		seen := map[int]bool{}
		for k, sid := range rule.Inputs {
			if sid < 0 || sid >= len(a.Streams) || a.Streams[sid].Derived {
				return bad("input %d is not a source stream", sid)
			}
			if seen[sid] {
				return bad("input %d is listed twice (break-it V2)", sid)
			}
			seen[sid] = true
			if rule.KeyAttrs == nil {
				continue
			}
			attrs := rule.KeyAttrs[k]
			if len(attrs) != len(rule.KeyTypes) {
				return bad("stream %d's key has %d parts, the rule's %d", sid, len(attrs), len(rule.KeyTypes))
			}
			for p, attr := range attrs {
				if attr < 0 || attr >= a.Schemas[sid].NumAttrs() || a.Schemas[sid].Attr(attr).Type != rule.KeyTypes[p] {
					return bad("key attribute %d of stream %d is not a %v attribute", attr, sid, rule.KeyTypes[p])
				}
			}
		}
	}
	for bi, b := range a.Sinks {
		if b.Stream < 0 || b.Stream >= len(a.Streams) || !a.Streams[b.Stream].Derived || b.Sink == nil {
			return fmt.Errorf("runtime: app %d sink %d: binds no derived stream, or has no sink", i, bi)
		}
	}
	return nil
}

// ---- callers ----

func (r *Runtime) isClosed() bool {
	select {
	case <-r.stopped:
		return true
	default:
		return false
	}
}

func (r *Runtime) appOf(app AppID) (*appState, error) {
	if r.isClosed() {
		return nil, ErrClosed
	}
	apps := r.appList()
	if app < 0 || int(app) >= len(apps) {
		return nil, fmt.Errorf("runtime: no app %d", app)
	}
	if apps[app] == nil || appPhase(apps[app].phase.Load()) == phaseRemoved {
		return nil, ErrRemoved
	}
	return apps[app], nil
}

// gate is every entry point's caller-side check (design s3-3-1 §2.5 MUST 3): the app's phase, then the drain flag.
func (r *Runtime) gate(app AppID) (*appState, error) {
	a, err := r.appOf(app)
	if err != nil {
		return nil, err
	}
	switch appPhase(a.phase.Load()) {
	case phaseInactive:
		return nil, ErrInactive
	case phaseDraining:
		return nil, ErrDrained // the phase changes before the sources are cancelled (design s3-3-1 §2.4, MUST 3)
	case phaseRemoved:
		return nil, ErrRemoved
	}
	if a.draining.Load() {
		return nil, ErrDrained
	}
	return a, nil
}

func (r *Runtime) enqueue(ctx context.Context, it item) error {
	if err := ctx.Err(); err != nil { // an expired context queues nothing: select would pick at random (QA Q15)
		return err
	}
	select {
	case r.intake <- it:
		return nil
	case <-r.stopped:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) request(ctx context.Context, it item) (reply, error) {
	it.reply = make(chan reply, 1)
	if err := r.enqueue(ctx, it); err != nil {
		return reply{}, err
	}
	return r.await(ctx, it.reply)
}

func (r *Runtime) await(ctx context.Context, ch chan reply) (reply, error) {
	select {
	case rep := <-ch:
		return rep, rep.err
	case <-r.stopped:
		return reply{}, ErrClosed
	case <-ctx.Done():
		return reply{}, ctx.Err()
	}
}

// streamStats is a's per-stream counters: the router's (late, after the drain) and the ingest step's.
func streamStats(a *appState) []StreamStats {
	out := make([]StreamStats, len(a.app.Streams))
	if a.ing == nil { // removed: zero rows of the app's length (design s3-3-1 MUST 2)
		return out
	}
	for i := range out {
		c := a.ing.counts[i]
		out[i] = StreamStats{Late: a.late[i], AfterDrain: a.after[i], Rejected: c.rejected, Unknown: c.unknown, FlagOverrides: c.overrides, Batches: a.batches[i]}
	}
	return out
}

// Send hands one source event of app to the intake; it blocks for backpressure and respects ctx. It checks the event
// first (review F9) and takes e.Values: the caller MUST NOT change them afterwards (review F4).
func (r *Runtime) Send(ctx context.Context, app AppID, e event.Event) error {
	a, err := r.gate(app)
	if err != nil {
		return err
	}
	if e.StreamID < 0 || e.StreamID >= len(a.app.Streams) || a.app.Streams[e.StreamID].Derived {
		return fmt.Errorf("runtime: app %d: stream %d is not a source stream", app, e.StreamID)
	}
	sch := a.app.Schemas[e.StreamID]
	if len(e.Values) != sch.NumAttrs() {
		return fmt.Errorf("runtime: app %d stream %q: %d values for %d attributes", app, sch.Stream(), len(e.Values), sch.NumAttrs())
	}
	for i, v := range e.Values {
		at := sch.Attr(i)
		switch {
		case v.Kind != at.Type:
			return fmt.Errorf("runtime: app %d stream %q: attribute %q holds a %v value; it is declared %v", app, sch.Stream(), at.Name, v.Kind, at.Type)
		case v.Null && !at.Optional:
			return fmt.Errorf("runtime: app %d stream %q: attribute %q is null, and it is not optional (break-it V1)", app, sch.Stream(), at.Name)
		case v.Null && i == sch.TimeAttr():
			// the decoder refuses a null time attribute (event.DecodeNull); the Go boundary agrees, so no event enters
			// at timestamp 0 (S3.3-1's break-it S1)
			return fmt.Errorf("runtime: app %d stream %q: the time attribute %q is null", app, sch.Stream(), at.Name)
		case v.Kind == event.TFloat && !v.Null && (math.IsNaN(v.F) || math.IsInf(v.F, 0)):
			return fmt.Errorf("runtime: app %d stream %q: attribute %q is %v: NaN and infinities are never values (S3.1-2's D2)", app, sch.Stream(), at.Name, v.F)
		}
		if err := recordable(v); err != nil {
			return fmt.Errorf("runtime: app %d stream %q: attribute %q %v (owner D1 of S3.2b-2's design)", app, sch.Stream(), at.Name, err)
		}
	}
	if ta := sch.TimeAttr(); ta != event.ArrivalTime && e.Ts != e.Values[ta].T {
		return fmt.Errorf("runtime: app %d stream %q: Ts %d differs from the time attribute %q, %d: an event-time stream's time is its attribute (S3.2a-1's D2; owner D1 of S3.2b-2's design)",
			app, sch.Stream(), e.Ts, sch.Attr(ta).Name, e.Values[ta].T)
	}
	if lineBound(sch, e) > event.MaxLogLine {
		return fmt.Errorf("runtime: app %d stream %q: the event could not fit one recording line of %d bytes (owner D1 of S3.2b-2's design)", app, sch.Stream(), event.MaxLogLine)
	}
	return r.enqueue(ctx, item{kind: iEvent, app: app, ev: e})
}

// CheckRows is TableWrite's per-row check, shared with the facade so a deploy's starting rows are refused at Deploy,
// not at Start (S3.3-1's break-it B5): every value a recording can copy exactly (owner D1 of S3.2b-2's design), every
// row within one recording line.
func CheckRows(table string, attrs []event.Attr, rows [][]event.Value) error {
	for ri, row := range rows {
		for ci, v := range row {
			name := fmt.Sprint(ci)
			if ci < len(attrs) {
				name = fmt.Sprintf("%d %q", ci, attrs[ci].Name)
			}
			if v.Kind == event.TFloat && !v.Null && (math.IsNaN(v.F) || math.IsInf(v.F, 0)) {
				return fmt.Errorf("row %d: attribute %s: the float %v is not finite (S3.1-2's D2)", ri, name, v.F)
			}
			if err := recordable(v); err != nil {
				return fmt.Errorf("row %d: attribute %s %v (owner D1 of S3.2b-2's design)", ri, name, err)
			}
		}
		if rowBound(table, attrs, row) > event.MaxLogLine {
			return fmt.Errorf("row %d could not fit one recording line of %d bytes (owner D1 of S3.2b-2's design)", ri, event.MaxLogLine)
		}
	}
	return nil
}

// Closed says Close has begun: the facade checks it before reading a program (§3.7: Deploy after Close is ErrClosed).
func (r *Runtime) Closed() bool { return r.isClosed() }

// Tick hands a wall tick to the intake, behind every event already there.
func (r *Runtime) Tick(ctx context.Context, now int64) error {
	if !event.TimestampInRange(now) {
		return fmt.Errorf("runtime: Tick at %d ms: outside the years 0000-9999, which a recording cannot write (owner D1 of S3.2b-2's design; break-it S2)", now)
	}
	if r.isClosed() {
		return ErrClosed
	}
	return r.enqueue(ctx, item{kind: iTick, now: now})
}

// TableWrite applies rows to one of app's tables in intake order (owner F14) and returns once the router has applied
// them: put rows are whole rows; delete rows are key values, in key order.
func (r *Runtime) TableWrite(ctx context.Context, app AppID, tableID int, op TableOp, rows [][]event.Value) error {
	a, err := r.gate(app)
	if err != nil {
		return err
	}
	if a.app.Tables == nil || tableID < 0 || tableID >= a.app.Tables.Len() || (op != TablePut && op != TableDelete) {
		return fmt.Errorf("runtime: app %d: no table %d, or an unknown operation", app, tableID)
	}
	tb := a.app.Tables.Table(tableID)
	if err := CheckRows(tb.Name(), tb.Attrs(), rows); err != nil {
		return fmt.Errorf("runtime: app %d table %d: %w", app, tableID, err)
	}
	_, err = r.request(ctx, item{kind: iWrite, app: app, table: tableID, op: op, rows: rows})
	return err
}

// Stats is a consistent cut of the counters, taken through the intake (design §6).
func (r *Runtime) Stats(ctx context.Context) (Stats, error) {
	if r.isClosed() {
		return Stats{}, ErrClosed
	}
	rep, err := r.request(ctx, item{kind: iStats})
	return rep.stats, err
}

// tick is the internal ticker (Config.TickEvery): a wall tick into the intake, behind the events before it.
func (r *Runtime) tick() {
	defer r.wg.Done()
	t := time.NewTicker(r.cfg.TickEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			select {
			case r.intake <- item{kind: iTick, now: r.cfg.Now()}:
			case <-r.stopped:
				return
			}
		case <-r.stopped:
			return
		}
	}
}

// ---- the router ----

type send struct {
	shard int // -1: a sink send
	msg   message
	sink  *sinkQ
	si    sinkItem
}

type drainWait struct {
	app       *appState
	acks      int
	pending   int64
	replies   []chan reply  // the Drain calls waiting for this drain's result
	recMarker chan struct{} // closed once the recording's stop is written; nil: no recording, or it ended
}

type statsWait struct {
	acks  int
	stats Stats
	reply chan reply
}

// installWait is a pending mAdd or mRemove (design s3-3-1 §2.3): acked by every shard, then completed in take.
type installWait struct {
	app      *appState
	acks     int
	err      error      // the first shard error of an add
	reply    chan reply // the caller's
	remove   bool       // an mRemove (or an add's rollback)
	rollback bool       // the remove undoes a failed add: reply err, not nil
}

type router struct {
	r           *Runtime
	pending     []send
	pendingHigh int
	mailboxHigh []int
	stamp       int64
	token       uint64
	drains      map[uint64]*drainWait
	installs    map[uint64]*installWait
	stats       map[uint64]*statsWait
	offers      [][]offer // per shard, reused: a message's offers are a view into it until delivered (review F4)
	// deliver hands one shard its message; the message's adv and offers are views valid only during the call. Live:
	// send, which copies them; Sync: the shard's step, in place.
	deliver func(shard int, m message)
}

func newRouter(r *Runtime) *router {
	return &router{
		r: r, mailboxHigh: make([]int, len(r.shards)), stamp: r.start, drains: map[uint64]*drainWait{},
		installs: map[uint64]*installWait{}, stats: map[uint64]*statsWait{}, offers: make([][]offer, len(r.shards)),
	}
}

// send is the live deliver: the message crosses goroutines, so its views are copied.
func (rt *router) send(shard int, m message) {
	m.adv = append([]clock.WMAdvance(nil), m.adv...)
	if len(m.offers) > 0 {
		m.offers = append([]offer(nil), m.offers...)
	}
	rt.push(send{shard: shard, msg: m})
}

func (r *Runtime) route(rt *router) {
	defer r.wg.Done()
	for {
		if r.stopping() {
			return
		}
		if len(rt.pending) > 0 {
			head := &rt.pending[0]
			if head.shard >= 0 {
				mb := r.shards[head.shard].mailbox
				select {
				case mb <- head.msg:
					rt.mailboxHigh[head.shard] = max(rt.mailboxHigh[head.shard], len(mb))
					rt.pop()
				case h := <-r.handoff:
					rt.take(h)
				case <-r.stopped:
					return
				}
			} else {
				select {
				case head.sink.q <- head.si:
					head.sink.high = max(head.sink.high, len(head.sink.q))
					rt.pop()
				case h := <-r.handoff:
					rt.take(h)
				case <-r.stopped:
					return
				}
			}
			continue
		}
		select {
		case it := <-r.intake:
			rt.ingest(it)
		case h := <-r.handoff:
			rt.take(h)
		case <-r.stopped:
			return
		}
	}
}

func (rt *router) push(s send) {
	rt.pending = append(rt.pending, s)
	rt.pendingHigh = max(rt.pendingHigh, len(rt.pending))
}

func (rt *router) pop() {
	rt.pending[0] = send{}
	rt.pending = rt.pending[1:]
	if len(rt.pending) == 0 {
		rt.pending = nil
	}
}

// now is the monotone wall stamp shared by arrivals and ticks (review F6).
func (rt *router) now(t int64) int64 {
	rt.stamp = max(rt.stamp, t)
	return rt.stamp
}

func (rt *router) ingest(it item) {
	r := rt.r
	switch it.kind {
	case iAdd:
		rt.add(it)
	case iRemove:
		rt.remove(it)
	case iActivate:
		a := rt.app(it.app)
		switch {
		case a == nil || a.ing == nil || a.removing:
			it.reply <- reply{err: ErrRemoved}
		case a.drained:
			it.reply <- reply{err: ErrDrained}
		case a.activated:
			it.reply <- reply{err: ErrStarted}
		default:
			a.activated = true
			a.started.Store(true)
			it.reply <- reply{}
		}
	case iQuiet:
		if a := rt.app(it.app); a != nil {
			a.quiet = true // the ticks stop before the sources (S3.1-1's D3; design s3-3-1 §2.6)
		}
	case iBatchEnd:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			return
		}
		if a.drained {
			a.after[it.stream]++
			return
		}
		a.batches[it.stream]++
		a.rec.put(recItem{kind: rBatchEnd, stream: it.stream, batch: it.batch})
	case iEvent:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			return
		}
		if a.drained {
			a.after[it.ev.StreamID]++
			return
		}
		stamp := rt.now(r.cfg.Now())
		if a.app.Schemas[it.ev.StreamID].TimeAttr() == event.ArrivalTime {
			it.ev.Ts = stamp // live stamping at ingest (item 3; design s3-2b-2 §4, MUST 6)
		}
		rt.event(a, &it.ev, stamp, it.info)
		// queued after the ingest step, still in intake order: the step writes the flag into a declared text_invisibles
		// slot, and the channel hand-off orders that write before the recorder goroutine reads the event (a race the
		// race row caught at T4)
		a.rec.put(recItem{kind: rEvent, ev: it.ev, arrival: stamp, payload: it.payload})
	case iReject:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			return
		}
		if a.drained {
			a.after[it.stream]++
			return
		}
		stamp := rt.now(r.cfg.Now())
		a.rec.put(recItem{kind: rRejected, stream: it.stream, arrival: stamp, fault: it.fault})
		a.ing.Reject(it.stream, stamp)
	case iTick:
		stamp := rt.now(it.now)
		for _, a := range r.appList() {
			if a == nil || a.ing == nil || !a.activated || a.drained || a.quiet { // the clock starts at Start (owner, 2026-11-08)
				continue
			}
			a.rec.put(recItem{kind: rTick, arrival: stamp}) // the raised stamp: what the ingest step uses (QA G6)
			rt.tickApp(a, stamp)
		}
	case iWrite:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			it.reply <- reply{err: ErrRemoved}
			return
		}
		if a.drained {
			it.reply <- reply{err: ErrDrained}
			return
		}
		err := a.write(it.table, it.op, it.rows)
		if err == nil {
			for _, row := range copyRows(it.rows) { // one record per row: Records counts lines (break-it S1)
				a.rec.put(recItem{kind: rTable, table: it.table, op: it.op, rows: [][]event.Value{row}})
			}
		}
		it.reply <- reply{err: err}
	case iDrain:
		a := rt.app(it.app)
		if a == nil || a.ing == nil {
			it.reply <- reply{err: ErrRemoved}
			return
		}
		if a.drained { // a retried Drain waits for the same flush (owner, 2026-11-05)
			if a.drainRes != nil {
				it.reply <- *a.drainRes
			} else {
				d := rt.drains[a.drainTok]
				d.replies = append(d.replies, it.reply)
			}
			return
		}
		if !a.activated { // never activated: nothing was delivered, nothing recorded (design s3-3-1 §2.4, MUST 8)
			a.drained = true // the queued header is never written: no recorder goroutine runs (a.rec is a caller's to read)
			res := reply{}
			a.drainRes = &res
			it.reply <- res
			return
		}
		a.drained = true
		rt.token++
		a.drainTok = rt.token
		dw := &drainWait{app: a, replies: []chan reply{it.reply}}
		if a.rec.put(recItem{kind: rStop}) {
			if m := make(chan struct{}); a.rec.put(recItem{kind: rMarker, marker: m}) {
				dw.recMarker = m // the drain waits until the stop is written (design s3-2b-2 §7)
			}
		}
		rt.drains[rt.token] = dw
		rt.drainApp(a, rt.token)
	case iStats:
		rt.token++
		st := Stats{PendingHigh: rt.pendingHigh, MailboxHigh: append([]int(nil), rt.mailboxHigh...)}
		for _, a := range r.appList() {
			if a == nil { // never installed here
				st.Streams, st.Recording, st.Rules = append(st.Streams, nil), append(st.Recording, RecordingStats{}), append(st.Rules, nil)
				continue
			}
			st.Streams = append(st.Streams, streamStats(a)) // a removed app: zero rows of its length (MUST 2)
			st.Recording = append(st.Recording, a.rec.stats())
			rules := make([]RuleStats, len(a.app.Rules))
			for i := range rules {
				rules[i].KeyErrors = a.keyErr[i]
			}
			st.Rules = append(st.Rules, rules)
		}
		st.SaturatedParks = make([]int64, len(r.shards))
		rt.stats[rt.token] = &statsWait{stats: st, reply: it.reply}
		for sid := range r.shards {
			rt.push(send{shard: sid, msg: message{kind: mStats, token: rt.token}})
		}
	}
}

// event is design §4: the ingest step (stamping e in place), the offers by key, and one message per shard, each a
// view delivered before the next step (design s3-2b-1 v2 §4). The caller checks the app is not drained and gives the
// arrival stamp. adv is the ingest step's, valid until its next call.
func (rt *router) event(a *appState, e *event.Event, arrival int64, info event.Decoded) (late bool, adv []clock.WMAdvance) {
	reg := a.ing.Registry()
	before := reg.Admission(e.StreamID)
	late, adv = a.ing.Event(e, arrival, info)
	if late {
		a.late[e.StreamID]++
		return true, adv
	}
	adm := reg.Admission(e.StreamID)
	tables, _ := e.Tables.(*table.Snapshot) // the event's own snapshot: its walk's timers read it too (D5)
	for i := range rt.offers {
		rt.offers[i] = rt.offers[i][:0]
	}
	for _, rr := range a.byStream[e.StreamID] {
		key, ok := routeKey(rr.attrs, e.Values)
		if !ok {
			a.keyErr[rr.rule]++ // a null or NaN part: this rule's key error only (SEMANTICS §9, §10)
			continue
		}
		sid := rt.shardOf(a.id, rr.rule, rr.attrs != nil, key)
		rt.offers[sid] = append(rt.offers[sid], offer{rule: rr.rule, key: key})
	}
	for sid := range rt.r.shards {
		if len(rt.offers[sid]) == 0 && len(adv) == 0 && adm == before {
			continue
		}
		m := message{kind: mWalk, app: a.id, adv: adv, tables: tables, admStream: e.StreamID, adm: adm}
		if len(rt.offers[sid]) > 0 {
			m.ev, m.offers = *e, rt.offers[sid]
		}
		rt.deliver(sid, m)
	}
	return false, adv
}

// tickApp is one app's idle tick, shared by the router and Sync (combined review C4 of S3.2b-1).
func (rt *router) tickApp(a *appState, stamp int64) []clock.WMAdvance {
	adv := a.ing.Tick(stamp)
	if len(adv) > 0 {
		rt.broadcast(a, mWalk, adv, a.ing.Snapshot(), 0, false)
	}
	return adv
}

// drainApp ends the app's input and sends every shard the drain barrier's message, shared by the router and Sync.
func (rt *router) drainApp(a *appState, token uint64) []clock.WMAdvance {
	a.drained = true
	adv := a.ing.Drain()
	rt.broadcast(a, mDrain, adv, a.ing.Snapshot(), token, true)
	return adv
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

// broadcast delivers one advance-only (or drain) message to every shard, with the app's snapshot (owner D5 of
// S3.2a-1); final marks a drain or the end, whose advances on-arrival rules take too (owner D1 of S3.2b-1).
func (rt *router) broadcast(a *appState, kind msgKind, adv []clock.WMAdvance, tables *table.Snapshot, token uint64, final bool) {
	for sid := range rt.r.shards {
		rt.deliver(sid, message{kind: kind, app: a.id, adv: adv, tables: tables, token: token, admStream: -1, final: final})
	}
}

// routeKey is an event's key for a route (design s3-2b-2 §2.5): part i is the event's value of attrs[i]; ok is false
// when a part is null or a NaN float. A float zero part is stored as +0.0 (one key, one shard); the router is the only
// writer of a key's parts (MUST 16). A key-less route's key is the pseudo-key "". Rest is fresh per event: the
// message owns it (ADR-0010 §4).
func routeKey(attrs []int, values []event.Value) (partition.Key, bool) {
	if attrs == nil {
		return partition.Single(event.Value{Kind: event.TString}), true
	}
	part := func(attr int) (event.Value, bool) {
		v := values[attr]
		if v.Null || (v.Kind == event.TFloat && math.IsNaN(v.F)) {
			return v, false
		}
		if v.Kind == event.TFloat && v.F == 0 {
			v.F = 0 // -0.0 is +0.0
		}
		return v, true
	}
	first, ok := part(attrs[0])
	if !ok {
		return partition.Key{}, false
	}
	key := partition.Single(first)
	if len(attrs) > 1 {
		key.Rest = make([]event.Value, len(attrs)-1)
		for i, attr := range attrs[1:] {
			if key.Rest[i], ok = part(attr); !ok {
				return partition.Key{}, false
			}
		}
	}
	return key, true
}

// shardOf is fnv-1a over (app, rule, the key's parts in order): a key-less rule hashes (app, rule). A single-part key
// hashes as it did before multi-part keys.
func (rt *router) shardOf(app AppID, rule int, keyed bool, key partition.Key) int {
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(app))
	h.Write(b[:])
	binary.LittleEndian.PutUint64(b[:], uint64(rule))
	h.Write(b[:])
	if keyed {
		for i := range key.Parts() { // inline: a hasher passed on as an interface escapes, one allocation per event
			key := key.Part(i)
			h.Write([]byte{byte(key.Kind)})
			switch key.Kind {
			case event.TString:
				h.Write([]byte(key.Str))
			case event.TInt:
				binary.LittleEndian.PutUint64(b[:], uint64(key.I))
				h.Write(b[:])
			case event.TTimestamp:
				binary.LittleEndian.PutUint64(b[:], uint64(key.T))
				h.Write(b[:])
			case event.TDuration:
				binary.LittleEndian.PutUint64(b[:], uint64(key.D))
				h.Write(b[:])
			case event.TFloat:
				binary.LittleEndian.PutUint64(b[:], math.Float64bits(key.F))
				h.Write(b[:])
			case event.TBool:
				if key.B {
					h.Write([]byte{1})
				} else {
					h.Write([]byte{0})
				}
			}
		}
	}
	return int(h.Sum64() % uint64(len(rt.r.shards)))
}

// take routes a handoff's items in order: outputs to their sinks, then the drain acks and stats replies.
func (rt *router) take(h handoff) {
	r := rt.r
	for _, it := range h.items {
		switch it.kind {
		case oEvent:
			if a := rt.app(it.app); a != nil && a.ing != nil {
				for _, q := range a.sinks[it.ev.StreamID] {
					rt.push(send{shard: -1, sink: q, si: sinkItem{ev: it.ev}})
				}
			}
		case oAck:
			if w := rt.installs[it.token]; w != nil {
				rt.installAck(w, it)
				continue
			}
			d := rt.drains[it.token]
			d.acks++
			d.pending += it.pending
			if d.acks < len(r.shards) {
				continue
			}
			delete(rt.drains, it.token)
			var markers []chan struct{}
			for _, q := range d.app.all {
				m := make(chan struct{})
				markers = append(markers, m)
				rt.push(send{shard: -1, sink: q, si: sinkItem{marker: m}})
			}
			res := reply{pending: d.pending, markers: markers, recMarker: d.recMarker}
			d.app.drainRes = &res
			for _, ch := range d.replies {
				ch <- res
			}
		case oStats:
			w := rt.stats[it.token]
			w.acks++
			for ai := range it.stats.rules {
				for ri, ps := range it.stats.rules[ai] {
					rs := &w.stats.Rules[ai][ri]
					rs.ConstructLate += int64(ps.ConstructLate)
					rs.Purged += int64(ps.Purged)
					rs.Panics += int64(ps.Panics) + it.stats.panics[ai][ri]
					rs.Suppressed += int64(it.stats.emits[ai][ri].Suppressed)
					rs.Errored += int64(it.stats.emits[ai][ri].Errored)
					rs.OutboxHigh = max(rs.OutboxHigh, it.stats.outHigh[ai][ri])
				}
			}
			w.stats.SaturatedParks[h.shard] = it.stats.parks
			if w.acks < len(r.shards) {
				continue
			}
			delete(rt.stats, it.token)
			for _, a := range r.appList() {
				if a == nil || a.ing == nil {
					w.stats.SinkErrors, w.stats.SinkHigh = append(w.stats.SinkErrors, nil), append(w.stats.SinkHigh, nil)
					continue
				}
				errs := make([]int64, len(a.all))
				for i, q := range a.all {
					errs[i] = q.errors.Load()
				}
				w.stats.SinkErrors = append(w.stats.SinkErrors, errs)
				high := make([]int, len(a.all))
				for i, q := range a.all {
					high[i] = q.high
				}
				w.stats.SinkHigh = append(w.stats.SinkHigh, high)
			}
			w.reply <- reply{stats: w.stats}
		}
	}
}

// ---- shards ----

type shard struct {
	id      int
	mailbox chan message
	rules   [][]*partition.Rule // [app][rule]
	bodies  [][]partition.Body  // [app][rule]: for their plan.Counted
	arrival [][]bool            // [app][rule]: an on-arrival rule (owner Q1 and D1 of S3.2b)
	reads   [][][]int           // [app][stream]: the rules reading it
	panics  [][]int64           // [app][rule]
	outbox  []outItem
	parks   int64
	burst   [][]int64 // [app][rule]: outputs emitted by the message being processed
	outHigh [][]int64 // [app][rule]: the most outputs one message made the rule emit
	touched [][2]int  // the (app, rule) pairs that emitted during the message: reset after it
}

func newShard(id, mailbox int) *shard { return &shard{id: id, mailbox: make(chan message, mailbox)} }

// grow makes the shard's per-app slices hold slot app (nil slots for ids never installed here).
func (sh *shard) grow(app AppID) {
	for int(app) >= len(sh.rules) {
		sh.rules, sh.bodies, sh.arrival = append(sh.rules, nil), append(sh.bodies, nil), append(sh.arrival, nil)
		sh.reads, sh.panics = append(sh.reads, nil), append(sh.panics, nil)
		sh.burst, sh.outHigh = append(sh.burst, nil), append(sh.outHigh, nil)
	}
}

// install builds the shard's slot for app (design s3-3-1 §2.4: each shard builds its own state on mAdd); an error
// leaves the slot freed.
func (sh *shard) install(app AppID, a *App) (err error) {
	sh.grow(app)
	defer func() {
		if err != nil {
			sh.uninstall(app)
		}
	}()
	ai := int(app)
	rules := make([]*partition.Rule, len(a.Rules))
	bodies := make([]partition.Body, len(a.Rules))
	arrival := make([]bool, len(a.Rules))
	reads := make([][]int, len(a.Streams))
	sh.burst[ai], sh.outHigh[ai] = make([]int64, len(a.Rules)), make([]int64, len(a.Rules))
	for ri, rule := range a.Rules {
		streams, name := a.Streams, rule.Name
		emit := func(e event.Event) {
			if e.StreamID < 0 || e.StreamID >= len(streams) || !streams[e.StreamID].Derived {
				// a body's fault: partition recovers it and resets the key (owner F10); the router never sees it
				// (break-it B1)
				panic(fmt.Sprintf("runtime: rule %q emitted on stream %d, which is not a derived stream of app %d", name, e.StreamID, app))
			}
			sh.outbox = append(sh.outbox, outItem{kind: oEvent, app: app, ev: e})
			if sh.burst[ai][ri]++; sh.burst[ai][ri] == 1 {
				sh.touched = append(sh.touched, [2]int{ai, ri})
			}
		}
		body, err := newBody(rule, emit)
		if err != nil {
			return fmt.Errorf("runtime: app %d rule %d (%q): %w", app, ri, rule.Name, err)
		}
		pr, err := partition.NewRule(partition.Config{Inputs: rule.Inputs, KeyTypes: rule.KeyTypes, TTL: rule.TTL, Body: body, OnArrival: rule.OnArrival})
		if err != nil {
			return fmt.Errorf("runtime: app %d rule %d (%q): %w", app, ri, rule.Name, err)
		}
		rules[ri], bodies[ri], arrival[ri] = pr, body, rule.OnArrival
		for _, sid := range rule.Inputs {
			reads[sid] = append(reads[sid], ri)
		}
	}
	sh.rules[ai], sh.bodies[ai], sh.arrival[ai], sh.reads[ai] = rules, bodies, arrival, reads
	sh.panics[ai] = make([]int64, len(rules))
	return nil
}

// uninstall frees the slot; indexes never shift (design s3-3-1 §2.4).
func (sh *shard) uninstall(app AppID) {
	sh.grow(app)
	ai := int(app)
	sh.rules[ai], sh.bodies[ai], sh.arrival[ai], sh.reads[ai] = nil, nil, nil, nil
	sh.panics[ai], sh.burst[ai], sh.outHigh[ai] = nil, nil, nil
}

// newBody calls a rule's NewBody; its panic is Start's error, never its caller's panic.
func newBody(rule Rule, emit Emit) (b partition.Body, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("NewBody panicked: %v", p)
		}
	}()
	return rule.NewBody(emit), nil
}

// stopping says Close has begun: a loop checks it first, so nothing starts after stop (break-it C1).
func (r *Runtime) stopping() bool { return r.isClosed() }

// run is a shard's loop: it waits only on its mailbox and, with a non-empty outbox, the handoff (design §3).
func (sh *shard) run(r *Runtime) {
	defer r.wg.Done()
	for {
		if r.stopping() {
			return
		}
		var ho chan handoff
		if len(sh.outbox) > 0 {
			ho = r.handoff
			if len(sh.mailbox) == cap(sh.mailbox) {
				sh.parks++ // the moment exit 3 must reach: a full mailbox while the handoff is pending (review F16)
			}
		}
		select {
		case m := <-sh.mailbox:
			sh.process(m)
		case ho <- handoff{shard: sh.id, items: sh.outbox}:
			sh.outbox = nil
		case <-r.stopped:
			return
		}
	}
}

// process is design §4's shard step: apply, walk the on-arrival rules offered, offer, then walk every rule of the app
// with the message's snapshot (design s3-2b-1 v2 §3). An on-arrival rule's clock takes only its stream's admission
// watermark, and a drain's or the end's advances (owner D1 of S3.2b-1).
func (sh *shard) process(m message) {
	defer sh.foldBursts()
	switch m.kind {
	case mStats:
		sh.outbox = append(sh.outbox, outItem{kind: oStats, token: m.token, stats: sh.stats()})
		return
	case mAdd:
		err := sh.install(m.app, m.add)
		sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token, err: err})
		return
	case mRemove:
		sh.uninstall(m.app)
		sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token})
		return
	}
	rules, arrival := sh.rules[m.app], sh.arrival[m.app]
	if rules == nil { // a message for a slot this shard freed (a remove raced a drain's last advances): nothing to walk
		if m.kind == mDrain {
			sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token})
		}
		return
	}
	for _, a := range m.adv {
		for _, ri := range sh.reads[m.app][a.StreamID] {
			if !arrival[ri] || m.final {
				rules[ri].Apply(a)
			}
		}
	}
	if m.admStream >= 0 && m.adm != math.MinInt64 {
		adv := clock.WMAdvance{StreamID: m.admStream, Contribution: m.adm, Moved: true}
		for _, ri := range sh.reads[m.app][m.admStream] {
			if arrival[ri] {
				rules[ri].Apply(adv)
			}
		}
	}
	for _, o := range m.offers {
		if arrival[o.rule] { // a purge that fell due before this event comes first (design s3-2b-1 v2 §3)
			sh.advance(m.app, o.rule, m.tables)
		}
	}
	for _, o := range m.offers {
		sh.offer(m.app, o, m.ev)
	}
	for ri := range rules {
		sh.advance(m.app, ri, m.tables)
	}
	if m.kind == mDrain {
		var pending int64
		for _, pr := range rules {
			pending += int64(pr.Pending())
		}
		sh.outbox = append(sh.outbox, outItem{kind: oAck, app: m.app, token: m.token, pending: pending})
	}
}

// stats is the shard's counters for a Stats cut.
// foldBursts ends a message: each rule's outputs from it become its high-water mark if higher (ADR-0006 §5).
func (sh *shard) foldBursts() {
	for _, p := range sh.touched {
		sh.outHigh[p[0]][p[1]] = max(sh.outHigh[p[0]][p[1]], sh.burst[p[0]][p[1]])
		sh.burst[p[0]][p[1]] = 0
	}
	sh.touched = sh.touched[:0]
}

func (sh *shard) stats() *shardStats {
	st := &shardStats{parks: sh.parks}
	for ai, rules := range sh.rules {
		ps := make([]partition.Stats, len(rules))
		es := make([]plan.Stats, len(rules))
		for ri, pr := range rules {
			ps[ri] = pr.Stats()
			if c, ok := sh.bodies[ai][ri].(plan.Counted); ok {
				es[ri] = c.Stats()
			}
		}
		if rules == nil { // a removed app: zero rows keep [app] indexes stable
			st.rules, st.emits, st.panics, st.outHigh = append(st.rules, nil), append(st.emits, nil), append(st.panics, nil), append(st.outHigh, nil)
			continue
		}
		st.rules = append(st.rules, ps)
		st.emits = append(st.emits, es)
		st.panics = append(st.panics, append([]int64(nil), sh.panics[ai]...))
		st.outHigh = append(st.outHigh, append([]int64(nil), sh.outHigh[ai]...))
	}
	return st
}

func (sh *shard) offer(app AppID, o offer, e event.Event) {
	defer func() {
		if recover() != nil {
			sh.panics[app][o.rule]++
		}
	}()
	sh.rules[app][o.rule].Offer(o.key, e)
}

// advance walks one rule; a panic partition did not catch is the shard boundary's last resort: counted, and the walk
// is resumed until it returns (each panic consumed one item; review F1), with a cap against a panic that consumes none.
func (sh *shard) advance(app AppID, ri int, tables *table.Snapshot) {
	for range 1 << 16 {
		if sh.tryAdvance(app, ri, tables) {
			return
		}
	}
}

func (sh *shard) tryAdvance(app AppID, ri int, tables *table.Snapshot) (done bool) {
	defer func() {
		if recover() != nil {
			sh.panics[app][ri]++
		}
	}()
	sh.rules[app][ri].Advance(tables)
	return true
}

// ---- sink flushers ----

// run is a flusher's loop: it waits only on its queue and its own sink (design §3); a marker is a close.
func (q *sinkQ) run(r *Runtime) {
	defer r.wg.Done()
	defer close(q.returned)
	for {
		if r.stopping() {
			return
		}
		select {
		case it := <-q.q:
			if r.stopping() { // stop won the race: drop the rest (design §6)
				return
			}
			if it.marker != nil {
				close(it.marker)
				if it.last { // Remove's last marker: nothing follows (design s3-3-1 §2.4; the queue is never closed)
					return
				}
				continue
			}
			q.call(it.ev)
		case <-r.stopped:
			return
		}
	}
}

// call delivers one event; an error or a panic is counted (review F8).
func (q *sinkQ) call(e event.Event) {
	defer func() {
		if recover() != nil {
			q.errors.Add(1)
		}
	}()
	if err := q.sink(e); err != nil {
		q.errors.Add(1)
	}
}

// copyRows is a table write's rows as the record's own: the caller may reuse its slices once TableWrite returns.
func copyRows(rows [][]event.Value) [][]event.Value {
	out := make([][]event.Value, len(rows))
	for i, row := range rows {
		out[i] = append([]event.Value(nil), row...)
	}
	return out
}
