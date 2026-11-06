# S3.2a-1 break-it: the D1 clock, partitions, the snapshot carrier, D4, the replay format layer (breaker QA)

> **Historical — S3.2a-1, the replay format layer and partitions; merged in PR #10.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-05--s32as-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2a-1-design.md) · [record](../sprints/s3-2a-1-record.md) · reviews: [design: architecture](s3-2a-1-design-architecture.md), **break-it: QA**. **← [S3.1-2](../sprints/s3-1-2-record.md) · [S3.2a-2](../sprints/s3-2a-2-record.md) →**

**Base:** e56bb27 (`s3-2a-1/integration`). **Branch:** `s3-2a-1/breakit` (worktree `/Users/ramith/code/cep-wt/s3-2a-1-breakit`). Not pushed.
**Red-test commits:** 4048a6f (K1; guards), 46383dc (K2, K3; guards). **Guard-only commit:** dbddea9 (table, event).
**Test runs (pinned Go 1.27.3, GOTOOLCHAIN=local GOFLAGS=):** I ran each of the 3 red tests alone with `go test -run '^Name$'`. Each exits 1, for its stated reason. Every `TestBreakitGuard*` test passes in `./internal/partition ./internal/playback ./internal/table ./internal/event` (exit 0). The table guards also pass under `-race` (exit 0). The pre-commit gate passed on all three commits.

**Result:** 0 blockers, 2 should-fix, 1 later. All three are in this sprint's code.

**Method:**
- I read design note v2 (all of it), then read the code under attack adversarially: `internal/clock/rule.go`, `internal/partition`, `internal/table` Set/Snapshot, `internal/event/decode.go` (cleanObject and the RFC 3339 reader), and all of `internal/playback`.
- I also read the executor's D1 `drain` and `_next_timer`. Key order there is `last_act` insertion order: a purge deletes the key, and a rebirth re-inserts it at the end. The Go side matches this, because `bear()` assigns a fresh birth at the next release.
- **D1 equivalence:** I tested it in the Go rule, not against the executor. A randomized differential compared three runs over 300 seeds × 40 messages: a walk after every message, a walk after random chunks, and one walk at the end only. Each seed used two inputs and string keys, with these cases:
  - equal timestamps;
  - construct-late events;
  - Completion timers due at an event's own instant;
  - CronClose timers chained at the instant being walked;
  - discards;
  - TTL purges inside large jumps;
  - then +inf.
- I ran randomized round trips through the reader and the encoder: 3000 lines covering every type, nulls, escapes, malformed UTF-8, and the float and timestamp edges.
- I ran a D4 oracle (`json.Compact` after a per-byte U+FFFD repair) over 3000 generated objects.
- Plus targeted envelope, line and recording cases, and a concurrent-writer `-race` test.

**Not tried:**
- A Go-versus-executor differential on the same randomized logs. The committed oracle fixtures (design §6) cover this partly.
- Fuzzing with `go test -fuzz`.
- Whole-engine runs (the router and shards are S3.2a-2).

## Findings (ranked)

