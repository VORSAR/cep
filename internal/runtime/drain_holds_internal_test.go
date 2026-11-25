package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// dhRounds is each black-box race's repetitions within one test.
const dhRounds = 500

// dhSpin caps the wait for a Drain's first move, which it makes at once: a missing move fails in seconds, not at the
// package's timeout.
const dhSpin = 5 * time.Second

// dhAwaitDraining waits, while the test holds a.runMu, for a Drain blocked on that lock to have made its first move
// (the phase to draining, before the lock), and reports whether it came. If it never comes it unlocks runMu, so the
// Drain and the cleanup's Close can finish; the caller then fails with dhNoFirstMove.
func dhAwaitDraining(ctx context.Context, a *appState) bool {
	wctx, cancel := context.WithTimeout(ctx, dhSpin)
	defer cancel()
	if !cgUntil(wctx, func() bool { return appPhase(a.phase.Load()) == phaseDraining }) {
		a.runMu.Unlock()
		return false
	}
	return true
}

// dhNoFirstMove is the failure when dhAwaitDraining's move never came.
const dhNoFirstMove = "a Drain blocked on the run lock never moved the phase to draining (holdDraining's first move, " +
	"before the lock)"

// dhStuckRun starts one Run on app id that ends ctx's cancel once Drain has cancelled the sources, then stays until
// release is closed: a Drain with that ctx queues the quiet item and then meets its deadline on a stuck run.
func dhStuckRun(t *testing.T, ctx context.Context, r *Runtime, id AppID, cancel context.CancelFunc, release chan struct{}) {
	t.Helper()
	if err := r.Run(ctx, id, func(src context.Context) error { <-src.Done(); cancel(); <-release; return nil }); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// Activate overtaken, between the router's answer and its own last step, by a Drain whose ctx had ended: Activate
// returns nil and the app takes input.
func TestActivateOvertakenByADrainThatQueuedNothingLeavesTheAppTakingInput(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	a := r.appList()[id]
	drained := make(chan error, 1)
	moved := false
	r.activateHold = func() { // no t.Fatal in here: Activate must reach its last step, or Close waits on its flushers
		a.runMu.Lock()
		go func() { _, err := r.Drain(ldExpired(), id); drained <- err }()
		moved = dhAwaitDraining(ctx, a)
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatal(err)
	}
	if !moved {
		t.Fatal(dhNoFirstMove)
	}
	a.runMu.Unlock()
	if err := <-drained; err == nil {
		t.Fatalf("the expired Drain returned nil")
	}
	if p := appPhase(a.phase.Load()); p != phaseActive {
		t.Errorf("phase after Activate returned nil and the expired Drain failed: %d; want %d (active)", p, phaseActive)
	}
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Errorf("Send after Activate returned nil: %v; want nil", err)
	}
	if err := r.Activate(ctx, id); !errors.Is(err, ErrStarted) {
		t.Errorf("a second Activate: %v; want ErrStarted", err)
	}
}

// A Drain whose ctx had ended, run between the router's yes to Activate and Activate's last step, which starts the
// app's flushers and recorder: the input stays closed until they run, then Activate opens it.
func TestAnEndedDrainDuringActivateKeepsTheInputClosedUntilTheGoroutinesRun(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	a := r.appList()[id]
	var (
		drained, sent error
		inside        appPhase
	)
	r.activateHold = func() {
		_, drained = r.Drain(ldExpired(), id)
		inside, sent = appPhase(a.phase.Load()), r.Send(ctx, id, cgEv(1))
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatal(err)
	}
	if drained == nil {
		t.Fatalf("the expired Drain returned nil")
	}
	if inside != phaseInactive || !errors.Is(sent, ErrInactive) {
		t.Errorf("before Activate started the app's goroutines: phase %d, Send %v; want %d (inactive) and ErrInactive",
			inside, sent, phaseInactive)
	}
	if p := appPhase(a.phase.Load()); p != phaseActive {
		t.Errorf("phase after Activate returned nil: %d; want %d (active)", p, phaseActive)
	}
	if err := r.Send(ctx, id, cgEv(2)); err != nil {
		t.Errorf("Send after Activate returned nil: %v; want nil", err)
	}
}

