package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// TestDrainBarrier (design v2 §5; owner F5; F2, F11).
//
// App 0: stream "in" (Delay 0; schema [k string]), derived "out" ([what string]), table T(x string) keyed x. Rule R
// keyed by k, TTL 1 h; on each release it emits "ev@<s>" and schedules Completion at ts + 8 s (ref = release count);
// on a fire it emits "tm@<s>". Sink 0 records in order.
// App 1: stream "q" (Delay 0, IdleAfter 5 s; schema [k string]), derived "out1" ([what string]); rule Q keyed by k,
// TTL 1 h; on a release it schedules Completion at ts + 1 s and on a fire emits "tm@<s>"; sink 1 records, and the
// test can hold it.
// Config.Now reads a value the test sets: T0 until step 3, then T0 + 100 s, the same as the Tick's argument, so the
// tick's time is T0 + 100 s whether the router takes it from the argument or from Now (design §4's clamp; gap Q14).
//
//  1. App 0: a@10 s, a@12 s, a@20 s. Clock 12 s releases ev@10 (timer 18 s); clock 20 s releases ev@12 (timer 20 s),
//     fires tm@18. Drain: Ingest.Drain → 20.001 s: releases ev@20 (timer 28 s; an event precedes a timer at one
//     instant), fires tm@20. Expected: Drain returns nil with Pending 1 (the 28 s timer; a's TTL timer, due 1 h after
//     20 s, is excepted: F11); sink 0 holds exactly [ev@10, ev@12, tm@18, ev@20, tm@20] the moment Drain returns.
//  2. After the drain, for app 0: Send, TableWrite → ErrDrained.
//  3. App 1 keeps its ticks (F2): b@0 s on "q" (arrival T0). Arm sink 1's hold; Tick(T0 + 100 s): q has been quiet
//     100 s ≥ 5 s, so its floor = 0 + 100 s − 0 → releases b@0 (timer 1 s), fires tm@1 → sink 1's held call (the
//     positive witness the tick ran). Tick returns nil (app 1 is live). App 0 had its chance at that tick and fired
//     nothing: sink 0 still holds exactly the five outputs (no tm@28).
//  4. With sink 1 held, Drain(app 1) under a context whose deadline has passed returns an error that is
//     context.DeadlineExceeded (the barrier cannot complete while the sink is held). The test then releases the sink.
//     Then Tick: every app is drained or draining... not asserted (gap Q13).
func TestDrainBarrier(t *testing.T) {
	ctx := rtCeiling(t)
	tb, err := table.New("T", []event.Attr{rtA("x", event.TString)}, []string{"x"})
	if err != nil {
		t.Fatalf("table.New: %v", err)
	}
	out0, out1 := newRTLog(), newRTLog()
	hold1 := newRTHold()
	emitEv := func(b *rtBody, in *partition.Instance, e event.Event) {
		b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{rtStr(fmt.Sprintf("ev@%d", (e.Ts-rtT0)/rtSec))}})
	}
	emitTm := func(b *rtBody, in *partition.Instance, fires []clock.Timer, _ any) {
		for _, f := range fires {
			b.emit(event.Event{StreamID: 1, Ts: f.Due, Values: []event.Value{rtStr(fmt.Sprintf("tm@%d", (f.Due-rtT0)/rtSec))}})
		}
	}
	app0 := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("what", event.TString))},
		Tables:  table.NewSet([]*table.Table{tb}),
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R",
				sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + 8*rtSec, Kind: clock.Completion, Ref: n}}
				},
				onEvent: emitEv, onTimers: emitTm})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(e event.Event) error { out0.add("out", e.Values[0].Str); return nil }}},
	}
	app1 := runtime.App{
		Streams: []clock.StreamSpec{{Name: "q", IdleAfter: 5 * rtSec}, rtDerived("out1")},
		Schemas: []event.Schema{rtSchema(t, "q", rtA("k", event.TString)), rtSchema(t, "out1", rtA("what", event.TString))},
		Rules: []runtime.Rule{{Name: "Q", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "Q",
				sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + 1*rtSec, Kind: clock.Completion, Ref: 1}}
				},
				onTimers: emitTm})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(e event.Event) error {
			hold1.at()
			out1.add("out1", e.Values[0].Str)
			return nil
		}}},
	}
	var now atomic.Int64
	now.Store(rtT0)
	r := rtStart(t, runtime.Config{Shards: 2, Now: now.Load}, app0, app1)
	t.Cleanup(hold1.free)

	// 1.
	for _, s := range []float64{10, 12, 20} {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(s), rtStr("a")))
	}
	res := rtDrain(t, ctx, r, 0)
	want0 := []string{"ev@10", "ev@12", "tm@18", "ev@20", "tm@20"}
	if got := out0.get("out"); !rtEqual(got, want0) {
		t.Fatalf("sink 0 when Drain returned: %v, want %v (every output before the barrier)", got, want0)
	}
	if res.Pending != 1 {
		t.Errorf("Pending %d, want 1 (the 28 s timer; TTL timers excepted)", res.Pending)
	}

	// 2.
	if err := r.Send(ctx, 0, rtEv(0, rtAt(30), rtStr("a"))); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("Send after the drain: %v, want ErrDrained", err)
	}
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtStr("p")}}); !errors.Is(err, runtime.ErrDrained) {
		t.Errorf("TableWrite after the drain: %v, want ErrDrained", err)
	}

	// 3.
	rtSend(t, ctx, r, 1, rtEv(0, rtAt(0), rtStr("b")))
	// Send returns once queued; the router stamps the arrival when it takes the event. A Stats call is an intake barrier:
	// once it returns, b has been stamped at T0, so moving the clock now cannot restamp it (coordinator, 2026-11-05: the
	// first real run moved the clock first and b looked freshly active at the tick).
	if _, err := r.Stats(ctx); err != nil {
		t.Fatalf("Stats (an intake barrier): %v", err)
	}
	hold1.arm()
	now.Store(rtAt(100))
	if err := r.Tick(ctx, rtAt(100)); err != nil {
		t.Fatalf("Tick with app 1 live: %v, want nil", err)
	}
	rtWait(t, ctx, hold1.entered, "app 1's tick output at its sink (the tick ran)")
	if got := out0.get("out"); !rtEqual(got, want0) {
		t.Errorf("sink 0 after a tick: %v, want still %v (no tick reaches a drained app: F2)", got, want0)
	}

	// 4.
	expired, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	if _, err := r.Drain(expired, 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Drain under a passed deadline with the sink held: %v, want context.DeadlineExceeded", err)
	}
	hold1.free()
}

