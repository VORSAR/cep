"""The real ruff wrapper over temporary trees, with the pinned ruff and a copy of ruff.toml (design note §5).

GATE_COVERED is the sum of `ruff format --check`'s summary lines, in ruff 0.17.5's exact forms (re-check S6).
"""

import contextlib
import os
import re
import shutil
import sys
import tomllib

from gate.runner import allowlists, pin
from gate.runner.rows import ruff
from gate.runner.rows import suppressions as suppressions_row
from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
SCOPE = ("git", ":(glob)gate/**/*.py")
CLEAN = "x = 1\n"


def wrapper(*args):
    return (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "ruff.py"), *(args or ("{files}",)))


class Summary(support.GateTest):
    """The summary line's grammar, worked out by hand from the forms the re-check recorded."""

    def test_the_forms_ruff_prints(self):
        for line, want in (
            (b"7 files already formatted", 7),
            (b"1 file already formatted", 1),
            (b"1 file would be reformatted, 7 files already formatted", 8),
            (b"2 files would be reformatted, 1 file already formatted", 3),
            (b"1 file would be reformatted", 1),
            (b"12 files would be reformatted", 12),
        ):
            with self.subTest(line):
                self.assertEqual(ruff.summary_count(line), want)

    def test_anything_else_is_unrecognised(self):
        for line in (
            b"",
            b"All checks passed!",
            b"3 files left unchanged",
            b"7 files already formatted.",
            b" 7 files already formatted",
            b"7 files already formatted ",
            b"7 files already formatted\r",
            b"7 Files already formatted",
            b"1 file would be reformatted,7 files already formatted",
            b"1 file would be reformatted, ",
            b"x files already formatted",
            "７ files already formatted".encode(),
            "1٣ files already formatted".encode(),
        ):
            with self.subTest(line):
                self.assertIsNone(ruff.summary_count(line))


