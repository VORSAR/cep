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

BK-B3: the instrument pin's strength rule (teeth.python_cases) counts a case as red when its live code calls a red
helper by name; it does not see that the call's failure is swallowed (`with contextlib.suppress(AssertionError)`,
`try: … except AssertionError: pass`), that the driver re-defines the helper as a no-op, or that the call is dead
by any form but a literal `if False:` (`if 0:`, `for _ in ():`, an `if True: return` before it). Each keeps the
driver's case count and passes the pin. Checked on the real tree: test_suppressions_planted.py's
assertSuppressionsRed reduced to `with contextlib.suppress(AssertionError): self.assertRed(…)` (its three other
assertions deleted) left test_instruments_pin.RealTree green.

BK-S2: ADR-0015 §3.6 (annotated 2026-11-12) names BA13's deliberate forge as one route — a test module that writes
the shim's report file itself and exits 0. A second route needs no file: a module that, at import, replaces
unittest.TextTestRunner.run (or pyshim's own _Result, looked up after the import) makes the shim itself write
`passed <n> tests` to the report for a module whose only test fails. Same residue class (every test module is a law
path), but the ADR's sentence names one route of several.
"""

import contextlib
import os
import sys
import unittest

from gate.runner import suppressions, teeth
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


DRIVER = """import contextlib
import unittest

from gate.proofs.drivers import support


class Planted(support.GateCase):
{body}
"""
RED = '        self.assertRed(run, "lint", "suppressions", "exit 1: failed: x")\n'
HOLLOWED = {
    "the helper re-defined as a no-op": (
        "    def assertRed(self, *args):\n        pass\n\n    def test_a(self):\n        run = self.gate()\n" + RED
    ),
    "the failure swallowed by contextlib.suppress": (
        "    def test_a(self):\n        run = self.gate()\n        with contextlib.suppress(AssertionError):\n    "
        + RED
    ),
    "the failure swallowed by try/except": (
        "    def test_a(self):\n        run = self.gate()\n        try:\n    "
        + RED
        + "        except AssertionError:\n            pass\n"
    ),
    "dead under if 0": "    def test_a(self):\n        run = self.gate()\n        if 0:\n    " + RED,
    "dead in a loop over ()": "    def test_a(self):\n        run = self.gate()\n        for _ in ():\n    " + RED,
    "dead after an if True: return": (
        "    def test_a(self):\n        run = self.gate()\n        if True:\n            return\n" + RED
    ),
}


class TeethSeesAHollowedCase(unittest.TestCase):
    """BK-B3: a case whose red assertion cannot fail is a fault, not a counted red case."""

    def test_the_plain_case_is_counted_as_a_control(self):
        source = DRIVER.format(body="    def test_a(self):\n        run = self.gate()\n" + RED)
        self.assertEqual(teeth.python_cases(source, "d.py"), (["Planted.test_a"], []))

    def test_each_hollowed_case_is_a_fault(self):
        for why, body in HOLLOWED.items():
            with self.subTest(why=why):
                cases, faults = teeth.python_cases(DRIVER.format(body=body), "d.py")
                self.assertEqual(cases, ["Planted.test_a"])
                self.assertTrue(faults, f"{why}: counted as a red case")


FORGE = """import unittest

_real = unittest.TextTestRunner.run


def _forged(self, suite):
    result = _real(self, unittest.TestSuite())  # runs nothing
    result.testsRun = 3
    return result


unittest.TextTestRunner.run = _forged


class T(unittest.TestCase):
    def test_red(self):
        self.fail("a real failure")
"""


class ShimRunnerPatchForge(support.GateTest):
    """BK-S2: a module that patches the runner the shim calls is not a pass."""

    def test_a_module_patching_the_runner_is_red(self):
        self.write("pkg/__init__.py", "")
        self.write("pkg/test_m.py", FORGE)
        report = os.path.join(self.root, "pkg.test_m.report")
        shim = os.path.join(support.GATE, "runner", "pyshim.py")
        flags = ("-I", "-B", "-X", "dev", "-W", "error")  # rows.SHIM_FLAGS
        code, out, err = support.run_tool([sys.executable, *flags, shim, self.repo, "pkg.test_m", report], self.repo)
        with open(report, encoding="utf-8") as f:
            said = f.read()
        self.assertEqual(code, 1, (said, out, err))
        self.assertNotIn("passed", said)


if __name__ == "__main__":
    unittest.main()
