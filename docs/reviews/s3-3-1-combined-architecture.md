# S3.3-1 — architecture review of the combined build (blind)

> **Historical — S3.3-1, the public Go API: `cep`, the connector contracts and the API gate; merged in PR #23.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-07--s33s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-3-1-design.md) · [record](../sprints/s3-3-1-record.md) · reviews: [design: breaker](s3-3-1-design-breaker.md), [design: challenger](s3-3-1-design-challenger.md), [design: architecture](s3-3-1-design-architecture.md), [design: QA](s3-3-1-design-qa.md), **combined build: architecture**, [break-it: QA](s3-3-1-breakit-qa.md). **← [S3.2c](../sprints/s3-2c-record.md) · [S3.3-2 (next)](../sprints/sprint-plan.md#status) →**

**Reviewed:** `a8a63ad..1da314c` in `/Users/ramith/code/cep-wt/s3-3-1-review` (read-only), against `docs/sprints/s3-3-1-design.md` v2 (with its "as built" and "corrected in the build" notes), `docs/sprints/decisions.md` (2026-11-07 sections) and `docs/sprints/s3-3-1-record.md`.
**Method:** source read directly (GitNexus not consulted: the time box went to the runtime and to the scratch runs; its freshness was not checked). Scratch copy: `/private/tmp/claude-501/-Users-ramith-code-cep/c32ea7c4-ab50-4044-8f33-e9ed338fa3bd/scratchpad/review/wt` (a `git archive` of HEAD), scratch test `wt/internal/runtime/zz_review_test.go`, output `review/race1.txt`. Toolchain: the pinned Go 1.27.3, `GOTOOLCHAIN=local`.
**Hindsight:** one search ("S3.3-1 public Go API lifecycle Add Remove Drain design decisions"). No page held a decision about these findings, so nothing from memory is used here.

**Counts:** 1 blocker, 5 should-fix, 5 later.

---

## Blocker

### B1. `Activate` and the router's never-started drain race on `a.rec` (a data race, reproduced under `-race`); the recorder goroutine then outlives `Remove`

**Where:**
- `internal/runtime/lifecycle.go:68-85` (`Activate`: the CAS, then `a.started.Store(true)`, then it reads `a.rec` at `:82` and starts `a.rec.run`)
- `internal/runtime/runtime.go:926-933` (router `iDrain`: `if !a.started.Load() { a.drained = true; a.rec = nil; … }`, the write at `:928`)
- `internal/runtime/lifecycle.go:148-153` (`Drain` reads `was` **before** its two CASes, so `revert()` can restore a phase that is out of date)

