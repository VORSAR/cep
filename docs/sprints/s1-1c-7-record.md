# S1.1c-7 — sprint record

**Sprint:** S1.1c-7 — correctness and truthful tests: every behaviour defect the survey found, sprint ids in text users see, and every test that does not test what it claims · **Routine:** the full loop without the semantics panel · **Size:** about 39 SP (S1.1c-6's rate) against the 2-week cap · **Branch:** `s1-1c-7/integration`
**What to build and how each item is proven:** [the plan check §12](s1-1c-7-plan-check.md#12-the-card) · **How it runs:** [the plan check §1](s1-1c-7-plan-check.md#1-routine-shape-waves) · **Process:** [00-sprint-model.md](00-sprint-model.md)

*This record holds the plan check, the owner's decisions at the gates, every proof as it is made, the finding ledger, the done-done checklist and the retro. Status stays in code and git.*

---

## Plan check (step 0)

The full plan is [s1-1c-7-plan-check.md](s1-1c-7-plan-check.md) (v3), drafted read-only from `main` @ 6602ee3, then 5b4ddac.
- **The survey before it** ([summary](../reviews/s1-1c-7-survey.md), [every finding](../reviews/s1-1c-7-survey-findings.md)): 650 source files read in full by fresh `refactoring-specialist` agents with GitNexus; 1,442 findings in twelve groups. The three gate holes (G1) went up as PR #51 (`main` a2a63be, 5b4ddac). The committed findings list's sha256 is `b7fce96229f55e94c97d9f96750ee0bf17df717c1cb6d14d341554d61c069520`; the summary's is `d30b4f523c934b5677d93af2c962ca2e529ba4399a7a76820f144379b3193dc2`.
- **Three rounds of blind reviews** ([architecture](../reviews/s1-1c-7-plan-check-architecture.md), [QA](../reviews/s1-1c-7-plan-check-qa.md)):
  - v1: 2 blockers each;
  - v2: one blocker (QA) and 28 should-fix between them;
  - v3's narrow re-check: no blocker, seven should-fix, all written into v3.

  Every finding's disposition is in the plan check's §10. One was rejected, with its reason (PA3-L2).
- **What the plan did not know at first** (§0):
  - the 443 findings first placed in this sprint needed about 97 commits before a fix round;
  - `mutate.py` reads only top-level Go tests, with no expected outcomes and no Python mode;
  - the gate couples to test names, paths and texts in ten places;
  - 13 TEMP size exclusions expire on 2027-02-11, and 15 TEMP vulns entries on 2026-12-14.

## Decisions at the gates

**The survey — the owner, 2026-11-20** ([decisions](decisions.md#owner-decisions-2026-11-20--s11c-7s-survey)): every source file read by parallel agents as the sprint's first step, up to six agents for that read-only job only; everything wrong reported; the groups placed at the plan check; exported renames each need the owner's yes; the gate holes as a fix PR at once; both survey documents committed.

**The plan check — the owner, 2026-11-20** ([decisions](decisions.md#owner-decisions-2026-11-20--s11c-7s-plan-check); twelve answers, three not the recommendation):
1. **The re-cut into four:**
   - S1.1c-7 correctness (185 open findings);
   - S1.1c-8 names;
   - S1.1c-9 size and the exclusion machinery's retirement;
   - S1.1c-10 the rest.

   S1.1c-5-3's forward and `LAST_PART` move to S1.1c-9.
2. **No spike** (not the recommendation); about 39 SP from S1.1c-6's rate.
3. **The design notes decide undecided behaviour and user text** (not the recommendation), with dry runs, checked by the blind reviewers.
4. **Deletions with proof;** merges, splits and moves later.
5. **The wide naming rule** (not the recommendation): 429 names in 105 files.
6. **One Python naming tool,** registered as a row from T0 in S1.1c-8.
7. **The weighted, seeded spot-check.**
8. **S1.1c-7 renames no existing test.**
9. **G10-22 and G10-15 are internal,** so the design notes decide them.
10. **G2-24 is fixed in full,** replacing a 2026-11-14 backlog item.
11. **The 2026-12-14 vulns expiry:** an upgraded golangci-lint first, otherwise the owner renews.
12. **The card is approved;** the branch is pushed as it grows.

**The card — the owner, 2026-11-20:** approved, so the sprint passes G0.

**The design (G1) — the owner, 2026-11-20** ([decisions](decisions.md#owner-decisions-2026-11-20--s11c-7s-design-g1); five answers):
1. **Up to six agents** for every S1.1 job until S1.1 closes.
2. **G10-20's leftover window** becomes known issue S1.1c-7-1, forwarded to S1.1c-10. *(annotated 2026-11-22: closed in the sprint by the Drain fix, below.)*
3. **S3.3-1-3 stays at S8,** with E1's recommendation recorded in its row.
4. **G10-1 is fixed with option B:** starting rows are applied in the activation step, with `table.Check` in its own commit before wave 2.
5. **The build:** W0 with four agents, then A1, B, E1 and A2, then E2 and CD.

The nine design notes are `docs/sprints/s1-1c-7-design-*.md`; the coordination record holds the cross-note rulings, the review dispositions and the pre-check results.

**Pull requests — the owner, 2026-11-21, during W0** ([decisions](decisions.md#owner-decisions-2026-11-21--s11s-pull-requests); three answers):
1. **One pull request per wave:**
   - PR 1: the plan, the design and W0;
   - PR 2: lanes A1, A2, B and E1;
   - PR 3: E2, CD, T4 and G5.
2. **The rule holds for all of S1.1,** each pull request at most 100 commits.
3. **Each pull request passes T2, T4 and a blind architecture and QA review** of its combined build, with the findings fixed before it reaches the owner.

## How it runs

At most two agents at once; the plan check §1 holds the order:
1. G0;
2. the design notes, their blind reviews and G1;
3. W0 (`mutate.py`'s contract, the bench defects, the proof tools' own tests, the naming tool);
4. W1 (A1, B) → spot-check → W2 (E1, A2) → spot-check → W3 (E2, CD) → spot-check;
5. combine.

## Measured results

*(Each proof is written here as it is made, with the commit it measured.)*

**G0** (the coordinator, 2026-11-20):
- **The docs commit** (e417f51): the doclinks row over every tracked doc, 212 files, exit 0.
- **The marker and the re-cut** (d521725):
  - **Red first:** before the move, `test_parts`' three re-pinned bound tests failed against the old value (`'S1.1c-7' != 'S1.1c-9'`; `forward S1.1c-10 is past S1.1c-7`; a forward at S1.1c-9 red as past S1.1c-7), and the decoupled `PrintedParts` and `TheOrder` tests passed on the old tree.
  - **After the move:** `test_parts`, `test_aging.PrintedParts`, `test_sprintid`, `test_sprints_pin`, `test_known_issues_pin` and `test_sprintid_parts` are green.
  - **G2-17 and G2-29 (K2e):** the old versions, in a scratch worktree at e417f51 with the re-cut data copied in, are red (`['S1.1c-5' … 'S1.1c-10'] != ['S1.1c-5', 'S1.1c-6', 'S1.1c-7']`; `32 != 29`); the new are green.
  - **The gate runner's whole suite outside the gate** (`python3.15 -B -X utf8 -m unittest discover -s gate/runner/tests -t .`): 1,301 tests. The 30 failures are all `test_summarize`'s 30, which fail the same way at e417f51 when run this way. The gate's own T2 run below is the check that counts.
- **The coordinator's fixes** (a2aeb02):
  - **G2-16 (K2e):** `python3.15 -m gate.runner.tests.test_aging` ran 43 tests against the module's 47; after the fix, 47 = 47.
  - **G2-25 (K2e):** an AST check over `test_instruments_pin.py` (every `subprocess.run` passes `timeout=`, every `go` call passes `env=`) named three faults at `:257` and `:261`; after the fix, none. `test_aging` and `test_instruments_pin` are green, 70 tests, the teeth test on the pinned go.
- **The evidence commit** (docs/sprints/s1-1c-7-plants/):
  - `survey-index.json`: the 1,442 findings' id, group and path; its ids equal the committed list's.
  - `lanes.py`: 186 ledger ids, 44 lane commits by package × kind.
  - `nameproto.py` and `naming-prototype.json`: on a2aeb02's tree, the decided scope finds **426 names in 105 files**. That is 429 at 5b4ddac less the three sprint-named tests the re-cut replaced (G3-26, G3-61, G3-62). It is the naming tool's acceptance list (plan check §3).
  - `check_ledger.py`: version 1.
- **G0's gate runs** (at 37df879, mains power, one at a time):
  - **T2:** exit 0, run 20261120T092347Z-50708e, 138.6 s wall. The `pytests` row passed, covering 130 modules, `test_summarize` included.
  - **T3, lint job:** exit 0, run 20261120T092606Z-bc6f8b. `aging` PASS on the real tree.
  - **T4, lint job:** exit 0, run 20261120T092617Z-a5e554. `aging` PASS: the marker S1.1c-7 does not reach S1.1c-9.
  - **T4, proofs job alone:** exit 0, run 20261120T092632Z-7ccf84, 232.3 s, covered 39 (the 2026-11-12 rule after the marker's move).

**G1** (the coordinator, 2026-11-20, after c3b6aab and 13887e0; on battery, which voids no pass or fail):
- **T2:** exit 0, run 20261120T115822Z-7a9497, 204.4 s wall; `pytests` covered 130.
- **T4, proofs job alone:** exit 0, run 20261120T122600Z-b78e9f, **491.7 s** (232.3 s on mains at G0), covered 39. The 2026-11-12 rule applies, since gate/known-issues.json is copied into every proof world. The margin to the 600 s cap is narrow on battery: watch it at T4.

**W0** (the coordinator, as each hand-back lands):
- **`table.Check`** (G10-1's Condition A; lane table-check, branch commit cc413c8, picked onto the integration branch):
  - **Red first:** a stub returning nil failed all six refusal subtests at `check_test.go:93`.
  - **Revert plant, re-run by the coordinator:** `mutate.py --file internal/table/table.go`, with Check's loop replaced by `return nil` and `--test TestCheckRefusesWhatUpsertRefusesAndChangesNothing`, reads `outcome: red`.
  - `go test -race ./internal/table` ok.
  - **The agent's extras:** a second plant, `return t.Upsert(rows)`, reads red at the "changes nothing" assertion; T2 exit 0 (run 20261121T041518Z-714e48).
  - **Beyond the design:** a sixth case, a non-finite float, which Upsert also refuses.
- **The naming tool** (W0-6, report-only; lane W0-py-b, branch commits 77789dc and 99cefb0, picked onto the integration branch):
  - **Red first:** before the matcher existed, the shim ran `gate.runner.tests.test_testnames` and reported an import error.
  - **Green:** 9 tests pass under the shim.
  - **Acceptance, re-run by the coordinator:** the tool's hits over the tree, against `naming-prototype.json`, give 426 against 426, missed 0, extra 0 (433 files, 7,044 names, 426 hits in 105 files).
  - **The agent's extra checks:** 12 scratch plants each read red in the test that owns their rule, and every existing gate check passes with the new files.
  - **Revert plant, run by the coordinator at cdbe019:** `mutate.py --file gate/runner/testnames.py`, with `match`'s return replaced by `return ()` and `--py gate.runner.tests.test_testnames.Golden.test_main_lists_only_tracked_test_files_outside_testdata`, reads `outcome: red`. The same plant, run directly, fails once at `test_testnames.py:179`: 0 hits where the test wants 2.
- **W0-go** (W0-7, tools/archcheck tests: G2-147, -149, -150, -151, -152 in part, -153; W0-8, tools/astequal tests: G2-154, -155; branch commits 9c4d5ad and fec48a2, picked onto the integration branch):
  - **Plants and controls:** the agent ran 14 entries by hand, each at d34e1fe and at its branch commit, every outcome as expected.
  - **Once W0-3 landed, the coordinator re-ran 13 of them** as the `--expect` batch `docs/sprints/s1-1c-7-plants/W0-go.json`, with old d34e1fe and new 3c12b2d (archcheck) or 64d74ec (astequal), the commits on the integration branch: `expect: 13 entries, 13 as expected; differing: none`. G2-151-live's data edit is an ordinary entry there.
  - **G2-153's proof stays the design's probe file** (`tools/archcheck/probe_test.go`, uncommitted, in a throwaway worktree): green at d34e1fe, red at its commit. A find/replace plant cannot add a file.
  - **Re-run by the coordinator:** G2-154-usage reads green against the old tests and red at fec48a2. The D-1 grep finds `exec.Command(` at three lines at d34e1fe and none at fec48a2.
  - **Load runs:** `TestCLIWriteFailureIsExit2` and `TestRunExitCodes` each 50/50 under load.
  - The agent's T2 run exited 0 (run 20261121T042221Z-b5303b).
  - **Carried to S1.1c-8:** G2-148's name, and G2-152's name "-cases on a missing file".
  - **New items found** (not survey findings), carried to S1.1c-10: astequal_test.go's `src(pairs...)` drops an odd last element, and tools/astequal/main.go's production `git()` has no ceiling.
- **`mutate.py --patch` and `--py`** (W0-1; lane W0-py-a, branch commit 8115ca6, picked as cdbe019):
  - **Red first, re-run by the coordinator:** the new `test_mutate.py` and `support.py` against the old tool (64d74ec) fail 14 of 35 tests, with 29 failure entries, since each failing subtest counts once. The agent reported 27.
  - **Feature-off plants:** 11 entries, one per counter-example #9 to #16, each on a test id that fails once. They are in `docs/sprints/s1-1c-7-plants/W0-1.json`. Re-run by the coordinator at 8115ca6 and again at cdbe019: `counts: red 11, green 0, timeout 0, invalid 0`, about 18 s, no worktree left.
  - **Green:** the five bench modules pass under the shim on the integration branch: 16, 20, 10, 10 and 35, which is 91 tests.
  - **GitNexus:** detect_changes on the staged pick rates it high. All 12 affected flows are mutate.py's own entry points, and no code outside gate/bench imports the tool.
- **`mutate.py --subtest`** (W0-2; lane W0-py-a, branch commit 4536730, picked as 93d3445):
  - **GO_SPACES:** built from 25 integer code points, which equal both Go's `unicode.IsSpace` and `isSpace` in `testing/match.go`, the function `rewrite` uses. The coordinator measured them on go1.27.4 and checked the tool's set against them.
  - **Item 37's fixture test:** subtest names holding U+00A0 and U+2003, written as escapes, are rewritten exactly as `go test` names them.
  - **Red first, re-run by the coordinator:** the new `test_mutate.py` against 8115ca6's tool gives `failed: 11 failures, 0 errors`, across the 9 new tests.
  - **Feature-off plants:** counter-examples #1 to #8 and item 37, 9 entries in `docs/sprints/s1-1c-7-plants/W0-2.json`. Re-run by the coordinator at 4536730: `counts: red 9, green 0, timeout 0, invalid 0`, no worktree left.
  - **Green:** `test_mutate` passes 44 tests under the shim.
  - **GitNexus:** detect_changes on the staged pick rates it critical. All 24 affected flows are mutate.py's own.
- **`mutate.py --expect`, with the `id` and per-entry `timeout` keys** (W0-3; lane W0-py-a, branch commit e4ed921, picked as c3e2ef2):
  - **Red first, re-run by the coordinator:** the 5 new tests against 4536730's tool give `failed: 10 failures, 0 errors`.
  - **Feature-off plants:** counter-examples #14b, #14c, #17 (two plants), #17a and #18, 6 entries in `docs/sprints/s1-1c-7-plants/W0-3.json`. Re-run by the coordinator at e4ed921: `counts: red 6, green 0, timeout 0, invalid 0`, no worktree left.
  - **Green:** `test_mutate` passes 49 tests under the shim.
  - **GitNexus:** detect_changes on the staged pick rates it critical. All 23 affected flows are mutate.py's own.
  - **Choices beyond the design,** accepted by the coordinator because each one refuses or reads invalid rather than giving a false verdict:
    - each revision gets its own plant, built from that revision's file;
    - a Go control entry must name its `pkg`;
    - #17's fifth entry is a Python one.
  - **A known limit:** `pkg` is checked against the working tree, not against each revision, so lanes run `--expect` from a checkout of their own commit.
- **The bench fixes** (W0-4, the K1s G10-2, G10-3, G10-4; W0-5, the K2s G2-6, G2-7; lane W0-py-a, branch commits 3b40205 and 42bd6a0, picked as 535ecb7 and 8d18dbb):
  - **The messages:** W0-4's message was split into a subject and a body. W0-5's gained its proofs, which its branch commit left pending.
  - **W0-4, red first, re-run by the coordinator:** the four new tests against e4ed921's tools each fail once (`failed: 1 failures, 0 errors` in test_escapes, test_mutate, test_load and test_compare).
  - **W0-4's revert plants:** `docs/sprints/s1-1c-7-plants/W0-4.json`, re-run by the coordinator at 8d18dbb: `counts: red 4, green 0, timeout 0, invalid 0`.
  - **W0-5's `--expect` batch:** `docs/sprints/s1-1c-7-plants/W0-5.json`. The coordinator pointed it at the integration commits (old 535ecb7, new 8d18dbb) and re-ran it: `expect: 4 entries, 4 as expected; differing: none`.
  - **The agent's G2-7 stress run:** `stress.py --runs 50 gate.bench.tests.test_load` at 42bd6a0, giving `stress: 11 tests, 0 below 50/50`.
  - **Green at 8d18dbb:** the five bench modules pass under the shim, 109 tests (17, 21, 10, 11, 50).
  - **GitNexus:** W0-4 is rated high. Its flows are compare.py's and escapes.py's own, and no gate row runs either tool. W0-5 changes no symbol.
- **The one-failure rule for `--py`** (raised by W0-1; it binds every lane): the plan check §2 makes red "the shim's only problem is `failed: 1 failures, 0 errors`". unittest counts each failing subtest as one failure, so an id whose plant breaks two subtests reads invalid. The rule stands as the plan states it. Lanes write each pinning test, or choose each plant, so that the id fails once. A lane that cannot do so reports it, and the coordinator takes it to the owner.

**PR 1: the W0 spot-check, the blind reviews and the fix round** (the owner's rule for every S1.1 pull request; [decisions](decisions.md#owner-decisions-2026-11-21--s11c-7s-pr-1-review)):

**The spot-check** (plan check §7.6), by a fresh qa-expert, blind, at d699d2e:
- **The draw:** every K1 (G10-1-A, G10-2, -3, -4); no K2-del; lane W0-py's K2 (G2-6, -7); lane W0-go's five lowest-ranked K2 (G2-155, -150, -154, -147, -152); the coordinator's G0 fixes (G2-16, -17, -25, -29); and W0-1, W0-2 and W0-3's plant batches in full.
- **The result:** 16 of 17 drawn ids pass all five checks. Every proof re-ran to its recorded outcome words.
- **G2-17 failed the design-match check** (should-fix): the plan check said the test reads fixtures, but d521725 kept it reading the live plan without saying so, and its witness needed a live issue old enough to age.
- **The owner's choice:** keep the live-plan check (the one check of S1.1c-5's design, decision 2, on the real files) and make the witness made-up issues.
- **The fix, f65a3a9:**
  - WITNESS-1 names no decision.
  - WITNESS-2 names a real decisions.md section that does not name it.
  - **Proofs, re-run by the coordinator at f65a3a9:**
    - with gate/known-issues.json's entries emptied, the old test fails and the new one passes;
    - a plant making `knownissues.names` return True reads `outcome: red`;
    - a plant switching rule 1 off reads `outcome: red`;
    - `test_aging` passes 47 tests.
- **Re-check:** that lane's whole K2e class (G2-16, -17, -25, -29) was in the draw. Only G2-17 failed.

**The blind reviews of `main..d699d2e`:** architecture (PR1A-1 to -7) and QA with a break-it (PR1Q-1 to -13).
- **Blockers:**
  - Go runs outside `--expect` read the working tree through `-overlay` while naming HEAD (PR1A-2 = PR1Q-7). A dirty test file could give a false red.
  - A plant in a file the build never compiles read green (PR1A-1 = PR1Q-11).
  - A patch without its final newline glued on the next source line, a false red (PR1Q-1).
- **The owner chose "one way of running"** for the first two.

**The fix round** (fresh python-engineer; branch s1-1c-7/pr1-fix; each commit picked onto the integration branch):

| Fix | Commit | Findings | Plants (`docs/sprints/s1-1c-7-plants/`) |
|---|---|---|---|
| F1: every Go run in a worktree at the commit; uncompiled `.go` plants refused | 72d39ef | PR1A-1, PR1A-2, PR1Q-7, PR1Q-11 | PR1-F1.json, 6 |
| F2: bad input refused before any build; testdata, `_` and `.` paths treated as data | b621171 | PR1Q-1, -2, -3, -5, -6, -13; PR1A-3 | PR1-F2.json, 17 |
| F3: the judges | 6496fc2 | PR1A-4 in part, PR1A-5, PR1Q-4, PR1A-6 | PR1-F3.json, 3 |
| F4: Go runs judged from `go test -json` actions | da15a51 | PR1A-4 (the depth-2 forge) | PR1-F4.json, 8 |
| F5: cancelling hunks and a directory refused; durations parsed exactly as Go's `time.ParseDuration` | 84fa71a | PR1A2-1 = PR1Q2-1, PR1Q2-3, PR1Q-13 (rest) | PR1-F5.json, 4 |

- **Red first:** each commit's new tests failed against the previous tool (the agent's runs).
- **Re-run by the coordinator** at the branch tips c56da34 and 83127d4, and again on the integration branch at 84fa71a:
  - the five batches give `red 6`, `red 17`, `red 3`, `red 8` and `red 4`, each with `green 0, timeout 0, invalid 0`;
  - the five bench modules pass, 140 tests (17, 21, 10, 11, 81).
- **The reviewers' replays at F5:** every blocker's replay passes. The only failure is PR1Q-8, the naming tool's, which goes to S1.1c-8.
- **Process breaches by the fix agent,** recorded: F2's impact queries ran after editing, and F3's detect_changes ran after committing. Neither recurred in F4 or F5.

**The narrow re-check** (the same two reviewers, at c56da34 and c6bae08):
- architecture: PR1A-1 to -6 closed;
- QA: PR1Q-1 to -7 and -11 closed, PR1Q-13 in part.

Its new later items:
- PR1A2-1 = PR1Q2-1, PR1Q2-3 and PR1Q-13's rest: fixed by F5;
- PR1A2-2, the witness never reaching the decisions reader: fixed by f65a3a9's WITNESS-2;
- PR1Q2-2, a literal `x#01` sibling reading invalid: recorded as a known limit (Go names both alike, and invalid is the safe answer).

**Forwarded and recorded:**
- To S1.1c-8 (binding input): the naming tool's PR1A-7 = PR1Q-8 (continuation lines in a grouped declaration read as names), PR1Q-9 (nested and starred Python bindings, a BOM) and PR1Q-10 (token-rule coverage, a question for the owner). Also G2-152's witness, which is gated on the case name `-cases`; the S1.1c-8 rename must move that gate, or key it on a field.
- Known limits, in mutate.py's docstring:
  - PR1Q2-2;
  - a deliberately hostile test writing go's 0x16 framing byte, or forging unittest's last `Ran` line after its summary (the residue class of ADR-0015 §3.6);
  - PR1Q-12: the `--expect` old side is judged by old_rev's own pyshim. No effect in this sprint.

**The risk register** (coordinator, 2026-11-21; the owner: "accuracy of the software matters more than speed of delivery, always"). Each HIGH or CRITICAL symbol, with its calls-only rating and hidden callers:

| Change | Calls only | Real exposure | Proof or action |
|---|---|---|---|
| mutate.py (W0, F1–F5) | CRITICAL (inside Bench) | every proof in the sprint | 2 blind reviews, a narrow re-check, 38 plants; every lane batch re-run on the fixed tool |
| `go_foreign` (A1, G3-72) | HIGH, 6 direct | the suppressions row, LAW Δ's exemptions; proofs driver texts | the coupling edit in the same commit; T2; T4 proofs |
| `_json_file` (A1, G10-8) | HIGH, 1 direct, 3 flows | the vulns row | T2; T4 proofs |
| `q2Start` (B, G2-108) | HIGH, 21 direct | 22 playback tests | load 50/50 over all 22 |
| `gitnexus_reindex.main` (A2, G10-5) | LOW | hidden caller: `.claude/settings.json`, every session | stress 50/50; a live smoke test of the settings.json command, twice at once |
| `runBlock` (B), `Runtime.Tick` (E1) | LOW, MEDIUM | error text on the Deploy and refusal paths | T2; a grep for old-text pins |

**PR 1's gate and load runs** (the coordinator, at 06e79d9, on mains, with no agent running):
- **T2:** exit 0, run 20261121T111105Z-e0af24, 197 s.
- **T4, lint job:** exit 0, run 20261121T111856Z-43284c.
  - The first run, 20261121T111423Z-cddc46, failed only on `vulns: the copy was fetched 8 days ago (2026-11-13), more than 7: run make vulndb`.
  - The copy was refreshed with `fetch-vulndb` (`modified 2026-11-11T02:51:11Z`), and the row then reads `vulns | PASS | covered 7`.
- **T4, proofs job alone:** exit 0, run 20261121T111439Z-4e22a2, 231 s.
- **W0's load re-runs:**
  - G2-7: `stress.py --runs 50 gate.bench.tests.test_load` gives `stress: 11 tests, 0 below 50/50`, 696 s.
  - G2-149: `load.py -- -race -count=50 -run '^TestCLIWriteFailureIsExit2$' ./tools/archcheck` gives `TestCLIWriteFailureIsExit2: 50/50 under load (hogs 28)`.
  - G2-154: `load.py -- -race -count=50 -run '^TestRunExitCodes$' ./tools/astequal` gives `TestRunExitCodes: 50/50 under load (hogs 28)`.

**PR 1's CI and F6:**
- PR #52's first CI run (44907887725) failed one test on Linux. `Compiled.test_a_plant_in_a_race_only_file_is_compiled_under_race` read `go list ./m at …: exit 2: go: -race requires cgo`.
- **Cause:** the gate and the workflows pin `CGO_ENABLED=0`, and only the race row turns cgo on, for its own go (`gate/runner/rows/race.py:32`). On darwin the race detector needs no cgo, which is why every local run passed.
- **The fix,** F6 (dc8ba2c): under `--race`, mutate.py's `go list` and `go test` run with `CGO_ENABLED=1`; without `--race`, the environment passes through.
- **Proofs:**
  - red first under `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`, at 9c17b21;
  - green at dc8ba2c;
  - `docs/sprints/s1-1c-7-plants/PR1-F6.json`: `counts: red 2, green 0, timeout 0, invalid 0`;
  - `test_mutate` passes 83 tests under `CGO_ENABLED=0`;
  - PR1-F1 to -F5 re-run at F6's tree, all red (the agent's runs).

**Main's red nightly** ([decisions](decisions.md#owner-decisions-2026-11-21--mains-red-nightly-and-pr-1s-ci)):
- **What failed:** main's nightly at 5b4ddac, run 44896088970, failed in `race-stress`: `panic: test timed out after 10m0s`.
- **Cause:** the row runs 20 repeats under one `go test` with no `-timeout`. It took 519.8 s on 2026-11-20 and 601 s on 2026-11-21, at the same commit.
- **Not a hang:** the timeout's stack dump showed the running tests 0–1 s old, and no goroutine blocked for a minute.
- **The fix, PR #53:** `fix/race-stress-timeout` (4a86074).
  - `-timeout 25m` under `--repeat`, held in `STRESS_TIMEOUT_MIN`.
  - A pin test reads the race-stress row and checks 25 min + 60 s + 120 s ≤ its `timeout_s`.
  - `--repeat=1` and no-repeat cases.
- **Proofs, re-run by the coordinator:**
  - green, 27 tests;
  - the budget pin red at `timeout_s` 600;
  - the `repeat > 1` mutant red;
  - the timeout removed, red (2 failures).
- **Reviews:** blind architecture and QA reviews found no blocker. The should-fix (FRT-A-1 = FRT-Q-1) and FRT-Q-2 are fixed in the commit.
- **Gate on the branch:** T2 exit 0 (20261121T135342Z-00c60c), T4 lint exit 0, T4 proofs exit 0 (230 s).
- **Forwarded to the backlog,** pending the owner's acceptance *(annotated 2026-11-21: the owner chose to fix both in S1.1c-7, de42f64; [decisions](decisions.md#owner-decisions-2026-11-21--after-pr-52-and-53-merged))*:
  - FRT-A-2: race.py reads `--repeat` twice; read it once in the parse loop after S1.1c-7 lands;
  - FRT-Q-3: `--repeat=0` and `-1` reach go with weak reasons. This predates the fix; S1.1c-7's A1-1 already refuses a non-number.

**Process changes:**
- heavy, load and timing runs are serialised on a quiet machine (E1 held after C5);
- every lane proof is re-run on the integration branch;
- a trial combine of F1 and 27 lane commits onto c6bae08 gave 28 of 28 clean;
- GitNexus is re-indexed after the lane picks, and the HIGH symbols are re-rated before PR 2's review, whose brief names them.

**Main's CI after PR #52 and #53** (the owner's report, 2026-11-22): the gate workflow is green at e7bceb3, PR #52's last commit, and at 35588ba, #53's. The next scheduled nightly, the first whose race-stress has `-timeout 25m`, is #53's real proof.

**W1 and W2: lanes A1, A2, B and E1** (the coordinator, 2026-11-21 to 2026-11-22):
- **The lanes' commits:**
  - A1, 7: 95c96f9 to 87acc34, G3-72's driver coupling made at the pick;
  - A2, 9: c669865 to 262ae0d;
  - B, 6: eaaec1c to 125a170;
  - E1, C1 to C5: e99e127 to aeec09c. E1 was then held, so that heavy runs stayed serial. C6 is bc23749, and C7, the lane's last, is e0f0e13.
- **Picked, then rebased:**
  - Each lane commit was picked onto the integration branch once PR 1's content was final. A trial combine onto c6bae08 had given 28 of 28 clean.
  - After PR #52 and #53 merged, the branch was rebased onto main (06f17b5) with no conflicts. Its tree differed from before only in #53's two files.
  - [`commit-map.json`](s1-1c-7-plants/commit-map.json) maps each lane commit to its pick.
- **Every lane proof, re-run by the coordinator** at 06f17b5 after the rebase, one at a time, alone, on mains power:
  - **Plain batches:**
    - `A1-2.json` red 2, `G3-72.json` red 2, `B-K1.json` red 3.
    - `A1-1.json` is refused at 06f17b5, because de42f64 rewrote the race.py lines its hunks match. At its own commit, 95c96f9, it reads red 10. mutate.py has not changed since that commit.
  - **The 17 `--expect` batches,** each `expect: <n> entries, <n> as expected; differing: none`:
    - A1: A1-3-G2-47 2, A1-4a 17, A1-4b 21, A1-5-G2-52 1;
    - A2: A2-C1 1, A2-C2 20, A2-C3 2, A2-C4 2, A2-C5 4, A2-C6 2, A2-C7 2, A2-C8 2, A2-C9 1;
    - B: B-K2a 13, B-K2b 10;
    - E1: E1-C2 10, E1-C3 14.
  - **G7a-5's revert plant:** `outcome: red`.
  - **G2-98's live-data plant,** which no one had run before: `B-G2-98.json`, 2 of 2 as expected. The new side fails at s3_2b_2_corpus_test.go:273 naming partial-count, checked by hand.
  - **No mutate worktree was left** afterwards.
- **Load and stress runs** (`gate/bench/load.py` under 28 hogs; `gate/runner/stress.py --runs 50`), at e23b6dc, the head before the rebase. Its tree differs from 06f17b5 only in race.py and test_rows_go.py.
  - A1's stress runs for G2-22, -42, -44, -47 and -48, A2's G10-5 and every B and E1 load run: N/N.
  - **G2-45's stress** covers test_rows_go, which de42f64 changed, so it was re-run at the head, bd413f3: `stress: 39 tests, 0 below 50/50`, 2,310 s.
  - **G10-20's K1c race plant** (`E1-G10-20-revert.diff` under `--race --repeat 50 --load`): caught 7 of 50. Its load run: 50/50.
  - **G2-92 (TestLiveEqualsReplay)** ran out of time twice, with no failure:
    - at run 39, on go's `-timeout 50m`, at about 79 s a run under the hogs;
    - at run 46, on load.py's own 3,600 s `--wall`.

    All 83 finished runs passed. The clean run at 06f17b5, with `--wall 6600` and `-timeout 105m`: 50/50 in 3,888 s.
- **The gate at 06f17b5** (mains power, no agent running):
  - **T2:** exit 0, run 20261121T220554Z-901a89, 214 s.
  - **T4, lint job:** exit 0, run 20261121T220928Z-a70856.
  - **T4, proofs job alone:** exit 0, run 20261121T220944Z-ced2d3, 239 s.
- **The re-index hook, live (G10-5):** two calls at once, with a git payload and HEAD one commit past the index. Both exit 0, and one run starts: one pidfile, one analyze.
- **E1 C6 (G10-21), bc23749:**
  - The coordinator's re-run at bc23749: the revert plant `E1-C6-G10-21-revert.json` red 1; `go test` and `go test -race` over `internal/runtime` ok.
  - **Load** (`TestABatchEndOnADerivedStreamOnceStopBeganIsNeitherTakenNorCounted`): 50/50.
  - **Timing,** parent 06f17b5 against bc23749: `compare.py` on BenchmarkSendPath and BenchmarkSendPayloadPath, 10 pairs, mains power before and after.
    - Run 1: exit 1, `verdict: costs (BenchmarkSendPayloadPath/recorded)`. No benchmark's time moved (ratios 0.989 to 1.008, every p ≥ 0.17). The flag was B/op, 1089 before against 1078 after: fewer bytes, with allocs/op equal.
    - Run 2, ground rule 13a's confirming re-run: exit 0, `verdict: no cost`, B/op 1089/1089.
    - The flag did not repeat, so C6 has no cost.
- **E1 C7 (G10-1's runtime half, option B), e0f0e13:**
  - The coordinator's re-runs at e0f0e13: the five revert plants, `E1-C7-G10-1-reverts.json`, red 5 (apply ×2, size, check, build); `go test` and `go test -race` over `internal/runtime` ok; `go test ./...` exit 0, 21 packages.
  - **Loads** (`TestActivateAppliesTheStartingRowsInItsStep`, `TestAddRefusesStartingRowsItCouldNotApply`, `TestStartingRowsBeyondRecordQueueRecordInFull`): 50/50 each.
  - **Timing, Send and SendPayload,** parent bc23749 with the benchmark file overlaid against e0f0e13: exit 0, `verdict: no cost` (ratios 0.992 to 1.007).
  - **BenchmarkTableWritePath could not be judged.** At compare.py's `-test.cpu=1` it fails on both binaries, 3 runs of 3 each (`recording queue full after record 67…`); at the default 14 CPUs it passes. The cause is the recorder's starvation, below. The table-write timing, on a benchmark with no recorder (the owner's choice), is in PR 2's fix round, below.
- **The recorder's starvation,** found by C7's benchmark and already on main ([decisions](decisions.md#owner-decisions-2026-11-22--the-recorders-starvation-and-c7s-timing)):
  - **What happens:** with one CPU, a caller that waits for each `TableWrite`'s reply and the router hand the processor to each other, and the app's recorder goroutine rarely runs. Its 65,536-record queue fills, and the recording ends with "recording queue full". Detection goes on.
  - **Evidence:** bc23749 with the benchmark file and e0f0e13 both fail the same way, the recorder having written about 1,700 of some 67,000 records.
  - **Send is not affected:** a scratch probe of the recorded Send and SendPayload worlds at one CPU keeps its recording (1,000,839 records for 1,000,000 events, not Failed). So the recorded hot-path benchmarks measure what they claim.
  - The fix is in PR 2's fix round, below.
- **No spot-check for W1 and W2** (the owner, [decisions](decisions.md#owner-decisions-2026-11-22--pr-2s-remaining-steps)): the coordinator's full re-run above stands in for the sample.
- **A mistake in the coordinator's timing script, corrected:** the pinned-Go setup file sets a variable the script also used, so four test binaries and two scratch files were written into the gate's tools cache. They were moved out before any gate run; the cache again holds only its tools.
- **G2-24, the coordinator's coupling commit, bd413f3** (built by a fresh python-engineer, test-first; in PR 2 by the owner's choice, [decisions](decisions.md#owner-decisions-2026-11-22--g2-24s-code-lists)):
  - `test_instruments_pin.py`'s CodeClosure judges all 31 instruments whose row runs a Python module. It names the two kinds it cannot judge, each with its reason: the Go-test instruments keyed under another row, and the rows with no Python module. Since 168d7de (PR2Q-13) it names each such instrument by id, so leaving the judged set needs a visible edit.
  - **Red first,** re-run by the coordinator: the new test on the old law.json, `failed: 27 failures, 0 errors`.
  - **law.json:** 65 module entries added over 27 instruments, and `gate/runner/pyshim.py` removed from pyfloor, pytests and proofs (the owner's strict rule). Nothing else in the file changed (checked field by field).
  - **Proof:** `G2-24.json` (gofmt's row module dropped from its list), old green → new red, control green/green: 2 of 2 as expected, re-run on the pick.
- **The coordinator's rulings:**
  - **E1 C3's G2-120 keeps no clock assertion.** After End, the on-arrival rule's clock is MaxInt64 by owner D1 of S3.2b-1, so only the message's admStream can show a missing admission.
  - **E1's G2-111 landed in C3,** not in C2 as the design planned.
  - **B's G2-107:** the stop rule fired at pre-check 1, so the guard is unchanged and the rest goes to S1.1c-10.
  - **E1 C7's three departures from the design's words,** none changing behaviour:
    - App.Loads' doc reads "A built app (NewSync) takes none": runtime.Start is New, then Add and Activate, so it applies the rows (the design note is annotated);
    - Activate's added sentence follows the sentence it was to split;
    - BenchmarkTableWritePath loops on `b.Loop`, which times the same span as the design's `ResetTimer`, `b.N` loop and `StopTimer`. Both timed binaries carry the same file.
  - **E1 C6:** written inline, the design's condition takes BatchEnd's cyclomatic complexity to 11, over the smells row's 10. The refused branch's queuing moved into `queueRefusedBatchEnd`, with the condition word for word and the accepted path byte-unchanged. Accepted: the same behaviour, and no waiver.
- **Hidden concurrency, found and handled:** the session's own GitNexus re-index hook starts a background re-index of about 23 s after any Bash call that runs git. Timing and load runs now start only when `pgrep -f 'gitnexus.*analyze'` is empty, and no git command runs during them.

**PR 2 (#54): opened, reviewed and fixed** (2026-11-22; decisions: [the remaining steps](decisions.md#owner-decisions-2026-11-22--pr-2s-remaining-steps), [the review](decisions.md#owner-decisions-2026-11-22--pr-2s-review), [the Drain fix's design](decisions.md#owner-decisions-2026-11-22--the-drain-fixs-design)):
- **Opened at 730fa8e** on the owner's word, with its open items stated in its body.
  - **The `gate` workflow:** green on all seven jobs.
  - **The `canary` workflow:** red on the pin test only, because ruff 0.17.6 was out. The `ruff` row itself passed on it.
- **PR #55 takes ruff 0.17.6** (`fix/ruff-0.17.6`: 7998f23, and 01ad58e for ADR-0015's annotation).
  - Both archives' sha256 equal the digests GitHub publishes, checked by the coordinator as well as the builder.
  - The gate at 7998f23: T2 exit 0 (20261122T060146Z-eea942), T4 lint exit 0 (20261122T060511Z-8185ad), T4 proofs exit 0 (20261122T060526Z-5d8b1f, 263 s). Its CI: green on all seven jobs.
- **The findings, checked by the coordinator before any fix** (the table is under [Findings](#findings)):
  - PR2A-1's replay: red at 730fa8e and at main's 35588ba.
  - PR2Q-1's step replay: red (`phase active: true; draining flag: false; Send: <nil>`).
  - PR2Q-2's and PR2Q-3's plants: every lane test stayed green, so both gaps were real.
- **The fix round.** Fresh agents, one worktree each; the coordinator verified every commit and then picked it onto the integration branch.
  - **Two tests and a comment:**
    - 32b2e97 (PR2Q-2): the plant `Unwrap` → `Inner` red on the new test.
    - 285cdbd (PR2Q-3): the plant that reverses `applyLoads`' loop red on the new test.
    - 8532c4e (PR2A-5): astequal, 17 declarations equal.
  - **The recorder's starvation** ([design](s1-1c-7-design-starvation.md)): e31828f (the tests), 0bd086d (the fix), 4f17618 (the benchmark split), 86aea25 (the flag dropped, after the load runs).
    - Red at the test commit: all four feeds (`TableWrite`, `BatchEnd`, `Tick`, late `Send`) read `Records:64 Failed:true`.
    - Green at the fix: 20 of 20 at one CPU; the package with and without `-race`.
    - `E1-starvation-plants.json`: red 14. Its plant 1 replaces the yield with `_ = goruntime.Gosched`, since deleting the line leaves an unused import.
    - `BenchmarkTableWritePathRecorded` at one CPU: PASS 3 of 3. Before the fix the recorded benchmark failed 3 of 3.
    - **Under load the flag failed** (the quiet window below): the one-CPU test passed 34 of 50, and 18 of 50 under `-race`. The `waiting` flag read a recorder paused in the middle of a record as busy, so the router ended the recording instead of yielding. The owner dropped the flag ([decision](decisions.md#owner-decisions-2026-11-22--the-starvation-fix-under-load)).
    - **86aea25:** at a full queue the router yields at most twice and offers the record again, whatever the recorder is doing. The guard test is renamed `TestARecorderBlockedInItsRecorderEndsTheRecordingAfterTwoTurns` and asserts two turns.
      - `E1-starvation-plants-2.json`: red 4. The plants: the yield made a no-op; the bound at 3; the bound at 1; the re-offer removed.
      - `E1-starvation-plants.json` was written against the flag. It is kept as the proof of 0bd086d.
  - **Three gate test gaps:**
    - 4dc13d9 (PR2Q-11): the plant `PR2Q-11-check-before-lock.diff` red.
    - ec2bb80 (PR2Q-12): the plant `return faults + shape` → `faults += shape` red.
    - 168d7de (PR2Q-13, PR2A-4): `PR2Q-13.json`, 3 of 3 as expected. Both of the reviewer's silent cases are green before the fix and red after.
  - **`Drain`'s holds** ([design](s1-1c-7-design-drain.md)): cd459b7, one commit. It closes five windows: known issue S1.1c-7-1, PR2A-1, PR2Q-1, and two the design's own tests found (two Drains of a never-activated app; two failed Drains leaving an app draining for good).
    - Red on the old `drain.go` and `app.go`, with the committed tests through an overlay, `-race -count=3`: five tests fail 3 of 3; the stuck-run race test fails 1 of 3.
    - Green at the commit: the ten new tests under `-race -count=3`; the package and the root; 0 DATA RACE lines.
    - `E1-drain-plants.json`: red 15, each entry on a deterministic test.
    - **G10-20's plant is re-cut** as `E1-G10-20-revert-2.diff`, since `revertUnlessQuieted` is gone. It is weaker than the first: the builder saw the race in 1 of 20 runs and 3 of 100, because the planted read is followed at once by the same Drain's lock. The coordinator's run under `--race --repeat 50 --load`: caught 3 of 50 (k ≥ 1).
  - **The combined head, cd459b7:** `go test ./...` exit 0, 21 packages; `go test -race ./internal/runtime/...` ok.
- **Timings and load runs.** No agent ran during them, and no git command. Each run started only when no GitNexus re-index was running. Load runs use 28 hog processes.
  - **The quiet window at cd459b7,** on mains power throughout:
    - **C7's table-write path, with no recorder:** bc23749 against e0f0e13, both built with the fix round's `BenchmarkTableWritePath`: no cost. This is G10-1's timing.
    - **The fix round's per-event paths:** 730fa8e against cd459b7, on `BenchmarkTableWritePath`, `BenchmarkSendPath` and `BenchmarkSendPayloadPath`: no cost.
    - **Under load, `-race`, 50 runs each:** the ten Drain tests, G10-20's test and the load-order test, 50 of 50 each.
    - **The hook's `CheckUnderLock`:** stress 50 of 50.
    - **The starvation test with the flag:** 34 of 50, and 18 of 50 under `-race`. The flag was dropped (above).
  - **At 86aea25:**
    - **The fix round's per-event paths,** 730fa8e against 86aea25, on mains: no cost. An earlier run of this comparison was on battery and is void.
    - **Under load:** the one-CPU test and the two-turns test under `-race`, 50 of 50 each. The runtime package's 199 tests, every test N of N, under `-race` (one run each) and plain (three runs each).
    - **The one-CPU test, plain, by yield bound:**
      - at 2: 49 of 50 in the first run, on battery; then 150 of 150 and 600 of 600, the last on mains;
      - at 8: 150 of 150 and 600 of 600.
      - The owner kept 2 ([decision](decisions.md#owner-decisions-2026-11-22--the-starvation-fix-under-load), item 2).
    - **Residual risk:** under extreme load on one CPU, a recording can still, very rarely, end at a full queue, as it could on main before the fix. That is one failure in about 850 loaded runs at bound 2, and none in the 750 since. Detection never waits for the recording.
- **The narrow re-check** (2026-11-25; [decision](decisions.md#owner-decisions-2026-11-25--pr-2s-re-check)):
  - **The reviews:** fresh blind reviewers on 730fa8e..86aea25, 15 minutes each ([architecture](../reviews/s1-1c-7-pr2-architecture.md#re-check-of-the-fix-round-2026-11-25), [QA](../reviews/s1-1c-7-pr2-qa.md#re-check-of-the-fix-round-2026-11-25)).
    - No blockers.
    - RCA-1, a gap the Drain fix opened, and six smaller findings. All are fixed on the owner's word; the table is under [Findings](#findings).
  - **The coordinator's checks before any fix:**
    - RCA-1: reproduced 3 of 3 at 86aea25, and absent at 730fa8e.
    - RCQ-1: its plant left the whole of `internal/runtime` and `internal/playback` green.
  - **The fixes,** by one fresh builder in its own worktree. The coordinator verified them and fast-forwarded the integration branch.
    - da2885c (RCA-1):
      - Activate's last step sets `a.opened` once the app's goroutines run, before its move to active, and `restorePhase` reads it.
      - A test-only hook, `openHold`, sits between the mark and the move.
      - A built app never reaches the restore, so `build` does not set the mark.
    - acee0a5 (RCQ-1): a test of a record queued on a turn.
    - d1201f9 (RCA-2, RCA-3): the comments.
    - 57d329d (RCQ-2, RCQ-3): the drain-holds spins stop after 5 s, and the blocked-recorder test releases its Recorder in a cleanup.
  - **Proofs, from the coordinator's runs:**
    - **Red first:** `TestAnEndedDrainDuringActivateKeepsTheInputClosedUntilTheGoroutinesRun`, with the old `drain.go` and `lifecycle.go` through an overlay, `-race -count=3`: fails 3 of 3 (phase 1, Send nil). Green at da2885c.
    - **`E1-recheck-plants.json`: red 7.**
      - P6 makes `turn` claim a record it did not queue, so its cleanup can wait out the 60 s ceiling.
      - P6b, one more slot per queue, shows the test failing at once.
    - **At 57d329d:** `go test ./...`, 21 packages ok; `go test -race -count=3 ./internal/runtime/...` ok, 0 DATA RACE lines.
    - **Load:** the 15 tests of the two changed test files, `-race`, 50 runs under load: every test 50 of 50.
- **The gate at PR 2's final head, 1c93c9b.** It ran on battery; no row judges timing.
  - T2: exit 0, run 20261125T142028Z-792f4f, 251 s.
  - T4 lint: exit 0, run 20261125T142440Z-9eb373.
  - T4 proofs: exit 0, run 20261125T142501Z-063632, 337 s.
- **#54's CI at 1c93c9b:** the `gate` workflow green on all seven jobs. The `canary` workflow was red on the pin test alone, from the new Python builds, which #56 took ([decision](decisions.md#owner-decisions-2026-11-25--the-python-builds-20261124)).
- **Merged** by the owner on 2026-11-25, by rebase-merge: 61908cb. The lane branches, their worktrees and the remote PR branches were deleted with the owner's OK.

**W3: lanes E2, CD1 and CD2, and the recording queue's release** (the coordinator, 2026-11-25 to 2026-11-26; [decisions](decisions.md#owner-decisions-2026-11-25--w3s-start)):
- **Base:** main at cc2c2c1, after #54 and #56 merged; the integration branch was made from it. Four agents at once, each in its own worktree: lanes E2, CD1 and CD2, and the queue's release.
- **The commits**, each lane commit mapped to its pick in [`commit-map.json`](s1-1c-7-plants/commit-map.json)'s `w3`:
  - **CD1, 7:** da0d1a3 to f788400: the parser's K1 and K2, the event package's bench input and K2, then the clock, text and model K2s.
  - **CD2, 9:** d0fad16 to 153cf5e, the eight lane commits and the G7a-3 coupling commit a30808c. The lane made the coupling commit on the coordinator's authority, because the design puts its three files in one commit.
  - **E2, 8:** 13bc086 to 99f3072.
  - **The recording queue's release, 1:** 1cfad34 (owner, 2026-11-25; PR 2's review, PR2A-3 and PR2Q-10).
- **Every stage was verified by the coordinator before its pick:**
  - Each plain and `--expect` batch was re-run. The CD2 `--expect` batches were regenerated at the final commits, because the lane amended its messages after its runs; the trees are identical.
  - The packages' tests ran plain and under `-race`.
  - Each escape-bearing line was byte-checked; each K2b coverage block and each K2e check was re-taken; the naming tool showed no new hit.
  - **Batch counts, as re-run:**
    - CD1: parser-K1 red 2; event-K2 19 of 19; clock-K2 6 of 6; model-K2 2 of 2.
    - CD2: C1 red 1; C2 red 2; C3 red 1; C4 9 of 9; C5 red 1 at its own commit; C7 2 of 2; C8 8 of 8; C9 2 of 2.
    - E2: E2-1 red 2 and 2 of 2; E2-2 13 of 13; E2-4 red 1; E2-5 13 of 13; E2-6 3 of 3; E2-7 6 of 6; E2-8 2 of 2.
    - The queue's release: red 4.
- **Load runs** (`load.py`, 28 hogs, `-race -count=50`), each test 50/50 with no DATA RACE line:
  - the queue's release, its three tests;
  - E2-2's four tests;
  - the event package's three tests (G2-68, G2-70, G2-73);
  - G2-133's snapshot test;
  - G2-143's test.

  G2-143's plant under `mutate.py --repeat 50 --load`: the old test caught 0 of 50, the new test 50 of 50.
- **T2 at each of the seven picks:** exit 0 every time (runs 20261125T154915Z-324199, 155801Z-c4a35b, 160253Z-a0b354, 160718Z-552291, 161248Z-e292e5, 161730Z-4134cd, 162235Z-d9ae7f).
  - The proofs row alone at T4, after the FUZZ_FILE and `notbuilt.json` edits: exit 0, run 20261125T155258Z-57be18.
- **Where the design and the code differed.** Each was reported by its lane and none changes a design's behaviour:
  - E2-1: `bind` returns the starting rows as a fourth result; the design left that plumbing open.
  - E2-3: two rendered sites beyond the design's list, apphandle.go:12 and io/cepio.go:40, inside G7a-2's widening (coordination item 24).
  - CD1: gen_test.go needed the `testing` import, so the design's line numbers there are one higher.
  - CD2:
    - C4: gofmt rewrote the design's one-line closure;
    - C7: `wg.Go` in place of `Add` and `Done`, with the same semantics;
    - C8: G2-140's grep needs `-P` or `[[:space:]]` on Apple's git.
  - E2-6 and E2-7:
    - G2-144's control takes the old_test/new_test green/green shape, the rule the second QA pass gave for a new test (DQ2-L2);
    - G2-143's coverage block is keyed by the branch body's first statement (plan check §7.2).
  - The queue's release: a never-activated app's queue is released in the router's remove step, so Remove releases the queue in every case.
- **Incident:** CD1's stage 2 ran about 20 minutes past its box. Two escape-transport slips were caught by the byte checks before any proof and repaired from a clean tree. Every escape afterwards was written from Python by bytes.
- **No spot-check for W3** (owner, 2026-11-25).
- **The final gate at PR 3's head:** named in PR 3's description; this record takes it at the close (G5).

## The finding ledger

186 ids: G10's 23, G7a's 6, G2's 156 and G3-72. The ledger is built through the sprint and checked by `docs/sprints/s1-1c-7-plants/check_ledger.py` (plan check §7.5).

- **Commit ids.** The G0 and W0 rows name each commit as it was before PR #52's rebase-merge. The W1 and W2 rows name the commits on the integration branch after its rebase onto main (06f17b5). [`commit-map.json`](s1-1c-7-plants/commit-map.json) maps both kinds, and each lane branch's commit, to the new ids.
- **Proof files.** Each batch or script a row names is in [`s1-1c-7-plants/`](s1-1c-7-plants/), its revisions on the current history.

| Id | Outcome | Commit | Proof | Reason or target |
|---|---|---|---|---|
| G10-6 | fixed elsewhere | 5b4ddac (PR #51) | the fix PR's red-first tests: `test_git` GitFailures, `test_lawdelta`'s two | the same defect as G1-1 |
| G2-16 | fixed | a2aeb02 | K2e: a direct run 43 → 47, equal to the module's 47 | |
| G2-17 | fixed | d521725 | K2e: old red on the re-cut data, new green | |
| G2-25 | fixed | a2aeb02 | K2e: the AST check `docs/sprints/s1-1c-7-plants/g2_25_ast.py` (the spot-checker's), 3 faults → none | |
| G2-29 | fixed | d521725 | K2e: old red on the re-cut data (32 != 29), new green | |
| G10-2 | fixed | 535ecb7 | K1: `W0-4.json` revert plant red; red first `1 != 0` | |
| G10-3 | fixed | 535ecb7 | K1: `W0-4.json` revert plants (mutate.py, load.py) red; red first `2 != 0` twice | |
| G10-4 | fixed | 535ecb7 | K1: `W0-4.json` revert plant red; red first names BenchmarkA twice | |
| G2-6 | fixed | 8d18dbb | K2a: `W0-5.json` G2-6 green → red, control green/green | the exact `math.comb` value rejected (design-W0) |
| G2-7 | fixed | 8d18dbb | K2a: `W0-5.json` G2-7 green → red, control green/green; stress 50/50 | |
| G2-147 | fixed | 3c12b2d | K2e: the D-1 grep, `exec.Command(` at archcheck_test.go:550 and :557 → none; the control G2-147-control green/green | |
| G2-148 | carried in full to S1.1c-8 | | | rename only |
| G2-149 | fixed | 3c12b2d | K2a: `W0-go.json` G2-149-clean and -finding green → red, control green/green; load 50/50 | |
| G2-150 | fixed | 3c12b2d | K2a: `W0-go.json` G2-150 green → red, control green/green | |
| G2-151 | fixed | 3c12b2d | K2a + K2e: `W0-go.json` G2-151-map green → red, G2-151-live red → green, control green/green | |
| G2-152 | fixed in part | 3c12b2d | K2e: the unread fixture dropped (its `cliRoot` line 2 → 1); the witness asserts the missing-file fault | the name "-cases on a missing file" → S1.1c-8 |
| G2-153 | fixed | 3c12b2d | K2e: the design's probe file, green → red | |
| G2-154 | fixed | 64d74ec | K2a: `W0-go.json` G2-154-usage, -git and -parse green → red, control green/green; load 50/50 | |
| G2-155 | fixed | 64d74ec | K2e: the D-1 grep, `exec.Command(` at astequal_test.go:221 → none | |
| G10-7 | fixed | 95c96f9, ec2bb80 | K1: `A1-1.json`, four revert plants red (the whole batch re-run at 95c96f9: red 10 of 10); red first: RuffWaiverTableOfTheWrongShape, 4 failures. PR 2's review (PR2Q-12): ec2bb80 pins that a wrong-shaped table is not compared both ways; the plant `return faults + shape` → `faults += shape` red | |
| G10-8 | fixed | 95c96f9 | K1: `A1-1.json`, three revert plants red; red first: UnreadableCopyFile, 3 failures (IsADirectoryError out of scan) | |
| G10-9 | fixed | 95c96f9 | K1: `A1-1.json`, revert plant red; red first: TrackedListFailure | |
| G10-10 | fixed | 95c96f9 | K1: `A1-1.json`, revert plant red; red first: the Race test (`'exit 1' !=` the usage reason) | de42f64 also refuses a count below 1 (PR #53's FRT-Q-3) |
| G10-11 | fixed | 95c96f9 | K1: `A1-1.json`, revert plant red; red first: the Row test (one failed: line, not two) | |
| G10-12 | fixed | 4c103df | K1: `A1-2.json`, revert plant red at test_step.py:391; red first | |
| G10-13 | fixed | 4c103df | K1: `A1-2.json`, revert plant red at test_teeth.py:285; red first | |
| G2-18 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, a direct run 45 of the loader's 47 → 47 of 47 | |
| G2-23 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, 42 of 46 → 46 of 46 | |
| G2-26 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, 26 of 27 → 27 of 27 | |
| G2-30 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, 12 of 17 → 17 of 17 | |
| G2-31 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, 51 of 64 → 64 of 64 | |
| G2-34 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, 6 of 8 → 8 of 8 | |
| G2-37 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, DocLine with support.run_tool refused: exit 1 → exit 0 | |
| G2-40 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, private Copy attributes at four lines → none | |
| G2-43 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, the access at :221 with no seam comment → none | |
| G2-47 | fixed | c1d9b33 | K2e (live data): `A1-3-G2-47.json` old red → new green, control green/green; stress 50/50 | |
| G2-48 | fixed | c1d9b33 | K2e: `a1_3_checks.py`, sigkills 3 → 0; stress 50/50 | |
| G2-20 | fixed | cb40d47 | K2a: `A1-4a.json` G2-20 green → red, control green/green | |
| G2-21 | fixed in part | cb40d47 | K2a: `A1-4a.json` G2-21a to -21d green → red, control green/green | two renames, `..._is_pinned` → `..._is_red`, to S1.1c-8 |
| G2-22 | fixed | cb40d47 | K2a: `A1-4a.json` G2-22 green → red, control green/green; stress 50/50 | |
| G2-38 | fixed | cb40d47 | K2a: `A1-4a.json` G2-38 green → red, control green/green | |
| G2-41 | fixed | cb40d47 | K2a: `A1-4a.json` G2-41 green → red, control green/green | |
| G2-42 | fixed | cb40d47 | K2d: `A1-4a.json` G2-42 timeout → red, control green/green; stress 50/50 | |
| G2-44 | fixed | cb40d47 | K2a: `A1-4a.json` G2-44 green → red, control green/green; stress 50/50 | |
| G2-27 | fixed in part | bf73b1a | K2a: `A1-4b.json` G2-27 green → red, control green/green | the rename to `test_dropping_isolation_or_no_bytecode_or_utf8_mode_is_refused`, to S1.1c-8 |
| G2-28 | fixed | bf73b1a | K2a: `A1-4b.json` G2-28 green → red, control green/green | |
| G2-32 | fixed | bf73b1a | K2a: `A1-4b.json` G2-32 green → red, control green/green | |
| G2-33 | fixed | bf73b1a | K2a: `A1-4b.json` G2-33-attribute, -setattr, -del and -flag green → red, three controls green/green | |
| G2-35 | fixed | bf73b1a | K2a: `A1-4b.json` G2-35 green → red, control green/green | |
| G2-36 | fixed in part | bf73b1a | K2a: `A1-4b.json` G2-36 green → red, control green/green | the rename to `test_an_experiment_suffix_is_red_naming_the_version`, to S1.1c-8 |
| G2-45 | fixed | bf73b1a | K2a: `A1-4b.json` G2-45 green → red, control green/green; stress 50/50 | |
| G2-53 | fixed | bf73b1a | K2a: `A1-4b.json` G2-53 green → red, control green/green | |
| G2-49 | fixed in part | c71a26b | K2-del: `a1_5_covered.py`, the four traced runs equal, the old test's 6 lines in both new runs | the rename to `test_the_enabled_set_is_the_configs_and_no_enabled_constant_exists`, to S1.1c-8 |
| G2-52 | fixed in part | c71a26b | K2-del: `A1-5-G2-52.json`, old red on the removed assertion's test, new red on `test_the_cap_constants` | the rename of `test_the_testdata_holds_exactly_the_real_entries_and_their_notice`, to S1.1c-8 |
| G3-72 | fixed | 87acc34 | K1: `G3-72.json`, two revert plants red; red first at test_suppressions.py:755 and :775; the proofs job exit 0 with the driver's coupling (test_suppressions_planted.py:42-43) | |
| G2-19 | rejected | | | the real-file test and the planted test compute the same expression, and the smells row already reds a real target of the wrong shape (design-a1a G2-19) |
| G2-50 | rejected | | | the test already fails on a raise: unittest records any exception from the block as an error, which pyshim fails (design-a1b G2-50) |
| G2-54 | rejected | | | every private seam fails loudly on a rename, so none passes vacuously; the real fix is a production rename outside S1.1c-7 (design-a1b G2-54) |
| G2-39 | carried in full to S1.1c-8 | | | rename only |
| G2-46 | carried in full to S1.1c-8 | | | rename only |
| G2-51 | carried in full to S1.1c-8 | | | rename only: one name covers both assertions, so no split |
| G10-14 | fixed | c669865 | K1: `A2-C1.json`, the revert plant red/red; red first `(1, []) != (2, [])` | |
| G2-55 | fixed in part | 37fe273 | K2a: `A2-C2.json` G2-55-start, -crash and -signal green → red, controls green/green | three renames to S1.1c-8; the duplicate of test_tree_digest to S1.1c-10 |
| G2-56 | fixed | 37fe273 | K2a: `A2-C2.json` G2-56 green → red, control green/green | |
| G2-59 | fixed | 37fe273 | K2a: `A2-C2.json` G2-59 green → red, control green/green | |
| G2-60 | fixed | 37fe273 | K2e (broken fixture): the manifest loader on a_tool, old refused (`field 'strip' must end with '/'`), new accepted | |
| G2-61 | fixed | 37fe273 | K2a: `A2-C2.json` G2-61-readonly, -cache and -hash green → red, controls green/green | |
| G2-63 | fixed | 37fe273 | K2a: `A2-C2.json` G2-63 green → red, control green/green | |
| G2-64 | fixed | 37fe273 | K2a: `A2-C2.json` G2-64 green → red, control green/green | |
| G2-9 | fixed | bb85a47 | K2a: `A2-C3.json` G2-9 green → red, control green/green | |
| G2-8 | fixed | 17623dd | K2-del: `A2-C4.json` G2-8 old red on the removed test, new red on test_f1_feeds' real-seam test, control green/green; `a2_covset.py`, gate/canary's covered set 510 lines in all four runs, the deleted test's own set inside it | |
| G2-10 | fixed | 17623dd | K2-del: `a2_covset.py`, the covered set unchanged (510 lines, four runs equal); the deleted test's own set empty | |
| G2-11 | fixed in part | 0830621 | K2a: `A2-C5.json` G2-11 green → red, control green/green | the rename to S1.1c-8 |
| G2-12 | fixed | 0830621 | K2a: `A2-C5.json` G2-12 green → red, control green/green | |
| G2-5 | fixed in part | 581a415 | K2a: `A2-C6.json` G2-5 green → red at assertNotIn, control green/green | moving the cases into test_regen.Main, to S1.1c-10 |
| G2-15 | fixed | fb7f089 | K2e (live data): `A2-C7.json` G2-15 old red → new green, control green/green | |
| G2-13 | fixed | 0ac6df8 | K2e: a direct run 15 of the loader's 20 → 20 of 20 | |
| G2-14 | fixed | 0ac6df8 | K2b: `A2-C8.json` G2-14 green → red, control green/green; `a2_linecount.py`, gitnexus_reindex.py:68 runs 0 → 1 times | |
| G10-5 | fixed in part | 262ae0d, 4dc13d9 | K1c: `A2-C9.json`, the revert plant red/red (the test's hold makes the race deterministic: 1/1); stress 50/50. PR 2's review (PR2Q-11): 4dc13d9 pins the pidfile check under the lock; `PR2Q-11-check-before-lock.diff` red; stress 50/50 | the "EPERM means alive" sub-claim rejected (the follow-up runs as the hook's own user); the pid-reuse sub-claim to S1.1c-10 |
| G2-57 | carried in full to S1.1c-8 | | | rename only |
| G2-58 | carried in full to S1.1c-8 | | | rename only |
| G2-62 | carried in full to S1.1c-10 | | | the split to S1.1c-10, its names to S1.1c-8 |
| G10-18 | fixed | eaaec1c | K1: `B-K1.json` G10-18 revert plant red; red first | |
| G10-19 | fixed | eaaec1c, 32b2e97 | K1: `B-K1.json` G10-19 and G10-19-chain revert plants red; red first. PR 2's review (PR2Q-2): 32b2e97 reaches the driver's error through `errors.Is`; the plant `Unwrap` → `Inner` red | |
| G2-89 | fixed | 576b0f0 | K2e: grep `case model.BodyWindow:` and `r.Body.Window.Source` in baseline_test.go, none → 2 lines; TestBaseline green, so no stored key moved | |
| G2-90 | fixed in part | 576b0f0 | K2a: `B-K2a.json` G2-90a and -90b green → red, control green/green | the move to internal/validate to S1.1c-10; the rename to S1.1c-8 |
| G2-91 | fixed in part | 576b0f0 | K2a: `B-K2a.json` G2-91 green → red, control green/green | the rename to S1.1c-8 |
| G2-92 | fixed | 576b0f0 | K2a (risk named): `B-K2a.json` G2-92 green → red, control green/green; load 50/50 at 06f17b5 | |
| G2-94 | fixed | 576b0f0 | K2a: `B-K2a.json` G2-94 green → red, control green/green | |
| G2-103 | fixed | 576b0f0 | K2a: `B-K2a.json` G2-103 green → red, control green/green | |
| G2-106 | fixed | 576b0f0, 0d42dda | K2e: `b_g2_73_check.py` over the 19 pairs, 19 with neither t.Parallel nor a reason → 0; load 50/50. The partition part (E2-5): `cd1_g2_73_check.py` over the four pairs 4 → 0 | |
| G2-107 | fixed in part | 576b0f0 | the stop rule fired at pre-check 1 (no seed drops an event construct-late), so the guard is unchanged; the header now says the count is compared but not exercised | a generator that drops an event construct-late, to S1.1c-10 |
| G2-108 | fixed | 576b0f0 | K2a: `B-K2a.json` G2-108 green → red, control green/green; load 50/50 | |
| G2-96 | fixed | 0e2b630 | K2a (subtest): `B-K2b.json` G2-96 green → red, control green/green | |
| G2-97 | fixed | 0e2b630 | K2c: `B-K2b.json`, the slowdown plant (sink.go, 25 ms per delivery) old red → new green, control green/green; load 50/50 | |
| G2-98 | fixed | 0e2b630 | K2e (live data): `B-G2-98.json`, partial-count given a numeric ceiling, old green → new red (s3_2b_2_corpus_test.go:273 names partial-count), control green/green | |
| G2-99 | fixed | 0e2b630 | K2a: `B-K2b.json` G2-99 green → red, control green/green; load 50/50 | |
| G2-100 | fixed | 0e2b630 | K2e: `defer func()` in the test, 0 → 1 | |
| G2-102 | fixed | 0e2b630 | K2e: `enEntries[...]` by position, 3 lines → none | |
| G2-104 | fixed | 0e2b630 | K2a: `B-K2b.json` G2-104a and -104b green → red, controls green/green | |
| G2-109 | fixed | 0e2b630 | K2e: unguarded `st.Streams[0][n]` and `st.Rules[0][n]` reads, 3 lines → none; the guards' texts 0 → 1 each | |
| G2-110 | fixed in part | 0e2b630 | K2e: `, _ := playback.NewDriver`, 1 line → none | the rename of `d2` to S1.1c-8 |
| G2-93 | fixed in part | ee30320 | K6: astequal, 59 declarations equal; no `// Output:` line | porting driver_test and exits_test to NewPlanDriver, to S1.1c-10 |
| G2-95 | fixed | 20010b5 | K2-del: `b_covered_set.py`, the deleted test's 6 blocks (init only) inside both remaining-suite runs | |
| G2-105 | fixed | 125a170 | K2e (bench input): under an overlay plant, the parent's four benchmarks exit 0 and the new four exit 1 | |
| G2-101 | carried in full to S1.1c-8 | | | rename only |
| G2-113 | fixed | e99e127 | K2-del: `e1_covset.py`, the deleted test's blocks (union 201) inside the remaining suite's two runs (intersection 1504) | |
| G2-129 | fixed | e99e127 | K2-del: the same subset check; the "no stream" row alone covers the guard's block, shard.go:183 | |
| G2-112 | fixed in part | 7400bb4 | K2a: `E1-C2.json` G2-112 green → red, control green/green; load 50/50 | the rename and the file rename to S1.1c-8; the move to S1.1c-10 |
| G2-116 | fixed | 7400bb4 | K2a: `E1-C2.json` G2-116 green → red, control green/green; load 50/50 | |
| G2-117 | fixed | 7400bb4 | K2c: `E1-C2.json`, the slowdown plant (400 ms before queueDrain) old red → new green, control green/green; load 50/50 | |
| G2-118 | fixed | 7400bb4 | K2a: `E1-C2.json` G2-118 green → red, control green/green; load 50/50 | |
| G2-128 | fixed | 7400bb4 | K2a: `E1-C2.json` G2-128 green → red, control green/green; load 50/50 | |
| G2-131 | fixed | 7400bb4 | K2e: `e1_astcheck.go.txt`, 3 hits → 0; load 50/50 over the three tests | |
| G2-111 | fixed in part | f618b66 | K2a: `E1-C3.json` G2-111 green → red, control green/green; load 50/50 | the renames to S1.1c-8; the merge and delete to S1.1c-10 |
| G2-114 | fixed | f618b66 | K2b: `E1-C3.json` G2-114 green → red, control green/green; boundary.go:221's block covered 0 → 1; load 50/50 | |
| G2-115 | fixed | f618b66 | K2a: `E1-C3.json` G2-115 (the router hashes the raw value) green → red, control green/green; load 50/50 | |
| G2-120 | fixed | f618b66 | K2a: `E1-C3.json` G2-120 (Sync.End delivers its walk to shard 0), old TestBroadcastCarriesNoAdmission green → new TestSyncEndBroadcastsAWalkWithNoAdmission red, control green/green | |
| G2-122 | fixed | f618b66 | K2e: `&rtLog{}`, 1 hit → 0; load 50/50 | |
| G2-125 | fixed | f618b66 | K2a: `E1-C3.json` G2-125 green → red, control green/green | |
| G2-127 | fixed | f618b66 | K2a: `E1-C3.json` G2-127 (time imported as tm) green → red, control green/green; the proofs job exit 0 (nosleep_test.go is a law path) | |
| G2-132 | fixed | f618b66 | K2d: `E1-C3.json` G2-132 old timeout → new red, control green/green | |
| G7a-5 | fixed | 4ee80f0 | K1: `E1-G7a-5-revert.diff` red (re-run at 06f17b5); red first: all 11 subtests | |
| G10-20 | fixed | aeec09c, cd459b7 | K1c: `E1-G10-20-revert.diff` under `--race --repeat 50 --load` caught 7 of 50 (k ≥ 1); load 50/50; red first: DATA RACE, drain.go:119 against :28. After the Drain fix (cd459b7) the plant is re-cut as `E1-G10-20-revert-2.diff`: caught 3 of 50 under the same flags; load 50/50 | the leftover logic window, known issue S1.1c-7-1, went to S1.1c-10 at G1; closed in PR 2 by the Drain fix (owner, 2026-11-22) |
| G2-119 | carried in full to S1.1c-8 | | | rename only |
| G2-121 | carried in full to S1.1c-8 | | | a rename and a doc comment |
| G2-123 | carried in full to S1.1c-10 | | | a split |
| G2-124 | carried in full to S1.1c-10 | | | moving three tests onto the gr* helpers changes how they measure, and no plant shows an outcome difference today |
| G2-126 | carried in full to S1.1c-10 | | | cleanups in ldRuntime and rdOverflowed touch about 20 callers and change no outcome on real code |
| G2-130 | carried in full to S1.1c-10 | | | a fidelity refactor: no production defect tells the fixture from the real tombstone |
| G2-24 | fixed | bd413f3, 168d7de | K2a: `G2-24.json` green → red, control green/green; red first: CodeClosure on the old law.json, 27 failures. PR 2's review (PR2Q-13, PR2A-4): 168d7de names each not-judged instrument by id; `PR2Q-13.json` 3 of 3 as expected | |
| G10-21 | fixed in part | bc23749 | K1h: `E1-C6-G10-21-revert.json` red; red first at lifecycle_test.go:755 and :770; load 50/50; timing no cost on the confirming re-run | the counting rule (a drained batch end counted, a drained Send not) to S3.3-1-3's fix at S8 (owner, G1) |
| G10-1 | fixed | c52933a, e0f0e13, 13bc086 | K1: `table.Check`'s revert plant red (W0). K1h: `E1-C7-G10-1-reverts.json` red 5; loads 50/50; Send and SendPayload timing no cost; the table-write timing with no recorder (4f17618's benchmark on both trees) no cost. The facade half (E2-1): `E2-1.json` red 2; `E2-1-expect.json` 2 of 2 as expected; red first lifecycle_test.go:448 | the facade builds the rows, hands them to Add and keeps no reference (PR 2's review, PR2A-2, PR2Q-5) |
| G10-16 | fixed | da0d1a3 | K1: `CD1-parser-K1.json` red 2; red first parseerror_contract_test.go:259, :262, :272 |  |
| G2-75 | fixed | ce348a4 | K2e: the overlay plant's panic count 1 → 0; the guard red at expect_pos_test.go:47 |  |
| G2-76 | fixed | ce348a4 | K2e: the live-data plant (`fragmentz` in fragments.json) 14 fault lines → exactly 1 |  |
| G2-78 | fixed | ce348a4 | K2e: the two seams' grep 2 → 0; FuzzParse kept, body only |  |
| G2-79 | fixed | ce348a4 | K2e: `var X = []pecRow{` 4 + 1 → 0; `panic(` 1 → 0; `pecEdit(t` 0 → 123; `git diff -w` shows no row line; the PERMANENT forbidigo waiver removed (exemptions −1) |  |
| G2-77 | rejected |  |  | a listed document with no pin compares 0 = 0 at fragment_test.go:1164, so `got.ruleLike == 0` at :1172 is its only guard (coordination item 32) |
| G2-80 | rejected |  |  | Parse's doc promises a *Error; the type assertion at vsc_known_issues_test.go:188 pins exactly that (coordination item 32) |
| G2-69 | fixed | 60a1a57 | K2e (bench input, R4): decode_bench_test.go:142 holds the pair's two escapes (byte check); coverage of `unescape.go:77.5,79.1` 0 → 1; TestDecodeLeafAllocations stays 0 |  |
| G2-68 | fixed | 5b783df | K2d: `CD1-event-K2.json` old timeout / new red; load 50/50 |  |
| G2-70 | fixed | 5b783df | K2b: `CD1-event-K2.json` ×6 green/red, six controls green/green; blocks `unescape.go:72`, `:77`, `decode.go:276`, `:114` 0 → 1 |  |
| G2-71 | fixed | 5b783df | K2a: `CD1-event-K2.json` green/red, control green/green |  |
| G2-72 | fixed | 5b783df | K2b: `CD1-event-K2.json` ×2 green/red, control green/green; `unescape.go:77.5,79.1` under TestDecodeDifferential 0 → 1 |  |
| G2-73 | fixed | 5b783df | K2e: `cd1_g2_73_check.py` over the 7 pairs 7 → 0; load 50/50 |  |
| G2-65 | fixed in part | f937d9b | K2a: `CD1-clock-K2.json` green/red, control green/green; every kind count and the ties ≥ 100 on the real code | the rename to `TestRandomWalkScenariosExerciseEveryCallback` to S1.1c-8 |
| G2-66 | fixed | f937d9b | K2a: `CD1-clock-K2.json` green/red, control green/green |  |
| G2-67 | fixed | f937d9b | K2a: `CD1-clock-K2.json` green/red, control green/green |  |
| G2-134 | fixed | 7bde86d, 8f3e6dd | K2e: `cd1_g2_73_check.py`: the plan half 1 → 0, the text half 7 → 0 |  |
| G2-74 | fixed | f788400 | K2a: `CD1-model-K2.json` green/red (subtest "every object's keys ascend bytewise"), control green/green |  |
| G10-15 | fixed | d0fad16 | K1: `CD2-C1.json` red 1 (the revert patch); red first nodes_test.go:236 and :239 in three subtests |  |
| G7a-3 | fixed | a30808c | K1, the coupling commit (expr, notbuilt.json:17, test_notbuilt.py:20): `CD2-C2.json` red 2 (TestBuildRunnable, TestCorpus); red first runnable_test.go:39 ×5 and corpus_test.go:131; the proofs row alone at T4 PASS |  |
| G10-17 | fixed | 47c1406 | K1: `CD2-C3.json` red 1; red first on the 72 pins (literalvalue_test.go:143 ×64, expect_test.go:175 ×5, coverage_s3_3_2_test.go:65 ×3) and literalvalue_test.go:17 by hand |  |
| G2-85 | fixed | 7bde86d | K2a: `CD2-C4.json` ×3 green/red, control green/green |  |
| G2-86 | fixed | 7bde86d | K2a: `CD2-C4.json` ×2 green/red, control green/green |  |
| G2-87 | fixed in part | 7bde86d | K2e: the `func itoa` grep 1 → 0 (strconv.FormatInt); the message truthful on the finding's plant (internal_test.go:63) | the file rename to S1.1c-8; the move of TestBuildRefusesAnUncheckedModel to S1.1c-10 |
| G2-88 | fixed | 7bde86d | K2a: `CD2-C4.json` green/red, control green/green |  |
| G10-22 | fixed | b9218d9 | K1 (choice A): `CD2-C5.json` red 1, run at its own commit; red first table_test.go:98 ×5; the three texts byte-identical |  |
| G7a-6 | fixed | a4beb93 | K6: astequal 43 declarations equal; no `// Output:` changed; the sweep red on the removed line, clean on the new |  |
| G2-133 | fixed in part | 6a22a47 | K2a: `CD2-C7.json` green/red (the new side red at 20000 iterations, no tuning), control green/green; load 50/50 | the rename to `TestSetSnapshotIsStableUnderConcurrentWriters` (naming the never-falls check too) and the move to set_test.go to S1.1c-8 |
| G2-135 | fixed in part | 173940b | K2a: `CD2-C8.json` green/red, control green/green | the rename ("a negative rate-limit override") to S1.1c-8; the move as a subtest to S1.1c-10 |
| G2-136 | fixed | 173940b | K2b: `CD2-C8.json` green/red, control green/green; `nulls.go:64.3,66.10` 0 → 1 |  |
| G2-138 | fixed | 173940b | K2a: `CD2-C8.json` green/red, control green/green |  |
| G2-139 | fixed | 173940b | K2a: `CD2-C8.json` green/red (the two-file patch), control green/green |  |
| G2-140 | fixed | 173940b | K2e: the `after, _ := app.MarshalCanonical()` grep 1 → 0 (with `-P`: Apple Git's `-E` has no `⧵s`) |  |
| G2-137 | fixed | 153cf5e | K2-del: `CD2-C9.json` red/red (the exprs.go plant red on the deleted row's subtest at the parent and on H3 at the commit), control green/green |  |
| G7a-1 | fixed | 13bc086 | K1: the revert plant in `E2-1.json` red; red first deploy_test.go:247 and coverage_s3_3_1_test.go:85 |  |
| G2-1 | fixed | 1bb9761 | K2a: `E2-2-expect.json` green/red, control green/green; load 50/50 |  |
| G2-2 | fixed | 1bb9761 | K2a: `E2-2-expect.json` ×4 subtests green/red, control green/green; load 50/50 |  |
| G2-3 | fixed | 1bb9761 | K2a: `E2-2-expect.json` green/red, control green/green |  |
| G2-4 | fixed | 1bb9761 | K2a: `E2-2-expect.json` ×2 green/red, two controls green/green; loads 50/50 ×2 |  |
| G7a-2 | fixed | 1635739 | K6 (widened, coordination item 24): astequal 10 files, 121 declarations equal; no `// Output:` changed; `e2_jargon_sweep.py` 42 hits → 0 over 22 files |  |
| G7a-4 | fixed | 2d01f52 | K1: `E2-4.json` red 1; red first objectkey_test.go:34 ×3 |  |
| G2-81 | fixed | 0d42dda | K2a: `E2-5-expect.json` green/red (the `Purged++` patch), control green/green |  |
| G2-82 | fixed in part | 0d42dda | K2a: `E2-5-expect.json` green/red, control green/green | the rename to `TestNewRuleRefusesABadConfigAndAcceptsEveryScalarKeyType` to S1.1c-8 |
| G2-83 | fixed | 0d42dda | K2a: `E2-5-expect.json` both green/red, prefix and negzero red/red by design, control green/green |  |
| G2-84 | fixed | 0d42dda | K2a: `E2-5-expect.json` message and swap ×3 green/red, control green/green on the four computed CheckTimers texts |  |
| G2-141 | fixed in part | 21bbc24 | K2a: `E2-6-expect.json` green/red, control green/green | the rename to `TestASecondOpenReturnsErrReopened` to S1.1c-8 |
| G2-143 | fixed | 21bbc24 | K2b: the plant under `--repeat 50 --load`: the old test caught 0 of 50, the new test 50 of 50; `source.go:58.3,58.19` 0 → 1 in two runs; load 50/50 |  |
| G2-144 | fixed | 28774de | K2a: `E2-7-expect.json` green/red (old_test TestEmbeddedFieldIsRefused, new_test TestEmbeddedInterfaceIsRefused), control green/green |  |
| G2-145 | fixed | 28774de | K2a: `E2-7-expect.json` green/red, control green/green |  |
| G2-146 | fixed | 28774de | K2a: `E2-7-expect.json` green/red, control green/green on the computed iota line |  |
| G2-156 | fixed | 99f3072 | K2a: `E2-8-expect.json` green/red (old_test TestShapeCorpusGolden, new_test TestJudgeReportsEachLoweringCodeAtTheLoweringStage), control green/green |  |
| G10-23 | carried in full to S1.1c-10 |  |  | `print`'s error path is unreachable without a seam, so no red test exists today (coordination item 25) |
| G2-142 | carried in full to S1.1c-8 |  |  | the rename to `TestEndBatchIsCountedByTheStream` is its only fix |

## Test material

Each lane's design note holds every finding's kind, its exact change and its proof's material (plan check §5, §7); W0's counter-examples are in the plan check §6.

## Scope changes

- **PR #53's FRT-A-2 and FRT-Q-3 fixed in the sprint** (de42f64), not put in the backlog ([owner, 2026-11-21](decisions.md#owner-decisions-2026-11-21--after-pr-52-and-53-merged)).
- **G2-24 by the strict rule,** in PR 2 ([owner, 2026-11-22](decisions.md#owner-decisions-2026-11-22--g2-24s-code-lists)).
- **The recorder's starvation fixed in the sprint** ([owner, 2026-11-22](decisions.md#owner-decisions-2026-11-22--the-recorders-starvation-and-c7s-timing)), by the approved [design](s1-1c-7-design-starvation.md).
- **`Drain`'s five race windows fixed in the sprint** ([owner, 2026-11-22](decisions.md#owner-decisions-2026-11-22--pr-2s-review)), by the approved [design](s1-1c-7-design-drain.md). Known issue S1.1c-7-1 closes; S1.1c-7-2 and S1.1c-7-3 open.
- **ruff 0.17.6** in its own pull request, #55 ([owner, 2026-11-22](decisions.md#owner-decisions-2026-11-22--ruff-0176)).
- **W1 and W2 without their spot-check** ([owner, 2026-11-22](decisions.md#owner-decisions-2026-11-22--pr-2s-remaining-steps)).
- **What follows S1.1c-7 is re-planned** ([owner, 2026-11-22](decisions.md#owner-decisions-2026-11-22--the-mvp-first-re-plan)); the re-cut is made at a plan check after PR 2.
- **The Python builds 20261124** in their own pull request, #56 ([owner, 2026-11-25](decisions.md#owner-decisions-2026-11-25--the-python-builds-20261124)).
- **The recording queue's release** in PR 3, PR 2's review findings PR2A-3 and PR2Q-10 ([owner, 2026-11-25](decisions.md#owner-decisions-2026-11-25--w3s-start)).
- **W3 without its spot-check** ([owner, 2026-11-25](decisions.md#owner-decisions-2026-11-25--w3s-start)).

## Findings

**PR 2 (#54), the blind reviews at 730fa8e** ([architecture](../reviews/s1-1c-7-pr2-architecture.md); [QA](../reviews/s1-1c-7-pr2-qa.md), two passes). No finding is a blocker introduced by the pull request. "Forwarded" means to the survey's debt work, S1.1c-10 as printed today.

| Id | Severity | Finding | Disposition |
|---|---|---|---|
| PR2A-1 | should-fix, on main | An Activate racing an expired Drain leaves the app activated in the router and refusing all input | fixed: the Drain fix, cd459b7 |
| PR2Q-1 | should-fix, on main | Known issue S1.1c-7-1 understated: with a stuck run the app keeps taking input after Drain returned its error | fixed: the Drain fix, cd459b7; the known issue is closed |
| PR2A-2, PR2Q-5 | later | A refused Activate leaves a partial load when rows change after Add, against `App.Loads`' contract | forwarded to lane E2's W3 design: the facade hands over rows it owns, or Add copies them. *(2026-11-26: met by E2-1's construction, 13bc086: Deploy builds the rows, hands them to Add and keeps no reference.)* |
| PR2A-3, PR2Q-10 | later | The recording queue holds 208 bytes per starting row for the app's life: about 198 MiB per million rows | forwarded to lane E2's W3 design and told to the owner. *(2026-11-26: fixed, 1cfad34: the queue is released at Remove, the owner's decision of 2026-11-25.)* |
| PR2A-4, PR2Q-13 | should-fix | G2-24's witness shared the rule that decides who is judged, so an instrument could leave the judged set unseen | fixed: 168d7de |
| PR2A-5 | later | `build`'s comment named Start as a caller | fixed: 8532c4e |
| PR2Q-2 | should-fix | The playback wrapper's `Unwrap` had no test | fixed: 32b2e97 |
| PR2Q-3 | should-fix | The starting rows' order was not pinned | fixed: 285cdbd |
| PR2Q-4 | later | Two router branches could apply loads wrongly with no test red | forwarded |
| PR2Q-6 | later | One starting-rows test proves "before it answers" only by timing | forwarded |
| PR2Q-7 | later | A test's name still claims the row G2-129 deleted | forwarded with the renames |
| PR2Q-8 | later | treehash's `--details` with no root, and an empty root | forwarded |
| PR2Q-9 | later | The hook's `running()` reads a pidfile of 0 or -1 as a live run | forwarded |
| PR2Q-11 | should-fix | No test pinned that the hook's pidfile check is made under the lock | fixed: 4dc13d9 |
| PR2Q-12 | should-fix | G10-7's "no both-ways comparison" was claimed and not tested | fixed: ec2bb80 |
| PR2Q-14 | later | G2-8's deletion drops one check no production seam can reach today | forwarded |
| PR2Q-15 | later | G10-9's fault can run to several lines on a multi-line git error | forwarded |
| PR2Q-16 | later | A `go env` that exits 0 with no output is recorded | forwarded |
| PR2Q-17 | later | `_methods` walks a lattice of diamonds in exponential time | forwarded |
| PR2Q-18 | later | A lock file that cannot be opened turns the re-index off in silence | forwarded |
| PR2Q-19 | later | Seven of nine `LOADER_HOOKS` names have no test | forwarded |
| PR2Q-20 | later | The closure rule follows named imports only | forwarded |

**PR 2's re-check, at 86aea25** ([architecture](../reviews/s1-1c-7-pr2-architecture.md#re-check-of-the-fix-round-2026-11-25), [QA](../reviews/s1-1c-7-pr2-qa.md#re-check-of-the-fix-round-2026-11-25)). No blockers.

| Id | Severity | Finding | Disposition |
|---|---|---|---|
| RCA-1 | should-fix, new in cd459b7 | A Drain that failed during Activate's last step opened the input before the app's flushers and recorder ran | fixed: da2885c |
| RCQ-1 | should-fix | No test pinned `turn`'s count of a record it queued, named in the queue-full text | fixed: acee0a5 |
| RCA-2 | later | The recorder's comments bounded the yields per call and called them brief | fixed: d1201f9 |
| RCA-3, RCQ-4 | later | `holdDraining`'s comment bounded its lock wait by another Drain's deadline | fixed: d1201f9 |
| RCQ-2 | later | Two drain-holds tests hung instead of failing when the move before the lock was lost | fixed: 57d329d |
| RCQ-3 | later | The blocked-recorder test waited out the 60 s ceiling on any failure | fixed: 57d329d |

## Done-done

```
Sprint S1.1c-7 — correctness and truthful tests · routine: full · SP ≈ 39 · cap 2 weeks
Entry (G0)   ☑ card complete and approved (owner, 2026-11-20)
             ☑ dependencies merged, main green (S1.1c-6, PR #49; close-out #50; the gate-hole fix, PR #51, main 5b4ddac)
             ☑ fits the cap, lanes ≤ 8 days, commits projected ≈ 80 ≤ 95 (plan check §1)
             ☑ plan check reviewed (architect, QA; three rounds) and answered by the owner (2026-11-20)
             ☑ test material: each lane's design note, before G1 ☑ `gate/current-sprint` set, the first commit after the plan check's (d521725, with the re-cut)
Design (G1)  ☑ architecture + the-fool findings resolved (two parts, no blocker; dispositions in the coordination record) ☑ QA testability findings resolved (two passes; DQ-B1 fixed by CD1's byte-exact re-emission)
             ☑ owner decisions recorded (2026-11-20, five) ☐ ADRs written for lasting decisions (reviewed at G5)
Build (G2)   ☐ every lane at T2 ☐ T3 combine-check exit 0 ☐ tests on the real code ☐ every breaker's red test committed
Review (G3)  ☐ architecture review of the combined diff ☐ QA break-it run ☐ every finding fixed, forwarded or rejected with reason
Verify (G4)  ☐ T4 strict-gate exit 0, run complete ☐ owner's check passed ☐ every acceptance line met by its named proof
Exit (G5)    ☐ one PR, ≤ 100 commits, merged by the owner ☐ main CI green on the merge commit
             ☐ sprint record + five-line retro ☐ known issues ☐ sprint-plan §Status line ☐ documents reconciled ☐ memory updated
             ☐ worktrees removed ☐ points and duration recorded ☐ the row closed in gate/sprints.json as in the plan table
```

## Retro

*(at G5)*
