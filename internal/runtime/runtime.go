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
	Suppressed, Errored                      int64 // from each body's plan.Counted (design s3-2b-1 §4)
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

// Runtime is a running engine core (design §2–§6).
type Runtime struct {
	cfg       Config
	apps      []*appState
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
	ing      *Ingest     // the router's
	byStream [][]route   // per stream: the rules reading it
	sinks    [][]*sinkQ  // per stream: its bindings
	all      []*sinkQ    // every binding, in order
	draining atomic.Bool // a caller's fast path (owner F5)
	drained  bool        // the router's, by intake order
	drainTok uint64      // the router's: the drain's token while its acks are due
	drainRes *reply      // the router's: the drain's result, once known; a retried Drain waits on it (owner)
	late     []int64     // per stream: stream_late (the router's)
	after    []int64     // per stream: refused after the drain (the router's)
	keyErr   []int64     // per rule: key_errors (the router's)
}

type route struct {
	rule int
	attr int // -1: key-less
}

type sinkQ struct {
	q      chan sinkItem
	sink   Sink
	errors atomic.Int64
}

type sinkItem struct {
	ev     event.Event
	marker chan struct{} // non-nil: a drain's flush marker, closed on reaching it
}

type itemKind uint8

const (
	iEvent itemKind = iota + 1
	iTick
	iWrite
	iDrain
	iStats
)

type item struct {
	kind  itemKind
	app   AppID
	ev    event.Event
	now   int64
	table int
	op    TableOp
	rows  [][]event.Value
	reply chan reply // buffered 1: the router never blocks on it
}

type reply struct {
	err     error
	pending int64
	markers []chan struct{}
	stats   Stats
}

type msgKind uint8

const (
	mWalk msgKind = iota + 1 // advances, and an event's offers
	mDrain
	mStats
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
}

type offer struct {
	rule int
	key  event.Value
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
}

type shardStats struct {
	rules  [][]partition.Stats // [app][rule]
	emits  [][]plan.Stats      // [app][rule]: each body's plan.Counted, zero for a body that is not
	panics [][]int64           // [app][rule]: the shard boundary's last-resort recoveries
	parks  int64
}

type handoff struct {
	shard int
	items []outItem
}

