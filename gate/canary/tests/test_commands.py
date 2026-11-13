"""The runner's three canary commands (S1.1c-4 design §4.7; reviews DA-S7, DA-S8, DQ-S9): thin calls into
gate/canary, each CanaryError written to <dir>/error.txt with exit 2. Never the network: the feeds are F1's
recorded ones, the archives F1's fake seam."""

import contextlib
import email.message
import io
import json
import os
import tempfile
import unittest

from gate.canary.candidate import CanaryError
from gate.canary.tests import support as canary_support
from gate.runner import main
from gate.toolchain import manifest
from gate.toolchain.tests import support

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
MANIFEST_ID = (
    "gate.toolchain.tests.test_manifest.Load.test_the_committed_tools_json_loads_in_file_order_for_both_platforms"
)
FAIL_BLOCK = """\
======================================================================
FAIL: test_the_committed_tools_json_loads_in_file_order_for_both_platforms ({id}) (platform='{platform}')
----------------------------------------------------------------------
Traceback (most recent call last):
  File "/x/gate/toolchain/tests/test_manifest.py", line 152, in test_the_committed_tools_json_loads_in_file_order
    self.assertEqual(
AssertionError: Lists differ: ['/c/python-3.15.2-6767ca67fdcf', '/c/python-floor'] != ['/c/python-3.15.2-285a6b924713']

First differing element 0:
'/c/python-3.15.2-6767ca67fdcf'
"""
EXCEPTION_LINE = (
    "AssertionError: Lists differ: ['/c/python-3.15.2-6767ca67fdcf', '/c/python-floor'] != "
    "['/c/python-3.15.2-285a6b924713']"
)
# One pin test, as lane F1 measured it (gate/canary/pin_tests.json, 2026-11-13).
PIN_TESTS = {
    "schema": 1,
    "note": "the summary tests' own",
    "entries": [
        {
            "id": MANIFEST_ID,
            "row": "pyfloor",
            "pattern": "AssertionError: Lists differ: \\[(?:\\('py|'/c/py)",
            "measured": "2026-11-13",
        },
    ],
}
TAIL = """\
----------------------------------------------------------------------
Ran 45 tests in 2.305s

FAILED (failures=2)
"""


def write(path: str, text: str) -> str:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)
    return path


def read(path: str) -> str:
    with open(path, encoding="utf-8") as f:
        return f.read()


def run(command, *args, **kwargs) -> tuple[int, str]:
    out = io.StringIO()
    with contextlib.redirect_stdout(out):
        code = command(*args, **kwargs)
    return code, out.getvalue()


def recorded_get(url: str, headers) -> tuple[int, email.message.Message, bytes]:
    feeds = (("go.dev", "go"), ("golangci", "golangci-lint"), ("ruff", "ruff"), ("pypi", "lark"))
    name = next((tool for needle, tool in feeds if needle in url), "python")
    with open(os.path.join(canary_support.FEEDS, canary_support.FEED_FILE[name]), "rb") as f:
        return 200, email.message.Message(), f.read()


