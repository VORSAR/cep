# S1.1c-7 design note — A1b (gate/runner tests)

*S1.1c-7's design note for chunk A1b (gate/runner tests), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes.*


## The note

## S1.1c-7 design note: chunk A1b (gate/runner/tests, 20 findings)

### Header

- **Chunk:** A1b, lane A1. 20 findings: G2-27, -28, -30, -31, -32, -33, -34, -35, -36, -45, -46, -47, -48, -49, -50, -51, -52, -53, -54 and G3-72.
- **Base:** `s1-1c-7/integration` @ d5217259fa399824230a1eaa37df073d1e64ac67. `git rev-parse HEAD` matches the commit the brief gives for the index.
- **Files read in full or in the cited ranges:**
  - Tests: test_run_guard.py, test_sprintid_parts.py, test_verdict.py, test_suppressions.py (1-60, 600-799), test_teeth.py (1-60, 145-278), test_vulndb.py (1-175, 360-560, 700-735), test_vulns_row.py (1-40, 139-200), test_rows_go.py, test_rows_ruff.py (1-160, 194-300), test_stop_children.py, test_term.py, test_stress.py (120-140), support.py (selected).
  - Production: gate/run.py, gate/term.py, gate/runner/pyshim.py, suppressions.py (1-120, 235-265), teeth.py (40-60, 300-432), vulndb.py (18-60, 133-300), rows/vulns.py (215-275), rows/race.py, rows/vet.py, golangci.py (100-125), rows/__init__.py (head), stress.py (head).
  - Config and data: ruff.toml, gate/law.json and gate/registry.json (pyfloor entries), the plan check, testing.md.
- **GitNexus:** 12 calls, all returned `epistemic: exact`.
  - `context`: run.refusal, suppressions.go_foreign, teeth._defined_names, rows/vulns.build_info, vulndb.findings, term.critical, rows/race.main, PLANTED_MUT, golangci.LINTERS, stress._one_run.
  - `impact` upstream: StopTest.kill_child, vulndb._parse.
- **Could not check:**
  - I could not read the `gitnexus://repo/cep/context` resource; no resource-reading tool exists in this session. Instead I confirmed HEAD = d521725 against the brief.
  - I ran no tests, as the brief forbids. To make expected texts byte-exact I called four pure functions with the pinned interpreter under `-I -B`, which writes no files: `teeth.python_cases`, `suppressions.go_foreign`, `rows.vulns.build_info` and `vulndb.findings`. Every exact fault text below was printed by the current code.
  - Hindsight memory: not used.

### Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G2-27 | accepted | K2a | C1 | rename → S1.1c-8 | add subcase `-I -B -X utf8=0`; argv `--no-such-flag` for every subcase |
| G2-28 | accepted | K2a | C1 | – | assert `code == 0` before the `__pycache__` walk |
| G2-30 | accepted | K2e | C1 | – | main guard moved to the end of test_sprintid_parts.py |
| G2-31 | accepted | K2e | C1 | – | main guard moved to the end of test_suppressions.py |
| G2-32 | accepted | K2a | C1 | – | Census reads `golangci.LINTERS`; stale comment clause removed |
| G2-33 | accepted | K2a | C1 | – | test_b gets a red call; exact fault list asserted in all 15 subcases of the three tests |
| G2-34 | accepted | K2e | C1 | – | main guard moved to the end of test_verdict.py |
| G2-35 | accepted | K2a | C1 | – | new test runs the matcher over `[PLANTED_MUT, GO-2023-1758]` |
| G2-36 | accepted | K2a | C1 | rename → S1.1c-8 | only the red outcome, exact fault text |
| G2-45 | accepted | K2a | C1 | – | new `Race.test_narrowed_to_one_of_two_packages_is_red`; docstring names race |
| G2-46 | accepted | – | none | in full → S1.1c-8 | rename only |
| G2-47 | accepted | K2e (live data) | C1 | – | all of a path's plants in the one waived file; `want` lists every code |
| G2-48 | accepted | K2e | C1 | – | the pidfile is removed once the process is proven gone, so the cleanup never signals a free pid |
| G2-49 | accepted in part | K2-del | C2 | rename → S1.1c-8 | reversed duplicate `assertEqual` removed; hasattr pin kept (reason given) |
| G2-50 | rejected | – | none | – | the test already fails on a raise (unittest records it as an error) |
| G2-51 | accepted | – | none | in full → S1.1c-8 | a name can cover both assertions; no split needed |
| G2-52 | accepted | K2-del | C2 | rename → S1.1c-8 | the repeated `MEMBER_CAP` check at :718 removed |
| G2-53 | accepted | K2a | C1 | – | a module entry beside the bad go version, whose finding must still come out |
| G2-54 | rejected | – | none | – | seams fail loudly on a rename; the real fix is a production rename, outside S1.1c-7 |
| G3-72 | accepted | K1 | C3 (coordinator coupling) | – | drop "(decision 7)" and "(S1.1c-5)" from the two fault texts |

Totals: 16 fixed (4 of them in part), 2 carried in full to S1.1c-8, 2 rejected. 3 commits.

### Rules that hold for every `--py` proof below

**Red means exactly one failure.**
- pyshim counts every failing subtest as its own failure (`pyshim.py:108-109`, `len(result.failures)`; unittest `case.py` `testPartExecutor` → `addSubTest`).
- A plant that fails k > 1 subtests of the one id therefore reads `failed: k failures`, which is invalid under §2's red rule.
- Every plant here fails exactly one assertion or one subtest of its id. I checked each against the code.

**Git, the real tree and other revisions.**
- None of this chunk's changed tests compares against another revision (merge base, `main`), so none is outside `--py` for that reason.
- Tests that touch git do so only in their own temporary worlds (`support.GateTest`):
  - test_rows_go `Race`
  - test_rows_ruff `Wrapper`
  - the `Row` tests in test_suppressions (not changed here)
- Tests that read files read them from `support.REPO`, which in the detached worktree is the copy, so the committed plant is what they read:
  - test_suppressions `Census` reads `testdata/directives.json` and `golangci`
  - test_run_guard reads `tools.json` and copies `gate/run.py`
  - test_vulndb reads `testdata/vulndb`
  - test_rows_ruff copies `ruff.toml`
- Each finding below says whether `--py` proves it.

**`pkg` is left out of the py entries.** It has no meaning for Python. W0-py's per-mode `REQUIRED_KEYS` decides; if `pkg` turns out to be required, the value is `"gate/runner"`.

### Findings

#### G2-27: `test_each_interpreter_flag_is_required` (test_run_guard.py:71-74)

**1. Verdict: accepted.**
- `gate/run.py:37`: `if not (sys.flags.isolated and sys.dont_write_bytecode and sys.flags.utf8_mode):`.
- The test drops only `-I` (`("-B","-X","utf8")`) or `-B` (`("-I","-X","utf8")`).
- Checked on the pinned 3.15.2: `sys.flags.utf8_mode` is `1` with no `-X utf8`, and `0` with `-X utf8=0`. So leaving out `-X utf8` is never refused, and the `utf8_mode` clause has no test.
- GitNexus `context(refusal)`: called only by `run.main`; calls `pinned_python`.
- The refusal text "must be started with -I -B -X utf8" stays. It is an instruction, and following it always passes the guard (see Choices).

