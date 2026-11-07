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

*(filled as the work lands)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S3.3-1 — the public Go API · routine: the full loop with the design panel (owner) · SP about 8 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-07) ☑ dependencies merged, main green (S3.2c's PRs #21, #22; main a8a63ad)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner ☑ test material: QA's tests first
Design (G1)  ☑ the design note (v1 ce272a4; v2 build-ready) ☑ the panel's three reports ☑ QA's testability review ☑ owner decisions recorded
Build (G2)   ☐ T2 ☐ T3 ☐ tests on the real code
Review (G3)  ☐ architecture review of the combined build ☐ QA break-it ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ the fix round under the race detector ☐ T4 exit 0 ☐ owner's check ☐ exits 3, 4, 5 met by their proofs
Exit (G5)    ☐ one PR, merged by the owner ☐ main CI green ☐ record + retro ☐ known issues updated
             ☐ status line ☐ documents reconciled ☐ memory updated ☐ worktrees removed ☐ points and duration
```

## For the owner (the owner's check)

*(written from the sprint's diff before the PR)*

## Retro

*(at close-out)*
