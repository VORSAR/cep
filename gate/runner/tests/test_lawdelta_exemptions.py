"""The lawdelta row's exemption section and countdown, and the list kinds' sub-lines (S1.1c-2 design §2.5, §2.6).

Each test owns a temporary repository with its own small gate/law.json. The row runs in-process with
`lawdelta.today()` patched (DQ-S9) and its own working directory. The section comes after the block (or HEAD_IS_MAIN
or `LAW Δ: none`) and before the uncommitted-edits line: `exemptions: <n> in code, <m> listed (<±d> this diff)`,
then the added and removed in-code exemptions named (owner, 2026-11-12, design 1), then the countdown.
"""

import contextlib
import io
import json
import os
from datetime import date, timedelta
from unittest import mock

from gate.runner import git
from gate.runner.rows import lawdelta
from gate.runner.tests import support

IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")
TODAY = date(2026, 11, 12)
LAW_ONLY = {"schema": 1, "note": "n", "paths": [{"path": "gate/law.json", "kind": "law"}], "instruments": []}
WITH_LISTS = {
    "schema": 1,
    "note": "n",
    "paths": [
        {"path": "gate/law.json", "kind": "law"},
        {"path": "tools/allowlists/", "kind": "allowlist"},
        {"path": "gate/known-issues.json", "kind": "known-issues"},
    ],
    "instruments": [],
}
GO_META = 'PERMANENT reason="fixture generator; output is never security-relevant" ref=docs/a.md#title sprint=S1.1c-2'
PY_MARK = "x = 1  # fmt: skip  # a reason that is long enough\n"


def day(k: int) -> str:
    return (TODAY + timedelta(days=k)).isoformat()


def entry(target: str, **over: str) -> dict[str, str]:
    e = {
        "target": target,
        "kind": "PERMANENT",
        "reason": "a reason long enough",
        "ref": "docs/a.md#title",
        "sprint": "S1.1c-2",
    }
    e.update(over)
    return e


def temp(target: str, expiry: str) -> dict[str, str]:
    return {
        "target": target,
        "kind": "TEMP",
        "reason": "a reason long enough",
        "expiry": expiry,
        "issue": "BA9",
        "sprint": "S1.1c-2",
    }


def allowlist(rule: str, *entries: dict) -> str:
    return json.dumps({"schema": 1, "note": "n", "rule": rule, "entries": list(entries)}, indent=1) + "\n"


def go_file(*directives: str, pad: int = 0) -> str:
    body = "".join(f"var _ = {i} //nolint:gosec // {d}\n" for i, d in enumerate(directives))
    return "package y\n\n" + "// padding\n" * pad + body


