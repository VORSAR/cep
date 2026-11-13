"""Lane F1's own candidate tests (S1.1c-4 design §4.3, as clarified on integration at ac146bd from QA's W0c point 8):
python-floor's minor is the bootstrap's floor on purpose, so a newer CPython minor is never its candidate and never
listed in newer_lines; its patch and build-tag moves within 3.12 are upgrades as for python. Never the network."""

import copy
import unittest

from gate.canary import candidate
from gate.canary.tests import feedsupport as fs


class FloorHasNoNewerLine(unittest.TestCase):
    def setUp(self) -> None:
        self.pins = fs.pinned()
        self.feeds = fs.feeds()
        self.archive = fs.FakeArchive(self, self.pins)

    def test_the_floor_lists_no_newer_line_though_the_feed_offers_one(self):
        offered = {
            a["name"].split("+")[0]
            for release in self.feeds["python-floor"]
            if not release["prerelease"] and not release["draft"]
            for a in release["assets"]
        }
        self.assertIn("cpython-3.15.2", offered, "the floor's feed offers a newer stable minor")
        cand = candidate.build(copy.deepcopy(self.pins), self.feeds, self.archive)
        self.assertIn("python-floor", [u.name for u in cand.upgrades], "the floor's build move is still an upgrade")
        self.assertEqual([line for line in cand.newer_lines if line.startswith("python-floor ")], [])


if __name__ == "__main__":
    unittest.main()
