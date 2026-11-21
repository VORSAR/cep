package runtime

// The lifecycle's waits at their deadline and at Close: each Drain, Remove and Close wait gives up at its own ctx or
// ends when Close stops the runtime, with the error its contract names (lifecycle.go's Drain, Remove and Close docs).
// Internal: the deadline is a cancel fired once the caller is parked in the wait under test (its goroutine's stack),
// or by the held fixture itself, never by a timer. Prefix: ld.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// heldFlushRecorder is a Recorder whose record calls return at once, counted, and whose every Flush closes entered,
// calls onFlush (when set) and blocks until release.
type heldFlushRecorder struct {
	onFlush func()
	entered chan struct{}
	release chan struct{}
	enter   func() // closes entered, once
	let     func() // closes release, once

	mu      sync.Mutex
	records int // every record call, Stop included
	stops   int
	flushes int
}

func newHeldFlushRecorder(onFlush func()) *heldFlushRecorder {
	h := &heldFlushRecorder{onFlush: onFlush, entered: make(chan struct{}), release: make(chan struct{})}
	h.enter = sync.OnceFunc(func() { close(h.entered) })
	h.let = sync.OnceFunc(func() { close(h.release) })
	return h
}

func (h *heldFlushRecorder) record(stop bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records++
	if stop {
		h.stops++
	}
	return nil
}

func (h *heldFlushRecorder) Header(int64) error                            { return h.record(false) }
func (h *heldFlushRecorder) Event(*event.Event, int64, []byte) error       { return h.record(false) }
func (h *heldFlushRecorder) Rejected(int, int64, *event.DecodeError) error { return h.record(false) }
func (h *heldFlushRecorder) Tick(int64) error                              { return h.record(false) }
func (h *heldFlushRecorder) Table(int, TableOp, [][]event.Value) error     { return h.record(false) }
func (h *heldFlushRecorder) Stop() error                                   { return h.record(true) }
func (h *heldFlushRecorder) BatchEnd(int, string) error                    { return h.record(false) }

func (h *heldFlushRecorder) Flush() error {
	h.mu.Lock()
	h.flushes++
	h.mu.Unlock()
	h.enter()
	if h.onFlush != nil {
		h.onFlush()
	}
	<-h.release
	return nil
}

// counts is the record calls, the Stop calls and the Flush calls so far.
func (h *heldFlushRecorder) counts() (records, stops, flushes int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.records, h.stops, h.flushes
}

// ldRuntime is a one-shard runtime whose sink queues hold 4 (the router never waits on a held sink here) and a fixed
// clock; ldClose closes it in the test's cleanup.
func ldRuntime(t *testing.T) *Runtime {
	t.Helper()
	r, err := New(Config{Shards: 1, SinkQueue: 4, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// ldClose is the cleanup's Close: every goroutine back once the test released what it held.
func ldClose(t *testing.T, r *Runtime) {
	t.Helper()
	if err := r.Close(cgCtx(t)); err != nil {
		t.Errorf("the cleanup's Close: %v; want nil once every held call is released", err)
	}
}

// ldExpired is a context that has already ended (context.Canceled).
func ldExpired() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// ldStarted adds app and activates it.
func ldStarted(t *testing.T, ctx context.Context, r *Runtime, app App) AppID {
	t.Helper()
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	return id
}

// ldParked says goroutine gid is parked in a select with (*Runtime).fn on its stack and none of the frames not.
func ldParked(gid, fn string, not ...string) bool {
	for _, s := range rsStacks() {
		if !strings.HasPrefix(s, "goroutine "+gid+" [select") {
			continue
		}
		if !strings.Contains(s, "internal/runtime.(*Runtime)."+fn+"(") {
			return false
		}
		for _, n := range not {
			if strings.Contains(s, n) {
				return false
			}
		}
		return true
	}
	return false
}

// ldWhen runs do on its own goroutine once cond holds; at ctx's end it gives up, and the caller's wait under test then
// fails at the test's own deadline.
func ldWhen(ctx context.Context, cond func() bool, do func()) {
	go func() {
		if cgUntil(ctx, cond) {
			do()
		}
	}()
}

// ldInWait are the frames of the calls that precede a lifecycle function's own waits: a goroutine parked under one of
// them is not yet in the wait under test.
var ldInWait = []string{"(*Runtime).await(", "(*Runtime).awaitRuns(", "(*Runtime).enqueue(", "(*Runtime).requestQueued("}

// ldHeldBody returns an on-arrival body whose OnEvent blocks until let; entered closes at the first.
func ldHeldBody() (body *rsHeld, let func()) {
	body = &rsHeld{entered: make(chan struct{}), release: make(chan struct{}), once: &sync.Once{}}
	return body, sync.OnceFunc(func() { close(body.release) })
}

// ldWait waits for ch under ctx, naming the state.
func ldWait(t *testing.T, ctx context.Context, ch <-chan struct{}, state string) {
	t.Helper()
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatalf("%s: the ceiling", state)
	}
}

// ldErr receives a goroutine's error under ctx, naming it.
func ldErr(t *testing.T, ctx context.Context, ch <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		t.Fatalf("%s never returned: the ceiling", what)
		return nil
	}
}

// ---- Drain ----

// A Drain whose ctx has ended queues nothing (enqueue refuses an expired ctx): the never-activated app is not drained
// (QA Q15; break-it R1) — its phase reverts to inactive and the drain flag clears, so it activates and takes input.
func TestDrainOfAnInactiveAppWithAnExpiredContextLeavesItInactive(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := r.Drain(ldExpired(), id); !errors.Is(err, context.Canceled) {
		t.Fatalf("Drain with an expired ctx: %v; want context.Canceled", err)
	}
	a := r.appList()[id]
	if p := appPhase(a.phase.Load()); p != phaseInactive {
		t.Errorf("phase after the refused Drain: %d; want inactive (%d)", p, phaseInactive)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatalf("Activate after the refused Drain: %v; want nil (the app was never drained)", err)
	}
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Errorf("Send after the refused Drain: %v; want nil (the drain flag is clear)", err)
	}
}

