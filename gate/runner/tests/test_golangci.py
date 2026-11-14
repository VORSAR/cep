"""The smells row's config pin and report reader (S1.1c-5 design note §2.2; §8 card line 1, rows 1.1-1.6).

The expected document is the design note's §2.1 text, copied here by hand (DESIGN_TOML), never produced by the code
under test. Every expected difference line is §2.2's format: "<name>: <path>: <got!r> ≠ pinned <want!r>",
"<name>: <path>: not pinned (remove it)", "<name>: <path>: missing (pinned <want!r>)".
"""

import copy
import json
import os
import tempfile
import tomllib
import unittest

from gate.runner import golangci

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
TESTDATA = os.path.join(REPO, "gate", "runner", "testdata", "golangci")

DESIGN_TOML = r"""
version = "2"

[run]
relative-path-mode = "gomod"
tests = true
build-tags = []
modules-download-mode = "readonly"

[output]
show-stats = false

[issues]
max-issues-per-linter = 0
max-same-issues = 0
uniq-by-line = false
new = false
fix = false

[severity]
default = "error"

[formatters]
enable = []

[linters]
default = "none"
enable = ["errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive"]

[linters.settings.errcheck]
check-type-assertions = false
check-blank = false
disable-default-exclusions = true
exclude-functions = [
  "(*bytes.Buffer).Write", "(*bytes.Buffer).WriteByte", "(*bytes.Buffer).WriteRune", "(*bytes.Buffer).WriteString",
  "fmt.Print", "fmt.Printf", "fmt.Println",
  "fmt.Fprint(*bytes.Buffer)", "fmt.Fprintf(*bytes.Buffer)", "fmt.Fprintln(*bytes.Buffer)",
  "fmt.Fprint(*strings.Builder)", "fmt.Fprintf(*strings.Builder)", "fmt.Fprintln(*strings.Builder)",
  "fmt.Fprint(os.Stderr)", "fmt.Fprintf(os.Stderr)", "fmt.Fprintln(os.Stderr)",
  "(*io.PipeReader).CloseWithError", "(*io.PipeWriter).CloseWithError",
  "math/rand.Read", "(*math/rand.Rand).Read",
  "(*strings.Builder).Write", "(*strings.Builder).WriteByte",
  "(*strings.Builder).WriteRune", "(*strings.Builder).WriteString",
  "(hash.Hash).Write",
]

[linters.settings.forbidigo]
analyze-types = true
exclude-godoc-examples = false
forbid = [
  { pattern = '^(fmt\.Print.*|print|println)$', msg = "[fmtprint]" },
  { pattern = '^panic$', msg = "[panic]" },
  { pattern = '^reflect\..*$', msg = "[reflect]" },
  { pattern = '^unsafe\..*$', msg = "[unsafe]" },
  { pattern = '^os\.(Getenv|LookupEnv|Environ)$', msg = "[getenv]" },
]

[linters.settings.funlen]
lines = 60
statements = 100000
ignore-comments = false

[linters.settings.gocritic]

[linters.settings.gocyclo]
min-complexity = 10

[linters.settings.lll]
line-length = 120
tab-width = 1

[linters.settings.nolintlint]
require-explanation = true
require-specific = true
allow-unused = false
allow-no-explanation = []

[linters.settings.revive]
enable-all-rules = false
[[linters.settings.revive.rules]]
name = "file-length-limit"
severity = "error"
arguments = [{ max = 500, skipComments = false, skipBlankLines = false }]
[[linters.settings.revive.rules]]
name = "argument-limit"
severity = "error"
arguments = [5]

[linters.exclusions]
generated = "disable"
warn-unused = false
presets = []
paths = []
paths-except = []
[[linters.exclusions.rules]]
path = '_test\.go$'
linters = ["funlen", "gocyclo", "lll", "revive"]
[[linters.exclusions.rules]]
path = '_test\.go$'
linters = ["forbidigo"]
text = '\[reflect\]'
[[linters.exclusions.rules]]
path = '_test\.go$'
linters = ["forbidigo"]
text = '\[fmtprint\]'
[[linters.exclusions.rules]]
path-except = '^internal/(event|model|metrics|clock|pattern|window|expr|text|table|partition)/'
linters = ["forbidigo"]
text = '\[reflect\]'
[[linters.exclusions.rules]]
path = '^(cmd|io)/'
linters = ["forbidigo"]
text = '\[getenv\]'
"""

NAME = ".golangci.toml"


def design() -> dict:
    return tomllib.loads(DESIGN_TOML)


def at(doc: dict, *keys: str) -> dict:
    for key in keys:
        doc = doc[key]
    return doc


