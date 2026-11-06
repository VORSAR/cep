package runtime_test

import (
	"context"
	"errors"
	"fmt"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
	"github.com/vorsar/cep/internal/table"
)

// rtHasBody records, per released event, whether its own Tables snapshot has x = "p" in table 0.
func rtHasBody(log *rtLog) func(runtime.Emit) partition.Body {
	return rtBodies(rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
		snap, ok := e.Tables.(*table.Snapshot)
		if !ok || snap == nil {
			log.add(b.key(in), fmt.Sprintf("tables=%T", e.Tables))
			return
		}
		log.add(b.key(in), fmt.Sprintf("has=%v", snap.Table(0).Has(rtStr("p"))))
	}})
}

// TestTableWriteInIntakeOrder (owner F14; design v2 §2, §4): a TableWrite is an intake item, applied by the router in
// intake order, so an event sent before it reads the old table and one sent after reads the new, with no barrier.
// Table T(x string) keyed x; stream "in" (Delay 0; [k string]); rule R keyed by k records has=<"p" in T> from the
// event's own Tables. Sequence: a@1 s, put p, b@2 s, delete p, c@3 s, Drain. Expected: R/a [has=false], R/b
// [has=true], R/c [has=false]. A put of a row with the wrong arity and a write to table 1 (out of range) return errors.
func TestTableWriteInIntakeOrder(t *testing.T) {
	ctx := rtCeiling(t)
	tb, err := table.New("T", []event.Attr{rtA("x", event.TString)}, []string{"x"})
	if err != nil {
		t.Fatalf("table.New: %v", err)
	}
	log := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0)},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString))},
		Tables:  table.NewSet([]*table.Table{tb}),
		Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtHasBody(log)}},
	}
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtStr("p")}}); err != nil {
		t.Fatalf("put: %v", err)
	}
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(2), rtStr("b")))
	if err := r.TableWrite(ctx, 0, 0, runtime.TableDelete, [][]event.Value{{rtStr("p")}}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(3), rtStr("c")))
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, [][]event.Value{{rtStr("q"), rtStr("extra")}}); err == nil {
		t.Errorf("a put with the wrong arity: nil, want the table's error")
	}
	if err := r.TableWrite(ctx, 0, 1, runtime.TablePut, [][]event.Value{{rtStr("q")}}); err == nil {
		t.Errorf("a write to table 1 (out of range): nil, want an error")
	}
	rtDrain(t, ctx, r, 0)
	want := map[string][]string{"R/a": {"has=false"}, "R/b": {"has=true"}, "R/c": {"has=false"}}
	got := log.snapshot()
	for k, w := range want {
		if !rtEqual(got[k], w) {
			t.Errorf("%s: %v, want %v", k, got[k], w)
		}
	}
}

// TestSendValidation (design v2 §2, F9): Send checks in the caller's goroutine and returns an error, never panicking
// the router. Stream "in" (Delay 0; [k string, n int]), derived "out" ([k string]); rule R keyed by k. Refused: app 7
// (out of range); stream 1 (derived); stream 5 (out of range); one value (want 2); n given as a string. Then a valid
// a@1 s and Drain: the runtime is alive, and R released only a (s0#1: the refused Sends took no Seq).
func TestSendValidation(t *testing.T) {
	ctx := rtCeiling(t)
	log := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString), rtA("n", event.TInt)), rtSchema(t, "out", rtA("k", event.TString))},
		Rules:   []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec, NewBody: rtBodies(rtBody{rule: "R", log: log})}},
	}
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	for _, c := range []struct {
		name string
		app  runtime.AppID
		e    event.Event
	}{
		{"app out of range", 7, rtEv(0, rtAt(1), rtStr("a"), rtInt(1))},
		{"a derived stream", 0, rtEv(1, rtAt(1), rtStr("a"))},
		{"stream out of range", 0, rtEv(5, rtAt(1), rtStr("a"), rtInt(1))},
		{"too few values", 0, rtEv(0, rtAt(1), rtStr("a"))},
		{"a value of the wrong kind", 0, rtEv(0, rtAt(1), rtStr("a"), rtStr("1"))},
	} {
		if err := r.Send(ctx, c.app, c.e); err == nil {
			t.Errorf("%s: Send returned nil, want an error", c.name)
		}
	}
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a"), rtInt(1)))
	rtDrain(t, ctx, r, 0)
	if got := log.snapshot(); len(got) != 1 || !rtEqual(got["R/a"], []string{"ev s0#1 @1000"}) {
		t.Errorf("released %v, want only R/a [ev s0#1 @1000]", got)
	}
}

