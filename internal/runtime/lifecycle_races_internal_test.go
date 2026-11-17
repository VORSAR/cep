package runtime

// The lifecycle's scheduling-only branches (S1.1c-6 design §4.3): each is reachable only when two goroutines
// interleave in one window, so each test repeats the race ldRounds times and asserts what every interleaving must
// give. Run at -cpu 1,4 under -race. Prefix: ld.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// ldRounds is each race's repetitions within one test.
const ldRounds = 64

// ldRace runs other on its own goroutine and mine on the test's, and returns other's error.
func ldRace(t *testing.T, ctx context.Context, other, mine func() error) (otherErr, mineErr error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- other() }()
	mineErr = mine()
	return ldErr(t, ctx, done, "the racing call"), mineErr
}

// ldAdd adds a cgApp with a no-op sink.
func ldAdd(t *testing.T, ctx context.Context, r *Runtime) AppID {
	t.Helper()
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return id
}

// Activate racing Remove of an inactive app: the removal refuses an active app (ErrActive at the caller, or
// ErrDrainIncomplete from the router once it activated the app first), Activate refuses a removed one, and never both
// refuse (a Remove storing phaseRemoved between appOf's load and Activate's own is ErrRemoved).
func TestActivateRacingRemoveOfAnInactiveApp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range ldRounds {
		id := ldAdd(t, ctx, r)
		rmErr, actErr := ldRace(t, ctx, func() error { return r.Remove(ctx, id) }, func() error { return r.Activate(ctx, id) })
		if actErr != nil && !errors.Is(actErr, ErrRemoved) {
			t.Fatalf("Activate racing Remove: %v; want nil or ErrRemoved", actErr)
		}
		if rmErr != nil && !errors.Is(rmErr, ErrActive) && !errors.Is(rmErr, ErrDrainIncomplete) {
			t.Fatalf("Remove racing Activate: %v; want nil, ErrActive or ErrDrainIncomplete", rmErr)
		}
		if actErr != nil && rmErr != nil {
			t.Fatalf("both refused: Activate %v, Remove %v", actErr, rmErr)
		}
	}
}

// Drain racing Remove of an inactive app: the Remove always succeeds; the Drain succeeds or is ErrRemoved.
func TestDrainRacingRemoveOfAnInactiveApp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range ldRounds {
		id := ldAdd(t, ctx, r)
		rmErr, drErr := ldRace(t, ctx, func() error { return r.Remove(ctx, id) },
			func() error { _, err := r.Drain(ctx, id); return err })
		if rmErr != nil {
			t.Fatalf("Remove racing Drain: %v; want nil", rmErr)
		}
		if drErr != nil && !errors.Is(drErr, ErrRemoved) {
			t.Fatalf("Drain racing Remove: %v; want nil or ErrRemoved", drErr)
		}
	}
}

// Two Removes of one app: exactly one removes it, the other is ErrRemoved.
func TestRemoveRacingRemove(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range ldRounds {
		id := ldAdd(t, ctx, r)
		e1, e2 := ldRace(t, ctx, func() error { return r.Remove(ctx, id) }, func() error { return r.Remove(ctx, id) })
		if (e1 == nil) == (e2 == nil) || !errors.Is(errors.Join(e1, e2), ErrRemoved) {
			t.Fatalf("two Removes: %v and %v; want one nil and one ErrRemoved", e1, e2)
		}
	}
}

// A Drain whose deadline passes as the recording's stop is written keeps the recording: the marker is closed before
// Drain's wait, so whichever ready arm the select takes, the recording is not cut.
func TestDrainAtItsDeadlineKeepsARecordingWrittenAsItPassed(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	t.Cleanup(func() { r.recordingWait = nil; ldClose(t, r) })
	for range ldRounds {
		h := newHeldFlushRecorder(nil)
		h.let()
		app := cgApp(t, func(event.Event) error { return nil })
		app.Sinks = nil
		app.Recorder = h
		id := ldStarted(t, ctx, r, app)
		dctx, cancel := context.WithCancel(ctx)
		r.recordingWait = func() {
			cancel()
			cgUntil(ctx, func() bool { _, s, _ := h.counts(); return s == 1 && ldRecorderIdle(self) })
		}
		_, err := r.Drain(dctx, id)
		cancel()
		r.recordingWait = nil
		if err != nil {
			t.Fatalf("Drain at its deadline with the stop written: %v; want nil", err)
		}
		if rs := r.appList()[id].rec.stats(); rs.Failed {
			t.Fatalf("recording %+v; want it kept (the stop was written as the deadline passed)", rs)
		}
		if err := r.Remove(ctx, id); err != nil {
			t.Fatalf("Remove: %v", err)
		}
	}
}

