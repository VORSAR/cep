# S1.1c-6 — sprint record

**Sprint:** S1.1c-6 — the strict lint set: the size limits in runtime, validate, event, parser, plan and playback · **Routine:** the full loop without the semantics panel · **Size:** 13 SP (the spike and the re-cut) against the 2-week cap · **Branch:** `s1-1c-6/integration`
**What to build and how each item is proven:** [the S1.1c-6 card](sprint-plan.md#s11c-6--the-strict-lint-set-the-size-limits-in-runtime-validate-event-parser-plan-and-playback) · **How it runs:** [the plan check §1](s1-1c-6-plan-check.md#1-routine-shape-waves) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, measured results, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan is [s1-1c-6-plan-check.md](s1-1c-6-plan-check.md) (v3), drafted read-only from `main` @ 4d2b9eb.
- **The re-measure.** The pinned golangci-lint under both `GOOS` gives the 139 size findings exactly as capped: 110 functions, 8 files over 500 lines, 2 six-parameter signatures, in 12 packages.
- **What the card did not know** (§0): T4, nightly and `proofs` red by construction from the G0 marker until the last size entry goes; 38 hot-path functions behind decision 6 with two benchmarks in the tree; the gate's coverage in-package only; proof drivers anchored on code in files to be split; no proof yet for a cross-package move.
- **The blind reviews of v1:** [architecture](../reviews/s1-1c-6-plan-check-architecture.md) 1 blocker, 15 should-fix, 6 later; [QA](../reviews/s1-1c-6-plan-check-qa.md) 2 blockers, 8 should-fix, 3 later. Every finding is placed in v2 §10.
- **The spike before G0** ([findings](../reviews/s1-1c-6-spike.md)): `ingest` split at no cost; `parseRFC3339`'s split costs about 18–20 % (four helpers over the inliner's budget, bounds checks 4 → 12). The lane work projected at about 18 agent-hours, against the roughly 2 the close-out re-cut assumed, so the part was re-cut by known issue (v3).

## Decisions at the gates

