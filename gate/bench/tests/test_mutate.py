"""mutate.py: one mutation committed in a throwaway worktree of HEAD, its one designated test run there, and a tracked
file never written."""

import contextlib
import faulthandler
import json
import os
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import unittest

from gate import wait
from gate.bench.tests import support

MODULE = "module example.com/m\n\ngo 1.27\n"

SOURCE = """\
package m

func Add(a, b int) int {
\treturn a + b
}
"""

TESTS = """\
package m

import (
\t"context"
\t"fmt"
\t"os"
\t"syscall"
\t"testing"
\t"time"
)

func TestAdd(t *testing.T) {
\tif Add(2, 3) != 5 {
\t\tt.Fatal("Add(2, 3) != 5")
\t}
}

func TestWeak(t *testing.T) {
\tif Add(0, 0) != 0 {
\t\tt.Fatal("Add(0, 0) != 0")
\t}
}

func TestHang(t *testing.T) {
\tif Add(1, 1) != 2 {
\t\t<-make(chan struct{})
\t}
}

func TestDeadline(t *testing.T) {
\tctx, cancel := context.WithTimeout(context.Background(), time.Second)
\tdefer cancel()
\tdone := make(chan struct{})
\tgo func() {
\t\tif Add(2, 2) == 4 {
\t\t\tclose(done)
\t\t}
\t}()
\tselect {
\tcase <-done:
\tcase <-ctx.Done():
\t\tt.Fatal("no sum before the context deadline")
\t}
}

func TestHold(t *testing.T) {
\tfifo := os.Getenv("HOLD_FIFO")
\tif fifo == "" {
\t\tt.Skip("held only under the SIGTERM test")
\t}
\tif err := os.WriteFile(os.Getenv("HOLD_PGID"), []byte(fmt.Sprint(syscall.Getpgrp())), 0o644); err != nil {
\t\tt.Fatal(err)
\t}
\tf, err := os.OpenFile(fifo, os.O_WRONLY, 0)
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif _, err := f.Write([]byte("s")); err != nil {
\t\tt.Fatal(err)
\t}
\t<-make(chan struct{})
}

func TestTable(t *testing.T) {
\tcases := []struct {
\t\tname       string
\t\ta, b, want int
\t}{
\t\t{"F3", 2, 3, 5},
\t\t{"F30", 0, 0, 0},
\t\t{"E-SYN-001/not a participle error", 1, 2, 3},
\t\t{"x+y (meta)", 0, 0, 0},
\t\t{"dup", 0, 0, 0},
\t\t{"dup", 0, 0, 0},
\t\t{"case", 2, 3, 5},
\t\t{"case#1", 0, 0, 0},
\t}
\tfor _, c := range cases {
\t\tt.Run(c.name, func(t *testing.T) {
\t\t\tif Add(c.a, c.b) != c.want {
\t\t\t\tt.Fatalf("Add(%d, %d) != %d", c.a, c.b, c.want)
\t\t\t}
\t\t})
\t}
}

func TestSetup(t *testing.T) {
\tif Add(1, 1) != 2 {
\t\tt.Fatal("setup: Add(1, 1) != 2")
\t}
\tt.Run("after setup", func(t *testing.T) {})
}

func TestSkips(t *testing.T) {
\tt.Run("skipped", func(t *testing.T) {
\t\tif Add(2, 3) == 5 {
\t\t\tt.Skip("skipped while Add(2, 3) == 5")
\t\t}
\t})
}

func TestSpaces(t *testing.T) {
\tfor _, name := range []string{"nb\\u00a0sp", "em\\u2003sp"} {
\t\tt.Run(name, func(t *testing.T) {
\t\t\tif Add(2, 3) != 5 {
\t\t\t\tt.Fatal("Add(2, 3) != 5")
\t\t\t}
\t\t})
\t}
}

func TestData(t *testing.T) {
\tdata, err := os.ReadFile("testdata/want.txt")
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif got := fmt.Sprintln(Add(2, 3)); got != string(data) {
\t\tt.Fatalf("Add(2, 3) = %q, testdata/want.txt holds %q", got, data)
\t}
}

func TestGoData(t *testing.T) {
\tfor _, name := range []string{"testdata/want.go", "_data/want.go"} {
\t\tdata, err := os.ReadFile(name)
\t\tif err != nil {
\t\t\tt.Fatal(err)
\t\t}
\t\tif want := fmt.Sprintf("package want\\n\\nconst Sum = %d\\n", Add(2, 3)); string(data) != want {
\t\t\tt.Fatalf("%s holds %q, want %q", name, data, want)
\t\t}
\t}
}
"""

ADD_HEAD = "func TestAdd(t *testing.T) {\n"

IGNORED = """\
//go:build ignore

package m

func Ignored() int {
\treturn 1
}
"""

RACE_ONLY = """\
//go:build race

package m

func Raced() int {
\treturn 1
}
"""

UNUSED = """\
package u

func U() int {
\treturn 1
}
"""

NEG = """\
package m

func Mul(a, b int) int {
\treturn a * b
}

func Neg(a int) int {
\treturn -a
}

func Twice(a int) int {
\treturn a + a
}

func Double(a int) int {
\treturn a + a
}
"""

NEG_TESTS = """\
package m

import "testing"

func TestNeg(t *testing.T) {
\tif Neg(2) != -2 {
\t\tt.Fatal("Neg(2) != -2")
\t}
}

func TestNeg2(t *testing.T) {
\tif Neg2(2) != -2 {
\t\tt.Fatal("Neg2(2) != -2")
\t}
}

func TestSep(t *testing.T) {
\tif Sep(1) != 2 {
\t\tt.Fatal("Sep(1) != 2")
\t}
}
"""

TAIL = """\
package m

func Abs(a int) int {
\tif a < 0 {
\t\treturn -a
\t}
\treturn a
}

func Neg2(a int) int {
\treturn -a
}
"""

SEP_COMMENT = "// Sep: a form feed \x0c, a line separator \u2028, both inside one line."

SEP = f"""\
package m

{SEP_COMMENT}
func Sep(a int) int {{
\treturn a + 1
}}
"""

GO_DATA = "package want\n\nconst Sum = 5\n"

GUARD = """\
package g

import (
\t"testing"

\t"example.com/m/m"
)

func TestGuard(t *testing.T) {
\tif m.Add(2, 3) != 5 {
\t\tt.Fatal("m.Add(2, 3) != 5")
\t}
}
"""

FIND = "return a + b"
CAUGHT = "return a - b"
BROKEN = "return a +"
CAP_S = 60