// Close past its deadline returns ctx.Err() whichever ready arm its last wait takes: the goroutines all returned (the
// app's OnRemoved waits for the runtime's own to exit) or the deadline.
func TestCloseAfterItsDeadlineReturnsTheDeadline(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	for range ldRounds {
		r := ldRuntime(t)
		app := cgApp(t, func(event.Event) error { return nil })
		app.OnRemoved = func() { cgUntil(ctx, func() bool { return rsRouters(self) == 0 }) }
		if _, err := r.Add(ctx, app); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := r.Close(ldExpired()); !errors.Is(err, context.Canceled) {
			t.Fatalf("Close with an expired ctx: %v; want context.Canceled", err)
		}
		ldClose(t, r)
	}
}

// Remove's stop reaches the recorder with records still queued: the goroutine writes every one, then flushes, so the
// file holds every record queued before the stop (the drain before it was cut at its deadline).
func TestRemoveWritesEveryRecordQueuedBeforeItsStop(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { r.recordingWait = nil; ldClose(t, r) })
	for range ldRounds {
		w := newRDGated()
		app := cgApp(t, func(event.Event) error { return nil })
		app.Recorder = w
		app.OnRemoved = w.let // the gate opens once the stop is closed: the queue and the stop both ready
		id := ldStarted(t, ctx, r, app)
		ldWait(t, ctx, w.entered, "the recorder never reached its header")
		for i := range 4 {
			if err := r.Send(ctx, id, cgEv(i)); err != nil {
				t.Fatalf("Send %d: %v", i, err)
			}
		}
		dctx, cancel := context.WithCancel(ctx)
		r.recordingWait = cancel
		_, err := r.Drain(dctx, id)
		cancel()
		r.recordingWait = nil
		if err != nil {
			t.Fatalf("Drain cut at its deadline: %v; want nil", err)
		}
		if rs := r.appList()[id].rec.stats(); !rs.Failed || !strings.Contains(rs.Err, "incomplete") {
			t.Fatalf("recording %+v after the cut drain; want it marked incomplete", rs)
		}
		if err := r.Remove(ctx, id); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if lines, flushedAt := w.written(); lines != 6 || flushedAt != 6 {
			t.Fatalf("lines %d, flushed at %d; want 6 and 6 (the header, 4 events, the stop)", lines, flushedAt)
		}
	}
}

// Close that gives up stops the recorder with records still queued: the goroutine writes every one, then flushes.
func TestCloseWritesTheRecordsQueuedWhenItGaveUp(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	for range ldRounds {
		r := ldRuntime(t)
		w := newRDGated()
		app := cgApp(t, func(event.Event) error { return nil })
		app.Recorder = w
		app.OnRemoved = w.let // the gate opens once the runtime is stopped: the queue and the stop both ready
		id := ldStarted(t, ctx, r, app)
		ldWait(t, ctx, w.entered, "the recorder never reached its header")
		for i := range 4 {
			if err := r.Send(ctx, id, cgEv(i)); err != nil {
				t.Fatalf("Send %d: %v", i, err)
			}
		}
		if _, err := r.Stats(ctx); err != nil { // behind the sends in intake order: their records are queued
			t.Fatal(err)
		}
		if err := r.Close(ldExpired()); !errors.Is(err, context.Canceled) {
			t.Fatalf("Close with an expired ctx: %v; want context.Canceled", err)
		}
		ldClose(t, r)
		if lines, flushedAt := w.written(); lines != 5 || flushedAt != 5 {
			t.Fatalf("lines %d, flushed at %d; want 5 and 5 (the header and 4 events)", lines, flushedAt)
		}
	}
}
