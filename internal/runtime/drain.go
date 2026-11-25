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
//
// The app's phase is draining while any Drain of it is in flight, and for good once one has queued an item (the
// quiet item or the drain item). A Drain that returns an error having queued nothing leaves the app as it is without
// that Drain: draining if another Drain is in flight or has queued an item, else taking input if Activate has
// started the app's flushers and recorder, else inactive.
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
	a, err := r.appOf(app)
	if err != nil {
		return DrainResult{}, err
	}
	if appPhase(a.phase.Load()) == phaseRemoved {
		return DrainResult{}, ErrRemoved
	}
	a.holdDraining()
	var runs []error
	if a.started.Load() {
		runs, err = r.awaitRuns(ctx, a)
		if err != nil {
			a.releaseDraining() // a no-op once the quiet item is queued: the app stays draining
			return DrainResult{Runs: runs}, err
		}
	}
	ch, err := r.queueDrain(ctx, a, app)
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

// markDraining moves an inactive or active app to phaseDraining; a removed app stays removed.
func (a *appState) markDraining() {
	if !a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining)) {
		a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining))
	}
}

// holdDraining begins a Drain: the phase moves to draining at once, before any lock, so the input is refused from
// here; then the Drain is counted in a.holds under runMu and the phase is moved again, because a releaseDraining
// that ran between the first move and the lock may have put it back. The wait on runMu is bounded by another
// Drain's own deadline (awaitRuns holds the lock while it waits for the runs).
func (a *appState) holdDraining() {
	a.markDraining()
	a.runMu.Lock()
	defer a.runMu.Unlock()
	a.holds++
	a.markDraining()
}

// releaseDraining ends a Drain that queued nothing (QA Q15; break-it R1). Once a Drain has queued the quiet item
// the app is quieted and stays draining. Otherwise this Drain's hold is given up, and when it was the last one the
// drain flag is cleared and the phase restored. A Drain that queued its drain item never calls this, so its hold
// keeps the app draining. It reads waited under runMu, which awaitRuns holds while it queues the quiet item and
// sets waited.
func (a *appState) releaseDraining() {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	if a.waited {
		return
	}
	a.holds--
	if a.holds > 0 {
		return // another Drain is in flight, or queued its drain item: the phase is its to keep
	}
	a.draining.Store(false)
	a.restorePhase()
}

// restorePhase moves a draining app back to what Activate has made it, read now and not when the Drain began: to
// inactive, then on to active once Activate's last step has started the app's flushers and recorder (a.opened), and
// not merely once the router has activated it (a.started), so the input never opens before they run. The read comes
// after the move, so an Activate whose own move to active met phaseDraining is finished here. An active app reads
// inactive for an instant on the way: a call landing in it is refused as not activated, as it was refused as
// drained just before. The caller holds runMu.
func (a *appState) restorePhase() {
	a.phase.CompareAndSwap(int32(phaseDraining), int32(phaseInactive))
	if a.opened.Load() {
		a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive))
	}
}

// queueDrain sets the drain flag, then queues the drain item and returns its reply channel. When nothing was queued
// it gives up this Drain's hold, which clears the flag and restores the phase once no Drain holds the app.
func (r *Runtime) queueDrain(ctx context.Context, a *appState, app AppID) (chan reply, error) {
	a.draining.Store(true)
	it := item{kind: iDrain, app: app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		a.releaseDraining() // nothing was queued by this Drain (QA Q15; break-it R1)
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