class Base(unittest.TestCase):
    def setUp(self) -> None:
        faulthandler.dump_traceback_later(600, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = tmp.name
        self.repo = os.path.join(self.tmp, "repo")
        os.mkdir(self.repo)
        self.env = {**os.environ, **support.GIT_ENV}
        support.write(
            self.repo,
            {
                "go.mod": MODULE,
                "m/m.go": SOURCE,
                "m/n.go": NEG,
                "m/m_test.go": TESTS,
                "m/n_test.go": NEG_TESTS,
                "g/g_test.go": GUARD,
                "m/testdata/want.txt": "5\n",
                "m/ignored.go": IGNORED,
                "m/race.go": RACE_ONLY,
                "u/u.go": UNUSED,
                "m/tail.go": TAIL,
                "m/sep.go": SEP,
                "m/testdata/want.go": GO_DATA,
                "m/_data/want.go": GO_DATA,
            },
        )
        self.git("init", "-q")
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "c")
        self.tracked = os.path.join(self.repo, "m", "m.go")
        self.before = self.snapshot()
        self.addCleanup(self.assertUntouched, self.before)
        self.log, self.fake_env = support.logging_go(self.tmp, self.env)

    def git(self, *argv: str) -> str:
        return support.git(self.repo, self.env, *argv)

    def snapshot(self) -> dict[str, tuple[bytes, int]]:
        """The bytes and mtime of each source file a mutation may name."""
        names = ("m/m.go", "m/n.go", "m/testdata/want.txt")
        return {name: support.stamp(os.path.join(self.repo, name)) for name in names}

    def assertUntouched(self, before: dict[str, tuple[bytes, int]]) -> None:
        self.assertEqual(self.snapshot(), before, "a tracked file was written")

    def restore(self, name: str) -> None:
        """A test that dirtied a file itself puts HEAD's bytes and the first mtime back before the untouched check."""
        path = os.path.join(self.repo, name)
        data, mtime = self.before[name]
        with open(path, "wb") as f:
            f.write(data)
        os.utime(path, ns=(mtime, mtime))

    def mutate(self, *argv: str, env: dict[str, str] | None = None) -> tuple[int, str]:
        out_path = os.path.join(self.tmp, "mutate.stdout")
        return support.tool("mutate", list(argv), self.repo, self.env if env is None else env, out_path)

    def single(self, test: str, replace: str = CAUGHT, *extra: str) -> tuple[int, str]:
        return self.mutate("--file", "m/m.go", "--find", FIND, "--replace", replace, "--test", test, *extra)

    def refused(self, *argv: str) -> str:
        """Exit 2 with a refusal as the last line, and no go command run by this call (the fake go's log, emptied
        first, never written), so one subTest's go call never fails the next."""
        with contextlib.suppress(FileNotFoundError):
            os.remove(self.log)
        code, out = self.mutate(*argv, env=self.fake_env)
        self.assertEqual(code, 2, out)
        self.assertTrue(out.splitlines()[-1].startswith("error: refused: "), out)
        self.assertFalse(os.path.exists(self.log), "a go command ran before the refusal")
        return out

    def write_batch(self, *entries: dict[str, object]) -> str:
        """The entries in a batch file of their own (a name no other call of this test reuses)."""
        fd, path = tempfile.mkstemp(suffix=".json", dir=self.tmp)
        os.close(fd)
        with open(path, "w", encoding="utf-8") as f:
            json.dump(list(entries), f)
        return path

    def write_patch(self, text: str) -> str:
        """The diff in a file of its own (a name no other call of this test reuses)."""
        fd, path = tempfile.mkstemp(suffix=".diff", dir=self.tmp)
        os.close(fd)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def add_python(self) -> None:
        """The Python fixture beside the Go one, committed: pkg/ and a copy of the shim."""
        support.write(
            self.repo,
            {
                "pkg/__init__.py": "",
                "pkg/calc.py": PY_CALC,
                "pkg/tests/__init__.py": "",
                "pkg/tests/test_calc.py": PY_TESTS,
            },
        )
        os.makedirs(os.path.join(self.repo, "gate", "runner"))
        shutil.copyfile(
            os.path.join(support.REPO, "gate", "runner", "pyshim.py"),
            os.path.join(self.repo, "gate", "runner", "pyshim.py"),
        )
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "python")

    def dirty_go_tests(self) -> None:
        """An uncommitted edit failing TestAdd outright in m/m_test.go, put back at cleanup."""
        self.assertEqual(TESTS.count(ADD_HEAD), 1)
        self.addCleanup(support.write, self.repo, {"m/m_test.go": TESTS})
        dirty = TESTS.replace(ADD_HEAD, ADD_HEAD + '\tt.Fatal("an uncommitted edit")\n')
        support.write(self.repo, {"m/m_test.go": dirty})


class Outcomes(Base):
    def test_a_caught_mutation_is_red(self):
        code, out = self.single("TestAdd")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)
        self.assertIn("-\treturn a + b\n+\treturn a - b\n", out)
        self.assertIn("--- a/m/m.go\n+++ b/m/m.go\n", out)
        self.assertIn("test: TestAdd", out)
        self.assertIn(f"head: {self.git('rev-parse', 'HEAD')}", out)
        self.assertIn("go: go version go", out)

    def test_an_uncaught_mutation_is_green(self):
        code, out = self.single("TestWeak")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: green", out)

    def test_a_mutant_that_does_not_compile_is_invalid(self):
        code, out = self.single("TestAdd", BROKEN)
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)
        self.assertIn("m.go:5:1: syntax error", out)  # the tail of the build error, at the `}`

    def test_a_test_hanging_past_the_timeout_is_timeout(self):
        code, out = self.single("TestHang", CAUGHT, "--timeout", "2s")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: timeout", out)

    def test_a_failure_at_the_tests_own_context_deadline_is_red_not_timeout(self):
        code, out = self.single("TestDeadline", CAUGHT, "--timeout", "30s")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_exit_0_with_no_pass_line_for_the_test_is_invalid(self):
        code, out = self.single("TestNoSuchTest")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)


