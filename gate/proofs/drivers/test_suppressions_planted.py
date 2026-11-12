"""The suppressions row's planted proofs (S1.1c-2 lane A; design §2.3, §2.4, §2.8; QA's W0 "on integration").

The world holds the gate with its tests (tools/allowlists/ruff.json names test files: a world without them would be
red for a stale target), tools/allowlists/, ruff.toml and every tracked Markdown file under docs/, which the refs
resolve against. Each case plants one exemption in that otherwise green world and runs the real row from
gate/registry.json: a bare `//nolint` and a `// nolint:` with a space (both honoured by golangci-lint, so red with
the canonical form's text), a canonical `//nolint:errcheck` with full metadata (red: no linter is enabled until
S1.1c-4), an allowlist entry whose ref names no heading, a TEMP entry 5 days past its expiry (green at T2, red at T4
and at the nightly), `# fmt: skip -- …` (a form ruff ignores) and an inline `# noqa` (ruff runs --ignore-noqa).

Nothing here is a calendar date, the marker or a count of existing exemptions (design DQ-S9, DQ-S10): the expiry is
computed from today in UTC, 5 days from the expiry boundary, and the marker, a known issue id and a resolving ref
are read from the world's own copies. Until lane A lands every case is red on the G0 stub's
`exit 1: suppressions: not built yet (lane A)`; until the coordinator adds `nightly` to the row's tiers, the nightly
case is red too (the runner refuses a tier with no rows).
"""

import json
import os
from datetime import UTC, datetime, timedelta

from gate.proofs.drivers import support

ROW = "suppressions"
JOB = "lint"
GO = "internal/planted/planted.go"
GO_LINE = 3
PY = "gate/planted.py"
PY_LINE = 1
CLOCK = "tools/allowlists/clock.json"
PURITY = "tools/allowlists/purity.json"
CLOCK_FILE = "internal/planted/clock.go"
CLOCK_TARGET = f"{CLOCK_FILE}: time"
EXPIRED_DAYS = 5  # 5 days from the boundary (expiry == today), so a run across midnight UTC changes nothing
NO_HEADING = "no-such-heading-planted"
# design §2.8, exactly
NOT_CANONICAL = "write //nolint:<linter> (two slashes, no space)"
NOT_ENABLED = "//nolint:errcheck: errcheck is not enabled (none is until S1.1c-4)"
RUFF_IGNORES = "ruff ignores this form; write # fmt: skip  # <reason>"
NOQA = "ruff runs --ignore-noqa: this comment waives nothing; remove it"


def line(text: str) -> str:
    """A failed line as the row prints it: the row's prefix, then design §2.8's text."""
    return f"{ROW}: {text}"


