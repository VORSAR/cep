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

BK-S4: golangci-lint v2.15.0 honours gosec's own `#nosec` waiver — measured with the pinned binary (`run
--no-config --default=none --enable=gosec`): `"crypto/md5" // #nosec G501` and `md5.Sum(b) // #nosec G401` drop
both findings, which reappear when the comments go. §0.3's table and go_directives know only `nolint`: a `#nosec`
in a tracked .go file is no directive and no fault, and no PREFILTER pattern finds it. No linter is enabled until
S1.1c-4, so today it waives nothing; the day gosec is enabled every `#nosec` already in the tree is a live waiver
with no metadata and no LAW Δ line.

BK-B4 (Python side; the Go side is tools/archcheck/breakit_s11c2_test.go): the pin counts a driver's cases from its
source, the proofs row runs what unittest loads. A module-level `load_tests` returning only the green control, a
`del Planted.test_red` after the class, or a case moved into a class that is not a TestCase keeps the static count
while the shim runs and reports less — measured: pyshim says `passed 1 tests` for a module of three cases, two of
them failing.

BK-S5: gate/known-issues.json may list one id twice. knownissues.check keys the entries by id, so the second hides
the first: an id re-entered with a later first_seen (and its Found cell edited to match) passes the pin, and LAW
Δ's known-issues line reads it as `S3.3-2-1: first seen S3.3 → S1.1c-2` on a file it counts as `(1 entries)` while
it holds two. allowlists.load refuses a target listed twice; the known-issues file does not. S1.1c-3's aging reads
first_seen.

BK-S6: the canonical generated-code header (`// Code generated … DO NOT EDIT.` on the first line, ADR-0015 §2.1)
takes a hand-written Go file out of the coverage floor's numerator and denominator (gate/coverage/main.py) and out
of archcheck's globals check, and nothing checks it is real: `freshness`, its anti-forgery, is annotated moot. It is
an in-code exemption the suppressions row neither reads nor counts.

BK-L1: docsync (a) reads a §3 line naming a file as listing the file's directory (§11.7). §3's data-file lines
therefore list directories as packages nobody described: `tools/deps-allowlist.json` lists `tools`,
`api/exports.golden` lists `api`, `tools/importcheck/rules.json` lists `tools/importcheck`. A new Go package in any
of them passes docsync.

BK-L2: a PERMANENT entry's ref must name a heading of a tracked Markdown file (anchors.resolves). A heading inside
an HTML comment — `<!--` … `## Gone` … `-->`, which renders as nothing and has no anchor on the rendered page —
satisfies it, so a waiver may cite a section the reader cannot find.

BK-S7: the pin never ties a proof to its instrument's row. An instrument may name another row's driver as its proof
(the cases count matching), and the strength rule's "a pass on the row under test" accepts `assertRow(…, ok=True)`
on any row id but "parity". So a row whose own driver is gone keeps a green pin by borrowing a neighbour's; LAW Δ
names the law.json edit (`instrument docsync: proofs … → …`), the pin does not.
"""

import contextlib
import json
import os
import sys
import unittest

from gate.runner import allowlists, anchors, docsync, knownissues, law, registry, sprintid, suppressions, teeth
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


NOSEC = 'package x\n\nimport (\n\t"crypto/md5" // #nosec G501\n)\n\nvar _ = md5.Sum(nil) // #nosec G401 -- reason\n'


class NosecIsAWaiver(unittest.TestCase):
    """BK-S4: a linter-native waiver golangci honours is read like //nolint."""

    def test_a_nosec_is_read(self):
        self.assertNotEqual(suppressions.go_directives("x.go", NOSEC), ([], []), "a #nosec passes unseen")

    def test_a_nosec_is_found_by_lawdeltas_prefilter(self):
        self.assertTrue(any(p in NOSEC for p in suppressions.PREFILTER))


RUNS_LESS = {
    "a load_tests hook": "\n\ndef load_tests(loader, tests, pattern):\n    return unittest.TestSuite()\n",
    "a case deleted after the class": "\n\ndel Planted.test_a\n",
}


