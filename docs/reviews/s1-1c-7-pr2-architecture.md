# S1.1c-7 PR 2 (#54) — architecture review

*The blind architecture review of PR 2's combined build, at 730fa8e (2026-11-22). The report is as the agent gave it; paths are made repo-relative, and `S/` is the agent's scratch folder. Each finding's disposition is in [the sprint record](../sprints/s1-1c-7-record.md).*

**Verdict:** I found no blocker introduced by this PR. E1 C7's accept path is atomic and the two concurrency fixes hold as stated. One defect goes beyond known issue S1.1c-7-1: a second window that leaves an activated app refusing all input (PR2A-1). It predates this PR and sits in the lines C5 edited.

I reviewed lane E1 and the hook's lock in depth, and G2-24 partly. Lanes A1, A2 (other than the hook) and B were not reached; see "Not reached".

## Blockers

None rated blocker. PR2A-1 is rated should-fix below, but on a strict reading of ground rule 15 it is a broken contract (Activate returned nil, yet no input is accepted). The owner should decide the rating.

## Should-fix

### PR2A-1 — an Activate racing an expired Drain leaves the app activated in the router and inactive to callers, for good

- **Where:**
  - `internal/runtime/drain.go:23` (markDraining), `:28` and `:88` (the revert), `:64-68` (revertDrain restores `was`)
  - `internal/runtime/lifecycle.go:140` (Activate's tail moves only inactive to active)
- **Scenario:**
  1. Activate passes its phase check and the router activates the app (`a.activated`, `a.started`).
  2. A Drain whose context has already ended runs `markDraining`: inactive to draining, `was = phaseInactive`.
  3. Activate's tail runs `CompareAndSwap(phaseInactive, phaseActive)`. It fails because the phase is draining. Activate returns nil.
  4. The Drain's quiet step fails on the ended context, and `revertUnlessQuieted(was)` puts the phase back to inactive.
- **Result:**
  - Every Send, SendPayload, TableWrite, BatchEnd and Run is refused with ErrInactive.
  - A second Activate is refused with ErrStarted.
  - The router still ticks the app.
  - The only way out is Drain, Remove and a new Add.
- **Reach:** the public facade, through `AppHandle.Start` racing `AppHandle.Stop` with an ended context.
- **Origin:** not introduced by this PR. C5's parent has the same revert at both sites (`if !a.waited { a.revertDrain(was) }`), read from the diff. I did not run the test on origin/main.
- **Why it matters here:** C5 edited these lines, and decisions.md:1299 and known-issues.md:17 record one leftover window (two Drains, phase back to active, "nothing is lost"). This is a different window (phase back to inactive, input refused), so S1.1c-7-1 is not the only one left.
- **Evidence:** red test `TestPR2AActivateRacingAnExpiredDrainLeavesTheAppLive`, run in the scratch worktree with `go test -race -count=1 -run 'TestPR2A' -v ./internal/runtime`:
  ```
  phase after Activate returned nil and the expired Drain failed: 0; want 1 (active)
  Send after Activate returned nil: runtime: the app is not activated; want nil (the failed Drain left the app live)
  a second Activate: runtime: the app is already activated (the router holds the app activated)
  --- FAIL: TestPR2AActivateRacingAnExpiredDrainLeavesTheAppLive
  ```
- **Replay:** the worktree is removed, so this is the test body (package `runtime`; imports `context`, `errors`, `goruntime "runtime"`, `testing`, `internal/event`). The test holds `runMu` only to fix the interleaving; no lock order prevents it in real use.
  ```go
  ctx := cgCtx(t); r := ldRuntime(t); t.Cleanup(func() { ldClose(t, r) })
  id, _ := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
  a := r.appList()[id]
  drained := make(chan error, 1)
  r.activateHold = func() {
  	a.runMu.Lock()
  	go func() { _, err := r.Drain(ldExpired(), id); drained <- err }()
  	for appPhase(a.phase.Load()) != phaseDraining { goruntime.Gosched() }
  }
  if err := r.Activate(ctx, id); err != nil { t.Fatal(err) }
  a.runMu.Unlock()
  <-drained // context.Canceled
  // phase is phaseInactive; r.Send(ctx, id, cgEv(1)) is ErrInactive; r.Activate is ErrStarted
  ```
- **Options (recommendation first):**
  1. Make the revert restore active when `a.started` is set and `was` is inactive. It is a small change in `revertDrain`, with this test as its red.
  2. Widen known issue S1.1c-7-1 to name this window and correct "nothing is lost", forwarding it with the phase-and-flag reconcile to S1.1c-10.

## Later

### PR2A-2 — the activation step is not all-or-nothing on its error branch

- **Where:** `internal/runtime/transitions.go:87-90` and `:99-106`.
- **What happens:** when `applyLoads` fails at load k, loads 0 to k-1 stay put and recorded and the app stays inactive. A retried Activate puts and records them again.
- **Reach:** only when rows change after Add (the documented contract on `App.Loads`) or when `table.Check` and `Upsert` later diverge. The W3 facade builds its own rows (`deploy.go:212`), so no user slice is aliased.
- **Evidence:** red test `TestPR2AActivationStepIsNotAtomicOnItsErrorBranch`, same command:
  ```
  Activate: table B: row 0: attribute 1 "v": want int, got string; rows in A 2, B 0; phase 0
  a refused Activate left 2 rows in A; want 0
  ```
  The test adds two loads (tables A and B), then changes B's row to a string value after Add.
- **Options:** copy Loads at Add so the runtime owns them; or say on `App.Loads` what the error branch leaves behind. E2 must not hand Loads a slice anything else can write.

### PR2A-3 — the recording queue's cost per starting row is 208 bytes, held past Remove

- **Where:** `internal/runtime/app.go:75` and `:90-96`.
- **Measured:** `unsafe.Sizeof(recItem{})` is 208 bytes. The default queue is 13 MiB, and 1,000,000 starting rows add 198 MiB.
- **When it is allocated:** up front, on the router goroutine in `router.add` (transitions.go:42).
- **How long it is held:** for the app's life, and after Remove until Close, because the tombstone keeps `a.rec` (transitions.go:149-152).
- **Also held:** the rows a second time in `a.app.Loads`, which is never cleared, on top of the table's own copies.
- **Against the design:** the note says the cost is "proportional to the rows the app already holds"; that is true but understates the constant and the lifetime.
- **Recommendation:** give E2 and the owner these numbers before W3 turns the path on for recorded apps with large tables.

### PR2A-4 — G2-24: the witness cannot go stale, but it cannot catch a reclassification either

- **Where:** `gate/runner/tests/test_instruments_pin.py:226-260`.
- **What holds:** 41 instruments = 31 judged + 6 `go_test` + 4 "no Python module" (recounted from law.json and registry.json). Each of the 31 rows names exactly one `.py`, so `roots[0]` is unambiguous. No instrument escapes both lists.
- **Gap 1:** `NOT_JUDGED` names two kinds, not the ten instrument ids. The ids are only in the commit message.
- **Gap 2:** the witness count uses the same registry predicate as the classification. A row whose command stops naming a `.py` leaves the judged set without any test edit.
- **Coupling dropped:** `gate/runner/pyshim.py` is now in no instrument's `code` (three named it on main). It is the child process the pyfloor, pytests and proofs rows run. It stays a tracked law path through `gate/runner/`.
- **Readers of `code`:** none changes behaviour.
  - `law.py:183-200` validates the paths only.
  - `law.py:739` shows the change as LAW delta lines.
  - `teeth.py:534-535` needs each path tracked, and all are under `gate/`.

### PR2A-5 — stale comment on `build`

`internal/runtime/build.go:13-14` says "Start and NewSync share it". `runtime.Start` is New, then Add and Activate (runtime.go:188-205), which is why it applies starting rows while `build` refuses them. The design note was annotated for this; the code comment was not.

## Tried and held

### E1 C7 (G10-1, option B) — pre-mortem and red-team

- **A refused Add changes nothing.** `validateApp`, then `checkLoads`, runs on the caller's goroutine before the item is queued (lifecycle.go:51-54).
- **The check matches the write.** `Table.Check` and `Upsert` share `checkRow` (table.go:193-200, :203-208), and `Upsert` has no other refusal (:134-150). `CheckRows` is the one TableWrite uses (requests.go:136).
- **An accepted Activate applies every row before any input.**
  - Loads are put before `a.activated = true` in one router step (transitions.go:86-93).
  - Ticks skip an unactivated app (router_in.go:89).
  - The gate refuses input until the phase is active, which is set after the router's reply (lifecycle.go:140).
- **Loads are applied at most once.** Two Activates, Activate against Remove, and a Drain queued before Activate all resolve in the router's switch (transitions.go:79-94), with no rows put on any refusal.
- **Recording bytes match today's Start.**
  - `applyWrite` is `writeTable`'s former tail unchanged (router_in.go:113-121): one `rTable` per row, no stamp.
  - The header is queued at Add (app.go:76).
  - One Upsert per load equals today's one TableWrite per table.
  - One difference, an improvement: a tick record can no longer fall between the header and the rows.
  - E2 must keep `p.Tables` order (deploy.go:209) in Loads.
- **Queue sizing.** The header plus all rows fit, because RecordQueue is at least 1 after defaults (build.go:76-78). `put` never blocks and is safe on a nil queue (recorder.go:91-93).
- **`build`'s refusal reaches every built path and no added one.**
  - GitNexus `impact(build, upstream, CALLS)`: direct callers New (nil apps) and NewSync; depth 2 newDriver, runtime.Start, root New.
  - Text search: no production code outside `internal/runtime` writes `App.Loads` at HEAD, and FromPlan sets none.
- **The interface for W3 is sound**, given PR2A-2's ownership condition and PR2A-3's numbers.

### G10-20 (C5)

- Every read and write of `waited` is under `runMu` (drain.go:75, :120-125).
- `revertUnlessQuieted` is called with no lock held (:28 after `awaitRuns` returned, :88), so there is no deadlock.
- The order `runMu` before `inflight.mu` appears only in Run (lifecycle.go:151-153).
- The wait is bounded by the other Drain's deadline, as the design states.
- S1.1c-7-1's window is as recorded, with one imprecision: input is refused by the draining flag only once the second Drain reaches `queueDrain` (drain.go:83). Before that it is accepted and processed ahead of the barrier, so nothing is lost.

### G10-5 (hook lock)

- The lock is held across the check, the Popen and the pidfile rename (`gate/hooks/gitnexus_reindex.py:131-136`; `start` writes the pidfile before returning).
- `LOCK_NB` means no wait at all. `BlockingIOError` is an `OSError`, so the losing call exits 0.
- The lock file survives `analyze --force`: in the index's storage directory it is dated Nov 21 and the index was rewritten Nov 22.
- One residual, older than this PR: a reused pid makes `running` true.

### G10-21 (C6)

- A derived stream's refused batch end is not queued (lifecycle.go:195-199). The accepted path is unchanged.
- The one production caller is root `input.go:46`. `AppHandle.Input` already refuses a derived stream (apphandle.go:102-104).
- A source stream's batch end queued between Stop and the drain item is still counted and recorded; that is known issue S3.3-1-3 (router_in.go:28-30).

### G7a-5 texts

No test or golden pins the removed tags. The one text-search hit is a test's own message at redrain_test.go:37.

## GitNexus notes for the coordinator

- `context(newAppState)` reports "exact" but lists only `build` and one test as callers. It misses `router.add` (transitions.go:42).
- `impact(build)` rates LOW with calls only; the brief says HIGH. The caller set matches the brief.
- The index holds `recordQueueFor` and `App.Loads`, so it is at C7 or later.

## Not reached

- **Hindsight memory:** no hindsight tool was available in this session, so no memory page was used.
- **GitNexus freshness resource:** not readable here (no resource tool); freshness is inferred as above. The `detect_changes` output (93k characters) was not read in full.
- **Lanes A1, A2 (other than the hook) and B:** not reviewed. That covers `go_foreign`'s texts, vulns' `_json_file` and `open_copy`, race.py's `--repeat`, treehash's usage, `q2Start`, and `runBlock`/`inPlayback`'s wrapper through `errors.Is`/`errors.As`.
- **Design notes a1a, a1b, a2 and b:** not compared with the code.
- **Import edges against `tools/importcheck/rules.json`, and `check` for cycles:** not run.
- **PR2A-1 on origin/main:** not run; its origin is read from the diff.

The scratch worktree `s17/pr2-arch-wt` is removed. Nothing was committed, and the main checkout is unchanged.

## Re-check of the fix round (2026-11-25)

A fresh blind reviewer, the range 730fa8e..86aea25, mainly `drain.go`, `app.go`, `recorder.go` and `runtime.go` and their tests; 15 minutes. **No blockers.** Ids RCA-1 to RCA-3. Dispositions: [the owner's decisions](../sprints/decisions.md#owner-decisions-2026-11-25--pr-2s-re-check) and the sprint record's Findings.

### RCA-1 — after the last hold is released, the phase can read active before Activate has started the app's flushers and recorder (should-fix)

- **Where:** `drain.go:96-101`. `restorePhase` reads `a.started`, which the router sets at `transitions.go:92`. This contradicts `lifecycle.go:78-79` ("starts the app's sink flushers and … recorder goroutine …, then opens its input") and `lifecycle.go:123`.
- **Scenario:**
  1. Activate's router step runs (`started` true, reply sent).
  2. Before `startAppGoroutines`, a Drain with an expired ctx holds the phase, fails to enqueue and releases; `restorePhase` moves draining → inactive → active.
  3. A Send in this window returns nil and is queued; the router processes it with no flusher or recorder running.
  4. Once `startAppGoroutines` runs, everything is delivered.
  - Edges:
    - If Close gives up inside the window, Activate returns ErrClosed, yet Sends to the app already returned nil. Their outputs are never flushed.
    - A burst of more than RecordQueue records in the window ends the recording: no recorder runs, so `turn`'s yields free nothing.
- **Evidence:** a temporary test in a scratch worktree, `-race -count=1`: `phase inside hold (goroutines not started): 1 (active=1); Send in that window: <nil>`, then `sink got 1 events`.
- **Coordinator's check:**
  - The same sequence fails 3 of 3 at 86aea25 under `-race -count=3`: phase 1, Send `<nil>`.
  - At 730fa8e it passes 3 of 3: phase 0, Send "runtime: the app is not activated", phase 1 after Activate. The gap is new with cd459b7.

### RCA-2 — the recorder comments say the yields "return at once" and the pause is "bounded by two yields"; that holds per record, not per item, and not under load (later)

- **Where:** `recorder.go:71-76` and `:125-128`.
- `Gosched` puts the router behind every runnable goroutine, not only the recorder.
- At one processor, a runnable CPU-bound recorder can keep the processor up to the scheduler's time slice per yield.
- One router item that makes k records, such as a `TableWrite` of k rows (`router_in.go:118`), can yield 2k times. A recorder that frees one slot per turn never ends the recording, so at one CPU detection runs at its pace.
- The bookkeeping is race-free: `turns` and `queued` are written only on the router's paths, and the one reader of `turns` is a test, after `Close`.
- Reasoning only, not measured.

### RCA-3 — `holdDraining`'s comment says the wait on `runMu` is "bounded by another Drain's own deadline"; with a Drain that has no deadline it is unbounded (later)

- **Where:** `drain.go:62-63`.
- `awaitRuns` holds `runMu` while it waits for a run that never returns, so a second Drain's `holdDraining`, and any release, block past their own ctx.
- Mostly older than the fix. The phase is already draining, so input is refused meanwhile.
- Reasoning only.

### Checked and sound

- **Holds counting:**
  - each release follows one hold, at most once per Drain;
  - a Drain whose drain item is queued keeps its hold;
  - once `waited` is set, a release returns early;
  - holds never underflow.
- **The unlocked first `markDraining`** racing a last release opens only a brief window, never a lasting state.
- **`restorePhase` and `markDraining` use only CAS,** so a removed phase is never overwritten.
- **Every ordering of Activate against a failed Drain** walked. No case leaves a successfully activated app refusing input, or an app stuck draining with no holds.
- **Lock order:** Run takes `runMu`, then `inflight.mu`. `startAppGoroutines` and `stopRuntime` take `inflight.mu` alone. The router never takes `runMu`. No deadlock.
- **Docs:** the Drain paragraph, `Config.RecordQueue`'s doc and `build`'s comment match the code.
- **Tests:**
  - the two-turns test is deterministic, and the drain-holds tests force their interleavings;
  - `go test -race -count=3 -run 'Drain|Activate|RecorderGetsATurn|RecorderBlocked|StartingRows' ./internal/runtime`: ok.
- **Not reviewed:** the gate's Python tests, the playback test, and the benchmark file.
