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

import ast
import copy
import json
import os
import subprocess
import unittest

from gate.runner import allowlists, law, registry, teeth
from gate.runner.tests import support

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
        red_rows = {path: frozenset(c["red_rows"]) for path, c in self.world["counted"].items() if "red_rows" in c}
        return teeth.pin(manifest, rows, frozenset(self.world["tracked"]), counted, self.teeth, red_rows=red_rows)

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

    # 7. a row instrument's Python proofs hold a red case on its own row (BK-S7, S1.1c-2's fix round)
    def test_an_instrument_borrowing_another_rows_driver_is_red(self):
        self.instrument("docsync")["proofs"] = copy.deepcopy(self.instrument("lawdelta")["proofs"])
        self.assertEqual(self.pin(), ["teeth: instrument docsync: no Python proof holds a red case on row docsync"])

    def test_a_proof_red_only_on_another_row_is_red(self):
        self.world["counted"]["gate/proofs/drivers/test_docsync_planted.py"]["red_rows"] = ["lawdelta"]
        self.assertEqual(self.pin(), ["teeth: instrument docsync: no Python proof holds a red case on row docsync"])

    def test_a_proof_with_no_rows_counted_is_red(self):
        del self.world["counted"]["gate/proofs/drivers/test_docsync_planted.py"]["red_rows"]
        self.assertNames("docsync")


class UnnamedDrivers(unittest.TestCase):
    """CA-S1: the drivers no instrument names (the four test_parity_* today) are found, so the real-tree pin can hold
    them to the strength rule too."""

    def test_a_tracked_driver_no_instrument_names(self):
        manifest = law.load_manifest(json.dumps(read("law.json")))
        tracked = {
            *read("world.json")["tracked"],
            "gate/proofs/drivers/test_parity_hash.py",
            "gate/proofs/drivers/support.py",
            "gate/proofs/drivers/__init__.py",
            "gate/proofs/drivers/data/test_x.py",
            "gate/runner/tests/test_x.py",
        }
        self.assertEqual(teeth.unnamed_drivers(manifest, tracked), ["gate/proofs/drivers/test_parity_hash.py"])


REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))


def module_file(name: str) -> str | None:
    """The repository file a dotted `gate.*` name imports: the package's __init__.py, else the module's .py."""
    base = os.path.join(*name.split("."))
    for rel in (os.path.join(base, "__init__.py"), base + ".py"):
        if os.path.isfile(os.path.join(REPO, rel)):
            return rel.replace(os.sep, "/")
    return None


def imported(rel: str) -> set[str]:
    """The repository files one module imports by name (every import statement, at any depth). `from P import n`
    is P.n when that is a module, else P's own file; a parent package's __init__.py is included only when the
    package itself is named (S1.1c-5 design §2.5's rule, "__init__.py files included where imported")."""
    with open(os.path.join(REPO, rel), "rb") as f:
        tree = ast.parse(f.read(), rel)
    package = rel.removesuffix(".py").replace("/", ".").removesuffix(".__init__")
    if not rel.endswith("__init__.py"):
        package = package.rpartition(".")[0]
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            names.update(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom):
            base = node.module or ""
            if node.level:
                parent = package.split(".")[: len(package.split(".")) - node.level + 1]
                base = ".".join([*parent, *([base] if base else [])])
            for alias in node.names:
                sub = f"{base}.{alias.name}"
                names.add(sub if module_file(sub) else base)
    return {f for name in names if name == "gate" or name.startswith("gate.") if (f := module_file(name))}


def closure(root: str) -> set[str]:
    seen, todo = set(), [root]
    while todo:
        rel = todo.pop()
        if rel not in seen:
            seen.add(rel)
            todo.extend(imported(rel) - seen)
    return seen


