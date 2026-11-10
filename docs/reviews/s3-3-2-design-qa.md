# S3.3-2 design v1 + the panel's fixes — testability review (the coordinator's, at the owner's direction)

**This sprint:** [design](../sprints/s3-3-2-design.md) · [record](../sprints/s3-3-2-record.md) · reviews: [design: breaker](s3-3-2-design-breaker.md) · [design: challenger](s3-3-2-design-challenger.md) · [design: architecture](s3-3-2-design-architecture.md) · **design: QA (testability)**

**Who and why:** the QA testability agent was stopped by the owner before it wrote its report; the owner directed the coordinator to do this review ("you do it", 2026-11-09). The coordinator wrote the design, so this departs from ground rule 14 (a reviewer never reviews their own work) by the owner's direction; the record says so. The second QA lane, writing these tests red, is the independent check on this table.

**Reviewed:** design v1 (0a3114c) with the fixes proposed by the breaker (B1–B2, S1–S7, L1–L3), the challenger (C1–C6) and the architect (A1–A12), taken as adopted unless the owner rules otherwise; QA lane 1 (`s3-3-2/qa`: 9aed986, 054d7db, bb4aef4).

**Counts:** 52 rows; 5 gaps (each with its resolution); 4 lane-1 amendments; 2 findings (should-fix).

## 1. The test table

Red-first key: **today** = red against today's code; **stub** = red against the step-1 stubs; **mut** = a one-line mutation of the named line, shown red once (sprint model §10).

### §2 positions (`internal/parser`)
| id | rule | test | assertion | red first | row |
|---|---|---|---|---|---|
| T1 | every expect position (§2) | `TestExpectPositions` | 07 block 1: `Expect.Pos`, `Given.Pos/NamePos`, `Then.Pos/NamePos`, `Record.Pos`, `Field.Pos/ValuePos` (incl. a `null` and a `-` value) at their line:col | today (zero) | unit |
| T2 | positions skip the canonical form | the existing parser goldens | unchanged | mut: drop the `Pos` skip in `MarshalCanonical` → goldens red | unit |

### §3 E-EXP-002 (`internal/validate/testdata/e-exp-002-*.cep`, the `TestStatic` harness; red: today, no finding)
| id | rule (finding) | program(s) | position |
|---|---|---|---|
| T3 | X1 unknown stream / table | lane 1: `given-unknown-stream`, `then-unknown-stream`, `given-table-unknown-table` | the name |
| T4 | X2 unknown field / column | lane 1: `given-unknown-field`, `then-unknown-field`, `given-table-unknown-column` | the field |
| T5 | X3 wrong type | lane 1: `given-wrong-type`, `then-wrong-type`, `given-table-wrong-type` | the value |
| T6 | X4 null for a non-optional field, written / omitted | `given-null-required`, `given-omitted-required` | value / the record's `{` |
| T7 | X4 the time attribute, even `optional` (B1) | `given-null-time-optional`, `given-omitted-time` | value / `{` |
| T8 | X4 a table column, incl. the key | `given-table-null-key`, `given-table-omitted-column` | value / `{` |
| T9 | X5 `null` in a then-record for a non-optional output attribute | `then-null-required` | the value |
| T10 | X6 a field named twice | `given-duplicate-field` | the second field |
| T11 | X7 a stream/table the rule does not use; `then` not the target | `given-unread-stream`, `given-table-unread`, `then-not-target` | the name |
| T12 | X8 `given` on a `time arrival` stream | `given-arrival-stream` | the name |
| T13 | X9 a stream given twice (A6; SPEC §6.8 "one per input stream") | `given-stream-twice` | the second name |
| T14 | X10 a given record names `text_invisibles` (C6, S1, A12) | `given-text-invisibles` | the field |
| T15 | X4's exception: `text_invisibles` omitted is fine | `TestOmittedTextInvisiblesIsAccepted` (validate unit test: Check finds nothing) | — (mut: drop the exception → red) |
| T16 | each kind's message text | `TestExpectMistakeMessages` (table test over T3–T14: rule, block ordinal, field/type named) | stub |
| T17 | E-EXP-002 alone, before any block | lane 1 `TestExpectMistakeIsReportedAloneBeforeAnyBlockRuns` | today |
| T18 | E-EXP-001's `Deploy` flag in the catalogue | `TestEveryStaticCodeIsAccountedFor` amended | today (lane 1 makes it red) |
| T19 | errors.md ↔ catalogue | the existing docsync (b) test | mut: catalogue entry without errors.md |

