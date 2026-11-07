# S3.2b-2 combined build: architecture review (step 7, blind)

> **Historical — S3.2b-2, multi-part keys, arrival time, the recorder, error counts, the gate's proofs; merged in PR #19.** Not current: what is built is the code; the decisions it carried out start at [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-06--s32b-2s-plan-check); what is next is [sprint-plan.md](../sprints/sprint-plan.md#status). **This sprint:** [design](../sprints/s3-2b-2-design.md) · [record](../sprints/s3-2b-2-record.md) · reviews: [design: architecture](s3-2b-2-design-architecture.md), **combined build: architecture**, [break-it: QA](s3-2b-2-breakit-qa.md). **← [S3.2b-1](../sprints/s3-2b-1-record.md) · [S3.2c](../sprints/s3-2c-record.md) →**

Reviewed: worktree `/Users/ramith/code/cep-wt/s3-2b-2-review`, detached at 6f65447 (`f32fd41..6f65447`, 24 commits), against `docs/sprints/s3-2b-2-design.md` v2 (§1.3, §2, §4–§10, §15, §16), the owner's decisions (S3.2b's plan check; S3.2b-1's review and break-it; S3.2b-2's plan check; S3.2b-2's design), and ADR-0005 §3, ADR-0006 §5, ADR-0010 §1, ADR-0015 §3.11.

## Summary

Multi-part keys hold together from the router to `partition`. Single-part keys hash with the same bytes and do not allocate. Live stamping, the ingest flag, the rejected item and the per-stream counters give the same results live and in replay on every path I traced. `go test -race` on runtime, playback and partition exits 0. The hand-off allocation budget held over 24 runs at `-cpu 1,2,8`.

The weak spot is where the recording meets the line limit and the recorder's lifecycle:

- **One payload can end the recording.** A payload that live intake accepts is checked against the line limit at its raw size. The recording keeps its cleaned form, which can be up to three times longer, so the line cannot be written and the recording ends. Any source can trigger this (C1).
- **`TableWrite` has no line-limit check at all** (C2).
- **A full queue leaves a broken file.** When the queue fills, the records already queued are dropped and the writer is never flushed. The file ends in a torn line and does not match the reported counts (C3).
- **A stuck recorder blocks `Drain`.** The drain waits for the recording's stop line, so `Drain` and `Close` hang until their deadline even though detection has finished (C4).

**Counts: 1 blocker, 3 should-fix, 3 later.** One owner question (C4). Every behaviour finding is backed by a scratch test I ran. Exit codes were read directly, never through a pipe.

Scratch material is under `/private/tmp/claude-501/-Users-ramith-code-cep/c32ea7c4-ab50-4044-8f33-e9ed338fa3bd/scratchpad/combined-review-2/`:
- `src/`: clean copy of 6f65447, used for the race run and the allocation runs.
- `src2/`: the same copy plus the two scratch tests:
  - `src2/internal/playback/zz_review_test.go`
  - `src2/internal/runtime/zz_review_test.go`
- Outputs: `zz1.out`, `zz4.out`, `race.out`, `alloc.out`.

## Findings

