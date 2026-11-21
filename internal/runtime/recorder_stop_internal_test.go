package runtime

// The recorder goroutine's ends (recorder.go's run): at Close's stop it writes what is queued, flushes and returns; at
// a full queue it writes what was queued before it, flushes and settles, and stays live until its stop. Internal: the
// goroutine is observed through its queue's channels and its Recorder. Prefix: ld (lifecycle_deadlines' helpers).

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// ldRecorderIdle says the recorder goroutine a lifecycle call on goroutine creator started is parked in its select.
func ldRecorderIdle(creator string) bool {
	for _, s := range rsStacks() {
		if strings.Contains(s, "internal/runtime.(*recQ).run(") &&
			strings.Contains(s, "created by github.com/vorsar/cep/internal/runtime.(*Runtime).") &&
			strings.Contains(s, " in goroutine "+creator+"\n") {
			return strings.Contains(strings.SplitN(s, "\n", 2)[0], "[select")
		}
	}
	return false
}

// Close that gives up stops an idle recorder: the goroutine flushes its Recorder once and returns, with no Stop written
// (the app was never drained).
func TestCloseStopsAnIdleRecorderItGaveUpOn(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	self := rsSelfID()
	r := ldRuntime(t)
	h := newHeldFlushRecorder(nil)
	h.let() // Flush returns at once
	app := cgApp(t, func(event.Event) error { return nil })
	app.Sinks = nil
	app.Recorder = h
	id := ldStarted(t, ctx, r, app)
	if !cgUntil(ctx, func() bool { n, _, _ := h.counts(); return n == 1 && ldRecorderIdle(self) }) {
		t.Fatal("the recorder never wrote its header and went idle: the ceiling")
	}
	q := r.appList()[id].rec
	if err := r.Close(ldExpired()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close with an expired ctx: %v; want context.Canceled", err)
	}
	ldClose(t, r)
	ldWait(t, ctx, q.returned, "the recorder goroutine never returned")
	if n, s, f := h.counts(); n != 1 || s != 0 || f != 1 {
		t.Errorf("records %d, stops %d, flushes %d; want 1, 0, 1 (the header, no stop, the stop's flush)", n, s, f)
	}
}

// A full queue ends the recording: the goroutine writes every record queued before it (the header and 64 events),
// flushes, then settles, and stays live (it returns only at its stop); a drain after it is not cut.
func TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles(t *testing.T) {
	t.Parallel()
	self := rsSelfID()
	ctx := cgCtx(t)
	w, r, id := rdOverflowed(t, ctx)
	t.Cleanup(func() { w.let(); ldClose(t, r) })
	q := r.appList()[id].rec
	w.let()
	ldWait(t, ctx, q.settled, "the overflowed recording never settled")
	if lines, flushedAt := w.written(); lines != 65 || flushedAt != 65 {
		t.Errorf("lines %d, flushed at %d when settled; want 65 and 65 (the header and the 64 queued events)", lines,
			flushedAt)
	}
	select {
	case <-q.returned:
		t.Error("the recorder goroutine returned at the overflow; want it live until its stop")
	default:
	}
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cgUntil(ctx, func() bool { return ldRecorderIdle(self) }) {
		t.Error("the recorder goroutine is not parked in its select after the drain; want it live until its stop")
	}
	if rs := st.Recording[id]; rs.Records != 65 || strings.Contains(rs.Err, "incomplete") {
		t.Errorf("Recording %+v after the drain; want 65 records and no cut", rs)
	}
}
