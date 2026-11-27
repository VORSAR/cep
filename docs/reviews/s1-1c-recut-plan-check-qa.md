# The S1.1c re-cut plan check — QA review

*The blind QA review of [the plan check](../sprints/s1-1c-recut-plan-check.md) v1 and the new 2026-11-27 section of [decisions.md](../sprints/decisions.md#owner-decisions-2026-11-27--the-names-before-the-pattern-engine), by a fresh `qa-expert` agent, blind to the architecture review. Each finding's disposition is in the plan check's §5. The report is as the agent gave it; paths are made repo-relative and the prototype's scratch paths are left out.*

I reviewed against `main` @ 82c80be9 and changed nothing in either checkout. To prototype, I cloned the repo into a scratch directory and applied §2's data change, the bound and the forwards there, with four scripts: `recut.py` (the data change), `g1.py` (the G1 rule over counter-examples), `agingsim.py` (the aging row's two checks against the real merge base), `lawsim.py` (the LAW Δ block).

**How I ran the tests.** `python3.15 -I -B -m unittest …` fails at once with `ModuleNotFoundError: No module named 'gate'`, because `-I` drops the working directory from the import path; I used `python3.15 -B -X utf8 -m unittest …` from the repo root, and the pinned interpreter for `test_summarize`. **Baseline on main:** the 22 modules that read the real plan files gave 520 tests, OK. **Not done:** the proofs job, the GitNexus freshness resource and the Hindsight search (no tool for either in this session).

## Findings, most severe first

None reaches blocker.

**RQ-1 — should-fix. The move check sees one of the three moved forwards; the exit proof claims all three, and two of the moves are checked by nothing.** `parts.forwards` and `_moves` judge only TEMP size entries of `smells.json` (`gate/runner/parts.py:105`, `:148-183`); all 13 name `S1.1c-5-3`. Counter-proof on the re-cut clone: moving `S1.1c-7-2` and `S1.1c-7-3` to `S1.1c-14` while keeping their old decisions is green everywhere (`agingsim.py` T4 `faults 0`; `test_known_issues_pin` and `test_aging` OK); reverting `S1.1c-5-3`'s decision alone gives 13 faults of the form `aging: smells.json: . funlen: forward moved S1.1c-9 → S1.1c-14 (issue S1.1c-5-3 → S1.1c-5-3) without a decision that names S1.1c-5-3`. **Fix:** reword §0.5 and §2's proof line; the S1.1c-7-2/-3 moves are checked by a reviewer against the LAW Δ lines; a forward-move check over every known issue to the backlog.

**RQ-2 — should-fix. G1 turns a forgotten marker bump green, while the docs still say T2 catches it; R3's mitigation is circular.** `docs/sprints/00-sprint-model.md:69` and `gate/runner/tests/test_sprints_pin.py:3-5` say a forgotten bump is red at T2; `g1.py` confirms that under G1 a marker left on the closed row just before the first open row passes — exactly S4.1's G0 forgetting the marker (marker `S1.1c-13`, first open row `S4.1`), so S4.1's not-built expiry would not arm for a whole sprint (`gate/runner/rows/notbuilt.py` reads the marker). **Fix:** update the three texts (and `sprintid.py:1-6`); make the allowance testable — the record is created at G0 (`00-sprint-model.md:255`), so "when `docs/sprints/<first-open-id>-record.md` is tracked, the marker must be at the first open row"; red-first tests with and without the next record.

**RQ-3 — should-fix. §2's change to `test_parts.py` turns 19 more tests red, and the data change turns 4 Rules tests red that §2 does not name.** With `LAST_PART = "S1.1c-14"` and no test edits, 7 tests in `test_parts.Rules` fail: the three §2 names plus `test_reached_at_t4_is_red`, `test_reached_at_nightly_is_red`, `test_reached_at_t3_is_a_line_not_red` and `test_not_reached_is_silent`, because `ORDER` (`test_parts.py:18-31`) does not print `S1.1c-14`. Re-printing `ORDER` as §2 says gives `FAILED (failures=21)` across `Moves`, `OnlyDecisionsMd`, `Caps` and `Base`, because `FixtureBound` patches `LAST_PART` to `"S1.1c-8"` (`:73-79`). **Fix:** name `FixtureBound`; keep the old fixture rows (fixtures need not follow the live plan) or re-point `FixtureBound`; list all 7 Rules tests.

**RQ-4 — should-fix. G1 is red on main's own data, and no test pins that change in behaviour.** With G1's `sprintid.py` and main's data, `test_sprints_pin.test_the_marker_is_the_first_open_row` fails: the printed part-row `S1.1c-7` sits two or more rows past the open umbrella `S1.1c`. §2's "the umbrella case keeps working" holds only for unprinted parts; G1 cannot land without closing `S1.1c` in the same commit (which matters for Q6 and Q9). **Fix:** `test_a_marker_on_a_printed_part_row_behind_an_open_umbrella_is_named`, green to red on main's code, and the behaviour change stated in §2 and Q6(a).

