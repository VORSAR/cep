"""gate/runner/suppressions.py (S1.1c-2 design §2.3, §2.7, §2.8; measured tables §0.2, §0.3) and its row (§2.4).

`go_comments` has one case per scanner state. §0.3's golangci-lint table and §0.2's ruff table are held here as data
and every form's classification is asserted, so a recogniser that drifts from what the pinned tools do is red. Every
Go-directive and Python-marker text of §2.8 is asserted verbatim. The policy's two examples (§2.7) are checked with
§2.7's context and the linter injected as enabled; they are fixtures here until the coordinator writes them into
DEVELOPMENT.md when lane A lands, after which this test should read DEVELOPMENT.md's fenced examples instead.
"""

import contextlib
import json
import os
import re
import sys
import unittest
from datetime import date

from gate.runner import allowlists, anchors, sprintid, suppressions
from gate.runner.rows import suppressions as row
from gate.runner.tests import support

TODAY = date(2026, 11, 12)
ORDER = sprintid.Order(
    (
        sprintid.Row("S1.1a-1", True),
        sprintid.Row("S3.1", True),
        sprintid.Row("S3.3", True),
        sprintid.Row("S1.1c", False),
        sprintid.Row("S4.1", False),
    )
)
META = (
    'PERMANENT reason="fixture generator; output is never security-relevant" ref=docs/sprints/decisions.md#title'
    " sprint=S1.1c-2"
)
# S1.1c-5 design §4.2: the non-canonical spellings' text names lower case and one list too.
WRITE = re.compile(
    r"^p\.go:(\d+): write //nolint:(<linter>|\S*) exactly \(two slashes, no space, lower case, one list\)$"
)
METADATA = 'a suppression carries its metadata: // PERMANENT reason="…" ref=… sprint=…'
RUFF_IGNORES = "ruff ignores this form; write # fmt: skip  # <reason>"
NO_REASON = "write its reason as another comment: # fmt: skip  # <reason, 12+ characters>"
ON_STATEMENT = "write # fmt: skip  # <reason> on the statement"
NOQA = "ruff runs --ignore-noqa: this comment waives nothing; remove it"
SPELLING = "ruff honours this spelling too; write # fmt: skip  # <reason>"


# §2.7's examples, read from DEVELOPMENT.md's suppression policy itself, so the policy and the checker cannot drift
# (DQ-S6, DA-S14): every line of the policy's ```go fence that starts //nolint.
def policy_examples() -> list[str]:
    with open(os.path.join(support.REPO, "docs", "development", "DEVELOPMENT.md"), encoding="utf-8") as f:
        text = f.read()
    section = text[text.index("## Suppression policy") :]
    fence = section[section.index("```go\n") + len("```go\n") : section.index("```", section.index("```go\n") + 6)]
    return [line for line in fence.splitlines() if line.startswith("//nolint")]


# §0.3, as data: (comment as written, golangci honours it, canonical).
GOLANGCI = (
    (f"//nolint:errcheck // {META}", True, True),
    (f"// nolint:errcheck // {META}", True, False),
    (f"//  nolint:errcheck // {META}", True, False),
    (f"//nolint errcheck // {META}", True, False),
    ("// nolint", True, False),
    (f"///nolint:errcheck // {META}", True, False),
    (f"//NOLINT:errcheck // {META}", False, False),
    (f"//\tnolint:errcheck // {META}", False, False),
    (f"// x nolint:errcheck // {META}", False, False),
    (f"// text //nolint:errcheck // {META}", False, False),
)
BLOCKS = ("/* nolint:errcheck */", "/*nolint:errcheck*/")

# §0.2, as data: (comment on the line, the faults' texts in any order; () is a valid marker).
REASON = "a reason that is long enough"
RUFF = (
    (f"# fmt: skip  # {REASON}", ()),
    (f"# fmt:skip  # {REASON}", ()),
    (f"# fmt: skip # {REASON}", ()),
    (f"# {REASON}  # fmt: skip", ()),
    ("# fmt: skip", (NO_REASON,)),
    ("# fmt:skip", (NO_REASON,)),
    ("# fmt: skip -- because of a long reason", (RUFF_IGNORES,)),
    ("# fmt: skip; because of a long reason", (RUFF_IGNORES,)),
    # design ambiguity: §2.3 lists "case variants (FMT: SKIP)" between two texts; ruff ignores them (§0.2), so they
    # take "ruff ignores this form".
    (f"# FMT: SKIP  # {REASON}", (RUFF_IGNORES,)),
    ("# fmt: off", (ON_STATEMENT,)),
    ("# fmt: on", (ON_STATEMENT,)),
    ("# yapf: disable", (ON_STATEMENT,)),
    ("# yapf: enable", (ON_STATEMENT,)),
    ("# isort: skip", (ON_STATEMENT,)),
    ("# isort: skip_file", (ON_STATEMENT,)),
    ("# ruff: isort: skip_file", (ON_STATEMENT,)),
    ("# noqa", (NOQA,)),
    ("# noqa: E402", (NOQA,)),
    # design ambiguity: a noqa segment is both a fault of its own and "not a reason", so the skip beside it is the
    # missing-reason fault too.
    ("# noqa # fmt: skip", (NOQA, NO_REASON)),
    # BK-B1 (S1.1c-2's fix round), measured with the pinned ruff 0.17.3 (`ruff format --isolated`, stdin; the
    # statement's layout kept or not): ruff strips the "#", trims the whitespace, and reads `fmt:` + any run of
    # spaces, tabs or form feeds + `skip`/`off`/`on` (`yapf:` likewise + `disable`/`enable`). Each spelling below
    # keeps the layout (`fmt: on`/`yapf: enable` close a region), so each is red; the two skips design §2.3 accepts
    # (one space, or none) stay accepted.
    (f"# fmt:  skip  # {REASON}", (SPELLING,)),
    (f"# fmt:\tskip  # {REASON}", (SPELLING,)),
    (f"#  fmt:   skip  # {REASON}", (SPELLING,)),
    (f"# fmt: \tskip  # {REASON}", (SPELLING,)),
    (f"#fmt:  skip  # {REASON}", (SPELLING,)),
    (f"# {REASON}  # fmt:  skip", (SPELLING,)),
    ("# fmt:  skip", (SPELLING,)),
    (f"# FMT:  SKIP  # {REASON}", (RUFF_IGNORES,)),
    ("# fmt:  skip -- because of a long reason", (RUFF_IGNORES,)),
    ("# fmt:  off", (ON_STATEMENT,)),
    ("# fmt:\toff", (ON_STATEMENT,)),
    ("# fmt:  on", (ON_STATEMENT,)),
    ("# yapf:  disable", (ON_STATEMENT,)),
    ("# yapf:\tdisable", (ON_STATEMENT,)),
    ("# yapf:  enable", (ON_STATEMENT,)),
)

