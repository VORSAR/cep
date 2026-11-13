"""candidate.build and candidate.structural_faults over the recorded feeds (S1.1c-4 design §4.3, §4.4; plan check §6
canary (1)-(8), (13); S1.1c-3 plan check §6 [4] (1)-(8)). The feeds are today's real responses, trimmed to the
fields the design reads (gate/canary/testdata/feeds/, fetched 2026-11-13); the pins are tools.json as it stood then
(testdata/pinned-tools.json). `archive` is the fake in support: never the network.

The line rule (owner, 2026-11-13, G1-Q1): only patch releases on each pinned line are tried; a newer line is listed in
Candidate.newer_lines and never downloaded.
"""

import copy
import json
import os
import re
import tempfile
import unittest

from gate.canary import candidate
from gate.canary.candidate import CanaryError, Upgrade
from gate.canary.tests import support as fs
from gate.toolchain import manifest

BUILD = "build 20261026 → 20261110"
RUFF_LINUX = "https://github.com/astral-sh/ruff/releases/download/0.17.5/ruff-x86_64-unknown-linux-gnu.tar.gz"
RUFF_LINUX_DIGEST = "7d261fc981a784668e19e9fa6ad437bfd95b67f2dc263fc1eab87d54d72fcc90"
PBS = "https://github.com/astral-sh/python-build-standalone/releases/download"
HEX64 = re.compile(r"[0-9a-f]{64}")


class CandidateTest(unittest.TestCase):
    def setUp(self) -> None:
        self.pins = fs.pinned()
        self.feeds = fs.feeds()
        self.archive = fs.FakeArchive(self, self.pins)

    def build(self) -> candidate.Candidate:
        return candidate.build(copy.deepcopy(self.pins), self.feeds, self.archive)

    def entry(self, cand: candidate.Candidate, name: str, platform: str = "linux-amd64") -> dict:
        return fs.tool(cand.tools, name)["platforms"][platform]

    def assert_loads(self, cand: candidate.Candidate) -> None:
        with tempfile.TemporaryDirectory(prefix="canary-candidate-") as d:
            path = os.path.join(d, "tools.json")
            with open(path, "w", encoding="utf-8") as f:
                json.dump(cand.tools, f)
            for platform in fs.PLATFORMS:
                with self.subTest(platform=platform):
                    self.assertEqual(len(manifest.load(path, platform)), 6)

    def assert_clean(self, cand: candidate.Candidate) -> None:
        upgraded = frozenset(u.name for u in cand.upgrades)
        self.assertEqual(candidate.structural_faults(self.pins, cand.tools, upgraded), [])

    def go_release(self, version: str, *, stable: bool = True) -> dict:
        files = [
            {
                "arch": arch,
                "filename": f"{version}.{goos}-{arch}.tar.gz",
                "kind": "archive",
                "os": goos,
                "sha256": fs.sha(f"{version}{goos}".encode()),
                "version": version,
            }
            for goos, arch in (("darwin", "arm64"), ("linux", "amd64"))
        ]
        return {"files": files, "stable": stable, "version": version}

    def pbs_release(self, tag: str, cpythons: list[str], **flags) -> dict:
        assets = []
        for v in cpythons:
            for triple in ("aarch64-apple-darwin", "x86_64-unknown-linux-gnu"):
                name = f"cpython-{v}+{tag}-{triple}-install_only.tar.gz"
                url = f"{PBS}/{tag}/cpython-{v}%2B{tag}-{triple}-install_only.tar.gz"
                assets.append({"browser_download_url": url, "digest": "sha256:" + fs.sha(url.encode()), "name": name})
        return {"assets": assets, "draft": False, "prerelease": False, "tag_name": tag, **flags}

    def pbs_prepend(self, release: dict) -> None:
        feed = [release, *self.feeds["python"]]
        self.feeds["python"] = feed
        self.feeds["python-floor"] = copy.deepcopy(feed)

    def lark_release(self, version: str, *, yanked: bool = False) -> str:
        url = f"https://files.pythonhosted.org/packages/0f/9e/{fs.sha(version.encode())[:60]}/lark-{version}.tar.gz"
        files = [
            {
                "digests": {"sha256": fs.sha(url.encode())},
                "filename": f"lark-{version}.tar.gz",
                "packagetype": "sdist",
                "url": url,
                "yanked": yanked,
            }
        ]
        self.feeds["lark"]["releases"][version] = files
        if not yanked:
            self.feeds["lark"]["info"]["version"] = version
            self.feeds["lark"]["urls"] = copy.deepcopy(files)
        return url


