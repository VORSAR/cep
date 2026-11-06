package runtime_test

// S3.2b-2's fix round (owner decisions of 2026-11-06, "S3.2b-2's review and break-it"; review C4; break-it S2).

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

// TestDrainWaitsForTheRecordingOnlyUntilItsDeadline (owner, 2026-11-06, decision 1): with a recorder stuck in a write,
// the drain still delivers every alert and returns success at its deadline, the recording marked failed.
func TestDrainWaitsForTheRecordingOnlyUntilItsDeadline(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	d := &rrDouble{hold: make(chan struct{})}
	var outs atomic.Int64
	r := rtStart(t, runtime.Config{Shards: 1}, rrApp(t, d, func(event.Event) error { outs.Add(1); return nil }))
	t.Cleanup(func() { close(d.hold) }) // runs before rtStart's Close: the stuck write returns
	for i := range 3 {
		rtSend(t, ctx, r, 0, rtEv(0, rtAt(float64(1+i)), rtStr("a")))
	}
	dctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if _, err := r.Drain(dctx, 0); err != nil {
		t.Fatalf("Drain: %v; want success: every alert went out", err)
	}
	if outs.Load() != 3 {
		t.Errorf("%d outputs; want 3", outs.Load())
	}
	if rs := rtStats(t, ctx, r).Recording[0]; !rs.Failed || !strings.Contains(rs.Err, "deadline") {
		t.Errorf("Recording %+v; want Failed: the stop not written by the drain's deadline", rs)
	}
}

// TestTickRefusesATimeARecordingCannotWrite (break-it S2; owner D1 of S3.2b-2's design): a tick is an entry point too;
// a time outside the years 0000-9999 is refused with an error to the caller.
func TestTickRefusesATimeARecordingCannotWrite(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := rtStart(t, runtime.Config{Shards: 1}, twApp(t, 0, func(event.Event) error { return nil }))
	for _, at := range []int64{253402300800000, -62167219200001} {
		if err := r.Tick(ctx, at); err == nil {
			t.Errorf("Tick(%d) accepted; want an error", at)
		}
	}
	if err := r.Tick(context.Background(), rtAt(5)); err != nil {
		t.Errorf("Tick at an ordinary time: %v", err)
	}
}