# W1-F3: the waivers the pinned tools honour, valid or not, each read as one exemption (the LAW Δ count); measured
# with the pinned ruff 0.17.3: the skip and region forms above by `ruff format`, the isort forms by `ruff check
# --select I --ignore-noqa` (`isort: skip_file`, `ruff: isort: skip_file`, `isort: off` at the head of a file and a
# trailing `isort: skip` waive I001; `isort: split` and `isort: on` waive nothing). (comment, waivers read)
HONOURED = (
    (f"# fmt: skip  # {REASON}", 1),
    ("# fmt: skip", 1),
    ("# fmt:skip", 1),
    ("# fmt:  skip", 1),
    (f"# fmt:\tskip  # {REASON}", 1),
    ("# fmt: off", 1),
    ("# fmt:  off", 1),
    ("# fmt: on", 0),
    ("# yapf: disable", 1),
    ("# yapf:\tdisable", 1),
    ("# yapf: enable", 0),
    ("# isort: skip", 1),
    ("# isort: skip_file", 1),
    ("# ruff: isort: skip_file", 1),
    ("# noqa", 0),
    (f"# FMT: SKIP  # {REASON}", 0),
    ("# fmt: skip -- because of a long reason", 0),
)


def ctx(tier: str = "T2") -> allowlists.Ctx:
    def resolve(ref: str) -> str:
        return anchors.resolves(ref, read_repo)

    return allowlists.Ctx(
        today=TODAY,
        tier=tier,
        order=ORDER,
        current=sprintid.parse("S1.1c-2"),
        issues=frozenset({"S3.3-2-1"}),
        resolve=resolve,
    )


def read_repo(path: str) -> str | None:
    """A tracked Markdown file of this repository, read from the work tree (the examples' ref is real)."""
    full = os.path.join(support.REPO, path)
    if not path.endswith(".md") or not os.path.isfile(full):
        return None
    with open(full, encoding="utf-8") as f:
        return f.read()


def fixture_ctx() -> allowlists.Ctx:
    """The META ref resolves through a fake: docs/sprints/decisions.md#title only."""
    return allowlists.Ctx(
        today=TODAY,
        tier="T2",
        order=ORDER,
        current=sprintid.parse("S1.1c-2"),
        issues=frozenset({"S3.3-2-1"}),
        resolve=lambda ref: "" if ref == "docs/sprints/decisions.md#title" else f"{ref} does not resolve",
    )


def go_line(comment: str) -> str:
    """A Go file whose line 3 holds the comment after a statement."""
    return f"package p\n\nvar _ = 1 {comment}\n"


class GoComments(unittest.TestCase):
    def comments(self, source: str) -> list[tuple[int, str]]:
        return list(suppressions.go_comments(source))

    def test_code_and_line(self):
        self.assertEqual(
            self.comments("x := 1 // hello\ny := 2\n// two // three\n"), [(1, " hello"), (3, " two // three")]
        )

    def test_block(self):
        self.assertEqual(self.comments("/* // not a line\n//nolint:x\n*/ x := 1 // real\n"), [(3, " real")])

    def test_string_with_an_escaped_quote(self):
        self.assertEqual(self.comments('s := "a \\" // b" // c\n'), [(1, " c")])

    def test_a_string_ends_at_a_newline(self):
        self.assertEqual(self.comments('s := "abc\n// x\n'), [(2, " x")])

    def test_raw_across_lines(self):
        self.assertEqual(self.comments('s := `a\n// not\n"` // yes\n'), [(3, " yes")])

    def test_rune_with_a_double_quote(self):
        self.assertEqual(self.comments("r := '\"' // c\n"), [(1, " c")])
        self.assertEqual(self.comments("r := '\\'' // c\n"), [(1, " c")])

    def test_nolint_in_a_string_is_nothing(self):
        self.assertEqual(self.comments('x := "//nolint:x"\n'), [])
        directives, faults = suppressions.go_directives("p.go", 'package p\n\nvar x = "//nolint:errcheck"\n')
        self.assertEqual((directives, faults), ([], []))


