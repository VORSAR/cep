# S1.1c-7 design note — A1a (gate/runner, rows, first test files)

*S1.1c-7's design note for chunk A1a (gate/runner, rows, first test files), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes. Its G3-72 section is superseded by A1b's identical design (coordination item 12); lane A1's one commit list is in the coordination record.*


## The note

## S1.1c-7 design note, chunk A1a (lane A1: gate/runner, rows, first tests files)

### 1. Header

- **Chunk:** A1a. **Findings:** 22: G10-7…-13, G3-72, and G2-18…-23, -26, -37…-44 as listed in chunk-A1a.md.
- **Files read in full or in the relevant part:**
  - production: rows/suppressions.py, rows/vulns.py, rows/pytests.py, rows/race.py, rows/__init__.py, rows/apidiff.py:84-100, rows/smells.py:222-250, scheduler.py:280-310 and :520-545, proc.py:20-100, teeth.py:255-445, suppressions.py:50-75 and :240-310, knownissues.py:50-70, clock.py, verdict.py, step.py, pin.py, gate/wait.py;
  - tests: every test file the findings name, plus test_rows_go.Race, test_step.RunFiles, test_teeth helpers and support.py;
  - drivers: test_suppressions_planted.py, test_vulns_planted.py, test_racestress_planted.py, test_pytests_vacuity.py.
- **GitNexus calls: 8.**
  - `impact` on ruff_pin, _methods, tracked_under_gate and go_foreign;
  - `context` on open_copy, _json_file, _write_go_env and rows/race.py:main.
- **Index freshness:** every response said "1 commit behind HEAD". HEAD is d521725, and that commit touches only coordinator files (parts.py, test_parts/aging/sprintid, sprints.json, smells.json, docs), none of this chunk's symbols. I could not read the `gitnexus://repo/cep/context` resource because no resource-reading tool was available.
- **Read-only Python checks:** I checked exact fault texts with `python3 -I -B` snippets that write no files, calling pure functions (tomllib, go_foreign, suppressions.read, knownissues.check). Every text marked "(verified)" below comes from those runs.
- **Not checked:**
  - W0's final `--py` rule for a test whose subtests all fail. To avoid depending on it, every test I add or change gives exactly one failure under its plant.
  - The `--py` timeout setting (G2-20, G2-22, G2-42 need long ones).
- **Hindsight:** no memory tools were available, so none was used.

### 2. Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-7 | accepted | K1 | C1 | – | ruff_pin checks the per-file tables' shape: one named fault, no crash, no per-character faults |
| G10-8 | accepted | K1 | C1 | – | open_copy/_json_file name an unreadable copy file: `vulns: cannot read <path>: <strerror>: run make vulndb` |
| G10-9 | accepted | K1 | C1 | – | a failed `git ls-files` in pytests becomes a `failed:` line through rows.finish |
| G10-10 | accepted | K1 | C1 | – | a `--repeat=` value that is not a whole number gives a `failed: usage:` line before go runs |
| G10-11 | accepted | K1 | C1 | – | a failed side input keeps the file faults already found |
| G10-12 | accepted | K1 | C1 | – | go-env.json written only on exit 0; otherwise the partial is removed and the exit code noted |
| G10-13 | accepted (input corrected) | K1 | C1 | – | `_methods` gets a `seen` guard like `_is_testcase` |
| G3-72 | accepted | K1 (coupling) | C2 | – | "(decision 7)" and "(S1.1c-5)" dropped from FOREIGN and LINE; the driver changes in the same coordinator commit |
| G2-18 | accepted | K2e | C3 | – | main guard moved to the end; `os` and `support` imports moved to the top |
| G2-19 | **rejected** | – | – | – | the claim is wrong (reason in §3) |
| G2-20 | accepted | K2a | C3 | – | exact assertion on the rename verdict |
| G2-21 | accepted | K2a | C3 | rename → S1.1c-8 | four exact fault lines; one malformed input fixed |
| G2-22 | accepted | K2a | C3 | – | witness that the started timer's thread ended without firing |
| G2-23 | accepted | K2e | C3 | – | main guard moved to the end |
| G2-26 | accepted | K2e | C3 | – | main guard moved to the end |
| G2-37 | accepted | K2e | C3 | – | DocLine becomes a `unittest.TestCase` |
| G2-38 | accepted | K2a | C3 | – | exact Directive asserted |
| G2-39 | accepted | – | – | **carried in full to S1.1c-8** | rename only |
| G2-40 | accepted | K2e | C3 | – | Copy built with `zip=` injected; no `.cache`/`.zip` access |
| G2-41 | accepted (proposed fix wrong) | K2a | C3 | – | exact keys refusal pinned; a fresh copy per field; no hand-called `setUp()` |
| G2-42 | accepted | K2d | C3 | – | real-timer row watchdogs (30 s) turn the one-worker hang into a named red |
| G2-43 | accepted | K2e | C3 | – | a `# private seam:` comment above the `_q` access |
| G2-44 | accepted | K2a | C3 | – | asserts no killpg after reap, and nothing spawns |

### 3. One section per finding

#### G10-7: rows/suppressions.py:159 `ruff_pin`

1. **Verdict: accepted.**
   - :152 drops `lint.per-file-ignores` from the pin, and :159 runs `lint.get(key, {}).items()` for both RUFF_WAIVERS keys.
   - Verified:
     - `per-file-ignores = ["E501"]` → `AttributeError: 'list' object has no attribute 'items'`;
     - `{"gate/x.py" = "E501"}` → four faults (codes `0`, `1`, `5`, `E`);
     - `{"gate/x.py" = [501]}` → `waives 501`;
     - `lint.extend-per-file-ignores = ["E501"]` → AttributeError.
   - GitNexus `impact`: d=1 is `main` (rows/suppressions.py), RuffPinReadsEveryWaiver.pin, RuffPin.pin and RuffPinRefuses.pin; risk LOW, exact. `main` has no try around the call (:250).
