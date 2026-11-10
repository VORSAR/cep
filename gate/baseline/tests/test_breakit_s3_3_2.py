"""QA break-it of S3.3-2 on the regen tool (design s3-3-2 §8.1, §8.3): each case is red on the build at 1f64dc3.

The harness is test_regen.py's: `main` on a temporary repository, the gate and Go stubbed through the three seams.
"""

import hashlib
import os
import unittest

from gate.baseline import regen
from gate.baseline.tests import test_regen as tr
from gate.baseline.tests.test_regen import F10, K1, R10, SHA_A, A, B, baseline, block, line, out, text


class BreakitParse(unittest.TestCase):
    def test_counts_that_are_not_integers_are_a_value_error(self):
        # parse promises a ValueError on a malformed line; a count that is an object raises TypeError, and a count
        # written as a string or a fraction is silently turned into an int.
        header = text(baseline()).split("\n")[0] + "\n"
        good = line(block(1, K1, A))
        cases = {
            "a count that is an object": good.replace('"late":0', '"late":{"n":1}'),
            "a count that is a string": good.replace('"late":0', '"late":"1"'),
            "a count that is a fraction": good.replace('"late":0', '"late":1.5'),
            "a count that is a bool": good.replace('"late":0', '"late":true'),
        }
        for name, bad in cases.items():
            with self.subTest(name), self.assertRaises(ValueError):
                regen.parse(header + bad)

    def test_a_key_written_twice_in_one_file_is_a_value_error(self):
        # Keys are unique within a file (equal keys take #2, #3, …); a file holding one key twice cannot be paired by
        # key, and classify keeps only the last of the two.
        with self.assertRaises(ValueError):
            regen.parse(text(baseline(block(1, K1, A), block(2, K1, B))))


class BreakitMain(unittest.TestCase):
    # test_regen.Main's harness, borrowed method by method so that its own tests do not run twice from here.
    setUp = tr.Main.setUp
    fake_fresh_run = tr.Main.fake_fresh_run
    store = tr.Main.store
    fresh_files = tr.Main.fresh_files
    snapshot = tr.Main.snapshot
    main = tr.Main.main

    def test_a_stored_block_sharing_its_key_is_never_dropped_silently(self):
        # The stored file holds K1 twice (block 1: A, block 2: B); the fresh run holds K1 once (B). The first stored
        # block is gone, yet no verdict names it and the tool writes the fresh file over it, exit 0.
        old = baseline(block(1, K1, A), block(2, K1, B), rev=self.rev)
        new = baseline(block(1, K1, B), rev=self.rev)
        self.store(old)
        self.fresh_files(new)
        before = self.snapshot()
        code, printed = self.main()
        self.assertNotEqual(code, 0, f"a stored block vanished without a verdict and the tool recorded:\n{printed}")
        self.assertEqual(self.snapshot(), before, "a refused run writes nothing")

    def test_a_file_only_in_the_old_run_whose_source_is_unchanged_is_refused(self):
        # §8.3: a key gone while the file's source_sha256 did not change is block-vanished, refused (a tool fault).
        # classify reads a file absent from the fresh run as "its source is gone" without looking: here the corpus
        # file is still there, byte for byte the one the stored header hashes, and the tool records block-removed
        # and deletes the stored baseline.
        corpus = os.path.join(self.repo, "docs", "language", "examples")
        os.makedirs(corpus, exist_ok=True)
        src = b"language cep/0.1;\n"
        with open(os.path.join(corpus, F10), "wb") as f:
            f.write(src)
        sha = hashlib.sha256(src).hexdigest()
        old = baseline(block(1, K1, A), sha=sha, rev=self.rev)
        self.store(old)
        self.fresh_files()  # the fresh run wrote nothing for F10
        before = self.snapshot()
        code, printed = self.main()
        self.assertNotEqual(code, 0, f"the stored baseline of a corpus file still there was removed:\n{printed}")
        self.assertNotIn("block-removed", printed)
        self.assertEqual(self.snapshot(), before, "a refused run writes nothing")

    def test_the_printed_accept_token_is_offered_only_when_it_can_record(self):
        # A value change under an unchanged SPEC rev can never be a spec-rev (the rev must be above the header's), yet
        # the tool prints "to accept it as a SPEC revision's change: ACCEPT <token>"; following it refuses again.
        old = baseline(block(1, K1, A, B), rev=self.rev, sha=SHA_A)
        new = baseline(block(1, K1, A, out("bob", "2026-01-01T10:01:00Z", scope="admin:all")), rev=self.rev)
        self.store(old)
        self.fresh_files(new)
        token = f"{F10}:{R10}:1"
        code, printed = self.main()
        self.assertEqual(code, 1, printed)
        if token in printed:
            code, again = self.main("--accept", token)
            self.assertEqual(code, 0, f"the tool offered ACCEPT {token}, and following it is refused again:\n{again}")


if __name__ == "__main__":
    unittest.main()
