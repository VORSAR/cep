# S1.1c-11 — plan check (v2, 2026-11-27; the nightly nets; reviews answered in §10; the owner's answers in §11)

*Step 0 of the sprint model ([§6](00-sprint-model.md#6-the-full-loop)). Drafted read-only against `main` @ 9b62eedf. Every code claim cites `file:line` on that commit; "inferred" marks a reading not proven by a run. The owner's six answers, all as recommended, are in [decisions.md](decisions.md#owner-decisions-2026-11-27--s11c-11s-plan-check). The scope is decided: [2026-11-22, CI coverage](decisions.md#owner-decisions-2026-11-22--ci-coverage) (packages A, B and D's macOS job), placed by [the MVP-first re-plan](decisions.md#owner-decisions-2026-11-22--the-mvp-first-re-plan) and [the re-cut](decisions.md#owner-decisions-2026-11-27--the-names-before-the-pattern-engine) (decisions 2 and 5; the order nets → names → S4.1 confirmed by the owner again on 2026-11-27, with "the low-hanging fruits" first). The re-cut's [binding input](s1-1c-recut-plan-check.md#3-binding-input-to-s11c-11s-s11c-12s-and-s11c-13s-plan-checks), item 2, is answered in Q2.*

```
Sprint S1.1c-11 — the nightly nets · routine: lighter, with one design note and blind reviews (Q1) · SP ≈ 10.5 (inferred) → 13 · cap 2 weeks
Entry (G0)   ☐ card complete and approved ☐ S1.1c-7 and the re-cut (#62) merged, main green (the owner reports main's CI)
             ☐ fits the cap, lanes ≤ 8 days ☐ test material exists, or making it is an item
             ☐ `gate/current-sprint` set to S1.1c-11 as the first commit; the record opened in or after it (sprint model §4 item 7)
             ☑ reviewed by the architect (§10) ☑ reviewed by QA (§10) ☑ answered by the owner (§11)
```

## What changed from v1

- **The nets judge has a control** (both reviews' blocker, RA-1, RQ-2): each test runs once without its plant first and must pass, selecting exactly its test; a net counts as caught only after that.
- **Nets carry their polarity and strength** (RQ-1, RA-4): five plants are tolerance proofs that must *pass* with the plant; plants caught only k of N times leave the live set for the race and load rows; `subtest` and per-side targets are kept.
- **Stale is caught at PR time** (RA-2, RQ-5): a fast anchor check at T2, no builds; `--changed` is dropped (RA-10); rename PRs run the whole nets row before merge.
- **`mutate.py` is not split** (RA-3, RQ-4): the row lives beside it in `gate/bench/`; the one change is typed refusals.
- **macOS by a nightly tier, not new rows** (RA-5): the test job's rows gain the nightly tier and the nightly test job runs on macOS.
- **PR CI does grow, a little** (RA-7, RQ-8): the anchor check and the new proof drivers; each is measured and budgeted.
- **The counts are corrected** (RQ-3, RA-15), budgets come from the CI runner with a burn-in (RQ-9), and the decision text v1 dropped is stated (RA-12, RQ-6).

## 0. What the code says that the plan does not (read first)

1. **Nothing reads the plant batches today, and the tool that runs them cannot run them as nets.** No gate code, workflow or Makefile names `docs/sprints/*-plants/` (`git grep -l -- -plants -- gate tools .github Makefile` is empty). `gate/bench/mutate.py --batch` checks every entry before the first build and refuses the whole batch when one entry's find text is not there exactly once (`gate/bench/mutate.py:522-524`, `:640-665`, exit 2 at `:1126-1127`); its outcomes are `red`, `green`, `timeout`, `invalid` (`:106`); "only red counts" and the designated test never runs without its plant (`:61-71`, `:1054-1066`). → §2 S1, Q2.
2. **The plant batches are 413 plants, not 2,281.** `docs/sprints/s1-1c-7-plants/` (the only plants directory) holds 75 JSON files; 1,442 of their 2,281 entries are the survey index and 426 the naming prototype. The re-cut plan check's §0 item 13 and its review RA-6 counted all of them; this sprint's docs commit annotates that number. Of the 413 (coordinator's count, re-derived by both reviews):
   - **296 carry a plant** (256 find texts, 40 diffs) and **117 are controls** (`control: true`, no plant: the test at a revision, unplanted);
   - at HEAD: 235 find texts occur exactly once, 20 no longer occur, 1 occurs twice; 33 diffs apply and 7 are refused (QA, by `mutate.changes_of`) — **28 plants no longer apply**;
   - **5 plants expect the test to pass with the plant** (tolerance proofs: G2-47 in `A1-3-G2-47.json`, G2-15 in `A2-C7.json`, G2-97 in `B-K2b.json`, g2-117 in `E1-C2.json`, G2-151-live in `W0-go.json`);
   - **some plants are caught only sometimes:** starve-3 is recorded as "may be red only some of the time", the G10-20 race plants were caught 3 of 50 under `--race --repeat 50 --load` (`docs/sprints/s1-1c-7-record.md:382`); starve-5 names a test that exists only in a design note (architecture review);
   - 44 entries carry `subtest`, 11 per-side targets (`old_test`/`new_test` and kin);
   - four standalone `.diff` plants sit beside the batches (`E1-G10-20-revert.diff`, `E1-G10-20-revert-2.diff`, `E1-G7a-5-revert.diff`, `PR2Q-11-check-before-lock.diff`).

   → §5 N2, Q2.
3. **A seeded sample of the live plants runs fast and is all caught.** 24 of the 222 find-text plants that apply, carry a plain `test` or `py` and expect red at their new revision (seed 20261127; 12 Go, 12 Python; the selection script and the 24 ids go into the record at G0), as one `mutate.py --batch --timeout 300s` at 9b62eedf on the owner's machine: `counts: red 24, green 0, timeout 0, invalid 0`, wall 42 s. Extrapolated to the whole live set, with a control run per distinct test (about 103 Go tests and the Python ids): roughly 10–15 minutes locally (inferred); the one `TestLiveEqualsReplay` plant (G2-92) carries a 300 s timeout. Every net can run every night. → §5 N4.
4. **The coming renames will touch most nets.** Of the 296, by name: 148 contain in their plant text a name the survey's G5 list renames, 26 a G6 name, 14 name a test on the history-name list; by file (architecture review, against the survey index): 268 plant into or test from a file with a G3–G6 or G8 finding (G5 248, G4 106, G8 82, G6 43, G3 30). Both are upper bounds (a short name such as `name` matches widely). Nets also sit on files this sprint and S4.1 edit (`gate/bench/mutate.py` 67, `internal/runtime/recorder.go` 23, `drain.go` 20). → §2 S2, Q2.
5. **Every new row brings law with it.** A row has 12 fields (`gate/runner/registry.py:22-37`); `teeth.pin` requires one instrument per row in `gate/law.json` with the row's tiers, `lands_in`, `min_wants` and `expected_scope`, its `code` equal to the row module's import closure, and a Python proof holding a red case on its own row (`gate/runner/teeth.py:487-559`; `gate/runner/tests/test_instruments_pin.py:218-295`); the workflows are generated from the registry and pinned by `ci-fresh` (`gate/runner/rows/ci_fresh.py:16-37`). Proof drivers run in PR CI's T4 `proofs` row (600 s cap, `exclusive`; 227–362 s locally), in parallel, one per core (`gate/runner/rows/__init__.py:130-146`); every new driver adds to PR CI. → §7, Q4.
6. **The nightly holds one long job.** `nightly.yml` has a `lint` job (suppressions, aging, vulns; 17 minutes) and a `nightly` job running `fuzz-parser` (900 s), `fuzz-decoder` (900 s) and `race-stress` (1,800 s) at once on one ubuntu runner (83 minutes; `.github/workflows/nightly.yml:15-66`); the generator makes one job per registry `job` and caps it at the rows' `timeout_s` summed plus 20 % and a minute, plus ten (`gate/ci/generate.py:219-230`). Its concurrency group is `nightly` for every ref with no cancelling (`nightly.yml:9-11`), so a dispatched run on a branch queues behind main's 03:00 run and can be replaced while pending (inferred from GitHub's group rules). The repository is public (`gh repo view`), so runner minutes are free; the binding limits are concurrent jobs and GitHub's six-hour job cap (inferred). → §2 S4.
7. **Only race-stress pins its go timeout against its budget.** `RaceStressBudget` (`gate/runner/tests/test_rows_go.py:212-226`) checks `25 m + 60 s + 120 s ≤ 1,800 s`, with one module constant `STRESS_TIMEOUT_MIN` (`gate/runner/rows/race.py:8-10`, `:25`); `gofuzz.py` passes no `-timeout` (`gate/runner/rows/gofuzz.py:28-43`); `load.py`'s default `--wall` is 3,600 s, equal to the registry's `timeout_s` maximum (`gate/bench/load.py:31`; `gate/runner/registry.py:19-20`). → §5 N4.
8. **The test job at the nightly tier selects nothing, and macOS runs one duplicated row.** `registry.select` refuses a tier and job that select zero rows (`gate/runner/registry.py:234-242`); the test job's rows (nine at T4: `unit`, `coverage-inclusion`, `pytests`, `differential`, `semantics`, `conformance`, `baseline`, `notbuilt`, `tripwire`) are tiered T2–T4; `RUNNERS = {"determinism-macos": "macos-latest"}` (`gate/ci/generate.py:44`); the summary fails any record whose (job, id) the registry does not hold (`gate/runner/summarize.py:176-213`). A tier may mix PR and nightly (`vulns` is `["T4", "nightly"]`). → §2 S3, Q3.
9. **The timing-sensitive tests and the threaded Python tests are named in the records, not in one list.** Go: `TestLiveEqualsReplay` (`internal/playback/equivalence_test.go:382`; 50/50 under load took 3,888 s locally, `s1-1c-7-record.md:304-308`), the 22 tests that reach `q2Start` (`internal/playback/helpers_s3_2b_2_test.go:92`), the recording-overflow tests (`internal/playback/s3_2b_2_fixround_test.go:36`, `internal/runtime/recorder_stop_internal_test.go:57`, with three neighbours). Python: `test_stop_children` (3 tests), `test_clock.RealTimers` (2), `test_main.Cli.test_workers_reach_the_scheduler`, `test_gitnexus_reindex.Overlap` (2). `stress.py` takes modules, not test ids (`gate/runner/stress.py:3`); `load.py` exits 0 when every test that ran is N/N, so a renamed test drops out silently (`gate/bench/load.py:136-147`). → §2 S5.
10. **A gate row that starts a process per input needs an owner's word.** testing.md rule 2 ([owner, 2026-11-12](decisions.md#owner-decisions-2026-11-12--s11c-2s-close-out)): such a row has a test that its process count does not grow with its inputs. The nets row must run one build per net. → Q5.
11. **The 2026-11-22 decision says "re-runs the committed plant batches".** Running a maintained copy instead changes its letter (ground rule 4: annotate). → Q2.

## 1. Routine, shape, waves

**The lighter routine, with one design note and blind reviews** (Q1). The sprint changes the gate and CI, not what the engine detects or emits ([sprint model §5](00-sprint-model.md#5-choosing-the-routine-at-the-plan-check)), but the nets runner is new gate logic: it gets a design note reviewed blind before code, and each PR's combined build gets a blind architecture and QA review ([2026-11-21, decision 3](decisions.md#owner-decisions-2026-11-21--s11s-pull-requests)). No semantics panel, no separate break-it.

**Waves, the easy nets first:**
- **W1 — configuration nets (PR 1):** lane **C** (race-stress per package, the `-cpu 1,4` row, one nightly job per long row) and lane **M** (the macOS nightly test job, the nightly concurrency group). Mostly registry, law and proof drivers.
- **W2 — the nets and the load rows (PR 2, with G5):** lane **A** (the nets: live set, judge, anchor check, inflow check) and lane **L** (the load row, the stress row, the timing list).

Two PRs under [S1.1's PR rule](decisions.md#owner-decisions-2026-11-21--s11s-pull-requests), each ≤ 100 commits. **Burn-in before each PR merges (Q6):** three consecutive green dispatched nightlies on its branch, each run's id and job times in the record. The design note is written and reviewed before W1.

## 2. Seams

**S1 — the live nets (lane A; read by S1.1c-12, -13 and later sprints).** A law file, `gate/nets.json`: a `nets` list and a `retired` list.

```json
{"id": "g2-111", "from": "docs/sprints/s1-1c-7-plants/E1-C3.json#0",
 "file": "internal/runtime/transitions.go",
 "find": "a.removing:\n\t\tit.reply <- reply{err: ErrRemoved}",
 "replace": "a.removing:\n\t\tit.reply <- reply{err: ErrStarted}",
 "test": "TestBreakitActivateRacesRemove", "pkg": "./internal/runtime", "timeout": "120s",
 "expect": "caught", "guards": "Activate on a removing app is refused with ErrRemoved, not ErrStarted (G2-111)"}
```

`patch` replaces `find`/`replace`; `py` replaces `test`/`pkg`; `subtest` narrows a Go test; `expect` is `caught` or `passes` (a tolerance plant). A retirement is `{"from": …, "reason": …}`. The row module, `gate/bench/nets.py`, imports `mutate.py`'s `entry`, `changes_of`, `Worktrees` and `run_one` (no split; `mutate.py` gains only typed refusals, `class Stale(Refused)`, so the judge never parses message text), in one worktree reset per entry (`mutate.py:782-785`). Per distinct test, then per net:

| Step | Outcome | Verdict |
|---|---|---|
| Control: the test runs without any plant | green, selecting exactly that test (and subtest) | its nets are judged |
| | red, timeout, or selects no test | each of its nets red: "unjudged: `<test>` fails without its plant" or "stale: selects no test" |
| Anchor: the find text occurs once / the hunks place / the file is tracked | no | red "stale", naming the count |
| Plant run, `expect: caught` | the test fails (Python: exactly one failure) | caught (pass) |
| | the test passes | red "escaped", naming the net and its test |
| Plant run, `expect: passes` | the test passes | pass |
| | the test fails | red "a tolerance net failed" |
| Any plant run | timeout / build or import error | red "timeout" / "invalid" |

The row writes each verdict as it comes, stops starting nets when its remaining budget cannot hold the next one's timeout (naming the rest red "not run"), and cleans its worktree on SIGTERM (`term.install()`, as `mutate.py`'s `main`).

**S2 — catching a stale net at the PR (lane A).** A T2 row, `nets-anchors`, over the same loader, no builds: each net's file is tracked; its find text occurs once or its hunks place (`mutate.changes_of`); its Go test is declared (`func <Test>(` in the package's `_test.go` files, parsed) or its Python id resolves (AST); the live and retired lists together cover every entry of every `docs/sprints/*-plants/*.json` (the inflow check); no id twice. Seconds per run. A PR that stales a net is red until it re-anchors the net in the same PR, or retires it with a reason. Rename PRs (S1.1c-12, -13) also run the whole `nets` row before merge (a `make nets` target), as the re-cut's binding input asks.

**S3 — the macOS test job (lane M; Q3).** The test job's rows gain the `nightly` tier, and `RUNNERS` is keyed by (tier, job), so the nightly `test` job runs on `macos-latest` while the T2–T4 `test` job stays on ubuntu. Records stay `(test, <row>)`: no summary change, no new instrument or driver; each row's instrument repeats its new tiers in `gate/law.json` (`teeth.py:484, 521-524`).

**S4 — one job per long nightly row, and a concurrency group per ref (lanes C and M).** `fuzz-parser`, `fuzz-decoder`, each race-stress row, the cpu row, `nets`, the load and stress rows and the macOS test job each get a nightly job of their own (a `job` value each); the generated caps follow. The nightly's concurrency group becomes `nightly-${{ github.ref }}`, so a dispatched branch run neither waits for nor replaces main's.

**S5 — the timing list (lane L; updated by S1.1c-13).** A law file names the Go tests the load row runs and the Python test ids the stress row runs, each with its count. `stress.py` gains test-id selection; the load and stress rows are red when a listed name selects no test, or when a test ran fewer times than its count.

## 3. Packages — where each new rule lives

| Rule | Where | Extends |
|---|---|---|
| A net is caught, escaped, passes, stale, unjudged | `gate/bench/nets.py` (pure judge + thin runner) | `gate/bench/mutate.py`'s `entry`, `changes_of`, `Worktrees`, `run_one` |
| A net's anchors and the inflow | `gate/bench/nets.py`'s static half, run by the `nets-anchors` row | `mutate.changes_of`; a Go `func Test` scan; Python AST |
| Race-stress per package, each go timeout in its budget | `gate/registry.json` rows; `gate/runner/rows/race.py` (a per-row `--timeout`) | `--repeat`, `STRESS_TIMEOUT_MIN`, `RaceStressBudget` |
| The `-cpu 1,4` pass | a registry row over `gate/runner/rows/gotest.py` or `unit.py` (design note), its failure line naming the `-cpu` value | `unit.judge` |
| Load and stress repeats, per listed test | rows over `gate/bench/load.py` and `gate/runner/stress.py` | their k/N verdicts |
| The nightly test job on macOS | `gate/ci/generate.py` | `RUNNERS` (`:44`) |

## 4. Ownership — one owner per file per wave

| Lane | Owns |
|---|---|
| **C** (W1) | `gate/runner/rows/race.py`; `test_rows_go.py`'s budget pins; the drivers that assert the nightly job name — `test_racestress_planted.py`, `test_fuzz_parser_planted.py`, `test_fuzz_decoder_planted.py`; the cpu row's driver |
| **M** (W1) | `gate/ci/generate.py`, `gate/ci/tests/` |
| **A** (W2) | `gate/nets.json`, `gate/bench/nets.py` and its tests, the typed refusals in `gate/bench/mutate.py`, the nets and nets-anchors drivers, `make nets` |
| **L** (W2) | `gate/bench/load.py`, `gate/runner/stress.py` (lane A must not change them), the timing list, the load and stress drivers |
| **Coordinator** | `gate/registry.json`, `gate/law.json` (instruments, tiers, import-closure `code` lists applied at each wave's end), the regenerated workflows (`make ci-workflow`), the Makefile, `gate/current-sprint`, docs. A registry `job` change lands in the same commit as the driver edits that follow it. |

## 5. The design note (before code)

One note, `s1-1c-11-design.md`, by a fresh `python-engineer` with GitNexus, reviewed blind by the architect (with the-fool) and QA before W1:
- **N1 — the nets row:** S1's states and exact lines; the typed refusals; the budget-aware loop; the per-net and per-control timeouts from the CI runner (N4); the process count (Q5).
- **N2 — seeding, one decision per plant:** every one of the 413 plants and 117 controls and the four `.diff` files becomes a net, a control the judge runs, or a retirement with a reason. Re-anchor or retire the 28 that no longer apply, each re-anchored net shown caught; mark the five tolerance plants `passes`; retire the plants caught only sometimes (starve-3, the G10-20 race plants) with "covered by the race or load rows", and starve-5 with "names no test"; retire the nets on files the gate edits every sprint (`gate/current-sprint`, `gate/registry.json`, `gate/law.json`); carry `subtest` and map per-side targets to the new side.
- **N3 — the anchor check and the inflow** (S2), with its time on the real set.
- **N4 — each new row's budget, measured on the CI runner:** per-package race-stress `--repeat` and go `-timeout` (at least three times the slowest CI run) from dispatched runs; the cpu row; the load row's counts (`TestLiveEqualsReplay` small); the stress row's runs; `load.py`'s `--wall` below `timeout_s`. Each pinned as `RaceStressBudget` pins race-stress.
- **N5 — PR CI:** each new driver's wall time; the load and stress drivers run with `--hogs 0` / `--no-hogs` and deterministic once-in-N plants (a counter file), never random; the proofs row's time before and after.

## 6. Counter-examples (each becomes a test, red first)

**The nets judge.** (1) Caught → pass. (2) Escaped → red, naming net and test. (3) A test red without its plant → its nets red "unjudged", never caught. (4) A test that selects no test, a skipped test (Go `t.Skip`, a Python skip) → stale or unjudged, never caught. (5) A subtest net is judged on that subtest alone (another subtest failing does not catch it). (6) A `passes` net that fails → red. (7) A find text zero or two times, a gone file, a hunk that does not place → stale. (8) A replace that no longer compiles, a Python plant that raises an import error → invalid, never caught. (9) A timeout → red. (10) One stale net among caught ones → the others still run. (11) The budget runs out → the rest red "not run", every earlier verdict kept. (12) Two nets with one id → refused at load.

**The anchor check and the inflow.** (13) A PR that edits a net's find text → `nets-anchors` red at T2, naming the net. (14) A renamed test a net names → red. (15) A plants entry neither a net nor retired → red. (16) A net deleted without a retirement → red. (17) A retirement without a reason → red.

**The rows.** (18) Each new row's go `-timeout` plus margins ≤ its `timeout_s` (a pin per row). (19) A planted race in each newly stressed package is caught by its row, and go ran it the row's count (as `test_racestress_planted.py` counts 20). (20) A test failing only at `GOMAXPROCS=1` is caught by the cpu row, the line naming `-cpu 1`, and passes the T2 `unit` row. (21) A deterministic once-in-N failure is caught by the load row; a Python one by the stress row; each listed test ran its count. (22) One stale name in the timing list among live ones → red, naming it. (23) The generator: the nightly `test` job runs on macOS with all its rows, the T4 `test` job on ubuntu; the summary reds a missing macOS record. (24) The concurrency group carries the ref.

## 7. Test plan per lane (the rig proves the line)

| Lane | Acceptance → rig |
|---|---|
| C | (18)–(20) → the race-stress and cpu drivers, `test_rows_go.py`; the fuzz drivers follow the job split |
| M | (23), (24) → `gate/ci/tests/test_generate.py`, the summary's tests; **once:** a dispatched nightly on a scratch branch with a `runtime.GOOS == "darwin"` failing test, red on macOS and green on ubuntu, its run id in the record, the branch deleted |
| A | (1)–(17) → `test_nets.py`, the nets and nets-anchors drivers; the whole live set judged at HEAD on the branch's dispatched nightlies |
| L | (21), (22) → the load and stress drivers |

*Verifiers at "done":* T2 exit 0; the proofs job alone exit 0, its wall time recorded against before; three consecutive green dispatched nightlies before each PR (Q6), not cancelled, each job's time recorded; after the final merge, main's first scheduled nightly (the owner reports it).

## 8. Risks and order

1. **R1 — renames and S4.1 make nets stale.** Most nets touch names or files S1.1c-12, -13 and S4.1 change (§0 item 4). *Mitigation:* S2's anchor check reds the PR that stales a net; it is re-anchored or retired in that PR; rename PRs run the whole `nets` row.
2. **R2 — a nightly row that is itself flaky stops the line.** The load and stress rows exist to find flakes. *Mitigation:* budgets from the CI runner (N4); three green nightlies before each merge; the sprint keeps time for the flakes they find.
3. **R3 — PR CI slows.** *Mitigation:* N5's budgets and measurements; no hogs in the proofs world.
4. **R4 — GitHub's concurrent-job limit.** About ten nightly jobs, one on macOS. *Mitigation:* each job's wall time in the record; jobs merge if the queue lengthens the nightly past its window (inferred limits).
5. **R5 — the timing list drifts from the tests.** *Mitigation:* S5's list is law; a name that selects nothing is red; S1.1c-13 updates it with each rename.

**Order:** the design note → W1 (C, M; PR 1, burn-in) → W2 (A, L; PR 2, burn-in) → G5.

## 9. Lane briefs (skeletons)

**Common to all:** your own worktree and branch; the pinned toolchain; write only your owned files (§4) and request registry and law entries in your report; commit early; never push or merge; tests on the real code; a planted case red before the fix; no AI attribution anywhere; never call anything an owner decision unless the brief quotes it; "done" = T2 `make check` exit 0 (read the `exit:` line) plus the proofs job alone; your full report in your final message; a hard time box.

- **C** (`python-engineer`): per N4, the race-stress rows, the cpu row, the job split; (18)–(20).
- **M** (`python-engineer`): S3 and S4's concurrency group; (23), (24); the one-time darwin demonstration.
- **A** (`python-engineer`): S1, S2, N1–N3; (1)–(17); the live set seeded per N2.
- **L** (`python-engineer`): S5; (21), (22).

## 10. Reviews and their dispositions

Two blind reviews of v1 against 9b62eedf ([architecture](../reviews/s1-1c-11-plan-check-architecture.md), with a the-fool pre-mortem; [QA](../reviews/s1-1c-11-plan-check-qa.md)). Both re-derived §0 item 2's counts; both found the same blocker.

| Id | Severity | Finding | Disposition |
|---|---|---|---|
| RA-1, RQ-2 | blocker | "Caught" had no control: a test red without its plant marks its nets caught; the 117 controls were set aside | Fixed: S1's control step, (3), (4) |
| RQ-1, RA-4 | blocker / should-fix | Five plants expect the test to pass; some are caught only k of N; `subtest` and per-side targets dropped; starve-5 names no test | Fixed: `expect`, `subtest`, N2's per-plant decisions, (5), (6) |
| RA-2, RQ-5 | should-fix | Stale shows only at the nightly; `--changed` under-selects | Fixed: S2's T2 anchor check, the inflow check, rename PRs run the whole row; `--changed` dropped (RA-10) |
| RA-3, RQ-4 | should-fix | Splitting `mutate.py` stales 67 nets and makes a runner↔bench cycle; nets on per-sprint files | Fixed: the row in `gate/bench/`, typed refusals only; N2 retires per-sprint-file nets |
| RA-5 | should-fix | Q3's option (a) did not fit the selection and the summary; a cheaper option missing | Fixed: S3 (the nightly tier and `RUNNERS` by tier and job), Q3 |
| RA-6, RQ-7 | should-fix | A darwin-only proof cannot be a proof driver | Fixed: (23) plus the one-time dispatched demonstration (§7) |
| RA-7, RQ-8 | should-fix | New drivers add to PR CI; hogs beside parallel drivers | Fixed: N5, Q4, exit 7 |
| RA-8 | should-fix | The job split breaks three drivers lane C did not own | Fixed: §4 |
| RA-9, RQ-14 | should-fix / later | Dispatched nightlies share main's concurrency group | Fixed: S4, (24); run ids recorded, conclusion not `cancelled` |
| RA-11 | should-fix | A watchdog kill loses every verdict | Fixed: S1's budget-aware loop, (11) |
| RA-12, RQ-6 | should-fix | Q2 changed the 2026-11-22 decision silently and misstated the binding input's options | Fixed: §0 item 11, Q2 restated |
| RA-13 | should-fix | `stress.py`/`load.py` unowned while A's import closure depends on them | Fixed: §4 |
| RQ-3, RA-15 | should-fix / later | 28 plants do not apply, not 21; four `.diff` files; the sample's ids; imprecise counts and the nightly's two jobs; the repository is public | Fixed: §0 items 2, 3, 6, 8; the sample's ids in the record |
| RQ-9 | should-fix | Budgets from the owner's machine; one shared stress constant; no burn-in | Fixed: N4, a per-row `--timeout`, Q6 |
| RQ-10 | should-fix | The timing list cannot see one stale name or a lost count | Fixed: S5, (21), (22) |
| RQ-11 | should-fix | `subtest` dropped | Fixed: S1, (5) |
| RQ-12 | should-fix | The process-per-input rule | To the owner: Q5 |
| RQ-13 | should-fix | No inflow rule; seeding unchecked | Fixed: S2's inflow check, (15); a sprint's plants join the nets at its close (sprint model §6 step 12, in the docs) |
| RA-14 | later | The rename measure lacked G3, G4 and G8 | Fixed: §0 item 4 |
| RQ-15 | later | `exclusive` as a cheaper split; the cpu line | The split is kept (separate runners, not only cores); (20) names `-cpu` |

## 11. Questions for the owner (G0)

*Answered 2026-11-27, all six as recommended ([decisions](decisions.md#owner-decisions-2026-11-27--s11c-11s-plan-check)).*

- **Q1 — The routine.** **(a, recommended) Lighter, with one design note and blind reviews** (§1). (b) The full loop, as S1.1c-1 to S1.1c-7 ran.
- **Q2 — The nets** (changes the letter of [2026-11-22, A](decisions.md#owner-decisions-2026-11-22--ci-coverage): "re-runs the committed plant batches"). **(a, recommended) A maintained live set, `gate/nets.json`,** judged per net with a control; a fast T2 anchor check so the PR that stales a net fixes or retires it; every committed plant either a net or retired with a reason, and each later sprint's plants join at its close; the decision annotated. (b) A per-net runner over the committed batches in place: no copy, but the coming renames leave most nets stale, reported and not fixed. (c) Each batch pinned to its recorded commit and mapped to HEAD through the ledger: right only as far as the ledger follows every rename.
- **Q3 — The macOS test job.** **(a, recommended) The nightly tier on the test job's rows, run on macOS:** no new rows or drivers, one definition. (b) Nine duplicated `*-macos` rows, as `determinism-macos` is: nine instruments and drivers to keep in step.
- **Q4 — PR CI.** **(a, recommended) Accept the small growth:** the T2 anchor check (seconds) and the new proof drivers (measured, no hogs), nothing moved out; PR CI stays about 10 minutes. (b) Also move `determinism-macos` to the nightly beside the macOS test job: a faster PR run, a darwin replay difference seen a night later.
- **Q5 — The nets row starts a build per net** (testing.md rule 2, [2026-11-12](decisions.md#owner-decisions-2026-11-12--s11c-2s-close-out)). **(a, recommended) An exception for this row, bounded:** one worktree, one control per distinct test, per-net timeouts from the CI runner, the budget-aware loop, nightly and the rename PRs only. (b) No exception: shard the nets into many rows whose process counts are each pinned.
- **Q6 — Burn-in.** **(a, recommended) Three consecutive green dispatched nightlies before each PR merges.** (b) One.

## 12. The card

| | |
|---|---|
| **Goal** | Every night, the committed nets re-check that the tests still catch their defects, the concurrent packages run repeated under race and at one CPU, the timing-sensitive tests run under load, and the test job runs on macOS; a PR that would leave a net stale is red at the PR; PR CI stays at about 10 minutes. |
| **Routine · size** | Lighter, with one design note and blind reviews (Q1) · ≈ 10.5 SP (inferred: C 2.5, M 1, A 5, L 2) → 13 |
| **Depends on** | S1.1c-7; the re-cut (#62) |
| **In scope** | 1. The nets: `gate/nets.json` seeded per N2, the `nets` row (nightly, `make nets`), the `nets-anchors` row (T2) with the inflow check (A). · 2. Race-stress for `internal/playback`, `internal/partition`, the root package and `io/inmemory` (B). · 3. A `-cpu 1,4` pass over `internal/...` (B). · 4. A nightly load row for the timing-sensitive Go tests (B). · 5. `stress.py` for the gate's threaded Python tests (B). · 6. The nightly split into parallel jobs and a per-ref concurrency group (B). · 7. A nightly macOS run of the test job (D). |
| **Not in scope** | Fuzz targets (C: with the sprints that add the inputs); the benchmark record (D: S10); coverage floors for `tools/` and `gate/` (C). |
| **Entry criteria** | The standard ones; the owner's answers to §11. |
| **Exit criteria** | The done-done block, plus: 1. A net whose test no longer catches its plant is red, naming it — **proof:** the nets driver. 2. A net whose test fails without its plant is never caught — **proof:** `test_nets.py` (3). 3. A PR that stales a net is red at T2 — **proof:** the nets-anchors driver. 4. Every committed plant is a net or retired with a reason — **proof:** the inflow check. 5. Every live net judged at HEAD passes — **proof:** the dispatched nightlies. 6. A planted race in each newly stressed package, a `-cpu 1`-only failure and a deterministic once-in-N load and Python failure are each caught by their row, each count reaching go — **proof:** the drivers. 7. The nightly test job runs on macOS — **proof:** the generator case plus the recorded darwin demonstration. 8. Each new row's go timeout fits its budget — **proof:** the pins. 9. PR CI about 10 minutes, the proofs job's time recorded before and after — **proof:** the PRs' gate runs. |
| **Owner's check** | *By running:* a dispatched nightly on the branch; read its jobs and times. *By reading:* the head of `gate/nets.json` and its retirements, one escaped and one unjudged planted case, and the LAW Δ block. |
| **Split line** | W1 (C, M) and W2 (A, L), one PR each. |
