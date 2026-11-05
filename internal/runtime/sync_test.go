package runtime_test

// Design note docs/sprints/s3-2b-1-design.md v1, §3 (the shard step: apply → walk → offers → walk; the on-arrival
// path), §4 (Sync, FromPlan, RuleStats.Suppressed/Errored from plan.Counted) and §7.5, §7.6, §8.6. Helpers are
// helpers_test.go's (rt*); Sync's stub panics, so syncNew recovers it into a failure naming the call.

import (
	goruntime "runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// syncNew is runtime.NewSync, a panic or an error failing the test.
func syncNew(t testing.TB, app runtime.App, start int64, virtual bool, out func(event.Event)) *runtime.Sync {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runtime.NewSync panicked: %v", r)
		}
	}()
	s, err := runtime.NewSync(app, start, virtual, out)
	if err != nil {
		t.Fatalf("NewSync: %v", err)
	}
	return s
}

// syncApp: S (source, `time arrival` schema with k; delay d), then the derived streams OA (an on-arrival rule's
// outputs) and OB (a buffered rule's timer outputs). Rule A: on arrival, keyed by k, TTL ttl, emits one OA event
// per event (k, its Seq). Rule B: buffered, keyed by k, TTL ttl, a Completion at Ts + 10 s per release, emitting
// one OB event (k, the timer's Ref) when it fires.
func syncApp(t testing.TB, d, ttl int64, log, births *rtLog) runtime.App {
	t.Helper()
	k := rtA("k", event.TString)
	n := rtA("n", event.TInt)
	return runtime.App{
		Streams: []clock.StreamSpec{rtSrc("S", d), rtDerived("OA"), rtDerived("OB")},
		Schemas: []event.Schema{rtSchema(t, "S", k), rtSchema(t, "OA", k, n), rtSchema(t, "OB", k, n)},
		Rules: []runtime.Rule{
			{Name: "A", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: ttl, OnArrival: true,
				NewBody: rtBodies(rtBody{rule: "A", log: log, births: births, onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
					b.emit(rtEv(1, e.Ts, in.Key, rtInt(int64(e.Seq))))
				}})},
			{Name: "B", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: ttl,
				NewBody: rtBodies(rtBody{rule: "B", sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + 10*rtSec, Kind: clock.Completion, Ref: e.Seq}}
				}, onTimers: func(b *rtBody, in *partition.Instance, fires []clock.Timer, tables any) {
					for _, f := range fires {
						b.emit(rtEv(2, f.Due, in.Key, rtInt(int64(f.Ref))))
					}
				}})},
		},
	}
}

// TestShardStepWalksBeforeOffers: design §3 and §7.5 — the shard step is apply → walk → offers → walk. Rule A (on
// arrival, TTL 1 min, S's delay 0): a@0 reaches the body at its own step; then a@2 min arrives: its advance moves the
// clock to 2 min, past a's TTL due (1 min), so the first walk purges a BEFORE the offer, and a@2 min starts a fresh
// instance (a second birth, Purged 1). With the old order (offers, then one walk) a@2 min would land in the old
// instance and re-arm its TTL: no purge. Live, with 1 and 3 shards; Stats is the barrier (it passes every shard).
func TestShardStepWalksBeforeOffers(t *testing.T) {
	t.Parallel()
	for _, shards := range []int{1, 3} {
		log, births := newRTLog(), newRTLog()
		app := syncApp(t, 0, 60*rtSec, log, births)
		app.Rules = app.Rules[:1]
		ctx := rtCeiling(t)
		r := rtStart(t, runtime.Config{Shards: shards}, app)
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(0), rtStr("a")))
		rtStats(t, ctx, r)
		if got := log.get("A/a"); !rtEqual(got, []string{"ev s0#1 @0"}) {
			t.Fatalf("shards %d: after a@0, A/a saw %v; want [ev s0#1 @0] at its own step (owner Q1)", shards, got)
		}
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(120), rtStr("a")))
		st := rtStats(t, ctx, r)
		if got := log.get("A/a"); !rtEqual(got, []string{"ev s0#1 @0", "purge", "ev s0#2 @120000"}) {
			t.Errorf("shards %d: A/a saw %v; want the purge before a@2 min (walk before offers)", shards, got)
		}
		if got := births.get("A/a"); len(got) != 2 {
			t.Errorf("shards %d: births %v, want 2 (a@2 min starts a fresh instance)", shards, got)
		}
		if rs := st.Rules[0][0]; rs.Purged != 1 || rs.ConstructLate != 0 {
			t.Errorf("shards %d: rule stats %+v, want Purged 1, ConstructLate 0", shards, rs)
		}
	}
}

