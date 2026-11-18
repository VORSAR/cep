package runtime

import (
	"context"
	"errors"
	"fmt"
)

// Remove unregisters a drained or inactive app: the router drops its routes, every shard frees its state, each sink
// flusher and the recorder goroutine return after their last marker (the queues are never closed), the slot is nil.
// Removing an active app is an error; removing twice is ErrRemoved.
func (r *Runtime) Remove(ctx context.Context, app AppID) error {
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if f := r.removeFault; f != nil { // the package's tests only
		if err := f(app); err != nil {
			return err
		}
	}
	switch appPhase(a.phase.Load()) {
	case phaseActive:
		return fmt.Errorf("%w (app %d)", ErrActive, app)
	case phaseRemoved:
		return ErrRemoved
	}
	rep, err := r.requestQueued(ctx, item{kind: iRemove, app: app}) // the router refuses a second removal (break-it B1)
	if err != nil {
		return err
	}
	a.phase.Store(int32(phaseRemoved))
	a.cancelSrc()
	// The sinks' context ends here, before the flushers are awaited (design s3-3-1 §3.6 as built): the app is being
	// discarded, so a Write still blocked on its context returns, the last marker passes, and the flusher returns.
	a.notifyRemoved()
	if err := r.awaitMarkers(ctx, rep.markers); err != nil {
		return err
	}
	if rep.recMarker != nil {
		return r.awaitSignal(ctx, rep.recMarker)
	}
	return nil
}

// notifyRemoved runs the app's OnRemoved at most once, through the sync.Once Remove and Close share.
func (a *appState) notifyRemoved() {
	a.removed.Do(func() {
		if a.onRemoved != nil {
			a.onRemoved()
		}
	})
}

// Close drains every active app and removes every app in reverse id order, then stops the router, the shards and
// the ticker. Past ctx's deadline it signals every goroutine and returns ctx.Err(); a Write or a run that never
// returns keeps its goroutine. A second Close waits, until its own ctx's deadline, for the goroutines the first left
// running: nil once they have all returned, ctx's error otherwise (break-it S2: the truthful contract).
func (r *Runtime) Close(ctx context.Context) error {
	var first error
	var failed []error
	r.closeOnce.Do(func() { first, failed = r.shutDown(ctx) })
	return r.awaitGoroutines(ctx, first, failed)
}

// shutDown is the first Close's work: the in-flight gate, every app closed in reverse id order unless the gate's wait
// met ctx's deadline, then the runtime stopped. first is the ctx error that ended it early, failed the removals that
// failed without one.
func (r *Runtime) shutDown(ctx context.Context) (first error, failed []error) {
	// The in-flight gate first (S3.3-1-2; design s3-3-2 §9.1): no Add or Activate begins from here, and the ones
	// inside finish before the apps are listed. Waited on with ctx: the router may be waiting on a stuck sink.
	select {
	case <-r.inflight.close():
	case <-ctx.Done():
		first = ctx.Err()
	}
	if first == nil {
		first, failed = r.closeApps(ctx)
	}
	r.stopRuntime()
	return first, failed
}

// closeApps drains every started app and removes every app not yet removed, in reverse id order, until the first ctx
// error, which it returns with the removals that failed without one.
func (r *Runtime) closeApps(ctx context.Context) (first error, failed []error) {
	apps := r.appList()
	for id := len(apps) - 1; id >= 0 && first == nil; id-- {
		a := apps[id]
		if a == nil || appPhase(a.phase.Load()) == phaseRemoved {
			continue
		}
		stop, fail := r.closeApp(ctx, id, a)
		first = stop
		if fail != nil {
			failed = append(failed, fail)
		}
	}
	return first, failed
}

// closeApp drains app id if the router started it, then removes it: stop is the error that ends Close's loop (a
// failure at ctx's deadline), failed a removal that failed without one; nil, nil for an Undeploy that got there first.
func (r *Runtime) closeApp(ctx context.Context, id int, a *appState) (stop, failed error) {
	if a.started.Load() { // activated by the router, whatever the caller-side phase reads
		if _, err := r.Drain(ctx, AppID(id)); err != nil && ctx.Err() != nil {
			return err, nil
		}
	}
	if err := r.Remove(ctx, AppID(id)); err != nil {
		switch {
		case ctx.Err() != nil:
			return err, nil
		case !errors.Is(err, ErrRemoved): // an Undeploy got there first: nothing to report
			return nil, fmt.Errorf("runtime: Close: app %d: %w", id, err)
		}
	}
	return nil, nil
}

// stopRuntime closes r.stopped under inflight.mu, then ends every app's sink context and runs its OnRemoved if no
// Remove did.
func (r *Runtime) stopRuntime() {
	r.inflight.mu.Lock() // excludes Activate's and Run's goroutine starts (R-S3)
	close(r.stopped)
	r.inflight.mu.Unlock()
	for _, a := range r.appList() {
		if a != nil {
			a.cancelSrc()
			a.notifyRemoved()
		}
	}
}

// awaitGoroutines waits, until ctx's deadline, for every goroutine the runtime started: first if a Close met its
// deadline, else the failed removals joined once they have all returned, ctx's error if they have not by then.
func (r *Runtime) awaitGoroutines(ctx context.Context, first error, failed []error) error {
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		if first == nil {
			return errors.Join(failed...) // a Remove that failed without ctx is reported, never dropped (A3)
		}
		return first
	case <-ctx.Done():
		if first != nil {
			return first
		}
		return ctx.Err()
	}
}