class TodaysFeeds(CandidateTest):
    """Case (1): the recorded feeds give ruff 0.17.5 and both Pythons' build 20261110 (plan check §0.12)."""

    def test_todays_upgrades_in_tools_json_order(self):
        self.assertEqual(
            self.build().upgrades,
            (
                Upgrade("python", "3.15.2", "3.15.2", BUILD),
                Upgrade("python-floor", "3.12.16", "3.12.16", BUILD),
                Upgrade("ruff", "0.17.3", "0.17.5"),
            ),
        )

    def test_todays_candidate_passes_manifest_load_on_both_platforms(self):
        self.assert_loads(self.build())

    def test_todays_candidate_has_no_structural_fault(self):
        self.assert_clean(self.build())

    def test_tools_at_their_pin_are_the_pins_and_never_downloaded(self):
        cand = self.build()
        for name in ("go", "golangci-lint", "lark"):
            with self.subTest(tool=name):
                self.assertEqual(fs.tool(cand.tools, name), fs.tool(self.pins, name))
        self.assertEqual({fs.tool_of(url) for url in self.archive.urls()}, {"python", "python-floor", "ruff"})

    def test_ruff_upgrade_entry_from_the_feed_and_the_archive(self):
        cand = self.build()
        ruff = fs.tool(cand.tools, "ruff")
        linux = ruff["platforms"]["linux-amd64"]
        self.assertEqual(ruff["version"], "0.17.5")
        self.assertEqual(linux["url"], RUFF_LINUX)
        self.assertEqual(linux["archive_sha256"], RUFF_LINUX_DIGEST)
        self.assertEqual(linux["strip"], "ruff-x86_64-unknown-linux-gnu/")
        self.assertEqual(linux["exec_sha256"], {"ruff": fs.sha(fs.member_data(RUFF_LINUX, "ruff"))})
        self.assertRegex(linux["tree_sha256"], HEX64)
        self.assertNotEqual(linux["tree_sha256"], fs.tool(self.pins, "ruff")["platforms"]["linux-amd64"]["tree_sha256"])
        self.assertIn((RUFF_LINUX, RUFF_LINUX_DIGEST), self.archive.calls)

    def test_pbs_build_tag_upgrade_url_and_name(self):
        # Review DQ-S7: the build tag sits twice in the url; 20261110 shipping the same 3.15.2 is an upgrade.
        cand = self.build()
        self.assertIn(Upgrade("python", "3.15.2", "3.15.2", BUILD), cand.upgrades)
        python = fs.tool(cand.tools, "python")
        self.assertEqual(python["version"], "3.15.2")
        self.assertEqual(
            python["platforms"]["linux-amd64"]["url"],
            f"{PBS}/20261110/cpython-3.15.2%2B20261110-x86_64-unknown-linux-gnu-install_only.tar.gz",
        )
        self.assertEqual(
            python["platforms"]["darwin-arm64"]["url"],
            f"{PBS}/20261110/cpython-3.15.2%2B20261110-aarch64-apple-darwin-install_only.tar.gz",
        )
        self.assertEqual(
            python["platforms"]["linux-amd64"]["archive_sha256"],
            "182e7adf7e4f0e32dafac5143f00d466afd20eef45918ea170f78884e503ac18",  # the feed's digest
        )
        self.assertEqual(python["platforms"]["linux-amd64"]["strip"], "python/")
        self.assertNotIn("20261026", json.dumps(python))

    def test_no_newer_line_on_todays_feeds_for_tools_at_their_latest_line(self):
        # go 1.27, golangci-lint v2.15, ruff 0.17, lark 1.3 are each upstream's newest line today; python's 3.16 is a
        # pre-release (3.16.0a2) and so is no line yet.
        for line in self.build().newer_lines:
            with self.subTest(line=line):
                self.assertFalse(line.split(" ")[0] in ("go", "golangci-lint", "ruff", "lark", "python"), line)