class TeethCountsWhatRuns(unittest.TestCase):
    """BK-B4: a driver whose module keeps unittest from loading a counted case is a fault."""

    def test_each_module_that_runs_less_than_it_counts_is_a_fault(self):
        for why, tail in RUNS_LESS.items():
            with self.subTest(why=why):
                source = DRIVER.format(body="    def test_a(self):\n        run = self.gate()\n" + RED) + tail
                cases, faults = teeth.python_cases(source, "d.py")
                self.assertEqual(cases, ["Planted.test_a"])
                self.assertTrue(faults, f"{why}: the case is counted but never runs")

    def test_a_case_in_a_class_unittest_does_not_load_is_a_fault(self):
        source = DRIVER.format(body="    def test_a(self):\n        run = self.gate()\n" + RED)
        source = source.replace("class Planted(support.GateCase):", "class Planted:")  # not a TestCase
        cases, faults = teeth.python_cases(source, "d.py")
        self.assertTrue(faults, f"{cases} counted from a class unittest never loads")


ISSUE = '{{"id": "S3.3-2-1", "first_seen": "{first_seen}", "forward": "", "decision": ""}}'
ONE = '{"schema": 1, "note": "n", "entries": [' + ISSUE.format(first_seen="S3.3") + "]}"
TWICE = (
    '{"schema": 1, "note": "n", "entries": ['
    + ISSUE.format(first_seen="S3.3")
    + ", "
    + ISSUE.format(first_seen="S1.1c-2")
    + "]}"
)
ISSUE_ORDER = sprintid.Order((sprintid.Row("S3.3", True), sprintid.Row("S1.1c", False)))


class KnownIssueListedTwice(unittest.TestCase):
    """BK-S5."""

    def test_the_pin_refuses_an_id_listed_twice(self):
        table = "| Id | Found |\n|---|---|\n| S3.3-2-1 | S1.1c-2 |\n"
        self.assertTrue(knownissues.check(TWICE, table, ISSUE_ORDER, lambda path: None), "an id listed twice passes")

    def test_law_delta_counts_every_entry(self):
        self.assertEqual(law.delta("known-issues", ONE, TWICE)[0], "(2 entries)")


class GeneratedHeaderIsAnExemption(unittest.TestCase):
    """BK-S6."""

    def test_a_generated_header_is_read(self):
        source = "// Code generated by hand. DO NOT EDIT.\n\npackage x\n\nfunc F() int { return 1 }\n"
        self.assertNotEqual(suppressions.read("internal/x/x.go", source), ([], []), "the header passes unseen")


class DocsyncDataFileLines(unittest.TestCase):
    """BK-L1."""

    def test_a_data_file_line_lists_no_package(self):
        tree = [
            "tools/deps-allowlist.json                 the dependency register",
            "api/exports.golden                        the hand-written public-export law file",
            "tools/importcheck/rules.json              the layer law",
        ]
        self.assertEqual(
            docsync.missing(["api", "tools", "tools/importcheck"], docsync.listed(tree)),
            ["api", "tools", "tools/importcheck"],
        )


class RefToACommentedHeading(unittest.TestCase):
    """BK-L2."""

    def test_a_heading_inside_an_html_comment_does_not_resolve(self):
        md = "# Title\n\n<!--\n## Gone\n-->\n"
        self.assertNotEqual(anchors.resolves("docs/a.md#gone", lambda path: md), "", "a commented heading resolves")


TEETH = os.path.join(support.GATE, "runner", "testdata", "teeth")


def teeth_fixture(name: str) -> dict:
    with open(os.path.join(TEETH, name), encoding="utf-8") as f:
        return json.load(f)


class PinTiesAProofToItsRow(unittest.TestCase):
    """BK-S7, on test_instruments_pin's own fixture."""

    def test_an_instrument_borrowing_another_rows_driver_is_red(self):
        manifest, rows, world = teeth_fixture("law.json"), teeth_fixture("registry.json"), teeth_fixture("world.json")
        with open(os.path.join(TEETH, "teeth.json"), encoding="utf-8") as f:
            targets = frozenset(e.target for e in allowlists.load(f.read(), "teeth").entries)
        by_id = {i["id"]: i for i in manifest["instruments"]}
        by_id["docsync"]["proofs"] = by_id["lawdelta"]["proofs"]  # docsync's own driver gone, lawdelta's borrowed
        counted = {path: (c["cases"], c["faults"]) for path, c in world["counted"].items()}
        faults = teeth.pin(
            law.load_manifest(json.dumps(manifest)),
            registry.loads(json.dumps(rows)),
            frozenset(world["tracked"]),
            counted,
            targets,
        )
        self.assertTrue(faults, "the docsync instrument passes on the lawdelta row's driver")


if __name__ == "__main__":
    unittest.main()