// A Drain whose ctx had ended, run once Activate has marked the app's goroutines running and before its move to
// active: the Drain's restore finishes the move, so the app takes input.
func TestAnEndedDrainAfterActivatesMarkStillLeavesTheAppActive(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	a := r.appList()[id]
	var drained error
	r.openHold = func() { _, drained = r.Drain(ldExpired(), id) }
	if err := r.Activate(ctx, id); err != nil {
		t.Fatal(err)
	}
	if drained == nil {
		t.Fatalf("the expired Drain returned nil")
	}
	if p := appPhase(a.phase.Load()); p != phaseActive {
		t.Errorf("phase after Activate returned nil: %d; want %d (active)", p, phaseActive)
	}
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Errorf("Send after Activate returned nil: %v; want nil", err)
	}
}

// A Drain whose ctx had ended, returning while another Drain of the app is still in flight, leaves the phase at
// draining; when that other Drain then gives up having queued nothing, the app takes input again.
func TestAnEndedDrainLeavesThePhaseToADrainStillInFlight(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	a := r.appList()[id]
	a.holdDraining() // a Drain in flight, before its first queued item
	if _, err := r.Drain(ldExpired(), id); !errors.Is(err, context.Canceled) {
		t.Fatalf("the Drain whose ctx had ended: %v; want context.Canceled", err)
	}
	if p := appPhase(a.phase.Load()); p != phaseDraining {
		t.Fatalf("phase while a Drain is in flight: %d; want %d (draining)", p, phaseDraining)
	}
	if err := r.Send(ctx, id, cgEv(1)); !errors.Is(err, ErrDrained) {
		t.Fatalf("Send while a Drain is in flight: %v; want ErrDrained", err)
	}
	a.releaseDraining() // the Drain in flight gives up, nothing queued
	if p := appPhase(a.phase.Load()); p != phaseActive {
		t.Fatalf("phase once every Drain gave up with nothing queued: %d; want %d (active)", p, phaseActive)
	}
	if err := r.Send(ctx, id, cgEv(2)); err != nil {
		t.Fatalf("Send once every Drain gave up with nothing queued: %v; want nil", err)
	}
}

// A Drain in flight when a Drain whose ctx had ended returns, which then queues the quiet item and meets its
// deadline on a stuck run: the app is quieted, so Send and Run are refused after both returned their errors.
func TestADrainStuckOnARunAfterAnEndedDrainReturnedRefusesInput(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	stuck, cancel := context.WithCancel(ctx)
	release := make(chan struct{})
	dhStuckRun(t, ctx, r, id, cancel, release)
	defer close(release)
	a := r.appList()[id]
	a.holdDraining() // the stuck Drain has begun
	if _, err := r.Drain(ldExpired(), id); !errors.Is(err, context.Canceled) {
		t.Fatalf("the Drain whose ctx had ended: %v; want context.Canceled", err)
	}
	if _, err := r.awaitRuns(stuck, a); !errors.Is(err, context.Canceled) { // the stuck Drain's next step
		t.Fatalf("the stuck Drain's wait for its runs: %v; want context.Canceled", err)
	}
	a.releaseDraining() // the stuck Drain returns its error
	if p := appPhase(a.phase.Load()); p != phaseDraining {
		t.Errorf("phase after a Drain returned its stuck-run error: %d; want %d (draining)", p, phaseDraining)
	}
	if err := r.Send(ctx, id, cgEv(1)); !errors.Is(err, ErrDrained) {
		t.Errorf("Send after a Drain returned its stuck-run error: %v; want ErrDrained", err)
	}
	if err := r.Run(ctx, id, func(context.Context) error { return nil }); !errors.Is(err, ErrDrained) {
		t.Errorf("Run after a Drain returned its stuck-run error: %v; want ErrDrained", err)
	}
}

