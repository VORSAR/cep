package runtime

// Known issue S3.3-1-2 (design docs/sprints/s3-3-2-design.md v2, §9.1): Add and Activate run inside the in-flight
// gate; Close closes it first and waits, bounded by its ctx, for the calls inside to leave, then drains and removes
// every app. Run under -race. Internal: the gate is unexported, and an Add in flight is observed through it. Prefix: cg.

import (
	"context"
	"errors"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// cgCeiling breaks a hang only (testing.md rule 4); generous for -race under a CPU hog.
const cgCeiling = 60 * time.Second

func cgCtx(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cgCeiling)
	t.Cleanup(cancel)
	return ctx
}

var cgT0 = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()

// cgPass is an on-arrival body that emits each event on stream 1 as (k, ts).
type cgPass struct {
	partition.TimerFree
	emit Emit
}

type cgState struct{}

func (cgState) Walk(func(component, key string, state any)) {}
func (cgState) Timers(dst []clock.Timer) []clock.Timer      { return dst }

func (b *cgPass) NewState(*partition.Instance) partition.OpState { return cgState{} }
func (b *cgPass) OnEvent(_ *partition.Instance, e event.Event) {
	b.emit(event.Event{StreamID: 1, Ts: e.Ts, Values: []event.Value{e.Values[0], {Kind: event.TTimestamp, T: e.Ts}}})
}

// cgApp is one source stream s (k string, ts) and one derived stream out, a pass-through rule, and sink on out.
func cgApp(t testing.TB, sink Sink) App {
	t.Helper()
	attrs := []event.Attr{{Name: "k", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}}
	s, err := event.NewSchema("s", attrs, 1)
	if err != nil {
		t.Fatal(err)
	}
	o, err := event.NewSchema("out", attrs, 1)
	if err != nil {
		t.Fatal(err)
	}
	return App{
		Streams: []clock.StreamSpec{{Name: "s"}, {Name: "out", Derived: true}},
		Schemas: []event.Schema{s, o},
		Rules: []Rule{{Name: "pass", Inputs: []int{0}, KeyTypes: []event.Type{event.TString}, OnArrival: true,
			NewBody: func(emit func(event.Event)) partition.Body { return &cgPass{emit: emit} }}},
		Sinks: []SinkBinding{{Stream: 1, Sink: sink}},
	}
}

func cgEv(i int) event.Event {
	ts := cgT0 + int64(i)
	return event.Event{StreamID: 0, Ts: ts, Values: []event.Value{{Kind: event.TString, Str: "k"}, {Kind: event.TTimestamp, T: ts}}}
}