class GoDirectives(unittest.TestCase):
    def test_golangcis_table(self):
        for comment, honoured, canonical in GOLANGCI:
            with self.subTest(comment=comment):
                directives, faults = suppressions.go_directives("p.go", go_line(comment))
                if not honoured:
                    self.assertEqual((directives, faults), ([], []))
                elif canonical:
                    self.assertEqual(faults, [])
                    self.assertEqual(len(directives), 1)
                    self.assertEqual((directives[0].path, directives[0].line), ("p.go", 3))
                else:
                    # design ambiguity: "<linter>" in the text may be written literally or as the linter named.
                    self.assertEqual(len(faults), 1, faults)
                    self.assertRegex(faults[0], WRITE)
                    self.assertEqual(WRITE.match(faults[0]).group(1), "3")

    def test_block_comments_are_not_directives(self):
        for comment in BLOCKS:
            with self.subTest(comment=comment):
                self.assertEqual(suppressions.go_directives("p.go", go_line(comment)), ([], []))

    def test_the_canonical_directive_is_read(self):
        directives, faults = suppressions.go_directives("p.go", go_line(f"//nolint:errcheck,gosec // {META}"))
        self.assertEqual(faults, [])
        self.assertEqual(
            directives,
            [
                suppressions.Directive(
                    "p.go",
                    3,
                    ("errcheck", "gosec"),
                    "PERMANENT",
                    {
                        "reason": "fixture generator; output is never security-relevant",
                        "ref": "docs/sprints/decisions.md#title",
                        "sprint": "S1.1c-2",
                    },
                )
            ],
        )

    def test_an_escaped_quote_in_the_reason_is_allowed(self):
        comment = (
            '//nolint:errcheck // PERMANENT reason="the \\"x\\" fixture generator" ref=docs/sprints/decisions.md#title'
            " sprint=S1.1c-2"
        )
        directives, faults = suppressions.go_directives("p.go", go_line(comment))
        self.assertEqual((len(directives), faults), (1, []))

    def assertFault(self, comment: str, text: str) -> None:
        directives, faults = suppressions.go_directives("p.go", go_line(comment))
        self.assertEqual(faults, [f"p.go:3: {text}"])

    def test_an_empty_linter_list(self):
        directives, faults = suppressions.go_directives("p.go", go_line(f"//nolint: // {META}"))
        self.assertEqual(len(faults), 1, faults)
        self.assertRegex(faults[0], WRITE)

    def test_a_trailing_comma(self):
        directives, faults = suppressions.go_directives("p.go", go_line(f"//nolint:errcheck, // {META}"))
        self.assertEqual(len(faults), 1, faults)
        self.assertRegex(faults[0], WRITE)

    def test_a_bare_nolint(self):
        directives, faults = suppressions.go_directives("p.go", go_line(f"//nolint // {META}"))
        self.assertEqual(len(faults), 1, faults)
        self.assertRegex(faults[0], WRITE)

    def test_missing_metadata(self):
        # design ambiguity: a canonical //nolint:<l> with no " // " metadata takes the metadata text, not "write".
        self.assertFault("//nolint:errcheck", METADATA)

    def test_unterminated_quote(self):
        self.assertFault(
            '//nolint:errcheck // PERMANENT reason="never closed ref=docs/sprints/decisions.md#title sprint=S1.1c-2',
            "unterminated quote",
        )

    def test_duplicate_key(self):
        self.assertFault(f"//nolint:errcheck // {META} sprint=S1.1c-2", "duplicate key sprint")

    def test_unknown_key(self):
        self.assertFault(f"//nolint:errcheck // {META} owner=me", "unknown key owner")


class Check(unittest.TestCase):
    def directives(self, comment: str) -> list[suppressions.Directive]:
        directives, faults = suppressions.go_directives("p.go", go_line(comment))
        self.assertEqual(faults, [])
        return directives

    def test_a_linter_outside_the_enabled_set_is_named(self):
        # S1.1c-5 design §4.2: the enabled set is the config's, passed in; ENABLED is gone (Enabled below).
        got = suppressions.check(self.directives(f"//nolint:errcheck // {META}"), fixture_ctx(), frozenset())
        self.assertEqual(got, ["p.go:3: //nolint:errcheck: errcheck is not enabled in .golangci.toml"])

    def test_a_valid_directive_with_its_linter_enabled_is_green(self):
        got = suppressions.check(
            self.directives(f"//nolint:errcheck // {META}"), fixture_ctx(), frozenset({"errcheck"})
        )
        self.assertEqual(got, [])

    def test_each_linter_not_enabled_is_named(self):
        got = suppressions.check(
            self.directives(f"//nolint:errcheck,gosec // {META}"), fixture_ctx(), frozenset({"errcheck"})
        )
        self.assertEqual(got, ["p.go:3: //nolint:gosec: gosec is not enabled in .golangci.toml"])

    def test_the_field_rules_apply(self):
        comment = '//nolint:errcheck // PERMANENT reason="short" ref=docs/sprints/decisions.md#title sprint=S1.1c-2'
        got = suppressions.check(self.directives(comment), fixture_ctx(), frozenset({"errcheck"}))
        self.assertEqual(got, ["p.go:3: reason must be at least 12 characters"])

    def test_a_temp_directive_expired_at_t4(self):
        comment = (
            '//nolint:errcheck // TEMP reason="close error surfaced later" expiry=2026-11-11 issue=S3.3-2-1'
            " sprint=S1.1c-2"
        )
        directives = self.directives(comment)
        c = fixture_ctx()
        self.assertEqual(suppressions.check(directives, c, frozenset({"errcheck"})), [])
        t4 = allowlists.Ctx(c.today, "T4", c.order, c.current, c.issues, c.resolve)
        self.assertEqual(suppressions.check(directives, t4, frozenset({"errcheck"})), ["p.go:3: expired 2026-11-11"])

    def test_the_policys_examples_are_valid(self):
        # §2.7: Ctx(today=2026-11-12, tier="T2", order=<fixture>, current=S1.1c-2, issues={"S3.3-2-1"}), the
        # linter injected as enabled. Read from DEVELOPMENT.md (two examples today; none is a failure).
        examples = policy_examples()
        self.assertEqual(len(examples), 2, examples)
        for example in examples:
            with self.subTest(example=example):
                directives, faults = suppressions.go_directives("x.go", go_line(example))
                self.assertEqual((len(directives), faults), (1, []))
                self.assertEqual(suppressions.check(directives, ctx(), frozenset(directives[0].linters)), [])


