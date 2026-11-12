"""QA's break-it of S1.1c-2's combined build (sprint model §6 step 7; decisions 2026-11-10, decision 7): plants the
gate misses, each red today. Findings BK-* in QA's break-it report.

BK-B1: ruff 0.17.3 honours `# fmt:` followed by any run of spaces or tabs before `skip`, `off` and `on` (and
`# yapf:` likewise before `disable`/`enable`) — measured with the pinned binary: `x = [1,2,   3]  # fmt:  skip`
stays unformatted under `ruff format --check --config ruff.toml`. `py_markers` reads none of these forms: no
marker, no fault, so the waiver is silent in the suppressions row and in LAW Δ's exemption count (no PREFILTER
pattern matches it either).
"""

import unittest

from gate.runner import suppressions

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


if __name__ == "__main__":
    unittest.main()
