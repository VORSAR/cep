# S3.1-1 — sprint record (S2.2b's routine, plus a clock break-it)

**Sprint:** S3.1-1 — the clock service and the metrics counters (S3.1's items 1–2; the owner split S3.1 at its card's line) · **Routine:** by the owner (2026-11-04): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it of the finished clock · **Size:** 6.5 SP of S3.1's 13 · **Branch:** `s3-1-1/integration`

## Plan and decisions

- The owner's plan-check answers: [sprint-plan §5](sprint-plan.md#owner-decisions-2026-11-04--s22b-closed-s31s-plan-check).
- The design: [s3-1-1-design.md](s3-1-1-design.md), with its open questions A1–A4. The API stub lands before G1, so QA's tests compile and run red.

## Done-done

```
Sprint S3.1-1 — the clock service · routine: lighter (QA first + architecture review + break-it) · SP 6.5 · cap 2 weeks
Entry (G0)   ☑ card complete and approved ☑ dependencies merged, main green (S2.2b's merge) ☑ fits the cap, no lanes
             ☑ plan check answered by the owner (2026-11-04) ☐ test material exists, or making it is an item (QA, first)
Design (G1)  ☐ architecture findings resolved ☐ owner decisions recorded (A1–A3) ☐ ADR-0004 annotated
Build (G2)   ☐ T2 ☐ T3 ☐ tests on the real code
Review (G3)  ☐ QA break-it of the clock ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ T4 / CI exit 0 ☐ owner's check passed ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
```

## Measured results

*(at G4)*

## Retro

*(at close-out)*