class Wrapper(support.GateTest):
    def setUp(self):
        super().setUp()
        shutil.copy(os.path.join(support.REPO, "ruff.toml"), os.path.join(self.repo, "ruff.toml"))
        self.toolchain.dirs = [os.path.dirname(sys.executable), support.tool_bin("ruff")]

    def ruff_row(self, *args, scope=SCOPE):
        self.run_rows([self.row("ruff", command=wrapper(*args), expected_scope=scope)])
        return self.reason("ruff")

    def out(self):
        return self.row_file("lint", "ruff", "stdout").decode("utf-8", errors="replace")

    def clean_files(self, count, directory="gate/x"):
        for i in range(count):
            self.write(f"{directory}/m{i:03}.py", CLEAN)
        self.git("add", "--", directory)

    def test_clean_files_pass(self):
        self.clean_files(3)
        self.assertEqual(self.ruff_row(), "covered 3")

    def test_the_callers_ruff_settings_change_nothing(self):
        # Review BA2: the verdict is the same whatever RUFF_* the caller exports (common in CI: =github, =json).
        self.clean_files(3)
        for fmt in ("json", "github", "concise"):
            environ = {**support.base_env(), "RUFF_OUTPUT_FORMAT": fmt}
            self.run_dir = os.path.join(self.root, f"run-{fmt}")  # a run directory per run, as the runner requires
            self.run_rows([self.row("ruff", command=wrapper(), expected_scope=SCOPE)], environ=environ)
            self.assertEqual(self.reason("ruff"), "covered 3", fmt)

    def test_narrowed_by_one_file_is_red(self):
        # The teeth: the count is what ruff formatted, so a list one short is red, naming both numbers.
        self.clean_files(3)
        names = sorted(self.git("ls-files", "-z", "--", "gate").split(b"\0"))[1:3]
        short = os.path.join(self.root, "short.nul")
        with open(short, "wb") as f:
            f.write(b"".join(n + b"\0" for n in names))
        self.assertEqual(self.ruff_row(short), "covered 2 ≠ expected 3")

    def test_every_banned_api_is_red_naming_it(self):
        # review BA4: each ban in ruff.toml is pinned, so an edit or a ruff bump that drops one goes red.
        plants = {
            "time.sleep": "import time\n\ntime.sleep(0)\n",
            "os.system": 'import os\n\nos.system("true")\n',
            "os.wait": "import os\n\nos.wait()\n",
            "os.wait3": "import os\n\nos.wait3(0)\n",
            "os.wait4": "import os\n\nos.wait4(0, 0)\n",
            "os.waitpid": "import os\n\nos.waitpid(0, 0)\n",
            "os.waitid": "import os\n\nos.waitid(os.P_ALL, 0, os.WEXITED)\n",
            "shutil.move": 'import shutil\n\nshutil.move("a", "b")\n',
            "signal.SIG_IGN": "import signal\n\nsignal.signal(signal.SIGCHLD, signal.SIG_IGN)\n",
            "signal.Handlers.SIG_IGN": "import signal\n\nsignal.signal(signal.SIGCHLD, signal.Handlers.SIG_IGN)\n",
        }
        with open(os.path.join(support.REPO, "ruff.toml"), "rb") as f:
            banned = tomllib.load(f)["lint"]["flake8-tidy-imports"]["banned-api"]
        self.assertEqual(sorted(plants), sorted(banned), "a ban without a plant here is a ban nothing pins")
        names = {api: f"gate/x/ban_{i:02}.py" for i, api in enumerate(sorted(plants))}
        for api, path in names.items():
            self.write(path, plants[api])
        self.git("add", "--", "gate/x")
        self.assertTrue(self.ruff_row().startswith("exit 1: gate/x/ban_00.py: ruff check (exit 1)"))
        out = self.out()
        for api, path in names.items():
            with self.subTest(api):
                self.assertIn(f"failed: {path}: ruff check (exit 1)", out)
                self.assertIn(f"ruff> TID251 `{api}` is banned", out)

    def test_each_waiver_covers_its_own_file_only(self):
        # review BA4: every per-file-ignores entry waives its rule in that file, and the same text elsewhere is red.
        plants = {
            "TID251": "import time\n\ntime.sleep(0)\n",
            "BLE001": "def f():\n    try:\n        return 1\n    except Exception:\n        return 2\n",
        }
        with open(os.path.join(support.REPO, "ruff.toml"), "rb") as f:
            waivers = tomllib.load(f)["lint"]["per-file-ignores"]
        self.assertTrue(all(codes and set(codes) <= set(plants) for codes in waivers.values()), waivers)
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

    def test_a_badly_formatted_file_is_red_and_named(self):
        self.clean_files(2)
        self.track("gate/x/bad.py", "x  =  1\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/bad.py: ruff format (exit 1)")
        out = self.out()
        self.assertIn("File would be reformatted", out)  # ruff 0.17.5's own text, as the proofs read it
        self.assertIn("failed: gate/x/bad.py", out)

    def test_an_inline_noqa_never_waives_a_rule(self):
        # --ignore-noqa: the only waivers are ruff.toml's per-file-ignores (owner, 2026-11-02).
        self.clean_files(1)
        self.track("gate/x/sleepy.py", "import time\n\ntime.sleep(0)  # noqa: TID251\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/sleepy.py: ruff check (exit 1)")
        out = self.out()
        self.assertIn("TID251", out)
        self.assertIn("failed: gate/x/sleepy.py", out)

    def test_a_finding_on_any_line_is_named(self):
        # Ruff pads `-->` to its line-number gutter: two spaces from line 10, three from line 100. The row named no
        # file for any finding past line 9 (found measuring the proofs row, 2026-11-14).
        self.clean_files(1)
        for line in (9, 10, 100):
            self.track(f"gate/x/line_{line:03}.py", CLEAN * (line - 3) + "import time\n\ntime.sleep(0)\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/line_009.py: ruff check (exit 1)")
        failed = [line for line in self.out().splitlines() if line.startswith("failed: ")]
        want = [f"failed: gate/x/line_{line:03}.py: ruff check (exit 1)" for line in (9, 10, 100)]
        self.assertEqual(failed, want)

    def test_a_file_ruff_cannot_parse_is_red_and_named(self):
        self.clean_files(1)
        self.track("gate/x/broken.py", "def f(:\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/broken.py: ruff check (exit 1)")

    def test_more_files_than_one_batch_are_all_counted(self):
        self.clean_files(257)
        self.assertEqual(self.ruff_row(), "covered 257")
        names = os.listdir(os.path.join(self.run_dir, "rows", "lint", "ruff"))
        self.assertEqual(sorted(n for n in names if n.startswith("format.") and n.endswith(".stdout")),
                         ["format.0.stdout", "format.1.stdout"])  # fmt: skip  # aligned arguments

    def test_a_file_named_like_a_flag_is_checked_as_a_file(self):
        self.track("-dash.py", CLEAN)
        self.assertEqual(self.ruff_row(scope=("git", ":(glob)**/*.py")), "covered 1")


