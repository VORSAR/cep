## S1.1c-7 pre-check: Go plant predictions in design-B, design-CD1 and design-CD2, checked against today's code

All 30 eligible plant runs gave the outcome the notes predict for the old code, and all 23 control tests pass. The old-outcome check therefore finds nothing wrong. Extra checks outside the brief found two problems in B that the old outcome cannot show:
- **G2-92's plant has no effect** on the code `TestLiveEqualsReplay` runs, so its predicted new red will not come. I tested a working replacement plant.
- **G2-107's stop rule fires:** with the note's test change, the real code fails the test.

I wrote no tracked file and made no git writes; `git status` is the same as at the start. No subtest, patch, Python, benchmark or load runs.

### 1. Header
- **Notes covered:** design-B.md with design-B-rev1.md; design-CD1.md with design-CD1-rev1.md; design-CD2.md with design-CD2-addendum.md.
- **Code:** HEAD 1f4ebba. No `*.go` file has changed since d521725.
- **Toolchain:** `/opt/homebrew/bin/go` is `go1.27.4 darwin/arm64`, the same as go.mod's `toolchain go1.27.4` and the pinned version.
- **GitNexus:** I have no resource-reading tool, so `index.commit` was not read. I made one `context` call, on `build` in `internal/runtime` (epistemic exact). Its callers are `New` (lifecycle.go), `NewSync` (sync.go) and tests.
- **Memory:** no Hindsight tool was available, so none was used.
- **Runs:**
  - **Old-code runs (mutate.py):** 30 runs.
    - B: 11 entries.
    - CD1: 9 entries, counting G2-72-bmp twice (the original and rev1's re-emitted entry).
    - CD2: 9 entries.
    - One candidate replacement plant for G2-92.
    - G2-92 and G2-107 each ran twice.
  - **Real tests:** 23 control tests.
- **Skipped, and why:**
  - **Subtest entries:** G10-18, G2-96, G10-16-a and -b, G2-70 ×6, G2-74, G2-136, G2-137, G2-138.
  - **Patch entries:** G2-97, G2-139, and the revert diffs of G10-15, G10-22 and G7a-3.
  - **Fix-commit plants** (their find text exists only after the fix): G10-19, G10-19-chain, G10-16. You asked me to skip B's K1 batch.
  - **Benchmark plants or data plants:** G2-105, G2-98, G2-76.
  - **Grep, K6 or K2-del proofs, which have no plant.**
  - **Not reached:** G2-66's new-side model check (the test-double change is too large to copy into an overlay safely), and every load run.
- **Raw outputs:** `(scratch)`

### 2. Mismatches
**Against the predicted old outcome: none.** Every mutate run printed the predicted word, and no find text was `invalid`. The problems below are in the new-side predictions or the proof instructions.

**Supplementary method.** I copied the note's test change into a file outside the repo and ran `go test -overlay`, which builds with the copy instead of the tracked file.

| Note | Finding / entry id | Command (short) | Predicted | Observed | Last lines of output | Likely cause |
|---|---|---|---|---|---|---|
| B | **G2-92** (stop-rule entry) | note's eqLive `len(stamps) != 0` check in an overlay, plus the build.go plant | new **red** | **green** | `--- PASS: TestLiveEqualsReplay`; a debug log showed `left=0 extra=0` in all 800 seed × shard runs, with and without the plant | The plant is on code `runtime.Start` never runs. `Start` calls `New`, which calls `build(cfg, nil)` (lifecycle.go:24), so build.go:30 never runs for the live runtime. It runs only through `NewSync` (sync.go:31). The live read at Start is `router.add`, transitions.go:40, reached through `Start` → `Add`. The note's "build.go:28-31 reads once at Start" is wrong for the live runtime. |
| B | G2-92, **replacement plant** (tested) | `mutate.py --file internal/runtime/transitions.go --find $'\tstart := rt.now(r.cfg.Now())\n' --replace $'\tstart := rt.now(rt.stamp)\n' --test TestLiveEqualsReplay --pkg internal/playback` | old green, new red | old **green**; with the new check **red** | `live: Config.Now read 1 times short of once at Start and once per event (design §7.8)` at seed 0, 1 shard | Use this plant. The risk that a missing read shifts later arrivals did not show: the old test stays green. |
| B | **G2-107** | note's coverage-guard change (`total.ConstructLate += …`, `\|\| total.ConstructLate == 0`) in an overlay, real code | new control green | **red** | `equivalence_test.go:404: the generator does not exercise the comparison: totals {ConstructLate:0 Purged:273 KeyErrors:884 … Suppressed:1165 Errored:950 …}` | The 200 seeds produce no construct-late at all. **The note's stop rule fires** (section 3). |
| CD1 | G2-79 grep | `grep -cE '^var [A-Za-z]+ = \[\]pecRow\{'` on the two files | old 4 + 1 | **3 + 1** | `pecG1Rows` (contract test :303) is not matched | The regex leaves out digits. It is not a lost backslash. Use `[A-Za-z0-9]+`, which gives 4 + 1 (:88, :263, :303, :369, and the operands file :15). |
| CD1 | G2-70 table; G2-72 and G2-69 text ("the block starting on line 76 and holding :77") | `go test -coverprofile`, then read the profile | blocks start on 71, 76, 275, 113 | Go reports each block from its first statement: `unescape.go:72.2,74.26`, `unescape.go:77.5,79.1`, `decode.go:276.4,277.1`, `decode.go:114.3,115.1` | At old under TestDecodeDifferential: `unescape.go:76.4,76.78 1 1` and `unescape.go:77.5,79.1 2 0` | No block starts on 71, 275 or 113, so those lookups find nothing. For G2-72, the block that does start on 76 is the `if pair :=` line, and it already has count 1 at old. Read literally, the instruction would fire G2-72's stop rule falsely. The real `:77` block is 0 at old, so the K2b claims hold; only the line numbers in the instructions are wrong. |
| CD2 | G2-136 coverage | `go test ./internal/validate -run '^TestOneFindingPerDefect$/^!=_over…E-NULL-002$' -covermode=count -coverprofile=…`, then `grep 'nulls.go:63\.'` | a block at 63.17, count 0 | the grep prints **nothing**; the block is `nulls.go:64.3,66.10 1 0` | the subtest passes, coverage 47.2% | The start column was guessed and is wrong. Grep for `nulls.go:64\.`. The old count is 0, as predicted. |
| CD2 | G2-87 (prose plant on body.go) | mutate on `TestStateWalksItsAnchor` | red at `:59` | **red** at `internal_test.go:60` | `walk [anchor/=], want [anchor/=42]` | Minor: :59 is the `if` and :60 the `Errorf`. The message matches. |

### 3. Stop-rule results
- **G2-92:**
  - Plant, old code: **green** twice, so the stop rule does not trigger.
  - The plant has no effect on this test (section 2); replace it with the transitions.go plant.
  - Control `TestLiveEqualsReplay`: pass.
- **G2-107:**
  - Plant, old code: **green** twice. Control on the old test: pass.
  - **STOP RULE FIRES.** With the note's test change, the real code reads red: ConstructLate is 0 over the 200 seeds.
  - Per the note, the lane commits no G2-107 change. The choice between a generator change (later part) and narrowing the header's claim is yours.
- **G2-94:**
  - Control: pass. Plant, old code: **green**.
  - Extra check with the note's `step` change: real code **green**, so no body panics happen today. With the plant it is **red**: `Replay: line 2, rule R: 1 body panics, each a silent key reset…`. The stop rule does not trigger.
- **G2-108** (entry unchanged in rev1, same JSON):
  - Control: pass. Plant, old code: **green**.
  - Extra check with the note's `Close` check: real code **green**. With the plant it is **red**: `helpers_s3_2b_2_test.go:120: Close: runtime: closed`.
  - Only the control test was run. rev1's 22-test `-race` load run was not run (load runs are out of scope).
- **CD1 stop rules** (extra checks):
  - **G2-65:** with the note's test body, the real code passes. Kinds 1:4335, 2:5177, 3:4493, 4:3105, 5:12188 and ties 10071, all at least 100. With the plant it is red: `timer kind 4 fired 0 times`.
  - **G2-68:** old reads **timeout** (122 s), not red, so the stop rule does not trigger.
  - **G2-72:** the old count of the block holding :77 is 0 under TestDecodeDifferential, so the stop rule does not trigger and K2b stands.
  - **G2-69:** with rev1's backslash-u input, TestDecodeLeafAllocations is green, so the allocation count does not move. The benchmark's old count for the :77 block is 0.
- **Lost backslashes in CD1** (your watch item):
  - Every JSON entry in design-CD1.md and the rev1 file parses, and its find text occurs exactly once at HEAD. The exceptions are the `…`-elided G2-70 and G2-72 lines in the original, which rev1 replaces.
  - G2-65's and G2-68's `\t` and `\n` came through intact: the parsed entries are byte-for-byte the ones I ran.
  - G10-16-a and -b find 0 times at HEAD. That is expected, not damage: they are revert plants on the fix commit, and their replace text is today's code exactly once (parseerror.go:18 and parse.go:94).
  - rev1's six G2-70 finds each occur exactly once.
  - rev1's "the old file prints" byte checks all match: decode_bench_test.go `total 4 142:0`; decode_test.go `total 11` with every line 0; gen_test.go `total 29 27:0 95:5 96:3 97:8 105:0`.
  - G2-78's grep gives old 2 and G2-79's `panic(` grep gives 1, both as predicted. `pecEdit(` occurs 123 times, which is 122 calls plus the definition.
  - G2-75's grep counts on the plant match: one `^panic: runtime error: index out of range` line and one `--- FAIL: TestExpectPositions/07_block_1` line.

### 4. All other results (mutate.py, today's tree)
| Note | Entry | Predicted | Observed |
|---|---|---|---|
| B | G2-90a (bounds.go E-PART-007 → TestBreakitPartitionKeyNotDropped) | green | green |
| B | G2-90b (`twice = false`) | green | green |
| B | G2-91 (reader.go duplicate `start` → TestEveryEnvelopeSchemaIsValid) | green | green |
| B | G2-99 (recorder.go `q.queued-1` → TestARecordingThatOverflowsStaysReadable) | green | green |
| B | G2-103 (reader.go `errors.New("EOF")` → TestBreakitGuardReaderEdges) | green | green |
| B | G2-104 (clock.go `r.streams[:0]` → TestDriverStreamLateMovesNothing) | green | green |
| B | G2-104 (same plant → TestRecordingArrivalStreamTsIsItsArrival) | green | green |
| CD1 | G2-65 (rule.go BatchClose → DelayDue) | green | green |
| CD1 | G2-66 (clock.go `strict()` drops Discard) | green | green |
| CD1 | G2-67 (clock.go `to := m`) | green | green |
| CD1 | G2-68 (decode.go Null flip) | timeout | timeout |
| CD1 | G2-71 (decode.go `string(key)`) | green | green |
| CD1 | G2-72-pair (unescape.go) | green | green |
| CD1 | G2-72-bmp, original (`\tdst = …(dst, r)\n` with an if-block) | green | green |
| CD1 | G2-72-bmp, rev1 (`min(r, 0x7f)+(utf8.RuneError-0x7f)*min(r>>7, 1)`) | green | green |
| CD1 | G2-75 (lower.go `r.Fields[:min(…,1)]` → TestExpectPositions) | red | red |
| CD2 | G2-85:07, G2-85:09, G2-85:clauses (plan.go timeAttr plant) | green ×3 | green ×3 |
| CD2 | G2-86:time-before-records | green | green |
| CD2 | G2-86:fields-before-null-time | green | green |
| CD2 | G2-87 (body.go `-s.anchor.At` → TestStateWalksItsAnchor) | red | red |
| CD2 | G2-88 (plan.go `return p, …`) | green | green |
| CD2 | G2-133 (table.go stale-snapshot plant) | green | green |
| CD2 | G2-135 (bounds.go E-DUR-002) | green | green |

### 5. Controls (`go test -count=1 -run '^T$' ./pkg`, real code)
| Test | pkg | pass/fail |
|---|---|---|
| TestLiveEqualsReplay | internal/playback | pass |
| TestDriverEmptyStreamNeverStalls | internal/playback | pass |
| TestRecordingStampsNeverFallBelowTheStart | internal/playback | pass |
| TestBreakitPartitionKeyNotDropped | internal/playback | pass |
| TestEveryEnvelopeSchemaIsValid | internal/playback | pass |
| TestARecordingThatOverflowsStaysReadable | internal/playback | pass |
| TestBreakitGuardReaderEdges | internal/playback | pass |
| TestDriverStreamLateMovesNothing | internal/playback | pass |
| TestRecordingArrivalStreamTsIsItsArrival | internal/playback | pass |
| TestBreakit_GranularityScenariosAreNonTrivial | internal/clock | pass |
| TestBreakit_D1_GranularityRandom | internal/clock | pass |
| TestBreakit_RegistryMatchesTheModel | internal/clock | pass |
| TestBreakitDecoderSharedConcurrently | internal/event | pass |
| TestDecodeDifferential | internal/event | pass |
| TestExpectPositions | internal/parser | pass |
| TestBuildExample07 | internal/plan | pass |
| TestBuildExample09 | internal/plan | pass |
| TestBuildPartitionedAndTimeClauses | internal/plan | pass |
| TestBuildNamesABlocksFirstFault | internal/plan | pass |
| TestBuildRefusesNotBuilt | internal/plan | pass |
| TestStateWalksItsAnchor | internal/plan | pass |
| TestBreakitGuardSnapshotUnderConcurrentWriters | internal/table | pass |
| TestBreakitANegativeDurationOverrideInABoundSlotIsRefused | internal/validate | pass |

Every control ran a single time, without `-race`. The playback tests start goroutines, so one passing run does not rule out a failure under load.