class LineRule(CandidateTest):
    """Patch releases on the pinned line only; a newer line listed, never tried (G1-Q1)."""

    def add_golangci(self, *tags: str) -> None:
        feed = self.feeds["golangci-lint"]
        new = [fs.gh_release(feed, "v2.15.0", t, rename=("2.15.0", t[1:])) for t in tags]
        self.feeds["golangci-lint"] = [*new, *feed]

    def test_newer_line_is_listed_never_tried(self):
        self.add_golangci("v2.16.0", "v2.15.1")
        cand = self.build()
        self.assertIn("golangci-lint v2.16.0", cand.newer_lines)
        self.assertIn(Upgrade("golangci-lint", "v2.15.0", "v2.15.1"), cand.upgrades)
        self.assertFalse([u for u in self.archive.urls() if "2.16.0" in u], self.archive.urls())

    def test_newer_line_alone_is_no_upgrade(self):
        self.add_golangci("v2.16.0")
        cand = self.build()
        self.assertIn("golangci-lint v2.16.0", cand.newer_lines)
        self.assertNotIn("golangci-lint", [u.name for u in cand.upgrades])
        self.assertEqual(fs.tool(cand.tools, "golangci-lint"), fs.tool(self.pins, "golangci-lint"))
        self.assertFalse([u for u in self.archive.urls() if "golangci" in u])

    def test_golangci_upgrade_strip_has_no_structural_fault(self):
        # Review DQ-S7: the version is v2.15.1, the strip and file name carry 2.15.1 without the v.
        self.add_golangci("v2.15.1")
        cand = self.build()
        lint = fs.tool(cand.tools, "golangci-lint")
        self.assertEqual(lint["version"], "v2.15.1")
        self.assertEqual(lint["platforms"]["linux-amd64"]["strip"], "golangci-lint-2.15.1-linux-amd64/")
        self.assertEqual(lint["platforms"]["darwin-arm64"]["strip"], "golangci-lint-2.15.1-darwin-arm64/")
        self.assertEqual(
            lint["platforms"]["linux-amd64"]["url"],
            "https://github.com/golangci/golangci-lint/releases/download/v2.15.1/golangci-lint-2.15.1-linux-amd64.tar.gz",
        )
        self.assert_clean(cand)
        self.assert_loads(cand)

    def test_go_patch_on_line_and_newer_line_listed(self):
        self.feeds["go"] = [self.go_release("go1.28.0"), self.go_release("go1.27.5"), *self.feeds["go"]]
        cand = self.build()
        self.assertIn(Upgrade("go", "go1.27.4", "go1.27.5"), cand.upgrades)
        go = fs.tool(cand.tools, "go")
        self.assertEqual(go["version"], "go1.27.5")
        self.assertEqual(go["platforms"]["linux-amd64"]["url"], "https://go.dev/dl/go1.27.5.linux-amd64.tar.gz")
        self.assertEqual(go["platforms"]["linux-amd64"]["archive_sha256"], fs.sha(b"go1.27.5linux"))
        self.assertEqual(go["platforms"]["linux-amd64"]["strip"], "go/")
        self.assertIn("go go1.28.0", cand.newer_lines)
        self.assertFalse([u for u in self.archive.urls() if "go1.28" in u])
        self.assert_clean(cand)

    def test_an_unstable_newer_line_is_not_listed(self):
        self.feeds["go"] = [self.go_release("go1.28rc1", stable=False), *self.feeds["go"]]
        self.assertFalse([line for line in self.build().newer_lines if "go1.28" in line])