// cgRuntime is a one-shard runtime with one-slot queues and a fixed clock; the test closes it itself.
func cgRuntime(t testing.TB) *Runtime {
	t.Helper()
	r, err := New(Config{Shards: 1, Mailbox: 1, Intake: 1, SinkQueue: 1, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// cgHeld is a sink whose first call closes entered and blocks until release, ignoring every context.
type cgHeld struct {
	once, free sync.Once
	entered    chan struct{}
	release    chan struct{}
}

func newCGHeld() *cgHeld { return &cgHeld{entered: make(chan struct{}), release: make(chan struct{})} }

func (h *cgHeld) sink(event.Event) error {
	h.once.Do(func() { close(h.entered) })
	<-h.release
	return nil
}
func (h *cgHeld) let() { h.free.Do(func() { close(h.release) }) }

// cgBlockRouter activates an app on h's sink and sends until the router waits on the full sink queue: event 1 is
// held in the sink, 2 fills the queue, 3 is the router's pending send, 4 sits in the intake (Send returns once
// queued, and with Intake 1 it can queue 4 only once the router took 3).
func cgBlockRouter(t testing.TB, ctx context.Context, r *Runtime, h *cgHeld) AppID {
	t.Helper()
	id, err := r.Add(ctx, cgApp(t, h.sink))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Fatalf("Send 1: %v", err)
	}
	select {
	case <-h.entered:
	case <-ctx.Done():
		t.Fatal("the sink was never called: the ceiling")
	}
	for i := 2; i <= 4; i++ {
		if err := r.Send(ctx, id, cgEv(i)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	return id
}

// cgRemoved reports whether app id's slot is gone or removed.
func cgRemoved(r *Runtime, id AppID) bool {
	apps := r.appList()
	if int(id) >= len(apps) || apps[id] == nil {
		return true
	}
	return appPhase(apps[id].phase.Load()) == phaseRemoved
}

// cgUntil polls cond (yielding each time: testing.md, a polling loop must Gosched) until it holds or ctx ends.
func cgUntil(ctx context.Context, cond func() bool) bool {
	for !cond() {
		if ctx.Err() != nil {
			return false
		}
		goruntime.Gosched()
	}
	return true
}

func (g *inflightGate) cgRead() (closing bool, n int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closing, g.n
}

// The gate alone: enter is refused once close has begun; close's channel is closed at once when no call is inside,
// and after the last leave otherwise.
func TestInflightGateRefusesAfterClose(t *testing.T) {
	defer func() {
		if v := recover(); v != nil {
			t.Fatalf("the gate panicked: %v", v) // the step-1 stub: not built
		}
	}()
	closed := func(ch <-chan struct{}) bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
	var empty inflightGate
	if ch := empty.close(); !closed(ch) {
		t.Error("close of an empty gate: its channel is open; want it closed at once")
	}
	if empty.enter() {
		t.Error("enter after close: admitted; want refused")
	}

	var g inflightGate
	if !g.enter() || !g.enter() {
		t.Fatal("enter before close: refused; want admitted")
	}
	ch := g.close()
	if closed(ch) {
		t.Error("close with two calls inside: its channel is closed; want it open")
	}
	if g.enter() {
		t.Error("enter after close, calls inside: admitted; want refused")
	}
	g.leave()
	if closed(ch) {
		t.Error("one call still inside: the channel is closed; want it open")
	}
	g.leave()
	if !closed(ch) {
		t.Error("the last call left: the channel is open; want it closed")
	}
}

// B2, A1: Close behind a router that waits on a blocked sink (a Write that ignores its context) returns its ctx's
// error at its deadline. A guard today (Close's drain is bounded by its ctx); red against a barrier awaited through
// requestQueued, which waits whatever ctx does.
func TestCloseReturnsAtDeadlineBehindABlockedSink(t *testing.T) {
	ctx := cgCtx(t)
	r := cgRuntime(t)
	h := newCGHeld()
	t.Cleanup(h.let)
	cgBlockRouter(t, ctx, r, h)

	cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Close(cctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Close behind a blocked sink: %v; want context.DeadlineExceeded", err)
		}
	case <-ctx.Done():
		t.Fatal("Close behind a blocked sink never returned: the ceiling")
	}
}

// A3a: Activate racing Close on one P, 200 times: whichever wins, Close returns nil and the app ends removed;
// Activate returns nil or ErrClosed.
func TestCloseDrainsAnAppActivatedBeforeItBegan(t *testing.T) {
	defer goruntime.GOMAXPROCS(goruntime.GOMAXPROCS(1))
	ctx := cgCtx(t)
	for i := range 200 {
		r := cgRuntime(t)
		id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
		if err != nil {
			t.Fatalf("iteration %d: Add: %v", i, err)
		}
		act := make(chan error, 1)
		go func() { act <- r.Activate(ctx, id) }()
		if err := r.Close(ctx); err != nil {
			t.Fatalf("iteration %d: Close: %v; want nil", i, err)
		}
		if err := <-act; err != nil && !errors.Is(err, ErrClosed) {
			t.Fatalf("iteration %d: Activate: %v; want nil or ErrClosed", i, err)
		}
		if !cgRemoved(r, id) {
			t.Fatalf("iteration %d: after Close the app is in phase %d; want removed", i, r.appList()[id].phase.Load())
		}
	}
}

// A3b, S3: an Add in flight when Close begins — its install held inside the router (the addHold seam). Close closes
// the gate and waits; once the hold is released the Add returns its id, and Close drains and removes both apps and
// returns nil. (Fixture edit, named in the record: QA's first form held the install behind another app's blocked
// sink, which lost the race about 1 run in 9 under -count=50 -cpu 1,4 — the router still reads handoffs while it
// waits on a sink, so an Add read just before the wait completed at once. The seam holds it deterministically.)
func TestCloseWaitsForAnAddInFlight(t *testing.T) {
	ctx := cgCtx(t)
	r := cgRuntime(t)
	first, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatalf("Add of the first app: %v", err)
	}
	hold := make(chan struct{})
	h := &cgHeld{} // the release below: let() closes hold once
	h.release = hold
	t.Cleanup(h.let)
	r.addHold = func() { <-hold }

	type added struct {
		id  AppID
		err error
	}
	add := make(chan added, 1)
	go func() {
		id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
		add <- added{id, err}
	}()
	// The gate's two steps take no lock the router holds: 10 s only breaks a hang (and bounds today's red run).
	gctx, gcancel := context.WithTimeout(ctx, 10*time.Second)
	defer gcancel()
	if !cgUntil(gctx, func() bool { _, n := r.inflight.cgRead(); return n == 1 }) {
		t.Fatal("the Add never entered the in-flight gate within 10 s")
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close(ctx) }()
	if !cgUntil(gctx, func() bool { c, _ := r.inflight.cgRead(); return c }) {
		t.Fatal("Close never closed the in-flight gate within 10 s")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned %v with an Add in flight; want it waiting", err)
	case a := <-add:
		t.Fatalf("the Add returned %v, %v while its install is held", a.id, a.err)
	default:
	}
	h.let()
	var a added
	select {
	case a = <-add:
	case <-ctx.Done():
		t.Fatal("the Add never returned: the ceiling")
	}
	if a.err != nil {
		t.Fatalf("the Add in flight: %v; want its id (it entered before Close began)", a.err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close: %v; want nil", err)
		}
	case <-ctx.Done():
		t.Fatal("Close never returned: the ceiling")
	}
	for _, id := range []AppID{first, a.id} {
		if !cgRemoved(r, id) {
			t.Errorf("app %d after Close: phase %d; want removed", id, r.appList()[id].phase.Load())
		}
	}
}

// A3: a Remove that fails with a non-ctx error appears in Close's result, never dropped (QA lane 2's test; the seam is
// removeFault).
func TestCloseReportsARemoveError(t *testing.T) {
	ctx := cgCtx(t)
	r := cgRuntime(t)
	planted := errors.New("cg: a planted remove fault")
	r.removeFault = func(AppID) error { return planted }
	if _, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil })); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Close(ctx); !errors.Is(err, planted) {
		t.Errorf("Close: %v; want it to carry the planted Remove error (errors.Join, A3)", err)
	}
}

// The combined review's R-S3: an Activate the router answered, held before its tail while Close gives up at its
// deadline, returns ErrClosed and starts nothing once released (it used to start its flushers, set the phase and
// return nil on a closed runtime).
func TestActivateAfterCloseGaveUpIsErrClosed(t *testing.T) {
	ctx := cgCtx(t)
	r := cgRuntime(t)
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	hold, held := make(chan struct{}), make(chan struct{})
	r.activateHold = func() { close(held); <-hold }
	act := make(chan error, 1)
	go func() { act <- r.Activate(ctx, id) }()
	select {
	case <-held:
	case <-ctx.Done():
		t.Fatal("Activate never reached its tail: the ceiling")
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := r.Close(cctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close with a cancelled ctx: %v; want context.Canceled", err)
	}
	close(hold)
	select {
	case err := <-act:
		if !errors.Is(err, ErrClosed) {
			t.Errorf("Activate after Close gave up: %v; want ErrClosed", err)
		}
	case <-ctx.Done():
		t.Fatal("Activate never returned: the ceiling")
	}
	if ph := appPhase(r.appList()[id].phase.Load()); ph == phaseActive {
		t.Errorf("the app is active on a closed runtime; want it never activated by the caller")
	}
}