**2. Kind: K2a.** The test passes when the code is wrong: the `utf8_mode` clause can be removed and nothing goes red.

**3. Exact change** (test_run_guard.py:71-74; the file stays Python 3.12-compatible for `pyfloor`):
```python
    def test_each_interpreter_flag_is_required(self) -> None:
        # Python 3.15 runs in UTF-8 mode by default, so leaving out -X utf8 changes nothing; -X utf8=0 turns the mode
        # off, and that is what the guard refuses. --no-such-flag: an interpreter let through stops at the runner's
        # usage at once, never in a gate run (the guard runs before any argument is parsed, run.py:43-50).
        for flags in (("-B", "-X", "utf8"), ("-I", "-X", "utf8"), ("-I", "-B", "-X", "utf8=0")):
            with self.subTest(flags):
                self.assertRefused(*run([pinned("python"), *flags, RUN, "--no-such-flag"]))
```
- **Why the argument changes from `--tier T2` to `--no-such-flag`:** under a guard plant that lets the run through, `--tier T2` would start a real T2 gate inside the test (inside the `--py` worktree). That run would hit the 240 s `faulthandler` ceiling (`exit=True`) and kill the shim before it writes its report, so the outcome would be invalid, never red.
- On the real code the outcome is unchanged: the refusal comes before parsing. `assertRefused` already asserts `"refused:"` present and `"usage:"` absent.
- **Carried to S1.1c-8:** rename to `test_dropping_isolation_or_no_bytecode_or_utf8_mode_is_refused`. Each verb's assertion:
  - "dropping isolation": subcase without `-I` is refused.
  - "or no bytecode": subcase without `-B` is refused.
  - "or utf8 mode": subcase with `-X utf8=0` is refused.

**4. Proof.** `--py` proves it: no git, and the copy's `run.py` and `tools.json` are read.
```json
{"id": "G2-27", "file": "gate/run.py",
 "find": "    if not (sys.flags.isolated and sys.dont_write_bytecode and sys.flags.utf8_mode):\n",
 "replace": "    if not (sys.flags.isolated and sys.dont_write_bytecode):\n",
 "py": "gate.runner.tests.test_run_guard.Guard.test_each_interpreter_flag_is_required",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
{"id": "G2-27-control", "control": true,
 "py": "gate.runner.tests.test_run_guard.Guard.test_each_interpreter_flag_is_required",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
```
- New-side failure: only the `utf8=0` subtest fails, on `assertIn("refused:", output)`. The output is the runner's usage, exit 2.
- The `find` text occurs exactly once (grep).
- Load run: not required. The test starts subprocesses one after another and has no thread and no wait other than `subprocess.run`.

**5. Commit:** C1.

#### G2-28: `test_no_bytecode_lands_in_the_repository` (test_run_guard.py:141-145)

**1. Verdict: accepted.**
- `run_copy()`'s exit code is thrown away.
- `run.py:43-48` returns before `run.py:50` imports `gate.runner.main` when refused, and a crash before the import also writes no bytecode. Either way the walk passes vacuously.
- GitNexus: `refusal` → `main` (as above).

**2. Kind: K2a.**

**3. Exact change:**
```python
    def test_no_bytecode_lands_in_the_repository(self) -> None:
        self.stub("def main(argv):\n    return 0\n")
        code, output = self.run_copy()
        self.assertEqual(code, 0, output)  # the stub ran: main.py was imported, the only way bytecode could land
        names = [n for _, dirs, _ in os.walk(self.repo) for n in dirs]
        self.assertNotIn("__pycache__", names)
```

**4. Proof.** `--py` proves it.
- The plant is a crash before the import: under `-I` the copy's repository is never on `sys.path`. `from gate.runner import main` then raises `ModuleNotFoundError`, which the `__main__` handler turns into exit 2.
```json
{"id": "G2-28", "file": "gate/run.py",
 "find": "    sys.path.insert(0, REPO)\n",
 "replace": "    pass  # planted: the repository is never put on sys.path\n",
 "py": "gate.runner.tests.test_run_guard.Copy.test_no_bytecode_lands_in_the_repository",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
{"id": "G2-28-control", "control": true,
 "py": "gate.runner.tests.test_run_guard.Copy.test_no_bytecode_lands_in_the_repository",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
```
- New-side failure: `assertEqual(code, 0, output)`, with 2 != 0.
- The `find` text occurs exactly once.

**5. Commit:** C1.

#### G2-30, G2-31, G2-34: main guard in the middle of the file

**1. Verdict: accepted, all three** (AST count, read-only):

| File | Guard at | Test methods before the guard | After the guard | `python -m` runs |
|---|---|---|---|---|
| test_sprintid_parts.py | :93-94 | 12 (`Passed`) | 5 (`ReachedPart`, :115) | 12 of 17 |
| test_suppressions.py | :608-609 | 46 | 13 (Census 1, Foreign 3, LineDirective 2, Nolint 5, Enabled 2; the `s1_1c_5_ctx` helper sits there too) | 46 of 59 |
| test_verdict.py | :82-83 | 6 | 2 (`LeftRunning`, :86) | 6 of 8 |

- `support.GateTest` defines no `test*` method (grep), so these counts are the loader's.
- GitNexus (survey): `LeftRunning`'s tests are among `verdict.decide`'s callers.
- The pytests row loads by name (`pyshim.py:90`), so the gate itself runs every test; only direct runs are short.

**2. Kind: K2e** (main guard in the middle of a file).

**3. Exact change.** Pure layout: the module's import-time statements and their order are otherwise unchanged.
- test_sprintid_parts.py: delete lines 93-96 (the guard, `unittest.main()` and the two blank lines after them).
- test_suppressions.py: delete lines 608-611.
- test_verdict.py: delete lines 82-85.
- Each file then ends with two blank lines followed by:
```python
if __name__ == "__main__":
    unittest.main()
```

**4. Proof (K2e check, from the worktree root at each revision, pinned python3.15).** Not a `--py` proof.
```
python3.15 -B -m gate.runner.tests.<module> 2>&1 | grep -E '^Ran [0-9]+ tests? in '
python3.15 -I -B -c "import sys, unittest; sys.path.insert(0, '.'); print(unittest.TestLoader().loadTestsFromName(sys.argv[1]).countTestCases())" gate.runner.tests.<module>
```
- Red: the direct run's count differs from the loader's count. Green: they are equal.
- Expected on the old revision (red): 12 vs 17, 46 vs 59, 6 vs 8.
- Expected on the new revision (green): 17/17, 59/59, 8/8.
- The direct run's pass state does not matter; only the `Ran N` count is compared.

**5. Commit:** C1.

#### G2-32: `Census.test_every_enabled_linter_has_a_row` (test_suppressions.py:656-665)

**1. Verdict: accepted.**
- :659 hard-codes `{"errcheck","forbidigo","funlen","gocritic","gocyclo","lll","nolintlint","revive"}`.
- `golangci.py:125` reads `LINTERS = tuple(PINNED["linters"]["enable"])`, and `golangci.py:38` holds the list. They are equal today.
- The comment "golangci.LINTERS is lane A's, so the list is the design's §2.2 literal here" is stale.
- GitNexus `context(LINTERS)` returns no incoming edges: attribute reads are not indexed. By grep its readers are rows/smells.py:144, test_golangci.py:329, test_rows_smells.py:27/:259 and test_suppressions.py:792-793.
- Census linters today (read from directives.json): errcheck, exclusions, forbidigo, funlen, gocritic, gocyclo, gosec, line, lll, nolint, nolintlint, revive.