class Stability(CandidateTest):
    """Case (2): never a pre-release, a draft, an unstable go, a CPython rc or a yanked package."""

    def test_prerelease_first_in_feed_takes_the_stable_on_the_line(self):
        self.feeds["ruff"] = [
            fs.gh_release(self.feeds["ruff"], "0.17.5", "0.17.6", prerelease=True),
            *self.feeds["ruff"],
        ]
        cand = self.build()
        self.assertIn(Upgrade("ruff", "0.17.3", "0.17.5"), cand.upgrades)
        self.assertFalse([u for u in self.archive.urls() if "0.17.6" in u])

    def test_draft_first_in_feed_is_skipped(self):
        self.feeds["ruff"] = [fs.gh_release(self.feeds["ruff"], "0.17.5", "0.17.6", draft=True), *self.feeds["ruff"]]
        self.assertIn(Upgrade("ruff", "0.17.3", "0.17.5"), self.build().upgrades)

    def test_go_not_stable_is_never_taken(self):
        self.feeds["go"] = [self.go_release("go1.27.5", stable=False), *self.feeds["go"]]
        cand = self.build()
        self.assertNotIn("go", [u.name for u in cand.upgrades])
        self.assertFalse([u for u in self.archive.urls() if "go1.27.5" in u])

    def test_pbs_cpython_rc_on_line_is_not_stable(self):
        # A non-prerelease pbs release shipping 3.15.3rc1: python stays on 3.15.2 (build 20261110); the floor's
        # 3.12.16 moves to the newer build.
        self.pbs_prepend(self.pbs_release("20261117", ["3.12.16", "3.15.3rc1"]))
        cand = self.build()
        self.assertIn(Upgrade("python", "3.15.2", "3.15.2", BUILD), cand.upgrades)
        self.assertIn(Upgrade("python-floor", "3.12.16", "3.12.16", "build 20261026 → 20261117"), cand.upgrades)
        self.assertNotIn("rc1", json.dumps(fs.tool(cand.tools, "python")))
        self.assertFalse([u for u in self.archive.urls() if "rc1" in u])

    def test_pbs_cpython_patch_move_is_named(self):
        # Case (7): a CPython patch move on the line is the upgrade's latest, the build tag moving with it.
        self.pbs_prepend(self.pbs_release("20261117", ["3.12.16", "3.15.3"]))
        cand = self.build()
        python = next(u for u in cand.upgrades if u.name == "python")
        self.assertEqual((python.pinned, python.latest), ("3.15.2", "3.15.3"))
        entry = fs.tool(cand.tools, "python")
        self.assertEqual(entry["version"], "3.15.3")
        self.assertEqual(
            entry["platforms"]["linux-amd64"]["url"],
            f"{PBS}/20261117/cpython-3.15.3%2B20261117-x86_64-unknown-linux-gnu-install_only.tar.gz",
        )
        self.assertEqual(entry["version_argv"], ["bin/python3.15", "--version"])
        self.assert_clean(cand)
        self.assert_loads(cand)

    def test_lark_url_from_feed(self):
        # Review DQ-S7: lark's url is a hash path no substitution can make; it and its digest come from the feed by
        # its file name, and a python package's version is its version_line.
        url = self.lark_release("1.3.2")
        cand = self.build()
        self.assertIn(Upgrade("lark", "1.3.1", "1.3.2"), cand.upgrades)
        lark = fs.tool(cand.tools, "lark")
        self.assertEqual((lark["version"], lark["version_line"]), ("1.3.2", "1.3.2"))
        for platform in fs.PLATFORMS:
            with self.subTest(platform=platform):
                entry = lark["platforms"][platform]
                self.assertEqual(entry["url"], url)
                self.assertEqual(entry["archive_sha256"], fs.sha(url.encode()))
                self.assertEqual(entry["strip"], "lark-1.3.2/")
        self.assert_clean(cand)
        self.assert_loads(cand)

    def test_lark_yanked_is_never_taken(self):
        self.lark_release("1.3.2", yanked=True)
        self.assertNotIn("lark", [u.name for u in self.build().upgrades])


class Refusals(CandidateTest):
    """Cases (3), (4), (6), each a named CanaryError; (8) the pins already latest."""

    def test_missing_digest_is_named(self):
        release = next(r for r in self.feeds["ruff"] if r["tag_name"] == "0.17.5")
        next(a for a in release["assets"] if "linux" in a["name"])["digest"] = None
        with self.assertRaises(CanaryError) as cm:
            self.build()
        self.assertEqual(str(cm.exception), "ruff linux-amd64: no published digest")

    def test_missing_go_digest_is_named(self):
        release = self.go_release("go1.27.5")
        del release["files"][0]["sha256"]
        self.feeds["go"] = [release, *self.feeds["go"]]
        with self.assertRaises(CanaryError) as cm:
            self.build()
        self.assertEqual(str(cm.exception), "go darwin-arm64: no published digest")

    def test_digest_mismatch_through_archive_is_named(self):
        error = CanaryError("ruff linux-amd64: the download's sha256 is not the published one")
        self.archive.fail[RUFF_LINUX] = error
        with self.assertRaises(CanaryError) as cm:
            self.build()
        self.assertEqual(str(cm.exception), "ruff linux-amd64: the download's sha256 is not the published one")

    def test_newest_on_the_line_older_than_the_pin_is_named(self):
        self.feeds["ruff"] = [r for r in self.feeds["ruff"] if r["tag_name"] not in ("0.17.3", "0.17.4", "0.17.5")]
        with self.assertRaises(CanaryError) as cm:
            self.build()
        self.assertEqual(str(cm.exception), "ruff: the newest on its line, 0.17.2, is older than the pin 0.17.3")

    def test_pins_already_latest_give_no_upgrades_and_the_pins(self):
        self.feeds["ruff"] = [r for r in self.feeds["ruff"] if r["tag_name"] not in ("0.17.4", "0.17.5")]
        self.feeds["python"] = self.feeds["python-floor"] = [
            r for r in self.feeds["python"] if r["tag_name"] != "20261110"
        ]
        cand = self.build()
        self.assertEqual(cand.upgrades, ())
        self.assertEqual(cand.tools, self.pins)
        self.assertEqual(self.archive.calls, [])


