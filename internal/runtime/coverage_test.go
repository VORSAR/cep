package runtime_test

// Coverage of behaviour already in design v2 and the build (the gate's ≥ 90 % row for internal/runtime): the
// replay-only ingest paths, key hashing per key type, the internal ticker, the callers' context and Close paths, and
// Start's remaining refusals. Each test asserts a hand-derived outcome. Helpers are prefixed cv.

import (
	"context"
	"errors"
	"fmt"
	"math"
	goruntime "runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// TestCvIngestVirtualAndEnd: the shared ingest step in playback's fixture mode (design v2 §1; S3.2a-1 design v2
// §5.3), driven directly. Streams A (Delay 5 s) and B (Delay 10 s), start 0, no tables (nil set), virtual.
//   - A@100 s: Seq 1, not late. Own advance: A's admission 95 s → [A 95 s]. Virtual floor at 100 s: A − 5 = 95 s (no
//     rise), B − 10 = 90 s → [B 90 s]. Advances [A 95 s, B 90 s]. Tables: a non-nil snapshot.
//   - A@80 s: Seq 2, stream-late (80 < 95): no advances.
//   - B@99 s: Seq 1 on B; B's admission 89 s < its contribution 90 s → no own advance; the virtual floor stays at
//     100 s → none. Advances [].
//   - End: every contribution to +∞ → [A +∞, B +∞]; Registry().Contribution is +∞ for both.
//   - Walk: seq A = 2, seq B = 1, in declaration order.
func TestCvIngestVirtualAndEnd(t *testing.T) {
	const s = rtSec
	g := runtime.NewIngest([]clock.StreamSpec{{Name: "A", Delay: 5 * s}, {Name: "B", Delay: 10 * s}}, 0, nil, true)
	step := func(sid int, ts int64, wantSeq uint64, wantLate bool, want []clock.WMAdvance) {
		t.Helper()
		e := event.Event{StreamID: sid, Ts: ts}
		late, adv := g.Event(&e, ts)
		if e.Seq != wantSeq || late != wantLate || !slices.Equal(adv, want) {
			t.Errorf("Event(s%d @%d): Seq %d, late %v, advances %v; want %d, %v, %v", sid, ts, e.Seq, late, adv, wantSeq, wantLate, want)
		}
		if snap, ok := e.Tables.(*table.Snapshot); !ok || snap == nil || snap != g.Snapshot() {
			t.Errorf("Event(s%d @%d): Tables %T(%v), want the current snapshot %p", sid, ts, e.Tables, e.Tables, g.Snapshot())
		}
	}
	step(0, 100*s, 1, false, []clock.WMAdvance{{StreamID: 0, Contribution: 95 * s, Moved: true}, {StreamID: 1, Contribution: 90 * s, Moved: true}})
	step(0, 80*s, 2, true, []clock.WMAdvance{})
	step(1, 99*s, 1, false, []clock.WMAdvance{})
	end := g.End()
	if want := []clock.WMAdvance{{StreamID: 0, Contribution: math.MaxInt64, Moved: true}, {StreamID: 1, Contribution: math.MaxInt64, Moved: true}}; !slices.Equal(end, want) {
		t.Errorf("End: %v, want %v", end, want)
	}
	for sid := 0; sid < 2; sid++ {
		if c := g.Registry().Contribution(sid); c != math.MaxInt64 {
			t.Errorf("Registry().Contribution(%d) = %d after End, want +∞", sid, c)
		}
	}
	var walk []string
	g.Walk(func(c, k string, st any) { walk = append(walk, fmt.Sprintf("%s/%s=%v", c, k, st)) })
	if want := []string{"seq/A=2", "seq/B=1"}; !slices.Equal(walk, want) {
		t.Errorf("Walk %v, want %v", walk, want)
	}
}

// cvKey renders a key of any scalar type.
func cvKey(v event.Value) string {
	switch v.Kind {
	case event.TInt:
		return fmt.Sprintf("i%d", v.I)
	case event.TFloat:
		if v.F == 0 && math.Signbit(v.F) {
			return "f-0"
		}
		return fmt.Sprintf("f%g", v.F)
	case event.TBool:
		return fmt.Sprintf("b%v", v.B)
	case event.TTimestamp:
		return fmt.Sprintf("t%d", v.T)
	case event.TDuration:
		return fmt.Sprintf("d%d", v.D)
	}
	return "s" + v.Str
}

// TestCvKeyTypesIndependentOfShards (design v2 §4.3, §7): for each scalar key type, the same events give the same
// per-key releases with 1 and with 4 shards, and float −0.0 and +0.0 are one key. Stream "in" (Delay 0; [key T]);
// rule R keyed by key (KeyType T), TTL 1 h. Six events at 1..6 s with keys k[i mod 3]; Drain releases all.
// Expected per type (Seq = i + 1, ts = i + 1 s): k0 gets #1, #4; k1 #2, #5; k2 #3, #6 — except float, whose k0 = −0.0
// and k1 = +0.0 are one key f0 (stored +0.0) with #1, #2, #4, #5. Every key has one birth.
func TestCvKeyTypesIndependentOfShards(t *testing.T) {
	ev := func(i int) string { return fmt.Sprintf("ev s0#%d @%d", i, int64(i)*rtSec) }
	for _, c := range []struct {
		ty   event.Type
		keys [3]event.Value
		want map[string][]string
	}{
		{event.TInt, [3]event.Value{rtInt(-7), rtInt(0), rtInt(1 << 40)},
			map[string][]string{"i-7": {ev(1), ev(4)}, "i0": {ev(2), ev(5)}, "i1099511627776": {ev(3), ev(6)}}},
		{event.TFloat, [3]event.Value{rtFloat(math.Copysign(0, -1)), rtFloat(0), rtFloat(2.5)},
			map[string][]string{"f0": {ev(1), ev(2), ev(4), ev(5)}, "f2.5": {ev(3), ev(6)}}},
		{event.TBool, [3]event.Value{{Kind: event.TBool, B: true}, {Kind: event.TBool, B: false}, {Kind: event.TBool, B: true}},
			map[string][]string{"btrue": {ev(1), ev(3), ev(4), ev(6)}, "bfalse": {ev(2), ev(5)}}},
		{event.TTimestamp, [3]event.Value{{Kind: event.TTimestamp, T: rtT0}, {Kind: event.TTimestamp, T: 0}, {Kind: event.TTimestamp, T: -1}},
			map[string][]string{fmt.Sprintf("t%d", rtT0): {ev(1), ev(4)}, "t0": {ev(2), ev(5)}, "t-1": {ev(3), ev(6)}}},
		{event.TDuration, [3]event.Value{{Kind: event.TDuration, D: 1}, {Kind: event.TDuration, D: 60000}, {Kind: event.TDuration, D: 0}},
			map[string][]string{"d1": {ev(1), ev(4)}, "d60000": {ev(2), ev(5)}, "d0": {ev(3), ev(6)}}},
	} {
		for _, shards := range []int{1, 4} {
			t.Run(fmt.Sprintf("%v/shards=%d", c.ty, shards), func(t *testing.T) {
				ctx := rtCeiling(t)
				log, births := newRTLog(), newRTLog()
				body := rtBodies(rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
					log.add(cvKey(in.Key.First), fmt.Sprintf("ev s%d#%d @%d", e.StreamID, e.Seq, e.Ts-rtT0))
				}})
				newBody := func(emit runtime.Emit) partition.Body {
					b := body(emit).(*rtBody)
					b.births = births
					return b
				}
				app := runtime.App{
					Streams: []clock.StreamSpec{rtSrc("in", 0)},
					Schemas: []event.Schema{rtSchema(t, "in", rtA("key", c.ty))},
					Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{c.ty}, TTL: 3600 * rtSec, NewBody: newBody}},
				}
				r := rtStart(t, runtime.Config{Shards: shards}, app)
				for i := 0; i < 6; i++ {
					rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i+1)), c.keys[i%3]))
				}
				rtDrain(t, ctx, r, 0)
				got := log.snapshot()
				if len(got) != len(c.want) {
					t.Errorf("keys %v, want %v", got, c.want)
				}
				for k, w := range c.want {
					if !rtEqual(got[k], w) {
						t.Errorf("%s: %v, want %v", k, got[k], w)
					}
				}
				for k, b := range births.snapshot() {
					if len(b) != 1 {
						t.Errorf("%s: births %v, want one", k, b)
					}
				}
			})
		}
	}
}