// A Drain of an active app whose ctx has ended queues nothing (the quiet item is refused): the app stays active and
// takes input (break-it R1).
func TestDrainOfAnActiveAppWithAnExpiredContextLeavesItActive(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	if _, err := r.Drain(ldExpired(), id); !errors.Is(err, context.Canceled) {
		t.Fatalf("Drain with an expired ctx: %v; want context.Canceled", err)
	}
	if p := appPhase(r.appList()[id].phase.Load()); p != phaseActive {
		t.Errorf("phase after the refused Drain: %d; want active (%d)", p, phaseActive)
	}
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Errorf("Send after the refused Drain: %v; want nil", err)
	}
}

// Drain waits for the router's barrier answer until its deadline: the shard holds an event in its body, so the barrier
// never answers, and Drain returns ctx.Err() — the barrier is queued, so the app stays draining (no revert, I2).
func TestDrainGivesUpAtItsDeadlineBeforeTheBarrierAnswers(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	body, let := ldHeldBody()
	t.Cleanup(func() { let(); ldClose(t, r) })
	app := cgApp(t, func(event.Event) error { return nil })
	app.Rules[0].NewBody = func(func(event.Event)) partition.Body { return body }
	id := ldStarted(t, ctx, r, app)
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	ldWait(t, ctx, body.entered, "the shard never reached the body")
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ldWhen(ctx, func() bool { return ldParked(self, "await") }, cancel)
	res, err := r.Drain(dctx, id)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Drain at its deadline with the barrier unanswered: %v; want context.Canceled", err)
	}
	if res.Pending != 0 || len(res.Runs) != 0 {
		t.Errorf("DrainResult %+v; want zero (no barrier answer)", res)
	}
	if p := appPhase(r.appList()[id].phase.Load()); p != phaseDraining {
		t.Errorf("phase after the given-up Drain: %d; want draining (%d): the barrier was queued", p, phaseDraining)
	}
}

// Drain waiting on a sink's marker ends with ErrClosed once Close has given up and stopped the runtime.
func TestDrainWaitingOnASinkIsErrClosedWhenCloseGivesUp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	h := newCGHeld()
	t.Cleanup(func() { h.let(); ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, h.sink))
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	ldWait(t, ctx, h.entered, "the sink was never called")
	closed := make(chan error, 1)
	ldWhen(ctx, func() bool { return ldParked(self, "Drain", ldInWait...) }, func() { closed <- r.Close(ldExpired()) })
	if _, err := r.Drain(ctx, id); !errors.Is(err, ErrClosed) {
		t.Errorf("Drain waiting on the held sink when Close gave up: %v; want ErrClosed", err)
	}
	if err := ldErr(t, ctx, closed, "Close"); !errors.Is(err, context.Canceled) {
		t.Errorf("Close with an expired ctx: %v; want context.Canceled", err)
	}
}