// A Drain that moved the phase to draining before it could take the run lock, and whose move a release then put
// back, holds the phase at draining once it has the lock.
func TestADrainThatMarkedBeforeAReleaseRestoredThePhaseStillHoldsItAtDraining(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	a := r.appList()[id]
	a.runMu.Lock()
	held := make(chan struct{})
	go func() { a.holdDraining(); close(held) }()
	if !dhAwaitDraining(ctx, a) { // the Drain's first move, made before the lock
		t.Fatal(dhNoFirstMove)
	}
	a.restorePhase() // what a last release does under the lock
	if p := appPhase(a.phase.Load()); p != phaseActive {
		t.Errorf("phase after the restore: %d; want %d (active)", p, phaseActive)
	}
	a.runMu.Unlock()
	<-held
	if p := appPhase(a.phase.Load()); p != phaseDraining {
		t.Errorf("phase once the Drain holds the app: %d; want %d (draining)", p, phaseDraining)
	}
	a.releaseDraining()
	if err := r.Send(ctx, id, cgEv(1)); err != nil {
		t.Errorf("Send once that Drain gave up with nothing queued: %v; want nil", err)
	}
}

// A Drain whose ctx had ended, queueing nothing while an earlier Drain's quiet item stands, leaves the drain flag
// and the phase refusing input.
func TestAnEndedDrainOfAQuietedAppLeavesItRefusingInput(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	stuck, cancel := context.WithCancel(ctx)
	release := make(chan struct{})
	dhStuckRun(t, ctx, r, id, cancel, release)
	if _, err := r.Drain(stuck, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("the Drain stuck on a run: %v; want context.Canceled", err)
	}
	if _, err := r.Drain(ldExpired(), id); !errors.Is(err, context.Canceled) {
		t.Fatalf("the Drain whose ctx had ended: %v; want context.Canceled", err)
	}
	if err := r.Send(ctx, id, cgEv(1)); !errors.Is(err, ErrDrained) {
		t.Errorf("Send to a quieted app: %v; want ErrDrained", err)
	}
	close(release)
	if _, err := r.Drain(ctx, id); err != nil {
		t.Errorf("a repeated Drain once the run returned: %v; want nil", err)
	}
	if err := r.Remove(ctx, id); err != nil {
		t.Errorf("Remove after the repeated Drain: %v; want nil", err)
	}
}

// A Drain that quieted the app and then met its deadline on a stuck run, racing a Drain whose ctx had already ended:
// both return errors, and the input is refused from there, whichever takes the run lock first.
func TestADrainStuckOnARunRacingAnEndedDrainRefusesInput(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range dhRounds {
		id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
		stuck, cancel := context.WithCancel(ctx)
		release := make(chan struct{})
		dhStuckRun(t, ctx, r, id, cancel, release)
		ended, live := ldRace(t, ctx, func() error { _, err := r.Drain(ldExpired(), id); return err },
			func() error { _, err := r.Drain(stuck, id); return err })
		a := r.appList()[id]
		phase, errSend := appPhase(a.phase.Load()), r.Send(ctx, id, cgEv(1))
		errRun := r.Run(ctx, id, func(context.Context) error { return nil })
		close(release)
		if ended == nil || live == nil {
			t.Fatalf("the ended Drain: %v, the stuck Drain: %v; want an error from each", ended, live)
		}
		if phase != phaseDraining || !errors.Is(errSend, ErrDrained) || !errors.Is(errRun, ErrDrained) {
			t.Fatalf("after a Drain stuck on a run: phase %d, Send %v, Run %v; want %d and ErrDrained twice", phase, errSend,
				errRun, phaseDraining)
		}
	}
}

