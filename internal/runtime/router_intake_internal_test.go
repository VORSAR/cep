package runtime

// The router's intake dispatch, driven in place: a built runtime (no goroutines), its router, and ingest called on the
// test goroutine with each item kind. Internal: the router's state (drained, quiet, removing, the counters) is
// unexported, and several of these branches are reachable live only by a caller racing a Drain or a Remove.
// Prefix: ig.

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

type igWorld struct {
	r    *Runtime
	rt   *router
	a    *appState
	msgs int // messages the router delivered to a shard
}

// igNew builds one app (cgApp: a source stream with time attribute ts, a pass-through rule) on one shard, with timeAttr
// replacing the source stream's time attribute (1, or event.ArrivalTime).
func igNew(t *testing.T, timeAttr int) *igWorld {
	t.Helper()
	return igBuild(t, timeAttr, 1, nil)
}

// igBuild is igNew on shards shards, with edit (when not nil) changing the app before it is built.
func igBuild(t *testing.T, timeAttr, shards int, edit func(*App)) *igWorld {
	t.Helper()
	app := cgApp(t, func(event.Event) error { return nil })
	attrs := []event.Attr{{Name: "k", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}}
	s, err := event.NewSchema("s", attrs, timeAttr)
	if err != nil {
		t.Fatal(err)
	}
	app.Schemas[0] = s
	if edit != nil {
		edit(&app)
	}
	r, err := build(Config{Shards: shards, Mailbox: 1, Intake: 1, SinkQueue: 1, RecordQueue: 1, Now: func() int64 {
		return cgT0
	}}, []App{app})
	if err != nil {
		t.Fatal(err)
	}
	w := &igWorld{r: r, rt: newRouter(r), a: r.appList()[0]}
	w.rt.deliver = func(int, message) { w.msgs++ }
	return w
}

// req ingests it with a reply channel and returns the reply the router gave: exactly one, before ingest returns. The
// channel has room for a second, so a step that answers twice fails here instead of blocking the test.
func (w *igWorld) req(t *testing.T, it item) reply {
	t.Helper()
	it.reply = make(chan reply, 2)
	w.rt.ingest(it)
	if n := len(it.reply); n != 1 {
		t.Fatalf("ingest(kind %d) gave %d replies; want exactly one before it returns", it.kind, n)
	}
	return <-it.reply
}

// igGone makes the app unreachable the two ways the router checks: its slot nil, or its ingest step gone.
var igGone = map[string]func(w *igWorld){
	"nil slot":   func(w *igWorld) { list := []*appState{nil}; w.r.apps.Store(&list) },
	"nil ingest": func(w *igWorld) { w.a.ing = nil },
}

func TestRouterActivateAnswersByTheAppsState(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		set  func(w *igWorld)
		want error
	}{
		"nil slot":   {igGone["nil slot"], ErrRemoved},
		"nil ingest": {igGone["nil ingest"], ErrRemoved},
		"removing":   {func(w *igWorld) { w.a.activated = false; w.a.removing = true }, ErrRemoved},
		"drained":    {func(w *igWorld) { w.a.activated = false; w.a.drained = true }, ErrDrained},
		"activated":  {func(*igWorld) {}, ErrStarted},
		"inactive":   {func(w *igWorld) { w.a.activated = false }, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igNew(t, 1)
			c.set(w)
			if rep := w.req(t, item{kind: iActivate, app: 0}); !errors.Is(rep.err, c.want) || (c.want == nil) != (rep.err == nil) {
				t.Fatalf("iActivate: %v; want %v", rep.err, c.want)
			}
			if c.want == nil && (!w.a.activated || !w.a.started.Load()) {
				t.Fatal("an accepted iActivate left the app unactivated; want activated and started")
			}
		})
	}
}

