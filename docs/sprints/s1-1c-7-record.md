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
2. **G10-20's leftover window** becomes known issue S1.1c-7-1, forwarded to S1.1c-10.
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

## The finding ledger

186 ids: G10's 23, G7a's 6, G2's 156 and G3-72. The ledger is built through the sprint and checked by `docs/sprints/s1-1c-7-plants/check_ledger.py` (plan check §7.5).

| Id | Outcome | Commit | Proof | Reason or target |
|---|---|---|---|---|
| G10-6 | fixed elsewhere | 5b4ddac (PR #51) | the fix PR's red-first tests: `test_git` GitFailures, `test_lawdelta`'s two | the same defect as G1-1 |
| G2-16 | fixed | a2aeb02 | K2e: a direct run 43 → 47, equal to the module's 47 | |
| G2-17 | fixed | d521725 | K2e: old red on the re-cut data, new green | |
| G2-25 | fixed | a2aeb02 | K2e: the AST check, 3 faults → none | |
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

## Test material

Each lane's design note holds every finding's kind, its exact change and its proof's material (plan check §5, §7); W0's counter-examples are in the plan check §6.

## Scope changes

*(none after G0)*

## Findings

*(review and break-it findings, with their dispositions)*

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
