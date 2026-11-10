# S3.3-2 — sprint record (the full loop, with the design panel)

**Sprint:** S3.3-2 — the expect sandbox, the conformance runner, the Tier-2 baselines and the clock freeze · **Routine:** the full loop with the design panel (owner, 2026-11-08) · **Size:** about 7.5 SP, re-sized to about 9.75 after the panel (owner, 2026-11-09) · **Branch:** `s3-3-2/integration`
**What to build and how each item is proven:** [sprint plan — S3.3](sprint-plan.md#s33--the-deploy-pipeline-and-the-conformance-runner--clock-freeze) (items 3, 4, 6, 7, the forwards, exits 8 and 9), its "how S3.3-2 runs" note and the plan check's readings · **Design:** [s3-3-2-design.md](s3-3-2-design.md) (v2, build-ready; v1 at 0a3114c) · **Reviews:** [design: breaker](../reviews/s3-3-2-design-breaker.md), [design: challenger](../reviews/s3-3-2-design-challenger.md), [design: architecture](../reviews/s3-3-2-design-architecture.md), [design: QA (testability)](../reviews/s3-3-2-design-qa.md) · **Process:** [00-sprint-model.md](00-sprint-model.md) · **Before:** [S3.3-1](s3-3-1-record.md)

## Plan and decisions

- **The plan check (owner, 2026-11-08):** three answers, one as recommended ([decisions](decisions.md#owner-decisions-2026-11-08--s33-2s-plan-check)):
  - **1 (≠ R):** an expect block runs with the deployment's param values; a different result refuses the deploy (E-EXP-001).
  - **2:** a mistake inside a block (an unknown stream, table or field, a wrongly typed value) is its own static code, found before any block runs: one new entry in `errors.md`, a line in SPEC's change log, no grammar change.
  - **3 (≠ R):** the full loop with the design panel.
- **The design (owner, 2026-11-09, after the panel):** eight answers, seven as recommended ([decisions](decisions.md#owner-decisions-2026-11-09--s33-2s-design-after-the-panel)): "not built yet" is `errors.ErrUnsupported`; E-EXP-002 refuses every kind the panel found; E-EXP-001 points at the expected record that did not come out, or at `then`; the baselines are keyed by content; after a SPEC revision only named blocks' values are recorded; the not-built expiry is S1.1c's; **`optional` on a time attribute is refused now (≠ R: E-TIME-003, SPEC rev 10)**; one sprint, about 9.75 points. The panel: [breaker](../reviews/s3-3-2-design-breaker.md) (2 blockers, 7 should-fix, 3 later), [challenger](../reviews/s3-3-2-design-challenger.md) (0, 5, 1), [architect with the-fool](../reviews/s3-3-2-design-architecture.md) (1, 8, 3; its blocker the breaker's B2); every finding verified by the coordinator against the source and mapped to [design v2](s3-3-2-design.md) §14. v2 replaces v1's `Close` barrier with an in-flight gate (§9.1), new design the combined review checks.
- **A departure from the routine (owner's direction):** the QA testability agent was stopped by the owner before it wrote its report, and the owner directed the coordinator to do that review ("you do it"); the coordinator wrote the design, so this departs from ground rule 14. The [testability review](../reviews/s3-3-2-design-qa.md) is the coordinator's; QA lane 2, writing its tests red, is the independent check.
- **Binding from earlier decisions:** an expect block's lists interleave by time, each in its written order, ties to the list written first ([S3.3's plan check](decisions.md#owner-decisions-2026-11-07--s33s-plan-check), 4); a block passes only when the outputs match the then-list in number as well as content (S3.2b-1's M2); the sandbox never opens connectors and ignores the annotations ([S3.3-1's design](decisions.md#owner-decisions-2026-11-07--s33-1s-design-after-the-panel), 5); between S3.3-1 and this sprint `Deploy` runs no blocks.
- **Known issues in scope:** S3.3-1-1 (`apipartition` names a method's entry `<Type>.<Method>`; the law file is rewritten), S3.3-1-2 (`Runtime.Close` refuses new `Add`s before it takes its snapshot).
- **Exits (from the card):** 1, 07 and 09 (and 10) pass their blocks through `Deploy`; 2, the six planted wrong expects fail `Deploy` with E-EXP-001 naming the first divergence; 6, `gate/regen-baseline` refuses timestamp, ordering and count changes and records an additive one; 7, the clock freeze (the `clockgolden` proof until S1.1c's LAW Δ block); 8 and 9, added at the plan check (the new expect-block code at its position; an override that changes a block's result refuses the deploy).

## Scope changes

- **Added at the design (owner, 2026-11-09, 7, ≠ R):** E-TIME-003's second reason — a stream's time attribute declared `optional` is refused (a SPEC change-log line, rev 10), about +0.25.
- **Re-sized (owner, 2026-11-09, 8):** about 7.5 → about 9.75 points, the panel's fixes included; one sprint.
- **Forwarded:** the conformance runner's not-built expiry → S1.1c (owner, 6); the breaker's L2 (a block with no given event passes vacuously) → known issues at close-out (later).

## Measured results

*(filled as the work lands)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S3.3-2 — the expect sandbox, the runner, the baselines, the clock freeze · routine: the full loop with the design panel (owner) · SP about 7.5 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-07; S3.3-2's plan check 2026-11-08) ☑ dependencies merged, main green (S3.3-1's PRs #23, #24; main 1fa3a5b)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner ☑ test material: QA's tests first
Design (G1)  ☑ the design note (v2) ☑ the panel's three reports ☑ QA's testability review (the coordinator's, owner's direction) ☑ owner decisions recorded
Build (G2)   ☐ T2 ☐ T3 ☐ tests on the real code
Review (G3)  ☐ architecture review of the combined build ☐ QA break-it ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ the fix round under the race detector ☐ T4 exit 0 ☐ owner's check ☐ exits 1, 2, 6, 7, 8, 9 met by their proofs
Exit (G5)    ☐ one PR, merged by the owner ☐ main CI green ☐ record + retro ☐ known issues S3.3-1-1, S3.3-1-2 closed
             ☐ the clock freeze declared ☐ status line ☐ documents reconciled ☐ memory updated ☐ worktrees removed ☐ points and duration
```

## For the owner (the owner's check)

*(written from the sprint's diff before the PR; proposed: by running, the conformance runner over the corpus, then one expected value changed in 09's block and `Deploy` refusing with E-EXP-001 naming it; by reading, the baseline directory's table of diff classes)*

## Retro

*(at close-out)*
