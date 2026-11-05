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
- **The design (G1, 2026-11-05):** [s3-2a-1-design.md](s3-2a-1-design.md) v2.
  - **v1's review** ([s3-2a-1-design-architecture.md](../reviews/s3-2a-1-design-architecture.md)) found 1 blocker, 8 should-fix and 4 later. Its blocker, F1: walking a shard's clock once per batch of messages changes results, against ADR-0010 §2.
  - **QA's partition tests** (27, on `s3-2a-1/qa-partition`, red on the stub) raised gaps A1–A9.
  - **The owner's decisions:** D1–D7 ([sprint-plan §5, "S3.2a-1's design"](sprint-plan.md#owner-decisions-2026-11-05--s32a-1s-design-after-the-architecture-review)). On F1 and on punctuation records, the owner asked first which option is algorithmically correct, then chose it:
    - **D1:** an instant is finalized only once the clock is past it, and S3.1-1's seal is retired. This fixes an older defect in the clock and the oracle; measured on scratch copies, it changes 3 of the oracle's 47 checks and 20 of the clock's 43 tests.
    - **D2:** `ts` only when a stream has no time attribute.
    - **D3:** recording records (start, arrival, tick, stop).
    - **D4:** objects cleaned at decode.
    - **D5:** a timer reads the triggering arrival's tables.
    - **D6 (≠ R):** replay streams and stops at the first fault, a payload fault included.
    - **D7:** an object partition key is a deploy error, in S3.3.
  - **Annotated:** ADR-0004, ADR-0005, ADR-0006, ADR-0010, ADR-0012 and SEMANTICS §6.4's ordering paragraph. Forwards were written onto the S3.2a-2, S3.3, S4.1 and S8 cards.

## Scope changes

- **2026-11-05 (owner, D1, D3, D4):** about +3.5 points:
  - D1: the clock's finalization rule, the oracle's drain and the revised tests, about 2;
  - D3: the recording records, about 1;
  - D4: the decoder's object cleaning, about 0.5.

  From 5.5 to about 9 points, under the cap. Nothing moved out.

## Done-done

```
Sprint S3.2a-1 — the replay format layer and partitions · routine: S3.1's (QA first + architecture review + break-it) · SP 5.5 → ~9 (D1, D3, D4) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2a's, v3) ☑ dependencies merged, main green (S3.1-2's merge, PR #9)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-05) ☑ test material: QA's tests, an item
Design (G1)  ☑ architecture review resolved (design v2 §9) ☑ owner decisions recorded (D1–D7) ☑ ADRs and SEMANTICS annotated
Build (G2)   ☐ T2 (make check exit 0) ☐ T3 ☐ tests on the real code (QA's, red first)
Review (G3)  ☐ QA break-it ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 exit 0 ☐ CI on the PR ☐ owner's check
             ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration
```