class OtherPackage(Base):
    """--pkg: the designated test lives in another package (a guard test of the mutated package's contract)."""

    def test_a_mutation_caught_by_a_test_in_another_package_is_red(self):
        code, out = self.single("TestGuard", CAUGHT, "--pkg", "g")
        self.assertEqual(code, 0, out)
        self.assertIn("package: ./g", out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_without_pkg_the_files_own_package_runs_and_finds_no_such_test(self):
        code, out = self.single("TestGuard")
        self.assertEqual(code, 1, out)
        self.assertIn("package: ./m", out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)

    def test_a_pkg_with_a_leading_dot_slash_is_printed_once(self):
        code, out = self.single("TestGuard", CAUGHT, "--pkg", "./g")
        self.assertEqual(code, 0, out)
        self.assertIn("package: ./g\n", out)
        self.assertNotIn("././", out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_batch_entry_takes_its_own_pkg(self):
        path = os.path.join(self.tmp, "batch.json")
        entries = [
            {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestGuard", "pkg": "g"},
            {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"},
        ]
        with open(path, "w", encoding="utf-8") as f:
            json.dump(entries, f)
        code, out = self.mutate("--batch", path)
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "counts: red 2, green 0, timeout 0, invalid 0", out)


class Refusals(Base):
    """Refused with exit 2 before any go command: a fake go on PATH records every call."""

    def test_witness_the_fake_go_records_a_call(self):
        code, out = self.mutate(
            "--file", "m/m.go", "--find", FIND, "--replace", CAUGHT, "--test", "TestAdd", env=self.fake_env
        )
        self.assertEqual(code, 1, out)  # the fake prints nothing: invalid
        self.assertTrue(os.path.exists(self.log), out)

    def test_a_find_occurring_twice_is_refused(self):
        out = self.refused("--file", "m/m.go", "--find", "int", "--replace", "uint", "--test", "TestAdd")
        self.assertIn("occurs 2 times", out)  # `b int) int`

    def test_an_absent_find_is_refused(self):
        out = self.refused("--file", "m/m.go", "--find", "return a * b", "--replace", "x", "--test", "TestAdd")
        self.assertIn("occurs 0 times", out)

    def test_a_dirty_file_is_refused(self):
        with open(self.tracked, "a", encoding="utf-8") as f:
            f.write("// dirty\n")
        with open(self.tracked, "rb") as f:
            dirty = f.read()
        self.addCleanup(self.restore, "m/m.go")
        mtime = os.stat(self.tracked).st_mtime_ns
        out = self.refused("--file", "m/m.go", "--find", FIND, "--replace", CAUGHT, "--test", "TestAdd")
        self.assertIn("differs from HEAD", out)
        with open(self.tracked, "rb") as f:
            self.assertEqual(f.read(), dirty)
        self.assertEqual(os.stat(self.tracked).st_mtime_ns, mtime)

    def test_an_untracked_file_is_refused(self):
        support.write(self.repo, {"m/u.go": "package m\n\nfunc U() int { return 1 }\n"})
        out = self.refused("--file", "m/u.go", "--find", "return 1", "--replace", "return 2", "--test", "TestAdd")
        self.assertIn("not tracked", out)

    def test_a_subtest_name_is_refused(self):
        out = self.refused("--file", "m/m.go", "--find", FIND, "--replace", CAUGHT, "--test", "TestAdd/sub")
        self.assertIn("TestAdd/sub", out)

    def test_a_pkg_that_is_not_a_tracked_directory_is_refused(self):
        for pkg, reason in (("nope", "not a directory"), ("../m", "repo-relative"), ("/abs", "repo-relative")):
            with self.subTest(pkg=pkg):
                out = self.refused(
                    "--file", "m/m.go", "--find", FIND, "--replace", CAUGHT, "--test", "TestAdd", "--pkg", pkg
                )
                self.assertIn(reason, out)

    def test_help_prints_the_usage_and_exits_0_before_any_go_command(self):
        code, out = self.mutate("--help", env=self.fake_env)
        self.assertEqual(code, 0, out)
        self.assertTrue(out.startswith("usage: gate/bench/mutate.py "), out)
        self.assertNotIn("error:", out)
        self.assertFalse(os.path.exists(self.log), "a go command ran")

    def test_a_batch_entry_with_an_extra_key_is_refused_before_any_build(self):
        good = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        path = os.path.join(self.tmp, "batch.json")
        with open(path, "w", encoding="utf-8") as f:
            json.dump([good, {**good, "extra": 1}], f)
        out = self.refused("--batch", path)
        self.assertIn("entry 2", out)

    def test_an_unknown_key_or_an_expect_key_in_a_plain_batch_is_refused(self):
        head = self.git("rev-parse", "HEAD")
        plant = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        sides = {"id": "A", "old_rev": head, "new_rev": head, "expect": {"old": "red", "new": "red"}}
        cases = (
            ("bogus", "--expect", {**plant, **sides, "bogus": "x"}, "unknown keys ['bogus']"),
            ("old_rev in a plain batch", "--batch", {**plant, "old_rev": head}, "an --expect key in a plain batch"),
            ("find with patch", "--batch", {**plant, "patch": NEG_PATCH}, "patch: not both"),
            ("test with py", "--batch", {**plant, "py": "pkg.tests.Calc"}, "test or py: not both"),
            ("control with find", "--expect", {**plant, **sides, "control": True}, "a control entry has no mutation"),
        )
        for name, mode, spec, fragment in cases:
            with self.subTest(name):
                out = self.refused(mode, self.write_batch(spec))
                self.assertIn(fragment, out.splitlines()[-1])
        with self.subTest("a timeout key in an --expect batch is accepted"):
            code, out = self.mutate(
                "--expect", self.write_batch({**plant, **sides, "timeout": "30s"}), env=self.fake_env
            )
            self.assertEqual(code, 1, out)  # the fake go prints nothing: invalid, off its expectation
            self.assertNotIn("error: refused", out)

    def test_a_bad_entry_timeout_is_refused_before_any_build(self):
        # No subTest: each failing subTest is a failure of its own, and --py's red is exactly one.
        for value in ("0s", "soon", -1, True):
            out = self.refused(
                "--batch",
                self.write_batch(
                    {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd", "timeout": value}
                ),
            )
            self.assertIn(f"timeout {value!r} ", out.splitlines()[-1], value)

    def test_a_plant_that_changes_nothing_is_refused_before_any_build(self):
        # No subTest: a plant reading every case wrong must stop this test at its first case (--py's one failure).
        context_only = "--- a/m/n.go\n+++ b/m/n.go\n@@ -7,3 +7,3 @@\n func Neg(a int) int {\n \treturn -a\n }\n"
        cases = (
            (["--file", "m/m.go", "--find", FIND, "--replace", FIND], "find equals replace"),
            (["--patch", self.write_patch(context_only)], "a hunk of m/n.go has the same old and new lines"),
        )
        for argv, reason in cases:
            out = self.refused(*argv, "--test", "TestAdd")
            self.assertEqual(out.splitlines()[-1], f"error: refused: a plant that changes nothing: {reason}", out)

    def test_an_entry_timeout_go_cannot_read_is_refused_naming_the_batch_key(self):
        # No subTest (as above). Go's time.Duration holds at most 2**63 - 1 ns, about 2562047.788h.
        longest = "is longer than Go's longest duration, about 2562047h"
        cases = (
            ("0s", "timeout '0s' is not positive"),
            ("2562048h", f"timeout '2562048h' {longest}"),
            (10**30, f"timeout '1000000000000000000000000000000s' {longest}"),
            (1e20, f"timeout '100000000000000000000s' {longest}"),
            (float("inf"), "timeout inf is not a Go duration or positive seconds"),
            (float("nan"), "timeout nan is not a Go duration or positive seconds"),
        )
        plant = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        for value, reason in cases:
            out = self.refused("--batch", self.write_batch({**plant, "timeout": value}))
            self.assertEqual(out.splitlines()[-1], f"error: refused: batch entry 1: {reason}", out)

    def test_a_numeric_entry_timeout_is_written_as_a_go_duration_without_an_exponent(self):
        plant = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        with contextlib.suppress(FileNotFoundError):
            os.remove(self.log)
        path = self.write_batch({**plant, "timeout": 0.00001}, {**plant, "timeout": "2562047h"})
        code, out = self.mutate("--batch", path, env=self.fake_env)
        self.assertEqual(code, 1, out)  # the fake go test prints nothing: invalid, twice
        self.assertIn("timeout: 0.00001s\n", out)
        self.assertIn("timeout: 2562047h\n", out)
        with open(self.log, encoding="utf-8") as f:
            tests = [line.split() for line in f if line.split()[1] == "test"]
        self.assertEqual([argv[argv.index("-timeout") + 1] for argv in tests], ["0.00001s", "2562047h"], out)

    def test_an_id_that_is_not_one_token_or_a_note_holding_a_line_break_is_refused_before_any_build(self):
        # No subTest (as above).
        head = self.git("rev-parse", "HEAD")
        plant = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        sides = {"old_rev": head, "new_rev": head, "expect": {"old": "red", "new": "red"}}
        forged = "x\nexpect: 9 entries, 9 as expected; differing: none"
        cases = (
            ("--batch", {**plant, "id": "a b"}, "id 'a b' is not one printable token"),
            ("--batch", {**plant, "id": ""}, "id '' is not one printable token"),
            ("--expect", {**plant, **sides, "id": forged}, f"id {forged!r} is not one printable token"),
            ("--batch", {**plant, "note": "one\ntwo"}, "note 'one\\ntwo' holds a control character or a line break"),
            (
                "--batch",
                {**plant, "note": "one\u2028two"},
                "note 'one\\u2028two' holds a control character or a line break",
            ),
        )
        for mode, spec, reason in cases:
            out = self.refused(mode, self.write_batch(spec))
            self.assertEqual(out.splitlines()[-1], f"error: refused: batch entry 1: {reason}", out)

    def test_a_subtest_with_a_cpu_list_is_refused_before_any_build(self):
        argv = ("--file", "m/m.go", "--find", FIND, "--replace", CAUGHT, "--test", "TestTable", "--subtest", "F3")
        out = self.refused(*argv, "--cpu", "1,2")
        self.assertEqual(
            out.splitlines()[-1], "error: refused: a subtest with --cpu 1,2: go test runs it once per value"
        )
        code, out = self.mutate(*argv, "--cpu", "2", env=self.fake_env)
        self.assertEqual(code, 1, out)  # one value: the fake go ran, printed nothing, invalid


def ran(out: str) -> list[str]:
    """The `ran:` lines a run printed, in order."""
    return [line for line in out.splitlines() if line.startswith("ran: ")]


class Subtests(Base):
    """--subtest: the level path rewritten as go test names it, QuoteMeta'd and anchored per level, a probe counting
    the runs first, and the subtest's own FAIL, PASS or SKIP line judged."""

    def subtest(self, test: str, name: str, replace: str = CAUGHT) -> tuple[int, str]:
        return self.single(test, replace, "--subtest", name)

    def test_a_subtest_failing_against_the_plant_is_red_while_its_sibling_passes(self):
        code, out = self.subtest("TestTable", "F3")
        self.assertEqual(code, 0, out)
        self.assertIn("subtest: TestTable/F3", out)
        self.assertEqual(ran(out), ["ran: TestTable/F3"], out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_subtest_passing_while_its_sibling_fails_is_green(self):
        code, out = self.subtest("TestTable", "F30")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: green", out)

    def test_a_subtest_that_does_not_exist_is_invalid(self):
        code, out = self.subtest("TestTable", "NoSuch")
        self.assertEqual(code, 1, out)
        self.assertIn("probe: 0 subtests run as TestTable/NoSuch", out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)

    def test_a_name_with_a_slash_spaces_or_metacharacters_runs_exactly_itself(self):
        cases = (
            ("E-SYN-001/not a participle error", "red", "TestTable/E-SYN-001/not_a_participle_error"),
            ("x+y (meta)", "green", "TestTable/x+y_(meta)"),
        )
        for name, word, full in cases:
            with self.subTest(name):
                code, out = self.subtest("TestTable", name)
                self.assertEqual(ran(out), [f"ran: {full}"], out)
                self.assertEqual(out.splitlines()[-1], f"outcome: {word}", out)

    def test_f3_never_runs_its_sibling_f30(self):
        code, out = self.subtest("TestTable", "F3")
        self.assertEqual(ran(out), ["ran: TestTable/F3"], out)

    def test_two_subtests_rewritten_to_one_name_are_invalid(self):
        code, out = self.subtest("TestTable", "dup")
        self.assertEqual(code, 1, out)
        self.assertIn("probe: 2 subtests run as TestTable/dup", out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)

    def test_a_parent_failing_before_t_run_is_invalid_never_red(self):
        code, out = self.subtest("TestSetup", "after setup")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)

    def test_go_spaces_in_a_subtest_name_are_rewritten_as_go_test_names_them(self):
        """U+00A0 and U+2003 (Go spaces, not ASCII): the names go test itself prints are the ones mutate.py runs."""
        done = subprocess.run(
            ["go", "test", "-count=1", "-v", "-run", "^TestSpaces$", "./m"],
            cwd=self.repo,
            env=self.env,
            capture_output=True,
            timeout=300,
        )
        names = re.findall(r"(?m)^=== RUN\s+(TestSpaces/\S+)$", done.stdout.decode("utf-8", errors="replace"))
        self.assertEqual(names, ["TestSpaces/nb_sp", "TestSpaces/em_sp"], done.stdout)  # go test's own rewriting
        for raw, go_name in zip(("nb\u00a0sp", "em\u2003sp"), names, strict=True):
            with self.subTest(go_name):
                code, out = self.subtest("TestSpaces", raw)
                self.assertEqual(code, 0, out)
                self.assertIn(f"subtest: {go_name}", out)
                self.assertEqual(ran(out), [f"ran: {go_name}"], out)
                self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_sibling_named_with_a_one_digit_suffix_is_not_counted_as_a_repeat(self):
        """go test writes a repeated name as #NN, two digits or more: a literal sibling `case#1` is another test."""
        code, out = self.subtest("TestTable", "case")
        self.assertEqual(code, 0, out)
        self.assertEqual(ran(out), ["ran: TestTable/case"], out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_skipped_subtest_is_invalid(self):
        code, out = self.subtest("TestSkips", "skipped", "return b + a")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: invalid", out)


TWO_HUNKS = """\
--- a/m/n.go
+++ b/m/n.go
@@ -4 +4 @@
-\treturn a * b
+\treturn a / b
@@ -8 +8 @@
-\treturn -a
+\treturn a
"""

NEG_PATCH = """\
--- a/m/n.go
+++ b/m/n.go
@@ -7,3 +7,3 @@
 func Neg(a int) int {
-\treturn -a
+\treturn a
 }
"""


class Patch(Base):
    """--patch: a unified diff, each hunk placed by its old lines occurring exactly once in HEAD's file."""

    def test_a_two_hunk_patch_applies_each_hunk_once(self):
        code, out = self.mutate("--patch", self.write_patch(TWO_HUNKS), "--test", "TestNeg")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)
        self.assertIn("-\treturn a * b\n+\treturn a / b\n", out)  # both hunks in the printed diff
        self.assertIn("-\treturn -a\n+\treturn a\n", out)
        self.assertIn("package: ./m", out)

    def test_a_patch_from_git_diff_output_applies(self):
        clone = os.path.join(self.tmp, "clone")
        subprocess.run(
            ["git", "clone", "-q", self.repo, clone], env=self.env, check=True, capture_output=True, timeout=60
        )
        neg = os.path.join(clone, "m", "n.go")
        with open(neg, encoding="utf-8") as f:
            text = f.read()
        self.assertEqual(text.count("\treturn -a\n"), 1)
        with open(neg, "w", encoding="utf-8") as f:
            f.write(text.replace("\treturn -a\n", "\treturn a\n"))
        done = subprocess.run(
            ["git", "-C", clone, "diff", "--no-color"], env=self.env, check=True, capture_output=True, timeout=60
        )
        diff = done.stdout.decode()
        lines = diff.splitlines()
        self.assertEqual(lines[0], "diff --git a/m/n.go b/m/n.go", diff)  # git's own headers, each witnessed first
        self.assertTrue(lines[1].startswith("index "), diff)
        self.assertEqual(lines[2:4], ["--- a/m/n.go", "+++ b/m/n.go"], diff)
        self.assertRegex(lines[4], r"^@@ -\d+,\d+ \+\d+,\d+ @@ func ", diff)
        code, out = self.mutate("--patch", self.write_patch(diff), "--test", "TestNeg")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_hunk_line_without_its_newline_is_refused_before_any_build(self):
        # git apply --check calls it a corrupt patch; read as it stands, "\treturn a" took the next line's "}".
        out = self.refused("--patch", self.write_patch(NEG_PATCH.removesuffix(" }\n") + " }"), "--test", "TestNeg")
        self.assertIn("a hunk line without its newline", out.splitlines()[-1])

    def test_a_crlf_patch_is_refused_before_any_build(self):
        out = self.refused("--patch", self.write_patch(NEG_PATCH.replace("\n", "\r\n")), "--test", "TestNeg")
        self.assertIn("CRLF", out.splitlines()[-1])

    def test_a_context_line_holding_a_form_feed_or_a_line_separator_applies(self):
        patch = (
            f"--- a/m/sep.go\n+++ b/m/sep.go\n@@ -3,4 +3,4 @@\n {SEP_COMMENT}\n func Sep(a int) int {{\n"
            "-\treturn a + 1\n+\treturn a + 2\n }\n"
        )
        code, out = self.mutate("--patch", self.write_patch(patch), "--test", "TestSep")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_hunk_matching_only_inside_a_line_is_refused_before_any_build(self):
        inside = "--- a/m/m.go\n+++ b/m/m.go\n@@ -4 +4 @@\n-a + b\n+a - b\n"  # m.go's line is "\treturn a + b"
        out = self.refused("--patch", self.write_patch(inside), "--test", "TestAdd")
        self.assertEqual(out.splitlines()[-1], "error: refused: a hunk of m/m.go occurs 0 times, not exactly once")

    def test_a_git_diff_hunk_whose_old_line_also_ends_another_line_applies(self):
        clone = os.path.join(self.tmp, "clone")
        subprocess.run(
            ["git", "clone", "-q", self.repo, clone], env=self.env, check=True, capture_output=True, timeout=60
        )
        support.write(
            clone,
            {
                "m/tail.go": TAIL.replace(
                    "func Neg2(a int) int {\n\treturn -a\n", "func Neg2(a int) int {\n\treturn a\n"
                )
            },
        )
        done = subprocess.run(
            ["git", "-C", clone, "diff", "--no-color", "-U0"], env=self.env, check=True, capture_output=True, timeout=60
        )
        diff = done.stdout.decode()
        self.assertIn("\n-\treturn -a\n+\treturn a\n", diff)  # zero context: the old block is "\treturn -a\n" alone
        self.assertEqual(TAIL.count("\treturn -a\n"), 2)  # Neg2's line, and the tail of Abs's "\t\treturn -a"
        code, out = self.mutate("--patch", self.write_patch(diff), "--test", "TestNeg2")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_hunk_matching_twice_or_never_or_a_dirty_file_or_a_bad_header_is_refused_before_any_build(self):
        hunk = "--- a/m/n.go\n+++ b/m/n.go\n@@ -1 +1 @@\n-{old}\n+{new}\n"
        cases = (
            ("twice", hunk.format(old="\treturn a + a", new="\treturn a - a"), "occurs 2 times"),
            ("never", hunk.format(old="\treturn a % b", new="\treturn a"), "occurs 0 times"),
            ("dirty", NEG_PATCH, "differs from HEAD"),
            ("count", NEG_PATCH.replace("@@ -7,3 +7,3 @@", "@@ -7,4 +7,4 @@"), "shorter than its header"),
            ("paths", NEG_PATCH.replace("+++ b/m/n.go", "+++ b/m/m.go"), "name different files"),
        )
        for name, text, reason in cases:
            with self.subTest(name):
                if name == "dirty":
                    self.addCleanup(self.restore, "m/n.go")
                    with open(os.path.join(self.repo, "m", "n.go"), "a", encoding="utf-8") as f:
                        f.write("// dirty\n")
                out = self.refused("--patch", self.write_patch(text), "--test", "TestNeg")
                self.assertIn(reason, out)


PY_CALC = "def add(a, b):\n    return a + b\n"

PY_TESTS = """\
import subprocess
import threading
import unittest

from pkg import calc


class Calc(unittest.TestCase):
    def test_add(self):
        self.assertEqual(calc.add(2, 3), 5)

    def test_weak(self):
        self.assertEqual(calc.add(0, 0), 0)

    def test_skips(self):
        self.skipTest("skipped on purpose")

    def test_lists_tracked_files(self):
        done = subprocess.run(["git", "ls-files"], capture_output=True, check=True, timeout=60)
        self.assertIn("pkg/calc.py", done.stdout.decode().splitlines())

    def test_tree_is_clean(self):
        subprocess.run(["git", "diff", "--quiet", "HEAD"], check=True, timeout=60)
        done = subprocess.run(["git", "status", "--porcelain"], capture_output=True, check=True, timeout=60)
        self.assertEqual(done.stdout.decode(), "")

    def test_waits_forever(self):
        threading.Event().wait()

    def test_fails_and_leaks(self):
        open(__file__)
        self.assertEqual(calc.add(2, 3), 5)


class Empty(unittest.TestCase):
    pass
"""

CALC = "pkg.tests.test_calc.Calc."
PY_FIND = "return a + b"


class PyMode(unittest.TestCase):
    """--py: the unittest id runs under the pytests row's shim in a detached worktree holding the committed plant."""

    def setUp(self) -> None:
        faulthandler.dump_traceback_later(600, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = tmp.name
        self.repo = os.path.join(self.tmp, "repo")
        os.mkdir(self.repo)
        self.env = {**os.environ, **support.GIT_ENV}
        support.write(
            self.repo,
            {
                "pkg/__init__.py": "",
                "pkg/calc.py": PY_CALC,
                "pkg/tests/__init__.py": "",
                "pkg/tests/test_calc.py": PY_TESTS,
            },
        )
        os.makedirs(os.path.join(self.repo, "gate", "runner"))
        shutil.copyfile(
            os.path.join(support.REPO, "gate", "runner", "pyshim.py"),
            os.path.join(self.repo, "gate", "runner", "pyshim.py"),
        )
        self.git("init", "-q")
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "c")
        self.before = self.snapshot()
        self.worktrees = self.git("worktree", "list", "--porcelain")
        self.checked_in_the_test = False
        self.addCleanup(self.assertUntouched)

    def git(self, *argv: str) -> str:
        return support.git(self.repo, self.env, *argv)

    def snapshot(self) -> dict[str, tuple[bytes, int]]:
        names = self.git("ls-files").splitlines()
        return {name: support.stamp(os.path.join(self.repo, name)) for name in names}

    def assertUntouched(self) -> None:
        """Every tracked file byte- and mtime-identical, and the worktree list as it was before the run (#16).

        The test of exactly these makes them its own assertions instead: a failing cleanup is a second failure, and
        --py's red is exactly one."""
        if self.checked_in_the_test:
            return
        self.assertEqual(self.snapshot(), self.before, "a tracked file was written")
        self.assertEqual(self.git("worktree", "list", "--porcelain"), self.worktrees, "a worktree was left")

    def mutate(self, *argv: str) -> tuple[int, str]:
        return support.tool("mutate", list(argv), self.repo, self.env, os.path.join(self.tmp, "mutate.stdout"))

    def py(self, test: str, replace: str, *extra: str) -> tuple[int, str]:
        return self.mutate("--py", test, "--file", "pkg/calc.py", "--find", PY_FIND, "--replace", replace, *extra)

    def test_a_caught_python_mutation_is_red_and_a_missed_one_green(self):
        for test, code_want, word in ((CALC + "test_add", 0, "red"), (CALC + "test_weak", 1, "green")):
            with self.subTest(test):
                code, out = self.py(test, "return a - b")
                self.assertEqual(code, code_want, out)
                self.assertEqual(out.splitlines()[-1], f"outcome: {word}", out)
                self.assertIn(f"py: {test}", out)
                self.assertIn("-    return a + b\n+    return a - b\n", out)
                self.assertIn(f"copy: detached worktree at {self.git('rev-parse', 'HEAD')}", out)
                self.assertRegex(out, r"(?m)^python: Python 3\.")

    def test_a_broken_import_a_skip_or_zero_tests_is_invalid(self):
        cases = (
            ("import", CALC + "test_add", "return a +"),
            ("skip", CALC + "test_skips", "return b + a"),
            ("zero", "pkg.tests.test_calc.Empty", "return b + a"),
            ("leak", CALC + "test_fails_and_leaks", "return a - b"),
        )
        # No subTest: each failing subTest is a failure of its own, and --py's red is exactly one failure, so a
        # plant reading every case red must stop this test at its first case.
        for name, test, replace in cases:
            code, out = self.py(test, replace)
            self.assertEqual(code, 1, f"{name}: {out}")
            self.assertEqual(out.splitlines()[-1], "outcome: invalid", f"{name}: {out}")
            self.assertIn(f"report: pyshim: {test}: ", out, name)

    def test_a_test_running_git_ls_files_reads_as_in_the_tree(self):
        test = CALC + "test_lists_tracked_files"
        report = os.path.join(self.tmp, "direct.report")
        shim = os.path.join(self.repo, "gate", "runner", "pyshim.py")
        with open(os.path.join(self.tmp, "direct.stdout"), "wb") as out:
            subprocess.run(
                [support.PY, "-I", "-B", "-X", "dev", "-W", "error", shim, self.repo, test, report],
                cwd=self.repo,
                env=self.env,
                stdin=subprocess.DEVNULL,
                stdout=out,
                stderr=out,
                timeout=120,
            )
        with open(report, encoding="utf-8") as f:
            self.assertEqual(f.read(), f"pyshim: {test}: passed 1 tests\n")  # the outcome in the tree itself
        code, out = self.py(test, "return b + a")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: green", out)

    def test_a_plant_elsewhere_leaves_git_diff_and_status_clean(self):
        code, out = self.py(CALC + "test_tree_is_clean", "return b + a")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: green", out)

    def test_a_test_waiting_past_the_timeout_is_timeout(self):
        code, out = self.py(CALC + "test_waits_forever", "return b + a", "--timeout", "5s")
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines()[-1], "outcome: timeout", out)

    def test_a_python_patch_is_applied_in_the_copy(self):
        path = os.path.join(self.tmp, "calc.diff")
        with open(path, "w", encoding="utf-8") as f:
            f.write(
                "--- a/pkg/calc.py\n+++ b/pkg/calc.py\n@@ -1,2 +1,2 @@\n def add(a, b):\n"
                "-    return a + b\n+    return a - b\n"
            )
        code, out = self.mutate("--py", CALC + "test_add", "--patch", path)
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)
        self.assertEqual(self.snapshot(), self.before)  # pkg/calc.py untouched

    def test_a_py_run_leaves_the_tree_and_the_worktree_list_as_they_were(self):
        self.checked_in_the_test = True
        code, out = self.py(CALC + "test_add", "return a - b")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)  # the witness: the plant ran somewhere
        self.assertIn("copy: detached worktree at ", out)
        self.assertEqual(self.snapshot(), self.before)
        self.assertEqual(self.git("worktree", "list", "--porcelain"), self.worktrees)

    def test_repeat_with_load_runs_the_py_id_each_time_and_the_hogs_are_gone(self):
        code, out = self.py(CALC + "test_add", "return a - b", "--repeat", "2", "--load")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "caught 2/2", out)
        self.assertEqual(re.findall(r"(?m)^run \d/2: (\w+)$", out), ["red", "red"], out)
        pids = support.pids_line(out, "load:")
        self.assertGreaterEqual(len(pids), 1, out)  # the witness: hogs ran
        self.assertEqual([p for p in pids if support.alive(p)], [], out)

    def test_a_plain_batch_runs_each_py_entry_in_the_one_worktree(self):
        path = os.path.join(self.tmp, "batch.json")
        entries = [
            {"file": "pkg/calc.py", "find": PY_FIND, "replace": "return a - b", "py": CALC + "test_add"},
            {"file": "pkg/calc.py", "find": PY_FIND, "replace": "return a - b", "py": CALC + "test_weak"},
        ]
        with open(path, "w", encoding="utf-8") as f:
            json.dump(entries, f)
        code, out = self.mutate("--batch", path)
        self.assertEqual(code, 1, out)
        self.assertIn("entry 1: red", out)
        self.assertIn("entry 2: green", out)
        self.assertEqual(out.splitlines()[-1], "counts: red 1, green 1, timeout 0, invalid 0", out)


