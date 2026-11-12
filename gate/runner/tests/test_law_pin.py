"""The law manifest's pin (S1.1c-1 design §2.6; plan check §2 S1): every path ADR-0015 §1.3 names that git tracks is
listed in gate/law.json, and the one it names that does not exist (tools/analyzers/) is still untracked. Reads the
real files, so a law path dropped from the manifest is red at T2."""

import os
import unittest

from gate.runner import law
from gate.runner.tests import support

# ADR-0015 §1.3's list, as tracked today (the paths with a trailing "/" cover every file under them).
ADR_0015_1_3 = (
    "gate/registry.json",
    "gate/run.py",
    "gate/runner/",
    "gate/baseline/regen.py",
    "gate/law.json",
    ".github/workflows/",
    "tools.json",
    "tools/deps-allowlist.json",
    "tools/importcheck/rules.json",
    "api/exports.golden",
    "api/clock.golden",
    "gate/current-sprint",
    "docs/language/grammar.lark",
    "docs/language/grammar.ebnf",
    "docs/language/SPEC.md",
    "docs/language/SEMANTICS.md",
    "docs/language/errors.md",
    "docs/architecture/ARCHITECTURE.md",
    "docs/architecture/adr/",
    # As-built law the ADR names elsewhere (owner, 2026-11-11, the fix round): the analyzer and its purity exceptions in
    # code (§2 as annotated), the workflow generator, the planted proofs the min_wants count pins, ruff.toml (§1.4), and
    # the two module files (§1.4; ADR-0009).
    "tools/archcheck/",
    "gate/ci/",
    "gate/proofs/drivers/",
    "ruff.toml",
    "go.mod",
    "gate/proofs/go.mod",
    # §1.3 names gate/ whole: the catch-alls, beside the specific entries above (owner, 2026-11-12, S1.1c-2's plan
    # check, carried): the rest of the gate, the tools, the hooks, the Makefile, the language corpus; the one allowlist
    # grammar's files (§3.10) and the known-issues data (decision 13).
    "gate/",
    "tools/",
    ".githooks/",
    "Makefile",
    "docs/language/examples/",
    "docs/language/invalid/",
    "tools/allowlists/",
    "gate/known-issues.json",
)
# Named by §1.3, not built: the go/analysis analyzers (the as-built analyzer is tools/archcheck/, owner 2026-11-04).
NOT_YET = ("tools/analyzers/",)


class LawPin(unittest.TestCase):
    def test_every_tracked_path_of_adr_0015_1_3_is_listed(self):
        with open(os.path.join(support.REPO, "gate", "law.json"), encoding="utf-8") as f:
            listed = {entry.path for entry in law.load_manifest(f.read()).paths}
        missing = [path for path in ADR_0015_1_3 if path not in listed]
        self.assertEqual(missing, [], f"ADR-0015 §1.3 names {missing} and gate/law.json does not list them")

    def test_the_unbuilt_path_is_still_untracked(self):
        for path in NOT_YET:
            with self.subTest(path):
                code, out, _ = support.run_tool(["git", "ls-files", "-z", "--", path], support.REPO)
                self.assertEqual(code, 0)
                self.assertEqual(out, b"", f"{path} is tracked now: list it in gate/law.json and here")


if __name__ == "__main__":
    unittest.main()
