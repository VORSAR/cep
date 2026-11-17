"""mutate.py: one mutation through -overlay, its one designated test, and a tracked file never written."""

import contextlib
import faulthandler
import json
import os
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
"""

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
        support.write(self.repo, {"go.mod": MODULE, "m/m.go": SOURCE, "m/m_test.go": TESTS, "g/g_test.go": GUARD})
        self.git("init", "-q")
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "c")
        self.tracked = os.path.join(self.repo, "m", "m.go")
        self.before = self.snapshot()
        self.addCleanup(self.assertUntouched, self.before)

    def git(self, *argv: str) -> str:
        return support.git(self.repo, self.env, *argv)

    def snapshot(self) -> tuple[bytes, int]:
        with open(self.tracked, "rb") as f:
            return f.read(), os.stat(self.tracked).st_mtime_ns

    def assertUntouched(self, before: tuple[bytes, int]) -> None:
        self.assertEqual(self.snapshot(), before, "the tracked file was written")

    def mutate(self, *argv: str, env: dict[str, str] | None = None) -> tuple[int, str]:
        out_path = os.path.join(self.tmp, "mutate.stdout")
        return support.tool("mutate", list(argv), self.repo, self.env if env is None else env, out_path)

    def single(self, test: str, replace: str = CAUGHT, *extra: str) -> tuple[int, str]:
        return self.mutate("--file", "m/m.go", "--find", FIND, "--replace", replace, "--test", test, *extra)


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

    def setUp(self) -> None:
        super().setUp()
        self.log = os.path.join(self.tmp, "go.log")
        body = (
            "import sys\n"
            f"with open({self.log!r}, 'a', encoding='utf-8') as f:\n"
            "    f.write(' '.join(sys.argv) + '\\n')\n"
        )
        self.fake_env = {**self.env, "PATH": support.fake_go(self.tmp, body) + os.pathsep + self.env["PATH"]}

    def refused(self, *argv: str) -> str:
        code, out = self.mutate(*argv, env=self.fake_env)
        self.assertEqual(code, 2, out)
        self.assertTrue(out.splitlines()[-1].startswith("error: refused: "), out)
        self.assertFalse(os.path.exists(self.log), "a go command ran before the refusal")
        return out

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
        self.addCleanup(self.restore)
        mtime = os.stat(self.tracked).st_mtime_ns
        out = self.refused("--file", "m/m.go", "--find", FIND, "--replace", CAUGHT, "--test", "TestAdd")
        self.assertIn("differs from HEAD", out)
        with open(self.tracked, "rb") as f:
            self.assertEqual(f.read(), dirty)
        self.assertEqual(os.stat(self.tracked).st_mtime_ns, mtime)

    def restore(self) -> None:
        """This test dirtied the file itself; HEAD's bytes and the first mtime go back before the untouched check."""
        with open(self.tracked, "wb") as f:
            f.write(self.before[0])
        os.utime(self.tracked, ns=(self.before[1], self.before[1]))

    def test_an_untracked_file_is_refused(self):
        support.write(self.repo, {"m/n.go": "package m\n\nfunc N() int { return 1 }\n"})
        out = self.refused("--file", "m/n.go", "--find", "return 1", "--replace", "return 2", "--test", "TestAdd")
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

    def test_a_batch_entry_with_an_extra_key_is_refused_before_any_build(self):
        good = {"file": "m/m.go", "find": FIND, "replace": CAUGHT, "test": "TestAdd"}
        path = os.path.join(self.tmp, "batch.json")
        with open(path, "w", encoding="utf-8") as f:
            json.dump([good, {**good, "extra": 1}], f)
        out = self.refused("--batch", path)
        self.assertIn("entry 2", out)


class Batch(Base):
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
