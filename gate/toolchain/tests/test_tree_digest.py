"""BA10: the tree hash in a child process (S1.1c-3 design §3.6; owner, S1.1c-3's plan check, decision 10; DA-S12,
DA-S13, DQ-S11, DQ-S12).

The seam: `parity.TREE_CHILD` (the argv of the child that hashes one tool's tree), `parity.TREE_CAP_S` (how long it
may run), both read when called; `parity.tree_digest(home, *, spawn, cap_s=None, argv=None) -> TreeResult` starts
`[*argv, home]` through `spawn` and reads the digest the child prints; `parity.TreeCapExceeded(OSError)` past the
cap, after the child's own process group is killed. `spawn` has the shape of the runner's `Group.spawn` (argv, then
cwd, env, stdout and stderr by keyword; a Popen, or None once the group was signalled): the fake here accepts any
keywords and opens a path given for stdout or stderr, so it does not pin which of them `tree_digest` passes.

Red today by existence: none of the four names is in gate/toolchain/parity.py, and `parity.check` takes no spawn.
"""

import contextlib
import errno
import os
import signal
import subprocess
import sys
import unittest
from unittest import mock

from gate import wait
from gate.toolchain import parity
from gate.toolchain.tests import support
from gate.toolchain.tests.test_parity import RECOVER, ParityTest, write_file

# A child that prints fixed lines, as the hashing child prints the realpath it hashed, its details and its digest
# (proof-world speed-up design note §3.2); one whose realpath holds a newline, read from the end.
REAL = "/the/real/home"
DETAILS = "d0" * 32
DIGEST = "c0" * 32
PRINTS = f"print({REAL!r})\nprint({DETAILS!r})\nprint({DIGEST!r})\n"
PRINTS_NEWLINE = f"print('/a\\nb')\nprint({DETAILS!r})\nprint({DIGEST!r})\n"
# A child that starts a grandchild in its own process group, writes both pids to the marker (argv[1]), then waits
# on an event nothing sets. The grandchild does the same wait. Neither ever answers.
BLOCKS = """import os, subprocess, sys, threading
grandchild = subprocess.Popen([sys.executable, "-I", "-B", "-c", "import threading; threading.Event().wait()"])
with open(sys.argv[1] + ".tmp", "w") as f:
    f.write(f"{os.getpid()} {grandchild.pid}")
os.rename(sys.argv[1] + ".tmp", sys.argv[1])
threading.Event().wait()
"""
SMALL_CAP_S = 0.5  # the cap under test; the child is already up when it starts (the spawn waits for the marker)


def group_gone(pgid: int) -> bool:
    """No process of the group is left: macOS answers EPERM for a group of zombies only, so that is not gone."""
    try:
        os.killpg(pgid, 0)
    except ProcessLookupError:
        return True
    except PermissionError:
        return False
    return False


class Spawner:
    """A plain Popen in a new session, recording every argv; optionally holding its return until a marker exists."""

    def __init__(self, test: unittest.TestCase, *, marker: str | None = None, refuse: bool = False) -> None:
        self.test = test
        self.marker = marker
        self.refuse = refuse
        self.argvs: list[list[str]] = []
        self.procs: list[subprocess.Popen] = []

    def __call__(self, argv, **kw) -> subprocess.Popen | None:
        self.argvs.append(list(argv))
        if self.refuse:
            return None
        kw.update(stdin=subprocess.DEVNULL, start_new_session=True)
        if kw.get("env") is not None:
            kw["env"] = dict(kw["env"])
        with contextlib.ExitStack() as files:  # closed once the child holds them
            for name in ("stdout", "stderr"):
                if isinstance(kw.get(name), str):
                    kw[name] = files.enter_context(open(kw[name], "wb"))
            proc = subprocess.Popen(list(argv), **kw)
        self.procs.append(proc)
        self.test.addCleanup(support.kill_group, proc.pid)
        self.test.addCleanup(proc.wait)
        if self.marker is not None:
            wait.until(f"the tree child to write {self.marker}", lambda: os.path.exists(self.marker))
        return proc


def child(path: str, body: str) -> tuple[str, ...]:
    write_file(path, body.encode(), 0o444)
    return (sys.executable, "-I", "-B", path)


