"""S1.1c-3's break-it (QA, blind) on BA10, the tree hash in a child. Each test is a finding, red until fixed."""

import errno
import os
import sys
import tempfile
import unittest
from unittest import mock

from gate.runner import proc
from gate.toolchain import manifest, parity

DIGEST = "ab" * 32


def a_tool() -> manifest.Tool:
    return manifest.Tool(
        name="go",
        version="1.0",
        command="go",
        path_dir="bin",
        version_argv=("version",),
        version_line="go1.0",
        url="https://example.invalid/go.tar.gz",
        archive_sha256="00" * 32,
        strip="go",
        exec_sha256={},
        tree_sha256=DIGEST,
    )


def refusing_spawn(argv, **_kw):
    raise OSError(errno.EMFILE, "Too many open files")


class TreeChildFailureIsNotADamagedTool(unittest.TestCase):
    """BK-S5: a tree child that cannot start, or that dies without hashing (a crash, a signal), is reported as
    `go: tree differs: run make clean-tools tools` — the damaged-tool reason, whose advice (re-download every tool)
    cannot help. `_tree_failure` turns every OSError from `tree_digest` (spawn's, and "the tree child exited <n>")
    into an empty digest. The design gives the cap its own reason for this very reason (§3.6, DA-S13e: "a slow disk is
    not a damaged tool: clean-tools would not help"); a child that never ran is not a damaged tool either, and the
    child's stderr (written to the scratch file) never reaches the reason."""

    def setUp(self) -> None:
        self.dir = tempfile.TemporaryDirectory(prefix="bk-s11c3-")
        self.addCleanup(self.dir.cleanup)
        self.home = os.path.join(self.dir.name, "home")
        os.makedirs(self.home)
        self.scratch = os.path.join(self.dir.name, "scratch")
        os.makedirs(self.scratch)

    def reason(self, spawn, argv) -> str | None:
        tree = parity.Tree(spawn=spawn, env={"PATH": os.defpath}, scratch=self.scratch)
        with mock.patch.object(parity, "TREE_CHILD", parity.TREE_CHILD if argv is None else tuple(argv)):
            return parity._tree_failure(a_tool(), self.home, tree)

    def test_a_child_that_cannot_start(self):
        got = self.reason(refusing_spawn, None)
        self.assertIsNotNone(got)
        self.assertNotIn("tree differs", got, "BK-S5: a spawn failure (EMFILE) is reported as a damaged tool")

    def test_a_child_that_crashes(self):
        got = self.reason(parity.popen, (sys.executable, "-I", "-B", "-c", "raise RuntimeError('the hasher broke')"))
        self.assertIsNotNone(got)
        self.assertNotIn(
            "tree differs", got, "BK-S5: a crashed tree child (exit 1, a traceback) is reported as a damaged tool"
        )

    def test_a_child_killed_by_a_signal(self):
        got = self.reason(
            parity.popen, (sys.executable, "-I", "-B", "-c", "import os, signal; os.kill(os.getpid(), signal.SIGKILL)")
        )
        self.assertIsNotNone(got)
        self.assertNotIn(
            "tree differs", got, "BK-S5: a tree child killed by a signal (-9) is reported as a damaged tool"
        )


class FinishedTreeHashLeavesNoLiveGroup(unittest.TestCase):
    """BK-L2: after a tree hash finishes, the runner's parity Group still holds the reaped child's pid as its live
    process group. `tree_digest` reaps with `proc.wait` (rightly never `Group.reap`, which has no cap), but nothing
    clears `Group._pgid`, which `Group.reap` clears under its lock; so a stop that arrives after a hash and before the
    next spawn TERMs, and `kill_left` probes and KILLs, a pid group the kernel may have handed to another process. The
    residue accepts the pid-reuse window of reap's probe (an instant); this window lasts until the next spawn."""

    def test_the_group_forgets_a_reaped_tree_child(self):
        with tempfile.TemporaryDirectory(prefix="bk-s11c3-") as d:
            home = os.path.join(d, "home")
            os.makedirs(home)
            group = proc.Group()
            got = parity.tree_digest(
                home,
                spawn=group.spawn,
                argv=(sys.executable, "-I", "-B", "-c", f"print({DIGEST!r})"),
                env={"PATH": os.defpath},
                scratch=d,
            )
            self.assertEqual(got, DIGEST)
            self.assertIsNone(
                group._pgid, "BK-L2: the parity group still names the reaped tree child's pid as its live group"
            )


if __name__ == "__main__":
    unittest.main()