class PyMarkers(unittest.TestCase):
    def test_ruffs_table(self):
        for comment, texts in RUFF:
            with self.subTest(comment=comment):
                markers, faults = suppressions.py_markers("gate/x.py", f"import os\nx = 1  {comment}\n")
                self.assertEqual(sorted(faults), sorted(f"gate/x.py:2: {t}" for t in texts))
                if not texts:
                    self.assertEqual(len(markers), 1)
                    self.assertEqual(
                        markers[0],
                        suppressions.Directive("gate/x.py", 2, ("fmt",), "MARKER", {"reason": REASON}),
                    )

    def test_a_reason_of_twelve_characters_is_green_eleven_red(self):
        markers, faults = suppressions.py_markers("gate/x.py", "x = 1  # fmt: skip  # " + "r" * 12 + "\n")
        self.assertEqual((len(markers), faults), (1, []))
        markers, faults = suppressions.py_markers("gate/x.py", "x = 1  # fmt: skip  # " + "r" * 11 + "\n")
        self.assertEqual(faults, [f"gate/x.py:1: {NO_REASON}"])

    def test_the_reason_is_the_other_segments_joined(self):
        markers, _ = suppressions.py_markers("gate/x.py", "x = 1  # first part  # fmt: skip  # second part\n")
        self.assertEqual(markers[0].meta, {"reason": "first part  # second part"})

    def test_a_marker_inside_a_string_is_nothing(self):
        self.assertEqual(suppressions.py_markers("gate/x.py", 'x = "# fmt: skip"\ny = "# noqa"\n'), ([], []))

    def test_a_noqa_in_a_hook_script(self):
        _, faults = suppressions.py_markers(".githooks/pre-commit", "#!/usr/bin/env python3\nimport os  # noqa\n")
        self.assertEqual(faults, [f".githooks/pre-commit:2: {NOQA}"])


class Prefilter(unittest.TestCase):
    def test_every_recognised_form_contains_a_pattern(self):
        # design ambiguity: a case variant (an upper-case FMT: SKIP) is recognised as red but no PREFILTER pattern
        # (git grep is case-sensitive) matches it; it never counts as an exemption, so it is left out here.
        forms = [c for c, honoured, _ in GOLANGCI if honoured] + [c for c, _ in RUFF if "FMT" not in c]
        for form in forms:
            with self.subTest(form=form):
                self.assertTrue(any(p in form for p in suppressions.PREFILTER), form)


class EveryHonouredWaiverIsRead(unittest.TestCase):
    """W1-F3: the readers return every waiver the pinned tools honour, valid or not, so the LAW Δ count holds a
    faulted one too; the fault is the reader's, and `check` adds nothing for it."""

    def test_the_python_waivers(self):
        for comment, n in HONOURED:
            with self.subTest(comment=comment):
                self.assertEqual(suppressions.count("gate/x.py", f"import os\nx = 1  {comment}\n"), n)

    def test_the_go_directives_golangci_honours(self):
        for comment, honoured, _ in GOLANGCI:
            with self.subTest(comment=comment):
                self.assertEqual(suppressions.count("p.go", go_line(comment)), 1 if honoured else 0)

    def test_a_reasonless_skip_is_read_with_an_empty_reason_and_not_valid(self):
        markers, faults = suppressions.py_markers("gate/x.py", "x = 1  # fmt: skip\n")
        self.assertEqual(faults, [f"gate/x.py:1: {NO_REASON}"])
        want = suppressions.Directive("gate/x.py", 1, ("fmt",), "MARKER", {"reason": ""}, valid=False)
        self.assertEqual(markers, [want])

    def test_check_adds_nothing_for_a_directive_its_reader_faulted(self):
        directives, faults = suppressions.go_directives("p.go", go_line(f"// nolint:errcheck // {META}"))
        self.assertEqual((len(directives), len(faults)), (1, 1), faults)
        self.assertFalse(directives[0].valid)
        self.assertEqual(suppressions.check(directives, fixture_ctx(), frozenset()), [])


GENERATED_HEADER = "// Code generated by hand. DO NOT EDIT.\n"


