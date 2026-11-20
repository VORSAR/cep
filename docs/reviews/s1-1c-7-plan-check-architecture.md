# S1.1c-7 plan check — architecture review

*The architecture reviews of [the plan check](../sprints/s1-1c-7-plan-check.md), three rounds, by a fresh `architect-reviewer` agent, blind to the other discipline's review. Each finding's disposition is in the plan check's §10. The reports are as the agents gave them; paths are made repo-relative.*

## Round 1: v1, blind (ids PA-*)

I reviewed the plan against `fix/gate-holes` (6602ee3 plus the two fix-PR commits 85802d4 and 363e5f6). I made no edits, no file writes, no gate runs and no git writes. I used GitNexus `impact` on `cgApp` and `context` on `(*Runtime).gate`, plus grep and reads at that commit. I could not read the freshness resource (that tool was not available to me), but the GitNexus answers matched the code wherever I checked. I applied the-fool myself in its evidence-audit and pre-mortem modes, because this session cannot ask anyone to pick a mode.

**Plan claims checked.**
- **Hold:** `LAST_PART` at `gate/runner/parts.py:20`; `hotpath_bench_test.go:3-6`; `rsSelectAt` and `case mb <- head.msg:` (`route_stop_internal_test.go:66, :157`); `ldParked` (`lifecycle_deadlines_internal_test.go:120`); `CORPUS_TEST` at `test_tripwire_planted.py:16`; the proofs row's `min_wants` 39; the lane table's sums (443).
- **Off:** `CORPUS_TEST` is at `test_determinism_macos_planted.py:15`, not :16. `astequal`'s key form is not `<dir>.<name>` (PA-S5).

### Blockers

**PA-B1: The commit budget cannot meet the 100-commit cap (ground rule 19).**
- **Claim:** "One commit per file per change kind", 60 lane commits, about 90 in total.
- **Evidence:** In the findings list, G2 sits in 106 distinct files and G3 in 142 (207 files between them). G10 and G7a add 28 more files. Each K1 fix is its own red-first commit. Per file and per kind, the lanes alone come to roughly 270 commits or more. The rule also cannot hold for helpers: `cgApp` has 41 direct callers in about 12 files (GitNexus `impact`). A per-file rename commit does not compile.
- **Why it matters:** As written, the plan breaks the one-PR, ≤ 100-commit rule.
- **Fix:** Group commits by package × change kind; a helper rename and all its callers go in one commit. Each design note carries a commit column with a hard ceiling per lane. The coordinator projects the total before G1, and a re-cut is ready if the projection passes about 95.

**PA-B2: The changed gate rows do not meet their real tool before G1 (sprint model §10, the 2026-11-15 rule).**
- **Claim:** W0 builds the naming check after G1.
- **Evidence:** The check extends `testhygiene` (tiers T1–T4) and `pytests` (T2–T4), so both rows change. The rule requires running the real tool with the decided config on the real tree at every tier the row lands in, before G1, with what it finds going into the design.
- **Why it matters:** Only that run shows the word list's false reds and the survey's misses before the names are fixed in the design.
- **Fix:** Before G1, run a prototype of the word list over every tracked test file, including `gate/proofs/drivers/` and `docs/language/checks/`. Reconcile its hits with G3's 258 findings, one by one. The design note then fixes the list and the tiers.

### Should-fix

**PA-S1: The ownership table contradicts the lane counts.**
- **Evidence:**
  - G2-16, G2-17, G3-26..29 (`test_aging.py`), G3-61..64 (`test_parts.py`) and G2-29 (`test_sprintid.py`) are in coordinator-owned files but counted in lane A1.
  - G2-147..153 and G3-254..258 (`tools/archcheck`) and G2-154/155 (`tools/astequal`) are in W0-owned files but counted in lane E.
  - G10-9 is in `rows/pytests.py`, which §4 gives to W0 py and takes out of A1.
