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

## The finding ledger

186 ids: G10's 23, G7a's 6, G2's 156 and G3-72. The ledger is built through the sprint and checked by `docs/sprints/s1-1c-7-plants/check_ledger.py` (plan check §7.5).

| Id | Outcome | Commit | Proof | Reason or target |
|---|---|---|---|---|
| G10-6 | fixed elsewhere | 5b4ddac (PR #51) | the fix PR's red-first tests: `test_git` GitFailures, `test_lawdelta`'s two | the same defect as G1-1 |
| G2-16 | fixed | a2aeb02 | K2e: a direct run 43 → 47, equal to the module's 47 | |
| G2-17 | fixed | d521725 | K2e: old red on the re-cut data, new green | |
| G2-25 | fixed | a2aeb02 | K2e: the AST check, 3 faults → none | |
| G2-29 | fixed | d521725 | K2e: old red on the re-cut data (32 != 29), new green | |

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