**RQ-5 — should-fix. Every row closed crashes the T2 pin at S12's close-out.** `g1.py` with (S11 ✓, S12 ✓) and marker S12 gives `ValueError: every row of gate/sprints.json is closed` from `first_open()` (`sprintid.py:103-107`), outside G1's `try`. **Fix:** a red-first test and handling of the case.

**RQ-6 — should-fix. The exit item "the LAW Δ block lists exactly §2's law files" is wrong as written.** `lawsim.py` after §2's ADR-0015 annotation and the `testnames.py` docstring edit gives an ADR-0015 line with "an ADR changed — owner decision required", lines for `gate/known-issues.json`, `gate/sprints.json` and `tools/allowlists/smells.json`, and `gate/runner/: N files` grouped, so `sprintid.py`, `parts.py`, `testnames.py` and their tests are never listed one by one. **Fix:** state the expected block, with N and the file list checked against `git diff --stat main -- gate/runner/`.

**RQ-7 — should-fix. The exit proofs show green but never show red without the decision.** At T3, with no merge base, the move check prints a plain line and passes (`parts.py:136-143`); `test_aging_planted` sets `main` to HEAD (`gate/proofs/drivers/test_aging_planted.py:73-74`), so the proofs job never sees a move. **Fix:** a T4 aging run with `S1.1c-5-3`'s decision reverted (13 faults), then PASS restored, both in the commit message; the T4 PASS line showing the resolved base.

**RQ-8 — should-fix (§3). A rename that breaks a plant batch can merge unseen.** `git grep -l -- -plants -- gate tools .github Makefile` is empty; package A is nightly. **Fix:** a rename PR re-runs, before merge, every plant batch whose `test` or `find` names a renamed symbol, the result in the record.

**RQ-9 — should-fix (§3). A rename silently drops a fuzz seed corpus.** `internal/event/testdata/fuzz/FuzzDecode/3de82d51f677fb17` is keyed by the fuzz function's name; renaming `FuzzDecode` orphans it and `go test` still passes. **Fix:** list `testdata/fuzz/<Name>/` among the references, with a check that every such directory names an existing `Fuzz<Name>`.

**RQ-10 — should-fix (§3). The coverage, move and level rules are not operational as written.** Coverage before moves gives no command (bind it to the `unit` row's `go test -coverprofile`, no `-coverpkg`, on a scratch tree, with the post-move T2 coverage row as the final proof). A move's proof misses a copy left in the old home (bind it to the test-list comparer: the same multiset of (test, subtest) through the ledger). Only the scenario side of the level rule is checkable; add an AST check that no `gate/**/tests/test_*.py` imports another `test_*` module. The reference list omits `gate/registry.json`'s `-run` patterns (loud: `gate/runner/rows/gotest.py:25-34`) and `CORPUS_TEST`, a G3 rename target read by `test_tripwire_planted.py:16` and `test_determinism_macos_planted.py:15`.

**RQ-11 — later. G1 has a condition that is always true, and it admits stale parts of the closed row.** `order.rows[at].closed` with `at == want - 1` is always true; with (S1.1c ✓, S4.1 open), marker `S1.1c-8` or `S1.1c-99` → `""`. **Fix:** drop the clause or comment it; a counter-example test for the stale part.

**RQ-12 — later. Nothing checks that the forwards in `docs/product/known-issues.md` match the JSON** (`gate/runner/knownissues.py:148-162` compares ids, Found, opened and correctness only). A reviewer check suffices.

**RQ-13 — later. The struck rows' pointer cells must not contain ✓:** `plan_rows` raises on a tick outside the id cell even in a struck row (`sprintid.py:187-188`).

## Claims verified true (by a run unless marked)

- §0.1: with the old rule and the re-cut data, the pin is red; exactly 9 tests red: `test_first_open`, the pin, and 7 in `test_parts.Rules`.
- §0.4: `LAST_PART = "S1.1c-9"`; all 13 TEMP size entries name `S1.1c-5-3` with expiry 2027-02-11.
- §0.5: a struck base forward `S1.1c-9` is placed at row `S1.1c`, so the move to `S1.1c-14` counts as later.
- §0.7: `testnames: 439 files, 7292 names; 425 hits in 105 files (report only)`.
- Q9(a): with G1 and the re-cut data, marker `S1.1c-7` is green.
- G1's four new tests: each red on main's code (two by behaviour, two by message text only).
- The new decisions section: its anchor resolves and it names the three issue ids; `test_known_issues_pin` and the real-tree `test_aging` tests pass on the re-cut data.
- The aging row on the real tree at T3 and T4 against base 82c80be9: `faults 0` (first open row `S1.1c-11`).
- `ThePlanTable` and the `compare_rows` pin pass with the struck rows skipped and removed.
- The not-built proof's fixtures (inferred): they depend only on S1.1c sitting before S4.1.