// Drain waiting on the recording returns once Close has given up: the stopped arm ends the wait without the deadline's
// cut, so the recording is not marked incomplete by this Drain (owner, 2026-11-06: waited for only until the deadline).
func TestDrainWaitingOnTheRecordingReturnsWhenCloseGivesUp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	w := newRDGated()
	t.Cleanup(func() { w.let(); ldClose(t, r) })
	app := cgApp(t, func(event.Event) error { return nil })
	app.Recorder = w
	id := ldStarted(t, ctx, r, app)
	ldWait(t, ctx, w.entered, "the recorder never reached its header")
	closed := make(chan error, 1)
	r.recordingWait = func() { go func() { closed <- r.Close(ldExpired()) }() }
	if _, err := r.Drain(ctx, id); err != nil {
		t.Errorf("Drain waiting on the held recording when Close gave up: %v; want nil", err)
	}
	if err := ldErr(t, ctx, closed, "Close"); !errors.Is(err, context.Canceled) {
		t.Errorf("Close with an expired ctx: %v; want context.Canceled", err)
	}
	if rs := r.appList()[id].rec.stats(); rs.Failed {
		t.Errorf("recording %+v after the stopped wait; want not failed (no deadline cut)", rs)
	}
}

// ---- Remove ----

// Remove calls OnRemoved once, and Close after it does not call it again (the shared sync.Once).
func TestRemoveCallsOnRemovedOnce(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	var n atomic.Int32
	app := cgApp(t, func(event.Event) error { return nil })
	app.OnRemoved = func() { n.Add(1) }
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Remove(ctx, id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := n.Load(); got != 1 {
		t.Errorf("OnRemoved calls after Remove: %d; want 1", got)
	}
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := n.Load(); got != 1 {
		t.Errorf("OnRemoved calls after Close: %d; want 1 (once per app)", got)
	}
}

// ldDrainingOnHeldSink activates an app on h's sink, holds an event in it and gives up a Drain at its deadline, so the
// app is draining (Remove accepts it) with the drain's marker unpassed behind the held sink.
func ldDrainingOnHeldSink(t *testing.T, ctx context.Context, r *Runtime, h *cgHeld, self string) AppID {
	t.Helper()
	id := ldStarted(t, ctx, r, cgApp(t, h.sink))
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	ldWait(t, ctx, h.entered, "the sink was never called")
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ldWhen(ctx, func() bool { return ldParked(self, "Drain", ldInWait...) }, cancel)
	if _, err := r.Drain(dctx, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("Drain behind the held sink at its deadline: %v; want context.Canceled", err)
	}
	return id
}

// Remove waiting on a sink's last marker ends with ErrClosed once Close has given up and stopped the runtime.
func TestRemoveWaitingOnASinkIsErrClosedWhenCloseGivesUp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	h := newCGHeld()
	t.Cleanup(func() { h.let(); ldClose(t, r) })
	id := ldDrainingOnHeldSink(t, ctx, r, h, self)
	closed := make(chan error, 1)
	ldWhen(ctx, func() bool { return ldParked(self, "Remove", ldInWait...) }, func() { closed <- r.Close(ldExpired()) })
	if err := r.Remove(ctx, id); !errors.Is(err, ErrClosed) {
		t.Errorf("Remove waiting on the held sink when Close gave up: %v; want ErrClosed", err)
	}
	if err := ldErr(t, ctx, closed, "Close"); !errors.Is(err, context.Canceled) {
		t.Errorf("Close with an expired ctx: %v; want context.Canceled", err)
	}
}

// Remove waiting on a sink's last marker gives up at its deadline with ctx.Err(); the app is removed meanwhile.
func TestRemoveGivesUpAtItsDeadlineBehindABlockedSink(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	h := newCGHeld()
	t.Cleanup(func() { h.let(); ldClose(t, r) })
	id := ldDrainingOnHeldSink(t, ctx, r, h, self)
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ldWhen(ctx, func() bool { return ldParked(self, "Remove", ldInWait...) }, cancel)
	if err := r.Remove(rctx, id); !errors.Is(err, context.Canceled) {
		t.Errorf("Remove behind the held sink at its deadline: %v; want context.Canceled", err)
	}
	if !cgRemoved(r, id) {
		t.Errorf("the app is not removed after the router's answer")
	}
}

