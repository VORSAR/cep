"""The deps row's two S1.1c-4 lines (design note §3.1, §3.2; reviews DA-S4, DA-S5, DQ-B1, DQ-S1, DQ-S3; owner
2026-11-13, the design's decision 3): the tidy line, read from `go mod tidy -diff` as sets per file, and the T4
`go.sum` line, from `git diff --name-only HEAD`.

The tidy tests fake `gomods.run` (its stdout returned, its stderr file written, as the real one keeps them); the
`go.sum` tests run the real git in the test's own repository. Every expected text is written from the design note
by hand; ROOT_DIFF is the real root's `go mod tidy -diff` output, captured once (pinned go1.27.4, GOPROXY=off,
GOFLAGS=-mod=readonly, 2026-11-13 at 16f5bd3).
"""

import os
from unittest import mock

from gate.runner.rows import deps as deps_row
from gate.runner.rows import gomods
from gate.runner.tests import support

ROOT_DIFF = "\n".join(
    (
        "diff current/go.mod tidy/go.mod",
        "--- current/go.mod",
        "+++ tidy/go.mod",
        "@@ -9,12 +9,3 @@",
        " \tgo.uber.org/goleak v1.3.0",
        " \tgolang.org/x/text v0.43.0",
        " )",
        "-",
        "-// The gate's apidiff tool (declared in gate/proofs/go.mod) builds with these two at these versions: one "
        "version of a",
        "-// module across the repository's modules (owner, 2026-11-11, S1.1c-1's design, 7). x/text requires both "
        "at older",
        "-// versions, so they are in every embedder's graph either way. `go mod tidy` drops these lines; the deps "
        "row then",
        "-// turns red on the two versions, which is the signal to put them back.",
        "-require (",
        "-\tgolang.org/x/mod v0.42.0 // indirect",
        "-\tgolang.org/x/tools v0.52.1-0.20261106145633-d407941a339a // indirect",
        "-)",
        "",
        "diff current/go.sum tidy/go.sum",
        "--- current/go.sum",
        "+++ tidy/go.sum",
        "@@ -14,13 +14,7 @@",
        " github.com/stretchr/testify v1.8.0/go.mod h1:yNjHg4UonilssWZ8iaSj1OCr/vHnekPRkoO+kdMU+MU=",
        " go.uber.org/goleak v1.3.0 h1:2K3zAYmnTNqV73imy9J1T3WC+gmCePx2hEGkimedGto=",
        " go.uber.org/goleak v1.3.0/go.mod h1:CoHD4mav9JJNrW/WLlf7HGZPjdw8EucARQHekz1X6bE=",
        "-golang.org/x/mod v0.42.0 h1:ICeL3t2L30ljSPHHxDnR4fqVcBt/zR2zJAb9juavmQg=",
        "-golang.org/x/mod v0.42.0/go.mod h1:u5fzIrRInkL8MaO1cbRkUHT64v303YIT5KEqpNr7jRs=",
        "-golang.org/x/sync v0.24.0 h1:iqiDrGoDgyuvAKrhLQthUEzU/R378N78gXbBjxed7oA=",
        "-golang.org/x/sync v0.24.0/go.mod h1:sUUOizhqBxiL6pEWpqNLUiaJn1ShEbZ6BBqskPbjZm0=",
        " golang.org/x/text v0.43.0 h1:1QivrlhwAsnMOcqgOdl9my7s9OyZl8O5/UGUTJVXoWc=",
        " golang.org/x/text v0.43.0/go.mod h1:hbSIYA/amXcRXTFSZqh8tWHEoqRvvbKPJ2uz/KU+a+s=",
        "-golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a h1:Yqozs0FNPRoPGS4hNGlrNwRENWr5H5nVG9sN/f+5640=",
        "-golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a/go.mod "
        "h1:ItcP5UNaVGG+Yc4eIYmaFLNJ4soEopkx5Y2d4lcEurs=",
        " gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        " gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=",
        "",
        "",
    )
)
# QA's measured stderr of `go mod tidy -diff` offline in an empty module cache (design note §0.3).
OFFLINE_STDERR = (
    "go: downloading golang.org/x/exp v0.0.0-20261106192419-47ce03d25ef0\n"
    "go: golang.org/x/exp/cmd/apidiff: module lookup disabled by GOPROXY=off\n"
)
LOOKUP = "go: golang.org/x/exp/cmd/apidiff: module lookup disabled by GOPROXY=off"
IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@t.invalid", "-c", "commit.gpgsign=false")
GOMOD_HEAD = "diff current/go.mod tidy/go.mod\n--- current/go.mod\n+++ tidy/go.mod\n@@ -1,6 +1,6 @@\n"


