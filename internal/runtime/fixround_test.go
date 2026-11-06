package runtime_test

// The coordinator's tests for S3.2a-2's fix round (2026-11-05), each red before its fix:
//   - the owner's decision on the break-it's question: a retried Drain waits for the same flush;
//   - two gaps the breaker noted without a test: a schema whose stream name differs from its stream's, and a NewBody
//     that panics inside Start;
//   - Send refuses a NaN or infinite float (S3.1-2's D2: the Go API refuses non-finite floats).

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// fxApp: stream "in" [k string] (Delay 0), derived "out" [what string]; rule R keyed by k emits "ev@<s>" per release.
func fxApp(t *testing.T, sink runtime.Sink) runtime.App {
	t.Helper()
	return runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("what", event.TString))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
				b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{rtStr(fmt.Sprintf("ev@%d", (e.Ts-rtT0)/rtSec))}})
			}})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: sink}},
	}
}

// TestRetriedDrainWaitsForTheSameFlush (owner, 2026-11-05): the first Drain is cancelled while its flush is held at a
// slow sink; a second Drain started then returns only after the flush, with the same Pending, and every output is
// through the sink when it returns. By hand: a@1 s, a@2 s, a@3 s; the drain (to 3.001 s) releases all three: three
// outputs; Pending 0 (no body timers).
func TestRetriedDrainWaitsForTheSameFlush(t *testing.T) {
	ctx := rtCeiling(t)
	out := newRTLog()
	hold := newRTHold()
	r := rtStart(t, runtime.Config{Shards: 2}, fxApp(t, func(e event.Event) error {
		hold.at()
		out.add("out", e.Values[0].Str)
		return nil
	}))
	t.Cleanup(hold.free)
	for _, s := range []float64{1, 2, 3} {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(s), rtStr("a")))
	}
	hold.arm()
	first, cancelFirst := context.WithCancel(ctx)
	firstDone := make(chan error, 1)
	go func() { _, err := r.Drain(first, 0); firstDone <- err }()
	rtWait(t, ctx, hold.entered, "the drain's first output held at the sink")
	cancelFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first Drain, cancelled during its flush: %v, want context.Canceled", err)
	}
	type result struct {
		res runtime.DrainResult
		err error
	}
	second := make(chan result, 1)
	go func() { res, err := r.Drain(ctx, 0); second <- result{res, err} }()
	hold.free()
	got := <-second
	if got.err != nil {
		t.Fatalf("second Drain: %v, want nil: it waits for the first drain's flush (owner, 2026-11-05)", got.err)
	}
	if want := []string{"ev@1", "ev@2", "ev@3"}; !rtEqual(out.get("out"), want) {
		t.Errorf("sink when the second Drain returned: %v, want %v", out.get("out"), want)
	}
	if got.res.Pending != 0 {
		t.Errorf("Pending %d, want 0", got.res.Pending)
	}
}

// TestStartRefusesSchemaNameMismatch: a schema aligned by index must describe its own stream.
func TestStartRefusesSchemaNameMismatch(t *testing.T) {
	app := fxApp(t, func(event.Event) error { return nil })
	app.Schemas[0] = rtSchema(t, "other", rtA("k", event.TString))
	if _, err := runtime.Start(runtime.Config{Shards: 1, Now: rtNow}, []runtime.App{app}); err == nil {
		t.Fatal("Start accepted a schema named \"other\" for stream \"in\"")
	}
}

// TestStartRecoversNewBodyPanic: a NewBody that panics makes Start return an error, never panic its caller.
func TestStartRecoversNewBodyPanic(t *testing.T) {
	app := fxApp(t, func(event.Event) error { return nil })
	app.Rules[0].NewBody = func(runtime.Emit) partition.Body { panic("a planner bug") }
	var err error
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("Start panicked: %v, want an error", p)
			}
		}()
		_, err = runtime.Start(runtime.Config{Shards: 2, Now: rtNow}, []runtime.App{app})
	}()
	if err == nil {
		t.Fatal("Start with a panicking NewBody returned no error")
	}
}

// TestSendRefusesNonFiniteFloat: NaN and ±Inf are never values (S3.1-2's D2); Send refuses them in any float
// attribute, key or not.
func TestSendRefusesNonFiniteFloat(t *testing.T) {
	ctx := rtCeiling(t)
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString), rtA("f", event.TFloat)), rtSchema(t, "out", rtA("what", event.TString))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R"})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(event.Event) error { return nil }}},
	}
	r := rtStart(t, runtime.Config{Shards: 1}, app)
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		e := event.Event{StreamID: 0, Ts: rtAt(1), Values: []event.Value{rtStr("a"), {Kind: event.TFloat, F: f}}}
		if err := r.Send(ctx, 0, e); err == nil {
			t.Errorf("Send accepted the float %v", f)
		}
	}
}
