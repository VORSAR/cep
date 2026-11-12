"""The LAW Δ kinds `allowlist` and `known-issues` (S1.1c-2 design §2.5): one sub-line per changed entry.

`law.delta` returns a list for these two kinds; `law.block` renders the file's line, then the sub-lines indented by
two spaces, at most 6, then `  (and <k> more)`. The rendering is asserted through the real lawdelta row in
test_lawdelta_exemptions.py; here the sub-lines themselves.
"""

import json
import re
import unittest

from gate.runner import law

REF = "docs/a.md#title"


def entry(target: str, **over: str) -> dict[str, str]:
    e = {"target": target, "kind": "PERMANENT", "reason": "a reason long enough", "ref": REF, "sprint": "S1.1c-2"}
    e.update(over)
    return e


def allowlist(*entries: dict) -> str:
    return json.dumps({"schema": 1, "note": "n", "rule": "purity", "entries": list(entries)})


def issues(*entries: dict) -> str:
    return json.dumps({"schema": 1, "note": "n", "entries": list(entries)})


def issue(id: str, **over: object) -> dict[str, object]:
    e = {
        "id": id,
        "first_seen": "S1.1a-1",
        "forward": "S1.1c-3",
        "decision": "docs/d.md#x",
        "correctness": False,
        "opened": "2026-11-01",
    }
    e.update(over)
    return e


def subs(got: object) -> list[str]:
    """The sub-lines of a list-returning kind. Design §10.13: `delta` returns [summary, *sub_lines], the summary
    `(<n> entries)` or `file added: <n> entries`; it is checked for that form and set aside."""
    assert isinstance(got, list), got
    assert re.fullmatch(r"\(\d+ entries\)|file added: \d+ entries", got[0]), got
    return got[1:]


class Kinds(unittest.TestCase):
    def test_the_two_kinds_exist(self):
        self.assertIn("allowlist", law.KINDS)
        self.assertIn("known-issues", law.KINDS)
        self.assertNotIn("allowlist", law.GROUPED)


class Allowlist(unittest.TestCase):
    def test_added_permanent(self):
        got = law.delta(
            "allowlist",
            allowlist(entry("internal/a: sync.Pool")),
            allowlist(entry("internal/a: sync.Pool"), entry("internal/x: sync.Once")),
        )
        self.assertEqual(subs(got), ["+internal/x: sync.Once PERMANENT S1.1c-2 ref=docs/a.md#title"])

    def test_added_temp(self):
        temp = entry("x.go: time", kind="TEMP", ref="", expiry="2026-12-01", issue="BA9")
        del temp["ref"]
        got = law.delta("allowlist", allowlist(), allowlist(temp))
        self.assertEqual(subs(got), ["+x.go: time TEMP S1.1c-2 expiry=2026-12-01 issue=BA9"])

    def test_removed(self):
        got = law.delta(
            "allowlist",
            allowlist(entry("internal/a: sync.Pool"), entry("internal/x: sync.Once")),
            allowlist(entry("internal/a: sync.Pool")),
        )
        self.assertEqual(subs(got), ["−internal/x: sync.Once"])

    def test_a_changed_field(self):
        got = law.delta(
            "allowlist",
            allowlist(entry("internal/a: sync.Pool")),
            allowlist(entry("internal/a: sync.Pool", sprint="S1.1c-1")),
        )
        self.assertEqual(subs(got), ["internal/a: sync.Pool: sprint S1.1c-2 → S1.1c-1"])

    def test_reason_changed(self):
        got = law.delta(
            "allowlist",
            allowlist(entry("internal/a: sync.Pool")),
            allowlist(entry("internal/a: sync.Pool", reason="another reason, as long")),
        )
        self.assertEqual(subs(got), ["internal/a: sync.Pool: reason changed"])

    def test_file_added(self):
        got = law.delta("allowlist", None, allowlist(entry("internal/a: sync.Pool"), entry("internal/b: sync.Map")))
        self.assertIsInstance(got, list)
        self.assertTrue(any(s.endswith("file added: 2 entries") for s in got), got)
        self.assertEqual(
            subs(got),
            [
                "+internal/a: sync.Pool PERMANENT S1.1c-2 ref=docs/a.md#title",
                "+internal/b: sync.Map PERMANENT S1.1c-2 ref=docs/a.md#title",
            ],
        )

    def test_every_entry_is_returned_the_cap_is_blocks(self):
        new = [entry(f"internal/p{i}: sync.Once") for i in range(8)]
        got = law.delta("allowlist", allowlist(), allowlist(*new))
        self.assertEqual(len(subs(got)), 8)


class KnownIssues(unittest.TestCase):
    def added(self, **over: object) -> str:
        (line,) = subs(law.delta("known-issues", issues(issue("BA9")), issues(issue("BA9"), issue("QB10", **over))))
        return line

    def test_added(self):
        # S1.1c-3 design §3.3 (DA-S11): an added entry's line names both fields, their values visible.
        line = self.added(first_seen="S1.1a-2")
        self.assertTrue(line.startswith("+QB10 first seen S1.1a-2"), line)
        self.assertIn("correctness", line)
        self.assertIn("opened", line)
        self.assertIn("2026-11-01", line)
        self.assertNotEqual(self.added(correctness=True), self.added(correctness=False))

    def test_removed(self):
        got = law.delta("known-issues", issues(issue("BA9"), issue("QB10")), issues(issue("BA9")))
        self.assertEqual(subs(got), ["−QB10"])

    def test_forward(self):
        got = law.delta("known-issues", issues(issue("BA9")), issues(issue("BA9", forward="S1.1c-4")))
        self.assertEqual(subs(got), ["BA9: forward S1.1c-3 → S1.1c-4"])

    def test_decision_set_changed_cleared(self):
        for old, new, word in (
            ("", "docs/d.md#x", "set"),
            ("docs/d.md#x", "docs/d.md#y", "changed"),
            ("docs/d.md#x", "", "cleared"),
        ):
            with self.subTest(word=word):
                got = law.delta("known-issues", issues(issue("BA9", decision=old)), issues(issue("BA9", decision=new)))
                self.assertEqual(subs(got), [f"BA9: decision {word}"])

    def test_correctness_changed_is_named(self):
        # S1.1c-3 design §3.3 (DA-S11): a known issue becoming a correctness bug starts ground rule 10's clock.
        for old, new in ((False, True), (True, False)):
            with self.subTest(old=old, new=new):
                got = law.delta(
                    "known-issues", issues(issue("BA9", correctness=old)), issues(issue("BA9", correctness=new))
                )
                (line,) = subs(got)
                self.assertTrue(line.startswith("BA9: correctness"), line)

    def test_opened_changed_is_named(self):
        got = law.delta("known-issues", issues(issue("BA9")), issues(issue("BA9", opened="2026-10-01")))
        (line,) = subs(got)
        self.assertTrue(line.startswith("BA9: opened"), line)
        self.assertIn("2026-11-01", line)
        self.assertIn("2026-10-01", line)

    def test_the_other_kinds_keep_their_one_liner(self):
        self.assertIsInstance(law.delta("marker", "S1.1c-1\n", "S1.1c-2\n"), str)


if __name__ == "__main__":
    unittest.main()
