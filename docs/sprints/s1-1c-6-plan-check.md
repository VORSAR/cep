# S1.1c-6 — plan check (v2, 2026-11-15; after the blind architecture and QA reviews; for the owner)

*Step 0 of the full loop ([00-sprint-model.md §6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ 4d2b9eb. Every code claim cites `file:line` on that commit; "measured" marks a run made for this plan (the pinned golangci-lint v2.15.0 and Go 1.27.4 under both `GOOS`; coverage profiles of the 12 packages); "inferred" marks a reading not proven by a run. "PA-" and "PQ-" mark the blind reviews of v1 ([architecture](../reviews/s1-1c-6-plan-check-architecture.md), [QA](../reviews/s1-1c-6-plan-check-qa.md)); every claim of theirs that changes the plan was verified by the coordinator. Two scope answers were taken while drafting ([owner, 2026-11-15](decisions.md#owner-decisions-2026-11-15--s11c-6s-scope-before-its-plan-check)); nothing else here is an owner decision until the owner answers §11.*

```
Sprint S1.1c-6 — the strict lint set: the size limits in every other package, all 139 findings, no exclusion left
routine: full (no semantics panel) · SP ≈ 13, confirmed or re-cut by the spike before G0 (§1) · cap 2 weeks · one PR
Entry (G0)   ☐ card complete and approved (§12) ☑ dependencies merged, main green (S1.1c-5, PR #47; close-out #48; main 4d2b9eb)
             ☐ fits the cap, lanes ≤ 8 days (the spike, §1) ☐ test material: the nets are each lane's first item (§7)
             ☐ `gate/current-sprint` set to S1.1c-6 as the first commit after this plan check's
             ☑ reviewed by the architect ☑ reviewed by QA ☐ answered by the owner (§11)
```
*At G0 the full done-done block ([§13](00-sprint-model.md#13-done-done-the-exit-checklist)) is copied into the sprint record, `s1-1c-6-record.md`.*

## 0. What the code says that the plan does not (read first)

1. **The 139 are exactly as capped, in 110 functions, 8 files and 2 signatures** (measured). Under `linux/amd64` and `darwin/arm64` alike: gocyclo 110, funlen 19, file length 8, argument limit 2, matching the 31 TEMP entries' caps (sum 139) package by package. The only other findings are the 11 `fmt.Print` hits in package-main tools that the row drops (`gate/runner/rows/smells.py:8-9`).
   - **By package:** `internal/runtime` 22, `internal/validate` 22, `tools/archcheck` 21, `internal/playback` 17, `internal/event` 15, `internal/expr` 9, `internal/plan` 8, root `.` 7, `internal/parser` 7, `tools/apipartition` 6, `internal/partition` 3, `internal/model` 2.
   - **The heavy ones (cyclomatic):** `parser.parseError` 59 (`internal/parser/parse.go:121`), `(*router).ingest` 48 (`internal/runtime/runtime.go:895`, 145 lines), `event.parseRFC3339` 43 (`internal/event/decode.go:817`), `runtime.validateApp` 34 (`runtime.go:476`), `(*checker).selects` 34 (`internal/validate/exprs.go:251`), `(*arith).Eval` 33 (`internal/expr/nodes.go:291`), `(*Runtime).bind` 33 (`deploy.go:121`), `(*builder).node` 32 (`internal/expr/expr.go:168`), `(*checker).partition` 31 (`internal/validate/bounds.go:30`), `(*checker).graph` 30 (`internal/validate/graph.go:15`).
   - **The files over 500 lines:** `tools/archcheck/main.go` 1755, `internal/runtime/runtime.go` 1564, `internal/event/decode.go` 936, `internal/expr/nodes.go` 710, `internal/parser/lower.go` 583, `internal/partition/partition.go` 539, `internal/runtime/lifecycle.go` 525, `internal/playback/reader.go` 525.
   - **The two six-parameter functions:** `(*router).broadcast` (`runtime.go:1116`), `plan.expect` (`internal/plan/expect.go:75`).
2. **From the G0 marker commit, T4, the nightly tier and the `proofs` row are red on every branch until the last size entry goes** (PQ-B1; verified). Part rule 4 reds each entry at T4 and nightly once the marker reaches its forward (`gate/runner/parts.py:125-130`; `RED_EXPIRED`, `gate/runner/allowlists.py:34`); all 31 forward to S1.1c-6. `test_aging_planted.py:44` copies the real `smells.json` and the real marker into its T4 world, so its five cases go red with it (`:101`, `:122-130`). This is S1.1c-5's deadline working as designed (its design §3.5 rule 4: "a part's own branch is red at T4 from its G0 until the package is clean"). It collides with sprint model §10's `proofs` run after a marker move, and with any lane-branch run of a nightly row through `--tier nightly` (which pulls in `aging`). → §7 (the run schedule), Q6.
3. **Decision 6 needs a performance net before each hot-path split; the tree has two benchmarks and no per-function pins on these paths.** `git grep '^func Benchmark'`: `BenchmarkRuleClockFireGroup`, `BenchmarkTableCheckValue` (lane D's), archcheck's `BenchmarkX`. 23 test files use `testing.AllocsPerRun`; the runtime's live path has only `TestHandoffAllocationBudget` (`internal/runtime/s3_2b_2_alloc_test.go:17-45`), a `runtime.MemStats` ceiling per event (3.1 one shard, 8.5 four shards; measured 3.00, 8.1), an amortised average that cannot pin equality (PQ-S1). 38 of the 110 functions are on a per-event, per-payload, per-recorded-line or per-replayed-line path (§3; PA-S6). Five of them (`route`, `ingest`, `take`, `(*shard).process`, `(*recQ).run`) run on the router, shard or recorder goroutine and cannot be benchmarked alone without hand-built internals (PA-S7, PQ-S8). → §3, §7, Q3.
4. **Many functions to be split are not fully covered, and the gate's coverage row counts in-package tests only.** `coverage-inclusion` reads the `unit` row's profile, which has no `-coverpkg` (`gate/runner/rows/unit.py:23`), floor 90 per package (`gate/coverage/main.py:33`) (PQ-S2). In-package (measured): 59 of the 110 below 100 % of statements, 25 below 90 %; `tools/apipartition` reads 0 % in-process (its tests run the built binary). Over all tests (PQ-S4): `event.CleanLen` 87.0 %, `playback.sameValue` 66.7 %, `(*Reader).rejectedRecord` 59.1 %, `(*Recorder).Table` 70.0 %, `partition.keyOf` 90.0 %, `(*recQ).run` 73.9 % (its unreached blocks drain records queued when `stop` or `r.stopped` fires, `internal/runtime/recorder.go:169, 188-194`), `Remove` 80.8 %, `ingest` 85.7 %. The packages' margins over the floor: playback 90.4, parser 91.8, runtime 92.3, partition 92.9, model 93.9, event 94.2, plan 95.4, root 95.6, validate 97.5, expr 97.8. A split that extracts unreached error returns can push playback under 90. → §7, Q4.
5. **The lifecycle code holds two open defects and real concurrency.** `Drain` (23), `Close` (20), `Remove` (16), `Activate` (12), `awaitRuns` (11), `(*router).installAck` (11) in `internal/runtime/lifecycle.go`; `(*recQ).run` (12) in `internal/runtime/recorder.go`. Known issues S3.3-1-3 (forward S8) and S3.3-2-1 (forward S5.1) live in this code ([known-issues.md](../product/known-issues.md)). `race` runs at T4 only, `race-stress` at nightly only (`gate/registry.json`). → §6, §7, Q7.
6. **A cross-package move has no proof today and may need an import edge the law forbids** (PA-B1; verified). `astequal` keys declarations by directory and package, so a move is unequal by design (`tools/astequal/main.go:8-13`; its test `TestMovedToAnotherPackageUnequal`). The import rules are exhaustive law (`tools/importcheck/rules.json:4`): `internal/validate` may import event, model, expr, window and metrics only (`:17`); runtime may not import validate or playback; playback imports runtime. Lanes branch from the integration branch and meet only at combine, so a move between two lanes' packages has no owner. → §1 (the moves step), §2, §5.3.
7. **An intermediate split is red at the commit hook.** Over-cap is red at every tier, T0 included (`gate/runner/rows/smells.py:396-401`, outside the tier branch). A cyclomatic-30 function split into two of 15 raises its package's count, so the commit is refused. Every commit leaves each (package, rule) count at or below its cap. → §9.
8. **A cleaned package is red at T1 and T2 until its entry goes, so a lane cannot reach T2 alone** (PA-S2). "`<pkg> <rule>` waives nothing … remove it" on a full-tree run (`rows/smells.py:386-393`). One file holds every package's entries; the three size issues span lanes: S1.1c-5-1 runtime and validate; -2 event, parser, plan, playback; -3 root, expr, model, partition, apipartition, archcheck (PA-S1). → §1 (the lane's hand-back), §4.
9. **Proof drivers anchor on code in files this sprint splits** (PA-S3; verified). `test_fuzz_decoder_planted.py:20, :34` asserts `func (d *Decoder) Decode(` appears once in `internal/event/decode.go` ("the plant's anchor moved"); `test_fuzz_parser_planted.py:18-19` anchors `func Parse(` in `internal/parser/parse.go`; `test_api_planted.py:18-21` anchors `NewStreamRegistry` in the clock and `(*AppHandle).Start` in `apphandle.go`. These worlds copy every tracked file under `internal/`, and the hook worlds copy `tools/archcheck/**` and `tools/allowlists/**` (`support.py:498-508`), so lane commits and the coordinator's removals each trigger §10's `proofs` run (PQ-S5). → §5.1 (pinned placements), §7.
10. **The gate's law names lane A's files as instrument code.** `gate/law.json:313-315` (`archcheck`) and `:677-679` (`testhygiene`) name `tools/archcheck/main.go`; `:488-492` (`apipartition`) and `:514-518` (`clockgolden`) name `tools/apipartition/main.go` (PA-S4). A split's new files must join those `code` lists. Law paths also sit inside lane directories (`gate/law.json:165-217`: `internal/runtime/nosleep_test.go`, `internal/validate/docsync_test.go`, `internal/expr/docsync_test.go`, the parser's `errorcounts_test.go`, `keywords_test.go`, `fragment*_test.go`, `parseerror_contract_test.go`; PA-L2). → §4.
11. **`test_aging.py:520-569` encodes the clock point that printing S1.1c-7 moves** (PA-S12, PQ-S6). The test reads the real `gate/sprints.json` and pins `["S1.1c-5", "S1.1c-6"]` (`:540`), `closing = {"S1.1c-5", "S1.1c-6", "S4.1"}` (`:541`), the marker `"S4.2"`, its name `test_at_s4_1s_close…` (`:528`), its docstring (`:521-526`) and a witness (`:565-569`); aging rule 1 counts closed rows after `first_seen` (`gate/runner/aging.py:45-54`, two allowed). `:544` pins S1.1c-5-1…4 present in the real `gate/known-issues.json`. Row `S1.1c` stays open until S1.1c-7 closes (PA-L3). → §4.
12. **When the last entry goes, the exclusion machinery guards an empty list.** Rule 5(b) reds any new size entry after S1.1c-5 (`parts.py:166-167`), and `forwards` returns early with no entries (`:131-132`). → Q5.
13. **One PR holds at most 100 commits; two PRs would leave `main` red.** S1.1c-5 used 96. A first PR merged with the marker at S1.1c-6 and entries left would put them red at nightly on `main` (finding 2), against ground rule 11. → §1 (the budget), Q8.
14. **No production file is sprint-named; there is no `init()`, no `//go:` directive and no build-tagged production file in the 12 packages** (PQ counter-example 11, measured). The only call-initialised package variable is `internal/parser/lexer.go:31`. 64 of the 201 Go test files are sprint-named (S1.1c-7). Per-line `//nolint` waivers sit in the production code that moves: model 26, partition 7, parser 2, playback 2, runtime 2 (e.g. `runtime.go:1320`), expr 1, plan 1, root 1 (PQ-S3); `astequal` does not see comments, the `smells` row does.
15. **Placement candidates** (PA focus 3; the design lists them, the owner decides each at G1):
    - **The literal-to-value rule is written three times:** `validate.fits`/`as` (`internal/validate/resolve.go:57-80`), `plan.value` (`internal/plan/expect.go:181-204`, one of the 110; "as validate.as"), `expr.literalNode` (`internal/expr/nodes.go:20-37`). All three may import `internal/model`; one `model` function is a candidate. They differ (`plan.value` takes a string for `TObject`), so each caller's error text must stay.
    - **The recording line's size arithmetic sits in runtime, the encoder in playback** (`internal/runtime/boundary.go:65-67, :88`; `CheckRows`, `runtime.go:683`, used by `deploy.go:221`; `MaxLogLine` in `internal/event/decode.go:20`). None of the 110: backlog unless the owner wants it now.
    - **`tools/archcheck/main.go` holds two instruments** (`archcheck`, `testhygiene`); the hygiene evaluator (around `:804-1560`) is a self-contained concern. A new package would change the registry rows' commands (a gate-row change), so: files inside package main.
    - **Not candidates:** the two `validateApp` (`runtime.go:476` checks `runtime.App` from `runtime.FromPlan`; `internal/playback/reader.go:151` checks `playback.App`; `validate` may not import either, and they are preconditions between packages, not a program's E-codes; PA-S5). `Deploy` and `bind` (`deploy.go:24, :121`): only the root may import the parser, and ADR-0005 §4 places the expects run there.
16. **No gate row changes in this sprint** (if Q5 is answered as recommended), so the retro's smoke rule (sprint model §10) has no subject; the coordinator's edits are law data, each read by `proofs` and T4.

## 1. Routine, shape, waves

**Full loop without the semantics panel.** No rule changes, but the work rewrites the runtime's concurrency, the event decoder, expression evaluation and the deploy-time validator ([§5](00-sprint-model.md#5-choosing-the-routine-at-the-plan-check): "concurrency; … anything that changes what a rule detects"); the loop proves nothing changed. **One PR** (§0.13).

**Before G0 — the spike** (ground rule 8: time-boxed, findings only, no production code; PA-S15). In a scratch worktree off `main`, one agent splits `(*router).ingest` (48) and `event.parseRFC3339` (43) to the limits behind their nets, 30 minutes each, and reports the time per finding at that weight and the nets' cost. The coordinator projects the sprint from it. "Fits the cap" is ticked only on that projection; if it does not fit, the owner re-cuts before G0.

**The order (at most two agents at once; a queue — the next starts when one finishes):**

| Step | Who | What |
|---|---|---|
| G0 | coordinator | The law commit: the marker `S1.1c-6`; S1.1c-7 printed after S1.1c-6 in `gate/sprints.json` and the plan's table; `test_aging.py:520-569` re-derived in the same commit (§4); the record opened. Then `--job proofs`, expected red per Q6. |
| Design | coordinator; then the architect reviewer and QA blind; then the owner | The design note (§5). **G1.** |
| W0 | python-engineer (one agent) | The benchmark comparer (Q3), red first, with its tests. `astequal`'s rename map (PA-B1), red first, only if the owner approves a move at G1. |
| Moves | coordinator, on the integration branch | Each approved cross-package move, one commit each, before any lane branches (§2). |
| Lanes | go-engineer per sub-lane | R (`internal/runtime`: R1 nets, R2 moves and the hot-path splits, R3 lifecycle and the rest) and V (`internal/validate`: V1 nets and moves, V2 splits) first, per decision 2 of S1.1c-5's plan check; then E (`internal/event`) and X (`internal/expr`, `plan`, `partition`, `model`); then P (`internal/playback`) and T (root, `internal/parser`); A last (`tools/archcheck`: A1, A2; `tools/apipartition` with A2), behind its proofs. |
| Measure | coordinator, alone (no gate running) | After each lane: the benchmark comparisons of Q3 for its leaf functions (PA-S8). |
| Combine | coordinator | Cherry-picks; each issue closed when its last lane is in (§4); T3; the review and break-it; one fix round; the nightly job once; the path comparison `main` vs combined (Q3); T4; the owner's check. |

**Each sub-lane, in its own worktree:** (1) its nets, committed before any split (§7); (2) pure file moves to the design's file map, each `astequal` exit 0; (3) splits by concern, one commit per source file or concern, each at or below every cap (§0.7); (4) **hand-back** (PA-S2): 0 size findings on its own packages under the pinned golangci-lint for both `GOOS`, its packages' `go test` green, every commit through T0, its report. Then the coordinator commits the removal of the lane's own `smells.json` entries on the lane's branch and runs T2 and the lane's verifiers itself, reading the `exit:` line.

**Commit budget (line by line, PA-S10):** plan check v2 with its reviews 1 · the owner's answers 1 · G0 1 · design v1 1, its reviews and v2 1, G1's answers 1 · W0 2 (+2 if a move is approved) · moves ≤ 3 · lanes 7 × ≤ 8 (nets 2, file moves 1, splits ≤ 4, removal 1) = 56 · combine: issue closures 3, the docs rows of §4 1, `law.json` code lists 1 · the fix round ≤ 10 · the record, T4 and G5 docs 3. **At most 89; slack 11.** The coordinator counts at each combine; if the projection passes 95, a lane's split commits (which no `astequal` proves) are squashed on its branch before cherry-pick.

## 2. Seams

- **Cross-package moves (PA-B1).** None is made in a lane. Each approved move is a coordinator commit on the integration branch **before the first lane branches**, so every lane starts from a tree that holds it. A move commit changes only the declaration's package and its callers' qualifiers, with its tests moved with it into the receiving package (PQ-S2). Its proof: `astequal` with a declared rename map (`old-package.key=new-package.key`; W0, red first) over source and destination, plus T2. A move that needs a new edge in `tools/importcheck/rules.json` changes ARCHITECTURE §3 and is its own owner question at G1.
- **The coordinator's commits on lane branches** (the lane's own `smells.json` entries) are a known seam, re-resolved at combine; the coordinator owns the file.
- **No other seam.** A lane edits only its own packages; a change that would need another package's file stops the lane and comes back to the coordinator.
- **The coordinator's shared files** (one owner, every wave): `tools/allowlists/smells.json`, `gate/known-issues.json`, `docs/product/known-issues.md`, `gate/law.json`, `gate/registry.json`, `gate/sprints.json`, `gate/current-sprint`, `gate/proofs/drivers/*.py`, `gate/runner/tests/test_aging.py`, `tools/importcheck/rules.json` (only on an owner answer), the docs.

## 3. Packages and the hot-path list

No new rule. The design's file map (§5.1) names every new file. **The hot-path list: 38 functions** (verified against callers by the architect; PA-S6):

| Path | Entry point | Functions in the 110 | Net (Q3) |
|---|---|---|---|
| Live event | the facade's `SendRow`/`Emit`/`Upsert`/`Delete` (one `row` each), `(*Runtime).Send`, `SendPayload` | root `(*row).run`, `(*row).put`; runtime `Send`, `SendPayload`, `route`, `routeKey`, `ingest`, `take`, `(*shard).process`, `broadcast` | path benchmarks at `SendRow`, `Send`, `SendPayload`; leaf benchmarks for `routeKey`, `(*row).put` |
| Rule evaluation | `(*body).OnEvent` | plan `(*body).OnEvent`; expr `(*compare).Eval`, `(*arith).Eval`, `(*cast).Eval`; partition `keyOf`, `(*Rule).Offer` | leaf benchmarks for the three `Eval`s, `keyOf`; `OnEvent`, `Offer` under the `Send` path |
| Payload decode | `(*Decoder).Decode` per payload; `SendPayload` | event `(*Decoder).value`, `(*scanner).top`, `value`, `object`, `str`, `number`, `parseRFC3339`, `appendUnescaped`, `cleanObject`, `CleanLen` (`internal/runtime/boundary.go:119`; PA-L1), `compactAndValid` | leaf benchmarks per function, inputs chosen to reach each |
| Recording | the recorder per record | playback `AppendValue`, `appendString`, `appendEvent`, `(*Recorder).Table`, `(*Trace).Step` (tracing only); runtime `(*recQ).run` | leaf benchmarks for the encoders; a path benchmark recording one event |
| Replay | `(*Reader)` per line | playback `(*Reader).readLine`, `record`, `eventRecord`, `tableRecord`, `rejectedRecord` | a path benchmark replaying one line of each kind |

The rest (deploy-time validation, parsing, plan building, lifecycle, the gate tools) is not on a hot path; its nets are behaviour only.

## 4. Ownership — one owner per file per wave

| Lane | Owns (production and tests; law paths inside are read-only, PA-L2) |
|---|---|
| **R** | `internal/runtime/**` |
| **V** | `internal/validate/**` |
| **E** | `internal/event/**` |
| **X** | `internal/expr/**`, `internal/plan/**`, `internal/partition/**`, `internal/model/**` |
| **P** | `internal/playback/**` |
| **T** | the root package's `*.go`, `internal/parser/**` |
| **A** | `tools/archcheck/**`, `tools/apipartition/**` |
| **W0** | the benchmark comparer and its tests (path fixed by the design; Python under the pinned interpreter); `tools/astequal/**` if a move is approved |
| **Coordinator** | the files of §2, and: **G0** — the marker; S1.1c-7 printed; `test_aging.py:520-569` re-derived (ids `["S1.1c-5","S1.1c-6","S1.1c-7"]`, `closing` `{"S1.1c-5","S1.1c-6","S1.1c-7"}`, marker `"S4.1"`, the test renamed for S1.1c-7's close, its docstring; the witness shown red first in a scratch copy). **On each lane's branch** — the removal of that lane's own entries. **At combine** — an issue closed (its `known-issues.json` entry, its table row, `test_aging.py:544` reduced) once its last lane is in; a split archcheck's or apipartition's new files added to the `code` lists of §0.10; the S3.3-1-3 row's pointer to `iBatchEnd` and ADR-0010 §3's `decode.go` if they move (PA-S11). **At G5** — only `S1.1c-6` closed in `gate/sprints.json` (row `S1.1c` closes with S1.1c-7). |

## 5. The design note (before code)

Written by the coordinator after the owner answers this plan check, reviewed blind by the architect reviewer (with the-fool) and the QA agent, decided by the owner at G1. Per package:

1. **The file map.** Every production file after the sprint, named for one concern, with the declarations it holds and its **projected size after the splits** (at most about 450 lines when moved; PA-L4). **Pinned placements** that do not move (§0.9): `(*Decoder).Decode` in `internal/event/decode.go`, `Parse` in `internal/parser/parse.go`, `(*AppHandle).Start` in `apphandle.go`, `NewStreamRegistry` in `internal/clock/clock.go`; moving one instead is a coordinator commit to its driver. `tools/apipartition` stays in `main.go` (404 lines) unless a split would pass 500.
2. **The split plan per function.** For each of the 110: the helpers by name and concern, under S1.1c-5 design §7.3's rules (split by concern, each function named for what it does, no `xPart1..n`, no option struct whose only job is to carry more than five values; an existing domain type may be passed). For each, **the order of checks it keeps, down to which helper returns first** (§6.1). **No handler tables on the hot-path list**; elsewhere a table is a package-level var never written outside its declaration (`globalscheck`, `tools/archcheck/main.go:766-792`) or a method set on an existing type, and never refers back to itself (an initialisation cycle) (PA-S13).
3. **The placement list (owner, today, decision 2).** Every function in the wrong package (starting from §0.15), where it belongs, the internal API it changes, its import edges against `rules.json`, the tests that move with it, and its proof (§2). Each is an owner question at G1; without a yes the function stays.
4. **The nets per package** (§7): the hot-path list with each benchmark's name, input and path, the escape diff's scope, and, for Q4, every block left short with its reason.
5. **Each lane's hand-back state** (§1) and its exact verifier commands (§7).

## 6. Counter-examples (the break-it tries these first; each is a test or a net)

1. **The order of checks.** `validate` and `runtime.validateApp` return the first error found; grouping checks can change which error a two-fault input names. → conformance (E-codes, positions) and a two-fault case per split checker where none exists.
2. **Side effects across an early return.** A counter before a `return` that moves into a helper whose return no longer exits the caller. → the stats tests; `ingest`'s and `process`'s counters.
3. **`break` and `continue` in a moved loop body**, including **`break` after a `select` inside a `for`**: `recQ.run`'s overflow drain exits the `for` only because its `break` follows the `select` (`internal/runtime/recorder.go:177-186`); moved into `default:` or a helper, it loops forever. → an in-package overflow-drain test, stressed under load (PQ 1).
4. **`defer`, `recover` and named results.** Defer order (`recQ.run` runs `settle`, then `close(q.returned)`, then `wg.Done`, `recorder.go:158-160`); a `recover` moved away from the code it guards (partition's F10, cited at `runtime.go:1318-1319`); `err :=` shadowing `install`'s named `err`, which the deferred `uninstall` reads (`runtime.go:1301-1306`). → the lifecycle and drain tests, partition's F10 tests, a later-rule-fails case (PQ 2, 5, 9).
5. **`select` priority.** One multi-case `select` split into sequential checks changes which ready channel wins (`stop`, `overflow`, `r.stopped`). → Remove, Close and overflow tests; `race-stress` (PQ 3).
6. **Lock scope and happens-before.** A mutex held across moved code, or an atomic `Store` (`draining`, `phase`, `started`, `runtime.go:188-200`) moved across a channel send. → `race`; the lifecycle tests at `-cpu 1,4`; `race-stress` (PQ 10).
7. **Goroutines and captured variables.** → `race`; the goroutine tests (`internal/runtime/goroutines_s3_3_2_test.go`).
8. **Short-circuit lost.** `x && f()` rewritten so `f` runs when `x` is false. → a characterisation case per operand (Q4; PQ 4).
9. **Allocation, escape and inlining.** A closure, an interface conversion, a `&local` or a broken inline budget on a hot path. → `AllocsPerRun` equality and the `-gcflags=-m=2` escape diff (Q3; PQ 7).
10. **Error identity and text.** A rebuilt `*event.DecodeError`, a new `%w`, a moved `fmt.Errorf` prefix. → the decoder's error-position tests; `FuzzDecode`'s oracle (`internal/event/fuzz_test.go:12`); the golden error tests; `fragments` (`internal/parser/parseerror_contract_test.go`); `docsync` (PQ 6).
11. **Map iteration made visible.** → determinism, the baselines.
12. **A per-line waiver left behind.** A `//nolint:forbidigo // PERMANENT …` must stay on its panic's line (§0.14). → the `smells` row (an issue silenced from another line is red, `rows/smells.py:10-11`); `suppressions` for the metadata (PQ 8).
13. **A pinned placement moved** (§0.9). → `proofs` at lane done for E, T and P.
14. **Coverage moved out of its package.** A characterisation test written in another package, or a moved function whose tests stay behind. → the brief's rule; `coverage-inclusion` (PQ 12).
15. **The two known issues' behaviour.** S3.3-1-3 and S3.3-2-1 are not fixed in passing (a fix is a behaviour change with no red test first). → the break-it checks their behaviour before and after.

## 7. Test plan per lane (the rig proves the line)

**Every lane, before any split (its nets commit):**
- **Coverage (Q4):** each function to be split at 100 % of its blocks from **its own package's tests** (the gate's measure), characterisation tests first where short, named for the behaviour, in the owning package; a case per operand of each compound condition; one hand mutation shown red per new test and per control transfer the split will move across a helper boundary, quoted with its diff; any block left short is the design's, with its reason. A new timing- or concurrency-sensitive test is stressed under load before it merges (sprint model §10).
- **Performance (Q3), the hot-path list only:** the leaf benchmarks and their `AllocsPerRun` pins (equality), and the path benchmarks of §3. The runtime's MemStats ceiling stays a ceiling; its one-shard budget is tightened to equality only if stable over 20 runs (PQ-S1). The benchmark files are not edited after the nets commit (`git diff <nets>..HEAD -- <bench files>` empty).

**Every lane, at hand-back:** §1's state; `astequal <c>^ <c>` exit 0 for each in-package move commit (PQ-S3).

**The coordinator, after the lane's removal commit:** T2 exit 0; the lane's verifiers:

| Lane | Acceptance → rig | Verifiers beyond T2 |
|---|---|---|
| R | runtime behaviour unchanged → unit, conformance and baseline (both also at T2, PQ-L1), the boundary, tripwire and goroutine tests; lifecycle → `race` | `go test -race -count=3 ./internal/runtime/...`; the nightly job alone (`--tier nightly --job nightly`, which leaves out `aging`; inferred, confirmed at G0) |
| V | the same E-codes, positions and messages → conformance, `docsync-b-catalog`, the negative tests | — |
| E | the same values and errors → the decoder tests, the alloc pins | the nightly job alone (`fuzz-decoder`); `--job proofs` (§0.9) |
| X | the same values, errors and partitioning → expr, plan, partition tests; the canonical encoding → model's golden and parity tests | — |
| P | the same recordings and replays → the baselines, the corpus test, `determinism` (darwin locally; linux in CI, PQ-L2) | `--job proofs` |
| T | the same deploy and parse errors → `fragments`, `docsync-d`, the parser goldens | the nightly job alone (`fuzz-parser`); `--job proofs` |
| A | archcheck and apipartition judge the same → their tests; the planted proofs | `--job proofs` |

**The `proofs` and T4 schedule (§0.2, Q6):** `--job proofs` after G0, after lanes E, T, P and A, and on the combined branch before T3. Every run before the last lane's removal is red by construction; it is recorded as **red as expected** only when its failed lines are exactly `aging`'s "has reached its part S1.1c-6" lines for the entries still present and `test_aging_planted.py`'s five cases, quoted in the record — never reported green. The first green `proofs` run follows the last removal.

**The combined branch:** T3; the nightly job once; the path benchmarks `main` vs combined under Q3's rule (PA-S7); T4 (`smells`, `aging`, `conformance`, `baseline`, `determinism`, `differential`, `race`, `apidiff` with no public change, `coverage-inclusion` ≥ 90 per package, `proofs`).

## 8. Risks and order

1. **Complexity moved, not removed.** *Mitigation:* the design's split plan per function, reviewed before code; the combined architecture review against it.
2. **A lifecycle regression only load shows.** *Mitigation:* §6.3–6.7; `race -count=3` and the nightly job on lane R; no known-issue fix in passing.
3. **A function that resists a safe split and is not on a hot path** (`Drain` 23, `Close` 20; PA-S9). *Mitigation:* Q7.
4. **Benchmark noise deciding a function's shape.** *Mitigation:* Q3's deterministic allocation and escape checks first; the time rule from prebuilt binaries, measured alone (PA-S8).
5. **A gate tool blinded by its own refactor.** *Mitigation:* lane A last, behind `proofs`.
6. **Red-by-construction runs read as noise, or worse, as green.** *Mitigation:* Q6's exact expected lines, quoted.
7. **The size.** *Mitigation:* the spike before G0; sub-lanes for R, V and A.
8. **The 100-commit cap.** *Mitigation:* §1's budget, counted at each combine.
9. **Agents that drag or leave shells open.** *Mitigation:* a 30-minute box per sub-lane; no open-ended waits; the coordinator stops leftover shells after each lane.

**Order:** the owner's answers → the spike → G0 → the design → G1 → W0 → the moves → lanes (R, V; E, X; P, T; A) → combine → T3 → review and break-it → one fix round → the nightly job → the path comparison → T4 → the owner's check.

## 9. Lane briefs (skeletons; the coordinator adds paths, worktree and the report file)

**Common to all:** your own worktree and branch from the integration branch; the pinned toolchain first; write only your owned files (§4), never a law path inside them; never edit `smells.json`, `known-issues.json` or a proof driver; commit early, one commit per source file or concern, each leaving every (package, rule) count at or below its cap; never `--no-verify`; a moved test unchanged, a new test named for what it tests and placed in the package that owns the code, never named for a sprint; keep each per-line `//nolint` on its line; never push or merge; no Claude attribution anywhere; never call anything an owner decision unless the owner made it; no open-ended waits (`tail -f`, a sleep loop, `cat` on stdin); a 30-minute box, with a wrap-up report when it runs out; never run a gate tier or a benchmark comparison while another agent runs one (the coordinator measures); **hand-back** = §1's state; your final message gives the commits, the coverage before and after, each mutation shown red, each `astequal` exit and the open items.

**Sub-lane R2 — the runtime's hot path** (go-engineer). *Goal:* `ingest`, `take`, `(*shard).process`, `route`, `routeKey`, `Send`, `SendPayload`, `broadcast` below the limits, `runtime.go` moved to the design's file map. *Read:* the design's runtime section; ADR-0010 §4; MUST 16; §6.2–6.9. *Order:* file moves → `ingest` → `take` → `process` → the rest. *Accept:* §7 row R; the path benchmarks' pins green. *Not in this lane:* lifecycle (R3); either lifecycle known issue; any exported identifier.

The other sub-lanes follow the same shape with their §7 rows.

## 10. Reviews of this plan check

The [architecture review](../reviews/s1-1c-6-plan-check-architecture.md) of v1 found 1 blocker, 15 should-fix and 6 later; the [QA review](../reviews/s1-1c-6-plan-check-qa.md) found 2 blockers, 8 should-fix and 3 later. Each claim below that changed the plan was verified by the coordinator.

| Finding | Severity | Disposition |
|---|---|---|
| PA-B1 a cross-package move has no proof | blocker | §0.6, §1 (W0, the moves step), §2: coordinator moves before the lanes, `astequal` rename map red first, a new import edge an owner question; card exit 2 |
| PQ-B1 T4, nightly and `proofs` red from G0 | blocker | §0.2, §7 (the schedule, exact expected lines), Q6 |
| PQ-B2 exit 1's proof cannot fail | blocker | §12 exit 1 rewritten (T4 `aging` and the `smells` summary) |
| PA-S1 / PQ-S5 shared files across lanes; issues span lanes; `proofs` triggers | should-fix | §0.8, §2, §4: lane branches remove their own entries; issues close at combine; §7's schedule |
| PA-S2 a lane cannot reach T2 alone | should-fix | §1: the hand-back; the coordinator runs T2 |
| PA-S3 drivers anchor on code that moves | should-fix | §0.9, §5.1 pinned placements, §6.13, §7 |
| PA-S4 apipartition and clockgolden code lists | should-fix | §0.10, §4, §5.1 |
| PA-S5 `validateApp` misframed | should-fix | §0.15: not a candidate; real candidates listed |
| PA-S6 the hot path mislabelled, 38 not 33 | should-fix | §0.3, §3 |
| PA-S7 / PQ-S8 per-function nets for goroutine loops; compounding loss | should-fix | §3 (path and leaf), §7, Q3, card exit 3 |
| PA-S8 benchmarks under contention | should-fix | §1 (Measure), §9, Q3 |
| PA-S9 no exit for a function that resists a split | should-fix | §8.3, Q7 |
| PA-S10 the budget; two PRs | should-fix | §0.13, §1's line-by-line budget; two PRs rejected (`main` red), Q8 |
| PA-S11 / PQ-L3 docs naming moved code | should-fix | §4 (combine); live docs name files, not lines (measured) |
| PA-S12 / PQ-S6 `test_aging.py:520-569` | should-fix | §0.11, §4 (G0, re-derived, red first) |
| PA-S13 handler tables | should-fix | §5.2 |
| PA-S14 "no exclusion" vs kept-whole waivers | should-fix | §12 goal and exit 1 |
| PA-S15 the size unmeasured | should-fix | §1 the spike before G0; sub-lanes |
| PQ-S1 the runtime pin is a budget; 23 files | should-fix | §0.3, §7 |
| PQ-S2 coverage in-package; playback's margin | should-fix | §0.4, §2 (tests move with code), §7, Q4 |
| PQ-S3 `astequal`'s scope; waivers | should-fix | §0.14, §6.12, §7 |
| PQ-S4 Q4 infeasible as written | should-fix | Q4 changed |
| PQ-S7 Q3's tool and threshold | should-fix | W0 (the comparer), Q3 changed |
| PA-L1 `CleanLen`'s caller | later | §3 fixed |
| PA-L2 law paths in lane directories | later | §4, §9 |
| PA-L3 row `S1.1c` closes with S1.1c-7 | later | §4 (G5) |
| PA-L4 projected file sizes | later | §5.1 |
| PA-L5 Q5's alternatives misordered | later | Q5 reordered; the recommendation kept (the same end state, and this sprint stays free of gate-row changes) |
| PA-L6 23 files | later | §0.3 |
| PQ-L1 conformance and baseline at T2 | later | §7 |
| PQ-L2 determinism on the Mac is darwin only | later | §7 row P |
| PQ's acceptance rewrites | — | taken into §12, except "`astequal main HEAD` naming exactly the keys": it names only the first differing key (S1.1c-5 design §7.4), so the file-map diff and the review prove placement instead |
| PQ's counter-examples 1–12 | — | §6 (3, 4, 5, 6, 8, 9, 10, 12, 14) and §0.14 (11) |

## 11. Questions for the owner (recommendation first; options ranked by code health)

**Answered while drafting ([owner, 2026-11-15](decisions.md#owner-decisions-2026-11-15--s11c-6s-scope-before-its-plan-check)):** Q1, the sprint-named test files → their own part, S1.1c-7; Q2, logic in the wrong package → moved, each move approved at G1.

- **Q3 — when a split "costs" on a hot path (decision 6).** → **Allocations decide first, exactly: allocations equal (`AllocsPerRun` pins and `-benchmem`) and no new heap escape in the function (`-gcflags=-m=2` diff). Time second: two prebuilt test binaries, ten interleaved pairs at `-test.cpu=1 -test.benchtime=1s`, measured by the coordinator alone; a split costs time only if the slowdown is both significant (Mann–Whitney, p < 0.05, a small stdlib-Python comparer with tests, W0) and larger than 2 % on the median, re-run once before it stands. Applied to each leaf function at its split, and to each path `main` vs combined before T4.** Allocation and escape are where a split really costs and both are exact; the two-part time rule ignores noise and ignores real but trivial shifts. Others: the same with QA's stricter interval (p < 0.01, the shift's 95 % range wholly above +3 %); the median alone, 20 pairs, 3 %; lane D's overlapping ranges.
- **Q4 — how much coverage before a split.** → **Each function to be split at 100 % of its blocks from its own package's tests; a case per operand of each compound condition; one mutation shown red per new test and per control transfer the split moves across a helper boundary; any block left short listed in the design with its reason (an invariant panic, or reachable only by scheduling and covered by a named stress test), reviewed at G1.** It proves every branch a split touches, closes statement coverage's blind spot, and keeps the gate's own measure honest. Others: 100 % over all tests (`-coverpkg`); hot-path and lifecycle functions only; the package floor only.
- **Q5 — the exclusion machinery once the list is empty.** → **Keep it dormant through S1.1c-6, then retire it in S1.1c-7**: `smells.json`, the `smells` allowlist rule, `parts.py`, the row's cap and liveness steps and their planted cases, DEVELOPMENT.md and ADR-0015 §2.2's annotation, so a size finding has no path but a fix or a waiver under decision 6. S1.1c-7 changes the gate anyway (its naming check), so the retro's smoke rule runs once, and this sprint keeps no gate-row change. Others: retire it in S1.1c-6 (the same end state, but a gate-row change and new proof worlds mid-sprint); keep it forever.
- **Q6 — the runs that are red by construction (§0.2).** → **Run every `proofs`, nightly and T4 run when the rules say; before the last lane's removal, record each as "red as expected" only when its failed lines are exactly `aging`'s "has reached its part S1.1c-6" lines for the entries still present and `test_aging_planted.py`'s five cases, quoted; never call one green. The first green run follows the last removal.** It keeps §10's rule and ground rule 12 both. Others: rework the aging driver so its real-tree case does not read the live list (a gate change that weakens that case); skip the §10 runs until the last lane (against the rule).
- **Q7 — a function that resists a safe split and is not on a hot path** (`Drain`, `Close`). → **Split it anyway behind extra nets (the lifecycle tests at `-cpu 1,4`, `race -count=3`, the nightly `race-stress`, the known issues' behaviour pinned); if the lane still cannot, it stops and you decide then.** Then, in order: you move that issue's forward to S1.1c-7, with `LAST_PART` in the same commit (a gate change); a PERMANENT waiver outside decision 6 (against ADR-0015 §2.2).
- **Q8 — confirms.** The hot-path list of §3 (38), with path benchmarks for the goroutine loops and leaf benchmarks elsewhere. The spike before G0, and sub-lanes for R, V and A. One PR with §1's budget (two PRs would leave entries red on `main`). The `proofs` and nightly schedule of §7. The placement candidates of §0.15 decided at G1 (the literal-to-value merge into `model`; the size arithmetic to the backlog; archcheck split into files inside package main; the `validateApp` pair, `Deploy` and `bind` stay).
- **Owner's check additions** (refined from the diff at the end): *By reading:* LAW Δ (`smells.json` emptied, S1.1c-5-1…3 closed, `test_aging.py`, the instruments' `code` lists), the record's benchmark and coverage tables, every kept-whole waiver with its samples. *By hand:* plant an over-long function in any package and see it red; open `internal/runtime` and read the file names.

## 12. The card (for the plan's table and §3, approved at G0)

| | |
|---|---|
| **Goal** | Every package meets ADR-0015 §2's size limits (cyclomatic ≤ 10, function ≤ 60 lines, file ≤ 500 lines, parameters ≤ 5) with no TEMP size exclusion left (`smells.json` empty) and PERMANENT in-code waivers only under decision 6, each citing its measured samples; each big file split into files named by concern and each approved misplaced function moved; with no change in behaviour or hot-path performance. |
| **Routine · size** | Full loop without the semantics panel · 13 SP, confirmed by the spike before G0 |
| **Depends on** | S1.1c-5 (PR #47), main green |
| **In scope** | 1. The 139 size findings in the 12 packages, in seven lanes (R, V and A as sub-lanes). · 2. Each lane's nets first: Q4's coverage, Q3's performance net for the 38 hot-path functions. · 3. The design's file map and the approved cross-package moves, done before the lanes. · 4. W0: the benchmark comparer; `astequal`'s rename map if a move is approved. · 5. `smells.json` emptied; S1.1c-5-1…3 closed. · 6. S1.1c-7 printed after S1.1c-6. |
| **Not in scope** | The sprint-named test files and the naming check → S1.1c-7. Retiring the exclusion machinery → S1.1c-7 (Q5). The lifecycle known issues S3.3-1-3, S3.3-2-1 → S8, S5.1. The recording line's size arithmetic → backlog unless the owner places it (§0.15). S1.1c-5's backlog (OC-S1, OC-L1, OC-L2, CO-1, the known-issues anchor pin, the runner's `CEP_*` names) → backlog. |
| **Entry criteria** | The standard ones, plus this plan check answered and the spike's projection inside the cap. |
| **Exit criteria** | The done-done block, plus: |
| | 1. **No size finding and no exclusion.** `smells` green on the real tree with its summary `smells: 0 issues dropped by 0 exclusions`, and `aging` green at T4 with the marker at S1.1c-6 (no size entry survives) — **proof:** T4's `smells` and `aging` lines quoted in the record; the teeth `test_smells_planted.py` cases 1–4, 27–28 and `test_aging_planted.py`'s five cases green after the last removal. |
| | 2. **No behaviour change.** T4 green (conformance, baseline, determinism darwin and linux in CI, differential, race, apidiff with no public change, `coverage-inclusion` ≥ 90 per package); the nightly job once after the last removal; every in-package move commit `astequal` exit 0 and every cross-package move its rename-map proof, both re-run by the coordinator at G5; every split function at Q4's bar at its lane's nets commit, re-derived by the coordinator — **proof:** T4, the nightly run, the record's `astequal` and coverage tables. |
| | 3. **No hot-path loss.** Each hot-path net landed before its first split and unchanged after it; each split met Q3's rule or the function stays whole with its samples; each hot path end to end met Q3's rule `main` vs combined — **proof:** the alloc pins at T2; the record's benchmark tables. |
| | 4. **Files by concern, logic in place.** Every production file at most 500 lines; `git ls-files` of production `*.go` in the 12 packages equals the design's file map; every approved move done and no other — **proof:** `smells`; the coordinator's file-map diff quoted; the combined architecture review. |
| | 5. **The debt closed.** S1.1c-5-1…3 absent from `known-issues.json` and the table — **proof:** T4 `aging`; `test_aging.py`'s re-derived real-file test green at T2. |
| **Owner's check** | §11's additions. |