2. **Kind: K1.** It is a behaviour fix in gate Python. R5 does not apply (no Go per-event code).
3. **Change.**
   - Production, rows/suppressions.py. Add after `_pair`:
     ```python
     def _waived(lint: dict) -> tuple[set[tuple[str, str]], list[str]]:
         """The (file, code) pairs lint's per-file tables waive, and a fault for each table that is not a table of string
         lists (tomllib gives a list, a string or a number as readily as a table)."""
         waived: set[tuple[str, str]] = set()
         faults: list[str] = []
         for key in RUFF_WAIVERS:
             table = lint.get(key, {})
             if type(table) is not dict or any(
                 type(codes) is not list or any(type(code) is not str for code in codes) for codes in table.values()
             ):
                 faults.append(f"{RUFF_TOML}: lint.{key} is not a table of code lists")
                 continue
             waived |= {(file, code) for file, codes in table.items() for code in codes}
         return waived, faults
     ```
   - In `ruff_pin`, replace line 159 (`waived = {(file, code) for key in RUFF_WAIVERS for file, codes in lint.get(key, {}).items() for code in codes}`) with:
     ```python
         waived, shape = _waived(lint)
         if shape:  # a both-ways comparison against an unreadable table would name only false pairs
             return faults + shape
     ```
   - Test: a new class in gate/runner/tests/test_suppressions.py after `RuffPinRefuses`:
     ```python
     SHAPE = "ruff.toml: lint.{key} is not a table of code lists"


     class RuffWaiverTableOfTheWrongShape(support.GateTest):
         """lint.per-file-ignores is left out of the whole-document pin (ruff.json checks it both ways), so its shape is
         checked on its own: a table of string lists, or one named fault and no both-ways comparison."""

         def pin(self, lines: str) -> list[str]:
             text = pinned_ruff_toml().replace("[lint]\n", f"[lint]\n{lines}", 1)
             self.assertNotEqual(text, pinned_ruff_toml())  # witness: the edit applied
             self.write("ruff.toml", text)
             with contextlib.chdir(self.repo):
                 try:
                     return row.ruff_pin({})
                 except (AttributeError, TypeError) as e:  # the row died with a traceback and no named fault
                     self.fail(f"ruff_pin raised {e!r}")

         def test_a_list_in_place_of_the_table_is_named(self):
             self.assertEqual(self.pin('per-file-ignores = ["E501"]\n'), [SHAPE.format(key="per-file-ignores")])

         def test_a_string_in_place_of_a_code_list_is_named_once(self):
             self.assertEqual(
                 self.pin('per-file-ignores = {"gate/x.py" = "E501"}\n'), [SHAPE.format(key="per-file-ignores")]
             )

         def test_a_code_that_is_not_a_string_is_named(self):
             self.assertEqual(self.pin('per-file-ignores = {"gate/x.py" = [501]}\n'), [SHAPE.format(key="per-file-ignores")])

         def test_an_extend_table_of_the_wrong_shape_is_named(self):
             self.assertEqual(
                 self.pin('extend-per-file-ignores = ["E501"]\n'),
                 [
                     "ruff.toml: lint.extend-per-file-ignores: not pinned (remove it)",
                     SHAPE.format(key="extend-per-file-ignores"),
                 ],
             )
     ```
   - The pin line in the last test follows from pin.py's `_walk` (a key not in RUFF_PINNED["lint"] gives `not pinned (remove it)`).
4. **Proof.** `mutate.py --py <id> --patch <revert>` on C1, once for each of the four ids `gate.runner.tests.test_suppressions.RuffWaiverTableOfTheWrongShape.<test>`.
   - **Revert plant:** one hunk in rows/suppressions.py, replacing the three new lines with the old line 159. `_waived` stays defined but unused.
   - **Expected red lines:** the `self.fail(...)` in `pin` for the list and extend tests (old AttributeError); the `assertEqual` for the string and int tests.
5. **Commit:** C1.

#### G10-8: rows/vulns.py:75-84 `_json_file`, :94-131 `open_copy`

1. **Verdict: accepted.**
   - :79 and :102 catch only FileNotFoundError.
   - A directory at vulndb.zip, stamp.json or fetch-error.txt raises IsADirectoryError out of `scan` (:339). `main` (:439) has no handler.
   - GitNexus `context`: open_copy ← scan only; _json_file ← open_copy only; exact.
2. **Kind: K1.**
3. **Change.** This picks behaviour no design decides; see §5, choice 1.
   - rows/vulns.py, `_json_file`: docstring "(present, parsed): parsed is the ValueError when the file is not JSON, the OSError when it exists but cannot be read." After `except FileNotFoundError: return False, None`, add `except OSError as e: return True, e`.
   - Add the helper:
     ```python
     def _unreadable(path: str, e: OSError) -> str:
         return f"vulns: cannot read {path}: {e.strerror}: run make vulndb"
     ```
   - In `open_copy`, after `except FileNotFoundError: ...` at :102-103, add:
     ```python
         except OSError as e:
             copy.faults.append(_unreadable(path, e))
     ```
   - Replace :114-115 with:
     ```python
         stamp_path = os.path.join(root, vulndb_fetch.STAMP)
         present, stamp = _json_file(stamp_path)
         if isinstance(stamp, OSError):  # the copy's origin cannot be read: it is not scanned
             copy.faults.append(_unreadable(stamp_path, stamp))
             copy.zip, present, stamp = None, False, None
         copy.stamp = stamp if present and not isinstance(stamp, ValueError) else ("not JSON" if present else None)
     ```
   - Replace :117-118 with:
     ```python
         error_path = os.path.join(root, vulndb_fetch.ERROR)
         present, error = _json_file(error_path)
         if isinstance(error, OSError):
             copy.faults.append(_unreadable(error_path, error))
         elif present:
     ```
     The old `if present:` body stays as the `elif` body.
   - Test in test_vulns_row.py: add `import errno`, and after `Exempt`:
     ```python
     class UnreadableCopyFile(Scan):
         """A file of the copy that exists but cannot be read (a directory in its place) is one named fault, never a
         traceback out of scan; an unreadable vulndb.zip or stamp.json leaves the copy unscanned."""

         def scan_faults(self, name: str) -> list[str]:
             path = os.path.join(self.copy, name)
             if os.path.exists(path):
                 os.remove(path)
             os.mkdir(path)
             dep = gomods.Module("example.com/dep", "v1.0.0", "", "", "", True)
             try:
                 faults, self.lines = self.scan(loaded(dep))
             except OSError as e:  # the row died with a traceback and no named fault
                 self.fail(f"scan raised {e!r}")
             return faults

         def unreadable(self, name: str) -> str:
             return f"vulns: cannot read {os.path.join(self.copy, name)}: {os.strerror(errno.EISDIR)}: run make vulndb"

         def test_a_directory_in_place_of_the_zip_is_named(self):
             self.assertEqual(self.scan_faults("vulndb.zip"), [self.unreadable("vulndb.zip")])

         def test_a_directory_in_place_of_the_stamp_is_named_and_the_copy_is_not_scanned(self):
             self.assertEqual(self.scan_faults("stamp.json"), [self.unreadable("stamp.json")])
             self.assertFalse(any(" entries; " in x for x in self.lines), self.lines)

         def test_a_directory_in_place_of_the_fetch_error_is_named_and_the_copy_is_scanned(self):
             self.assertEqual(self.scan_faults("fetch-error.txt"), [self.unreadable("fetch-error.txt")])
             self.assertTrue(any("1 entries; 1 modules (1 built)" in x for x in self.lines), self.lines)
     ```
   - Use a directory, not chmod: CI may run as root, and root can read a mode-000 file.
4. **Proof.** `--py --patch` on C1 for the three ids `gate.runner.tests.test_vulns_row.UnreadableCopyFile.test_*`.
   - **Revert plant:** remove the new `except OSError` in `open_copy` and in `_json_file`, and restore :114-118 to their old text. Keep `_unreadable`.
   - **Expected red line:** the `self.fail` in `scan_faults` (IsADirectoryError out of scan).
   - **Driver check (§0 item 5):** test_vulns_planted.py:429-693 asserts `no vulndb.zip at`, `is not a zip`, `no stamp.json at`, `fetched … days ago`, `has not changed since`, `sha256 is not the stamp's`, `last make vulndb failed at`, `db.json is missing or malformed`, and the findings and waiver texts. All are kept, because the change only adds branches for an OSError other than FileNotFoundError.
5. **Commit:** C1.

#### G10-9: rows/pytests.py:22-26 `tracked_under_gate`

