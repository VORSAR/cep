"""Process groups: spawn rules, the group probe after the leader exits (design note §4 steps 2-4)."""

import contextlib
import os
import signal
import subprocess
import sys
from unittest import mock

from gate import wait
from gate.runner import proc
from gate.runner.tests import support


class Group(support.GateTest):
    def spawn(self, group, *actions):
        out, err = os.path.join(self.root, "out"), os.path.join(self.root, "err")
        return group.spawn(support.fix(*actions), cwd=self.repo, env=support.base_env(), stdout=out, stderr=err)

    def read(self, name):
        with open(os.path.join(self.root, name), "rb") as f:
            return f.read()

    def test_a_step_gets_its_own_session_and_process_group(self):
        path = os.path.join(self.root, "ids")
        code = f"import os; open({path!r}, 'w').write(f'{{os.getpgid(0)}} {{os.getsid(0)}}')"
        group = proc.Group()
        p = group.spawn([sys.executable, "-I", "-c", code], cwd=self.repo, env=support.base_env(),
                        stdout=os.path.join(self.root, "out"), stderr=os.path.join(self.root, "err"))  # fmt: skip
        self.assertEqual(group.reap(p), (0, False))
        with open(path, encoding="utf-8") as f:
            self.assertEqual(f.read(), f"{p.pid} {p.pid}")

    def test_stdin_is_dev_null(self):
        path = os.path.join(self.root, "stdin")
        group = proc.Group()
        self.assertEqual(group.reap(self.spawn(group, "stdinfile", path))[0], 0)
        self.assertEqual(self.read("stdin"), b"|eof")

    def test_output_goes_to_the_named_files(self):
        group = proc.Group()
        code = "import sys; print('o'); print('e', file=sys.stderr)"
        p = group.spawn([sys.executable, "-I", "-c", code], cwd=self.repo, env=support.base_env(),
                        stdout=os.path.join(self.root, "out"), stderr=os.path.join(self.root, "err"))  # fmt: skip
        self.assertEqual(group.reap(p), (0, False))
        self.assertEqual((self.read("out"), self.read("err")), (b"o\n", b"e\n"))

    def test_the_exit_code_comes_back(self):
        group = proc.Group()
        self.assertEqual(group.reap(self.spawn(group, "status", 7)), (7, False))

    def test_a_signal_death_comes_back_negative(self):
        group = proc.Group()
        self.assertEqual(group.reap(self.spawn(group, "killself", 9)), (-9, False))

    def test_a_process_left_in_the_group_is_reported_and_killed(self):
        w = self.witness("start")
        group = proc.Group()
        p = self.spawn(group, "spawn", 3, "start", w.path, "block")
        w.started(1)
        self.assertEqual(group.reap(p), (0, True))
        w.gone()

    def test_a_survivor_is_named_with_the_answer_that_fired(self):
        # Known issue S1.1c-1-1 (owner, 2026-11-11): a positive probe names which answer fired and what was in the
        # group, so a rare "left running processes" explains itself; what counts as a failure is unchanged.
        w = self.witness("start")
        group = proc.Group()
        p = self.spawn(group, "spawn", 3, "start", w.path, "block")
        w.started(1)
        self.assertEqual(group.reap(p), (0, True))
        self.assertTrue(group.left_detail.startswith("the group still exists: "), group.left_detail)
        self.assertIn("rowfix.py", group.left_detail)
        w.gone()

    def test_a_group_that_may_not_be_signalled_is_named_so(self):
        # An EPERM answer with a live member still counts, and names the answer that fired.
        real = os.killpg

        def probe_says_eperm(pgid, sig):
            if sig == 0:
                raise PermissionError(1, "Operation not permitted")
            return real(pgid, sig)

        w = self.witness("start")
        group = proc.Group()
        p = self.spawn(group, "spawn", 3, "start", w.path, "block")
        w.started(1)
        with mock.patch.object(proc.os, "killpg", probe_says_eperm):
            self.assertEqual(group.reap(p), (0, True))
        self.assertTrue(
            group.left_detail.startswith("the group exists but may not be signalled (EPERM): "), group.left_detail
        )
        self.assertIn("rowfix.py", group.left_detail)
        w.gone()

    def test_a_clean_reap_names_nothing(self):
        group = proc.Group()
        self.assertEqual(group.reap(self.spawn(group, "status", 0)), (0, False))
        self.assertEqual(group.left_detail, "")

    def test_signal_reaches_every_process_in_the_group(self):
        w = self.witness("start")
        group = proc.Group()
        # Both the leader and the grandchild it starts hold the witness, and neither ends on its own.
        p = self.spawn(group, "start", w.path, "spawn", 3, "start", w.path, "block", "block")
        w.started(2)
        group.signal(signal.SIGKILL)
        self.assertEqual(group.reap(p)[0], -signal.SIGKILL)
        w.gone()

    def test_nothing_starts_after_the_group_is_signalled(self):
        group = proc.Group()
        group.signal(signal.SIGTERM)
        self.assertIsNone(self.spawn(group, "status", 0))

    def test_signalling_a_finished_step_is_harmless(self):
        group = proc.Group()
        group.reap(self.spawn(group, "status", 0))
        group.signal(signal.SIGTERM)

    def test_a_missing_command_raises_oserror(self):
        group = proc.Group()
        with self.assertRaises(OSError):
            group.spawn(["no-such-command-xyz"], cwd=self.repo, env=support.base_env(),
                        stdout=os.path.join(self.root, "out"), stderr=os.path.join(self.root, "err"))  # fmt: skip


