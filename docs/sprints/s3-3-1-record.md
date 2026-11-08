# S3.3-1 — sprint record (the full loop, with the design panel)

**Sprint:** S3.3-1 — the public Go API: `cep` (`Runtime`, `Deploy`, `AppHandle`), the connector contracts `io`, the `Table` API, the API gate · **Routine:** the full loop with the design panel (owner, 2026-11-07) · **Size:** about 8 SP, re-sized to about 9.5 after the panel · **Branch:** `s3-3-1/integration`
**What to build and how each item is proven:** [sprint plan — S3.3](sprint-plan.md#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze) (items 1, 2, 5 and the forwards on the facade), its "how it runs" note and the plan check's readings · **Process:** [00-sprint-model.md](00-sprint-model.md) · **Before:** [S3.2c](s3-2c-record.md)

## Plan and decisions

- **The plan check (owner, 2026-11-07):** five answers, three as recommended ([decisions](decisions.md#owner-decisions-2026-11-07--s33s-plan-check)):
  - **1:** S3.3 runs in two parts; this is S3.3-1 (items 1, 2, 5 and the forwards on the facade; about 8 points); S3.3-2 is the expect sandbox, the runner, the baselines and the clock freeze.
  - **2:** the export check and a clock-golden row are gate rows now; S1.1c's LAW Δ block lists them later.
  - **3 (≠ R):** `apidiff` runs at T4 as well as the export list; `golang.org/x/exp` is a new build-time dependency, approved.
  - **4:** an expect block's lists interleave by time (built in S3.3-2).
  - **5 (≠ R):** the full loop with the design panel.
- **Binding forwards on the card:** D3 (an `optional` or `object` primary-key column is a deploy error with a new code), D7 (an `object` partition key, likewise), D2 (the row boundary refuses a non-finite float), B5 (an object enters as its JSON text), L2 (`Deploy` never mutates the checked model), D4 (recording off unless turned on); S3.2a-1's D1 (`Stop`'s drain is `clock.StreamRegistry.Drain`).
- **The design (owner, 2026-11-07, after the panel):** seven answers, six as recommended ([decisions](decisions.md#owner-decisions-2026-11-07--s33-1s-design-after-the-panel)): a stopped app stays stopped (F5 annotated); `WithTableRows`; the destination retries, never the engine (ADR-0008 §2 annotated); `OutEvent.Canonical` a fresh copy per output (≠ R); an unbuilt connector refused unless Go binds one; the batch-close signal recorded now; `apidiff` red only when the golden did not change. The panel: [breaker](../reviews/s3-3-1-design-breaker.md) (2 blockers, 10 should-fix, 2 later), [challenger](../reviews/s3-3-1-design-challenger.md) (1, 4, 3), [architect with the-fool](../reviews/s3-3-1-design-architecture.md) (2, 15, 7), [QA on testability](../reviews/s3-3-1-design-qa.md) (1, 10, 4); every finding verified by the coordinator against the source and mapped to [design v2](s3-3-1-design.md) §12. Re-sized at about 9.5 points.
- **Exits (from the card):** 3, two apps in one runtime share no state and `Runtime.Close` stops them in reverse deploy order; 4, two `in` predicates on one table within one event see one snapshot under a concurrent `Table.Upsert`; 5, an unlisted export turns `apipartition` red and `apidiff` reports a planted signature change; and the owner's check, a 20-line Go program embedding the engine. Exits 1, 2, 6, 7 are S3.3-2's.

## Scope changes

None.

## Measured results

- **The build** ([design v2](s3-3-1-design.md) §10, steps 1–8, one commit each on `s3-3-1/integration`): the stubs (adcea0f), the runtime's per-app lifecycle (2891a14; the existing runtime and playback suites green under `-race` on the first run, one guard widened: `TestBkGuardAfterCloseAndLeaks` accepts `ErrDrained` and `ErrRemoved` during `Close`, as §2.7 foresaw), the two deploy errors (dc6bed9), the facade, the contracts and `inmemory` (740ae18; a scratch run of example 09 end to end under `-race` before any test), `tools/apipartition` and the two goldens (1e8f4a4), the gate rows, `apidiff` as a go tool, the allowlist and the ADR annotations (b925cf2; each new row run alone through the gate runner: `apipartition` covered 3, `clockgolden` covered 1, `apidiff` covered 3, exit 0 each).
- **QA's tests-first lane** (six commits, red against the stubs: 19 runtime tests, 35 facade tests, 6 `inmemory`, 1 playback, 3 validator programs; `io`'s one test green by design): cherry-picked at 57b3cc1..c71da4b. **First full run** (`go test -race ./...`): 14 packages green, 13 tests red → the fixes of 15ff549 (the caller-side gate refusing a draining app, which also ended two crashes in QA's tests; a post-drain `BatchEnd` counted in `AfterDrain`; a removed app's zero rows; the row's allocations; `inmemory`'s `Close` waiting for the pump and `Drain` deep-copying). **Second run**: 6 runtime and 6 facade tests red → the fixes of d3fd195 (`SendPayload` through the gate; a `Drain` that queued nothing leaves the app live, keeping break-it R1; a repeated `Drain` not waiting again; `Remove` ending the sinks' context before awaiting the flushers; `Undeploy` of a stopped app without a second `Stop`). **Third run** (`go test -race ./...` at d3fd195): **all 17 packages green**, exit 0.
- **Fixture edits to QA's tests, each named here** (the lane's tests are QA's; the coordinator edits them only to fit the real signatures or the design, with the reason in the commit): `lcGoroutines(t, ctx, 0)` spun to the ceiling → `goruntime.NumGoroutine()`; `lcApp`'s and the snapshot test's hand-made rules are on-arrival, as plan's plain rules are (a single event otherwise waits in the release buffer); the reverse-order runtime test keeps buffered rules so the barrier's order is observable; the stuck-run test uses a live 200 ms deadline (an expired context queues nothing and leaves the app live, R1) and releases its run once; `TestRemovedAppKeepsItsSlot` expects the app's two stream rows; two facade tests wait for their sources' emits before stopping (a row emitted after `Stop` began is refused, MUST 3); the concurrent-emit test uses one timestamp (interleaved times from eight goroutines are stream-late on a bounded watermark); `TestInmemorySinkCopiesCanonical` and `Example_embed` read `Canonical` as the whole canonical line (ADR-0003/0005), not the attrs object; `TestUnbuiltConnectorAnnotation` carries example 01's annotation on a plain-rule program (example 01's pattern body is not built before S4.1); `TestCloseStopsAppsInReverseDeployOrder` expects example 09's one rate-limited output per app and reads the reverse order from the sources' cancellation; the two static-code programs leave `docs/language/invalid/` (parse-level counter-examples only); `TestSendRowAllocationsAboveSend` measures the facade's own allocations exactly (a fill that returns an error after writing: the row is built, nothing is sent — 2.00, budget 3) instead of a difference between two different apps whose engine-side work drifted by one between runs. **The build's own test:** `coverage_s3_3_1_test.go` (`BuildInfo`, `DeployError.Error`, the options' nil checks, a custom `ConnectorFactory` through `@source`/`@sink`, bool, duration and object values across the boundary, `EndBatch`, `Undeploy` of a never-started app) — the coverage row's floor: 88.55% after QA's tests, 96.0% with it; and `io/inmemory/coverage_s3_3_1_test.go` (the sink's per-type copies, the source's refusals, its context checks and its emit-error path) — 80.59% after QA's tests, 94.0% with it.
- **Design corrections found by the build** (v2 annotated in place): the facade's error sentinels are its own, mapped from the engine's (`archcheck`'s globals rule); `cepio.Writer` stands in for `io.Writer` (the facade may not import `io`); the static codes' counter-examples are the validator's test programs; `Remove` ends the sinks' context before awaiting the flushers, and `Undeploy` removes a stopped app without a second `Stop` (§3.6); example 01 cannot deploy before S4.1 (§3.4).
- **T2 (`make check`):** the first run (at d3fd195) red on three rows — `ci-fresh` (the workflow not regenerated for the new rows), `unit` (the allocation test's cross-app difference), `coverage-inclusion` (the facade at 88.55%) — fixed in 51269f3; the second run (at 51269f3) every row PASS but `coverage-inclusion` (`io/inmemory` at 80.59%), fixed in e2970f3; `unit` and `coverage-inclusion` re-run together at e2970f3: PASS, exit 0 (the only change since the second run, a test file in `io/inmemory`). Gate wall about 99 s. · **T3:** *(pending: after QA's second lane is cherry-picked)* · **T4:** *(pending)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S3.3-1 — the public Go API · routine: the full loop with the design panel (owner) · SP about 8 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-07) ☑ dependencies merged, main green (S3.2c's PRs #21, #22; main a8a63ad)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner ☑ test material: QA's tests first
Design (G1)  ☑ the design note (v1 ce272a4; v2 build-ready) ☑ the panel's three reports ☑ QA's testability review ☑ owner decisions recorded
Build (G2)   ☑ T2 ☐ T3 ☑ tests on the real code (go test -race ./... green, 17 packages)
Review (G3)  ☐ architecture review of the combined build ☐ QA break-it ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ the fix round under the race detector ☐ T4 exit 0 ☐ owner's check ☐ exits 3, 4, 5 met by their proofs
Exit (G5)    ☐ one PR, merged by the owner ☐ main CI green ☐ record + retro ☐ known issues updated
             ☐ status line ☐ documents reconciled ☐ memory updated ☐ worktrees removed ☐ points and duration
```

## For the owner (the owner's check)

*(written from the sprint's diff before the PR)*

## Retro

*(at close-out)*
