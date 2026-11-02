"""Planted: tests that report OK to plain unittest, yet prove nothing."""

import socket
import unittest


class Vacuous(unittest.TestCase):
    def test_leak(self):
        socket.socket()

    @unittest.skip("planted")
    def test_skip(self):
        pass

    @unittest.expectedFailure
    def test_expected_failure(self):
        self.fail("planted")
