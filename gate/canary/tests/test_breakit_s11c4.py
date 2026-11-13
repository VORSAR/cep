"""S1.1c-4 break-it (QA with the-fool, step 7): the canary's sort on a red record that is more than its pins.

Red on the build at 1b06251 for the reason in its comment; the fix makes it green.
"""

import unittest

from gate.canary import sort

PIN = "gate.toolchain.tests.test_manifest.Load.test_the_committed_tools_json_loads_in_file_order_for_both_platforms"
PIN_TESTS = {PIN: r"AssertionError: Lists differ: \[(?:\('py|'/c/py)"}
PIN_TEXT = "AssertionError: Lists differ: [('python', 'linux/amd64'), ...] != [...]"


class BreakIt(unittest.TestCase):
    def test_bk_6_a_record_red_for_left_running_processes_is_a_break_even_when_its_failed_lines_are_pins(self):
        # Red today: sort reads a red record only through its `failures` (main._failures: the row's `failed:` lines,
        # printed by rows.finish as the row ends, and the unittest blocks behind them) and ignores its reason. The
        # runner's verdict.decide ranks "left running processes" (and a timeout, a stop, a kill by signal) above
        # the row's own exit code, so a candidate under which pytests prints its pin red, exits 1 and leaves a
        # child behind gives a record whose reason is the leak and whose failures are the pin: sorted "Pins to
        # update" with no break, the canary exits 0 and the leak is never named (design §4.5: a red record is pins
        # only when every failure in it is a pin; the leak is not one).
        record = {
            "id": "pytests",
            "job": "lint",
            "verdict": "fail",
            "reason": "left running processes: pid 4242 (python3.15)",
            "failures": [[PIN, PIN_TEXT]],
        }
        got = sort.sort([record], PIN_TESTS, [])
        self.assertTrue(
            any("left running processes" in str(b) for b in got.breaks),
            f"the leak is not a break: pins {list(got.pins)}, breaks {list(got.breaks)}",
        )
