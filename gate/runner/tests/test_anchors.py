"""gate/runner/anchors.py (S1.1c-2 design §2.1, §2.8): the heading anchors moved out of rows/doclinks.py, and a ref.

`slug`, `prose` and `anchors` move verbatim, so their cases are doclinks' current behaviour. `split_ref` and
`resolves` are new: a ref is `<repository path>#<heading anchor>`; every text of §2.8's split_ref and resolves rows
is asserted verbatim. `read(path)` is injected: it returns a tracked Markdown file's text, or None (DA-L5: an
untracked local doc never satisfies a ref).
"""

import unittest

from gate.runner import anchors

DOC = """# Title

## A heading with `code`, and punctuation!

```
## Not a heading (fenced)
```

## Dup

## Dup

<a id="custom"></a>
"""


def reader(files: dict[str, str]):
    """A `read` over the given tracked Markdown files: None for anything else."""
    return lambda path: files.get(path)


class Moved(unittest.TestCase):
    def test_slug_is_githubs_anchor(self):
        self.assertEqual(anchors.slug("A heading with `code`, and punctuation!"), "a-heading-with-code-and-punctuation")
        self.assertEqual(
            anchors.slug("Owner decisions, 2026-11-04 — S3.1-2's plan check"),
            "owner-decisions-2026-11-04--s31-2s-plan-check",
        )
        self.assertEqual(anchors.slug("[a link](x.md) and _emphasis_"), "a-link-and-emphasis")

    def test_prose_skips_fenced_blocks_and_numbers_lines_from_one(self):
        got = list(anchors.prose(["a", "```", "b", "```", "c"]))
        self.assertEqual(got, [(1, "a"), (5, "c")])

    def test_prose_closes_a_fence_only_with_its_own_marker(self):
        got = list(anchors.prose(["````", "```", "x", "````", "y"]))
        self.assertEqual(got, [(5, "y")])

    def test_anchors_number_a_repeat_skip_fences_and_read_html_ids(self):
        got = anchors.anchors(DOC)
        self.assertEqual(got, {"title", "a-heading-with-code-and-punctuation", "dup", "dup-1", "custom"})

    def test_the_patterns_moved_with_them(self):
        self.assertTrue(anchors.FENCE.match("```go"))
        self.assertTrue(anchors.HEADING.match("## x"))
        self.assertEqual(anchors.HTML_ANCHOR.findall('<a id="custom"></a>'), ["custom"])

    def test_doclinks_imports_them(self):
        from gate.runner.rows import doclinks

        self.assertIs(doclinks.slug, anchors.slug)
        self.assertIs(doclinks.prose, anchors.prose)
        self.assertIs(doclinks.anchors, anchors.anchors)


class SplitRef(unittest.TestCase):
    def test_a_path_and_an_anchor(self):
        self.assertEqual(anchors.split_ref("docs/a.md#the-heading"), ("docs/a.md", "the-heading"))

    def test_no_anchor_splits_to_an_empty_anchor(self):
        # design ambiguity: §2.1 gives split_ref -> tuple[str, str] and puts "has no anchor" in resolves, so a ref
        # without "#" (or with an empty fragment) splits to (path, "") rather than raising.
        self.assertEqual(anchors.split_ref("docs/a.md"), ("docs/a.md", ""))
        self.assertEqual(anchors.split_ref("docs/a.md#"), ("docs/a.md", ""))

    def assertRefused(self, ref: str, text: str) -> None:
        with self.assertRaises(ValueError) as caught:
            anchors.split_ref(ref)
        self.assertEqual(str(caught.exception), text)

    def test_empty(self):
        self.assertRefused("", "ref is empty")

    def test_absolute(self):
        self.assertRefused("/docs/a.md#x", "ref must be a repository path, not absolute")

    def test_climbing_out(self):
        self.assertRefused("../a.md#x", "ref must not climb out with ..")
        self.assertRefused("docs/../../a.md#x", "ref must not climb out with ..")

    def test_more_than_one_hash(self):
        self.assertRefused("docs/a.md#x#y", "ref has more than one #")


class Resolves(unittest.TestCase):
    READ = staticmethod(reader({"docs/a.md": DOC}))

    def test_a_heading_anchor_resolves(self):
        self.assertEqual(anchors.resolves("docs/a.md#a-heading-with-code-and-punctuation", self.READ), "")
        self.assertEqual(anchors.resolves("docs/a.md#dup-1", self.READ), "")

    def test_no_anchor(self):
        # design ambiguity: "<path>" is substituted, "<heading-anchor>" is written literally (it is the advice).
        want = "ref has no anchor: write docs/a.md#<heading-anchor>"
        self.assertEqual(anchors.resolves("docs/a.md", self.READ), want)
        self.assertEqual(anchors.resolves("docs/a.md#", self.READ), want)

    def test_an_untracked_file_reads_none_and_is_not_a_tracked_markdown_file(self):
        # `read` returns None for an untracked file (DA-L5): the ref is red even if the file is on the disk.
        self.assertEqual(
            anchors.resolves("docs/untracked.md#title", self.READ), "docs/untracked.md is not a tracked Markdown file"
        )

    def test_a_missing_heading(self):
        self.assertEqual(anchors.resolves("docs/a.md#nowhere", self.READ), "docs/a.md has no heading #nowhere")

    def test_a_fenced_heading_is_not_a_heading(self):
        got = anchors.resolves("docs/a.md#not-a-heading-fenced", self.READ)
        self.assertEqual(got, "docs/a.md has no heading #not-a-heading-fenced")

    def test_split_refs_faults_are_the_why(self):
        # design ambiguity: resolves returns "" or the why and never raises, so split_ref's ValueError texts come
        # back as the why, verbatim.
        for ref, text in (
            ("", "ref is empty"),
            ("/docs/a.md#title", "ref must be a repository path, not absolute"),
            ("../docs/a.md#title", "ref must not climb out with .."),
            ("docs/a.md#title#x", "ref has more than one #"),
        ):
            with self.subTest(ref=ref):
                self.assertEqual(anchors.resolves(ref, self.READ), text)


if __name__ == "__main__":
    unittest.main()