| id | sev | contract broken | red test | repro (one line) |
|---|---|---|---|---|
| K1 | should-fix (arguably blocker as a "broken contract": the owner's call) | design §1 "Advance ... after each message or after a whole chunk; D1 makes the two the same"; §9 B1 "every callback is the same"; §10 "Chunking is free under D1" | `internal/partition/breakit2_test.go` `TestBreakitNegZeroKeyFollowsWalkFrequency` | float key, TTL 10: -0.0 at 0; clock to 1; an advance alone to 15; +0.0 at 20; clock to 21. Walking per message, the body sees `in.Key` = +0. Walking only at the chunk end, it sees -0 |
| K2 | should-fix | design §5.4 "The round trip: for every event a Reader accepts, AppendEvent gives a line the Reader reads back" | `internal/playback/breakit2_test.go` `TestBreakitTimestampOffsetOutsideEncodableYears` | `{"stream":"A","attrs":{"ts":"0000-01-01T00:00:00+00:01","k":"a"}}`: the reader accepts it (Ts -62167219260000), then AppendEvent fails with "outside the years 0000-9999". Same for `9999-12-31T23:59:59.999-00:01` |
| K3 | later | design §5.5 "for `end`, the last line + 1"; §5.2 "Line numbers count every line from 1" | `TestBreakitEndAtCountsEveryLine` | a log of one record followed by two blank lines: `end` is at 2, want 4 |

## Should-fix

### K1: a -0.0/+0.0 key's `Instance.Key` depends on walk frequency

**Where:** `internal/partition/partition.go`, `Offer` and `free`. `Instance.Key` is "the first event's key value", and `keyOf` makes -0.0 and +0.0 one key (design §3, correct). Which event counts as "first" depends on whether the purged instance is still resident when the returning event is offered:
- **Walking per message:** the walk at clock 15 purges the key and frees it. +0.0 then creates a new instance, so `Key` is +0.
- **Walking per chunk:** +0.0 is offered to the still-resident instance, whose TTL timer is pending. In the next walk the purge fires; the instance holds the buffered event, so it stays listed (design §3). It is reborn with `Key` still -0.

**What changes:** every callback reads `in.Key`. So does the trace's `"key"`, which `AppendValue` writes as `-0` or `0`. B1 accepted that Ordinals differ with walk frequency, "while every callback is the same". This difference is in a callback argument.

**Impact:** No detection changes, because the two keys compare equal. But S3.2a-2's shards are told they may chunk freely (§10). A body or output that shows the key would then print `-0` or `0` depending on the chunk size, which breaks ADR-0010 §2's batch-size invariance on the output text. Replay stays deterministic, because its walk schedule is fixed.

**Likelihood:** low. It needs a float-typed partition key, a source that sends -0.0, and the same key returning as +0.0 after a TTL purge.

**Fix area:** `partition.Offer`. Two options:
- store a canonical key value (-0.0 → +0.0) when the instance is created (the smallest fix);
- or refresh `in.Key` at rebirth (when `in.Op == nil` in `onEvent`).

The test then passes. Either way, design §3's "the first event's key value" wants one line of annotation.

### K2: the reader accepts timestamps the encoder refuses

**Where:** `parseRFC3339` (`internal/event/decode.go`) checks the year field 0000–9999 *before* the offset is applied. So `0000-01-01T00:00:00+00:01` is -62167219260000 ms, one minute before the encoder's `minTimestamp`, and `9999-12-31T23:59:59.999-00:01` is past `maxTimestamp`. The reader accepts both, as an event-time attribute and as an envelope `ts` alike. `AppendValue` / `AppendEvent` then refuse them.

**Effects:**
- Design §5.4's round-trip claim is false at these instants.
- In a replay, the same event's trace entry (`ts`) sets `Trace.err`, so `Trace.Bytes()` returns an error for the whole replay.
- `Instance.Events` and the clock handle the value fine; only output fails.

**Likelihood:** negligible in practice (years 0000 and 9999 with a non-zero offset). It was found by reading the code; the randomized round trip only uses in-range edges.

**Fix area:** two options:
- check the UTC instant against the 0000–9999 range in `parseRFC3339`. This changes what the decoder accepts, live ingest included, so it is a format change and needs the owner (ground rule 3);
- or have the reader fault on such a `ts`. That covers playback only, and live output would still fail at encode.

## Later

### K3: the `end` entry's line ignores trailing blank lines

**Where:** `Driver.End` sets `Line` to `d.line + 1`, where `d.line` is the last *record's* line. The design says "the last line + 1", and lines count every line, blank ones included. A log ending in blank lines gives `end` an `at` that is too small.

**Impact:** traces only; two logs that differ only in trailing blank lines give the same trace (arguably a feature).

**Fix area:** either `Replay` passes the reader's line count to `End`, or the design note's §5.5 is annotated to "the last record's line + 1" and the test's expectation is changed to match. Recommendation: annotate the doc, because the trace then ignores trailing whitespace.

## Observations not filed (no red test; for the coordinator)

- **Unknown payload attributes are ignored.** `eventRecord` does not check the payload decoder's `Decoded.Unknown`, while table rows and the envelope do. A fixture with a typo in an attribute name (`"xx":1` for `x`) reads as `x` null with no fault. This matches live ingest (P4 counts unknown keys), so it is not a contract break. Fixture authors may be surprised by it.
- **A payload fault's offset is measured in the wrong text.** The `DecodeError` offset of a payload fault is an offset into the *compacted* `attrs` text that the reader re-decodes (`vals[eAttrs].Obj`), not into the line. It is accurate but can mislead a person who counts columns in the file.

## Tried and held

**D1 clock and partition** (`internal/partition/breakit2_test.go`):
- **Walk frequency** (`TestBreakitGuardRandomWalkFrequencyInvariance`): 300 seeds. The three walk schedules gave identical callback logs, identical `Stats`, and identical construct-late verdicts for every Offer. `CheckTimers` held after every walk.
  - The logs also covered timers due at an event's own instant (released after the instant's events), chained timers at the instant being walked, and purges and discards inside jumps.
  - No event at an already-finalized instant was admitted, and every construct-late event was counted.
