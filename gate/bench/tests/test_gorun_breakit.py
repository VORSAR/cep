"""Break-it (S1.1c-6 combined build): gorun's group kill after the leader exits is skipped by a stop in that window."""

import os
import signal
import subprocess
import sys
import tempfile
import unittest

from gate import wait

CAP_S = 20.0
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

# A leader that starts a grandchild in its own group, writes both pids and exits 0 at once, leaving the grandchild.
LEADER = """\
import os, subprocess, sys
g = subprocess.Popen([sys.executable, "-I", "-c", "import signal\\nwhile True:\\n    signal.pause()"])
with open(sys.argv[1] + ".tmp", "w") as f:
    f.write(f"{os.getpid()} {g.pid}")
os.rename(sys.argv[1] + ".tmp", sys.argv[1])
"""


def gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    return False


class StopAfterTheLeaderExits(unittest.TestCase):
    def test_a_stop_after_the_leader_exits_still_kills_its_group(self):
        # The module docstring: "a group left behind by a leader that exited is killed too. A TERM or an interrupt of
        # the caller kills the child's group before the stop goes on." The driver's first _killpg call (the post-wait
        # group KILL; no _stop ran, since the leader exited by itself) first sends the driver its own TERM, which
        # term.install() raises as Terminated at the next call boundary, before os.killpg runs. The driver exits 3
        # when the stop comes out of run as Terminated; the grandchild must still be gone.
        with tempfile.TemporaryDirectory() as tmp:
            pids = os.path.join(tmp, "pids")
            driver = (
                "import os, signal, sys\n"
                f"sys.path.insert(0, {REPO!r})\n"
                "from gate import term\n"
                "from gate.bench import gorun\n"
                "real_killpg = gorun._killpg\n"
                "calls = []\n"
                "def killpg_after_a_stop(pgid, sig):\n"
                "    calls.append(sig)\n"
                "    if len(calls) == 1:\n"
                "        os.kill(os.getpid(), signal.SIGTERM)\n"
                "    real_killpg(pgid, sig)\n"
                "gorun._killpg = killpg_after_a_stop\n"
                "term.install()\n"
                "try:\n"
                f"    gorun.run([sys.executable, '-I', '-c', {LEADER!r}, {pids!r}], cwd={tmp!r}, timeout_s=600)\n"
                "except term.Terminated:\n"
                "    sys.exit(3)\n"
            )
            err_path = os.path.join(tmp, "driver.stderr")
            with open(err_path, "wb") as err:
                proc = subprocess.Popen(
                    [sys.executable, "-I", "-B", "-c", driver], stdin=subprocess.DEVNULL, stdout=err, stderr=err
                )
            code = proc.wait(timeout=CAP_S)
            with open(err_path, "rb") as f:
                self.assertEqual(code, 3, f.read().decode("utf-8", errors="replace"))
            with open(pids, encoding="ascii") as f:
                leader, grandchild = (int(p) for p in f.read().split())
            try:
                wait.until("the exited leader's grandchild gone", lambda: gone(grandchild), cap_s=5.0)
            finally:
                if not gone(grandchild):
                    os.kill(grandchild, signal.SIGKILL)
            self.assertTrue(gone(leader))


if __name__ == "__main__":
    unittest.main()
