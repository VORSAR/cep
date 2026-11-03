"""The real ruff wrapper over temporary trees, with the pinned ruff and a copy of ruff.toml (design note §5).

GATE_COVERED is the sum of `ruff format --check`'s summary lines, in ruff 0.17.3's exact forms (re-check S6).
"""

import os
import shutil
import sys

from gate.runner.rows import ruff
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

    def test_a_badly_formatted_file_is_red_and_named(self):
        self.clean_files(2)
        self.track("gate/x/bad.py", "x  =  1\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/bad.py: ruff format (exit 1)")
        out = self.out()
        self.assertIn("File would be reformatted", out)  # ruff 0.17.3's own text, as the proofs read it
        self.assertIn("failed: gate/x/bad.py", out)

    def test_an_inline_noqa_never_waives_a_rule(self):
        # --ignore-noqa: the only waivers are ruff.toml's per-file-ignores (owner, 2026-11-02).
        self.clean_files(1)
        self.track("gate/x/sleepy.py", "import time\n\ntime.sleep(0)  # noqa: TID251\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/sleepy.py: ruff check (exit 1)")
        out = self.out()
        self.assertIn("TID251", out)
        self.assertIn("failed: gate/x/sleepy.py", out)

    def test_a_file_ruff_cannot_parse_is_red_and_named(self):
        self.clean_files(1)
        self.track("gate/x/broken.py", "def f(:\n")
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/broken.py: ruff check (exit 1)")

    def test_more_files_than_one_batch_are_all_counted(self):
        self.clean_files(257)
        self.assertEqual(self.ruff_row(), "covered 257")
        names = os.listdir(os.path.join(self.run_dir, "rows", "lint", "ruff"))
        self.assertEqual(sorted(n for n in names if n.startswith("format.") and n.endswith(".stdout")),
                         ["format.0.stdout", "format.1.stdout"])  # fmt: skip

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
        ))  # fmt: skip
        self.assertEqual(self.ruff_row(), "exit 1: gate/x/m.py: ruff check (exit 1)")