// Two Drains of one active app at once, one whose ctx had already ended: the app ends draining and Remove takes it.
func TestTwoDrainsAtOnceOneAlreadyEndedLeaveTheAppRemovable(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range dhRounds {
		id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
		live, ended := ldRace(t, ctx, func() error { _, err := r.Drain(ctx, id); return err },
			func() error { _, err := r.Drain(ldExpired(), id); return err })
		if live != nil || ended == nil {
			t.Fatalf("the live Drain: %v, the ended Drain: %v; want nil and an error", live, ended)
		}
		if p := appPhase(r.appList()[id].phase.Load()); p != phaseDraining {
			t.Fatalf("phase after a Drain that returned nil: %d; want %d (draining)", p, phaseDraining)
		}
		if err := r.Remove(ctx, id); err != nil {
			t.Fatalf("Remove after a Drain that returned nil: %v; want nil", err)
		}
	}
}

// Two Drains of one never-activated app at once, one whose ctx had already ended: the app ends drained.
func TestTwoDrainsOfANeverActivatedAppOneAlreadyEndedLeaveItDrained(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range dhRounds {
		id := ldAdd(t, ctx, r)
		live, ended := ldRace(t, ctx, func() error { _, err := r.Drain(ctx, id); return err },
			func() error { _, err := r.Drain(ldExpired(), id); return err })
		if live != nil || ended == nil {
			t.Fatalf("the live Drain: %v, the ended Drain: %v; want nil and an error", live, ended)
		}
		a := r.appList()[id]
		if p := appPhase(a.phase.Load()); p != phaseDraining || !a.draining.Load() {
			t.Fatalf("after a Drain that returned nil: phase %d, drain flag %v; want %d and true", p, a.draining.Load(),
				phaseDraining)
		}
		if err := r.Send(ctx, id, cgEv(1)); !errors.Is(err, ErrDrained) {
			t.Fatalf("Send after a Drain that returned nil: %v; want ErrDrained", err)
		}
	}
}

// Two Drains at once whose ctx had both ended queue nothing: an active app takes input again, an inactive one can
// still be activated.
func TestTwoEndedDrainsAtOnceLeaveTheAppAsItWas(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range dhRounds {
		active, idle := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil })), ldAdd(t, ctx, r)
		for _, id := range []AppID{active, idle} {
			one, two := ldRace(t, ctx, func() error { _, err := r.Drain(ldExpired(), id); return err },
				func() error { _, err := r.Drain(ldExpired(), id); return err })
			if one == nil || two == nil {
				t.Fatalf("two ended Drains: %v, %v; want an error from each", one, two)
			}
		}
		if err := r.Send(ctx, active, cgEv(1)); err != nil {
			t.Fatalf("Send to an active app after two Drains that queued nothing: %v; want nil", err)
		}
		if err := r.Activate(ctx, idle); err != nil {
			t.Fatalf("Activate after two Drains that queued nothing: %v; want nil", err)
		}
		if err := r.Send(ctx, idle, cgEv(1)); err != nil {
			t.Fatalf("Send after that Activate: %v; want nil", err)
		}
	}
}

// Activate racing a Drain whose ctx had ended, which queues nothing: an Activate that returned nil left the app
// taking input, and one refused while the Drain was in flight can be repeated.
func TestActivateRacingAnEndedDrainLeavesTheAppUsable(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	for range dhRounds {
		id := ldAdd(t, ctx, r)
		ended, act := ldRace(t, ctx, func() error { _, err := r.Drain(ldExpired(), id); return err },
			func() error { return r.Activate(ctx, id) })
		if ended == nil {
			t.Fatal("the ended Drain returned nil")
		}
		if errors.Is(act, ErrDrained) { // refused while the Drain was in flight: nothing was drained
			act = r.Activate(ctx, id)
		}
		if act != nil {
			t.Fatalf("Activate: %v; want nil", act)
		}
		if err := r.Send(ctx, id, cgEv(1)); err != nil {
			t.Fatalf("Send after Activate returned nil: %v; want nil", err)
		}
	}
}