class TreeDigest(ParityTest):
    def test_the_child_spawned_is_tree_child_read_when_called(self):
        # DA-S12: a default argument bound at import would never see this reassignment.
        planted = child(os.path.join(self.world, "prints.py"), PRINTS)
        home = self.home("go")
        os.makedirs(home)
        spawn = Spawner(self)
        with mock.patch.object(parity, "TREE_CHILD", planted):
            got = parity.tree_digest(home, spawn=spawn)
        self.assertEqual(spawn.argvs, [[*planted, home]])
        self.assertEqual(got, parity.TreeResult(real=REAL, details=DETAILS, digest=DIGEST))

    def test_an_argv_given_is_the_child_spawned(self):
        planted = child(os.path.join(self.world, "prints.py"), PRINTS)
        home = self.home("go")
        os.makedirs(home)
        spawn = Spawner(self)
        self.assertEqual(parity.tree_digest(home, spawn=spawn, argv=planted), parity.TreeResult(REAL, DETAILS, DIGEST))
        self.assertEqual(spawn.argvs, [[*planted, home]])

    def test_the_lines_are_read_from_the_end_so_a_realpath_may_hold_a_newline(self):
        planted = child(os.path.join(self.world, "prints.py"), PRINTS_NEWLINE)
        home = self.home("go")
        os.makedirs(home)
        got = parity.tree_digest(home, spawn=Spawner(self), argv=planted)
        self.assertEqual(got, parity.TreeResult("/a\nb", DETAILS, DIGEST))

    def test_a_child_that_printed_fewer_than_three_lines_matches_no_pin(self):
        # A garbled answer is read as no digest at all, as a garbled digest always was: the tree differs.
        planted = child(os.path.join(self.world, "prints.py"), f"print({DIGEST!r})\n")
        home = self.home("go")
        os.makedirs(home)
        self.assertEqual(parity.tree_digest(home, spawn=Spawner(self), argv=planted), parity.TreeResult("", "", ""))

    def test_a_child_past_its_cap_is_killed_with_its_group_and_the_cap_named(self):
        # DQ-S12: the cap fires, the child's own process group is killed (never Group.signal), the error names it.
        marker = os.path.join(self.world, "pids")
        planted = (*child(os.path.join(self.world, "blocks.py"), BLOCKS), marker)
        home = self.home("go")
        os.makedirs(home)
        spawn = Spawner(self, marker=marker)
        with self.assertRaises(parity.TreeCapExceeded) as caught:
            parity.tree_digest(home, spawn=spawn, cap_s=SMALL_CAP_S, argv=planted)
        self.assertIsInstance(caught.exception, OSError)
        self.assertIn(str(SMALL_CAP_S), str(caught.exception))
        (proc,) = spawn.procs
        with open(marker, encoding="utf-8") as f:
            pid, grandchild = map(int, f.read().split())
        self.assertEqual(pid, proc.pid)
        wait.until(f"the tree child {pid} to be gone", lambda: proc.poll() is not None)
        self.assertEqual(proc.returncode, -signal.SIGKILL)
        wait.until(f"the tree child's group {pid} (grandchild {grandchild}) to be gone", lambda: group_gone(pid))


