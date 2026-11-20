# S1.1c-7 design — the coordination record

*The coordinator's log while the nine design notes were written, reviewed and pre-checked (2026-11-20): cross-note rulings, spot checks of each note's claims against the code, the review dispositions, lane A1's one commit list, and the G1 preparation. The notes are docs/sprints/s1-1c-7-design-*.md.*


## Cross-note items, rulings and spot checks

## S1.1c-7 design notes — coordinator items raised by the notes (to settle before G1)

| # | From | Item | Proposed handling |
|---|---|---|---|
| 1 | A2 (G2-62) | The ledger needs "carried in full to S1.1c-10" as well as "… to S1.1c-8" (a finding whose only fix is a split or other later non-simple change). | Generalise the outcome to "carried in full to <sprint>"; check_ledger v2 accepts S1.1c-8 or S1.1c-10; plan §7.5 annotated. |
| 2 | A2 (G2-15) | A plant on `gate/current-sprint` is committed inside mutate's throwaway worktree; that commit must not run the repo's hooks (core.hooksPath is shared). | W0's `--py` commits the plant with hooks disabled for that throwaway commit only (`git -c core.hooksPath=<empty dir> commit`); a counter-example asserts no gate run. Not a project commit, so ground rule "never --no-verify" is not engaged; say so in W0's note review. |
| 3 | A2 (G10-5) | K1c with a hold is deterministic: k/N is 1/1 unless `--py` supports `--repeat`. | Accept 1/1 for a hold-driven logic race (plan §7.1 says "red first through a hold"); `--repeat` for `--py` not required. |
| 4 | A2 | Commit count 9 vs plan line 8 (ceiling 10). | Within ceiling; running total tracked. |

Spot checks of A2's claims (coordinator, 2026-11-20): G10-5 check-then-start at gitnexus_reindex.py:122/:128 ✓; G2-15 :189 writes "S4.1\n" ✓; G2-8's twin test_f1_feeds.py:66-68 asserts the real text ✓; G2-13 direct run 15 vs module 20 ✓; G2-9 fixture order go 3, golangci-lint 4 ✓.
| 5 | B (G2-106) | G2-106 spans playback (B) and partition (E2: ttl_test.go:122, panic_test.go:123/173/204). | Routed: the partition part added to E2's note (message sent to s17-design-e2, 2026-11-20). |
| 6 | B (G2-94 twin) | internal/partition/helpers_test.go `world.advance` ignores body panics like playback's ldWorld.step; not a survey finding. | E2 lists it as carried (new finding, later part) unless a fix in its chunk touches it. |
| 7 | B (G2-89) | internal/model/model.go:199 comment wrong for window bodies; CD's file; not a survey finding. | Sent to CD1's brief: K6 in CD1 only if a CD fix makes it false, else carried to the later comments work. |
| 8 | B | Stop rules: G2-92 (old may read red), G2-107 (control may read red), G2-94/G2-108 (red control = real defect). | The lane runs those plants first; a stop comes back to the coordinator. Keep in B's brief. |
| 9 | B (G10-19) | The note's first chain plant (Unwrap → nil) cannot fail `errors.As` (As stops at inPlayback); the corrected plant is `fault("%v", inPlayback{err})`. | Build uses the corrected plant only. |
| 10 | B (G2-90) | The plan listed G2-90 as undecided; the note shows SPEC rev 4 decides E-PART-004 (SPEC.md:283, validate bounds.go:125). | Reviewers to confirm; not a choice. |
| 11 | B | Line drift: G2-96's row is reader_test.go:96 (note says :99). | Builders locate by text, never by line number alone. |

Spot checks of B (coordinator): G10-18 records.go:127 fires when the key IS present ✓; G2-96 the `strings.HasPrefix(c.name, "payload")` exemption at reader_test.go:122 and the row decode:false at :96 ✓ (line drift); G2-98 twTracked lists partial-count, no check("partial-count") ✓; G2-103 breakit2_test.go:80 compares err.Error() to "EOF" ✓; Trace has no late counter ✓.