**2. Kind: K2a.** A linter enabled in the pinned config without a census row stays green.

**3. Exact change:**
```python
    def test_every_enabled_linter_has_a_row(self):
        # Design §4.1 step 3: at least one row per enabled linter, "none in source" included.
        from gate.runner import golangci  # as the two later tests in this file import it

        linters = set(golangci.LINTERS)
        self.assertLessEqual(linters, {item["linter"] for item in census()})
        ...  # the per-item loop unchanged
```

**4. Proof.** `--py` proves it.
```json
{"id": "G2-32", "file": "gate/runner/golangci.py",
 "find": "        \"enable\": [\"errcheck\", \"forbidigo\", \"funlen\", \"gocritic\", \"gocyclo\", \"lll\", \"nolintlint\", \"revive\"],\n",
 "replace": "        \"enable\": [\"errcheck\", \"forbidigo\", \"funlen\", \"gocritic\", \"gocyclo\", \"lll\", \"misspell\", \"nolintlint\", \"revive\"],\n",
 "py": "gate.runner.tests.test_suppressions.Census.test_every_enabled_linter_has_a_row",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
{"id": "G2-32-control", "control": true, "py": "gate.runner.tests.test_suppressions.Census.test_every_enabled_linter_has_a_row",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "green"}}
```
- New-side failure: `assertLessEqual`, which runs before the subtests, so there is one failure.
- `misspell` has no census row.
- The `find` text occurs exactly once.

**5. Commit:** C1.

#### G2-33: three teeth tests accept any fault (test_teeth.py:222-252)

**1. Verdict: accepted.** Re-checked with `teeth.python_cases` on the current code:
- **"an attribute set in a case"** gives `["teeth: d.py:6: assertRow is a verdict helper's name: a driver may not define or assign it", "teeth: d.py::D.test_b asserts no gate verdict"]`. The second fault keeps `assertTrue(faults)` green with the redefinition rule gone. This confirms the survey.
- **The survey missed the same masking in the module-shape test:**
  - `setattr` gives `[... a module-level Expr ..., ... setattr() at import ...]`;
  - `delattr` gives the same pair;
  - `del` gives `[... a module-level Delete ..., ... a del at import may remove a counted case]`.

  Deleting the hook rule or the del rule leaves the other fault, so each of those subcases stays green.
- GitNexus `context(_defined_names)`: called only by `_shape_faults`.

**2. Kind: K2a.**

**3. Exact change.** Expected texts are exact, as printed by `python_cases`.
```python
    def test_a_driver_may_not_define_a_verdict_helpers_name(self):
        named = "is a verdict helper's name: a driver may not define or assign it"
        for why, (source, fault) in {
            "a method": (
                HEAD + "    def assertGateFailed(self, run):\n        pass\n\n" + RED,
                f"teeth: d.py:5: assertGateFailed {named}",
            ),
            "a class attribute": (HEAD + "    assertRed = print\n\n" + RED, f"teeth: d.py:5: assertRed {named}"),
            "an attribute set in a case": (  # the case has a verdict, so the redefinition is its only fault
                HEAD + case("test_b", "self.assertRow = print", "run = self.run_row()", VERDICT) + RED,
                f"teeth: d.py:6: assertRow {named}",
            ),
            "a module function": ("def assertRed(*args):\n    pass\n\n\n" + HEAD + RED, f"teeth: d.py:1: assertRed {named}"),
            "an import alias": ("from os import path as assertRed\n" + HEAD + RED, f"teeth: d.py:1: assertRed {named}"),
            "assertParityPassed": (
                HEAD + "    def assertParityPassed(self, run, job):\n        pass\n\n" + RED,
                f"teeth: d.py:5: assertParityPassed {named}",
            ),
        }.items():
            with self.subTest(why=why):
                _, faults = teeth.python_cases(source, "d.py")
                self.assertEqual(faults, [fault])

    def test_a_module_shape_that_changes_what_unittest_loads_is_a_fault(self):
        at = "teeth: d.py:11:"  # HEAD and RED are 8 lines; each tail starts with two blank lines
        expr = f"{at} a module-level Expr may change what unittest loads"
        for why, (tail, want) in {
            "setattr": ("\n\nsetattr(D, 'test_red', None)\n", [expr, f"{at} setattr() at import may change what unittest loads"]),
            "delattr": ("\n\ndelattr(D, 'test_red')\n", [expr, f"{at} delattr() at import may change what unittest loads"]),
            "del": (
                "\n\ndel D.test_red\n",
                [f"{at} a module-level Delete may change what unittest loads", f"{at} a del at import may remove a counted case"],
            ),
            "__test__": ("\n\n__test__ = False\n", [f"{at} __test__ decides what unittest loads"]),
            "load_tests assigned": ("\n\nload_tests = None\n", [f"{at} load_tests decides what unittest loads"]),
            "an attribute assigned": (
                "\n\nD.test_red = None\n",
                [f"{at} a module-level assignment to an attribute or item may change what unittest loads"],
            ),
            "an if at module level": ("\n\nif True:\n    D.test_red = None\n", [f"{at} a module-level If may change what unittest loads"]),
            "a call at module level": ("\n\nprint(D)\n", [expr]),
        }.items():
            with self.subTest(why=why):
                _, faults = teeth.python_cases(HEAD + RED + tail, "d.py")
                self.assertEqual(faults, want)

    def test_a_class_level_test_flag_is_a_fault(self):
        _, faults = teeth.python_cases(HEAD + "    __test__ = False\n\n" + RED, "d.py")
        self.assertEqual(faults, ["teeth: d.py:5: __test__ decides what unittest loads"])
```
- `VERDICT` is the module constant at :145. Its definition comes before the class.
- Let `ruff format` wrap the lines.

**4. Proof.** `--py` proves it: the tests are pure. Each plant fails exactly one subtest, or the one assertion.
```json
{"id": "G2-33-attribute", "file": "gate/runner/teeth.py",
 "find": "    if isinstance(node, ast.Attribute) and isinstance(node.ctx, (ast.Store, ast.Del)):\n        return [node.attr]\n",
 "replace": "    # planted: a stored attribute defines no name\n",
 "py": "gate.runner.tests.test_teeth.StrengthRuleTightened.test_a_driver_may_not_define_a_verdict_helpers_name",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
{"id": "G2-33-setattr", "file": "gate/runner/teeth.py",
 "find": "LOADER_HOOKS = frozenset({\"setattr\", \"delattr\",",
 "replace": "LOADER_HOOKS = frozenset({\"delattr\",",
 "py": "gate.runner.tests.test_teeth.StrengthRuleTightened.test_a_module_shape_that_changes_what_unittest_loads_is_a_fault",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
{"id": "G2-33-del", "file": "gate/runner/teeth.py",
 "find": "            fault(node, \"a del at import may remove a counted case\")\n",
 "replace": "            pass  # planted: a del at import is not named\n",
 "py": "gate.runner.tests.test_teeth.StrengthRuleTightened.test_a_module_shape_that_changes_what_unittest_loads_is_a_fault",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
{"id": "G2-33-flag", "file": "gate/runner/teeth.py",
 "find": "fault(node, f\"{name} decides what unittest loads\")",
 "replace": "fault(node, f\"{name} is reserved\")",
 "py": "gate.runner.tests.test_teeth.StrengthRuleTightened.test_a_class_level_test_flag_is_a_fault",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
```
- Plus three control entries, one per test id, each expecting `{"old":"green","new":"green"}`.
- Under `G2-33-attribute`, only "an attribute set in a case" changes. The other five subcases define names through FunctionDef, Name or alias.
- Every `find` text occurs exactly once (grep).