def gomod_diff(*lines: str) -> str:
    return GOMOD_HEAD + "".join(f"{line}\n" for line in lines)


def root_with(*lines: str) -> str:
    """The root's real diff with more changed go.mod lines, in a hunk of their own."""
    return gomod_diff(*lines) + ROOT_DIFF


class FakeRun:
    """gomods.run for the tidy line: per module dir, the stdout it returns and the stderr it leaves in row_dir."""

    def __init__(self, outputs: dict[str, tuple[str, str]]) -> None:
        self.outputs = outputs
        self.calls: list[tuple[str, str, tuple[str, ...], tuple[int, ...]]] = []

    def __call__(self, row_dir, env, cwd, name, argv, *, ok=(0,)):
        self.calls.append((cwd, name, tuple(argv), tuple(ok)))
        stdout, stderr = self.outputs[cwd]
        for suffix, text in (("stdout", stdout), ("stderr", stderr)):
            with open(os.path.join(row_dir, f"{name}.{suffix}"), "w", encoding="utf-8") as f:
                f.write(text)
        return stdout


class Tidy(support.GateTest):
    def tidy(self, root: tuple[str, str], proofs: tuple[str, str] = ("", "")) -> list[str]:
        fake = FakeRun({".": root, "gate/proofs": proofs})
        with mock.patch.object(gomods, "run", fake):
            lines = deps_row.tidy_lines(self.run_dir, {})
        self.assertEqual(
            [(cwd, argv, ok) for cwd, _, argv, ok in fake.calls],
            [(mod, ("go", "mod", "tidy", "-diff"), (0, 1)) for mod in (".", "gate/proofs")],
        )
        return lines

    def test_tidy_go_sum_lines_never_count_as_go_mod_pairs(self):
        # The real root diff: go.mod's two pinned requires and go.sum's six lines, each read in its own file.
        self.assertEqual(self.tidy((ROOT_DIFF, "")), [])

    def test_tidy_that_cannot_run_offline_is_red_not_clean(self):
        self.assertEqual(self.tidy((ROOT_DIFF, ""), ("", "")), [], "the witness: empty stdout, empty stderr is clean")
        self.assertEqual(
            self.tidy((ROOT_DIFF, ""), ("", OFFLINE_STDERR)),
            [f"deps: go mod tidy could not run offline in gate/proofs: {LOOKUP}: run go mod download in gate/proofs"],
        )

    def test_tidy_single_line_require_is_read(self):
        # A single-line require is a pair: the root's allowance met in that form is green ...
        single = (
            "diff current/go.mod tidy/go.mod\n--- current/go.mod\n+++ tidy/go.mod\n@@ -9,12 +9,3 @@\n"
            "-require golang.org/x/mod v0.42.0 // indirect\n"
            "-require golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a\n"
        )
        go_sum = ROOT_DIFF[ROOT_DIFF.index("diff current/go.sum") :]
        self.assertEqual(self.tidy((single + go_sum, "")), [])
        # ... and in gate/proofs the same form names its pair, not the line.
        self.assertEqual(
            self.tidy((ROOT_DIFF, ""), (gomod_diff("-require go.uber.org/goleak v1.3.0"), "")),
            ["deps: go mod tidy would change gate/proofs: go.mod: -go.uber.org/goleak v1.3.0"],
        )

    def test_tidy_changed_go_or_toolchain_line_is_red(self):
        for line in ("-toolchain go1.27.4", "+toolchain go1.27.5", "-go 1.27.0", "+go 1.28.0"):
            with self.subTest(line=line):
                self.assertEqual(
                    self.tidy((root_with(line), "")), [f"deps: go mod tidy would change .: go.mod: {line}"]
                )
        for line in ("-tool golang.org/x/exp/cmd/apidiff", "+replace example.com/a => ../a", "-retract v1.0.0"):
            with self.subTest(line=line):
                self.assertEqual(
                    self.tidy((root_with(line), "")), [f"deps: go mod tidy would change .: go.mod: {line}"]
                )

    def test_tidy_root_adds_nothing_and_removes_exactly_the_allowance(self):
        self.assertEqual(
            self.tidy((root_with("+\tgolang.org/x/text v0.44.0 // indirect"), "")),
            ["deps: go mod tidy would change .: go.mod: +golang.org/x/text v0.44.0"],
        )
        self.assertEqual(
            self.tidy((root_with("-\tgo.uber.org/goleak v1.3.0"), "")),
            ["deps: go mod tidy would change .: go.mod: -go.uber.org/goleak v1.3.0"],
        )
        extra_sum = ROOT_DIFF + "diff current/go.sum tidy/go.sum\n--- current/go.sum\n+++ tidy/go.sum\n@@ -1 +1 @@\n"
        self.assertEqual(
            self.tidy((extra_sum + "+example.com/a v1.0.0 h1:x=\n", "")),
            ["deps: go mod tidy would change .: go.sum: +example.com/a v1.0.0 h1:x="],
        )
        # The allowance not removed (tidy keeps a pinned require) is a difference too.
        without_tools = ROOT_DIFF.replace(
            "-\tgolang.org/x/tools v0.52.1-0.20261106145633-d407941a339a // indirect\n", ""
        )
        self.assertEqual(
            self.tidy((without_tools, "")),
            [
                "deps: go mod tidy would change .: go.mod: golang.org/x/tools "
                "v0.52.1-0.20261106145633-d407941a339a is in TIDY_ALLOWED but tidy keeps it"
            ],
        )

    def test_tidy_gate_proofs_with_any_change_is_red(self):
        for diff, first in (
            (gomod_diff("-\tgolang.org/x/mod v0.42.0 // indirect"), "go.mod: -golang.org/x/mod v0.42.0"),
            (
                gomod_diff("-\tgolang.org/x/exp v0.0.0-1 // indirect", "+\tgolang.org/x/exp v0.0.0-1"),
                "go.mod: -golang.org/x/exp v0.0.0-1",
            ),
            (
                ROOT_DIFF[ROOT_DIFF.index("diff current/go.sum") :],
                "go.sum: -golang.org/x/mod v0.42.0 h1:ICeL3t2L30ljSPHHxDnR4fqVcBt/zR2zJAb9juavmQg=",
            ),
        ):
            with self.subTest(first=first):
                self.assertEqual(
                    self.tidy((ROOT_DIFF, ""), (diff, "")), [f"deps: go mod tidy would change gate/proofs: {first}"]
                )

    def test_tidy_that_exits_outside_0_and_1_is_a_named_line(self):
        def failing(row_dir, env, cwd, name, argv, *, ok=(0,)):
            raise gomods.Failed("go mod tidy -diff: flag provided but not defined: -diff")

        with mock.patch.object(gomods, "run", failing):
            self.assertEqual(
                deps_row.tidy_lines(self.run_dir, {}),
                [
                    f"deps: go mod tidy could not run in {mod}: go mod tidy -diff: flag provided but not defined: -diff"
                    for mod in (".", "gate/proofs")
                ],
            )