class Structural(unittest.TestCase):
    """structural_faults (design §4.4; review QA-S9): every difference but an upgraded tool's version, version_line,
    url, digests and substituted strip is named."""

    def setUp(self) -> None:
        self.pins = fs.pinned()
        self.cand = copy.deepcopy(self.pins)

    def faults(self, *upgraded: str) -> list[str]:
        return candidate.structural_faults(self.pins, self.cand, frozenset(upgraded))

    def upgrade_golangci(self, strip_version: str = "2.15.1") -> None:
        lint = fs.tool(self.cand, "golangci-lint")
        lint["version"] = "v2.15.1"
        lint["version_line"] = "golangci-lint has version 2.15.1 built with go1.27.4"
        for platform, entry in lint["platforms"].items():
            entry["url"] = entry["url"].replace("2.15.0", "2.15.1")
            entry["strip"] = f"golangci-lint-{strip_version}-{platform}/"
            entry["archive_sha256"] = entry["tree_sha256"] = "a" * 64
            entry["exec_sha256"] = {"golangci-lint": "b" * 64}

    def test_identical_documents_are_clean(self):
        self.assertEqual(self.faults(), [])

    def test_moved_path_dir_is_named(self):
        # Case (13): a moved path_dir is a break, never "pins to update" (the sort reads this list).
        fs.tool(self.cand, "ruff")["path_dir"] = "bin"
        self.cand_upgrade_ruff()
        faults = self.faults("ruff")
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].startswith("canary: ruff.path_dir changed: "), faults)
        self.assertIn("bin", faults[0])

    def cand_upgrade_ruff(self) -> None:
        ruff = fs.tool(self.cand, "ruff")
        ruff["version"] = "0.17.5"
        for entry in ruff["platforms"].values():
            entry["url"] = entry["url"].replace("0.17.3", "0.17.5")

    def test_upgraded_tool_value_changes_are_clean(self):
        self.upgrade_golangci()
        self.assertEqual(self.faults("golangci-lint"), [])

    def test_strip_not_the_substituted_pin_is_named(self):
        self.upgrade_golangci(strip_version="v2.15.1")  # the v form: not what the archive holds
        faults = self.faults("golangci-lint")
        self.assertEqual(len(faults), 2, faults)
        for fault in faults:
            self.assertTrue(fault.startswith("canary: golangci-lint."), fault)
            self.assertIn("strip", fault)

    def test_value_change_of_a_tool_not_upgraded_is_named(self):
        self.upgrade_golangci()
        faults = self.faults()
        self.assertTrue(faults)
        self.assertTrue(all(f.startswith("canary: golangci-lint.") for f in faults), faults)
        self.assertTrue([f for f in faults if ".version changed" in f], faults)

    def test_pbs_build_tag_in_both_url_places_is_clean(self):
        python = fs.tool(self.cand, "python")
        for entry in python["platforms"].values():
            entry["url"] = entry["url"].replace("20261026", "20261110")
        self.assertEqual(self.faults("python"), [])

    def test_exec_key_added_is_named(self):
        self.cand_upgrade_ruff()
        fs.tool(self.cand, "ruff")["platforms"]["linux-amd64"]["exec_sha256"]["ruff-server"] = "c" * 64
        faults = self.faults("ruff")
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].startswith("canary: ruff.platforms.linux-amd64.exec_sha256"), faults)
        self.assertIn("ruff-server", faults[0])

    def test_version_argv_change_is_named(self):
        self.cand_upgrade_ruff()
        fs.tool(self.cand, "ruff")["version_argv"] = ["ruff", "version"]
        faults = self.faults("ruff")
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].startswith("canary: ruff.version_argv changed: "), faults)

    def test_tool_removed_is_named(self):
        self.cand["tools"] = [t for t in self.cand["tools"] if t["name"] != "lark"]
        faults = self.faults()
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].startswith("canary: lark"), faults)

    def test_tool_added_is_named(self):
        extra = copy.deepcopy(fs.tool(self.pins, "ruff"))
        extra["name"] = "ruff-next"
        self.cand["tools"].append(extra)
        faults = self.faults()
        self.assertEqual(len(faults), 1, faults)
        self.assertTrue(faults[0].startswith("canary: ruff-next"), faults)

    def test_tool_moved_is_named(self):
        tools = self.cand["tools"]
        i, j = (next(k for k, t in enumerate(tools) if t["name"] == n) for n in ("go", "golangci-lint"))
        tools[i], tools[j] = tools[j], tools[i]
        faults = self.faults()
        self.assertTrue(faults)
        self.assertTrue(
            [f for f in faults if f.startswith("canary: go") or f.startswith("canary: golangci-lint")], faults
        )


if __name__ == "__main__":
    unittest.main()