**5. Commit:** C1.

#### G2-35: `PLANTED_MUT` never used (test_vulndb.py:70, :449-455, :474)

**1. Verdict: accepted.**
- GitNexus `context(PLANTED_MUT)`: no incoming references.
- `test_every_row_agrees` (:529-535) loads `PLANTED[entry_id]` only for an AGREEMENT id. The DIFFERS row (:449-455) has id GO-2023-1758, so the matcher sees GO-2023-1758 alone. That repeats the "between the ranges" row at :423.
- The two-entry case the row documents (query mode returned both entries unfiltered) is never run.
- Checked on the real code: `findings({mutagen: "v0.16.7"}, ∅, "go1.27.4", [PLANTED_MUT, GO-2023-1758])` returns `[Finding("GO-2099-0003", …, fixed="")]` with no faults.

**2. Kind: K2a.** A matcher that leaks one entry's blocks into the next entry's verdict passes.

**3. Exact change.**
- AGREEMENT stays as it is: its 39 rows are pinned at :524 for the law model.
- New test in class `Agreement`, after :535:
```python
    def test_two_entries_for_one_module_are_each_judged_by_their_own_ranges(self):
        # The DIFFERS row "query over GO-2023-1758 + PLANTED GO-2099-0003": query mode returned both entries; the
        # matcher, given both, reports only the planted one, which affects every version (the witness that it ran).
        got = scan({"github.com/mutagen-io/mutagen": "v0.16.7"}, "go1.27.4", [PLANTED_MUT, real("GO-2023-1758")])
        self.assertEqual([f.id for f in got], ["GO-2099-0003"])
```

**4. Proof.** `--py` proves it.
- The plant needs two sites, so it is a `--patch` (vulndb.py:255-261 and :266-272 at the base). It moves `blocks` out of the entry loop, so the first entry's blocks carry into the second.
- The planted entry comes first so that the leak reaches GO-2023-1758.
```json
{"id": "G2-35", "patch": "--- a/gate/runner/vulndb.py\n+++ b/gate/runner/vulndb.py\n@@ -255,6 +255,7 @@\n     except InvalidVersion:\n         go = None\n         faults.append((\"\", \"\", f\"vulns: the go version {tag} cannot be ordered: stdlib and toolchain not judged\"))\n+    blocks: dict[str, list[Sequence[Mapping[str, object]]]] = {}  # planted: one entry's blocks leak into the next\n     for entry in entries:\n         entry_id = entry.get(\"id\") if isinstance(entry, Mapping) else None\n         if not isinstance(entry_id, str) or not isinstance(entry.get(\"affected\"), list):\n@@ -266,7 +267,6 @@\n         if withdrawn is not None:\n             faults.append((entry_id, \"\", f\"vulns: {entry_id}: withdrawn {_shown(withdrawn)} is not an RFC 3339 time\"))\n             continue\n-        blocks: dict[str, list[Sequence[Mapping[str, object]]]] = {}\n         for block in entry[\"affected\"]:\n             package = block.get(\"package\") if isinstance(block, Mapping) else None\n             if not isinstance(package, Mapping) or package.get(\"ecosystem\") != \"Go\":\n",
 "old_py": "gate.runner.tests.test_vulndb.Agreement.test_every_row_agrees",
 "new_py": "gate.runner.tests.test_vulndb.Agreement.test_two_entries_for_one_module_are_each_judged_by_their_own_ranges",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
```
- Plus a control with `old_py`/`new_py` as above, expecting `{"old":"green","new":"green"}`.
- **Old side green:** every AGREEMENT row calls `findings` with one entry, so nothing can leak.
- **New side red:** the result is `["GO-2023-1758", "GO-2099-0003"]`.
- If A1a's note edits `findings`, rebuild the patch at the lane base. The two edits are the same.

**5. Commit:** C1.

#### G2-36: `test_an_experiment_suffix_is_parsed_or_red` (test_vulns_row.py:179-184)

**1. Verdict: accepted.**
- `rows/vulns.py:223`: `head, sep, go = lines[0].rpartition(": ")` gives `go1.27.0 X:boringcrypto`.
- `vulndb.go_version` raises on it (`_GO_TAG.fullmatch`, vulndb.py:158-160), so `build_info` always returns `(None, faults)` (:256-257) and the `else` branch is dead.
- Real code output: `(None, ["vulns: golangci-lint: go version -m: the Go version 'go1.27.0 X:boringcrypto' cannot be ordered"])`.
- GitNexus `context(build_info)`: callers are `rows/vulns.scan` and five `BuildInfo` tests; it calls `vulndb.go_version` and `_module_fields`.

**2. Kind: K2a.** A silent strip of the suffix passes.

**3. Exact change:**
```python
    def test_an_experiment_suffix_is_parsed_or_red(self):
        # vulndb.go_version refuses the suffix (test_vulndb's GoVersion pins it): never a silent strip.
        info, faults = vulns.build_info(TOOL, PIN, 0, minimal(MOD, DEP, go="go1.27.0 X:boringcrypto"))
        self.assertIsNone(info)
        self.assertEqual(
            faults, [f"vulns: {TOOL}: go version -m: the Go version 'go1.27.0 X:boringcrypto' cannot be ordered"]
        )
```
**Carried to S1.1c-8:** rename to `test_an_experiment_suffix_is_red_naming_the_version`.
- "is red" ← `assertIsNone(info)`.
- "naming the version" ← the exact fault text.

**4. Proof.** `--py` proves it: the function is pure.
```json
{"id": "G2-36", "file": "gate/runner/rows/vulns.py",
 "find": "    head, sep, go = lines[0].rpartition(\": \")\n",
 "replace": "    head, sep, go = lines[0].rpartition(\": \")\n    go = go.split(\" \")[0]  # planted: the experiment suffix stripped in silence\n",
 "py": "gate.runner.tests.test_vulns_row.BuildInfo.test_an_experiment_suffix_is_parsed_or_red",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
```
- Plus a control.
- Old side: the old `else` branch sees `("go1.27.0", [])`, so it is green.
- The `find` text occurs exactly once.
- If A1a's G10 fixes edit `build_info`, re-check this `find` at the lane base.

**5. Commit:** C1.

#### G2-45: `Race` has no narrowed case (test_rows_go.py:1-4, :197-234)

**1. Verdict: accepted.**
- The module docstring promises that every wrapper's narrowed run is red against a full control. `Race` (:197) has only `test_passing_packages`, `test_a_data_race_fails_naming_its_test` and `test_a_repeat_runs_every_test_that_many_times` (that last one runs `./...`).
- `rows/race.py:28` passes `*(patterns or ["./..."])`, and `:30` covers `len(packages)`.
- GitNexus `context(race.main)`: calls `rows.run_tool`, `unit.judge`, `rows.covered`, `rows.finish`.