class TreeFailure(ParityTest):
    """`_tree_failure`, reached through `parity.check` at full tree with a spawn."""

    def test_a_hash_past_its_cap_is_its_own_reason_not_a_differing_tree(self):
        # DA-S13e: a slow disk is not a damaged tool; `make clean-tools tools` would not help.
        marker = os.path.join(self.world, "pids")
        planted = (*child(os.path.join(self.world, "blocks.py"), BLOCKS), marker)
        tool = self.tool("go")
        spawn = Spawner(self, marker=marker)
        with mock.patch.object(parity, "TREE_CHILD", planted), mock.patch.object(parity, "TREE_CAP_S", SMALL_CAP_S):
            reasons = parity.check([tool], self.cache, self.env, full_tree=True, spawn=spawn, scratch=self.scratch)
        self.assertEqual(reasons, [f"go: tree hash exceeded its cap of {SMALL_CAP_S} s"])
        self.assertNotIn(f"tree differs: {RECOVER}", reasons[0])
        (proc,) = spawn.procs
        wait.until(
            f"the tree child {proc.pid}'s group to be gone", lambda: proc.poll() is not None and group_gone(proc.pid)
        )

    def test_a_refused_spawn_is_named_stopped_before_the_tree_hash(self):
        # proc.py: Group.spawn returns None once the group was signalled; that is a stop, not a differing tree.
        tool = self.tool("go")
        spawn = Spawner(self, refuse=True)
        reasons = parity.check([tool], self.cache, self.env, full_tree=True, spawn=spawn, scratch=self.scratch)
        self.assertEqual(len(reasons), 1, reasons)
        self.assertTrue(reasons[0].startswith("go: "), reasons)
        self.assertIn("stopped before the tree hash", reasons[0])
        self.assertNotIn("tree differs", reasons[0])
        self.assertEqual(len(spawn.argvs), 1)

    def failure(self, spawn) -> list[str]:
        return parity.check([self.tool("go")], self.cache, self.env, full_tree=True, spawn=spawn, scratch=self.scratch)

    def planted_failure(self, body: str) -> list[str]:
        planted = child(os.path.join(self.world, "planted.py"), body)
        with mock.patch.object(parity, "TREE_CHILD", planted):
            return self.failure(Spawner(self))

    def test_a_tree_the_child_could_not_read_is_a_differing_tree(self):
        # BK-S5 / CA-L4: treehash's exit 3 (an OSError while reading the tree) is the one child failure that is a
        # damaged or unreadable tree, so it alone keeps the damaged-tool reason and its advice.
        reasons = self.planted_failure("import sys\nprint('Permission denied', file=sys.stderr)\nsys.exit(3)\n")
        self.assertEqual(reasons, [f"go: tree differs: {RECOVER}"])

    def test_the_real_child_on_an_unreadable_file_is_a_differing_tree(self):
        # The real treehash.py exits 3 on an OSError: the witness is its stderr in the scratch's tree-go.err.
        tool = self.tool("go")
        unreadable = os.path.join(self.home("go"), "bin", "unlisted")
        write_file(unreadable, b"not listed in exec_sha256\n", 0o000)
        reasons = parity.check([tool], self.cache, self.env, full_tree=True, spawn=Spawner(self), scratch=self.scratch)
        with open(os.path.join(self.scratch, "tree-go.err"), encoding="utf-8") as f:
            self.assertIn("Permission denied", f.read())
        self.assertEqual(reasons, [f"go: tree differs: {RECOVER}"])

    def test_a_child_that_exits_otherwise_is_named_with_its_exit_and_first_stderr_line(self):
        # BK-S5 / CA-L4: a crash (exit 1, a traceback, an import failure) is not a damaged tool.
        reasons = self.planted_failure("import sys\nsys.stderr.write('the hasher broke\\nand more\\n')\nsys.exit(1)\n")
        self.assertEqual(reasons, ["go: tree hash failed (exit 1): the hasher broke"])

    def test_a_child_killed_by_a_signal_is_named_with_its_exit(self):
        # Popen's returncode for a signal is the signal's number, negated; an empty stderr adds nothing.
        reasons = self.planted_failure("import os, signal\nos.kill(os.getpid(), signal.SIGKILL)\n")
        self.assertEqual(reasons, ["go: tree hash failed (exit -9)"])

    def test_a_child_that_could_not_start_is_named_with_the_error(self):
        # BK-S5: the spawner's own OSError (EMFILE, ENOMEM, a missing interpreter): no hash ran.
        def refuses(argv, **_kw):
            raise OSError(errno.EMFILE, "Too many open files")

        self.assertEqual(self.failure(refuses), ["go: tree hash could not start: [Errno 24] Too many open files"])

    def test_check_takes_spawn_and_scratch_as_required_keywords(self):
        # CA-L1 (design §10 C8): a caller that left spawn out would run the tree child outside the run's parity
        # group, where no stop reaches it (BA10). The witness: given both, an empty tool list passes.
        given = {"spawn": Spawner(self), "scratch": self.scratch}
        timing = parity.Timing()
        self.assertEqual(parity.check([], self.cache, self.env, full_tree=True, **given), [])
        self.assertEqual(parity._check([], self.cache, self.env, full_tree=True, timing=timing, **given), [])
        for missing in sorted(given):
            kw = {k: v for k, v in given.items() if k != missing}
            with self.subTest(missing=missing):
                with self.assertRaises(TypeError) as caught:
                    parity.check([], self.cache, self.env, full_tree=True, **kw)
                self.assertIn(f"'{missing}'", str(caught.exception))
                with self.assertRaises(TypeError) as caught:
                    parity._check([], self.cache, self.env, full_tree=True, timing=timing, **kw)
                self.assertIn(f"'{missing}'", str(caught.exception))

    def test_one_child_per_tool_and_the_real_child_agrees_with_the_pin(self):
        # testing.md rule 2: one child per tool, each given its own home; the real TREE_CHILD (treehash.py) prints
        # the digest the fixture pinned by hand, so check passes.
        tools = [self.tool("go"), self.tool("ruff")]
        spawn = Spawner(self)
        self.assertEqual(
            parity.check(tools, self.cache, self.env, full_tree=True, spawn=spawn, scratch=self.scratch), []
        )
        self.assertEqual(len(spawn.argvs), len(tools))
        self.assertEqual([argv[-1] for argv in spawn.argvs], [self.home("go"), self.home("ruff")])
        self.assertTrue(all(argv[:-1] == list(parity.TREE_CHILD) for argv in spawn.argvs), spawn.argvs)

    def test_no_child_below_full_tree(self):
        tool = self.tool("go")
        spawn = Spawner(self)
        self.assertEqual(
            parity.check([tool], self.cache, self.env, full_tree=False, spawn=spawn, scratch=self.scratch), []
        )
        self.assertEqual(spawn.argvs, [])


if __name__ == "__main__":
    unittest.main()
