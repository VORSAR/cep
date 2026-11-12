"""The lawdelta row's proofs (S1.1c-1, plan check v2 section 6 "LAW Δ"; ADR-0015 section 1.3): the LAW Δ
block is the summary's first line, a planted law edit appears with its semantic one-liner, an empty delta says
`none`, a world with no `main` is a FAIL (never a silent pass, risk 1), an untracked listed path is a FAIL naming
it, a HEAD that is `main` says so, and a registry without the row says `not run`.

The world holds the gate and every tracked file under every path gate/law.json lists, so the row's liveness check
reads the real manifest against a real tree. Branches are named explicitly: `main` before the first commit, the
change on `change` after it, so the merge base is the copied tree (test_api_planted's shape; review QA-5).
"""

import json
import os

from gate.proofs.drivers import support

ROW = "lawdelta"  # the row its red cases land on: teeth.red_rows ties this proof to it (BK-S7)
LAW = "gate/law.json"
RULES = "tools/importcheck/rules.json"
MARKER = "gate/current-sprint"
TIER = "T1"
BLOCK = "LAW Δ"
# The edge is a new line of rules.json's `imports` (a numstat of +1/-0; an edge added to an existing package would
# be +1/-1), so the one-liner names the package and the edge as written here.
EDGE_ANCHOR = '    "internal/event": [],\n'
EDGE_LINE = '    "internal/planted": ["internal/event"],\n'
EDGE_DELTA = f"{RULES} +1/−0 — +1 edge internal/planted → internal/event"
NOT_RUN = f"{BLOCK}: not run"
NONE = f"{BLOCK}: none"
HEAD_IS_MAIN = f"{BLOCK}: HEAD is main — read the PR run"
FAILED = f"{BLOCK}: FAILED — see the lawdelta row"
NO_BASE = "lawdelta: no merge base with main"
UNTRACKED = "gate/planted-untracked.json"


def law_paths() -> list[str]:
    with open(os.path.join(support.REPO, LAW), "rb") as f:
        return [entry["path"] for entry in json.loads(f.read())["paths"]]


