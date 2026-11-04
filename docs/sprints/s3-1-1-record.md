# S3.1-1 — sprint record (S2.2b's routine, plus a clock break-it)

**Sprint:** S3.1-1 — the clock service and the metrics counters (S3.1's items 1–2; the owner split S3.1 at its card's line) · **Routine:** by the owner (2026-11-04): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it of the finished clock · **Size:** 6.5 SP of S3.1's 13 · **Branch:** `s3-1-1/integration`

## Plan and decisions

- The owner's plan-check answers: [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check).
- The design: [s3-1-1-design.md](s3-1-1-design.md) v2. v1's review ([s3-1-1-design-architecture.md](../reviews/s3-1-1-design-architecture.md): 8 blockers, 7 should-fix, 1 later) showed ADR-0004, the reference executor and SEMANTICS disagreeing on the clock's order, its owner and quiet streams; the owner chose the most accurate option each time, had the choices' architectural impact reviewed ([s3-1-1-decisions-architecture.md](../reviews/s3-1-1-decisions-architecture.md)), and settled what that raised: decisions D1–D8, [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-04--s31-1s-clock-after-two-architecture-reviews).
- The oracle changed first: the reference executor walks forward in event time; six new harness checks, red first; 47 pass.
- QA's tests were written first against v1, then revised by QA to v2 (26 tests) before the implementation was moved in; the implementation was written apart from them.

## Done-done

```
Sprint S3.1-1 — the clock service · routine: lighter (QA first + architecture review + break-it) · SP ~8.5 (6.5 + D1–D8) · cap 2 weeks
Entry (G0)   ☑ card complete and approved ☑ dependencies merged, main green (S2.2b's merge) ☑ fits the cap, no lanes
             ☑ plan check answered by the owner (2026-11-04) ☑ test material exists (QA's tests, first; revised to v2)
Design (G1)  ☑ architecture findings resolved (design v2 §8; two reviews) ☑ owner decisions recorded (D1–D8, sprint-plan §5)
             ☑ ADR-0004/0006/0012/0013 and SEMANTICS annotated ☑ the oracle changed first (47 checks, 6 new, red first)
Build (G2)   ☑ T2 (make check exit 0) ☐ T3 ☑ tests on the real code (QA's 26 pass at the first run)
Review (G3)  ☐ QA break-it of the clock ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ T4 / CI exit 0 ☐ owner's check passed ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
```

## Measured results

*(at G4)*

## Retro

*(at close-out)*