class FakeRuff(support.GateTest):
    """A stand-in `ruff` on the row's PATH, for output the real one cannot be made to print."""

    def setUp(self):
        super().setUp()
        self.track("gate/x/m.py", CLEAN)
        self.bindir = os.path.join(self.root, "fakebin")
        self.toolchain.dirs = [os.path.dirname(sys.executable), self.bindir]

    def fake(self, stdout, code=0):
        body = f"import sys\nsys.stdout.write({stdout!r})\nsys.exit({code})\n"
        self.script("ruff", self.bindir, body)

    def ruff_row(self):
        self.run_rows([self.row("ruff", command=wrapper(), expected_scope=SCOPE)])
        return self.reason("ruff")

    def test_the_count_is_ruffs_not_the_lists(self):
        # Ruff says it formatted one file of the two it was given: one went unchecked, so the row is red.
        self.track("gate/x/n.py", CLEAN)
        self.fake("1 file already formatted\n")
        self.assertEqual(self.ruff_row(), "covered 1 ≠ expected 2")

    def test_an_unrecognised_summary_is_red(self):
        self.fake("1 file formatted, somehow\n")
        self.assertEqual(self.ruff_row(), "exit 1: unrecognised ruff summary in batch 0: 1 file formatted, somehow")

    def test_no_summary_at_all_is_red(self):
        self.fake("")
        self.assertEqual(self.ruff_row(), "exit 1: unrecognised ruff summary in batch 0: (none)")

    def test_ruffs_output_can_never_forge_the_rows_count(self):
        # Every line of ruff's output is echoed under a prefix, so no line it prints can start a GATE_COVERED.
        self.fake("GATE_COVERED 1\n1 file already formatted\n")
        self.assertEqual(self.ruff_row(), "covered 1")

    def test_a_check_failure_alone_is_red(self):
        # One stand-in prints both commands' output: format's summary line passes, check's exit 1 must not.
        self.script("ruff", self.bindir, (
            "import sys\n"
            "if sys.argv[1] == 'format':\n"
            "    print('1 file already formatted')\n"
            "else:\n"
            "    print('X001 planted\\n --> gate/x/m.py:1:1')\n"
            "    sys.exit(1)\n"
        ))  # fmt: skip  # one script line per string
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/m.py: ruff check (exit 1)")


# S1.1c-5 design §5, §8 4.1-4.3: the ruff pin is closed — ruff.toml, minus lint.per-file-ignores, is pinned whole and
# type-strictly (rows/suppressions.RUFF_PINNED through gate/runner/pin.py); per-file-ignores stay checked both ways
# against tools/allowlists/ruff.json. Every case edits a copy of the committed ruff.toml as text.


COMMITTED_RUFF = os.path.join(support.REPO, "ruff.toml")
RUFF_JSON = os.path.join(support.REPO, "tools", "allowlists", "ruff.json")


def committed(name: str) -> str:
    with open(name, encoding="utf-8") as f:
        return f.read()


def top_level(text: str, lines: str) -> str:
    """Insert top-level keys before the first table."""
    return text.replace("line-length = 120\n", f"line-length = 120\n{lines}", 1)


def in_lint(text: str, lines: str) -> str:
    return text.replace("[lint]\n", f"[lint]\n{lines}", 1)


