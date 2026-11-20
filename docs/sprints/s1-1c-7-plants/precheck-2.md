## S1.1c-7 pre-check: plant predictions for E1, E2 + E2 addendum and W0-go, run against today's tree

**Result:** 28 of the 29 eligible entries came out as the notes predicted. The one mismatch is G2-146: its test name is a placeholder (the note says so itself), and with the real name it matches too. No tracked file was written: `git status` is the same as at the start, and HEAD is 1f4ebba.

### 1. Header
- **Notes covered:**
  - design-E1.md (internal/runtime). I skipped the G10-1 addendum entries, as you said.
  - design-E2.md and design-E2-addendum.md (root package, internal/partition, io/inmemory, tools/apipartition, tools/shape).
  - design-W0.md, W0-go only (tools/archcheck, tools/astequal).
- **Eligible entries: 29.** E1 has 9, E2 has 8, the E2 addendum has 5 and W0-go has 7.
  - I made 30 mutate runs: G2-146 ran twice, once under its placeholder name and once under the real name.
  - I made 28 control runs.
- **How I ran them:** one at a time, in this order: all mutate runs, then all controls.
  - `--timeout 180s` on G2-118 and G2-132.
  - No `-race`, no load runs.
  - My driver script, the spec of every entry exactly as passed, and results.json (each run's output tail) are in (scratch) (outside the repo).
- **Toolchain:** `go` from PATH is go1.27.4 darwin/arm64, which matches go.mod's `toolchain go1.27.4`. mutate printed the same version and `head: 1f4ebba…` on every run.
- **Time:** about 17 minutes, well inside the 40-minute box. Every mutate except G2-132 took 0.8–2.4 s; G2-132 took 182.1 s.

**Skipped, and why:**
- **E1:**
  - `--patch` plants: G10-20 revert (also `--race --repeat --load`), G7a-5 revert, G2-115, G2-117 (the slowdown plant) and G2-127.
  - G10-21 revert: its `find` is code the fix has not written yet. Today, lifecycle.go:179 already reads `stream < len(a.app.Streams) {`, which is the revert's replace text. Its test `TestABatchEndOnADerivedStreamAfterTheDrainIsNotCounted` does not exist yet (`git grep` finds nothing).
  - G2-120's new_test `TestSyncEndBroadcastsAWalkWithNoAdmission` does not exist yet. I ran its old_test instead.
  - No plant to run: G2-113 and G2-129 (covered-set proof), G2-122 (grep check), G2-131 (AST script).
  - The G10-1 addendum entries (your instruction).
- **E2 and the addendum:**
  - `--patch` plants: G10-1 revert, G7a-1 revert, G7a-4 revert, G2-81 and G2-83-both.
  - `subtest` entries: G2-2-type, -unset, -time and -null; G2-84-message, -swap-hidden, -swap-phantom and -swap-moved.
  - G2-106 is a Python check, not a mutation.
  - New tests that do not exist yet (`git grep` exit 1): G2-144's `TestEmbeddedInterfaceIsRefused` and G2-156's `TestJudgeReportsEachLoweringCodeAtTheLoweringStage`. G2-156's control predicts old "invalid" because its test is absent, and that holds.
  - G2-143 is the coordinator's `--repeat 50 --load` proof. I ran it once, without load (the brief forbids load runs).
- **W0-go:**
  - Grep checks: G2-147, G2-152 and G2-155.
  - G2-151-live: its file is tools/allowlists/purity.json, not Go. The note itself says an overlay cannot plant it; it needs `--expect`'s worktree application.
  - G2-153 is a probe file in a worktree.
  - The W0-py entries are out of scope.

### 2. Mismatches
| Note | Entry | Command (short) | Predicted old | Observed | Last lines of output | Likely cause |
|---|---|---|---|---|---|---|
| E2 addendum | G2-146 | `--file tools/apipartition/main.go --find 'sig += fmt.Sprintf(" // iota %d", specIndex)' --replace '…specIndex+1)' --test TestBreakitIotaSwapChangesTheSurface` | green | **invalid** | `testing: warning: no tests to run` / `ok github.com/vorsar/cep/tools/apipartition 0.755s [no tests to run]` / `outcome: invalid` | The test name is a placeholder that does not exist (the note says so at :267). The real test around :28 is `TestBreakitIotaRepetitionKeepsTypeAndValue` (tools/apipartition/breakit_s3_3_1_test.go:26). With that name the outcome is **green**, as predicted. The JSON entry and its control need that name. |

The `find` text matched exactly once in every entry. There were no build failures and no refusals.

### 3. Stop rules
My notes have no plant stop rules. The B stop rules (G2-92, G2-107, G2-94, G2-108) belong to the other checker.

### 4. All other results
| Note | Entry | Predicted old | Observed |
|---|---|---|---|
| E1 | g2-111 (transitions.go, TestBreakitActivateRacesRemove) | green | green |
| E1 | g2-112 (lifecycle.go, TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled) | green | green |
| E1 | g2-114 (boundary.go `case nonFinite(v)`, TestSendRefusesNonFiniteFloat) | green | green |
| E1 | g2-116 (drain.go `a.rec.cut()`, TestCloseRemovesAnAppWhoseDeadlinePassedAsItsDrainEnded) | green | green |
| E1 | g2-118 (recorder.go, TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles, `--timeout 180s`) | green | green (1.5 s) |
| E1 | g2-120, old_test (sync.go, TestBroadcastCarriesNoAdmission) | green | green |
| E1 | g2-125 (boundary.go, TestKeyErrorsAndZeroKeys) | green | green |
| E1 | g2-128 (shutdown.go, TestCloseStopsAnIdleRecorderItGaveUpOn) | green | green |
| E1 | g2-132 (requests.go, TestTickRefusesATimeARecordingCannotWrite, `--timeout 180s`) | timeout | timeout (182.1 s) |
| E2 | G2-1 (sink.go, TestRowReaderNullsAndTypes) | green | green |
| E2 | G2-3 (diagnostic.go, TestDeployReportsEveryDiagnosticAtOnce) | green | green |
| E2 | G2-4-table (row.go, TestTableRowsAreCheckedAsTableWriteChecksThem) | green | green |
| E2 | G2-4-deploy (row.go, TestWithTableRowsAreVisibleToTheFirstEvent) | green | green |
| E2 | G2-82 (internal/partition/partition.go, TestNewRuleRefuses) | green | green |
| E2 | G2-141 (io/inmemory/source.go, TestBreakitOpenTwice) | green | green |
| E2 | G2-144, old_test (tools/apipartition/main.go, TestEmbeddedFieldIsRefused) | green | green |
| E2 and addendum | G2-156, old_test (tools/shape/main.go, TestShapeCorpusGolden) | green | green |
| E2 addendum | G2-83-prefix (partition.go, TestMultiPartKeysAreOneInstancePerTuple) | **red** | red |
| E2 addendum | G2-83-negzero (partition.go, same test) | **red** | red |
| E2 addendum | G2-143, one run without load (source.go, TestCviSourceRefusalsAndContexts) | k of 50, not predicted ("k < 50") | green on its one run. This fits k < 50 but is not the 50-run proof. |
| E2 addendum | G2-145 (apipartition main.go `os.WriteFile(golden…)`, TestEmbeddedFieldIsRefused) | green | green |
| E2 addendum | G2-146 with the real name TestBreakitIotaRepetitionKeepsTypeAndValue | green | green |
| W0 | G2-149-clean (archcheck main.go, TestCLIWriteFailureIsExit2) | green | green |
| W0 | G2-149-finding (archcheck main.go, TestCLIWriteFailureIsExit2) | green | green |
| W0 | G2-150 (archcheck main.go "channel send" → "channel type", TestPurity) | green | green |
| W0 | G2-151-map (archcheck main.go, TestListsPurity) | green | green |
| W0 | G2-154-usage (astequal main.go, usage text → "", TestRunExitCodes) | green | green |
| W0 | G2-154-git (astequal main.go, `readSides` error drops `err`, TestRunExitCodes) | green | green |
| W0 | G2-154-parse (astequal main.go, `compareSources` error drops `err`, TestRunExitCodes) | green | green |

For G2-154-git and -parse, the note says the replace is "the same ending in `fmt.Fprintln(stderr, \"astequal:\")`". I built it as the find text with `, err` removed from the last call.

### 5. Controls
I ran each as `go test -count=1 -run '^<Test>$' ./<pkg>`. Every one exited 0 and printed `ok`; none said `[no tests to run]`.

| Test | pkg | pass/fail |
|---|---|---|
| TestBreakitActivateRacesRemove | internal/runtime | pass |
| TestBreakitAddAfterAClosedDeadlineIsNotLeftInstalled | internal/runtime | pass |
| TestSendRefusesNonFiniteFloat | internal/runtime | pass |
| TestCloseRemovesAnAppWhoseDeadlinePassedAsItsDrainEnded | internal/runtime | pass |
| TestAnOverflowedRecordingWritesItsQueuedRecordsThenSettles | internal/runtime | pass |
| TestBroadcastCarriesNoAdmission | internal/runtime | pass |
| TestKeyErrorsAndZeroKeys (G2-115 and G2-125 controls) | internal/runtime | pass |
| TestNoSleepOrPollingInRuntime (G2-127 control) | internal/runtime | pass |
| TestCloseStopsAnIdleRecorderItGaveUpOn | internal/runtime | pass |
| TestTickRefusesATimeARecordingCannotWrite | internal/runtime | pass |
| TestRowReaderNullsAndTypes | . | pass |
| TestSendRowChecks (G2-2 control) | . | pass |
| TestDeployReportsEveryDiagnosticAtOnce | . | pass |
| TestTableRowsAreCheckedAsTableWriteChecksThem | . | pass |
| TestWithTableRowsAreVisibleToTheFirstEvent | . | pass |
| TestPanicInOnPurgeStillPurges (G2-81 control) | internal/partition | pass |
| TestNewRuleRefuses | internal/partition | pass |
| TestMultiPartKeysAreOneInstancePerTuple (G2-83 control) | internal/partition | pass |
| TestCheckTimers_PlantedFaults (G2-84 control) | internal/partition | pass |
| TestBreakitOpenTwice | io/inmemory | pass |
| TestCviSourceRefusalsAndContexts | io/inmemory | pass |
| TestEmbeddedFieldIsRefused (G2-144 old_test, G2-145 control) | tools/apipartition | pass |
| TestBreakitIotaRepetitionKeepsTypeAndValue (G2-146 control, real name) | tools/apipartition | pass |
| TestShapeCorpusGolden | tools/shape | pass |
| TestCLIWriteFailureIsExit2 (G2-149 and G2-152 control) | tools/archcheck | pass |
| TestPurity | tools/archcheck | pass |
| TestListsPurity | tools/archcheck | pass |
| TestRunExitCodes | tools/astequal | pass |

These new tests do not exist yet, so I did not run them; `git grep` finds no definition: TestEmbeddedInterfaceIsRefused, TestJudgeReportsEachLoweringCodeAtTheLoweringStage and TestSyncEndBroadcastsAWalkWithNoAdmission.

**Not reached:** nothing inside the brief's scope.

I did not use GitNexus: the MCP resource reader was not available to me, and this job was measurement only. I did not search Hindsight memory: I raised no design-choice findings.