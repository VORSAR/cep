"""Process groups: spawn rules, the group probe after the leader exits (design note §4 steps 2-4)."""

import os
import signal
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
        # macOS answers EPERM for a group whose only member is caught mid-exit; it still counts, and is named.
        real = os.killpg

        def probe_says_eperm(pgid, sig):
            if sig == 0:
                raise PermissionError(1, "Operation not permitted")
            return real(pgid, sig)

        group = proc.Group()
        p = self.spawn(group, "status", 0)
        with mock.patch.object(proc.os, "killpg", probe_says_eperm):
            self.assertEqual(group.reap(p), (0, True))
        self.assertTrue(
            group.left_detail.startswith("the group exists but may not be signalled (EPERM): "), group.left_detail
        )

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


class Witnesses(support.GateTest):
    def test_end_of_file_without_start_bytes_is_refused(self):
        # design-QA counter-example 32: a fixture that never opens its FIFO must not pass on end-of-file.
        w = self.witness("start")
        with self.assertRaises(wait.WaitTimeout):
            w.started(1, cap_s=0)
        with self.assertRaises(AssertionError):
            w.gone()