**Proof:** `TestReviewActivateRacesNeverStartedDrain` in the scratch copy runs 3,000 inactive apps, each with a Recorder, and calls `Activate` and `Drain` on two goroutines (the facade's `Start` and `Stop` called at the same time; `AppHandle` says it "is safe for concurrent use", `apphandle.go:13`). `go test -race -run TestReview ./internal/runtime/` exited **1** with `WARNING: DATA RACE`: a read at `lifecycle.go:82` by `Activate` and an earlier write at `runtime.go:928` by the router (`review/race1.txt:3-16`).

**Why it happens:** `Activate` wins the CAS from inactive to active. `Drain` then CASes active→draining. `Drain` reads `a.started` before `Activate` has stored it, so it skips `awaitRuns` and queues `iDrain`. The router also reads `started == false`, so it takes the never-started path and writes `a.rec = nil`. Meanwhile `Activate` reads `a.rec` and starts the recorder goroutine. After that:
- `Remove` passes the check at `lifecycle.go:371`, because `drained && drainRes != nil`. In `installAck`, `a.started` is now true but `a.rec` is nil (`:406`), so `a.rec.stop` is never closed. The recorder goroutine keeps running until `Close` closes `r.stopped`. That is a goroutine leak per app, which breaks MUST 1 (the router writes a field that a caller goroutine reads) and MUST 6's "returns on its last marker".
- The recording holds a header and no `stop`, and `Stats` reports it as `Records 0` (because `a.rec` is nil), so the truncation is never reported.
- Separately, `was` (`:148`) can say inactive while the CAS that actually succeeded was active→draining. If the enqueue then fails, `revert()` (`:152`) puts an *activated* app back to inactive, with its flushers already running.

**Code as it is** (`lifecycle.go:63-87`, `runtime.go:926-933`):
```go
	if !a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive)) { … }
	a.started.Store(true)
	for _, q := range a.all { r.wg.Add(1); go q.run(r) }
	if a.rec != nil { r.wg.Add(1); go a.rec.run(r) }
	return nil
```
```go
		if !a.started.Load() { // never activated
			a.drained = true
			a.rec = nil
			res := reply{}
			a.drainRes = &res
			it.reply <- res
			return
		}
```

**Proposed code.** The router orders an activation against a drain or a remove. "Activated" becomes a router-owned fact, set by an intake item. The router never writes `a.rec`.
```go
// runtime.go: itemKind gains
	iActivate // design s3-3-1 §2.4 as corrected (review B1): the router orders an activation against a drain or a remove

// appState gains (router-owned):
	activated  bool // set by iActivate; read only by the router
	recDropped bool // the router discarded the recording of a never-activated app (a.rec is never written after Add)

// lifecycle.go
func (r *Runtime) Activate(ctx context.Context, app AppID) error {
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if !a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive)) {
		switch appPhase(a.phase.Load()) {
		case phaseActive:
			return ErrStarted
		case phaseDraining:
			return ErrDrained
		}
		return ErrRemoved
	}
	it := item{kind: iActivate, app: app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		a.phase.CompareAndSwap(int32(phaseActive), int32(phaseInactive)) // nothing queued: still inactive
		return err
	}
	// The router answers at once (no shard round trip); its answer is waited for whatever ctx says, so the router's
	// view and the goroutines started here never disagree.
	var rep reply
	select {
	case rep = <-it.reply:
	case <-r.stopped:
		return ErrClosed
	}
	if rep.err != nil { // a Drain or a Remove reached the router first: nothing is started
		return rep.err
	}
	a.started.Store(true)
	for _, q := range a.all {
		r.wg.Add(1)
		go q.run(r)
	}
	if a.rec != nil {
		r.wg.Add(1)
		go a.rec.run(r)
	}
	return nil
}

// runtime.go, router ingest:
	case iActivate:
		a := rt.app(it.app)
		switch {
		case a == nil || a.ing == nil:
			it.reply <- reply{err: ErrRemoved}
		case a.drained:
			it.reply <- reply{err: ErrDrained}
		default:
			a.activated = true
			it.reply <- reply{}
		}
	…
	case iDrain:
		…
		if !a.activated { // never activated (MUST 8): nothing delivered, nothing recorded; Activate will refuse
			a.drained = true
			a.recDropped = true
			res := reply{}
			a.drainRes = &res
			it.reply <- res
			return
		}
```
In the same change:
- `rt.remove`, `lifecycle.go:371`: `if a.activated && !(a.drained && a.drainRes != nil)`.
- `installAck`, `:400`: `if a.activated {` … `if a.rec != nil && !a.recDropped { close(a.rec.stop); res.recMarker = a.rec.returned }`.
- `iStats`, `runtime.go:955`: `if a.recDropped || a.ing == nil { st.Recording = append(st.Recording, RecordingStats{}) } else { … a.rec.stats() }`.
- `Drain` takes `was` from the CAS that succeeded:
```go
	var was appPhase
	switch {
	case a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining)):
		was = phaseInactive
	case a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining)):
		was = phaseActive
	default:
		if appPhase(a.phase.Load()) == phaseRemoved {
			return DrainResult{}, ErrRemoved
		}
		was = phaseDraining // a repeated Drain: never reverted
	}
	revert := func() {
		if was != phaseDraining {
			a.phase.CompareAndSwap(int32(phaseDraining), int32(was))
		}
	}
	…
	if was == phaseActive || (was == phaseDraining && a.started.Load()) {
		runs, err = r.awaitRuns(ctx, a)
		…
	}
```
With the item in place, the router sees exactly one of two orders. If `iDrain` comes first, the app is drained, no markers are pushed, and `Activate` gets `ErrDrained` and starts nothing. If `iActivate` comes first, the barrier's markers wait for the flushers, which `Activate` starts right after the reply.

**Tests:**
- `TestActivateAndDrainAtOnceHaveNoRace` (`internal/runtime/lifecycle_test.go`, under `-race`): 3,000 apps, each with an `lcRec`. For each, `Activate` and `Drain` run on two goroutines, then `Remove`. Assertions: there is no race report; every `Remove` returns nil; after the loop, `goruntime.NumGoroutine()` is back to the value it had before the loop (no recorder goroutine is left); and for each app, either `Activate` returned `ErrDrained` and the double's log is empty, or `Activate` returned nil and the log ends with `stop`.
- `TestDrainRevertKeepsAnActivatedAppActive`: call `Activate` with its CAS forced first (a sequential call), then `Drain` with an already-expired ctx. Assert that `Drain` returns `context.DeadlineExceeded` and that `Send` then still returns nil (the app is still active, not inactive).

---

## Should-fix

### S1. A deployed, not yet started app is ticked: its recording queues ticks before `Start`, and its clocks move before its tables are loaded (reproduced)

**Where:** `internal/runtime/runtime.go:888`, `if a == nil || a.ing == nil || a.drained || a.quiet {`. An inactive app passes this check. Its routes are installed at `Add` (`lifecycle.go:427`), so `rt.tickApp` advances its ingest and `a.rec.put(rTick)` queues a record behind the header.
**Proof:** `TestReviewInactiveAppIsTicked` (scratch; `TickEvery: 1ms`; the app is left inactive for 50 ms, then `Activate` and `Drain`) failed with "recording of an app inactive for 50 ms: 52 calls, 50 ticks; first five [header tick tick tick tick]" (`review/race1.txt:39-40`).
**Why it matters:** design §2.4 says "Nothing accepts the app's input until Activate", and §3.5 (and ADR-0012 §3, the owner's design 2) says tables are loaded before sources. Today a deployed app's processing-time clock advances, and its recording grows (up to `RecordQueue`, after which the recording fails), before `Start` writes its starting rows. Until S4.1 no detection changes. Once absences exist, an app deployed at t0 and started at t0+10m would see 10 minutes of idle time before its first table row. That is the start-up mirror of S3.1-1's D3 ("a shutdown never fabricates an absence"). The design never mentions ticks for an inactive app, so this is unstated drift.
**Code as it is:**
```go
		for _, a := range r.appList() {
			if a == nil || a.ing == nil || a.drained || a.quiet {
				continue
			}
```
**Proposed** (with B1's router-owned flag; without B1, use `!a.started.Load()`):
```go
		for _, a := range r.appList() {
			if a == nil || a.ing == nil || a.drained || a.quiet || !a.activated { // an inactive app's clocks start at Activate
				continue
			}
```
**Test:** `TestInactiveAppIsNotTicked` (`internal/runtime/lifecycle_test.go`, no sleep). Use `New` with `TickEvery: 0`, then `Add` an app with an `lcRec`, call `r.Tick(ctx, rtAt(5))` three times, then `Activate`, one `Send` at `rtAt(1)`, and `Drain`. Assertion: the double's log is exactly `[header event stop]`. Raised as an owner question below, because it decides what a recording holds between `Deploy` and `Start`.

### S2. `Add` (and `Remove`) whose ctx ends after the item is queued leaves an orphan (code reading; not reproduced)

**Where:** `internal/runtime/lifecycle.go:53` (`r.request(ctx, item{kind: iAdd…})`) with `runtime.go:572-588`: `await` returns `ctx.Err()` while the router goes on installing the app. The id is consumed and the app stays installed, inactive, with its shard state and sink queues. The caller never learns its id. The facade's `Deploy` then calls `cancelSinks()` (`deploy.go:80`) and returns the error, and the orphan stays until `Close`. `Remove` has the same shape at `lifecycle.go:254-266`: if ctx ends before the reply, the router still removes the app (and stores phase removed, `:413`), but `onRemoved` is never called by `Remove`. A retried `Undeploy` returns `ErrUndeployed`, so a `Write` blocked on the sinks' context keeps its flusher until `Close` (§3.6 says the context ends at `Remove`).
**Code as it is:**
```go
	rep, err := r.request(ctx, item{kind: iAdd, add: &app})
	if err != nil {
		return 0, err
	}
	return rep.id, nil
```
**Proposed:** once the item is queued, wait for the router's answer, which is bounded (one shard round trip), and undo it if ctx has ended:
```go
	it := item{kind: iAdd, add: &app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		return 0, err
	}
	var rep reply
	select {
	case rep = <-it.reply:
	case <-r.stopped:
		return 0, ErrClosed
	}
	if rep.err != nil {
		return 0, rep.err
	}
	if err := ctx.Err(); err != nil { // the caller gave up while the shards built: undo, so no orphan is left
		_ = r.Remove(context.Background(), rep.id)
		return 0, err
	}
	return rep.id, nil
```
In `Remove`, replace `r.request(ctx, …)` with the same enqueue-then-wait-for-the-reply pattern, so that `phase.Store`, `cancelSrc` and `onRemoved` always run once the router has removed the app.
**Test:** `TestAddWhoseContextEndsLeavesNoApp` (`internal/runtime/lifecycle_test.go`). Use `Shards: 1` with a body whose `NewBody` blocks on a channel the test closes after ctx's cancel (the shard's `mAdd` is parked). `Add` returns `context.Canceled`. Then call `Stats`: `len(st.Streams[0]) == 2` and every row is zero (the slot is a tombstone). Then `Remove(ctx, 0)` returns `ErrRemoved`.

### S3. The fixture edit to `TestSendRowAllocationsAboveSend` weakens MUST 13: the success path is no longer measured

**Where:** `alloc_test.go:89-105` (edit named in the record, "measures the facade's own allocations exactly … instead of a difference"). The asserted figure is a `fill` that returns an error, so `SendRow` returns at `input.go:34-35` before building the event, calling `mapErr` and calling `runtime.Send`. MUST 13 (design §4.1) bounds a *sent* row: "the budget is `runtime.Send`'s own plus three". After this edit, an allocation added on the success path (a closure, a `fmt.Errorf`, an escaping `event.Event`) is invisible to the test. The `sendAllocs`/`rowAllocs` figures are only logged (`:88`). The drift the record cites came from measuring two *different* apps, and measuring both on one app removes it without dropping the success path.
**Proposed test** (an internal test, `package cep`, file `alloc_internal_test.go`; it imports no `io/*`, so §8's law holds):
```go
func TestSendRowAllocationsAboveSendSameApp(t *testing.T) {
	ctx := context.Background()
	r, err := New(Config{Shards: 1})
	if err != nil { t.Fatal(err) }
	defer r.Close(ctx)
	h, err := r.Deploy(ctx, "fwd", fwdSrc) // the same five-attribute forwarding program alloc_test.go deploys
	if err != nil { t.Fatal(err) }
	if err := h.Start(ctx); err != nil { t.Fatal(err) }
	in, _ := h.Input("S")
	shared := buildValues(in) // one []event.Value the fill below would write, reused: Send's own figure
	send := func() { _ = h.rt.Send(ctx, h.id, event.Event{StreamID: in.stream, Ts: shared[in.timeAttr].T, Values: shared}) }
	row := func() { _ = in.SendRow(ctx, "", fill) }
	for i := 0; i < 2000; i++ { send(); row() }
	s := testing.AllocsPerRun(5000, send)
	w := testing.AllocsPerRun(5000, row)
	t.Logf("Send %.2f, SendRow %.2f, above %.2f", s, w, w-s)
	if w-s > 3.0 { t.Errorf("SendRow allocates %.2f above Send on the same app; MUST 13 allows 3", w-s) }
}
```
Keep the existing `fillOnly` figure as a second assertion (`own <= 3`).

### S4. A latent race and nil-index panic: the removal tombstone writes fields that `SendPayload` and `Drain` read on the caller's goroutine

**Where:** `internal/runtime/lifecycle.go:414`, `a.ing, a.dec, a.rec, a.sinks, a.quiet = nil, nil, nil, nil, true` (router). Readers on caller goroutines, after their phase check: `boundary.go:110` (`a.dec[stream].Decode`, which panics with an index out of range on a nil slice), `boundary.go:124` (`a.rec != nil`), and `lifecycle.go:193` (`a.rec.fail(...)` when the recording's stop misses the deadline, while a concurrent `Undeploy`'s `Remove` nils it). The window is the time between `gate` and the read, during which a full `Drain` and `Remove` must complete. That is narrow, and `cep` never calls `SendPayload`, so this was not reproduced. It is the same class as B1 (MUST 1: "the router is the only goroutine that writes"; it may write only what only it reads).
**Code as it is:**
```go
		a.ing, a.dec, a.rec, a.sinks, a.quiet = nil, nil, nil, nil, true
```
**Proposed:**
```go
		// The tombstone drops only router-read state; a.dec and a.rec are read by caller goroutines past their phase
		// check (SendPayload, Drain's deadline path) and stay as built (review S4).
		a.ing, a.sinks, a.quiet = nil, nil, true
```
`iStats` (`runtime.go:955`) then guards `a.rec.stats()` with `if a.ing == nil { RecordingStats{} }`, keeping §3.7's "zero rows" for an undeployed app.
**Test:** `TestSendPayloadDuringRemoveHasNoRace` (`internal/runtime/lifecycle_test.go`, `-race`): 200 cycles. In each, one goroutine calls `SendPayload` in a loop until it gets `ErrDrained` or `ErrRemoved`, while a second goroutine runs `Drain` then `Remove`. Assertions: no race report, no panic, and every `SendPayload` error is `nil`, `ErrDrained` or `ErrRemoved`.

### S5. `Deploy`'s parameter diagnostics come out in map order (the `DeployError` text is non-deterministic)

**Where:** `deploy.go:37`, `for pname, literal := range o.params {`. With two bad overrides, the two `E-PARAM-001` diagnostics (both at 0:0) come out in random order. Design §3.1 says `DeployError.Diagnostics` is in "validate's order: line, column, code, message", and the messages differ by name, so the order is defined and the code does not follow it.
**Proposed:**
```go
	pnames := make([]string, 0, len(o.params))
	for pname := range o.params {
		pnames = append(pnames, pname)
	}
	slices.Sort(pnames) // the messages begin with the name: validate's order (line 0, col 0, code, message)
	for _, pname := range pnames {
		literal := o.params[pname]
		…
```
**Test:** `TestBadParamOverridesAreInOrder` (`deploy_test.go`): a program with params `a` and `b`, deployed 20 times with `WithParam("b", "@")` and `WithParam("a", "@")`. Assertion: every `DeployError.Error()` is identical and diagnostic 0 names `a`.

---

## Later

- **L1.** `Close` called a second time waits on `r.wg` under ctx (`lifecycle.go:322-332`). After a first `Close` that gave up on a stuck `Write`, the second call returns `ctx.Err()`, not §3.7's "nil (a second Close)". Proposed: return `first` (nil) at once when `closeOnce` has already run and the first `Close` returned; or annotate §3.7 "nil once every goroutine returned". Test: `TestSecondCloseAfterAStuckSink` asserts whichever text the owner keeps.
- **L2.** `Deploy` on a closed runtime parses and validates first (`deploy.go:20-78`). A program with a defect returns a `*DeployError`, not §3.7's `ErrClosed`. Proposed: the engine exports `func (r *Runtime) Closed() bool`, and `Deploy` returns `ErrClosed` before `parser.Parse`. Test: a `LifecycleTable` row deploying an invalid program on a closed runtime asserts `errors.Is(err, cep.ErrClosed)`.
- **L3.** `apipartition`'s Δ names an entry `<pkg>.<ident>` (`tools/apipartition/main.go`). `cep.AttrIndex` (and `AttrType`) exist twice in `api/exports.golden:3-6`, once on `*reader` and once on `*row`, so a change to one of them reads as ambiguous. Proposed: for a method, name it `<pkg>.<Recv>.<ident>` in the Δ line (the golden text is unchanged). Test: `TestDeltaMessage` gains a method case expecting `fixture.T.M`.
- **L4.** Design §6.4 says the allowlist names "ten graph-only modules". `tools/deps-allowlist.json:118` names seven, which matches the real graph: the scratch `go mod graph` (exit 0) adds exactly `x/exp, x/mod, x/sync, x/tools` plus those seven. Correct the design's text, not the file.
- **L5.** `Close` concurrent with `Deploy`/`Start` (`lifecycle.go:294-310`): an app added after `Close`'s snapshot is stopped by `close(r.stopped)` without a drain, and `Activate`'s `r.wg.Add` can run concurrently with `Close`'s `r.wg.Wait`, which `sync.WaitGroup` forbids once the counter reaches zero. This is caller misuse. With B1's `iActivate`, `Activate` refuses once `r.stopped` is closed. Document "Deploy and Start MUST NOT run concurrently with Close" on `Runtime.Close`.

---

## The contracts (check 1)

| MUST | Enforced at | Pinned by | Verdict |
|---|---|---|---|
| 1 router-only writes | `lifecycle.go:353-354` (copy-on-write `apps.Store`), `runtime.go:528-535` (`appOf` nil/removed → `ErrRemoved`) | `TestAddRemoveSendUnderLoadHasNoRace` | **broken by B1** (router writes `a.rec`, read by `Activate`); S4 latent |
| 2 dense ids, tombstone, zero rows | `lifecycle.go:411-414`, `runtime.go:594-596`, `:948-951`, shard `stats()` nil row | `TestRemovedAppKeepsItsSlot` | holds |
| 3 phase on caller side, `ErrClosed` first | `runtime.go:539-556` (`gate`), `appOf`'s `isClosed` first | `TestAppStateErrors` | holds |
| 4 Add no goroutine; Activate flushers+recorder; Run sources | `lifecycle.go:46-58`, `:78-85`, `:98-107` | `TestAddStartsNoGoroutine` | holds |
| 5 Close reverse order, goleak | `lifecycle.go:295-309` | `TestCloseStopsAppsInReverseAddOrder` | holds for sequential use (L5) |
| 6 queue never closed; last marker via the FIFO | `runtime.go` `sinkQ.run` `if it.last { return }`; `lifecycle.go:401-405` (`rt.push`) | `TestRemoveDeliversNothingAfterTheFlush`, `TestRemoveWhileOutputsAreInFlight` | holds for sinks; the recorder's stop is missed in B1's interleaving |
| 7 add rollback | `lifecycle.go:419-425`, shard `install` defer `uninstall` | `TestAddRollsBackWhenOneShardFails` | holds |
| 8 inactive drain at once, nothing recorded | `runtime.go:926-933` | `TestCloseWithAnInactiveApp` | holds sequentially; B1 under concurrency |
| 9 cancel, await, stuck count, no barrier, repeat | `lifecycle.go:199-238`, `:155-163` | `TestDrainCancelsTheSourcesBeforeTheBarrier`, `TestDrainWithARunThatIgnoresCancel` | holds (the `was` revert is B1) |
| 10 one stamp for Ingest and header | `lifecycle.go:350-352` → `newAppState` (`runtime.go:404-421`) | `TestAddedAppStartIsItsHeaders` | holds |
| 11 BatchEnd recorded, counted, refused, replayed | `runtime.go:845-855`, `recorder.go` `rBatchEnd`, `sync.go:84-91`, `lifecycle.go:114-132` | `TestRecorderRecordsBatchEnd`, `TestBatchEndRoundTrip`, `TestBatchEndReplaysToTheSameStats` | holds |
| MUST NOT sleep/poll | no `time.Sleep/After/Tick` outside `tick()` | `TestNoSleepOrPollingInRuntime` | holds |
| 12 no model in cep | `tools/importcheck/rules.json:21` (`""` row without `internal/model`) | `TestHandleReachesNoModel` + archcheck | holds |
| 13 ≤ 3 allocations above Send | `row.go` (header + values; the kinds are the mask) | `TestSendRowAllocationsAboveSend` | code holds (2.00); **test weakened, S3** |
| 14 engine owns values; Set after fill panics | `row.go` `run` (`armed = false`), `put` (`panic(ErrRowUsedAfterFill)`) | `TestSendRowValuesAreTheEnginesOnceSent`, `TestRowWriterKeptPastFillPanics` | holds |
| 15 check order | `row.go` `put`: armed → index → type → key-only → null; `run`: fill err → w.err → unset/time | `TestSendRowChecks` | holds |

**§3.7, cell by cell:** every cell matches the code (`mapErr`, `cep.go:49-65`, maps `ErrDrained→ErrStopped`, `ErrRemoved→ErrUndeployed`; `Undeploy`, `apphandle.go:84-93`, drains on `ErrActive`/`ErrDrainIncomplete`). Two exceptions: "runtime closed × Close: nil" (L1) and "runtime closed × Deploy: ErrClosed" for an invalid program (L2).

## Drift (check 2)

Every in-place correction the record lists is present in the design: own sentinels (§3.1), `cepio.Writer` (§3.1), the static counter-examples (§7), `Remove` cancelling sinks before awaiting (§3.6), example 01 (§3.4), the `apipartition` one-line reason (§6.1), the `apidiff` F2–F4 (§6.4). I found unstated drift in three places:
- ticks reach an inactive app (S1);
- `Runtime.ErrActive` and `ErrDrainIncomplete` are new engine errors that §2.1's `var` block does not list (`runtime.go:92-95`; §2.4 only quotes the texts). Polish: add them to §2.1;
- `recQ.stop` (a closed channel), not §2.4's "`rMarker` with `last`", ends the recorder at `Remove` (`recorder.go:122-131`). That is equivalent and correct, because every record is put before the close on the router goroutine, but the design text describes the other mechanism. Polish: correct §2.4.

I saw no unannotated ADR departure; the six named in §3.8 are in the diff's ADR files.

## Concurrency (check 3), beyond B1, S2, S4

- `iAdd`/`iRemove` under a token mirror the drain pattern. `mAdd` precedes any message for the app in each FIFO mailbox, and routes are installed only after every ack. So `sh.rules[m.app]` cannot be indexed past the end, and `process` tolerates a freed slot (`rules == nil`).
- The last sink markers travel through `rt.push`, behind earlier outputs. `recQ.stop` is closed after every `put` on the same goroutine, so the stop drain sees everything.
- Goroutine ownership: router, shards and ticker are started by `New` and stop on `r.stopped`. Flushers and the recorder are started by `Activate` and stop on the last marker, `stop` or `r.stopped`. Runs are started by `Run`, are cancelled by `srcCtx` at `Drain`/`Remove`/`Close`, and are awaited by `Drain`. `Close`'s waiter goroutine (`lifecycle.go:323`) outlives a `Close` that times out on a stuck `Write`. That is pre-existing and matches §2.1.
- `awaitRuns`' `select` on `rs.done`/`ctx.Done()` (`lifecycle.go:225-232`) picks at random when both are ready. A run that returned can be counted as stuck when ctx expired meanwhile. This is harmless because the barrier cannot be queued with an expired ctx anyway. If you touch it, poll `rs.done` first.

## The fixture edits (check 4)

All fit the real signatures or the design: on-arrival rules (as plan builds them), `NumGoroutine`, the live deadline for R1, the waits before `Stop` (MUST 3), one timestamp for eight emitters, `Canonical` as the whole line (ADR-0003/0005), example 01's annotation on a plain rule, example 09's one output per app, the invalid/ move. The one exception is `TestSendRowAllocationsAboveSend` (S3).

## The public surface (check 5)

- `api/exports.golden` (205 lines) and design §3–§5 list the same names, both ways. The `cep`, `cepio` and `inmemory` names all appear in both. The methods of the unexported `*row`, `*reader` and `*emitter` are listed by design (§6.1), and they are the interfaces' implementations.
- Nothing should be unexported.
- Evolution: S3.3-2's sandbox enters inside `Deploy` (§3.1's comment) with no signature change, or through a new `DeployOption`. S5.1's connectors bind through `ConnectorFactory`, `Source`/`Sink`/`Mapper` and `AttrType`, which are already present, so no frozen signature needs to change. One caveat: adding a method to `cepio.RowWriter`/`RowReader`/`Emitter` later is a breaking change for third-party implementations, and `apidiff` will say so.

## The gate (check 6)

- The three rows match §6.3: `apipartition` and `clockgolden` are T2–T4 and blocking; `apidiff` is T4, blocking and exclusive; `proofs` has `min_wants` 19 (`gate/registry.json`).
- `go.mod` adds the `tool` line and four pinned indirect requires.
- The allowlist names them, plus the seven graph-only modules that `go mod graph` actually shows (L4).
- I did not re-run the gate; the record's T3 at 64a03c0 is the coordinator's claim.

## Question for the owner

**Should an app that is deployed but not yet started see the engine's idle ticks?** Today it does (S1). Its clock moves forward from `Deploy`, and its recording collects one tick per tick interval until `Start`. Pros of stopping that: a recording starts at `Start` with the tables first, as you decided (design 2), and once absence patterns exist (S4.1), a slow `Start` can never invent "nothing happened for 10 minutes". Con: a one-line change to the runtime plus one test now. **Recommendation:** an app's clock starts at `Start`, so do not tick it before then.
