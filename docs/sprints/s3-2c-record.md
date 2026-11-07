# S3.2c — sprint record (tests first, one review, a break-it)

**Sprint:** S3.2c — the spec cleanup: the language documents agree, and overflow is an error · **Routine:** by the owner (2026-11-06): QA's tests first; a one-page design note and the build in the main session; one architecture review of the design and the code together; a short QA break-it on overflow; one fix round, under the race detector before T4 · **Size:** 3 SP (items 2.5) · **Branch:** `s3-2c/integration`
**What to build and how each item is proven:** [sprint plan — S3.2c](sprint-plan.md#s32c--the-spec-cleanup-the-language-documents-agree-and-overflow-is-an-error), its "how it runs" note and the plan check's readings · **Process:** [00-sprint-model.md](00-sprint-model.md) · **Before:** [S3.2b-2](s3-2b-2-record.md)

## Plan and decisions

- **Placed by the owner** before S3.3 (2026-11-06, [decisions](decisions.md#owner-decisions-2026-11-06--the-open-items-after-s32b-2)), with integer overflow decided: an expression error for that rule, counted, as division by zero is.
- **The plan check (owner, 2026-11-06):** three answers, all as recommended ([decisions](decisions.md#owner-decisions-2026-11-06--s32cs-plan-check)): the card as written; QA's tests first, one review, a break-it; the overflow rule over every 64-bit operation — ints, durations, timestamps.
- **In this sprint:** item 1, the five contradictions (known issue S2.2b-1); item 2, the evaluation's two notes (E-IDLE-001's wording; every reserved word in SPEC Appendix B, test-pinned); item 3, overflow as an expression error, SPEC rev 6.
- **Exits:** 1, the contradictions resolved with no verdict changed; 2, Appendix B equal to `grammar.lark`'s list; 3, every 64-bit overflow an expression error for its rule only; 4, no in-range result changed.

## Scope changes

None.

## Measured results

*(filled as the work lands)*

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|

## Done-done

```
Sprint S3.2c — the spec cleanup · routine: tests first, one review, a break-it (owner) · SP 3 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-06) ☑ dependencies merged, main green (S3.2b's PRs #14, #19; main 53facc5, the gate green)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner ☑ test material: QA's tests first, an item
Design (G1)  ☐ the design note ☐ owner decisions recorded
Build (G2)   ☐ T2 ☐ T3 ☐ tests on the real code
Review (G3)  ☐ one architecture review (design and code) ☐ QA break-it on overflow ☐ every finding fixed, forwarded or rejected
Verify (G4)  ☐ the fix round under the race detector ☐ T4 exit 0 ☐ owner's check ☐ exits 1–4 met by their proofs
Exit (G5)    ☐ one PR, merged by the owner ☐ main CI green ☐ record + retro ☐ known issues S2.2b-1, S3.2b-1-1 closed
             ☐ status line ☐ documents reconciled ☐ memory updated ☐ worktrees removed ☐ points and duration
```

## For the owner (the owner's check)

*(written from the sprint's diff before the PR)*

## Retro

*(at close-out)*