1. **Verdict: accepted.** :25 raises `SystemExit(f"git ls-files failed: …")`. The message goes to stderr with no `failed:` line, so verdict.py:54 gives a bare `exit 1`. GitNexus `impact`: d=1 is pytests `main` only; exact, LOW.
2. **Kind: K1.**
3. **Change.**
   - Production:
     ```python
     def tracked_under_gate(row_dir: str) -> set[str]:
         out, err = os.path.join(row_dir, "tracked.stdout"), os.path.join(row_dir, "tracked.stderr")
         code = rows.run_tool(["git", "ls-files", "-z", "--", "gate"], out, err)
         if code != 0:
             raise OSError(f"git ls-files: exit {code}: {rows.read_text(err).strip()}")
         return {rows.show(path) for path in rows.read_list(out)}
     ```
     The wording copies rows/suppressions.py:80.
   - In `main`, replace :90 with:
     ```python
         try:
             tracked = tracked_under_gate(row_dir)
         except OSError as e:  # the walk needs the tracked set: one named line, as every row's fault
             rows.covered(0)
             return rows.finish([str(e)])
         failures = misplaced(tracked, row_scope(row_dir, paths)) + missing_init(paths)
     ```
   - Test: a new class in test_rows_py.py:
     ```python
     class TrackedListFailure(support.GateTest):
         def test_a_failed_git_ls_files_is_a_failed_line(self):
             outside = os.path.join(self.root, "outside")  # no repository here or above: git ls-files fails
             os.mkdir(outside)
             listing = os.path.join(self.root, "files.nul")
             with open(listing, "wb"):
                 pass
             env = dict(support.base_env(), GATE_ROW_DIR=self.run_dir, GIT_CEILING_DIRECTORIES=self.root)
             argv = [sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "pytests.py"), listing]
             code, out, err = support.run_tool(argv, outside, env)
             text = out.decode("utf-8", errors="replace")
             self.assertEqual(code, 1, err.decode("utf-8", errors="replace"))
             self.assertRegex(text, r"\AGATE_COVERED 0\nfailed: git ls-files: exit [1-9][0-9]*: fatal: not a git repository")
             self.assertEqual(text.count("failed: "), 1, text)
     ```
4. **Proof.** `--py --patch` on C1: `gate.runner.tests.test_rows_py.TrackedListFailure.test_a_failed_git_ls_files_is_a_failed_line`.
   - **Revert plant:** restore the SystemExit raise and `main`'s old line :90.
   - **Expected red line:** the `assertRegex` (stdout is empty).
   - **Driver check:** test_pytests_vacuity.py's worlds are git repositories, so `ls-files` succeeds and its asserted texts are unchanged.
5. **Commit:** C1.

#### G10-10: rows/race.py:24

1. **Verdict: accepted.** `int(arg.removeprefix("--repeat="))` raises ValueError, giving a traceback and a bare `exit 1`. GitNexus `context`: `main` ← the file only; it calls `rows.start`, `run_tool`, `unit.judge`, `covered` and `finish`.
2. **Kind: K1.**
3. **Change.** See §5, choice 2.
   - Production:
     ```python
             if arg.startswith("--repeat="):
                 value = arg.removeprefix("--repeat=")
                 try:
                     repeat = int(value)
                 except ValueError:
                     rows.covered(0)
                     return rows.finish([f"usage: --repeat=N with N a whole number, not {value!r}"])
     ```
   - Test in test_rows_go.py, class `Race`:
     ```python
         def test_a_repeat_that_is_not_a_number_is_a_usage_failure(self):
             self.package("a")
             self.assertEqual(self.race("--repeat=x", "./..."), "exit 1: usage: --repeat=N with N a whole number, not 'x'")
             self.assertFalse(os.path.exists(os.path.join(self.run_dir, "rows", "race", "race", "test.json")))  # go never ran
     ```
4. **Proof.** `--py --patch` on C1: `gate.runner.tests.test_rows_go.Race.test_a_repeat_that_is_not_a_number_is_a_usage_failure`. It needs the pinned go.
   - **Revert plant:** restore the old one-line `repeat = int(...)`.
   - **Expected red line:** the first `assertEqual` (the reason is `exit 1`).
   - **Driver check:** test_racestress_planted.py:7,31 passes `--repeat=20`, which is valid, so it is unchanged.
5. **Commit:** C1.

#### G10-11: rows/suppressions.py:241-246 `main`

1. **Verdict: accepted.** `in_code` runs first (:239). On SIDE_FAULTS, :246 returns `[f"suppressions: {e}"]` and drops `file_faults`. (grep) `file_faults` is otherwise used only at :253.
2. **Kind: K1.**
3. **Change.**
   - Replace :246 with:
     ```python
             return rows.finish([f"suppressions: {fault}" for fault in [str(e), *file_faults]])
     ```
   - Test in test_suppressions.py, class `Row`:
     ```python
         def test_a_side_input_that_fails_keeps_the_listed_files_faults(self):
             self.world("docs/sprints/decisions.md#title")
             self.track("gate/known-issues.json", "{")  # not JSON: a side input that fails
             self.track(".githooks/pre-commit", "#!/usr/bin/env python3\nimport os  # noqa\n")
             code, out = self.run_row(["tools/allowlists/purity.json", ".githooks/pre-commit"])
             failed = [line for line in out.splitlines() if line.startswith("failed: ")]
             self.assertEqual(code, 1, out)
             self.assertEqual(
                 failed,
                 [
                     "failed: suppressions: Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
                     f"failed: suppressions: .githooks/pre-commit:2: {NOQA}",
                 ],
                 out,
             )
     ```
   - The JSON error text is verified.
4. **Proof.** `--py --patch` on C1: `gate.runner.tests.test_suppressions.Row.test_a_side_input_that_fails_keeps_the_listed_files_faults`.
   - **Revert plant:** restore :246.
   - **Expected red line:** the second `assertEqual` (only one line).
   - **Driver check:** no case in test_suppressions_planted.py fails a side input, so its asserted texts are unchanged.
5. **Commit:** C1.

#### G10-12: scheduler.py:296-309 `Runner._write_go_env`

1. **Verdict: accepted.**
   - :307-308 throw away `reap`'s return code and rename the partial file. `proc.Group.reap` returns `(returncode, left)` (proc.py:55-70).
   - GitNexus `context`: ← `_parity_work`; calls `write_atomic`, `Group.reap`, `Group.spawn`.
   - (grep) No proof driver reads go-env.*. test_step.py:360-378 holds the two existing go-env tests.