// ldDrainedRecording activates an app with no sink and rec as its Recorder and drains it (the stop is written; Flush
// is not called at a drain), so a Remove then waits only on the recorder goroutine's return.
func ldDrainedRecording(t *testing.T, ctx context.Context, r *Runtime, rec Recorder) AppID {
	t.Helper()
	app := cgApp(t, func(event.Event) error { return nil })
	app.Sinks = nil
	app.Recorder = rec
	id := ldStarted(t, ctx, r, app)
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	return id
}

// Remove waiting on the recorder goroutine (held in its last Flush) ends with ErrClosed once Close has given up.
func TestRemoveWaitingOnTheRecorderIsErrClosedWhenCloseGivesUp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	closed := make(chan error, 1)
	h := newHeldFlushRecorder(sync.OnceFunc(func() { go func() { closed <- r.Close(ldExpired()) }() }))
	t.Cleanup(func() { h.let(); ldClose(t, r) })
	id := ldDrainedRecording(t, ctx, r, h)
	if err := r.Remove(ctx, id); !errors.Is(err, ErrClosed) {
		t.Errorf("Remove waiting on the flushing recorder when Close gave up: %v; want ErrClosed", err)
	}
	if err := ldErr(t, ctx, closed, "Close"); !errors.Is(err, context.Canceled) {
		t.Errorf("Close with an expired ctx: %v; want context.Canceled", err)
	}
}

// Remove waiting on the recorder goroutine (held in its last Flush) gives up at its deadline with ctx.Err().
func TestRemoveGivesUpAtItsDeadlineWhileTheRecordingFlushes(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h := newHeldFlushRecorder(cancel)
	t.Cleanup(func() { h.let(); ldClose(t, r) })
	id := ldDrainedRecording(t, ctx, r, h)
	if err := r.Remove(rctx, id); !errors.Is(err, context.Canceled) {
		t.Errorf("Remove while the recording flushes at its deadline: %v; want context.Canceled", err)
	}
	if _, _, f := h.counts(); f != 1 {
		t.Errorf("%d Flush calls; want 1 (the stop's)", f)
	}
}

// ---- Close ----

// Close's Remove gives up at the deadline while the removed recording flushes: Close returns ctx.Err().
func TestCloseGivesUpWhileARemovedRecordingFlushes(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h := newHeldFlushRecorder(cancel)
	t.Cleanup(func() { h.let(); ldClose(t, r) })
	app := cgApp(t, func(event.Event) error { return nil })
	app.Recorder = h
	ldStarted(t, ctx, r, app)
	if err := r.Close(cctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Close while the removed recording flushes: %v; want context.Canceled", err)
	}
	if _, s, f := h.counts(); s != 1 || f != 1 {
		t.Errorf("%d Stop and %d Flush calls; want 1 and 1 (drained, then removed)", s, f)
	}
}

// Close that gives up before removing an app still calls its OnRemoved, once, as it stops the runtime.
func TestCloseCallsOnRemovedOfAnAppItCouldNotRemove(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	var n atomic.Int32
	app := cgApp(t, func(event.Event) error { return nil })
	app.OnRemoved = func() { n.Add(1) }
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Close(ldExpired()); !errors.Is(err, context.Canceled) {
		t.Errorf("Close with an expired ctx: %v; want context.Canceled", err)
	}
	if cgRemoved(r, id) {
		t.Errorf("the app is removed; want it left in place (Close gave up first)")
	}
	if got := n.Load(); got != 1 {
		t.Errorf("OnRemoved calls: %d; want 1", got)
	}
	ldClose(t, r)
	if got := n.Load(); got != 1 {
		t.Errorf("OnRemoved calls after a second Close: %d; want 1", got)
	}
}

// A second Close waits for the goroutines the first left running only until its own deadline: ctx.Err() while a sink
// is held, nil once it is released (break-it S2).
func TestASecondCloseWaitsUntilItsOwnDeadline(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	h := newCGHeld()
	t.Cleanup(h.let)
	id := ldStarted(t, ctx, r, cgApp(t, h.sink))
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	ldWait(t, ctx, h.entered, "the sink was never called")
	if err := r.Close(ldExpired()); !errors.Is(err, context.Canceled) {
		t.Fatalf("the first Close: %v; want context.Canceled", err)
	}
	past, cancel := context.WithDeadline(ctx, time.UnixMilli(cgT0))
	defer cancel()
	if err := r.Close(past); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("the second Close with the sink held: %v; want context.DeadlineExceeded (its own deadline)", err)
	}
	h.let()
	if err := r.Close(ctx); err != nil {
		t.Errorf("the third Close after the release: %v; want nil", err)
	}
}