// Each per-stream input of a gone app is dropped uncounted; of a drained app, counted in after; otherwise taken.
func TestRouterStreamInputsOfAGoneOrDrainedApp(t *testing.T) {
	t.Parallel()
	ev := cgEv(1)
	kinds := map[string]item{
		"event":     {kind: iEvent, app: 0, ev: ev},
		"reject":    {kind: iReject, app: 0, stream: 0, fault: &event.DecodeError{}},
		"batch end": {kind: iBatchEnd, app: 0, stream: 0, batch: "b1"},
	}
	for kn, it := range kinds {
		for gn, gone := range igGone {
			t.Run(kn+"/"+gn, func(t *testing.T) {
				t.Parallel()
				w := igNew(t, 1)
				a := w.a
				gone(w)
				w.rt.ingest(it)
				if w.msgs != 0 || a.after[0] != 0 || a.batches[0] != 0 {
					t.Fatalf("a gone app's %s: msgs %d after %d batches %d; want nothing", kn, w.msgs, a.after[0], a.batches[0])
				}
			})
		}
		t.Run(kn+"/drained", func(t *testing.T) {
			t.Parallel()
			w := igNew(t, 1)
			w.a.drained = true
			w.rt.ingest(it)
			if w.msgs != 0 || w.a.after[0] != 1 || w.a.batches[0] != 0 || w.a.ing.counts[0].rejected != 0 {
				t.Fatalf("a drained app's %s: msgs %d after %d batches %d; want counted in after only", kn, w.msgs,
					w.a.after[0], w.a.batches[0])
			}
		})
	}
	w := igNew(t, 1)
	w.rt.ingest(kinds["batch end"])
	if w.a.batches[0] != 1 || w.a.after[0] != 0 {
		t.Fatalf("a live batch end: batches %d after %d; want batches 1", w.a.batches[0], w.a.after[0])
	}
}

// An event on a `time arrival` stream takes the router's stamp as its time; one with a time attribute keeps its own.
func TestRouterStampsAnArrivalTimeEvent(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		timeAttr int
		wantTs   int64
	}{"arrival": {event.ArrivalTime, cgT0}, "attribute": {1, cgT0 + 500}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igNew(t, c.timeAttr)
			var got int64 = -1
			w.rt.deliver = func(_ int, m message) {
				if m.ev.StreamID == 0 && len(m.offers) > 0 {
					got = m.ev.Ts
				}
			}
			w.rt.ingest(item{kind: iEvent, app: 0, ev: cgEv(500)})
			if got != c.wantTs {
				t.Fatalf("the offered event's ts %d; want %d", got, c.wantTs)
			}
		})
	}
}

func TestRouterWriteAndDrainOfAGoneOrDrainedApp(t *testing.T) {
	t.Parallel()
	for gn, gone := range igGone {
		for kind, k := range map[string]itemKind{"write": iWrite, "drain": iDrain} {
			t.Run(gn+"/"+kind, func(t *testing.T) {
				t.Parallel()
				w := igNew(t, 1)
				gone(w)
				if rep := w.req(t, item{kind: k, app: 0}); !errors.Is(rep.err, ErrRemoved) || w.msgs != 0 {
					t.Fatalf("kind %d of a gone app: %v, msgs %d; want ErrRemoved, none", k, rep.err, w.msgs)
				}
			})
		}
	}
	w := igNew(t, 1)
	w.a.drained = true
	if rep := w.req(t, item{kind: iWrite, app: 0}); !errors.Is(rep.err, ErrDrained) {
		t.Fatalf("iWrite of a drained app: %v; want ErrDrained", rep.err)
	}
}

// A tick reaches only an app that is present, activated, not drained and not quiet; each condition alone skips it.
func TestRouterTickSkipsEachUnreachableApp(t *testing.T) {
	t.Parallel()
	cases := map[string]func(w *igWorld){
		"nil slot":    igGone["nil slot"],
		"nil ingest":  igGone["nil ingest"],
		"inactive":    func(w *igWorld) { w.a.activated = false },
		"drained":     func(w *igWorld) { w.a.drained = true },
		"quiet":       func(w *igWorld) { w.a.quiet = true },
		"reached (0)": func(*igWorld) {},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igNew(t, 1)
			g := w.a.ing
			set(w)
			w.rt.ingest(item{kind: iTick, now: cgT0 + 60_000})
			if want := name == "reached (0)"; (g.stamp == cgT0+60_000) != want {
				t.Fatalf("a tick: the app's ingest stamp %d; want reached %v (the positive case is the witness)", g.stamp, want)
			}
		})
	}
}