// Start validates the apps, builds every shard's rules, and starts the router, the shards and the sink flushers.
func Start(cfg Config, apps []App) (*Runtime, error) {
	r, err := build(cfg, apps)
	if err != nil {
		return nil, err
	}
	rt := newRouter(r)
	rt.deliver = rt.send // a message crosses goroutines: its views are copied
	r.wg.Add(1)
	go r.route(rt)
	for _, sh := range r.shards {
		r.wg.Add(1)
		go sh.run(r)
	}
	for _, a := range r.apps {
		for _, q := range a.all {
			r.wg.Add(1)
			go q.run(r)
		}
	}
	if r.cfg.TickEvery > 0 {
		r.wg.Add(1)
		go r.tick()
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
	r := &Runtime{
		cfg: cfg, intake: make(chan item, cfg.Intake), handoff: make(chan handoff, cfg.Shards),
		stopped: make(chan struct{}),
	}
	start := cfg.Now()
	for i := range apps {
		app := apps[i]
		a := &appState{
			id: AppID(i), app: app, ing: NewIngest(app.Streams, start, app.Tables, false),
			byStream: make([][]route, len(app.Streams)), sinks: make([][]*sinkQ, len(app.Streams)),
			late: make([]int64, len(app.Streams)), after: make([]int64, len(app.Streams)),
			keyErr: make([]int64, len(app.Rules)),
		}
		for ri, rule := range app.Rules {
			for k, sid := range rule.Inputs {
				attr := -1
				if rule.KeyAttr != nil {
					attr = rule.KeyAttr[k]
				}
				a.byStream[sid] = append(a.byStream[sid], route{rule: ri, attr: attr})
			}
		}
		for _, b := range app.Sinks {
			q := &sinkQ{q: make(chan sinkItem, cfg.SinkQueue), sink: b.Sink}
			a.sinks[b.Stream] = append(a.sinks[b.Stream], q)
			a.all = append(a.all, q)
		}
		r.apps = append(r.apps, a)
	}
	for sid := range cfg.Shards {
		sh, err := r.newShard(sid)
		if err != nil {
			return nil, err
		}
		r.shards = append(r.shards, sh)
	}
	return r, nil
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
		case rule.KeyAttr != nil && len(rule.KeyAttr) != len(rule.Inputs):
			return bad("%d key attributes for %d inputs", len(rule.KeyAttr), len(rule.Inputs))
		case rule.KeyAttr != nil && rule.TTL <= 0:
			return bad("a keyed rule needs a TTL above 0")
		case rule.KeyAttr == nil && (rule.TTL != 0 || rule.KeyType != event.TString):
			return bad("a key-less rule has TTL 0 and the string pseudo-key")
		case rule.KeyType < event.TString || rule.KeyType >= event.TObject:
			return bad("key type %v cannot key a partition", rule.KeyType)
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
			if rule.KeyAttr == nil {
				continue
			}
			attr := rule.KeyAttr[k]
			if attr < 0 || attr >= a.Schemas[sid].NumAttrs() || a.Schemas[sid].Attr(attr).Type != rule.KeyType {
				return bad("key attribute %d of stream %d is not a %v attribute", attr, sid, rule.KeyType)
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
	if app < 0 || int(app) >= len(r.apps) {
		return nil, fmt.Errorf("runtime: no app %d", app)
	}
	return r.apps[app], nil
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

// Send hands one source event of app to the intake; it blocks for backpressure and respects ctx. It checks the event
// first (review F9) and takes e.Values: the caller MUST NOT change them afterwards (review F4).
func (r *Runtime) Send(ctx context.Context, app AppID, e event.Event) error {
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if a.draining.Load() {
		return ErrDrained
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
		case v.Kind == event.TFloat && !v.Null && (math.IsNaN(v.F) || math.IsInf(v.F, 0)):
			return fmt.Errorf("runtime: app %d stream %q: attribute %q is %v: NaN and infinities are never values (S3.1-2's D2)", app, sch.Stream(), at.Name, v.F)
		}
	}
	return r.enqueue(ctx, item{kind: iEvent, app: app, ev: e})
}

// Tick hands a wall tick to the intake, behind every event already there.
func (r *Runtime) Tick(ctx context.Context, now int64) error {
	if r.isClosed() {
		return ErrClosed
	}
	return r.enqueue(ctx, item{kind: iTick, now: now})
}

// TableWrite applies rows to one of app's tables in intake order (owner F14) and returns once the router has applied
// them: put rows are whole rows; delete rows are key values, in key order.
func (r *Runtime) TableWrite(ctx context.Context, app AppID, tableID int, op TableOp, rows [][]event.Value) error {
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if a.draining.Load() {
		return ErrDrained
	}
	if a.app.Tables == nil || tableID < 0 || tableID >= a.app.Tables.Len() || (op != TablePut && op != TableDelete) {
		return fmt.Errorf("runtime: app %d: no table %d, or an unknown operation", app, tableID)
	}
	_, err = r.request(ctx, item{kind: iWrite, app: app, table: tableID, op: op, rows: rows})
	return err
}

// Drain runs the drain barrier for app (design §5): the app's input ends (owner F5); every output produced before the
// barrier has been through its sink when it returns. A second Drain of the app waits for the same flush and returns
// the same result (owner, 2026-11-05).
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
	a, err := r.appOf(app)
	if err != nil {
		return DrainResult{}, err
	}
	set := !a.draining.Swap(true)
	it := item{kind: iDrain, app: app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		if set { // nothing was queued, and no earlier Drain set the flag: the app is not drained (QA Q15; break-it R1)
			a.draining.Store(false)
		}
		return DrainResult{}, err
	}
	rep, err := r.await(ctx, it.reply)
	if err != nil {
		return DrainResult{}, err
	}
	for _, m := range rep.markers {
		select {
		case <-m:
		case <-r.stopped:
			return DrainResult{}, ErrClosed
		case <-ctx.Done():
			return DrainResult{}, ctx.Err()
		}
	}
	return DrainResult{Pending: rep.pending}, nil
}

// Stats is a consistent cut of the counters, taken through the intake (design §6).
func (r *Runtime) Stats(ctx context.Context) (Stats, error) {
	if r.isClosed() {
		return Stats{}, ErrClosed
	}
	rep, err := r.request(ctx, item{kind: iStats})
	return rep.stats, err
}

// Close stops every goroutine and drops what is left; it fires no pending timer. A second Close returns nil once
// everything has stopped. A Sink that never returns keeps its flusher: Close then returns ctx's error.
func (r *Runtime) Close(ctx context.Context) error {
	r.closeOnce.Do(func() { close(r.stopped) })
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	app     *appState
	acks    int
	pending int64
	replies []chan reply // the Drain calls waiting for this drain's result
}

type statsWait struct {
	acks  int
	stats Stats
	reply chan reply
}

type router struct {
	r           *Runtime
	pending     []send
	pendingHigh int
	mailboxHigh []int
	stamp       int64
	token       uint64
	drains      map[uint64]*drainWait
	stats       map[uint64]*statsWait
	offers      [][]offer // per shard, reused: a message's offers are a view into it until delivered (review F4)
	// deliver hands one shard its message; the message's adv and offers are views valid only during the call. Live:
	// send, which copies them; Sync: the shard's step, in place.
	deliver func(shard int, m message)
}

func newRouter(r *Runtime) *router {
	return &router{
		r: r, mailboxHigh: make([]int, len(r.shards)), stamp: math.MinInt64, drains: map[uint64]*drainWait{},
		stats: map[uint64]*statsWait{}, offers: make([][]offer, len(r.shards)),
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
	case iEvent:
		a := r.apps[it.app]
		if a.drained {
			a.after[it.ev.StreamID]++
			return
		}
		rt.event(a, &it.ev, rt.now(r.cfg.Now()))
	case iTick:
		stamp := rt.now(it.now)
		for _, a := range r.apps {
			if a.drained {
				continue
			}
			rt.tickApp(a, stamp)
		}
	case iWrite:
		a := r.apps[it.app]
		if a.drained {
			it.reply <- reply{err: ErrDrained}
			return
		}
		it.reply <- reply{err: a.write(it.table, it.op, it.rows)}
	case iDrain:
		a := r.apps[it.app]
		if a.drained { // a retried Drain waits for the same flush (owner, 2026-11-05)
			if a.drainRes != nil {
				it.reply <- *a.drainRes
			} else {
				d := rt.drains[a.drainTok]
				d.replies = append(d.replies, it.reply)
			}
			return
		}
		a.drained = true
		rt.token++
		a.drainTok = rt.token
		rt.drains[rt.token] = &drainWait{app: a, replies: []chan reply{it.reply}}
		rt.drainApp(a, rt.token)
	case iStats:
		rt.token++
		st := Stats{PendingHigh: rt.pendingHigh, MailboxHigh: append([]int(nil), rt.mailboxHigh...)}
		for _, a := range r.apps {
			st.StreamLate = append(st.StreamLate, append([]int64(nil), a.late...))
			st.AfterDrain = append(st.AfterDrain, append([]int64(nil), a.after...))
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
func (rt *router) event(a *appState, e *event.Event, arrival int64) (late bool, adv []clock.WMAdvance) {
	reg := a.ing.Registry()
	before := reg.Admission(e.StreamID)
	late, adv = a.ing.Event(e, arrival)
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
		key := event.Value{Kind: event.TString}
		if rr.attr >= 0 {
			key = e.Values[rr.attr]
			if key.Null || (key.Kind == event.TFloat && math.IsNaN(key.F)) {
				a.keyErr[rr.rule]++
				continue
			}
			if key.Kind == event.TFloat && key.F == 0 {
				key.F = 0 // -0.0 is +0.0: one key, one shard
			}
		}
		sid := rt.shardOf(a.id, rr.rule, rr.attr >= 0, key)
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

// shardOf is fnv-1a over (app, rule, the key): a key-less rule hashes (app, rule).
func (rt *router) shardOf(app AppID, rule int, keyed bool, key event.Value) int {
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(app))
	h.Write(b[:])
	binary.LittleEndian.PutUint64(b[:], uint64(rule))
	h.Write(b[:])
	if keyed {
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
	return int(h.Sum64() % uint64(len(rt.r.shards)))
}

// take routes a handoff's items in order: outputs to their sinks, then the drain acks and stats replies.
func (rt *router) take(h handoff) {
	r := rt.r
	for _, it := range h.items {
		switch it.kind {
		case oEvent:
			for _, q := range r.apps[it.app].sinks[it.ev.StreamID] {
				rt.push(send{shard: -1, sink: q, si: sinkItem{ev: it.ev}})
			}
		case oAck:
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
			res := reply{pending: d.pending, markers: markers}
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
				}
			}
			w.stats.SaturatedParks[h.shard] = it.stats.parks
			if w.acks < len(r.shards) {
				continue
			}
			delete(rt.stats, it.token)
			for _, a := range r.apps {
				errs := make([]int64, len(a.all))
				for i, q := range a.all {
					errs[i] = q.errors.Load()
				}
				w.stats.SinkErrors = append(w.stats.SinkErrors, errs)
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
}

func (r *Runtime) newShard(id int) (*shard, error) {
	sh := &shard{id: id, mailbox: make(chan message, r.cfg.Mailbox)}
	for _, a := range r.apps {
		rules := make([]*partition.Rule, len(a.app.Rules))
		bodies := make([]partition.Body, len(a.app.Rules))
		arrival := make([]bool, len(a.app.Rules))
		reads := make([][]int, len(a.app.Streams))
		for ri, rule := range a.app.Rules {
			app, streams, name := a.id, a.app.Streams, rule.Name
			emit := func(e event.Event) {
				if e.StreamID < 0 || e.StreamID >= len(streams) || !streams[e.StreamID].Derived {
					// a body's fault: partition recovers it and resets the key (owner F10); the router never sees it
					// (break-it B1)
					panic(fmt.Sprintf("runtime: rule %q emitted on stream %d, which is not a derived stream of app %d", name, e.StreamID, app))
				}
				sh.outbox = append(sh.outbox, outItem{kind: oEvent, app: app, ev: e})
			}
			body, err := newBody(rule, emit)
			if err != nil {
				return nil, fmt.Errorf("runtime: app %d rule %d (%q): %w", a.id, ri, rule.Name, err)
			}
			pr, err := partition.NewRule(partition.Config{Inputs: rule.Inputs, KeyType: rule.KeyType, TTL: rule.TTL, Body: body, OnArrival: rule.OnArrival})
			if err != nil {
				return nil, fmt.Errorf("runtime: app %d rule %d (%q): %w", a.id, ri, rule.Name, err)
			}
			rules[ri], bodies[ri], arrival[ri] = pr, body, rule.OnArrival
			for _, sid := range rule.Inputs {
				reads[sid] = append(reads[sid], ri)
			}
		}
		sh.rules = append(sh.rules, rules)
		sh.bodies = append(sh.bodies, bodies)
		sh.arrival = append(sh.arrival, arrival)
		sh.reads = append(sh.reads, reads)
		sh.panics = append(sh.panics, make([]int64, len(rules)))
	}
	return sh, nil
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
	if m.kind == mStats {
		sh.outbox = append(sh.outbox, outItem{kind: oStats, token: m.token, stats: sh.stats()})
		return
	}
	rules, arrival := sh.rules[m.app], sh.arrival[m.app]
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
		st.rules = append(st.rules, ps)
		st.emits = append(st.emits, es)
		st.panics = append(st.panics, append([]int64(nil), sh.panics[ai]...))
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
