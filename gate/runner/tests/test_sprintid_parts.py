"""The part order (S1.1c-3 plan check §0.15, §2 S2, §6 [3] "Aging" (7); owner 2026-11-12, S1.1c-3's plan check,
decision 8): `sprintid.Order.passed(part, marker)` says whether the sprint marker has passed a sprint part.

A part is past when the marker sits in a later row of the printed order, or in a later part of the same row, the
parts' numbers compared as integers (S1.1c-10 follows S1.1c-3; a string sort says the opposite). Aging's part rule
and the allowlists' inline part check (`allowlists.py` `sprint_fault`) move onto it. The order here is a fixture in
gate/sprints.json's shape, so no case depends on today's closed marks; nothing reads the real tree.

design question (part-less marker): the plan names "a part-less marker where a part is needed" a named red
(§2 S2) without saying at which layer. These cases pin the pure function's conservative answer, False (S1.1c is
read as n = 0, before every part of its row, as `allowlists.sprint_fault` reads a part-less current sprint today), and
test_aging.py pins the named red at aging's layer. If the design makes `passed` raise instead, this case changes at G1.
"""

import unittest

from gate.runner import sprintid

ORDER = sprintid.Order(
    tuple(
        sprintid.Row(id, closed)
        for id, closed in (
            ("S1.1a-1", True),
            ("S3.3", True),
            ("S1.1c", False),
            ("S4.1", False),
            ("S10", False),
        )
    )
)


class Passed(unittest.TestCase):
    def test_a_later_part_of_the_same_row_has_passed_the_part(self):
        self.assertIs(ORDER.passed("S1.1c-3", "S1.1c-4"), True)

    def test_the_same_part_has_not_passed_itself(self):
        self.assertIs(ORDER.passed("S1.1c-3", "S1.1c-3"), False)

    def test_an_earlier_part_of_the_same_row_has_not_passed_the_part(self):
        self.assertIs(ORDER.passed("S1.1c-4", "S1.1c-3"), False)

    def test_parts_are_compared_as_numbers(self):
        # A string comparison puts "S1.1c-10" before "S1.1c-3": it would call S1.1c-10 passed at S1.1c-3.
        self.assertIs(ORDER.passed("S1.1c-10", "S1.1c-3"), False)
        self.assertIs(ORDER.passed("S1.1c-3", "S1.1c-10"), True)
        self.assertIs(ORDER.passed("S1.1c-9", "S1.1c-10"), True)

    def test_a_part_of_an_earlier_row_is_passed_by_a_later_rows_marker(self):
        self.assertIs(ORDER.passed("S3.3-2", "S1.1c-3"), True)
        self.assertIs(ORDER.passed("S1.1c-5", "S4.1"), True)

    def test_the_printed_order_decides_not_the_numbers(self):
        # S3.3 is printed before S1.1c: by stage numbers S1.1c-3 would come first.
        self.assertIs(ORDER.passed("S1.1c-3", "S3.3-2"), False)
        self.assertIs(ORDER.passed("S4.1-1", "S10"), True)

    def test_a_part_of_a_later_row_is_not_passed(self):
        self.assertIs(ORDER.passed("S4.1-2", "S1.1c-3"), False)

    def test_a_part_less_marker_in_the_parts_row_has_not_passed_it(self):
        """The conservative reading (the module docstring's design question): S1.1c is before S1.1c-1."""
        self.assertIs(ORDER.passed("S1.1c-1", "S1.1c"), False)
        self.assertIs(ORDER.passed("S1.1c-3", "S1.1c"), False)

    def test_a_part_whose_row_is_not_in_the_order_is_an_error_naming_it(self):
        with self.assertRaises(ValueError) as caught:
            ORDER.passed("S7.7-1", "S1.1c-3")
        self.assertIn("S7.7-1", str(caught.exception))

    def test_a_marker_whose_row_is_not_in_the_order_is_an_error_naming_it(self):
        with self.assertRaises(ValueError) as caught:
            ORDER.passed("S1.1c-3", "S7.7-1")
        self.assertIn("S7.7-1", str(caught.exception))

    def test_text_that_is_not_a_sprint_id_is_an_error_naming_it(self):
        for part, marker in (("S1.1c-03", "S1.1c-4"), ("S1.1c-3", "s1.1c-4")):
            with self.subTest(part=part, marker=marker):
                with self.assertRaises(ValueError) as caught:
                    ORDER.passed(part, marker)
                said = str(caught.exception)
                self.assertTrue(part in said or marker in said, said)


if __name__ == "__main__":
    unittest.main()