// A stats cut keeps a never-installed slot as an empty row, so the rows stay indexed by AppID.
func TestRouterStatsKeepsANilSlotsRow(t *testing.T) {
	t.Parallel()
	w := igNew(t, 1)
	list := []*appState{nil, w.a}
	w.r.apps.Store(&list)
	w.rt.ingest(item{kind: iStats, reply: make(chan reply, 1)})
	var st Stats
	for _, sw := range w.rt.stats {
		st = sw.stats
	}
	if len(st.Streams) != 2 || st.Streams[0] != nil || st.Rules[0] != nil || st.Recording[0] != (RecordingStats{}) ||
		len(st.Streams[1]) != 2 {
		t.Fatalf("stats rows %+v; want a nil row for slot 0 and the app's rows at 1", st)
	}
}

// A skipped app does not end the tick: the apps after it in the list are still reached.
func TestRouterTickReachesTheAppsAfterASkippedOne(t *testing.T) {
	t.Parallel()
	w := igNew(t, 1)
	b := w.r.newAppState(1, w.a.app, cgT0)
	b.activated = true
	w.a.quiet = true
	list := []*appState{w.a, b}
	w.r.apps.Store(&list)
	w.rt.ingest(item{kind: iTick, now: cgT0 + 60_000})
	if w.a.ing.stamp == cgT0+60_000 || b.ing.stamp != cgT0+60_000 {
		t.Fatalf("a tick: stamps %d, %d; want the quiet app skipped and the next reached", w.a.ing.stamp, b.ing.stamp)
	}
}

// A repeated Drain waits for the first one's flush and starts no second barrier: while it is in flight it joins the
// first's replies; once the result is in, it is answered with that result at once.
func TestRouterRepeatedDrainJoinsTheFirstBarrier(t *testing.T) {
	t.Parallel()
	w := igNew(t, 1)
	first, second := make(chan reply, 1), make(chan reply, 1)
	w.rt.ingest(item{kind: iDrain, app: 0, reply: first})
	if w.msgs != 1 || w.rt.token != 1 {
		t.Fatalf("the first drain: msgs %d token %d; want the barrier to the one shard under token 1", w.msgs, w.rt.token)
	}
	w.rt.ingest(item{kind: iDrain, app: 0, reply: second})
	if w.msgs != 1 || w.rt.token != 1 || len(w.rt.drains[1].replies) != 2 {
		t.Fatalf("a drain in flight: msgs %d token %d replies %d; want no second barrier, the call joined",
			w.msgs, w.rt.token, len(w.rt.drains[1].replies))
	}
	w.a.drainRes = &reply{pending: 7}
	if rep := w.req(t, item{kind: iDrain, app: 0}); rep.pending != 7 || w.msgs != 1 || w.rt.token != 1 {
		t.Fatalf("a drain after the result: pending %d msgs %d token %d; want the first result, no barrier",
			rep.pending, w.msgs, w.rt.token)
	}
}

// igRec is a Recorder that accepts every call; the tests read the app's recording queue, never the Recorder.
type igRec struct{}

func (igRec) Header(int64) error                            { return nil }
func (igRec) Event(*event.Event, int64, []byte) error       { return nil }
func (igRec) Rejected(int, int64, *event.DecodeError) error { return nil }
func (igRec) Tick(int64) error                              { return nil }
func (igRec) Table(int, TableOp, [][]event.Value) error     { return nil }
func (igRec) Stop() error                                   { return nil }
func (igRec) BatchEnd(int, string) error                    { return nil }
func (igRec) Flush() error                                  { return nil }

// igQueued takes every record queued so far from a's recording queue, without blocking.
func igQueued(a *appState) []recItem {
	var out []recItem
	for {
		select {
		case it := <-a.rec.q:
			out = append(out, it)
		default:
			return out
		}
	}
}

// Known issue S3.3-1-3 as it stands: a batch end queued between Stop (the caller's phase is draining) and the drain
// item is taken by the router, which reads its own a.drained (still false): counted in Batches and recorded.
func TestRouterTakesABatchEndBeforeItsDrainItem(t *testing.T) {
	t.Parallel()
	w := igBuild(t, 1, 1, func(app *App) { app.Recorder = igRec{} })
	w.a.phase.Store(int32(phaseDraining))
	w.a.draining.Store(true)
	igQueued(w.a) // the header
	w.rt.ingest(item{kind: iBatchEnd, app: 0, stream: 0, batch: "b1"})
	recs := igQueued(w.a)
	if w.a.batches[0] != 1 || w.a.after[0] != 0 || len(recs) != 1 || recs[0].kind != rBatchEnd || recs[0].batch != "b1" {
		t.Fatalf("a batch end before the drain item: batches %d after %d records %+v; want counted and recorded once",
			w.a.batches[0], w.a.after[0], recs)
	}
}

