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

## How it runs

At most two agents at once; the plan check §1 holds the order: G0 → the design note, its blind reviews and the owner (G1) → W0 (the benchmark comparer; `astequal`'s rename map if a move is approved) → the coordinator's cross-package moves → the lanes, a queue of two (R and V; E and P; Q) → the coordinator's measurements after each lane → combine, T3, the review and break-it, one fix round, the nightly job, the path comparison, T4, the owner's check.

## Measured results

- **G0's smoke of the moved bound** (sprint model §10, the retro's rule; a one-row registry holding `aging`, at 6a38bdd's tree): T3 exit 0 with 18 countdown lines; T4 and nightly exit 1 with exactly 18 lines "the marker S1.1c-6 has reached its part S1.1c-6", the entries of S1.1c-5-1 and -2; no move-check fault for S1.1c-5-3's 13 entries and no bound fault.
- **The `proofs` row after the marker move** (`--tier T4 --job proofs`, at 6a38bdd, run 20261115T113755Z-c5a427, 329 s wall): **red as expected** ([owner](decisions.md#owner-decisions-2026-11-15--s11c-6s-plan-check), decision 4). Every driver green except `test_aging_planted.py`, 5 failures: its four red cases each fail with their own planted line (QA-AGING-1, -2, -3, -4, so the teeth hold) plus exactly the 18 "has reached its part S1.1c-6" lines; `test_the_real_tree_ages_green` fails on the same aging row. Not green; the first green `proofs` run follows the last lane's removal.

## Test material

The nets are each lane's first item (the plan check §7): the coverage bar's characterisation tests and mutations, the allocation pins, the escape diff and the benchmarks for the 31 hot-path functions.

## Scope changes

The re-cut after the spike (2026-11-15, before G0): S1.1c-5-3's packages moved to S1.1c-7. After G0: the fix of `internal/parser/parse.go:363`'s panic on a whitespace-only notation added to lane Q (owner, 2026-11-16, G1 decision 7; one commit, red first; nothing moves out — the budget absorbs it).

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S1.1c-6 — the strict lint set: the size limits in runtime, validate, event, parser, plan and playback · routine: full · SP 13 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-15)
             ☑ dependencies merged, main green (S1.1c-5, PR #47; close-out #48; main 4d2b9eb — main CI: the owner reports) ☑ fits the cap, lanes ≤ 8 days (the spike and the re-cut)
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
