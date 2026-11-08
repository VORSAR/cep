# S3.3-1 break-it (step 7): QA on the combined build

> **Historical — S3.3-1, the public Go API: `cep`, the connector contracts and the API gate; merged in PR #23.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-07--s33s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-3-1-design.md) · [record](../sprints/s3-3-1-record.md) · reviews: [design: breaker](s3-3-1-design-breaker.md), [design: challenger](s3-3-1-design-challenger.md), [design: architecture](s3-3-1-design-architecture.md), [design: QA](s3-3-1-design-qa.md), [combined build: architecture](s3-3-1-combined-architecture.md), **break-it: QA**. **← [S3.2c](../sprints/s3-2c-record.md) · [S3.3-2 (next)](../sprints/sprint-plan.md#status) →**

- Worktree: `/Users/ramith/code/cep-wt/s3-3-1-breakit`, branch `s3-3-1/breakit`, based on `1da314c` (integration head). Nothing pushed. No non-test file edited.
- Toolchain: Go 1.27.3 (pinned, `GOTOOLCHAIN=local`), Python 3.15.2 (pinned). Every exit code below was read from a completed run.
- Time used: about 25 minutes of the 45.
- GitNexus: not used (I did not read the `context` resource, so I have no `index.commit` to quote). I read the code directly.
- Hindsight: one search ("S3.3-1 public Go API facade lifecycle..."). It returned only general pages. No recorded decision was used.

## Commits (one per package, all red tests plus guards that hold, in the same file)

| commit | package | file |
|---|---|---|
| `d155433` | `cep` (root) | `/Users/ramith/code/cep-wt/s3-3-1-breakit/breakit_s3_3_1_test.go` |
| `11629a5` | `tools/apipartition` | `/Users/ramith/code/cep-wt/s3-3-1-breakit/tools/apipartition/breakit_s3_3_1_test.go` |
| `4b33ffa` | `internal/runtime` | `/Users/ramith/code/cep-wt/s3-3-1-breakit/internal/runtime/breakit_s3_3_1_test.go` |
| `2c19af0` | `io/inmemory` | `/Users/ramith/code/cep-wt/s3-3-1-breakit/io/inmemory/breakit_s3_3_1_test.go` |
| `573111a` | `gate/runner` (pytests) | `/Users/ramith/code/cep-wt/s3-3-1-breakit/gate/runner/tests/test_apidiff_breakit.py` |

The pre-commit hook (parity, gofmt, archcheck, ruff) passed on every commit with exit 0.

Proving runs, each exit 1 (red as intended):
- `go test -race -count=1 -run TestBreakit .`: 11 FAIL, 5 PASS (guards).
- `go test -count=1 -run 'TestBreakitTwoRemoves|TestBreakitActivateRaces' ./internal/runtime`: both FAIL in 3 of 3 runs, without `-race`.
- `go test -race -count=1 -run TestBreakit ./io/inmemory`: both FAIL.
- `go test -count=1 -run Breakit ./tools/apipartition`: 2 FAIL, 1 PASS (guard).
- `python3.15 -B -X utf8 -m unittest gate.runner.tests.test_apidiff_breakit`: 3 FAIL.

Counts: **6 blockers, 6 should-fix, 3 later.** 9 guards hold.

---

## Blockers

### B1. A second remove of one app waits until its deadline (two Undeploys at once; Close while Undeploy runs)
- **Where:** `internal/runtime/lifecycle.go:367` (`router.remove`: `if a == nil || a.ing == nil`) and `:414` (`installAck` clears `a.ing` only when every shard has acked).
- **What happens now:** two `Remove` calls on one drained app both pass the caller-side phase check (`:248-253`, phase is draining or inactive).
  - Both `iRemove` items reach the router before the first `installAck`, so both pass `:367`.
  - The second `installAck` (`:398-405`) pushes a second set of `last` markers into sink queues whose flushers already returned on the first set.
  - The second `Remove` then waits on them at `:269` until its ctx ends.
  - Facade effect: `AppHandle.Undeploy` called twice at once hangs one caller to its deadline. `Runtime.Close` racing an `Undeploy` hangs inside its own `Remove` (`lifecycle.go:306`) until its deadline, so the runtime cannot shut down.
  - This first showed up as a 60 s hang in a wider interleaving test. The goroutine dump had 15 `Remove`s parked at `lifecycle.go:268` and `Close.func1` parked in `Remove`.