// Close's Drain gives up on a run that does not return: Close reports the drain's stuck-runs error, not a plain
// deadline (the Remove it would otherwise reach returns ctx.Err()).
func TestCloseReportsTheDrainsStuckRunError(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	release := make(chan struct{})
	t.Cleanup(func() { close(release); ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	run := func(src context.Context) error {
		<-src.Done() // the drain cancelled the source: its deadline passes now, with this run still running
		cancel()
		<-release
		return nil
	}
	if err := r.Run(ctx, id, run); err != nil {
		t.Fatalf("Run: %v", err)
	}
	err := r.Close(cctx)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "1 source goroutine(s) did not return before the deadline") {
		t.Errorf("Close with a stuck run: %v; want the drain's stuck-runs error wrapping context.Canceled", err)
	}
}

// Close's Remove that fails at the deadline gives Close its error as it is (the removeFault seam stands for a Remove
// error other than ctx's): not wrapped as a failed app, not replaced by ctx.Err(), and Close stops at that app — the
// apps are visited in reverse id order, so app 1's error is the one, and app 0 is not asked.
func TestCloseReportsTheRemoveErrorItGotAtItsDeadline(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range 2 {
		if _, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil })); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	faults := []error{errors.New("app 1's planted Remove fault"), errors.New("app 0's planted Remove fault")}
	var asked []AppID
	r.removeFault = func(app AppID) error { asked = append(asked, app); cancel(); return faults[len(asked)-1] }
	if err := r.Close(cctx); !errors.Is(err, faults[0]) || err.Error() != faults[0].Error() {
		t.Errorf("Close: %v; want app 1's Remove error itself", err)
	}
	if len(asked) != 1 || asked[0] != 1 {
		t.Errorf("Remove asked for %v; want [1] (Close stops at the first deadline error)", asked)
	}
}

// Close drains an app, and its deadline passes as the drain ends with the recording written: the drain is complete
// (the recording is kept, not Failed), so Close goes on to remove the app, and that Remove's error is Close's
// (removeFault, past the deadline).
func TestCloseRemovesAnAppWhoseDeadlinePassedAsItsDrainEnded(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	t.Cleanup(func() { r.recordingWait = nil; ldClose(t, r) })
	h := newHeldFlushRecorder(nil)
	h.let()
	app := cgApp(t, func(event.Event) error { return nil })
	app.Sinks = nil
	app.Recorder = h
	id := ldStarted(t, ctx, r, app)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r.recordingWait = func() {
		cancel()
		if !cgUntil(ctx, func() bool { _, s, _ := h.counts(); return s == 1 && ldRecorderIdle(self) }) {
			t.Error("the recorder never wrote the stop and parked: the ceiling")
		}
	}
	errFault := errors.New("the planted Remove fault")
	r.removeFault = func(AppID) error { return errFault }
	if err := r.Close(cctx); !errors.Is(err, errFault) || err.Error() != errFault.Error() {
		t.Errorf("Close: %v; want the Remove's error (the drain ended complete, so Close removes the app)", err)
	}
	if rs := r.appList()[id].rec.stats(); rs.Failed {
		t.Errorf("recording %+v; want it kept (the stop was written as the deadline passed)", rs)
	}
}

// ---- Install ----

// An add whose every shard fails keeps the first shard error it received, and its rollback replies with that one.
func TestAddKeepsTheFirstShardsError(t *testing.T) {
	t.Parallel()
	w := igBuild(t, 1, 2, nil)
	first, second := errors.New("the first shard's error"), errors.New("the second shard's error")
	wt := &installWait{app: w.a, reply: make(chan reply, 1)}
	w.rt.installAck(wt, outItem{err: first})
	w.rt.installAck(wt, outItem{err: second})
	rb, ok := w.rt.installs[w.rt.token]
	if !ok || !rb.rollback {
		t.Fatalf("no rollback queued after two failed acks: %+v", rb)
	}
	if !errors.Is(rb.err, first) {
		t.Fatalf("the rollback's error: %v; want the first ack's", rb.err)
	}
	w.rt.installAck(rb, outItem{token: w.rt.token})
	w.rt.installAck(rb, outItem{token: w.rt.token})
	select {
	case rep := <-wt.reply:
		if !errors.Is(rep.err, first) {
			t.Errorf("the add's reply error: %v; want the first ack's", rep.err)
		}
	default:
		t.Fatal("the rollback did not reply after its two acks")
	}
}