class Feeds(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo, self.out = tmp.name, os.path.join(tmp.name, "candidate")
        write(os.path.join(self.repo, "tools.json"), json.dumps(canary_support.pinned()))

    def test_every_tools_feed_is_written(self):
        code, said = run(
            main.canary_feeds,
            ["--out", os.path.join(self.out, "feeds.json")],
            repo=self.repo,
            environ={"GITHUB_TOKEN": "t"},
            get=recorded_get,
        )
        self.assertEqual(code, 0, said)
        self.assertTrue(said.endswith("GATE PASSED\nexit: 0\n"), said)
        self.assertEqual(json.loads(read(os.path.join(self.out, "feeds.json"))), canary_support.feeds())
        self.assertFalse(os.path.exists(os.path.join(self.out, "error.txt")))

    def test_a_spent_rate_limit_is_written_to_error_txt_and_exits_2(self):
        def spent(url, headers):
            if "github" not in url:
                return recorded_get(url, headers)
            answer = email.message.Message()
            answer["x-ratelimit-remaining"], answer["x-ratelimit-limit"], answer["x-ratelimit-reset"] = "0", "1000", "0"
            return 403, answer, b"{}"

        code, said = run(
            main.canary_feeds,
            ["--out", os.path.join(self.out, "feeds.json")],
            repo=self.repo,
            environ={"GITHUB_TOKEN": "t"},
            get=spent,
        )
        self.assertEqual(code, 2, said)
        self.assertTrue(said.endswith("GATE FAILED\nexit: 2\n"), said)
        want = "python: GitHub's rate limit (1000/h) is spent until 1970-01-01T00:00:00Z\n"
        self.assertEqual(read(os.path.join(self.out, "error.txt")), want)
        self.assertFalse(os.path.exists(os.path.join(self.out, "feeds.json")))

    def test_no_token_is_an_error_txt(self):
        code, _ = run(
            main.canary_feeds,
            ["--out", os.path.join(self.out, "feeds.json")],
            repo=self.repo,
            environ={},
            get=recorded_get,
        )
        self.assertEqual(code, 2)
        self.assertIn("GITHUB_TOKEN", read(os.path.join(self.out, "error.txt")))


class Candidate(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = tmp.name
        self.pins = canary_support.pinned()
        write(os.path.join(self.tmp, "tools.json"), json.dumps(self.pins))
        self.feeds = write(os.path.join(self.tmp, "candidate", "feeds.json"), json.dumps(canary_support.feeds()))
        self.out = os.path.join(self.tmp, "candidate")

    def test_the_candidate_and_its_upgrades_are_written_with_run_version_lines(self):
        ran = []

        def version_line(tool, archive, strip, platform, scratch):
            self.assertTrue(os.path.isfile(archive))
            ran.append((tool["name"], strip, platform))
            return f"{tool['name']} as run"

        code, said = run(
            main.canary_candidate,
            ["--feeds", self.feeds, "--out", self.out + "/"],
            repo=self.tmp,
            archive=canary_support.FakeArchive(self, self.pins),
            version_line=version_line,
        )
        self.assertEqual(code, 0, said)
        platform = manifest.platform_key()
        # The recorded feeds' upgrades (F1's measurement, 2026-11-13): both pythons' build tag, ruff's patch.
        self.assertEqual([name for name, _, _ in ran], ["python", "python-floor", "ruff"])
        self.assertEqual({p for _, _, p in ran}, {platform})
        upgrades = json.loads(read(os.path.join(self.out, "upgrades.json")))
        self.assertEqual(
            upgrades["upgrades"],
            [
                {"latest": "3.15.2", "name": "python", "note": "build 20261026 → 20261110", "pinned": "3.15.2"},
                {"latest": "3.12.16", "name": "python-floor", "note": "build 20261026 → 20261110", "pinned": "3.12.16"},
                {"latest": "0.17.5", "name": "ruff", "note": "", "pinned": "0.17.3"},
            ],
        )
        self.assertIsInstance(upgrades["newer_lines"], list)
        path = os.path.join(self.out, "tools.json")
        tools = {t["name"]: t for t in json.loads(read(path))["tools"]}
        self.assertEqual(tools["ruff"]["version_line"], "ruff as run")
        self.assertEqual(tools["go"]["version_line"], canary_support.tool(self.pins, "go")["version_line"])
        for p in canary_support.PLATFORMS:
            self.assertEqual(len(manifest.load(path, p)), len(self.pins["tools"]))
        self.assertFalse(os.path.exists(os.path.join(self.out, "error.txt")))

    def test_a_canary_error_through_main_is_error_txt_and_exit_2(self):
        empty = write(os.path.join(self.tmp, "empty.json"), "{}")
        code, said = run(main.main, ["canary-candidate", "--feeds", empty, "--out", self.out + "/"])
        self.assertEqual(code, 2, said)
        self.assertTrue(said.endswith("GATE FAILED\nexit: 2\n"), said)
        self.assertEqual(read(os.path.join(self.out, "error.txt")), "python: no feed was fetched\n")
        self.assertFalse(os.path.exists(os.path.join(self.out, "tools.json")))

    def test_the_version_line_is_the_archives_answer_with_gos_template_kept(self):
        for name, argv, pinned, answer, want in (
            (
                "go",
                ["bin/go", "version"],
                "go version go1.27.5 {goos}/{goarch}",
                "go version go1.27.5 linux/amd64",
                "go version go1.27.5 {goos}/{goarch}",
            ),
            ("ruff", ["ruff", "--version"], "ruff 0.17.5", "ruff 0.17.5", "ruff 0.17.5"),
        ):
            with self.subTest(name):
                script = f"#!/bin/sh\necho '{answer}'\n".encode()
                archive = support.write_archive(
                    os.path.join(self.tmp, f"{name}.tar.gz"),
                    [
                        support.Member(f"{name}/", "d", mode=0o755),
                        support.Member(f"{name}/{argv[0]}", data=script, mode=0o755),
                    ],
                )
                tool = {"name": name, "version_argv": argv, "version_line": pinned}
                self.assertEqual(main._version_line(tool, archive, f"{name}/", "linux-amd64", self.tmp), want)

    def test_a_version_argv_that_fails_is_a_canary_error(self):
        archive = support.write_archive(
            os.path.join(self.tmp, "bad.tar.gz"),
            [support.Member("bad/", "d", mode=0o755), support.Member("bad/x", data=b"#!/bin/sh\nexit 3\n", mode=0o755)],
        )
        with self.assertRaisesRegex(CanaryError, r"^bad linux-amd64: x --version exit 3"):
            main._version_line(
                {"name": "bad", "version_argv": ["x", "--version"], "version_line": "x"},
                archive,
                "bad/",
                "linux-amd64",
                self.tmp,
            )


class Summary(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = tmp.name
        self.repo = os.path.join(self.tmp, "repo")  # the summary's own world: tools.json and pin_tests.json
        with open(os.path.join(REPO, "tools.json"), encoding="utf-8") as f:
            self.tools = write(os.path.join(self.repo, "tools.json"), f.read())
        write(os.path.join(self.repo, "gate", "canary", "pin_tests.json"), json.dumps(PIN_TESTS))
        self.artifacts = os.path.join(self.tmp, "artifacts")
        self.candidate = os.path.join(self.tmp, "candidate")
        self.summary = write(os.path.join(self.tmp, "step-summary.md"), "# before\n")
        os.makedirs(self.artifacts)

    def summarize(self, needs: dict) -> tuple[int, str]:
        argv = [
            "--artifacts",
            self.artifacts,
            "--candidate",
            self.candidate,
            "--attempt",
            "1",
            "--summary",
            self.summary,
        ]
        return run(main.canary_summary, argv, repo=self.repo, environ={"NEEDS": json.dumps(needs)})

    def record(self, job: str, row: str, verdict: str, stdout: str, stderr: dict[str, str]) -> None:
        base = os.path.join(self.artifacts, f"records-{job}-1")
        record = {"id": row, "job": job, "verdict": verdict, "reason": f"exit 1: {row} failed", "blocking": True}
        write(os.path.join(base, "records", job, f"{row}.json"), json.dumps(record))
        write(os.path.join(base, "rows", job, row, "stdout"), stdout)
        for module, text in stderr.items():
            write(os.path.join(base, "rows", job, row, "py", f"{module}.stderr"), text)

    def test_a_failed_candidate_job_names_its_error_txt_first(self):
        # Reviews DA-S8, DQ-S9: the rate limit named on the summary's first line, and the canary red.
        write(os.path.join(self.candidate, "error.txt"), "python: GitHub's rate limit (1000/h) is spent until X\n")
        code, said = self.summarize({"candidate": {"result": "failure"}, "lint": {"result": "skipped"}})
        self.assertEqual(code, 1, said)
        first = "Upgrades: unknown — the candidate job failure: python: GitHub's rate limit (1000/h) is spent until X"
        self.assertTrue(said.startswith(first + "\n"), said)
        self.assertIn("canary: the candidate job ended failure", said)
        self.assertIn("canary: no records from job lint", said)
        self.assertTrue(said.endswith("GATE FAILED\nexit: 1\n"), said)
        self.assertEqual(read(self.summary), "# before\n" + said.removesuffix("GATE FAILED\nexit: 1\n"))

    def test_a_pin_red_read_from_the_rows_stderr_is_a_pin_to_update(self):
        os.makedirs(self.candidate)
        write(os.path.join(self.candidate, "tools.json"), read(self.tools))
        write(os.path.join(self.candidate, "upgrades.json"), '{"newer_lines":[],"upgrades":[]}\n')
        blocks = "".join(FAIL_BLOCK.format(id=MANIFEST_ID, platform=p) for p in canary_support.PLATFORMS) + TAIL
        self.record(
            "lint",
            "pyfloor",
            "fail",
            "failed: gate/toolchain/tests/test_manifest.py: pyshim: gate.toolchain.tests.test_manifest: failed 1\n",
            {"gate.toolchain.tests.test_manifest": blocks},
        )
        code, said = self.summarize({"candidate": {"result": "success"}, "lint": {"result": "failure"}})
        self.assertEqual(code, 0, said)
        self.assertIn(f"Pins to update: {MANIFEST_ID} (gate/toolchain/tests/test_manifest.py)\n", said)
        self.assertIn("Breaks: none\n", said)

    def test_a_failed_module_without_a_failure_block_is_a_break(self):
        row = os.path.join(self.tmp, "row")
        write(
            os.path.join(row, "stdout"),
            "failed: gate/a/tests/test_b.py: exit 1, and the shim made no report\n"
            "failed: gate/toolchain/tests/test_manifest.py: pyshim: gate.toolchain.tests.test_manifest: failed 2\n",
        )
        blocks = "".join(FAIL_BLOCK.format(id=MANIFEST_ID, platform=p) for p in canary_support.PLATFORMS) + TAIL
        write(os.path.join(row, "py", "gate.toolchain.tests.test_manifest.stderr"), blocks)
        text = EXCEPTION_LINE
        self.assertEqual(
            main._failures(row, "pytests"),
            [
                ["pytests", "gate/a/tests/test_b.py: exit 1, and the shim made no report"],
                [MANIFEST_ID, text],
                [MANIFEST_ID, text],
            ],
        )


if __name__ == "__main__":
    unittest.main()