// A drained app's Activate is ErrDrained, whether it was activated before its drain or never (the drained check comes
// before the activated one).
func TestRouterActivatesADrainedAppWithErrDrained(t *testing.T) {
	t.Parallel()
	for name, activated := range map[string]bool{"activated": true, "never activated": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igNew(t, 1)
			w.a.activated, w.a.drained = activated, true
			w.a.started.Store(activated) // build starts every app it builds
			if rep := w.req(t, item{kind: iActivate, app: 0}); !errors.Is(rep.err, ErrDrained) {
				t.Fatalf("iActivate of a drained app: %v; want ErrDrained", rep.err)
			}
			if w.a.activated != activated || w.a.started.Load() != activated {
				t.Fatal("a refused iActivate changed the app; want it left as it was")
			}
		})
	}
}

// A drain for a slot this shard freed (a remove raced the drain's last advances) is acked with nothing pending, so the
// barrier still completes.
func TestShardAcksADrainForAFreedSlot(t *testing.T) {
	t.Parallel()
	w := igNew(t, 1)
	sh := w.r.shards[0]
	sh.uninstall(0)
	sh.process(message{kind: mDrain, app: 0, token: 7, admStream: -1, final: true,
		adv: []clock.WMAdvance{{StreamID: 0, Contribution: cgT0, Moved: true}}})
	if len(sh.outbox) != 1 || sh.outbox[0].kind != oAck || sh.outbox[0].token != 7 || sh.outbox[0].pending != 0 {
		t.Fatalf("a drain for a freed slot: outbox %+v; want one ack of token 7, nothing pending", sh.outbox)
	}
}

// A walk for a slot this shard freed walks nothing and answers nothing; an offer in it would index the freed slot.
func TestShardSkipsAWalkForAFreedSlot(t *testing.T) {
	t.Parallel()
	w := igNew(t, 1)
	sh := w.r.shards[0]
	sh.uninstall(0)
	sh.process(message{kind: mWalk, app: 0, admStream: 0, adm: cgT0, ev: cgEv(1), offers: []offer{{rule: 0}},
		adv: []clock.WMAdvance{{StreamID: 0, Contribution: cgT0, Moved: true}}})
	if len(sh.outbox) != 0 {
		t.Fatalf("a walk for a freed slot: outbox %+v; want nothing", sh.outbox)
	}
	sh.process(message{kind: mDrain, app: 0, token: 3, admStream: -1}) // the witness: the slot is still answered
	if len(sh.outbox) != 1 || sh.outbox[0].token != 3 {
		t.Fatalf("the drain after the walk: outbox %+v; want its ack", sh.outbox)
	}
}

// A walk's advances reach a rule unless it is on-arrival and the message is not final (process's !arrival || final).
func TestShardAppliesAWalksAdvancesByTheRulesKind(t *testing.T) {
	t.Parallel()
	c := cgT0 + 5_000
	for name, row := range map[string]struct {
		onArrival, final, want bool
	}{
		"not on-arrival":    {false, false, true},
		"on-arrival":        {true, false, false},
		"on-arrival, final": {true, true, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igBuild(t, 1, 1, func(app *App) { app.Rules[0].OnArrival = row.onArrival })
			sh := w.r.shards[0]
			sh.process(message{kind: mWalk, app: 0, admStream: -1, final: row.final,
				adv: []clock.WMAdvance{{StreamID: 0, Contribution: c, Moved: true}}})
			if got := sh.rules[0][0].Clock() == c; got != row.want {
				t.Fatalf("the rule's clock %d after an advance to %d; want applied %v", sh.rules[0][0].Clock(), c, row.want)
			}
		})
	}
}

