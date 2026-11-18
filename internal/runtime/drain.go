package runtime

import (
	"context"
	"errors"
	"fmt"
)

// Drain runs the drain barrier for app (design §5; s3-3-1 §2.4): the app's input ends (owner F5); every output
// produced before the barrier has been through its sink when it returns. For an active app the ticks stop first
// (S3.1-1's D3), then the source context is cancelled and every Run goroutine awaited until ctx's deadline; a run
// that did not return makes Drain return the stuck count wrapped in ctx.Err() without the barrier (the input is
// refused from here; a repeated Drain runs the barrier without waiting again). A never-activated app is drained at
// once: nothing was delivered, nothing is recorded. A second Drain of a drained app returns the same result.
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
	a, err := r.appOf(app)
	if err != nil {
		return DrainResult{}, err
	}
	if appPhase(a.phase.Load()) == phaseRemoved {
		return DrainResult{}, ErrRemoved
	}
	// was is the phase this Drain moved from — taken from the CAS that succeeded, so a revert never restores a phase
	// another call changed meanwhile (review B1); phaseDraining: this Drain moved nothing, so it reverts nothing.
	was := phaseDraining
	switch {
	case a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining)):
		was = phaseInactive
	case a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining)):
		was = phaseActive
	}
	revert := func() { // nothing was queued: the app is not drained (QA Q15; break-it R1)
		if was != phaseDraining {
			a.phase.CompareAndSwap(int32(phaseDraining), int32(was))
		}
	}
	var runs []error
	if a.started.Load() {
		runs, err = r.awaitRuns(ctx, a)
		if err != nil {
			if !a.waited { // the quiet item was never queued: live, as before this Drain
				revert()
			}
			return DrainResult{Runs: runs}, err
		}
	}
	set := !a.draining.Swap(true)
	it := item{kind: iDrain, app: app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		if set { // nothing was queued, and no earlier Drain set the flag: the app is not drained (QA Q15; break-it R1)
			a.draining.Store(false)
			if !a.waited {
				revert()
			}
		}
		return DrainResult{Runs: runs}, err
	}
	rep, err := r.await(ctx, it.reply)
	if err != nil {
		return DrainResult{Runs: runs}, err
	}
	for _, m := range rep.markers {
		select {
		case <-m:
		case <-r.stopped:
			return DrainResult{Runs: runs}, ErrClosed
		case <-ctx.Done():
			return DrainResult{Runs: runs}, ctx.Err()
		}
	}
	// every alert is out: the recording is waited for only until the deadline (owner, 2026-11-06)
	if rep.recMarker != nil {
		if r.recordingWait != nil { // the package's tests only
			r.recordingWait()
		}
		select {
		case <-rep.recMarker:
		case <-r.stopped:
		case <-ctx.Done():
			select {
			case <-rep.recMarker: // written as the deadline passed: complete
			default: // a live recording's stop, or a stopped recording's queued records, still unwritten (owner, 2026-11-11)
				a.rec.cut()
			}
		}
	}
	return DrainResult{Pending: rep.pending, Runs: runs}, nil
}

// awaitRuns quiets the ticks, cancels the source context and waits for every Run goroutine, once per app; a repeated
// Drain gets the collected returns at once.
func (r *Runtime) awaitRuns(ctx context.Context, a *appState) ([]error, error) {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	again := a.waited
	if !a.waited {
		if err := r.enqueue(ctx, item{kind: iQuiet, app: a.id}); err != nil {
			return nil, err // nothing queued: Drain leaves the app live (break-it R1)
		}
		a.waited = true
		a.cancelSrc()
	}
	runs := make([]error, len(a.runs))
	stuck := 0
	for i, rs := range a.runs {
		if again { // a repeated Drain runs the barrier without waiting for the runs again (design s3-3-1 §2.4)
			select {
			case <-rs.done:
				if !errors.Is(rs.err, context.Canceled) {
					runs[i] = rs.err
				}
			default:
			}
			continue
		}
		select {
		case <-rs.done:
			if !errors.Is(rs.err, context.Canceled) {
				runs[i] = rs.err
			}
		case <-ctx.Done():
			stuck++
		}
	}
	if stuck > 0 {
		return runs, fmt.Errorf("runtime: app %d: %d source goroutine(s) did not return before the deadline: %w", a.id, stuck,
			ctx.Err())
	}
	return runs, nil
}
