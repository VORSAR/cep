"""The instrument pin over fixtures (S1.1c-2 design §4.2, §5 lane D): each of the pin's six tests is red on its
planted fixture and green on the correct one.

The fixtures under gate/runner/testdata/teeth/ are a small manifest with `instruments` (three row instruments for
the registry fixture's three rows, the unit row exempt through teeth.json, and one Go-test instrument), the registry
fixture (three real rows), the teeth list, and world.json: the tracked files and what python_cases / go_cases count
per proof file. Each plant edits one fact of the correct fixture. The pin over the real files is lane D's to add once
gate/law.json holds `instruments` (§4.2: counts are measured, never hard-coded).

design ambiguity: §4.2 says the pin test does the I/O and teeth.py is pure, but names no pure entry point for the six
checks; these tests call `teeth.pin(manifest, rows, tracked, counted, teeth_targets) -> list[str]`, `counted` mapping
a proof path to the (cases, faults) pair python_cases or go_cases returns, `teeth_targets` the targets of
tools/allowlists/teeth.json.
design ambiguity: the pin's fault texts are not written in the design; each red asserts a fault naming the planted
instrument or row id.
"""

import copy
import json
import os
import subprocess
import unittest

from gate.runner import allowlists, law, registry, teeth

TESTDATA = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "testdata", "teeth")


def read(name: str) -> dict:
    with open(os.path.join(TESTDATA, name), encoding="utf-8") as f:
        return json.load(f)


class Pin(unittest.TestCase):
    def setUp(self):
        self.law = read("law.json")
        self.registry = read("registry.json")
        self.world = read("world.json")
        with open(os.path.join(TESTDATA, "teeth.json"), encoding="utf-8") as f:
            self.teeth = frozenset(e.target for e in allowlists.load(f.read(), "teeth").entries)

    def instrument(self, id: str) -> dict:
        return next(i for i in self.law["instruments"] if i["id"] == id)

    def row(self, id: str) -> dict:
        return next(r for r in self.registry["rows"] if r["id"] == id)

    def pin(self) -> list[str]:
        manifest = law.load_manifest(json.dumps(self.law))
        rows = registry.loads(json.dumps(self.registry))
        counted = {path: (c["cases"], c["faults"]) for path, c in self.world["counted"].items()}
        return teeth.pin(manifest, rows, frozenset(self.world["tracked"]), counted, self.teeth)

    def assertNames(self, id: str) -> None:
        faults = self.pin()
        self.assertTrue(faults, "the plant was not seen")
        self.assertTrue(any(id in f for f in faults), faults)

    def test_the_correct_fixture_is_green(self):
        self.assertEqual(self.pin(), [])

    # 1. registry <-> row instruments, both ways
    def test_a_row_with_no_instrument_is_red(self):
        vet = copy.deepcopy(self.row("docsync"))
        vet["id"] = "vetfix"
        self.registry["rows"].append(vet)
        self.assertNames("vetfix")

    def test_an_instrument_for_a_missing_row_is_red(self):
        self.registry["rows"] = [r for r in self.registry["rows"] if r["id"] != "lawdelta"]
        self.assertNames("lawdelta")

    def test_two_row_instruments_for_one_row_are_red(self):
        twin = copy.deepcopy(self.instrument("docsync"))
        twin["id"] = "docsync-twin"
        self.law["instruments"].append(twin)
        self.assertNames("docsync")

    # 2. each row pinned: blocking, tiers, lands_in, min_wants, expected_scope
    def test_a_row_made_non_blocking_is_red(self):
        self.row("docsync")["blocking"] = False
        self.assertNames("docsync")

    def test_narrowed_tiers_are_red(self):
        self.row("docsync")["tiers"] = ["T2", "T3"]
        self.assertNames("docsync")

    def test_a_lowered_min_wants_is_red(self):
        self.row("lawdelta")["min_wants"] = 0
        self.assertNames("lawdelta")

    def test_a_narrowed_expected_scope_is_red(self):
        self.row("docsync")["expected_scope"] = ["go", "./internal/..."]
        self.assertNames("docsync")

    def test_a_changed_lands_in_is_red(self):
        self.row("docsync")["lands_in"] = "S1.1c-3"
        self.assertNames("docsync")

    # 3. a go_test is listed by -cases
    def test_a_go_test_the_listing_lacks_is_red(self):
        self.instrument("docsync-d")["go_test"] = "internal/parser.TestGone"
        self.assertNames("docsync-d")

    # 4. code and proofs are tracked law paths
    def test_a_proof_path_outside_the_law_is_red(self):
        self.instrument("lawdelta")["proofs"] = [{"path": "scripts/test_outside.py", "cases": 3}]
        self.world["counted"]["scripts/test_outside.py"] = {"cases": ["a", "b", "c"], "faults": []}
        self.assertNames("lawdelta")

    def test_an_untracked_code_path_is_red(self):
        self.instrument("docsync")["code"].append("gate/runner/gone.py")
        self.assertNames("docsync")

    # 5. cases counted
    def test_cases_off_by_one_are_red(self):
        self.instrument("docsync")["proofs"][0]["cases"] = 3
        self.assertNames("docsync")

    def test_cases_one_short_are_red(self):
        self.instrument("lawdelta")["proofs"][0]["cases"] = 2
        self.assertNames("lawdelta")

    def test_a_go_proof_counts_only_its_test_and_subtests(self):
        # world.json lists TestUnrelated beside the named test: counting it would make 3.
        self.instrument("docsync-d")["proofs"][0]["cases"] = 3
        self.assertNames("docsync-d")

    def test_a_driver_fault_is_red(self):
        self.world["counted"]["gate/proofs/drivers/test_docsync_planted.py"]["faults"] = ["plants nothing red"]
        self.assertNames("docsync")

    # 6. exempt names a teeth.json entry
    def test_an_exempt_naming_no_teeth_entry_is_red(self):
        self.teeth = frozenset()
        self.assertNames("unit")