def edits() -> list[tuple[str, object, str]]:
    """(subTest name, edit(doc) -> None, an expected line or a "<prefix>…" path prefix), each from §8 row 1.2."""

    def put(*path, value):
        def edit(doc):
            at(doc, *path[:-1])[path[-1]] = value

        return edit

    def drop(*path):
        def edit(doc):
            del at(doc, *path[:-1])[path[-1]]

        return edit

    def fifth_rule(doc):
        doc["linters"]["exclusions"]["rules"].append({"path": "x", "linters": ["lll"]})

    def less_errcheck(doc):
        doc["linters"]["settings"]["errcheck"]["exclude-functions"].remove("(hash.Hash).Write")

    def less_revive(doc):
        doc["linters"]["enable"].remove("revive")

    def formats(doc):
        doc["output"]["formats"] = {"json": {"path": "x.json"}}

    return [
        (
            "issues.new-from-rev",
            put("issues", "new-from-rev", value="HEAD"),
            "issues.new-from-rev: not pinned (remove it)",
        ),
        (
            "issues.new-from-patch",
            put("issues", "new-from-patch", value="x.patch"),
            "issues.new-from-patch: not pinned (remove it)",
        ),
        ("issues.whole-files", put("issues", "whole-files", value=True), "issues.whole-files: not pinned (remove it)"),
        ("run.tests=false", put("run", "tests", value=False), "run.tests: False ≠ pinned True"),
        (
            "max-issues-per-linter removed",
            drop("issues", "max-issues-per-linter"),
            "issues.max-issues-per-linter: missing (pinned 0)",
        ),
        ("max-same-issues removed", drop("issues", "max-same-issues"), "issues.max-same-issues: missing (pinned 0)"),
        ("uniq-by-line=true", put("issues", "uniq-by-line", value=True), "issues.uniq-by-line: True ≠ pinned False"),
        (
            'generated="lax"',
            put("linters", "exclusions", "generated", value="lax"),
            "linters.exclusions.generated: 'lax' ≠ pinned 'disable'",
        ),
        (
            "funlen.statements=-1",
            put("linters", "settings", "funlen", "statements", value=-1),
            "linters.settings.funlen.statements: -1 ≠ pinned 100000",
        ),
        (
            "exclude-godoc-examples removed",
            drop("linters", "settings", "forbidigo", "exclude-godoc-examples"),
            "linters.settings.forbidigo.exclude-godoc-examples: missing (pinned False)",
        ),
        ('exclusions.paths=["x"]', put("linters", "exclusions", "paths", value=["x"]), "linters.exclusions.paths…"),
        ("a fifth exclusions.rules entry", fifth_rule, "linters.exclusions.rules…"),
        ('run.build-tags=["x"]', put("run", "build-tags", value=["x"]), "run.build-tags…"),
        ("output.formats.json.path", formats, "output.formats…"),
        (
            'severity.default="warning"',
            put("severity", "default", value="warning"),
            "severity.default: 'warning' ≠ pinned 'error'",
        ),
        (
            "errcheck.disable-default-exclusions removed",
            drop("linters", "settings", "errcheck", "disable-default-exclusions"),
            "linters.settings.errcheck.disable-default-exclusions: missing (pinned True)",
        ),
        ("one exclude-functions entry removed", less_errcheck, "linters.settings.errcheck.exclude-functions…"),
        ("enable minus revive", less_revive, "linters.enable…"),
    ]


