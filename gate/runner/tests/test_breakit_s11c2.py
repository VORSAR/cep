"""QA's break-it of S1.1c-2's combined build (sprint model §6 step 7; decisions 2026-11-10, decision 7): plants the
gate misses, each red today. Findings BK-* in QA's break-it report.

BK-B1: ruff 0.17.3 honours `# fmt:` followed by any run of spaces or tabs before `skip`, `off` and `on` (and
`# yapf:` likewise before `disable`/`enable`) — measured with the pinned binary: `x = [1,2,   3]  # fmt:  skip`
stays unformatted under `ruff format --check --config ruff.toml`. `py_markers` reads none of these forms: no
marker, no fault, so the waiver is silent in the suppressions row and in LAW Δ's exemption count (no PREFILTER
pattern matches it either).

BK-B2: the ruff pin reads only ruff.toml's [lint.per-file-ignores] and [lint.extend-per-file-ignores] (and refuses
lint.ignore, lint.extend-ignore). Pinned ruff 0.17.3, run as the ruff row runs it (`check --ignore-noqa --no-cache
--config ruff.toml`), also honours the deprecated top-level `ignore`, `extend-ignore` and `[per-file-ignores]` (a
warning on stderr, exit 0, "All checks passed!") and every table of a file named by `extend = "<path>"` — measured.
Each waives a rule with no ruff.json entry and the pin stays green; a file named by `extend` outside a law path
(docs/, say) then changes with no LAW Δ line at all.
"""

import contextlib
import unittest

from gate.runner import suppressions
from gate.runner.rows import suppressions as row
from gate.runner.tests import support

REASON = "the table reads better laid out by hand"

# (comment as written; ruff 0.17.3 keeps the layout — measured with the pinned binary)
HONOURED_BY_RUFF = (
    f"# fmt:  skip  # {REASON}",
    f"# fmt:\tskip  # {REASON}",
    f"#  fmt:   skip  # {REASON}",
    f"# {REASON}  # fmt:  skip",
)
REGIONS_HONOURED_BY_RUFF = ("# fmt:  off", "# fmt:\toff", "# yapf:  disable", "# yapf:\tdisable")


class FmtSkipWhitespace(unittest.TestCase):
    """BK-B1."""

    def test_every_skip_ruff_honours_is_read(self):
        for comment in HONOURED_BY_RUFF:
            with self.subTest(comment=comment):
                markers, faults = suppressions.py_markers("gate/x.py", f"x = [1,2,   3]  {comment}\n")
                self.assertNotEqual((markers, faults), ([], []), "a waiver ruff honours passes unseen")

    def test_every_region_ruff_honours_is_red(self):
        for comment in REGIONS_HONOURED_BY_RUFF:
            with self.subTest(comment=comment):
                _, faults = suppressions.py_markers("gate/x.py", f"{comment}\nx = [1,2,   3]\n")
                self.assertTrue(faults, "a region ruff honours passes unseen")

    def test_every_form_ruff_honours_is_found_by_lawdeltas_prefilter(self):
        for comment in (*HONOURED_BY_RUFF, *REGIONS_HONOURED_BY_RUFF):
            with self.subTest(comment=comment):
                self.assertTrue(any(p in comment for p in suppressions.PREFILTER), comment)


SELECT = '[lint]\nselect = ["TID"]\n'


class RuffPinReadsEveryWaiver(support.GateTest):
    """BK-B2: every waiver pinned ruff honours from ruff.toml is either pinned in ruff.json or red."""

    def pin(self, ruff_toml: str, base: str = "") -> list[str]:
        self.write("ruff.toml", ruff_toml)
        if base:
            self.write("docs/base.toml", base)
        with contextlib.chdir(self.repo):
            return row.ruff_pin({})

    def test_the_lint_table_is_red_today_as_a_control(self):
        self.assertEqual(
            self.pin(SELECT + '[lint.per-file-ignores]\n"gate/x.py" = ["TID251"]\n'),
            ["ruff.toml waives TID251 for gate/x.py without an entry in tools/allowlists/ruff.json"],
        )

    def test_a_top_level_ignore_is_red(self):
        self.assertTrue(self.pin('ignore = ["TID251"]\n' + SELECT), "a top-level ignore waives TID251 everywhere")

    def test_a_top_level_extend_ignore_is_red(self):
        self.assertTrue(self.pin('extend-ignore = ["TID251"]\n' + SELECT), "a top-level extend-ignore waives it")

    def test_a_top_level_per_file_ignores_is_pinned(self):
        self.assertTrue(
            self.pin('[per-file-ignores]\n"gate/x.py" = ["TID251"]\n\n' + SELECT),
            "a top-level [per-file-ignores] waives TID251 for gate/x.py",
        )

    def test_a_waiver_in_an_extended_file_is_pinned(self):
        self.assertTrue(
            self.pin(
                'extend = "docs/base.toml"\n' + SELECT,
                base='[lint.per-file-ignores]\n"gate/x.py" = ["TID251"]\n',
            ),
            "docs/base.toml's waiver, read through extend, waives TID251 for gate/x.py",
        )


if __name__ == "__main__":
    unittest.main()
