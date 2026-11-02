"""gate/wait.py: every wait names its state and has a cap (testing.md rule 1)."""

import queue
import unittest

from gate import wait


class FakeTime:
    """A clock that only moves when the code under test sleeps."""

    def __init__(self):
        self.t = 100.0
        self.slept = []

    def now(self):
        return self.t

    def sleep(self, s):
        self.slept.append(s)
        self.t += s


class Until(unittest.TestCase):
    def test_returns_the_first_truthy_value(self):
        ft = FakeTime()
        values = iter([None, 0, "ready"])
        got = wait.until("the marker", lambda: next(values), cap_s=10, poll_s=1, now=ft.now, sleep=ft.sleep)
        self.assertEqual(got, "ready")
        self.assertEqual(ft.slept, [1, 1])

    def test_a_state_already_true_never_sleeps(self):
        ft = FakeTime()
        self.assertTrue(wait.until("the marker", lambda: True, cap_s=10, now=ft.now, sleep=ft.sleep))
        self.assertEqual(ft.slept, [])

    def test_the_cap_raises_and_names_the_state(self):
        ft = FakeTime()
        with self.assertRaises(wait.WaitTimeout) as caught:
            wait.until("row X to start", lambda: False, cap_s=3, poll_s=1, now=ft.now, sleep=ft.sleep)
        self.assertIn("row X to start", str(caught.exception))
        self.assertIn("3", str(caught.exception))
        self.assertEqual(ft.slept, [1, 1, 1])

    def test_a_zero_cap_checks_exactly_once(self):
        ft = FakeTime()
        calls = []
        with self.assertRaises(wait.WaitTimeout):
            wait.until("anything", lambda: calls.append(1), cap_s=0, now=ft.now, sleep=ft.sleep)
        self.assertEqual(calls, [1])
        self.assertEqual(ft.slept, [])

    def test_the_last_sleep_never_overshoots_the_cap(self):
        ft = FakeTime()
        with self.assertRaises(wait.WaitTimeout):
            wait.until("anything", lambda: False, cap_s=2.5, poll_s=1, now=ft.now, sleep=ft.sleep)
        self.assertEqual(ft.slept, [1, 1, 0.5])

    def test_the_state_must_be_named(self):
        with self.assertRaises(ValueError):
            wait.until("", lambda: True, cap_s=1)


class Get(unittest.TestCase):
    def test_returns_an_item_already_queued(self):
        q = queue.SimpleQueue()
        q.put("x")
        self.assertEqual(wait.get(q, "the x event", cap_s=1), "x")

    def test_the_cap_raises_and_names_the_state(self):
        with self.assertRaises(wait.WaitTimeout) as caught:
            wait.get(queue.SimpleQueue(), "the x event", cap_s=0)
        self.assertIn("the x event", str(caught.exception))

    def test_the_state_must_be_named(self):
        with self.assertRaises(ValueError):
            wait.get(queue.SimpleQueue(), "", cap_s=1)


if __name__ == "__main__":
    unittest.main()
