# S1.1c-6 — design note (v3, 2026-11-16: v2 answered by the owner at G1 — [decisions.md](decisions.md#owner-decisions-2026-11-16--s11c-6s-design-g1), six as recommended, Q5 and Q7 ≠ R)

*Step 1 of the full loop for S1.1c-6, from the answered [plan check](s1-1c-6-plan-check.md) (v3), the owner's answers of 2026-11-15 ([scope](decisions.md#owner-decisions-2026-11-15--s11c-6s-scope-before-its-plan-check), [plan check](decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check), [re-cut](decisions.md#owner-decisions-2026-11-15--s11c-6-re-cut-after-the-spike)) and [the spike](../reviews/s1-1c-6-spike.md). Drafted read-only against `s1-1c-6/integration` @ 6bf339c, whose production Go is identical to `main` 4d2b9eb, so every `file:line` is on that tree. The six package sections (§4–§9) were drafted by the repo's `refactoring-specialist`, one per package group, following the repo's skills `gitnexus-refactoring`, `gitnexus-impact-analysis` and `architecture-designer`; the coordinator verified each section's load-bearing claims (noted at the head of each) and wrote §0–§3 and §10–§12. v1 was reviewed blind by the `architect-reviewer` with `the-fool`'s pre-mortem ([review](../reviews/s1-1c-6-design-architecture.md): 1 blocker, 14 should-fix, 11 later) and the `qa-expert` ([review](../reviews/s1-1c-6-design-qa.md): 2 blockers, 14 should-fix, 5 later); v2 places every finding (§12), rewrites §1–§3 and §10, and amends the package sections where they were wrong. "Measured" marks a run made for this note; "inferred" a reading no run proved; "scratch" an experiment on a copy outside the repo. Nothing here is an owner decision unless it cites one; §10 holds the questions for G1.*

## 0. What the design found that the plan check did not