- **Allocations** (`TestBreakitGuardResidentKeyAllocatesNothing`): Offer + Advance on a resident, born key with TTL > 0 allocates 0 per step. Measured with `AllocsPerRun`, for a string key and for a -0.0 float key, so the TTL re-arm is included.
- **After +inf** (`TestBreakitGuardOfferAfterEndOfInput`): at +inf every key purges and is freed. A later Offer is construct-late and counted, and leaves no instance.
- **By reading `rule.go`:**
  - `CancelAll` ranges a map, but the heap's pop order is a total order (`seq` is unique), and `Timers` / `Events` sort. So no map order reaches output.
  - The executor's birth order matches Go's, including rebirth after a purge.
  - The TTLPurge-first, Discard-second order within an instant matches the executor's (1, order, 1/2) items.
  - A body that schedules `TTLPurge` itself replaces the partition's timer (`keyOf` ignores ref), and `CheckTimers` reports it.

**Reader** (`internal/playback/breakit2_test.go`):
- **`TestBreakitGuardReaderEdges`.** These are faults at their line:
  - a BOM mid-file; a NUL byte inside a string or between records; a lone CR between two records;
  - `"stop":false`; `"stop":"true"`; a record after stop;
  - a second header;
  - an arrival 1 ms before the previous tick;
  - a put with an optional key column null or absent; a delete with a null key; a delete with a non-key column;
  - `attrs` as an array; the header as an array; a header with an extra key;
  - a table record carrying `stream`.

  These read cleanly:
  - CR CR LF line endings; a final line ending in CR with no LF;
  - blank lines after stop; a header after blank lines;
  - an arrival equal to the previous tick;
  - repeated keys, where the last wins, both for `stream` and for `attrs`.
- **`TestBreakitGuardLongLines`:** a line of exactly 16 MiB (plus CRLF) is read. 16 MiB + 1 is a fault at its line, and `Next` returns the same error again.
- **`TestBreakitGuardRecordsAreFresh`:** mutating one record's `Values` or `Row` leaves the others unchanged.
- **`TestBreakitGuardDriverTableEdges`:** a delete of a missing key is a normal table step (no panic). The event's `Tables` is the last table step's snapshot.
- **`TestBreakitGuardReplayEdges`:** fn's error stops Replay at that step and is returned. A recording with only a header and a stop has no advances at the stop (nothing admitted) or at End.

