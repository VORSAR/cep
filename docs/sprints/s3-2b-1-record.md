# S3.2b-1 — sprint record (S3.1's routine)

**Sprint:** S3.2b-1 — the plan, the replay loop through the runtime, rate limits and the material (S3.2b's items 1, 2, 6, 9; the owner split S3.2b at its card's line) · **Routine:** by the owner (2026-11-06): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it · **Size:** about 7 SP (6.5 carded, plus the on-arrival path for plain rules) · **Branch:** `s3-2b-1/integration`
**What to build and how each item is proven:** [sprint plan — S3.2b](sprint-plan.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time), its "how it runs" note and the plan check's readings · **Process:** [00-sprint-model.md](00-sprint-model.md)

## Plan and decisions

- **The plan check (owner, 2026-11-06):** seven answers, all as recommended ([sprint-plan §5, "S3.2b's plan check"](sprint-plan.md#owner-decisions-2026-11-06--s32bs-plan-check)). Those that bind this sprint:
  - **Q1:** a plain filter rule's alert goes out as soon as its stream accepts the event; a rate limit compares event times in arrival order;
  - **Q2:** S3.1's routine, and the split along the card's line.

  Q3–Q7 bind S3.2b-2: `text_invisibles` (E-TYPE-001 for a wrong declaration), the allocation budget of the live hand-off, macOS as the second OS, goleak v1.3.0, the tripwire's ceilings.
- **In this sprint:**
  - **Item 1 — `internal/plan` (minimal):** the stream-node graph from the checked `*model.App`, the routing table, filter-rule operator chains (filter, projection, emission). `plan` owns its description types; the runtime builds from them (plan-check reading 1).
  - **Item 2 — the replay drive loop is the runtime with one shard and a synchronous step** (ADR-0005 §3): the runtime's router and shard steps on one thread, no goroutines, fed by the playback driver; S3.1's and S3.2a's exit lines re-run through it.
  - **Item 6 — the emission stage with rate limiting:** `output first every D`, the anchor per key in `partition.Instance` (one global anchor for a key-less rule), suppressed outputs counted; the three §8 harness checks ported as Go tests naming their source.
  - **Item 9 — test material for examples 07 and 09** (QA, before the engine runs): at least one fire-path and one near-miss expect block each; 09 gains a second event inside its rate-limit interval. They run green in S3.3.
  - **Owner Q1 — the on-arrival path for plain rules**, with SEMANTICS §8 and SPEC §5.1 annotated at G1.
- **Exits:**
  1. The same log run live (N shards) and in replay (one shard) gives identical per-key output order — the live/replay equivalence test, with randomized comparisons (S3.2a-2's forward).
  5. A second output inside the rate-limit interval is suppressed and counted; a key-less rule uses one global anchor — the three §8 ports.

  Exits 2, 3, 4, 6, 7 and 8 are S3.2b-2's.
- **The design (G1, 2026-11-06):** [s3-2b-1-design.md](s3-2b-1-design.md) v2.
  - **v1's review** ([s3-2b-1-design-architecture.md](../reviews/s3-2b-1-design-architecture.md)) found 2 blockers, 7 should-fix and 3 later, each proven with a scratch program (the coordinator re-ran F2 and F4 on a fresh copy: 2 alerts where the rule gives 1; 1–2 allocations per routed event):
    - F1: a plain rule using an expression that cannot run yet (text operators, `count`, `chain`) was built and never fired;
    - F2: purging an on-arrival key on the rule clock re-opened a suppression in multi-stream replays and live after idleness.
  - **QA's tests first** (34, red on the stub; every existing test green; `go vet` clean) raised gaps G1–G7.
  - **The owner's decisions** ([sprint-plan §5, "S3.2b-1's design"](sprint-plan.md#owner-decisions-2026-11-06--s32b-1s-design-after-the-architecture-review)): test imports for `plan` and `playback`; D1 (purge on the stream's admission); D2 (07 per tenant); D3 (a derived `ts` must be a timestamp, E-TYPE-001); D4 (≠ R: a nullable select into a non-optional attribute is a deploy error, E-NULL-003); D5 (both here).
  - **The rate anchor** stays in the body's per-key state, as ADR-0006 §2 records (review F5); design v1 had moved it.
  - **Annotated:** SEMANTICS §8 and §9, SPEC §5.1, ARCHITECTURE §5, ADR-0005 §3, ADR-0006 §2.
- **Forwards this sprint carries:** the routing-table type `internal/runtime` defines, which the planner now produces (S3.2a's plan-check reading 2); `runtime.Rule`, written "as S3.2b's planner will describe it".

## Scope changes

- **2026-11-06 (owner, plan check):** the on-arrival path for plain rules (Q1), +0.5. S3.2b-1 6.5 → about 7 points; nothing moved out.
- **2026-11-06 (owner, design):** D3 and D4 in the validator (+2, D5), D1's admission purge (+0.5), the review's other fixes (about +0.5). About 7 → about 10 points, under the cap; nothing moved out.

## Measured results

*(filled as the sprint runs)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S3.2b-1 — the plan, the replay loop, rate limits, the material · routine: S3.1's (QA first + architecture review + break-it) · SP ~7 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2b's, v3) ☑ dependencies merged, main green (S3.2a-2's PR #11; PRs #12, #13; nightly run by hand green)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-06) ☑ test material: QA's tests and item 9, items
Design (G1)  ☐ architecture review resolved ☐ QA testability ☐ owner decisions recorded ☐ ADRs, SEMANTICS and SPEC annotated
Build (G2)   ☐ T2 (make check exit 0) ☐ T3 ☐ tests on the real code (QA's, red first)
Review (G3)  ☐ architecture review of the combined diff ☐ QA break-it ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ T4 exit 0 ☐ owner's check ☐ exits 1 and 5 met by their proofs
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line
             ☐ documents reconciled ☐ memory updated ☐ worktrees removed ☐ points and duration
```