class GeneratedMarker(unittest.TestCase):
    """BK-S6 (owner, 2026-11-12, S1.1c-2's review and break-it 3): the generated-code marker is Go's own rule
    (`go/ast.IsGenerated`): a line `// Code generated <generator> DO NOT EDIT.` in a comment before the package
    clause. It is read as an in-code exemption of kind GENERATED, line numbered; the row requires it listed."""

    def generated(self, source: str) -> list[suppressions.Directive]:
        return [d for d in suppressions.read("internal/x/x.go", source)[0] if d.kind == "GENERATED"]

    def test_before_the_package_clause_it_is_read(self):
        cases = {
            "the first line": (GENERATED_HEADER + "\npackage x\n", 1),
            "after another comment": ("// Package x is x.\n" + GENERATED_HEADER + "package x\n", 2),
            "a carriage return before the newline (Go strips it)": (
                GENERATED_HEADER.replace("\n", "\r\n") + "package x\n",
                1,
            ),
            "a line of a block comment": ("/*\n" + GENERATED_HEADER + "*/\npackage x\n", 2),
        }
        for why, (source, line) in cases.items():
            with self.subTest(why=why):
                want = suppressions.Directive("internal/x/x.go", line, ("generated",), "GENERATED", {})
                self.assertEqual(self.generated(source), [want])

    def test_anything_else_is_no_marker(self):
        cases = {
            "after the package clause": "package x\n\n" + GENERATED_HEADER,
            "no generator named": "// Code generated DO NOT EDIT.\npackage x\n",
            "no final period": "// Code generated by hand. DO NOT EDIT\npackage x\n",
            "a block comment's own text": "/* Code generated by hand. DO NOT EDIT. */\npackage x\n",
            "in a string": 'package x\n\nvar s = "// Code generated by hand. DO NOT EDIT."\n',
        }
        for why, source in cases.items():
            with self.subTest(why=why):
                self.assertEqual(self.generated(source), [])

    def test_the_prefilter_finds_it(self):
        self.assertTrue(any(p in GENERATED_HEADER for p in suppressions.PREFILTER))


def pinned_ruff_toml() -> str:
    """The committed ruff.toml without its per-file waivers: the whole ruff pin's green base (S1.1c-5 design §5), so
    a case adds only the setting it is about."""
    with open(os.path.join(support.REPO, "ruff.toml"), encoding="utf-8") as f:
        text = f.read()
    return text[: text.index("[lint.per-file-ignores]")]


class RuffPinRefuses(support.GateTest):
    """BK-B2: ruff.toml settings the pin does not read are red (the pin reads only [lint]'s per-file tables)."""

    def pin(self, ruff_toml: str) -> list[str]:
        self.write("ruff.toml", ruff_toml)
        self.write("docs/base.toml", "")
        with contextlib.chdir(self.repo):
            return row.ruff_pin({})

    def test_extend_is_refused_even_naming_an_empty_file(self):
        self.assertEqual(
            self.pin('extend = "docs/base.toml"\n' + pinned_ruff_toml()),
            ["ruff.toml sets extend: the ruff pin reads ruff.toml alone; write every setting there"],
        )

    def test_top_level_waivers_are_refused(self):
        for key, value in (
            ("ignore", '["TID251"]'),
            ("extend-ignore", '["TID251"]'),
            ("per-file-ignores", '{"gate/x.py" = ["TID251"]}'),
            ("extend-per-file-ignores", '{"gate/x.py" = ["TID251"]}'),
        ):
            with self.subTest(key=key):
                self.assertEqual(
                    self.pin(f"{key} = {value}\n" + pinned_ruff_toml()),
                    [f"ruff.toml sets top-level {key}: write per-file waivers under [lint] (ruff deprecates it)"],
                )


SHAPE = "ruff.toml: lint.{key} is not a table of code lists"


class RuffWaiverTableOfTheWrongShape(support.GateTest):
    """lint.per-file-ignores is left out of the whole-document pin (ruff.json checks it both ways), so its shape is
    checked on its own: a table of string lists, or one named fault and no both-ways comparison."""

    def pin(self, lines: str) -> list[str]:
        text = pinned_ruff_toml().replace("[lint]\n", f"[lint]\n{lines}", 1)
        self.assertNotEqual(text, pinned_ruff_toml())  # witness: the edit applied
        self.write("ruff.toml", text)
        with contextlib.chdir(self.repo):
            try:
                return row.ruff_pin({})
            except (AttributeError, TypeError) as e:  # the row died with a traceback and no named fault
                self.fail(f"ruff_pin raised {e!r}")

    def test_a_list_in_place_of_the_table_is_named(self):
        self.assertEqual(self.pin('per-file-ignores = ["E501"]\n'), [SHAPE.format(key="per-file-ignores")])

    def test_a_string_in_place_of_a_code_list_is_named_once(self):
        self.assertEqual(
            self.pin('per-file-ignores = {"gate/x.py" = "E501"}\n'), [SHAPE.format(key="per-file-ignores")]
        )

    def test_a_code_that_is_not_a_string_is_named(self):
        self.assertEqual(self.pin('per-file-ignores = {"gate/x.py" = [501]}\n'), [SHAPE.format(key="per-file-ignores")])

    def test_an_extend_table_of_the_wrong_shape_is_named(self):
        self.assertEqual(
            self.pin('extend-per-file-ignores = ["E501"]\n'),
            [
                "ruff.toml: lint.extend-per-file-ignores: not pinned (remove it)",
                SHAPE.format(key="extend-per-file-ignores"),
            ],
        )


IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")
ROW = os.path.join(support.REPO, "gate", "runner", "rows", "suppressions.py")
SPRINTS = '{"schema": 1, "note": "n", "rows": [{"id": "S3.1", "closed": true}, {"id": "S1.1c", "closed": false}]}\n'
KNOWN = (
    '{"schema": 1, "note": "n", "entries": [{"id": "S3.3-2-1", "first_seen": "S3.3", "forward": "", "decision": ""}]}\n'
)


class Row(support.GateTest):
    """The row in a world of its own: the files it reads, a files list, GATE_TIER (design §2.4)."""

    def world(self, purity_ref: str) -> None:
        self.track("gate/sprints.json", SPRINTS)
        self.track("gate/current-sprint", "S1.1c-2\n")
        self.track("gate/known-issues.json", KNOWN)
        self.track("ruff.toml", pinned_ruff_toml())
        with open(os.path.join(support.REPO, ".golangci.toml"), encoding="utf-8") as f:
            self.track(".golangci.toml", f.read())  # the enabled set is the config's (S1.1c-5 design §4.2)
        self.track("docs/sprints/decisions.md", "# Title\n")
        self.write("docs/architecture/adr/untracked.md", "# Title\n")
        entry = (
            '{"target": "internal/table: sync.Mutex", "kind": "PERMANENT", "reason": "the one write mutex here",'
            f' "ref": "{purity_ref}", "sprint": "S3.1-2"}}'
        )
        self.track(
            "tools/allowlists/purity.json",
            f'{{"schema": 1, "note": "n", "rule": "purity", "entries": [{entry}]}}\n',
        )

    def run_row(self, files: list[str]) -> tuple[int, str]:
        self.git(*IDENTITY, "commit", "-q", "-m", "world")
        listing = os.path.join(self.root, "files.nul")
        with open(listing, "wb") as f:
            f.write(b"".join(p.encode() + b"\0" for p in files))
        row_dir = os.path.join(self.root, "row")
        os.makedirs(row_dir, exist_ok=True)
        env = dict(support.base_env(), GATE_ROW_DIR=row_dir, GATE_TIER="T2")
        code, out, err = support.run_tool([sys.executable, "-I", "-B", "-X", "utf8", ROW, listing], self.repo, env)
        return code, out.decode("utf-8", errors="replace") + err.decode("utf-8", errors="replace")

    def test_a_clean_world_is_green(self):
        self.world("docs/sprints/decisions.md#title")
        code, out = self.run_row(["tools/allowlists/purity.json"])
        self.assertEqual(code, 0, out)

    def test_an_untracked_doc_never_satisfies_a_ref(self):
        # DA-L5: `read` gives None for a file git does not track, though it is on the disk.
        self.world("docs/architecture/adr/untracked.md#title")
        code, out = self.run_row(["tools/allowlists/purity.json"])
        self.assertEqual(code, 1, out)
        self.assertIn("docs/architecture/adr/untracked.md is not a tracked Markdown file", out)

    def test_the_hooks_are_in_scope(self):
        self.world("docs/sprints/decisions.md#title")
        self.track(".githooks/pre-commit", "#!/usr/bin/env python3\nimport os  # noqa\n")
        code, out = self.run_row(["tools/allowlists/purity.json", ".githooks/pre-commit"])
        self.assertEqual(code, 1, out)
        self.assertIn(f".githooks/pre-commit:2: {NOQA}", out)

    def test_a_side_input_that_fails_keeps_the_listed_files_faults(self):
        self.world("docs/sprints/decisions.md#title")
        self.track("gate/known-issues.json", "{")  # not JSON: a side input that fails
        self.track(".githooks/pre-commit", "#!/usr/bin/env python3\nimport os  # noqa\n")
        code, out = self.run_row(["tools/allowlists/purity.json", ".githooks/pre-commit"])
        failed = [line for line in out.splitlines() if line.startswith("failed: ")]
        self.assertEqual(code, 1, out)
        self.assertEqual(
            failed,
            [
                "failed: suppressions: Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
                f"failed: suppressions: .githooks/pre-commit:2: {NOQA}",
            ],
            out,
        )

    # BK-S6: a generated file is listed by path in tools/allowlists/generated.json (rule `generated`).

    def generated_list(self, *targets: str) -> None:
        entries = ", ".join(
            f'{{"target": "{t}", "kind": "PERMANENT", "reason": "written by the fixture generator",'
            ' "ref": "docs/sprints/decisions.md#title", "sprint": "S1.1c-2"}'
            for t in targets
        )
        self.track(
            "tools/allowlists/generated.json",
            f'{{"schema": 1, "note": "n", "rule": "generated", "entries": [{entries}]}}\n',
        )

    def test_a_generated_marker_not_listed_is_red(self):
        self.world("docs/sprints/decisions.md#title")
        self.generated_list()
        self.track("internal/x/gen.go", GENERATED_HEADER + "\npackage x\n")
        code, out = self.run_row(["tools/allowlists/purity.json", "internal/x/gen.go"])
        self.assertEqual(code, 1, out)
        self.assertIn("internal/x/gen.go:1: a generated file is listed by path in tools/allowlists/generated.json", out)

    def test_a_listed_generated_file_is_green(self):
        self.world("docs/sprints/decisions.md#title")
        self.generated_list("internal/x/gen.go")
        self.track("internal/x/gen.go", GENERATED_HEADER + "\npackage x\n")
        code, out = self.run_row(["tools/allowlists/purity.json", "internal/x/gen.go"])
        self.assertEqual(code, 0, out)

    def test_a_listed_file_without_the_marker_is_a_stale_target(self):
        self.world("docs/sprints/decisions.md#title")
        self.generated_list("internal/x/gen.go")
        self.track("internal/x/gen.go", "package x\n\n" + GENERATED_HEADER)
        code, out = self.run_row(["tools/allowlists/purity.json", "internal/x/gen.go"])
        self.assertEqual(code, 1, out)
        self.assertIn(
            "tools/allowlists/generated.json: internal/x/gen.go: carries no generated marker before its package"
            " clause (a stale target)",
            out,
        )

    def test_a_listed_file_that_does_not_exist_is_a_stale_target(self):
        self.world("docs/sprints/decisions.md#title")
        self.generated_list("internal/x/gone.go")
        code, out = self.run_row(["tools/allowlists/purity.json"])
        self.assertEqual(code, 1, out)
        self.assertIn("tools/allowlists/generated.json: internal/x/gone.go: does not exist (a stale target)", out)