**2. Kind: K2a.** A `race.py` that ignores its package arguments passes every Race test.

**3. Exact change:**
- Docstring line 1: "The real gofmt, vet and unit wrappers" → "The real gofmt, vet, unit and race wrappers".
- New test in `Race`, after `test_passing_packages` (its full control):
```python
    def test_narrowed_to_one_of_two_packages_is_red(self):
        self.package("a")
        self.package("b")
        self.assertEqual(self.race("./a"), "covered 1 ≠ expected 2")
```
"≠" is U+2260, as in `Unit`'s twin at :194.

**4. Proof.** `--py` proves it: the world is a temporary git repository, and `wrapper()` runs the copy's `rows/race.py`.
```json
{"id": "G2-45", "file": "gate/runner/rows/race.py",
 "find": "*(patterns or [\"./...\"])",
 "replace": "\"./...\"",
 "old_py": "gate.runner.tests.test_rows_go.Race.test_passing_packages",
 "new_py": "gate.runner.tests.test_rows_go.Race.test_narrowed_to_one_of_two_packages_is_red",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
```
- Plus a control.
- Under the plant, all three old Race tests stay green. The new one reads "covered 2".
- **Load:** `run_rows` runs the runner on a thread (`support.py:214`), so §7.2's rule applies: `python3.15 -I -B -X utf8 gate/runner/stress.py --runs 50 gate.runner.tests.test_rows_go` must read 50/50 for every test.

**5. Commit:** C1.

#### G2-46: `test_a_compile_error_prints_no_object_and_still_fails` (test_rows_go.py:122-126)

**Verdict: accepted, carried in full to S1.1c-8.** No commit.
- "prints no object" describes go vet's output, not the wrapper's behaviour. There is no production plant for a tool's output, so a K2a assertion on it would have no proof.
- The behaviour is already proven:
  - the anchored regex `^exit 1: go vet exit 1:` makes vet's exit the row's first failure line;
  - every diagnostic carries an absolute `posn` that sorts before "go vet exit", so a parsed diagnostic would break the anchor (rows/vet.py:42-45, `sorted(failures)`).
- Proposed name: `test_a_compile_error_fails_on_vets_exit_code_naming_the_file`.
  - "fails on vet's exit code" ← the reason's first failure is `go vet exit 1: …`.
  - "naming the file" ← `b/b.go:3:13` in the reason and `b/b.go` in stdout.

#### G2-47: `test_each_waiver_covers_its_own_file_only` (test_rows_ruff.py:139-153)

**1. Verdict: accepted.**
- ruff.toml:39-44 has six entries with one code each.
- With two codes on one path, the second `self.write(path, …)` overwrites the first plant, so the first waiver is never exercised in its file.
- `want` lists only the `_0` files, so the test goes falsely red.

**2. Kind: K2e (a live-data pin).**

**3. Exact change:**
```python
        for i, (path, codes) in enumerate(sorted(waivers.items())):
            # Every code's plant in the waived file at once, TID251's import first (E402 is selected), so each waiver
            # is exercised there.
            self.write(path, "\n\n".join(plants[code] for code in sorted(codes, key=list(plants).index)))
            for j, code in enumerate(codes):
                self.write(f"gate/x/elsewhere_{i:02}_{j}.py", plants[code])
        self.git("add", "--", "gate")
        self.ruff_row()
        failed = [line for line in self.out().splitlines() if line.startswith("failed: ")]
        want = [
            f"failed: gate/x/elsewhere_{i:02}_{j}.py: ruff check (exit 1)"
            for i, (_, codes) in enumerate(sorted(waivers.items()))
            for j in range(len(codes))
        ]
        self.assertEqual(failed, want)
```
- The combined file is `import time\n\ntime.sleep(0)\n\n\ndef f():\n    try:\n        return 1\n    except Exception:\n        return 2\n`, which is ruff-format clean.
- With one code per entry (today), the plants and `want` are byte-identical to before.

**4. Proof: K2e, live data.**
- The scratch copy is `mutate.py --py`'s detached worktree. `Wrapper.setUp` copies `support.REPO/ruff.toml`, which is the copy's.
- The ruff row does not read the pin (`rows/ruff.py` has no `pin.`).
```json
{"id": "G2-47", "file": "ruff.toml",
 "find": "\"gate/wait.py\" = [\"TID251\"]",
 "replace": "\"gate/wait.py\" = [\"TID251\", \"BLE001\"]",
 "py": "gate.runner.tests.test_rows_ruff.Wrapper.test_each_waiver_covers_its_own_file_only",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "red", "new": "green"}}
```
- Plus a control (green/green).
- Old side: one failure, an extra `elsewhere_05_1` line.
- ruff.toml is coordinator-owned law data. The plant exists only in the throwaway worktree. It is not under `*_test.go`, `tests/` or `testdata/`.
- **Load:** `stress.py --runs 50 gate.runner.tests.test_rows_ruff`, 50/50 (the runner runs on a thread).

**5. Commit:** C1.

#### G2-48: `kill_child` can signal a reused pid (test_stop_children.py:64-68)

**1. Verdict: accepted.**
- The cleanup sends SIGKILL to the pid in `child.pid` whatever happened before:
  - in `StressHarness` (:164-165) the test has just proven that pid free (`ProcessLookupError`);
  - in `DriverRunProcess` `assertGone` has proven the child exited.
- After either, the pid can belong to an unrelated process. The window is small but real, and it breaks "each test owns its world".
- GitNexus `impact(kill_child)`: risk UNKNOWN, 0 callers. It is registered through `addCleanup` (:24), which the index does not link (grep).

**2. Kind: K2e** (a structure or hygiene defect in a test helper, with no production plant possible).

**3. Exact change:**
```python
    def kill_child(self) -> None:
        """The cleanup: a child the test did not prove gone is killed. A proven-gone child's pidfile is removed, so a
        pid that may already be another process's is never signalled."""
        with contextlib.suppress(FileNotFoundError, ValueError), open(self.pidfile, encoding="ascii") as f:
            pid = int(f.read())
            with contextlib.suppress(ProcessLookupError):
                os.kill(pid, signal.SIGKILL)

    def proven_gone(self) -> None:
        """The child is gone, so its pid may be reused at once: the cleanup must not signal it."""
        os.unlink(self.pidfile)
```
- `assertGone`: add `self.proven_gone()` after the `try/except` around `wait.until`.
- `StressHarness.test_a_stopped_harness_kills_its_hogs_and_starts_no_further_run`: add `self.proven_gone()` after the `with self.assertRaises(ProcessLookupError): os.kill(hog, 0)` block.
- Why the pidfile always exists at those points:
  - the child writes it before its witness byte, which `w.started(1)` awaits;
  - in `StressHarness`, `one_hog` writes it before the run's witness;
  - a missing file would be a loud error, never a silent pass.

**4. Proof (K2e check, at each revision, from the worktree root).** Not `--py`.
```
python3.15 -I -B -c "
import os, signal, sys, unittest
sys.path.insert(0, os.getcwd())
sent, real = [], os.kill
def kill(pid, sig):
    if sig == signal.SIGKILL:
        sent.append(pid)
    return real(pid, sig)
os.kill = kill
suite = unittest.TestLoader().loadTestsFromName('gate.runner.tests.test_stop_children')
result = unittest.TextTestRunner(verbosity=0).run(suite)
print('passed' if result.wasSuccessful() and result.testsRun == 3 else 'failed', 'sigkills', len(sent))
"
```
- Red: `passed sigkills 3` on the old revision, one per test from `kill_child`.
- Green: `passed sigkills 0` on the new revision.
- The cleanup's `Popen.kill` sends nothing on a passing run, because `exit_code` already set `returncode`.
- **Load:** `stress.py --runs 50 gate.runner.tests.test_stop_children`, 50/50.