// A walk's admission reaches an on-arrival rule only with a stream (admStream >= 0) and a value (not MinInt64).
func TestShardAppliesAnAdmissionOnlyWithAStreamAndAValue(t *testing.T) {
	t.Parallel()
	c := cgT0 + 5_000
	for name, row := range map[string]struct {
		stream int
		adm    int64
		want   bool
	}{
		"no stream": {-1, c, false},
		"both":      {0, c, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igNew(t, 1)
			sh := w.r.shards[0]
			sh.process(message{kind: mWalk, app: 0, admStream: row.stream, adm: row.adm})
			if got := sh.rules[0][0].Clock() == c; got != row.want {
				t.Fatalf("the on-arrival rule's clock %d after admission %d of stream %d; want applied %v",
					sh.rules[0][0].Clock(), row.adm, row.stream, row.want)
			}
		})
	}
}

// An output of an app that is gone (its slot nil, or the removed app's tombstone) is dropped; a live app's goes to
// each of its stream's sinks (the witness).
func TestRouterDropsAnOutputOfARemovedApp(t *testing.T) {
	t.Parallel()
	out := event.Event{StreamID: 1, Ts: cgT0, Values: cgEv(0).Values}
	cases := map[string]struct {
		set  func(w *igWorld)
		want int
	}{
		"tombstone": {igGone["nil ingest"], 0},
		"nil slot":  {igGone["nil slot"], 0},
		"live":      {func(*igWorld) {}, 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igNew(t, 1)
			c.set(w)
			w.rt.take(handoff{shard: 0, items: []outItem{{kind: oEvent, app: 0, ev: out}}})
			if len(w.rt.pending) != c.want {
				t.Fatalf("an output: %d sink sends pending; want %d", len(w.rt.pending), c.want)
			}
		})
	}
}

// The drain barrier at 4 shards: the reply waits for every shard's ack and carries the sum of their pending counts.
func TestRouterDrainBarrierWaitsForEveryShard(t *testing.T) {
	t.Parallel()
	w := igBuild(t, 1, 4, nil)
	rep := make(chan reply, 1)
	w.rt.ingest(item{kind: iDrain, app: 0, reply: rep})
	if w.msgs != 4 {
		t.Fatalf("the drain barrier: %d messages; want one per shard, 4", w.msgs)
	}
	tok := w.a.drainTok
	for sid := range 4 {
		if len(rep) != 0 {
			t.Fatalf("the drain answered after %d of 4 acks; want only after the last", sid)
		}
		w.rt.take(handoff{shard: sid, items: []outItem{{kind: oAck, app: 0, token: tok, pending: int64(sid + 1)}}})
	}
	select {
	case res := <-rep:
		if res.pending != 1+2+3+4 || len(res.markers) != 1 || w.a.drainRes == nil {
			t.Fatalf("the drain's reply: pending %d markers %d; want 10 and one sink marker", res.pending, len(res.markers))
		}
	default:
		t.Fatal("the drain gave no reply after the 4th ack; want one")
	}
	if _, ok := w.rt.drains[tok]; ok {
		t.Fatal("the completed barrier is still waiting; want it removed")
	}
}

// A stats cut at 4 shards: the reply waits for every shard's part and sums the rules' counters; a gone app's sink
// rows are nil (its slot nil, or its tombstone).
func TestRouterStatsSumsEveryShardsPart(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(w *igWorld){
		"live":      func(*igWorld) {},
		"tombstone": igGone["nil ingest"],
		"nil slot":  igGone["nil slot"],
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igBuild(t, 1, 4, nil)
			rep := make(chan reply, 1)
			w.rt.ingest(item{kind: iStats, reply: rep})
			tok := w.rt.token
			set(w)
			for sid := range 4 {
				if len(rep) != 0 {
					t.Fatalf("stats answered after %d of 4 parts; want only after the last", sid)
				}
				part := w.r.shards[sid].stats()
				part.panics[0][0] = int64(sid + 1)
				part.parks = int64(10 * (sid + 1))
				w.rt.take(handoff{shard: sid, items: []outItem{{kind: oStats, token: tok, stats: part}}})
			}
			var st Stats
			select {
			case res := <-rep:
				st = res.stats
			default:
				t.Fatal("stats gave no reply after the 4th part; want one")
			}
			if st.Rules[0][0].Panics != 10 || len(st.SaturatedParks) != 4 || st.SaturatedParks[3] != 40 {
				t.Fatalf("stats: panics %d parks %v; want 10 and [10 20 30 40]", st.Rules[0][0].Panics, st.SaturatedParks)
			}
			if wantRow := name == "live"; (st.SinkErrors[0] != nil) != wantRow || (st.SinkHigh[0] != nil) != wantRow {
				t.Fatalf("stats: sink rows %v %v; want a row %v", st.SinkErrors, st.SinkHigh, wantRow)
			}
		})
	}
}