class SuppressionsPlanted(support.DriverTest):
    def setUp(self) -> None:
        super().setUp()
        self.world.copy_gate(tests=True)
        for rel in self.world.tracked("tools/allowlists"):
            self.world.copy(rel)
        for rel in self.world.tracked("docs"):
            if rel.endswith(".md"):
                self.world.copy(rel)
        self.marker = self.read_world("gate/current-sprint").strip()
        self.issue = self.json_world("gate/known-issues.json")["entries"][0]["id"]
        self.ref = self.json_world(PURITY)["entries"][0]["ref"]
        self.assertTrue(self.marker and self.issue and "#" in self.ref)

    # ---------------------------------------------------------------------------------------------------- the world

    def read_world(self, rel: str) -> str:
        return support.read_text(os.path.join(self.world.repo, rel))

    def json_world(self, rel: str) -> dict:
        return json.loads(self.read_world(rel))

    def write_json(self, rel: str, doc: dict) -> None:
        self.world.write(rel, json.dumps(doc, indent=2, ensure_ascii=False) + "\n")

    def metadata(self) -> str:
        return f'PERMANENT reason="a planted exemption for the proof" ref={self.ref} sprint={self.marker}'

    def go(self, comment: str) -> None:
        """One Go file whose line GO_LINE ends with `comment`."""
        self.world.write(GO, f"package planted\n\nfunc F() {{}} {comment}\n")

    def expired_entry(self) -> tuple[int, str]:
        """A TEMP clock entry that expired EXPIRED_DAYS ago, for a file that exists: (its index, its expiry)."""
        expiry = (datetime.now(UTC).date() - timedelta(days=EXPIRED_DAYS)).isoformat()
        self.world.write(CLOCK_FILE, "package planted\n")
        doc = self.json_world(CLOCK)
        doc["entries"].append(
            {
                "target": CLOCK_TARGET,
                "kind": "TEMP",
                "reason": "a planted exemption past its expiry",
                "expiry": expiry,
                "issue": self.issue,
                "sprint": self.marker,
            }
        )
        self.write_json(CLOCK, doc)
        return len(doc["entries"]) - 1, expiry

    def run_row(self, tier: str = "T2") -> support.Run:
        self.world.commit()
        return self.world.run(registry=self.world.registry(support.registry_row(ROW)), tier=tier)

    def in_scope(self) -> list[str]:
        """The row's {files}, from the world by the row's own pathspecs: its covered count, never hard-coded."""
        row = support.registry_row(ROW)
        self.assertEqual(row["expected_scope"][0], "git")
        out = self.world.git("ls-files", "-z", "--", *row["expected_scope"][1:])
        return [p for p in out.split("\0") if p]

    # ----------------------------------------------------------------------------------------------- the verdicts

    def assertSuppressionsRed(self, run: support.Run, first: str, where: str) -> None:
        """Red for exactly the planted reason, and every failed line about the plant: the world is otherwise green."""
        self.assertRed(run, JOB, ROW, f"exit 1: {line(first)}")
        out = run.row_stdout(JOB, ROW)
        failed = [x.removeprefix("failed: ") for x in out.split("\n") if x.startswith("failed: ")]
        self.assertTrue(failed, out)
        self.assertEqual([x for x in failed if not x.startswith(line(where))], [], out)

    # -------------------------------------------------------------------------------------------------- the cases

    def test_a_bare_nolint_is_red_with_the_canonical_form(self):
        self.go("//nolint")
        self.assertSuppressionsRed(self.run_row(), f"{GO}:{GO_LINE}: {NOT_CANONICAL}", f"{GO}:{GO_LINE}: ")

    def test_a_nolint_with_a_space_is_red_with_the_canonical_form(self):
        self.go(f"// nolint:errcheck // {self.metadata()}")
        self.assertSuppressionsRed(self.run_row(), f"{GO}:{GO_LINE}: {NOT_CANONICAL}", f"{GO}:{GO_LINE}: ")

    def test_a_canonical_nolint_with_full_metadata_is_red_while_no_linter_is_enabled(self):
        self.go(f"//nolint:errcheck // {self.metadata()}")
        self.assertSuppressionsRed(self.run_row(), f"{GO}:{GO_LINE}: {NOT_ENABLED}", f"{GO}:{GO_LINE}: ")

    def test_an_allowlist_ref_naming_no_heading_is_red(self):
        doc = self.json_world(PURITY)
        entry = doc["entries"][0]
        path = entry["ref"].split("#")[0]
        entry["ref"] = f"{path}#{NO_HEADING}"
        self.write_json(PURITY, doc)
        where = f"{PURITY}: entries[0] ({entry['target']}): "
        self.assertSuppressionsRed(self.run_row(), f"{where}{path} has no heading #{NO_HEADING}", where)

    def test_an_expired_temp_entry_is_green_at_t2(self):
        self.expired_entry()
        run = self.run_row("T2")
        files = self.in_scope()
        self.assertIn(CLOCK, files)
        self.assertRow(run, JOB, ROW, True, f"covered {len(files)}")
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)

    def test_an_expired_temp_entry_is_red_at_t4(self):
        index, expiry = self.expired_entry()
        where = f"{CLOCK}: entries[{index}] ({CLOCK_TARGET}): "
        self.assertSuppressionsRed(self.run_row("T4"), f"{where}expired {expiry}", where)

    def test_an_expired_temp_entry_is_red_at_the_nightly(self):
        index, expiry = self.expired_entry()
        where = f"{CLOCK}: entries[{index}] ({CLOCK_TARGET}): "
        self.assertSuppressionsRed(self.run_row("nightly"), f"{where}expired {expiry}", where)

    def test_a_fmt_skip_with_trailing_text_is_red(self):
        self.world.write(PY, "PLANTED = [1, 2]  # fmt: skip -- keep this list's layout as written\n")
        self.assertSuppressionsRed(self.run_row(), f"{PY}:{PY_LINE}: {RUFF_IGNORES}", f"{PY}:{PY_LINE}: ")

    def test_an_inline_noqa_in_the_gate_is_red(self):
        self.world.write(PY, "import os  # noqa: F401\n")
        self.assertSuppressionsRed(self.run_row(), f"{PY}:{PY_LINE}: {NOQA}", f"{PY}:{PY_LINE}: ")