if __name__ == "__main__":
    unittest.main()


class RealTree(unittest.TestCase):
    """The pin over the real files (design §4.2; the coordinator, once gate/law.json's instruments were written from
    lane D1's measurement and lane D2's -cases): every registry row pinned, every proof counted, every case asserting a
    verdict. A deleted proof case, a demoted row or a lowered min_wants is red here at T2."""

    def test_the_real_tree_holds_its_teeth(self):
        repo = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
        with open(os.path.join(repo, "gate", "law.json"), encoding="utf-8") as f:
            manifest = law.load_manifest(f.read())
        rows = registry.load(os.path.join(repo, "gate", "registry.json"))
        listed = subprocess.run(["git", "ls-files", "-z"], cwd=repo, capture_output=True, check=True).stdout
        tracked = {p.decode() for p in listed.split(b"\0") if p}
        go_files = sorted({p.path for i in manifest.instruments for p in i.proofs if p.path.endswith("_test.go")})
        self.assertTrue(go_files, "no Go proof file is pinned")
        cases = subprocess.run(
            ["go", "run", "./tools/archcheck", "-cases", *go_files], cwd=repo, capture_output=True, text=True
        )
        self.assertEqual(cases.returncode, 0, cases.stderr)
        counted = dict(teeth.go_cases(cases.stdout))
        for inst in manifest.instruments:
            for proof in inst.proofs:
                if proof.path.endswith(".py"):
                    with open(os.path.join(repo, proof.path), encoding="utf-8") as f:
                        counted[proof.path] = teeth.python_cases(f.read(), proof.path)
        with open(os.path.join(repo, "tools", "allowlists", "teeth.json"), encoding="utf-8") as f:
            targets = {e["target"] for e in json.load(f)["entries"]}
        self.assertEqual(teeth.pin(manifest, rows, tracked, counted, targets), [])
        self.assertGreaterEqual(len(manifest.instruments), len(rows))