// TestSinkPanicCounted (design v2 §3, F8; §6, F12): a flusher recovers its sink's panic, counts it as a sink error,
// and continues. Stream "in" (Delay 0; [k string]), derived "out" ([n int]); rule R keyed by k emits one output per
// release, n = its Seq. Events 1..4 s; Drain releases all four. The sink panics on n = 1 and 3, returns an error on
// n = 4, and accepts n = 2. Expected after Drain: the sink was called for 1, 2, 3, 4 in order; SinkErrors 3.
func TestSinkPanicCounted(t *testing.T) {
	ctx := rtCeiling(t)
	calls := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("n", event.TInt))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R", onEvent: func(b *rtBody, in *partition.Instance, e event.Event) {
				b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{rtInt(int64(e.Seq))}})
			}})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(e event.Event) error {
			n := e.Values[0].I
			calls.add("sink", fmt.Sprint(n))
			switch n {
			case 1, 3:
				panic("a sink's panic")
			case 4:
				return errors.New("a sink's error")
			}
			return nil
		}}},
	}
	r := rtStart(t, runtime.Config{Shards: 2}, app)
	for i := 1; i <= 4; i++ {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(i)), rtStr("a")))
	}
	rtDrain(t, ctx, r, 0)
	if got := calls.get("sink"); !rtEqual(got, []string{"1", "2", "3", "4"}) {
		t.Errorf("sink calls %v, want [1 2 3 4] (the flusher continues after a panic)", got)
	}
	if n := rtStats(t, ctx, r).SinkErrors[0][0]; n != 3 {
		t.Errorf("SinkErrors %d, want 3 (two panics, one error)", n)
	}
}

// TestCloseStopsEverything (design v2 §2, §6; F7, F17). The goroutine count is taken before Start; a runtime with 4
// shards, a sink, and TickEvery 1 h (the ticker goroutine exists, never fires) gets a@1 s and a@2 s (a's release
// schedules Completion at ts + 1 h, emitted on fire). Close returns nil; the count is then polled (runtime.Gosched, no
// sleep) until it is back to the baseline or the ceiling passes (F17). After Close every method returns ErrClosed,
// and a second Close returns nil. No timer output ever reached the sink (Close fires nothing). Not parallel: it
// counts the process's goroutines.
func TestCloseStopsEverything(t *testing.T) {
	ctx := rtCeiling(t)
	base := goruntime.NumGoroutine()
	outs := newRTLog()
	app := runtime.App{
		Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
		Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString)), rtSchema(t, "out", rtA("what", event.TString))},
		Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 7200 * rtSec,
			NewBody: rtBodies(rtBody{rule: "R",
				sched: func(e event.Event, n uint64) []clock.Timer {
					return []clock.Timer{{Due: e.Ts + 3600*rtSec, Kind: clock.Completion, Ref: n}}
				},
				onTimers: func(b *rtBody, in *partition.Instance, fires []clock.Timer, _ any) {
					b.emit(event.Event{StreamID: 1, Ts: fires[0].Due, Values: []event.Value{rtStr("tm")}})
				}})}},
		Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(e event.Event) error { outs.add("out", e.Values[0].Str); return nil }}},
	}
	r, err := runtime.Start(runtime.Config{Shards: 4, TickEvery: time.Hour, Now: rtNow}, []runtime.App{app})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(1), rtStr("a")))
	rtSend(t, ctx, r, 0, rtEv(0, rtAt(2), rtStr("a")))
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for n := goruntime.NumGoroutine(); n > base; n = goruntime.NumGoroutine() {
		if ctx.Err() != nil {
			buf := make([]byte, 1<<16)
			buf = buf[:goruntime.Stack(buf, true)]
			t.Fatalf("ceiling hit waiting for the goroutine count to fall to %d (it is %d):\n%s", base, n, buf)
		}
		goruntime.Gosched()
	}
	if err := r.Send(ctx, 0, rtEv(0, rtAt(3), rtStr("a"))); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("Send after Close: %v, want ErrClosed", err)
	}
	if err := r.Tick(ctx, rtAt(4)); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("Tick after Close: %v, want ErrClosed", err)
	}
	if err := r.TableWrite(ctx, 0, 0, runtime.TablePut, nil); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("TableWrite after Close: %v, want ErrClosed", err)
	}
	if _, err := r.Drain(ctx, 0); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("Drain after Close: %v, want ErrClosed", err)
	}
	if _, err := r.Stats(ctx); !errors.Is(err, runtime.ErrClosed) {
		t.Errorf("Stats after Close: %v, want ErrClosed", err)
	}
	if err := r.Close(ctx); err != nil {
		t.Errorf("a second Close: %v, want nil", err)
	}
	if got := outs.get("out"); len(got) != 0 {
		t.Errorf("sink outputs %v, want none (Close fires no pending timer)", got)
	}
}