PY_ADD = "    def test_add(self):\n"


class OneTree(Base):
    """Every run, Go and Python, in the run's worktree of HEAD with the plant committed (the owner's "one way of
    running", 2026-11-21): an uncommitted edit never reaches a run, a data file read at run time is planted, and the
    `head:` line names the tree tested."""

    def test_an_uncommitted_edit_never_reaches_a_go_run(self):
        self.dirty_go_tests()
        done = subprocess.run(
            ["go", "test", "-count=1", "-v", "-run", "^TestAdd$", "./m"],
            cwd=self.repo,
            env=self.env,
            capture_output=True,
            timeout=300,
        )
        self.assertIn("--- FAIL: TestAdd (", done.stdout.decode("utf-8", errors="replace"))  # the edit, compiled
        code, out = self.single("TestAdd", "return b + a")
        self.assertEqual(code, 1, out)
        self.assertIn(f"head: {self.git('rev-parse', 'HEAD')}", out)
        self.assertEqual(out.splitlines()[-1], "outcome: green", out)

    def test_a_plant_in_a_data_file_the_test_reads_is_red(self):
        code, out = self.mutate(
            "--file", "m/testdata/want.txt", "--find", "5", "--replace", "6", "--test", "TestData", "--pkg", "m"
        )
        self.assertEqual(code, 0, out)
        self.assertIn("-5\n+6\n", out)
        self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_plain_batch_of_go_and_py_entries_prints_one_head_and_runs_both_at_it(self):
        self.add_python()
        head = self.git("rev-parse", "HEAD")
        self.dirty_go_tests()
        self.assertEqual(PY_TESTS.count(PY_ADD), 1)
        self.addCleanup(support.write, self.repo, {"pkg/tests/test_calc.py": PY_TESTS})
        dirty = PY_TESTS.replace(PY_ADD, PY_ADD + '        self.fail("an uncommitted edit")\n')
        support.write(self.repo, {"pkg/tests/test_calc.py": dirty})
        path = self.write_batch(
            {"file": "m/m.go", "find": FIND, "replace": "return b + a", "test": "TestAdd"},
            {"file": "pkg/calc.py", "find": PY_FIND, "replace": "return b + a", "py": CALC + "test_add"},
        )
        code, out = self.mutate("--batch", path)
        self.assertEqual(code, 1, out)
        self.assertEqual([line for line in out.splitlines() if line.startswith("head: ")], [f"head: {head}"], out)
        self.assertIn("entry 1: green", out)
        self.assertIn("entry 2: green", out)
        self.assertEqual(out.splitlines()[-1], "counts: red 0, green 2, timeout 0, invalid 0", out)


