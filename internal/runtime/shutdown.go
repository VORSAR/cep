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
	a.removed.Do(func() {
		if a.onRemoved != nil {
			a.onRemoved()
		}
	})
	for _, m := range rep.markers {
		select {
		case <-m:
		case <-r.stopped:
			return ErrClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if rep.recMarker != nil {
		select {
		case <-rep.recMarker:
		case <-r.stopped:
			return ErrClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Close drains every active app and removes every app in reverse id order, then stops the router, the shards and
// the ticker. Past ctx's deadline it signals every goroutine and returns ctx.Err(); a Write or a run that never
// returns keeps its goroutine. A second Close waits, until its own ctx's deadline, for the goroutines the first left
// running: nil once they have all returned, ctx's error otherwise (break-it S2: the truthful contract).
func (r *Runtime) Close(ctx context.Context) error {
	var first error
	var failed []error
	r.closeOnce.Do(func() {
		// The in-flight gate first (S3.3-1-2; design s3-3-2 §9.1): no Add or Activate begins from here, and the ones
		// inside finish before the apps are listed. Waited on with ctx: the router may be waiting on a stuck sink.
		select {
		case <-r.inflight.close():
		case <-ctx.Done():
			first = ctx.Err()
		}
		apps := r.appList()
		for id := len(apps) - 1; id >= 0 && first == nil; id-- {
			a := apps[id]
			if a == nil || appPhase(a.phase.Load()) == phaseRemoved {
				continue
			}
			if a.started.Load() { // activated by the router, whatever the caller-side phase reads
				if _, err := r.Drain(ctx, AppID(id)); err != nil && ctx.Err() != nil {
					first = err
					break
				}
			}
			if err := r.Remove(ctx, AppID(id)); err != nil {
				switch {
				case ctx.Err() != nil:
					first = err
				case !errors.Is(err, ErrRemoved): // an Undeploy got there first: nothing to report
					failed = append(failed, fmt.Errorf("runtime: Close: app %d: %w", id, err))
				}
			}
		}
		r.inflight.mu.Lock() // excludes Activate's and Run's goroutine starts (R-S3)
		close(r.stopped)
		r.inflight.mu.Unlock()
		for _, a := range r.appList() {
			if a != nil {
				a.cancelSrc()
				a.removed.Do(func() {
					if a.onRemoved != nil {
						a.onRemoved()
					}
				})
			}
		}
	})
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