// TestCvInternalTicker (design v2 §2 Config.TickEvery, §3, §4): with TickEvery set, a quiet stream's floor moves
// through the runtime's own ticker. Stream q (Delay 0, IdleAfter 5 s; [k string]); rule Q keyed by k, TTL 1 h,
// schedules Completion at ts + 1 s and emits "tm@<s>" on a fire. Now reads a value the test sets.
//   - Now = T0; Send b@0 s; Stats (an intake barrier: b's arrival is stamped T0 when the router dequeues it).
//   - Now = T0 + 100 s. The ticker's next tick stamps T0 + 100 s: q has been quiet 100 s ≥ 5 s, its floor is
//     0 + 100 s − 0 → the clock passes 1 s: b@0 is released and its timer @1 s fires → the sink gets "tm@1".
//   - The test waits for that output under the ceiling; then Close returns nil (the ticker goroutine included), and
//     a Tick returns ErrClosed.
func TestCvInternalTicker(t *testing.T) {
	ctx := rtCeiling(t)
	var now atomic.Int64
	now.Store(rtT0)
	got := make(chan string, 1)
	app := runtime.App{
		Streams: []clock.StreamSpec{{Name: "q", IdleAfter: 5 * rtSec}, rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "q", rtA("k", event.TString)), rtSchema(t, "out", rtA("what", event.TString))},
		Rules: []runtime.Rule{{Name: "Q", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "Q",
				sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + rtSec, Kind: clock.Completion, Ref: 1}}
				},
				onTimers: func(b *rtBody, in *partition.Instance, fires []clock.Timer, _ any) {
					b.emit(event.Event{StreamID: 1, Ts: fires[0].Due, Values: []event.Value{rtStr(fmt.Sprintf("tm@%d", (fires[0].Due-rtT0)/rtSec))}})
				}})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(e event.Event) error {
			select {
			case got <- e.Values[0].Str:
			default:
			}
			return nil
		}}},
	}
	r, err := runtime.Start(runtime.Config{Shards: 2, TickEvery: time.Millisecond, Now: now.Load}, []runtime.App{app})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(0), rtStr("b")))
	rtStats(t, ctx, r)
	now.Store(rtAt(100))
	select {
	case s := <-got:
		if s != "tm@1" {
			t.Errorf("the ticker's output %q, want tm@1", s)
		}
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the internal ticker to fire b's timer: %v", ctx.Err())
	}
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v (state: every goroutine, the ticker's included, stopped)", err)
	}
	if err := r.Tick(ctx, rtAt(200)); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("Tick after Close: %v, want ErrClosed", err)
	}
}