class Compiled(Base):
    """A .go plant in a file ./<pkg>'s test build never compiles is refused before any build: `go list -deps -test`
    at the revision planted names the files compiled. A non-.go plant is applied in the worktree (OneTree)."""

    def go_calls(self, *argv: str) -> tuple[int, str, list[list[str]]]:
        """A run under the logging fake go (go list handed to the real go): its exit, its output, and each go call's
        arguments."""
        with contextlib.suppress(FileNotFoundError):
            os.remove(self.log)
        code, out = self.mutate(*argv, env=self.fake_env)
        with open(self.log, encoding="utf-8") as f:
            return code, out, [line.split()[1:] for line in f]

    def refused_after_listing(self, *argv: str) -> str:
        """Exit 2 with a refusal as the last line, and one go call, the go list naming the files compiled: no
        build ran."""
        code, out, calls = self.go_calls(*argv)
        self.assertEqual(code, 2, out)
        self.assertTrue(out.splitlines()[-1].startswith("error: refused: "), out)
        self.assertEqual([call[0] for call in calls], ["list"], out)
        return out

    def test_a_plant_in_a_go_build_ignore_file_is_refused_before_any_build(self):
        out = self.refused_after_listing(
            "--file", "m/ignored.go", "--find", "return 1", "--replace", "this is not Go", "--test", "TestAdd"
        )
        self.assertEqual(out.splitlines()[-1], "error: refused: m/ignored.go is not compiled by ./m's test build")

    def test_a_plant_in_a_package_or_a_test_file_the_test_build_never_compiles_is_refused(self):
        # No subTest: a plant reading every case wrong must stop this test at its first case (--py's one failure).
        cases = (
            (("u/u.go", "return 1", "return 2", "TestAdd", "m"), "u/u.go is not compiled by ./m's test build"),
            (  # m is compiled into g's test, m's own tests are not
                ("m/m_test.go", "func TestWeak(", "func TestWeak2(", "TestGuard", "g"),
                "m/m_test.go is not compiled by ./g's test build",
            ),
        )
        for (file, find, replace, test, pkg), reason in cases:
            argv = ("--file", file, "--find", find, "--replace", replace, "--test", test, "--pkg", pkg)
            out = self.refused_after_listing(*argv)
            self.assertEqual(out.splitlines()[-1], f"error: refused: {reason}", out)

    def test_a_go_file_under_testdata_or_an_underscore_directory_is_data_and_planted(self):
        # No subTest (as above). The go tool never compiles testdata, _ or . directories: a test may read them.
        for path in ("m/testdata/want.go", "m/_data/want.go"):
            argv = ("--file", path, "--find", "Sum = 5", "--replace", "Sum = 6", "--test", "TestGoData", "--pkg", "m")
            code, out = self.mutate(*argv)
            self.assertEqual(code, 0, f"{path}: {out}")
            self.assertEqual(out.splitlines()[-1], "outcome: red", out)

    def test_a_plant_in_a_race_only_file_is_compiled_under_race(self):
        argv = ("--file", "m/race.go", "--find", "return 1", "--replace", "return 2", "--test", "TestAdd", "--race")
        code, out, calls = self.go_calls(*argv)
        self.assertEqual(code, 1, out)  # the fake go test prints nothing: invalid
        self.assertEqual([call[0] for call in calls], ["list", "version", "test"], out)  # version: the record
        self.assertIn("-race", calls[0], out)


