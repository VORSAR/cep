# S1.1c-11 PR 1 (W1) — QA review

*The blind QA review of W1's combined build (`main..s1-1c-11/integration` at dde50079), by a fresh `qa-expert` agent with a 10-minute box, blind to the architecture review. Dispositions: [the owner's decisions](../sprints/decisions.md#owner-decisions-2026-11-28--s11c-11s-pr-1-review) and the record. Condensed as the agent gave it; paths repo-relative.*

**Verdict:** no blockers; three should-fix, five later. Ran `gate.runner.tests.test_rows_go.RaceStressBudget` and `gate.ci.tests.test_generate` (38 tests OK); the Go-backed classes and the drivers taken from the record.

- **WQ-1 — should-fix.** The burn-in table leaves out `race-stress-inmemory` and `race-stress-partition`, so "every other new row is inside the rule" has no numbers behind it for them. **Fix:** add both rows for runs A, B, C.
- **WQ-2 — should-fix.** The budget pins catch a timeout too long, never one too short: playback's 28m passed every pin and was caught only by reading the burn-in; at 49m it sits at 3.01 × its slowest run (976.4 s), an 11 s margin, with runs spreading about 50 % (639.5–976.4 s). **Fix:** a floor pin from a table of the slowest CI seconds per row (owner to decide; it goes beyond counter-example 18's letter).
- **WQ-3 — should-fix.** The macOS `pytests` timeout is 2.25 × its slowest run (runs 236.7, 400.4, 319.0 s): flaky by design. **Fix:** 1200 s, a nightly budget, or the risk recorded (owner).
- **WQ-4 — later.** The cpu driver shows `-cpu 4` stayed green only by `assertNotIn("-cpu 4:", out)`, which also passes if the second pass never ran (the unit tests cover it). **Fix:** read `test.cpu4.json` and assert a pass event for `TestPlantedOnlyAtOneCPU`.
- **WQ-5 — later.** The unit-row half of counter-example 20 depends on the host having more than one CPU (`GOMAXPROCS` is removed from the environment, `gate/runner/env.py:11-12`). **Fix:** an explicit `os.cpu_count() > 1` precondition.
- **WQ-6 — later.** The race-stress pin does not enforce one package (it checks the last non-dash argument). **Fix:** exactly one non-flag argument after `race.py`.
- **WQ-7 — later.** The drivers prove `--repeat` reached go (20 run events), not `--timeout` (the argv unit tests and the pin cover it together). **Fix, if wanted:** an argv check through the existing `LOGGED_GO` wrapper.
- **WQ-8 — later.** The design note's names for the cpu cases differ from the built tests. **Fix:** a name map in the design or record.

**Counter-examples:** (18) met for the maximum (WQ-2); (19) met — four drivers plant in the row's own package, check the job, count 20 runs; (20) met for the `-cpu 1` attribution (WQ-4, WQ-5 for the halves resting on unit tests); (23) met (real-registry generator case; the summary case shown red on a plant); (24) met. **Exit lines:** 6 met for W1's part; 7 met (generator case plus darwin run 46112477166); 8 met for the maximum only; 9 — the proofs job's time recorded (248.3 s before, 228–256 s after), PR CI's total unmeasured until the PR's gate run.