2. **Kind: K1.** It changes a run file, not a verdict.
3. **Change.**
   - Production:
     ```python
             if proc is not None:
                 code, _ = self._parity_group.reap(proc)
                 if code == 0:
                     os.rename(partial, os.path.join(self._run_dir, "go-env.json"))
                 else:  # its partial stdout describes nothing: dropped, the exit code noted beside go's own stderr
                     os.remove(partial)
                     with open(errors, "ab") as f:
                         f.write(f"go env: exit {code}\n".encode())
     ```
   - Test in test_step.py, class `RunFiles`:
     ```python
         def test_a_go_env_that_fails_leaves_no_go_env_json(self):
             bindir = os.path.join(self.root, "bin")
             self.script(
                 "go", bindir,
                 "import sys\nsys.stdout.write('{\"GOOS\": ')\nsys.stderr.write('go: planted failure\\n')\nsys.exit(3)\n",
             )
             os.symlink(shutil.which("git"), os.path.join(bindir, "git"))
             self.toolchain.dirs = [bindir]
             result = self.run_rows([self.row("a")], environ=dict(support.base_env(), PATH=""), record_go_env=True)
             self.assertEqual(result.exit_code, 0)
             self.assertFalse(os.path.exists(os.path.join(self.run_dir, "go-env.json")), "a failed go env was recorded")
             self.assertFalse(os.path.exists(os.path.join(self.run_dir, "go-env.json.tmp")))
             with open(os.path.join(self.run_dir, "go-env.stderr"), encoding="utf-8") as f:
                 self.assertEqual(f.read(), "go: planted failure\ngo env: exit 3\n")
     ```
4. **Proof.** `--py --patch` on C1: `gate.runner.tests.test_step.RunFiles.test_a_go_env_that_fails_leaves_no_go_env_json`.
   - **Revert plant:** restore the old two lines.
   - **Expected red line:** the first `assertFalse`.
   - It runs a thread through the runner's parity thread, but the fix is not concurrent and the test asserts after `join`, so no load run is needed.
5. **Commit:** C1.

#### G10-13: teeth.py:293-300 `_methods`

1. **Verdict: accepted, with the finding's input corrected.**
   - The stated `class A(B)` / `class B(A)` never reaches `_methods`: `_judge` (:406) calls it only when `_is_testcase` is true, and `_is_testcase`'s `seen` makes a pure cycle not a test case.
   - The defect is real when one class also names a TestCase base: `class D(Base, support.DriverTest)` with `class Base(D)`. Then `_methods(D)` → `Base` → `D` → … ends in RecursionError.
   - GitNexus `impact`: d=1 `_judge`; d=2 `python_cases` and `red_rows`; d=3 only tests (test_teeth, test_breakit_s11c2, test_instruments_pin.RealTree); LOW, exact.
2. **Kind: K1.**
3. **Change.** The guard only, with no new fault text (§5, choice 3).
   - Production:
     ```python
     def _methods(
         cls: ast.ClassDef, classes: Mapping[str, ast.ClassDef], seen: frozenset[str] = frozenset()
     ) -> dict[str, Function]:
         """The class's methods over its bases defined in the same module, each base walked once on a path (a cycle of
         bases parses, though the module could never import)."""
         out: dict[str, Function] = {}
         for base in cls.bases:
             if (
                 isinstance(base, ast.Name)
                 and base.id in classes
                 and classes[base.id] is not cls
                 and base.id not in seen
             ):
                 out.update(_methods(classes[base.id], classes, seen | {cls.name}))
         out.update({n.name: n for n in cls.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))})
         return out
     ```
   - Test in test_teeth.py, class `StrengthRuleTightened`:
     ```python
         def test_bases_that_form_a_cycle_are_walked_once(self):
             source = HEAD.replace("class D(support.DriverTest)", "class D(Base, support.DriverTest)") + RED + (
                 "\nclass Base(D):\n    pass\n"
             )
             try:
                 found = teeth.python_cases(source, "d.py")
             except RecursionError:
                 self.fail("a cycle of bases sends the methods walk round it until RecursionError")
             self.assertEqual(found, (["D.test_red"], []))
     ```
4. **Proof.** `--py --patch` on C1: `gate.runner.tests.test_teeth.StrengthRuleTightened.test_bases_that_form_a_cycle_are_walked_once`.
   - **Revert plant:** the old signature and condition.
   - **Expected red line:** the `self.fail`.
   - **Driver check:** no real driver has a base cycle, so the instruments pin's counts are unchanged.
5. **Commit:** C1.

#### G3-72: suppressions.py:61-62 FOREIGN and LINE (production text users see)

1. **Verdict: accepted.**
   - :61 ends "(decision 7)" and :62 ends "(S1.1c-5)". Both reach every fault `go_foreign` makes (:254, :259); output verified.
   - GitNexus `impact(go_foreign)`: 6 direct callers; flows `exemptions` (rows/lawdelta.py) and `in_code` (rows/suppressions.py); **risk MEDIUM (warning)**.
   - lawdelta counts Directives, never these texts, so only fault text changes.
2. **Kind: K1** (plan §0 item 8). It goes in a **coupling commit**, because a planted driver asserts both texts.
3. **Change.** New text, §5 choice 4:
   ```python
   FOREIGN = "{linter} honours this form; the only waiver is //nolint:{linter} // PERMANENT|TEMP …"
   LINE = "a //line directive moves findings between files; not allowed"
   ```
   - **Dry run.** Every pin of the old text, from `git grep "honours this form\|moves findings between files"`:
     - gate/runner/suppressions.py:61-62 (production);
     - gate/runner/tests/test_suppressions.py:616-619, `FOREIGN_FAULT` and `LINE_FAULT`, used at :678, :680, :709;
     - gate/runner/tests/test_breakit_s11c2.py:359 `NOSEC_FAULT`, used at :369;
     - **gate/proofs/drivers/test_suppressions_planted.py:42-43, `FOREIGN` and `LINE_DIRECTIVE`. The coordinator changes these in the same commit.**
     - docs/sprints/s1-1c-5-design.md:372-373 is historical material and stays as it is.
     - No pin in gate/canary/pin_tests.json.
   - Each pin drops the same parenthetical.
   - New K1 test in test_suppressions.py, class `LineDirective`. It has no subtests, so it gives one failure:
     ```python
         def test_a_line_directive_and_a_nosec_are_refused_naming_only_the_rule(self):
             _, faults = suppressions.go_foreign("p.go", "package p\n\n//line x.go:1\nvar _ = 1 // #nosec G401\n")
             self.assertEqual(
                 faults,
                 [
                     "p.go:3: a //line directive moves findings between files; not allowed",
                     "p.go:4: gosec honours this form; the only waiver is //nolint:gosec // PERMANENT|TEMP …",
                 ],
             )
     ```
4. **Proof.** `--py --patch` on C2: `gate.runner.tests.test_suppressions.LineDirective.test_a_line_directive_and_a_nosec_are_refused_naming_only_the_rule`.
   - **Revert plant:** restore the old :61-62.
   - **Expected red line:** the `assertEqual`.
   - Then `--job proofs` alone at T4: the driver's case count is unchanged, and LAW Δ shows the driver and `gate/runner/`.
5. **Commit:** C2, the coordinator commit carrying the lane's change.

#### G2-18: test_allowlists.py:392-393, guard before LaneDPackages; local imports at :409-411

1. **Verdict: accepted.** The guard is mid-file, so a direct run exits before `LaneDPackages` (:396) is defined.
2. **Kind: K2e** (main guard).
3. **Change.**
   - Move `if __name__ == "__main__":\n    unittest.main()` to the end of the file.
   - Add `import os` to the stdlib block and `from gate.runner.tests import support` after `from gate.runner import allowlists, sprintid`.
   - Delete the local imports at :409-411.
4. **Proof.** From the worktree root:
   - `python3.15 -B -m gate.runner.tests.test_allowlists 2>&1 | grep -E '^Ran [0-9]+ tests? in'`
   - against `python3.15 -B -c 'import unittest, gate.runner.tests.test_allowlists as m; print(unittest.defaultTestLoader.loadTestsFromModule(m).countTestCases())'`.
   - **Old: red** (the direct count is 2 lower). **New: green** (equal).
