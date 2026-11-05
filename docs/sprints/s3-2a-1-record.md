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

## Measured results

- **Tests first** (QA, four agents, apart from the build):
  - **Wave A:** the oracle's D1 checks (3 changed, 1 added, red on the old drain), 22 clock tests revised or added for D1, 35 partition tests, the clock-view and `table.Set` tests.
  - **Wave B:** the reader, driver and recordings (15 tests, 47 fault cases), exits 1 and 2, the encoder and trace (24), and D4 (2 new plus 4 existing decoder tests moved to the D4 oracle).
  - **Red evidence:** every test was seen red on the stub or the old code before the implementation was committed.
  - **One slip in a test:** exit 1's rule R3 read an attribute before checking the stream; it was fixed when the test first met the real code.
- **Exit 1:** `TestTraceDeterministic` — two replays of a three-stream fixture give byte-identical traces, with stream-late events at lines 7 and 13 and the construct-late event R2/c at line 9. `TestTracePlantedMapOrderIsRed` — a body that writes over a 16-entry map gives differing traces within 20 replays.
- **Exit 2:** `TestTableRecordFromNextEvent` (put and delete) — an event buffered before a table record reads the old membership after release; the next event reads the new one; a timer fired in a walk reads that step's snapshot, by pointer identity (D5).
- **Exit 4:** `TestOneTTLTimerPerInstance` — exactly one TTL timer per born key after every walk (10,000 events on one key; 100 keys → 100 timers); a re-arm allocates 0. `TestScheduleReplacementAllocatesNothing`: 0 allocations (1 before the fix).
- **D1:** the oracle passes 48 of 48; `TestRuleClock_D1_WalkTimingNeverChangesAResult` — walking after every step or once at the end gives identical callbacks.
- **Coverage:** `internal/clock` 98.0%, `internal/partition` 94.9%, `internal/table` 98.1%, `internal/playback` 93.8%, `internal/event` 99.0%.
- **The gate:**
  - T2 `make check`: exit 0.
  - T4 `make strict-gate` on the final tree, after the fix round and the QB4 fix: every row PASS, including the grammar differential (63) and `proofs` (14); exit 0; wall 84.0 s.

## The break-it (G3)

A fresh QA agent ([s3-2a-1-breakit-qa.md](../reviews/s3-2a-1-breakit-qa.md)) found 0 blockers, 2 should-fix and 1 later, each committed as a red test and handled in one batched round:

| id | severity | finding | disposition |
|---|---|---|---|
| K1 | should-fix | with a float key, the key value a body sees (`-0` or `0`) depended on how often the rule walked | fixed: a float zero is stored as +0.0 |
| K2 | should-fix | the RFC 3339 reader accepted a timestamp whose UTC instant lies outside 0000–9999, which the encoder refuses | fixed (owner: refuse at decode) |
| K3 | later | the end entry's line ignores trailing blank lines | the design was annotated (last record's line + 1); the test became a guard |

**What held, kept as guard tests:**
- 300 randomized seeds gave identical callbacks, stats and verdicts whether the clock walked per message, per random chunk or only at the end, with `CheckTimers` holding throughout;
- 0 allocations for Offer + Advance on a resident key, for `AppendEvent` and for a steady `Set.Snapshot`;
- every reader edge tried behaved as designed (16 MiB lines, BOM and NUL mid-file, records after stop);
- 3,000 randomized lines round-trip bit-exact;
- table snapshots stay race-free under `-race`;
- D4 matches `json.Compact` on 3,000 generated objects.

**T4's first run** failed one row: `pytests`, on S1.1a-1's `test_breakit_leak` (QB4), a test of the gate itself.
- **The cause:** a load-dependent race in the test's own planted module. It passed 3 of 3 alone and 4 of 4 under parallel Go load, and failed every time with the module delayed 3 s.
- **The fix (owner: here):** the helper waits for a go-ahead that follows the kill's registration. With the same 3 s delay, the fixed test passes.

**Not filed, forwarded:** an unknown attribute in a hand-written expect block should be a fault in S3.3's conformance runner.

**The oracle fixtures (review F8)** are in:
- **Fixture 1:** stream-late and construct-late lines match `run_pattern_multi` over 300 events on three streams.
- **Fixture 2:** all 88 absence completions match by line, key and due, over 200 events.
- **Checks on the fixtures:** rerunning the generator reproduces the fixtures exactly, and two planted faults turn the tests red.

## For the owner (the owner's check)

- **By reading: exit 1's fixture.** Read `internal/playback/testdata/log/exit1.jsonl` beside its trace, which `TestTraceDeterministic` prints on failure. Three streams; stream-late events at lines 7 and 13; R2/c construct-late at line 9.
- **By reading: exit 2's scenario.** `TestTableRecordFromNextEvent`: an event read before a table record keeps the old membership, however late it is released.
- **By reading: D1 in one test.** `TestRuleClock_D1_NoSealedInstant`: an event at the instant of a key's pending timer, arriving after the clock reached that instant, is processed before the timer. In SEMANTICS terms, an event at a deadline always defeats it.
- **What changed in the language documents.** Dated annotations in SEMANTICS §6.4's ordering paragraph (D1), ADR-0004, ADR-0005 (the log format: D2, D3, D4, D6), ADR-0006, ADR-0010 and ADR-0012. No syntax changed.
- **A behaviour change older code sees:**
  - an object attribute is now stored compact (D4);
  - a timestamp whose UTC instant is outside 0000–9999 is now an event error (K2);
  - the clock releases an event only once the construct clock is past its instant (D1).
- **No new dependency.**

## Done-done

```
Sprint S3.2a-1 — the replay format layer and partitions · routine: S3.1's (QA first + architecture review + break-it) · SP 5.5 → ~9 (D1, D3, D4) · cap 2 weeks
Entry (G0)   ☑ card complete and approved (S3.2a's, v3) ☑ dependencies merged, main green (S3.1-2's merge, PR #9)
             ☑ fits the cap, no lanes ☑ plan check answered by the owner (2026-11-05) ☑ test material: QA's tests, an item
Design (G1)  ☑ architecture review resolved (design v2 §9) ☑ owner decisions recorded (D1–D7) ☑ ADRs and SEMANTICS annotated
Build (G2)   ☑ T2 (make check exit 0) ☐ T3 (in T4) ☑ tests on the real code (QA's, red first)
Review (G3)  ☑ QA break-it (0 blockers, 2 should-fix, 1 later) ☑ every finding fixed or dispositioned, each with its test
Verify (G4)  ☑ T4 exit 0 (final tree) ☐ CI on the PR ☐ owner's check
             ☑ every acceptance line met by its named proof (exits 1, 2, 4; exit 3 is S3.2a-2's)
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ record + retro ☐ known issues ☐ status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration
```