**Scope, while the plan check was drafted — the owner, 2026-11-15** ([decisions](decisions.md#owner-decisions-2026-11-15--s11c-6s-scope-before-its-plan-check); both as recommended):
1. **The sprint-named test files get their own part, S1.1c-7**, with a check refusing sprint ids in file names.
2. **Logic in the wrong package moves to the package it belongs in, each move approved by the owner at G1.**

**Plan check — the owner, 2026-11-15** ([decisions](decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check); all six as recommended):
1. **A hot-path split costs** only on unequal allocations, a new heap escape, or a slowdown both significant (Mann–Whitney, p < 0.05) and over 2 % on the median, from two prebuilt binaries measured alone, re-run once before the verdict stands; per leaf function and per path `main` against combined.
2. **The coverage bar:** each function to be split at 100 % of its blocks from its own package's tests, a case per operand, one mutation shown red per new test and per control transfer moved across a helper boundary; a block left short is listed with its reason.
3. **The exclusion machinery stays dormant here and is retired in S1.1c-7.**
4. **Runs red by construction** are recorded "red as expected" only with exactly the expected lines, never green.
5. **A function that resists a safe split** is split behind extra nets; if the lane still cannot, the owner decides then.
6. **Confirmed:** the 38 hot-path functions; the spike before G0; one PR within the counted budget; the `proofs` and nightly schedule; the placement candidates as the design's starting list.

**The re-cut after the spike — the owner, 2026-11-15** ([decisions](decisions.md#owner-decisions-2026-11-15--s11c-6-re-cut-after-the-spike); as recommended): **two parts by known issue.** S1.1c-6 cleans S1.1c-5-1 and -2 (91 findings); S1.1c-7 cleans S1.1c-5-3 (48) with its decided scope. S1.1c-5-3's forward and `LAST_PART` move to S1.1c-7 at G0.

**The card — the owner, 2026-11-15:** approved, so the sprint passes G0.

**The design — the owner, 2026-11-16** ([decisions](decisions.md#owner-decisions-2026-11-16--s11c-6s-design-g1); six as recommended, two ≠ R). The note is [s1-1c-6-design.md](s1-1c-6-design.md) v2, drafted per package by the repo's `refactoring-specialist` with its skills; its blind reviews: [architecture](../reviews/s1-1c-6-design-architecture.md) (the `architect-reviewer` with the-fool) 1 blocker, 14 should-fix, 11 later; [QA](../reviews/s1-1c-6-design-qa.md) 2 blockers, 14 should-fix, 5 later; every finding placed in the note's §12.
1. **The literal merge: validate and plan only**, by lane M before V and Q; expr untouched (its fence).
2. **W0 builds four tools in `gate/bench/`**: the comparer, the escape check, the mutation runner, the load runner.
3. **A block no test can reach is kept with a guard test** of its contract (a third reason); removals to the backlog.
4. **A mutation counts only as a named `--- FAIL`** of its designated test; timeouts and build failures do not count; k/20 for scheduling.
5. **A whole path may be up to 5 % slower** (≠ R: recommended 2 %); each split still at 2 %.
6. **`build`'s `RecordQueue` default before the loop: accepted.**
7. **The parser's latent panic is fixed now** (≠ R, a scope change): lane Q, its own commit, red first.
8. **Confirmed:** `failSite`; cyclomatic 10; the sub-lane cut and the budget (≤ 98 of 100); the design's backlog items.

**During the build — the owner, 2026-11-16** ([decisions](decisions.md#owner-decisions-2026-11-16--during-s11c-6s-lane-v-e-graph-001-reached-from-source); all three as recommended):
1. **E-GRAPH-001 reached from source is fixed in this sprint** (a scope change): a rule that reads a derived stream gets E-GRAPH-004 whatever else poisons it; lane V's one fix commit after its splits, red first.
2. **The 12 statements no test watched get their tests now** (a scope change): one test-only commit after lane V's splits.
3. **A cycle through an undeclared stream is silenced in the same fix:** the cycle check stays quiet when a rule in the cycle has a name finding; E-GRAPH-001's report line is listed unreachable in v0.1.

**During the build — the owner, 2026-11-17** ([decisions](decisions.md#owner-decisions-2026-11-17--during-s11c-6s-lane-r); both as recommended):
1. **`Activate`'s and `Drain`'s removed-app checks** (`internal/runtime/lifecycle.go:99`, `:190`) take G1 decision 3's reason, "defensive: guaranteed by the router's refusal of a removed app", with the router tests as their guard tests; the code is unchanged.
2. **The comparer leaves out only bytes per operation that are not stable:** allocations per operation always exact; bytes exact when each side's own runs agree, else each side's range printed and bytes left out of that benchmark's verdict.

**During the build — the owner, 2026-11-18** ([sub-lane R2b](decisions.md#owner-decisions-2026-11-18--during-s11c-6s-lane-r-sub-lane-r2b); [the slowdown review](decisions.md#owner-decisions-2026-11-18--a-refactors-large-slowdown-triggers-an-architecture-review)):
1. **The route loop's keep-going step gets a test** in one test-only commit after lane R's last split (its mutation only hangs, inside `Add`).
2. **`broadcast`'s new parameter is no cost, and `gate/bench/escapes.py` was fixed to match design §2.4 step 2** (≠ R; 432df34).
3. **Timing runs continue with the owner's applications open.**
4. **A new rule, [ground rule 13a](../ground-rules.md):** a refactor at least 5 % slower than the code before it (split or path, significant, re-run) stops; the function stays unsplit and an architecture review brings the owner a better design's options (≠ R on keeping the function unsplit meanwhile). Lane E's walkers (`cleanObject`, `CleanLen`) are reviewed now.

## How it runs

At most two agents at once; the plan check §1 holds the order: G0 → the design note, its blind reviews and the owner (G1) → W0 (the benchmark comparer; `astequal`'s rename map if a move is approved) → the coordinator's cross-package moves → the lanes, a queue of two (R and V; E and P; Q) → the coordinator's measurements after each lane → combine, T3, the review and break-it, one fix round, the nightly job, the path comparison, T4, the owner's check.

## Measured results

- **G0's smoke of the moved bound** (sprint model §10, the retro's rule; a one-row registry holding `aging`, at 6a38bdd's tree): T3 exit 0 with 18 countdown lines; T4 and nightly exit 1 with exactly 18 lines "the marker S1.1c-6 has reached its part S1.1c-6", the entries of S1.1c-5-1 and -2; no move-check fault for S1.1c-5-3's 13 entries and no bound fault.
- **The `proofs` row after the marker move** (`--tier T4 --job proofs`, at 6a38bdd, run 20261115T113755Z-c5a427, 329 s wall): **red as expected** ([owner](decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check), decision 4). Every driver green except `test_aging_planted.py`, 5 failures: its four red cases each fail with their own planted line (QA-AGING-1, -2, -3, -4, so the teeth hold) plus exactly the 18 "has reached its part S1.1c-6" lines; `test_the_real_tree_ages_green` fails on the same aging row. Not green; the first green `proofs` run follows the last lane's removal.

- **Decision 1 per hot split, lanes R and E so far** (the coordinator alone, `gate/bench/compare.py`: 10 interleaved pairs at `-test.cpu=1 -test.benchtime=1s`, a fixed-iteration allocation pass; one A/A per session, every A/A no cost; the owner's applications open, [owner, 2026-11-18](decisions.md#owner-decisions-2026-11-18--during-s11c-6s-lane-r-sub-lane-r2b), decision 3). **No cost:** runtime `ingest` (ddbcb27), `take` (6ada23e), `process` (30a1c45), `route` (ebf3760) on the seven `BenchmarkSendPath`/`BenchmarkSendPayloadPath` cases; event `compactAndValid` (53fb5f0), `appendUnescaped` (e65d6a0, 7-8 % faster). Allocations equal in every comparison. **Costs, both runs, so restored whole by a new commit with a PERMANENT waiver** (design §2.4 step 5): `cleanObject` and `CleanLen` (their `isSpace` split, restored in c54fe6c) and `routeKey` (its `keyPart` split, restored in 1cb4b7f); each restore measured against the commit before its split: no cost (`CleanLen` −2.0 %, p 0.21; `CleanObject` +0.1 %; `RouteKey/float-zero` +0.3 %, p 0.81). The samples the waivers cite:
  - `cleanObject`, `BenchmarkCleanObject`, run 1: median before 287.55 ns/op, after 325.35 ns/op, ratio 1.1315, p 1.083e-05, allocs/op 7/7: costs; before (ns/op) 287.6 282.9 285.1 287.5 295.6 284.7 299.8 286.5 289 303.6; after 319 323 330.8 334.7 321.4 325.5 332 325.2 324.8 334.4.
  - `cleanObject`, `BenchmarkCleanObject`, run 2: median before 199.1 ns/op, after 218.8 ns/op, ratio 1.0989, p 1.083e-05, allocs/op 7/7: costs; before (ns/op) 198 196.9 197.5 205 199 196.1 202.7 199.5 201.2 199.2; after 223 216.4 219.4 218.6 218.2 219.9 223.3 218.5 219 216.8.
  - `CleanLen`, `BenchmarkCleanLen`, run 1: median before 175.9 ns/op, after 218 ns/op, ratio 1.2393, p 1.083e-05, allocs/op 0/0: costs; before (ns/op) 171.5 168 179.5 176.1 175.7 171.8 181.2 180.7 168.5 179.1; after 210.2 208.5 218.4 211.9 217.6 219.2 224.2 210.5 222.2 220.5.
  - `CleanLen`, `BenchmarkCleanLen`, run 2: median before 115.75 ns/op, after 140.5 ns/op, ratio 1.2138, p 1.083e-05, allocs/op 0/0: costs; before (ns/op) 115.8 116.8 114.7 118.4 112.4 112 115.7 114.7 116.7 117.4; after 139.9 138.2 140.3 142.1 140.7 143.4 141.3 139.7 146.7 137.5.
  - `routeKey`, `BenchmarkRouteKey/float-zero`, run 1: median before 28.83 ns/op, after 29.41 ns/op, ratio 1.0201, p 1.083e-05, allocs/op 0/0: costs; before (ns/op) 28.95 28.82 28.77 29.2 28.84 28.84 28.88 28.78 28.76 28.79; after 29.45 29.57 29.28 29.39 29.38 29.43 29.32 29.6 29.49 29.35.
  - `routeKey`, `BenchmarkRouteKey/float-zero`, run 2: median before 18.765 ns/op, after 19.25 ns/op, ratio 1.0258, p 0.01469, allocs/op 0/0: costs; before (ns/op) 28.93 20.95 18.59 18.5 18.75 18.53 18.99 18.77 18.91 18.76; after 29.41 29.45 19.28 19.05 18.82 19.22 19.08 19.31 19.21 19.56.
- **The lanes' paths** (design §2.4 step 4, the owner's 5 % margin, [G1 decision 5](decisions.md#owner-decisions-2026-11-16--s11c-6s-design-g1)): runtime, nets 049b9f8 to 30a1c45: `BenchmarkSendPath/shards=1` +2.6 % (p 0.018) and `/shards=1/recorded` +3.9 % (p 0.019), accepted (≤ 5 %); nets to f9558c7, the next session: worst +1.3 %, accepted. Event, nets 55ab33d to e65d6a0: `CleanLen` +22.8 % and `CleanObject` +12.4 % (the split restored in c54fe6c), and `BenchmarkParseRFC3339` +2.4 % (p 0.0001) and `BenchmarkDecoderValue` +1.9 % with neither function's code changed: a pure move alone shifted them, so code layout moves a leaf benchmark by about 2 %.
- **The escape check on `broadcast(m message)`** (f9558c7): `escapes.py` exit 1 only for "leaking param: m", the new parameter carrying the two pointers (`adv`, `tables`) that leaked before into the same `rt.deliver` call; the callers' leak lines unchanged (`tickApp`, `drainApp`: content of `a`; `(*Sync).End`: content of `s`), 781 package heap lines before and after, the allocation pin 0. No cost under design §2.4 step 2; the tool is fixed to match the rule ([owner, 2026-11-18](decisions.md#owner-decisions-2026-11-18--during-s11c-6s-lane-r-sub-lane-r2b), decision 2).

## Test material

The nets are each lane's first item (the plan check §7): the coverage bar's characterisation tests and mutations, the allocation pins, the escape diff and the benchmarks for the 31 hot-path functions.

## Scope changes

The re-cut after the spike (2026-11-15, before G0): S1.1c-5-3's packages moved to S1.1c-7. After G0: the fix of `internal/parser/parse.go:363`'s panic on a whitespace-only notation added to lane Q (owner, 2026-11-16, G1 decision 7; one commit, red first; nothing moves out — the budget absorbs it). During lane V: the E-GRAPH-001 fix (one commit, red first) and the tests of the 12 statements no test watched (one test-only commit) added to lane V (owner, 2026-11-16, decisions 1 and 2 of that day's lane V section; the commit projection is recounted at combine).

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S1.1c-6 — the strict lint set: the size limits in runtime, validate, event, parser, plan and playback · routine: full · SP 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-15)
             ☑ dependencies merged, main green (S1.1c-5, PR #47; close-out #48; main 4d2b9eb — main CI green on both merges, the owner reported 2026-11-16) ☑ fits the cap, lanes ≤ 8 days (the spike and the re-cut)
             ☑ plan check reviewed (architect, QA) and answered by the owner (2026-11-15)
             ☑ test material: the nets are each lane's first item ☑ `gate/current-sprint` set, the first commit after the plan check's
Design (G1)  ☑ architecture + the-fool findings resolved (design §12) ☑ QA testability findings resolved (design §12)
             ☑ owner decisions recorded (2026-11-16) ☐ ADRs written for lasting decisions (none lasting beyond decisions.md so far; reviewed at G5)
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0
             ☐ conformance cases for every construct shipped or changed
             ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff: no drift
             ☐ QA break-it run ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 strict-gate / CI exit 0, run complete ☐ owner's check passed
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner
             ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line
             ☐ documents reconciled (plan + reasons only) ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded
             ☐ the row closed in gate/sprints.json as in the plan table (S1.1c-6 only; row S1.1c closes with S1.1c-7)
```

## Retro

*(written at close-out)*