class Forms(Base):
    """Each mode's required keys: one mutation form (file, find and replace, or patch) and one target (test or py)."""

    def batch(self, *entries: dict[str, object]) -> str:
        """The entries in a batch file of their own (a name no other call of this test reuses)."""
        fd, path = tempfile.mkstemp(suffix=".json", dir=self.tmp)
        os.close(fd)
        with open(path, "w", encoding="utf-8") as f:
            json.dump(list(entries), f)
        return path

    def test_a_mutation_or_a_target_given_twice_or_in_part_is_refused_before_any_build(self):
        good = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        patch = self.write_patch(NEG_PATCH)
        cases = (
            ("find with patch", ["--batch", self.batch({**good, "patch": NEG_PATCH})], "patch: not both"),
            (
                "no replace",
                ["--batch", self.batch({"file": "m/m.go", "find": FIND, "test": "TestAdd"})],
                "missing replace",
            ),
            ("test with py", ["--batch", self.batch({**good, "py": "pkg.tests.Calc"})], "test or py: not both"),
            ("neither target", ["--file", "m/m.go", "--find", FIND, "--replace", CAUGHT], "neither is given"),
            ("pkg with py", ["--patch", patch, "--py", "a.b", "--pkg", "m"], "not with py"),
            ("race with py", ["--patch", patch, "--py", "a.b", "--race"], "--race and --cpu"),
            ("patch with file", ["--patch", patch, "--file", "m/m.go", "--test", "TestNeg"], "not both"),
            ("a one-part py id", ["--patch", patch, "--py", "calc"], "'calc' is not a dotted unittest id"),
            ("a bad py id part", ["--patch", patch, "--py", "pkg.1x"], "'pkg.1x' is not a dotted unittest id"),
            (
                "an absolute patch path",
                [
                    "--patch",
                    self.write_patch(NEG_PATCH.replace("a/m/", "/m/").replace("b/m/", "/m/")),
                    "--test",
                    "TestNeg",
                ],
                "repo-relative",
            ),
            (
                "a new file",
                [
                    "--patch",
                    self.write_patch("--- /dev/null\n+++ b/m/x.go\n@@ -0,0 +1 @@\n+package m\n"),
                    "--test",
                    "TestNeg",
                ],
                "in place",
            ),
        )
        for name, argv, fragment in cases:
            with self.subTest(name):
                out = self.refused(*argv)
                self.assertIn(fragment, out.splitlines()[-1])

    def test_a_batch_patch_entry_holds_the_diff_text_and_a_go_entry_reads_red(self):
        code, out = self.mutate("--batch", self.batch({"patch": NEG_PATCH, "test": "TestNeg"}))
        self.assertEqual(code, 0, out)
        self.assertIn("entry 1: red", out)
        self.assertEqual(out.splitlines()[-1], "counts: red 1, green 0, timeout 0, invalid 0", out)


