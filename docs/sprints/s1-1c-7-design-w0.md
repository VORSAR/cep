# S1.1c-7 design note — W0 (the proof tools and the naming tool)

*S1.1c-7's design note for chunk W0 (the proof tools and the naming tool), written by a fresh `refactoring-specialist` with GitNexus from [the plan check](s1-1c-7-plan-check.md) §5 and the survey's findings; reviewed blind (architecture with the-fool, QA twice) and pre-checked against the code ([the reviews](../reviews/s1-1c-7-design-architecture.md), [QA](../reviews/s1-1c-7-design-qa.md)); every cross-note ruling and review disposition is in [the coordination record](s1-1c-7-design-coordination.md). **Later parts supersede earlier ones:** each revision replaces the sections it names, and the coordinator's corrections at the end of a part override that part. Escapes written ⧵ stand for one ASCII backslash (agent output passes through JSON, which decodes a single backslash-u); the builder types a backslash and checks the file's bytes. Part A's `GO_SPACES` literal arrived mangled: implement it from Go's `unicode.IsSpace` (coordination item 37).*


## The note

## S1.1c-7 design note — chunk W0 (W0-py and W0-go), plus the tool contracts

### Header

- **Chunk:** W0. 14 findings: G10-2, G10-3, G10-4, G2-6, G2-7 (W0-py); G2-147 to G2-155 (W0-go).
- **Base:** the branch is now at **37df879**, not d521725. The two commits since then (a2aeb02 and 37df879) touch `gate/runner/tests/test_aging.py`, `test_instruments_pin.py` and `docs/` only. None of this chunk's files changed, so the index (fresh at d521725) holds for them.
- **Files read in full:**
  - `gate/bench/mutate.py`, `load.py`, `gorun.py`, `tests/test_mutate.py`, `tests/support.py`, `tests/test_load.py`;
  - `gate/runner/pyshim.py`, `rows/__init__.py`, `rows/pytests.py`;
  - plan check §0–§12;
  - the prototype `docs/sprints/s1-1c-7-plants/nameproto.py` (committed at 37df879; it differs from the scratchpad copy only in its docstring, its CLI and its `decided()` filter) and its outputs (`naming-prototype.json`: 426 decided hits; the scratchpad output: 429 at 5b4ddac).
- **Files read in part:** `gate/bench/compare.py`, `escapes.py`, `tests/test_compare.py`, `tests/test_escapes.py`; `tools/archcheck/archcheck_test.go` and `main.go`; `tools/astequal/astequal_test.go`, `main.go` and `canon.go`; `tools/apipartition/main_test.go`; `ruff.toml`; the registry rows; `gate/law.json`; `testing.md`.
- **GitNexus calls (7):** impact on `escapes.check`; context on `compare._run`; context on `mutate.judge`; impact on `gitIn`; impact on `archcheck_test.check`; context on `LoadLists`; context on `mutate.read_batch`.
- **What I could not check (rules: read-only, no runs):**
  - The exact `-gcflags=-m=2` text for G10-2's new fixture. Only lines the existing tests already rely on are asserted.
  - Go 1.27's test2json behaviour on a timeout panic. G2-7's plant is chosen so the result does not depend on it.
  - No plant or test was run. Every "red/green" below is a prediction the build must confirm.
- `detect_changes` does not apply: no file was changed.

### Summary table