// Activate returns, and Close after it returns: Activate's tail releases the in-flight lock that its own deferred
// leave and Close's stop take (R-S3). Each runs on its own goroutine, so a lock never released fails this test at its
// own deadline.
func TestCloseReturnsAfterAnActivation(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	id := ldAdd(t, ctx, r)
	act := make(chan error, 1)
	go func() { act <- r.Activate(ctx, id) }()
	if err := ldErr(t, ctx, act, "Activate"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close(ctx) }()
	if err := ldErr(t, ctx, closed, "Close after an Activate"); err != nil {
		t.Errorf("Close: %v; want nil", err)
	}
}

// A Remove whose app is removed after its lookup (the removeFault seam runs between appOf and the phase check, and
// removes the app there) is ErrRemoved: the deterministic form of TestRemoveRacingRemove's window.
func TestRemoveOfAnAppRemovedAfterItsLookupIsErrRemoved(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldAdd(t, ctx, r)
	var inner error
	r.removeFault = func(app AppID) error { r.removeFault = nil; inner = r.Remove(ctx, app); return nil }
	if err := r.Remove(ctx, id); !errors.Is(err, ErrRemoved) {
		t.Errorf("Remove of an app removed after its lookup: %v; want ErrRemoved", err)
	}
	if inner != nil {
		t.Errorf("the inner Remove: %v; want nil", inner)
	}
}

// Activate once Close has begun — the in-flight gate closing while an Add is inside, the runtime not yet stopped — is
// ErrClosed, refused by the gate alone (S3.3-1-2). Activate runs on its own goroutine: a refusal the gate misses
// leaves it waiting on the held router, and this test fails at its own deadline.
func TestActivateIsErrClosedOnceCloseHasBegun(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	id := ldAdd(t, ctx, r)
	hold := make(chan struct{})
	let := sync.OnceFunc(func() { close(hold) })
	r.addHold = func() { <-hold }
	added := make(chan error, 1)
	app := cgApp(t, func(event.Event) error { return nil })
	go func() { _, err := r.Add(ctx, app); added <- err }()
	if !cgUntil(ctx, func() bool { _, n := r.inflight.cgRead(); return n == 1 }) {
		t.Fatal("the Add never entered the in-flight gate: the ceiling")
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close(ctx) }()
	t.Cleanup(func() {
		let()
		if err := ldErr(t, cgCtx(t), closed, "Close"); err != nil {
			t.Errorf("Close: %v; want nil once the Add is released", err)
		}
	})
	if !cgUntil(ctx, func() bool { c, _ := r.inflight.cgRead(); return c }) {
		t.Fatal("Close never closed the in-flight gate: the ceiling")
	}
	act := make(chan error, 1)
	go func() { act <- r.Activate(ctx, id) }()
	if err := ldErr(t, ctx, act, "Activate"); !errors.Is(err, ErrClosed) {
		t.Errorf("Activate once Close has begun: %v; want ErrClosed", err)
	}
	if r.isClosed() {
		t.Errorf("the runtime is stopped; want Close still waiting on the Add (the gate alone refused)")
	}
	let()
	if err := ldErr(t, ctx, added, "the held Add"); err != nil && !errors.Is(err, ErrClosed) {
		t.Errorf("the held Add: %v; want nil or ErrClosed", err)
	}
}

// A repeated Drain after one that gave up on a stuck run reports what the run returned meanwhile (Run's doc: a run's
// return value is reported by Drain; awaitRuns' repeat reads the collected returns at once): its error, as it is.
func TestARepeatedDrainReportsTheErrorOfARunThatReturnedMeanwhile(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	release := make(chan struct{})
	errRun := errors.New("the run's own error")
	run := func(src context.Context) error {
		<-src.Done()
		cancel() // the first Drain's deadline passes with this run still running
		<-release
		return errRun
	}
	if err := r.Run(ctx, id, run); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := r.Drain(dctx, id); err == nil || !strings.Contains(err.Error(), "did not return before the deadline") {
		t.Fatalf("the first Drain: %v; want the stuck-runs error", err)
	}
	close(release)
	ldWait(t, ctx, r.appList()[id].runs[0].done, "the released run never returned")
	res, err := r.Drain(ctx, id)
	if err != nil {
		t.Fatalf("the repeated Drain: %v", err)
	}
	if len(res.Runs) != 1 || !errors.Is(res.Runs[0], errRun) {
		t.Errorf("the repeated Drain's Runs %v; want [%v]", res.Runs, errRun)
	}
}