class Batch(Base):
    def test_an_entry_timeout_replaces_the_runs_timeout_for_that_entry(self):
        hang = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestHang"}
        code, out = self.mutate(
            "--batch", self.write_batch({**hang, "timeout": "2s"}, {**hang, "timeout": 3}), "--timeout", "5s"
        )
        self.assertIn("timeout: 2s", out)
        self.assertIn("timeout: 3s", out)
        self.assertNotIn("timeout: 5s", out)
        self.assertIn("entry 1: timeout", out)
        self.assertIn("entry 2: timeout", out)
        self.assertEqual(code, 1, out)

    def test_red_green_invalid_counts_and_exit_1(self):
        path = os.path.join(self.tmp, "batch.json")
        entries = [
            {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd", "note": "sum caught"},
            {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestWeak", "note": "zero misses it"},
            {"file": "m/m.go", "find": FIND, "replace": BROKEN, "test": "TestAdd"},
        ]
        with open(path, "w", encoding="utf-8") as f:
            json.dump(entries, f)
        code, out = self.mutate("--batch", path)
        self.assertEqual(code, 1, out)
        self.assertIn("entry 1: red", out)
        self.assertIn("entry 2: green", out)
        self.assertIn("entry 3: invalid", out)
        self.assertIn("note: zero misses it", out)
        self.assertEqual(out.splitlines()[-1], "counts: red 1, green 1, timeout 0, invalid 1", out)


class Expect(Base):
    """--expect: each entry at its old and its new revision, in one worktree per revision reset between entries."""

    def setUp(self) -> None:
        super().setUp()
        self.add_python()
        self.rev1 = self.git("rev-parse", "HEAD")
        tests = os.path.join(self.repo, "m", "m_test.go")
        with open(tests, encoding="utf-8") as f:
            text = f.read()
        weak = '\t\tt.Fatal("Add(0, 0) != 0")\n\t}\n'
        self.assertEqual(text.count(weak), 1)
        with open(tests, "w", encoding="utf-8") as f:
            f.write(text.replace(weak, weak + '\tif Add(2, 3) != 5 {\n\t\tt.Fatal("Add(2, 3) != 5")\n\t}\n'))
        self.git("commit", "-q", "-am", "TestWeak strengthened")
        self.rev2 = self.git("rev-parse", "HEAD")

    def revs(self) -> dict[str, str]:
        return {"old_rev": self.rev1, "new_rev": self.rev2}

    def test_an_entry_off_its_expectation_exits_1_naming_it_and_the_control_reads_the_real_code(self):
        plant = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestWeak"}
        weak = {"test": "TestWeak", "pkg": "m"}
        path = self.write_batch(
            {"id": "A", **plant, **self.revs(), "expect": {"old": "green", "new": "red"}},
            {"id": "B", **plant, **self.revs(), "expect": {"old": "red", "new": "red"}},
            {"id": "C", "control": True, **weak, **self.revs(), "expect": {"old": "green", "new": "green"}},
            {"id": "D", "control": True, **weak, **self.revs(), "expect": {"old": "red", "new": "green"}},
            {
                "id": "E",
                "file": "pkg/calc.py",
                "find": PY_FIND,
                "replace": "return a - b",
                "old_py": CALC + "test_weak",
                "new_py": CALC + "test_add",
                **self.revs(),
                "expect": {"old": "green", "new": "red"},
            },
        )
        code, out = self.mutate("--expect", path)
        self.assertEqual(code, 1, out)
        old, new = self.rev1[:12], self.rev2[:12]
        self.assertIn(f"entry A new {new}: red (expected red)", out)
        self.assertIn(f"entry C old {old}: green (expected green)", out)  # after A's and B's plants at both revisions
        self.assertIn(f"entry C new {new}: green (expected green)", out)
        self.assertIn(f"entry E old {old}: green (expected green)", out)
        self.assertIn(f"entry E new {new}: red (expected red)", out)
        self.assertEqual(out.splitlines()[-1], "expect: 5 entries, 3 as expected; differing: B, D", out)

    def test_an_entry_without_a_revision_or_naming_an_absent_one_is_refused(self):
        plant = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestWeak"}
        expect = {"expect": {"old": "green", "new": "red"}}
        cases = (
            ("missing old_rev", {"id": "A", **plant, "new_rev": self.rev2, **expect}, "missing old_rev"),
            (
                "40 zeros",
                {"id": "Z", "control": True, "test": "TestWeak", "pkg": "m", "old_rev": "0" * 40, "new_rev": self.rev2}
                | expect,
                "names no commit",
            ),
            (
                "a 12-hex prefix",
                {"id": "P", **plant, "old_rev": self.rev1[:12], "new_rev": self.rev2, **expect},
                "not a full commit id",
            ),
        )
        for name, spec, fragment in cases:
            with self.subTest(name):
                out = self.refused("--expect", self.write_batch(spec))
                self.assertIn(fragment, out.splitlines()[-1])


class Repeat(Base):
    def test_a_deterministic_catch_is_caught_3_of_3_and_the_hogs_are_gone(self):
        code, out = self.single("TestAdd", CAUGHT, "--repeat", "3", "--load")
        self.assertEqual(code, 0, out)
        self.assertEqual(out.splitlines()[-1], "caught 3/3", out)
        self.assertEqual(out.count(": red\n"), 3, out)
        pids = support.pids_line(out, "load:")
        self.assertGreaterEqual(len(pids), 1, out)  # the witness: hogs ran
        self.assertEqual([p for p in pids if support.alive(p)], [], out)

    def test_repeat_without_load_is_refused(self):
        code, out = self.single("TestAdd", CAUGHT, "--repeat", "3")
        self.assertEqual(code, 2, out)


class Terminated(Base):
    def test_sigterm_mid_run_leaves_the_file_and_no_process_of_the_group(self):
        fifo = os.path.join(self.tmp, "hold.fifo")
        os.mkfifo(fifo)
        fd = os.open(fifo, os.O_RDONLY | os.O_NONBLOCK)
        self.addCleanup(os.close, fd)
        pgid_path = os.path.join(self.tmp, "hold.pgid")
        env = {**self.env, "HOLD_FIFO": fifo, "HOLD_PGID": pgid_path}
        script = os.path.join(support.REPO, "gate", "bench", "mutate.py")
        argv = [sys.executable, "-I", "-B", script, "--file", "m/m.go", "--find", FIND, "--replace", "return b + a"]
        with open(os.path.join(self.tmp, "mutate.stdout"), "wb") as out:
            proc = subprocess.Popen(
                [*argv, "--test", "TestHold", "--timeout", "300s"],
                cwd=self.repo,
                env=env,
                stdin=subprocess.DEVNULL,
                stdout=out,
                stderr=out,
            )
        self.addCleanup(proc.wait)
        self.addCleanup(self.kill, proc)

        def started() -> bytes:
            try:
                return os.read(fd, 1)
            except BlockingIOError:
                return b""

        self.assertEqual(wait.until("the held test to write its start byte", started, cap_s=CAP_S), b"s")
        with open(pgid_path, encoding="ascii") as f:
            pgid = int(f.read())
        self.addCleanup(self.killpg, pgid)
        self.assertTrue(self.group_alive(pgid))  # the witness: the go group is running
        proc.send_signal(signal.SIGTERM)
        self.assertEqual(proc.wait(timeout=CAP_S), 2)

        def at_eof() -> bool:
            try:
                return os.read(fd, 64) == b""
            except BlockingIOError:
                return False

        wait.until("every holder of the witness fifo to be gone", at_eof, cap_s=CAP_S)
        self.assertFalse(self.group_alive(pgid), f"process group {pgid} outlived mutate.py")
        self.assertEqual(stat.S_IFMT(os.stat(self.tracked).st_mode), stat.S_IFREG)

    @staticmethod
    def group_alive(pgid: int) -> bool:
        try:
            os.killpg(pgid, 0)
        except ProcessLookupError:
            return False
        return True

    @staticmethod
    def kill(proc: subprocess.Popen[bytes]) -> None:
        with contextlib.suppress(ProcessLookupError):
            proc.kill()

    @staticmethod
    def killpg(pgid: int) -> None:
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(pgid, signal.SIGKILL)


if __name__ == "__main__":
    unittest.main()
