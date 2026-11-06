# S3.2b-1 — sprint record (S3.1's routine)

**Sprint:** S3.2b-1 — the plan, the replay loop through the runtime, rate limits and the material (S3.2b's items 1, 2, 6, 9; the owner split S3.2b at its card's line) · **Routine:** by the owner (2026-11-06): QA's tests first and independent; an architecture review of the design note; the build in the main session; a QA break-it · **Size:** about 10.5 SP (6.5 carded; the owner's additions below) · **Branch:** `s3-2b-1/integration`
**What to build and how each item is proven:** [sprint plan — S3.2b](sprint-plan.md#s32b--one-runtime-live-and-replay-rate-limits-errors-arrival-time), its "how it runs" note and the plan check's readings · **Process:** [00-sprint-model.md](00-sprint-model.md)

## Plan and decisions

- **The plan check (owner, 2026-11-06):** seven answers, all as recommended ([sprint-plan §5, "S3.2b's plan check"](sprint-plan.md#owner-decisions-2026-11-06--s32bs-plan-check)). Those that bind this sprint:
  - **Q1:** a plain filter rule's alert goes out as soon as its stream accepts the event; a rate limit compares event times in arrival order;
  - **Q2:** S3.1's routine, and the split along the card's line.

  Q3–Q7 bind S3.2b-2: `text_invisibles` (E-TYPE-001 for a wrong declaration), the allocation budget of the live hand-off, macOS as the second OS, goleak v1.3.0, the tripwire's ceilings.
- **In this sprint:**
  - **Item 1 — `internal/plan` (minimal):** the stream-node graph from the checked `*model.App`, the routing table, filter-rule operator chains (filter, projection, emission). `plan` owns its description types; the runtime builds from them (plan-check reading 1).
  - **Item 2 — the replay drive loop is the runtime with one shard and a synchronous step** (ADR-0005 §3): the runtime's router and shard steps on one thread, no goroutines, fed by the playback driver; S3.1's and S3.2a's exit lines re-run through it.
  - **Item 6 — the emission stage with rate limiting:** `output first every D`, the anchor per key in the body's per-key state, as ADR-0006 §2 records *(annotated 2026-11-06, combined review C3: the card's "in `partition.Instance`" was superseded at the design)* (one global anchor for a key-less rule), suppressed outputs counted; the three §8 harness checks ported as Go tests naming their source.
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
- **The review and break-it (G3, 2026-11-06):** the combined-build review ([s3-2b-1-combined-architecture.md](../reviews/s3-2b-1-combined-architecture.md)) found 0 blockers, 3 should-fix and 6 later; the QA break-it ([s3-2b-1-breakit-qa.md](../reviews/s3-2b-1-breakit-qa.md)) found 1 blocker and 2 later. The owner's decisions ([sprint-plan §5, "S3.2b-1's review and break-it"](sprint-plan.md#owner-decisions-2026-11-06--s32b-1s-review-and-break-it)):
  - **multi-part partition keys are supported** (≠ R), written `(k, j) of S` (as R), with no limit on the parts (≠ R); S3.2b-1 refuses a stream keyed twice with E-PART-004 meanwhile (as R); "don't drop anything";
  - **D3's error points at the `ts` attribute** (≠ R), once model attributes carry a position;
  - **the work splits into parts of about 10 points each:** both additions are S3.2b-2's.
- **Forwards this sprint carries:** the routing-table type `internal/runtime` defines, which the planner now produces (S3.2a's plan-check reading 2); `runtime.Rule`, written "as S3.2b's planner will describe it".

## Scope changes

- **2026-11-06 (owner, plan check):** the on-arrival path for plain rules (Q1), +0.5. S3.2b-1 6.5 → about 7 points; nothing moved out.
- **2026-11-06 (owner, design):** D3 and D4 in the validator (+2, D5), D1's admission purge (+0.5), the review's other fixes (about +0.5). About 7 → about 10 points, under the cap; nothing moved out.
- **2026-11-06 (owner, review and break-it):** E-PART-004 (+0.5) here. Multi-part keys (about 2.5) and D3's position at the attribute (about 1) go to S3.2b-2 by the owner's split into parts of about 10 points each (S3.2b-1 about 10.5, S3.2b-2 about 9.5).

## Measured results

- **Exit 1, live against replay:** `TestLiveEqualsReplay`, 200 seeded recordings × 1, 2, 4 and 7 shards, per rule and key identical outputs and equal counters; its teeth red with a shard-dependent body. The playback package runs in about 2.5 s.
- **Exit 5, the three §8 ports:** `TestRatePortWorkedExample`, `TestRatePortTie`, `TestRatePortGlobalAnchor` (each naming its source check), green.
- **Owner D1:** `TestOnArrivalPurgesOnItsStreamsAdmission` gives the written rule's 2 alerts; with design v1's feed planted, 4.
- **Allocation:** a non-matching event of a resident key through `Sync`, 0 allocations, with the watermarks moving every step (break-it guard) and steady (QA).
- **Tests first:** QA's 34 tests were red on the stub, every existing test green. Each later behaviour change had a red test, except the one-input check (C6); the D1 feed, the stamp clamp and the two C1 guards were shown red by removing their line.
- **Concurrency:** 7 shards under `-race` with concurrent senders, table writes, ticks and stats scrapes, race-clean (break-it guard, 3 runs).
- **Coverage:** `internal/plan` 97.3% (first run 88.0%, under the floor; tests added).
- **The gate:** T2 `make check` exit 0 (52 s); T3 `make combine-check` exit 0 (53 s), before the fix round.

## Findings

| # | From | Severity | Finding | Disposition |
|---|---|---|---|---|
| F1–F12 | design review | 2 blockers, 7 should-fix, 3 later | see [the design's §12](s3-2b-1-design.md#12-review-and-test-gap-dispositions) | each placed in design v2 and built |
| B1 | break-it | blocker | `k of S, j of S` passed Check; Build kept only `j`, merging keys and losing alerts | fixed: E-PART-004 at deploy and Build refuses it (red test 3358258); multi-part keys → S3.2b-2 (owner) |
| L1 | break-it | later | E-NULL-003 refused selecting a rule's own optional partition key, never null where the rule runs | fixed (test first); flow-sensitive cases rejected: conservative by design, `coalesce` is the fix |
| L2 | break-it | later | a built plan compiles bodies from the checked model at `Start`; mutating it after Build would change rules | forwarded to S3.3 (Deploy owns the model) |
| — | break-it | later | integer overflow wraps silently (older code) | known issue S3.2b-1-1, for the owner's spec pass |
| C1 | combined review | should-fix | two D1 lines (drain/end feed; admission-only message) had no test | fixed: two guards, each red with its line removed |
| C2 | combined review | should-fix | D3's error at the stream's declaration, not the attribute | owner: at the attribute, S3.2b-2; annotated meanwhile |
| C3 | combined review | should-fix | three docs still put anchors in `partition.Instance` | fixed: annotated (ARCHITECTURE §3, the card's item 6, this record) |
| C4 | combined review | later | Sync copied the router's tick, write and drain glue | fixed: one shared step each |
| C5 | combined review | later | the replay's Walk yields only Seq counters | accepted deviation, recorded in design §13 (Phase 2's snapshot seam) |
| C6 | combined review | later | the one-input check was not red first | recorded; a mutation shows its test catches removal |
| C7 | combined review | later | the MUST 10 test's comment said keyed | fixed: comment; the keyed path is the break-it guard's |
| C8 | combined review | later | the default null rule is wrong for an aggregate | forwarded to S8 |
| C9 | combined review | later | TimerFree dropped a timer silently; patterns and windows cannot call plan | fixed: it panics (test first); the wrapping forwarded to S4.1 |
| M1, M2 | material | — | the spec does not define a given list's arrival order or require the output count to match | forwarded to S3.3 |

## Done-done

```
Sprint S3.2b-1 — the plan, the replay loop, rate limits, the material · routine: S3.1's (QA first + architecture review + break-it) · SP ~10.5 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2b's, v3) ☑ dependencies merged, main green (S3.2a-2's PR #11; PRs #12, #13; nightly run by hand green)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-06) ☑ test material: QA's tests and item 9, items
Design (G1)  ☑ architecture review resolved (design v2 §12) ☑ QA testability (34 tests red first; G1–G7 placed) ☑ owner decisions recorded (D1–D5) ☑ ADRs, SEMANTICS and SPEC annotated
Build (G2)   ☑ T2 (make check exit 0) ☑ T3 (exit 0) ☑ tests on the real code (QA's, red first)
Review (G3)  ☑ architecture review of the combined diff (0 blockers) ☑ QA break-it (1 blocker) ☑ every finding fixed, forwarded or rejected (table above)
Verify (G4)  ☑ T4 exit 0 (final tree, wall 95.8 s) ☑ CI on PR #14 (lint, test, race, proofs, strict-gate: pass) ☑ owner's check (merged)
             ☑ exits 1 and 5 met by their proofs (TestLiveEqualsReplay; the three §8 ports)
Exit (G5)    ☑ one PR (#14, 40 commits), merged by the owner ☐ main CI green on the merge commit (the owner reports it)
             ☑ record + retro ☑ known issues (S3.2b-1-1) ☑ status line
             ☑ documents reconciled ☑ memory updated ☑ worktrees removed ☑ points and duration
```

## For the owner (the owner's check)

- **By reading:** the live/replay scenario (`internal/playback/equivalence_test.go`); examples 07 and 09's expect blocks; the three new deploy errors (`go test ./internal/validate -run 'TestStatic/(e-part-004|e-null-003|e-type-001)' -v`).
- **Rule-file changes, by hand** (the LAW Δ block's instrument is S1.1c's): `tools/importcheck/rules.json` (test imports), SPEC rev 4, SEMANTICS §8 and §9, errors.md, examples 07 and 09 with their parser goldens.
- **No new dependency.**

**Closed (G5), 2026-11-06.** PR #14 merged by the owner (40 commits, main 4ffcee9); CI green on the PR. Points: about 10.5; duration: about one day (plan check 2026-11-05 to merge 2026-11-06).

## Retro

- **Worked:** blind reviews that proved each claim with a scratch program found two design blockers before any code (F1: rules that could never fire were built; F2: a purge re-opened a suppression) and the break-it found a third after it (B1: two keys of one stream merged). QA's 34 tests, written first against a stub, specified the build: the plan package went green on its first build.
- **Hurt:** design v1 moved the rate anchor against a recorded annotation (F5) and missed the multi-stream purge (F2); a network error and a session restart stopped the material agent twice; one exit code was read from `head` instead of the command (caught and re-run); the one-input check was written before its test.
- **Keep:** re-run an agent's key scratch program on a fresh copy before acting on it; prove a guard test red by removing the line it guards; ask the owner through the question dialog in small batches, options ranked by accuracy.
- **Change next sprint:** S3.2b-2 grew to about 9.5 points with multi-part keys and D3's position; its plan check starts with the language-design step for `(k, j) of S`.
- **Lasting-rule proposals still open for the owner:** (1) a design claim about a library's behaviour is checked with a scratch program before review; (2) a new CI workflow is run once by hand in the sprint that adds it; (3) new: a guard test that pins wiring (not a defect) is proven red by a mutation before it counts.
