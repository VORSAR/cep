"""Planted: passes only for string hash seeds that make this string's hash even."""

import unittest


class Hash(unittest.TestCase):
    def test_even(self):
        self.assertEqual(hash("stress plant") % 2, 0)