1. **`internal/validate` collects and sorts its findings; it does not stop at the first** (§5 §0; verified: `internal/validate/validate.go:62-65`). A split there keeps the **multiset** of findings and the **resolved copy** `Check` returns (QA DQ-S2): suppression, first-match switches, poison writes before their reads, and every `${param}` slot write. `runtime.validateApp`, the parser's `parseError`, `event.NewSchema` and playback's reader return the first fault and keep that order, proven per helper boundary (§2.1).
2. **`a.ing == nil` is true in production** (verified: `internal/runtime/lifecycle.go:509`, the removed app's tombstone). The spike was wrong; the operand gets real cases (§4 §3).
3. **`parseRFC3339` has an inliner-shaped re-split** (§6 E.2.1; scratch, measured: equal *results* to the original on 2,115,410 inputs, re-run by the coordinator; bounds checks 4 → 2 against the spike's 4 → 12). Decision 1's measurement still decides time; the frozen original becomes a committed reference test (§6 E.3).
4. **One reordering:** `runtime.build` defaults `RecordQueue` before the validate loop (§4 item 9), unobservable (verified by both reviews).
5. **The literal-to-value rule** is written three times; v2 recommends merging two of them (validate's `fits` and plan's `value`) into `model.LitValue`, and leaving `expr.literalNode` alone: expr's import of `model` is fenced by an owner decision to "the model's expression types … expr reads `model.Expr` only" (`docs/sprints/decisions.md:274`), and a model function would pass the checker only by its name prefix (architecture DA-B1; §5 §4.1, §10 Q1).
6. **The nets need tools, not hand procedures.** The reviews showed that a hand mutation can be counted red when it does not compile (QA DQ-B1), that writing a mutant into a tracked file survives a killed run (DQ-S3, observed), that tied integer benchmark times need an exact conditional p-value (DA-S2), and that a plain escape diff calls a free split costly (DQ-B2, measured). W0 builds four small tools (§3) that make these rules mechanical.
7. **Runtime coverage varies run to run** (QA DQ-S1: `runtime.go:908` covered in 2 of 6 runs). Coverage is measured in five separate runs, and a block counts only if covered in all five (§2.3).
8. **Latent defect found in passing:** `internal/parser/parse.go:363` panics on the notation `"\t"` (inferred unreachable through participle). Fixed in this sprint (owner, G1 decision 7, ≠ R): lane Q's own commit before the parser's splits, red first (§7 Q.6).

## 1. Lanes, order and ownership (two agents at a time)

Sub-lanes are sized at the spike's measured rates so each fits a 30-minute box (architecture DA-S11), with a wrap-up report at the box. Each lane's sub-lanes run one after another in one slot.

| Step | Who | What | Commits |
|---|---|---|---|
| G1 | owner | the answers to §10 | 1 |
| M | `go-engineer` (lane M), the coordinator verifying | the literal-to-value merge of validate and plan, if approved (§5 §4.1) | ≤ 3 |
| W0 | `python-engineer` | the four tools of §3, test first: (a) `compare.py` and `escapes.py`; (b) `mutate.py` and `load.py` | 2 |
| Slot A | `go-engineer` per sub-lane | **R1a** in-place and characterisation nets (router, shard, boundary, build, validate); **R1b** lifecycle deadline, recorder and stress nets; **R1c** benchmarks and allocation pins; **R2a** the pure move, then `ingest`, `take`, `process`; **R2b** `route`, `routeKey` with `broadcast`, `Send` with `SendPayload`; **R3a** `build`, `validateApp`, `(*recQ).run`, `Activate`, `awaitRuns`, `installAck`; **R3b** `Drain`; **R3c** `Remove`, `Close` (§4) | ≤ 16 |
| Slot B | `go-engineer` per sub-lane | **V1** nets; **V2** the move and the phase 2–5 splits; **V3** the phase 6 and expects splits (§5) · **E1** nets; **E2** the move, the object walkers, `appendUnescaped`, the scanner; **E3** `(*Decoder).value`, `parseRFC3339`, `NewSchema` (§6) · **P1** nets; **P2** the moves, the encoders, `Table`, `Step`, the reader; **P3** the records, `runBlock`, the verdict (§8) · **Q1** parser and plan nets; **Q2** the parser's fix (G1 decision 7) and splits (§7); **Q3** plan (§9) | ≤ 12 + 13 + 9 + 10 |
| Measure | coordinator, alone | at each hand-back of a sub-lane with hot functions (R2a, R2b, R3a, E2, E3, P2, P3, Q3), with no agent running (the next sub-lane in either slot waits): decision 1 per split commit (`c^` vs `c`) and the lane's path comparison (nets commit vs lane head) (§2.4) | — |
| Restore | coordinator | a split that costs is restored whole with its waiver by a new commit on the lane branch (§2.4) — no branch rewriting | ≤ 4 |
| Combine | coordinator | cherry-picks; the lane's own `smells.json` entries removed on its branch at hand-back (5 commits); S1.1c-5-1 and -2 closed when their last lane is in (2); the doc rows (§4, §6) (1); T3 | 8 |
| Review, fix round, T4, owner's check | `architect-reviewer` (with `the-fool`) and `qa-expert` blind; the coordinator | the combined diff against this note; the break-it; one batched fix round; the nightly job once; the path comparison `main` vs combined; T4 | ≤ 8 + 3 docs |

**Commit budget** (DA-L6, recounted; G1 decision 8): the plan check, G0, the spike, the design v1 and v2 so far 9 · G1 1 · M 3 · W0 2 · lanes R 16, V 12, E 13, P 9, Q 10 (the parser's fix) · restores ≤ 4 · combine 8 · fix round ≤ 8 · record, T4 and G5 docs 3. **At most 98 of 100.** The coordinator counts at each combine; if the projection passes 98, a lane's non-hot split commits of one file are squashed on its branch before cherry-pick (they carry no `astequal` proof and no per-commit measurement).

**Ownership** follows the plan check §4: each lane owns its packages' files (law paths inside them read-only); lane M owns `internal/model/literal.go`, its test and the two callers' edits for its three commits, before V and Q branch; the coordinator owns `tools/allowlists/smells.json`, `gate/known-issues.json`, `docs/product/known-issues.md`, `gate/law.json`, `gate/registry.json` and the proof drivers.

## 2. Rules every lane follows

**2.1 Refactoring** (S1.1c-5 design §7.3, quoted: "split by concern, each new function named for what it does; no `xPart1..n`, no option struct whose only purpose is to pass > 5 values (a struct that already exists as a domain type may be passed); no behaviour change in the same commit as a move; tests are not reflowed in a refactor commit"), plus:
- No exported identifier changes name or signature. **An internal function a nets test calls directly keeps its signature** (QA DQ-L4: `(*router).ingest`, `(*shard).process`, `(*checker).build`, `(*checker).mayBeNull`, `(*lowerer).str`, `bareDuration`, `diverge` and the rest named in the sections).
- On the hot path, helpers take items, messages and events by value — never `&it` or `&m` into a non-inlined callee unless the escape tool shows no new heap escape (DA-L2).
- No handler tables on the hot-path list (plan check §5.2); elsewhere a table is a never-written package-level var.
- A first-match switch is cut into contiguous runs in its original order, called in that order. **Every function that returns its first error gets one two-fault case per adjacent helper boundary** (QA DQ-S8).
- An accumulator a helper returns is never combined with `||` inline.
- **Concurrency** (QA DQ-S14): a `select` moves whole and is never split into sequential checks; a `defer` stays with the code whose return it guards; a Lock and its Unlock stay in one function (except `startAppGoroutines`' documented pair, §4 item 11); an atomic `Store` or `Swap` keeps its place relative to channel sends.
- An invariant a function sets for its callers stays in that function (`broadcast`'s `admStream = -1`, §4 item 6; DA-S5).
- The pinned placements stay: `(*Decoder).Decode` in `internal/event/decode.go`, `Parse` in `internal/parser/parse.go`.
- A per-line `//nolint` stays on its line when the code moves (the `smells` row reds one silenced from another line).
- A helper's name says what it does; the sections' names are provisional where a reviewer found them wrong (DA-L11: `checkKeyShape`/`checkKeyTypes` are named by the checks they hold).

**2.2 Commits.**
1. **The nets commit(s) first:** tests only, in new files named for the behaviour they test, never for a sprint. **No test file is edited after the lane's nets commits** (`git diff <last nets commit>..HEAD -- <the package's *_test.go>` empty at hand-back; tests that could exist only after a split are dropped, DA-S8).
2. **The pure move commit:** `astequal <c>^ <c>` exit 0.
3. **Split commits:** one per source file or concern; a commit may hold several hot-path functions only if each has its own benchmark that reaches no other function of the commit (DA-S3) — functions measured only through a shared path benchmark (`ingest`, `take`, `process`, `route`; the scanner's `top`, `value`, `object`, `str`, `number`; playback's `record`) get one commit each.
4. Every commit leaves each (package, rule) count at or below its cap; never `--no-verify`.

**2.3 The coverage bar** (decision 2 of the plan check, quoted: "Each function to be split reaches 100 % of its blocks from its own package's tests. Each operand of a compound condition has its case. One mutation is shown red per new test and per control transfer the split moves across a helper boundary. Any block left short is listed in the design with its reason (an invariant panic, or reachable only by scheduling and covered by a named stress test) and reviewed at G1.") The procedure:
1. **Five separate runs** (QA DQ-S1): `go test -count=1 -coverprofile=<file> ./internal/<pkg>` in five separate processes (no `-coverpkg`: the gate's measure); a block is covered only if covered in all five; the lane's table lists every block missed in any run.
2. **Operands:** in a scratch copy (never the worktree), each operand replaced with `true`, then `false`, run through `mutate.py`; where every test stays green, the operand gets its case.
3. **Mutations through `mutate.py` only** (§3.3): each runs the one designated test (`-count=1 -run '^<test>$'`); outcomes red, green, timeout, invalid; only red counts. **Each control transfer's mutation is shown red twice** (QA DQ-S9): at the nets commit, as the equivalent edit on the original code (decision 2's bar), and at the split commit, on the helper. A mutation spec must compile: "drop a propagation" means replacing `if err := f(); err != nil { return err }` by `_ = f()`, never leaving a variable unused. A scheduling-dependent mutant runs N = 20 times under load (`-race -cpu 1,4`, `load.py`) and is reported as k/N; one red in none of the N is named "reasoned, not proven" for G1 (DQ-S5). An equivalent mutant is listed with its argument beside a different mutation of the same transfer shown red (§10 Q4).
4. **Stress under load** (QA DQ-S6; `docs/development/testing.md`): a new timing- or concurrency-sensitive test passes `go test -race -count=50 -cpu 1,4 -run '^<test>$'` with CPU hogs running (`load.py`), quoted as "50/50 under load"; a block "reachable only by scheduling" is covered only if that stress run's own coverage profile shows it hit.
5. The coordinator re-derives each lane's tables at its nets commit.

**2.4 The performance rule** (decision 1 of the plan check, quoted: "Allocations decide first, exactly: allocations equal (`AllocsPerRun` pins and `-benchmem`) and no new heap escape in the function (a `-gcflags=-m=2` diff). Time second: two prebuilt test binaries, ten interleaved pairs at `-test.cpu=1 -test.benchtime=1s`, measured by the coordinator with nothing else running. A split costs time only if the slowdown is both significant (Mann–Whitney, p < 0.05 …) and larger than 2 % on the median, re-run once before the verdict stands."). The procedure:
1. **Allocations:** the nets commit's `AllocsPerRun` pins assert equality and stay unedited; `compare.py`'s fixed-iteration pass compares allocs/op and B/op exactly. (`AllocsPerRun` and `-benchmem` truncate averages, so an amortised addition below one allocation per operation is invisible; DQ-L5.)
2. **Escapes** (`escapes.py`, §3.2; QA DQ-B2, DA-S10): per original function, the multiset of `moved to heap` and `escapes to heap` lines over that function and its new helpers (read from `-m=2`'s "… in F:" lines), at `c^` and at `c`. A new such line in the group costs. A `leaking param` line is a diagnostic, counted only when a parameter the original function already had now leaks where it did not. Package-level counts are a backstop. An escape that moves from a cold to a hot path with the same text (playback's E2) is caught by the allocation pins, not by this diff.
3. **Time:** the coordinator builds `go test -c` at `c^` and at `c` for each split commit (DA-S3) and runs `compare.py` with no agent running; "costs" stands only if the re-run also says costs (both quoted).
4. **The lane's path:** at each hot sub-lane's hand-back, the coordinator compares each path benchmark at the lane's nets commit against the lane head, so costs below the threshold that add up show before T4.
5. **A split that costs** is restored by a new commit on the lane branch: the original function back in its moved file, with a per-line waiver naming exactly the size linters that fire on it plus `lll` (the waiver line passes 120 columns), e.g. `//nolint:gocyclo,lll // PERMANENT reason="<the measured cost>" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-6` (precedent `internal/table/table.go:240`; DA-S4: `nolintlint`'s `allow-unused = false` reds an unused `funlen`). Its samples go in the record.
6. **Before T4:** each path benchmark, `main` against the combined branch: **accepted when its slowdown is not significant or its median is at most 5 % slower, with allocations equal** ([owner, G1 decision 5](decisions.md#owner-decisions-2026-11-16--s11c-6s-design-g1), ≠ R). Above that, the split with the largest measured share is restored whole with its waiver and the path re-measured; if still above, it goes to the owner with every per-function verdict beside it. The lane-level comparisons of step 4 use the same 5 % margin, so a path drifting toward it shows early.

*(Annotated 2026-11-18: a cost of at least 5 % on a split's own benchmark or a path, against the code before the refactor, significant and confirmed by the re-run, now stops the split: the function stays unsplit, an architecture review of its area seeks a better design or algorithm, and the owner decides from its options before the lane hands back — [owner, 2026-11-18](decisions.md#owner-decisions-2026-11-18--a-refactors-large-slowdown-triggers-an-architecture-review), [ground rule 13a](../ground-rules.md). Steps 5 and 6 still apply below 5 %.)*

**2.5 Hand-back and the coordinator's steps per sub-lane.** A sub-lane hands back with: 0 size findings in its packages under the pinned golangci-lint for both `GOOS` (the last sub-lane of a lane), its packages' `go test` green, every commit through T0, and its report (commits; the coverage tables; each mutation's outcome with its diff; each `astequal` exit; each `escapes.py` result; open items). The coordinator then, at the lane's last hand-back, commits the removal of the lane's own `smells.json` entries on the lane's branch, runs T2 and the lane's verifiers (plan check §7), measures (§2.4) with no agent running, and runs `--job proofs` where the plan check's §7 says (red as expected until the last removal: [owner, decision 4](decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check)).

## 3. W0 — four small tools (`python-engineer`; stdlib Python under the pinned interpreter; test first)

All live in a new package `gate/bench/` with `__init__.py` in `gate/bench/` and `gate/bench/tests/` (the `pytests` row's `missing_init` check, `gate/runner/rows/pytests.py:67-77`; DA-L7), their tests in `gate/bench/tests/` run by the `pytests` row. `gate/` is a law path, so every change is a LAW Δ line. None is a gate row: the coordinator and the lanes run them by hand and quote their output in the record. A shared module, `gate/bench/gorun.py`, runs a `go` command in its own session (`start_new_session`) with a wall-clock limit that kills the whole process group (precedent `gate/runner/tests/test_breakit_leak.py`; DA-S1, DQ-S3), and parses `go test` output. `astequal`'s rename map, planned in the plan check for an approved cross-package move, is not built: the merge of §5 §4.1 is not a move (DA-S13).

**3.1 `compare.py` — decision 1's time and allocation verdict** (DA-S2, DQ-S10).
```
python3.15 -I -B gate/bench/compare.py --before <test binary> --after <test binary> --bench <regex> \
    [--dir <package dir>] [--pairs 10] [--benchtime 1s] [--alloc-iters 20000]
```
- **Time pass:** the two binaries in ABBA order (before, after, after, before, …) `pairs` times each, `-test.run=^$ -test.bench=<regex> -test.cpu=1 -test.benchtime=<benchtime> -test.count=1`, in `--dir`.
- **Allocation pass:** each binary once at `-test.benchtime=<alloc-iters>x -test.benchmem`, so both run the same iteration count; allocs/op and B/op compared exactly. A before side whose own repeated runs disagree in allocs/op or B/op is exit 2 (unjudgeable), not "costs".
- **Statistic:** per benchmark, the medians, their ratio and the **exact two-sided Mann–Whitney p-value conditional on the observed ties** (midranks; a dynamic program over doubled midranks, valid at n ≤ 20 per side).
- **Verdict:** **costs** if allocations differ, or if p < 0.05 and median(after) > 1.02 × median(before); else **no cost**. Exit 0 no cost, 1 costs (naming it), 2 an error: a benchmark missing on either side, a binary that exits non-zero, **a regex that matches no benchmark**, an unjudgeable allocation pass.
- **For the record:** the full samples, both binaries' commits and build IDs, `go version`.
- **Tests** (`gate/bench/tests/test_compare.py`): p against brute-force enumeration of every split for small cases with ties; n = 10 vs 10 fully separated p = 2/C(20,10) = 1.0825e-5; the 2 % boundary both ways; a significant 1 % shift is no cost; an allocation difference costs whatever the times; zero matched benchmarks, a missing benchmark and a failing binary are exit 2; a self-disagreeing before side is exit 2; the ABBA order; parsing real `go test -bench` output (fixture text, including integer ns/op ≥ 1000); an **A/A run** (the same binary on both sides) says no cost — the coordinator also runs one A/A per measuring session.

**3.2 `escapes.py` — decision 1's escape check** (DQ-B2, DA-S10).
```
python3.15 -I -B gate/bench/escapes.py --pkg ./internal/<pkg> --before <rev> --after <rev> \
    --function <original name> [--helper <name>]...
```
- Builds `go build -gcflags=-m=2 <pkg>` at each revision (from `git worktree`-free `git archive` copies outside the tree), groups `moved to heap` and `escapes to heap` lines by the function named in their "… in F:" line, and compares the multiset over the original function and its listed helpers. Exit 1 on a new line in the group (naming it), 0 otherwise; `leaking param` lines are printed as a diagnostic and count only for a parameter of the original function that leaks after and not before.
- **Tests:** a split that adds only `leaking param: dst to result` lines → no cost (the measured `AppendValue` case); `&it` passed to a non-inlined helper → `moved to heap: it` → cost; an inlined helper holding an escape → counted once; positions and the ` in F:` suffix normalised.

**3.3 `mutate.py` — one mutation, its designated test, and a tree never touched** (DA-S1, DQ-B1, DQ-S3, DQ-S4).
```
python3.15 -I -B gate/bench/mutate.py --file <go file> --find <exact text> --replace <text> \
    --test '<TestName>' [--timeout 120s] [--repeat 20 --load] [--race]
python3.15 -I -B gate/bench/mutate.py --batch <mutations.json>
```
- Refuses to start unless `--file` is tracked and unchanged against `HEAD` and `--find` occurs exactly once.
- Writes the mutant **outside the tree** and builds it through `go test -overlay=<json>`, so the tracked file is never written (no restore, no window in which a killed run leaves a mutant).
- Runs `go test -count=1 -run '^<TestName>$' -timeout <timeout>` through `gorun.py`; `--timeout` must exceed the test's own ceiling plus its run time (`rtCeiling` is 60 s, `internal/runtime/helpers_test.go:43`).
- **Outcomes:** **red** — the build succeeded and `--- FAIL: <TestName>` (or its panic) was printed, including a failure at the test's own context deadline; **green** — the test passed; **timeout** — `go test`'s `-timeout` backstop fired; **invalid** — a build or vet failure, or no `--- FAIL:` line for a non-zero exit. Only red counts.
- `--repeat N --load` runs it N times under `load.py`'s CPU hogs and reports k/N.
- Batch mode reads `[{"file", "find", "replace", "test", "note"}]` and prints one line per mutation with its diff.
- **Tests** (`gate/bench/tests/test_mutate.py`, on a fixture module): the tracked file byte-equal after a red, a green, a timeout, an invalid run and a SIGTERM mid-run; a non-unique or absent `--find` and a dirty file refused before any build; a mutant that does not compile → invalid; a test hanging past `--timeout` → timeout; a hung child killed with its group; `--repeat` reporting k/N.

**3.4 `load.py` — a `go test` run repeated under load** (DQ-S6).
```
python3.15 -I -B gate/bench/load.py [--hogs <n>] [--coverprofile <file>] -- <go test arguments>
```
- Starts CPU hogs (one per half-core, as `gate/runner/stress.py` does) in their own process group, runs the `go test` command through `gorun.py`, stops the hogs, and reports the pass count (`-count=50` → "k/50 under load"); with `--coverprofile`, keeps the run's profile so a scheduling-only block can be shown hit.
- **Tests:** hogs stopped after a pass, a failure and a timeout; the pass count parsed from `go test -json` output.

## 4. internal/runtime (lane R: R1a → R1b → R1c → R2a → R2b → R3a → R3b → R3c)

*Drafted read-only on `s1-1c-6/integration` @ 6bf339c by the refactoring specialist (gitnexus-refactoring, gitnexus-impact-analysis, architecture-designer); production Go identical to `main` 4d2b9eb. "Measured": in-package coverage with the pinned Go 1.27.4 (`-covermode=set`, package 92.5 %), allocation pins and path benchmarks in a `git archive` copy, and the `-gcflags=-m=2` list (all in the coordinator's scratchpad, `rt-design/`). GitNexus risks from `impact` upstream with tests; `detect_changes`: 0 changed symbols. The coordinator verified the caps (gocyclo 16, funlen 3, file length 2, argument limit 1), `go r.route(rt)` (`lifecycle.go:31`) and the tombstone `a.ing, a.sinks, a.quiet = nil, nil, true` (`lifecycle.go:509`).*

### 1. File map

Moves first, pure: one commit, `astequal <c>^ <c>` exit 0 (imports re-derived per file; `astequal` compares imports as a set per context). Sizes from the declaration ranges; post-split sizes ±10 %.

| File (one concern) | Declarations | From (current `file:line`) | After move → after splits |
|---|---|---|---|
| `runtime.go` — the public surface | package doc; `AppID`, `Emit`, `Rule`, `Sink`, `SinkBinding`, `App`, `Recorder`, `Config`, the `Err*` vars, `TableOp` + consts, `DrainResult`, `RuleStats`, `StreamStats`, `RecordingStats`, `Stats`, `Runtime`, `Start` | runtime.go:1-179, 344-363 | 200 → 200 |
| `app.go` — one app's runtime-side state | `appState`, `runState`, `route`; `(*Runtime).newAppState`, `(*appState).installRoutes`, `(*Runtime).appList`, `(*appState).write` | runtime.go:181-224, 424-473, 1099-1112 | 110 → 110 |
| `messages.go` — what the goroutines exchange | `itemKind`+consts, `item`, `reply`, `msgKind`+consts, `message`, `offer`, `outKind`+consts, `outItem`, `shardStats`, `handoff` | runtime.go:240-342 | 115 → 115 |
| `build.go` — checking and building apps | `build`, `validateApp`; new `withDefaults`, `checkSchemas`, `checkRule`, `checkKeyShape`, `checkKeyTypes`, `checkRuleInputs`, `checkKeyAttrs`, `checkSinks` | runtime.go:365-422, 475-535 | 130 → 215 |
| `requests.go` — the caller side: gate, queue, waits | `isClosed`, `stopping`, `appOf`, `gate`, `enqueue`, `request`, `await`, `Closed`, `Tick`, `TableWrite`, `tick`; new `awaitSignal`, `awaitMarkers` | runtime.go:537-613, 705-737, 748-765, 1365-1366 | 145 → 175 |
| `boundary.go` — source-event entry points and the Go boundary's checks | `recordable`, `lineBound`, `rowBound`, `payloadLimit`, `SendPayload` (stay); `Send`, `CheckRows` (move in); new `checkEvent`, `checkValues`, `checkValue`, `nonFinite`, `asDecodeError`, `recordedFault` | boundary.go:1-132; runtime.go:629-703 | 210 → 275 |
| `stats.go` — the consistent stats cut, end to end | `(*Runtime).Stats`, `streamStats`, `(*shard).stats`; new `(*router).statsCut`, `statsAck`, `completeStats` | runtime.go:615-627, 739-746, 1453-1485 (the line-1453 "stats is…" comment goes with `stats`) | 60 → 135 |
| `router.go` — the router goroutine and its FIFO | `send`, `drainWait`, `statsWait`, `installWait`, `router`, `newRouter`, `(*router).send`, `route`, `push`, `pop`, `now`, `broadcast`, `(*router).app`; new `sendHead` | runtime.go:767-894, 1114-1121; lifecycle.go:422-429 | ~150 → ~165 |
| `router_out.go` — the router's step per shard handoff (v2, DA-L1: the dispatcher beside its helpers) | `take`; new `toSinks`, `ack`, `drainAck` | runtime.go:1197-1269 | ~75 → ~110 |
| `router_in.go` — the router's step per input item (v2, DA-L1: was `input.go`; `ingest` beside its helpers) | `ingest`, `event`, `tickApp`, `copyRows`; new `inputApp`, `ingestEvent`, `ingestReject`, `ingestBatchEnd`, `tickApps`, `writeTable` | runtime.go:895-1089, 1557-1564 | ~210 → ~240 |
| `transitions.go` — the router's side of each lifecycle step | `add`, `remove`, `installAck`, `drainApp`; new `ingestLifecycle`, `activate`, `beginDrain`, `completeRemove`, `rollBackAdd` | lifecycle.go:431-525; runtime.go:1091-1097 | 115 → 205 |
| `keys.go` — routing by key | `routeKey`, `shardOf`; new `keyPart` | runtime.go:1123-1195 | 85 → 95 |
| `shard.go` — the shard goroutine | `shard`, `newShard`, `grow`, `install`, `uninstall`, `newBody`, `(*shard).run`, `process`, `foldBursts`, `offer`, `advance`, `tryAdvance`; new `applyAdvances`, `applyAdmission`, `offerAll`, `ackDrain` | runtime.go:1271-1451, 1454-1461, 1487-1514 | 210 → 255 |
| `sink.go` — the sink flushers | `sinkQ`, `sinkItem`, `(*sinkQ).run`, `call` | runtime.go:226-238, 1516-1555 | 60 → 60 |
| `lifecycle.go` — the caller-side lifecycle (rest) | `appPhase`+consts, `New`, `Add`, `requestQueued`, `Activate`, `Run`, `BatchEnd`; new `activationRefusal`, `startAppGoroutines` | lifecycle.go:1-176 (stays) | 176 → 205 |
| `drain.go` — the drain barrier, caller side | `Drain`, `awaitRuns`; new `(*appState).markDraining`, `revertDrain`, `queueDrain`, `awaitRecordingStop`, `runReturn` | lifecycle.go:178-299 | 130 → 190 |
| `shutdown.go` — removing an app, closing the runtime | `Remove`, `Close`; new `(*appState).notifyRemoved`, `shutDown`, `closeApps`, `closeApp`, `stopRuntime`, `awaitGoroutines` | lifecycle.go:301-418 | 125 → 205 |
| `recorder.go`, `ingest.go`, `sync.go`, `inflight.go` | unchanged, except `recQ.drainQueue` (new) and `sync.go:125`'s call to `broadcast` | — | 266→260; 138; 179; 56 |

After the move every file is ≤ 375 lines and the file-length count is 0; after the splits every file is ≤ 275. No new name collides with a planted proof file (`test_racestress_planted.py` copies `racy.go` and `racy_test.go` into `internal/runtime`). `nosleep_test.go` (law path) scans every non-test file in the directory, so new files are covered without editing it. Coordinator doc edit at combine: known-issues S3.3-1-3's "`runtime.go` `iBatchEnd`" becomes `router_in.go` `ingestBatchEnd` (plan check §4). `ingest.go` (the `Ingest` arithmetic shared with playback) keeps its name; v2's `requests.go` and `router_in.go` no longer read as its synonyms (DA-L1).

### 2. Split plan per finding

Cyclomatic counts use gocyclo's rules (+1 per `if`/`for`/`range`/non-default `case` or comm clause/`&&`/`||`; closures counted in their enclosing function). Each helper states its checks in the original order. **Escape rule for the hot path:** never pass `*item` or `&m`; helpers take `item`, `message` pieces and `event.Event` by value (a `&it` handed to a non-inlined callee whose parameter leaks would move `it` to the heap, one allocation per item; the spike's by-value helpers kept the escape lists equal, measured).

**Hot path (R2)**

1. **`(*router).ingest`** — cc 48, 145 lines, hot; risk LOW (d=1: `route`). Dispatcher (8): `case iAdd, iRemove, iActivate, iQuiet, iDrain: rt.ingestLifecycle(it)` / `iBatchEnd: rt.ingestBatchEnd(it)` / `iEvent: rt.ingestEvent(it)` / `iReject: rt.ingestReject(it)` / `iTick: rt.tickApps(it.now)` / `iWrite: rt.writeTable(it)` / `iStats: rt.statsCut(it.reply)` (the spike's grouping, verified). Helpers: `func (rt *router) ingestLifecycle(it item)` (7); `func (rt *router) activate(it item)` (6); `func (rt *router) inputApp(id AppID, stream int) *appState` (4: nil if the app is gone, `a == nil || a.ing == nil`; nil and `a.after[stream]++` if drained); `func (rt *router) ingestBatchEnd(it item)` (2); `func (rt *router) ingestEvent(it item)` (3); `func (rt *router) ingestReject(it item)` (2); `func (rt *router) tickApps(now int64)` (7); `func (rt *router) writeTable(it item)` (6); `func (rt *router) beginDrain(it item)` (6, transitions.go); `func (rt *router) statsCut(rep chan reply)` (5, stats.go). Control transfers: every `return` in the case bodies (runtime.go:923, 927, 934, 938, 952, 956, 975, 979, 1001, 1008) ended `ingest`, which has nothing after its switch; they become helper returns; the `continue` at :966 stays in `tickApps`' loop. Mutations: delete the `return` after `a.after[stream]++` in `inputApp`; in `beginDrain` delete the `return` after the joined repeated Drain (the spike's surviving mutant, red with `TestRouterRepeatedDrainJoinsTheFirstBarrier`); delete the `return` after `ErrRemoved` and after `ErrDrained` in `writeTable` and `beginDrain`; delete the `!a.activated` `return` in `beginDrain`; swap the `a.drained`/`a.activated` cases in `activate`. **S3.3-1-3 kept:** `inputApp` reads the router's `a.drained`, not the caller's phase, so a batch end queued between Stop and the drain item is still counted and recorded; pinned by a new test (§3).

2. **`(*router).take`** — cc 20, 70 lines, hot; risk LOW (d=1: `route`). Dispatcher (5): `for _, it := range h.items { switch it.kind { case oEvent: rt.toSinks(it); case oAck: rt.ack(it); case oStats: rt.statsAck(h.shard, it) } }`. Helpers: `func (rt *router) toSinks(it outItem)` (4); `func (rt *router) ack(it outItem)` (2: `if w := rt.installs[it.token]; w != nil { rt.installAck(w, it); return }; rt.drainAck(it)`); `func (rt *router) drainAck(it outItem)` (4); `func (rt *router) statsAck(shard int, it outItem)` (4: fold, set parks, `if w.acks < len(rt.r.shards) { return }`, `delete`, then `rt.completeStats(w)`); `func (rt *router) completeStats(w *statsWait)` (6, stats.go). The three `continue` (runtime.go:1211, 1217, 1247) each ended the loop body (nothing follows the switch); they become `return`. Mutations: delete the `return` in `ack` (nil `d` dereference panic); delete the `return` in `drainAck` (the drain completes on the first shard; `TestDrainBarrier` runs at 2 shards, `internal/runtime/drain_test.go:86`, and R1a adds a 4-shard case); delete the `return` in `statsAck` (stats replied after 1 of N acks; a 4-shard stats sum test). `moved to heap: res` (:1226) moves into `drainAck`, the same allocation.

3. **`(*shard).process`** — cc 20, hot; risk LOW (d=1: `(*shard).run`, Sync's step). Stays in `process` (8): the `defer sh.foldBursts()`, the three control cases with their returns, the `rules == nil` branch with its mDrain ack and return. Helpers, in order: `func (sh *shard) applyAdvances(app AppID, adv []clock.WMAdvance, final bool)` (5); `func (sh *shard) applyAdmission(app AppID, stream int, adm int64)` (5); `func (sh *shard) offerAll(app AppID, offers []offer, ev event.Event, tables *table.Snapshot)` (4: the on-arrival advances first, then the offers); the `for ri := range rules { sh.advance(...) }` loop inline; `func (sh *shard) ackDrain(app AppID, token uint64)` (2, when `m.kind == mDrain`). No `return`, `break`, `defer` or `recover` crosses a boundary; only loops move; the helpers re-index `sh.rules[app]`, which nothing writes during a walk (install/uninstall run only on control kinds, which have already returned). Mutations, one per helper: `||`→`&&` in `applyAdvances`; drop `arrival[ri]` in `applyAdmission`; reverse the two loops in `offerAll`; skip the pending sum in `ackDrain`. Possible cost: `ev` copied by value into `offerAll` (88 B) once per message — the path benchmark decides; fallback: pass `m *message` (its parameter does not leak; inferred, checked by the escape diff).

4. **`(*Runtime).route`** — cc 14, hot; risk UNKNOWN (the graph has no caller: `go r.route(rt)`, lifecycle.go:31). `route` (8) keeps the loop, the `stopping` check, the intake select, and `if len(rt.pending) > 0 { if !rt.sendHead() { return }; continue }`. `func (rt *router) sendHead() bool` (8) keeps both 3-arm selects verbatim (select priority unchanged) and returns false on `r.stopped`; the returns at :850 and :860 become `return false`. Flipping `return false`→`true` is an **equivalent mutant** (route's loop top re-reads `r.stopping()`, :836, on the same channel); the mutation shown red is deleting the `case <-r.stopped` arm (the router blocks forever on a full mailbox; `TestCloseStopsTheRouterWaitingOnAFullMailbox` red at its own ctx deadline).

5. **`routeKey`** — cc 11, hot leaf; risk LOW. `func keyPart(v event.Value) (event.Value, bool)` (6) replaces the closure; the call site indexes `values[attr]` first, as the closure did; `routeKey` 6. The closure was already a function boundary: no new transfer. Mutations: drop `v.F = 0`; `||`→`&&`.

6. **`(*router).broadcast`** — 6 parameters, hot (per tick and drain); risk LOW (d=1: `tickApp`, `drainApp`, `(*Sync).End`). New signature `func (rt *router) broadcast(m message)` — the domain type it delivers; cc 2. **`broadcast` sets `m.admStream = -1` itself** (v2, DA-S5: a forgotten field would be 0, stream 0, and `process` applies an admission when `m.admStream >= 0 && m.adm != math.MinInt64`, `runtime.go:1425`). Callers set the rest: runtime.go:1086 → `rt.broadcast(message{kind: mWalk, app: a.id, adv: adv, tables: a.ing.Snapshot()})`; runtime.go:1095 → the same with `kind: mDrain, token: token, final: true`; sync.go:125 → `kind: mWalk, final: true`. Order unchanged (`Snapshot()` already evaluated before the call). Rejected: reading `a.ing.Snapshot()` inside to reach 5 parameters (hides the caller's snapshot point, D5). Mutation: drop the `m.admStream = -1` assignment in `broadcast` (an on-arrival rule takes stream 0's admission; the in-place process test).

7. **`(*Runtime).Send`** — cc 20, hot; risk HIGH (15 direct callers, plus 13 call sites dropped by receiver typing: a lower bound). `Send` (3): gate → `checkEvent` → enqueue. Helpers: `func checkEvent(app AppID, a *App, e event.Event) error` (9: stream, value count, `checkValues`, time attribute, `lineBound`); `func checkValues(app AppID, sch event.Schema, values []event.Value) error` (4: per value, `checkValue` then `recordable`); `func checkValue(app AppID, sch event.Schema, i int, v event.Value) error` (7: the four cases in order, the fourth `case nonFinite(v)`); `func nonFinite(v event.Value) bool` (4, inlinable). Every error text byte-identical. Error order: per value kind → null-required → null-time → non-finite → recordable, then the next value; then Ts; then the line bound. Mutations: delete each `if err != nil { return err }` at the two new boundaries; swap the null-required and null-time cases (two-fault case).

8. **`(*Runtime).SendPayload`** — cc 11, hot; risk LOW (lower bound: 3 call sites dropped). `SendPayload` (9). Helpers: `func asDecodeError(err error) *event.DecodeError` (2); `func recordedFault(name string, payload []byte, info event.Decoded) *event.DecodeError` (3: the `CleanLen` check, then the depth check; nil = none). Order: stream → length → decode → convert → cleaned length → depth → item. Mutation: return nil instead of the depth fault.

**Lifecycle and the rest (R3)** — each behind decision 5's nets.

9. **`build`** — cc 14; risk HIGH (94 upstream via `New`/`NewSync`). `func withDefaults(cfg Config) Config` (7) takes the six defaults, `RecordQueue` included. **The one reordering in the section:** `RecordQueue` is now defaulted before the validate loop — safe because `validateApp` reads no `cfg` field and an error discards `cfg`; a reviewer must accept it. `build` 8.

10. **`validateApp`** — cc 34; risk HIGH (28 upstream). `validateApp(i int, a *App) error` (4): `checkSchemas`, then for each rule `checkRule`, then `checkSinks`. Helpers: `func checkSchemas(i int, a *App) error` (4); `func checkRule(i, ri int, a *App) error` (5: builds `bad`, cases 1–2 inline); `func checkKeyShape(rule Rule, bad func(format string, args ...any) error) error` (6: cases 3–5); `func checkKeyTypes(rule Rule, bad func(format string, args ...any) error) error` (7: cases 6–7); `func checkRuleInputs(a *App, rule Rule, bad func(format string, args ...any) error) error` (8, keeps the `continue`); `func checkKeyAttrs(a *App, sid int, attrs []int, types []event.Type, bad func(format string, args ...any) error) error` (6); `func checkSinks(i int, a *App) error` (5). A `switch` of returning cases ≡ sequential returning checks: first-wins kept. Mutations: one per new `if err != nil { return err }`, one per two-fault case (§3).

11. **`Activate`** — cc 12; risk MEDIUM. `Activate` (7). Helpers: `func activationRefusal(p appPhase) error` (4); `func (r *Runtime) startAppGoroutines(a *appState) error` (4: `inflight.mu` Lock, deferred Unlock, `isClosed` → `ErrClosed`, start the flushers, start the recorder, phase CAS). Defer order kept: the helper's Unlock runs before Activate's deferred `inflight.leave()` (LIFO). Invariant: goroutine starts and the CAS happen under `inflight.mu`, which `Close`'s `close(r.stopped)` also takes (lifecycle.go:390-392; R-S3) — no start after the stop. Mutations: delete the helper's `return ErrClosed` (red with `TestActivateAfterCloseGaveUpIsErrClosed`); delete the `defer` Unlock (Close deadlocks; red at the test's own deadline).

12. **`awaitRuns`** — cc 11; risk HIGH (44 upstream). `func runReturn(err error) error` (2: nil for `context.Canceled`; assigning nil to a fresh slice slot ≡ not assigning). `awaitRuns` 9. `runMu` held across the whole function, unchanged; no transfer moves. Mutation: `errors.Is`→`!errors.Is` in `runReturn`.

13. **`Drain`** — cc 23, 72 lines; risk HIGH (164 upstream). `Drain` (9): appOf → removed check → `was := a.markDraining()` → `awaitRuns` if started (on error, `if !a.waited { a.revertDrain(was) }`) → `queueDrain` → `await` → `awaitMarkers` → `awaitRecordingStop` → result. Helpers: `func (a *appState) markDraining() appPhase` (3: the two CAS); `func (a *appState) revertDrain(was appPhase)` (2, replacing the closure); `func (r *Runtime) queueDrain(ctx context.Context, a *appState, app AppID, was appPhase) (chan reply, error)` (4: `Swap`, enqueue, on failure `if set { Store(false); if !a.waited { revertDrain } }`); `func (r *Runtime) awaitRecordingStop(ctx context.Context, a *appState, m chan struct{})` (7: nil check, the test hook, the 3-arm select with the inner select and `cut`). Invariants: I1 the phase CAS precedes `awaitRuns` and the flag; I2 a revert only when nothing was queued; I3 `draining.Swap(true)` precedes the iDrain send (the gate's fast path); I4 Drain returns only after every marker, then (until the deadline) the recording; I5 `cut` only at the deadline with the marker unclosed. Mutations: `queueDrain`'s error `return` dropped; `Store(false)` dropped; `awaitMarkers`' error ignored in Drain; `cut` dropped; `revertDrain` dropped on the `awaitRuns` error path.

14. **`Remove`** — cc 16; risk HIGH (160 upstream). `Remove` (9). Helpers: `func (a *appState) notifyRemoved()` (2, shared with `Close`); `func (r *Runtime) awaitSignal(ctx context.Context, ch <-chan struct{}) error` (4, the same 3-arm select); `func (r *Runtime) awaitMarkers(ctx context.Context, markers []chan struct{}) error` (3, shared with `Drain`). The `recMarker` wait becomes `if rep.recMarker != nil { return r.awaitSignal(ctx, rep.recMarker) }`. Invariants: the phase store and `cancelSrc` follow the router's answer; `onRemoved` runs before the flushers are awaited (design s3-3-1 §3.6), at most once (`sync.Once` shared with Close). Mutations: each `return err` after `awaitMarkers`/`awaitSignal`; `awaitSignal`'s `ErrClosed` → nil.

15. **`Close`** — cc 20; risk CRITICAL (36 direct, 171 upstream). `Close` (1): `r.closeOnce.Do(func() { first, failed = r.shutDown(ctx) }); return r.awaitGoroutines(ctx, first, failed)`. Helpers: `func (r *Runtime) shutDown(ctx context.Context) (first error, failed []error)` (4); `func (r *Runtime) closeApps(ctx context.Context) (first error, failed []error)` (6: the reverse loop and its `continue`); `func (r *Runtime) closeApp(ctx context.Context, id int, a *appState) (stop, failed error)` (7); `func (r *Runtime) stopRuntime()` (3: Lock, `close(r.stopped)`, Unlock, then `cancelSrc` and `notifyRemoved` per app); `func (r *Runtime) awaitGoroutines(ctx context.Context, first error, failed []error) error` (5). The `break` at :378 becomes `return err, nil` plus the loop's `first == nil` condition. `appList()` is no longer read when the in-flight select set `first` (a pure read). Mutations: skip `closeApp`'s Drain-failure return (red with `TestCloseReportsTheDrainsStuckRunError`, which checks the stuck-runs message Remove's plain deadline error would replace); drop the `case ctx.Err() != nil` arm; move `close(r.stopped)` outside the lock (the race row and `TestActivateAfterCloseGaveUpIsErrClosed`).

16. **`(*router).installAck`** — cc 11; risk LOW. `installAck` (6). Helpers: `func (rt *router) completeRemove(w *installWait)` (5), `func (rt *router) rollBackAdd(w *installWait)` (2). Order kept: markers pushed, `close(a.rec.stop)`, phase store, fields dropped, then the reply. Mutation: in `completeRemove` drop `res.err = w.err` (red with `TestAddRollsBackWhenOneShardFails`).

17. **`(*recQ).run`** — cc 12, hot (per record); risk LOW (lower bound). `func (q *recQ) drainQueue()` (3): `for { select { case it := <-q.q: q.handle(it); default: return } }`. `run` (6) keeps the single 4-arm select (§6.5 of the plan check) and the three defers in place (`settle` → `close(returned)` → `wg.Done`); the stop and stopped arms are `drainQueue(); flush(); return`; the overflow arm `drainQueue(); flush(); settle()`. This removes the plan check §6.3's break-after-select by construction. Mutations: `return`→`break` in `drainQueue`'s default (a busy loop, red at the test's deadline); drop `settle()` in the overflow arm (the drain cuts the recording; `TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles`); drop the stop arm's `return` (Remove never returns; red at the test's deadline).

The three file-length and funlen findings close with the move (`runtime.go`, `lifecycle.go`) and with items 1, 2 and 13.

### 3. Nets

**Coverage today, in-package (measured, blocks):**

| Function | Blocks covered | Uncovered |
|---|---|---|
| `ingest` | 48/59 (one run; `:908` missed in 4 of 6 runs, QA DQ-S1) | runtime.go:908, 910, 923-924, 934-935, 937-939, 942-943, 952-953, 955-957, 974-976, 978-980, 991-993, 1023-1025 |
| `Send` | 17/19 | runtime.go:656, 673-676 |
| `SendPayload` | 17/19 | boundary.go:115-116, 120-122 |
| `route` | 17/18 | runtime.go:850 |
| `process` | 24/27 | runtime.go:1413-1416 |
| `build` | 25/27 | runtime.go:385-386, 416-417 |
| `validateApp` | 31/33 | runtime.go:496, 520-521 |
| `Activate` | 18/19 | lifecycle.go:99 |
| `Drain` | 31/37 | lifecycle.go:190-191, 222-223, 229-230, 235, 247, 250 |
| `Remove` | 19/25 | lifecycle.go:318, 330-331, 337, 339, 346, 348 |
| `Close` | 24/28 | lifecycle.go:384, 398-399, 411, 416 |
| `recQ.run` | 12/17 | recorder.go:169, 188-189, 191, 193-194 |
| `take`, `broadcast`, `routeKey`, `awaitRuns`, `installAck` | 100 % | — |

**Characterisation tests** (package `runtime`, internal; new files named for what they test; every wait bounded by a context deadline, so "red by timeout" is the test's own failure):
- **`router_intake_internal_test.go`:** the spike's fixture `igNew` and its 8 tests (verified sound; they bring `ingest` to 100 %), with one fix (`TestRouterWriteAndDrainOfAGoneOrDrainedApp` uses one subtest name for both kinds; name them `gn+"/"+kind`); `TestRouterTakesABatchEndBeforeItsDrainItem` (caller phase draining, `a.drained` false → counted in `Batches` and recorded: pins S3.3-1-3 as it stands); `TestShardAcksADrainForAFreedSlot`, `TestShardSkipsAWalkForAFreedSlot` (fixture: `build` + `r.shards[0]` after `uninstall`; covers :1413-1416); take's operand cases `TestRouterDropsAnOutputOfARemovedApp` (tombstone `a.ing == nil`) and `/nil slot`.
- **v2 additions (QA DQ-S1, S7, S8, S12):** the spike's eight router tests are named in R1a's brief and committed with the fixture (`TestRouterActivateAnswersByTheAppsState`, `TestRouterStreamInputsOfAGoneOrDrainedApp`, `TestRouterStampsAnArrivalTimeEvent`, `TestRouterWriteAndDrainOfAGoneOrDrainedApp`, `TestRouterTickSkipsEachUnreachableApp`, `TestRouterStatsKeepsANilSlotsRow`, `TestRouterTickReachesTheAppsAfterASkippedOne`, `TestRouterRepeatedDrainJoinsTheFirstBarrier`); `TestRouterActivatesADrainedAppWithErrDrained` (`:908`, deterministic, in place); operand rows for `ingest` `:905` (the tombstone and `a.removing`, each alone), `:965` (the tombstone, `!a.activated`, `a.drained`, `a.quiet`, each alone), `process` `:1420` (a non-arrival rule; an arrival rule, not final; final) and `:1425` (each operand alone), `routeKey` `:1133` (`v.Null`; a NaN float), `SendPayload` `boundary.go:100` (its own `{-1, len, derived}` table) and each condition in `take`; two-fault cases at every adjacent boundary of `validateApp` (checkSchemas | the first rule check; case 2 | case 3; case 5 | case 6; the key checks | the input checks | the key-attribute checks; the last rule | `checkSinks`) and of `recordedFault` (a cleaned length past the bound and a depth fault → the length named).
- **`route_stop_internal_test.go`** (v2, DA-L8, QA DQ-S13): `TestCloseStopsTheRouterWaitingOnAFullMailbox` (:850): Mailbox 1, Intake 1, a body held in `OnEvent`; Sends until the router's pending FIFO holds one item, waited on as a state (the stats' `PendingHigh` or a test hook), never a timer; Close with a short ctx; then, **with the body still held**, wait (under the ceiling) for the router goroutine to have exited, by the goroutine-id helper; then release the body. Without the `case <-r.stopped` arm the router cannot exit while the body is held, so the mutation is red.
- **`boundary_checks_internal_test.go`:** `TestSendRefusesANullOptionalTimeAttribute` (:656); `TestSendRefusesAnEventOverOneRecordingLine` (:673-676; a string of `MaxLogLine/6+1` bytes); `TestSendPayloadRejectsAPayloadThatGrowsPastOneLineWhenCleaned` (:120-122; inferred that Decode accepts malformed UTF-8 in a string — verify first); operand tables `TestSendRefusesEachNonSourceStream` {-1, len, derived}, null×{required, optional}, null-time×{time attr, other}, float×{NaN, ±Inf, null, finite}, Ts×{arrival-time, mismatch, match}; `TestRouteKeyKeepsANonZeroFloatPart` (the missing operand of `v.Kind == TFloat && v.F == 0`).
- **`build_validate_internal_test.go`:** `TestBuildRefusesAnInvalidApp` (:385-386); `TestBuildReturnsARuleBuildError` (a `NewBody` that panics; :416-417); `TestValidateAppRefusesEachMalformedRule` (one case per switch case and operand, :496 and :520-521 included); `TestValidateAppNamesTheFirstFault` two-fault cases (no inputs + no body; KeyAttrs count + TTL 0; schema count + bad sink; rule 0 + rule 1 bad; bad input + duplicate); fixture `cgApp` mutated per case.
- **`lifecycle_deadlines_internal_test.go`** (fixtures `cgHeld`, `cgBlockRouter`, `rdGated`, a new `heldFlushRecorder` whose `Flush` blocks until released, SinkQueue 4 so the router never blocks on the held sink): Drain — `TestDrainOfAnInactiveAppWithAnExpiredContextLeavesItInactive` (:222-223), `TestDrainGivesUpAtItsDeadlineBeforeTheBarrierAnswers` (:229-230), `TestDrainWaitingOnASinkIsErrClosedWhenCloseGivesUp` (:235), `TestDrainWaitingOnTheRecordingReturnsWhenCloseGivesUp` (:247); Remove — `TestRemoveCallsOnRemovedOnce` (:330-331), `TestRemoveWaitingOnASinkIsErrClosedWhenCloseGivesUp` (:337), `TestRemoveGivesUpAtItsDeadlineBehindABlockedSink` (:339), `TestRemoveWaitingOnTheRecorderIsErrClosedWhenCloseGivesUp` (:346), `TestRemoveGivesUpAtItsDeadlineWhileTheRecordingFlushes` (:348); Close — `TestCloseGivesUpWhileARemovedRecordingFlushes` (:384), `TestCloseCallsOnRemovedOfAnAppItCouldNotRemove` (:398-399), `TestASecondCloseWaitsUntilItsOwnDeadline` (:416), `TestCloseReportsTheDrainsStuckRunError`; Recorder — `TestCloseStopsAnIdleRecorderItGaveUpOn` (recorder.go:188-189, 193-194), `TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles` (stressed `-count=50 -cpu 1,4`); Install — `TestAddKeepsTheFirstShardsError` (operand `it.err != nil && w.err == nil`). No test is added after the split (§2.2; DA-S8): Activate's phase errors are already pinned.
- **Blocks left short, for G1 — reachable only by scheduling, each with a named stress test** (`-cpu 1,4`, ≥ 64 iterations): lifecycle.go:99, 190-191, 318 (a Remove stores `phaseRemoved` between `appOf`'s load, runtime.go:556, and the function's own load) — `TestActivateRacingRemoveOfAnInactiveApp`, `TestDrainRacingRemoveOfAnInactiveApp`, `TestRemoveRacingRemove`; lifecycle.go:250 (the recording marker and the deadline both ready) — `TestDrainAtItsDeadlineKeepsARecordingWrittenAsItPassed`; lifecycle.go:411 (`first` written only under `ctx.Err() != nil`, :367, :376-377, :383-384, so `ctx.Done` is always ready and both arms return `first`) — `TestCloseAfterItsDeadlineReturnsTheDeadline`; recorder.go:169, 191 (`q.q` and stop/stopped both ready) — `TestRemoveWritesEveryRecordQueuedBeforeItsStop`, `TestCloseWritesTheRecordsQueuedWhenItGaveUp`.
- **Never true in production — needs an owner answer (§5):** boundary.go:114-116 (`!errors.As`; inferred: `decode.go` has no `errors.New`/`fmt.Errorf`); the `a == nil` slot operands (§5 Q2).

**Performance (decision 1).** Path benchmarks (`hotpath_bench_test.go`, package `runtime`): stream `s` (k string, ts timestamp, event-time); one rule keyed by `k` (`KeyAttrs [[0]]`, TTL 60 s, a body that emits nothing); five resident keys; 2,000 warm events; b.N events prebuilt with ascending ts (1 ms); a Stats cut after the loop. Measured, indicative (darwin/arm64, `-cpu 1`, 20000x, one run):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkSendPath/shards=1` | 754 | 472 | 3 |
| `/shards=4` | 1901 | 2656 | 8 |
| `/shards=1/recorded` | 730 | 674 | 3 |
| `BenchmarkSendPayloadPath/plain` (`{"k":"ka","ts":"<RFC 3339 ms>"}`, ascending) | 1334 | 746 | 6 |
| `/recorded` | 1179 | 1054 | 9 |

To add, not measured: `BenchmarkSendPath/emitting` (one output per event: the take→sink path) and `BenchmarkSendPayloadPath/rejected` (the reject path). Do not reuse `twApp` (its rule is key-less, leaving `routeKey`'s keyed branch and `shardOf`'s loop unexercised, though `TestHandoffAllocationBudget` says "five resident keys", s3_2b_2_alloc_test.go:11-12, 34-41). Leaf benchmarks and exact pins (`AllocsPerRun`, measured): `BenchmarkRouteKey/{keyless, one-part, two-parts, float-zero}` 0 / 0 / 1 / not yet measured; `broadcast` in place at 4 shards 0; extra exact nets recommended: `TestRouterIngestAllocations` (no-op deliver: 0 at 1 and 4 shards, 1 for a two-part key; live `rt.send` with the FIFO popped: 3 at 1 shard, 8 at 4, matching the MemStats ceiling's 3.00/8.1), `TestShardProcessAllocations` (mWalk with one offer: 0), `TestRouterTakeAllocations` (one output to one sink: 1, the FIFO's re-grow after `pop` sets it to nil, runtime.go:884-886). `TestHandoffAllocationBudget` stays a ceiling. Escape check (v2): `escapes.py` (§2.4, §3.2) per split commit, the original function and its new helpers as one group. Today's hot-path functions hold these `-m=2` escape lines: `Send` 44, `ingest` 20, `SendPayload` 17, `take` 12, `process` 7, `event` 4, `recQ.put` 3, `route` 2, `routeKey` 2, `broadcast` 2, `recQ.handle` 2, `recQ.run` 2. `can inline` lines for `keyPart`, `nonFinite`, `inputApp` are diagnostic only.

### 4. Placement

- **The two `validateApp` stay — confirmed.** `runtime.validateApp` (runtime.go:476) checks `runtime.App`, the Go API's precondition for `Add` and `NewSync`; `internal/validate` may import only event, model, expr, window and metrics (`tools/importcheck/rules.json:17`).
- **The recording-line size arithmetic stays in runtime; backlog — confirmed.** `lineBound`/`rowBound`/`payloadLimit`/`CheckRows` (boundary.go:45-88, runtime.go:683, used by `deploy.go:221`): none is among the 22; the knowledge is the playback encoder's envelope (`256 +`, ×6 escape; boundary.go:65-66); runtime may not import playback; the only shared home, `internal/event` next to `MaxLogLine`, would put encoder knowledge in the decoder's package and cross lanes R and E. The real fix (playback owns the bound, the facade injects it) is a design change.
- Nothing else in runtime belongs elsewhere, and nothing elsewhere belongs here (`Ingest`, `Sync` are shared with playback, which may import runtime). `CheckRows` moves within the package only.

### 5. Risks and owner questions (recommendation first)

1. **Q-R1 — boundary.go:114-116, never true in production (inferred).** → keep as a defensive conversion, listed at G1 as "never true today" with its argument. Other: remove it in its own commit (a future decoder returning a non-`DecodeError` would then record a nil fault).
2. **Q-R2 — the `a == nil` slot guards** (runtime.go:556, 903-905, the `ingest` guards, 1022, 1251; lifecycle.go:372, 394): no code stores a nil slot (runtime.go:404-411, lifecycle.go:444-446), and `rt.app` returns nil only for an id beyond the list (lifecycle.go:423-429), which callers cannot pass — never true (inferred). **Corrects the spike's item 8:** `a.ing == nil` *is* true in production (the tombstone of a removed app, lifecycle.go:509). → keep, reached by the in-place tests; removal to the backlog.
3. **Q-R3 — mutations red only at a deadline.** → they count when the test's own context deadline turns them red with a message; the runner uses `-timeout 120s`.
4. **Q-R4 — equivalent mutants.** → list each with its argument and show a different, red mutation of the same transfer (as for `sendHead`).

Risks: `TestHandoffAllocationBudget`'s comment is misleading (its rule is key-less) — should-fix, to the backlog, the test not edited this sprint; inlining and copying cost — the path verdict decides `process`/`offerAll` (by value vs pointer) and the per-value `checkValue` call; if either costs, apply decision 6's re-split once; S3.3-2-1 is in `apphandle.go`, not runtime — Activate and Run keep gate → `inflight.mu` → `isClosed`, unchanged.

### 6. Commit plan (15; each at or below every cap; v2: one path-measured hot function per commit, DA-S3)

Caps (`smells.json`): gocyclo 16, funlen 3, file length 2, argument limit 1. Sub-lanes run one after another (R1 → R2 → R3): `transitions.go` gets `ingest`'s helpers in R2 and `installAck`'s in R3.

| # | Sub-lane | Commit | Counts after (gocyclo/funlen/file/arg) |
|---|---|---|---|
| 1 | R1a | router-intake (the spike's eight tests and the v2 additions), route-stop, shard in-place, boundary, routeKey, build and validate characterisation tests | 16/3/2/1 |
| 2 | R1b | lifecycle deadline, recorder and stress tests | 16/3/2/1 |
| 3 | R1c | benchmarks and allocation pins (never edited afterwards) | 16/3/2/1 |
| 4 | R2a | pure move to the file map (`astequal` exit 0) | 16/3/0/1 |
| 5 | R2a | split `ingest` (path-only: its own commit) | 15/2/0/1 |
| 6 | R2a | split `take` | 14/1/0/1 |
| 7 | R2a | split `process` | 13/1/0/1 |
| 8 | R2b | split `route` | 12/1/0/1 |
| 9 | R2b | split `routeKey` (leaf benchmark); `broadcast(m message)` (per tick; its in-place pin) | 11/1/0/0 |
| 10 | R2b | split `Send` and `SendPayload` (each its own path benchmark) | 9/1/0/0 |
| 11 | R3a | split `build` and `validateApp` | 7/1/0/0 |
| 12 | R3a | split `recQ.run` | 6/1/0/0 |
| 13 | R3a | split `Activate`, `awaitRuns`, `installAck` | 3/1/0/0 |
| 14 | R3b | split `Drain` (with `awaitSignal`/`awaitMarkers`) | 2/0/0/0 |
| 15 | R3c | split `Remove` and `Close` (`notifyRemoved`) | 0/0/0/0 |

Hand-back: 0 findings in both `GOOS`; `go test -race -count=3 ./internal/runtime/...`; the lifecycle tests at `-cpu 1,4`; the nightly job alone. Gaps: the time medians are single runs under load (the verdict is the coordinator's interleaved pairs); the `CleanLen`-growth and `Decode`-error readings are inferred; no mutation was run in this design step.

## 5. internal/validate (lane V: V1 → V2 → V3)

*Drafted read-only on `s1-1c-6/integration` @ 6bf339c by the refactoring specialist (gitnexus-refactoring, gitnexus-impact-analysis, architecture-designer); production Go identical to `main` 4d2b9eb. "Measured" = a run for this section (pinned Go 1.27.4; profile in the coordinator's scratchpad). GitNexus `impact` upstream on all 19 functions: every one risk LOW, d=1 its single in-package caller, module `Validate` only (one false edge: `graph ← TestCycle`; the test calls `cycle`, `units_test.go:300`). The coordinator verified §0's sort (`validate.go:62-65`), the caps (gocyclo 19, funlen 3) and `exprs.go`'s 409 lines.*

### 0. The one fact that shapes every split here

`check` sorts all findings by (Line, Col, Code, Msg) before returning them (`validate.go:62-65`); nothing reads `c.diags` before that sort (`validate.go:62, 66, 68, 71, 131`); `Fix` and `Lint` are functions of `Code` (`validate.go:131-134`). So **the order in which findings are appended cannot be seen**: a two-fault program's first-named error is set by source position, not by which check ran first. The plan check's §6.1 premise ("returns the first error found") does not hold for validate; it collects every finding. A split must keep **the multiset of findings and the resolved copy**, decided by four things:

- **(S) suppression:** every `return`, `continue` or `break` that stops later checks on the same item after a finding ("one defect, one finding", `validate.go:4-5`);
- **(F) first-match switches:** at most one finding per item (e.g. `selects`' 7-case switch, `givenRecord`'s two switches);
- **(P) poison writes before their reads:** `poisoned`, `noTarget`, `badKey` written in `names`; `effective`, `badParam` in `resolve`'s defaults then overrides; `skipDur`, `skipExpr` in `resolve`; `badTs` in `selects`, carried across rules; `graphFound`, `reported` local accumulators;
- **(O) the resolved copy** (v2, QA DQ-S2): `Check` also returns the copy with every `${param}` slot resolved (`validate.go:44-46`) — `n.Lit = v`, both `n.Operand.Strings` writes, `d.Ms, d.Param`, `it.Value = v` (`resolve.go`); a split keeps each write.

Every two-fault test below asserts the **exact sorted list** of (code, line:col). Phases stay in `run`'s order (`validate.go:103-109`); no split touches `run`, `report`, `check` or the poison maps' declarations.

Binding rules (`.golangci.toml`): gocyclo > 10 (`:64-65`) counts every `if`, `for`, non-default `case`, `&&`, `||`, **including inside closures**; funlen 60 lines with comments (`:57-60`); lll 120 (`:68`); revive argument limit 5 (`:84-86`). Caps: `internal/validate gocyclo` 19, `funlen` 3 (`tools/allowlists/smells.json`), today exactly at the caps.

### 1. File map

All files are under 450 lines; only `exprs.go` would pass 500 after the splits (~530 projected), so it is the only file split — a pure move before any split.

| File | Concern | Declarations after the sprint | Now → projected |
|---|---|---|---|
| `validate.go` | entry point, checker, shared lookups | package doc, `Diagnostic`, `Options`, `Override`, `Check`, `check`, `checker`, `run`, `index`, `report`, `unknown`, `deepCopy`, `copyValue`, `rules`, `sources`, `read`, `hasAnnotation`, `attrOf`, `attrNames`, `streamNames`, `tableNames`, `present` | 257 → 257 |
| `names.go` | phase 2: names | `nearest`, `distance`, `names`, **`primaryKey`, `keyColumn`, `ruleNames`, `sourceNames`, `partitionNames`, `targetName`, `sortAttr`**, `captures` | 133 → ~170 |
| `resolve.go` | phase 3: params | `resolve`, **`defaults`, `overrides`, `resolveRule`**, `fits`, `as`, `kindName`, `value`, `resolveAnnotations`, `durationSlots`, `resolveDuration`, `expressions`, `setOp`, `resolveExpr`, **`resolveLiteral`, `resolveOperand`** | 213 → ~245 |
| `graph.go` | phase 4: graph and time | `graph`, **`producersAndReaders`, `streamClasses`, `classify`, `untimedStream`, `timedStream`, `streamRef`, `streamRefs`, `producerOf`, `derivedReads`, `unexplainedCycle`**, `invisiblesDecl`, `timeAttr`, `cycle`, **`feedEdges`**, `ruleNames`, `idle`, **`timerReason`** | 222 → ~275 |
| `bounds.go` | phase 5: partition and bounds | `bounds`, `partition`, **`anyBadKey`, `unkeyedReads`, `liveKeys`, `repeatedAttrs`, `repeatedStreams`, `partCounts`, `bracketedSingles`, `keyTypes`, `objectKeys`**, `attrNamesOf`, `zeroDurations`, **`boundSlots`**, `ttl`, **`ttlBound`, `ttlBounds`**, `emitModes`, `clauses`, **`todos`, `lateness`, `windowClauses`**, `human`, `notPositive` | 284 → ~345 |
| `exprs.go` | phase 6: driver and typing | `expressions`, `filter`, `before`, **`placement`, `singleColumnKey`, `oneColumnTable`, `callPlacement`, `callName`**, `build`, **`buildError`, `captureHint`, `nameCandidates`**, `isAttr`, `dotted`, `walk` | 409 → ~210 |
| `nulls.go` (new, moved) | null rules (E-NULL-001/002/003 support) | `nulls`, **`nullTested`, `nullBlind`, `optionalOperands`, `nullFinding`**, `opName`, `mayBeNull`, **`attrMayBeNull`, `anyMayBeNull`, `callMayBeNull`**, `exprText` | 0 → ~150 |
| `regex.go` (new, moved) | RE2 pre-scan (E-REGEX-001) | `regex`, `unsupported`, **`backreference`, `lookaround`** | 0 → ~60 |
| `selects.go` (new, moved) | select binding (E-SEL-*, E-SINK-001) | `selects`, **`bindItems`, `itemType`, `itemName`, `binding`, `itemValue`, `outputTs`, `unboundAttrs`, `sortSink`**, `assignable`, `aggregating`, `perEvent` | 0 → ~170 |
| `expects.go` | E-EXP-002 | `expects`, `expectBlock`, **`givenTable`, `givenStream`, `thenSection`**, `givenRecord`, **`givenValue`, `leftOut`**, `thenRecord`, `mistake`, `blockValue`, `objectText`, `unknownText`, `readsTable`, `attrNamed`, `article` | 210 → ~245 |
| `connectors.go` | annotation vocabulary | `connector`, `connectors`, `builtin`, `connectorNames`, `find`, `typeOf`, `connectorOf`, `annotations`, **`connectorTable`, `connectorItems`**, `mapAnnotation`, `quoted` | 138 → ~155 |
| `scope.go`, `catalog.go` | unchanged | — | 121, 90 |

Bold names are new. The move commit (§6, commit 2): `exprs.go:144-201` (except `isAttr`, `dotted`, `walk`, which stay) and `:353-409` → `nulls.go`; `:210-246` → `regex.go`; `:248-351` → `selects.go`. Proof: `astequal <c>^ <c>` exit 0 (the package's union of keyed declarations unchanged; imports not keyed, inferred from S1.1c-5 design §7.4; the lane quotes the exit). Not recommended: `partition` to its own file (`bounds.go` is one phase, `bounds.go:11-12`); `deepCopy`/`copyValue` to `internal/model` (one caller; backlog).

### 2. Split plan per finding

Legend: **cc** cyclomatic; **T#** a control transfer crossing a new helper boundary (each gets a mutation, §3.4); **H#** a two-fault row of `TestOneFindingHidesAnother` (§3.3). Statements move verbatim unless a rewrite is stated; receivers are `(c *checker)` unless pure. Every function's GitNexus risk is LOW.

**2.1 `partition`** (`bounds.go:30`; cc 31, 117 lines). Today's order: (a) no keys → E-PART-001 if the body is not plain, return; (b) any `badKey` → return silently; (c) E-PART-002 per unkeyed read, collected; (d) E-PART-006, the first repeated attribute per key, `break`, marks `bad`; (e) E-PART-004 for a repeated stream, marks `bad`; (f) `live` = unmarked keys, empty → return; (g) E-PART-007 once, then **return**; (h) E-PART-005 per grouped single; (i) E-PART-003, the first differing part per key, `break`; (j) E-PART-008, the first object part per key, `break`.
```go
func (c *checker) partition(r *model.Rule)                                         // cc 6, ~18 lines
func (c *checker) anyBadKey(keys []model.PartitionKey) bool                        // (b) cc 3
func (c *checker) unkeyedReads(r *model.Rule)                                      // (c) cc 5
func (c *checker) liveKeys(r *model.Rule) []*model.PartitionKey                    // (d)(e)(f-filter) cc 3
func (c *checker) repeatedAttrs(r *model.Rule, bad map[*model.PartitionKey]bool)   // (d) cc 4
func (c *checker) repeatedStreams(r *model.Rule, bad map[*model.PartitionKey]bool) // (e) cc 3
func (c *checker) partCounts(r *model.Rule, live []*model.PartitionKey) bool       // (g) true = reported; cc 4
func (c *checker) bracketedSingles(r *model.Rule, live []*model.PartitionKey)      // (h) cc 4
func (c *checker) keyTypes(r *model.Rule, live []*model.PartitionKey)              // (i) cc 5
func (c *checker) objectKeys(r *model.Rule, live []*model.PartitionKey)            // (j) cc 5
```
Body: (a) inline; `if c.anyBadKey(keys) { return }`; `c.unkeyedReads(r)`; `live := c.liveKeys(r)`; `if len(live) == 0 || c.partCounts(r, live) { return }`; then (h), (i), (j). `anyBadKey` takes `r.Partition.Keys` itself, never a copy (the `&keys[i]` pointers are `names`', `names.go:84, 88, 93`). **T1** the return at `:40` → `anyBadKey` true → caller returns; **T2** the return at `:103` → `partCounts` true → caller returns; the breaks at `:66, :129, :145` move with their loops. Proof: H1, H2, the existing `TestGroupedKeyFindings` (`s3_2b_2_test.go:50`).

**2.2 `zeroDurations`** (`bounds.go:162`; cc 12, 24 lines). `func boundSlots(r *model.Rule) []*model.DurationOrParam` — the slot list in today's order (`:163-179`), cc 7; `zeroDurations` keeps the loop and condition `:180-184`, cc 6. No transfer crosses; findings collected.

**2.3 `ttl`** (`bounds.go:189`; cc 12, 32 lines). The local type `bound` moves to package level as `type ttlBound struct{ what string; d *model.DurationOrParam }`; `func ttlBounds(r *model.Rule) []ttlBound` (`:198-213` verbatim, `output first every` last), cc 6; `ttl` keeps its guard `:191` and report loop `:214-219`, cc 7. No transfer crosses.

**2.4 `clauses`** (`bounds.go:227`; cc 12, 31 lines). `clauses` = `c.todos(r); c.lateness(r); c.windowClauses(r)`, cc 1; `func (c *checker) todos(r *model.Rule)` (`:228-235`, cc 4); `func (c *checker) lateness(r *model.Rule)` (`:236-242`, cc 3); `func (c *checker) windowClauses(r *model.Rule)` (`:243-256`, the non-window `return` first, cc 7). **T3** the return at `:244` now exits `windowClauses` — equivalent, it was `clauses`' tail; its mutation (delete the guard) is red only for a Go-built model whose plain body carries a `Window` (H17).

**2.5 `annotations`** (`connectors.go:84`; cc 11, 30 lines). `func (c *checker) connectorTable(name string) ([]connector, bool)` (source → Sources, sink → Sinks, else `nil, false`; cc 3); `func (c *checker) connectorItems(a *model.Annotation, table []connector)` (`:97-111` verbatim, cc 8). Loop body: `table, ok := c.connectorTable(a.Name); if !ok { c.unknown(...); continue }; c.connectorItems(a, table)`, cc 3. The `continue` stays in the loop.

**2.6 `expectBlock`** (`expects.go:28`; cc 14, 49 lines).
```go
func (c *checker) givenTable(r *model.Rule, at string, g *model.Given)                         // :33-44, cc 4
func (c *checker) givenStream(r *model.Rule, at string, g *model.Given, given map[string]bool) // :47-62, cc 6
func (c *checker) thenSection(r *model.Rule, at string, th *model.Then)                        // :64-75, cc 4
```
Loop: `if g.Table { c.givenTable(r, at, g); continue }; c.givenStream(r, at, g, given)`; then `c.thenSection(r, at, &b.Then)`; cc 3. `given[g.Name] = true` stays only in `givenStream`'s default branch (F): a stream refused as stamped on arrival is not marked given. Sections still checked in written order.

**2.7 `givenRecord`** (`expects.go:80`; cc 16, 32 lines; already 5 parameters, so no helper gets 6).
```go
func (c *checker) givenValue(at, owner string, a *model.Attr, f *model.Field, timeAttr string) // :97-103; cc 4
func leftOut(attrs []model.Attr, timeAttr string, seen map[string]bool) []string              // :105-110's condition; attrs order; cc 7
```
`givenRecord` keeps the field loop with its 3-case switch and three `continue`s (S, F), calls `givenValue`, then `for _, name := range leftOut(...) { c.mistake(rec.Pos, at+fmt.Sprintf("%s.%s is required; the block leaves it out", owner, name)) }`; cc 7. `seen` is complete before `leftOut` runs.

**2.8 `before`** (`exprs.go:62`; cc 16, 36 lines). Today: depth-first, the node before its arguments, arguments in order, stopping at the first finding (`true`).
```go
func (c *checker) before(n *model.Expr, count, chain bool) bool        // if c.placement(n,count,chain) {return true}; args loop :91-95; cc 4
func (c *checker) placement(n *model.Expr, count, chain bool) bool     // switch In/Text/Call → helper; default false; cc 4
func (c *checker) singleColumnKey(n *model.Expr) bool                  // :65-69, cc 3
func (c *checker) oneColumnTable(n *model.Expr) bool                   // :71-74, cc 3
func (c *checker) callPlacement(n *model.Expr, count, chain bool) bool // :76-89 with callName; cc 6
func callName(n *model.Expr) string                                    // "ns:name" or name; cc 2 (also used by mayBeNull)
```
**T4–T7** the four `return true` (`:68, :73, :83, :87`) now return from helpers; `before` returns `true` when `placement` does. `filter`'s short-circuit `c.skipExpr[root] || c.before(...)` (`:46`) untouched. Proof: H3–H6, the existing composite-key `in` and `count()` in a filter cases (`units_test.go:52-90`).

**2.9 `build`** (`exprs.go:100`; cc 13, 43 lines).
```go
func (c *checker) build(root *model.Expr, sc *scope) (expr.Expr, bool)       // err==nil → e,true; else c.buildError; nil,false; cc 2
func (c *checker) buildError(root *model.Expr, sc *scope, err error)        // :105-114, then hint, then unknown; cc 4
func (c *checker) captureHint(xe *expr.Error, sc *scope, at model.Pos) bool // What=="attribute", no ".", sc.bare<0: :121-127; cc 6
func (c *checker) nameCandidates(xe *expr.Error, sc *scope) []string        // :116-139 minus the hint; Cut, qualifier "" if !found; cc 6
```
`buildError` order: a non-`*expr.Error` → E-TYPE-001; a code other than E-NAME-001 reported as is; the capture hint (**T8**: `true` returns before `unknown`); then `c.unknown(at, xe.What, xe.Name, c.nameCandidates(xe, sc))`. `strings.Cut`'s `found` ≡ `strings.Contains(xe.Name, ".")`; `Cut` in both helpers, as today. Proof: H7.

**2.10 `nulls`** (`exprs.go:148`; cc 15, 35 lines).
```go
func nullTested(root *model.Expr, sc *scope) map[*model.Attr]bool                      // walk 1, :149-156; cc 4
func nullBlind(n *model.Expr, inAbsence bool) bool                                      // :158; cc 4
func optionalOperands(n *model.Expr, sc *scope, handled map[*model.Attr]bool) []string // :162-170; cc 6
func (c *checker) nullFinding(n *model.Expr, offenders []string, inAbsence bool)       // the switch :171-180; cc 3
```
`nulls`: `handled := nullTested(root, sc)` first, then the second walk with `if !nullBlind(n, inAbsence) { return }; c.nullFinding(n, optionalOperands(n, sc, handled), inAbsence)`; cc 2. `handled` is complete before any finding.

**2.11 `mayBeNull`** (`exprs.go:357`; cc 15, 45 lines). The switch keeps its case order: Literal; Attr/Raw → `attrMayBeNull`; Neg/Arith → `anyMayBeNull(n.Args, …)`; two-valued kinds → false; Call → `callMayBeNull`; default `return true`; cc 6.
```go
func attrMayBeNull(n *model.Expr, sc *scope, r *model.Rule) bool                 // :362-378; cc 6
func (c *checker) anyMayBeNull(args []model.Expr, sc *scope, r *model.Rule) bool // ContainsFunc(&x copy), as :380; cc 1
func (c *checker) callMayBeNull(n *model.Expr, sc *scope, r *model.Rule) bool    // :384-398 with callName; cc 4
```
Each `return` becomes `return helper(...)`, value preserved; `NullIfAll`'s double negation verbatim.

**2.12 `unsupported`** (`exprs.go:228`; cc 11, 19 lines; pure).
```go
func unsupported(p string) (string, int) // for i: if p[i]=='\\' && i+1<len(p) { if w := backreference(p[i+1:]); w != "" {return w, i}; i++; continue }; if w := lookaround(p[i:]); w != "" {return w, i}; cc 6
func backreference(rest string) string   // :232-237; cc 4
func lookaround(s string) string         // :239-242; cc 5
```
**T9** the `continue` after `i++` is essential: without it `\(?=` would scan `(?=` as a lookahead (today the escaped character is skipped, `:238`).

**2.13 `selects`** (`exprs.go:251`; cc 34, 75 lines). Today: (1) target nil when `noTarget`; (2) per item: name; typing (`skipExpr` || `before` || `build`); untyped → `broken`, continue; target nil → continue; first-match switch E-SEL-001×4, E-TYPE-001, E-NULL-003, E-SEL-002; then `bound[name]`; (3) target nil → return; (4) E-TYPE-001 at `ts` once per stream (`badTs`); (5) E-SEL-001 for unbound attributes unless `broken`; (6) E-SINK-001.
```go
func (c *checker) selects(r *model.Rule, sc *scope)   // target; bound, broken := c.bindItems(r, sc, target); if target==nil {return}; c.outputTs(target); if !broken { c.unboundAttrs(r, target, bound) }; c.sortSink(r, target); cc 4
func (c *checker) bindItems(r *model.Rule, sc *scope, target *model.Stream) (map[string]bool, bool) // loop :259-304; cc 6
func (c *checker) itemType(r *model.Rule, sc *scope, item *model.SelectItem) (event.Type, bool)     // skip || before(window, pattern) → TInvalid,false; build; cc 4
func itemName(item *model.SelectItem) string                                                          // :261-264; cc 3
func binding(target *model.Stream, name string, i int, bound map[string]bool) string                 // the four E-SEL-001 texts, "" when it binds; cc 5
func (c *checker) itemValue(r *model.Rule, sc *scope, item *model.SelectItem, typ event.Type, target *model.Stream) // E-TYPE-001, E-NULL-003, E-SEL-002 in that order; cc 7
func (c *checker) outputTs(target *model.Stream)                                                      // :308-313 (badTs write with its guard); cc 5
func (c *checker) unboundAttrs(r *model.Rule, target *model.Stream, bound map[string]bool)            // :314-321 minus the break; cc 4
func (c *checker) sortSink(r *model.Rule, target *model.Stream)                                       // :322-326, window recomputed from r; cc 5
```
`bindItems` per item: `typ, typed := c.itemType(...)`; `!typed` → `broken = true; continue`; target nil → continue; `name := itemName(item)`; `if msg := binding(...); msg != "" { c.report("E-SEL-001", item.Pos, "%s", msg) } else { c.itemValue(...) }`; `if name != "" { bound[name] = true }`. `var typ event.Type`'s zero is `TInvalid` (`internal/event/event.go:17`). **T10** `!c.skipExpr && !c.before` → `skip || before` in `itemType`, short-circuit kept; **T11** the `else` splitting the first-match switch between `binding` and `itemValue`; **T12** the `break` on `broken` (`:315-317`) → the guard `if !broken` (equivalent: nothing in the loop changes `broken`). `badTs` (P): `outputTs` runs exactly when it does today. Proof: H8–H10, the existing `TestOutputAttributes` (`outputs_test.go:12`).

**2.14 `graph`** (`graph.go:15`; cc 30, 83 lines).
```go
func (c *checker) producersAndReaders() (map[string][]*model.Rule, map[string]bool)               // :16-28; cc 5
func (c *checker) streamClasses(producers map[string][]*model.Rule, readers map[string]bool) bool // per stream: if c.classify(...) {found = true}; c.timeAttr(s); c.invisiblesDecl(s); cc 3
func (c *checker) classify(s *model.Stream, prod []*model.Rule, read bool) bool                   // TimeNone → c.untimedStream(s, prod); false; else c.timedStream(s, prod, read); cc 2
func (c *checker) untimedStream(s *model.Stream, prod []*model.Rule)                              // len 0 → E-TIME-001; len>1 → E-GRAPH-003 per prod[1:]; cc 4
func (c *checker) timedStream(s *model.Stream, prod []*model.Rule, read bool) bool                // len 0 → false; read||@source → E-GRAPH-002 per prod, true; else E-TIME-002, false; cc 5
type streamRef struct{ name string; at model.Pos }
func streamRefs(r *model.Rule) []streamRef                                                         // sources, then partition keys (:69-80); cc 3
func (c *checker) producerOf(name string, producers map[string][]*model.Rule) *model.Rule          // the `derived` closure :54-59; cc 4
func (c *checker) derivedReads(producers map[string][]*model.Rule) bool                            // :60-90; cc 6
func (c *checker) unexplainedCycle(graphFound bool)                                                // :94-97; cc 3
```
`graph`: `producers, readers := c.producersAndReaders()`; `sourceWritten := c.streamClasses(producers, readers)`; `derivedRead := c.derivedReads(producers)`; `c.unexplainedCycle(sourceWritten || derivedRead)`; `c.idle()`; cc 2. Splitting `classify` re-associates the 4-case switch (`:33-50`) — equivalent (untimed: only cases 1 and 4 can match; timed: only 2 and 3); every operand row in §3.2. Both accumulators run to completion: **never** `found = found || c.classify(...)` or `c.streamClasses(...) || c.derivedReads(...)` inline. **T13** `streamClasses`' result; **T14** `derivedReads`' result. Proof: H11, the existing "E-GRAPH-002 E-GRAPH-002" case.

**2.15 `cycle`** (`graph.go:134`; cc 13, 45 lines; pure; signature `func cycle(rules []model.Rule) []int` kept). `func feedEdges(rules []model.Rule) [][]int` (`:135-144`, cc 6); the search with its `visit` closure stays (cc 8, ~32 lines). No transfer crosses.

**2.16 `idle`** (`graph.go:191`; cc 12, 32 lines). `func timerReason(r *model.Rule, streams []string) string` (the `why` switch `:199-211`, `""` for the default; cc 5); `idle`: `why := timerReason(r, streams); if why == "" { continue }` — **T15** the default's `continue`; cc 9. `reported` stays local, written before the report.

**2.17 `names`** (`names.go:49`; cc 21, 62 lines).
```go
func (c *checker) primaryKey(t *model.Table)          // :55-74 with its continue; cc 3
func (c *checker) keyColumn(t *model.Table, k string) // :61-73; cc 5
func (c *checker) ruleNames(r *model.Rule)            // sourceNames, partitionNames, targetName, sortAttr, captures — today's order; cc 1
func (c *checker) sourceNames(r *model.Rule)          // :77-82 (poisoned); cc 3
func (c *checker) partitionNames(r *model.Rule)       // :83-97 (badKey); cc 5
func (c *checker) targetName(r *model.Rule)           // :98-101 (noTarget); cc 2
func (c *checker) sortAttr(r *model.Rule)             // :102-107 (poisoned); cc 5
```
`names` keeps three loops (annotations, `primaryKey`, `ruleNames`); cc 4. Poison written here, read only by later phases (P).

**2.18 `resolve`** (`resolve.go:14`; cc 11, 41 lines). **Order matters (P):** defaults, then overrides in sorted name order, then the slots. `func (c *checker) defaults()` (`:15-24`, cc 3); `func (c *checker) overrides()` (`:25-42`, cc 5); `func (c *checker) resolveRule(r *model.Rule)` (`:47-52`, cc 3); `resolve`: `c.defaults(); c.overrides(); annotations loop; for _, r := range c.rules() { c.resolveRule(r) }`; cc 3. Proof of order: H14.

**2.19 `resolveExpr`** (`resolve.go:174`; cc 12, 40 lines). Argument recursion first, then a 2-case switch calling `c.resolveLiteral(n, root)` (`:180-184`, cc 2) or `c.resolveOperand(n, root)` (`:186-211` with its four returns, cc 6); cc 6. **T16** the returns now exit `resolveOperand` — equivalent (the case was `resolveExpr`'s tail). Proof: H15.

### 3. Nets (the V1 nets commit, before any split)

**3.1 Coverage now (measured, in-package; package 97.5 %).** At 100 %: `zeroDurations`, `ttl`, `clauses`, `annotations`, `expectBlock`, `givenRecord`, `nulls`, `unsupported`, `selects`, `graph`, `cycle`, `idle`, `names`, `resolve`. Short:

| Function | % | Uncovered block | Reachable? | Test to add |
|---|---|---|---|---|
| `partition` | 98.6 | `bounds.go:142-144` (E-PART-008 at the part of a grouped key) | yes, via `Check` | `TestOperands/partition_object_part`: `partition by (k, o) of S`, `o object` |
| `before` | 90.5 | `exprs.go:86-88` (`chain()` outside a pattern select) | yes | H6 |
| `build` | 92.6 | `exprs.go:107-109` (an error that is not an `*expr.Error`) | not via `Check` (every builder error is an `*expr.Error`, `internal/expr/expr.go:70-80`, `nodes.go:35`; inferred); yes by a direct call: `expr.Builtins()` returns a fresh registry (`registry.go:96-104`), so a test can `Register` a descriptor whose `Factory` returns `errors.New` | `TestABuildErrorWithoutACodeIsETYPE001` (direct `c.build` on `&checker{reg: r}`) |
| `mayBeNull` | 89.3 | `exprs.go:363-364` (`text_invisibles` is never null) | yes | `TestOperands/invisibles_select` |
| `mayBeNull` | | `:367-368` (an attribute the scope cannot place), `:400` (an unknown kind) | not via `Check` (runs only on an item just built with the same scope, `exprs.go:268, 290`; the builder resolves through `scope.Attr`, `expr.go:176-178`, and refuses unknown kinds, `expr.go:253`); yes by a direct call | `TestMayBeNullIsConservative` (direct) |
| `resolveExpr` | 92.3 | `resolve.go:189-191` (an undeclared param in a text operand) | yes | H15 |

Not flagged but touched (no bar): `fits` 87.5 % (`resolve.go:72`), `kindName` 66.7 %, `thenRecord` 84.6 %, `hasAnnotation` 50 % (`validate.go:216` closure). **A gap block coverage hides:** `hasAnnotation`'s closure never runs, so no in-package test has a timed, produced, unread stream with any annotation, and the operand `hasAnnotation(s, "source")` (`graph.go:36`) has never decided a result — rows G6, G7.

**3.2 Operand cases** (`internal/validate/operands_test.go`, `TestEachConditionOperandDecides`, table-driven through the parser like `units_test.go:31`, reusing its `base` preamble). Procedure: replace each operand in a scratch copy with `true`, then `false`; where every test stays green, add the row; quote each red in the record. Rows [expected findings]:
- **zeroDurations `:181`:** a slot from an undeclared param [E-PARAM-001 only]; a negative override [E-DUR-003 "negative"]; a written `0 ms` [E-DUR-003 "zero"]; an absent slot [none]; positive [none].
- **ttl `:191`:** no partition with `output first every` [no E-TTL-002]; `ttl ${nope}` with `within 5 min` [E-PARAM-001 only]; `ttl 0` (exists); `ttl 1 min` with `within 5 min` [E-TTL-002]. **ttl `:215`:** `within ${nope}` with `ttl 1 min` [E-PARAM-001 only].
- **clauses `:252`:** batch over a stream with no `@source` [none]; punctuating [none]; non-punctuating (exists, `units_test.go:284`).
- **annotations `:99`, `:108`:** `@source` with no `type` [no type finding]; an unknown type [E-NAME-001 at the value, keys not checked]; a known type with an unknown key [E-NAME-001 at the key].
- **givenRecord `:89`, `:97`, `:99`, `:106`:** a given table with a `text_invisibles` column [not "computed"]; a stream field `text_invisibles` [the "computed" mistake]; an optional non-time attribute given null [none]; an optional time attribute given null ["required; gives it null"]; an optional time attribute left out ["leaves it out"]; a declared `text_invisibles` left out of a stream record [none].
- **before `:65`, `:71`, `:81`, `:85`:** `in` against an unknown table [E-NAME-001 from build]; `in` against a one-column table [none]; a text operator against an unknown table or a one-column table; `count()` in a window select [none]; `chain()` in a pattern select [none].
- **build `:119`:** a bare name a capture has, in a pattern select [hint]; a qualified unknown name in a pattern select [no hint]; a bare unknown name in a plain rule [candidates].
- **nulls `:151`, `:158`, `:167`:** `coalesce(note,'x') is null` (not an attribute); `!=` outside an absence [E-NULL-002]; `<` outside an absence [none]; `<` inside an absence [E-NULL-001]; an optional attribute already tested `is null` [none].
- **unsupported:** §3.3's regex table.
- **selects:** `:262` an aliased attribute vs a bare non-attribute item; `:290` an optional target vs a non-optional target fed by an optional attribute; `:294` an aggregating vs a non-aggregating window; `:308` each of the four operands (no `ts`, a timed target, `ts` declared timestamp, a second rule into the same stream); `:322` no sink, a changelog sink, a non-window rule, a non-sort window.
- **mayBeNull `:362`, `:376`:** `raw(text_invisibles)` vs `text_invisibles`; the rule's own partition key vs another stream's key of the same name.
- **graph:** G1–G5 untimed with 0, 1, 2 producers; timed with 0 producers; timed, 1 producer, read; **G6** timed, 1 producer, unread, `@source` [E-GRAPH-002]; **G7** timed, 1 producer, unread, `@sink` only [E-TIME-002]; `:55` a timed stream with a producer is not derived; `:83` one rule reading a derived stream twice [one E-GRAPH-004]; `:94` by H11.
- **cycle `:139`:** a rule reading one producer's output from two steps [one edge] (direct `cycle` call, as `TestCycle`).
- **idle `:204`, `:214`:** a plain rule over one clocked stream without `idle after` [none]; a pattern rule without an absence [none]; `idle after 0` written [E-DUR-003, no E-IDLE-001]; two timer rules on one stream [one E-IDLE-001].
- **names `:102-103`:** a sort window over an unknown stream [one E-NAME-001].
- **resolveExpr `:179`, `:185`:** a literal that is not a param; a text operand with no param.

**3.3 Two-fault and direct tests.** `TestOneFindingHidesAnother` (`internal/validate/hiding_test.go`), each row the exact sorted list of `code@line:col`:
- **H1** `partition by nope of S` in a rule reading S and R → E-NAME-001 only.
- **H2** `partition by (k, n) of S, (k) of R` → E-PART-007 only (the lane checks `TestGroupedKeyFindings` first).
- **H3** `S[count() > 'a']` → E-FN-001 only.
- **H4** (exists) a composite-key `in`.
- **H5** a text operator against `Pair` → E-TBL-001 only.
- **H6** `from S select chain(k) as k` → E-FN-001 only.
- **H7** a pattern select with a bare `k` → one E-NAME-001 with "did you mean a.k?".
- **H8** `select k + 1 as k` → E-TYPE-001 only (no "Out.k is bound by no select item").
- **H9** `select k, n as k` → E-SEL-001 "bound twice" only.
- **H10** `select count() + ${nope} as k` → E-PARAM-001 only.
- **H11** a written source plus a derived read → E-GRAPH-002 and E-GRAPH-004.
- **H14** `param p int = 5`, override `'x'`, `S[k == ${p}]` → E-PARAM-001 only.
- **H15** a text operator over `${nope}` → E-PARAM-001 only.
- **H17** a Go-built model whose plain body carries `Window{Kind: sort, Emit: "due"}` → no E-EMIT-002 (the style of `TestAGoBuiltModel`, `units_test.go:357`).
- **H12** (v2, QA DQ-S2) an expect block on a rule whose target is unknown → only E-NAME-001 (the `noTarget` poison, `names.go:100`).
- **H13** (v2) a sort window over an unknown sort attribute plus a second fault in the same rule → one E-NAME-001 (the sort-attribute poison, `names.go:105`).
- **`TestCheckResolvesEveryParamSlot`** (v2): exact values in the App `Check` returns — a filter literal from `${p}`, a single text operand, a keyword set `"a, b"` → `["a","b"]`, a duration slot, an annotation value.

H5's and H15's operator spellings are inferred; the lane takes them from the existing `${terms}` cases. Direct tests: `internal/validate/regex_test.go` `TestUnsupportedNamesTheFirstNonRE2Construct` (`\1`, `\9`, `\0` [none], `\a` [none], `\k<x>`, `\(?=x)` [none, the escaped parenthesis], a trailing `\`, `(?=`, `(?!`, `(?<=`, `(?<!`, a clean pattern, two constructs [the first offset]); `internal/validate/nullable_test.go` `TestMayBeNullIsConservative` (on `&checker{reg: expr.Builtins()}` with `&scope{bare: -1}`: an unresolvable attribute → true; `Kind: "bogus"` → true; a `LitNull` literal → true — never typeable through `Check`, `nodes.go:37`, `LitNull` "only in expect records", `model.go:113`); `internal/validate/builderror_test.go` `TestABuildErrorWithoutACodeIsETYPE001` (`Descriptor` fields from `registry.go:107-115`; `Kind: expr.Scalar`; factory `errors.New("boom")` → E-TYPE-001 at the node, message "boom"; inferred).

**3.4 Mutations, one per transfer, each shown red with its diff in the record:** T1–T2 (partition): H1, H2; T3 (windowClauses): H17; T4–T7 (before): H4, H5, H3, H6; T8 (captureHint): H7; T9 (unsupported `continue`): `\(?=x)`; T10 (itemType short-circuit): H10; T11 (binding `else`): H9; T12 (broken guard): H8; T13–T14 (accumulators): the existing "E-GRAPH-002 E-GRAPH-002" and H11; T15 (idle default): the plain-rule row; T16 (resolveOperand returns): H15; the defaults-before-overrides swap: H14; (v2, QA DQ-S2) the poison writes `names.go:80` (`TestOneFindingPerDefect`), `:100` (H12), `:105` (H13) and the four slot writes (`TestCheckResolvesEveryParamSlot`) — or an equivalent mutant named with its argument; one mutation per new test besides. A mutation that stays green gets a new row or is named equivalent with its argument.

**3.5 Behaviour nets at hand-back (plan check §7 row V):** `go test ./internal/validate/...` (TestStatic, TestCorpusClean, the negative tests); `./conformance/...`; `docsync-b-catalog` (`docsync_test.go`, a law path, read-only); 0 size findings under the pinned golangci-lint for both `GOOS`; `astequal <c>^ <c>` exit 0 for commit 2. No performance net: none of these is on the hot-path list.

### 4. Placement

**4.1 The literal-to-value rule: one `internal/model` function.** New file `internal/model/literal.go`:
```go
// LitType is the type a literal of kind k has on its own: event.TInvalid for a null or a param, which have none.
func LitType(k LiteralKind) event.Type                      // 6 cases; cc 7
// LitValue is l as a value of type t: a literal of t's own type, or an int literal for a float, which widens. ok is
// false for every other pair (a null, a param, a string for an object, a mismatch); each caller says why in its own words.
func LitValue(l Literal, t event.Type) (event.Value, bool) // Int&&TFloat → widen; t==TInvalid || LitType(l.Kind)!=t → false; else litPayload; cc 5
func litPayload(l Literal, t event.Type) event.Value         // switch t: Str/I/F/B/D(Ms)/T(Ms), exactly the fields plan and expr set today; cc 7
```
- **`validate.fits`** (`resolve.go:57-73`) → `_, ok := model.LitValue(l, t); return ok`. Identical truth table (string fits only string, so `TObject`+string stays false and `blockValue` keeps handling objects first, `expects.go:144`; null and param false). `validate.as` stays (Literal → Literal for the copy). All E-PARAM-001 and E-EXP-002 texts stay in validate.
- **`plan.value`** (`internal/plan/expect.go:181-204`, cc 18, one of the 110, lane Q) → `case LitNull` typed null; `case LitString && TObject` → `CleanObject` (owner 2026-11-10, kept); `if v, ok := model.LitValue(l, t); ok { return v, nil }`; then the same `fmt.Errorf("a %s literal is not a %s", l.Kind, t)`. cc 5, so plan's finding goes by design; the cases are disjoint, only null-first ordering matters, and it is kept.
- **`expr.literalNode`** (`internal/expr/nodes.go:20-37`, not flagged) → `LitParam` → the same E-PARAM-001 `*Error`; `if v, ok := model.LitValue(lit, model.LitType(lit.Kind)); ok { return &literal{v}, nil }`; then the same `typeErr("a %s literal cannot appear in an expression", lit.Kind)`.
- Import edges (`tools/importcheck/rules.json`): model → event exists; validate, plan, expr → model exist; expr's fence allows the `Lit` prefix (`:36`), which `LitValue` and `LitType` match. **No new edge.** Tests that move: none (no test calls `fits`, `as`, `value` or `literalNode` directly; grep).
- Proof (a merge, not a move, so `astequal` and the rename map do not apply): the domain is finite: 9 literal kinds (8 named plus an unknown `"bogus"`) × 9 types (8 named plus an out-of-range `Type(99)`) = 81 pairs (v2, DA-S14, QA DQ-S11), with `(LitParam, TInvalid)` → false. **A** pin each caller's table, green before the change, in its own package (`TestFitsEveryKindAndType`, validate; `TestValueEveryKindAndType`, plan, values and error texts; `TestLiteralNodeEveryKind`, expr — assumes expr's tests are internal, inferred); **B** add `model.LitValue`, `LitType`, `litPayload` with `TestLitValueEveryKindAndType`, red first in a scratch run; **C** switch the three callers; `git diff A..C` on A's test files empty and green; then T2. Three coordinator commits on the integration branch before lanes V and Q branch (the plan's "moves ≤ 3").
- **v2 recommendation (DA-B1, DA-L9, DA-L10): validate and plan only, by lane M (a `go-engineer`, the coordinator verifying), before lanes V and Q; `expr.literalNode` stays as it is.** expr's import of `model` is fenced by an owner decision to "the model's expression types … expr reads `model.Expr` only" (`docs/sprints/decisions.md:274`); `LitValue` is behaviour and passes the checker only by its `Lit` name prefix (`tools/archcheck/main.go:500-527`), so the expr leg would need that decision amended. The correctness benefit is all in validate↔plan (validate's check accepts exactly what plan converts, by construction). `validate.as` keeps its own int→float widening for the copy, so the rule lives in two places, not one (DA-L9). expr's tests are all external (`package expr_test`), so an expr pin would go through the public builder (DA-S14). *Options, healthiest first:* (1) validate and plan now, expr untouched; (2) all three, with the fence decision explicitly amended (annotating `decisions.md:274` and the `rules.json` note, a LAW Δ line); (3) no merge, split `plan.value` in place.

**4.2 Others.** None of validate's 19 belongs elsewhere. `deepCopy`/`copyValue` (`validate.go:147-172`) could become a model clone (one caller: backlog). `nearest`/`distance` have no twin (`playback.nearest`, `expect.go:215`, is another concept). `setOp` (`resolve.go:165`) mirrors parser keywords but is validate's own split rule (owner, 2026-11-04): stays. Nothing elsewhere belongs in validate (the two `validateApp`: PA-S5).

### 5. Risks and owner questions (recommendation first)

1. **Q-V1 — the literal-to-value merge (4.1).** → (v2) validate and plan in S1.1c-6, lane M's three commits before lanes V and Q; expr untouched (§10 Q1).
2. **Q-V2 — blocks reachable only by a direct in-package call** (`build` `:107-109`; `mayBeNull` `:367-368`, `:400`, its `LitNull` operand). → accept direct-call characterisation tests as meeting decision 2's "from its own package's tests". Others: list them with a "defensive default behind the builder's contract" reason (decision 2 names only an invariant panic or scheduling, so the owner answers at G1); remove the defaults (changes Go-built models' behaviour: no).
3. **Q-V3 — the V commit cut.** → (v2, DA-S11) V1 = the nets (1 commit); V2 = the move and the phase 2–5 splits (5); V3 = the phase 6 and expects splits (6).
4. **Risk:** re-associating `graph`'s switch and `selects`' first-match switch. *Mitigation:* G1–G7, H8–H11, T11–T14.
5. **Risk:** a helper accumulator written with `||` (`streamClasses`, `derivedReads`, `partCounts`) would skip findings. *Mitigation:* forbidden by name; T13, T14.
6. **Risk:** equivalent mutants (T3 observable only on an inconsistent Go-built model). *Mitigation:* H17; any other named in the record with its argument.
7. **For the coordinator:** plan check §6.1's "return the first error found" is wrong for validate (§0); the break-it tests multisets.

### 6. Commit plan (12 commits; each at or below the cap at the hook: gocyclo 19 / funlen 3)

| # | Sub-lane | Commit | gocyclo / funlen after |
|---|---|---|---|
| 1 | V1 | `test(validate)`: the nets (`hiding_test.go`, `operands_test.go`, `regex_test.go`, `nullable_test.go`, `builderror_test.go`); every flagged function at 100 % of its blocks; mutations quoted | 19 / 3 |
| 2 | V2 | `refactor(validate)`: `exprs.go` → `exprs.go`, `nulls.go`, `regex.go`, `selects.go` (pure move, `astequal` exit 0) | 19 / 3 |
| 3 | V2 | split `names` | 18 / 3 |
| 4 | V2 | split `resolve`, `resolveExpr` | 16 / 3 |
| 5 | V2 | split `graph`, `cycle`, `idle` | 13 / 2 |
| 6 | V2 | split `partition`, `zeroDurations`, `ttl`, `clauses` | 9 / 1 |
| 7 | V3 | split `annotations` | 8 / 1 |
| 8 | V3 | split `expectBlock`, `givenRecord` | 6 / 1 |
| 9 | V3 | split `before`, `build` (adds `callName`) | 4 / 1 |
| 10 | V3 | split `nulls`, `mayBeNull` (uses `callName`) | 2 / 1 |
| 11 | V3 | split `unsupported` | 1 / 1 |
| 12 | V3 | split `selects` | 0 / 0 |

Each split commit leaves the original ≤ 10 and every helper ≤ 10 and ≤ 60 lines, with no lll, revive or other finding. No test file is edited after commit 1 (`git diff <c1>..HEAD -- internal/validate/*_test.go` empty). Hand-back: 0 size findings in `internal/validate` under both `GOOS`; `go test ./internal/validate/... ./conformance/...` green; every commit through T0; then the coordinator removes the two `internal/validate` entries and runs T2. The literal merge (4.1), if approved, is lane M's three commits **before** commit 1.

**Gaps:** H5's and H15's parser spellings and H17's Go-built model are the lane's; which operand rows already exist is settled by §3.2's mutation procedure; projected line counts are estimates (~4–6 lines of signature and doc per helper).

## 6. internal/event (lane E: E1 → E2 → E3)

*Drafted read-only on `s1-1c-6/integration` @ 6bf339c by the refactoring specialist (gitnexus-refactoring, gitnexus-impact-analysis, architecture-designer); production Go identical to `main` 4d2b9eb. "Measured" = a run with the pinned Go 1.27.4; "scratch" = an experiment on a copy of `internal/event` in a module outside the repo (`ep-design/exp/` in the coordinator's scratchpad). Hand-counted cyclomatic values are inferred; the lanes confirm them with the pinned golangci-lint. The coordinator verified the caps, `colon`'s two callers (`decode.go:564, :650`), `decode.go:818`/`:834`, and re-ran the scratch re-split's `TestDifferential` (ok).*

Caps (`tools/allowlists/smells.json`, S1.1c-5-2): file length 1, funlen 2, gocyclo 12; 15 findings. Hot path: all 11 decode functions (plan check §3); `NewSchema` is deploy-time. Imports: `internal/event` imports nothing (`tools/importcheck/rules.json`); every move stays inside the package.

### E.1 File map

| File (concern) | Declarations | Moved from | Lines moved → final (projected) |
|---|---|---|---|
| `decode.go` — the payload decoder: limits, `Decoder`, `Decode`, per-attribute values, type errors | `InvisiblesAttr`, `MaxDepth`, `MaxLogLine`, `Decoder`, `NewDecoder`, `Decoded`, `DecodeReason` + consts, `DecodeError`, `(*DecodeError).Error`, `span`, **`(*Decoder).Decode` (pinned, `gate/proofs/drivers/test_fuzz_decoder_planted.py:20`)**, `record`, `value` + its new helpers, `timeNote`, `typeErr`, `excerpt`, `jsonKind`, `parseInt` | stays: `decode.go:1-201`, `:328-398` | 272 → ~330 |
| `object.go` — an object value's stored form | `cleanObject`, `CleanObject`, `CleanLen`, `ExactObject`, `compactAndValid`, new `isSpace` | `decode.go:203-297`, `:302-326` | ~126 → ~132 |
| `unescape.go` — a JSON string token's decoded text | `decodeString`, `needsUnescape`, `appendUnescaped` + its new helpers, `hex4` | `decode.go:400-489` | ~96 → ~130 |
| `scanner.go` — the one-pass JSON syntax check | `scanner`, `syntax`, `ws`, `document`, `top`, `value`, `enter`, `object`, `array`, `str`, `isHex`, `isDigit`, `number`, `digits`, `literal` + new `at`, `atEither`, `member`, `unicodeEscape`, `exponent` (`colon` folded into `member`, E.2.5) | `decode.go:491-808` | ~322 → ~360 |
| `rfc3339.go` — RFC 3339 timestamps and their range | `TimestampInRange`, `parseRFC3339` + its helpers, `minRFC3339`/`maxRFC3339`, `daysIn`, `daysFromCivil` (`digitsAt` removed by the re-split) | `decode.go:299-300`, `:810-936` | ~133 → ~185 |
| `event.go` — types, schema, value, order | unchanged, plus `NewSchema`'s two helpers | — | 192 → ~205 |

The pure move comes first and alone (commit E2, `astequal` exit 0), then the splits. Imports per file: `decode.go` `fmt`, `math`, `strconv`; `object.go` `unicode/utf8`; `unescape.go` `unicode/utf16`, `unicode/utf8`; `scanner.go` `fmt`; `rfc3339.go` none. ADR-0010 §3 names `internal/event/decode.go` as "a single-pass JSON scanner": after the move the scanner lives in `scanner.go` and `Decode` stays in `decode.go`; the coordinator annotates the ADR at combine (plan check §4, PA-S11).

### E.2 Split plan per finding

GitNexus `impact` upstream (with tests):

| Function | Risk | Impacted | d=1 callers |
|---|---|---|---|
| `parseRFC3339` | HIGH | 14 | `value` |
| `(*Decoder).value` | HIGH | 22 | `Decode` |
| `cleanObject` | CRITICAL | 24 | `value`, `CleanObject` |
| `CleanLen` | LOW | 4 | `(*Runtime).SendPayload` (`internal/runtime/boundary.go:119`) |
| `compactAndValid` | LOW | 3 | `cleanObject`, `CleanLen`, `ExactObject` |
| `appendUnescaped` | LOW | 3 | `decodeString`, `record`, `value` |
| `(*scanner).top` | HIGH | 14 | `document` |
| `(*scanner).value` | CRITICAL | 24 | 5 |
| `(*scanner).object` | HIGH | — (a lower bound: 45 call sites dropped by receiver typing; grep: one caller, `decode.go:605`) | `value` |
| `(*scanner).str` | HIGH | 10 | 3 |
| `(*scanner).number` | HIGH | 10 | 1 |
| `NewSchema` | CRITICAL | 206 | 24 |

No flagged function changes its signature, so no caller is edited. The only symbol deleted is `colon` (callers `decode.go:564`, `:650`, both folded into `member`).

**Inline costs today** (measured, `-gcflags=-m=2`; budget 80): inlined `ws` 24, `isDigit` 8, `isHex` 24, `hex4` 55, `digitsAt` 51, `daysIn` 35, `TimestampInRange` 8, `jsonKind` 25, `timeNote` 6; not inlined `daysFromCivil` 83, `parseRFC3339` 930, `value` 1324, `top` 652, `object` 590, `(*scanner).value` 602, `str` 522, `number` 415, `appendUnescaped` 663, `cleanObject` 292, `CleanLen` 265, `compactAndValid` 178, `colon` 136. **Bounds checks today** (measured, `-d=ssa/check_bce/debug=1`), 65 in `decode.go`: `parseRFC3339` 3 (`:856, :860, :864`) plus `digitsAt` 1 (`:901`); `top` 7, `appendUnescaped` 7, `number` 6, `object` 5, `value` 4, `cleanObject` 3. The BCE diff is a diagnostic only.

#### E.2.1 `parseRFC3339` — cc 43, 70 lines, hot

**Order of checks:** every rejection returns `(0, false)`, with no error offset, and no reordered check can index out of range; the order of rejection is unobservable. What must stay equal is the accept set and the value (proof: the spike's differential and the 50-case table).

**The one inliner-shaped re-split to try** (built in scratch, iteration 3, `ep-design/exp/ev/decode.go`; gave the original's results — values and accept/reject, not its time — on 2,115,410 inputs — every truncation, every 1-byte substitution and insertion, two-position substitutions over 20 seeds — `ep-design/exp/ev/diff_test.go`, measured and re-run by the coordinator):
```go
// rfc3339.go
func parseRFC3339(t []byte) (int64, bool) {
	h, ok := rfc3339Head(t)
	if !ok || !dateDigits(h) || !clockDigits(h) {
		return 0, false
	}
	year, month, day := rfc3339Date(h)
	hour, minute, sec := rfc3339Clock(h)
	if !dateExists(year, month, day) || !clockValid(hour, minute, sec) {
		return 0, false
	}
	ms, i, ok := rfc3339Fraction(t)
	if !ok {
		return 0, false
	}
	ms *= fractionScale(i - len("2006-01-02T15:04:05.")) // no fraction: i = 19, scale 1, ms 0
	offset, ok := rfc3339Offset(t[i:])
	if !ok {
		return 0, false
	}
	if sec == 60 { // a leap second: the last millisecond of its minute
		sec, ms = 59, 999
	}
	secs := ((daysFromCivil(year, month, day)*24+hour)*60+minute-offset)*60 + sec
	v := secs*1000 + ms
	if !TimestampInRange(v) { // an offset carried the instant out of RFC 3339's years (break-it K2, owner)
		return 0, false
	}
	return v, true
}
```
`parseRFC3339` cc 10, ~30 lines (hand count, inferred).

| Helper (full signature) | Concern | Inline cost (scratch, measured) | cc (hand) |
|---|---|---|---|
| `func rfc3339Head(t []byte) (*[19]byte, bool)` | length ≥ 20, dashes, `T`/`t`, colons; returns `(*[19]byte)(t)` | 57 ✓ | 6 |
| `func dateDigits(h *[19]byte) bool` | `max(h[0]-'0', …, h[9]-'0') <= 9` over the 8 date digits | 52 ✓ | 1 |
| `func clockDigits(h *[19]byte) bool` | the same over the 6 clock digits | 40 ✓ | 1 |
| `func num2(hi, lo byte) int64` | `int64(hi)*10 + int64(lo) - '0'*11` | 10 ✓ | 1 |
| `func rfc3339Date(h *[19]byte) (year, month, day int64)` | the values | **84 ✗** (year = `num2*100+num2`); read the year as one expression `int64(h[0])*1000+int64(h[1])*100+int64(h[2])*10+int64(h[3]) - '0'*1111`, projected under 80 (inferred) | 1 |
| `func rfc3339Clock(h *[19]byte) (hour, minute, sec int64)` | the values | 61 ✓ | 1 |
| `func dateExists(year, month, day int64) bool` | month 1–12, day 1–`daysIn` | 54 ✓ | 4 |
| `func clockValid(hour, minute, sec int64) bool` | hour ≤ 23, minute ≤ 59, sec ≤ 60 | 12 ✓ | 3 |
| `func rfc3339Fraction(t []byte) (ms int64, next int, ok bool)` | the optional `.digits` at `t[19:]`, the first three kept | 64 ✓ | 6 |
| `func fractionScale(n int) int64` | 100, 10 or 1 | 12 ✓ | 3 |
| `func rfc3339Offset(z []byte) (int64, bool)` | `Z`/`z` when `len(z) == 1`, else the numeric offset | **88 ✗** — one non-inlined call per timestamp | 3 |
| `func rfc3339NumOffset(z []byte) (int64, bool)` | `±HH:MM` exactly, hour ≤ 23, minute ≤ 59 | **120 ✗** — a second call only for numeric offsets | 6 |
| `func signedOffset(sign byte, minutes int64) int64` | the sign | 9 ✓ | 2 |

- **Bounds checks:** 4 → 2 (scratch, measured: one at `t[i:]`, one in `rfc3339Fraction`); the spike's split went 4 → 12. Low because the head is read through a `*[19]byte` view (constant indices into an array pointer need no check; the conversion is guarded by the length test).
- **Escapes:** no "escapes to heap" or "moved to heap" line in the new range (scratch, measured). **Allocations:** 0, as today.
- **Residual cost against the spike:** one non-inlined call per timestamp (two for a numeric offset), against the spike's four, with fewer bounds checks than today.
- **Control transfers crossing a new boundary:** each helper's `false` becomes the caller's `return 0, false` (mutation: each helper's result inverted, e.g. `clockValid`'s `sec <= 60` → `sec < 60`, red by "a leap second reads as :59.999"); the fraction's `case 0` rejection is now `ok = i > 20` (mutation `i >= 20`, red by `"…00.Z"`); the offset's trailing-byte rejection (old `:876`) is now `len(z) == 1` (mutation `len(z) >= 1`, red by `"…Z "`); the digit count's scale (old `:843-850`) is now `fractionScale` (mutation case 1 → 10, red by `".5"`).
- **Verdict path:** (1) the lane commits the re-split with the spike's `BenchmarkParseRFC3339` from the nets commit unchanged; (2) the coordinator applies decision 1 (prebuilt binaries of the nets commit and the split commit, 10 interleaved pairs, re-run once); (3) **if it costs**, the function is restored by a new commit on the lane branch (§2.4): the original function in `rfc3339.go` plus, on its `func` line, `//nolint:gocyclo,funlen,lll // PERMANENT reason="split costs +<x> % median (Mann-Whitney p=<p>, 10 pairs, re-run <x2> %), allocations 0 = 0; samples in s1-1c-6-record.md" ref=docs/sprints/decisions.md#owner-decisions-2026-11-14--s11c-5s-plan-check sprint=S1.1c-6` (precedent `internal/table/table.go:240`), citing both measurements (the spike's +19.7 %, `docs/reviews/s1-1c-6-spike.md`, and this attempt's).

#### E.2.2 `cleanObject` 13, `CleanLen` 13, `compactAndValid` 12 — hot: three separate splits, no shared walker

**Recommendation:** three separate splits sharing one inlinable leaf, `func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }` (inline cost 16, measured). **Against one shared walker:** the three loops produce different things (a `bool` with early exits, a count, a built string); one walker needs a per-byte callback (a func value, or an interface or generic method through a GC-shape dictionary), and Go cannot inline through a walker that is not itself inlined — one indirect call per payload byte on loops that run per recorded payload (`boundary.go:119, :129`) and per object value (`decode.go:196`), inferred ~1–2 ns per byte; `compactAndValid`'s early exits would need a stop signal. Their shared state is only `inStr`; the `cleanObject`/`CleanLen` duplication (build vs count) is kept and pinned by a property test (E.3).

Splits (no new call on the loop path): `compactAndValid` `decode.go:309`'s `c == ' ' || c == '\t' || c == '\n' || c == '\r'` → `isSpace(c)` (12 → 9); `cleanObject` `:217`'s `c != ' ' && c != '\t' && c != '\n' && c != '\r'` → `!isSpace(c)` (13 → 10); `CleanLen` `:260` the same (13 → 10). Operands evaluated in the same order; no control transfer crosses (a predicate only). One mutation per site: `isSpace` dropping `'\r'`, red by the `\r` operand case. **Margin option** (E-Q3): extracting the non-ASCII arm (`decode.go:232-239`, `:275-282`) into `appendCleanRune` and `cleanRuneLen` brings both to 8, at one non-inlined call per non-ASCII rune of a non-compact object (inferred).

*(Annotated 2026-11-18: the `isSpace` split cost `CleanLen` +23.9 % and `CleanObject` +13.2 % (an inverted branch fall-through, four taken branches more per byte); under ground rule 13a the owner chose one run walker with byte tables instead, reversing "no shared walker" above — [owner](decisions.md#owner-decisions-2026-11-18--the-architecture-review-of-lane-es-walkers-and-scanner), decision 1; [the review](../reviews/s1-1c-6-event-walkers-review.md).)*

#### E.2.3 `appendUnescaped` — cc 18, hot (only for strings with an escape or malformed UTF-8: `needsUnescape`)
```go
func appendUnescaped(dst, in []byte) []byte                   // loop: case '\\' → appendEscape; < RuneSelf → append; default → appendRune; cc 4
func appendEscape(dst, in []byte, i int) ([]byte, int)        // the escape at in[i]; b f n r t, u → appendUnicodeEscape, default '"' '\\' '/'; returns i+2; cc 7
func appendUnicodeEscape(dst, in []byte, i int) ([]byte, int) // decode.go:441-454 verbatim, `continue`s → returns (…, i+6) for a pair, (…, i) after one escape; cc 7
func appendRune(dst, in []byte, i int) ([]byte, int)          // decode.go:463-469; returns i+size; cc 3
```
Order unchanged (one byte position at a time). Transfers: (1) `continue` at `:448` (a pair: i advanced by 12) → `return …, i+6` after the first `i += 6` (mutation: return `i`, red by a surrogate-pair case); (2) `continue` at `:454` → `return …, i` (mutation `i+2`, red by `éx`); (3) `i += 2` at `:458` → `appendEscape`'s final return (mutation `i+1`, red by `\n`); (4) `r = utf8.RuneError` at `:451` (mutation removed, red by a lone surrogate). All three helpers expected over 80 (each calls `utf8.AppendRune`, `utf16.DecodeRune` or `utf8.DecodeRune`; inferred): one call per escape and per non-ASCII rune in strings that need unescaping. Escape check: `dst` stays "leaking param: dst to result" with no "escapes to heap"; `decodeString`'s `buf [128]byte` (`:407`) and `record`'s `buf [64]byte` (`:113`) stay on the stack. If decision 1 says it costs: the PERMANENT waiver on `appendUnescaped`.

#### E.2.4 `(*Decoder).value` — cc 28, 70 lines, hot
```go
func (d *Decoder) value(b []byte, i int, sp span, closing int) (Value, error)
// reserved → absent → null → switch a.Type { case TString: return stringValue(a, raw, sp.start); case TInt, TDuration: intValue;
// TFloat: floatValue; TBool: boolValue; TTimestamp: timestampValue; TObject: objectValue }; default typeErr(… "a value of a declared type" …)
// cc 10 (1 + 3 ifs + 6 cases), ~32 lines. A switch of direct calls, not a table (§5.2).
func nullable(a Attr, isTime bool) bool                         // a.Optional && !isTime; inline cost 6 (measured)
func absentValue(a Attr, isTime bool, closing int) (Value, error) // decode.go:137-142; cc 2
func nullValue(a Attr, isTime bool, at int) (Value, error)        // decode.go:146-150; cc 2
func stringValue(a Attr, raw []byte, at int) (Value, error)       // :154-157; 2
func intValue(a Attr, raw []byte, at int) (Value, error)          // :159-166; 3
func floatValue(a Attr, raw []byte, at int) (Value, error)        // :168-173; 5
func boolValue(a Attr, raw []byte, at int) (Value, error)         // :175-181; 3
func timestampValue(a Attr, raw []byte, at int) (Value, error)    // :183-193, its `var buf [64]byte` moved with it; 4
func objectValue(a Attr, raw []byte, at int) (Value, error)       // :195-198; 3
```
Order: reserved, absent, null, type, exactly as today; every `fmt.Sprintf` and `typeErr` argument moves verbatim (texts and offsets equal). Transfers: each case's two returns become the helper's results. One mutation per helper: `stringValue` dropping the `'"'` test (red by a number for a string attribute); `intValue` swapping TInt and TDuration; `floatValue` ignoring `err` (red by `1e999`); `boolValue` swapping `t`/`f`; `timestampValue` skipping the unescape (red by `"2026-01-01T10:00:00Z"`); `objectValue` refusing `[`; `absentValue`/`nullValue` with `nullable` inverted, plus the time note. Inline: every per-type helper calls a non-inlined function (> 80, inferred): one extra call level per present attribute and an 11-word `Value` returned through memory (inferred) — the split most likely to show time. If decision 1 says it costs: the PERMANENT waiver on `value` (no cheaper shape keeps cc ≤ 10).

*(Annotated 2026-11-18: the split above cost `BenchmarkDecoderValue` +37 %: `Value` exceeds the compiler's 4-field register limit, so every helper that returns one adds an 88-byte store, copy and reload. "No cheaper shape keeps cc ≤ 10" no longer holds: under ground rule 13a the owner chose the review's option O1, readers returning plain values into `value`'s named result, with an exception to decision 1 for its measured +4.7 % to +5.9 % leaf cost — [owner](decisions.md#owner-decisions-2026-11-18--the-review-of-decodervalue-and-the-scanners-u-cost), decision 1; [the review](../reviews/s1-1c-6-decoder-value-review.md).)*

#### E.2.5 The scanner — `top` 12, `value` 11, `object` 13, `str` 11, `number` 17, all hot

New inlinable predicates (measured):
```go
func (s *scanner) at(c byte) bool          { return s.i < len(s.b) && s.b[s.i] == c }                    // cost 15
func (s *scanner) atEither(a, b byte) bool { return s.i < len(s.b) && (s.b[s.i] == a || s.b[s.i] == b) } // cost 23
```
**`member`** (new; folds `colon`, `decode.go:588-596`):
```go
// member reads one `"name": value` pair from s.i, whitespace first, and leaves s.i just after the value; it returns
// the name's text between its quotes and where the value starts.
func (s *scanner) member() (key []byte, vstart int, err error)
```
Body: `ws`; `!s.at('"')` → "want a member name"; `str`; `key = s.b[kstart+1 : s.i-1]`; `ws`; `!s.at(':')` → "want ':'"; `i++`; `ws`; `vstart = s.i`; `value`. cc 5. Calls per member unchanged (`member` + `str` + `value` against today's `str` + `colon` + `value`).

Splits: **`top`** — `if s.at('}')`, the loop `key, vstart, err := s.member()`, `d.record(…)`, the separator switch inline (`:572-584` verbatim): 12 → 7. **`object`** — `enter`, `if s.at('}')`, the loop `if _, _, err := s.member()`, the separator switch inline (`:656-669`): 13 → 8. **`(*scanner).value`** — `:610` becomes `case c == '-' || isDigit(c):` (`isDigit` cost 8, inlined): 11 → 10. **`str`** — the `\u` arm (`:722-730`) becomes `func (s *scanner) unicodeEscape() error` (k loop and both faults, `s.i += 6`; cost 190, not inlined; only per `\u` escape): 11 → 9. **`number`** — sign and integer switch (`:751-765`) stay; `if s.at('.') { s.i++; digits }`; then `return s.exponent()`, with `func (s *scanner) exponent() error { if !s.atEither('e', 'E') { return nil }; s.i++; if s.atEither('+', '-') { s.i++ }; return s.digits() }` (cost 127, not inlined: **one extra call per number**): 17 → 9.

Order and offsets: each `syntax(…)` moves with its check, same `at`, same text; `!s.at('"')` is exactly De Morgan of `:556`. Transfers: `member`'s three error returns (mutation: return `nil` on a `str` fault, red by an unterminated key); `unicodeEscape`'s two faults plus the advance (mutations: no `s.i += 6`, red by `"A"`; drop the bound check, red by `"\u12`); `exponent`'s `nil` for "no exponent" (mutation: `return s.digits()` always, red by `1`). Decision 1 decides `member` and `exponent` per function.

*(Annotated 2026-11-18: `object` through `member` and `str` through `unicodeEscape` cost the scanner's path +8.5 % then +8.1 %; under ground rule 13a the owner chose one `name` reader for `object` and `top` and a register-loop `str` instead (`member` and `colon` removed) — [owner](decisions.md#owner-decisions-2026-11-18--the-architecture-review-of-lane-es-walkers-and-scanner), decision 2.)*

#### E.2.6 `NewSchema` — cc 12, deploy-time
```go
func checkAttrs(stream string, own []Attr) error                 // event.go:107-125 verbatim (declaredAt map, the three faults in order); cc 6
func checkTimeAttr(stream string, own []Attr, timeAttr int) error // event.go:126-135 verbatim; cc 5
```
`NewSchema` keeps the stream and count checks and the copy, calls `checkAttrs` then `checkTimeAttr`, returning `Schema{}` on an error; cc 5. Order: stream, count, each attribute (name, duplicate, type), time range, time type (`event.go:89-92`). `%w` wrapping and texts unchanged. Mutation per transfer: drop either helper's error (red by `schema_test.go`'s cases, inferred).

### E.3 Nets (commit E1, before any move)

**Coverage today** (in-package, measured; package 94.2 %): every flagged function at 100 % except:

| Function | Now | Uncovered | Test to add (package, file) |
|---|---|---|---|
| `CleanLen` | **0.0 %** (only runtime and playback tests reach it) | `decode.go:252-285`, all of it | `TestCleanLenIsTheStoredLength` (event_test, new `internal/event/object_len_test.go`): each row asserts `CleanLen(raw) == len(CleanObject(raw))` and a pinned number — compact (fast path); a space, a tab, a newline, a CR outside strings (one per operand of `:260`); whitespace inside a string kept; `\"` and `\\` escapes; a valid é; a malformed `\xff` (3 bytes out); **a literal U+FFFD `EF BF BD`** (the `size == 1` operand of `:277` deciding alone) |
| `(*Decoder).value` | 97.7 % | `decode.go:200` (the default case: `NewSchema` rejects every type outside TString…TObject, `event.go:120`) | `TestDecodeAnUndeclaredTypeIsATypeFault` (package event, new `internal/event/decode_value_test.go`), white-box `Schema{stream: "S", attrs: []Attr{{Name: "x", Type: TInvalid}}, timeAttr: ArrivalTime}` (E-Q2) |

**Operand cases** (new files, package event):
- `internal/event/rfc3339_operands_test.go`: the spike's 50-case `rfc3339Cases` table verbatim as `TestParseRFC3339EachOperand`, with `TestParseRFC3339Allocs` and `BenchmarkParseRFC3339` (spike diff lines 183–305); covers `:818-819, :828-829, :837, :860, :865, :884`.
- `internal/event/rfc3339_reference_test.go` (v2, QA DQ-S12): a frozen copy of today's function as `parseRFC3339Reference` and `TestParseRFC3339MatchesItsReference`, the scratch differential's generator (every truncation, every 1-byte substitution and insertion, two-position substitutions over 20 seeds; 2,115,410 inputs), comparing value and ok — a committed net that outlives the split.
- `internal/event/value_operands_test.go`: `TestDecodeAbsentAndNullByOptionality` (`:137`, `:146` `a.Optional && !isTime`: optional non-time → null value; required → missing or null error; optional time attribute → error with the time note); `:168` `raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9')`: `-1`, `0`, `9`, `"x"` (below '0'), `true` (above '9'); `:169` `1e999`.
- `internal/event/object_operands_test.go` (`TestCleanObjectEachOperand`, `TestCompactAndValidEachOperand`): `:217, :234, :309, :319` — the four whitespace bytes; malformed byte, literal U+FFFD, valid é.
- `internal/event/unescape_operands_test.go` (`TestAppendUnescapedEachOperand`): each simple escape `b f n r t " \ /`; `:444` `r < 0xDC00 && i+6 <= len(in) && in[i] == '\\' && in[i+1] == 'u'` — a low surrogate first (`\uDC00`), a high surrogate at the end, followed by `x…`, followed by `\nxxxx`, followed by `A`; `:445` a valid pair (😀); `:464` malformed vs U+FFFD.
- `internal/event/scanner_operands_test.go` (`TestScannerFaultsEachOperand`, offset and text per case): `:550/:637` `{`, `{}`, `{"a":1}`; `:556/:644` `{"a":1,` and `{"a":1,x`; `:610` `-`, `0`, `/`, `:`; `:715` `"\`; `:723-727` `"\u12`, `"\u12g4"`; `:760` a digit run at the end; `:766` `1`, `1.`, `1.5`; `:772` `1`, `1e5`, `1E5`, `1x`; `:774` `1e`, `1e+5`, `1e-5`.
- `internal/event/schema_operands_test.go` (`TestNewSchemaEachOperand`): `event.go:120` (TInvalid, `Type(8)`), `:127` (−2, `len`); and (v2, QA DQ-S8) `TestNewSchemaNamesTheFirstFault`: a duplicate attribute plus a bad time attribute → the duplicate named.

**Can never decide alone (E-Q1):** `decode.go:834` `i < len(t)` — always true (`:818` guarantees `len(t) >= 20`, `i = 19`; the spike's example). After the re-split it is `rfc3339Fraction`'s own guard (`len(t) == 19`), still unreachable through `parseRFC3339` (§10 Q3, list b).

**Mutations:** one per new test, plus each transfer in E.2, each quoted with its diff in the lane report (the shared mutation runner, W0).

**Leaf benchmarks** (new `internal/event/decode_bench_test.go`, package event; `b.ReportAllocs()`, `for b.Loop()`, one pass over a fixed list per iteration; never edited after E1). Inputs fixed by this design:

| Benchmark | Reaches | Inputs (accept / reject mix) |
|---|---|---|
| `BenchmarkParseRFC3339` | `parseRFC3339` | the 50 `rfc3339Cases` (22 accepted, 28 rejected) — the spike's exact benchmark, so its +19.7 % is comparable |
| `BenchmarkDecoderValue` | `value` and its accept helpers | precomputed spans over `{"s":"alpha","i":7,"d":1500,"f":2.5,"b":true,"ts":"2026-01-01T10:00:00Z","o":{"x":[1,2]},"x":null}`, plus an absent optional and the reserved slot: 10 accepts |
| `BenchmarkDecoderValueFaults` | `value`'s error paths | one type fault per type (7), required null, required missing, time attribute null: 10 rejects |
| `BenchmarkCleanObject` / `BenchmarkCleanLen` / `BenchmarkCompactAndValid` | the three walkers | six objects: compact; spaced with `\t\n\r`; escapes and `é`; `\xff`; literal U+FFFD; valid é |
| `BenchmarkAppendUnescaped` | `appendUnescaped` and helpers | into a reused `[128]byte`: `a\"b\\c\/d\n\t\r\b\f`, `é`, `😀`, `\ud800x`, `\xffab`, a 40-byte ASCII run with one `\n` |
| `BenchmarkScannerValue` | `(*scanner).value`, `object`, `str`, `number`, `member`, `exponent` | `{"a":{"b":[1,-2.5e+3,true,false,null]},"c":"xé\n"}`, `[0,1e5,"s"]`, `-0.0E-1` (all accepted) |
| `BenchmarkScannerDocument` | `document`, `top`, `record` | the 8-attribute payload above through `scanner.document(d, spans, &info)` |
| `BenchmarkScannerFaults` | the syntax faults | the 16 fault inputs of `scanner_operands_test.go` |

**`AllocsPerRun` pins** (new `TestDecodeLeafAllocations` in `internal/event/leaf_allocs_test.go`, package event, not parallel; measured today on the scratch copy): `compactAndValid` over the six objects **0**; `CleanLen` **0**; `cleanObject` over the six **7** (compact alone 1, spaced alone 1); `appendUnescaped` over the six into `[128]byte` **0**; `parseRFC3339` **0**; `(*scanner).value` over the three **0**; `Decode` of the 8-attribute payload into `make([]Value, 0, 16)` **3**; the existing `TestDecodeAllocationBudget` stays at **4** (`internal/event/s3_2b_2_alloc_test.go:23-30`). The lane re-measures at E1 and pins equality.

**Escape check (v2):** `escapes.py` (§2.4, §3.2) per split commit, the original function and its new helpers as one group (`-m=2` names the function in its "… in F:" lines; DA-S10, QA DQ-L2), `leaking param` lines a diagnostic (QA DQ-B2); the `can inline`/`cannot inline` lines of every new helper are quoted; `-d=ssa/check_bce/debug=1` counts per function quoted as a diagnostic.

**Fuzz:** `FuzzDecode` (`internal/event/fuzz_test.go:14`, nightly `fuzz-decoder`) reaches `value`, the scanner, `parseRFC3339`, `appendUnescaped`, `cleanObject`, `compactAndValid` through `Decode`; not `CleanLen` or `ExactObject` (inferred), hence the property test. `fuzz_test.go` is the driver's `FUZZ_FILE` and is not edited.

### E.4 Placement

Nothing moves in or out: `event` may import nothing; `CleanLen`/`CleanObject` (runtime boundary, playback), `ExactObject` (runtime, validate), `TimestampInRange` (runtime, playback) and `MaxLogLine` belong to the event form they describe; the recording-size candidate (plan check §0.15) is runtime and playback code, backlog. `parseRFC3339` and the parser's `lowerer.timestamp` (`internal/parser/lower.go:528`, `time.Parse` with UTC required) read different languages (a payload with any offset; a rule literal in UTC only): not a merge candidate.

### E.5 Risks and owner questions (recommendation first)

- **E-Q1 — `decode.go:834`'s `i < len(t)`, which can never decide alone.** → (v2) accept the reason "guaranteed by the length check at `:818`" (§10 Q3, list b), code unchanged; no post-split helper test (§2.2, DA-S8). Other: remove it in its own commit before the split (the helper would then trust its caller).
- **E-Q2 — `decode.go:200`.** → reach it with the white-box `Schema` literal test. Other: list it as unreachable through `NewSchema` (decision 2 names no such reason).
- **E-Q3 — `cleanObject` and `CleanLen` land exactly at 10.** → accept (zero added calls). Other: extract the non-ASCII arm for margin (one call per non-ASCII rune of non-compact objects).
- **Risks:** `(*Decoder).value` (+1 call level and a `Value` copy per attribute), `number` (+1 call per number) and `appendUnescaped` (+1 call per escape) may each fail decision 1's time rule; each then becomes a PERMANENT waiver in its commit's slot, with samples (`value` the likeliest). `rfc3339Date` must be brought from 84 to ≤ 80 by the year expression; if not, one more non-inlined call remains and the measurement decides. `colon`'s removal is safe only because its two callers fold into `member`.

### E.6 Lane E commit plan (12; each at or below every cap; v2: one path-shared hot function per commit, DA-S3)

| # | Sub-lane | Commit | event file-length / funlen / gocyclo after (cap 1/2/12) |
|---|---|---|---|
| 1 | E1 | nets: the operand tests, the `CleanLen` property, the reference differential, the white-box value case, benchmarks, alloc pins (new files only) | 1 / 2 / 12 |
| 2 | E2 | pure move of `decode.go` into the E.1 map (`astequal` exit 0; `Decode` stays in `decode.go`) | 0 / 2 / 12 |
| 3 | E2 | `isSpace`; `compactAndValid`, `cleanObject`, `CleanLen` (each its own leaf benchmark) | 0 / 2 / 9 |
| 4 | E2 | `appendUnescaped` split | 0 / 2 / 8 |
| 5 | E2 | `at`, `atEither`, `member`; `top` (`colon` kept until commit 7) | 0 / 2 / 7 |
| 6 | E2 | `(*scanner).value` | 0 / 2 / 6 |
| 7 | E2 | `object` (with `member`; `colon` removed) | 0 / 2 / 5 |
| 8 | E2 | `str` (`unicodeEscape`) | 0 / 2 / 4 |
| 9 | E2 | `number` (`exponent`) | 0 / 2 / 3 |
| 10 | E3 | `(*Decoder).value` split | 0 / 1 / 2 |
| 11 | E3 | `parseRFC3339` re-split | 0 / 0 / 1 |
| 12 | E3 | `NewSchema` split | 0 / 0 / 0 |

A split that costs is restored whole with its waiver by a new commit on the lane branch (§2.4). After each commit: `go test ./internal/event`, the alloc pins, the `-m=2` diff. Hand-back: 0 findings under the pinned golangci-lint on both `GOOS`; then the coordinator runs `fuzz-decoder` alone and `--job proofs` (plan check §7 row E).

---

## 7. internal/parser (lane Q, with plan: Q1 → Q2)

Caps: file length 1, funlen 1, gocyclo 5 (S1.1c-5-2); 7 findings, none on a hot path. Imports allowed: event, model, metrics, `github.com/alecthomas/participle/v2`. Law paths (read-only): `errorcounts_test.go`, `keywords_test.go`, `fragment*_test.go`, `parseerror_contract_test.go`, `testdata/fragments/research-spans.jsonl` (`gate/law.json:169-221`); new tests go in new files. Pinned: `func Parse(name, src string) (*Result, error) {` stays in `internal/parser/parse.go` (`test_fuzz_parser_planted.py:18-19`).

### Q.1 File map

| File (concern) | Declarations | Moved from | Lines moved → final (projected) |
|---|---|---|---|
| `parse.go` — the entry point and lexing faults | `Position`, `Result`, `Reassemble`, `grammar`, `filterGrammar`, **`Parse` (pinned)**, `mk`, `lexError`, `invalidUTF8` | stays: `parse.go:1-117`, `:419-439` | ~140 → ~140 |
| `parseerror.go` — participle's stop mapped to one catalogue code | `parseError` + `failSite` and its methods, `endsTheChain`, `abandonedClause`, `unitMends`, `bareDuration` + helpers, `expectsName`, `closingBracket` | `parse.go:118-298`, `:402-417` | ~200 → ~285 |
| `rulewords.go` — participle's notation in rule words | `isNameChar`, `ruleWords`, `tokenWords`, `firstExpected` + helpers, `wordsFor` | `parse.go:300-400` | ~110 → ~130 |
| `lexer.go` | unchanged, plus `joinPragma`'s helpers | — | 229 → ~255 |
| `lower.go` — the tree's statements and expressions to the model | `lowerError`, `lowerer`, `fail`, `lower`, `types`, `attrType`, `pos`, `name`, `paramName`, `attrs` … `atom` | stays: `lower.go:1-424` | 424 → 424 |
| `literal.go` — literals: `ParseLiteral` and the lowerer's literal methods | `literalGrammar`, `ParseLiteral`, `notALiteral` + `signedLiteral`, `minIntDigits`, `literal`, `number`, `units`, `unitRank`, `duration`, `durationOrParam`, `timestamp`, `str` + `escape`, `isTripleQuoted` | `lower.go:425-583` | ~215 → ~240 |
| `errors.go`, `grammar.go`, `keywords.go` | unchanged | — | 152, 469, as now |

`parse.go` must shed lines: at 439 plus `parseError`'s split (~+65) it would pass 500. Imports follow use (`go build` checks them): `parse.go` `errors`, `strings`, `unicode/utf8`, participle, participle's `lexer`, model; `parseerror.go` `errors`, `slices`, `strings`, participle, `lexer`; `rulewords.go` `errors`, `fmt`, `strconv`, `strings`, participle; `literal.go` gains `math`, `strconv`, `strings`, `time`; `lower.go` keeps what its statements use (inferred).

### Q.2 Split plan per finding

GitNexus impact upstream (with tests):

| Function | Risk | Impacted | d=1 callers |
|---|---|---|---|
| `parseError` | CRITICAL | 110 | 2: `Parse`, itself |
| `joinPragma` | CRITICAL | 51 | `lexAll` |
| `(*lowerer).str` | CRITICAL | 17 | 5 |
| `bareDuration` | LOW | 3 | `parseError` |
| `firstExpected` | LOW | 4 | `ruleWords` |

No signature changes. Parse-error texts and positions stay exact: every `mk(…)` call moves verbatim.

#### Q.2.1 `parseError` — cc 59, 102 lines
```go
// failSite is where participle stopped: the code tokens, the index of the one it refused (len(toks) at the end of
// input), participle's position for that end, and its message, which names what it expected there.
type failSite struct {
	toks []Token
	at   int
	pos  Position
	msg  string
}
func newFailSite(toks []Token, perr participle.Error) failSite // parse.go:126-133 (the at search); cc 4
func (s *failSite) tok(i int) Token                             // the closure at :134-139; 3
func (s *failSite) expects(what string) bool                    // the closure at :157; 1
func (s *failSite) later(word string) bool                      // the closure at :159-166; 4
func (s *failSite) filterError() error                          // :141-155: the inner error to map instead (with F7's `]` substitution), or nil; 7
func (s *failSite) tokenMistake(name string) *Error             // the six groups below, called in order, first non-nil wins; 6
func (s *failSite) startMistake(name string) *Error             // cases :168-173 (E-CMT-001, E-PRAG-001, E-RES-001); 5
func (s *failSite) arrowMistake(name string) *Error             // cases :177-183 behind one `cur.Value != "->"` guard (E-PAT-005 ×3); 6
func (s *failSite) nameMistake(name string) *Error              // cases :184-192 (E-KW-001, E-KW-002, synonyms E-SYN-001, E-TXT-001); 9
func (s *failSite) operatorMistake(name string) *Error          // cases :193-200 (E-PAT-003, E-SYN-002, E-SYN-003, E-SYN-004); 9
func (s *failSite) missingPartMistake(name string) *Error       // cases :201-204 (E-PAT-005 no positive step, E-WIN-001); 6
func (s *failSite) boundMistake(name string) *Error             // cases :205-208 (E-DUR-001, E-PAT-004); 6
func (s *failSite) missingClause(name string) *Error            // :210-217 (E-ORD-001 or the clause's code); 4
func (s *failSite) bareDurationMistake(name string) *Error      // :218-222 (F3's E-DUR-001); 4
```
`parseError` keeps the non-participle fallback, `newFailSite`, the recursion `if inner := s.filterError(); inner != nil { return parseError(name, toks, inner) }`, then `tokenMistake`, `missingClause`, `bareDurationMistake`, and the final `mk(name, "E-SYN-001", s.tok(s.at).Pos, ruleWords(perr))`; cc 6, ~25 lines.

**Order of checks:** the 18 switch cases are cut into **contiguous runs in their original order**, each a group whose `switch` returns the first match, and the groups are called in the same order — first-match preserved. The `->` guard is factored out (each of the three cases begins with `cur.Value == "->"`; all operands are pure). Then the clause loop, F3, the fallback, as today. Groups mutually exclusive on `cur.Value` (arrow / operator / missing-part) cannot reorder anything; every other adjacent boundary gets a two-fault case in `TestParseErrorFirstMatchAcrossGroups` (e.g. `=` at the file start → E-PRAG-001, not E-SYN-002).

The groups read `cur`, `prev`, `prev2`, `next` and `kind` through `s.tok(…)` and `typeName`, re-derived per group: parse errors are cold, and nothing is held that could go out of step (DA-L5). **Transfers:** (1) the recursion's trigger — `filterError` → `nil` always, red by the filter cases of `fragment_test.go` (inferred); (2) F7's `ute.Unexpected = toks[close]` (`:149`) moved into `filterError` — removed, red by the early-ending-filter case; (3) `lexer.Upgrade`'s failure falling through to the switch (`:144`) becomes `filterError` returning `nil`; (4) each group's `nil` fall-through — one mutation per group (always `nil`), red by its first case. **§7.3:** `failSite` has 4 fields; it gives the three closures a home as methods, not a carrier for more than five values (Q-Q3).

#### Q.2.2 `firstExpected` — cc 13
```go
func quotedFirst(s string) (string, bool)    // :339-343 — the literal without its quotes; 3
func tokenTypeFirst(s string) (string, bool) // :347-352 — tokenWords, else the bare name; 3
func productionFirst(s string) string        // :354-361 — productionWords, else the name; 4
```
`firstExpected` keeps the switch (`""`, `"`, `(` with the recursion, `<`, a name); two `if … ok` fall-throughs land on the final `strings.TrimRight(strings.Fields(s)[0], "?*+!)|")` exactly as the cases fall out today; cc 8. Mutation: `quotedFirst` returning `ok=true`, red by an unterminated quote.

#### Q.2.3 `bareDuration` — cc 12
```go
func tokenValue(toks []Token, i int) string   // the closure at :264-269; 3
func inDurationSlot(toks []Token, i int) bool // :274's `p1 == "bounded" || p1 == "after" && p2 == "idle" || p1 == "(" && p2 == "delay"`; 5
```
The loop becomes `if typeName(toks[i].Type) == "Number" && units[tokenValue(toks, i+1)] == 0 && inDurationSlot(toks, i) { return i }` — the `continue` at `:272` folded into the same operands in the same order; cc 6. Mutation: `||` for the first `&&`, red by `5 min`.

#### Q.2.4 `joinPragma` — cc 21 (`lexer.go:127`)
```go
func pragmaLead(code []Token, i int) bool                                              // :134-135 negated (same operands, same order); 4
func pragmaID(code []Token, i int) (string, bool)                                      // :138-148 (reads rules.Symbols() itself); 8
func pragmaRest(name string, code []Token, i int, joined string) ([]Token, int, error) // :150-165: re-lexed rest and `used`; 9
```
`joinPragma`: append; `if !pragmaLead { continue }`; `if !ok { continue }`; append the pragma token; `pragmaRest` (error returns `nil, err`); `out = append(out, rest...)`; `i += used`; cc 5. Transfers: the three `continue`s become helper `false`s (mutation: `pragmaLead` always true, red by an index panic or a wrong join); `relex`'s error return (`:162`) moves into `pragmaRest` (mutation: error swallowed, red only by the white-box case below); `used` 3 or 4 (mutation: always 3, red by `cep/0.1e5x`).

#### Q.2.5 `(*lowerer).str` — cc 15 (`lower.go:541`, moved to `literal.go`)
```go
func isTripleQuoted(quoted string) bool                                                   // :542's compound; 3
func (l *lowerer) escape(b *strings.Builder, quoted, body string, i int, at Position) int // :561-577: writes the escape whose letter is body[i]; returns 4 after a valid \u, else 0; 8 (5 params, at the limit)
```
`str` keeps the raw and no-backslash returns and the loop `i++; i += l.escape(&b, quoted, body, i, at)`; cc 6. Transfer: `i += 4; continue` (`:566-567`) becomes `return 4` (mutation `return 0`, red by `"A"` → `A0041`). `fail`'s first-error-wins unchanged.

### Q.3 Nets (commit Q-P1)

**Coverage today** (in-package, measured; package 91.8 %): `parseError` 100 %, `(*lowerer).str` 100 %, plus:

| Function | Now | Uncovered | Test to add (package parser, new file) |
|---|---|---|---|
| `joinPragma` | 96.6 % | `lexer.go:162-163` (the `relex` error: through `Parse` the rest is digits, `.`, `e`, signs and letters, all lexable — inferred unreachable) | `TestJoinPragmaReportsARelexFailure` (`joinpragma_test.go`): hand-built tokens with a Number `0.1#`, so the rest `#` is refused (inferred; the lane shows it red then green) — Q-Q1 |
| `bareDuration` | 80.0 % | `parse.go:266-267` (out of range), `:278` (none found) | `TestBareDurationFindsTheFirstUnitlessNumber` (`bareduration_test.go`), white-box over `lexAll` tokens: a number last; a number at index 0 or 1; `bounded 1`; `idle after 5`; `delay(5)`; `after 5` without `idle`; `( 5` without `delay`; `5 min`; a `;` ending the scan; none → −1 |
| `firstExpected` | 83.3 % | `parse.go:337` (`""`), `:351` (`<x>` not in `tokenWords`), `:363` (the fallback) | `TestFirstExpectedReadsEachNotation` (`firstexpected_test.go`): `""`, `" ~"`, `"\"for\""`, `"\"unterminated"`, `"(<ident>"`, `"<string>"`, `"<foo>"`, `"<noclose"`, `"Duration"`, `"[x]"` |

**Operand tables:** `TestParseErrorEachOperand` (`parseerror_operands_test.go`; input → code, line, column for `:129, :148, :172, :177, :179, :182, :184, :188, :191, :193, :197, :199, :201, :203, :205, :207, :219`) plus `TestParseErrorFirstMatchAcrossGroups`; `TestJoinPragmaEachOperand` (`lexer.go:134` 4 operands, `:139-141` 7, `:154-155` 4); `TestStrEachOperand` (`literal_str_test.go`, white-box `l.str(…)`: `lower.go:542` — `'''` of length 3 decides `len >= 6`, which the lexer never yields, so the direct call is needed; `:553` — a trailing lone backslash, direct call only; `:563` — `\u12`, `\uzzzz`). Not an operand: `parse.go:340`'s `strconv.Unquote` cannot fail after `QuotedPrefix` succeeds (inferred).

**Latent defect found in passing (not fixed; ground rule 7):** `parse.go:363` `strings.Fields(s)[0]` panics when `s` is all whitespace other than `' '` (e.g. `"\t"`); participle's notation never yields that (inferred). The table leaves `"\t"` out.

**Fuzz:** `FuzzParse` (`internal/parser/fuzz_test.go:16`, nightly `fuzz-parser`) reaches all five through `Parse`. Other nets: `fragments`, `docsync-d` and the goldens pin the texts. No benchmarks (no hot path).

### Q.4 Placement

Nothing moves: the literal-to-value candidate is validate, plan and expr code; the parser's string escapes (SPEC §2.4) and timestamp rule (UTC only) are a different language from the decoder's JSON and RFC 3339 with offsets (merging would change behaviour); `units` and `types` stay with their users.

### Q.5 Risks and owner questions (recommendation first)

- **Q-Q1 — `lexer.go:162-163`.** → reach it with the white-box token test. Other: list it as "a defensive return unreachable from `Parse`" (decision 2 names no such reason).
- **Q-Q2 — `parse.go:363`'s panic on an all-whitespace notation.** → a backlog item with a red test when taken. Other: fix it now (a behaviour change; a scope change for the owner).
- **Q-Q3 — `failSite` under §7.3.** → accept: a 4-field type turning `parseError`'s three closures into methods. Other: pass `toks`, `at`, `pos`, `msg` to each group (five parameters with `name`, at the limit).
- **Risk:** first-match order across the six groups. *Mitigation:* contiguous cuts, the cross-group two-fault test, `fragments`, `parseerror_contract_test.go`.

### Q.6 Parser's commit plan within lane Q (≤ 4; each at or below every cap)

| # | Commit | parser file-length / funlen / gocyclo after (cap 1/1/5) |
|---|---|---|
| Q-P1 | nets: the five new test files | 1 / 1 / 5 |
| Q-P1b | `fix(parser)`: `firstExpected` on a notation that is only whitespace (G1 decision 7, ≠ R): `TestFirstExpectedOnAWhitespaceOnlyNotation` (`"\t"`, `"\t \n"`) shown red first (an index-out-of-range panic at `parse.go:363`), then `firstExpected` returns `""` for it, as it does for the empty notation (`:337`); its own commit, before the moves (no behaviour change in a move commit) | 1 / 1 / 5 |
| Q-P2 | pure moves: `parse.go` → `parse.go`, `parseerror.go`, `rulewords.go`; `lower.go:425-583` → `literal.go` (`astequal` exit 0; `Parse` stays in `parse.go`) | 0 / 1 / 5 |
| Q-P3 | `parseError`, `firstExpected`, `bareDuration` | 0 / 0 / 2 |
| Q-P4 | `joinPragma`, `(*lowerer).str` | 0 / 0 / 0 |

Verifiers (plan check §7 row Q): `fragments`, `docsync-d`, the parser goldens; then the coordinator runs the nightly job alone (`fuzz-parser`) and `--job proofs`.

**Gaps (both packages):** `rfc3339Date`'s ≤ 80 year form is inferred, not built; the inline costs of the `value`, `appendUnescaped` and `NewSchema` helpers are inferred; hand-counted cyclomatic values need the pinned lint; `TestJoinPragmaReportsARelexFailure`'s reachability is inferred; no times were measured (decision 1's comparisons are the coordinator's, run alone).

## 8. internal/playback (lane P: P1 → P2 → P3)

*Drafted read-only on `s1-1c-6/integration` @ 6bf339c by the refactoring specialist (gitnexus-refactoring, gitnexus-impact-analysis, architecture-designer); production Go identical to `main` 4d2b9eb; measured with the pinned Go 1.27.4 (in-package profiles: playback 90.4 %, plan 95.4 %; `-gcflags=-m=2` for both packages; allocation counts from a `git archive` copy with a scratch test). Every risk level is from `impact` upstream with tests; `detect_changes`: 0 changed symbols. The coordinator verified the caps (playback gocyclo 15, funlen 1, file length 1; plan gocyclo 5, funlen 2, argument limit 1), `plan.value`'s two callers (`internal/plan/expect.go:136, :167`) and the PERMANENT panic at `plan.go:258`.* Nothing on playback's side changes the recording-size arithmetic: no encoded byte changes, `lineBound` and `rowBound` (`internal/runtime/boundary.go:47-84`) stay valid caps, and runtime may not import playback (`tools/importcheck/rules.json:18`): backlog, as §6 says.

### P.1 File map (after the sprint)

| File | Concern | Holds | Projected lines |
|---|---|---|---|
| `playback.go` | the package's types | unchanged (`Stream`, `Table`, `App`, `RecordKind`, `TableOp`, `Record`, `LoadError`, `StepKind`, `Step`) | 111 |
| `driver.go` | the replay driver | unchanged; keeps its two PERMANENT `//nolint:forbidigo,lll` panics at `:76`, `:91` | 153 |
| `encode.go` | canonical JSON encoding | consts, `errNonFinite`/`errTimeRange`, `AppendValue`, **`appendFloat`, `appendTimestamp`, `appendObject`**, `appendTime`, `hexDigits`, `appendString`, **`appendEscape`**, `AppendEvent`, `appendEvent`, **`appendAttrs`, `appendEventTail`** | ~225 |
| `reader.go` | reading a log line by line, and the app it is read against | `maxLine`, `errLongLine`, `Reader`, `NewReader`, **`envelopeSchemas`, `(*Reader).addTable`**, `validateApp`, **`validateStreams`, `validateTables`**, `Next`, `readLine`, **`(*Reader).rawLine`, `trimLineEnd`**, `blank`, `fault`, `decodeMsg` | ~265 |
| `records.go` (new) | decoding one log record by kind (design note §5.2) | moved from `reader.go:15-65` (the `e*` key consts, `envelopeAttrs`, `recordKinds`) and `reader.go:261-516` (`record` … `eventRecord`, `reasonOf`, `stream`, `headerRecord`, `batchEndRecord`, `tickRecord`, `stopRecord`); new: **`envelope`, `recordKind`, `rejectedFault`, `advance`, `tableOp`, `tableRow`, `eventKeys`, `arrivalKeys`, `eventPayload`, `eventArrival`** | ~370 (moved part ~320) |
| `recorder.go` | writing a recording | as today, plus **`(*Recorder).tableLine`** | ~235 |
| `trace.go` | the canonical trace | as today, plus **`(*Trace).tableEntry`** | ~200 |
| `expect.go` | running expect blocks in a sandbox | `Counts`, `BlockResult`, `RunExpects`, `runBlock` (lines 1-103), plus **`seedGivenTables`, `feedGivenLists`, `nextGiven`, `blockCounts`** | ~135 |
| `verdict.go` (new) | the verdict and the first divergence (design s3-3-2 §5.3–5.4) | moved from `expect.go:105-302` (`diverge`, `alignCap`, `nearest`, `drops`, `sameValue`, `canon`, `thenText`, `outputText`); new: **`matches`, `inOrder`, `firstMisplaced`, `matchedPrefix`, `thenRecordFirst`, `lcsLength`, `missingThen`, `unexpectedOutput`, `samePayload`, `sameObject`** | ~250 |

Pure moves before any split, in one commit (P3): `reader.go:15-65` and `:261-516` → `records.go` unchanged (leaving `reader.go` at ~220, clearing the file-length finding); `expect.go:105-302` → `verdict.go` unchanged; each file keeps only its imports (`records.go` `errors`, `fmt`, `event`; `verdict.go` `fmt`, `math`, `strings`, `event`, `plan`); `astequal` exit 0. No test file is renamed; `s3_2b_2_corpus_test.go` stays (two proof drivers name it by path).

### P.2 Split plan per finding

Cyclomatic counts follow gocyclo; "hot" = on the plan check's §3 list. `.golangci.toml:84-86` enables only revive's `argument-limit` (5); no result-count rule.

**Inlining today (measured, `-m=2`):** none of the hot functions inlines (`AppendValue` 644, `appendString` 298, `appendEvent` 943, `readLine` 359, `record` 1214, `eventRecord` 1288, `tableRecord` 920, `rejectedRecord` 952, `(*Recorder).Table` 740, `(*Trace).Step` 1849); only `AppendEvent` (73), `fault` (68), `blank` (26) and `record.func1` (`present`, 6) inline. Each split adds at most a few direct calls to a function that is already a real call; the lane records whether the small helpers inline (`appendFloat`, `appendTimestamp`, `appendObject`, `trimLineEnd`, `advance`, `tableOp` expected to; inferred).

**Escape invariants a split must keep (measured; checked by the escape diff):**
- (E1) `reader.go:278:13: func literal does not escape`, and `present does not escape` in every function that receives it.
- (E2) `moved to heap: de` only inside error branches (`reader.go:265, :490, :520`); every `var de *event.DecodeError` stays inside its `if err != nil`.
- (E3) `appendEvent`'s `var stack [64]int` never "moved to heap"; `encode.go:143` `make([]int, 0, n) does not escape`.
- (E4) `recorder.go:161:19` `append does not escape`: the delete path's `cols` is stack-backed (0 allocations) — `cols` is never *returned* from a helper, only passed down.
- (E5) `trace.go:94:15` `"#" + ~r0 does not escape`: the table name string is never returned from a helper.

**`AppendValue`** (`encode.go:31`; cc 15; hot, recording; risk CRITICAL — 10 direct callers, 8 processes).
```go
func appendFloat(dst []byte, f float64) ([]byte, error)    // NaN/±Inf refused, else strconv 'g' -1; cc 3, ~6 lines
func appendTimestamp(dst []byte, ms int64) ([]byte, error) // the 0000–9999 range refused, else appendTime; cc 3, ~6 lines
func appendObject(dst []byte, obj any) ([]byte, error)     // obj.(string) or "an object value is not its JSON text (%T)"; cc 2, ~7 lines
```
`AppendValue` keeps the null guard (`v.Null && v.Kind != event.TInvalid`) and the seven cases; TFloat, TTimestamp, TObject become `return appendFloat(dst, v.F)`, `return appendTimestamp(dst, v.T)`, `return appendObject(dst, v.Obj)`; the `"a value of type %v is not encodable"` return stays last; cc 10, ~22 lines. Order unchanged. Transfers: the three error returns (errNonFinite, errTimeRange, the object error), one mutation each (the error dropped). `%T` of `obj` is the same dynamic type as `%T` of `v.Obj`.

**`appendString`** (`encode.go:76`; cc 16; hot; risk CRITICAL — 7 direct, 9 processes). `func appendEscape(dst []byte, c byte) []byte` (the ASCII escape switch, lines 87-102 moved whole; cc 7, ~18 lines). `appendString` keeps the loop, the fast ASCII `continue`, `i++; start = i; continue`, the non-ASCII switch; only lines 87-102 become `dst = appendEscape(dst, c)`; cc 10, ~30 lines. No `continue`, `break` or `return` crosses; the no-escape fast path is byte-identical. Mutation: swap two escape letters inside `appendEscape` (e.g. `'\t'` written as `'n'`).

**`appendEvent`** (`encode.go:135`; cc 16; hot; risk HIGH — 2 direct, 3 processes).
```go
// appendAttrs writes order's attributes as "name":value members, leaving out skip; dst is returned as far as written on an error.
func appendAttrs(dst []byte, s event.Schema, vals []event.Value, order []int, skip string) ([]byte, error) // lines 152-173; cc 7, ~25 lines
// appendEventTail writes ,"batch" (when set), ,"stream" and, on a `time arrival` stream, ,"ts", then the closing brace.
func appendEventTail(dst []byte, s event.Schema, batch string, ts int64) ([]byte, error)                     // lines 175-188; cc 5, ~16 lines
```
`appendEvent` keeps the value-count check, `stack`/`order` and the insertion sort (inline for E3), then `dst = append(dst, `{"attrs":{`...)`, `dst, err := appendAttrs(dst, s, e.Values, order, skip)`, `if err != nil { return dst, err }`, `dst = append(dst, '}')`, `return appendEventTail(dst, s, e.BatchID, e.Ts)`; cc 7, ~25 lines. On an error the partial `dst` is returned exactly as today; `order` reads "does not escape" in `appendAttrs` (E3). Transfers: the kind-mismatch, AppendValue-error and ts-range returns; the skip `continue` stays in the moved loop. Mutations: drop each returned error in `appendEvent`.

**`(*Recorder).Table`** (`recorder.go:150`; cc 13; hot; risk LOW, lower bound — 2 call sites untyped).
```go
// tableLine writes one table record: {"op":…,"row":{…},"table":…}.
func (r *Recorder) tableLine(name, opName string, cols []event.Attr, row []event.Value) error // lines 167-190; cc 5, ~22 lines
```
`Table` keeps the range check, builds `cols` (E4), loops `if err := r.tableLine(t.Name, opName, cols, row); err != nil { return err }`; cc 9, ~22 lines. `b := r.buf[:0]` and `r.line` stay per row. Transfers: the row-width, AppendValue and write-error returns, all through `Table`. Mutation: ignore `tableLine`'s error in the loop (`TestRecorderTableStopsAtTheFirstBadRow` red: a later row gets written).

**`(*Trace).Step`** (`trace.go:74`; cc 11; tracing only; risk LOW). `func (t *Trace) tableEntry(s *Step)` (the StepTable case body, lines 87-98 moved whole; cc 4, ~14 lines). `Step` keeps 7 cases, `default: return`, `t.close()`; cc 8. E5 kept (the name string stays local to `tableEntry`). No transfer crosses.

**`NewReader`** (`reader.go:93`; cc 13; not hot; risk HIGH — 12 direct).
```go
func envelopeSchemas() (env, header, rejected event.Schema, err error) // lines 97-110, the same three error texts; cc 4, ~20 lines
func (r *Reader) addTable(i int, t Table) error                       // lines 121-145; cc 7, ~27 lines
```
Order: `validateApp`, `envelopeSchemas` (the composite literal stays, using `event.NewDecoder(env)` etc.), the stream loop, `for i, t := range app.Tables { if err := rd.addTable(i, t); err != nil { return nil, err } }`; cc 6. Transfers: `addTable`'s two errors (covered, P.3) and `envelopeSchemas`' three (unreachable, §10 Q3).

**`validateApp`** (`reader.go:151`; cc 11; not hot; risk HIGH — 2 direct: `NewReader`, `newDriver` at `driver.go:36`). `func validateStreams(streams []Stream) error` (lines 155-167; cc 7); `func validateTables(tables []Table) error` (lines 168-177 with their own seen map; cc 4). `validateApp` keeps the nil check, then `if err := validateStreams(app.Streams); err != nil { return err }`, then `return validateTables(app.Tables)`; cc 3. Order unchanged (nil; each stream in order: no schema, declared twice, negative bound; each table in order).

**`(*Reader).readLine`** (`reader.go:214`; cc 14; hot, replay; risk LOW).
```go
// rawLine is the next line with its ending, read whole across buffer-full chunks; errLongLine past maxLine+2 bytes.
func (r *Reader) rawLine() ([]byte, error) // the loop: ReadSlice, accumulate, the bound, then `return chunk, err`; cc 5
// trimLineEnd strips one trailing "\n", then one trailing "\r".
func trimLineEnd(b []byte) []byte          // lines 234-239; cc 5
```
`readLine`: `chunk, err := r.rawLine()`; `if errors.Is(err, io.EOF) && len(chunk) == 0 { return nil, io.EOF }`; `if err != nil && !errors.Is(err, io.EOF) { return nil, err }`; `chunk = trimLineEnd(chunk)`; `if len(chunk) > maxLine { return nil, errLongLine }`; `return chunk, nil`; cc 6. `errLongLine` from `rawLine` arrives with a nil chunk and leaves through the second check, unchanged — the one transfer that crosses (mutation: `rawLine`'s `return nil, errLongLine` → `continue`; `TestReaderStopsAnEndlessLine` red). Allocations 0 per line today (measured), must stay 0.

**`(*Reader).record`** (`reader.go:262`; cc 17; hot; risk LOW).
```go
// envelope is line decoded against the envelope schema; refused for a decode fault, an unknown key, or a record after the stop.
func (r *Reader) envelope(line []byte) ([]event.Value, error)          // lines 263-277, r.envBuf = vals kept before the unknown-key check; cc 5
// recordKind is the line's kind: the first choosing key present; refused when none is, or a key the kind does not allow is present.
func (r *Reader) recordKind(vals []event.Value) (RecordKind, error)      // lines 279-294, testing !vals[i].Null directly; returns rk.kind; cc 7
```
`record`: `vals, err := r.envelope(line)`, then `present := func(i int) bool { return !vals[i].Null }` (created in `record` as today; E1), `kind, err := r.recordKind(vals)`, then the six-case switch on `kind` with `return r.eventRecord(vals, present)` last; cc 9 (the dispatch stays in `record`: moving it out would copy one more large `Record` per line). v2 (DA-S6): `recordKind` takes `vals`, not the `present` closure — today the closure inlines at `reader.go:281` and `:291` (about 20 calls per line: 7 kinds, 13 keys), and a func-value parameter of a non-inlined helper would turn them into indirect calls. Which error a faulty line names, in order: decode fault, unknown key, record after the stop, no choosing key, disallowed key, the per-kind checks — as today. Transfers: `envelope`'s 4 returns, `recordKind`'s 2; mutations: the propagation in `record` dropped per helper. `vals` is never reassigned after `present` captures it.

**`(*Reader).rejectedRecord`** (`reader.go:315`; cc 13; hot; risk LOW).
```go
// rejectedFault is the rejected record's fault: its reason and attribute, refused when the pair is inconsistent.
func (r *Reader) rejectedFault(vals []event.Value) (event.DecodeReason, string, error) // lines 326-341; returns reason, f[0].Str; cc 8
// advance moves the recording's clock to arrival; refused when arrival is earlier than the previous arrival or tick.
func (r *Reader) advance(arrival int64) error                                        // the text "an arrival before the previous arrival or tick"; cc 2
```
Order: header; `!present(eStream) || !present(eArrival)`; `r.stream`; `rejectedFault`; `advance(vals[eArrival].T)`; cc 7. `advance` also replaces the identical check in `eventRecord` (`reader.go:505-508`: same text, same `r.last` write), returning the literal `nil`, never a nil `*LoadError`. Transfers: `rejectedFault`'s 5 returns, `advance`'s return.

**`(*Reader).tableRecord`** (`reader.go:370`; cc 13; hot; risk LOW).
```go
func (r *Reader) tableOp(table, op string) (TableOp, error)                                          // lines 381-389; cc 3
func (r *Reader) tableRow(name string, tid int, op TableOp, vals []event.Value) ([]event.Value, error) // lines 390-410; cc 8
```
`tableRecord` keeps the unknown-table, no-op and no-row checks, then `tableOp`, then `tableRow`; cc 6. The `[]byte(vals[eRow].Obj.(string))` conversion moves inside `tableRow` unchanged. Transfers: `tableOp`'s error; `tableRow`'s decode, unknown and null-key returns.

**`(*Reader).eventRecord`** (`reader.go:463`; cc 23; hot; risk LOW).
```go
// eventKeys checks an event's keys against its stream and the log: attrs, ts by the stream's time, arrival, a non-empty batch.
func (r *Reader) eventKeys(st *Stream, name string, arrivalTime bool, vals []event.Value, present func(int) bool) error // cc 9
// arrivalKeys checks arrival against the recording header and the stream's time (cases 4-6 of lines 479-484).
func (r *Reader) arrivalKeys(name string, arrivalTime bool, present func(int) bool) error                               // cc 9
// eventPayload decodes the attrs against stream sid's schema; a fault is a *LoadError with its Decode cause.
func (r *Reader) eventPayload(sid int, name string, vals []event.Value) ([]event.Value, event.Decoded, error)           // lines 488-494, de inside the if (E2); cc 2
// eventArrival is the event's arrival in a recording, the clock advanced to it; 0 outside a recording.
func (r *Reader) eventArrival(ts int64, arrivalTime bool, vals []event.Value) (int64, error)                            // lines 499-509 via advance; cc 4
```
`eventKeys` order: `!present(eAttrs)`; `arrivalTime && !present(eTs)`; `!arrivalTime && present(eTs)`; `arrivalKeys` (its three cases in written order); `present(eBatch) && vals[eBatch].Str == ""` — the seven checks keep their written order (a line with no attrs and an empty batch still names "an event needs attrs"). `eventRecord` keeps `stream`, `st`, `name`, `arrivalTime`, the ts choice and the batch; cc 7, ~25 lines. Transfers: `eventKeys`' 5 returns, `arrivalKeys`' 3, `eventPayload`'s 1, `eventArrival`'s 1 — one propagation mutation each. Per event line ~7 direct calls more, against 5–6 allocations and a JSON decode per line; expected under 2 % (inferred); the coordinator measures.

**`runBlock`** (`expect.go:49`; cc 14; not hot, deploy-time expects; risk LOW).
```go
func seedGivenTables(d *Driver, x *plan.Expect) (table int, err error) // lines 65-71; returns the failing gt.Table and TableWrite's own error; cc 4
func feedGivenLists(d *Driver, x *plan.Expect)                         // lines 73-88, the `for {}` with its break moved whole; cc 3
func nextGiven(x *plan.Expect, next []int) int                         // the best-list choice (lines 75-80): earliest ts, ties to the list written first; cc 5
func blockCounts(st runtime.Stats, x *plan.Expect) Counts              // lines 92-100; cc 3
```
`runBlock` keeps `fault`, the `defer`/`recover`, `NewPlanDriver`, then `if tid, err := seedGivenTables(d, x); err != nil { return BlockResult{}, fault("given table %s: %w", p.Tables[tid].Name, err) }` (one `%w`, as today), then `feedGivenLists`, `d.End()`, `res`, `res.Counts = blockCounts(d.Stats(), x)`, `diverge`; cc 4. The inner `err` shadows the named result only inside the `if`, and every return there sets both results; the deferred recover still writes the named results. Transfer: `seedGivenTables`' error return (mutation: drop it).

**`diverge`** (`verdict.go`, was `expect.go:106`; cc 29, 100 lines; not hot; risk LOW).
```go
func matches(o []event.Event, t []plan.ThenRecord, i, j int) bool                                                   // was the match closure; cc 3
func inOrder(o []event.Event, t []plan.ThenRecord) bool                                                             // every output i matches then-record i (caller has len equal); cc 3
func firstMisplaced(o []event.Event, t []plan.ThenRecord) (thenIdx, outIdx int)                                      // lines 131-152; -1,-1 unless n == m, all matched, one misplaced; break kept; cc 10
func matchedPrefix(o []event.Event, t []plan.ThenRecord) int                                                        // lines 157-160; cc 4
func thenRecordFirst(o []event.Event, t []plan.ThenRecord, i int) bool                                              // the switch of 178-185 and line 203 in that order; cc 4
func lcsLength(o []event.Event, t []plan.ThenRecord, a, b int) int                                                  // the lcs closure, linear memory; cc 4
func missingThen(target event.Schema, o []event.Event, r plan.ThenRecord, i int, reason string) (string, int, int)    // was thenFirst; cc 2
func unexpectedOutput(target event.Schema, x *plan.Expect, o []event.Event, i int, reason string) (string, int, int) // was outputFirst; cc 2
```
`diverge` order: panics; `len(o) == len(t) && inOrder(o, t)` → pass; `reason := drops(c)`; `firstMisplaced`; `i := matchedPrefix`; `thenRecordFirst` (`missingThen` if true, else `unexpectedOutput`); cc 6, ~16 lines. No new struct; every helper ≤ 5 parameters. `thenRecordFirst` keeps the order `i == m` → false, `i == n` → true, the cap → true, then `lcsLength(o,t,i,i+1) >= lcsLength(o,t,i+1,i)`. Mutations: swap the first two cases; delete the cap case; `>=` → `>`; drop `!used[i]`; drop the `break`.

**`sameValue`** (`verdict.go`, was `expect.go:244`; cc 15; not hot; risk LOW). `func samePayload(a, b event.Value) bool` (the kind switch, the float sign check kept, TObject → `sameObject`; cc 9); `func sameObject(a, b any) bool` (both strings and equal; the owner 2026-11-10 comment moves with it; cc 3). `sameValue` keeps the null rule and the kind check, then `return samePayload(a, b)`; cc 5.

### P.3 Nets (lane P's nets commits, before any split)

**Coverage now** (in-package, measured; package 90.4 % against the floor of 90): `AppendValue` 94.4, `appendString` 100, `appendEvent` 95.0, `runBlock` 95.0, `diverge` 98.4, `sameValue` 46.7, `NewReader` 83.3, `validateApp` 82.4, `readLine` 90.5, `record` 96.9, `rejectedRecord` 59.1, `tableRecord` 96.2, `eventRecord` 97.1, `(*Recorder).Table` 66.7, `(*Trace).Step` 100. The splits only move blocks and the nets add tests, so the package's percentage can only rise; the blocks left short are those of §10 Q3. Characterisation tests (behaviour-named, in the package; each asserts the exact text and, for the reader, `LoadError.Line`):

| Function | Uncovered `file:line` | Test (and the operand cases it adds) |
|---|---|---|
| AppendValue | `encode.go:61` | `TestAppendValueRefusesAValueOfNoType`: `{Kind: TInvalid}`, `{Kind: TInvalid, Null: true}` (the `v.Kind != TInvalid` operand), `{Kind: event.Type(99)}`; also NaN, +Inf, −Inf, finite (`||`) and `min−1`, `max+1`, in range (`||`) where the encode tests lack them |
| appendString | none | `TestAppendStringEscapesEachClass`, only if the encode tests miss any of: 0x1f, `"`, `\`, plain; a literal U+FFFD (size 3: raw) vs a malformed byte (size 1); U+2028 and U+2029 |
| appendEvent | `encode.go:138-139`, `:143-144` | `TestAppendEventRefusesAWrongValueCount`; `TestAppendEventOrdersMoreThan64Attributes` (65 attributes in reverse name order, keys sorted; its allocation count pinned as measured, expected 1, inferred); operands of `skip != "" && a.Name == skip`: skip empty; matching; set but absent from the schema |
| Recorder.Table | `recorder.go:152-153, 157-162, 168-170, 177-178, 183-184, 189-190` | `TestRecorderTableRefusesAnUnknownTable` (−1 and len, the `||` operands); `TestRecorderTableWritesADeleteAsItsKeyColumns` (exact line `{"op":"delete","row":{"k":"a"},"table":"T"}`); `TestRecorderTableWritesEveryColumnOfAPut` (two columns, exact line); `TestRecorderTableRefusesARowOfTheWrongWidth`; `TestRecorderTableRefusesAnUnencodableValue` (a NaN column); `TestRecorderTableStopsAtTheFirstFailedWrite` (a failing writer); `TestRecorderTableStopsAtTheFirstBadRow` (good, bad width, good: exactly one line) |
| Trace.Step | none | `TestTraceNamesAnUnknownTableByItsId`, if missing (−1 → `"#-1"`, len → `"#<n>"`, the `&&` operands) |
| NewReader | `reader.go:124-125, 136-137, 141-142` (and three defensive blocks, §10 Q3) | `TestNewReaderRefusesATableItCannotDecode` (no attributes); `…AKeyColumnNotDeclared`; `…AKeyNamedTwice` (`Key: ["k","k"]`; that `event.NewSchema` refuses duplicates is inferred, the lane confirms) |
| validateApp | `reader.go:153-154, :160, :172` | `TestNewReaderRefusesAnAppItCannotRead`: nil app; a zero stream; a stream declared twice; a negative `Delay`; a negative `IdleAfter` (the `||` operands); a table with no name; a table declared twice |
| readLine | `reader.go:221-222, 232-233` | `TestReaderStopsAnEndlessLine`: a reader yielding `'a'` that returns a sentinel error after 2×maxLine bytes; want the `LoadError` "the line is longer than 16777216 bytes" at line 1, never the sentinel. `TestReaderReturnsAReadError`: Next returns the reader's error itself. Operands: EOF with an empty chunk, EOF with a last line and no newline, another error, nil; `"\n"` alone, `"\r\n"`, a final `"\r"` |
| record | `reader.go:269` (defensive, §10 Q3) | `TestReaderNamesTheFirstFaultOfALine` (two-fault lines): an unknown key after the stop names the unknown key; a line after the stop with no choosing key names the stop; table plus stream keys names `a table record has no "stream" key` |
| rejectedRecord | `reader.go:317-345` (all its error returns) | `TestReaderRefusesAMalformedRejectedRecord`: no header; no stream; no arrival; neither (the `||` operands); unknown stream; derived stream; a nested decode fault (`"reason":1`); a nested unknown key; reason `"bogus"`; syntax with an attr; type without an attr; arrival before the header's start; plus the valid cases (syntax with no attr, each other reason with an attr) |
| tableRecord | `reader.go:402` | `TestReaderRefusesAPutRowWithAnUndeclaredColumn` |
| eventRecord | `reader.go:486` | `TestReaderRefusesAnEventWithAnEmptyBatch`; every operand of the seven-case switch (`reader.go:471-487`): arrival stream with and without ts; attribute stream with and without ts; arrival with no header; arrival on an arrival stream in a recording; a recording's attribute-time event without arrival; batch present but empty; batch present and non-empty (the lane fills only those `reader_test.go`/`s3_2b_2_errors_test.go` lack) |
| runBlock | `expect.go:56-57, 63-64` | `TestRunExpectsReportsAPanicOutsideTheRule`: a built plan whose then-record names attribute 99 of a rule that emits, so `matches` indexes past the output's values; want "…expect block 1: a panic: runtime error: index out of range…" (panic site inferred, the lane confirms). `TestRunExpectsReportsASandboxThatCannotBeBuilt`: `plan.App{Streams: []plan.Stream{{}}, Rules: []plan.Rule{{Name: "R"}}, Expects: []plan.Expect{{Block: 1}}}` gives "playback: rule R, expect block 1: playback: stream 0 has no schema". `TestRunExpectsCountsAStreamsLateEventsOnce` (two hand-built lists of one stream: the `!given` false branch). Operands of `expect.go:77`: an exhausted list, the first candidate, earlier wins, a tie to the first written (exists, `expect_test.go:199`), later loses |
| diverge | `expect.go:184` | `TestDivergeReadsByPositionPastTheAlignmentCap` (internal): o = [X, t0…t1999], t = [t0…t1999]; 2001×2000 > alignCap, so "then-record 1 has no output…" (without the cap the LCS would choose "output 1 was not expected…", so the mutation shows red) |
| sameValue | `expect.go:246-247, 249-250, 259, 263, 265-267, 269` | `TestSameValueIsTheExpectEquality` (internal, table): both null, a null only, b null only, kinds differ, each kind equal and unequal, +0.0 vs −0.0, NaN vs NaN (false), an object equal / unequal / either side not a string, a non-null TInvalid (false) |

v2 (QA DQ-S8): `TestReaderNamesTheFirstFaultOfARejectedRecord` (one two-fault case per adjacent boundary of `rejectedFault`) and `TestReaderNamesTheFirstFaultOfAnEvent` (no attrs + an empty batch → "an event needs attrs"; one case per adjacent boundary of `eventKeys` and `arrivalKeys`). One mutation per new test and per moved control transfer (P.2), each quoted with its diff.

**Performance nets (hot functions only).** New files `internal/playback/encode_bench_test.go` and `internal/playback/replay_bench_test.go` (package `playback`), not edited after the nets commit. `AllocsPerRun` equality pins, measured now:

| Function | Fixed input | Allocations now |
|---|---|---|
| AppendValue | each kind (string with quotes, U+2028 and `\xff`; int; float; bool; timestamp; duration; object `{"a":1}`; a null int) | 0 each |
| appendString | `plain ascii text with "quotes" and \n and \x01 and é` | 0 |
| appendEvent | `S(k string, n int, ts timestamp)` with `{a,1,1}`, batch `b` | 0 (also the existing `TestAppendEventZeroAllocs`, `encode_test.go:328`, and `breakit2_test.go:396`) |
| Recorder.Table | put `T(k string, v int)` key k | 0 |
| Recorder.Table | delete | 0 |
| Recorder.Event | typed (payload nil) | 4 |
| Recorder.Event | with a payload | 0 |
| Trace.Step | each of the 8 kinds, buffer reset | 0 |
| Reader.Next | fixture event / event on a `time arrival` stream / recording event / table put / table delete / batch end / tick / rejected | 6 / 5 / 6 / 6 / 5 / 0 / 0 / 6 |
| readLine | every kind above | 0 |

Benchmarks: `BenchmarkAppendValue/<kind>`, `BenchmarkAppendStringPlain` (64 B ASCII), `BenchmarkAppendStringEscapes` (the string above), `BenchmarkAppendEvent`, `BenchmarkRecorderTable/{put,delete}`; the recording path `BenchmarkRecordOneEvent/{typed,payload}` to `io.Discard`; tracing `BenchmarkTraceStepTable`; the replay path `BenchmarkReplayLine/<kind>` (the 8 kinds; Next over a reader that repeats the line, after the header for the recording kinds) and the leaf `BenchmarkReadLine`. Escape check (v2): `escapes.py` (§2.4, §3.2) per split commit, the original function and its new helpers as one group; a helper returning `dst` adds `leaking param: dst to result` lines, which are diagnostics (QA DQ-B2, measured on `AppendValue`'s split); E1–E5 hold, E2 checked by the pins.

### P.4 Placement

- `validateApp` stays (confirmed): it checks `playback.App` (an alias of `plan.App`, `playback.go:25`) for `NewReader` (`reader.go:94`) and `newDriver` (`driver.go:36`); `validate` may import only event, model, expr, window and metrics (`rules.json:17`); runtime's `validateApp` checks `runtime.App`.
- The recording-size arithmetic stays in runtime (backlog); playback's side changes nothing.
- Backlog, recorded for the owner (neither among the 110): the RFC 3339 range written twice (`encode.go:16-19` and `event.TimestampInRange`, `internal/event/decode.go:300`, used by `stamp`, `recorder.go:34`); `reasonOf` (`reader.go:351`) and `reasonNames` (`recorder.go:102`), inverse maps kept in two files.

### P.5 Risks and owner questions (recommendation first)

- **P-Q1 — blocks no in-package test can reach** (decision 2 allows "an invariant panic, or reachable only by scheduling"; these fit neither): `reader.go:99-100, 104-105, 109-110` (`event.NewSchema` on three constant, valid schemas); `reader.go:269`, `:524` (a `Decode` error that is not a `*event.DecodeError`: `decode.go` builds its errors only as `&DecodeError`, `:140, :149, :336, :505`; lane E re-checks after its split); two redundant operands — `reader.go:159` `s.Schema.NumAttrs() == 0` alone (`NewSchema` refuses an empty name and zero attributes, `event.go:98-101`; `Schema`'s fields are unexported, `event.go:64-68`) and `reader.go:234` `n > 0` (always true there). → list them as left short with the reason "defensive default: constant or contract-guaranteed input", reviewed at G1, code unchanged (§10 Q3). Others: an invariant panic with a PERMANENT waiver each (a code change with no test that can fail first); deleting the redundant operands (behaviour-neutral but not a pure split).
- **R1 — replay time.** ~7 extra direct calls per event line, projected under 2 % against a µs-scale `Next` (inferred). If the measurement fails decision 1, `eventRecord` keeps `arrivalKeys` inline, and at worst stays whole with a PERMANENT waiver citing the samples.
- **R2 — `(*Recorder).Table`'s 0-allocation delete path depends on E4** (`cols` passed down, never returned); the pin catches a slip.

### P.6 Commit plan (lane P, 9 commits; caps gocyclo 15, funlen 1, file length 1)

1. `test(playback)`: the coverage nets of P.3, mutations quoted. Counts unchanged.
2. `test(playback)`: the alloc pins and benchmarks (`encode_bench_test.go`, `replay_bench_test.go`). Counts unchanged.
3. `refactor(playback)`: pure moves to `records.go` and `verdict.go`; `astequal` exit 0. File length 1 → 0.
4. `refactor(playback)`: `encode.go` (`AppendValue`, `appendString`, `appendEvent`). gocyclo 15 → 12.
5. `refactor(playback)`: `recorder.go` `Table` and `trace.go` `Step`. 12 → 10.
6. `refactor(playback)`: `reader.go` (`NewReader`, `validateApp`, `readLine`). 10 → 7.
7. `refactor(playback)`: `records.go` `record` alone (every replay kind's benchmark reaches it). 7 → 6.
8. `refactor(playback)`: `records.go` `rejectedRecord`, `tableRecord`, `eventRecord` (each its own replay-kind benchmark). 6 → 3.
9. `refactor(playback)`: `expect.go` `runBlock` and `verdict.go` (`diverge`, `sameValue`). gocyclo 3 → 0, funlen 1 → 0.

Sub-lanes: P1 = commits 1–2; P2 = 3–6; P3 = 7–9.

Verifiers: plan check §7 row P (the baselines, the corpus test, `determinism`) and `--job proofs` by the coordinator.

---

## 9. internal/plan (lane Q, with the parser: Q1 → Q3)

### Q.1 File map (after the sprint)

| File | Concern | Holds | Projected lines |
|---|---|---|---|
| `plan.go` | building an app's description | package doc, `App`, `Stream`, `Connector`, `Table`, `Rule`, `Stats`, `Counted`, `ErrNotBuilt`, `Build`, `attrs`, `connector`, `litText`, `stream`, `rule`, **`(*Rule).key`, `keyAttrs`, `newBody`**, `selected` (lines 1-280) | ~300 |
| `body.go` (new) | the plain body: compiling and running it | moved from `plan.go:281-472` (`scope` and its methods, `bodySpec`, `projSpec`, `compile`, `Anchor`, `state`, `Walk`, `proj`, `row` and its methods, `body`, `Stats`, `NewState`, `OnEvent`, `out`, `addSat`); new **`(*bodySpec).project`, `(*body).passes`, `(*body).project`** | ~240 |
| `expect.go` | expect blocks as values | `Expect`, `GivenTable`, `GivenList`, `GivenEvent`, `ThenRecord`, `expects`, **`givens`, `givenTable`, `givenList`, `thens`, `thenRecord`, `schemaIndex`**, `values`, **`fieldIndex`**, `value` (+ **`literalValue`, `intValue`** in variant b) | ~250 (b) / ~230 (a) |

Pure move first: `plan.go:281-472` → `body.go`, `astequal` exit 0 (without it, `rule`'s helpers would push `plan.go` to ~497 lines, at the limit). The PERMANENT `//nolint:forbidigo,lll` panic (`plan.go:258`) moves into `newBody`, on the same line as its `panic`.

### Q.2 Split plan per finding

**`expect`** (`expect.go:75`; cc 20, 69 lines, 6 parameters; not hot; risk HIGH, lower bound — 20 call sites untyped; d=1 `expects`). **No existing domain type carries the six:** `streams` is `Build`'s `ids` (`plan.go:85-93`), `tables` is built in `expects` (`expect.go:54-57`), no plan or model type holds both maps, `b` is `&r.Expects[bi]` and `r` is used only for `InsertInto`. So `expect` is **dissolved**: `expects` composes two four-parameter functions; no new struct.
```go
func givens(app *model.App, streams, tables map[string]int, b *model.Expect) (Expect, error)     // Line/Col/ThenLine/ThenCol + Tables + Lists, givens in written order; cc 5
func givenTable(app *model.App, tables map[string]int, g *model.Given) (GivenTable, error)       // lines 80-92; cc 4
func givenList(app *model.App, streams map[string]int, g *model.Given) (GivenList, error)        // lines 95-117, timeIdx := fieldIndex(st.Attrs, st.Time.Attr); cc 5
func thens(p *App, streams map[string]int, target string, t *model.Then) ([]ThenRecord, error)   // lines 119-143; nil when no records (as today); cc 5
func thenRecord(target event.Schema, stream string, rec model.Record) (ThenRecord, error)        // lines 125-141; cc 4
func schemaIndex(s event.Schema, name string) int                                                // the last attribute named name, -1 if none; cc 3
```
`expects`: `x, err := givens(app, streams, tables, b)`; `if err == nil { x.Then, err = thens(p, streams, r.InsertInto, &b.Then) }`; then the same wrap "plan: rule %s, expect block %d: %w (Build is given only a checked model)"; cc 6. Order unchanged (every given in written order — unknown table, its rows; unknown stream, no time attribute, its records — then the target check, then the then-records). Every index search keeps the **last** match, with no early return (an unchecked model with duplicate names behaves as today). Transfers: `givens`' returns; `thens`' target and record returns. Mutations: drop the `err == nil` guard (a given fault would be overwritten by a then fault); drop each propagation.

**`values`** (`expect.go:149`; cc 11; not hot; risk LOW). `func fieldIndex(attrs []model.Attr, name string) int` (the last attribute named name, -1 if none; cc 3). `values` unchanged apart from `idx := fieldIndex(attrs, f.Name)`; cc 9.

**`value`** (`expect.go:181`; cc 18; not hot; risk LOW; d=1 `expect`, `values`).
- **(a) With the merge (§5 §4.1; the coordinator's commit, before lane Q):** `value` keeps `case l.Kind == model.LitNull` (typed null), then `case l.Kind == model.LitString && t == event.TObject` (with `CleanObject(TrimSpace)` and its comment, owner 2026-11-10), then `if v, ok := model.LitValue(l, t); ok { return v, nil }`, then the same `fmt.Errorf("a %s literal is not a %s", l.Kind, t)`; cc 5. Verified for plan: callers exactly `expect.go:136` and `:167`; no test calls `value` directly; the error text and the TObject string case as described; `strings` still needed for TrimSpace; plan may import model with no fence (`rules.json:16`). **Two conditions the merge must meet, or plan changes:** (i) `LitValue(l, t)` true for exactly seven pairs — string/string, int/int, int/float, float/float, bool/bool, duration/duration, timestamp/timestamp — and false for every other pair, **including `(LitParam or an unknown kind, TInvalid)`** (if built as `t == LitType(l.Kind)` with `LitType` returning `TInvalid` for param, null or unknown kinds, `(LitParam, TInvalid)` becomes a value where plan returns "a param literal is not a invalid" today; plan's tests feed unchecked models, `internal_test.go:88`, so the input is reachable); (ii) `litPayload` sets only the type's own field (the widened float has `I` = 0). A `model` test for each.
- **(b) Without the merge (split in place, commit Q3):**
```go
func literalValue(l model.Literal, t event.Type) (event.Value, bool) // switch l.Kind: each case returns (event.Value{Kind: t, <field>: …}, t == <its type>); LitInt → intValue; default false; cc 7
func intValue(n int64, t event.Type) (event.Value, bool)              // TInt → I; TFloat → F: float64(n); else false; cc 3
```
`value` has exactly (a)'s shape with `literalValue` in place of `model.LitValue`; cc 5. (b) is the merge's function, kept in plan.

**`rule`** (`plan.go:196`; cc 18, 67 lines; not hot; risk CRITICAL — 1 direct, 56 at depth ≤ 3, through Deploy).
```go
func (s *bodySpec) project(out event.Schema, items []model.SelectItem) error           // lines 213-227: nattrs, tsIdx, sel; in body.go; cc 4
func (pr *Rule) key(r *model.Rule, src event.Schema) error                             // lines 232-254: skip other streams, keyed twice, KeyAttrs/KeyTypes/TTL; cc 5
func keyAttrs(k model.PartitionKey, src event.Schema) ([]int, []event.Type, error)     // lines 239-252 with the E-NAME-001/E-PART-006 comment; cc 6
func newBody(spec bodySpec, r *model.Rule) func(emit func(event.Event)) partition.Body // lines 255-262; the PERMANENT nolint stays on the panic line; cc 2
```
`rule` keeps the body kind, source, target, `sc`, `spec`, the filter; then `spec.project(p.Streams[target].Schema, r.Select)`, `spec.compile()`, `pr`, `pr.key(r, sc.schema)`, `pr.NewBody = newBody(spec, r)`; cc 8, ~30 lines. `spec` is not mutated after the closure is made and `compile` only reads `s` (`plan.go:333-357`), so capturing it by value equals today. Transfers: `project`'s unbound-attribute return; `key`'s keyed-twice and `keyAttrs` returns.

**`(*body).OnEvent`** (`body.go`, was `plan.go:415`; cc 11; hot — rule evaluation under the `Send` path, plan check §3; `impact` resolves no callers: UNKNOWN, called through `partition.Body`, `internal/partition/partition.go:57, :330, :344`).
```go
func (b *body) passes() bool                    // the filter: nil passes; an error counts Errored and fails; `if v.Null || !v.B { return false }`; cc 4
func (b *body) project(vals []event.Value) bool // the select loop: an error counts Errored and fails; the int widening kept; cc 5
```
`OnEvent` keeps both `defer`s and their order (`b.row = row{}` registered first, `clear(vals)` after the buffer); calls `if !b.passes() { return }` before the buffer and `if !b.project(vals) { return }` after `defer clear(vals)`; the ts slot and `out` stay; cc 5. An early return runs the same defers as today. `vals` reads "does not escape" in `project`; `OnEvent` already cannot inline (`unhandled op DEFER`, measured). Mutations: drop `Errored++` in `passes`; `passes` true for a null result; `OnEvent` ignores `project`'s false (the lane confirms which existing test turns red for each: `TestFilterTrueFalseNull`, `TestDivisionByZeroDropsForThatRule`).

### Q.3 Nets

**Coverage now** (in-package, measured; package 95.4 %): `expect` 100, `values` 100, `value` 100, `rule` 91.5, `OnEvent` 100.

| Function | Gap | Test |
|---|---|---|
| rule | `plan.go:234, 237-238, 249-251` | `TestBuildRefusesAnUncheckedPartition` (package `plan`, hand-built model as at `internal_test.go:88`): a key of another stream is skipped (the rule stays key-less); keyed twice; an unknown key attribute; a key attribute named twice (both `||` operands) |
| rule | `plan.go:258` | left short with its reason: an **invariant panic** (decision 2's allowed reason) — `compile` succeeded at Build, and the same spec compiles again the same way |
| expect | operands | `!ok` alone: then names an unknown stream → "then names Nope, not the rule's target Out"; two-fault: an unknown given table plus a wrong then → "unknown table U" (a new behaviour-named test in `expect_test.go`; the sprint-named `coverage_s3_3_2_test.go` not touched) |
| values | operands | text_invisibles in a given stream (false slot) vs in a table (null); the null time attribute on a table row (timeIdx −1) vs a stream |
| value (b) | operands | `TestValueConvertsALiteralToItsFieldType` (package `plan`, direct, exact struct equality): each kind with its own type; int → float (`I` stays 0); each kind with a wrong type; param; null; string → object |
| value (a) | operands | int → object → "a int literal is not a object" (the `&&` operands of the TObject case); the `LitValue` pairs tested in model by the merge |
| OnEvent | operands | a null, false and true filter (`TestFilterTrueFalseNull` exists); a widened int vs an unwidened value |

Performance (v2, DA-S7): `BenchmarkBodyOnEvent/{filtered-out, projected, widened}` in a new `internal/plan/body_bench_test.go` (package `plan`), in lane Q's nets — the runtime's `Send` path benchmark uses a stub body that never reaches `OnEvent` (`internal/runtime/api_test.go:47-221`). Pins: the existing `TestFilterNoMatchAllocatesNothing` (`body_test.go:367`, 0), and playback's `TestSuppressedMatchAllocatesNothing` and `TestBreakitNoMatchResidentKeyAdvancingAllocs` (0, on OnEvent's replay path); the emitting path pinned at its measured value (expected 1, the `slices.Clone` in `out`; inferred). Escape diff: `-gcflags=-m=2 ./internal/plan` for `OnEvent`, `passes`, `project`.

### Q.4 Placement

`value` → `model.LitValue`: the merge (§5 §4.1), the owner decides at G1. Nothing else in plan is misplaced (`litText`, `plan.go:145`, formats connector keys — another concern).

### Q.5 Risks and owner questions (recommendation first)

- **The merge's two conditions** (Q.2 (a) (i), (ii)) → the merge's `model` tests include `LitValue(Literal{Kind: LitParam}, TInvalid)` → false and exact-struct values for all seven pairs; without them, plan's `value` can change on unchecked models (§10 Q1).
- **R — `OnEvent` time:** two extra direct calls per event per rule, measured under `Send` (µs-scale), projected well under 2 % (inferred); if it fails, `OnEvent` stays whole with a PERMANENT waiver citing the samples.

### Q.6 Commit plan (plan's share of lane Q, 5 commits; caps gocyclo 5, funlen 2, argument limit 1)

1. `test(plan)`: the nets of Q.3. Counts unchanged (with the merge already in: gocyclo 4).
2. `refactor(plan)`: pure move of `plan.go:281-472` to `body.go`; `astequal` exit 0.
3. `refactor(plan)`: `expect.go` (`expect` dissolved, `values`, and `value` in variant b). Argument limit 1 → 0, funlen 2 → 1, gocyclo 5 → 2 (b) or 4 → 2 (a).
4. `refactor(plan)`: `plan.go` `rule`. Funlen 1 → 0, gocyclo 2 → 1.
5. `refactor(plan)`: `body.go` `OnEvent` (hot: its own commit). gocyclo 1 → 0.

Verifiers: plan check §7 row Q (the plan and expect tests, `fragments`, `docsync-d`; the nightly `fuzz-parser`; `--job proofs`).

## 10. Questions for the owner at G1 (recommendation first; options ranked by code health) — answered 2026-11-16

*The answers ([decisions](decisions.md#owner-decisions-2026-11-16--s11c-6s-design-g1)): Q1 validate and plan only; Q2 the four tools; Q3 kept with a guard test (asked again with the code); Q4 only a named `--- FAIL`; **Q5 a 5 % path margin (≠ R)**; Q6 accepted; **Q7 fixed now (≠ R)**; Q8 confirmed. The text below is the questions as asked.*

- **Q1 — the literal-to-value merge** (§5 §4.1; §9 Q.2; DA-B1). → **Merge validate's `fits` and plan's `value` into `model.LitValue` (with `LitType`, `litPayload`) in a new `internal/model/literal.go`, by lane M before lanes V and Q; leave `expr.literalNode` as it is.** The proof: each caller's 9 × 9 table (the 8 named kinds plus an unknown; the 8 named types plus `Type(99)`) pinned green first, the model functions added red first with `(LitParam, TInvalid)` → false and the exact value of each of the seven accepted pairs, then the callers switched with their tables unchanged. One rule for what validate accepts and plan converts, by construction; no new import edge; your fence on expr's use of `model` (2026-11-04: "expr reads `model.Expr` only") untouched. Others: all three callers, with that fence decision amended (expr would call model behaviour); no merge (split `plan.value` in place).
- **Q2 — the W0 tools as new scope** (§3; DA-S13). → **Build the four small tools in `gate/bench/` (the comparer, the escape check, the mutation runner, the load runner), test first, by a `python-engineer` before the lanes.** The reviews showed each hand procedure going wrong in a way that would record a false proof: a mutant that does not compile counted as caught; a killed run leaving a mutant in a tracked file (it happened during the review); a p-value wrong under tied timings; a free split called costly by a plain escape diff (measured). The plan check's `astequal` rename map is dropped (the merge is not a move). Others: hand procedures with these rules written out (cheaper, and the failure modes stay); only the comparer (the plan check's W0).
- **Q3 — a third reason for a block left short.** Two lists (DA-S9, QA DQ-L3):
  - **(a) Reached only by a direct call or white-box state in the package's own tests** (validate `build` `exprs.go:107-109`, `mayBeNull` `:367-368`, `:400`; event `decode.go:200`; parser `lexer.go:162-163`, `lower.go:542`, `:553`; runtime's in-place router and shard cases, the `a == nil` slot guards each lane shows reached). → **This meets decision 2 ("from its own package's tests") as it stands, on two conditions: the function keeps its signature (§2.1), and the test says in a comment which contract keeps the block unreachable through the API.** A note, not a question — unless you disagree.
  - **(b) Unreachable by any test** (runtime `boundary.go:114-116`; playback `reader.go:99-110`, `:269`, `:524`; the redundant operands event `decode.go:834`, playback `reader.go:159`, `:234`; any `a == nil` guard a lane cannot reach). → **Accept a third reason, "defensive: guaranteed by <the named contract>", code unchanged, each with a guard test that fails if the contract changes** (e.g. every error from the decoder's fault table satisfies `errors.As` with a `*event.DecodeError` target; `envelopeSchemas` succeeds), **removals to the backlog.** Others: turn each into an invariant panic with a waiver (a code change with no test that can fail first); delete them now, each in its own commit (behaviour-neutral, but not a pure split).
- **Q4 — how a mutation counts** (§2.3, §3.3; DA-S1, QA DQ-B1, S4, S5, S9). → **Only "red" counts: the mutant builds and its designated test reports `--- FAIL` — including a failure at the test's own context deadline; `go test`'s `-timeout` backstop is "timeout" and a build or vet failure is "invalid", neither counted; a scheduling-dependent mutant runs 20 times under load and is reported k/N, and one red in none is named "reasoned, not proven"; an equivalent mutant is listed with its argument beside a different mutation of the same transfer shown red; each transfer's mutation is shown red at the nets commit (on the original code) and at the split commit (on the helper).** Others: count deadline reds only at a stricter `-timeout` (the drain and stop transfers would then have no proof); refuse equivalent mutants (some transfers cannot be mutated observably).
- **Q5 — a path that costs while every split passed** (§2.4 step 6; DA's missing question). → **The lane-level path comparison at each hand-back catches it early; if a path still costs, the split with the largest measured share is restored whole with its waiver first, then the path is re-measured; if it still costs, it comes to you with every per-function verdict beside it.** Other: accept any path cost under a set margin (e.g. 3 %), which needs a number from you.
- **Q6 — the one reordering:** `runtime.build` defaults `RecordQueue` before the validate loop (§4 item 9). → **Accept** (unobservable: `validateApp` reads no `cfg` field; an error discards `cfg`; verified by both reviews).
- **Q7 — the latent panic at `internal/parser/parse.go:363`** (`strings.Fields(s)[0]` on the notation `"\t"`; participle never yields one, inferred). → **To the backlog, with `"\t"` recorded now as its failing input and a red test when it is taken.** Other: fix it now (a behaviour change and a scope change).
- **Q8 — confirms:** the parser's 4-field `failSite` (it gives `parseError`'s three closures a home as methods; within §7.3); `cleanObject` and `CleanLen` landing at exactly cyclomatic 10 (gocyclo reports > 10); the sub-lane cut and the commit budget of §1 (at most 97 of 100, with the squash rule); and the backlog items the design found — `TestHandoffAllocationBudget`'s misleading comment, the RFC 3339 range written twice (`internal/playback/encode.go:16-19`, `event.TimestampInRange`), `reasonOf`/`reasonNames` as inverse maps in two files, the recording line's size arithmetic in runtime, `validate.deepCopy` as a model clone, `CheckRows`' non-finite condition duplicating `nonFinite` (DA-L3, unless lane R uses `nonFinite` when the code moves), and list (b)'s removals.

## 11. Risks

1. **A hot split that costs time** (likeliest: `event.(*Decoder).value`, `(*scanner).number`, `appendUnescaped`, `playback.eventRecord`, `parseRFC3339`'s residual call, runtime's `offerAll` copy). *Mitigation:* per-function commits and `c^`-vs-`c` verdicts (§2.2, §2.4); a costed split restored by a new commit; the lane-level path check.
2. **The measurement gate becomes the critical path** (DA pre-mortem 1). *Mitigation:* fixed points at hot hand-backs with no agent running; ABBA order and the exact tied p-value; an A/A run per session — if an A/A run ever says "costs", the comparer is fixed before any verdict stands.
3. **Nets weaker than recorded** (DA pre-mortem 2). *Mitigation:* `mutate.py`'s outcomes (only a named `--- FAIL` is red); five-run coverage; stress under load with its own coverage profile.
4. **A lifecycle regression only load shows.** *Mitigation:* §4's invariants, the §2.1 concurrency rules, the deadline and stress tests, Drain's I1 and I3 mutations k/N, `race -count=3`, the nightly `race-stress`.
5. **Coverage under the gate's floor** (playback 90.4 % against 90). *Mitigation:* nets first; splits only move blocks; the five-run measure at hand-back.
6. **Sub-lanes overrun their box.** *Mitigation:* the cut of §1 at the spike's rates; a wrap-up at the box; the next sub-lane starts from the last commit.
7. **The commit budget** (at most 97 of 100). *Mitigation:* counted at each combine; the squash rule for non-hot split commits.
8. **Mutation cost** (a mutant red only by hanging costs at least the test's own 60 s ceiling, QA DQ-S4). *Mitigation:* one designated test per mutant; deterministic tests preferred to deadline reds; batch runs between agents' work.

## 12. Reviews of this note and their dispositions

The [architecture review](../reviews/s1-1c-6-design-architecture.md) of v1 (the `architect-reviewer` with `the-fool`) found 1 blocker, 14 should-fix and 11 later; the [QA review](../reviews/s1-1c-6-design-qa.md) (the `qa-expert`) found 2 blockers, 14 should-fix and 5 later. The coordinator verified each claim that changed the note (noted at the head of each filed review).

| Finding | Severity | Disposition |
|---|---|---|
| DA-B1 the merge's expr leg widens an owner fence | blocker | §0.5, §5 §4.1, §10 Q1: the recommendation narrowed to validate and plan; the fence untouched |
| DQ-B1 non-building or vet-failing mutants counted red | blocker | §2.3 step 3, §3.3: outcomes red / green / timeout / invalid; compilable specs; tests |
| DQ-B2 the escape rule calls a free split costly | blocker | §2.4 step 2, §3.2: `escapes.py`, moved/escapes lines per function group, `leaking param` a diagnostic |
| DA-S1 / DQ-S3 the runner writes tracked files; killed runs leave mutants | should-fix | §3.3: `-overlay`, own session, group kill; tests incl. SIGTERM |
| DA-S2 / DQ-S10 the comparer's statistics | should-fix | §3.1: exact conditional p with ties, ABBA, zero matches exit 2, both runs must say costs, fixed-iteration allocation pass, A/A |
| DA-S3 bundled hot functions; the baseline mis-attributes | should-fix | §2.2 step 3, §2.4 steps 3–4; the R, E, P, Q commit tables re-cut |
| DA-S4 the waiver template red at T0 | should-fix | §2.4 step 5; §6 E.2.1 |
| DA-S5 `broadcast` scatters `admStream = -1` | should-fix | §4 item 6; §2.1 |
| DA-S6 playback's `recordKind` closure parameter | should-fix | §8 P.2 `record` |
| DA-S7 `OnEvent` has no time net | should-fix | §9 Q.3 `BenchmarkBodyOnEvent`; Q.6 commit 5 |
| DA-S8 post-split tests | should-fix | §2.2 step 1 (none); §4 §3, §6 E.3, E.5 amended |
| DA-S9 / DQ-L3 Q3 mixes reachable and unreachable | should-fix | §10 Q3 lists (a) and (b) |
| DA-S10 / DQ-L2 three normalisers, one on a wrong premise | should-fix | §2.4 step 2, §3.2; §4 §3, §6 E.3, §8 P.3 amended |
| DA-S11 the sub-lanes do not fit the box | should-fix | §1 re-cut by the spike's rates |
| DA-S12 the measurement windows and restores unscheduled | should-fix | §1 (Measure, Restore), §2.4 step 5, §2.5 |
| DA-S13 the mutation runner as unasked scope; the rename map dropped silently | should-fix | §3 (dropped, said), §10 Q2 |
| DA-S14 / DQ-S11 the merge's proof framing | should-fix | §5 §4.1: 9 × 9 tables; expr's tests are external |
| DQ-S1 runtime coverage varies run to run | should-fix | §2.3 step 1 (five runs); §4 §3 `:908` |
| DQ-S2 validate's poison writes and resolved copy untested | should-fix | §5 §0 (O), §3.3 H12, H13, `TestCheckResolvesEveryParamSlot`, §3.4 |
| DQ-S4 the timeout outcome; mutation cost | should-fix | §3.3, §10 Q4, §11 risk 8 |
| DQ-S5 scheduling-dependent mutants | should-fix | §2.3 step 3 (k/N); Drain I1 and I3 mutations to R1b's list |
| DQ-S6 stress not under load; blocks not shown hit | should-fix | §2.3 step 4, §3.4 |
| DQ-S7 missing operand cases in runtime | should-fix | §4 §3 v2 additions |
| DQ-S8 two-fault cases per helper boundary | should-fix | §2.1 rule; §4 §3, §6 E.3, §8 P.3 rows |
| DQ-S9 when a transfer's mutation runs | should-fix | §2.3 step 3 (shown red twice) |
| DQ-S12 nets only in the scratchpad | should-fix | §4 §3 (the spike's tests named); §6 E.3 the committed reference differential |
| DQ-S13 the route-stop test cannot tell the mutant apart | should-fix | §4 §3: waits on state; the router's exit asserted with the body held |
| DQ-S14 concurrency counter-examples without a rule | should-fix | §2.1 concurrency rules |
| DA-L1 runtime file names | later | §4 §1: `requests.go`, `router_in.go`, `router_out.go` |
| DA-L2 by-value rule vs the `m *message` fallback | later | §2.1 |
| DA-L3 `CheckRows`' duplicated condition | later | §10 Q8 (backlog unless lane R uses `nonFinite`) |
| DA-L4 `parseRFC3339`'s 13 helpers; "equal results" | later | §0.3, §6 E.2.1 reworded; fewer helpers if the measurement allows (lane E's choice, recorded) |
| DA-L5 `failSite`'s derived fields | later | §7 Q.2.1 |
| DA-L6 the commit budget | later | §1 recounted (≤ 97) |
| DA-L7 W0 home, `__init__.py`, the record's provenance | later | §3 |
| DA-L8 a timer used as a probe | later | §4 §3 route-stop test |
| DA-L9 the widening rule stays in two places | later | §5 §4.1 |
| DA-L10 who writes the merge | later | §1 lane M |
| DA-L11 small runtime shapes and names | later | §2.1 (names by content); `ingest` → `ingestLifecycle` dispatching twice is gocyclo's shape |
| DQ-L1 `TestDrainBarrier` at 2 shards | later | §4 item 2 |
| DQ-L4 signatures nets tests call | later | §2.1 |
| DQ-L5 truncated allocation averages | later | §2.4 step 1 |
| DA pre-mortem and inversion | — | §11 risks 1–3, 6, 7 |
| DQ acceptance-line rewrites | — | to the sprint record's card at G1 (exits 2 and 3 as QA rewrote them, with the rename-map proof dropped) |