5. **Commit:** C3.

#### G2-19: test_allowlists.py:417 `test_a_planted_entry_is_seen`

**Verdict: rejected.**
- The real-file test (:408-415) and the planted test both compute the same pure expression, `sorted(self.packages(text) & set(self.LANE_D))`, over a text. The planted test runs that expression on an entry for `internal/table`.
- The only thing a copy of the real file would add is the real file's target shape, and the smells row already enforces it: rows/smells.py:237-240 reds any target that is not `"<package dir> <rule>"`. All 13 real targets have that form (checked).
- So planting into the real file proves nothing the current test does not.
- The repeated expression is a duplication note for the later part's duplication work. It needs no ledger carry.

#### G2-20: test_apidiff_breakit.py:85

1. **Verdict: accepted.** The only assertion is `assertNotIn("at the merge base failed")`. rows/apidiff.py:91-98 states the rename verdict.
2. **Kind: K2a.**
3. **Change.** Replace :85 with:
   ```python
           self.assertEqual(
               reason,
               "exit 1: module renamed from ex.com/m to ex.com/n: every public import path changed (incompatible)",
               f"{reason}\n--- stdout:\n{self.stdout()}",
           )
   ```
4. **Proof.** `--expect` entry:
   ```json
   {"id": "G2-20", "file": "gate/runner/rows/apidiff.py",
    "find": "[f\"module renamed from {base_module} to {module}: every public import path changed (incompatible)\"]",
    "replace": "[]",
    "py": "gate.runner.tests.test_apidiff_breakit.ApidiffBreakit.test_renamed_module_is_compared_not_a_tool_failure",
    "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   ```
   - The control is the same `py` with `"control": true`, expect `{"old": "green", "new": "green"}`.
   - The plant makes a rename pass silently (reason `covered 1`).
   - The merge base lives in the test's own world, not in the cep repository, so `--py` applies.
   - It needs the pinned go and builds apidiff (row timeout_s 300), so it needs a long `--py` timeout.
5. **Commit:** C3.

#### G2-21: test_breakit_s11c2.py:143-162 `RuffPinReadsEveryWaiver`

1. **Verdict: accepted.**
   - Four cases assert only that some fault exists.
   - The input of `test_a_top_level_per_file_ignores_is_pinned` is malformed. `[per-file-ignores]` is placed before SELECT's top-level keys, so `target-version` and `line-length` fall into that table. Verified: it gives three faults, two of them unrelated (`line-length: missing`, `target-version: missing`).
   - GitNexus `impact(ruff_pin)`: these four are d=2 callers through `RuffPinReadsEveryWaiver.pin`.
2. **Kind: K2a.**
3. **Change** (texts verified):
   ```python
   TOP = "ruff.toml sets top-level {key}: write per-file waivers under [lint] (ruff deprecates it)"
   EXTEND = "ruff.toml sets extend: the ruff pin reads ruff.toml alone; write every setting there"

       def test_a_top_level_ignore_is_red(self):
           self.assertEqual(self.pin('ignore = ["TID251"]\n' + SELECT), [TOP.format(key="ignore")])

       def test_a_top_level_extend_ignore_is_red(self):
           self.assertEqual(self.pin('extend-ignore = ["TID251"]\n' + SELECT), [TOP.format(key="extend-ignore")])

       def test_a_top_level_per_file_ignores_is_pinned(self):
           # the table after the top-level keys, so the waiver is the document's only change
           self.assertEqual(
               self.pin(SELECT + '\n[per-file-ignores]\n"gate/x.py" = ["TID251"]\n'), [TOP.format(key="per-file-ignores")]
           )

       def test_a_waiver_in_an_extended_file_is_pinned(self):
           self.assertEqual(
               self.pin('extend = "docs/base.toml"\n' + SELECT, base='[lint.per-file-ignores]\n"gate/x.py" = ["TID251"]\n'),
               [EXTEND],
           )
   ```
   - **Carried to S1.1c-8:**
     - `test_a_top_level_per_file_ignores_is_pinned` → `test_a_top_level_per_file_ignores_is_red`;
     - `test_a_waiver_in_an_extended_file_is_pinned` → `test_a_waiver_in_an_extended_file_is_red`. The finding did not name this one; I added it.
     - The verb "red" is backed by the `assertEqual` on the single refusal line.
   - **Outcome: fixed in part.**
   - The three cases other than extend repeat `test_suppressions.RuffPinRefuses`; that duplication goes to the later part.
4. **Proof.** `--expect` entries, file gate/runner/rows/suppressions.py.
   - **P21a**, a two-hunk patch. It drops the clearer texts; the pin's generic line remains, so the old tests stay green:
     - hunk 1: `    if "extend" in top:` → `    if False:`
     - hunk 2: `        if top.get(key):  # an empty one waives nothing, as for lint.ignore` → `        if False:  # an empty one waives nothing, as for lint.ignore`
     - Entries G2-21a, -21b and -21d use it, with py ids `…RuffPinReadsEveryWaiver.test_a_top_level_ignore_is_red`, `…test_a_top_level_extend_ignore_is_red` and `…test_a_waiver_in_an_extended_file_is_pinned`, each expect `{"old": "green", "new": "red"}`.
   - **P21b**, a two-hunk patch. It makes the top-level waiver unseen:
     - hunk 1 as P21a's hunk 2;
     - hunk 2: `    pinned = {**top, "lint": {key: value for key, value in lint.items() if key != "per-file-ignores"}}` → `    pinned = {**{k: v for k, v in top.items() if k not in (*RUFF_GLOBAL, *RUFF_WAIVERS)}, "lint": {key: value for key, value in lint.items() if key != "per-file-ignores"}}`
     - Entry G2-21c, py `…test_a_top_level_per_file_ignores_is_pinned`, expect `{"old": "green", "new": "red"}`. Under P21b the old malformed input still has its two unrelated faults, which is exactly the finding's defect.
   - Control: `…test_the_lint_table_is_red_today_as_a_control`, `"control": true`, expect green/green.
   - All find texts are exact-once (grep -c = 1). None is touched by C1.
5. **Commit:** C3.

#### G2-22: test_clock.py:39 `test_a_cancelled_timer_does_not_fire`

1. **Verdict: accepted.** A 3600 s delay can never fire inside the test, so `cancel()` is never tested. clock.py:31-36 returns the started `threading.Timer`.
2. **Kind: K2a.** The test runs threads, so a load run is needed too.
3. **Change.** Add `import threading`.
   ```python
       def test_a_cancelled_timer_does_not_fire(self):
           q = queue.SimpleQueue()
           before = set(threading.enumerate())
           timer = clock.RealTimers().start("cancelled", 3600, lambda: q.put("cancelled"))
           started = [t for t in threading.enumerate() if t not in before]
           self.assertEqual(len(started), 1)  # witness: start started one timer thread
           timer.cancel()
           try:
               wait.until("the cancelled timer's thread to end", lambda: not started[0].is_alive(), cap_s=30)
           except wait.WaitTimeout as e:  # WaitTimeout is not an AssertionError: named as this test's failure
               self.fail(f"cancel did not reach the timer start started: {e}")
           self.assertTrue(q.empty())  # its thread ended without calling it, so it never will
   ```