**Encoder** (`internal/playback/breakit2_test.go`):
- **`TestBreakitGuardRoundTripRandom`:** 3000 generated lines. Every one the reader accepts encodes, reads back with bit-equal values, and encodes to a byte-identical fixed point. The values covered:
  - **strings:** every control escape, surrogate pairs, lone `\ud800`, raw U+2028/2029/DEL, malformed bytes, `\" \\ \/ \b \t`;
  - **ints:** ±MaxInt64;
  - **floats:** random bits in 'g', 'e' and 'f' form, plus -0, 5e-324, MaxFloat64 and 1e21;
  - **timestamps:** years 0000 and 9999, a leap second, a 9-digit fraction with a `+05:30` offset, lowercase `t`/`z`;
  - **objects:** messy whitespace, depth 50;
  - **batch ids** with escapes.
- **`TestBreakitGuardAppendEventAllocatesNothing`:** 0 allocations into a buffer with room. An `Obj` that is a `[]byte` is an error, not output.
- **By reading the code:** the trace's keys are sorted bytewise in every entry kind.
- I did not repeat QA's string escape check against `encoding/json`. It already covers malformed UTF-8, and on Go 1.27.3 that becomes raw U+FFFD.

**Tables** (`internal/table/breakit2_test.go`):
- `TestBreakitGuardFailedWritePublishesNothing`: a refused Upsert or Delete, and an empty one, return the same `*Snapshot`. A write to one table gives a new snapshot that shares the other table's Version; the old snapshot is unchanged. A steady-state `Snapshot` allocates 0.
- `TestBreakitGuardSnapshotUnderConcurrentWriters`: three writer goroutines (Upsert, Delete, Load) ran against 20000 `Set.Snapshot` calls on one goroutine under `-race`. No race was reported, and every snapshot stayed stable.

**D4** (`internal/event/breakit2_test.go`):
- `TestBreakitGuardObjectCompactRandom`: 3000 generated objects and arrays, nested up to 40 levels, with whitespace (including CR and LF) everywhere. Strings ended in an escaped quote or backslash, held malformed bytes at their very end (`"\xff"`, a truncated `\xe2\x82`, an encoded surrogate), sat in keys, or contained structural characters.
  - The stored text equals `json.Compact` of the per-byte-repaired text.
  - A string attribute read through P8 agrees with `encoding/json` on the repaired bytes.

## Commits

- 4048a6f `test(partition)`: K1 (red); guards: randomized walk-frequency invariance, resident-key allocations, an offer after +inf.
- 46383dc `test(playback)`: K2 and K3 (red); guards: reader edges, 16 MiB lines, fresh records, table steps, Replay edges, the randomized round trip, AppendEvent allocations.
- dbddea9 `test(table, event)`: guards only: a refused write publishes nothing, `-race` snapshot under concurrent writers, D4 compaction against `json.Compact`.

## Dispositions (coordinator, 2026-11-05)

| id | severity | disposition |
|---|---|---|
| K1 | should-fix | **Fixed** (60ab5d9): a float zero key is stored as +0.0 whichever zero arrives first. `TestBreakitNegZeroKeyFollowsWalkFrequency` passes; QA's `TestKeys_NegativeAndPositiveZeroAreOneKey` now expects +0.0. |
| K2 | should-fix | **Fixed** (702d14c), owner's choice (refuse at decode): a timestamp's UTC instant must lie in RFC 3339's years; the decoder refuses one an offset carries outside them (a live event error; a replay fault, D6). The test `TestTimestampUTCInstantInRFC3339Range` was added red first. S3.1-2's hand case and the differential's oracle were moved to the rule. |
| K3 | later | **Rejected as a code change; the design is annotated** (10d367f): the end entry's line is the last record's line + 1, because trailing blank lines are not steps. The red test became the guard `TestBreakitGuardEndAtTheLastRecordsLine`. |
| — | not filed | Unknown attributes in a fixture's `attrs` are skipped, as live ingest skips them. A typo then reads as null: **forwarded to S3.3**, where the expect blocks become logs, so that the conformance runner can make an unknown attribute in a hand-written expect block a fault. |
| — | not filed | A payload fault's offset counts into the compacted `attrs` text: **later**, noted; the message names the attribute. |
