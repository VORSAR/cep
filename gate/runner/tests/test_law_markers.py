"""The rule markers' pin, the gate's side (S1.1c-3 design §10 C11; DQ-S16): every marker form the law documents use is
one `law.MARKER_COMMENT` matches, so the LAW Δ block counts a marker-only change apart from a change to the text
(`law._unmarked`, `law._markers`). The Go fragment test is the one judge of which forms are valid; this side asserts
only coverage: a comment that looks like a marker (its body begins `cep` or is `text`, in any case, with or without
spaces) and that `MARKER_COMMENT` does not match is named "a marker the LAW Δ block would not count apart".

First over a synthetic fixture holding each form; then over SPEC.md, SEMANTICS.md and errors.md, which hold no
marker comment today, so that pass is vacuous until lane B's markers land (it then judges them).
"""

import os
import re
import unittest

from gate.runner import law
from gate.runner.tests import support

DOCUMENTS = ("SPEC.md", "SEMANTICS.md", "errors.md")
COMMENT = re.compile(r"<!--(.*?)-->", re.DOTALL)
NOT_APART = "a marker the LAW Δ block would not count apart"


def looks_like_a_marker(body: str) -> bool:
    bare = body.strip().casefold()
    return bare.startswith("cep") or bare == "text"


def unmatched(text: str) -> list[str]:
    """Each comment that looks like a marker but that `MARKER_COMMENT` does not match whole, named."""
    return [
        f"{m.group(0)}: {NOT_APART}"
        for m in COMMENT.finditer(text)
        if looks_like_a_marker(m.group(1)) and not law.MARKER_COMMENT.fullmatch(m.group(0))
    ]


class Fixture(unittest.TestCase):
    def test_the_forms_the_documents_use_are_counted_apart(self):
        for form in ("<!--cep:chain-->", "<!--cep-meta:expr-->", "<!--cep-invalid:expr-->", "<!--text-->"):
            with self.subTest(form=form):
                self.assertEqual(law.MARKER_COMMENT.findall(f"a rule {form} sentence"), [form])
                self.assertEqual(unmatched(f"a rule {form} sentence"), [])

    def test_a_near_miss_is_named(self):
        for form in ("<!-- cep:chain -->", "<!--CEP:chain-->", "<!-- text -->", "<!--Cep-meta:expr-->"):
            with self.subTest(form=form):
                self.assertEqual(law.MARKER_COMMENT.findall(f"a rule {form} sentence"), [])
                self.assertEqual(unmatched(f"a rule {form} sentence"), [f"{form}: {NOT_APART}"])

    def test_forms_only_the_go_side_judges_are_counted_apart(self):
        # DQ-S16: MARKER_COMMENT matches these, so LAW Δ counts them as markers; whether each is a valid marker is
        # the Go fragment test's verdict (an unknown kind, an empty name, a run-on word), never this file's.
        for form in ("<!--cep-frob:x-->", "<!--cep:-->", "<!--cepx-->"):
            with self.subTest(form=form):
                self.assertEqual(law.MARKER_COMMENT.findall(f"a rule {form} sentence"), [form])
                self.assertEqual(unmatched(f"a rule {form} sentence"), [])

    def test_a_comment_that_is_no_marker_is_left_alone(self):
        self.assertEqual(unmatched("prose <!-- an editor's note --> more prose"), [])


class Documents(unittest.TestCase):
    def test_every_marker_in_the_law_documents_is_counted_apart(self):
        # Vacuous on this branch: the three documents hold no comment yet; the fixture's near misses are the witness
        # that a missed form would be named.
        for name in DOCUMENTS:
            path = os.path.join(support.REPO, "docs", "language", name)
            with open(path, encoding="utf-8") as f:
                text = f.read()
            self.assertTrue(text, f"{path} is empty")  # the document was read
            with self.subTest(document=name):
                self.assertEqual(unmatched(text), [])


if __name__ == "__main__":
    unittest.main()