| # | From | Item | Proposed handling |
|---|---|---|---|
| 12 | A1a + A1b (G3-72) | G3-72 designed twice: it is in chunk A1b, but the coordinator's prompt to A1a said it was A1a's (coordinator error). Both agree byte-for-byte on the new FOREIGN/LINE texts and the coupling commit with test_suppressions_planted.py:42-43; A1b also names test_breakit_s11c2.py:359. | A1b's design is canonical; A1a's G3-72 section and its commit C2 are superseded. A1a's commit count drops by one. |
| 13 | E1 (G10-21) | The counting rule (a drained BatchEnd counted, a drained Send not) is decided by design s3-3-1 MUST 11 and T30 (owner, design 6); E1 chose (C): keep it, fix the comments, stop queuing a DERIVED stream's drained BatchEnd (a correctness defect: the recording becomes unreadable, records.go:277). E1 recommends (B), stop queuing any drained BatchEnd, as known issue S3.3-1-3's fix (forward S8), which needs the owner. | G1: tell the owner; no decision needed now unless the owner wants S3.3-1-3 pulled in (it would touch E2's root files and the coordinator's known-issues). |
| 14 | E1 (G10-20) | The fix closes the data race; a logic window remains (a Drain whose quiet item failed can put the phase back to active after another Drain passed markDraining; the `draining` flag still refuses input, so no input is lost). New, not a survey finding. | G1: propose a known issue (correctness: no) with a forward, or the backlog (ground rule 7); owner decides. |
| 15 | E1 (G2-127) | internal/runtime/nosleep_test.go is a law path: the lane's edit shows in LAW Δ. No driver or case count changes. | Lane commit (C3); no coordinator commit; the proofs row is not triggered (copy_gate copies gate/ only). |
| 16 | E1 (G2-115) | breakit_test.go:388 may have the same 16-shard blind spot (unchecked). | The lane checks at build; if so, the same Shards: 3 change in C3 under G2-115. |
| 17 | E1 | 6 commits (5 lane + 1 K2-del) vs plan line 4 + pool. | Within the running total; recount at G1. |
| 18 | A1a/A1b | Rejections verified by the coordinator: G2-19 (smells.py:237-239 reds a bad target shape, so planting into the real file adds nothing) ✓; G2-50 (unittest's testPartExecutor bare `except:` records any exception as an error, so "no exception" is a real check) ✓; G2-54 (a renamed seam fails loudly; the coupling concern goes to the later placement work) ✓. | Ledger: rejected, with these reasons; G2-54 also forwards the seam-coupling note to S1.1c-10. |

Spot checks of E1 (coordinator): G10-20 drain.go:28 and :80 read `waited` unlocked, :119 writes under runMu ✓; G10-21 lifecycle.go:179 drained branch has no Derived check, records.go:277 refuses a derived stream's record ✓; G2-113 build.go:158-160 refuses duplicate inputs ✓; G2-131 third site breakit_s3_3_2_test.go:32 ✓; G2-114 rtSchema appends a ts attribute (helpers_test.go:75) ✓.
| 19 | W0 | Item 2 (hooks in the throwaway worktree) is designed: `-c core.hooksPath=/dev/null` on `worktree add` and `commit` (design-W0 Part A step 3/5, risk iii). | Resolved by the note; reviewers check. |
| 20 | W0 | 8 commits: W0-1 `--patch`/`--py`, W0-2 `--subtest`, W0-3 `--expect`, W0-4 bench K1s, W0-5 bench K2s, W0-6 naming tool, W0-7 archcheck K2s, W0-8 astequal K2s. W0-go runs its proofs only after W0-3 lands (its proofs are `--expect` entries) and designates top-level tests. | Matches plan §1 (tools 4 + lanes 4). |
| 21 | W0 | Two survey proposals rejected inside accepted findings: G2-6's exact `math.comb` value (assert the printed p ≥ 0.3333 instead) and G2-147's runCLI swap (CommandContext with a ceiling instead). | Reviewers check the reasons. |

Spot checks of W0 (coordinator): G10-3 `except BaseException` in mutate.py:296 and load.py:149 catches argparse's SystemExit(0) ✓; G2-154 rows "bad revision"/"no arguments"/"parse error" have out "" and the check is `strings.Contains(stdout, tc.out)` (astequal_test.go:277-279, :295) ✓; G2-155 gitIn uses exec.Command with no context (astequal_test.go:219-222) ✓.
| 22 | E2 (G10-1) | Choice B: starting rows travel in `runtime.App.Loads` and the router applies them in Activate's step; the runtime half (K1h) is E1's, W2, before E2's W3. | E1 resumed (2026-11-20) to add the seam as a new commit; E2-1 depends on it. Risk: all starting rows' records must fit RecordQueue together. |
| 23 | E2 | Six open items (G2-83, -84, -143, -145, -146, -156 plant/input text). | E2 resumed for a completion pass. G2-143 proof decided: old k/50 under the plant via `mutate.py --repeat 50 --load` (k recorded), new 50/50 red, blocked branch covered in two new runs. |
| 24 | E2 (G7a-2) | Widened to all rendered public godoc in `.`, `./io`, `./io/inmemory` (49 comment lines swept). | Reviewers check the widening stays K6 (comments only) and within G7a-2's reason. |
| 25 | E2 | G10-23 carried in full to S1.1c-10 (print's error path unreachable without a seam). | Ledger: carried in full to S1.1c-10. |

Spot checks of E2 (coordinator): G10-1 apphandle.go:44-51 Activate then a TableWrite loop returning on the first error ✓; G7a-1 connectors.go:22-24 connectorSprint maps to S5.1/S5.2 ✓; G2-3 deploy_test.go:68's second Contains makes the count unchecked ✓; G2-82 errNoKeyTypes used only at partition.go:223/236 ✓; G2-81 partition.go:354-355 Purged++ before onPurge ✓.
| 26 | E1 addendum (G10-1 runtime half) | Option B holds only with Condition A (Add checks starting rows with the table's own check: a new exported `func (t *Table) Check(rows [][]event.Value) error` in internal/table, CD's file) and Condition B (build refuses an app with Loads). E1 adds C7 (K1h), at its ceiling of 6 lane commits. | **Ruling (coordinator):** `table.Check` is its own small commit (test `TestCheckRefusesWhatUpsertRefusesAndChangesNothing`, revert plant `return nil` first), landing BEFORE W2, by a fresh go-engineer during W1, owning internal/table/table.go for W1 only (CD owns it in W3): one owner per file per wave holds. It uses one coupling-commit slot of the budget. |
| 27 | E1 addendum | Facade behaviour: a bad starting row now fails at Deploy (Add) instead of at Start; matches design s3-3-1 §3.4's intent ("a row TableWrite would refuse fails the deploy, not the start"). The facade's CheckRows at deploy.go:221 stays as an earlier refusal with the facade's wording. | G1: tell the owner (public behaviour, decided by the design); reviewers check. |
| 28 | E2 addendum | Six items completed; G2-84: CheckTimers has two messages (partition.go:476, :485), the four cases' exact texts computed; G2-146's line `p B const const B T = iota // iota 1`; G2-156's inputs from the parser's own TestLoweringCodes (no conformance example exists for the four lowering codes). G2-146's test name is a placeholder (enclosing function not read). | Build fills the name; controls check every computed text (a red control stops the lane). |
| 29 | Q11 | golangci-lint v2.15.0 is still the latest release (gh release list, 2026-11-20: v2.15.0 2026-10-16); the 15 vulns.json TEMP entries expire 2026-12-14. | Re-check in early December; if still none, the owner renews by decision before 2026-12-14. |

Spot checks of E2's addendum (coordinator): CheckTimers' two messages at partition.go:476 and :485 ✓; the iota marker at apipartition/main.go:270 and entry.line at :36 ✓; parser.LoweringCodes at errors.go:59 ✓.
| 30 | CD1 (G2-134) | G2-134 spans internal/text (CD1) and internal/plan (CD2): `TestBuildCarriesExpects`. | Routed: message sent to s17-design-cd2 to add the plan half to its plan K2 commit; same lane CD at build. |
| 31 | CD1 | model.go:199's wrong comment is carried to the later comments work (no CD fix makes it false). | Ledger n/a (not a survey finding); listed in the record's carried items. |
| 32 | CD1 | Rejections verified by the coordinator: G2-77 (a listed document with no pin compares 0 = 0 at fragment_test.go:1164, so `got.ruleLike == 0` at :1172 is its only guard; fragmentDocs at :1101, pins at :1143) ✓; G2-80 (Parse's doc at parse.go:58 promises a *Error; the type assertion at vsc_known_issues_test.go:188 pins exactly that) ✓. | Ledger: rejected, with these reasons. |
| 33 | CD1 | 7 commits (plus the plan half of G2-134 in CD2's plan commit). | Recount with CD2 at G1 (lane CD's line 14, ceiling 16). |

Spot check of CD1 (coordinator): G2-71 invisiblesKeys holds `event.InvisiblesAttr, "text_invisibles", "text_invisibles"` with no escapes (gen_test.go:105) ✓.
| 34 | CD2 (G10-17) | New user text "a literal of kind %s is not of type %s" replaces "a %s literal is not a %s" (expect.go:239); the note counts 72 pins + 1 doc line; the coordinator's `git grep -c` counts 73 test lines holding "literal is not a". | Build reconciles the exact pin list against `git grep` before the commit; the dry run lists every file. |
| 35 | CD2 (G7a-3) | New `what` texts "a text operator", "count()", "chain()"; notbuilt.json:17 and test_notbuilt.py:20 change in the same coordinator coupling commit. | Coordinator commit during lane CD (W3); `--job proofs` after (notbuilt.json is copied by a T4 proof). |
| 36 | CD2 | 9 commits; lane CD total 16 (CD1 7 + CD2 9) = its ceiling. G2-134's plan half arrives as an addendum (CD2 resumed with the late message). | At the ceiling: no spare for a fix round in lane CD; running total re-projected at G1. |

Spot checks of CD2 (coordinator): G10-17 text at internal/plan/expect.go:239 ✓ (73 test lines pin it); G10-22 three unexported sentinels at internal/table/table.go:50-54 ✓; G2-137's stronger duplicate H3 (`count() > 'a'`) at hiding_test.go:61 ✓.
| 37 | W0 rev1 | A.1's `GO_SPACES` literal arrived mangled by the agent→coordinator transport (raw U+0085/U+00A0/U+1680/U+2028… and line breaks), the same failure as DQ-B1. | W0-py's brief: implement GO_SPACES from Go's `unicode.IsSpace` definition (\t \n \v \f \r space, U+0085, U+00A0, and Unicode category Zs plus U+2028/U+2029), never from the note's literal; prove it with a fixture test whose subtest names hold U+00A0 and U+2003 and read Go's own rewritten names; write such characters in source as escapes. Every brief: escape-bearing text uses the ⧵ notation. |
| 38 | B rev1 | B-K1's plain batch (three entries, red), "Not parallel:" marker with the toolchain's AllocsPerRun panic reason, G2-108's load run over the 22 q2Start tests (GitNexus: q2Start HIGH risk, 25 symbols), per-entry timeouts for TestLiveEqualsReplay (300s), load runs with `-timeout 50m`. | Accepted; checked: encode_test.go has 18 tests (17 + the AllocsPerRun one) ✓. |
| 39 | E1 rev1 | DA-S1: recordQueueFor = cfg + Σrows (headroom cfg−1; unchanged for loadless apps, so every queue-full test keeps its bound); test "two tables beyond RecordQueue record in full" + plant g10-1-size-revert. DA-S7: new BenchmarkTableWritePath (not frozen) in C7, timed parent-with-overlay vs commit. G10-21's test now covers Stop→drain item (Batches 0 on the derived stream = no rBatchEnd). K2-del subset proofs; per-entry timeouts 180; G2-131 load names all three tests. 7 commits (6 lane = ceiling + 1 K2-del). | Accepted; checked: igRec at router_intake_internal_test.go:268, cgCtx/cgApp take testing.TB (close_gate_test.go:23, :49), RecordQueue default at build.go:72-73 ✓. G1: owner informed of the queue sizing. |
| 40 | Pre-check 2 (E1, E2 + addendum, W0-go) | 29 eligible Go entries run against today's tree (go1.27.4): 28 as predicted; G2-146's placeholder test name → real `TestBreakitIotaRepetitionKeepsTypeAndValue` (green as predicted). All 28 controls pass; every find exact-once; no build failure. G2-132 old reads timeout at 182 s (as predicted). | E2 told the real name (revision running). Evidence: s17/precheck-2.md. |
| 41 | CD1 rev1 (DQ-B1) | G2-69..72 re-emitted with ⧵ = 0x5C, hex bytes, code points, and builder byte checks (bytes([92,117]) counts per line, max-byte < 128 checks). Old-file baselines verified by the coordinator: decode_bench_test.go total 4, :142 0 (raw 😀 f0 9f 98 80); decode_test.go total 11, the six rows 0; gen_test.go total 29, :27 0, :95 5, :96 3, :97 8, :105 0 ✓. CD1 flags its first note's other backslash-bearing texts (batch JSON \t \n \" in G10-16, G2-65, G2-68; G2-78/79 regexes; G2-79's header text) as possibly decoded. | Pre-check 1 tests those finds empirically (an "invalid" = a lost backslash); any hit goes back to CD1 with the ⧵ notation. The CD lane brief carries the byte-check steps verbatim. |
| 42 | E2 rev1 (DA-S3) | E2 shows DA-S3's trigger unreachable: an optional key column is refused by the validator (E-TBL-002, validate/names.go:72-83; fixtures e-tbl-002-optional-key.cep, e-tbl-002-object-key.cep) and SetNull on a non-optional attribute by the RowWriter (row.go:90-91); every other table refusal is already a Deploy refusal (RowWriter, CheckRows). Choice (iii): no facade table.Check call; a new test pins Deploy's facade wording for a null key, with a plant that goes red if runtime wording becomes reachable. Facade-only candidate F added and rejected with reasons (F leaves the concurrent-Send window open and makes Start ignore its ctx). | Accepted: the coordinator verified names.go:72-83 and row.go:90-91 ✓. `table.Check` (Condition A) stays as the runtime-level guard: reachable from runtime tests (TestAddRefusesStartingRowsItCouldNotApply), not from Deploy. DA-S3 resolved by (iii). G1: owner sees B's full cost. |
| 43 | Pre-check 1 (B, CD1, CD2) | 30 old-side runs and 23 controls as predicted; extra checks found B G2-92's plant ineffective (build.go off the live path) → replacement transitions.go plant tested (old green, new red); B G2-107's stop rule FIRED (0 ConstructLate over 200 seeds) → coordinator's choice: narrow the header's claim, carry the generator change to S1.1c-10, ledger "fixed in part"; CD1 coverage-block line numbers wrong (first statement: unescape.go:72/:77, decode.go:276/:114) — a literal reading would fire G2-72's stop rule falsely; CD1 G2-79 regex needs digits; CD2 G2-136 grep line 64; CD2 G2-87 line :60. | Corrections appended to design-B-rev1.md, design-CD1-rev1.md, design-CD2-addendum.md. No CD1 backslash lost outside G2-69..72 (every JSON parses, finds exact-once). |

## Review dispositions

## S1.1c-7 design reviews — dispositions (coordinator)

### Architecture review, round 1 (DA-; CD1/CD2 not yet reviewed — the reviewer was resumed for them)

| Id | Short | Checked | Disposition | Note to revise |
|---|---|---|---|---|
| DA-S1 | Starting rows' records must fit RecordQueue together, pinned as a failure; a bounded fix exists | app.go:75 sizes the queue once per app from cfg ✓ | **Accepted (design-note choice, Q3):** size this app's queue at Add as `max(cfg.RecordQueue, 1 + Σ len(rows) + headroom)`; the "one over fails" test becomes "two tables whose rows together exceed RecordQueue record in full"; docs reworded. G1: owner informed. | E1 addendum |
| DA-S2 | A third agent in W1 breaks rule 17 | — | **Superseded:** owner, 2026-11-20, up to 6 agents for all S1.1 work. `table.Check` keeps its own commit before W2. | — |
| DA-S3 | E2's facade has no chosen text/test/dry run for the new Deploy refusal (a row the table refuses now fails at Add) | deploy.go:109-113 maps Add's error; facade checks at :212, :221 lack the table's key-null rule ✓ (reviewer) | **Accepted:** E2 chooses the text (recommended: call `table.Check` in the facade with the facade's own wording before Add, so the text stays `cep: deploy X: WithTableRows("T"): …`), adds the null-optional-key test, dry-runs the pins, corrects its :78 claim. | E2 |
| DA-S4 | mutate.py's worktree self-check compares the whole repository's list (sibling lanes' worktrees) | — | **Accepted:** assert only this run's own path is gone and its tmp removed; the whole-list check only in counter-example #16's isolated repo. | W0 |
| DA-S5 | B's K1 revert plants as `--expect` entries at the lane base, where the find text does not yet exist | — | **Accepted:** plain K1 batch at the B-K1 commit, expecting red (as W0 and the other notes do). | B |
| DA-S6 | "Runs serially:" / "It runs serially" duplicate the repo's existing "Not parallel: <reason>" marker | 12 files use "Not parallel:" ✓ | **Accepted:** every note uses `Not parallel: <reason>` and one shared check pattern. | B, E2, CD1, CD2 (+addendum) |
| DA-S7 | K1h timing measures no edited code | runtime has three benchmarks (hotpath) ✓ (reviewer) | **Accepted:** C7 adds a new (not frozen) table-write benchmark timed parent vs commit; C6's note states the edit is on the refused post-drain branch only, so the Send comparison is a collateral-cost check. | E1 (+addendum) |
| DA-L1 | G10-1 candidates miss a facade-only option; B's full cost should reach the owner | — | **Accepted:** E2 adds the facade-only option with its rejection reason; G1 tells the owner B's cost (3 commits across 3 lanes, new internal API, public behaviour change). | E2 |
| DA-L2 | G10-21 test's source half duplicates lifecycle_test.go:724/:759 | — | **Accepted:** drop the source half. | E1 |
| DA-L3 | A1 has two notes and no single commit list | — | **Accepted:** coordinator writes one ordered A1 list in the G1 docs (and decides the rows/runner K1 split). | coordinator |
| DA-L4 | Coupling commits 4 vs plan line 2 | — | **Accepted:** re-projected ≈ 83 with all notes (coordinator, items 33–36). | coordinator |
| DA-L5 | `--expect` rebuilds every side in a fresh worktree | — | **Accepted:** reuse one worktree per revision across entries, reset between; never -trimpath (frame tables). | W0 |
| DA-L6 | §7.3's proofs-row rule names coupled files; item 15 said not triggered | — | **Accepted:** run the proofs row after E1's nosleep_test.go edit (cheap). | coordinator |
| DA-L7 | Record G7a-2's widening site by site | — | **Accepted:** the ledger row lists each widened site as "chosen by the design note". | coordinator (ledger) |
| DA-L8 | G10-20's helper can make an expired-ctx Drain wait on runMu | — | **Accepted:** E1 states the bounded wait. | E1 |

Open from the reviewer's verdict: check whether G10-22 (CD2) changes the table texts E1's `TestAddRefusesStartingRowsItCouldNotApply` and `table.Check`'s test match (a hidden W3 coupling into internal/runtime). CD2 says G10-22's text stays byte-identical; confirm at the CD review.

### Architecture review, part two (CD1, CD2, CD2 addendum; items 30–36)

| Id | Short | Disposition | Note to revise |
|---|---|---|---|
| DA-S6 (widened) | The CD2 addendum's check uses "Runs serially:"; CD1 uses the repo's `Not parallel:` and has a script | **Accepted:** CD1's marker and its G2-73 §4 script are the ONE shared check for B, E2, CD1 and the CD2 addendum. | B, E2, CD2 addendum |
| Amendment 2 | E1's `TestAddRefusesStartingRowsItCouldNotApply` pins table texts that no W3 change touches (G10-22 choice A keeps them byte-identical) | **Accepted:** coupling recorded in the record; no action. | record |
| DA-L9 | Lane CD's line is 13 (ceiling 15), not 14/16; CD's 16 includes the bench-input commit (G2-69), a K2-del (C9) and a coordinator coupling commit (C2) | **Accepted:** CD = 13 lane + 1 bench + 1 K2-del + 1 coupling; 2 commits of slack. Coordinator items 33/36 corrected. | coordinator |
| DA-L10 | G7a-3's coordinator commit edits internal/expr files CD's lane also edits in W3 | **Accepted:** the lane prepares C2's expr hunks on its branch; the coordinator adds notbuilt.json and test_notbuilt.py to that commit, then `--job proofs`; stated in the CD brief. | CD brief |
| Item 34 | No discrepancy: 72 regex matches + literalvalue_test.go:17 edited by hand = 73 | **Closed.** | — |

### QA review (DQ-)

| Id | Short | Checked | Disposition | Note to revise |
|---|---|---|---|---|
| DQ-B1 | CD1's escape inputs (G2-69..72) lost their backslashes again | save.py does a plain json.loads (no unescape); design-CD1.md:315 holds a raw 😀 (`od -c`) ✓ — cause: the agent's JSON tool input decoded a single `\u` | **Accepted (blocker):** CD1 re-emits the four sections with U+29F5 standing for the backslash, hex bytes and code points, and a builder `od -c` check. The build is still protected by K2b's coverage and the plants. Lesson for every brief: escape-bearing text uses the ⧵ notation. | CD1 (resumed) |
| DQ-S1 | B's K1 revert plants as `--expect` at the lane base | = DA-S5 | **Accepted** (B resumed); A1b's G3-72 "or `--expect` with new red" wording corrected by the coordinator in design-A1b.md. | B; A1b (done) |
| DQ-S2 | W0 counter-examples #5 and #7 plants cannot read red | — | **Accepted** (W0 resumed). | W0 |
| DQ-S3 | E1's K2-del covered-set proof has no deterministic fallback | — | **Accepted:** the deleted test's own covered blocks ⊆ the remaining suite's intersection; G2-129's row covers shard.go:184 (E1 resumed). | E1 |
| DQ-S4 | `parse_patch` a/ b/ prefixes unspecified | — | **Accepted** (W0 resumed; counter-example #9b). | W0 |
| DQ-S5 | B G2-108 has no load run | — | **Accepted** (B resumed). | B |
| DQ-L1 | E1 G10-20 `--test '^…$'` refused by mutate.py | mutate.py checks `isidentifier()` ✓ | **Accepted** (E1). | E1 |
| DQ-L2 | E1 G10-21's test does not witness the unreadable-recording motive | — | **Accepted as recommended** (E1 adds a recording-reader assertion if cheap). | E1 |
| DQ-L3 | E1 G2-131 load-run placeholder | — | **Accepted** (E1). | E1 |
| DQ-L4 | E2 G7a-1 misses deploy_test.go:233's doc comment | — | **Accepted** (E2, with DA-S3). | E2 |
| DQ-L5 | No batch states its timeout | — | **Accepted:** W0 adds a per-entry `timeout` key; notes name it for slow entries. | W0, E1, B, CD1, A1a |
| DQ-L6 | W0 G2-149's list-file bytes | — | **Accepted** (W0). | W0 |
| DQ-L7 | W0 G2-147/155 grep vs AST | — | **Accepted as a named deviation** (W0). | W0 |
| Item 26 | Third agent in W1 | — | **Superseded:** the owner's six-agent lift. | — |
| Not reached | A1b G2-27..49, A2's G2s, CD1's K2s, CD2's G2s and G7a-6, the CD2 addendum, E2 addendum G2-143/145/146/156 | — | **A second, narrow QA pass** over exactly these sections (fresh qa-expert), when a slot frees. | coordinator |

CD2 addendum: "Runs serially:" → "Not parallel:" by the coordinator (DA-S6 widened), noted at its end.

### Second QA pass (DQ2-)

| Id | Short | Checked | Disposition | Note |
|---|---|---|---|---|
| DQ2-S1 | A2 duplicate ids, wrong class (G2-11), placeholders (G2-59/63/64); E2 G2-146 name | class names verified with `ast` (Scans, ArchivePinsTest, Reader, Main) ✓ | **Accepted:** corrections appended to design-A2.md; G2-146 already fixed in E2 rev1. | A2, E2 |
| DQ2-L1 | CD1 G2-79 grep misses digits | = pre-check 1 | **Accepted:** `[A-Za-z0-9]+`, old 4 + 1 (in CD1 rev1's corrections). | CD1 |
| DQ2-L2 | E2 G2-156 control old "invalid" | — | **Accepted:** old_test/new_test, green/green. | E2 |
| DQ2-L3 | A2 G2-10/G2-8 covered set without fallback | — | **Accepted:** per-test traced set ⊆ the remaining suite's. | A2 |
| DQ2-L4 | A1b G2-46's carried reason too strong | — | **Accepted:** reason restated (pinning go vet's shape); stays carried. | A1b |
| DQ2-L5 | CD1 G2-134 counts span two commits | — | **Accepted:** 7 at CD-text-K2, 1 at C4. | CD1 |
| DQ2-L6 | E2 G2-143 commands drop -I -B | — | **Accepted.** | E2 |

## G1 preparation, lane A1's commit list and the owner's questions

## S1.1c-7 G1 preparation (coordinator; after the design reviews and revisions)

### State
- **Design set:** nine notes, each with its revision or addenda, all checked by the coordinator.
- **Reviews:** two blind reviews (architecture in two parts, QA), a second narrow QA pass, and two pre-checks of the Go plants against today's code. Pre-check 2: 29/29 as predicted once a placeholder test name was fixed.
- **Ledger coverage:** 185 open ids have a design entry, plus G10-6 fixed elsewhere (186).
  - **Rejected, reasons verified by the coordinator (5):** G2-19, G2-50, G2-54, G2-77, G2-80.
  - **Carried in full (16):**
    - to S1.1c-8, the renames: G2-39, -46, -51, -57, -58, -101, -119, -121, -142, -148;
    - to S1.1c-10, the non-simple changes: G10-23, G2-62, -123, -124, -126, -130.
  - Further renames are carried in part.
- **Commit projection:** ≈ 83 of 100. The fix round is inside it.

### Owner questions (recommendation first)
1. **G10-20's leftover window.** The data-race fix locks the read. A smaller logic window remains: a Drain whose quiet step failed can put the app back to "active" after another Drain has started draining. Input is still refused, so nothing is lost.
   - **Recommended:** a known issue (correctness: no), forwarded to S1.1c-10.
   - Others: the backlog; fix it now in E1 (lane at its commit ceiling).
2. **G10-21 and known issue S3.3-1-3.** E1 fixed the undecided part: a derived stream's batch end after a drain no longer reaches the recording, where it made the recording unreadable. The counting rule itself is designed (s3-3-1 MUST 11, T30, owner design 6). E1 recommends one rule for all input as S3.3-1-3's fix.
   - **Recommended:** keep S3.3-1-3 at S8 and record E1's recommendation in its row.
   - Other: pull it into S1.1c-7 now.
3. **G10-1's design** (the design notes chose it; its cost is for the owner to see). A failed Start can leave an app half-started. Fix B applies the starting rows inside the activation step, so a failed Start leaves nothing changed and can be retried.
   - **Cost:**
     - 3 commits across 3 lanes;
     - new internal API: `App.Loads`, `TableLoad`, `table.Check`;
     - each app's recording queue grows by its starting rows;
     - Deploy now owns starting rows end to end.
   - **The cheaper option F** (facade only) leaves two holes: a concurrent Send can see a table before its starting rows, and Start can block past its deadline.
   - **Recommended:** B. Other: F.
4. **The build schedule with six agents:**
   - **W0**, four agents at once:
     - mutate.py's contract, then the bench fixes;
     - the naming tool;
     - W0-go's tool tests;
     - `table.Check`.
   - **W1 + W2** together: four lanes, A1, B, E1 and A2.
   - **W3:** E2 and CD.
   - A stratified spot-check follows each wave.
   - **Recommended:** this. Other: the plan's five waves.

### For the owner's information
- **DA-S1:** each app's recording queue is the configured size plus one slot per starting row (E1's choice). It is unchanged for an app without starting rows.
- **DA-S3:** resolved without a new facade check. The validator (E-TBL-002) and the row writer already refuse every row the table would, in Deploy's own wording. A test pins that wording.
- **Transport lesson:** agent output passes through JSON, which decodes a single `⧵u`. CD1's escapes and W0's `GO_SPACES` lost their backslashes on the way. Every brief now writes escapes with ⧵ (U+29F5) and has the builder check bytes. The proofs (coverage, plants) would have caught any slip at build.
- **New items found by the notes** (not survey findings), carried to S1.1c-10:
  - partition's `world.advance` ignores body panics;
  - A2's G10-5 pid-reuse part;
  - the `model.go:199` comment.
- **G3-72** was designed twice (a coordinator prompt error); A1b's identical design stands.

### A1: one ordered commit list (DA-L3)
1. **A1-1, K1, gate/runner/rows:** G10-7, -8, -9, -10, -11.
2. **A1-2, K1, gate/runner:** G10-12 (scheduler.py), G10-13 (teeth.py).
3. **A1-3, K2e, gate/runner/tests:** A1a G2-18, -23, -26, -37, -40, -43; A1b G2-30, -31, -34, -47, -48.
4. **A1-4, K2a/K2d, gate/runner/tests:** A1a G2-20, -21, -22, -38, -41, -42, -44; A1b G2-27, -28, -32, -33, -35, -36, -45, -53. That is 15 findings, so it may split by file into A1-4a and A1-4b.
5. **A1-5, K2-del (pool):** G2-49, G2-52.
6. **Coupling (coordinator, with the lane's change):** G3-72, with test_suppressions_planted.py:42-43. Then `--job proofs`.

All of these touch gate/runner/tests, several test_suppressions.py, so they run in order in one lane. That is 4–5 lane commits against a ceiling of 6, plus 1 pool and 1 coupling commit.

### Build briefs to write after G1
- **W0-py-a** (python-engineer): design-W0 Part A as revised (design-W0-rev1), W0-1 to W0-5. GO_SPACES comes from Go's `unicode.IsSpace` (item 37).
- **W0-py-b** (python-engineer): Part B, W0-6.
- **W0-go** (go-engineer): W0-7, W0-8. Proofs run after W0-3 lands.
- **table-check** (go-engineer): design-E1-addendum Condition A.
- Then one brief per lane, from its note, revision, addenda and coordinator items. Every brief: escapes in ⧵ notation, with byte checks.