// An admission reaches only the on-arrival rules reading the stream; a rule that is not on-arrival waits for the
// watermark.
func TestShardAppliesAnAdmissionOnlyToOnArrivalRules(t *testing.T) {
	t.Parallel()
	c := cgT0 + 5_000
	w := igBuild(t, 1, 1, func(app *App) { app.Rules[0].OnArrival = false })
	sh := w.r.shards[0]
	sh.process(message{kind: mWalk, app: 0, admStream: 0, adm: c})
	if sh.rules[0][0].Clock() == c {
		t.Fatalf("a rule that is not on-arrival took admission %d; want it ignored", c)
	}
}

// broadcast's messages carry no admission (admStream -1), so process applies none: a zero admStream would be stream
// 0's admission at 0 (a message's adm is 0 unless set), which the on-arrival rule's clock would take. Reached only
// through broadcast's callers, never called directly (so broadcast's signature is free to change): ingest's iTick
// (tickApp) and iDrain (drainApp), at 4 shards, each message processed in place by its shard. Both the messages'
// field and its effect on every shard's rule are checked, and the effect alone catches a zero admission in each
// subtest: no event is admitted, so the rule's clock stays MinInt64 unless an admission lands. The tick's one
// advance (stream 0 quiet 60 s past IdleAfter 1 s: its floor, start cgT0 plus the 60 000 ms elapsed, delay 0) is a
// walk's, which an on-arrival rule does not take; the drain has no advance (nothing admitted: clock's Drain). A drain
// after an admitted event would hide the zero admission behind its final advance (one past that event's time).
func TestBroadcastCarriesNoAdmission(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		it    item
		kind  msgKind
		adv   []clock.WMAdvance
		final bool
	}{
		"tick": {item{kind: iTick, now: cgT0 + 60_000}, mWalk,
			[]clock.WMAdvance{{StreamID: 0, Contribution: cgT0 + 60_000, Moved: true}}, false},
		"drain": {item{kind: iDrain, app: 0, reply: make(chan reply, 1)}, mDrain, nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := igBuild(t, 1, 4, func(app *App) { app.Streams[0].IdleAfter = 1_000 })
			var got []int // the shards delivered to, in order
			w.rt.deliver = func(sid int, m message) {
				got = append(got, sid)
				if m.admStream != -1 || m.kind != c.kind || m.app != 0 || m.final != c.final ||
					!slices.Equal(m.adv, c.adv) {
					t.Errorf("a %s's message to shard %d: %+v; want kind %d of app 0, final %v, advances %v and "+
						"admStream -1", name, sid, m, c.kind, c.final, c.adv)
				}
				w.r.shards[sid].process(m)
			}
			w.rt.ingest(c.it)
			if !slices.Equal(got, []int{0, 1, 2, 3}) { // the witness: the caller broadcast once to every shard
				t.Fatalf("a %s at 4 shards delivered to shards %v; want 0, 1, 2, 3", name, got)
			}
			for sid, sh := range w.r.shards {
				if clk := sh.rules[0][0].Clock(); clk != math.MinInt64 {
					t.Errorf("a %s: shard %d's on-arrival rule's clock %d; want MinInt64, no admission taken", name,
						sid, clk)
				}
			}
		})
	}
}

// A never-activated app's drain is answered at once with an empty result: nothing was delivered, so no barrier goes to
// the shards (design s3-3-1 §2.4, MUST 8).
func TestRouterDrainsANeverActivatedAppWithoutABarrier(t *testing.T) {
	t.Parallel()
	w := igNew(t, 1)
	w.a.activated = false
	if rep := w.req(t, item{kind: iDrain, app: 0}); rep.err != nil || rep.pending != 0 || rep.markers != nil {
		t.Fatalf("a never-activated app's drain: %+v; want an empty result", rep)
	}
	if w.msgs != 0 || w.rt.token != 0 || !w.a.drained || w.a.drainRes == nil || len(w.rt.drains) != 0 {
		t.Fatalf("a never-activated app's drain: msgs %d token %d drained %v; want no barrier, the app drained with its "+
			"result kept", w.msgs, w.rt.token, w.a.drained)
	}
}