4. **Proof.**
   - `--expect` entry:
     ```json
     {"id": "G2-22", "file": "gate/runner/clock.py",
      "find": "        return timer\n", "replace": "        return threading.Timer(delay_s, fire)\n",
      "py": "gate.runner.tests.test_clock.RealTimers.test_a_cancelled_timer_does_not_fire",
      "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
     ```
     The plant returns an unstarted Timer, so `cancel` misses the started one.
   - Control: the same py, `"control": true`, green/green.
   - The `--py` timeout must exceed 30 s.
   - Load: `python3.15 -I -B -X utf8 gate/runner/stress.py --runs 50 gate.runner.tests.test_clock.RealTimers.test_a_cancelled_timer_does_not_fire`, 50/50.
5. **Commit:** C3.

#### G2-23: test_deps.py:405-406, guard before ReplaceApproval

1. **Verdict: accepted.**
2. **Kind: K2e.**
3. **Change:** move the guard to the end of the file.
4. **Proof:** the G2-18 commands with `gate.runner.tests.test_deps`. Old red (`ReplaceApproval`'s tests are missing from the direct run), new green.
5. **Commit:** C3.

#### G2-26: test_registry.py:236-237, guard before TheRepositorysRegistry (:240)

1. **Verdict: accepted.**
2. **Kind: K2e.**
3. **Change:** move the guard to the end of the file.
4. **Proof:** the G2-18 commands with `gate.runner.tests.test_registry`. Old red (one test short), new green.
5. **Commit:** C3.

#### G2-37: test_breakit_s11c1.py:21 `DocLine(support.GateTest)`

1. **Verdict: accepted.** The two tests call only `law.doc_line` (pure). GateTest.setUp runs `git init` and a commit's worth of tracking for each case.
2. **Kind: K2e** (the structure of the test's world).
3. **Change:** `class DocLine(unittest.TestCase):` and add `import unittest` after `import os`. Folding into test_law.DocLine (:558) is a merge, carried to the later part as optional. **Outcome: fixed.**
4. **Proof.**
   ```
   python3.15 -B -c 'import unittest; from unittest import mock; from gate.runner.tests import support; from gate.runner.tests import test_breakit_s11c1 as m
   with mock.patch.object(support, "run_tool", side_effect=AssertionError("a git world was built")):
       r = unittest.TextTestRunner().run(unittest.defaultTestLoader.loadTestsFromTestCase(m.DocLine))
   raise SystemExit(0 if r.wasSuccessful() and r.testsRun == 2 else 1)'
   ```
   Old: exit 1 (setUp's `git` errors). New: exit 0.
5. **Commit:** C3.

#### G2-38: test_breakit_s11c2.py:302-304 `test_a_generated_header_is_read`

1. **Verdict: accepted.** It asserts only `!= ([], [])`. Verified result: `([Directive('internal/x/x.go', 1, ('generated',), 'GENERATED', {}, True)], [])`.
2. **Kind: K2a.**
3. **Change:**
   ```python
           self.assertEqual(
               suppressions.read("internal/x/x.go", source),
               ([suppressions.Directive("internal/x/x.go", 1, ("generated",), "GENERATED", {})], []),
               "the header passes unseen",
           )
   ```
4. **Proof.**
   ```json
   {"id": "G2-38", "file": "gate/runner/suppressions.py",
    "find": "Directive(path, line, (\"generated\",), \"GENERATED\", {})",
    "replace": "Directive(path, line, (\"generated\",), \"MARKER\", {})",
    "py": "gate.runner.tests.test_breakit_s11c2.GeneratedHeaderIsAnExemption.test_a_generated_header_is_read",
    "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   ```
   - The find is exact-once; :307's form has `line + k`.
   - The plant's wrong kind makes rows/suppressions.generated() (:182) skip the file, so an unlisted generated file passes the row.
   - Control: the same py, `"control": true`, green/green.
5. **Commit:** C3.

#### G2-39: test_clock.py:15,34

**Verdict: accepted. Carried in full to S1.1c-8.** The rename is its only fix.
- `test_a_date_worked_out_with_date_1` → `test_run_id_of_a_known_instant_matches_date_utc`. "matches" is backed by `assertEqual(clock.run_id(1793631845999, "00ff00"), "20261102T150405Z-00ff00")`.
- `test_fires_once_after_its_delay` → `test_a_real_timer_fires_after_its_delay`. "fires" is backed by `wait.get(q, …) == "fired"`.

#### G2-40: test_fix_s11c4.py:95-108; test_breakit2_s11c4.py:40-41

1. **Verdict: accepted.**
   - test_fix_s11c4's `MemberUnreadable.test_entries_of_marks_the_scan_incomplete` reads `copy.zip.open`, calls `copy.cache.clear()` and assigns `copy.zip`.
   - test_breakit2_s11c4's `BreakIt2.test_bk2_3_…` assigns `copy.zip`.
   - `Copy` takes `zip=` as a field (vulns.py:58).
2. **Kind: K2e** (private seams).
3. **Change.**
   - test_fix_s11c4.py:
     ```python
         def test_entries_of_marks_the_scan_incomplete(self):
             body = zip_of(db_members(osv("GO-2099-0200", "example.com/dep", "1.0.1")))
             entries, complete = vulns.entries_of(vulns.Copy("/x", zip=zipfile.ZipFile(io.BytesIO(body))))
             self.assertEqual((len(entries), complete), (1, True))  # the witness: the same copy read whole
             real = zipfile.ZipFile(io.BytesIO(body))

             def open_(name: str):
                 if name == "ID/GO-2099-0200.json":
                     raise zlib.error("Error -3 while decompressing data: invalid code lengths set")
                 return real.open(name)

             self.assertEqual(vulns.entries_of(vulns.Copy("/x", zip=mock.Mock(open=open_))), ([], False))
     ```
     A fresh Copy is the state the old test reached by hand (an empty cache, and Members rebuilt for the new zip, vulns.py:70-71), so the outcome is the same on real code.
   - test_breakit2_s11c4.py:40-41 → `copy = vulns.Copy("/nonexistent", zip=zipfile.ZipFile(io.BytesIO(data)))`.
4. **Proof.**
   ```
   python3.15 -I -B -c '
   import ast, sys
   bad = []
   for path, names in (("gate/runner/tests/test_fix_s11c4.py", {"test_entries_of_marks_the_scan_incomplete"}),
                       ("gate/runner/tests/test_breakit2_s11c4.py", {"test_bk2_3_the_entries_read_never_hold_every_member_in_memory_at_once"})):
       tree = ast.parse(open(path, encoding="utf-8").read())
       for fn in ast.walk(tree):
           if isinstance(fn, ast.FunctionDef) and fn.name in names:
               bad += [f"{path}:{n.lineno}: .{n.attr}" for n in ast.walk(fn) if isinstance(n, ast.Attribute) and n.attr in ("cache", "zip", "members")]
   print("\n".join(bad)); sys.exit(1 if bad else 0)'
   ```
   Old: red (test_fix_s11c4 :101, :102, :108 and test_breakit2_s11c4 :41). New: exit 0.
5. **Commit:** C3.

#### G2-41: test_known_issues_fields.py:132-138

1. **Verdict: accepted. The proposed fix is wrong** (plan §0 item 1).
   - Verified: deleting `correctness` or `opened` gives only `gate/known-issues.json: entries[0]: want the keys id, first_seen, forward, decision, correctness, opened`, from knownissues.py:61-62.
   - So `assertRefused`, which forbids the generic text, would fail on real code.
   - The real weakness: `any(field in f …)` is met by any refusal that lists all the keys, and `self.setUp()` is called by hand.
2. **Kind: K2a.**
3. **Change** (one assertion, so the plant gives exactly one failure):
   ```python
       def test_a_missing_field_is_refused_naming_it(self):
           refusals = {}
           for field in ("correctness", "opened"):
               self.data = copy.deepcopy(DATA)
               del self.entry("QB10")[field]
               refusals[field] = self.check()
           keys = ["gate/known-issues.json: entries[0]: want the keys id, first_seen, forward, decision, correctness, opened"]
           self.assertEqual(refusals, {"correctness": keys, "opened": keys})
   ```
4. **Proof.**
   ```json
   {"id": "G2-41", "file": "gate/runner/knownissues.py",
    "find": "raise ValueError(f\"entries[{i}]: want the keys {', '.join(ENTRY_KEYS)}\")",
    "replace": "raise ValueError(f\"entries[{i}]: unexpected keys {', '.join(ENTRY_KEYS)}\")",
    "py": "gate.runner.tests.test_known_issues_fields.Fields.test_a_missing_field_is_refused_naming_it",
    "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   ```
   - The plant is a misleading refusal that still lists the key, so the old test stays green.
   - Control: `…Fields.test_entries_carrying_both_fields_pass`, green/green.
5. **Commit:** C3.

#### G2-42: test_main.py:114 `Cli.test_workers_reach_the_scheduler`

1. **Verdict: accepted.** With one worker, row x blocks opening f1 forever. FakeTimers never fire, so the test hangs to the 240 s faulthandler ceiling (support.py:22).
2. **Kind: K2d.**
3. **Change.** Import `from gate.runner import clock, main`.
   ```python
       def test_workers_reach_the_scheduler(self):
           # Two rows that meet through two FIFOs finish only side by side. With one worker the first row waits forever;
           # its real watchdog (30 s, a ceiling that breaks the hang, never a measure) turns that into a named timeout.
           f1, f2 = self.fifo("f1"), self.fifo("f2")
           rows = (
               self.good("x", "openw", f1, "openr", f2, timeout_s=30),
               self.good("y", "openr", f1, "openw", f2, timeout_s=30),
           )
           env = dict(support.base_env(), VORSAR_CEP_WORKERS="2")
           code = self.cli(
               "--tier", "T2", "--registry", self.reg(*rows), "--run-dir", self.run_dir,
               environ=env, timers=clock.RealTimers(),
           )
           self.assertEqual(code, 0, self.output)
   ```
4. **Proof.**
   ```json
   {"id": "G2-42", "file": "gate/runner/main.py", "find": "        workers=count,\n", "replace": "        workers=1,\n",
    "py": "gate.runner.tests.test_main.Cli.test_workers_reach_the_scheduler",
    "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "timeout", "new": "red"}}
   ```
   - Under the plant, the new test reds after about two 30 s row timeouts plus kill grace, with the summary naming `timeout after 30s`.
   - **The `--py` timeout must be between about 90 s and 240 s (180 s suggested).** Past 240 s, faulthandler exits the process and the outcome would read invalid.
   - Control: the same py, `"control": true`, green/green.
   - Load: `python3.15 -I -B -X utf8 gate/runner/stress.py --runs 50 gate.runner.tests.test_main.Cli.test_workers_reach_the_scheduler`, 50/50.
5. **Commit:** C3.

#### G2-43: test_parity_stop.py:221 `self.runner._q.put_nowait`

1. **Verdict: accepted.** (AST scan) This is the file's only private access on a non-`self` object. No public hook plants a bug in the loop.
2. **Kind: K2e** (private seam). It is fixed by the finding's first option, a comment; no production hook is added.
3. **Change.** Insert above :221:
   `# private seam: scheduler.Runner._q, the loop's queue; no public hook plants a bug in the loop, and an event it does not know makes _handle raise`
4. **Proof.**
   ```
   python3.15 -I -B -c '
   import ast, sys
   path = "gate/runner/tests/test_parity_stop.py"; src = open(path, encoding="utf-8").read(); lines = src.splitlines()
   bad = [n.lineno for n in ast.walk(ast.parse(src)) if isinstance(n, ast.Attribute) and n.attr.startswith("_") and not n.attr.startswith("__") and not (isinstance(n.value, ast.Name) and n.value.id == "self") and "# private seam:" not in lines[n.lineno - 2]]
   print(bad); sys.exit(1 if bad else 0)'
   ```
   Old: `[221]`, exit 1. New: exit 0.
5. **Commit:** C3.

#### G2-44: test_proc.py:119 `Group.test_signalling_a_finished_step_is_harmless`

1. **Verdict: accepted.** It has no assertion. "Harmless" means no signal reaches the reaped leader's pid, which the kernel may reuse (proc.py:27-33, `_pgid`).
2. **Kind: K2a.**
3. **Change:**
   ```python
       def test_signalling_a_finished_step_is_harmless(self):
           group = proc.Group()
           self.assertEqual(group.reap(self.spawn(group, "status", 0)), (0, False))
           with mock.patch.object(proc.os, "killpg") as killpg:
               group.signal(signal.SIGTERM)
           killpg.assert_not_called()  # no signal reaches the reaped leader's pid, which the kernel may reuse
           self.assertIsNone(self.spawn(group, "status", 0))  # and nothing starts after the signal
   ```
4. **Proof.** A patch entry on gate/runner/proc.py with two hunks, each exact-once:
   - hunk 1: `        return None if leader is None or leader.returncode is not None else leader.pid` → `        return None if leader is None else leader.pid`
   - hunk 2: `            if self._leader is proc:\n                self._leader = None\n` → `            pass\n`
   - py `gate.runner.tests.test_proc.Group.test_signalling_a_finished_step_is_harmless`, expect `{"old": "green", "new": "red"}`. The old test's real killpg hits a dead group, and ProcessLookupError is suppressed.
   - Control: `…Group.test_nothing_starts_after_the_group_is_signalled`, green/green.
   - Load: `stress.py --runs 50 gate.runner.tests.test_proc.Group.test_signalling_a_finished_step_is_harmless`, 50/50. It starts a process; the run is cheap, so I include it.
5. **Commit:** C3.

### 4. Commit plan (package × kind)

1. **C1, K1, gate/runner + gate/runner/rows:** G10-7, -8, -9, -10, -11, -12, -13 (7 findings).
   - Production: rows/suppressions.py, rows/vulns.py, rows/pytests.py, rows/race.py, scheduler.py, teeth.py.
   - Tests: test_suppressions.py, test_vulns_row.py, test_rows_py.py, test_rows_go.py, test_step.py, test_teeth.py.
   - Proofs: 14 `--py --patch` revert-plant runs, each red.
   - If the coordinator counts gate/runner and gate/runner/rows as two packages, split it into C1a (G10-12, -13) and C1b (the rest).
2. **C2, K1 coupling commit (the coordinator, with the lane's change):** G3-72.
   - gate/runner/suppressions.py:61-62, test_suppressions.py:616-619 plus the new test, test_breakit_s11c2.py:359, **gate/proofs/drivers/test_suppressions_planted.py:42-43**.
   - Then the proofs row alone at T4.
3. **C3, K2, gate/runner/tests (test files only):** G2-18, -20, -21, -22, -23, -26, -37, -38, -40, -41, -42, -43, -44 (13 findings).
   - Over 12 findings, so it may split by file: C3a, K2e (G2-18, -23, -26, -37, -40, -43); C3b, K2a/K2d (G2-20, -21, -22, -38, -41, -42, -44).
   - `--expect` batch: 10 entries with 6 controls, plus 6 K2e checks and 3 stress runs.
- **Commit count:** 3 (2 lane plus 1 coordinator coupling). That becomes 4–5 if C1 and C3 split. Chunk A1b's K2 commits on gate/runner/tests may merge with C3.
- **Order:** C1 → C2 → C3. Every K2 plant's find text is unchanged by C1 and C2, so each applies at both `<lane base>` and C3.
- **Batch file:** docs/sprints/s1-1c-7-plants/A1.json.

### 5. Choices made (each "chosen by the design note")

1. **G10-8, an unreadable copy file.**
   - Input: a directory, or a file the gate cannot read, at vulndb.zip, stamp.json or fetch-error.txt.
   - Candidates:
     - (a) treat it as missing, which would mislead with "no stamp.json";
     - (b) treat it as malformed, which would give a second, false "malformed" line;
     - (c) one named fault, `vulns: cannot read <path>: <strerror>: run make vulndb`, never silenced by the `vulndb` waiver (it is not one of the AVAILABILITY prefixes). An unreadable zip or stamp leaves the copy unscanned; an unreadable fetch-error.txt still lets the zip be scanned, as a malformed one does (:121-122).
   - **Chosen: (c).** It refuses rather than guesses (the origin is unknown without the stamp), and each fault is one line, as the module's docstring says.
   - Dry run: no test or driver pins any text for this case. The new prefix is not in AVAILABILITY.
2. **G10-10, a bad `--repeat=` value.**
   - Candidates: exit 2 as usage, or a `failed:` line through rows.finish.
   - **Chosen:** `failed: usage: --repeat=N with N a whole number, not 'x'`, with `GATE_COVERED 0` and exit 1. Every row reports through rows.finish, and the reason reads `exit 1: usage: …`. *(annotated 2026-11-21: after PR #53's reviews (FRT-A-2, FRT-Q-3, FRT-A2-1, FRT-Q2-1, FRT-A2-3) the text is `usage: --repeat=N with N a whole number from 1 to 999999999, not '<value>'`, N accepted only as `[1-9][0-9]{0,8}`, and any other dash argument is refused with `usage: '<arg>' is not a package pattern or --repeat=N`; the sketches above keep the A1-1 text — owner, 2026-11-21, fix both in S1.1c-7 now)*
   - Only a value int() refuses is changed. Zero and negative values stay go's business, so no untested behaviour changes.
   - Dry run: no pin (grep `--repeat=x`).
3. **G10-13, a base cycle.**
   - Candidates: a new teeth fault, or a guard only.
   - **Chosen: guard only.** Such a module raises NameError at import, so the proofs row already reds it. teeth only has to stop crashing. No new text.
4. **G3-72, the text.**
   - Candidates: drop the parentheticals, or link the decisions.md anchor (a long URL in a one-line fault).
   - **Chosen: drop.** New text: `{linter} honours this form; the only waiver is //nolint:{linter} // PERMANENT|TEMP …` and `a //line directive moves findings between files; not allowed`.
   - The dry-run list is in G3-72 above: 4 code sites, including the driver.
5. **G10-7.** Text `ruff.toml: lint.<key> is not a table of code lists`, and the both-ways comparison is skipped when the shape is wrong. Dry run: no pin.
6. **G10-9.** Text `git ls-files: exit <n>: <stderr>`, as rows/suppressions.py:80. The old SystemExit text "git ls-files failed: …" is pinned by no test; test_git.py:239 pins git.py's own different text, which is untouched.
7. **G10-12.** On failure the partial file is removed and `go env: exit <n>` is appended to go-env.stderr. Dry run: test_step.py:368-378 asserts only "go env" in go-env.stderr, so it still passes.

### 6. Couplings and risks

- **§0 item 5, planted drivers' asserted texts:**
  - Only G3-72 changes any: test_suppressions_planted.py:42-43. It is coordinator-owned (gate/proofs is A2's tree) and goes in commit C2.
  - G10-7 and G10-11: test_suppressions_planted keeps every text.
  - G10-8: test_vulns_planted :429-693 keeps every text.
  - G10-9: test_pytests_vacuity :36-40 keeps every text.
  - G10-10: test_racestress_planted :7, :31 keeps every text.
  - G10-12 and G10-13 have no driver texts.
- **The whole gate/ tree and law paths:** `gate/runner/` is a law path, so C1–C3 show in LAW Δ. After the hand-back, the proofs row runs alone at T4.
- **R5 (per-event code):** none. Every change is gate Python, with no Go per-event path. **R4:** no benchmark. **§0 item 4:** no frame table and no line-anchored mutant here.
- **GitNexus risk:** `impact(go_foreign)` is **MEDIUM**. The text-only change keeps lawdelta's Directive counts. Everything else is LOW and exact.
- **Seams:**
  - C2 edits test_suppressions.py and test_breakit_s11c2.py. C1 adds tests to test_suppressions.py.
  - Chunk A1b may also edit test_suppressions.py. It is the same lane, so one build agent orders the edits; this is named, not a conflict.
  - No other owner's file except the driver in C2.
- **Open for the coordinator and W0:**
  - (a) how `--py` counts red for a test whose subtests all fail. My tests avoid the question.
  - (b) `--py` timeouts: about 180 s for G2-42; more than 30 s for G2-22; G2-20 builds apidiff with go.
- **Pre-mortem (the-fool), riskiest fixes:**
  - G2-42 could fail falsely under load if two row processes take more than 30 s to start. That is unlikely, but the 50/50 stress run is the check.
  - G10-8's stamp→unscanned choice could hide integrity faults until `make vulndb` runs. Accepted, because the row is red either way.
  - In G2-22, `threading.enumerate()` diffing could pick up a stray thread. The `len(started) == 1` witness catches that.

### 7. Carried items

- **To S1.1c-8 (renames):**
  - G2-21: `test_a_top_level_per_file_ignores_is_pinned` → `…_is_red`, and `test_a_waiver_in_an_extended_file_is_pinned` → `…_is_red`.
  - G2-39, in full: `test_a_date_worked_out_with_date_1` → `test_run_id_of_a_known_instant_matches_date_utc`, and `test_fires_once_after_its_delay` → `test_a_real_timer_fires_after_its_delay`.
- **To the later part (duplication and placement), no ledger carry:**
  - G2-21's overlap with `test_suppressions.RuffPinRefuses`;
  - G2-37's optional fold into test_law.DocLine;
  - the duplicated expression noted under G2-19.
- **Findings not reached:** none.
