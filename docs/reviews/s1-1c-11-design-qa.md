# S1.1c-11 design note — QA review

*The blind QA (testability) review of [the design note](../sprints/s1-1c-11-design.md) v1 (db8521c5), by a fresh `qa-expert` agent with a 10-minute box, blind to the architecture review. The note's v2 answers every finding. The report is as the agent gave it, condensed; paths are repo-relative.*

**Blocker**

- **DQ-1 — the control's timeout is wrong in the loop and in the rule.** The loop reserves the group's largest timeout but runs `replace(group[0], changes=())`, which carries `group[0].timeout` into `run_one` (`-timeout e.timeout`, wall `seconds(e.timeout) + WALL_EXTRA_S`); N4.3 sizes a net's timeout from its plant's wall, and a caught plant fails fast while the clean control runs the whole test (`TestLiveEqualsReplay`: about 78 s a run under load). Its nets read `unjudged` every night: a false red. **Fix:** the control at its own timeout (max(60 s, 3 × the slowest control wall), or the max of that and the nets'); a fake `run_one` test; a pin that no net's timeout is below its control's wall.

**Should-fix**

- **DQ-2.** Most judge states and loader refusals have no named test (control timeout/invalid, plant timeout/invalid under either polarity, caught and passes alone, the compile-check Stale, other `Refused`, `GATE_COVERED`; loader refusals 1, 2, 3, 6, 8). **Fix:** a table-driven `Judge` class and one `Load` test per refusal with exact messages.
- **DQ-3.** Counter-examples (4) skip, (5) sibling subtest, (8) compile/import error, (9) timeout have no nets-level test; (7) covers only "find occurs 0 times"; (11) covers the planned budget stop, not a SIGTERM mid-run; (15) and (16) share one world. **Fix:** judge tests, a sibling-subtest driver world (`escaped`), the missing stale cases, a SIGTERM test (verdicts kept, no worktree left), separate inflow and deletion cases.
- **DQ-4.** N3 check 6's premise is false: `survey-index.json` (1,442 entries) and `naming-prototype.json` (426) are lists; only `commit-map.json` is an object; the unstated "with a mutation" filter does the excluding, so a malformed entry silently leaves the inflow check, and a net's or retirement's `from` can name a non-plant. **Fix:** define a plant; name the three non-plant files; red on any other entry with neither a mutation nor `control: true`; checks 7 and 8 for retirements too; tests for check 5 (inherited methods, longest-prefix split), check 7 (index past the end, missing `.diff`) and check 8.
- **DQ-5.** A renamed subtest is not caught at T2 (check 4 scans only `func Test…(`), and D2's claim is too broad. **Fix:** a string-literal check for the subtest, or state the gap; a world renaming a subtest literal.
- **DQ-6.** The budget driver case is flaky unless its world is pinned (a window as wide as one net's wall). **Fix:** n1 `"1s"` and n2 `"600s"` on different targets, `--budget 400`; the Budget pin asserts `budget ≥ _need(control) + _need(net)` for the largest group.
- **DQ-7.** Re-anchored nets are "shown caught" by `mutate.py` alone, without a control — the plan check's blocker again for 20 nets, and the three `passes` nets are only inferred green. **Fix:** prove each with `nets.py --judge` over a one-net `nets.json`, recording the control line.
- **DQ-8.** Nets flaky by design are not screened (timing-sensitive targets such as `TestRecorderGetsATurnAtOneCPU` with four subtest nets, `TestLiveEqualsReplay`, the recorder-overflow tests; the row runs with `cpu=None`; macOS reuses Linux budgets). **Fix:** a net not caught/passes in all three burn-in nightlies is retired or re-measured, never retried; state whether any net needs `cpu`.
- **DQ-9.** Lanes C and L's counter-examples (19)–(22), the S5 timing list and the cpu row's module are not designed. **Fix:** design them in the note, or record that the lanes do.
- **DQ-10.** N5's "30 s per case" is unenforced and per case; the nets driver could take about 150 s. **Fix:** a budget per driver; whether worlds share `GOCACHE`; measure on the CI runner in PR 1.
- **DQ-11.** N6.3's real-registry case can only go green after the coordinator's tier edits; the demonstration never shows the darwin test green on Linux. **Fix:** order the edits or use a synthetic registry; also dispatch the gate workflow on the scratch branch.

**Later**

- **DQ-12.** Timeouts carried sum to 265 nets, not 264; rule 6 should read "the later batch is kept".
- **DQ-13.** Print output tails for every non-green control.
- **DQ-14.** The `need <s> s` sum is misleading (about 100,000 s beside `budget 3300`).
- **DQ-15.** `GATE_COVERED` should count caught + passes + escaped + tolerance failed only.
- **DQ-16.** The sketch treats `run_one`'s 3-tuple as a string.
- **DQ-17.** A tracked file deleted from the working tree is a read error, not a stale line; add a test.
- **DQ-18.** Say whether `gorun.run` kills a running `go test` before `Worktrees.close()` on SIGTERM.

**N2 spot-checks:** confirmed 72 batch files, 413 entries, 117 controls, 256 find texts and 40 patches, 4 `.diff` files, exactly 5 entries expected green; the retirements G2-24, PR2Q-13 #0 and #1, starve-2 (#5–#9), starve-3 (#10), starve-4 (#11–#12), starve-5 (#13), starve-1 #0 (duplicate of starve2-a); three targets. 264 + 16 + 20 = 300 = 296 + 4. Not verified: the split at HEAD, the 225 targets, the 26 subtest nets.