class Probe(support.GateTest):
    """The group probe on its own (known issue S1.1c-1-1, owner 2026-11-12): a group whose members are all zombies,
    or that has none by the time `ps` looks, is finished; a live member is left running; a failing `ps` stays red.

    macOS answers EPERM to `killpg(pgid, 0)` for a group whose only members are zombies, Linux answers that the group
    exists; either way `ps` lists them with a state starting with Z, and that is what the rule reads."""

    def state(self, pid: int) -> str:
        """What `ps` says of one pid's state now, read without the code under test; "" once it is not listed."""
        _, out, _ = support.run_tool(["ps", "-o", "stat=", "-p", str(pid)], self.root)
        return out.decode("utf-8", errors="replace").strip()

    def start(self, argv: list[str]) -> subprocess.Popen[bytes]:
        """A process in its own session, which the test reaps itself (a kill first, for one still running)."""
        child = subprocess.Popen(
            argv,
            cwd=self.root,
            env=support.base_env(),
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )

        def reap() -> None:
            if child.returncode is None:
                with contextlib.suppress(ProcessLookupError, PermissionError):
                    os.killpg(child.pid, signal.SIGKILL)
            child.wait()

        self.addCleanup(reap)
        return child

    def zombie(self, child: subprocess.Popen[bytes]) -> None:
        """Wait, never reaping, until `ps` lists the child as a zombie; then its group still exists."""
        wait.until(f"pid {child.pid} listed by ps as a zombie", lambda: self.state(child.pid).startswith("Z"), cap_s=60)
        with contextlib.suppress(PermissionError):  # ProcessLookupError here would mean no group: no witness
            os.killpg(child.pid, 0)

    def test_a_group_whose_only_member_is_a_zombie_is_finished(self):
        child = self.start([sys.executable, "-I", "-c", "pass"])
        self.zombie(child)
        self.assertEqual(proc.probe(child.pid), (False, ""))
        self.assertTrue(self.state(child.pid).startswith("Z"))  # still unreaped: the probe saw the zombie
        self.assertEqual(child.wait(), 0)

    def test_a_live_member_is_left_running_and_named(self):
        w = self.witness("start")
        child = self.start(support.fix("start", w.path, "block"))
        w.started(1)
        left, detail = proc.probe(child.pid)
        self.assertTrue(left, detail)
        self.assertTrue(detail.startswith(f"the group still exists: {child.pid} "), detail)
        self.assertIn("rowfix.py", detail)

    def test_a_zombie_leader_with_a_live_member_is_left_running(self):
        w = self.witness("start")
        leader = self.start(support.fix("spawn", 3, "start", w.path, "block"))
        w.started(1)
        self.zombie(leader)
        left, detail = proc.probe(leader.pid)
        self.assertTrue(left, detail)
        self.assertIn(f"{leader.pid} Z", detail)
        self.assertIn("rowfix.py", detail)

    def test_no_group_at_all_is_finished(self):
        child = self.start([sys.executable, "-I", "-c", "pass"])
        self.zombie(child)  # the group existed
        self.assertEqual(child.wait(), 0)
        self.assertEqual(self.state(child.pid), "")
        self.assertEqual(proc.probe(child.pid), (False, ""))

    def test_a_ps_that_cannot_start_stays_left_running(self):
        child = self.start([sys.executable, "-I", "-c", "pass"])
        self.zombie(child)

        def no_ps(argv, **kwargs):
            raise FileNotFoundError(2, "No such file or directory", argv[0])

        with mock.patch.object(proc.subprocess, "run", no_ps):
            left, detail = proc.probe(child.pid)
        self.assertTrue(left, detail)
        self.assertIn(": ps failed: ", detail)

    def test_a_ps_that_exits_non_zero_stays_left_running(self):
        child = self.start([sys.executable, "-I", "-c", "pass"])
        self.zombie(child)

        def ps_fails(argv, **kwargs):
            return subprocess.CompletedProcess(argv, 1, b"", b"ps: no listing\n")

        with mock.patch.object(proc.subprocess, "run", ps_fails):
            left, detail = proc.probe(child.pid)
        self.assertTrue(left, detail)
        self.assertIn(": ps failed: ", detail)


class Witnesses(support.GateTest):
    def test_end_of_file_without_start_bytes_is_refused(self):
        # design-QA counter-example 32: a fixture that never opens its FIFO must not pass on end-of-file.
        w = self.witness("start")
        with self.assertRaises(wait.WaitTimeout):
            w.started(1, cap_s=0)
        with self.assertRaises(AssertionError):
            w.gone()