class GoSum(support.GateTest):
    def setUp(self) -> None:
        super().setUp()
        self.track("go.sum", "example.com/a v1.0.0 h1:a=\n")
        self.track("gate/proofs/go.sum", "example.com/b v1.0.0 h1:b=\n")
        self.git(*IDENTITY, "commit", "-q", "-m", "the world")

    def lines(self, tier: str) -> list[str]:
        return deps_row.gosum_lines(self.run_dir, {**os.environ, "GATE_TIER": tier}, self.repo)

    def test_go_sum_line_only_at_T4(self):
        self.assertEqual(self.lines("T4"), [], "T4 clean: no line")
        self.write("go.sum", "example.com/a v1.0.0 h1:changed=\n")
        self.write("gate/proofs/go.sum", "")
        self.assertEqual(self.lines("T3"), [], "T3 dirty: no line")
        self.assertEqual(
            self.lines("T4"),
            [
                f"deps: {name} differs from HEAD at T4: commit it, or fix the step that wrote it"
                for name in ("gate/proofs/go.sum", "go.sum")
            ],
        )

    def test_a_git_failure_is_a_named_line(self):
        self.git("update-ref", "-d", "HEAD")
        lines = self.lines("T4")
        self.assertEqual(len(lines), 1, lines)
        self.assertTrue(
            lines[0].startswith("deps: the T4 go.sum check could not run: git diff --name-only HEAD"), lines
        )
