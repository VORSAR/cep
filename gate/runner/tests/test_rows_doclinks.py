"""The doclinks row (owner, 2026-11-06): every relative link and #anchor in the tracked Markdown resolves.

A temporary repository holds a few Markdown files. Links that resolve (to a tracked file or directory, to a heading's
anchor in another file or the same one by GitHub's slug rules with a repeated heading numbered, to an HTML anchor)
cover each file. A template under docs/sprints/templates/ resolves from docs/sprints/, where its copies land, and its
<…> placeholders are skipped. URLs and links inside code are never read. A link to a missing or untracked file, or
to an anchor no heading makes, fails the row naming file:line, the target and why.
"""

import os
import sys

from gate.runner.tests import support

ROWS = os.path.join(support.GATE, "runner", "rows")
SCOPE = ("git", ":(glob)**/*.md")
TARGET = """# Target

## A heading with `code`, and punctuation!

### Owner decisions, 2026-11-04 — S3.1-2's plan check

## Dup

## Dup

<a id="custom"></a>
"""
SOURCE = """# Source

[file](b.md) · [directory](sub/) · [heading](b.md#a-heading-with-code-and-punctuation)
[decision](b.md#owner-decisions-2026-11-04--s31-2s-plan-check) · [repeat](b.md#dup-1) · [html](b.md#custom)
[here](#local-heading) · [url](https://example.com/nowhere.md) · [mail](mailto:x@example.com)
`[in code](nowhere.md)` and [a link with a title](b.md "title")

```
[fenced](nowhere.md)
```

[ref]: b.md

## Local heading
"""


class Doclinks(support.GateTest):
    def doclinks(self):
        command = (sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "doclinks.py"), "{files}")
        self.run_rows([self.row("doclinks", command=command, expected_scope=SCOPE)])
        return self.reason("doclinks")

    def test_links_that_resolve_cover_every_file(self):
        self.track("b.md", TARGET)
        self.track("sub/c.md", "# C\n")
        self.track("a.md", SOURCE)
        self.assertEqual(self.doclinks(), "covered 3")

    def test_a_missing_file_is_named(self):
        self.track("b.md", TARGET)
        self.track("a.md", "# A\n\n[gone](gone.md)\n")
        self.assertEqual(self.doclinks(), "exit 1: a.md:3: gone.md (missing file)")

    def test_an_untracked_file_is_missing(self):
        self.write("loose.md", "# Loose\n")
        self.track("a.md", "[loose](loose.md)\n")
        self.assertEqual(self.doclinks(), "exit 1: a.md:1: loose.md (missing file)")

    def test_a_missing_anchor_is_named(self):
        self.track("b.md", TARGET)
        self.track("a.md", "[two headings make dup and dup-1, never dup-2](b.md#dup-2)\n")
        self.assertEqual(self.doclinks(), "exit 1: a.md:1: b.md#dup-2 (missing anchor)")

    def test_a_missing_local_anchor_is_named(self):
        self.track("a.md", "# A\n\n[here](#elsewhere)\n")
        self.assertEqual(self.doclinks(), "exit 1: a.md:3: #elsewhere (missing anchor)")

    def test_a_broken_reference_definition_is_named(self):
        self.track("a.md", "# A\n\n[ref]: ../outside.md\n")
        self.assertEqual(self.doclinks(), "exit 1: a.md:3: ../outside.md (missing file)")

    def test_a_template_resolves_from_the_sprints_directory(self):
        self.track("docs/sprints/00-sprint-model.md", "# Model\n\n## 6. The full loop\n")
        self.track(
            "docs/sprints/templates/plan-check.md",
            "[loop](00-sprint-model.md#6-the-full-loop) · [record](s<N>-record.md) · [card](sprint-plan.md#<anchor>)\n",
        )
        self.assertEqual(self.doclinks(), "covered 2")
