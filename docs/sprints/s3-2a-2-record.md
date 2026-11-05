# S3.2a-2 — sprint record (S3.1's routine)

**Sprint:** S3.2a-2 — the runtime core (S3.2a's item 3; the owner split S3.2a at its card's line) · **Routine:** by the owner (2026-11-05, all of S3.2a): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it · **Size:** about 7.5 SP (6 carded, plus the drain mechanism and one shared ingest step) · **Branch:** `s3-2a-2/integration`
**What to build and how each item is proven:** [sprint plan — S3.2a](sprint-plan.md#s32a--the-replay-format-and-the-runtime-core), its "how it runs" note and the forwards to S3.2a-2 · **Process:** [00-sprint-model.md](00-sprint-model.md)

## Plan and decisions

- **The plan check (owner, 2026-11-05):** two answers, both as recommended ([sprint-plan §5, "S3.2a-2's plan check"](sprint-plan.md#owner-decisions-2026-11-05--s32a-2s-plan-check)):
  - the drain barrier's mechanism is built here, and S3.3's `Stop` wraps it;
  - one single-threaded ingest step serves the router and the playback driver.

  The routine and the split are the owner's answers at S3.2a's plan check.
- **In this sprint:** item 3 — `internal/runtime`:
  - the ingest/router goroutine;
  - shards with bounded mailboxes, each key's work serial;
  - the non-blocking outbox handoff, with the router servicing every handoff while blocked;
  - sink queues and runtime-owned flushers, calling a sink through an internal function type;
  - the key-less pseudo-key;
  - panic recovery at the shard boundary;
  - an injectable idle-tick source;
  - the drain barrier;
  - the shared ingest step.
- **Exit 3:** progress with a full mailbox at a shard whose handoff is pending, plus a slow sink and a bursty source. The saturation test runs with test-double bodies under `-race` and a CPU hog, and must pass 50 of 50 — a recorded local run (plan-check reading 3); the gate's race row is S3.2b's.
- **Forwards from S3.2a-1** (under the S3.2a card):
  - the router reads the key attribute; a null, wrong-type or NaN key is a per-rule event error;
  - `Tables` comes from `table.Set.Snapshot`;
  - an event and its own advances travel in one message;
  - every message that can trigger a walk carries the router's snapshot, proven with 1 and with 4 shards;
  - a shard may walk per message or per chunk.

- **The design (G1, 2026-11-05):** [s3-2a-2-design.md](s3-2a-2-design.md) v2.
  - **v1's review** ([s3-2a-2-design-architecture.md](../reviews/s3-2a-2-design-architecture.md)) found 2 blockers, 15 should-fix and 5 later. The reviewer confirmed both blockers with scratch programs:
    - F1: a recovered panic cut a rule's walk short;
    - F2: stopping the ticker at drain raced.
  - **The owner's answers,** all as recommended ([sprint-plan §5, "S3.2a-2's design"](sprint-plan.md#owner-decisions-2026-11-05--s32a-2s-design-after-the-architecture-review)):
    - F14: table writes join the intake;
    - F5: a drain ends the app's input;
    - F10: a body's panic resets its key.
  - **Annotated:** ADR-0006 §5, ADR-0012 §3, §5 and §6, and SEMANTICS §4.
  - **QA's first four tests** (exit 3, D5, the ingest step, live against replay) are red on the stub; its gaps Q1–Q9 are placed in v2 §10.

## Scope changes

- **2026-11-05 (owner, plan check):**
  - the drain mechanism, from S3.3: +1;
  - the shared ingest step: +0.5.
- **2026-11-05 (owner, design):**
  - table writes through the intake: +0.5;
  - partition's panic reset: +0.5.
- **Total:** 6 → about 8.5 points, under the cap. Nothing moved out; S3.3 13 → 12.

## Done-done

```
Sprint S3.2a-2 — the runtime core · routine: S3.1's (QA first + architecture review + break-it) · SP ~8.5 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2a's, v3) ☑ dependencies merged, main green (S3.2a-1's merge, PR #10)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-05) ☑ test material: QA's tests, an item
Design (G1)  ☑ architecture review resolved (design v2 §10) ☑ owner decisions recorded (F5, F10, F14) ☑ ADRs and SEMANTICS annotated
Build (G2)   ☐ T2 (make check exit 0) ☐ T3 ☐ tests on the real code (QA's, red first)
Review (G3)  ☐ QA break-it ☐ every finding fixed, forwarded (owner consent for blockers) or rejected with reason
Verify (G4)  ☐ T4 exit 0 ☐ CI on the PR ☐ owner's check ☐ exit 3: the saturation run, 50/50 under -race and a CPU hog
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration
```