class LawDeltaPlanted(support.DriverTest):
    def law_world(self) -> None:
        """The gate plus every tracked file under every law path, uncommitted."""
        self.world.copy_gate()
        for path in law_paths():
            if path.endswith("/"):
                for rel in self.world.tracked(path):
                    self.world.copy(rel)
            else:
                self.world.copy(path)

    def on_main(self) -> None:
        """The copied tree as `main`'s one commit (git's default branch name never)."""
        self.world.git("checkout", "-q", "-b", "main")
        self.world.commit()

    def on_change(self) -> None:
        """A branch `change` from `main`, whose commits the tests add."""
        self.world.git("checkout", "-q", "-b", "change")

    def plant(self, rel: str, old: str, new: str) -> None:
        path = os.path.join(self.world.repo, rel)
        text = support.read_text(path)
        self.assertEqual(text.count(old), 1, f"the plant's anchor in {rel} moved")
        self.world.write(rel, text.replace(old, new))

    def run_row(self, row_id: str = "lawdelta") -> support.Run:
        self.world.commit()
        return self.run_committed(row_id)

    def run_committed(self, row_id: str = "lawdelta") -> support.Run:
        return self.world.run(registry=self.world.registry(support.registry_row(row_id)), tier=TIER)

    def first_line(self, run: support.Run) -> str:
        return run.stdout.split("\n")[0]

    def failed_lines(self, run: support.Run) -> list[str]:
        out = run.row_stdout("lint", "lawdelta")
        return [line.removeprefix("failed: ") for line in out.split("\n") if line.startswith("failed: ")]

    def assertBlockFirst(self, run: support.Run) -> list[str]:
        """The summary opens with the block; give its lines up to the row table's header."""
        lines = run.stdout.split("\n")
        self.assertTrue(lines[0].startswith(BLOCK), run.stdout)
        block = []
        for line in lines:
            if line.startswith("job "):
                break
            block.append(line)
        return block

    def test_a_planted_edge_is_named_in_the_block(self):
        self.law_world()
        self.on_main()
        self.on_change()
        self.plant(RULES, EDGE_ANCHOR, EDGE_ANCHOR + EDGE_LINE)
        run = self.run_row()
        self.assertRow(run, "lint", "lawdelta", True, "covered 1")
        block = self.assertBlockFirst(run)
        self.assertIn(EDGE_DELTA, block, run.stdout)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)

    def test_no_law_change_is_none_and_passes(self):
        self.law_world()
        self.on_main()
        self.on_change()
        self.world.write("planted-notes.txt", "not a law path\n")
        run = self.run_row()
        self.assertRow(run, "lint", "lawdelta", True, "covered 1")
        self.assertEqual(self.first_line(run), NONE, run.stdout)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)

    def test_no_main_is_a_fail_never_a_silent_pass(self):
        self.law_world()
        self.world.git("checkout", "-q", "-b", "work")
        run = self.run_row()
        self.assertGateFailed(run)
        record = run.record("lint", "lawdelta")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn(NO_BASE, record["reason"], record)
        failed = self.failed_lines(run)
        self.assertTrue(failed and failed[0].startswith(NO_BASE), run.row_stdout("lint", "lawdelta"))
        self.assertEqual(self.first_line(run), FAILED, run.stdout)

    def test_an_untracked_listed_path_is_a_fail_naming_it(self):
        self.law_world()
        path = os.path.join(self.world.repo, LAW)
        with open(path, encoding="utf-8") as f:
            law = json.load(f)
        law["paths"].append({"path": UNTRACKED, "kind": "code"})
        self.world.write(LAW, json.dumps(law, indent=2, ensure_ascii=False) + "\n")
        self.on_main()
        self.on_change()
        self.world.write("planted-notes.txt", "not a law path\n")
        run = self.run_row()
        self.assertGateFailed(run)
        record = run.record("lint", "lawdelta")
        self.assertEqual(record["verdict"], "fail", run.stdout)
        self.assertIn(UNTRACKED, record["reason"], record)
        self.assertEqual(self.first_line(run), FAILED, run.stdout)

    def test_head_on_main_says_so_and_passes(self):
        self.law_world()
        self.world.git("checkout", "-q", "-b", "work")
        self.world.commit()
        self.world.git("branch", "-M", "main")
        run = self.run_committed()
        self.assertRow(run, "lint", "lawdelta", True, "covered 1")
        self.assertEqual(self.first_line(run), HEAD_IS_MAIN, run.stdout)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)

    def test_a_moved_marker_is_named_in_the_block(self):
        self.law_world()
        self.on_main()
        self.on_change()
        # The marker as the sprint's G0 set it, read from the world's copy: a hard-coded "S1.1c-1" went red when
        # S1.1c-2's G0 moved the marker (S1.1c-2 W0, QA F1).
        old = support.read_text(os.path.join(self.world.repo, MARKER)).strip()
        new = "S4.2" if old == "S4.1" else "S4.1"
        self.plant(MARKER, f"{old}\n", f"{new}\n")
        run = self.run_row()
        self.assertRow(run, "lint", "lawdelta", True, "covered 1")
        block = self.assertBlockFirst(run)
        self.assertIn(f"{MARKER} +1/−1 — marker {old} → {new}", block, run.stdout)

    def test_a_registry_without_the_row_says_not_run(self):
        self.law_world()
        self.on_main()
        self.on_change()
        self.world.write("planted-notes.txt", "not a law path\n")
        run = self.run_row("ruff")
        self.assertTrue(self.first_line(run).startswith(NOT_RUN), run.stdout)
        cells = run.summary("lint", "ruff")
        self.assertEqual(cells[2], "PASS", run.stdout)
        # The row that did run passed over every file of its scope, counted from the world by the row's own
        # pathspecs, never a hard-coded count (S1.1c-2 W0; design §4.2: a pass on the row run).
        ruff = support.registry_row("ruff")
        self.assertEqual(ruff["expected_scope"][0], "git")
        in_scope = [p for p in self.world.git("ls-files", "-z", "--", *ruff["expected_scope"][1:]).split("\0") if p]
        self.assertTrue(in_scope)
        self.assertRow(run, "lint", "ruff", True, f"covered {len(in_scope)}")

    def test_a_dirty_law_file_is_named_on_a_last_line_and_the_row_passes(self):
        # owner, 2026-11-11 (design 2): at a local tier the block ends with the count of uncommitted law edits.
        self.law_world()
        self.on_main()
        self.on_change()
        self.world.write("planted-notes.txt", "not a law path\n")
        self.world.commit()
        self.world.write(MARKER, "S4.1\n")
        run = self.run_committed()
        self.assertRow(run, "lint", "lawdelta", True, "covered 1")
        block = self.assertBlockFirst(run)
        # Relations, not counts (design DQ-S10): none, the always-printed exemption line (design §2.6), then the edits.
        self.assertEqual(block[0], NONE, run.stdout)
        self.assertRegex(block[1], r"^exemptions: \d+ in code, \d+ listed \(\+0 this diff\)$")
        self.assertEqual(block[2], f"{BLOCK}: 1 law files have uncommitted edits (not shown)", run.stdout)
        self.assertEqual(run.tail(), ["GATE PASSED", "exit: 0"], run.stdout)