**5. Commit:** C1.

#### G2-49: `Enabled.test_enabled_equals_the_config_both_ways` (test_suppressions.py:787-794)

**1. Verdict: accepted in part.**
- :793 is :792 reversed. Equality of frozensets is symmetric, so :793 can never fail once :792 has passed.
- The `hasattr(suppressions, "ENABLED")` pin (:794) stays. It is neither weaker than nor a copy of another check: it is the only check that the removed constant stays removed (rows/suppressions.py:244 reads the config). Deleting a check that catches something nothing else catches is not a truthfulness fix and has no K2-del proof. The name mismatch is handled by the carried rename.

**2. Kind: K2-del** (a repeated assertion).

**3. Exact change:** delete line 793, `        self.assertEqual(frozenset(golangci.LINTERS), enabled)`.
**Carried to S1.1c-8:** rename to `test_the_enabled_set_is_the_configs_and_no_enabled_constant_exists`.
- "is the config's" ← `assertEqual(enabled, frozenset(golangci.LINTERS))`.
- "no enabled constant exists" ← `assertFalse(hasattr(suppressions, "ENABLED"))`.

**4. Proof: the covered-set form, §7.2's alternative.**
- A per-assertion plant is impossible: any plant that fails :793 fails :792 first.
- Python reading of the alternative: the set of gate production lines executed by the test, measured with stdlib `trace`. It is run twice at each revision, and all four outputs must be equal.
```
python3.15 -I -B -c "
import hashlib, os, sys, trace, unittest
root = os.getcwd(); sys.path.insert(0, root)
suite = unittest.TestLoader().loadTestsFromName(sys.argv[1])
tracer = trace.Trace(count=1, trace=0)
result = tracer.runfunc(unittest.TextTestRunner(verbosity=0).run, suite)
gate, tests = os.path.join(root, 'gate') + os.sep, os.path.join(root, 'gate', 'runner', 'tests') + os.sep
lines = sorted({(os.path.relpath(f, root), n) for (f, n) in tracer.results().counts if f.startswith(gate) and not f.startswith(tests)})
print('passed' if result.wasSuccessful() else 'failed', len(lines), hashlib.sha256(repr(lines).encode()).hexdigest())
" gate.runner.tests.test_suppressions.Enabled.test_enabled_equals_the_config_both_ways
```
- Expected: four identical `passed <n> <sha256>` lines.
- The deleted line runs no gate code: it compares two sets that already exist.
- Not `--py`.

**5. Commit:** C2.

#### G2-50: `test_a_critical_block_without_a_term_raises_nothing` (test_term.py:47-50)

**Verdict: rejected.** The test fails for its stated reason.
- Any exception out of the block, including `term.Terminated` (a BaseException), is caught by unittest's executor. The pinned 3.15 `unittest/case.py:54-75` has a bare `except:` after `KeyboardInterrupt` and `SkipTest`, and records it with `_addError`. pyshim fails any error (`pyshim.py:108-109`).
- A body that never runs is impossible: a `@contextlib.contextmanager` that does not yield raises `RuntimeError`, which is again an error.
- Evidence plant: `term.py:60` `if not _held["inside"] and _held["pending"]:` → `if not _held["inside"]:` makes the old test error.
- GitNexus `context(critical)`: three test callers, plus `gorun.run`, `support.run_process` and `stress.hogs`.
- The only gap is the outcome word: under `mutate.py` an error reads invalid, not red. That matters only if a future K1 on `critical` wants this test as its proof. That K1 would write an `assertRaises` (§2, "a crash defect").

#### G2-51: `test_the_cap_constants` (test_vulndb.py:402-404)

**Verdict: accepted, carried in full to S1.1c-8.**
- One name can cover both assertions, so the split the finding proposes (a Q4 split, for the later part) is not needed.
- Proposed name: `test_the_caps_are_64_and_4_mib_and_unknown_event_and_member_too_large_are_value_errors`.
  - "caps are 64 and 4 MiB" ← `(FETCH_CAP, MEMBER_CAP) == (64 << 20, 4 << 20)`.
  - "… are value errors" ← the `issubclass` line.
- If S1.1c-8's plan check prefers the split, the split goes to the later part (Q4).

#### G2-52: the agreement-row count, and the repeated `MEMBER_CAP` check (test_vulndb.py:520-524, :718)

**1. Verdict: accepted.**
- :718 `assertEqual(vulndb.MEMBER_CAP, 4 * 1024 * 1024)` repeats :403.
- :524 `len(AGREEMENT) == 39` is not covered by the test's name.

**2. Kind: K2-del** for :718.

**3. Exact change:** delete line 718.
**Carried to S1.1c-8:** rename `test_the_testdata_holds_exactly_the_real_entries_and_their_notice` → `test_the_testdata_holds_exactly_the_real_entries_and_notice_and_agreement_has_39_rows`.
- "holds exactly the real entries and notice" ← the `listdir` equality and the id checks.
- "agreement has 39 rows" ← `len(AGREEMENT) == 39`.
- The finding's alternative, a split, is Q4's (later part). Outcome: fixed in part.

**4. Proof.** `--py` proves it. The fault text "exceeds 4 MiB" is a literal at vulndb.py:412, so the plant leaves the subtests green.
```json
{"id": "G2-52", "file": "gate/runner/vulndb.py",
 "find": "MEMBER_CAP = 4 << 20",
 "replace": "MEMBER_CAP = 5 << 20",
 "old_py": "gate.runner.tests.test_vulndb.CheckDb.test_a_member_past_4_mib_is_red",
 "new_py": "gate.runner.tests.test_vulndb.Findings.test_the_cap_constants",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "red", "new": "red"}}
```
- Old side: red on :718, the removed assertion, with one failure.
- New side: red on :403, the remaining one.
- The `find` text occurs exactly once.

**5. Commit:** C2.

#### G2-53: "the walk going on" is not exercised (test_vulndb.py:386-399)

**1. Verdict: accepted.**
- The test passes `{}` modules and a single stdlib entry.
- `test_fix_s11c4.py:146-156` (CA-L9) also has no non-stdlib module. So `findings` returning right after the go-version fault (vulndb.py:257) passes the whole suite.
- GitNexus `context(findings)`: callers are `rows/vulns.scan` and `binary_findings`, plus the tests named above.
- Real code output for the new input: `([Finding("GO-2099-0030", "example.com/a", "v1.0.0", "2.0.0", True)], [the go-version fault])`.

**2. Kind: K2a.**

**3. Exact change** (`GO-2099-0030` is unused in the file, by grep):
```python
        e = entry("GO-2099-0029", ("stdlib", semver({"introduced": "0"}, {"fixed": "1.27.5"})))
        m = entry("GO-2099-0030", ("example.com/a", semver({"introduced": "0"}, {"fixed": "2.0.0"})))
        found, faults = vulndb.findings({"example.com/a": "v1.0.0"}, frozenset(), "devel go1.28-0123abcd", [e, m])
        self.assertEqual(found, [vulndb.Finding("GO-2099-0030", "example.com/a", "v1.0.0", "2.0.0", True)])
        # the faults assertion unchanged
```

