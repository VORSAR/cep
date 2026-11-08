package runtime_test

// QA's break-it on S3.3-1's combined build (step 7): the per-app lifecycle's entry points called at once.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/runtime"
)

// TestBreakitTwoRemovesAtOnce (design s3-3-1 §2.4: "removing twice is ErrRemoved"): two Remove calls on one drained
// app, at once, both return before a short deadline, one nil and the other nil or ErrRemoved. Today router.remove
// refuses a second remove only once installAck has cleared a.ing, so both pass; the second installAck pushes a second
// set of last markers into queues whose flushers already returned on the first, and its Remove waits on them until
// its ctx ends (lifecycle.go:268).
func TestBreakitTwoRemovesAtOnce(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 4})
	for i := range 40 {
		id := lcStarted(t, ctx, r, lcApp(t, lcCount(&n)))
		rtDrain(t, ctx, r, id)
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for j := range errs {
			wg.Add(1)
			go func() { defer wg.Done(); errs[j] = r.Remove(rctx, id) }()
		}
		wg.Wait()
		cancel()
		for j, err := range errs {
			if err != nil && !errors.Is(err, runtime.ErrRemoved) {
				t.Fatalf("iteration %d: Remove %d of two at once: %v; want nil or ErrRemoved", i, j, err)
			}
		}
	}
}

// TestBreakitActivateRacesRemove (run under -race, as T4 does): Activate and Remove of one inactive app at once.
// Activate's phase CAS (lifecycle.go:65) can win after router.remove read a.started false and before installAck
// stores phaseRemoved; installAck then clears a.rec and a.sinks (lifecycle.go:414) while Activate reads a.rec and
// starts the flushers (lifecycle.go:78-84): a data race, flushers started for a removed app with no last marker, and,
// when a.rec is read non-nil then nil, a nil recorder's goroutine. Want: Activate either wins (the app is active and
// Remove refuses it with ErrActive) or loses (ErrRemoved), never both.
func TestBreakitActivateRacesRemove(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	var n atomic.Int64
	r := lcNew(t, runtime.Config{Shards: 4})
	for i := range 60 {
		app := lcApp(t, lcCount(&n))
		app.Recorder = &lcRec{}
		id := lcAdd(t, ctx, r, app)
		var aerr, rerr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); aerr = r.Activate(ctx, id) }()
		go func() { defer wg.Done(); rerr = r.Remove(ctx, id) }()
		wg.Wait()
		if aerr == nil && rerr == nil {
			t.Fatalf("iteration %d: Activate and Remove of one app both returned nil: the app was activated and removed at once", i)
		}
		if aerr == nil { // Activate won: the app is active; drain and remove it
			rtDrain(t, ctx, r, id)
			lcRemove(t, ctx, r, id)
		}
	}
}