class PinnedDocument(unittest.TestCase):
    def assert_named(self, lines: list[str], expected: str) -> None:
        self.assertTrue(lines, "an edit gave no difference")
        for line in lines:
            self.assertTrue(line.startswith(f"{NAME}: "), line)
        if expected.endswith("…"):
            prefix = f"{NAME}: {expected.removesuffix('…')}"
            self.assertTrue(any(line.startswith(prefix) for line in lines), (prefix, lines))
        else:
            self.assertIn(f"{NAME}: {expected}", lines)

    def test_the_committed_config_equals_pinned(self):
        with open(os.path.join(REPO, golangci.CONFIG), "rb") as f:
            doc = golangci.load(f.read().decode("utf-8"))
        self.assertEqual(golangci.differences(doc), [])
        self.assertEqual(doc, design())  # the committed file is the design's §2.1 document

    def test_each_hiding_edit_is_named(self):
        self.assertEqual(golangci.differences(design()), [])  # witness: the unedited document has no difference
        for name, edit, expected in edits():
            with self.subTest(name):
                doc = design()
                edit(doc)
                self.assertNotEqual(doc, design())
                self.assert_named(golangci.differences(doc), expected)

    def test_a_type_change_is_a_difference(self):
        cases = [
            (
                "arguments=[5.0]",
                ("linters", "settings", "revive", "rules", 1, "arguments", 0),
                5.0,
                "linters.settings.revive.rules[1].arguments[0]: 5.0 ≠ pinned 5",
            ),
            (
                "max=500.0",
                ("linters", "settings", "revive", "rules", 0, "arguments", 0, "max"),
                500.0,
                "linters.settings.revive.rules[0].arguments[0].max: 500.0 ≠ pinned 500",
            ),
            ("run.tests=1", ("run", "tests"), 1, "run.tests: 1 ≠ pinned True"),
            (
                "funlen.lines=60.0",
                ("linters", "settings", "funlen", "lines"),
                60.0,
                "linters.settings.funlen.lines: 60.0 ≠ pinned 60",
            ),
        ]
        for name, path, value, expected in cases:
            with self.subTest(name):
                doc = copy.deepcopy(design())
                at(doc, *path[:-1])[path[-1]] = value
                self.assertEqual(doc, design())  # M-11: Python's == calls the two equal
                self.assertIn(f"{NAME}: {expected}", golangci.differences(doc))

    def test_a_duplicate_key_is_a_config_error(self):
        with self.assertRaises(golangci.ConfigError) as caught:
            golangci.load(DESIGN_TOML + '\n[severity]\ndefault = "warning"\n')
        self.assertTrue(str(caught.exception).startswith(f"{NAME}: "), str(caught.exception))
        with self.assertRaises(golangci.ConfigError):
            golangci.load("[run]\ntests = true\ntests = false\n")

    def test_an_upper_case_twin_key_is_not_pinned(self):
        doc = golangci.load(DESIGN_TOML + '\n[LINTERS]\ndefault = "all"\n')
        self.assertIn(f"{NAME}: LINTERS: not pinned (remove it)", golangci.differences(doc))


class ReadReport(unittest.TestCase):
    def write(self, text: str) -> str:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path = os.path.join(tmp.name, "smells-linux.json")
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def test_issues_error_and_warnings_are_read(self):
        path = os.path.join(TESTDATA, "report-error-warning.json")  # recorded (qa-design-exp/m1, float.json)
        report = golangci.read_report(path)
        self.assertEqual(
            report.issues,
            (
                golangci.Issue(
                    linter="forbidigo",
                    rule="forbidigo",
                    text='use of `fmt.Println` forbidden because "[fmtprint]"',
                    file="gen/tools/t/main.go",
                    line=3,
                    column=0,
                ),
            ),
        )
        self.assertEqual(
            report.error,
            'setup revive: cannot configure rule: "argument-limit": invalid value passed as argument number to the'
            ' "argument-limit" rule',
        )
        self.assertEqual(len(report.warnings), 1)
        self.assertTrue(report.warnings[0].startswith("Failed to get line 3 for file /work/m1/gen/tools/t/main.go"))
        self.assertEqual(report.enabled, frozenset(golangci.LINTERS) | {"typecheck"})

    def test_null_issues_is_empty(self):
        for doc in ({"Issues": None, "Report": {"Linters": []}}, {"Report": {"Linters": []}}):
            with self.subTest(doc=doc):
                report = golangci.read_report(self.write(json.dumps(doc)))
                self.assertEqual(report.issues, ())
                self.assertEqual(report.error, "")
                self.assertEqual(report.warnings, ())

    def test_a_truncated_document_is_a_report_error(self):
        with open(os.path.join(TESTDATA, "report-error-warning.json"), encoding="utf-8") as f:
            whole = f.read()
        path = self.write(whole[: len(whole) // 2])
        with self.assertRaises(golangci.ReportError) as caught:
            golangci.read_report(path)
        self.assertIn(path, str(caught.exception))


class RuleOf(unittest.TestCase):
    def test_reviews_two_size_rules_by_prefix(self):
        texts = {
            "argument-limit": "argument-limit: maximum number of arguments per function exceeded; max 5 but got 6",
            "file-length-limit": "file-length-limit: file length is 501 lines, which exceeds the limit of 500",
        }
        for rule, text in texts.items():
            with self.subTest(rule):
                self.assertEqual(golangci.rule_of({"FromLinter": "revive", "Text": text}), rule)
        for linter in ("funlen", "gocyclo", "errcheck", "forbidigo", "gocritic", "lll", "nolintlint", "typecheck"):
            with self.subTest(linter):
                self.assertEqual(golangci.rule_of({"FromLinter": linter, "Text": "argument-limit: x"}), linter)

    def test_another_revive_prefix_is_revive(self):
        for text in ("unused-parameter: parameter 'x' seems to be unused", "no prefix at all", ""):
            with self.subTest(text):
                self.assertEqual(golangci.rule_of({"FromLinter": "revive", "Text": text}), "revive")


if __name__ == "__main__":
    unittest.main()
