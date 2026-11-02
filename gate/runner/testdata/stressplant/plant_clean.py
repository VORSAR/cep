"""The control: tests with no dependence on order or seed."""

import unittest


class Clean(unittest.TestCase):
    def test_one(self):
        self.assertEqual(1 + 1, 2)

    def test_two(self):
        self.assertEqual(sorted([2, 1]), [1, 2])