if __name__ == "__main__":
    unittest.main()


# S1.1c-5 design §4, §8 2.1-2.7: only //nolint waives. The census (gate/runner/testdata/directives.json, design §4.1)
# is the data: every comment directive a linter golangci-lint v2.15.0 runs (or gosec, refused anyway, decision 7)
# honours, with its source reference and its confirmation by the pinned binary.
CENSUS = os.path.join(support.REPO, "gate", "runner", "testdata", "directives.json")
FOREIGN_FAULT = (
    "{where}: {linter} honours this form; the only waiver is //nolint:{linter} // PERMANENT|TEMP … (decision 7)"
)
LINE_FAULT = "{where}: a //line directive moves findings between files; not allowed (S1.1c-5)"
STANDALONE = (
    "{where}: //nolint stands alone on its line: it would waive the whole next declaration; put it at the end of the"
    " line it waives"
)
EXACTLY = re.compile(
    r"^p\.go:3: write //nolint:(<linter>|\S*) exactly \(two slashes, no space, lower case, one list\)$"
)
# The policy's example the twins use (design §4.2), verbatim.
ACCEPTED_LLL = (
    '//nolint:lll // PERMANENT reason="a URL that cannot wrap" ref=docs/architecture/adr/'
    "ADR-0015-build-enforcement-hostile-gate.md#2-architecture-constraints-as-analyzers-with-teeth-reference-a-in-go"
    " sprint=S1.1c-5"
)
# Judged elsewhere: //nolint forms by go_directives (Nolint below), a generated marker by go_generated.
NOT_FOREIGN = frozenset({"nolint", "nolintlint", "exclusions"})


def census() -> list[dict[str, object]]:
    with open(CENSUS, encoding="utf-8") as f:
        return json.load(f)


def placed(form: str, placement: str) -> tuple[str, int]:
    """A Go file holding the form in its census placement, and the form's line."""
    match placement:
        case "trailing":
            return f"package p\n\nvar _ = 1 {form}\n", 3
        case "file":
            return f"{form}\npackage p\n", 1
        case "residue":
            return f"package p\n\n{form}generate echo x\n", 3
        case _:  # above, region
            return f"package p\n\n{form}\nvar _ = 1\n", 3


class Census(unittest.TestCase):
    def test_every_enabled_linter_has_a_row(self):
        # Design §4.1 step 3: at least one row per enabled linter, "none in source" included; golangci.LINTERS is
        # lane A's, so the list is the design's §2.2 literal here.
        linters = {"errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive"}
        self.assertLessEqual(linters, {item["linter"] for item in census()})
        for item in census():
            with self.subTest(item=item):
                self.assertEqual(sorted(item), ["confirmed", "form", "honoured", "linter", "placement", "source"])
                self.assertIs(type(item["honoured"]), bool)


class Foreign(unittest.TestCase):
    def test_each_honoured_census_form_is_refused(self):
        rows = [r for r in census() if r["honoured"] and r["linter"] not in NOT_FOREIGN]
        self.assertGreater(len(rows), 10)  # witness: the census holds the forms
        for item in rows:
            form, linter = str(item["form"]), str(item["linter"])
            with self.subTest(form=form, placement=item["placement"]):
                source, line = placed(form, str(item["placement"]))
                directives, faults = suppressions.go_foreign("p.go", source)
                where = f"p.go:{line}"
                if linter == "line":
                    self.assertEqual(faults, [LINE_FAULT.format(where=where)])
                    continue
                self.assertEqual(faults, [FOREIGN_FAULT.format(where=where, linter=linter)])
                self.assertEqual(len(directives), 1, directives)
                d = directives[0]
                self.assertEqual(
                    (d.path, d.line, d.linters, d.kind, d.valid), ("p.go", line, (linter,), "FOREIGN", False)
                )

    def test_each_unhonoured_form_is_not_a_fault(self):
        # A form one linter ignores and another honours (`// #nosec`: errcheck no, gosec yes) is refused as honoured.
        honoured = {r["form"] for r in census() if r["honoured"]}
        rows = [r for r in census() if not r["honoured"] and r["form"] not in honoured]
        self.assertGreater(len(rows), 5)
        for item in rows:
            with self.subTest(form=item["form"], placement=item["placement"]):
                source, _ = placed(str(item["form"]), str(item["placement"]))
                self.assertEqual(suppressions.go_foreign("p.go", source), ([], []))

    def test_the_prefilter_finds_every_honoured_form(self):
        for item in census():
            if item["honoured"]:
                with self.subTest(form=item["form"]):
                    self.assertTrue(any(p in str(item["form"]) for p in suppressions.PREFILTER), item["form"])