**4. Proof.** `--py` proves it.
```json
{"id": "G2-53", "file": "gate/runner/vulndb.py",
 "find": "        faults.append((\"\", \"\", f\"vulns: the go version {tag} cannot be ordered: stdlib and toolchain not judged\"))\n",
 "replace": "        faults.append((\"\", \"\", f\"vulns: the go version {tag} cannot be ordered: stdlib and toolchain not judged\"))\n        return found, faults  # planted: the walk stops at a bad go version\n",
 "py": "gate.runner.tests.test_vulndb.Findings.test_an_invalid_go_version_is_a_fault_never_a_finding_or_a_crash",
 "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
```
Plus a control.

**5. Commit:** C1.

#### G2-54: private seams (`vulndb._parse`, `stress._one_run`, `subprocess.Popen._execute_child`)

**Verdict: rejected.** Every use fails loudly on a rename, so none can pass vacuously:
- a direct call raises `AttributeError`;
- `mock.patch.object` raises `AttributeError` on a missing attribute;
- in test_term.py:62-79 a seam that is no longer called leaves `forked` empty, and `assertEqual(len(forked), 1)` fails.

Why the alternatives are not taken:
- **Testing through public functions:** ordering tests through `affected` would weaken `Versions`. GitNexus `impact(_parse)`: 9 direct callers, risk MEDIUM, all in the Tests module.
- **Making the seams public:** that is a production rename, and S1.1c-7 renames no production function (§0 item 4).
- **`Popen._execute_child`:** has no public equivalent. Both interpreters are pinned (3.15.2, floor 3.12.16), so a version bump meets this test in the pytests row.

If the owner wants named public seams, they belong with the later part's public-name work (G6, S1.1c-10).

#### G3-72: gate faults ending in "(decision 7)" and "(S1.1c-5)" (suppressions.py:61-62; test_suppressions.py:616-619)

**1. Verdict: accepted.**
- GitNexus `context(go_foreign)`: callers are `suppressions.read`, `test_breakit_s11c2 NosecIsAWaiver.test_a_nosec_is_read`, and four tests in test_suppressions. It sits in the flows "Exemptions → _foreign" and "In_code → _foreign".
- `go_foreign` appends `f"{where}: {LINE}"` (:254) and `f"{where}: {FOREIGN.format(linter=linter)}"` (:259).

**2. Kind: K1** (production text). R5 does not apply: gate Python, no per-event Go path.

