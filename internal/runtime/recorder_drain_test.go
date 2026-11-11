package runtime

// The recorder-drain fix (owner, 2026-11-11: the flake in TestARecordingThatOverflowsStaysReadable is fixed at its
// root). Internal: the gate on the Recorder opens only once Drain waits for the recording, through the recordingWait
// seam, which a test outside the package cannot set. Prefix: rd.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// rdGated is a Recorder whose every call writes one line after the gate opens; the first call closes entered. It
// counts the lines written and the line count at its last Flush.
type rdGated struct {
	entered chan struct{}
	gate    chan struct{}
	enter   func() // closes entered, once
	let     func() // opens the gate, once

	mu        sync.Mutex
	lines     int
	flushedAt int // lines at the last Flush; -1: never flushed
}

func newRDGated() *rdGated {
	w := &rdGated{entered: make(chan struct{}), gate: make(chan struct{}), flushedAt: -1}
	w.enter = sync.OnceFunc(func() { close(w.entered) })
	w.let = sync.OnceFunc(func() { close(w.gate) })
	return w
}

func (w *rdGated) line() error {
	w.enter()
	<-w.gate
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines++
	return nil
}

func (w *rdGated) Header(int64) error                            { return w.line() }
func (w *rdGated) Event(*event.Event, int64, []byte) error       { return w.line() }
func (w *rdGated) Rejected(int, int64, *event.DecodeError) error { return w.line() }
func (w *rdGated) Tick(int64) error                              { return w.line() }
func (w *rdGated) Table(int, TableOp, [][]event.Value) error     { return w.line() }
func (w *rdGated) Stop() error                                   { return w.line() }
func (w *rdGated) BatchEnd(int, string) error                    { return w.line() }

func (w *rdGated) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushedAt = w.lines
	return nil
}

// written is the lines written and the line count at the last Flush.
func (w *rdGated) written() (lines, flushedAt int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lines, w.flushedAt
}

// TestDrainWaitsForAStoppedRecordingsQueuedRecords (owner D2 of S3.2b-2's design; review C3; the S3.2b-2 fix round's
// TestARecordingThatOverflowsStaysReadable, flaky under load): when the queue fills, the recording ends after the
// records already queued; the recorder still writes every one of them and flushes, and Drain returns only once it
// has, so Stats after Drain counts them. The timing is forced: the Recorder is held inside its first write while the
// router fills the queue past its 64 records, and the gate opens only once Drain waits for the recording (the seam)
// or, with a Drain that does not wait, after the Stats cut — then Records reads 0 against the 65 lines the recorder
// writes before Close returns.
func TestDrainWaitsForAStoppedRecordingsQueuedRecords(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	w, r, id := rdOverflowed(t, ctx)
	waiting := make(chan struct{})
	r.recordingWait = sync.OnceFunc(func() { close(waiting) }) // Close drains again: the seam fires once
	go func() {
		select {
		case <-waiting: // Drain waits for the recording: let it write
		case <-ctx.Done():
		}
		w.let()
	}()
	if _, err := r.Drain(ctx, id); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	w.let() // a Drain that returned without waiting left the recorder held: Close still needs it to finish
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	lines, flushedAt := w.written()
	rs := st.Recording[id]
	if !rs.Failed || !strings.Contains(rs.Err, "queue full") || lines != 65 || rs.Records != int64(lines) || flushedAt != lines {
		t.Fatalf("Recording %+v after Drain; the recorder wrote %d lines, flushed after line %d; want Failed (queue full), 65 lines = Records, flushed after the last",
			rs, lines, flushedAt)
	}
}

// TestDrainMarksAStoppedRecordingIncompleteAtItsDeadline (owner, 2026-11-11, relayed by the coordinator: at the
// drain's deadline a stopped recording still flushing is marked incomplete like a live one; the drain succeeds): the
// recording stopped on a full queue and the writer is never let through before the Stats cut; Drain with its own
// deadline returns nil; Stats then reads Failed, an error naming the full queue and the deadline's cut, and Records =
// the lines written at the cut, none. Close still waits for the recorder, let through at the end.
func TestDrainMarksAStoppedRecordingIncompleteAtItsDeadline(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	w, r, id := rdOverflowed(t, ctx)
	dctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond) // the drain's deadline; the test waits for Drain's return
	defer cancel()
	if _, err := r.Drain(dctx, id); err != nil {
		t.Fatalf("Drain: %v; want success at its deadline", err)
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lines, _ := w.written()
	w.let()
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rs := st.Recording[id]
	if !rs.Failed || !strings.Contains(rs.Err, "queue full") || !strings.Contains(rs.Err, "deadline") || !strings.Contains(rs.Err, "incomplete") || lines != 0 || rs.Records != int64(lines) {
		t.Fatalf("Recording %+v after Drain at its deadline; the recorder had written %d lines; want Failed, an error naming the full queue and the deadline's cut (incomplete), Records = 0 lines",
			rs, lines)
	}
}

// rdOverflowed is a one-shard runtime with one app recording on w through a queue of 64, activated, its recorder
// held inside its first write (the header) so it writes nothing until the gate opens, and the queue filled past its
// 64 records so the recording has stopped — confirmed through Stats (the positive witness).
func rdOverflowed(t *testing.T, ctx context.Context) (*rdGated, *Runtime, AppID) {
	t.Helper()
	w := newRDGated()
	r, err := New(Config{Shards: 1, RecordQueue: 64, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	app := cgApp(t, func(event.Event) error { return nil })
	app.Recorder = w
	id, err := r.Add(ctx, app)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	select {
	case <-w.entered:
	case <-ctx.Done():
		t.Fatal("the recorder never reached its first write: the ceiling")
	}
	for i := range 2 * 64 { // 64 fill the queue; the 65th ends the recording
		if err := r.Send(ctx, id, cgEv(i)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rs := st.Recording[id]; !rs.Failed || !strings.Contains(rs.Err, "queue full") {
		t.Fatalf("Recording %+v before the drain; want Failed: queue full", rs)
	}
	return w, r, id
}