- **Suggested fix (also fixes B2):** add a phase `phaseRemoving` between draining or inactive and removed.
  - In `Remove`, on the caller side, replace the `switch` at `:248` with a CAS loop:
    - `inactive → removing` or `draining → removing` succeeds: remember `was`.
    - `removing` or `removed`: return `ErrRemoved`.
    - `active`: return `ErrActive`.
  - Only the winner enqueues `iRemove`.
  - If `r.request` returns an error (including the router's `ErrDrainIncomplete`), restore `was` with `a.phase.CompareAndSwap(int32(phaseRemoving), int32(was))` before returning.
  - Every `switch appPhase(...)` that lists phases (`gate`, `Activate`, `Drain`, `Close`'s loop) must treat `phaseRemoving` like `phaseRemoved` for errors (`ErrRemoved`).
  - Alternative, router-only: add a router-owned `removing bool` on `appState`. Set it in `router.remove` before sending `mRemove`, and refuse with `ErrRemoved` when it is set. That fixes B1 but not B2.
- **Red tests:**
  - `internal/runtime/breakit_s3_3_1_test.go` `TestBreakitTwoRemovesAtOnce`, line 40: `iteration 0: Remove 0 of two at once: context deadline exceeded; want nil or ErrRemoved`.
  - `breakit_s3_3_1_test.go` (root) `TestBreakitConcurrentUndeployReturns`, line 312: `iteration 0: concurrent Undeploy 1 returned context deadline exceeded; want nil or ErrUndeployed (...)`.
  - `breakit_s3_3_1_test.go` (root) `TestBreakitCloseWhileUndeployReturns`, line 341: `iteration 1: Close while Undeploy ran: context deadline exceeded (Undeploy: <nil>); want nil`.

### B2. Activate and Remove of one inactive app both succeed, and race (Start racing Undeploy)
- **Where:** `internal/runtime/lifecycle.go:68` (Activate's CAS `inactive → active`), `:77-84` (stores `started`, starts the flushers over `a.all`, reads `a.rec` twice and runs `go a.rec.run(r)`), and `:413-414` (`installAck` stores `phaseRemoved` and sets `a.ing, a.dec, a.rec, a.sinks = nil`).
- **What happens now:**
  - `router.remove` reads `a.started == false` and proceeds. Activate's CAS still succeeds because the phase is stored as removed only later, at `:413`.
  - Both calls return nil: the app is "activated" and removed at once.
  - Flushers start for a removed app, with no last marker when `installAck` read `started` before `:77`. They exit only at `Close`.
  - The race detector reports `installAck` (`lifecycle.go:414`, write) against `Activate` (`lifecycle.go:82`, read of `a.rec`).
  - If `a.rec` is read non-nil at `:82` and nil at `:84`, `go (*recQ)(nil).run(r)` dereferences nil on an engine goroutine and the process crashes.
- **Suggested fix:** the `phaseRemoving` CAS of B1. Activate's CAS from inactive then fails once a Remove has begun, and `installAck` runs only after the CAS to removing, never concurrently with a successful Activate. Also read `a.rec` once in Activate: `if rec := a.rec; rec != nil { r.wg.Add(1); go rec.run(r) }`.
- **Red tests:**
  - `internal/runtime/breakit_s3_3_1_test.go` `TestBreakitActivateRacesRemove`, line 68: `iteration 0: Activate and Remove of one app both returned nil: the app was activated and removed at once`. Fails without `-race` too.
  - `breakit_s3_3_1_test.go` (root) `TestBreakitStartRacesUndeploy`: under `-race`, `testing.go:1867: race detected during execution of test`. The race report names `lifecycle.go:414` and `lifecycle.go:82`. Without `-race` it passes, because Start and Undeploy both returning nil is legal at the facade.

### B3. A ConnectorFactory that returns (nil, nil) panics Deploy (sink) or crashes the process at Start (source)
- **Where:** `deploy.go:126-130` then `:104` (`s.Accepts()` on a nil `cepio.Sink`), and `deploy.go:117-121` (a nil `cepio.Source` stored). Then `apphandle.go:55` runs `src.Open` on nil inside `internal/runtime/lifecycle.go:105`, an engine goroutine with no recover.
- **What happens now:**
  - A nil sink: `runtime error: invalid memory address or nil pointer dereference` on the caller's goroutine, inside `Deploy`.
  - A nil source: `Deploy` succeeds, and `Start` kills the process. I did not run that part: the test never calls `Start`.
- **Suggested fix:** in `bind`, right after each factory call:
  - `if s == nil { return nil, nil, fmt.Errorf("cep: deploy %s: stream %q: the connector factory returned no source and no error", name, stream) }`
  - The same for `Sink`, with "no sink".
- **Red tests:**
  - `breakit_s3_3_1_test.go` `TestBreakitFactoryNilSinkIsAnError`, line 74: `Deploy with a factory whose Sink returns (nil, nil) panicked: runtime error: invalid memory address or nil pointer dereference; want an error naming stream O`.
  - `TestBreakitFactoryNilSourceIsAnError`, line 95: `Deploy with a factory whose Source returns (nil, nil) succeeded; want an error naming stream S (Start would panic on an engine goroutine)`.

### B4. Reusing an inmemory.Source for a second Deploy crashes the process
- **Where:** `io/inmemory/source.go:96-97` (`s.opened.Store(true)`, `defer close(s.done)`).
- **What happens now:** a restart is a new Deploy (owner, design 1), and reusing the same `*inmemory.Source` is the natural way to write it. When the second `Open` returns, it closes `s.done` again and panics with `close of closed channel`. The panic happens on the engine's run goroutine (`internal/runtime/lifecycle.go:105`), which has no recover, so the process dies.
  - Verified through the facade with a scratch test, deleted and not committed: deploy, start and stop with `WithSource("S", src)`, then the same again. Output: `panic: close of closed channel ... inmemory.(*Source).Open ... source.go:116 ... cep.(*AppHandle).Start.func1 apphandle.go:55 ... runtime.(*Runtime).Run.func1 lifecycle.go:105`, `FAIL github.com/vorsar/cep`.
- **Suggested fix:**
  - `inmemory`: `if !s.opened.CompareAndSwap(false, true) { return errors.New("inmemory: the source was already opened: one Source feeds one Open; make a new Source for a new Deploy") }`, placed before `defer close(s.done)`.
  - Also document "one Open per Source" on `Source`.
  - **Owner question:** should `runtime.Run` (`lifecycle.go:102-106`) recover a panicking run into `rs.err = fmt.Errorf("runtime: app %d: run %d panicked: %v", …)`, so that it is reported in `StopReport.SourceErrors`? A panicking sink is already recovered and counted (`runtime.go:1467-1470`, `sinkQ.call`). A panicking source kills the process. Design §4.3 leaves only `fill`'s panic unrecovered, because it runs on the caller's goroutine. `Open` runs on the engine's.
- **Red test:** `io/inmemory/breakit_s3_3_1_test.go` `TestBreakitOpenTwice`, line 83: `the second Open panicked: close of closed channel; want it to return (an error naming the reuse, or nil)`.

### B5. WithTableRows passes values the engine refuses: Deploy succeeds and Start fails, leaving the app started with no rows and no sources
- **Where:** `deploy.go:160-170` (`t.rows(fills)` runs only the facade's index, type, null and unset checks) against `internal/runtime/runtime.go:664-673` (`TableWrite`'s `recordable` and `rowBound` checks, which run only at `Start`, from `apphandle.go:47`).
- **What happens now:**
  - Design §3.4 says "the rows are checked as `Table.Upsert` checks them, so a bad row fails the deploy, not the start".
  - A NaN, +Inf, malformed UTF-8, a non-compact or invalid object, or a year 10000 timestamp in `WithTableRows` deploys. `Start` then returns the engine's error after `Activate` succeeded.
  - Per §3.5 the app "stays started": its flushers and recorder run, it has no table rows and no sources, and the caller must `Stop` it.
- **Suggested fix:** export the engine's per-row check, used by both `TableWrite` and `bind`.
  - In `internal/runtime/runtime.go`: `func (r *Runtime) CheckTableRows(tables *table.Set, tableID int, rows [][]event.Value) error`, holding the loop now at `:664-673`. `TableWrite` calls it.
  - Or a package-level `func CheckTableRows(tb *table.Table, rows [][]event.Value) error`.
  - In `deploy.go`'s `bind`, after `t.rows(fills)`, call it against the plan's table. Any error fails the Deploy, wrapped as now with `WithTableRows(%q)`.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitWithTableRowsBadValueFailsTheDeploy`, line 174, six subtests. For example `Deploy accepted a starting row with NaN; Start then returned table T: row 0: attribute 1 "f": the float NaN is not finite; want Deploy to refuse it (design §3.4)`. Also `year 10000 ... is a timestamp outside the years 0000-9999 ...`.

### B6. Starting table rows are written in map order: two identical deploys record different bytes
- **Where:** `deploy.go:160` (`for table, fills := range o.tableRows`, a map), which feeds `h.tableRows` written in that order at `apphandle.go:46-50`.
- **What happens now:**
  - Design §3.5 step 2 says "for each table with starting rows, in table order".
  - With two tables, B's starting rows were recorded before A's in 22 of 24 identical deploys (19 of 24 on another run). The order changes from run to run.
  - Detection is not changed, since every table is loaded before any event. But the recording of the same input is not reproducible, which ground rule 11 treats as stop-the-line.
  - The same map pattern orders the Go-bound sources (`deploy.go:136`, `o.sources`) against §3.5's "the bound sources' stream order". That one is later (L3), because run order is concurrent anyway.
- **Suggested fix:**
  - Iterate tables by id: `for tid, t := range p.Tables { fills, ok := o.tableRows[t.Name]; if !ok { continue }; … }`.
  - Check unknown names first, with the names sorted for a stable error: `names := slices.Sorted(maps.Keys(o.tableRows)); for _, n := range names { if _, ok := h.tables[n]; !ok { return …ErrUnknownTable… } }`.
  - Do the same for `o.sources`: iterate `p.Streams` by id and look up `o.sources[name]`.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitTableRowsAreWrittenInTableOrder`, line 235: `in 22 of 24 identical deploys B's starting rows were recorded before A's; want A first every time (table order, design §3.5)`.

---

## Should-fix

### S1. An optional time attribute is accepted, and SetNull on it enters the event at timestamp 0 (1970)
- **Where:** `row.go:87` (`SetNull` is refused only for a non-optional attribute) and `input.go:39` (`e.Ts = w.values[h.timeAttr].T`, which is 0 for a null). The validator accepts `stream S (…, t timestamp optional) time t watermark …`. SPEC §4.2 requires only that the named attribute is `timestamp` (E-TIME-003).
- **What happens now:** `SendRow` with `SetNull(t)` is accepted. The event enters at unix 0 and produced one output.
- **Suggested fix:**
  - Facade, local: in `row.put`, after the optional check, add `if v.Null && attr == w.timeAttr { w.fail(fmt.Errorf("cep: the time attribute %q cannot be null", a.Name)); return }`.
  - Better, as a spec change for the owner: the validator refuses `optional` on a stream's time attribute (extend E-TIME-003, or add a new code).
  - It could be argued a blocker (wrong data enters). I rated it should-fix because the spec does not decide the question and only a program that marks its time attribute optional is exposed.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitNullTimeAttribute`, line 550: `SendRow with the time attribute SetNull was accepted (outputs 1); want an error naming t`.

### S2. A second Close returns the deadline, not nil, while a goroutine is still stuck
- **Where:** `internal/runtime/lifecycle.go:293` (`closeOnce`: the second call skips the body, so `first` stays nil) and `:323-333` (it waits on `r.wg` under ctx and returns `ctx.Err()`). This contradicts `runtime.go:21-22` in the facade ("A second Close returns nil") and design §3.7 ("nil (a second Close)").
- **What happens now:** a sink that ignores its ctx. The first Close returns DeadlineExceeded (correct). A second Close with the sink still stuck also returns DeadlineExceeded. After the sink returns, a third Close returns nil.
- **Suggested fix (owner choice; I recommend the first):**
  - Change the facade doc and §3.7 to: "a second Close waits for the goroutines the first left running until ctx's deadline: nil once they have returned, ctx's error otherwise". The behaviour is the truthful one.
  - Or keep the doc, and have the second call return nil at once.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitSecondCloseAfterAStuckSink`, line 280: `second Close with the sink still stuck: context deadline exceeded; want nil (a second Close returns nil, cep.Runtime.Close's doc and design §3.7)`.

### S3. apipartition: a const repeated by iota prints with no type or value; an exported alias of an unexported struct hides its fields
- **Where:** `tools/apipartition/main.go:200` (`vs := &ast.ValueSpec{Names: …, Type: s.Type}`, where an implicitly repeated spec has nil Type and no Values) and `:184` (unexported type specs are skipped, so a `type Public = hidden` alias carries none of `hidden`'s fields).
  - The golden shows the gap today: `api/exports.golden:126-180` has `cepio String const const String`, `cepio Changelog const const Changelog` and so on, against design §6.1's "every exported name with its type or value as written".
  - The cepio types' numbering is a contract: `row.go`'s `AttrType` depends on `event.TString == cepio.String`.
- **What happens now:**
  - Swapping two repeated names in an iota block changes their values and leaves the surface text, and so the apipartition gate, unchanged.
  - `apidiff` at T4 does report "value changed" as incompatible (`apidiff.go:285` in x/exp). But the row waives every incompatible change whenever `api/exports.golden` changed for any reason on the branch (`gate/runner/rows/apidiff.py:117`). So a value change bundled with any unrelated export addition passes both gates.
  - The same holds for a field-type change behind an exported alias of an unexported type.
- **Suggested fix:**
  - In `fileSurface`'s `*ast.GenDecl` case for `token.CONST`, carry the last explicit `Type` and `Values` across specs (`if s.Type == nil && len(s.Values) == 0 { use lastType, lastValues }`). Print each name with the inherited type and expression plus its iota position, e.g. `const B T = iota // iota 1`. This rewrites the golden: a law change for the owner to review.
  - For an exported alias whose target is an unexported identifier of the same package, refuse with exit 2, as embedded fields are refused: `type Public is an alias of the unexported hidden, whose fields this text cannot see: export the type or define Public`.
- **Red tests:**
  - `tools/apipartition/breakit_s3_3_1_test.go` `TestBreakitIotaRepetitionKeepsTypeAndValue`, line 31: `swapping B and C (B 1→2, C 2→1) left the surface unchanged: p A const const A T = iota / p B const const B / p C const const C ...`. Line 34: `want B printed with its type T (and its value), not a bare 'const B'`.
  - `TestBreakitAliasOfAnUnexportedTypeShowsItsFields`, line 46: `changing the exported field X of the type behind the exported alias Public left the surface unchanged: p Public type type Public = hidden`.

### S4. The apidiff row: a package removed at HEAD is a tool failure, red even with the golden changed
- **Where:** `gate/runner/rows/apidiff.py:96-99` handles a package absent at the merge base ("new package ...: compatible"). There is no counterpart for a package absent at HEAD. `:111-114` then reports `apidiff at HEAD failed with exit 1: loading ex.com/m/q: ... no required module provides package`.
- **What happens now:** a removed public package is red whatever `api/exports.golden` says, and the reason is a tool failure, not "removed".
- **Suggested fix:** before the HEAD run, add `if not has_package(pkg):` (relative to the current tree). Print `apidiff: {ip}: package removed at HEAD: incompatible`, count it as compared, and if `not golden_changed` append `f"incompatible API change in {ip}: the package was removed without {GOLDEN} changing"`. Then `continue`.
- **Red tests:** `gate/runner/tests/test_apidiff_breakit.py`:
  - `test_removed_package_with_golden_changed_passes`: `AssertionError: 'exit 1: ex.com/m/q: apidiff at HEAD failed with exit 1: loading ex.com/m/q: -: no required module provides package ex.com/m/q; ...' != 'covered 2'`.
  - `test_removed_package_without_golden_names_the_removal`: `'failed with exit' unexpectedly found in 'exit 1: ex.com/m/q: apidiff at HEAD failed with exit 1: ...'`.

### S5. A malformed WithParam literal is reported as a file without its version pragma
- **Where:** `internal/parser/literal.go:34` (`parseError(name, code, err)` maps the literal grammar's failure the way a file's is mapped).
- **What happens now:** `WithParam("threshold", "abc")` gives `E-PARAM-001 ... which is not a literal: literal:1:1: E-PRAG-001: the file does not start with the version pragma. Fix: add 'language cep/0.1;' as line 1.` This is misleading advice for an override.
- **Suggested fix:** in `ParseLiteral`, replace the `parseError` return with `mk(name, "E-SYN-001", <the token's position>, fmt.Sprintf("%q is not a literal: write a signed number, a string in quotes, a duration, a timestamp '…', true or false", text))`.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitWithParamNamesAndTypes`, line 620: `a malformed override's message: ... E-PRAG-001: the file does not start with the version pragma ...`.

### S6. inmemory.Source.Close waits forever behind a parked Push
- **Where:** `io/inmemory/source.go:46-56` (`push` holds `s.mu` while it waits on `s.ch <- it`) and `:80` (`Close` takes `s.mu`).
- **What happens now:** a `Push` on an unbuffered source with no `Open` yet, then `Close`. Close's doc says "Before Open it returns at once", and Push's doc says "ErrClosed after Close". Instead `Close` blocks until a pump takes the row, which with no `Open` is forever.
- **Severity:** it could be argued a blocker under "broken contract". I rated it should-fix because a `Push` with a ctx that ends is a workaround, and this is the programmatic or test connector.
- **Suggested fix:** do not hold the mutex across the send.
  - Add `quit chan struct{}`, closed by `Close`.
  - `push`: `s.mu.Lock(); if s.closed { unlock; return ErrClosed }; s.mu.Unlock(); select { case s.ch <- it: return nil; case <-s.quit: return ErrClosed; case <-ctx.Done(): return ctx.Err() }`.
  - `Close`: `s.once.Do(func(){ s.mu.Lock(); s.closed = true; s.mu.Unlock(); close(s.quit) })`.
  - `Open`: treat `<-s.quit` plus an empty `s.ch` as the end, instead of a closed `s.ch`. `s.ch` is never closed, so no send can panic.
- **Red test:** `io/inmemory/breakit_s3_3_1_test.go` `TestBreakitCloseUnblocksAParkedPush`, line 55: `Close with a Push parked and no Open did not return in 5 s; want it to return at once and the Push to get ErrClosed`. The test then frees both goroutines, so goleak stays clean.

---

## Later

### L1. A writer kept from a fill that panicked stays armed
- **Where:** `row.go:35-37` (`armed = false` runs only on a normal return).
- **What happens now:** after a panicking fill is recovered by the caller, a `Set` on the kept writer writes silently instead of panicking with `ErrRowUsedAfterFill`. The row is never sent, so nothing reaches the engine.
- **Fix:** `func (w *row) disarm() { w.armed = false }` and `defer w.disarm()` right after `w.armed = true`. An open-coded defer does not allocate, so MUST 13 is kept. Check with `TestSendRowAllocationsAboveSend`.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitWriterFromAPanickingFillIsDisarmed`, line 524: `Set on a writer kept from a fill that panicked: recovered <nil>; want a panic with ErrRowUsedAfterFill`.

### L2. Two malformed WithParam overrides come back in map order
- **Where:** `deploy.go:37` (`for pname, literal := range o.params`).
- **Fix:** `for _, pname := range slices.Sorted(maps.Keys(o.params)) { literal := o.params[pname]; … }`.
- **Red test:** `breakit_s3_3_1_test.go` `TestBreakitWithParamNamesAndTypes`, line 625: `two malformed overrides reported in a different order on deploy 3: ...`. It fired in 4 of 4 runs.

### L3. The apidiff row on a renamed module fails at the merge base on every run of the branch
- **Where:** `gate/runner/rows/apidiff.py:74` (`module = module_path()` reads HEAD's go.mod) and `:101-107` (`apidiff -w` at the merge base runs with HEAD's import path, which the base module lacks).
- **What happens now:** red with `ex.com/n/p: apidiff -w at the merge base failed with exit 1: loading ex.com/n/p: ...`. The golden names packages, not import paths, so it never changes on a rename, and the branch can never pass T4.
- **Fix:** read the base's module path from `<worktree>/go.mod`. If it differs, print `apidiff: module renamed from <old> to <new>: every import path changed: incompatible` and append that as a failure with no golden waiver. A module rename is then a stated red that the owner passes on purpose, not a tool failure.
- **Red test:** `test_apidiff_breakit.py` `test_renamed_module_is_compared_not_a_tool_failure`: `'at the merge base failed' unexpectedly found in 'exit 1: ex.com/n/p: apidiff -w at the merge base failed with exit 1: ...'`.

---

## What held (guards, committed green next to the reds)

- **The annotation's `${param}` reaches the factory, resolved and overridden** (`TestBreakitAnnotationParamIsOverridden`): `url = ${u}` arrives as `http://x`, and with `WithParam("u", "'http://y'")` as `http://y`.
- **WithParam's names and literal kinds** (`TestBreakitWithParamNamesAndTypes`, the parts other than S5 and L2):
  - An undeclared name (`threshhold`) is a DeployError.
  - A string literal for a float param is a DeployError.
  - `-0.5`, `1` for a float, `5 min`, `1 hour 30 min`, `'it\'s'` and `false` all deploy.
- **Recording replays the live run** (`TestBreakitRecordingWithBatchesTablesAndASourceReplays`). The setup:
  - a source emitting 3 batches of 4 rows, each closed by `EndBatch`;
  - `Input` with a 4th batch;
  - two starting table rows and a mid-run `Upsert`.

  Replaying the recording through `internal/playback` gives the live outputs byte for byte, and the same S counters: Batches 4, Late, Rejected and AfterDrain equal. Run 3 times under `-race`.
- **A recording writer that errors** (`TestBreakitRecordingWriterErrors`): `Recording.Failed` with "disk full", all 3 outputs delivered, `Undeploy` returns nil.
- **Deploying while another app drains** (`TestBreakitAddWhileAnotherAppDrains`):
  - App B deploys, starts, delivers and stops while A's `Stop` is parked on a sink that holds until its ctx ends.
  - A's `Stop` with a cancelled ctx returns `Canceled`, and A's `Undeploy` then returns nil, because the sink's ctx ends at the removal.
- **Boundary values** (`TestBreakitExtremeValuesCross`): `math.MinInt64` and `math.MaxInt64`, timestamps at year 0000 and at the last millisecond of 9999, and all optional attributes null cross `SendRow` unchanged.
- **apipartition on generics and tags** (`TestBreakitGenericTypeAndTagsArePrinted`): an exported generic type and its method print their type parameters. A struct tag change changes the surface.
- **No-model walk:** `TestHandleReachesNoModel` holds by its design. Its blind spots are func values (the plan's body closures hold the checked model, which design §3.2 accepts as the plan's) and pointers deduplicated by address alone. No defect found.
- **Boundary refusals through SendRow** (NaN, UTF-8, objects): already covered by QA's `TestSendRowChecks`, not repeated.

## Not tried in the time box
- `Undeploy` while a source is parked in `Emit` under backpressure.
- `Close` while a `Deploy` is in flight.
- A recording writer that blocks forever (it has no ctx to cancel it, as the sinks have).
- A source that ignores cancel and a sink that ignores ctx at once under `Close`. The runtime suite's `TestDrainWithARunThatIgnoresCancel` and `TestCloseHonoursTheDeadline` cover each alone.
- An apidiff merge base that lacks `api/exports.golden`. By reading `apidiff.py:92`, `git diff --name-only base -- golden` lists the added golden, so the first branch that adds it waives its incompatible changes. That is acceptable for a one-time introduction.
