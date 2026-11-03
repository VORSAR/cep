"""A stopped gate program stops what it started in sessions of its own (review BA1; break-it QB4).

Each case runs a real program in its own process that starts a child in a new session, holds it, and is then sent
SIGTERM alone, as the runner's watchdog would send it. The child holds a witness FIFO open, so its end shows as
end-of-file; it writes its pid first, so a child that outlived the stop is killed here whatever the verdict.
"""

import contextlib
import os
import signal
import subprocess
import sys

from gate import wait
from gate.runner.tests import support

CAP_S = 20  # a stopped child is gone within milliseconds


class StopTest(support.GateTest):
    def setUp(self):
        super().setUp()
        self.pidfile = os.path.join(self.root, "child.pid")
        self.addCleanup(self.kill_child)

    def program(self, body: str) -> subprocess.Popen[bytes]:
        """Start `body` (with REPO and ROOT defined) in its own process."""
        head = f"import os, signal, sys\nREPO, ROOT = {support.REPO!r}, {self.root!r}\nsys.path.insert(0, REPO)\n"
        path = os.path.join(self.root, "program.py")
        with open(path, "w", encoding="utf-8") as f:
            f.write(head + body)
        with open(os.path.join(self.root, "program.out"), "wb") as out:
            proc = subprocess.Popen([sys.executable, "-I", "-B", path], cwd=self.root, env=support.base_env(),
                                    stdin=subprocess.DEVNULL, stdout=out, stderr=subprocess.STDOUT)  # fmt: skip
        self.addCleanup(proc.wait)
        self.addCleanup(proc.kill)
        return proc

    def output(self) -> str:
        with open(os.path.join(self.root, "program.out"), "rb") as f:
            return f.read().decode("utf-8", errors="replace")

    def exit_code(self, proc: subprocess.Popen[bytes]) -> int:
        """The program's exit code; a program still running at the cap is killed and its output is the failure."""
        try:
            return proc.wait(timeout=wait.DEFAULT_CAP_S)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
            self.fail(f"the program did not exit within {wait.DEFAULT_CAP_S}s; its output:\n{self.output()}")

    def child(self, witness: str, on_term: str = "pass") -> str:
        """A child's code: write the pid, hold the witness, wait for ever; on TERM run `on_term`, then exit."""
        return (
            "import os, signal\n"
            f"def stop(s, f):\n    {on_term}\n    os._exit(0)\n"
            "signal.signal(signal.SIGTERM, stop)\n"
            f"open({self.pidfile!r}, 'w').write(str(os.getpid()))\n"
            f"os.write(os.open({witness!r}, os.O_WRONLY), b's')\n"
            "while True:\n    signal.pause()\n"
        )

    def kill_child(self) -> None:
        with contextlib.suppress(FileNotFoundError, ValueError), open(self.pidfile, encoding="ascii") as f:
            pid = int(f.read())
            with contextlib.suppress(ProcessLookupError):
                os.kill(pid, signal.SIGKILL)

    def assertGone(self, w: support.Witness) -> None:
        def at_eof() -> bool:
            try:
                return os.read(w.fd, 64) == b""
            except BlockingIOError:
                return False

        try:
            wait.until("the stopped program's child to be gone", at_eof, cap_s=CAP_S)
        except wait.WaitTimeout:
            self.fail(f"the child started in its own session is still running {CAP_S}s after its parent was stopped")


class DriverRunProcess(StopTest):
    def test_a_stopped_driver_passes_the_term_to_its_nested_gate_before_any_kill(self):
        # A nested gate stops its own rows, each in a session of its own, only if it is told: TERM first, then KILL.
        w = self.witness("start")
        termed = os.path.join(self.root, "termed")
        child = self.child(w.path, on_term=f"open({termed!r}, 'w').close()")
        proc = self.program(
            "from gate import term\n"
            "from gate.proofs.drivers import support\n"
            "term.install()\n"
            f"support.run_process([sys.executable, '-I', '-c', {child!r}], cwd=ROOT, env=dict(os.environ),\n"
            "                     out=os.path.join(ROOT, 'c.out'), err=os.path.join(ROOT, 'c.err'), cap_s=600)\n"
        )
        w.started(1)
        proc.send_signal(signal.SIGTERM)
        self.assertNotEqual(self.exit_code(proc), 0)
        self.assertGone(w)
        self.assertTrue(os.path.exists(termed), "the nested group was killed without the TERM that lets it clean up")


class StressHarness(StopTest):
    def test_a_stopped_harness_kills_its_hogs_and_starts_no_further_run(self):
        w = self.witness("start")
        go_on = self.fifo("go-on")
        seeds = os.path.join(self.root, "seeds")
        proc = self.program(
            "import contextlib\n"
            "from gate.runner import stress\n"
            "real_hogs = stress.hogs\n"
            "@contextlib.contextmanager\n"
            "def one_hog(count, python):\n"
            "    with real_hogs(1, python) as procs:\n"
            f"        open({self.pidfile!r}, 'w').write(str(procs[0].pid))\n"
            "        yield procs\n"
            "def run(seed, *rest):\n"
            f"    open({seeds!r}, 'a').write(f'{{seed}}\\n')\n"
            "    if seed == 0:  # only the first run holds; a later one, if it starts, shows in the seeds file\n"
            f"        os.write(os.open({w.path!r}, os.O_WRONLY), b's')\n"
            f"        os.read(os.open({go_on!r}, os.O_RDONLY), 1)  # held here until the test lets it go\n"
            "    return None\n"
            "stress.hogs, stress._one_run = one_hog, run\n"
            "sys.exit(stress.main(['--runs', '5', '--parallel', '1', 'gate.x.test_m']))\n"
        )
        w.started(1)
        with open(self.pidfile, encoding="ascii") as f:
            hog = int(f.read())
        proc.send_signal(signal.SIGTERM)
        support.release(go_on)  # the run in progress ends; the stop must keep the other four from starting
        self.assertNotEqual(self.exit_code(proc), 0)
        with open(seeds, encoding="ascii") as f:
            self.assertEqual(f.read(), "0\n", self.output())
        with self.assertRaises(ProcessLookupError):  # the harness reaped it: its pid is free
            os.kill(hog, 0)