class Exemptions(support.GateTest):
    def commit(self, message: str) -> None:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "-m", message)

    def main_world(self, files: dict[str, str] | None = None, manifest: dict | None = None) -> None:
        self.git("checkout", "-q", "-b", "main")
        self.track("gate/law.json", json.dumps(manifest or LAW_ONLY) + "\n")
        self.track("docs/a.md", "# Title\n")
        for rel, text in (files or {}).items():
            self.track(rel, text)
        self.commit("main")
        self.git("checkout", "-q", "-b", "change")

    def change(self, files: dict[str, str | None]) -> None:
        for rel, text in files.items():
            if text is None:
                self.git("rm", "-q", "--", rel)
            else:
                self.track(rel, text)
        self.commit("change")

    def run_row(self) -> tuple[int, list[str]]:
        self.counter += 1
        row_dir = os.path.join(self.root, f"row{self.counter}")
        os.mkdir(row_dir)
        out = io.StringIO()
        env = {"GATE_ROW_DIR": row_dir, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
        with (
            contextlib.chdir(self.repo),
            mock.patch.dict(os.environ, env),
            mock.patch.object(lawdelta, "today", return_value=TODAY),
            contextlib.redirect_stdout(out),
        ):
            code = lawdelta.main(["lawdelta.py"])
        return code, out.getvalue().splitlines()

    def section(self, lines: list[str]) -> list[str]:
        """The lines from `exemptions:` on (the block above it is not this test's)."""
        starts = [i for i, text in enumerate(lines) if text.startswith("exemptions: ")]
        self.assertEqual(len(starts), 1, lines)
        return lines[starts[0] :]

    # The exemption line.

    def test_plus_zero_and_nothing_else_for_an_unchanged_list(self):
        self.main_world({"tools/allowlists/purity.json": allowlist("purity", entry("internal/a: sync.Pool"))})
        self.change({"notes.txt": "not an exemption\n"})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        self.assertEqual(lines, ["GATE_COVERED 1", "LAW Δ: none", "exemptions: 0 in code, 1 listed (+0 this diff)"])

    def test_head_on_main_still_prints_the_line(self):
        # design ambiguity: "after the block (or HEAD_IS_MAIN/NONE)" read as: the line follows HEAD_IS_MAIN too.
        self.git("checkout", "-q", "-b", "main")
        self.track("gate/law.json", json.dumps(LAW_ONLY) + "\n")
        self.track("tools/allowlists/purity.json", allowlist("purity", entry("internal/a: sync.Pool")))
        self.commit("main")
        code, lines = self.run_row()
        self.assertEqual(
            (code, lines),
            (0, ["GATE_COVERED 1", lawdelta.HEAD_IS_MAIN, "exemptions: 0 in code, 1 listed (+0 this diff)"]),
        )

    def test_plus_one_for_a_listed_entry(self):
        self.main_world({"tools/allowlists/purity.json": allowlist("purity", entry("internal/a: sync.Pool"))})
        two = allowlist("purity", entry("internal/a: sync.Pool"), entry("internal/b: sync.Once"))
        self.change({"tools/allowlists/purity.json": two})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        self.assertEqual(self.section(lines), ["exemptions: 0 in code, 2 listed (+1 this diff)"])

    def test_minus_one_names_a_removed_marker(self):
        self.main_world({"gate/runner/z.py": "import os\n" + PY_MARK})
        self.change({"gate/runner/z.py": "import os\nx = 1\n"})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        want = ["exemptions: 0 in code, 0 listed (−1 this diff)", "  −gate/runner/z.py:2 # fmt: skip"]
        self.assertEqual(self.section(lines), want)

    def test_an_added_directive_is_named(self):
        self.main_world()
        self.change({"internal/x/y.go": go_file(GO_META)})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        want = [
            "exemptions: 1 in code, 0 listed (+1 this diff)",
            "  +internal/x/y.go:3 //nolint:gosec PERMANENT S1.1c-2 ref=docs/a.md#title",
        ]
        self.assertEqual(self.section(lines), want)

    def test_an_added_marker_is_named(self):
        # design ambiguity (settled by §10.16): §2.6 shows a removed marker only, `−<path>:<line>` then the skip
        # marker; an added one is read as the same form with "+".
        self.main_world()
        self.change({"gate/runner/z.py": "import os\n" + PY_MARK})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        want = ["exemptions: 1 in code, 0 listed (+1 this diff)", "  +gate/runner/z.py:2 # fmt: skip"]
        self.assertEqual(self.section(lines), want)

    def test_a_moved_directive_is_neither_added_nor_removed(self):
        self.main_world({"internal/x/y.go": go_file(GO_META)})
        self.change({"internal/x/y.go": go_file(GO_META, pad=4)})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        self.assertEqual(self.section(lines), ["exemptions: 1 in code, 0 listed (+0 this diff)"])

    def test_at_most_six_named_then_and_k_more(self):
        self.main_world()
        self.change({f"internal/x/f{i}.go": go_file(GO_META) for i in range(8)})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        got = self.section(lines)
        self.assertEqual(got[0], "exemptions: 8 in code, 0 listed (+8 this diff)")
        self.assertEqual(len(got), 8, got)
        self.assertTrue(all(s.startswith("  +internal/x/f") for s in got[1:7]), got)
        self.assertEqual(got[7], "  (and 2 more)")

    def test_a_git_error_is_a_failure(self):
        # design ambiguity: §2.6 says lawdelta turns GitError into FAIL; the failure's wording is not given.
        self.main_world()
        self.change({"notes.txt": "x\n"})
        with mock.patch.object(git, "grep_files", side_effect=git.GitError("git grep -l: exit 128")):
            code, lines = self.run_row()
        self.assertEqual(code, 1, lines)
        self.assertTrue(any(s.startswith("failed: lawdelta: ") and "git grep" in s for s in lines), lines)

    # The countdown (DA-S2): TEMP entries and directives at HEAD with k <= 14.

    def test_the_countdown_at_fourteen_zero_and_minus_one(self):
        lst = allowlist(
            "clock",
            temp("a.go: time", day(14)),
            temp("b.go: time", day(15)),
            temp("c.go: time", day(0)),
            temp("d.go: time", day(-1)),
        )
        self.main_world({"tools/allowlists/clock.json": lst})
        self.change({"notes.txt": "x\n"})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        got = self.section(lines)
        self.assertEqual(got[0], "exemptions: 0 in code, 4 listed (+0 this diff)")
        # design ambiguity: <where> for a list entry is read as the list's path, <target> as the entry's target.
        where = "  exemption tools/allowlists/clock.json"
        want = [
            f"{where}: a.go: time expires {day(14)} (in 14 days)",
            f"{where}: c.go: time expires {day(0)} (in 0 days)",
            f"{where}: d.go: time expired {day(-1)} (red at T4 and nightly)",
        ]
        self.assertEqual(sorted(got[1:]), sorted(want))

    def test_a_temp_directive_counts_down(self):
        meta = f'TEMP reason="close error surfaced later" expiry={day(3)} issue=BA9 sprint=S1.1c-2'
        self.main_world({"internal/x/y.go": go_file(meta)})
        self.change({"notes.txt": "x\n"})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        got = self.section(lines)
        # design ambiguity: a directive's <where>/<target> are not spelled out; its path and the date are.
        self.assertEqual(len(got), 2, got)
        self.assertTrue(got[1].startswith("  exemption ") and "internal/x/y.go" in got[1], got)
        self.assertTrue(got[1].endswith(f"expires {day(3)} (in 3 days)"), got)

    # The list kinds through law.block: the file line, then two-space sub-lines, at most 6.

    def test_a_list_entry_added_is_a_sub_line_and_an_unchanged_list_is_not_named(self):
        files = {
            "tools/allowlists/purity.json": allowlist("purity", entry("internal/a: sync.Pool")),
            "tools/allowlists/clock.json": allowlist("clock"),
            "gate/known-issues.json": json.dumps({"schema": 1, "note": "n", "entries": []}) + "\n",
        }
        self.main_world(files, WITH_LISTS)
        two = allowlist("purity", entry("internal/a: sync.Pool"), entry("internal/x: sync.Once"))
        self.change({"tools/allowlists/purity.json": two})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        i = next(n for n, s in enumerate(lines) if s.startswith("tools/allowlists/purity.json +"))
        self.assertTrue(lines[i].endswith("(2 entries)"), lines)
        self.assertEqual(lines[i + 1], "  +internal/x: sync.Once PERMANENT S1.1c-2 ref=docs/a.md#title")
        self.assertFalse(any("clock.json" in s for s in lines), lines)

    def test_a_new_list_and_the_cap(self):
        self.main_world(
            {"gate/known-issues.json": json.dumps({"schema": 1, "note": "n", "entries": []}) + "\n"}, WITH_LISTS
        )
        eight = allowlist("purity", *[entry(f"internal/p{i}: sync.Once") for i in range(8)])
        self.change({"tools/allowlists/purity.json": eight})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        i = next(n for n, s in enumerate(lines) if s.startswith("tools/allowlists/purity.json +"))
        self.assertTrue(lines[i].endswith("file added: 8 entries"), lines)
        self.assertTrue(all(s.startswith("  +internal/p") for s in lines[i + 1 : i + 7]), lines)
        self.assertEqual(lines[i + 7], "  (and 2 more)")

    def test_a_known_issue_forwarded(self):
        row = {"id": "BA9", "first_seen": "S1.1a-1", "forward": "S1.1c-3", "decision": "docs/a.md#title"}
        old = {"schema": 1, "note": "n", "entries": [row]}
        new = {"schema": 1, "note": "n", "entries": [row | {"forward": "S1.1c-4"}]}
        files = {"tools/allowlists/clock.json": allowlist("clock"), "gate/known-issues.json": json.dumps(old) + "\n"}
        self.main_world(files, WITH_LISTS)
        self.change({"gate/known-issues.json": json.dumps(new, indent=1) + "\n"})
        code, lines = self.run_row()
        self.assertEqual(code, 0, lines)
        i = next(n for n, s in enumerate(lines) if s.startswith("gate/known-issues.json +"))
        self.assertEqual(lines[i + 1], "  BA9: forward S1.1c-3 → S1.1c-4")


if __name__ == "__main__":
    import unittest

    unittest.main()