| Id | Verdict | Kind | Commit | Carried | One-line change |
|---|---|---|---|---|---|
| G10-2 | accepted | K1 | W0-4 | – | The before side counts the helpers that existed before and were not inlined. |
| G10-3 | accepted | K1 | W0-4 | – | `--help` on `mutate.py` and `load.py` prints the usage and exits 0 (plan §6 #19). |
| G10-4 | accepted | K1 | W0-4 | – | The duplicate-benchmark error names only the repeated names. |
| G2-6 | accepted (the proposed exact value rejected) | K2a | W0-5 | – | Assert the printed `p ≥ 0.3333`, not `math.comb`. |
| G2-7 | accepted | K2a | W0-5 | – | Assert TestSlow's `0/1` line, the panic line and the exact verdict line. |
| G2-147 | accepted (the proposed runCLI swap rejected) | K2e | W0-7 | – | `exec.CommandContext` with a 5-minute ceiling, and a named failure. |
| G2-148 | accepted | – | – | **carried in full to S1.1c-8** | Rename only. |
| G2-149 | accepted | K2a | W0-7 | – | Normal-mode cases (with a finding, clean), plus a verdict witness per case. |
| G2-150 | accepted | K2a | W0-7 | – | Assert one "channel type" finding and one "channel send" finding. |
| G2-151 | accepted | K2a + K2e | W0-7 | – | Compare the whole purity map; the live-data pin replaced by "loads". |
| G2-152 | accepted | K2e | W0-7 | **fixed in part**: the rename goes to S1.1c-8 | Drop the unread fixture; the witness asserts the missing-file fault. |
| G2-153 | accepted | K2e | W0-7 | – | `check` fails on an odd `other` count. |
| G2-154 | accepted | K2a | W0-8 | – | Error rows assert empty stdout and a stderr fragment; success rows assert empty stderr. |
| G2-155 | accepted | K2e | W0-8 | – | `gitIn` uses `exec.CommandContext` with a 2-minute ceiling. |

Carried in full: 1 (G2-148). Fixed in part: 1 (G2-152).

---

### Part A — `gate/bench/mutate.py`'s contract (plan §2), build-ready

#### A.1 Data and constants

These replace `ENTRY_KEYS` (`mutate.py:41`) and `REQUIRED_KEYS` (`:42`).

```python
OUTCOMES = ("red", "green", "timeout", "invalid")           # unchanged
PYTHON = "python3.15"                                        # the pytests row's interpreter (rows/pytests.py PYTHON)
MUTATION_KEYS = frozenset({"file", "find", "replace", "patch"})
TARGET_KEYS = frozenset({"test", "subtest", "py"})
SIDE_KEYS = frozenset(f"{s}_{k}" for s in ("old", "new") for k in TARGET_KEYS)
PLAIN_KEYS = MUTATION_KEYS | TARGET_KEYS | {"id", "note", "pkg"}
EXPECT_KEYS = PLAIN_KEYS | SIDE_KEYS | {"old_rev", "new_rev", "expect", "control"}
FULL_REV = re.compile(r"[0-9a-f]{40}")
GO_META = frozenset("\\.+*?()|[]{}^$")                       # regexp.QuoteMeta's set
GO_SPACES = frozenset("\t\n\v\f\r \u0085      　") | frozenset(map(chr, range(0x2000, 0x200B)))
THROWAWAY = {"GIT_AUTHOR_NAME": "mutate.py", "GIT_AUTHOR_EMAIL": "mutate@invalid",
             "GIT_COMMITTER_NAME": "mutate.py", "GIT_COMMITTER_EMAIL": "mutate@invalid"}

@dataclasses.dataclass(frozen=True)
class Target:
    test: str = ""                  # Go top-level Test ("" in the Python mode)
    levels: tuple[str, ...] = ()    # Go subtest path, each level rewritten as go test names it
    py: str = ""                    # unittest id ("" in the Go mode)
    @property
    def full(self) -> str: return "/".join((self.test, *self.levels))

@dataclasses.dataclass(frozen=True)
class Change:
    file: str
    original: str
    mutant: str

@dataclasses.dataclass(frozen=True)
class Entry:                        # replaces Mutation (used only inside mutate.py; GitNexus: read_batch ← mutate only)
    id: str
    changes: tuple[Change, ...]     # () only for a control entry
    old: Target                     # plain mode: old == new
    new: Target
    pkg: str                        # Go package dir; "" for Python
    note: str
    old_rev: str = ""
    new_rev: str = ""
    expect: tuple[str, str] = ("", "")
    control: bool = False
```

#### A.2 Functions (signatures, and the code each plant refers to)

- `go_rewrite(level: str) -> str`: `return "".join("_" if c in GO_SPACES else c for c in level)`.
- `quote_meta(text: str) -> str`: `return "".join("\\" + c if c in GO_META else c for c in text)`.
- `target(test: str | None, subtest: str | None, py: str | None) -> Target`. It refuses (exit 2):
  - both a Go and a Python designation, or neither;
  - `subtest` without `test`;
  - a `test` that is not `isidentifier()` and `startswith("Test")` (the existing rule, `mutate.py:93`);
  - a `py` id with fewer than two dotted parts, or any part that is not `isidentifier()`;
  - a `subtest` with an empty level (`split("/")` gives `""`);
  - a level holding a character that is neither a Go space nor `str.isprintable()`. Go would escape it with `strconv.QuoteRune`; it is refused rather than emulated. After the Go spaces are mapped, Python's non-printable set is exactly Go's (Cc, Cf, Cs, Co, Cn).

  It returns `Target(test, tuple(go_rewrite(l) for l in subtest.split("/")))`.
- `pattern(t: Target, probe: bool) -> str`:
  ```python
  parts = [f"^{t.test}$"]
  for level in t.levels:
      parts.append(f"^{quote_meta(level)}{'(#[0-9]+)?' if probe else ''}$")
  return "/".join(parts)
  ```
- `runs(text: str, t: Target) -> list[str]`: the names on the `=== RUN` lines at exactly the designated depth:
  ```python
  rx = r"^=== RUN\s+(" + re.escape(t.test) + "".join("/" + re.escape(l) + r"(?:#[0-9]+)?" for l in t.levels) + r")$"
  return re.findall(rx, text, re.MULTILINE)
  ```
- `judge_go(result: gorun.Result, t: Target) -> str`:
  ```python
  text = f"{result.stdout}\n{result.stderr}"
  if "panic: test timed out after" in text:
      return "timeout"
  if not t.levels:                      # today's top-level judging, unchanged (mutate.py:162-166)
      ...
  name = re.escape(t.full)
  if not re.search(rf"^=== RUN\s+{name}$", text, re.MULTILINE):
      return "invalid"
  if re.search(rf"^\s*--- SKIP: {name} \(", text, re.MULTILINE):
      return "invalid"
  if re.search(rf"^\s*--- FAIL: {name} \(", text, re.MULTILINE):
      return "red"
  if result.exit == 0 and not result.timed_out and re.search(rf"^\s*--- PASS: {name} \(", text, re.MULTILINE) \
          and not re.search(r"^\s*--- FAIL: ", text, re.MULTILINE):
      return "green"
  return "invalid"
  ```
  A parent FAIL without the subtest's own FAIL line falls through to `invalid`.
- `run_go(e: Entry, side: Target, args, scratch: str, cwd: str | None) -> tuple[str, gorun.Result, list[str]]`:
  - With subtest levels, it first runs the **probe** (the same argv with `-run pattern(t, probe=True)`). `n = len(runs(probe_text, t))`. If `n != 1`, the outcome is `invalid`, with the line `probe: <n> subtests run as <full>`.
  - The real run uses `-run pattern(t, probe=False)`.
  - It prints `ran: <name>` for each name `runs()` finds on the real run.
  - `cwd is None` is the plain and single mode: today's overlay over the real tree (`mutate.py:180-201`), one overlay entry per Change.
  - `cwd` set is the `--expect` mode: `go test` runs from the worktree, with no overlay (the plant is committed there; see `worktree`).
- `judge_py(result: gorun.Result, said: str, t: Target) -> str`:
  ```python
  if result.timed_out:
      return "timeout"
  head = f"pyshim: {t.py}: "
  if result.exit == 1 and said == head + "failed: 1 failures, 0 errors" \
          and re.search(r"^Ran 1 test in ", result.stderr, re.MULTILINE):
      return "red"
  if result.exit == 0 and said == head + "passed 1 tests":
      return "green"
  return "invalid"
  ```
  - `said` is the last line of the report file that starts with `head`, or `""`.
  - A failure beside a leak, a skip or any other problem is not the sole problem, so it is invalid (PA3-S2).
  - An import error, `ran 0`, a skip, an expected failure or an unexpected success is invalid.
  - `Ran 1 test in` (unittest's stderr at verbosity 2) makes a class id that runs several tests invalid even when exactly one of them fails.
- `worktree(rev: str, changes: tuple[Change, ...]) -> contextlib.AbstractContextManager[str]`:
  1. `before = git worktree list --porcelain`.
  2. `tmp = tempfile.mkdtemp(prefix="mutate-wt-")`, outside the repo: `pytests`' `misplaced()` walks `gate/` relative to the top level, untracked files included (`rows/pytests.py:46-63`).
  3. `path = <tmp>/wt`, then `_git("-c", "core.hooksPath=/dev/null", "worktree", "add", "--detach", path, rev)`.
  4. Write each change's mutant in the worktree.
  5. If there are changes: `git -C path -c core.hooksPath=/dev/null -c commit.gpgsign=false add -- <files>`, then `... commit -q -m "mutate.py plant"` under `env={**os.environ, **THROWAWAY}`.
  6. `yield path`.
  7. In `finally`: `git worktree remove --force path`. If that fails: `shutil.rmtree` and `git worktree prune`. Then `shutil.rmtree(tmp)`.
  8. If `git worktree list --porcelain` is not equal to `before`, raise `Internal("a worktree was left: …")`, exit 2.
  9. A SIGTERM (`term.install`) raises inside the `try`, so the `finally` runs.
- `run_py(e, side, args, rev) -> tuple[str, gorun.Result, str]`:
  - Inside `worktree(rev, e.changes)`.
  - The report file is `<a separate mkdtemp>/report`, never inside the worktree: an untracked file there would make `git status` dirty (#13a).
  - argv `[PYTHON, *rows.SHIM_FLAGS, os.path.join(wt, "gate", "runner", "pyshim.py"), wt, side.py, report]`. `rows.SHIM_FLAGS` comes from `gate.runner.rows`, so the flags cannot drift from the row's (`rows/__init__.py:24`).
  - `cwd=wt`, `env={**os.environ, "PWD": wt}`, `gorun.run(..., timeout_s=seconds(args.timeout))`.
- `parse_patch(text: str) -> list[tuple[str, list[tuple[str, str]]]]`: unified diff to `[(path, [(old_block, new_block), …])]`.
  - It refuses `--- /dev/null`, `+++ /dev/null`, a rename, `\ No newline at end of file`, a hunk with no `-` or `+` line, and a path that is absolute or holds `..`.
  - Hunk headers' line numbers are ignored.
- `apply(path: str, original: str, hunks) -> str`: applies the hunks in order on the progressively mutated text:
  ```python
  text = original
  for old, new in hunks:
      count = _occurrences(text, old)
      if count != 1:
          raise Refused(f"a hunk of {path} occurs {count} times, not exactly once")
      text = text.replace(old, new, 1)
  return text
  ```
- `base_text(path: str, rev: str | None) -> str`:
  - With `rev` None: the existing checks (tracked, `git diff --quiet HEAD -- path`, `mutate.py:105-108`), then it reads the file.
  - With `rev` set: `git cat-file -e <rev>:<path>`, then `git show <rev>:<path>`.
- `changes_of(entry: dict, rev: str | None) -> tuple[Change, ...]`: from `file`/`find`/`replace` (exact-once, as at `mutate.py:111-113`) or from `patch`.
- `read_batch(path: str, expect: bool) -> list[Entry]`. Every entry is checked before the first build or worktree.
  - **Allowed keys:** `EXPECT_KEYS` with `--expect`, `PLAIN_KEYS` otherwise. In a plain batch, an `--expect` key is refused as "an --expect key in a plain batch".
  - **Value types:**
    - `expect` must be a dict with exactly `old` and `new`, each in `OUTCOMES`;
    - `control` must be the literal `true`;
    - every other value must be a string.
  - **Required keys per mode (replaces `REQUIRED_KEYS`):**
    - Mutation: exactly one of {all of `file`, `find`, `replace`} or {`patch`}. A partial triple, or both forms, is refused. With `--expect` and `"control": true`, no mutation key is allowed.
    - Designation per side: `<side>_<key>` if present, else `<key>`. Each side resolves through `target()`. An entry is all Go or all Python. `pkg` is allowed with Go only.
    - `--expect` only: `id` (unique), `old_rev`, `new_rev` and `expect` are required. Each rev must match `FULL_REV` and `git rev-parse --verify --quiet <rev>^{commit}` must print the same id. The mutation is checked exact-once against each rev's blob.
    - Plain: `id` is optional; the old target equals the new.
  - `--race` and `--cpu` are refused with any Python entry.
- `expect(entries, args) -> tuple[int, list[str]]`:
  - For each entry, for each side: a fresh `worktree(rev, changes)`. Go runs `run_go(cwd=wt)`; Python runs the Python path.
  - Prints `entry <id> old <rev[:12]>: <outcome> (expected <word>)`, then the same line for new.
  - `differing = [e.id for e in entries if got[e.id] != e.expect]`.
  - Last line: `expect: <n> entries, <k> as expected; differing: <ids or none>`. Exit 0 if `differing` is empty, else 1.
- **CLI (`_parse_args`):**
  - single: (`--file --find --replace` | `--patch <diff file>`) + (`--test [--subtest]` | `--py`), with `[--pkg] [--timeout] [--race] [--cpu] [--repeat N --load]`;
  - `--batch <json>`;
  - `--expect <json>`.

  `--batch` and `--expect` exclude each other and exclude every single-mode flag. `--py` excludes `--subtest`, `--pkg`, `--race` and `--cpu`. `--repeat --load` works with `--py` and with `--subtest`.
- **Output:**
  - `record()` adds `python: <python3.15 --version>` for Python runs.
  - Single mode prints `subtest: <full>` (Go) or `copy: detached worktree at <head>` (Python).
  - An invalid outcome prints the tail and, for Python, the shim's report line.
  - The docstring is rewritten to this contract.
- **G10-3's parser change** (W0-4, both files):
  ```python
  class Helped(Exception):
      """--help printed the usage: exit 0."""

  class _Parser(argparse.ArgumentParser):
      def error(self, message: str) -> NoReturn:
          raise Refused(f"usage: {message}")            # load.py: Error

      def exit(self, status: int = 0, message: str | None = None) -> NoReturn:
          """Reached only from --help, after the usage is printed (error is overridden above)."""
          raise Helped
  ```
  In `main`, before `except Refused` (load.py: `except Error`), add `except Helped:` with `return 0`. ruff selects E, W, F, I, B, UP, SIM, TID, BLE and S60x, with no flake8-builtins (`ruff.toml`), so overriding `exit` raises no finding.

#### A.3 Counter-examples §6 #1–19 as named tests (`gate/bench/tests/test_mutate.py`)

**The Go fixture gains files:**
- `m/n.go`: `package m` with
  - `func Mul(a, b int) int {\n\treturn a * b\n}`
  - `func Neg(a int) int {\n\treturn -a\n}`
  - `func Twice(a int) int {\n\treturn a + a\n}`
  - `func Double(a int) int {\n\treturn a + a\n}`
- Tests in `m/m_test.go`:
  - `TestNeg`: `Neg(2) != -2` is a Fatal.
  - `TestTable`: rows `{"F3",2,3,5}`, `{"F30",0,0,0}`, `{"E-SYN-001/not a participle error",1,2,3}`, `{"x+y (meta)",0,0,0}`, `{"dup",0,0,0}`, `{"dup",0,0,0}`; each `t.Run(c.name, …)` Fatals when `Add(c.a,c.b) != c.want`.
  - `TestSetup`: `if Add(1, 1) != 2 { t.Fatal(...) }`, then `t.Run("after setup", func(t *testing.T) {})`.
  - `TestSkips`: `t.Run("skipped", …)` calls `t.Skip` when `Add(2, 3) == 5`.

  The existing `FIND`/`CAUGHT` stay. `m/m.go` is unchanged, so "occurs 2 times" (`test_mutate.py:245`) holds.
- `Base.snapshot` covers `m/m.go` and `m/n.go`.

**The Python fixture (`class PyMode`, its own temp repo)** holds:
- `gate/runner/pyshim.py`, copied from `support.REPO` (so a run in a worktree copies the copy's shim);
- `pkg/__init__.py`, `pkg/calc.py` (`def add(a, b):\n    return a + b\n`);
- `pkg/tests/__init__.py`, `pkg/tests/test_calc.py` with:
  - `Calc.test_add` (add(2,3)==5);
  - `test_weak` (add(0,0)==0);
  - `test_skips` (`self.skipTest`);
  - `test_lists_tracked_files` (`git ls-files` with `timeout=60` lists `pkg/calc.py`);
  - `test_tree_is_clean` (`git diff --quiet HEAD` with `check=True, timeout=60`, and `git status --porcelain` == "");
  - `test_waits_forever` (`threading.Event().wait()`);
  - `test_fails_and_leaks` (`open(__file__)`, then `assertEqual(calc.add(2, 3), 5)`);
  - `class Empty(unittest.TestCase): pass`.

Every PyMode test registers a cleanup asserting two things: every tracked file of the fixture is byte-identical (bytes and mtime), and `git worktree list --porcelain` equals its value before the run (#16).

| # | Test (class.method) | Asserts | §7.7 feature-off plant on the copy of HEAD's `gate/bench/mutate.py` (find → replace) | Expected red at |
|---|---|---|---|---|
| 1 | `Subtests.test_a_subtest_failing_against_the_plant_is_red_while_its_sibling_passes` | `--subtest F3`: exit 0, `subtest: TestTable/F3`, `ran: TestTable/F3` only, `outcome: red` | `rf"^\s*--- FAIL: {name} \("` → `rf"^--- FAIL: {name} \("` | `assertEqual(code, 0)` (invalid gives 1) |
| 2 | `Subtests.test_a_subtest_passing_while_its_sibling_fails_is_green` | `--subtest F30`: `outcome: green` | `rf"^\s*--- PASS: {name} \("` → `rf"^--- PASS: {name} \("` | the last line |
| 3 | `Subtests.test_a_subtest_that_does_not_exist_is_invalid` | `--subtest NoSuch`: `probe: 0 subtests run as TestTable/NoSuch`, `outcome: invalid` | `    if not t.levels:` (in judge_go) → `    if True:`, and `        if n != 1:` → `        if n > 1:` | the last line (top-level green) |
| 4 | `Subtests.test_a_name_with_a_slash_spaces_or_metacharacters_runs_exactly_itself` | `"E-SYN-001/not a participle error"`: red, `ran:` == [`TestTable/E-SYN-001/not_a_participle_error`]; `"x+y (meta)"`: green, `ran:` == [`TestTable/x+y_(meta)`] | `return "".join("\\" + c if c in GO_META else c for c in text)` → `return text` | the meta case's `ran:` (no RUN gives invalid) |
| 5 | `Subtests.test_f3_never_runs_its_sibling_f30` | `--subtest F3`: `ran:` lines == exactly `TestTable/F3` | `f"^{quote_meta(level)}{'(#[0-9]+)?' if probe else ''}$"` → the same without the final `$` | the `ran:` list (F30 also runs) |
| 6 | `Subtests.test_two_subtests_rewritten_to_one_name_are_invalid` | `--subtest dup`: `probe: 2 subtests run as TestTable/dup`, invalid | `        if n != 1:` → `        if n == 0:` | the last line (green) |
| 7 | `Subtests.test_a_parent_failing_before_t_run_is_invalid_never_red` | `--test TestSetup --subtest "after setup"`: invalid, exit 1 | `    if not t.levels:` → `    if True:` | `assertEqual(code, 1)` (red gives 0) |
| 8 | `Subtests.test_a_skipped_subtest_is_invalid` | `--test TestSkips --subtest skipped --replace "return b + a"`: invalid | same plant as #7 | the last line (green) |
| 9 | `Patch.test_a_two_hunk_patch_applies_each_hunk_once` | patch on `m/n.go` with hunks `-\treturn a * b`/`+\treturn a / b` and `-\treturn -a`/`+\treturn a`, `--test TestNeg`: red; both hunks in the printed diff | `    for old, new in hunks:` → `    for old, new in hunks[:1]:` | the last line (TestNeg green) |
| 10 | `Patch.test_a_hunk_matching_twice_or_never_or_a_dirty_file_is_refused_before_any_build` | three subTests under the fake go: hunk `-\treturn a + a` (twice), hunk `-\treturn a % b` (never), dirty `m/n.go`; each exit 2, `error: refused:`, no go log | `if count != 1:` → `if count > 1:` (the never case; the twice case uses `count == 0`) | `assertEqual(code, 2)` |
| 11 | `PyMode.test_a_caught_python_mutation_is_red_and_a_missed_one_green` | `--py pkg.tests.test_calc.Calc.test_add` with `return a - b`: red; `...test_weak`: green | `said == head + "failed: 1 failures, 0 errors"` → `said == head + "failed: 0 failures, 1 errors"` | the red subTest's last line |
| 12 | `PyMode.test_a_broken_import_a_skip_or_zero_tests_is_invalid` | `return a +` with test_add: invalid; test_skips (with `return b + a`): invalid; `pkg.tests.test_calc.Empty`: invalid; test_fails_and_leaks with `return a - b`: invalid | `    if result.exit == 1 and said == head + "failed: 1 failures, 0 errors" \` → `    if result.exit == 1 or said == head + "failed: 1 failures, 0 errors" \` | each subTest's last line (red) |
| 13 | `PyMode.test_a_test_running_git_ls_files_reads_as_in_the_tree` | `test_lists_tracked_files` with `return b + a`: green, and the shim run directly in the fixture repo passes | after the `worktree add` call, insert `os.remove(os.path.join(path, ".git"))` (find the add line, replace it with itself plus that line) | the last line |
| 13a | `PyMode.test_a_plant_elsewhere_leaves_git_diff_and_status_clean` | `test_tree_is_clean` with `return b + a`: green | the commit call line (`..."commit", "-q", "-m", "mutate.py plant"...`) → `pass` | the last line (invalid) |
| 14 | `PyMode.test_a_test_waiting_past_the_timeout_is_timeout` | `test_waits_forever`, `--timeout 5s`: timeout | `    if result.timed_out:\n        return "timeout"` (judge_py) → `    if False:\n        return "timeout"` | the last line |
| 15 | `PyMode.test_a_python_patch_is_applied_in_the_copy` | `--py …test_add --patch` (calc `+ b` → `- b`): red, `pkg/calc.py` untouched | the loop writing the mutants in `worktree` → over `changes[:0]` | the last line (green) |
| 16 | `PyMode.test_a_py_run_leaves_the_tree_and_the_worktree_list_as_they_were` | after #11's run: tracked bytes and mtimes equal, the worktree list equal | `git worktree remove --force` call → `pass` (the self-check then exits 2) | `assertEqual(code, 0)` |
| 17 | `Expect.test_an_entry_off_its_expectation_exits_1_naming_it_and_the_control_reads_the_real_code` | fixture rev2 strengthens TestWeak (`Add(2,3)!=5`). Entries: A (CAUGHT, TestWeak, expect green/red), B (CAUGHT, TestWeak, expect red/red), C (control, TestWeak, green/green), D (control, expect red/green), E (py with `old_py`/`new_py`). Result: exit 1, `entry C old …: green (expected green)`, last line `… differing: B, D` | `    differing = [e.id for e in entries if got[e.id] != e.expect]` → `    differing = []` | `assertEqual(code, 1)` |
| 17a | `Expect.test_an_entry_without_a_revision_or_naming_an_absent_one_is_refused` | missing `old_rev`; 40 zeros; a 12-hex prefix; each exit 2, `error: refused:`, no go log | the rev-check line → `if False:` | the `error: refused:` prefix (an internal error instead) |
| 18 | `Refusals.test_an_unknown_key_or_an_expect_key_in_a_plain_batch_is_refused` (the existing `test_a_batch_entry_with_an_extra_key_is_refused_before_any_build` stays) | `{"bogus"}` in `--expect`; `old_rev` in `--batch`; find with patch; test with py; control with find: each exit 2 | the allowed-keys check `keys - allowed` → `set()` | `assertEqual(code, 2)` |
| 19 | `Refusals.test_help_prints_the_usage_and_exits_0_before_any_go_command`; `test_load.Fake.test_help_prints_the_usage_and_exits_0_before_any_go_command` | exit 0, output starts `usage: gate/bench/mutate.py ` (or `load.py `), no `error:`, no go log | delete the `def exit…raise Helped` block (G10-3's K1 plant) | `assertEqual(code, 0)` |

**About the plants:**
- Each plant runs as `python3.15 -I -B gate/bench/mutate.py --py <id> --file gate/bench/mutate.py --find <f> --replace <r>`, from the commit that adds the feature. The running tool has `--py` from W0-1 on, and the copy is the one planted (§7.7).
- The plant texts are exact for the code given above. If the builder's code differs, the plant keeps the same intent, and the record quotes the actual find and replace.
- **Bootstrap witness for `--py` itself** (W0-1): before relying on `--py`, run #11's plant by hand.
  1. `git worktree add --detach $T/wt <W0-1>`
  2. Apply the replace in `$T/wt/gate/bench/mutate.py` with the Edit tool, not a shell editor.
  3. `python3.15 -I -B -X dev -W error $T/wt/gate/runner/pyshim.py $T/wt gate.bench.tests.test_mutate.PyMode.test_a_caught_python_mutation_is_red_and_a_missed_one_green $T/report`
  4. The report should say `failed: 1 failures, 0 errors`.
  5. `git worktree remove --force $T/wt`

---

### Part B — the naming tool (§3), build-ready

**Files** (W0-6). All are new and owned by W0-py (§4).

`gate/runner/testnames.py`: standard library only, no `gate` import, so it stays a pure leaf.
- **Data:** `Rule(id: str, rx: re.Pattern)`; `Name(path: str, kind: str, name: str, line: int)` with kinds `file`, `func`, `method`, `type`, `var`, `const`, `class`; `Hit(name: Name, rules: tuple[str, ...])`.
- `load_rules(path: str) -> tuple[Rule, ...]`: keys exactly `{"schema", "note", "rules"}`, `schema == 1`, each rule exactly `{"id", "pattern"}`, unique ids, each pattern compiles. Otherwise `ValueError` naming the file.
- `tokens(name: str) -> str`: the prototype's normalisation:
  ```python
  _SPLIT = re.compile(r"[_\-.]+")
  _CASE = re.compile(r"(?<=[a-z])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])|(?<=[0-9])(?=[A-Z])")
  return "_".join(p.lower() for part in _SPLIT.split(name) for p in _CASE.split(part) if p)
  ```
- `match(rules, name: str) -> tuple[str, ...]`: sorted ids of the rules where `re.search(rf"(?:^|_)(?:{r.rx.pattern})(?:_|$)", tokens(name))` matches.
- `in_scope(path: str) -> bool`: true for `*_test.go` or a basename matching `test_*.py`, with no `testdata` path component.
- `go_names(path: str, text: str) -> list[Name]`:
  1. The file name without `_test.go` (kind `file`, line 1).
  2. Blank comments, strings, runes and raw strings, keeping newlines (an own `_blank`, the algorithm of `gate/bench/escapes.py:_blank`, copied, not imported).
  3. Walk the lines with bracket depth `([{`. At depth 0, a line at column 0 starting `func `: a `(` next means skip the balanced receiver, then an identifier gives kind `method`; else an identifier gives kind `func`.
  4. `type `, `var `, `const ` followed by `(` open a group. Inside it, each line starting at relative depth 1 with an identifier starts a spec. Otherwise it is one spec on that line.
  5. A spec yields, for `type`, its first identifier (generic `[T any]` allowed); for `var`/`const`, the identifier list `a, b, c` up to the first token that is not `ident ,`.
  6. `_` is skipped. Lines inside a nested struct are at depth 2 and are skipped.
- `py_names(path: str, text: str) -> list[Name]`:
  - `ast.parse`; a `SyntaxError` raises `Fault(path, …)`.
  - The module stem (kind `file`).
  - Top-level `ClassDef` (kind `class`) and its direct `FunctionDef`/`AsyncFunctionDef` (kind `method`).
  - Top-level functions (`func`).
  - Top-level `Assign` targets and `AnnAssign` targets that are `Name`, `Tuple` or `List` of `Name` (kind `var`).
- `names(path, text) -> list[Name]` dispatches on the suffix. `judge(rules, names) -> list[Hit]` keeps names with at least one rule.
- `main(argv: list[str]) -> int`: `python3.15 -I -B gate/runner/testnames.py [--json <out>] [<path>...]`.
  - With no paths: `git ls-files -z -- '*_test.go' '*.py'` from the working directory (`subprocess.run(..., timeout=60)`), filtered by `in_scope`.
  - The word list is `os.path.join(<repo of __file__>, "gate", "testnames.json")`.
  - Prints `"{path}:{line}: {kind} {name}: {', '.join(rules)}"`, sorted, then `testnames: <F> files, <N> names; <H> hits in <G> files (report only)`.
  - `--json` writes `[{"path","line","kind","name","rules"}]`.
  - Exit 0 with or without hits (report only; S1.1c-8 adds the failing exit). Exit 2 on a fault (git failure, unreadable file, `SyntaxError`, a bad word list), with `error: …` as the last line.

`gate/testnames.json`:
```json
{"schema": 1, "note": "The naming rule's word list (plan check §3; owner, 2026-11-20, decision 5): each pattern matches a whole run of normalised tokens. Read by gate/runner/testnames.py only.",
 "rules": [
  {"id": "sprint id", "pattern": "s\\d+_\\d+[a-z]?(?:_\\d+)?|s\\d+c\\d+(?:_\\d+)?|s\\d+_\\d+_\\d+"},
  {"id": "review step", "pattern": "break_?its?\\d*|fix_?rounds?\\d*|fix_s\\d+\\S*"},
  {"id": "lane or wave", "pattern": "lane_[a-z]\\d?|wave_?\\d+"},
  {"id": "review id with a digit", "pattern": "(?:qb|qa|bk|pa|pq|cv|cvi|cg|ar|br)\\d+[a-z]?"},
  {"id": "review-role or sprint-shorthand prefix", "pattern": "bk|qa|qb|pq|pa|cv|cvi|cn|q2|pb"}]}
```
The patterns are the prototype's decided ones, character for character (`nameproto.py` CORE plus the two decided WIDE rules), so recall cannot drop.

`gate/runner/testdata/testnames/golden.json` holds `{"schema":1,"note":…,"names":[…],"sources":[…],"scope":[…]}`. Every fixture source is an **inline string**: a `test_*.py` fixture file under `gate/` would trip `pytests`' `misplaced()` ("no row runs it"), and a `.go` fixture would join `gofmt`/`testhygiene`. Rows:

- **names** (`{"row", "name", "rules"}`):
  - 20: `breakit_s3_3_1` → `["review step","sprint id"]`
  - 21: `TestBreakitDecoderShared` → `["review step"]`; `BreakIt` → `["review step"]`
  - 22: `LaneA`, `lane_d`, `wave2` → `["lane or wave"]`
  - 23: `TestTwoLanesMerge`, `TestLaneAware`, `planet`, `test_fixture_paths` → `[]`
  - 24: `test_qb3_reads_once`, `test_bk2_1_holds` → `["review id with a digit"]`
  - 25: `TestCvOptionsRefuseNil`, `bkMsg`, `pqSnapshot`, `pbReplay`, `cnFailWriter` → `["review-role or sprint-shorthand prefix"]`
  - 26: `pbx`, `cvs`, `TestPbufRoundTrip`, `qaz` → `[]`
  - 29: `test_s1_1c_10_does_not_match_s1_1c_1` → `["sprint id"]`
  - 30 (known greens): `cgApp`, `ldParked`, `hpApp`, `test_the_marker_at_todays_row_passes` → `[]`
  - 31: `BenchmarkSendPath` → `[]`
- **sources** (`{"row","path","text","names":[[kind,name],…],"hits":[[kind,name,[rules]],…]}`):
  - 27: `internal/x/group_test.go`, text `"package x\n\nconst (\n\tbkPhaseEvent = iota\n\tplainPhase\n)\n\nvar pqSec, pqMin = 1, 2\n\ntype (\n\tholder struct {\n\t\tbkInner int\n\t}\n)\n"`. Names: file `group`, const `bkPhaseEvent`, const `plainPhase`, var `pqSec`, var `pqMin`, type `holder`. `bkInner` is never extracted (a field). Hits: `bkPhaseEvent`, `pqSec`, `pqMin`.
  - 28: `internal/x/sink_test.go`, text `"package x\n\ntype bkSink struct{}\n\nfunc (s *bkSink) Write(p []byte) (int, error) { return len(p), nil }\n"`. Hits: type `bkSink` only; method `Write` extracted and green.
  - 21 (Python): `gate/x/tests/test_breakit.py`, text `"import unittest\n\nclass BreakIt(unittest.TestCase):\n    def test_holds(self):\n        pass\n\nqb3_cap = 1\n"`. Names: file `test_breakit`, class `BreakIt`, method `test_holds`, var `qb3_cap`. Hits: file `test_breakit`, class `BreakIt`, var `qb3_cap`.
  - 30: a Go source with `t.Run("F3/watermark", …)`. The extractor yields no subtest; the row holds a hit-free name list.
- **scope** (`{"row","path","in_scope"}`):
  - 31: `internal/playback/s3_2b_2.go` false
  - 20: `breakit_s3_3_1_test.go` true
  - `internal/x/testdata/a_test.go` false
  - `gate/proofs/drivers/test_x_planted.py` true
  - `bench/b_test.go` true
  - `docs/language/checks/test_c.py` true

`gate/runner/tests/test_testnames.py`, class `Golden`, with these tests:
- `test_every_golden_name_gets_the_verdict_its_row_names`
- `test_every_golden_source_yields_the_names_and_hits_its_row_names`
- `test_every_golden_path_is_in_or_out_of_scope_as_its_row_names`
- `test_the_golden_table_holds_every_counter_example_row` (rows 20–31 each present)
- `test_tokens_split_on_separators_and_case_changes`: `LaneA`→`lane_a`, `HTTPServer`→`http_server`, `s3_2b`→`s3_2b`, `x9Y`→`x9_y`, `a-b.c`→`a_b_c`, `wave2`→`wave2`
- `test_the_word_list_loads_and_each_rule_compiles` (the real `gate/testnames.json`)
- `test_a_word_list_with_an_unknown_key_or_a_bad_pattern_is_refused`
- `test_a_python_file_that_does_not_parse_is_a_fault_naming_it`
- `test_main_lists_only_tracked_test_files_outside_testdata`: a temp git repo with `a_test.go`, `testdata/b_test.go`, `c.go`, `pkg/test_d.py`, and an untracked `e_test.go`. `main` runs as a subprocess with `timeout=`; its output names `a_test.go` and `pkg/test_d.py` only, exit 0.

**Red first (§7.7):** commit `golden.json`, `testnames.json` and the tests first, and run the test module under the shim (red: the import of `gate.runner.testnames` fails, recorded). Then write the matcher. The reproducible plant: `mutate.py --py gate.runner.tests.test_testnames.Golden.test_every_golden_name_gets_the_verdict_its_row_names --file gate/runner/testnames.py --find <match's return line> --replace "    return ()"` should read red.

**Acceptance differential** (recorded once; not a unit test, because S1.1c-8's renames shrink the live hit set):
1. `git worktree add --detach $T/g0 37df879`
2. From `$T/g0`: `python3.15 -I -B <integration>/gate/runner/testnames.py --json $T/tool.json`
3. `python3.15 -I -c 'import json,sys; p={(h["path"],h["name"]) for h in json.load(open(sys.argv[1]))}; t={(h["path"],h["name"]) for h in json.load(open(sys.argv[2]))}; m=sorted(p-t); x=sorted(t-p); print("missed",len(m),*m,sep="\n"); print("extra",len(x)); sys.exit(1 if m else 0)' docs/sprints/s1-1c-7-plants/naming-prototype.json $T/tool.json`

Exit 0 with `missed 0` is the pass. Each extra goes into the record by class. The expected classes: Python `AnnAssign` or tuple targets; Go grouped specs the prototype's `GO_INNER` regex missed (a spec after a comment line, or a generic type). Any other class is named as found.

**Gate checks against the new files (checked in the code):**
- `pytests`' `misplaced()` (`rows/pytests.py:38-64`):
  - `test_testnames.py` is in `gate/runner/tests/`, tracked, and in the row's scope (`gate/**/test_*.py`);
  - `testnames.py` is not under a tests directory and does not start with `test_`;
  - `golden.json` and `testnames.json` are not `.py`;
  - `missing_init`: `gate/`, `gate/runner/` and `gate/runner/tests/` hold `__init__.py`.
- **Collisions** (`rows/__init__.py:77-101`): `gate.runner.testnames` and `gate.runner.tests.test_testnames` are distinct, and no tracked `testnames` path exists (`git ls-files`).
- **Teeth and law:**
  - `law.json` instruments: no instrument's `code` list or proofs name `gate/bench/**`, `tools/astequal` or `archcheck_test.go` (only `gate/proofs/drivers/test_archcheck_planted.py`, 1 case), so no case count moves;
  - `test_instruments_pin.py` has no rule that every module must belong to an instrument;
  - the new files show in LAW Δ under the `gate/runner/` and `gate/` code paths.
- **ruff:** scope `gate/**/*.py`, target py315, rules as above, `--ignore-noqa`. No per-file version pin is needed. The new code must use `timeout=` on every subprocess and no `time.sleep` (TID251). The test fixture's `threading.Event().wait()` is inside a string.
- **pyfloor:** a fixed list of 5 files (registry), none of them new, so there is no 3.12 constraint.
- **suppressions:** the new files carry no suppression comment.
- **gofmt and testhygiene:** no Go file is added under `gate/`.

---

### Findings

#### G10-2 — `gate/bench/escapes.py:340`
1. **Verdict: accepted.**
   - `check` sets `counted = {original_after, *(h for h in helpers if h not in after.inlined)}` (`:336`), but `old = _texts(before, {original_before})` (`:340`). The leak side counts the helpers that existed before (`members_before`, `:347`).
   - A helper that already existed, was not inlined and has a heap line shows as "new" and gives exit 1.
   - The existing `test_the_before_side_lists_the_helpers_that_existed_before` uses `store(p *int){sink = p}`, which has a leak line and no heap line, so the gap is untested.
   - GitNexus `impact(check, upstream)`: d=1 `main` only, risk LOW, epistemic exact.
2. **Kind: K1.** A behaviour fix in the hand-run tool. R5 does not apply (Python tooling).
3. **Change:**
   ```python
   counted_before = {original_before, *(h for h in helpers if h in before.functions and h not in before.inlined)}
   ...
   old, new = _texts(before, counted_before), _texts(after, counted)
   ```
   The docstring sentence "The group is …" gains: "on each side, over the members that exist at that revision".

   Test `Escapes.test_a_helper_that_existed_before_with_its_own_heap_line_is_no_cost` (`gate/bench/tests/test_escapes.py`):
   - `HEAPHELPER_BEFORE = "package m\n\nvar sink *int\n\n//go:noinline\nfunc store(n int) {\n\tx := n\n\tsink = &x\n}\n\nfunc Outer(n int) { store(n) }\n"`
   - `HEAPHELPER_AFTER` is the same with `//go:noinline\nfunc fresh(n int) { store(n) }\n\nfunc Outer(n int) { fresh(n) }`.
   - Call `self.escapes(before, after, "Outer", "store", "fresh")`. Assert `code == 0`; `"heap before: moved to heap: x x1"` in the lines; `"heap after: moved to heap: x x1"` in the lines (the witness that the helper has a heap line); last line `verdict: no new heap line in Outer's group`.
4. **Proof (K1):** plant `--file gate/bench/escapes.py --find "_texts(before, counted_before)" --replace "_texts(before, {original_before})"`. It keeps `counted_before`. Run with `--py gate.bench.tests.test_escapes.Escapes.test_a_helper_that_existed_before_with_its_own_heap_line_is_no_cost` at W0-4. Expected **red** at `self.assertEqual(code, 0, out)` (1 != 0).
5. **Commit:** W0-4.

#### G10-3 — `gate/bench/load.py:95`, `mutate.py:211`
1. **Verdict: accepted.** Both build `_Parser(prog=…)` with argparse's default `add_help` (`load.py:95`, `mutate.py:211`). `--help` prints the usage, then `parser.exit()` raises `SystemExit(0)`. `main`'s `except BaseException` (`load.py:149`, `mutate.py:296`) turns that into "error: internal error: SystemExit: 0" and exit 2. `compare.py:77` and `escapes.py` pass `add_help=False`.
2. **Kind: K1**, with user text (the help output).
3. **Change:** `Helped` and `_Parser.exit` as in A.2, plus `except Helped: return 0` in each `main`.
4. **Proof (K1):** each file's plant deletes the block below (find it, replace with ""):
   ```
       def exit(self, status: int = 0, message: str | None = None) -> NoReturn:
           """Reached only from --help, after the usage is printed (error is overridden above)."""
           raise Helped
   ```
   (preceded by a blank line). Run with `--py …test_mutate.Refusals.test_help_prints_the_usage_and_exits_0_before_any_go_command`, then with `…test_load.Fake.test_help…`. Each is expected **red** at `assertEqual(code, 0)` (2 != 0).
5. **Commit:** W0-4.

#### G10-4 — `gate/bench/compare.py:116`
1. **Verdict: accepted.** `raise Error(f"{binary} reported a benchmark twice: {', '.join(sorted(names))}")` lists every name. GitNexus: `_run` ← `_time_pass`, `_alloc_pass`; it calls `parse_bench` and `run`.
2. **Kind: K1** (error text).
3. **Change:** `repeated = sorted(n for n, k in collections.Counter(names).items() if k > 1)`, then `if repeated: raise Error(f"{binary} reported a benchmark twice: {', '.join(repeated)}")`, plus `import collections`.

   Test `Verdict.test_a_benchmark_reported_twice_is_named_alone`:
   - before's time run `{"out": support.output({"BenchmarkA": (1000,), "BenchmarkB": (10,)}) + support.output({"BenchmarkA": (1000,)})}`;
   - assert exit 2 and that the last line ends with `reported a benchmark twice: BenchmarkA`.
4. **Proof (K1):** plant find `", '.join(repeated)}")`, replace `", '.join(sorted(names))}")`. Expected **red** at the `endswith` assertion. Dry run: `git grep -n "reported a benchmark twice"` gives `compare.py:116` only; no test pins the text.
5. **Commit:** W0-4.

#### G2-6 — `gate/bench/tests/test_compare.py:232`
1. **Verdict: accepted.** `self.assertEqual(math.comb(4, 2), 6)` tests arithmetic. The proposed `p 0.3333` is **rejected**: at pairs 2, a real A/A run's p is 1/3, 2/3 or 1 (more with ties) and depends on noise. With n = 2 + 2, the tie-conditional distribution is symmetric, so p ≥ 2/6.
2. **Kind: K2a.**
3. **Change:** replace the line with:
   ```python
   m = re.search(r", p ([0-9.e+-]+), ", lines[0])
   self.assertIsNotNone(m, out)
   self.assertGreaterEqual(float(m[1]), 0.3333)  # at pairs 2 the smallest two-sided p is 2/6 = 1/3
   ```
   Drop `import math` (otherwise F401) and add `import re`.
4. **Proof:**
   ```json
   {"id": "G2-6", "old_rev": "<lane base>", "new_rev": "<this commit>", "file": "gate/bench/compare.py", "find": "f\"p {float(p):.4g}, {alloc_text}: {verdict}\"", "replace": "f\"p {float(p) / 10:.4g}, {alloc_text}: {verdict}\"", "py": "gate.bench.tests.test_compare.RealGo.test_a_real_test_binary_against_itself", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-6-control", "control": true, "old_rev": "<lane base>", "new_rev": "<this commit>", "py": "gate.bench.tests.test_compare.RealGo.test_a_real_test_binary_against_itself", "expect": {"old": "green", "new": "green"}}
   ```
   The plant changes only the printed p, so the verdict, and the old test, stay green deterministically.
5. **Commit:** W0-5.

#### G2-7 — `gate/bench/tests/test_load.py:259`
1. **Verdict: accepted.** `assertTrue("TestSlow" in … or "timed out" in out)` passes on a verdict naming no test, because the tail always shows "panic: test timed out". `tally` counts a run with no end as a failure (`load.py:75`), and `below` holds tests with `f or not k` (`:135`). So the verdict is deterministic whether or not test2json sends a fail for TestSlow.
2. **Kind: K2a.**
3. **Change:**
   ```python
   self.assertIn("TestSlow: 0/1 under load (hogs 1)\n", out)
   self.assertIn("\n  panic: test timed out after 3s\n", out)
   self.assertTrue(out.endswith("verdict: below N/N: TestSlow; go test exit 1\n"), out)
   ```
   No rename (the name's "or" understates and is not false).
4. **Proof:**
   ```json
   {"id": "G2-7", "old_rev": "<lane base>", "new_rev": "<this commit>", "file": "gate/bench/load.py", "find": "below = sorted(test for test, (k, f) in counts.items() if f or not k)", "replace": "below: list[str] = []", "py": "gate.bench.tests.test_load.DiedOutsideATest.test_a_binary_that_times_out_names_the_test_or_the_cause", "expect": {"old": "green", "new": "red"}}
   ```
   Plus the matching control entry (green/green). The test drives a real 3 s `-timeout` under hogs, so it also gets `python3.15 -I -B -X utf8 gate/runner/stress.py --runs 50 gate.bench.tests.test_load` → 50/50.
5. **Commit:** W0-5.

#### G2-147 — `tools/archcheck/archcheck_test.go:548-564`
1. **Verdict: accepted.** `exec.Command("go", "build", …)` (`:550`) and `exec.Command(bin, "-tests", list)` (`:557`) have no ceiling (testing.md rule 4).
   - The proposed `runCLI` swap is **rejected**. TestCLI already has the in-process case "-tests: a non-test file is an internal error naming it" (`:700-705`), so the swap would delete the only test of `main`'s wiring (`main.go:1754`: `os.Exit(cli(".", os.Args[1:], os.Stdout))`).
2. **Kind: K2e** (plan §7.2 names G2-147 there).
3. **Change:** following `tools/apipartition/main_test.go:52-53, 75-90`:
   ```go
   ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
   defer cancel()
   ... exec.CommandContext(ctx, "go", "build", "-o", bin, ".") ...
   cmd := exec.CommandContext(ctx, bin, "-tests", list)
   ...
   out, err := cmd.CombinedOutput()
   if ctx.Err() != nil {
       t.Fatalf("archcheck -tests: not finished under the 5m ceiling\n%s", out)
   }
   ```
   Add `"context"` and `"time"` to the imports. `testhygiene` bans only `time.{Sleep,After,AfterFunc,Tick,NewTimer,NewTicker}`, and `time.Duration` with `context.WithTimeout` is allowed (`archcheck_test.go:533-538`).
4. **Proof:** `git grep -nE '(^|[^[:alnum:]_.])exec\.Command\(' -- tools/archcheck/archcheck_test.go tools/astequal/astequal_test.go`. The old tree gives `:550`, `:557` and `astequal_test.go:221` (red). The new tree gives none.
   - This is a syntactic check over call text. Grep shows the text `exec.Command(` sits in no comment or string in these files.
   - An AST program would have to be a `.go` file, which the Go rows would load.
5. **Commit:** W0-7.

#### G2-148 — `archcheck_test.go:174`
1. **Verdict: accepted.** The body asserts only `wantOne(... "clock: internal/table may not import time")`. The comment at `:176` sends the cron.go half to `TestListsClockMembership` (`:461-469`), which tests it.
2. **Kind:** none in S1.1c-7.
3. **Carried in full to S1.1c-8:** the proposed name "an operator package may not import time". Its verb "may not import" is backed by `wantOne(t, check(t, "internal/table/x.go", …), "clock: internal/table may not import time")` at `:175`.
5. **Commit:** none.

#### G2-149 — `archcheck_test.go:1592-1633`
1. **Verdict: accepted.** The comment says "in each mode", but no case uses the normal mode. A usage error also returns 2 whatever the writer (`main.go:1695-1698`), so a mis-built case would pass vacuously.
2. **Kind: K2a.**
3. **Change:**
   - The case struct gains `verdict int`.
   - The existing cases get `verdict`: `-cases` 0; `-tests with a finding` 1; `-tests clean` 0.
   - New case `"normal mode with a finding"`: root `cliRoot(t, nil, nil, nil, map[string]string{"internal/event/planted.go": "package event\n\nimport _ \"" + mod + "internal/model\"\n"})`, list `"internal/event/planted.go\x00"`, args `[]string{list}`, verdict 1.
   - New case `"normal mode clean"`: root with `"internal/model/x.go": "package model\n"`, verdict 0.
   - Each subtest first runs the witness `var ok strings.Builder; if code := cli(root, args, &ok); code != c.verdict { t.Fatalf("witness: cli %v: exit %d, want %d; output %q", args, code, c.verdict, ok.String()) }`, then the existing failing-writer check.
4. **Proof:** two entries, both with `"test": "TestCLIWriteFailureIsExit2"` and `"pkg": "tools/archcheck"`:
   ```json
   {"id": "G2-149-clean", "file": "tools/archcheck/main.go", "find": "\t\treturn out.exit(1)\n\t}\n\treturn out.exit(0)\n}", "replace": "\t\treturn out.exit(1)\n\t}\n\tif !tests {\n\t\treturn 0\n\t}\n\treturn out.exit(0)\n}", "test": "TestCLIWriteFailureIsExit2", "pkg": "tools/archcheck", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   {"id": "G2-149-finding", "file": "tools/archcheck/main.go", "find": "\tif len(findings) > 0 {\n\t\treturn out.exit(1)", "replace": "\tif len(findings) > 0 {\n\t\tif !tests {\n\t\t\treturn 1\n\t\t}\n\t\treturn out.exit(1)", "test": "TestCLIWriteFailureIsExit2", "pkg": "tools/archcheck", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   ```
   Plus a control entry (green/green). The test uses `t.Parallel`, so also run `load.py -- -race -count=50 -run '^TestCLIWriteFailureIsExit2$' ./tools/archcheck` → 50/50.
5. **Commit:** W0-7.

#### G2-150 — `archcheck_test.go:188-193`
1. **Verdict: accepted.** Only `len(got) != 2` is asserted. GitNexus: `CheckSources` → `purity`, which reports `"purity: channel type: …"` (`main.go:617`) and `"purity: channel send: …"` (`:619`).
2. **Kind: K2a.**
3. **Change:**
   ```go
   has := func(want string) bool { return slices.ContainsFunc(got, func(f string) bool { return strings.Contains(f, want) }) }
   if len(got) != 2 || !has("purity: channel type: ") || !has("purity: channel send: ") {
       t.Fatalf("want the chan type and the send, got %q", got)
   }
   ```
4. **Proof:**
   ```json
   {"id": "G2-150", "file": "tools/archcheck/main.go", "find": "\"purity: channel send: only internal/runtime, io/* and cmd use channels (ADR-0015 §2.2)\"", "replace": "\"purity: channel type: only internal/runtime, io/* and cmd use channels (ADR-0015 §2.2)\"", "test": "TestPurity", "pkg": "tools/archcheck", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
   ```
   Plus a control entry. No other test pins the send's wording (`grep "channel send"` matches only `main.go:619`).
5. **Commit:** W0-7.

#### G2-151 — `archcheck_test.go:405-414, 453-458`
1. **Verdict: accepted, in two parts.**
   - (a) "the targets are read into the purity map" compares the outer length and the wanted inner slices only, so an extra import key passes.
   - (b) "the repository's own lists load" pins purity.json's real `internal/table: [Mutex]` (a live-data pin).
   - GitNexus `LoadLists` ← `cli`, `loadAt`, `testLists`, `TestListsPurity`, `TestFixGenerated`, `TestLoadListsFaults`.
2. **Kind:** (a) K2a; (b) K2e (live data).
3. **Change:**
   - (a) Replace the length check and loops with `if !reflect.DeepEqual(l.Purity, want) { t.Fatalf("purity %v, want %v", l.Purity, want) }` and add `"reflect"` to the imports.
   - (b) The body becomes `testLists(t)` alone (it fails on a load error), with the comment `// the real lists load; their content is the gate's, not this unit test's`.
4. **Proof:**
   - (a):
     ```json
     {"id": "G2-151-map", "file": "tools/archcheck/main.go", "find": "\t\tl.Purity[pkg][imp] = append(l.Purity[pkg][imp], ident)\n", "replace": "\t\tl.Purity[pkg][imp] = append(l.Purity[pkg][imp], ident)\n\t\tl.Purity[pkg][imp+\"/x\"] = []string{ident}\n", "test": "TestListsPurity", "pkg": "tools/archcheck", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "green", "new": "red"}}
     ```
     The other TestListsPurity subtests stay green on this plant: they read only the keys they name.
   - (b):
     ```json
     {"id": "G2-151-live", "file": "tools/allowlists/purity.json", "find": "\"target\": \"internal/table: sync.Mutex\"", "replace": "\"target\": \"internal/table: sync.RWMutex\"", "test": "TestListsPurity", "pkg": "tools/archcheck", "old_rev": "<lane base>", "new_rev": "<this commit>", "expect": {"old": "red", "new": "green"}}
     ```
     This is K2e's live-data shape. It needs `--expect`'s commit-in-worktree application, because an overlay cannot change a file read at run time.
5. **Commit:** W0-7.

#### G2-152 — `archcheck_test.go:1603-1606`
1. **Verdict: accepted.** The `-cases` case writes `sleepy` (`:1604`), but its args name `internal/x/gone_test.go`. `listCases` reads only the named paths (`main.go:1670-1671`). GitNexus: `cli` → `listCases`.
2. **Kind: K2e** (a misleading fixture). Fixed in part.
3. **Change:**
   - The root becomes `cliRoot(t, nil, nil, nil, map[string]string{})`. This changes no outcome: the file was never read, so the real code gives exit 2 before and after, and G2-149's witness gives 0.
   - The witness also asserts, for this case, `strings.HasPrefix(ok.String(), "internal/x/gone_test.go: fault : ")`, so the case states that it lists a missing file.
   - **Carried to S1.1c-8:** the name "-cases on a missing file". "on a missing file" is backed by the new `HasPrefix(... "gone_test.go: fault : ")` assertion.
4. **Proof:** `git show <rev>:tools/archcheck/archcheck_test.go | grep -c 'root := cliRoot(t, nil, nil, nil, map\[string\]string{sleepy: sleepySrc})'` gives 2 on the old file (`:1604` and `:1608`: a fixture beside a missing-file listing, red) and 1 on the new. Plus a control entry (green/green) on TestCLIWriteFailureIsExit2.
5. **Commit:** W0-7.

#### G2-153 — `archcheck_test.go:24-31`
1. **Verdict: accepted.** `for i := 0; i+1 < len(other); i += 2` drops an odd last element. GitNexus `impact(check)`: 9 d=1 callers, risk MEDIUM. Each call passes 0 or 2 extra arguments; the two multi-line calls at `:232-236` each pass a pair. So the new failure breaks no caller.
2. **Kind: K2e.**
3. **Change:** first line of the body: `if len(other)%2 != 0 { t.Fatalf("check: other holds %d strings, want path, source pairs", len(other)) }`.
4. **Proof:** a probe file in a scratch worktree, never committed, at each revision:
   1. `git worktree add --detach $T/wt <rev>`
   2. Write `$T/wt/tools/archcheck/probe_test.go` = `package main\n\nimport "testing"\n\nfunc TestCheckProbeOddOther(t *testing.T) {\n\tcheck(t, "internal/model/x.go", "package model\n", "internal/model/y.go")\n}\n`, with the inner `\n` written as a Go escape.
   3. `go test -count=1 -run '^TestCheckProbeOddOther$' ./tools/archcheck` from `$T/wt`.

   Old: `ok` (the odd argument is silently accepted: red for the check). New: FAIL "check: other holds 1 strings".
5. **Commit:** W0-7.

#### G2-154 — `tools/astequal/astequal_test.go:269-279`
1. **Verdict: accepted.** The error rows have `out: ""`, so `strings.Contains(stdout, "")` always holds, and stderr is never read. `run` writes the usage (`main.go:57`) and each `astequal: <err>` (`:65`, `:70`) to stderr.
2. **Kind: K2a.** W0-go designates the top-level test (plan §1).
3. **Change:**
   - Field `out` keeps its meaning for codes 0 and 1. A new field `errOut` holds the stderr fragment for code 2.
   - Rows:
     - "bad revision": `errOut: "astequal: git ls-tree -r -z --name-only no-such-rev -- p/a.go: "`
     - "no arguments": `"usage: astequal <before-rev> [<after-rev>] [<paths>]\n"`
     - "parse error": `"astequal: p/a.go:2:"`
   - Check:
     ```go
     if tc.code == 2 {
         if tc.errOut == "" || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.errOut) {
             t.Fatalf("stdout %q stderr %q; want no stdout and stderr holding %q", stdout.String(), stderr.String(), tc.errOut)
         }
         return
     }
     if tc.out == "" || stderr.Len() != 0 || !strings.Contains(stdout.String(), tc.out) {
         t.Fatalf("stdout %q stderr %q; want no stderr and stdout holding %q", stdout.String(), stderr.String(), tc.out)
     }
     ```
4. **Proof:** three entries, each with `"test": "TestRunExitCodes"`, `"pkg": "tools/astequal"` and expect old green, new red:
   - `G2-154-usage`: find `"usage: astequal <before-rev> [<after-rev>] [<paths>]"`, replace `""`;
   - `G2-154-git`: find `"readSides(s, paths)\n\tif err != nil {\n\t\t_, _ = fmt.Fprintln(stderr, \"astequal:\", err)"`, replace the same ending in `fmt.Fprintln(stderr, \"astequal:\")`;
   - `G2-154-parse`: the same shape with `"compareSources(before, after)\n\tif err != nil {\n\t\t_, _ = fmt.Fprintln(stderr, \"astequal:\", err)"`.

   Plus a control entry. The subtests use `t.Parallel`, so also run `load.py -- -race -count=50 -run '^TestRunExitCodes$' ./tools/astequal` → 50/50.
5. **Commit:** W0-8.

#### G2-155 — `astequal_test.go:219`
1. **Verdict: accepted.** `exec.Command("git", …)` has no ceiling. GitNexus `impact(gitIn)`: 5 d=1, 4 d=2, risk MEDIUM. The callers are `gitRepo`, `commitFiles` and the tests that build repos; behaviour is unchanged absent a hang.
2. **Kind: K2e.**
3. **Change:**
   ```go
   ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
   defer cancel()
   cmd := exec.CommandContext(ctx, "git", …)
   ...
   out, err := cmd.CombinedOutput()
   if ctx.Err() != nil {
       t.Fatalf("git %v: not finished under the 2m ceiling\n%s", args, out)
   }
   ```
   Then the existing error check. Add `"context"` and `"time"` to the imports.
4. **Proof:** the same `git grep` as G2-147: `astequal_test.go:221` on the old tree, none on the new.
5. **Commit:** W0-8.

---

### Commit plan (ordered)

W0-py has 6 commits, 2 of them lane fixes (plan budget: tools 3 + 1, lane 2). W0-go has 2. **Total 8.**

1. **W0-1** `mutate.py --patch` and `--py`, the shared `worktree()`, `REQUIRED` per mode. Tests #9–16. Proof: each feature-off plant red (A.3), plus the hand-run bootstrap witness.
2. **W0-2** `--subtest` (rewrite, QuoteMeta, probe, own-line judging, `ran:` lines). Tests #1–8. Proof: plants red.
3. **W0-3** `--expect` batches (id, revs, expect, per-revision keys, control). Tests #17, #17a, #18. Proof: plants red.
4. **W0-4** K1 for gate/bench: G10-2, G10-3 (#19), G10-4. Proof: a plain batch of four `--py` revert plants, run from a checkout of W0-4, each red.
5. **W0-5** K2 for gate/bench: G2-6, G2-7. Proof: the `--expect` entries plus controls; G2-7's stress run.
6. **W0-6** the naming tool. Proof: the red run before the matcher, the `--py` plant, and the acceptance differential.
7. **W0-7** (W0-go) K2 for tools/archcheck: G2-147, -149, -150, -151, -152, -153. Proof: entries plus controls, the grep check, the probe, the load run.
8. **W0-8** (W0-go) K2 for tools/astequal: G2-154, -155. Proof: entries plus controls, the grep check, the load run.

**Order between the halves:**
- W0-go writes its fixes beside W0-py; its files are disjoint.
- W0-go **runs its proofs only after W0-3 is on the integration branch**, because every W0-go proof is an `--expect` entry. W0-go then rebases.
- All W0-go entries designate the **top-level** test, as the plan requires for G2-154 and I extend to the archcheck subtests. So W0-go's proofs never depend on `--subtest` being right.
- The batches are committed by the coordinator as `docs/sprints/s1-1c-7-plants/W0-py.json` (`--expect`), `W0-py-k1.json` (plain) and `W0-go.json`.
- The proofs row runs alone at T4 after the hand-back (W0 touches `gate/`).

### Choices made (chosen by this design note)

1. **`--help` (G10-3).**
   - Input: `mutate.py --help`, `load.py --help`.
   - Candidates: (a) `add_help=False`, giving exit 2 "unrecognized arguments"; (b) usage and exit 0.
   - Chosen: (b), as plan §6 #19 states the outcome.
   - Dry run: `git grep -n "internal error: SystemExit"` finds no test pinning the old text.
2. **The duplicate-benchmark text (G10-4).** It lists only the repeated names. Dry run: `compare.py:116` only.
3. **G2-6's assertion is `p ≥ 0.3333`, not an exact value.** The exact p is noise-dependent (reason above).
4. **The probe suffix on every subtest level** (plan: the last level). A duplicated parent (`g`, `g#01`) would otherwise let `^g$` silently pick the first. This is a strict superset of the plan's probe; reviewers may narrow it.
5. **`--expect` applies Go plants by committing them in the worktree, not through an overlay.** This makes a data-file plant possible (G2-151-live) and matches `--py`. Plain and single modes keep the overlay. *(annotated 2026-11-21: superseded — every mode runs Go in a worktree at the commit with the plant committed; the overlay path is deleted — owner, [S1.1c-7's PR 1 review](decisions.md#owner-decisions-2026-11-21--s11c-7s-pr-1-review), decision 1)*
6. **A subtest level holding a non-printable rune is refused, not emulated.**
7. **`--patch` refuses new or deleted files, renames and the no-newline marker.** Revert plants keep new declarations and files, so these are not needed.
8. **Python red also requires `Ran 1 test in`.** A class id with one failure among several is invalid.
9. **The naming tool's file-kind name is the stem**, matching the prototype, so the differential compares `(path, name)`. Its Python scope adds `AnnAssign` and tuple targets (a superset, for recall).

### Couplings and risks

- **§0 item 5 anchors:** none touched. No `law.json` count moves (checked). `copy_gate` copies W0's `gate/` files, so the proofs row runs after the hand-back.
- **R5 (per-event code):** none. Every change is in Python tools, test files or `tools/` test code.
- **R4 (frozen benchmarks):** none. **Frame tables and line-anchored mutants:** none.
- **Other owners' files:** none edited. `tools/allowlists/purity.json` is planted only inside a throwaway worktree.
- **Pre-mortem (the-fool):**
  - (i) `--expect` worktrees left behind after a SIGTERM. Mitigated by the `finally` and the list self-check (#16).
  - (ii) The `--py` copy running the tree's shim instead of the copy's. The argv names `<wt>/gate/runner/pyshim.py`, and the PyMode fixture copies the shim from `support.REPO`.
  - (iii) A commit hook firing in the worktree. `-c core.hooksPath=/dev/null` is set on `worktree add` and `commit`.
  - (iv) G10-2's fixture compiling differently in Go 1.27, so `store` has no heap line. The test's "heap after" witness turns that into a visible failure, not a false pass.
  - (v) G2-152's grep check is weak (it checks the edit's text). A reviewer may prefer calling it a no-outcome refactor with control entries only.
- **Observation, not planned:** `astequal_test.go`'s `src(pairs...)` (`:14-20`) has the same odd-count habit as G2-153, and `astequal/main.go:183`'s production `git()` has no ceiling. Both are outside this chunk's findings.

### Carried items

- **G2-148:** carried in full to S1.1c-8. The name "an operator package may not import time" is backed by the `wantOne(... "clock: internal/table may not import time")` call at `:175`.
- **G2-152:** the name "-cases on a missing file" is carried to S1.1c-8, backed by the new witness `HasPrefix(ok.String(), "internal/x/gone_test.go: fault : ")`.


## Revision 1, after the blind reviews

## W0 design note, revision 1: replacement sections and change list

Base: 37df879. The brief's rules hold: read-only, nothing run. Every section below **replaces** the section of the same name in the first note. Everything else stands: Part B, every other finding, the summary table, the choices and the risks.

### Change list

| Review item | What changed | Where |
|---|---|---|
| DA-S4 | Step 8 checks only this run's own worktree paths and temp dirs; paths are compared with `realpath`. The whole-list comparison stays only inside #16's isolated fixture repo. | A.2 `Worktrees.close`; A.3 #16 |
| DA-L5 | One worktree per revision for a whole run (`Worktrees`), reset between entries (`checkout --force --detach`, then `clean -ffdx`, hooks off). No `-trimpath`, ever. | A.2 `Worktrees`, `_go`; A.3 #17 (second plant) |
| DQ-S2 | `ran()` is unanchored after the parent at the designated depth, so #5's plant now reads red. #3, #7 and #8 share one two-hunk `--patch`. | A.2 `runs`/`ran`/`run_go`; A.3 #3, #5, #7, #8 |
| DQ-S4 | `parse_patch` strips git's `a/` and `b/`, skips `diff --git`, `index` and mode lines, requires the two paths to agree and checks hunk counts. New counter-example #9b uses real `git diff` output. | A.2 `parse_patch`; A.3 #9b |
| DQ-L5 | New per-entry batch key `timeout`: a Go duration string or positive seconds, defaulting to `--timeout`. New counter-examples #14b and #14c. | A.1; A.2 `_entry_timeout`; A.3 #14b, #14c |
| DQ-L6 | G2-149's two new cases with their exact list-file and fixture bytes. | G2-149 |
| DQ-L7 | The grep for G2-147 and G2-155 is recorded as named deviation **D-1**, with why it is sound. The scope widens to every test file of both packages. | G2-147, G2-155 |
| Item 8 | No W0 Go test needs the serial marker. A lane that adds one writes `// Not parallel: <reason>` (the existing form, e.g. `internal/clock/firegroup_bench_test.go:40`). | – |

**Where the new work lands:**
- **W0-1:** #9b and the new `parse_patch`; `Worktrees` replaces the old `worktree()`.
- **W0-3:** the `timeout` key, #14b, #14c, and #17's reset plant.
- The commit count is unchanged: 8.

**Checked again in the code:**
- `os/exec` is imported without an alias in both test files: `archcheck_test.go:8`, `astequal_test.go:6`.
- The text `exec.` appears only at `archcheck_test.go:550, :557, :560` and `astequal_test.go:221`.
- Neither package's other test files (`breakit_test.go`, `breakit_s11c2_test.go`) contains `exec.`.

---

### A.1 Data and constants (replaces A.1)

```python
OUTCOMES = ("red", "green", "timeout", "invalid")                 # unchanged
PYTHON = "python3.15"                                              # rows/pytests.py PYTHON
MUTATION_KEYS = frozenset({"file", "find", "replace", "patch"})
TARGET_KEYS = frozenset({"test", "subtest", "py"})
SIDE_KEYS = frozenset(f"{s}_{k}" for s in ("old", "new") for k in TARGET_KEYS)
PLAIN_KEYS = MUTATION_KEYS | TARGET_KEYS | {"id", "note", "pkg", "timeout"}
EXPECT_KEYS = PLAIN_KEYS | SIDE_KEYS | {"old_rev", "new_rev", "expect", "control"}
FULL_REV = re.compile(r"[0-9a-f]{40}")
GO_META = frozenset("\\.+*?()|[]{}^$")
GO_SPACES = frozenset("\t\n\v\f\r \u0085      　") | frozenset(map(chr, range(0x2000, 0x200B)))
THROWAWAY = {"GIT_AUTHOR_NAME": "mutate.py", "GIT_AUTHOR_EMAIL": "mutate@invalid",
             "GIT_COMMITTER_NAME": "mutate.py", "GIT_COMMITTER_EMAIL": "mutate@invalid"}
NO_HOOKS = ("-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false")

@dataclasses.dataclass(frozen=True)
class Target:
    test: str = ""
    levels: tuple[str, ...] = ()
    py: str = ""
    @property
    def full(self) -> str: return "/".join((self.test, *self.levels))

@dataclasses.dataclass(frozen=True)
class Change:
    file: str
    original: str
    mutant: str

@dataclasses.dataclass(frozen=True)
class Entry:
    id: str
    changes: tuple[Change, ...]     # () only for a control entry
    old: Target
    new: Target
    pkg: str
    note: str
    timeout: str                    # a Go duration; the entry's "timeout", else the run's --timeout
    old_rev: str = ""
    new_rev: str = ""
    expect: tuple[str, str] = ("", "")
    control: bool = False
```

### A.2 Functions (replaces A.2; unchanged items are listed by name)

**Unchanged from the first note:** `go_rewrite`, `quote_meta`, `target`, `pattern`, `judge_py`, `apply`, `base_text`, `changes_of`, the CLI rules, the output rules, and G10-3's `Helped`/`_Parser.exit`.

**The probe count and the `ran:` list are now two functions:**

```python
def runs(text: str, t: Target) -> list[str]:
    """The probe's count: `=== RUN` names equal to the designated path, each level allowing a `#NN` suffix."""
    rx = r"^=== RUN\s+(" + re.escape(t.test) + "".join("/" + re.escape(l) + r"(?:#[0-9]+)?" for l in t.levels) + r")$"
    return re.findall(rx, text, re.MULTILINE)


def ran(text: str, t: Target) -> list[str]:
    """Every `=== RUN` name under the designated parent at the designated depth, whatever its last level: a run that
    selected a sibling (F30 beside F3) shows it here."""
    parent = re.escape(t.test) + "".join("/" + re.escape(l) + r"(?:#[0-9]+)?" for l in t.levels[:-1])
    return re.findall(r"^=== RUN\s+(" + parent + r"/[^/\n]+)$", text, re.MULTILINE)
```

**`run_go` and its helper `_go`** (exact text; the plants in A.3 refer to it):

```python
def run_go(e: Entry, side: Target, args: argparse.Namespace, scratch: str, cwd: str | None) -> tuple[str, gorun.Result, list[str]]:
    """One Go run of `side`: through the overlay in the tracked tree (cwd None) or in a worktree holding the plant."""
    if side.levels:
        probe = _go(e, args, scratch, cwd, pattern(side, probe=True))
        n = len(runs(f"{probe.stdout}\n{probe.stderr}", side))
        if n != 1:
            return "invalid", probe, [f"probe: {n} subtests run as {side.full}"]
    result = _go(e, args, scratch, cwd, pattern(side, probe=False))
    names = ran(f"{result.stdout}\n{result.stderr}", side) if side.levels else []
    return judge_go(result, side), result, [f"ran: {name}" for name in names]


def _go(e: Entry, args: argparse.Namespace, scratch: str, cwd: str | None, run: str) -> gorun.Result:
    top = cwd or os.getcwd()
    argv = ["go", "test"]
    if cwd is None:  # the tracked tree: the mutants through -overlay (mutate.py:180-201 today)
        argv.append(f"-overlay={_overlay(e, scratch, top)}")
    argv += ["-count=1", "-v", "-run", run, "-timeout", e.timeout]
    if args.race:
        argv.append("-race")
    if args.cpu:
        argv += ["-cpu", args.cpu]
    argv.append("./" + e.pkg)
    # Never -trimpath: frame tables and line-anchored tests match file paths in stack frames.
    return gorun.run(argv, cwd=top, env={**os.environ, "PWD": top}, timeout_s=seconds(e.timeout) + WALL_EXTRA_S)
```

**`judge_go`:**

```python
def judge_go(result: gorun.Result, t: Target) -> str:
    text = f"{result.stdout}\n{result.stderr}"
    if "panic: test timed out after" in text:
        return "timeout"
    if not t.levels:
        if re.search(rf"^--- FAIL: {re.escape(t.test)} \(", text, re.MULTILINE):
            return "red"
        if result.exit == 0 and not result.timed_out and re.search(rf"^--- PASS: {re.escape(t.test)} \(", text, re.MULTILINE):
            return "green"
        return "invalid"
    name = re.escape(t.full)
    if not re.search(rf"^=== RUN\s+{name}$", text, re.MULTILINE):
        return "invalid"
    if re.search(rf"^\s*--- SKIP: {name} \(", text, re.MULTILINE):
        return "invalid"
    if re.search(rf"^\s*--- FAIL: {name} \(", text, re.MULTILINE):
        return "red"
    if result.exit == 0 and not result.timed_out and re.search(rf"^\s*--- PASS: {name} \(", text, re.MULTILINE) \
            and not re.search(r"^\s*--- FAIL: ", text, re.MULTILINE):
        return "green"
    return "invalid"
```

**`Worktrees`** replaces `worktree()`. The plain batch, the single `--py` run, `--repeat` with `--py`, and `--expect` each open one `Worktrees` for the whole run.

```python
class Worktrees:
    """One detached worktree per revision for one run, reset between entries, so the Go build cache (keyed by the
    worktree's path) is reused; every one removed when the run ends, a stop included."""

    def __init__(self) -> None:
        self._trees: dict[str, tuple[str, str]] = {}           # rev -> (tmp, path)

    def __enter__(self) -> "Worktrees":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    def at(self, rev: str, changes: tuple[Change, ...]) -> str:
        """The worktree at `rev`, clean, with `changes` committed under the throwaway identity."""
        if rev in self._trees:
            path = self._trees[rev][1]
            _git_ok("-C", path, *NO_HOOKS, "checkout", "-q", "--force", "--detach", rev)
            _git_ok("-C", path, "clean", "-q", "-ffdx")
        else:
            tmp = tempfile.mkdtemp(prefix="mutate-wt-")          # outside the repo: pytests' misplaced() walks gate/
            path = os.path.join(tmp, "wt")
            self._trees[rev] = (tmp, path)                       # recorded first, so close() removes a half-made one
            _git_ok(*NO_HOOKS, "worktree", "add", "--detach", path, rev)
        for change in changes:
            with open(os.path.join(path, change.file), "wb") as f:
                f.write(change.mutant.encode("utf-8", errors="surrogateescape"))
        if changes:
            _git_ok("-C", path, *NO_HOOKS, "add", "--", *(c.file for c in changes))
            _git_ok("-C", path, *NO_HOOKS, "commit", "-q", "-m", "mutate.py plant", env=THROWAWAY)
        return path

    def close(self) -> None:
        """Remove each worktree; then this run's own paths must be gone from `git worktree list` and its temporary
        directories from disk. Other worktrees (sibling lanes') are never compared."""
        for tmp, path in self._trees.values():
            if _git("worktree", "remove", "--force", path).exit != 0:
                shutil.rmtree(path, ignore_errors=True)
                _git("worktree", "prune")
            shutil.rmtree(tmp, ignore_errors=True)
        listed = {os.path.realpath(line.removeprefix("worktree "))
                  for line in _git("worktree", "list", "--porcelain").stdout.splitlines() if line.startswith("worktree ")}
        left = [path for tmp, path in self._trees.values() if os.path.realpath(path) in listed or os.path.exists(tmp)]
        self._trees.clear()
        if left:
            raise Internal(f"a worktree was left: {', '.join(left)}")   # exit 2
```

- `_git_ok(*argv, env=None)` runs `_git` with the extra environment merged and raises `Internal("git <argv>: exit <n>: <first stderr line>")` on a non-zero exit.
- `realpath` is used on both sides because macOS's temporary directory sits behind `/private`.
- A SIGTERM raises `Terminated` (`term.install`), so `__exit__` runs.

**`run_py(e, side, args, trees: Worktrees, rev: str)`:** `wt = trees.at(rev, e.changes)`. The rest is as in the first note, with `timeout_s=seconds(e.timeout)`. The report file stays outside the worktree.

**`parse_patch`** (exact text; #9b's plant refers to it):

```python
_HUNK = re.compile(r"^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@")
_SKIP = ("diff --git ", "index ", "old mode ", "new mode ")
_UNSUPPORTED = ("new file mode ", "deleted file mode ", "rename from ", "rename to ", "similarity index ",
                "copy from ", "copy to ", "Binary files ", "GIT binary patch", "\\ No newline")


def parse_patch(text: str) -> list[tuple[str, list[tuple[str, str]]]]:
    """A unified diff (git diff's output included) as [(path, [(old block, new block), ...])]. Hunk start lines are
    ignored: each hunk is placed by its old block occurring exactly once; its counts must match its body."""
    files: list[tuple[str, list[tuple[str, str]]]] = []
    lines = text.splitlines(keepends=True)
    i = 0
    while i < len(lines):
        line = lines[i]
        if line.startswith(_SKIP):
            i += 1
        elif line.startswith(_UNSUPPORTED):
            raise Refused(f"--patch: {line.strip()!r} is not supported: a plant edits existing files in place")
        elif line.startswith("--- "):
            if i + 1 >= len(lines) or not lines[i + 1].startswith("+++ "):
                raise Refused("--patch: a --- line without its +++ line")
            files.append((_patch_path(line[4:], lines[i + 1][4:]), []))
            i += 2
        elif m := _HUNK.match(line):
            if not files:
                raise Refused("--patch: a hunk before any file header")
            old, new, i = _hunk(lines, i + 1, int(m[1] or 1), int(m[2] or 1))
            files[-1][1].append((old, new))
        elif line.strip():
            raise Refused(f"--patch: unexpected line {line.rstrip()!r}")
        else:
            i += 1
    if not files or any(not hunks for _, hunks in files):
        raise Refused("--patch: no file with a hunk")
    return files


def _patch_path(old: str, new: str) -> str:
    old, new = old.rstrip("\n").split("\t")[0], new.rstrip("\n").split("\t")[0]
    if old.startswith('"') or new.startswith('"'):
        raise Refused("--patch: a quoted path is not supported")
    if "/dev/null" in (old, new):
        raise Refused("--patch: a plant edits existing files in place")
    if old.startswith("a/") and new.startswith("b/"):
        old, new = old[2:], new[2:]
    if old != new:
        raise Refused(f"--patch: --- {old} and +++ {new} name different files")
    if not old or os.path.isabs(old) or ".." in old.split("/"):
        raise Refused(f"--patch: {old!r} is not a repo-relative path")
    return old


def _hunk(lines: list[str], i: int, old_n: int, new_n: int) -> tuple[str, str, int]:
    old: list[str] = []
    new: list[str] = []
    while len(old) < old_n or len(new) < new_n:
        if i >= len(lines):
            raise Refused("--patch: a hunk shorter than its header")
        line = lines[i]
        tag, body = (" ", line) if line == "\n" else (line[0], line[1:])   # an editor may strip a blank context line
        if tag == " ":
            old.append(body)
            new.append(body)
        elif tag == "-":
            old.append(body)
        elif tag == "+":
            new.append(body)
        else:
            raise Refused(f"--patch: a hunk line starting {tag!r}")
        i += 1
    if len(old) != old_n or len(new) != new_n:
        raise Refused("--patch: a hunk's lines do not match its header")
    if not old:
        raise Refused("--patch: a hunk with no old line cannot be placed")
    return "".join(old), "".join(new), i
```

- A file may appear in several `---`/`+++` pairs. `changes_of` merges the hunks of one path in order before calling `apply`.
- In single and plain mode, every path must be tracked and equal to HEAD. In `--expect`, every path must exist at each revision.

**`_entry_timeout(value: object, default: str) -> str`:**
- missing → `default` (the run's `--timeout`);
- a `str` → `seconds(value)` checks it (a Go duration, positive) and the string is kept;
- an `int` or `float` that is not a `bool` and is greater than 0 → `f"{value}s"`;
- anything else → `Refused(f"batch entry {n}: timeout {value!r} is not a Go duration or positive seconds")`.

**`read_batch(path, expect, default_timeout)`:** as in the first note, with three changes:
- `timeout` is allowed in both modes and resolved by `_entry_timeout`;
- the value types become: `expect` a dict of two outcome words; `control` the literal `true`; `timeout` a string or a positive number; everything else a string;
- in single mode, `Entry.timeout` is `--timeout`.

**`expect(entries, args)`:**

```python
with Worktrees() as trees:
    for entry in entries:
        got = []
        for side, rev, tgt, want in (("old", entry.old_rev, entry.old, entry.expect[0]),
                                     ("new", entry.new_rev, entry.new, entry.expect[1])):
            wt = trees.at(rev, entry.changes)
            outcome = run_go(entry, tgt, args, scratch, wt)[0] if not tgt.py else run_py(entry, tgt, args, trees, rev)[0]
            got.append(outcome)
            _print([f"entry {entry.id} {side} {rev[:12]}: {outcome} (expected {want})"])
        outcomes[entry.id] = tuple(got)
    differing = [e.id for e in entries if outcomes[e.id] != e.expect]
```

Each entry's header prints `timeout: <entry.timeout>`. Plain `--batch` prints the same `timeout:` line per entry.

### A.3 Counter-examples (replaces A.3's table; the fixtures and the bootstrap witness are unchanged)

**How the plants are run:** with `python3.15 -I -B gate/bench/mutate.py --py <id> --file gate/bench/mutate.py --find <f> --replace <r>`, or with `--patch <file>` where noted. Each runs from the commit that adds the feature, on the worktree copy (§7.7).

**Patch P-TOP** (used by #3, #7 and #8):

```diff
--- a/gate/bench/mutate.py
+++ b/gate/bench/mutate.py
@@ -1,3 +1,3 @@
         n = len(runs(f"{probe.stdout}\n{probe.stderr}", side))
-        if n != 1:
+        if n > 1:
             return "invalid", probe, [f"probe: {n} subtests run as {side.full}"]
@@ -1,3 +1,3 @@
         return "timeout"
-    if not t.levels:
+    if True:
         if re.search(rf"^--- FAIL: {re.escape(t.test)} \(", text, re.MULTILINE):
```

Start lines are ignored; the counts (3/3) are checked.

| # | Test | Asserts | Feature-off plant | Red at |
|---|---|---|---|---|
| 1 | `Subtests.test_a_subtest_failing_against_the_plant_is_red_while_its_sibling_passes` | `--subtest F3`: exit 0; `ran:` lines == [`ran: TestTable/F3`]; `outcome: red` | `rf"^\s*--- FAIL: {name} \("` → `rf"^--- FAIL: {name} \("` | `assertEqual(code, 0)` |
| 2 | `Subtests.test_a_subtest_passing_while_its_sibling_fails_is_green` | `--subtest F30`: `outcome: green` | `rf"^\s*--- PASS: {name} \("` → `rf"^--- PASS: {name} \("` | the last line |
| 3 | `Subtests.test_a_subtest_that_does_not_exist_is_invalid` | `--subtest NoSuch`: `probe: 0 subtests run as TestTable/NoSuch`, `outcome: invalid` | `--patch` P-TOP (both hunks are needed: the probe alone already returns invalid) | the last line (top-level green) |
| 4 | `Subtests.test_a_name_with_a_slash_spaces_or_metacharacters_runs_exactly_itself` | `"E-SYN-001/not a participle error"`: red, `ran:` == [`TestTable/E-SYN-001/not_a_participle_error`]; `"x+y (meta)"`: green, `ran:` == [`TestTable/x+y_(meta)`] | `return "".join("\\" + c if c in GO_META else c for c in text)` → `return text` | the meta subTest (probe 0 gives invalid) |
| 5 | `Subtests.test_f3_never_runs_its_sibling_f30` | `--subtest F3`: `ran:` lines == exactly [`ran: TestTable/F3`] | `parts.append(f"^{quote_meta(level)}{'(#[0-9]+)?' if probe else ''}$")` → the same without the final `$`. The probe still counts 1 (`runs` is anchored); the real run selects F3 and F30, and `ran()` (unanchored after the parent) lists both. | the `ran:` list |
| 6 | `Subtests.test_two_subtests_rewritten_to_one_name_are_invalid` | `--subtest dup`: `probe: 2 subtests run as TestTable/dup`, invalid | `        if n != 1:` → `        if n == 0:` | the last line (green) |
| 7 | `Subtests.test_a_parent_failing_before_t_run_is_invalid_never_red` | `--test TestSetup --subtest "after setup"`: invalid, exit 1 | `--patch` P-TOP (the probe gives n=0, passes `n > 1`, and the top-level FAIL reads red) | `assertEqual(code, 1)` |
| 8 | `Subtests.test_a_skipped_subtest_is_invalid` | `--test TestSkips --subtest skipped --replace "return b + a"`: invalid | `--patch` P-TOP (the probe gives 1; the top-level PASS reads green) | the last line |
| 9 | `Patch.test_a_two_hunk_patch_applies_each_hunk_once` | hand-written two hunks on `m/n.go` (`-\treturn a * b`/`+\treturn a / b`; `-\treturn -a`/`+\treturn a`), `--test TestNeg`: red; both hunks in the printed diff | `    for old, new in hunks:` → `    for old, new in hunks[:1]:` | the last line |
| 9b | `Patch.test_a_patch_from_git_diff_output_applies` | (1) `git clone -q <repo> <tmp>/clone`, with `timeout=`; (2) in the clone, replace `"\treturn -a\n"` with `"\treturn a\n"` in `m/n.go`; (3) write `git -C <clone> diff --no-color` stdout to `<tmp>/neg.diff`, with `timeout=`. Then `--patch <tmp>/neg.diff --test TestNeg`: red. The input holds `diff --git a/m/n.go b/m/n.go`, an `index ` line, `--- a/m/n.go`, `+++ b/m/n.go` and an `@@ … @@ func …` header, each asserted in `neg.diff` first. The original repo is never touched. | `    if old.startswith("a/") and new.startswith("b/"):` → `    if False:` (gives "a/m/n.go is not tracked", exit 2) | `assertEqual(code, 0)` |
| 10 | `Patch.test_a_hunk_matching_twice_or_never_or_a_dirty_file_or_a_bad_header_is_refused_before_any_build` | fake go, each subTest exit 2 with `error: refused:` and no go log: hunk `-\treturn a + a` (twice); `-\treturn a % b` (never); dirty `m/n.go`; header count 4 with a 3-line body; `--- a/m/n.go` with `+++ b/m/m.go` | `if count != 1:` → `if count > 1:` (the never subTest); `count == 0` (the twice subTest) | `assertEqual(code, 2)` |
| 11 | `PyMode.test_a_caught_python_mutation_is_red_and_a_missed_one_green` | as in the first note | as in the first note | as in the first note |
| 12 | `PyMode.test_a_broken_import_a_skip_or_zero_tests_is_invalid` | as in the first note | as in the first note | as in the first note |
| 13 | `PyMode.test_a_test_running_git_ls_files_reads_as_in_the_tree` | as in the first note | after `_git_ok(*NO_HOOKS, "worktree", "add", "--detach", path, rev)`, add the line `os.remove(os.path.join(path, ".git"))` | the last line |
| 13a | `PyMode.test_a_plant_elsewhere_leaves_git_diff_and_status_clean` | as in the first note | `            _git_ok("-C", path, *NO_HOOKS, "commit", "-q", "-m", "mutate.py plant", env=THROWAWAY)` → `            pass` | the last line |
| 14 | `PyMode.test_a_test_waiting_past_the_timeout_is_timeout` | as in the first note | as in the first note | as in the first note |
| 14b | `Batch.test_an_entry_timeout_replaces_the_runs_timeout_for_that_entry` | `--batch` with `--timeout 5s` and one entry `{"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestHang", "timeout": "2s"}`: the output holds `timeout: 2s` and `entry 1: timeout`. A second entry `"timeout": 3` (seconds) prints `timeout: 3s`. | `    timeout=_entry_timeout(entry.get("timeout"), default_timeout),` → `    timeout=default_timeout,` (prints `timeout: 5s`) | `assertIn("timeout: 2s", out)` |
| 14c | `Refusals.test_a_bad_entry_timeout_is_refused_before_any_build` | fake go; entries with `"timeout"` set to `"0s"`, `"soon"`, `-1` or `true`: each exit 2, `error: refused:`, no go log | in `_entry_timeout`, `if isinstance(value, str):\n        seconds(value)` → `if isinstance(value, str):\n        pass` (`"soon"` then reaches go and reads invalid, exit 1) | `assertEqual(code, 2)` |
| 15 | `PyMode.test_a_python_patch_is_applied_in_the_copy` | as in the first note | `        for change in changes:` (in `Worktrees.at`) → `        for change in changes[:0]:` | the last line |
| 16 | `PyMode.test_a_py_run_leaves_the_tree_and_the_worktree_list_as_they_were` | after #11's run, in the **isolated fixture repo**: tracked bytes and mtimes equal, and the whole `git worktree list --porcelain` equal to its value before the run (siblings cannot exist there) | `            if _git("worktree", "remove", "--force", path).exit != 0:` → `            if False:` (the own path stays listed, so `close()` raises; exit 2) | `assertEqual(code, 0)` |
| 17 | `Expect.test_an_entry_off_its_expectation_exits_1_naming_it_and_the_control_reads_the_real_code` | as in the first note. Entries A and B (plants) run before control C at the same revisions, so the run also proves the reset between entries. | (a) `    differing = [e.id for e in entries if outcomes[e.id] != e.expect]` → `    differing = []`; (b) `            _git_ok("-C", path, *NO_HOOKS, "checkout", "-q", "--force", "--detach", rev)` → `            pass` (C then reads A's plant: red at new, so the differing list becomes `B, C, D`) | `assertEqual(code, 1)` / the `differing: B, D` line |
| 17a | as in the first note | as in the first note | as in the first note | as in the first note |
| 18 | as in the first note; adds a `timeout` key in an `--expect` batch as accepted | as in the first note | as in the first note | as in the first note |
| 19 | as in the first note | as in the first note | as in the first note | as in the first note |

The plant texts are exact for the code in A.2. If the builder's code differs, the plant keeps the same intent, and the record quotes the actual find and replace or patch.

---

### G2-147 — `tools/archcheck/archcheck_test.go:548-564` (replaces the section)

1. **Verdict: accepted.**
   - `exec.Command("go", "build", …)` (`:550`) and `exec.Command(bin, "-tests", list)` (`:557`) have no ceiling (testing.md rule 4).
   - The proposed `runCLI` swap is rejected. TestCLI already covers the in-process case (`:700-705`), so the swap would remove the only test of `main.go:1754`'s wiring.
2. **Kind: K2e.**
3. **Change:** as in the first note: `exec.CommandContext` under `context.WithTimeout(t.Context(), 5*time.Minute)`, a `ctx.Err()` check that names the ceiling, and `"context"` and `"time"` added to the imports.
4. **Proof (named deviation D-1 from §7.2's "an AST check").**
   - The command: `git grep -nE '(^|[^[:alnum:]_.])exec\.Command\(' -- 'tools/archcheck/*_test.go' 'tools/astequal/*_test.go'`.
   - Old tree: `archcheck_test.go:550`, `:557`, `astequal_test.go:221` (red). New tree: no line (green).
   - **Why a grep instead of an AST check:** a Go AST program is a `.go` file. Committed as evidence under `docs/`, it would sit inside the module, where the `gofmt` row (scope `**/*.go`) and the Go build rows would load it. Python's standard library has no Go parser.
   - **Why the grep is sound on these files:**
     - Both files import `os/exec` without an alias or a dot import (`archcheck_test.go:8`, `astequal_test.go:6`). So every call to the function appears as the text `exec.Command(`.
     - The pattern requires `(` right after `Command`, so it never matches `exec.CommandContext(`.
     - The leading class excludes a longer selector such as `x.exec.Command(`.
     - The only `exec.` texts in both packages' test files are `:550`, `:557`, `:560` (`exec.ExitError`) and `astequal_test.go:221`. So no comment, string literal, method value (`exec.Command` without a call) or `exec.Cmd{}` literal can give a false hit or hide a miss.
     - The other test files of both packages (`breakit_test.go`, `breakit_s11c2_test.go`) contain no `exec.`.
5. **Commit:** W0-7.

### G2-149 — `archcheck_test.go:1592-1633` (replaces item 3)

3. **Change:**
   - The case struct gains `verdict int`. The existing cases get `verdict`: `-cases` 0, `-tests with a finding` 1, `-tests clean` 0.
   - Two cases are added, with exact bytes:
     ```go
     {"normal mode with a finding", 1, func(t *testing.T) (string, []string) {
         root := cliRoot(t, nil, nil, nil, map[string]string{"internal/event/planted.go": "package event\n\nimport _ \"" + mod + "internal/model\"\n"})
         list := filepath.Join(t.TempDir(), "files.nul")
         if err := os.WriteFile(list, []byte("internal/event/planted.go\x00"), 0o644); err != nil {
             t.Fatal(err)
         }
         return root, []string{list}
     }},
     {"normal mode clean", 0, func(t *testing.T) (string, []string) {
         root := cliRoot(t, nil, nil, nil, map[string]string{"internal/model/x.go": "package model\n"})
         list := filepath.Join(t.TempDir(), "files.nul")
         if err := os.WriteFile(list, []byte("internal/model/x.go\x00"), 0o644); err != nil {
             t.Fatal(err)
         }
         return root, []string{list}
     }},
     ```
   - Byte by byte:
     - The clean case's list file is the 20 bytes `internal/model/x.go` followed by one NUL (0x00), with no newline. Its fixture file is the 14 bytes `package model` followed by one LF (0x0A).
     - The finding case's list file is the 26 bytes `internal/event/planted.go` followed by one NUL. Its fixture is `package event`, LF, LF, `import _ "github.com/vorsar/cep/internal/model"`, LF (the `mod` const at `archcheck_test.go:77`).
   - The finding case's root and source are TestCLI's "normal mode: a finding after failed:" case (`:715-721`, exit 1 with an OK writer).
   - Each subtest first runs the witness: `var ok strings.Builder; if code := cli(root, args, &ok); code != c.verdict { t.Fatalf("witness: cli %v: exit %d, want %d; output %q", args, code, c.verdict, ok.String()) }`. Then the existing failing-writer check runs, wanting 2.
   - Items 1, 2, 4 and 5 are unchanged: the two `--expect` entries plus a control, and the `load.py` 50/50 run.

### G2-155 — `astequal_test.go:219` (replaces item 4)

4. **Proof:** deviation D-1, with the same command, reasons and soundness argument as G2-147. Old tree: `astequal_test.go:221` (red). New tree: no line. The other items are unchanged.
