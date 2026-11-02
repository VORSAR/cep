"""Planted: one test passes only if the other ran first, in the same process."""

import unittest

RAN = []


class Order(unittest.TestCase):
    def test_first(self):
        RAN.append(1)

    def test_needs_first(self):
        self.assertTrue(RAN)