class CodeClosure(unittest.TestCase):
    """S1.1c-5 design §2.5 (DA-S5, DQ-L3), §8 row L.1: a row instrument's Python `code` is its row module plus the
    transitive closure of its `gate.*` imports, compared both ways. Held for every instrument whose row runs a Python
    module (owner, S1.1c-7 Q10, survey G2-24).

    Who is judged: every instrument NOT_JUDGED does not name. A judged instrument is keyed by its row, its row's
    command in gate/registry.json names a .py module, and the .py files of its `code` are that module's closure. So an
    instrument leaves the judged set only through an edit to NOT_JUDGED: a row whose command stops naming its .py, or
    an instrument moved under another row, is red here by its own id.

    What the witness sees: every instrument is judged or named; each id NOT_JUDGED names is exactly one instrument, of
    the kind it is named under, so none that the rule could judge is named.

    What it cannot see: a named instrument's `code` list is not compared at all, a Go-test instrument's no more than a
    `go run` tool's, so a stranger file there passes; in a judged list only the .py files are compared, never config
    or Go files; the closure follows imports written by name, from the first .py argument of the row's command; two
    instruments for one row are teeth.pin's to refuse (RealTree below), not this test's."""

    # The instruments the import rule cannot judge, each named by id under its kind and the kind's reason (survey
    # G2-24: list the known gaps by name).
    NOT_JUDGED = {
        "go_test": (
            "a Go-test instrument, keyed by its go_test under another row: its code is Go test files and their data",
            ("docsync-b-catalog", "docsync-b-keywords", "docsync-c", "docsync-d", "fragments", "nosleep"),
        ),
        "no Python module": (
            "its row's command runs no .py module (a `go run` tool): there are no imports to follow",
            ("apipartition", "archcheck", "clockgolden", "testhygiene"),
        ),
    }

    @staticmethod
    def kind(instrument: dict, commands: dict[str, list[str]]) -> str:
        """The kind of instrument the import rule cannot judge that this one is, or "judgeable" for one it can."""
        if "go_test" in instrument and instrument["id"] != instrument["row"]:
            return "go_test"
        if instrument["row"] in commands and not any(a.endswith(".py") for a in commands[instrument["row"]]):
            return "no Python module"
        return "judgeable"

    def test_each_instruments_code_is_its_import_closure(self):
        with open(os.path.join(REPO, "gate", "law.json"), encoding="utf-8") as f:
            instruments = json.load(f)["instruments"]
        with open(os.path.join(REPO, "gate", "registry.json"), encoding="utf-8") as f:
            commands = {row["id"]: row["command"] for row in json.load(f)["rows"]}
        with self.subTest("the smells instrument is staged"):  # §8 counts: the smells instrument is new
            self.assertIn("smells", [i["id"] for i in instruments])
        listed = [(id, kind) for kind, (_, ids) in self.NOT_JUDGED.items() for id in ids]
        named = dict(listed)
        judged, found = [], []
        for instrument in instruments:
            if instrument["id"] in named:  # the one way out of the judged set
                found.append((instrument["id"], self.kind(instrument, commands)))
                continue
            judged.append(instrument["id"])
            with self.subTest(instrument=instrument["id"]):
                self.assertEqual(
                    instrument["id"],
                    instrument["row"],
                    "a row instrument is keyed by its row (one the import rule cannot judge is named in NOT_JUDGED)",
                )
                roots = [a for a in commands.get(instrument["row"], []) if a.endswith(".py")]
                self.assertTrue(
                    roots,
                    f"gate/registry.json has no row {instrument['row']!r} whose command names a .py module"
                    " (an instrument the import rule cannot judge is named in NOT_JUDGED)",
                )
                code = {p for p in instrument["code"] if p.endswith(".py")}
                want = closure(roots[0])
                self.assertEqual(
                    (sorted(want - code), sorted(code - want)), ([], []), "(missing from code, not imported)"
                )
        # Witness. No id is named under two kinds. Each named id is exactly one instrument, of the kind it is named
        # under: a missing one is absent on the left, a second one is there twice, and one the rule could judge reads
        # "judgeable". Every instrument is judged or named: the loop has no third way, and this holds it to that.
        self.assertEqual(len(listed), len(named), "NOT_JUDGED names an id under two kinds")
        self.assertEqual(sorted(found), sorted(named.items()), "(named instruments as found) != NOT_JUDGED")
        self.assertEqual(sorted([*judged, *(id for id, _ in found)]), sorted(i["id"] for i in instruments))


class RealTree(unittest.TestCase):
    """The pin over the real files (design §4.2; the coordinator, once gate/law.json's instruments were written from
    lane D1's measurement and lane D2's -cases): every registry row pinned, every proof counted, every case asserting a
    verdict. A deleted proof case, a demoted row or a lowered min_wants is red here at T2."""

    def test_the_real_tree_holds_its_teeth(self):
        repo = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
        with open(os.path.join(repo, "gate", "law.json"), encoding="utf-8") as f:
            manifest = law.load_manifest(f.read())
        rows = registry.load(os.path.join(repo, "gate", "registry.json"))
        listed = subprocess.run(["git", "ls-files", "-z"], cwd=repo, capture_output=True, check=True, timeout=60).stdout
        tracked = {p.decode() for p in listed.split(b"\0") if p}
        go_files = sorted({p.path for i in manifest.instruments for p in i.proofs if p.path.endswith("_test.go")})
        self.assertTrue(go_files, "no Go proof file is pinned")
        cases = subprocess.run(
            ["go", "run", "./tools/archcheck", "-cases", *go_files],
            cwd=repo,
            capture_output=True,
            text=True,
            env=support.go_env(),  # the pinned go, not whatever is first on the caller's PATH
            timeout=300,
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
        red_rows = {}
        for inst in manifest.instruments:
            for proof in inst.proofs:
                if proof.path.endswith(".py"):
                    with open(os.path.join(repo, proof.path), encoding="utf-8") as f:
                        red_rows[proof.path] = teeth.red_rows(f.read())
        self.assertEqual(teeth.pin(manifest, rows, tracked, counted, targets, red_rows=red_rows), [])
        self.assertGreaterEqual(len(manifest.instruments), len(rows))
        # CA-S1: every tracked driver no instrument names is held to the same strength rule.
        unnamed = teeth.unnamed_drivers(manifest, tracked)
        self.assertTrue(
            unnamed, "no unnamed driver: the parity drivers are expected here until an instrument names them"
        )
        faults = []
        for path in unnamed:
            with open(os.path.join(repo, path), encoding="utf-8") as f:
                faults += teeth.python_cases(f.read(), path)[1]
        self.assertEqual(faults, [])


if __name__ == "__main__":
    unittest.main()