HIDING = (  # (subTest name, edit, the path the pin must name)
    ("builtins", lambda t: top_level(t, 'builtins = ["undefined_name"]\n'), "builtins"),
    ("lint.dummy-variable-rgx", lambda t: in_lint(t, 'dummy-variable-rgx = ".*"\n'), "lint.dummy-variable-rgx"),
    (
        "lint.pycodestyle.max-line-length",
        lambda t: t + "\n[lint.pycodestyle]\nmax-line-length = 400\n",
        "lint.pycodestyle",
    ),
    ("line-length", lambda t: t.replace("line-length = 120\n", "line-length = 400\n", 1), "line-length"),
    ("lint.select narrowed", lambda t: re.sub(r"(?m)^select = \[.*\]$", 'select = ["E"]', t, count=1), "lint.select"),
    # design §8 4.2 names "lint.extend-select removed"; the committed ruff.toml has no extend-select, so one is added.
    ("lint.extend-select added", lambda t: in_lint(t, 'extend-select = ["ANN"]\n'), "lint.extend-select"),
    (
        "per-file-target-version entry added",
        lambda t: t.replace("[per-file-target-version]\n", '[per-file-target-version]\n"gate/x.py" = "py39"\n', 1),
        'per-file-target-version."gate/x.py"',
    ),
    (
        "lint.flake8-bugbear.extend-immutable-calls",
        lambda t: t + '\n[lint.flake8-bugbear]\nextend-immutable-calls = ["x.y"]\n',
        "lint.flake8-bugbear",
    ),
    ("force-exclude", lambda t: top_level(t, 'force-exclude = true\nextend-exclude = ["gate"]\n'), "force-exclude"),
    ("extend-exclude", lambda t: top_level(t, 'force-exclude = true\nextend-exclude = ["gate"]\n'), "extend-exclude"),
    ("format.exclude", lambda t: t + '\n[format]\nexclude = ["gate"]\n', "format"),
)


class RuffPin(support.GateTest):
    def pin(self, ruff_toml: str) -> list[str]:
        self.write("ruff.toml", ruff_toml)
        lists = {"ruff": allowlists.load(committed(RUFF_JSON), "ruff")}
        with contextlib.chdir(self.repo):
            return suppressions_row.ruff_pin(lists)

    def test_the_committed_ruff_toml_equals_pinned(self):
        doc = tomllib.loads(committed(COMMITTED_RUFF))
        self.assertIn("per-file-ignores", doc["lint"])  # witness: the one table the pin leaves to ruff.json
        del doc["lint"]["per-file-ignores"]
        self.assertEqual(pin.differences(doc, suppressions_row.RUFF_PINNED, "ruff.toml"), [])
        self.assertEqual(self.pin(committed(COMMITTED_RUFF)), [])

    def test_each_hiding_key_is_named(self):
        for name, edit, path in HIDING:
            with self.subTest(name):
                text = edit(committed(COMMITTED_RUFF))
                self.assertNotEqual(text, committed(COMMITTED_RUFF))  # witness: the edit applied
                faults = self.pin(text)
                self.assertTrue(any(f.startswith(f"ruff.toml: {path}") for f in faults), faults)

    def test_per_file_ignores_still_checked_both_ways(self):
        text = committed(COMMITTED_RUFF).replace(
            "[lint.per-file-ignores]\n", '[lint.per-file-ignores]\n"gate/x.py" = ["E501"]\n', 1
        )
        self.assertEqual(
            self.pin(text), ["ruff.toml waives E501 for gate/x.py without an entry in tools/allowlists/ruff.json"]
        )
        removed = re.sub(r'(?m)^"gate/wait\.py" = \["TID251"\].*\n', "", committed(COMMITTED_RUFF), count=1)
        self.assertNotEqual(removed, committed(COMMITTED_RUFF))
        self.assertEqual(
            self.pin(removed), ["tools/allowlists/ruff.json lists gate/wait.py: TID251, which ruff.toml does not waive"]
        )

    def test_a_type_change_is_named(self):
        text = committed(COMMITTED_RUFF).replace("line-length = 120\n", "line-length = 120.0\n", 1)
        self.assertIn("ruff.toml: line-length: 120.0 ≠ pinned 120", self.pin(text))