- **Why it matters:** Ground rule 17 (one owner per file). As written, these findings have no lane that may fix them.
- **Fix:** Assign each of these findings by id.
  - The archcheck and astequal findings move into W0. G2-154 (astequal's error rows check no stderr) is fixed before the rename map is built on top of those tests.
  - The coordinator's files are fixed in the G0 re-cut commit (see PA-S2).

**PA-S2: The G0 re-cut creates new sprint-named tests, and sprint-domain tests will be red.**
- **Evidence:**
  - Re-deriving `test_the_bound_is_s1_1c_7` and `test_a_forward_past_s1_1c_7_is_red` (`test_parts.py:103, :108`) and `test_at_s1_1c_7s_close…` (`test_aging.py:530`) produces S1.1c-9 names.
  - Gate tests whose subject is a sprint id will hit the word list: `test_s1_1c_10_does_not_match_s1_1c_1`, `test_the_marker_at_s4_1…` and `test_the_marker_at_s9_2_expires_all_nine`.
- **Fix:** The re-cut commit uses intent names. The design note states a rule for tests where a sprint id is the data: name the property, and keep the ids in the body. Add counter-examples for these cases.

**PA-S3: The naming check's reach and tier do not match the plan's claims.**
- **Evidence:**
  - `testhygiene`'s `expected_scope` excludes `gate/proofs/**` and `bench/**`.
  - `pytests` excludes `gate/proofs/drivers/**` (`gate/law.json`). `docs/language/checks/test_*.py` is in neither row.
  - Proof drivers do carry G3 findings (G3-22, G3-23).
  - Neither row runs at T0. The owner's check ("commit `breakit_s9_9_test.go` and see it refused") would pass the commit: `testhygiene` first runs at push (T1), `pytests` at T2.
  - `testhygiene`'s `code` list names only `tools/archcheck/main.go` (`law.json:678`). There are no planted-proof cases for the new check (`test_testhygiene_planted.py` has 2 cases, `test_pytests_vacuity.py` 6; `teeth.json` says every row gets a planted proof).
- **Fix:**
  - Name the scan set explicitly, drivers and docs checks included.
  - Either add T0 (a row change, so a law edit) or reword the owner's check to "push".
  - Add the planted-proof cases and update their `law.json` counts and `code` lists (coordinator).

**PA-S4: The word list's matching rule is inconsistent with its own patterns and counter-examples.**
- **Evidence:**
  - The boundaries include `_` and digit runs, yet the patterns contain both (`s\d+_\d+`, `wave\d`).
  - A case-change boundary splits `BreakIt` into `Break` and `It`, so counter-example 11 would be green. `class BreakIt` exists three times in the tree.
  - `lane_[a-z]` misses CamelCase `LaneA`.
  - Case sensitivity is not stated.
  - The recommended Q1 lets review ids with a digit through: `test_qb1`…`test_qb6` (about 20 tests) and `test_bk2_3`. `TestFix…` (G3-254) is green by design.
- **Fix:**
  - Match against a lowercase token sequence, where each pattern is itself a token sequence and adjacent tokens may be joined.
  - Add a prefix-plus-digit class (`qb|bk|pa|pq` followed by `\d+`); requiring the digit keeps false reds low.
  - Write a counter-example per pattern.
  - Keep one word list in one data file read by both checks, instead of two constants pinned equal across Go and Python.

**PA-S5: The rename proof is under-specified and could pass falsely.**
- **Evidence:**
  - `astequal`'s keys carry a context (directory, package name, "test", GOOS/GOARCH suffix, build constraint) and a kind prefix such as `func `, `type `, `var ` (`tools/astequal/main.go:8-13`; `decls.go:22-28, 91-103`).
  - A rename also changes every caller's body. Text substitution in the canonical form cannot see capture: a new name shadowed by a local in some scope, or a field or method with the same name, prints equal while the meaning changes.
  - Subtest names are string literals (6 G3 and 11 G2 subtests).
  - G3 moves Python tests between modules, so `pyastequal` and the unittest ids need a module, class and method key form.
- **Fix:** The design note specifies:
  - the map's key grammar;
  - substitution by resolved object (go/types `Uses`/`Defs`);
  - a capture refusal (exit 2);
  - how subtest literals are mapped;
  - the Python key form;
  - counter-examples for capture, shadowing and method-versus-function.

**PA-S6: The change kinds do not cover several kinds of finding.**
- **Evidence:**
  - (a) G3 proposals that fold or merge test functions, or delete overlapping tests (G3-1, -7, -12, -19, -255/256), are not pure moves. K4's astequal fails and the test list changes. This is G8 duplication work.
  - (b) G2 hygiene findings have no plant that the old test misses: G2-106 and G2-134 (missing `t.Parallel`), G2-131 (`t.Fatal` on a spawned goroutine), G2-109 (an unguarded index).
  - (c) G7a-2 and G7a-6 are comment-only: K1 needs a failing test, and K5 forbids a comment change on its own.
  - (d) `mutate.py` exits 1 for green, invalid and timeout alike, and accepts only top-level Test names (`mutate.py:13-15`).
- **Fix:**
  - S1.1c-7's G3 work is limited to renames and whole-declaration moves; folds go to S1.1c-8, or need K2's deletion proof.
  - Add a hygiene kind with its own proof, and a kind for comment-only text users see.
  - The "old test passes" half of K2 reads mutate.py's outcome line, not its exit code.

**PA-S7: There are hidden couplings to names and paths beyond §0.**
- **Evidence:**
  - Registry `-run` anchors: `^TestCorpus$`, `^TestBaseline$`, `^TestTripwire$`, `^(TestDeterminismGolden|TestBaseline)$` (`gate/registry.json:869, 903, 963, 995, 1027`).
  - Driver anchors:
    - `CORPUS_TEST` and `PLANT_SWITCH` (`test_tripwire_planted.py:16-17`, `test_determinism_macos_planted.py:15-16`);
    - `FUZZ_FILE` (the event and parser `fuzz_test.go`);
    - `BASELINE_TEST` (`test_regen_baseline.py:30`);
    - `TestCorpus` and `WANT_FILES` (`test_expect_planted.py:38, :98`).
  - Five canary pin ids in `gate/canary/pin_tests.json`.
  - `law.json`'s `paths` list 14 test files one by one; renaming one turns the liveness check red. G3-144, G3-145 and G3-153 touch the parser's contract and fragment files.
  - The proofs row's `min_wants` 39 equals the 39 drivers, so there is no headroom.
  - G3-201 splits the corpus file into three (lane B, W1), while the drivers that anchor that single file belong to A2 in W2.
- **Fix:**
  - Add one coupling table to §0: each anchor, its owner, and the rule that a rename lands together with its anchor edit, in one coordinator commit on the lane's branch.
  - In S1.1c-7, the corpus file is renamed only, not split.
  - Run the proofs row after each such commit (the 2026-11-12 rule).

**PA-S8: The LAW Δ block will be far larger than the owner's check suggests.**
- **Evidence:** `gate/` and `tools/` are whole law paths of kind `code`, so every A1, A2, W0 and tools edit (about 160 findings) shows in LAW Δ.
- **Fix:** Say so in the owner's check, and give a reading aid: the record's ledger keyed by LAW Δ path.

**PA-S9: G10 and G7a scope needs correcting and routing to the owner.**
- **Evidence:**
  - G10-6 is the same defect as G1-1, already fixed in 363e5f6, so 22 remain open.
  - G10-22 (export `ErrInvalidTable`) and G10-15 (`Register` refusing descriptors) change public API.
  - G7a-1, -3 and -5 change error text users see. G7a-3's own entry says this needs the owner's yes (ground rule 3), and the 2026-11-13 rule requires a dry run through the tests that pin the text (`runnable_test.go:22-26`, `deploy_test.go:246`, `coverage_s3_3_1_test.go:84`).
- **Fix:** P1 lists each fix that changes an exported surface or user-visible text, with its new wording and the dry-run result, for the owner at G1.

**PA-S10: R5 wrongly calls G10-21 caller-side.**
- **Evidence:** GitNexus shows `gate` is called by `Send` and `SendPayload`. The `AfterDrain` count sits in `ingest` (`router_in.go:36-37`). All three are among S1.1c-6's 31 hot-path functions.
- **Fix:** The design note decides per fix option. If the chosen fix touches one of those functions, the decision-1 timing rules apply, on mains power.

**PA-S11: The frozen benchmarks' world is broader than their names.**
- **Evidence:** `hotpath_bench_test.go:3-6` freezes "names and worlds". Its `hpApp` calls `cgApp` (GitNexus). G3-219 renames ten `cg` helpers, and G2-131 changes `cgApp`'s behaviour.
- **Fix:** Say explicitly that a pure, astequal-proven rename inside the world is allowed, and that a behaviour change must leave `hpApp`'s path unchanged.

**PA-S12: The 13 SP size and the lanes' "≤ 8 days" are not measured.**
- **Evidence:** S1.1c-6 was carded at 13 and built at about 19 for 91 findings (6602ee3). This sprint has 443 findings, four new tools and concurrency defects (G10-1, -5, -20, -21).
- **Fix:** A time-boxed spike, as S1.1c-6 had (ground rule 8): one package's G2 and G3 (for example `internal/clock` or `io/inmemory`) plus one K2 plant. Project the lanes from it before "fits the cap" is ticked.

**PA-S13: With the check off until the last lane, nothing protects the tree and no lane proves its own work.**
- **Evidence:** A lane can add a new sprint-named test or miss a rename, and this surfaces only at combine. "Off for the real tree" also needs a switch, and flipping it is a law change.
- **Fix:** Each lane's hand-back includes the check run on its own packages with 0 hits. Name the switch mechanism in the plan.

**PA-S14: The re-cut and W0 have details the plan does not list.**
- **Evidence:**
  - `parts.py:4`, the note in `smells.json` ("no later than S1.1c-8") and the reasons on the three `tools/archcheck` entries ("S1.1c-7 cleans it") all name the old bound.
  - S1.1c-5-3's `decision` must point to a section that names the id. Aging rule 1 (`aging.py`, `decided`) goes red once more than two closed rows follow `first_seen`, which happens when S1.1c-8 closes.
  - The TEMP entries expire on 2027-02-11, a hard date for S1.1c-9.
  - `tools/archcheck/main.go` has 1756 lines (gocyclo cap 19, funlen cap 1). Any new function over a limit is red at T0.
- **Fix:** Q8 lists all of these edits. W0's brief keeps new archcheck code inside the limits, or puts it in a new file that joins `law.json`'s `code` lists.

**PA-S15: Question for the owner, not a decision — S1.1c-8's size.**
- **Evidence:** S1.1c-8 carries 996 findings and is shown as "≈ 13", the same size as this sprint's 443. It also includes 41 public renames that the owner approves one by one.
- **Fix:** Mark S1.1c-8 as unsized until it is measured with the same spike method. Ask whether the three-part cut still holds if that projection comes out over the cap.

### Later

- **PA-L1:** The lane table's headers leave out `model` and `text` (lane D) and `conformance` (lane C). The `CORPUS_TEST` line in §0.4 should be :15 for the determinism driver.
- **PA-L2:** Move the proof and timing tool fixes (G10-3 `mutate.py`, G10-2 `escapes.py`) into W0. Otherwise W1's proofs and W2's proofs run on different versions of the tool.
- **PA-L3:** K3's "coverage per package unchanged" adds nothing once astequal exits 0 for a pure rename. Drop it, per the cheapest-proving-run rule.

### Verdict

**Not ready for the owner; revise to v2.** The overall cut is sound and gradual: correctness first, the proof tools in W0 before any lane, lanes by package, every finding re-checked, and blind QA spot-checks between waves. Two things block it:
- the commit budget cannot meet the 100-commit cap (PA-B1);
- the naming check skips the required real-tree run before G1 (PA-B2).

Behind them, the proofs and the ownership need the most work:
- the rename proof needs object-based substitution and a capture refusal (PA-S5);
- several finding shapes have no change kind (PA-S6);
- three ownership gaps (PA-S1);
- the couplings to registry patterns, proof drivers, canary pins and law paths, including a cross-lane, cross-wave seam on the corpus file (PA-S7).

The size claim (PA-S12) and S1.1c-8's projection (PA-S15) also need measuring before "fits the cap" is ticked.

## Round 2: v2, blind re-review (ids PA2-*)

### What I did
- **Read:** CLAUDE.md, ground-rules.md, and sprint model §6 and §10. In the scratchpad `s17/` directory: the plan (v2, all 655 lines), review-plan-arch.md (PA-), lanes.py (I ran it) and nameproto.py with its output. Code I read: gate/runner/{parts,aging,allowlists,sprintid,teeth}.py, rows/{pytests,pyfloor,ruff,aging}.py, gate/bench/{mutate,load}.py, runner/stress.py, proofs/drivers/{support,test_aging_planted}.py, tests/{test_parts,test_aging,test_sprints_pin,test_instruments_pin}.py, registry.json, law.json, known-issues.json, sprints.json, tools/allowlists/*.json, conformance/{notbuilt.json,corpus_test.go}, and Go 1.27.4's vendored testinggoroutine analyzer.
- **Checks:** HEAD is 5b4ddac.
- **GitNexus:** 5 calls. `context`: `gate` (ambiguous, then by uid), `hpApp`, `AppHandle.Start`. `impact`: `checkEvent` upstream.
- **Could not check:**
  - I could not read the freshness resource, because there is no resource-read tool in this session. The brief says the index is fresh at 5b4ddac.
  - No Hindsight tools were available, so I made no memory searches.
  - `detect_changes` does not apply: this is a plan, not a diff.
  - I did not open the-fool's skill file. I ran a pre-mortem by hand on the G0 order and on the proof tools.
- **Grep-only claims** are marked (grep).

No blockers on ground rule 15's scale. Several should-fix items: each one would turn a prescribed run red, make a proof unable to fail, or rest on a wrong fact.

### Should-fix

**PA2-S1: The G0 order turns T4 and the proofs row red after the marker commit.**
- **Claim (§12 item 6):** "2. The marker → S1.1c-7. Then `--job proofs`." The re-cut comes two commits later.
- **Evidence:**
  - All 13 size exclusions name S1.1c-5-3, whose `forward` is S1.1c-7 (`gate/known-issues.json`).
  - `parts.forwards` puts a fault in the faults list once `reached_part(forward, marker)` is true at a red tier (`gate/runner/parts.py:118-123`). Those tiers are T4 and nightly (`RED_EXPIRED`, `allowlists.py:34`). `rows/aging.py:81` runs it with the row's tier.
  - The proofs driver `test_aging_planted.py:122-129` (`test_the_real_tree_ages_green`) runs the aging row at T4 on a copy of the real tree. Its four red cases also assert "every failed line is about it" (`:93-100`).
  - So the proofs run the plan prescribes right after the marker fails five cases, and T4 stays red until the re-cut lands. The 2026-11-12 rule forces exactly that run after a marker move.
  - T0 (gofmt, archcheck, ruff, smells) does not see any of this.
- **Also missing from the re-cut:** `docs/sprints/sprint-plan.md`'s §3 table has to gain S1.1c-8, -9 and -10 in the same commit as `gate/sprints.json`. Otherwise `test_sprints_pin.py:24-27` and `test_sprintid.py:138` are red at T2. The current S1.1c-7 row text (size limits and names, `sprint-plan.md:63`) also needs rewording to the correctness scope.
- **Fix:** Order G0 as: docs commit → coordinator G2 fixes → re-cut (sprints.json **with** sprint-plan.md §3, known issue, LAST_PART, smells.json, test_parts.py) → marker → `--job proofs`. Or put the re-cut and the marker in one commit. Re-cut before marker is green at every tier: with the marker still at S1.1c-6, S1.1c-9 is not reached, rule 5 is met by the new decision, and the marker's row is still a part of the first open row S1.1c.

**PA2-S2: A wrong count is marked "re-checked", and a nearer TEMP expiry is missing from R6.**
- **Claim:** "the 31 TEMP size exclusions" (programme table, §0 item 10, R6). §10's re-check table says "31 reasons … yes".
- **Evidence:**
  - `tools/allowlists/smells.json` has **13** entries, all TEMP, all S1.1c-5-3, all expiring 2027-02-11. `git grep -c "S1.1c-7 cleans it"` returns 13.
  - `tools/allowlists/vulns.json` has **15** TEMP entries expiring **2026-12-14**: golangci-lint's stdlib advisories, "renewed only by an owner decision".
  - The suppressions row reports `expired` at T4 and nightly (`allowlists.py:194-203`). So main's nightly goes red on 2026-12-15, during S1.1c-8 or late S1.1c-7.
- **Fix:** Correct the count to 13 throughout §10. Add an R6 line for 2026-12-14 (renew by owner decision, or a golangci-lint build on go ≥ 1.27.2), with a date the owner sees at G0.

**PA2-S3: The subtest feature in `mutate.py` can report a false red, and its green rule is not defined.**
- **Claim (§2):** "Red is then the top-level `--- FAIL: TestX (` line with only that subtest run."
- **Evidence:**
  - `judge` matches only the top-level FAIL and PASS lines (`gate/bench/mutate.py:162-166`).
  - With `-run '^TestX$/^name$'`, the parent's own setup still runs. A plant that breaks that setup gives `--- FAIL: TestX (` although the designated subtest never ran. That is red credited to the wrong assertion: a K1 revert plant or K2d "new red" passes on it.
  - Under the current rule, a skipped subtest, or a subtest name that matches nothing, gives the parent's PASS line, which reads "green". The plan wants counter-example #3 to read invalid, but the stated rule gives green.
  - `^name$` also breaks on names with `/`, spaces (Go rewrites them to `_`) or regex metacharacters. The plan's own example is `"F3/watermark"`.
- **Fix:**
  - Red requires the subtest's own `--- FAIL: TestX/<name> (` line. Green requires its own `--- PASS: TestX/<name> (` line.
  - A parent FAIL without the subtest's FAIL line, a SKIP, or no RUN line is **invalid**.
  - Build the pattern per level with Go's name rewriting and `regexp.QuoteMeta`.
  - Add counter-examples for "parent setup fails before t.Run" (invalid) and for a nested or slashed name.

**PA2-S4: The evidence batch does not fit `mutate.py`'s batch contract, and the Python mode's copy cannot run tests that call git.**
- **Claim (§2):** "The contract is otherwise unchanged." The batch entries carry "id … and the expected outcomes at the old and new revisions". The Python mode uses "`git archive HEAD` into a temp directory … `python3.15 -B -X utf8 -m unittest <id>`".
- **Evidence:**
  - `read_batch` refuses any key outside `{file, find, replace, test, note, pkg}` with exit 2 (`mutate.py:41-42, 130-132`).
  - Batch mode exits 0 only when every entry is red (`:273`). K2a batches hold expected greens, so the committed batches cannot be re-run as they stand.
  - `git archive` gives a tree with no `.git`. About 11 gate test modules run git against the repository (grep: test_instruments_pin, test_aging RealTree, test_law_pin, test_known_issues_pin, test_lawdelta, test_main, …). Those error, so the mode reads invalid for exactly the A1 findings in those files (G2-24, G2-25).
  - The pytests row runs modules under `-I` through its shim. The plan drops `-I`, so a proof can disagree with the gate.
  - K2c's "50/50 under load (`--repeat 50 --load`)" uses the wrong tool. `--repeat` needs a mutation and counts reds, exiting 0 when k ≥ 1 (`mutate.py:251-260`). `gate/bench/load.py` counts passes N/N (`load.py:1-13`).
- **Fix:**
  - Add `id` and `expect` (`{"old": …, "new": …}`) as batch keys, with an exit contract of "0 when every entry meets its expected word". Or define the evidence as a separate schema plus a converter.
  - Copy the tree with a detached temporary worktree or `git clone --shared`, removed afterwards, so git-reading tests run. Or list them as outside `--py` with another proof.
  - Run Python tests under the pytests row's flags and shim.
  - In K2c and K1c, name `load.py` for "N/N passing" and `mutate --repeat` only for "the old code's k/N red".

**PA2-S5: The naming tool's acceptance rests on the prototype's blind spot.**
- **Claim (§3):** Go names come "from column-0 declarations", and "W0-py's tool must reproduce the prototype's hit list exactly on the G0 tree".
- **Evidence:**
  - nameproto.py's `GO_DECL = ^(?:type|var|const) (\w+)` only sees column 0.
  - Test files hold 30 grouped `var (`/`const (`/`type (` blocks in 23 files, with 151 names (grep). My run of the prototype's own tokenizer and the decided rules over those names gives **9 decided hits it misses**: `bkPhaseEvent`, `bkPhaseInclusive` and `bkPhaseStrict` (`internal/clock/breakit_test.go`); `pqSec`, `pqMin` and `pqHr` (`internal/plan/helpers_test.go`); `cnT0`, `cnBefore` and `cnHeader` (`internal/playback/reader_refusals_internal_test.go`).
  - §3 promises "types, vars and consts in test files".
  - The decided scope is a filter of the output (core plus the two decided wide rules: 420 names in 105 files, which I re-derived). The prototype also emits a "design row id" class that is not decided.
- **Fix:** The golden table is the spec, not the prototype. Parse grouped blocks and multi-name declarations, and add golden rows for both. State the decided-scope filter. List the 9 as expected extra hits in the differential. Commit the prototype script with its evidence, because a sha256 of a scratch file cannot be re-run.

**PA2-S6: The coupling table under-reports (PA-S7 is only partly done).**
- **Evidence:**
  - **G7a-3 (CD) and the corpus:** it changes `"text operators (S7)"` (`internal/expr`). `conformance/notbuilt.json:17` carries the same string as `construct`, and TestCorpus reds a wrong construct (`conformance/corpus_test.go:130`; registry `-run ^TestCorpus$`). notbuilt.json is a law path (kind `notbuilt`), and `test_notbuilt_expired.py` copies it at T4.
  - **CORPUS_TEST** is marked "No", but G2-98 (lane B) edits `internal/playback/s3_2b_2_corpus_test.go`. That file is CORPUS_TEST, holds `PLANT_SWITCH`, and hosts `TestTripwire` (`:202`, registry `^TestTripwire$`).
  - **FUZZ_FILE** is marked conditional, but G2-78 edits `internal/parser/fuzz_test.go`.
  - The canary pins also carry a `pattern` (failure text) per id (`gate/canary/pin_tests.json`). A2's body edits can change that text.
  - `copy_gate` copies every tracked gate/ file into every world (`proofs/drivers/support.py:289-299`), so every W0, A1 and A2 commit triggers the 2026-11-12 rule.
- **Fix:** Add rows for notbuilt.json and G7a-3 (text and construct in one commit, LAW Δ notbuilt delta, then proofs). Mark CORPUS_TEST and FUZZ_FILE "Yes" with G2-98 and G2-78. Add the canary `pattern` to the A2 rule. State that every gate/ commit triggers the proofs row.

**PA2-S7: The commit projection ignores the plan's own one-kind-per-commit rule (PA-B1 is only partly done).**
- **Claim:** "one per package for G2" and "one per package for G7a" give ≈ 80 in total. lanes.py reproduces 53 lane commits plus G3-72.
- **Evidence:**
  - §7 says "One kind of change per commit".
  - K2f's proof is "astequal exit 0 apart from the one key", which cannot hold in a commit that also edits test bodies.
  - A covered-set deletion proof is masked by additions in the same commit.
  - The root holds G7a-1 (K1) and G7a-2 (K6), two kinds in one package.
  - From the survey's fix texts (heuristic), renames sit in about 6 more packages and deletions in about 5.
  - Not counted at all: the naming-prototype evidence commit, which needs the post-re-cut G0 tree, and the coupling commits.
  - Realistic total: about 87 to 93, close to the plan's 95 re-cut trigger, with the fix round inside.
- **Fix:** Say whether "kind" means K2 or K2a–K2f. Count package × kind with K2f, K6 and covered-set deletions isolated, re-projected now from the survey, not first at G1.

**PA2-S8: Two plan statements contradict each other on when W0 runs.**
- **Evidence:** §1 says "W0 … run beside the coordinator's G0 work". §8 says "4. … G1. 5. W0 → W1 …". The sprint model §6 says "No code for the parts the design covers before G1", and §5 gives W0 a design note. W0's naming acceptance also needs the G0 tree.
- **Fix:** W0 after G1, as §8 says. Delete "beside the coordinator's G0 work".

**PA2-S9: R5's trigger is too narrow (PA-S10 is only partly done).**
- **Claim:** R5 applies to "a function on S1.1c-6's hot-path list".
- **Evidence:**
  - That list holds only the 38 of S1.1c-6's 110 split functions that sit on a hot path (`s1-1c-6-plan-check.md:18, 82-92`).
  - Per-event functions outside it are missed: `(*Runtime).gate` is called by Send, SendPayload, BatchEnd, Run and TableWrite (GitNexus `context`), and `checkEvent` is called by Send (GitNexus `impact`). G10-21's fix can land in `gate`, and G7a-5 edits `checkEvent`, `checkValues` and `checkValue` (boundary.go:164-226).
- **Fix:** Trigger on "upstream reaches an entry point of S1.1c-6 §3's five per-event paths" (GitNexus `impact`). Error-text-only edits on an error branch can be exempted by name.

**PA2-S10: The K2e proof for G2-131 cannot fail.**
- **Claim (§7.2):** "`go vet`'s `testinggoroutine` before and after (G2-131)". G2-131 is also listed under K2d.
- **Evidence:**
  - For `go func(){ cgApp(t, …) }()`, the analyzer inspects only the go statement's own region for a forbidden `t.` call. It does not descend into `cgApp` (`$GOROOT/src/cmd/vendor/golang.org/x/tools/go/analysis/passes/testinggoroutine/testinggoroutine.go:185-203, 91-127`). The finding itself says "go vet does not see through the helper".
  - The result is clean before and after.
- **Fix:** Use a mechanical AST check: no call taking the test's `t` inside a `go` statement body in the two files. Show it red on the old code. Pick one sub-kind for G2-131.

**PA2-S11: G2-24 runs against a recorded decision.**
- **Evidence:** G2-24 says "Judge every row instrument" (`test_instruments_pin.py:217-244`). decisions.md:1012 says "The other instruments' gaps go to the backlog … Rejected: widening all 30 instruments' code lists in this sprint." Widening them also edits `gate/law.json`, which the coordinator owns.
- **Fix:** Route it now. Either take the finding's own variant ("list the known gaps by name", A1's file only) or ask the owner whether the 2026-11-20 "nothing to the backlog" decision supersedes the 2026-11-14 one.

### Later

- **PA2-L1: R4's premise is wrong, and it misses two bench edits.**
  - G2-131's fix builds the App before the `go` statement. It does not change `cgApp`.
  - `hpApp` calls `cgApp` only at setup: its callers are `hpStart` and `hpIg` (GitNexus).
  - R4 should cover G2-69 (an input in `decode_bench_test.go`, whose header says each benchmark "keeps its name, so a split commit's measure is comparable") and G2-105 (`encode_bench_test.go`). Each should land in its own commit, never with a hot-path change that `compare.py` measures.
- **PA2-L2: The reason given for naming S1.1c-5-3 is wrong; the action is right.** §0 item 10 cites aging rule 1. The current decision already names S1.1c-5-3 (`knownissues.names(section) == True`), so rule 1 never fires. The real constraint is rule 5 (`parts.py:170-176`): a later forward needs a decision that names the id **and** is not the base's anchor. Say "a new section".
- **PA2-L3: The ledger's 185 is ambiguous.** §1's 185 excludes G10-6 and includes G3-72; lanes.py's 185 is the reverse. §7.5 adds G10-6 as fixed-elsewhere, which makes 186 rows. G3-61..64 ("fixed-elsewhere") belong in S1.1c-8's ledger. The ledger checker's location (committed or not) is unstated.
- **PA2-L4: File placement.**
  - `tests/testnames_golden.json` breaks the layout. pytests.py:61 says a tests directory holds test_*.py, `__init__.py` and `support.py`, and fixtures live in `gate/runner/testdata/`.
  - The word list under `gate/runner/` is law kind `code`, so LAW Δ shows only line counts for a rule change. Decide its home now (a `gate/*.json` with its own law kind in S1.1c-8) to avoid a move.
  - Keep `testnames.py`'s matcher pure (names in, verdicts out), with the listing in `main`.
  - Checked: no existing check refuses the module or its test. Misplaced, collisions, teeth, pyfloor (fixed scope), law liveness and archcheck/testhygiene (Go only) all pass it; ruff (T0) applies as normal. No row scans `docs/**/*.json`: doclinks reads `*.md` only, and the aging and suppressions drivers copy only `.md` from docs.
- **PA2-L5: A proposed name claims more than its test checks.** `test_the_bound_is_the_last_printed_size_part` still asserts a constant. Name what it checks.
- **PA2-L6: K6's astequal proof is blind to comments that carry meaning.** It ignores comments, including an Example's `// Output:` block. Add "no `// Output:` line changed" (git diff) for G7a-2 (example_test.go:14).
- **PA2-L7: Old decisions need annotations.** decisions.md:956 and :1075 record the bound "never past S1.1c-7". Moving LAST_PART to S1.1c-9 needs those lines annotated (ground rule 4); the docs commit does not list it.
- **PA2-L8: A1's row fixes and two possible seams need naming.**
  - A1's row fixes (G10-7..11) change gate rows' error paths. Say why the 2026-11-15 rule does not apply (scope, tiers and config unchanged), and that the asserted texts in A2's planted drivers (e.g. `test_vulns_planted.py:435-630`) must not change without the coordinator.
  - G10-1's option (a), "write the rows as part of activation", edits `internal/runtime` (E1, W2) from E2 (W3). The design notes should choose an option before W2, or name the seam (§6 step 0).

### PA-* dispositions I judge not fully done

| Id | Why |
|---|---|
| PA-B1 | The projection breaks the plan's own one-kind-per-commit rule; the real figure is near the 95 trigger (PA2-S7) |
| PA-B2 | The prototype is made the oracle and misses grouped declarations; "reproduce exactly" makes the miss permanent (PA2-S5) |
| PA-S7 | The table misses notbuilt.json and G7a-3, says "No" for CORPUS_TEST although G2-98 edits it, leaves FUZZ_FILE conditional although G2-78 edits it, and omits the canary `pattern` (PA2-S6) |
| PA-S10 | R5 keys on a list scoped to S1.1c-6's splits, missing `gate` and `checkEvent` (PA2-S9) |
| PA-S11 | It guards a change the finding does not propose; the real bench edits (G2-69, G2-105) are not covered (PA2-L1) |
| PA-S14 | 31 should be 13; sprint-plan.md §3 is missing; the marker-first order turns T4 red; the reason given is rule 1 when the constraint is rule 5 (PA2-S1, S2, L2) |

PA-S6(d) and PQ-S1 are carried into a `mutate.py` contract that still needs PA2-S3 and PA2-S4.

### Verdict

**Not ready for G0 as written, but close.** The shape holds up:
- the correctness-first cut, lanes by package, ownership by id (lanes.py reproduces the §1 table exactly);
- W0's tools before any lane, the naming tool kept off the gate in this sprint;
- deletions with proof, as the owner answered.

Change these first:
1. Reorder G0 so the re-cut, including sprint-plan.md §3, lands before or with the marker (PA2-S1).
2. Correct the 31 → 13 count and add the 2026-12-14 vulns expiry (PA2-S2).
3. Tighten `mutate.py`'s subtest judging, batch schema and Python copy (PA2-S3, S4).
4. Make the golden table the naming tool's spec, with grouped declarations parsed (PA2-S5).
5. Complete the coupling table (PA2-S6).
6. Re-project commits by package × kind (PA2-S7).
7. Settle when W0 runs (PA2-S8), widen R5's trigger (PA2-S9), replace the vet proof that cannot fail (PA2-S10), and route G2-24 (PA2-S11).

All of these are plan-text edits. A short v3, or a dispositions addendum, is enough. A narrow re-check of the new G0 order and the `mutate.py` contract would close it.

**Files:**
- Plan: s1-1c-7-plan-check.md
- Code: gate/runner/parts.py, gate/proofs/drivers/test_aging_planted.py, gate/runner/tests/test_sprints_pin.py, tools/allowlists/smells.json, tools/allowlists/vulns.json, gate/bench/mutate.py, gate/bench/load.py, conformance/notbuilt.json, conformance/corpus_test.go, internal/playback/s3_2b_2_corpus_test.go, docs/sprints/decisions.md

## Round 3: v3, narrow re-check by the same reviewer (ids PA3-*)

**Verdict: ready for G0 once PA3-S1, PA3-S2 and PA3-S3 are fixed in the plan text.** None of the three blocks G0 on ground rule 15's scale; all are edits to the plan, and no re-review is needed. The G0 order is now green at every tier.

### What I did
- Read v3 §0–§8, §10's second table, §11 and §12 at s17/s1-1c-7-plan-check.md, and re-ran lanes.py.
- Checked against the code at main 5b4ddac: gate/runner/{parts,knownissues,pyshim}.py, tests/{test_parts,test_known_issues_pin}.py, docs/product/known-issues.md, the updated nameproto.py and its output.
- GitNexus: 1 call this round (`impact` on plan `(*body).OnEvent`, upstream), 6 in total across both reviews.
- No file writes, no gate runs, no go test. Claims based only on grep are marked (grep).

### 1. The G0 order (§12): every tier stays green after each commit

**Commit 1, the docs commit.** No T0 row reads Markdown. doclinks passes because the survey files land in the same commit. The aging row is not reached: the forward is still S1.1c-7 and the marker is still S1.1c-6.

**Commit 2, the marker and the re-cut together:**
- **T0:** ruff on the four edited `.py` files. The smells row does not read the marker, and the reason text is free.
- **T2:**
  - `test_sprints_pin`: the table and sprints.json are edited together. The marker's row is S1.1c, which is the first open row (`sprintid.py:209-213`).
  - `test_sprintid.py:138`: both sides are live and edited together.
  - `test_known_issues_pin`: S1.1c-9 is now a printed row, and the new section resolves and names the id.
  - `test_aging` RealTree: one closed row since S1.1c-5. Aging's forward rule returns nothing for an open row (`aging.py:59-61`).
  - The two live pins (G2-17, G2-29) become fixtures.
- **T4:** `reached_part(S1.1c-9, S1.1c-7)` is false, and S1.1c-9 is not past LAST_PART (`parts.py:111-123`). Part rule 5 sees the move against main and is satisfied by the new section, which is not the base's anchor (`parts.py:170-176`). `test_aging_planted` passes because its world has its own merge base.
- **No other test pins the bound or smells.json's reasons** (grep).

**Commit 3 (G2-16, G2-25).** Both are K2e in one package. test_instruments_pin.py is a law path and a test file that the proof worlds copy, so the proofs row follows, as §0 item 5 says.

**Commit 4, the evidence commit.** No row scans `.py` or `.json` files under docs/. ruff and suppressions only take `gate/**/*.py`; pytests walks `gate/` only.

What is missing from commit 2 is PA3-S3 and PA3-L1 below.

### 2. `mutate.py`'s contract (§2)
It fits the existing code:
- Single mode and plain batches keep their meaning.
- New keys extend `ENTRY_KEYS`, and an unknown key is still refused (§6 #18).
- `REQUIRED_KEYS` has to become per-mode (patch or py instead of file/find/replace/test). That is an implementation detail.
- The only caller is `gate/bench/tests/test_mutate.py` (grep).
- The shim can run a single test id: `loadTestsFromName` accepts a method id (`pyshim.py:90`).
- The subtest judging closes PA2-S3.

One way a proof can still read falsely remains (PA3-S2).

### 3. The coupling table and R5
The coupling table is complete apart from PA3-L5. R5 is not complete (PA3-S1).

### 4. The commit projection
lanes.py gives 186 ids and 44 lane commits by package × kind. The budget table adds up to 80. The ceilings are mostly sound, with one gap (PA3-L3).

### Should-fix

**PA3-S1: R5's call-graph trigger cannot see interface dispatch (PA2-S9 is still open).**
- **Claim:** a fix whose GitNexus upstream reaches a per-event entry point is K1h.
- **Evidence:** `impact` on `internal/plan/body.go:body.OnEvent` returns 0 callers, risk UNKNOWN. The shard calls it through the `partition.Body` interface, which the index does not link.
  - So per-event rule evaluation would read as "not per-event": `OnEvent`, expr's `Eval`s, partition `keyOf` and `Offer`. All of these are on S1.1c-6 §3's list.
  - R5's own list of entry points also leaves out the facade's `SendRow`/`Emit`/`Upsert`/`Delete` and `(*Decoder).Decode`.
  - No current finding is known to hit this gap; the rule itself is wrong.
- **Fix:** K1h = GitNexus upstream reach **or** membership in S1.1c-6 §3's function list **or** being reached downstream from §3's entry points. Name all five §3 paths' entry points. An UNKNOWN or zero upstream counts as a yes until the note shows otherwise.

**PA3-S2: In the Python mode, the plant is visible to git.**
- **Evidence:**
  - The Go mode builds through `-overlay`, so git never sees the mutant (`mutate.py:180-201`).
  - `--py` writes the plant into the detached worktree. `git status`, `git diff` and lawdelta-style dirty checks in a test then see it, which can turn red or invalid for a reason that is not the plant's behaviour. §6 #13 covers only `ls-files`.
  - The shim needs a third argument, the report file (`pyshim.py:3, 54-58`); the plan's command omits it.
  - The shim folds every problem into one line (`pyshim.py:101-120`). A "1 failure" together with a leak or a skip needs a stated precedence.
- **Fix:**
  - State the limit: an id whose verdict reads working-tree git state needs another proof, or the note shows the plant is invisible to it. Add a counter-example with a test that asserts `git diff --quiet`.
  - Pass the report file to the shim.
  - Red only when the shim's sole problem is "1 failures, 0 errors". Any other problem is invalid.

**PA3-S3: Commit 2 leaves the known-issues table stating the old forward.**
- **Evidence:** `docs/product/known-issues.md:13` (S1.1c-5-3) has Forward "S1.1c-7" and "none may remain after S1.1c-7".
  - The pin compares only the Found cell, the opened date and the correctness word (`knownissues.py:148-162`), so nothing goes red.
  - After the re-cut, the document contradicts `gate/known-issues.json` (ground rules 21 and 22).
- **Fix:** Add the row (Forward → S1.1c-9, the sentence, and the move history with the new decision link) to commit 2.

### Later

- **PA3-L1: The test fixture needs a part past the new bound.** `test_parts.py`'s fixture `ORDER` (`:18-31`) prints parts up to S1.1c-9 only. The re-pinned "a forward past the bound is red" test needs S1.1c-10 in the fixture; otherwise the fault reads "not a printed part" instead of "past S1.1c-9" (`parts.py:105-112`).
- **PA3-L2: Commit 2 mixes kinds.** It holds the marker, data, a production constant, re-pins and two K2e fixes. G2-17 and G2-29's switch to fixtures is green on both trees, so it can land first as its own K2e commit. Commit 2 then has one purpose, and the K2e proof measures a clean commit.
- **PA3-L3: The per-lane ceilings can pass 95 together.** "Line plus 2" across eight lanes allows 44 + 16 lane commits, so 96 overall, above the 95 trigger. Add a running-total check at each wave hand-back: committed so far plus remaining projection ≤ 95.
- **PA3-L4: G7a-5's kind is inconsistent.** R5 names G7a-5 as a known K1h case, but lanes.py classes it K1 (`K1H = {'G10-21'}`). Pick one; the error-text exemption suggests K1.
- **PA3-L5: A hand-copied construct goes stale.** `gate/runner/tests/test_notbuilt.py:20` hand-copies `"text operators (S7)"`, and A1 owns the file. Update it in G7a-3's coordinator commit, or it becomes a stale copy (grep).
- **PA3-L6: Two `--expect` details are unnamed.** The flags that give the old and new revisions are not named. "Per-revision `test`/`subtest`/`py`" lists only `old_test`/`new_test`; name the subtest and py variants too.

### PA2 items

| Id | Status |
|---|---|
| PA2-S1 | Closed. The order is green at every tier; leftover items are PA3-S3 and L1. |
| PA2-S2 | Closed (13; R6 with Q11). |
| PA2-S3 | Closed. |
| PA2-S4 | Mostly closed; the rest is PA3-S2. |
| PA2-S5 | Closed. Verified: the prototype now gives 429 names in 105 files and catches `bkPhaseEvent`, `pqSec`, `cnT0` and `cnHeader`; the lane rule is `lane_[a-z]\d?`. |
| PA2-S6 | Closed, apart from PA3-L5. |
| PA2-S7 | Closed; the rest is PA3-L3. |
| PA2-S8 | Closed. |
| PA2-S9 | **Open** (PA3-S1). |
| PA2-S10 | Closed. |
| PA2-S11 | Closed by the owner (Q10). |
| PA2-L1 to PA2-L8 | Closed. |
