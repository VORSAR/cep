# S1.1c-7 design note: Drain's phase follows its holds, not a snapshot

*The design note for the fix of `Drain`'s race windows, by a fresh agent on 2026-11-22, as given; `S/` is its scratch folder. The owner's answers the same day ([decisions](decisions.md#owner-decisions-2026-11-22--the-drain-fixs-design)): Q1, the one-read `restorePhase` of §4.3; Q2, row 7a becomes a known issue.*

*Annotated 2026-11-25: since PR 2's re-check (RCA-1, da2885c), `restorePhase` reads `a.opened`, which Activate's last step sets once the app's flushers and recorder run, not `a.started`, so a failed Drain never opens the input before they run ([decisions](decisions.md#owner-decisions-2026-11-25--pr-2s-re-check)). Where this note says `a.started` in `restorePhase`, the code now has `a.opened`.*

Proposed file: `docs/sprints/s1-1c-7-design-drain.md`. Base: `730fa8e3` on `s1-1c-7/integration`. Nothing was committed; both scratch worktrees are removed.

## 0. Status

**Proven by runs (pinned go1.27.4, scratch worktree at 730fa8e3):**
- Both scratch red tests are red on the unchanged code: `TestPR2AActivateRacingAnExpiredDrainLeavesTheAppLive` (phase 0, Send `ErrInactive`) and `TestPR2QWindowWithAStuckRunLeavesTheAppTakingInput` (phase active, Send nil).
- With the prototype, `go test -count=1 ./internal/runtime/... .` exits 0 (`internal/runtime` ok, root ok).
- With the prototype, `CGO_ENABLED=1 go test -race -count=1 ./internal/runtime/...` exits 0 with 0 "DATA RACE" lines.
- No existing test was edited, and none pins the old behaviour.
- The ten new or ported tests are green under `-race -count=3`; seven plants each turn at least three tests red in 3 of 3 runs (§6).

**Two more windows of the same root cause, found by the black-box tests on the unchanged code (red 3 of 3):**
- **Window 4.** Two Drains of a never-activated app, one already ended: the live Drain returns nil, yet the phase reads inactive and the flag false.
- **Window 5.** Two Drains that both fail with nothing queued can leave the app stuck in draining for good. Activate then answers `ErrDrained` for an app nothing drained, which breaks "nothing was queued, so the app is not drained".
- Both come from the `set` and `was` split in `queueDrain`, and the same change fixes them.

**Window 1 is worse than its known-issue text:** after the live Drain returns nil the phase can read active. The test stopped at that phase check, so its Remove was never reached; `shutdown.go:23-24` answers `ErrActive` for that phase (by reading).

**Not proven:**
- `TestPR2QWindowWithAStuckRunLeavesTheAppTakingInput` could not run unchanged against the new code, because it calls `markDraining` and `revertUnlessQuieted` with the old signatures. I ported it step for step to the new helpers (`TestADrainStuckOnARunAfterAnEndedDrainReturnedRefusesInput`).
- The black-box window-3 test was green 3 of 3 on the unchanged code at 500 rounds (QA saw 2 hits in 3000). Its red on old code rests on the step replay only.
- The recommended `restorePhase` (one read, Q1) ran only on the Drain-related tests under `-race -count=3`, not the full suites. The full-suite runs used the two-read form.
- The committed name of the Activate test was not run; only its scratch name was. The rename is mechanical.
- No benchmark was run. The claim that the per-event path is untouched is by reading.
- The re-cut G10-20 plant (P8) was not run.
- No Hindsight search was made; the tools were not available in this session.

**Not mine:** the main checkout now shows `M docs/sprints/decisions.md` and `?? docs/reviews/s1-1c-7-pr2-architecture.md`. I made no edit there.

## 1. Graph

Repository `cep` (`/Users/ramith/code/cep`). The index's `meta.json` gives `lastCommit` 730fa8e3, which is HEAD, so the index is fresh. I read `meta.json` because the resource reader was not available.

| Symbol | Risk (upstream, tests included) | d=1 |
|---|---|---|
| `Runtime.Drain` | **HIGH**, lower bound (one call site with an untyped receiver dropped) | 24 direct, 36 in all; modules Runtime and Playback |
| `queueDrain` | MEDIUM | `Drain` |
| `awaitRuns` | MEDIUM | `Drain` |
| `revertUnlessQuieted` | LOW | `queueDrain` (the graph missed `Drain`'s own call at `drain.go:28`) |
| `revertDrain` | LOW | `revertUnlessQuieted` |
| `markDraining` | **UNKNOWN** | none resolved |
| `startAppGoroutines` | LOW | `Activate` |
| `Runtime.gate` | MEDIUM, 63 impacted, 5 direct | not touched |

- **HIGH on `Drain`:** its signature and its results for every single-Drain case are unchanged; only the failure exits change. The 24 direct callers are tests plus `closeApp` (`shutdown.go:106`) and the facade (`apphandle.go:69` and `:88`).
- **UNKNOWN on `markDraining`:** a text search over every `.go` file finds it only in `drain.go`. The same holds for `revertDrain` and `revertUnlessQuieted`.
- **`awaitRuns` by name:** `lifecycle_deadlines_internal_test.go:150` matches the frame `(*Runtime).awaitRuns(`. The name is kept.
- **`detect_changes`** (scope all, on the scratch worktree): 8 changed symbols in 2 files, risk low, 0 affected processes. The symbols are `appState`, `appState.runMu`, `Drain`, `markDraining`, `revertDrain`, `revertUnlessQuieted`, `queueDrain`, and `awaitRecordingStop` (line shift only).

## 2. The rule

A Drain **holds** the app's phase at draining from its first step. The holds are counted in `appState.holds` under `runMu`. A Drain gives its hold up only when it returns having queued nothing, and only while no Drain has queued the quiet item (`waited`). A Drain that queued its drain item never gives its hold up, so the app stays draining until Remove. When the last hold is given up, the drain flag is cleared and the phase is put back to what the router holds the app as at that moment: active if `a.started`, inactive otherwise. No snapshot is kept.

Three corrections to the coordinator's sketch:
1. **"Else restore" is wrong for a never-activated app.** It has no `waited`, so a failed Drain that leaves last would undo a drain item another Drain queued (window 4). Keeping the queuing Drain's hold fixes it without a second flag.
2. **Reading `a.started` and then writing the phase is not atomic against Activate's tail,** which takes no lock. `restorePhase` reads `started` after its move to inactive, so no lock is added to Activate.
3. **The flag's `set` logic goes.** A failed Drain could clear a flag another Drain relied on, or skip its own revert (window 5). The flag is now set by every `queueDrain` and cleared only together with the phase's restore, so flag true implies phase draining.

What binds, and how it is kept:
- **G10-20:** `waited` and `holds` are read and written only under `runMu`. The lock order is unchanged; the new helpers take `runMu` alone.
- **No new unbounded wait:** `holdDraining` takes `runMu` at Drain's entry. A started app's Drain already takes it as its next step (`awaitRuns`), so the bound is the same: another Drain's own deadline. For a never-started app the lock is new at entry, held by others only briefly, or by a Drain racing an Activate while it queues the quiet item (bounded by that Drain's context).
- **QA Q15, break-it R1:** a Drain that queued nothing leaves no drained state. It now also holds for two such Drains at once.
- **Activate's contract:** kept (window 2).
- **`nosleep_test.go`:** no timer or sleep is added.
- **Rule 13a:** `gate`, Send and the router are not touched. The new field is the last in `appState`, so no existing field's offset moves. Not benchmarked.

## 3. Interleavings

"Ended" is a Drain whose context has already ended; "live" is one that can queue.

| # | Race | Phase | Flag | `waited` | Input after | Evidence |
|---|---|---|---|---|---|---|
| 1 | ended ‖ live that completes, active app (window 1) | draining | true | true | `ErrDrained`; Remove nil | run |
| 2 | ended ‖ live stuck on a run (window 3) | draining | false, or true if the ended one reached `queueDrain` | true | Send and Run `ErrDrained`; a repeated Drain runs the barrier | run |
| 3 | ended ‖ ended, active app (window 5) | active | false | false | accepted | run |
| 4 | ended ‖ ended, inactive app (window 5) | inactive | false | false | `ErrInactive`; Activate nil, then accepted | run |
| 5 | ended ‖ live, never-activated app (window 4) | draining | true | false | `ErrDrained` | run |
| 6 | live ‖ live | draining | true | true | `ErrDrained`; both get the same result | existing tests |
| 7a | Activate ‖ ended; Drain marks before Activate's first check | inactive | false | false | Activate `ErrDrained`; a repeated Activate is nil | run; unchanged, see Q2 |
| 7b | Drain marks after the check, before the router | active | false | false | accepted | run (rounds) |
| 7c | Drain marks after the router, before Activate's tail (window 2) | active | false | false | accepted; second Activate `ErrStarted` | run, deterministic |
| 8 | Activate ‖ live | draining | true | either | drain item first: Activate `ErrDrained`; activate item first: Activate nil, then drained | existing tests; unchanged |
| 9 | ended ‖ Remove, inactive app | removed | n/a | false | `ErrRemoved`; no CAS overwrites removed | by reading |
| 10 | ended ‖ Remove, active app | active | false | false | Remove `ErrActive`, or `ErrDrainIncomplete` while the Drain is in flight | by reading; unchanged |
| 11 | a user's ended Drain ‖ Close | draining, then removed | true | true | Close's Drain holds, its Remove is nil | by reading |
| 12 | Drain ‖ Run | as the Drain ends | – | – | Run after the mark `ErrDrained`; after a Drain that queued nothing, accepted with a live source context | by reading; unchanged |

In row 11, on the old code Close's Remove could meet a phase put back to active and report a failed removal. I did not run that.

## 4. Code

Two files change.

### 4.1 `internal/runtime/app.go`

- Line 35, comment: `// guards runs and waited` becomes `// guards runs, waited and holds`.
- After `removing` (line 43), as the struct's last field:

```go
	holds     int                // under runMu: the Drains holding the phase at draining (in flight, or their drain item queued)
```

### 4.2 `internal/runtime/drain.go`: the prototype's diff in full

```diff
diff --git a/internal/runtime/app.go b/internal/runtime/app.go
index 240a54c1..0947eace 100644
--- a/internal/runtime/app.go
+++ b/internal/runtime/app.go
@@ -32,7 +32,7 @@ type appState struct {
 	start     int64              // the router's stamp when it installed the app: the Ingest's and the header's
 	srcCtx    context.Context    // the source context, made at Add; Drain cancels it
 	cancelSrc context.CancelFunc //
-	runMu     sync.Mutex         // guards runs and waited
+	runMu     sync.Mutex         // guards runs, waited and holds
 	runs      []*runState        // Run's goroutines
 	waited    bool               // Drain's caller side: the runs were awaited once
 	quiet     bool               // the router's: ticks skip the app (S3.1-1's D3)
@@ -41,6 +41,7 @@ type appState struct {
 	removed   sync.Once          // onRemoved runs once
 	activated bool               // the router's: the app was activated (iActivate); ticks reach it, its drain records
 	removing  bool               // the router's: a removal was accepted; a later Activate or Remove is refused
+	holds     int                // under runMu: the Drains holding the phase at draining (in flight, or their drain item queued)
 }
 
 // runState is one Run goroutine: err is written before done is closed.
diff --git a/internal/runtime/drain.go b/internal/runtime/drain.go
index 209b2055..6c87f66c 100644
--- a/internal/runtime/drain.go
+++ b/internal/runtime/drain.go
@@ -12,6 +12,11 @@ import (
 // that did not return makes Drain return the stuck count wrapped in ctx.Err() without the barrier (the input is
 // refused from here; a repeated Drain runs the barrier without waiting again). A never-activated app is drained at
 // once: nothing was delivered, nothing is recorded. A second Drain of a drained app returns the same result.
+//
+// The app's phase is draining while any Drain of it is in flight, and for good once one has queued an item (the
+// quiet item or the drain item). A Drain that returns an error having queued nothing leaves the app as it is without
+// that Drain: draining if another Drain is in flight or has queued an item, else taking input if the router has
+// activated it, else inactive.
 func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
 	a, err := r.appOf(app)
 	if err != nil {
@@ -20,16 +25,16 @@ func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
 	if appPhase(a.phase.Load()) == phaseRemoved {
 		return DrainResult{}, ErrRemoved
 	}
-	was := a.markDraining()
+	a.holdDraining()
 	var runs []error
 	if a.started.Load() {
 		runs, err = r.awaitRuns(ctx, a)
 		if err != nil {
-			a.revertUnlessQuieted(was) // the quiet item was never queued: live, as before this Drain
+			a.releaseDraining() // a no-op once the quiet item is queued: the app stays draining
 			return DrainResult{Runs: runs}, err
 		}
 	}
-	ch, err := r.queueDrain(ctx, a, app, was)
+	ch, err := r.queueDrain(ctx, a, app)
 	if err != nil {
 		return DrainResult{Runs: runs}, err
 	}
@@ -45,48 +50,66 @@ func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
 	return DrainResult{Pending: rep.pending, Runs: runs}, nil
 }
 
-// markDraining moves the app to phaseDraining and returns the phase it moved from.
-func (a *appState) markDraining() appPhase {
-	// was is the phase this Drain moved from — taken from the CAS that succeeded, so a revert never restores a phase
-	// another call changed meanwhile (review B1); phaseDraining: this Drain moved nothing, so it reverts nothing.
-	was := phaseDraining
-	switch {
-	case a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining)):
-		was = phaseInactive
-	case a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining)):
-		was = phaseActive
-	}
-	return was
+// markDraining moves an inactive or active app to phaseDraining; a removed app stays removed.
+func (a *appState) markDraining() {
+	if !a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining)) {
+		a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining))
+	}
 }
 
-// revertDrain moves the app back to was, the phase markDraining moved it from: nothing was queued, so the app is not
-// drained (QA Q15; break-it R1).
-func (a *appState) revertDrain(was appPhase) {
-	if was != phaseDraining {
-		a.phase.CompareAndSwap(int32(phaseDraining), int32(was))
-	}
+// holdDraining begins a Drain: the phase moves to draining at once, before any lock, so the input is refused from
+// here; then the Drain is counted in a.holds under runMu and the phase is moved again, because a releaseDraining
+// that ran between the first move and the lock may have put it back. The wait on runMu is bounded by another
+// Drain's own deadline (awaitRuns holds the lock while it waits for the runs).
+func (a *appState) holdDraining() {
+	a.markDraining()
+	a.runMu.Lock()
+	defer a.runMu.Unlock()
+	a.holds++
+	a.markDraining()
 }
 
-// revertUnlessQuieted moves the app back to was unless a Drain has queued the quiet item. It reads waited under
-// runMu, which awaitRuns holds while it queues that item and sets waited, so the read and the revert never race it.
-func (a *appState) revertUnlessQuieted(was appPhase) {
+// releaseDraining ends a Drain that queued nothing (QA Q15; break-it R1). Once a Drain has queued the quiet item
+// the app is quieted and stays draining. Otherwise this Drain's hold is given up, and when it was the last one the
+// drain flag is cleared and the phase restored. A Drain that queued its drain item never calls this, so its hold
+// keeps the app draining. It reads waited under runMu, which awaitRuns holds while it queues the quiet item and
+// sets waited.
+func (a *appState) releaseDraining() {
 	a.runMu.Lock()
 	defer a.runMu.Unlock()
-	if !a.waited {
-		a.revertDrain(was)
+	if a.waited {
+		return
+	}
+	a.holds--
+	if a.holds > 0 {
+		return // another Drain is in flight, or queued its drain item: the phase is its to keep
+	}
+	a.draining.Store(false)
+	a.restorePhase()
+}
+
+// restorePhase moves a draining app back to what the router holds it as, read now and not when the Drain began:
+// active once activated (a.started), inactive otherwise. An Activate whose own move to active met phaseDraining
+// may have set a.started since the first read, so it is read again after the move to inactive. The caller holds
+// runMu.
+func (a *appState) restorePhase() {
+	if a.started.Load() {
+		a.phase.CompareAndSwap(int32(phaseDraining), int32(phaseActive))
+		return
+	}
+	a.phase.CompareAndSwap(int32(phaseDraining), int32(phaseInactive))
+	if a.started.Load() {
+		a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive))
 	}
 }
 
 // queueDrain sets the drain flag, then queues the drain item and returns its reply channel. When nothing was queued
-// and no earlier Drain set the flag, it clears the flag and, if the quiet item was never queued, reverts the phase.
-func (r *Runtime) queueDrain(ctx context.Context, a *appState, app AppID, was appPhase) (chan reply, error) {
-	set := !a.draining.Swap(true)
+// it gives up this Drain's hold, which clears the flag and restores the phase once no Drain holds the app.
+func (r *Runtime) queueDrain(ctx context.Context, a *appState, app AppID) (chan reply, error) {
+	a.draining.Store(true)
 	it := item{kind: iDrain, app: app, reply: make(chan reply, 1)}
 	if err := r.enqueue(ctx, it); err != nil {
-		if set { // nothing was queued, and no earlier Drain set the flag: the app is not drained (QA Q15; break-it R1)
-			a.draining.Store(false)
-			a.revertUnlessQuieted(was)
-		}
+		a.releaseDraining() // nothing was queued by this Drain (QA Q15; break-it R1)
 		return nil, err
 	}
 	return it.reply, nil
```

Why `holdDraining` moves the phase twice: the first move is lock-free, as today, so input is refused at once. Between that move and the lock, another Drain's last release can restore the phase; the second move, under the lock with the count, undoes that.

### 4.3 `restorePhase`: the recommended form (Q1)

The prototype's second read of `a.started` matters only when the router activates the app between the first read and the move. No test of mine turns that branch red (plant P2b, green 3 of 3). The form below has one read, and every line is pinned by a deterministic test:

```go
// restorePhase moves a draining app back to what the router holds it as, read now and not when the Drain began:
// to inactive, then on to active once the router has activated it (a.started). The read comes after the move, so an
// Activate whose own move to active met phaseDraining is finished here. The caller holds runMu.
func (a *appState) restorePhase() {
	a.phase.CompareAndSwap(int32(phaseDraining), int32(phaseInactive))
	if a.started.Load() {
		a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive))
	}
}
```

Why it is correct against Activate's tail (`CompareAndSwap(phaseInactive, phaseActive)`, made after the router set `started`):
- The tail runs before the move to inactive: it fails, and `started` is already true, so the read sees it and moves the phase on to active.
- The tail runs after the move: it succeeds.

Its cost is the subject of Q1.

## 5. Tests

New file `internal/runtime/drain_holds_internal_test.go`, package `runtime`, helper prefix `dh`. None uses a sleep or a timer. As prototyped they do not call `t.Parallel()`.

Imports: `context`, `errors`, `goruntime "runtime"`, `testing`, `github.com/vorsar/cep/internal/event`.

How the stuck Drain is made deterministic: its Run waits for the source context, which Drain cancels straight after it queues the quiet item, and then ends the Drain's own context. So the deadline always falls after the quiet item and before the run returns.

```go
// dhRounds is each black-box race's repetitions within one test.
const dhRounds = 500

// dhStuckRun starts one Run on app id that ends ctx's cancel once Drain has cancelled the sources, then stays until
// release is closed: a Drain with that ctx queues the quiet item and then meets its deadline on a stuck run.
func dhStuckRun(t *testing.T, ctx context.Context, r *Runtime, id AppID, cancel context.CancelFunc, release chan struct{}) {
	t.Helper()
	if err := r.Run(ctx, id, func(src context.Context) error { <-src.Done(); cancel(); <-release; return nil }); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
```

### 5.1 Deterministic tests

**T1: window 2.** The test holds `runMu` inside the `activateHold` seam, so the Drain marks, then waits for the lock while Activate's tail runs.

```go
// Activate overtaken, between the router's answer and its own last step, by a Drain whose ctx had ended: Activate
// returns nil and the app takes input.
func TestActivateOvertakenByADrainThatQueuedNothingLeavesTheAppTakingInput(t *testing.T) {
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	a := r.appList()[id]
	drained := make(chan error, 1)
	r.activateHold = func() {
		a.runMu.Lock()
		go func() { _, err := r.Drain(ldExpired(), id); drained <- err }()
		for appPhase(a.phase.Load()) != phaseDraining {
			goruntime.Gosched()
		}
	}
	if err := r.Activate(ctx, id); err != nil {
		t.Fatal(err)
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
```

**T2: the count.** `a.holdDraining()` stands for a Drain in flight; the other Drain is a real call.

```go
// A Drain whose ctx had ended, returning while another Drain of the app is still in flight, leaves the phase at
// draining; when that other Drain then gives up having queued nothing, the app takes input again.
func TestAnEndedDrainLeavesThePhaseToADrainStillInFlight(t *testing.T) {
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
```

**T3: window 3, step by step.** This is the port of the QA replay.

```go
// A Drain in flight when a Drain whose ctx had ended returns, which then queues the quiet item and meets its
// deadline on a stuck run: the app is quieted, so Send and Run are refused after both returned their errors.
func TestADrainStuckOnARunAfterAnEndedDrainReturnedRefusesInput(t *testing.T) {
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
```

As prototyped, T3 and T5 wrote the Run inline; `dhStuckRun` has the same body.

**T4: the second move in `holdDraining`.** The test holds `runMu`, sees the first move, and calls `restorePhase` itself, which is what a last release does under that lock.

```go
// A Drain that moved the phase to draining before it could take the run lock, and whose move a release then put
// back, holds the phase at draining once it has the lock.
func TestADrainThatMarkedBeforeAReleaseRestoredThePhaseStillHoldsItAtDraining(t *testing.T) {
	ctx := cgCtx(t)
	r := ldRuntime(t)
	t.Cleanup(func() { ldClose(t, r) })
	id := ldStarted(t, ctx, r, cgApp(t, func(event.Event) error { return nil }))
	a := r.appList()[id]
	a.runMu.Lock()
	held := make(chan struct{})
	go func() { a.holdDraining(); close(held) }()
	for appPhase(a.phase.Load()) != phaseDraining { // the Drain's first move, made before the lock
		goruntime.Gosched()
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
```

**T5: a quieted app stays draining.** Three real calls in sequence.

```go
// A Drain whose ctx had ended, queueing nothing while an earlier Drain's quiet item stands, leaves the drain flag
// and the phase refusing input.
func TestAnEndedDrainOfAQuietedAppLeavesItRefusingInput(t *testing.T) {
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
```

### 5.2 Black-box tests

Two real calls race, 500 rounds each. They are always green with the fix, because they assert what every interleaving must give. Their red on a broken rule is by chance.

```go
// A Drain that quieted the app and then met its deadline on a stuck run, racing a Drain whose ctx had already ended:
// both return errors, and the input is refused from there, whichever takes the run lock first.
func TestADrainStuckOnARunRacingAnEndedDrainRefusesInput(t *testing.T) {
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
```

### 5.3 Red first, on the unchanged code

| Test | Unchanged code |
|---|---|
| scratch window-2 replay (T1 under its scratch name) | red, deterministic |
| scratch window-3 replay (old helpers) | red, deterministic |
| `TestTwoDrainsAtOnceOneAlreadyEndedLeaveTheAppRemovable` | red 3 of 3: "phase after a Drain that returned nil: 1" |
| `TestTwoDrainsOfANeverActivatedAppOneAlreadyEndedLeaveItDrained` | red 3 of 3: "phase 0, drain flag false" |
| `TestTwoEndedDrainsAtOnceLeaveTheAppAsItWas` | red 3 of 3: Activate answers `ErrDrained` |
| `TestADrainStuckOnARunRacingAnEndedDrainRefusesInput` | green 3 of 3 (the window is rare) |
| `TestActivateRacingAnEndedDrainLeavesTheAppUsable` | not run on the unchanged code |
| T2 to T5 | cannot compile on the unchanged code; they use the new helpers |

## 6. Revert plants

All in `internal/runtime/drain.go`, each run with `CGO_ENABLED=1 go test -race -count=3` on the Drain tests. The control with no plant was ok.

| Plant | Find | Replace | Red, 3 of 3 runs |
|---|---|---|---|
| P1: the count ignored | `if a.holds > 0 {` | `if false {` | T2, T3, `TestTwoDrainsAtOnceOneAlreadyEndedLeaveTheAppRemovable`, `TestTwoDrainsOfANeverActivatedAppOneAlreadyEndedLeaveItDrained` |
| P2: restore to inactive whatever the router holds | every `a.started.Load()` inside `restorePhase` | `false` | T1, T2, T4, `TestActivateRacingAnEndedDrainLeavesTheAppUsable`, `TestTwoEndedDrainsAtOnceLeaveTheAppAsItWas`, existing `TestDrainOfAnActiveAppWithAnExpiredContextLeavesItActive` |
| P2b: the two-read form's second read removed | the `if a.started.Load() {` after the move to inactive | `if false {` | **none; green 3 of 3** |
| P3: the second move under the lock removed | the `a.markDraining()` line after `a.holds++` | (deleted) | T4, `TestTwoDrainsAtOnceOneAlreadyEndedLeaveTheAppRemovable`, `TestTwoDrainsOfANeverActivatedAppOneAlreadyEndedLeaveItDrained` |
| P4: `queueDrain` keeps its hold on failure | the `a.releaseDraining()` line in `queueDrain` | (deleted) | `TestActivateRacingAnEndedDrainLeavesTheAppUsable`, `TestTwoEndedDrainsAtOnceLeaveTheAppAsItWas`, existing `TestDrainOfAnInactiveAppWithAnExpiredContextLeavesItInactive` |
| P5: the `awaitRuns` failure keeps its hold | the `a.releaseDraining()` line in `Drain` | (deleted) | T1, T2, `TestActivateRacingAnEndedDrainLeavesTheAppUsable`, `TestTwoEndedDrainsAtOnceLeaveTheAppAsItWas`, existing `TestDrainOfAnActiveAppWithAnExpiredContextLeavesItActive` |
| P6: release ignores `waited` | the three lines `if a.waited {`, `return`, `}` in `releaseDraining` | (deleted) | T3, T5, `TestADrainStuckOnARunRacingAnEndedDrainRefusesInput`, existing `TestDrainWithARunThatIgnoresCancel` |
| P7: the flag not cleared at the last release | `a.draining.Store(false)` in `releaseDraining` | (deleted) | `TestActivateRacingAnEndedDrainLeavesTheAppUsable`, `TestTwoEndedDrainsAtOnceLeaveTheAppAsItWas`, existing `TestDrainOfAnInactiveAppWithAnExpiredContextLeavesItInactive` |

- **P2 on the recommended one-read form** (the single `if a.started.Load() {` becomes `if false {`): T1, T2 and `TestDrainOfAnActiveAppWithAnExpiredContextLeavesItActive` were red in one run without `-race`.
- **P8, not run:** G10-20's plant `E1-G10-20-revert.diff` no longer applies, because `revertUnlessQuieted` is gone. Re-cut it: in `releaseDraining`, move the `if a.waited { return }` block above `a.runMu.Lock()`. It should turn `TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained` red under `-race`.

## 7. Docs

- **`Drain`'s doc comment:** the added paragraph in §4.2.
- **`docs/product/known-issues.md:17` and `gate/known-issues.json:86` (S1.1c-7-1):** closed, with nothing left of it. Both change in one commit, since a T2 test pins the ids to the table.
- **`docs/sprints/decisions.md`, decision 2 of 2026-11-20 (about line 1299):** annotate, do not rewrite. The note should say the claim "the draining flag still refuses input, so nothing is lost" was wrong once the live Drain met its deadline on a stuck run, and that the window is closed by this fix. The second reference near line 1462 gets the same pointer.
- **`docs/sprints/s1-1c-7-record.md`:** G10-20's row (line 481) names the old plant; add the re-cut plant and this fix's row.
- **`docs/sprints/s1-1c-7-design-e1.md`:** its G10-20 sections name `revertUnlessQuieted`. Add one line that the helper became `releaseDraining`, pointing here.
- **Not a doc change, for the VS Code extension repo:** nothing; no keyword or lexer is touched.

## 8. Commits

1. `fix(runtime): a Drain that queued nothing leaves the phase to the Drains still holding it`: `drain.go`, `app.go` and `drain_holds_internal_test.go`. One commit, as E1 C5 was, with the red-first evidence of §5.3 in its message.
2. `docs(product): known issue S1.1c-7-1 is closed`: `known-issues.md` and `gate/known-issues.json`.
3. `docs(sprint): the drain design note, the record's rows and the annotated decision`: this note, `s1-1c-7-record.md`, `s1-1c-7-design-e1.md`, `decisions.md`.

## 9. Pre-mortem: how this is wrong in six months

1. **A new early return in `Drain` between `holdDraining` and the drain item's queuing, without `releaseDraining`.** The app stays in draining with nothing queued. It fails safe (input refused, a repeated Drain works), but an inactive app could never be activated. The existing tests `TestDrainOf…WithAnExpiredContextLeavesIt…` would catch it only on a path they exercise. Mitigation: `Drain`'s comment, and P4 and P5 as standing plants.
2. **Someone "fixes" the count that never goes down.** `holds` grows by one for every Drain that queued its drain item, by design. A tidy-up that releases on success brings window 4 back; P1 and `TestTwoDrainsOfANeverActivatedAppOneAlreadyEndedLeaveItDrained` guard it.
3. **A second writer of `a.started`, or an un-activate.** `restorePhase` assumes `started` only ever goes from false to true, and only the router sets it.
4. **A new caller of `markDraining` outside `holdDraining`,** such as Close marking apps directly. It would be a phase without a hold, which the next release undoes.
5. **A long holder of `runMu`.** Every Drain now takes `runMu` at entry. Anything new that holds it across a blocking call delays the refusal of input for apps not yet marked. The first, lock-free move covers this today.
6. **The drain flag is removed as redundant.** After this fix, flag true implies phase draining, so `gate`'s second check looks dead. It is on the per-event path, so removing it is its own change with its own measurement.
7. **The black-box tests are read as proof.** They are green on the fix whatever the timing; their red is by chance. The deterministic ones (T1 to T5) carry the proof.

## 10. Questions for the owner

**Q1. When a failed Drain puts an active app back, may the phase read "inactive" for an instant on the way?**

Recommendation: **yes**, the one-read form of §4.3.

- **For:** every line is pinned by a test that fails when the line is broken. The form with no such instant has a branch that no test of mine turns red (P2b).
- **Against:** a Send, TableWrite, BatchEnd or Run that lands in that instant is refused with `ErrInactive` instead of `ErrDrained`. The instant sits inside a window where those calls were already refused, and both answers are spurious there.
- **If no:** build the two-read form as prototyped, and accept that its second read rests on reasoning plus a rounds test.

Example: `Stop` with an ended context fails on a running app; a `Send` in the same instant gets "not started" instead of "stopped", once, and the next `Send` is accepted.

**Q2. An Activate that arrives while a doomed Drain is still in flight gets `ErrDrained`, though nothing is drained (row 7a). Leave it, or fix it?**

Recommendation: **leave it in this fix and record it as a known issue.** It is older than this change and a repeated Activate succeeds.

- **For leaving:** fixing it means Activate waits for or retries past a Drain in flight, which adds a wait to Activate.
- **Against leaving:** through the facade a `Start` may report the app as stopped, and by the 2026-11-07 decision a stopped app needs a new Deploy, so a caller could redeploy for nothing. I did not check what `mapErr` returns here.

## 11. Files

- `internal/runtime/drain.go`
- `internal/runtime/app.go`
- `internal/runtime/lifecycle.go` (read only; Activate's tail is not changed)
- Kept scratch copies, the new file's source: `S/s17/drain-keep/` holds `drain-proto.diff`, `zz_drain_rule_test.go`, `zz_drain_blackbox_test.go` and `zz_pr2a1_test.go`.
- Run outputs: `S/s17/drain-full.out` and `S/s17/drain-race.out`.