// TestStartValidation (design v2 §2, F9, F21; QA Q4): Start refuses each bad app with an error (never a panic) and
// starts a valid one. The base app: "in" (Delay 0; [k string, n int]), derived "out" ([k string]); rule R reads in,
// keyed by k (attr 0, TString), TTL 1 h, a body; a sink on out.
func TestStartValidation(t *testing.T) {
	base := func(t *testing.T) runtime.App {
		return runtime.App{
			Streams: []clock.StreamSpec{rtSrc("in", 0), rtDerived("out")},
			Schemas: []event.Schema{rtSchema(t, "in", rtA("k", event.TString), rtA("n", event.TInt)), rtSchema(t, "out", rtA("k", event.TString))},
			Rules: []runtime.Rule{{Name: "R", Inputs: []int{0}, KeyAttrs: [][]int{{0}}, KeyTypes: []event.Type{event.TString}, TTL: 3600 * rtSec,
				NewBody: rtBodies(rtBody{rule: "R"})}},
			Sinks: []runtime.SinkBinding{{Stream: 1, Sink: func(event.Event) error { return nil }}},
		}
	}
	for _, c := range []struct {
		name string
		bad  func(a *runtime.App)
	}{
		{"an input out of range", func(a *runtime.App) { a.Rules[0].Inputs = []int{2}; a.Rules[0].KeyAttrs = [][]int{{0}} }},
		{"a derived input", func(a *runtime.App) { a.Rules[0].Inputs = []int{1} }},
		{"KeyAttr out of range", func(a *runtime.App) { a.Rules[0].KeyAttrs = [][]int{{2}} }},
		{"KeyAttr's length is not Inputs'", func(a *runtime.App) { a.Rules[0].KeyAttrs = [][]int{{0}, {0}} }},
		{"KeyAttr's type is not KeyType", func(a *runtime.App) { a.Rules[0].KeyAttrs = [][]int{{1}} }},
		{"a keyed rule with TTL 0", func(a *runtime.App) { a.Rules[0].TTL = 0 }},
		{"a key-less rule with a TTL", func(a *runtime.App) { a.Rules[0].KeyAttrs = nil }},
		{"a sink on a source stream", func(a *runtime.App) { a.Sinks[0].Stream = 0 }},
		{"a sink out of range", func(a *runtime.App) { a.Sinks[0].Stream = 9 }},
		{"a nil NewBody", func(a *runtime.App) { a.Rules[0].NewBody = nil }},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
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
	t.Run("valid", func(t *testing.T) { rtStart(t, runtime.Config{Shards: 2}, base(t)) })
}
