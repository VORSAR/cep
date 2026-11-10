"""gotest.py's named-pattern check (the S3.3-2 combined review's R-L1): a row whose pattern is an anchored list of
test names, `^(A|B)$`, fails when one of the names never ran, though the package count alone would read green."""

import unittest

from gate.runner.rows import gotest


def events(*tests: str) -> list[dict]:
    out = []
    for t in tests:
        out += [{"Action": "run", "Package": "p", "Test": t}, {"Action": "pass", "Package": "p", "Test": t}]
    return out + [{"Action": "pass", "Package": "p"}]


class MissingNames(unittest.TestCase):
    def test_every_named_test_ran_is_clean(self):
        got = gotest.missing_names(
            "^(TestDeterminismGolden|TestBaseline)$", events("TestDeterminismGolden", "TestBaseline")
        )
        self.assertEqual(got, [])

    def test_a_named_test_that_never_ran_is_reported(self):
        got = gotest.missing_names("^(TestDeterminismGolden|TestBaseline)$", events("TestDeterminismGolden"))
        self.assertEqual(got, ["no test ran: TestBaseline"])

    def test_a_single_anchored_name(self):
        self.assertEqual(gotest.missing_names("^TestCorpus$", events()), ["no test ran: TestCorpus"])

    def test_a_sub_test_does_not_count_as_its_parent_running(self):
        evs = [{"Action": "run", "Package": "p", "Test": "TestBaseline/x"}]
        self.assertEqual(gotest.missing_names("^(TestBaseline)$", evs), ["no test ran: TestBaseline"])

    def test_a_pattern_that_is_not_a_name_list_checks_nothing(self):
        self.assertEqual(gotest.missing_names("Close|Inflight", events()), [])


if __name__ == "__main__":
    unittest.main()