// TestSyncStartsNoGoroutine: design §7.6 — Sync runs every step on the caller's goroutine and starts none; an
// output reaches out during the step that emits it (§4). Not parallel: the goroutine count is the process's, and
// the paused parallel tests hold theirs still. A goroutine Sync started would be counted at once; the bounded
// loop only lets goroutines of earlier tests finish exiting (time breaks the hang only).
func TestSyncStartsNoGoroutine(t *testing.T) {
	before := goruntime.NumGoroutine()
	var outs []event.Event
	s := syncNew(t, syncApp(t, 0, 60*rtSec, nil, nil), rtT0, false, func(e event.Event) { outs = append(outs, e) })
	if _, _ = s.Event(rtEv(0, rtAt(0), rtStr("a")), rtAt(0)); len(outs) != 1 {
		t.Fatalf("%d outputs after a's step, want 1: rule A emits during its own step", len(outs))
	}
	s.Event(rtEv(0, rtAt(20), rtStr("b")), rtAt(20)) // releases a@0 to B: its timer at 10 s fires at clock 20 s
	s.Tick(rtAt(30))
	s.Drain()
	after := goruntime.NumGoroutine()
	deadline := time.Now().Add(10 * time.Second)
	for after > before && time.Now().Before(deadline) {
		goruntime.Gosched()
		after = goruntime.NumGoroutine()
	}
	if after > before {
		t.Errorf("%d goroutines after Sync's steps, %d before: Sync started goroutines (design §7.6)", after, before)
	}
}

type syncIn struct {
	ev      event.Event
	arrival int64
}

// TestSyncMatchesLiveOneShard: design §4 — Sync is the live runtime's steps on one shard, so the same input gives
// the same outputs, per derived stream in order, and the same counters as runtime.Start with Shards 1. The input
// has out-of-order and stream-late events (S's delay 10 s), two keys, and purges (TTL 30 s).
func TestSyncMatchesLiveOneShard(t *testing.T) {
	t.Parallel()
	in := []syncIn{
		{rtEv(0, rtAt(0), rtStr("a")), rtAt(0)},
		{rtEv(0, rtAt(5), rtStr("b")), rtAt(1)},
		{rtEv(0, rtAt(2), rtStr("a")), rtAt(2)},
		{rtEv(0, rtAt(30), rtStr("b")), rtAt(3)},
		{rtEv(0, rtAt(1), rtStr("a")), rtAt(4)}, // stream-late: 1 s < 30 s − 10 s
		{rtEv(0, rtAt(25), rtStr("a")), rtAt(5)},
		{rtEv(0, rtAt(100), rtStr("c")), rtAt(6)}, // the clock to 90 s: purges a and b
		{rtEv(0, rtAt(101), rtStr("a")), rtAt(7)},
	}
	// Live.
	var mu sync.Mutex
	stamps := []int64{rtT0}
	for _, x := range in {
		stamps = append(stamps, x.arrival)
	}
	now := func() int64 {
		mu.Lock()
		defer mu.Unlock()
		if len(stamps) == 0 {
			t.Error("Config.Now read more than once at Start and once per event (design §7.8)")
			return rtT0
		}
		v := stamps[0]
		stamps = stamps[1:]
		return v
	}
	live := map[int][]event.Event{}
	var lmu sync.Mutex
	sink := func(e event.Event) error {
		lmu.Lock()
		live[e.StreamID] = append(live[e.StreamID], e)
		lmu.Unlock()
		return nil
	}
	lapp := syncApp(t, 10*rtSec, 30*rtSec, nil, nil)
	lapp.Sinks = []runtime.SinkBinding{{Stream: 1, Sink: sink}, {Stream: 2, Sink: sink}}
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 1, Now: now}, lapp)
	for _, x := range in {
		rtSend(t, ctx, r, 0, x.ev)
	}
	lres := rtDrain(t, ctx, r, 0)
	lst := rtStats(t, ctx, r)
	// Replay.
	replay := map[int][]event.Event{}
	s := syncNew(t, syncApp(t, 10*rtSec, 30*rtSec, nil, nil), rtT0, false, func(e event.Event) { replay[e.StreamID] = append(replay[e.StreamID], e) })
	for _, x := range in {
		s.Event(x.ev, x.arrival)
	}
	_, sres := s.Drain()
	sst := s.Stats()
	for _, sid := range []int{1, 2} {
		if !slices.EqualFunc(live[sid], replay[sid], syncSameOut) {
			t.Errorf("stream %d: live %v\nreplay %v", sid, live[sid], replay[sid])
		}
	}
	if len(live[1]) == 0 || len(live[2]) == 0 {
		t.Errorf("the input exercises neither rule: live %v", live)
	}
	if lres != sres {
		t.Errorf("drain: live %+v, replay %+v", lres, sres)
	}
	if !slices.Equal(lst.StreamLate[0], sst.StreamLate[0]) || lst.StreamLate[0][0] != 1 {
		t.Errorf("StreamLate: live %v, replay %v; want [1 0 0] both", lst.StreamLate, sst.StreamLate)
	}
	if !slices.Equal(lst.Rules[0], sst.Rules[0]) {
		t.Errorf("rule stats: live %+v, replay %+v", lst.Rules[0], sst.Rules[0])
	}
}

