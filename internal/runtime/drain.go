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
	was := a.markDraining()
	var runs []error
	if a.started.Load() {
		runs, err = r.awaitRuns(ctx, a)
		if err != nil {
			a.revertUnlessQuieted(was) // the quiet item was never queued: live, as before this Drain
			return DrainResult{Runs: runs}, err
		}
	}
	ch, err := r.queueDrain(ctx, a, app, was)
	if err != nil {
		return DrainResult{Runs: runs}, err
	}
	rep, err := r.await(ctx, ch)
	if err != nil {
		return DrainResult{Runs: runs}, err
	}
	if err := r.awaitMarkers(ctx, rep.markers); err != nil {
		return DrainResult{Runs: runs}, err
	}
	// every alert is out: the recording is waited for only until the deadline (owner, 2026-11-06)
	r.awaitRecordingStop(ctx, a, rep.recMarker)
	return DrainResult{Pending: rep.pending, Runs: runs}, nil
}

// markDraining moves the app to phaseDraining and returns the phase it moved from.
func (a *appState) markDraining() appPhase {
	// was is the phase this Drain moved from — taken from the CAS that succeeded, so a revert never restores a phase
	// another call changed meanwhile (review B1); phaseDraining: this Drain moved nothing, so it reverts nothing.
	was := phaseDraining
	switch {
	case a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining)):
		was = phaseInactive
	case a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining)):
		was = phaseActive
	}
	return was
}

// revertDrain moves the app back to was, the phase markDraining moved it from: nothing was queued, so the app is not
// drained (QA Q15; break-it R1).
func (a *appState) revertDrain(was appPhase) {
	if was != phaseDraining {
		a.phase.CompareAndSwap(int32(phaseDraining), int32(was))
	}
}

// revertUnlessQuieted moves the app back to was unless a Drain has queued the quiet item. It reads waited under
// runMu, which awaitRuns holds while it queues that item and sets waited, so the read and the revert never race it.
func (a *appState) revertUnlessQuieted(was appPhase) {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	if !a.waited {
		a.revertDrain(was)
	}
}

// queueDrain sets the drain flag, then queues the drain item and returns its reply channel. When nothing was queued
// and no earlier Drain set the flag, it clears the flag and, if the quiet item was never queued, reverts the phase.
func (r *Runtime) queueDrain(ctx context.Context, a *appState, app AppID, was appPhase) (chan reply, error) {
	set := !a.draining.Swap(true)
	it := item{kind: iDrain, app: app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		if set { // nothing was queued, and no earlier Drain set the flag: the app is not drained (QA Q15; break-it R1)
			a.draining.Store(false)
			a.revertUnlessQuieted(was)
		}
		return nil, err
	}
	return it.reply, nil
}

// awaitRecordingStop waits for the recording's marker m, if there is one, until ctx's deadline: a marker still open
// at the deadline cuts the recording.
func (r *Runtime) awaitRecordingStop(ctx context.Context, a *appState, m chan struct{}) {
	if m != nil {
		if r.recordingWait != nil { // the package's tests only
			r.recordingWait()
		}
		select {
		case <-m:
		case <-r.stopped:
		case <-ctx.Done():
			select {
			case <-m: // written as the deadline passed: complete
			default: // a live recording's stop, or a stopped recording's queued records, still unwritten (owner, 2026-11-11)
				a.rec.cut()
			}
		}
	}
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
				runs[i] = runReturn(rs.err)
			default:
			}
			continue
		}
		select {
		case <-rs.done:
			runs[i] = runReturn(rs.err)
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

// runReturn is a run's return as Drain reports it: context.Canceled, the cancel Drain sent, reads as nil.
func runReturn(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