// TestDrainWithExpiredContextLeavesTheAppLive (QA gap Q15; coordinator, 2026-11-05): a Drain whose context has already
// expired queues nothing, so the app is not drained, and it must keep accepting input. Before the fix the draining
// flag stayed set (Send then returned ErrDrained for an app never drained), and a select between the expired context
// and a ready intake could queue the request anyway.
func TestDrainWithExpiredContextLeavesTheAppLive(t *testing.T) {
	ctx := rtCeiling(t)
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("what", event.TString))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttr: []int{0}, KeyType: event.TString, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R"})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(event.Event) error { return nil }}},
	}
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	for i := 0; i < 50; i++ { // the old race picked the intake about half the time: 50 tries make a pass by luck ~2^-50
		expired, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
		_, err := r.Drain(expired, 0)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("try %d: Drain with an expired context: %v, want DeadlineExceeded", i, err)
		}
		if err := r.Send(ctx, 0, rtEv(0, rtAt(float64(i)), rtStr("a"))); err != nil {
			t.Fatalf("try %d: Send after a Drain that queued nothing: %v, want nil (the app is not drained)", i, err)
		}
	}
	if _, err := r.Drain(ctx, 0); err != nil {
		t.Fatalf("a real Drain afterwards: %v, want nil", err)
	}
}