func syncSameOut(a, b event.Event) bool {
	return a.StreamID == b.StreamID && a.Ts == b.Ts && slices.Equal(a.Values, b.Values)
}

// qaCounted is a body that reports fixed plan.Stats: RuleStats sums them over the shards (design §4).
type qaCounted struct{ rtBody }

func (qaCounted) Stats() plan.Stats { return plan.Stats{Suppressed: 3, Errored: 2} }

// TestRuleStatsSumCounted: design §4 — RuleStats gains Suppressed and Errored, summed from each body's
// plan.Counted: one body per shard, so with 2 shards 6 and 4; Sync (one shard) 3 and 2.
func TestRuleStatsSumCounted(t *testing.T) {
	t.Parallel()
	app := syncApp(t, 0, 60*rtSec, nil, nil)
	app.Rules = app.Rules[:1]
	app.Rules[0].NewBody = func(emit runtime.Emit) partition.Body { return &qaCounted{rtBody{rule: "A", emit: emit}} }
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	if rs := rtStats(t, ctx, r).Rules[0][0]; rs.Suppressed != 6 || rs.Errored != 4 {
		t.Errorf("live, 2 shards: Suppressed %d, Errored %d; want 6, 4", rs.Suppressed, rs.Errored)
	}
	s := syncNew(t, app, rtT0, false, func(event.Event) {})
	if rs := s.Stats().Rules[0][0]; rs.Suppressed != 3 || rs.Errored != 2 {
		t.Errorf("Sync: Suppressed %d, Errored %d; want 3, 2", rs.Suppressed, rs.Errored)
	}
}

// TestFromPlan: design §4 — FromPlan maps plan.App onto runtime.App: per stream its spec (name, bound, idleness,
// derived) and schema; per rule its routing, TTL, OnArrival and body; the tables and sinks as given.
func TestFromPlan(t *testing.T) {
	t.Parallel()
	sch := rtSchema(t, "S", rtA("k", event.TString))
	osch := rtSchema(t, "O", rtA("k", event.TString))
	emitted := 0
	p := &plan.App{
		Streams: []plan.Stream{{Schema: sch, Delay: 5 * rtSec, IdleAfter: 60 * rtSec}, {Schema: osch, Derived: true}},
		Rules: []plan.Rule{{Name: "R", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: 60 * rtSec, OnArrival: true,
			NewBody: func(emit func(event.Event)) partition.Body {
				emit(rtEv(1, 0, rtStr("x")))
				return &rtBody{rule: "R"}
			}}},
	}
	sinks := []runtime.SinkBinding{{Stream: 1, Sink: func(event.Event) error { return nil }}}
	var app runtime.App
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("runtime.FromPlan panicked: %v", r)
			}
		}()
		app = runtime.FromPlan(p, nil, sinks)
	}()
	want := []clock.StreamSpec{{Name: "S", Delay: 5 * rtSec, IdleAfter: 60 * rtSec}, {Name: "O", Derived: true}}
	if !slices.Equal(app.Streams, want) || len(app.Schemas) != 2 || app.Schemas[0].Stream() != "S" || app.Schemas[1].Stream() != "O" {
		t.Errorf("streams %+v, schemas %v; want %+v and S, O", app.Streams, app.Schemas, want)
	}
	if len(app.Rules) != 1 {
		t.Fatalf("%d rules, want 1", len(app.Rules))
	}
	r := app.Rules[0]
	if r.Name != "R" || !slices.Equal(r.Inputs, []int{0}) || !slices.Equal(r.KeyAttr, []int{0}) || r.KeyType != event.TString || r.TTL != 60*rtSec || !r.OnArrival || r.NewBody == nil {
		t.Errorf("rule %+v; want R's routing, TTL 1 min, OnArrival and a body", r)
	}
	if len(app.Sinks) != 1 || app.Sinks[0].Stream != 1 {
		t.Errorf("sinks %+v, want the one given", app.Sinks)
	}
	if r.NewBody != nil {
		r.NewBody(func(event.Event) { emitted++ })
		if emitted != 1 {
			t.Errorf("the rule's NewBody did not pass emit through to plan's NewBody (%d emits)", emitted)
		}
	}
}