### §4 the plan (`internal/plan`)
| T20 | blocks as values (§4) | `TestBuildCarriesExpects` | 07: tables, lists, then attrs by index; an int literal for a float field widened; omitted/`null` typed nulls; `Ts` from the time attribute | stub |
| T21 | blocks on `plan.App`, not on the rule (A10) | `TestFromPlanCarriesNoBlocks` (reflect: `runtime.App` reaches no `plan.Expect`) | mut: add the field to `Rule` |

### §5 the sandbox (`internal/playback/expect_test.go`)
| T22 | MUST 1 fresh per block | `TestBlocksDoNotShareState` (+ lane 1 facade) | stub |
| T23 | MUST 2 the rule alone | `TestOtherRulesDoNotRun` (second rule's filter is an expression error: not counted; one rule in Stats) | mut: run the whole app |
| T24 | MUST 3 tables first | lane 1 `TestExpectGivenTableSeedsBeforeAnyEvent` | today |
| T25 | MUST 4 merge by time, ties to the first list | `TestListsInterleaveByTime` (synthetic two-input `plan.App`, three ties) + lane 1 07 block 6 | stub |
| T26 | MUST 5 end at `+∞` | `TestBlockEndsAtPlaybackEnd` (the end step's advance reaches `math.MaxInt64`) | mut: `End` → `Drain` |
| T27 | MUST 7 determinism | `TestRunExpectsIsDeterministic` (07, 09, 10 twice, deep-equal) | — (guard; mut: shuffle merge ties) |
| T28 | a panic fails the block (A2) | `TestPanicFailsADefeatPath` (hand-built plan, body panics on its event, then-list empty → divergence "panicked") | stub |
| T29 | counts name the drop (A2, S2) | `TestMissingOutputNamesTheDrop` (10 block 4 expecting one alert → "key error"; 07 with a stream-late event → "stream-late"; an overflow → "expression error") | stub |
| T30 | a fault is an error, not a panic (A4) | `TestSandboxFaultIsAnError` (hand-built plan, null key row → error, no panic) | stub |
| T31 | sandbox weight (C5, A10) | `TestRunExpectsBytes` (`testing.Benchmark` `AllocedBytesPerOp` on 07 < 200 KB) | today (2.9 MB) |
| T32 | −0.0 ≠ 0.0 (L1) | `TestNegativeZeroIsNotZero` | mut: drop the signbit check |
| T33 | the six divergence texts (C2) | `TestDivergenceTexts` (wrong value, missing, extra, another order, under empty then, wrong ts — each its own text) | stub |
| T34 | alignment beats position (C2) | `TestAlignedReportOnRateLimitTraps` (10 without its rate limit: blocks 3 and 5 read "was not expected") | mut: positional walk |
| T35 | position of E-EXP-001 (Q3, owner) | in T33: `Line, Col` = the owner's choice | stub |

### §6 the facade (module root)
| T36 | six planted wrong expects; one per failing block, in order | lane 1 | today |
| T37 | params in the blocks (exit 9) | lane 1 | today |
| T38 | nothing live (MUST 6) | lane 1 (guards) | mut: run the stage after `bind` → T39 red |
| T39 | a refused deploy calls no factory | `TestRefusedDeployCallsNoFactory` (a counting `ConnectorFactory`, an annotated stream, a failing block → 0 calls) | mut: stage after bind |
| T40 | ctx checked before bind (A9) | `TestCancelledDeployCallsNoFactory` | today |
| T41 | not built is public (Q1) | `TestNotBuiltIsPublic` (01 → `errors.Is(err, <the owner's choice>)`) | today |

### §7 the runner (`conformance/`), its row and proofs
| T42 | 07, 09, 10 deploy; listed files fail with their construct (C3, A5) | `TestCorpus` | stub (the construct check: mut a label) |
| T43 | annotated files deploy through an in-memory factory (A5) | `TestCorpus` once 01 builds; now: `TestCorpusFactoryBindsEveryAnnotation` (a plain-rule program with `@source`/`@sink` deploys) | today |
| T44 | the runner cannot pass empty (S6) | `TestCorpus` requires `wantFiles = 12` | mut: point it at an empty dir |
| T45 | the six planted proofs + a clean control (exit 2) | `gate/proofs/drivers/test_expect_planted.py` | today (row red only after the build; the drivers assert it) |
| T46 | the empty-corpus proof (S6) | same driver, `test_empty_corpus_is_red` | — |

### §8 Tier 2
| T47 | the baseline holds; 15 blocks run (S6) | `TestBaseline` (`internal/playback`) asserts the 15 blocks and the bytes | stub |
| T48 | both OSes (A8) | `determinism` and `determinism-macos` rows run `TestBaseline` | registry change; proof: the rows' `covered` |
| T49 | the classifier, one pytest per class | `gate/baseline/tests/test_regen.py`: additive-new-block, block-removed with the file changed, a gone key with the file unchanged (refused), count-change, ordering-change, timestamp-drift, spec-rev named (recorded), spec-rev unnamed (refused as value-change), value-change, format-change, Tier 1 red (refused), no change (exit 0), equal-givens `#2` keys | stub |
| T50 | the regen end to end (exit 6) | `gate/proofs/drivers/test_regen_baseline.py`: count-change (10's block extended), ordering-change (10 block 3 swapped, S7), timestamp-drift (a 1 ms shift), additive (a new rule) — refused ones leave the baseline byte-identical | stub |

### §9 the known issues
| T51 | `Close` refuses a `Deploy` and a `Start` once it has begun; returns at its deadline behind a blocked sink (B2, A1); drains an app activated before the barrier (A3a); waits for a pending install (A3b, S3); joins a non-ctx `Remove` error | lane 1 `TestCloseRefusesADeployOnceItHasBegun`; `TestCloseReturnsAtDeadlineBehindABlockedSink`; `TestCloseDrainsAnAppActivatedBeforeTheBarrier` (`-race`, `GOMAXPROCS=1`, 200 iterations); `TestCloseWaitsForAPendingInstall`; `TestCloseReportsARemoveError` | today / mut: barrier awaited through `requestQueued` |
| T52 | `apipartition` `<Type>.<Method>`, generics as `G.Get` (no type parameters); the rewrite is mechanical (A11) | lane 1's two tests; `TestGoldenRewriteIsFormatOnly` (old entries mapped through the new rule equal `--write`'s, both goldens) | today |

## 2. Gaps (and their resolution)
1. **`wantBlocks` is not observable through the public API** (S6): `Deploy` reports no block count, and the runner may import only `cep` and `io`. → the 15-block count lives in `TestBaseline` (T47), which runs every block; the runner keeps `wantFiles`.
2. **A pending install cannot be held from outside `internal/runtime`** (A3b): → `TestCloseWaitsForAPendingInstall` is an internal test in package `runtime` that parks one shard's install ack (an unbuffered test hook on the shard's mailbox, as the existing saturation tests hold a sink).
3. **The not-built expiry needs `gate/current-sprint`**, which S1.2's plan named but S1.1c now owns (it does not exist): → forward the T4 expiry to S1.1c (Q-E); the labels and the construct check are tested now (T42).
4. **E-EXP-001's position** is the owner's (Q3): T33/T35 and lane 1's position asserts follow the answer.
5. **A vacuous block** (`given S [ ] then Out [ ]`, L2) has no rule to test until the owner places it.

## 3. Amendments to lane 1
1. If Q3 moves E-EXP-001 to the diverging then-record: `exRefusedDeploy`'s position check (line 126) and every `exWant` take the then-record's (or `then`'s) line:col instead of `expect`'s.
2. `TestCloseRefusesADeployOnceItHasBegun` gains a `Start` case, deterministic once `Activate` checks `closing` caller-side (S3).
3. The wrong-order case asserts "another order" and the wrong-timestamp case "has no output … ts" (C2) once texts are fixed; today the tests assert only the code, position, rule and attribute — keep that, add the text in T33 at the playback level.
4. None of lane 1's E-EXP-002 programs change (X1–X3 stand); `TestExpectOmittedGivenAttributeIsNull` omits `appId`, declared optional in 07, so X4 keeps it valid.

## 4. Findings
- **Q-1 (should-fix):** X7's "a table the rule's expressions do not read" needs a precise definition to be testable: a table is read when a filter or select expression names it as an `in` operand or a keyword-set operand. Write that into §3.
- **Q-2 (should-fix):** the regen tool's "the corpus file changed in the same diff" (A7's `block-removed`) needs a defined base: compare the corpus file's bytes with the bytes recorded beside the baseline (a `source_sha256` in the header), not git — so the classifier is testable on synthetic files and needs no git state.
