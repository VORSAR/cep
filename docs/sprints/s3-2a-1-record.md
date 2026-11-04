# S3.2a-1 — sprint record (S3.1's routine)

**Sprint:** S3.2a-1 — the replay format layer and partitions (S3.2a's items 1–2; the owner split S3.2a at its card's line) · **Routine:** by the owner (2026-11-05): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it · **Size:** 5.5 SP of S3.2a's 11.5, before key evaluation moves to S3.2a-2's router · **Branch:** `s3-2a-1/integration`
**What to build and how each item is proven:** [sprint plan — S3.2a](sprint-plan.md#s32a--the-replay-format-and-the-runtime-core), its "how it runs" note and the plan check's readings · **Process:** [00-sprint-model.md](00-sprint-model.md)

## Plan and decisions

- **The plan check (owner, 2026-11-05):** four answers, all as recommended ([sprint-plan §5, "S3.2a's plan check"](sprint-plan.md#owner-decisions-2026-11-05--s32as-plan-check)):
  - S3.1's routine;
  - S3.2a split along its card's line;
  - S1.1c placed after S3.3;
  - a re-forecast now.
- **In this sprint:**
  - **Item 1 — `internal/playback`'s format layer:** the log format, the loader, the virtual-clock `Driver`, the canonical output encoder and the canonical trace.
  - **Item 2 — `internal/partition`:** `Instance`, `OpState`, the TTL purge timer and its re-arm, `LastActivity`, the state registry and the timer-derivability invariant. Key evaluation is not here; it moves to S3.2a-2's router (plan-check reading 1).
- **Exits:**
  1. A byte-identical canonical trace, red on a planted map-iteration order.
  2. A table record takes effect from the next event onward.
  3. *(Exit 3, the saturation test, is S3.2a-2's.)*
  4. One TTL timer per instance, re-armed on activity.
- **Binding forwards from S3.1-2:**
  - **The snapshot carrier (D1):** its replay half, the loader's snapshot at each log position.
  - **The timer-evaluation rule (F15).**
  - **The encoder's output for malformed UTF-8 in an object value (F11).**

  The log format's open points go to the owner as one list after the design review.

## Done-done

```
Sprint S3.2a-1 — the replay format layer and partitions · routine: S3.1's (QA first + architecture review + break-it) · SP 5.5 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2a's, v3) ☑ dependencies merged, main green (S3.1-2's merge, PR #9)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-05) ☑ test material: QA's tests, an item
Design (G1)  ☐ architecture review resolved ☐ owner decisions recorded ☐ ADRs annotated
Build (G2)   ☐ T2 (make check exit 0) ☐ T3 ☐ tests on the real code (QA's, red first)
Review (G3)  ☐ QA break-it ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 exit 0 ☐ CI on the PR ☐ owner's check
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration
```