class LineDirective(unittest.TestCase):
    def test_a_line_directive_is_refused(self):
        for form in ("//line x.go:1", "/*line x.go:1*/"):
            with self.subTest(form=form):
                _, faults = suppressions.go_foreign("p.go", f"package p\n\n{form}\nvar _ = 1\n")
                self.assertEqual(faults, [LINE_FAULT.format(where="p.go:3")])

    def test_line_in_prose_is_not_a_directive(self):
        # The compiler's forms only: "//line " and "/*line " at the comment's start.
        for form in ("// line x.go:1", "// the line x.go:1", "//lines x.go:1"):
            with self.subTest(form=form):
                self.assertEqual(suppressions.go_foreign("p.go", f"package p\n\n{form}\nvar _ = 1\n"), ([], []))


def s1_1c_5_ctx() -> allowlists.Ctx:
    """The accepted example's context: S1.1c-5 printed after S1.1c, its ADR-0015 ref resolving."""
    order = sprintid.Order((*ORDER.rows[:4], sprintid.Row("S1.1c-5", False), ORDER.rows[4]))
    ref = ACCEPTED_LLL.split("ref=")[1].split()[0]
    return allowlists.Ctx(
        today=TODAY,
        tier="T2",
        order=order,
        current=sprintid.parse("S1.1c-5"),
        issues=frozenset(),
        resolve=lambda r: "" if r == ref else f"{r} does not resolve",
    )


class Nolint(unittest.TestCase):
    def test_a_standalone_nolint_is_refused(self):
        waiver = f"//nolint:lll // {META}"
        for name, source, line in (
            ("above a func", f"package p\n\n{waiver}\nfunc F() {{}}\n", 3),
            ("above the package clause", f"{waiver}\npackage p\n", 1),
        ):
            with self.subTest(name):
                _, faults = suppressions.go_directives("p.go", source)
                self.assertIn(STANDALONE.format(where=f"p.go:{line}"), faults)
        with self.subTest("its twin trails its line"):
            directives, faults = suppressions.go_directives("p.go", go_line(waiver))
            self.assertEqual((len(directives), faults), (1, []))

    def assertRefusedWith(self, comment: str, text: str) -> None:
        _, faults = suppressions.go_directives("p.go", go_line(comment))
        self.assertTrue(any(f.startswith("p.go:3: ") and f.endswith(text) for f in faults), faults)

    def test_nolint_all_is_refused(self):
        self.assertRefusedWith(f"//nolint:all // {META}", "write the linter: //nolint:all waives every linter")

    def test_nolint_typecheck_is_refused(self):
        self.assertRefusedWith(f"//nolint:typecheck // {META}", "typecheck cannot be waived")

    def test_a_nolint_variant_golangci_honours_is_refused(self):
        # M-9's seven honoured spellings (re-confirmed in the census); the first six with full metadata, so the
        # spelling is the only fault; the seventh is canonical and red through the metadata rule alone.
        for spelling in (
            "// nolint:lll",
            "//nolint: lll",
            "//nolint:LLL",
            "//  nolint:lll",
            "//nolint:lll,",
            "//nolint",
        ):
            with self.subTest(spelling=spelling):
                _, faults = suppressions.go_directives("p.go", go_line(f"{spelling} // {META}"))
                self.assertEqual(len(faults), 1, faults)
                self.assertRegex(faults[0], EXACTLY)
        with self.subTest(spelling="//nolint:lll // x"):
            _, faults = suppressions.go_directives("p.go", go_line("//nolint:lll // x"))
            self.assertEqual(faults, [f"p.go:3: {METADATA}"])

    def test_an_accepted_lll_waiver_passes(self):
        # Red until the row's enabled set is the config's (design §4.2): today ENABLED is empty and lll "not enabled".
        from gate.runner import golangci  # lane A's reader

        with open(os.path.join(support.REPO, ".golangci.toml"), encoding="utf-8") as f:
            enabled = golangci.enabled(golangci.load(f.read()))
        directives, faults = suppressions.go_directives("p.go", go_line(ACCEPTED_LLL))
        self.assertEqual((len(directives), faults), (1, []))
        self.assertEqual(suppressions.check(directives, s1_1c_5_ctx(), enabled), [])


class Enabled(unittest.TestCase):
    def test_enabled_equals_the_config_both_ways(self):
        from gate.runner import golangci  # lane A's reader

        with open(os.path.join(support.REPO, ".golangci.toml"), encoding="utf-8") as f:
            enabled = golangci.enabled(golangci.load(f.read()))
        self.assertEqual(enabled, frozenset(golangci.LINTERS))
        self.assertEqual(frozenset(golangci.LINTERS), enabled)
        self.assertFalse(hasattr(suppressions, "ENABLED"), "ENABLED is removed as a constant (design §4.2)")

    def test_a_linter_not_in_the_config_is_not_enabled_text(self):
        directives, _ = suppressions.go_directives("p.go", go_line(f"//nolint:gosec // {META}"))
        got = suppressions.check(directives, fixture_ctx(), frozenset({"lll"}))
        self.assertEqual(got, ["p.go:3: //nolint:gosec: gosec is not enabled in .golangci.toml"])
