package runtime_test

// S3.3-1's fix round: the combined review's findings that QA's break-it did not already pin (docs/reviews/
// s3-3-1-combined-architecture.md B1, S1, S2, S4; the owner's decision of 2026-11-08, 1: an app's clock starts at
// Start). Prefix: fx.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/runtime"
)

// TestActivateRacesNeverStartedDrain (review B1): Activate and Drain of one inactive app with a Recorder, on two
// goroutines (the facade's Start and Stop at once). Either the router took the drain first — Activate is refused and
// the recorder is never called — or the activation first — the recording is whole (header … stop). A recording with
// a header and no stop, or a recorder goroutine Remove cannot stop, is the race the review found.
func TestActivateRacesNeverStartedDrain(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 1})
	for i := 0; i < 500; i++ {
		rec := &lcRec{}
		app := lcApp(t)
		app.Recorder = rec
		id := lcAdd(t, ctx, r, app)
		var wg sync.WaitGroup
		var actErr error
		wg.Add(2)
		go func() { defer wg.Done(); actErr = r.Activate(ctx, id) }()
		go func() { defer wg.Done(); _, _ = r.Drain(ctx, id) }()
		wg.Wait()
		if _, err := r.Drain(ctx, id); err != nil { // complete the drain whichever side won
			t.Fatalf("iteration %d: Drain after the race: %v", i, err)
		}
		lcRemove(t, ctx, r, id) // waits for the recorder goroutine, if one was started
		log := rec.log()
		switch {
		case actErr == nil && (len(log) == 0 || log[0] != "header" || log[len(log)-1] != "stop"):
			t.Fatalf("iteration %d: Activate succeeded and the recording is %v; want header … stop (a whole recording)", i, log)
		case actErr != nil && len(log) != 0:
			t.Fatalf("iteration %d: Activate refused (%v) and the recorder was called %v; want never", i, actErr, log)
		}
	}
}

// TestInactiveAppGetsNoTicks (review S1; owner, 2026-11-08, 1): ticks before Activate never reach an app, so its
// recording holds no tick from before its Start.
func TestInactiveAppGetsNoTicks(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 1})
	rec := &lcRec{}
	app := lcApp(t)
	app.Recorder = rec
	id := lcAdd(t, ctx, r, app)
	for i := 1; i <= 5; i++ {
		if err := r.Tick(ctx, rtAt(float64(i))); err != nil {
			t.Fatalf("Tick: %v", err)
		}
	}
	lcActivate(t, ctx, r, id)
	rtDrain(t, ctx, r, id)
	for _, k := range rec.log() {
		if k == "tick" {
			t.Fatalf("recording %v; want no tick: the app's clock starts at Activate", rec.log())
		}
	}
}

// TestAddWhoseContextEndsAfterQueuingReturnsItsID (review S2): once Add's item is queued, Add returns the router's
// answer whatever its context does, so an installed app always has an id its caller holds.
func TestAddWhoseContextEndsAfterQueuingReturnsItsID(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 1})
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	app := lcApp(t)
	app.Rules[0].NewBody = func(emit runtime.Emit) partition.Body {
		once.Do(func() { close(entered) })
		<-release // the shard installs the app only when the test lets it
		return &lcFwd{emit: emit}
	}
	addCtx, cancel := context.WithCancel(ctx)
	type result struct {
		id  runtime.AppID
		err error
	}
	done := make(chan result, 1)
	go func() { id, err := r.Add(addCtx, app); done <- result{id, err} }()
	rtWait(t, ctx, entered, "the shard to start installing the app")
	cancel() // the item is queued and being installed
	close(release)
	res := <-done
	if res.err != nil {
		t.Fatalf("Add whose context ended after queuing: %v; want the app's id (the router installed it)", res.err)
	}
	lcActivate(t, ctx, r, res.id)
	rtDrain(t, ctx, r, res.id)
	lcRemove(t, ctx, r, res.id)
}

// TestSendPayloadRacesRemove (review S4): a payload sent while its app is drained and removed is refused or
// accepted, never a race on the app's decoders (run under -race).
func TestSendPayloadRacesRemove(t *testing.T) {
	t.Parallel()
	ctx := rtCeiling(t)
	r := lcNew(t, runtime.Config{Shards: 2})
	for i := 0; i < 100; i++ {
		id := lcStarted(t, ctx, r, lcApp(t, func(event.Event) error { return nil }))
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				err := r.SendPayload(ctx, id, 0, []byte(`{"k":"a","ts":"2026-01-01T10:00:01Z"}`))
				if err != nil && !errors.Is(err, runtime.ErrDrained) && !errors.Is(err, runtime.ErrRemoved) {
					t.Errorf("SendPayload during Remove: %v; want nil, ErrDrained or ErrRemoved", err)
					return
				}
			}
		}()
		rtDrain(t, ctx, r, id)
		lcRemove(t, ctx, r, id)
		close(stop)
		wg.Wait()
	}
}
