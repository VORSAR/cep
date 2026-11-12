"""gate/runner/suppressions.py (S1.1c-2 design §2.3, §2.7, §2.8; measured tables §0.2, §0.3) and its row (§2.4).

`go_comments` has one case per scanner state. §0.3's golangci-lint table and §0.2's ruff table are held here as data
and every form's classification is asserted, so a recogniser that drifts from what the pinned tools do is red. Every
Go-directive and Python-marker text of §2.8 is asserted verbatim. The policy's two examples (§2.7) are checked with
§2.7's context and the linter injected as enabled; they are fixtures here until the coordinator writes them into
DEVELOPMENT.md when lane A lands, after which this test should read DEVELOPMENT.md's fenced examples instead.
"""

import contextlib
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
WRITE = re.compile(r"^p\.go:(\d+): write //nolint:(<linter>|\S*) \(two slashes, no space\)$")
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
    (f"//nolint:all // {META}", True, True),
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

    def test_no_linter_is_enabled_until_s11c4(self):
        got = suppressions.check(self.directives(f"//nolint:errcheck // {META}"), fixture_ctx(), frozenset())
        self.assertEqual(got, ["p.go:3: //nolint:errcheck: errcheck is not enabled (none is until S1.1c-4)"])
        self.assertEqual(suppressions.ENABLED, frozenset())

    def test_a_valid_directive_with_its_linter_enabled_is_green(self):
        got = suppressions.check(
            self.directives(f"//nolint:errcheck // {META}"), fixture_ctx(), frozenset({"errcheck"})
        )
        self.assertEqual(got, [])

    def test_each_linter_not_enabled_is_named(self):
        got = suppressions.check(
            self.directives(f"//nolint:errcheck,gosec // {META}"), fixture_ctx(), frozenset({"errcheck"})
        )
        self.assertEqual(got, ["p.go:3: //nolint:gosec: gosec is not enabled (none is until S1.1c-4)"])

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


class RuffPinRefuses(support.GateTest):
    """BK-B2: ruff.toml settings the pin does not read are red (the pin reads only [lint]'s per-file tables)."""

    def pin(self, ruff_toml: str) -> list[str]:
        self.write("ruff.toml", ruff_toml)
        self.write("docs/base.toml", "")
        with contextlib.chdir(self.repo):
            return row.ruff_pin({})

    def test_extend_is_refused_even_naming_an_empty_file(self):
        self.assertEqual(
            self.pin('extend = "docs/base.toml"\n[lint]\nselect = ["E"]\n'),
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
                    self.pin(f'{key} = {value}\n[lint]\nselect = ["E"]\n'),
                    [f"ruff.toml sets top-level {key}: write per-file waivers under [lint] (ruff deprecates it)"],
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
        self.track("ruff.toml", '[lint]\nselect = ["E"]\n')
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
