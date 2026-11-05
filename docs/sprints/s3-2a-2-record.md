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

## Measured results

- **Exit 3, the saturation run** (recorded local run; S3.2a's plan-check reading 3): `go test -race -count=50 ./internal/runtime`, with one busy-loop CPU hog per core (12) running throughout.
  - `TestSaturationProgress`: **50/50**.
  - Every runtime test: 600 of 600 passes, 0 failures, 0 data races.
  - The saturation test asserts `SaturatedParks ≥ 1`, so the "full mailbox while its handoff is pending" moment was reached in each run.
- **Tests first** (QA, apart from the build): 17 runtime, partition and playback tests, red on the stub or the old code before the build.
  - Two of them assumed `Send` waits for the router. It returns once queued, so they now wait on observable states (the first round's outputs at the sink; a `Stats` barrier) before changing their setup.
  - QA's gap Q15, a real defect, was fixed test-first: a `Drain` whose context had already expired left the app refusing input without draining it.
- **The replay driver on the shared ingest step:** every S3.2a-1 playback test passes unchanged.

- **Coverage** (the gate's row first failed at 88.46%; QA added coverage tests): `internal/runtime` 96.9%, `internal/partition` 96.4%, `internal/playback` 94.9%.
  - **Reached by no test:** `Offer`'s last-resort recover. `Start` and `Send` enforce the key type, and the router drops null keys, so `partition.Offer` has nothing left to panic on.
- **After the fix round:** `go test -race -count=50 ./internal/runtime` with 14 CPU hogs gave 1,700 passes, 0 failures, 0 data races; saturation 50/50.
- **The gate:**
  - T2 `make check`: exit 0 after the coverage tests.
  - T4 `make strict-gate` on the final tree: every row PASS, the grammar differential (63) and `proofs` (14) included; exit 0; wall 88.1 s.

## The break-it (G3)

A fresh QA agent ([s3-2a-2-breakit-qa.md](../reviews/s3-2a-2-breakit-qa.md)) found 1 blocker, 1 should-fix and 3 later. QA's coverage work found one more, the same defect as R1. Each has a red test, and all were handled in one batched round: every fix's test was red in one run, then every fix, then one verification.

| id | severity | finding | disposition |
|---|---|---|---|
| B1 | blocker | a body's emit on a stream outside its app panicked the router and killed the process | fixed: the emit checks its stream; a bad one is a body fault, so partition resets the key and counts it |
| R1 (and QA's) | should-fix (a broken contract) | a failed second Drain cleared the drained app's flag, so `Send` returned nil, not `ErrDrained` | fixed: only the Drain that set the flag clears it |
| C1 | later | a sink could still be called after `Close` began (a random `select`) | fixed: every loop checks the stop first |
| V1 | later | `Send` accepted a null in a required attribute | fixed: refused, along with NaN and ±Inf (S3.1-2's D2) |
| V2 | later | a rule listing one input twice saw every event twice | fixed: refused at `Start` |
| question | — | a retried Drain after a timeout | owner: it waits for the same flush |
| gaps | — | a schema whose name differs from its stream's; a `NewBody` that panics in `Start` | fixed, with tests |

**Two older tests changed with the decisions:** QA's coverage test now expects a second Drain to return the first drain's result, and the key test expects a NaN key to be refused by `Send`.

**What held:**
- tiny capacities with 7 shards and 4 apps under `-race` and CPU hogs, every drain flushing every output in per-key order;
- a `Send` racing a `Drain`, never lost;
- `Close` during traffic and during a drain, with no leaks;
- every key type across shard counts;
- a storm of body and sink panics.

**Not tried, forwarded:**
- a sink calling back into the runtime (a documented MUST NOT);
- randomized live-against-replay comparisons, which go to S3.2b's live/replay equivalence exit.

## Done-done

```
Sprint S3.2a-2 — the runtime core · routine: S3.1's (QA first + architecture review + break-it) · SP ~8.5 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2a's, v3) ☑ dependencies merged, main green (S3.2a-1's merge, PR #10)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-05) ☑ test material: QA's tests, an item
Design (G1)  ☑ architecture review resolved (design v2 §10) ☑ owner decisions recorded (F5, F10, F14) ☑ ADRs and SEMANTICS annotated
Build (G2)   ☑ T2 (make check exit 0) ☑ T3 (in T4) ☑ tests on the real code (QA's, red first)
Review (G3)  ☑ QA break-it (1 blocker, 1 should-fix, 3 later) ☑ every finding fixed, each with its red test
Verify (G4)  ☑ T4 exit 0 (final tree) ☐ CI on the PR ☐ owner's check ☑ exit 3: the saturation run, 50/50 under -race and a CPU hog
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration
```

## For the owner (the owner's check)

- **By reading: exit 3's scenario.** `internal/runtime/saturation_test.go`, a held body, a slow sink and four bursty senders, and its pass rate: 50/50 under -race with a CPU hog per core, twice (before and after the fix round).
- **By reading: the drain.** `TestDrainBarrier` and `TestRetriedDrainWaitsForTheSameFlush`: what `Drain` guarantees when it returns.
- **Behaviour older code now sees:**
  - the replay driver runs on the shared ingest step, with S3.2a-1's tests unchanged;
  - a rule body's panic resets its key.
- **No new dependency.**