// An add completes on the last shard's install ack: the router answers with the new id and makes the app reachable;
// an earlier ack only counts. Driven in place, so a misrouted ack fails here, on the test goroutine.
func TestRouterCompletesAnAddOnItsInstallAck(t *testing.T) {
	t.Parallel()
	w := igBuild(t, 1, 2, nil)
	app := cgApp(t, func(event.Event) error { return nil })
	rep := make(chan reply, 2)
	w.rt.ingest(item{kind: iAdd, add: &app, reply: rep})
	tok := w.rt.token
	if w.rt.installs[tok] == nil || len(w.rt.pending) != 2 {
		t.Fatalf("an add: install wait %v, %d sends pending; want a wait under token %d and one mAdd per shard",
			w.rt.installs[tok], len(w.rt.pending), tok)
	}
	added := w.r.appList()[1]
	w.rt.take(handoff{shard: 0, items: []outItem{{kind: oAck, app: 1, token: tok}}})
	if len(rep) != 0 || len(added.byStream[0]) != 0 {
		t.Fatalf("after 1 of 2 install acks: %d replies, %d routes; want none yet", len(rep), len(added.byStream[0]))
	}
	w.rt.take(handoff{shard: 1, items: []outItem{{kind: oAck, app: 1, token: tok}}})
	if n := len(rep); n != 1 {
		t.Fatalf("after the last install ack: %d replies; want exactly one", n)
	}
	if res := <-rep; res.err != nil || res.id != 1 || len(added.byStream[0]) != 1 || w.rt.installs[tok] != nil {
		t.Fatalf("the add's reply %+v, %d routes; want id 1, the app's route installed, the wait removed", res,
			len(added.byStream[0]))
	}
}

// rgCount is an on-arrival body whose state counts the key's events since its birth.
type rgCount struct {
	partition.TimerFree
	births *int
}

type rgState struct {
	cgState
	n int
}

func (b *rgCount) NewState(*partition.Instance) partition.OpState { *b.births++; return &rgState{} }
func (b *rgCount) OnEvent(in *partition.Instance, _ event.Event)  { in.Op.(*rgState).n++ }

// A key whose TTL purge fell due before its next event is purged first: the event starts a fresh state. Offered first,
// the event would land in the stale state and re-arm its TTL, and the purge would never fire (design s3-2b-1 v2 §3).
func TestShardPurgesADueKeyBeforeItsNextEvent(t *testing.T) {
	t.Parallel()
	const ttl = 1_000
	births := 0
	w := igBuild(t, 1, 1, func(app *App) {
		app.Rules[0].KeyAttrs, app.Rules[0].TTL = [][]int{{0}}, ttl
		app.Rules[0].NewBody = func(func(event.Event)) partition.Body { return &rgCount{births: &births} }
	})
	sh := w.r.shards[0]
	walk := func(i int) {
		ev := cgEv(i)
		key, ok := routeKey([]int{0}, ev.Values)
		if !ok {
			t.Fatal("the fixture's key")
		}
		sh.process(message{kind: mWalk, app: 0, admStream: 0, adm: ev.Ts, ev: ev, offers: []offer{{rule: 0, key: key}}})
	}
	walk(0)
	rule := sh.rules[0][0]
	if rule.Stats().Purged != 0 || rule.Len() != 1 || births != 1 {
		t.Fatalf("the first event: purged %d, %d keys, %d births; want none purged, one key born once",
			rule.Stats().Purged, rule.Len(), births)
	}
	walk(ttl + 1) // its admission passes the key's TTL timer (due at cgT0+ttl)
	if rule.Stats().Purged != 1 || births != 2 || rule.Len() != 1 {
		t.Fatalf("an event after the key's TTL: purged %d, %d births, %d keys; want the purge first, then a fresh birth",
			rule.Stats().Purged, births, rule.Len())
	}
}