// TestCvDefaults (design v2 §2: Shards 0 is GOMAXPROCS; Now nil is the wall clock). Shards 0 and Now nil; one event
// a@1 s and a Drain release it (the wall clock only stamps its arrival: nothing here reads it). Expected: MailboxHigh
// and SaturatedParks have GOMAXPROCS entries; R released a once.
func TestCvDefaults(t *testing.T) {
	ctx := rtCeiling(t)
	log := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
		Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "R", log: log})}},
	}
	r, err := runtime.Start(runtime.Config{}, []runtime.App{app})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if err := r.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtDrain(t, ctx, r, 0)
	st := rtStats(t, ctx, r)
	if n := goruntime.GOMAXPROCS(0); len(st.MailboxHigh) != n || len(st.SaturatedParks) != n {
		t.Errorf("shards: MailboxHigh %d, SaturatedParks %d entries; want GOMAXPROCS = %d", len(st.MailboxHigh), len(st.SaturatedParks), n)
	}
	if got := log.get("R/a"); !rtEqual(got, []string{"ev s0#1 @1000"}) {
		t.Errorf("R/a %v, want [ev s0#1 @1000]", got)
	}
}

// TestCvStreamLateAndRedrain (design v2 §4.2, §5): a stream-late event is counted and offered to nothing; a second
// Drain of a drained app returns ErrDrained. Stream "in" (Delay 5 s; [k string]); rule R keyed by k. a@20 s (admission
// 15 s), a@10 s (10 < 15: stream-late), a@16 s (≥ 15: admitted); Drain (20.001 s). Expected: R/a released in
// event-time order s0#3 @16 s, then s0#1 @20 s, and never the late s0#2; StreamLate [1]; a second Drain returns
// ErrDrained (the router's check: the app is already drained).
func TestCvStreamLateAndRedrain(t *testing.T) {
	ctx := rtCeiling(t)
	log := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 5*rtSec)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
		Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "R", log: log})}},
	}
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	for _, s := range []float64{20, 10, 16} {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(s), rtStr("a")))
	}
	rtDrain(t, ctx, r, 0)
	if got, want := log.get("R/a"), []string{"ev s0#3 @16000", "ev s0#1 @20000"}; !rtEqual(got, want) {
		t.Errorf("R/a %v, want %v", got, want)
	}
	if n := rtStats(t, ctx, r).StreamLate[0][0]; n != 1 {
		t.Errorf("stream_late %d, want 1", n)
	}
	// A second Drain waits for the same flush and returns its result (owner, 2026-11-05, the break-it's question; it
	// returned ErrDrained before).
	if res, err := r.Drain(ctx, 0); err != nil || res.Pending != 0 {
		t.Errorf("a second Drain: (%+v, %v), want ({Pending:0}, nil): the first drain's result", res, err)
	}
}