| id | severity | finding | evidence (6f65447; scratch) | fix |
|---|---|---|---|---|
| C1 | **blocker** (broken contract MUST 14 / review F4's stated purpose "every payload the intake accepts is one the reader can read back"; reachable by any source) | `SendPayload` checks the **raw** payload's length against the line limit. The recording, however, keeps `event.CleanObject(payload)`, in which each malformed UTF-8 byte becomes U+FFFD (3 bytes). The decoder accepts malformed UTF-8 in strings, so the payload is accepted, detected and not rejected. Its recording line is then over 16 MiB, the encoder refuses it, and **the recording ends for good**. Every later event goes unrecorded, and the recording no longer replays to the live run (owner D1's promise). One crafted or simply badly-encoded large payload from one source is enough. | `internal/runtime/boundary.go:88` (`len(payload) > payloadLimit(name)`), `:104` (the cleaned copy recorded); `internal/playback/recorder.go:35-37` (`errLineTooLong`). Scratch `TestZZPayloadExpandsPastTheLine` (`src2/internal/playback/zz_review_test.go`), output `zz1.out`: `raw payload 6291519 bytes; limit 16776959; cleaned 18874431` / `live S: {… Rejected:0 …}; outputs OA=2` / `Recording: {Records:1 Failed:true Err:playback: a recording line longer than 16777216 bytes}` | Check the payload's **cleaned** length, recording on or off, so detection never depends on the setting. Add `event.CleanLen(raw) int`: one pass that counts each malformed byte as 3 and whitespace outside strings as 0, or reuse `CleanObject` and compare its length. Reject with reason `syntax` when it exceeds `payloadLimit`. Test, red first: a payload under the raw limit whose cleaned form is over it is counted `Rejected` and recorded as a rejected line, and the recording continues. |
| C2 | should-fix (MUST 14 for `TableWrite`; the recording ends, but it is counted and detection is unaffected) | `TableWrite` runs `recordable` on each value but never bounds the line. `Send` has `lineBound`; `TableWrite` has nothing. A row whose recorded line exceeds `MaxLogLine` is accepted and ends the recording. Two ways to get there: a string of control characters (each written as `\u00XX`, 6 bytes), or an object text over 16 MiB (`ExactObject` checks form, not length). | `internal/runtime/runtime.go:557-575` (no bound); `internal/playback/recorder.go:136-175` (one line per row). Scratch `TestZZTableRowPastTheLine`, `zz1.out`: `TableWrite: <nil>` / `Recording: {Records:1 Failed:true Err:playback: a recording line longer than 16777216 bytes}; outputs OA=1` | Bound each row's recorded line the same way `lineBound` does: table name, column names, each string at 6×len+2, each object at its length, other kinds at 40, plus the envelope. Refuse the write with an error to the caller. Test, red first, one row per case (long escaped string, long object). Not proven: `lineBound` also leaves out `Event.BatchID`, so a `Send` with a huge `BatchID` would end a recording the same way. Count it, or refuse a `BatchID` on live `Send`. |
| C3 | should-fix (misleading report; the recording is unreadable at its tail) | When the queue fills, `put` sets `failed` under the mutex. From then on the recorder goroutine's `handle` skips **every record already queued**, and `run` skips `Flush` at stop because `isFailed()` is true. Three results: (a) records accepted onto the queue before the failure are dropped; (b) with a buffered writer, the file ends mid-line and replaying it faults at the last line; (c) the report disagrees with the file. The message says `queue full at record 118`, `Records` says 53, the file has 52 readable lines plus a torn one. Owner D2 says a full queue is "reported with the record where it stopped", and the design's lifecycle (§7, F17) says "the goroutine writes what is queued". A full queue is a *router-side* end; the writer is healthy. The unit test pins this behaviour (`flushed != 0` is an error). | `internal/runtime/recorder.go:64-75` (put sets failed), `:132` (handle skips when failed), `:117` (no Flush when failed); `internal/runtime/s3_2b_2_recorder_test.go:151-155`. Scratch `TestZZQueueFullTornFile` (`bufio.Writer` 4 KiB over a writer that stalls once, `RecordQueue: 64`), `zz1.out`: `Recording: {Records:53 Failed:true Err:runtime: recording queue full at record 118…}` / `file: 8192 bytes, 52 newlines, ends with newline: false` / `torn tail: "{\"arrival\":\"2026-01-01T10:00:52."` / `replay of the file: playback: line 53: not a log record: … unterminated string` | Keep a queue-full end separate from a writer error. On a full queue, the router stops enqueueing and records the message. The goroutine writes the N−1 records already queued, then flushes, so the file is a clean prefix and `Records` = N−1 matches the message. On a writer error, keep today's behaviour: no further calls, no flush. Change the queue-full test to expect a clean prefix of N−1 lines and one Flush. |
| C4 | should-fix (detection is done, but the caller sees a failure, and Stats shows a healthy recording); owner question Q-A | The drain adds the recording's stop marker to the markers `Drain` waits on. A recorder that never returns (a hung network disk) blocks `Drain` until ctx's deadline, even though every output is through its sink. The recording is not marked failed, because the queue never fills. `Close` then also returns ctx's error. This conflicts with owner D2 ("detection never waits") and is not in design §7 (which says only "the stop record is put at the drain") or the as-built differences in §16. | `internal/runtime/runtime.go:827-833` (marker put at the drain), `:1030-1032` (added to the drain's markers); `Drain` waits on every marker at `:599-607`. Scratch `TestZZDrainWaitsOnAStuckRecorder` (`src2/internal/runtime/zz_review_test.go`), `zz4.out`: `Drain after 2.002s: context deadline exceeded (deadline: true); outputs through the sink: 3` / `Recording: {Records:0 Failed:false Err:}` / `Close: context deadline exceeded` | Recommended: `Drain` does not wait on the recording. It answers for detection and its sinks. A caller that needs the file complete calls `Close`, which already waits, bounded by its ctx, for the recorder goroutine to write and flush. Alternative: keep the wait, but give up after a bound and record the recording as failed with "stop not written". Add §16 as-built text either way. Test: a recorder that blocks forever gives a `Drain` that returns with every output through its sink. |
| C5 | later (misleading doc) | MUST 9 still says "The recorder MUST be called **on the router goroutine**". That contradicts §7 (its own goroutine), MUST 13 ("the router MUST NOT … call a recorder") and owner D2. The `Recorder` interface comment in the code says "The router calls it on its own goroutine", which is ambiguous. | `docs/sprints/s3-2b-2-design.md:364`; `internal/runtime/runtime.go:57-59` | Annotate MUST 9: "called in intake order by the app's recorder goroutine; the router only enqueues". Reword the interface comment to name the recorder goroutine as the only caller. |
| C6 | later (doc drift) | Design §8 says `gate/tripwire.json` is "added to `gate/law.json`", but no such file exists: law files are owned by path (ADR-0015 §1.4: `gate/`). In the file, `"rules"` overrides are looked up by **rule name** for `outbox`, by **sink stream name** for `sink-depth`, and by **program name** for `drained_pending` (`"qa-eq": 8`). ADR-0015 §3.11's annotation says only "that rule its own under `"rules"`". | `internal/playback/s3_2b_2_corpus_test.go:236-251` (the three `check` calls and the names they pass); `gate/tripwire.json:4`; `docs/architecture/adr/ADR-0015-build-enforcement-hostile-gate.md:140` | Annotate §16 and the ADR-0015 note with what each metric's override key names. Drop the `gate/law.json` mention. |
| C7 | later (a guard without its test) | MUST 16 (`partition` never writes `Key.Rest`) holds in the code: `Offer` normalises only `key.First`, and `keyOf` normalises a local copy. No test offers a `Rest` holding `-0.0` and checks that it is untouched afterwards. The sprint model's rule is that a guard counts only once shown red. | `internal/partition/partition.go` (`Offer`, `keyOf`); `internal/partition/multipart_test.go:22-47` checks instance count only | Add a test: offer `Key{First: s, Rest: []Value{-0.0}}`, then assert `Signbit(Rest[0].F)` is still true and the stored instance's part is +0.0. Show it red by planting a write into `Rest`. |

## Checked and holding

- **Race and leaks:** `go test -race -count=1 ./internal/runtime ./internal/playback ./internal/partition` exit 0 (`race.out`). goleak's `TestMain` is in `internal/runtime` and `internal/playback`.
- **Allocation budget robustness:** `TestHandoffAllocationBudget -count=8 -cpu=1,2,8` exit 0 (`alloc.out`). The test is not parallel, so other tests in the package do not run while it measures.
- **MUST 1 / 12:**
  - Hashing: single-part `shardOf` writes the same bytes as before (kind byte, then the value).
  - Allocation: `keyOf` on a single part is `valueKey`, with no allocation; a key-less route is `Single("")`.
  - Plan for one stream: `plan.Rule` takes only the body source's key.
- **MUST 3:** `routeKey` treats a null or NaN in any part as that rule's `KeyErrors` (`runtime.go` `routeKey`; `routekey_internal_test.go`).
- **MUST 4:** the map key is `keyT` (part 1 in its typed field) plus `m`: per part the kind byte, length-prefixed strings, fixed 8-byte numbers, and float `-0.0` written as `+0.0`. It is injective per rule; the test covers kind-byte look-alike strings.
- **MUST 5:** `routeKey` allocates a fresh `Rest` per route per event; `deliver` copies the offers slice.
- **MUST 6:** only the router's `ingest` stamps a `time arrival` event. It does so before `put` and before `rt.event`. `Sync.Event` passes the arrival without stamping.
- **MUST 7:** the only writer of `Event.Invisibles` is `internal/runtime/ingest.go:62`. The expression node reads `Row.Invisibles`, and the decoder fills a declared slot with `false`, never written later.
- **MUST 8:** `iReject` raises the router's stamp (one `Config.Now` read), puts the rejected record, and calls `Ingest.Reject`. That counts and raises the stamp only: no Seq, clock or offer. `Sync.Reject` mirrors it, and the driver calls it for `RecRejected`.
- **MUST 13:** `put` is a `select`/`default`. The router-owned fields (`ended`, `queued`) are touched only on the router, or in `build` before any goroutine starts. The goroutine's results are under `mu`. A record copies the `Event` struct; its `Values` are read-only everywhere after intake. Table rows are copied (`copyRows`) before the reply. The payload is a fresh string made on the caller.
- **MUST 15:** the router's stamp starts at `r.start`. A non-virtual `NewIngest` starts at `start`; replay in recording mode starts at the header's start.
- **Live = replay (MUST 10) on traced paths:** typed lines leave out `text_invisibles` (F3). Payload lines carry the cleaned payload, so replay's decoder reproduces `Unknown` and `Reserved`; decoding the cleaned payload equals decoding the raw one for values, because the decoder already turns malformed UTF-8 into U+FFFD and keeps escapes. A tick record carries the raised stamp. Table records are put after a successful write and before its reply.
- **Depth budget:** an object in a table row nests at the same depth as one in an event's attrs (line, then row or attrs, then the value), so `ExactObject`'s depth-2 start fits both. `TableWrite`'s NaN and infinities are refused by `table.Upsert` (`internal/table/table.go:226`).
- **Tripwire and determinism:**
  - Missing ceilings: a missing metric entry fails, and `"none"` needs a reason.
  - Outbox granularity: per-message counting is pinned by `TestOutboxHighIsOneAdvancesEmissions` (65,537 from one event).
  - Determinism plant: it sits in the real test's path.
  - Runners: `RUNNERS` maps `determinism-macos` to `macos-latest`. `gotest.py` judges like the unit row and counts passed tests for `min_wants`.
- **Import law:** `test_external` lists goleak only, for runtime and playback test files.

## Not checked (time box)

- The validator's check order and messages (E-PART-003/004/005/006/007, D3's position, Q3's two counter-files). I checked only that their tests exist (`internal/validate/s3_2b_2_test.go`).
- The parser and grammar changes, and the `tools/shape` / lark differential.
- SPEC rev 5, errors.md and SYNTAX text against the code. I skimmed SEMANTICS §9 and §10 and the ADR-0005/0006/0010/0015 annotations and found nothing contradicting the code beyond C5 and C6.
- Not proven: a single `Recorder` value given to two apps would be called from two goroutines. `validateApp` does not refuse it, and `playback.Recorder` is not safe for concurrent use.
- GitNexus: this review rests on file reads and scratch runs. I did not read the index's freshness resource or use its query tools.

## Question for the owner

**Q-A (C4): should finishing a drain wait until the recording's last line is written?**

Today the drain waits. If the disk holding the recording hangs, the drain never reports done, even though every alert has already gone out. The caller sees a timeout, and the stats say the recording is fine.

- **Option 1 (recommended): the drain reports done once the alerts are out.**
  - The recording keeps writing in the background, and its state is in the stats.
  - Shutting the engine down (`Close`) waits, up to its own deadline, for the recording to finish.
  - Pro: a stuck disk never holds up alerts or the drain. This matches "detection never waits".
  - Con: a caller that reads the recording file right after the drain must call `Close` first.
- **Option 2: the drain waits, but only up to a limit.**
  - After the limit, the recording is marked failed ("stop not written").
  - Pro: a finished drain means a finished file.
  - Con: one more setting, and the drain is still slow when the disk is slow.