**3. Exact change.** New text is chosen in Choices below. The `…` is U+2026 and its bytes do not change.
- suppressions.py:61-62:
```python
FOREIGN = "{linter} honours this form; the only waiver is //nolint:{linter} // PERMANENT|TEMP …"
LINE = "a //line directive moves findings between files; not allowed"
```
- test_suppressions.py:616-619:
```python
FOREIGN_FAULT = "{where}: {linter} honours this form; the only waiver is //nolint:{linter} // PERMANENT|TEMP …"
LINE_FAULT = "{where}: a //line directive moves findings between files; not allowed"
```
- test_breakit_s11c2.py:359 (lane A1's file): `NOSEC_FAULT = "{where}: gosec honours this form; the only waiver is //nolint:gosec // PERMANENT|TEMP …"`.
- gate/proofs/drivers/test_suppressions_planted.py:42-43 is a planted driver, so the coordinator edits it in the same commit:
  - `FOREIGN = "{linter} honours this form; the only waiver is //nolint:{linter} // PERMANENT|TEMP …"`
  - `LINE_DIRECTIVE = "a //line directive moves findings between files; not allowed"`
- New red tests, each a single assertion so its plant gives exactly one failure. Inputs and outputs were checked with the current `go_foreign`.
  - In `Foreign`:
```python
    def test_a_foreign_fault_ends_with_the_waiver_to_write(self):
        _, faults = suppressions.go_foreign("p.go", "package p\n\nvar _ = 1 //revive:disable-line\n")
        self.assertIn("…", suppressions.FOREIGN)  # the ellipsis, byte-exact
        self.assertEqual(
            faults, ["p.go:3: revive honours this form; the only waiver is //nolint:revive // PERMANENT|TEMP …"]
        )
```
  - In `LineDirective`:
```python
    def test_a_line_directive_fault_ends_with_not_allowed(self):
        _, faults = suppressions.go_foreign("p.go", "package p\n\n//line x.go:1\nvar _ = 1\n")
        self.assertEqual(faults, ["p.go:3: a //line directive moves findings between files; not allowed"])
```

**4. Proof: K1 revert plants, red expected** (a plain batch on the fix commit, each expected red; `--expect` needs both revisions and the find text exists only at the new one — coordinator, after DQ-S1). `--py` proves it.
```json
{"id": "G3-72-foreign", "file": "gate/runner/suppressions.py",
 "find": "//nolint:{linter} // PERMANENT|TEMP …\"", "replace": "//nolint:{linter} // PERMANENT|TEMP … (decision 7)\"",
 "py": "gate.runner.tests.test_suppressions.Foreign.test_a_foreign_fault_ends_with_the_waiver_to_write"}
{"id": "G3-72-line", "file": "gate/runner/suppressions.py",
 "find": "moves findings between files; not allowed\"", "replace": "moves findings between files; not allowed (S1.1c-5)\"",
 "py": "gate.runner.tests.test_suppressions.LineDirective.test_a_line_directive_fault_ends_with_not_allowed"}
```
- Expected failing line: each new test's `self.assertEqual(faults, [...])`.
- Each `find` occurs once after the fix (`PERMANENT|TEMP` appears only in `FOREIGN`).
- The revert plant needs no new declaration.
- Then `--job proofs` alone at T4: the driver is a law path (law.json:613), and its case count is unchanged.

**5. Commit:** C3, a coordinator coupling commit.

### Commit plan

**C1: `gate/runner` tests, K2 (a and e), test files only, 13 findings.**
- Findings: G2-27, -28, -30, -31, -32, -33, -34, -35, -36, -45, -47, -48, -53.
- Files: test_run_guard.py, test_sprintid_parts.py, test_suppressions.py, test_teeth.py, test_verdict.py, test_vulndb.py, test_vulns_row.py, test_rows_go.py, test_rows_ruff.py, test_stop_children.py.
- Proofs:
  - the `--expect` entries above for G2-27, -28, -32, -33 (4 entries), -35, -36, -45, -47, -53, with their controls;
  - the main-guard count check for -30, -31, -34;
  - the SIGKILL check for -48;
  - stress 50/50 for test_rows_go, test_rows_ruff and test_stop_children.
- Lane A1's gate/runner K2 total passes 12 once A1a's are added, so §1 allows this split by file.

**C2: `gate/runner` tests, K2-del, 2 findings.**
- G2-49: covered set, four identical lines.
- G2-52: `--expect` red/red.

**C3: coordinator coupling commit, K1, G3-72.**
- Files: suppressions.py, test_suppressions.py, test_breakit_s11c2.py:359, and the coordinator's test_suppressions_planted.py:42-43.
- Proofs: two revert plants, red. Then `--job proofs` alone.
- The coordinator may fold C3 into the same coupling commit as A1a's G10 text changes, if those also move driver-asserted texts (test_vulns_planted.py:435-630). That stays one kind, K1.

**Chunk commit count: 3.** A1's ceiling is 4 + 2. C3 can be counted against the plan's "coupling commits" line instead.
Order: C1 → C2 → C3. All three touch test_suppressions.py, so they must land one after another. Each hand-back touches `gate/`, so the proofs row runs alone at T4.

Batch file: `docs/sprints/s1-1c-7-plants/A1.json` (merged with A1a's), committed with the proof docs.

### Choices made

**1. G3-72: the two fault texts.**
- Input: any honoured foreign directive, or a `//line` directive, in a Go file.
- Candidates:
  - (a) drop the parenthetical;
  - (b) replace it with a decisions.md anchor;
  - (c) point at DEVELOPMENT.md's suppression policy.
- Chosen: (a).
  - The text already says what to do ("the only waiver is //nolint:<linter> // PERMANENT|TEMP …"; "not allowed").
  - Its sibling faults carry no pointer either: `TYPECHECK`, `ALL`, `STANDALONE` at suppressions.py:51-60.
  - An anchor drifts as decisions.md grows.
- Dry run (`git grep "honours this form"`, `git grep "moves findings between files"`, `git grep "(decision 7)"`):
  - gate/runner/suppressions.py:61-62 (the change itself);
  - gate/runner/tests/test_suppressions.py:617, :619;
  - gate/runner/tests/test_breakit_s11c2.py:359;
  - gate/proofs/drivers/test_suppressions_planted.py:42-43 (coordinator);
  - docs/sprints/s1-1c-5-design.md:372-373, a history document left unchanged.
  - Nothing else pins either text (gate/canary, law.json and registry.json, by grep).

**2. G2-27: the refusal text "must be started with -I -B -X utf8" is kept.**
- The text is an instruction, and following it always passes the guard. Under the new subcase the UTF-8-mode clause is enforced.
- No user text changes, so no dry run.
- The test's argument change to `--no-such-flag` is a test input chosen for proof validity (see the finding), not behaviour.

**3. G2-49:** the hasattr pin is kept. Reason given in the finding.

**4. G2-50 and G2-54:** rejected. Reasons given in the findings.

**5. G2-46 and G2-51:** carried in full as renames rather than splits. A name can carry every assertion, and Q4 puts splits in the later part.

### Couplings and risks

**§0 item 5 couplings:**
- G3-72 moves asserted texts in the planted driver `test_suppressions_planted.py`. That needs a coordinator commit, followed by `--job proofs`.
- The whole-`gate/` copy (`copy_gate`): every commit here, then the proofs row alone at T4.
- None of the chunk's test files is a law path. test_run_guard.py is in the `pyfloor` row's `expected_scope` by path only (registry.json:188, law.json:394, timeout 120 s). G2-27 adds one fast refused start, and the file stays 3.12-clean.
- No canary pin, registry `-run` pin or `min_wants` count is touched.

**R5 (per-event code):** none. All edits are gate Python. **R4 (frozen benchmarks):** none. **Frame tables and line-anchored mutants:** none.

**Other owners' files:**
- gate/proofs/drivers/test_suppressions_planted.py:42-43: coordinator, per §0 item 5.
- test_breakit_s11c2.py:359 is lane A1's, but it may be listed in chunk A1a. The coordinator should assign that one line to C3.
- ruff.toml (G2-47) is touched only as a plant inside `mutate.py`'s worktree, never edited.

**Cross-chunk risk:** A1a's G10-7…13 fixes may edit vulndb.py `findings` or rows/vulns.py `build_info`. If so, re-check G2-35's patch hunks and the `find` texts of G2-36 and G2-53 at the lane base before the batch is committed. The plant semantics do not change.

**Flags for W0-py and the coordinator:**
- pyshim counts each failing subtest as one failure. Every subtest-bearing plant here fails exactly one subtest, so red is as §2 defines it.
- The ledger checker's K2-del rule must accept the covered-set form (§7.2 allows it). G2-49 uses it, through stdlib `trace` as this note's Python reading of `-coverpkg`. Reviewers should confirm that reading.
- G2-47's plant path is `ruff.toml`, a config file. The checker refuses only `*_test.go`, `tests/` and `testdata/` paths, so it accepts this one.

**Pre-mortem (the-fool, failure modes) on the riskiest fixes:**
- **G3-72:** a driver left on the old text turns T4's proofs row red. Mitigation: one commit for the text, its tests and the driver, then `--job proofs` alone.
- **G2-48:** `os.unlink` on a missing pidfile would error. It cannot be missing: the writer runs before the awaited witness, and the failure would be loud, not a false green.
- **G2-27:** a guard plant with `--tier T2` would hang until the 240 s ceiling and read invalid. That is why the argument is now `--no-such-flag`.
- **G2-33:** exact texts include line numbers. They are computed from fixed synthetic sources (HEAD and RED are 8 lines), and the current code printed every one.

### Carried items

**Renames to S1.1c-8** (proposed name ← assertions):

| Id | Current test | Proposed name | Assertions behind it |
|---|---|---|---|
| G2-27 | `Guard.test_each_interpreter_flag_is_required` | `test_dropping_isolation_or_no_bytecode_or_utf8_mode_is_refused` | the three subcases refused |
| G2-36 | `BuildInfo.test_an_experiment_suffix_is_parsed_or_red` | `test_an_experiment_suffix_is_red_naming_the_version` | `assertIsNone(info)`; the exact fault |
| G2-46 | `Vet.test_a_compile_error_prints_no_object_and_still_fails` | `test_a_compile_error_fails_on_vets_exit_code_naming_the_file` | the anchored `go vet exit 1` first line; `b/b.go` named. **In full.** |
| G2-49 | `Enabled.test_enabled_equals_the_config_both_ways` | `test_the_enabled_set_is_the_configs_and_no_enabled_constant_exists` | the `assertEqual`; the `hasattr` pin |
| G2-51 | `Findings.test_the_cap_constants` | `test_the_caps_are_64_and_4_mib_and_unknown_event_and_member_too_large_are_value_errors` | the caps tuple; `issubclass`. **In full.** |
| G2-52 | `Agreement.test_the_testdata_holds_exactly_the_real_entries_and_their_notice` | `test_the_testdata_holds_exactly_the_real_entries_and_notice_and_agreement_has_39_rows` | `listdir` equality and ids; `len(AGREEMENT) == 39` |

**Non-simple parts to the later part (Q4):**
- the G2-51 and G2-52 splits, only if S1.1c-8's plan check prefers splits to the names above;
- G2-54's public seams, only if the owner wants them (G6 work in S1.1c-10).

Findings not reached: none.

---

### Coordinator corrections after the second QA pass (2026-11-20; evidence: review-design-qa2.md)

1. **G2-46 stays "carried in full to S1.1c-8" (rename only), with its reason restated (DQ2-L4).** A plant exists (rows/vet.py:41-42 could print an object, which the anchored regex would miss), but asserting that no object appears would pin go vet's own output shape for a package that fails to compile, which this repo does not control. The rename records what the test does check.
2. **G3-72's proof** is a plain batch on the fix commit, each entry expected red (corrected earlier, after DQ-S1).