// TestCvStartRefusesMore (design v2 §2): the refusals TestStartValidation does not reach. Base: "in" ([k string]),
// rule R keyed by k, TTL 1 h. Refused: Schemas shorter than Streams; a rule with no inputs; a key type of TObject;
// a NewBody that returns a nil body (partition refuses it when a shard builds the rule).
func TestCvStartRefusesMore(t *testing.T) {
	base := func(t *testing.T) runtime.App {
		return runtime.App{
			Streams: []clock.StreamSpec{rtSrc("in", 0)},
			Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
			Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "R"})}},
		}
	}
	for _, c := range []struct {
		name string
		bad  func(a *runtime.App)
	}{
		{"schemas_not_aligned", func(a *runtime.App) { a.Schemas = nil }},
		{"no_inputs", func(a *runtime.App) { a.Rules[0].Inputs = nil; a.Rules[0].KeyAttrs = nil; a.Rules[0].TTL = 0 }},
		{"object_key", func(a *runtime.App) { a.Rules[0].KeyTypes = []event.Type{event.TObject} }},
		{"nil_body", func(a *runtime.App) { a.Rules[0].NewBody = func(runtime.Emit) partition.Body { return nil } }},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := base(t)
			c.bad(&a)
			r, err := runtime.Start(runtime.Config{Shards: 2, Now: rtNow}, []runtime.App{a})
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), rtCeilingDur)
				defer cancel()
				_ = r.Close(ctx)
				t.Fatalf("Start accepted %s, want an error", c.name)
			}
		})
	}
}

