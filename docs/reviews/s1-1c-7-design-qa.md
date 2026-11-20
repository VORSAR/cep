# S1.1c-7 design — QA review

*The blind QA review(s) of S1.1c-7's nine design notes. The reports are as the agents gave them; paths are made repo-relative. Every finding's disposition is in [the coordination record](../sprints/s1-1c-7-design-coordination.md).*


## First pass (ids DQ-)

## S1.1c-7 design review: QA half (blind)

### Header

**What I read**
- **Plan check:** §0, §1, §2, §4, §5, §6, §7, R1–R5 and §11 of `docs/sprints/s1-1c-7-plan-check.md`; the design brief; the review brief; `design-coord-items.md` items 1–36.
- **W0:** all of it (Part A's contract and the §6 counter-example table, Part B, every finding, the commit plan, choices, couplings).
- **E1:** the header, summary, G10-20, G10-21, G7a-5, G2-113 to G2-132 (every section from :397 on), the commit plan, choices, couplings and carried items.
- **E1-addendum:** all of it.
- **E2:** the header, summary, G10-1, G10-23, G7a-1, -2 and -4, the commit plan, choices, couplings and carried items.
- **E2-addendum:** G2-83 and G2-84 (the code part); G2-143 by its summary row only.
- **B:** the header, summary, G10-18, G10-19, G2-89, -90 (head only), -92 to -98, -105 to -110, the commit plan, choices, couplings and carried items.
- **A1a:** the header, summary, G10-7 to G10-13, G3-72 (superseded), G2-18 to G2-22.
- **A1b:** G2-50 to G2-54 and G3-72 (canonical). I did not read its commit plan or choices.
- **A2:** G10-5 and G10-14.
- **CD1:** the summary, G10-16 and G2-68 to G2-72.
- **CD2:** the summary, G10-15, G10-17, G10-22 and G7a-3.

**Not reached (time box exceeded):**
- A1b: G2-27 to G2-49, its commit plan and choices.
- A2: G2-5 to G2-64, its commit plan and choices.
- CD1: G2-65 to G2-67 and G2-73 to G2-134, its commit plan and choices.
- CD2: G7a-6 and every G2, its commit plan and choices.
- CD2-addendum.
- E2-addendum: G2-143's three-part proof, G2-145, G2-146 and G2-156.
- E2: G2-1 to G2-4, G2-81, G2-82, G2-141 and G2-144.
- My K2 sample is therefore not the full quarter of every note (the brief's sha256 draw). It covers W0, E1 and B in full, and E2, A1b and A2 in part.

**GitNexus:** 0 calls. The one augment attempt said the graph DB was held by another GitNexus process. I did not read the index freshness resource. Every code claim below comes from `sed`/`git grep` at 1f4ebba (production code equals d521725 for these files) and is marked "(grep)" where it rests on a search. I made one read-only computation: the FNV-1a shard pairs for G2-115, in `python3 -I -c`.

### Findings

**DQ-B1: CD1, G2-69, G2-70, G2-71 and G2-72. The raw-string inputs lost their backslashes; the plan's §0 item 1 failure has happened again.**
- **Claim:** every `\uXXXX` in a backtick span of design-CD1.md was decoded to the raw rune. The "new" inputs are byte-identical to the old ones.
- **Evidence:** `od -c` of design-CD1.md:315 shows the new `` []byte(`😀`) `` as the raw 4-byte rune, which is exactly the defect at decode_bench_test.go:142. The same holds at :355 (`{"v":"é中A"}` raw, while its Interpreted column `\\u00e9\\u4E2D\\u0041` survived) and in rows :356 to :360, whose Interpreted column is "–", so no byte-exact form survives at all. It also holds at :417, :436 to :445 (validFor, escapedStrings and the `stringPieces` literal) and at :469 and :474 inside the new test. :434's "the old escapes are `\u0000`, `\u001F`, `t`" shows that `t` became `t`.
- **Why it matters:**
  - A builder following the note writes no change, and the new tests go red. That forces re-investigation, which the process forbids.
  - Worse, the corrupted literals in TestTheGeneratorsEscapedSpellingsHoldTheirEscapes check raw runes. Its `slices.Contains(validFor[TString], "\"é中\"")` would pass on the old, defective list.
  - G2-72's `stringPieces` as written holds "é" and "😀" twice, so the note's own no-duplicate check fails it.
- **Fix to the note:** re-emit every input and every literal in these four sections as a Go interpreted string with `\\u` escapes, which survive (or as raw strings, checked by `od -c`). Give an Interpreted column for every row. Re-check that "verified" texts were not passed through a JSON layer. Then grep all notes for raw non-ASCII inside backticks near the words "escape", "surrogate" or "invisibles". On my check only CD1 is hit; W0:59's raw spaces are intended.

**DQ-S1: B, G10-18 and G10-19. The K1 revert plants are written as `--expect` entries whose find text exists only at the new revision.**
- **Evidence:**
  - The B note's entries at its lines 64 and 121 carry `old_rev` set to the lane base and `expect {"old":"green","new":"red"}`.
  - The find strings (`"a %s record takes no %q key"` and `fault("%w", inPlayback{err})`) are absent at the lane base. records.go:127 reads "has no"; expect.go:62 has no inPlayback.
  - W0's `read_batch` (design-W0 A.2) checks "the mutation exact-once against each rev's blob", so the batch is refused with exit 2 before any build.
  - G10-19's corrected chain plant ("old n/a, new red") has the same shape.
- **Why it matters:** the K1 proofs cannot run as written, and the ledger's K1 word is "red" on the fix commit, not green/red.
- **Fix to the note:** make all three entries a plain batch (or single runs) on B-K1's commit, each expected red, with the failing line quoted. Also correct A1b G3-72's "or `--expect` with `{"new":"red"}`": W0 requires both old and new.

**DQ-S2: W0, Part A counter-examples #5 and #7. Their feature-off plants cannot read red, against plan §7.7.**
- **#5 (F3 vs F30):**
  - The plant drops the final `$`, so F30 also runs.
  - But the `ran:` lines come from `runs()`, whose regex is itself anchored (`…/F3(?:#[0-9]+)?$`), so F30 never appears there.
  - mutate prints no go output on a red outcome. The test's assertion "`ran:` == exactly TestTable/F3" therefore stays green under the plant.
- **#7 (a parent failing before t.Run):**
  - The probe in `run_go` already finds 0 `=== RUN` lines and returns invalid before `judge_go` runs.
  - So the plant `if not t.levels: → if True:` never takes effect, and the test stays green.
- **Fix to the note:**
  - #5: make `ran:` list every `=== RUN` name under the top-level test at the designated depth (unanchored after the parent), or assert on the probe and run patterns.
  - #7: plant both `if n != 1: → if n > 1:` and `if not t.levels: → if True:` as one `--patch`, as #3 already does.
  - #3's two find/replace pairs likewise need `--patch`; say so.

**DQ-S3: E1, G2-113 and G2-129 (K2-del). The covered-set proof has no deterministic fallback in a timing-heavy package.**
- **Evidence:**
  - The proof (E1 :409-411) needs four whole-package `-coverpkg=./...` profiles of internal/runtime to be equal.
  - The note's own pre-mortem (:797) admits the set may differ between runs, and says it "falls back to plants where one exists".
  - For both findings the note shows that no plant exists (:401, :710).
  - B's G2-95 already carries the robust form (B :302): the deleted test alone against `-run '^$'`.
- **Fix to the note:** add the supplementary proof. Take the deleted test's own covered blocks (`-run '^TestBkDuplicateInputDoublesEvents$' -coverpkg=./...`) and show they are a subset of the intersection of the remaining suite's two runs. For G2-129, show that the "no stream" row's coverage alone includes shard.go:184's guard block.

**DQ-S4: W0, `parse_patch`. The `a/` and `b/` path prefixes are not specified, and every other note's `--patch` depends on them.**
- **Evidence:** E1 (:118, :458), E2-addendum G2-83, B G2-97 and CD2 (`git diff <C> <C>~1`) all produce `--- a/<path>` and `+++ b/<path>`. W0 A.2 refuses only absolute paths and `..`; it never says the `a/` and `b/` are stripped.
- **Fix to the note:** state that a leading `a/` or `b/` is stripped (git's default); that `diff --git`, `index` and mode lines are skipped; and that the two paths must agree. Add counter-example #9b for a `git diff` output.

**DQ-S5: B, G2-108. A goroutine-test fix has no 50/50 load run (plan §7.2).**
- **Evidence:** the change makes `q2Start`'s cleanup fail on Close's error (helpers_s3_2b_2_test.go:117-119, 30 s ceiling) in 24 tests. The note substitutes "the full package under -race at hand-back" (B :543).
- **Fix to the note:** `load.py -- -race -count=50 -run '<regex of the q2Start users>' ./internal/playback` → 50/50. Keep the stop rule (a red there is a real defect).

**DQ-L1: E1, G10-20. The red command uses `--test '^Test…$'`.**
- mutate.py:93 refuses a non-identifier (`isidentifier()`), so the run exits 2.
- **Fix:** `--test TestTwoDrainsAtOnceOneAlreadyCancelledLeaveTheAppDrained`.

**DQ-L2: E1, G10-21. The test pins the counting part of choice (C) but not its stated motive (an unreadable recording).**
- The revert plant is honest and red (lifecycle.go:179; the find is exact-once against the accepted branch at :186).
- The derived-batch-end-recorded-as-`rBatchEnd` window (router_in.go:44-50, records.go:277) is unwitnessed.
- **Fix:** optional. Either add a recording-reader assertion, or state that the counting test is the only proof.

**DQ-L3: E1, G2-131. The load-run command holds a placeholder.**
- It reads `<the test at :674>`, omits the third site's test (breakit_s3_3_2_test.go:32) and omits `./internal/runtime`.
- **Fix:** name all three tests.

**DQ-L4: E2, G7a-1. The dry run misses deploy_test.go:233's doc comment.**
- The comment ("refused naming http and S5.1") goes false in the same commit (grep).
- **Fix:** add it to the K1 commit's edits.

**DQ-L5: All notes. No batch states its `--timeout`.**
- K2d and slow entries need more than 60 s: E1 G2-118 and G2-132, CD1 G2-68, and A1a G2-20, -22 and -42. W0 has no per-entry timeout key.
- **Fix:** each note names the batch's `--timeout`, or W0 adds a `timeout` batch key.

**DQ-L6: W0, G2-149. The "normal mode clean" case's list file is not given.**
- **Fix:** write its exact bytes (`"internal/model/x.go\x00"`).

**DQ-L7: W0, G2-147 and G2-155. The plan asks for an AST check; the note uses a grep.**
- The grep is sound on these files: `exec.Command(` sits in no string or comment (grep). Record it as a named deviation.

### Coordinator items I disagree with (in part)

| # | Item | Disagreement |
|---|---|---|
| 9 | "Build uses the corrected plant only" | The corrected G10-19 entry is still an `--expect` with "old n/a", which the W0 contract refuses (DQ-S1). It must be a plain red run. |
| 26 | `table.Check` "during W1 by a fresh go-engineer" | W1 already runs A1 and B. A third agent breaks ground rule 17 (≤ 2 agents at once). Land it before W1, or between W1 and W2, or as E1's first commit with a recorded one-commit ownership of internal/table/table.go. Its proof shape (test plus `return nil` plant) is fine. |
| 12 | A1b's G3-72 canonical | Agree. A1b's "or `--expect` with `{"new":"red"}`" needs the DQ-S1 correction. |
| 21 | G2-6's exact value rejected | Agree, verified: `mann_whitney_p` is the exact tie-conditional test (compare.py:51-74), and `--pairs` is samples per side (compare.py:94-95), so the floor is 2/6. |

I agree with items 3, 13, 15, 16, 18, 19, 22, 27 and 32 as far as I checked them.

### Spot tests: would the proof catch a bad fix?

| # | Note / id | Kind | Checked against | Catches? |
|---|---|---|---|---|
| 1 | W0 G2-6 | K2a | compare.py:241 exact-once; p/10 < 0.3333 for every p ≥ 1/3 | yes |
| 2 | W0 G2-7 | K2a | load.py:135-137: `below=[]` gives "none"; the old `or "timed out"` stays green | yes |
| 3 | W0 G2-149/150/151 | K2a/K2e | main.go:1726-1729 and :619 exact-once; purity.json:7; `report` does not dedup (:421-423) | yes |
| 4 | W0 G2-154 | K2a | git error format main.go:193; ParseFile's `p/a.go:2:`; usage at :56 | yes |
| 5 | W0 §6 #5, #7 | tool | the anchored `runs()`; the probe precedes `judge_go` | **no** (DQ-S2) |
| 6 | W0 G10-2 | K1 | escapes.py:336-343 format `heap before: … x1` | yes |
| 7 | E1 G10-20 | K1c | drain.go:22-31 and :110-121; the A-before-B interleaving is reachable through public Drain; helpers exist | yes (command syntax DQ-L1) |
| 8 | E1 G7a-5 | K1 | bcAttrs and bcEv reach each branch (boundary.go:164-226); old parentheticals pinned by no test (git grep) | yes |
| 9 | E1 G2-114 | K2b | boundary.go:221 is the first body statement; old 2 values vs 3 attributes | yes |
| 10 | E1 G2-115 | K2a | FNV recomputed: 16 shards pair (2,2),(5,5),(12,12),(15,15); 3 shards (0,1) | yes |
| 11 | E1 G2-117 | K2c | 400 ms sleep before queueDrain against the 300 ms ctx; enqueue checks ctx first (requests.go:57) | yes |
| 12 | E1 G2-113/129 | K2-del | coverage determinism | **at risk** (DQ-S3) |
| 13 | E1-add G10-1 | K1h | header queued at Add; `put` gives "queue full after record 4" (recorder.go:91-110); table texts (table.go:215, :246) | yes |
| 14 | E2 G10-1 | K1 | Activate reads ctx once before queuing (lifecycle.go:94, requests.go:56-66); old ErrStarted on retry | yes |
| 15 | E2-add G2-83 | K2a | patch context byte-exact (partition.go:167-179) | yes |
| 16 | B G10-18/19 | K1 | plant text absent at the lane base | **cannot run** (DQ-S1) |
| 17 | B G2-97 | K2c | dctx made after the sends (s3_2b_2_build_test.go:117); sink.go hunks exact | yes |
| 18 | B G2-98 | K2e | `Ceiling any`, so `{"ceiling":0}` parses (corpus_test.go:190) | yes |
| 19 | A1b G2-52 | K2-del | MEMBER_CAP exact-once; literal "exceeds 4 MiB" keeps subtests green; :718 then :403 | yes |
| 20 | A2 G10-5 | K1c | revert: second call sees no pidfile; fix: the flock refusal is an OSError | yes |
| 21 | CD1 G2-70/72 | K2b | inputs byte-identical to old | **no** (DQ-B1) |
| 22 | CD2 G7a-4/E2 G7a-4 | K1 | old text partition.go:238-240 ends in the parenthetical; HasSuffix reds | yes |

### Verdict

**Not ready for G1 yet.** Three things must change first:
1. **DQ-B1:** CD1's escape inputs must be re-emitted byte-exactly, with the tests' literals re-checked.
2. **DQ-S1 and DQ-S2:** B's K1 entries and W0's counter-examples #5 and #7 are proofs that cannot fail, or cannot run, as written.
3. **DQ-S3 and DQ-S4:** they need a fallback proof and a one-line contract fix that four notes depend on.

The rest of what I checked is well built:
- W0's G2 entries, E1's K1, K1c and K1h material (DQ-L1 aside), E2's G10-1 test, the E1-addendum's three plants, A2's hold and A1b's K2-del all hold against the code.
- Rejections G2-19, G2-50, G2-54, G2-77 and G2-80 match the coordinator's checks. I re-verified G2-50's and G2-54's reasoning only, not the code lines.

I could not review: the larger part of A1b, A2, CD1 and CD2's K2 sections; the CD2-addendum; and E2-addendum's G2-143, -145, -146 and -156. These need a second QA pass, or the spot-check after each wave should weight them.


## Second, narrow pass over the sections the first did not reach (ids DQ2-)

## S1.1c-7 design notes: second, narrow QA review (blind)

**Verdict:** these sections can go to G1 once the note edits in DQ2-S1 and DQ2-L1 to DQ2-L6 are made. I found no blocker. Almost every plant I checked is exact-once, sits in production code, builds and matches its finding, and the predicted old and new words follow from the code.

The two real faults are bookkeeping, not proof design:
- **A2's batch entries cannot run as written:** duplicate ids, one wrong class name and three class-name placeholders.
- **CD1's G2-79 count check is mis-predicted.**

I went far over the 40-minute box (about 95 minutes).

### What I read and how

**Read in full:**
- **A1b:** G2-27 to G2-49, the commit plan, choices, couplings and carried items.
- **A2:** G2-5 to G2-64, §4 to §7.
- **CD1:** header, summary, G2-65 to G2-67, G2-73 to G2-80, G2-134, commit plan, choices, couplings and carried items. I skipped G2-68 to G2-72 (68 was reviewed in round one; 69 to 72 are being re-emitted).
- **CD2:** summary, G7a-6, G2-85 to G2-88, G2-133, G2-135 to G2-140, §4 to §7.
- **CD2 addendum:** all of it.
- **E2 addendum:** G2-143, G2-145, G2-146 and G2-156.
- The dispositions, round one's QA review and both architecture reviews.

**Tools:**
- **GitNexus:** none. I did not read `index.commit`.
- **Code claims:** checked with `sed`, `grep`, `git grep` and `od -c` at 1f4ebba (Go and gate code unchanged since d521725). Lines marked "(grep)" rest on a search.
- **Pure computations, read-only, with the pinned 3.15.2 under `-I -B`:**
  - `teeth.python_cases` on G2-33's sources;
  - AST and loader test counts for G2-13, -30, -31 and -34;
  - exact-once counts of several `find` texts;
  - class names behind the A2 test ids.

**Not checked:** runtime outcomes (no runs allowed). G2-65's thresholds, G2-66's model and G2-46's vet output are reasoned only.

### Findings

**DQ2-S1: A2 (G2-11, -55, -59, -61, -63, -64) and E2 addendum (G2-146). Several batch entries cannot run as written.**
- **Claim:** W0's `--expect` needs a unique `id` (design-W0.md:199), and pyshim loads the test id by name. As written:
  - **Duplicate ids:**
    - G2-55: "The three controls have the same ids with `"control":true`" (design-A2.md:454).
    - G2-61: "The same entry is repeated for" two more tests (:568).
  - **Wrong class:** G2-11 names `test_generate.Render…`, but the test is in class `Scans` (test_generate.py:189). The note itself says "to be confirmed".
  - **Placeholders:** `<class>` for:
    - G2-59: the class is `ArchivePinsTest` (test_archive_pins.py:106);
    - G2-63: the class is `Reader` (test_parity_handoff.py:315);
    - G2-64: the class is `Main` (test_vulndb_fetch.py:185).
  - **Placeholder test name:** E2 addendum G2-146 uses `TestBreakitIotaSwapChangesTheSurface`. The real name is `TestBreakitIotaRepetitionKeepsTypeAndValue` (breakit_s3_3_1_test.go:28).
- **Why it matters:**
  - A duplicate id makes the batch exit 2 before any build.
  - A wrong id reads "invalid".
  - Either way the builder must investigate, which the notes are meant to prevent.
- **Fix to the notes:**
  - Give each entry its own id: `G2-55-start-control`, `G2-61-readonly`, `G2-61-cache`, `G2-61-hash` and their controls.
  - Write the four class names and the real Go test name into the entries.

**DQ2-L1: CD1, G2-79. The first K2e check's predicted old count is wrong, and the check is blind to one table.**
- **Evidence:**
  - `grep -cE '^var [A-Za-z]+ = ⧵[⧵]pecRow⧵{'` matches only `pecContract`, `pecIncidentRows`, `pecVerbatimRows` and `peoOperandRows`, so the old count is 3 + 1.
  - `pecG1Rows` (parseerror_contract_test.go:303) has a digit, so it never matches. A left-over `var pecG1Rows` would therefore pass the check. The compiler would still catch it, because `pecEdit(t, …)` has no `t` in a package-level initialiser.
  - The other two checks hold: `panic(` count 1; 122 `pecEdit(p…` calls plus 1 definition.
- **Fix:** use the pattern `[A-Za-z0-9]+` and predict old **4 + 1**.

**DQ2-L2: E2 addendum, G2-156. The control's old word is "invalid".**
- **Evidence:** design-E2-addendum.md:329 uses `test` = the new test, with `"old":"invalid"`. A control is meant to show the real code green (plan §7.2).
- **Fix:** give the control `old_test: TestShapeCorpusGolden` and `new_test: TestJudgeReportsEachLoweringCodeAtTheLoweringStage`, expecting green/green, the same shape as the plant entry.

**DQ2-L3: A2, G2-10 (also G2-8, same commit). The covered-set proof has no deterministic fallback.**
- **Evidence:** the proof needs four whole-suite `trace` sets over `gate/canary/tests` discovery to be equal (design-A2.md:283-285).
- **Fix:** add round one's DQ-S3 form. Each deleted test's own traced set must be ⊆ the remaining suite's set.
  - The docstring test runs no production line, so its set is empty.
  - G2-8's removed test's lines (candidate.py:240-244) are covered by test_f1_feeds's two seam tests.
- A1b's G2-49 already uses the per-test form.

**DQ2-L4: A1b, G2-46 ("carried in full"). The reason "there is no production plant for a tool's output" is too strong.**
- **Evidence:** rows/vet.py:41-42 (`if obj:` then `positions(obj) or [json.dumps(obj)]`) can be planted to print an object. An object's text starts with `{`, which sorts after `go vet exit`, so the anchored regex at test_rows_go.py:125 would stay green.
- Whether go vet emits objects for a package that fails to compile is unverified (not run).
- **Fix:** keep the rename-only choice, but restate the reason ("asserting no object would pin go vet's own output shape"), or add `assertNotIn('{', reason)` with that plant if vet emits `{}`.

**DQ2-L5: CD1, G2-134. The predicted "old 8, new 0" spans two commits in two notes.**
- CD-text-K2 carries the 7 text pairs. The plan pair lands in CD2's C4 (CD2 addendum).
- At CD-text-K2 alone, the check over 8 pairs reads 1.
- **Fix:** run the 7 text pairs at CD-text-K2 (old 7, new 0) and the plan pair at C4 (old 1, new 0).

**DQ2-L6: E2 addendum, G2-143. The commands drop the pinned interpreter flags.**
- The commands read `python3.15 gate/bench/mutate.py …` and `python3.15 gate/bench/load.py …`.
- mutate.py's own usage says `python3.15 -I -B` (mutate.py:3-5). Without `-B`, bytecode can land under gate/.
- **Fix:** `python3.15 -I -B` in both commands. Everything else in the three-part proof holds:
  - the plant is exact-once (counted, source.go:57-58);
  - `Done()` is first read in the blocking select (source.go:52-58), since the quit select at :47-51 never reads it;
  - the coverage block is :57-58;
  - the load run is given.

### Checked and holding (no finding)

**Rejections:**
- **CD1 G2-77:** holds. A document added to `fragmentDocs` (fragment_test.go:1101) with no pin compares zero with zero at :1164, so `got.ruleLike == 0` (:1172) is its only guard.
- **CD1 G2-80:** holds. parse.go:58 documents a `*Error`.

**K2-del entries:**
- **A2 G2-8:** `DigestMismatch` subclasses `CanaryError` (candidate.py:43), so the `except CanaryError` plant reds both the removed test and test_f1_feeds:66-68.
- **CD2 G2-137:** the `E-FN-001` find is exact-once (exprs.go). The H3 subtest name is exact (hiding_test.go:60). Expectation red/red.
- **A1b G2-49:** the per-test `trace` set is valid, and stronger than the package set.

**Load runs present:** A1b G2-45, -47 and -48; CD2 G2-133; E2 addendum G2-143. A2's G2 tests start no thread. CD2 addendum: no goroutines.

**K2b first statements:**
- A2 G2-14: :68, the loop body. `World.run` calls `main` in-process (test_gitnexus_reindex.py:84-90), so `trace` sees it.
- CD2 G2-136: nulls.go:64.
- E2 addendum G2-143: source.go:58.

**K6, CD2 G7a-6:**
- The new text matches validate/names.go:72-85.
- The sweep is clean on "E-TBL-002" and "§4.3", and red on the removed line ("owner", "D3", "S3.3").
- astequal's argument order matches tools/astequal/main.go:3-4.

**Byte checks (`od -c`):**
- E2 addendum G2-156: `'C:⧵u005cq'` and `"'C:⧵⧵q'"` are intact; its four inputs equal lowering_test.go's `program(...)` expansions.
- E2 addendum G2-143: the `⧵n⧵t` find is intact.
- A2 G2-9: `⧵u2192` is intact.

### Spot tests: would the proof catch a bad fix?

| # | Note / id | Kind | Checked against | Catches? |
|---|---|---|---|---|
| 1 | A1b G2-27 | K2a | run.py:37 exact-once; `--no-such-flag` reaches the usage text (test_run_guard.py:60-64), so red at `assertIn("refused:")`, one failure | yes |
| 2 | A1b G2-33 | K2a | teeth.py:57, :354, :362, :372-373 exact-once; I re-ran `python_cases`, and every expected list matches; one subtest fails per plant | yes |
| 3 | A1b G2-30/31/34 | K2e | AST and loader counts re-computed: 12/17, 46/59, 6/8 | yes |
| 4 | A1b G2-35 | K2a | patch hunks match vulndb.py:255-260 and :266-272 exactly; the planted entry comes first, so the leak reaches GO-2023-1758 | yes |
| 5 | A1b G2-47 | K2e live | sorted waivers put gate/wait.py at index 5, so the old test has an extra `elsewhere_05_1` line: one failure | yes |
| 6 | A1b G2-48 | K2e | kill_child sends SIGKILL in all 3 tests; `Popen.kill` sends nothing once `returncode` is set | yes |
| 7 | A2 G2-5 | K2a | regen.py:151-153 and :319-321 hunks exact; no other accept guard (grep) | yes |
| 8 | A2 G2-14 | K2b | the plant returns the other repo's row, so `start` writes the pidfile (:99-102) | yes |
| 9 | A2 G2-15 | K2e live | S4.1 and S4.2 both in gate/sprints.json; old red at :196 | yes |
| 10 | A2 G2-11 / -59 / -63 / -64 | K2a | the class names behind the ids | **no, as written** (DQ2-S1) |
| 11 | CD1 G2-65 | K2a | rule.go:206 exact-once; OnTimers reacts only to TTLPurge, Completion and CronClose, so the old log is unchanged | yes (stop rule) |
| 12 | CD1 G2-66 | K2a | `keyOf` is per ref for Discard (rule.go:307-311); Schedule replaces, Cancel is only on true; the model mirrors both | yes (stop rule) |
| 13 | CD1 G2-67 | K2a | `to := add(m, 1)` exact-once (clock.go:183); Drain covers every stream, as the model does | yes |
| 14 | CD1 G2-74 | K2a | canonical.go:105 exact-once; the parent test does no check before `t.Run` | yes |
| 15 | CD1 G2-75 | K2e | lower.go:290 exact-once; example 07 lines 17-19 hold 1, 4 and 3 fields | yes |
| 16 | CD1 G2-79 | K2e | grep counts | in part (DQ2-L1) |
| 17 | CD2 G2-133 | K2a + load | table writes serialised (table.go:28, :142); one writer per table's "g"; the plant returns the first snapshot | yes |
| 18 | CD2 G2-139 | K2a | nulls.go:67-69 and validate.go:132-134 hunks exact | yes |
| 19 | CD2 G2-88 | K2a | `p` is in scope at plan.go:97 | yes |
| 20 | E2-add G2-143 | coordinator's 3-part proof | plant counted once; `parked` fires only in the blocking select | yes |
| 21 | E2-add G2-145 | K2a | `golden` in scope at main.go:48; find exact-once | yes |
| 22 | E2-add G2-146 | K2a | `specIndex+1` keeps `before != after` and drops "// iota 1" | yes (test name: DQ2-S1) |
| 23 | E2-add G2-156 | K2a | rows byte-identical to the parser's; find exact-once | yes (control: DQ2-L2) |

### Overlap with earlier reviews (not repeated)

These already apply to my sections and I did not re-raise them:
- **DQ-L5:** a per-entry `timeout`. It applies to A1b G2-45 and -47, CD1 G2-65 and -66, and CD2 G2-133.
- **DA-S6:** CD1 and the CD2 addendum already use `Not parallel:`.
- **DQ-B1:** CD1 G2-69 to G2-72.
- **DQ-S1:** A1b's G3-72 `--expect` wording, already corrected.

**Coordinator items:** I disagree with none that I checked. A2's open item "carried in full to S1.1c-10" (G2-62) is a real ledger gap and should stay on the coordinator's list.