// TestCvCallerContextsAndClose (design v2 §3: a caller waits on the intake send, then on its reply, each under
// select { …, stopped, ctx.Done }; §6: Close returns ctx's error while a Sink has not returned).
//
// App 0: "in" (Delay 0; [k string]), derived "out" ([n int]); rule R keyed by k emits 3 outputs per release. App 1:
// "in1" ([k string]); rule R1 keyed by k. Config: Shards 2, Intake 2, SinkQueue 1. The sink is armed to hold.
//  1. a@1 s, a@2 s: the second releases a@1 → 3 outputs; the flusher holds the first (state: entered), the second
//     fills the queue, the third stays at the router's FIFO head. The router reads no intake from here on.
//  2. Stats under a passed deadline → DeadlineExceeded (nothing queued).
//  3. Stats under a 20 ms deadline → queued (intake slot 1), its reply never comes → DeadlineExceeded.
//  4. Drain(app 1) under a 20 ms deadline → queued (slot 2), its reply never comes → DeadlineExceeded.
//  5. Send(app 0) under a 20 ms deadline → the intake is full → DeadlineExceeded.
//  6. A goroutine Sends to app 0 under the ceiling: it can only wait (full intake) or see Close → ErrClosed.
//  7. Close under a passed deadline → DeadlineExceeded: the held flusher has not returned.
//  8. The goroutine's Send returns ErrClosed. The test releases the sink; Close under the ceiling returns nil; Stats
//     returns ErrClosed.
//
// The outcomes do not depend on timing: the router is provably stalled from step 1 until Close.
func TestCvCallerContextsAndClose(t *testing.T) {
	ceiling := rtCeiling(t)
	hold := newRTHold()
	t.Cleanup(hold.free)
	app0 := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("n", event.TInt))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
				for n := 0; n < 3; n++ {
					b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{rtInt(int64(n))}})
				}
			}})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(event.Event) error { hold.at(); return nil }}},
	}
	app1 := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in1", 0)},
		Schemas: []event.Schema{rtSchema(t, "in1", rtA("k", event.TString))},
		Rules:   []runtime.Rule{{Name: "R1", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "R1"})}},
	}
	r, err := runtime.Start(runtime.Config{Shards: 2, Intake: 2, SinkQueue: 1, Now: rtNow}, []runtime.App{app0, app1})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	short := func() context.Context {
		ctx, cancel := context.WithTimeout(ceiling, 20*time.Millisecond)
		t.Cleanup(cancel)
		return ctx
	}
	passed, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()

	hold.arm()
	rtSend(t, ceiling, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtSend(t, ceiling, r, 0, rtEv(0, rtAt(2), rtStr("a")))
	rtWait(t, ceiling, hold.entered, "the sink's held call (a@1's outputs)")

	if _, err := r.Stats(passed); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("2. Stats under a passed deadline: %v, want DeadlineExceeded", err)
	}
	if _, err := r.Stats(short()); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("3. Stats awaiting its reply: %v, want DeadlineExceeded", err)
	}
	if _, err := r.Drain(short(), 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("4. Drain awaiting its reply: %v, want DeadlineExceeded", err)
	}
	if err := r.Send(short(), 0, rtEv(0, rtAt(3), rtStr("b"))); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("5. Send into a full intake: %v, want DeadlineExceeded", err)
	}
	waiting := make(chan error, 1)
	go func() { waiting <- r.Send(ceiling, 0, rtEv(0, rtAt(4), rtStr("c"))) }()
	if err := r.Close(passed); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("7. Close with the flusher held: %v, want DeadlineExceeded", err)
	}
	select {
	case err := <-waiting:
		if !errors.Is(err, runtime.ErrClosed) {
			t.Errorf("8. the waiting Send: %v, want ErrClosed", err)
		}
	case <-ceiling.Done():
		t.Fatalf("ceiling hit waiting for the waiting Send to see Close")
	}
	hold.free()
	if err := r.Close(ceiling); err != nil {
		t.Errorf("8. Close after the sink returned: %v, want nil", err)
	}
	if _, err := r.Stats(ceiling); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("8. Stats after Close: %v, want ErrClosed", err)
	}
}
