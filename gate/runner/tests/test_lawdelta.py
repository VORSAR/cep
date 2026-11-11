"""The lawdelta row (S1.1c-1 design §2.4): every `failed:` text, law.txt, GATE_COVERED after liveness, the sixth line.

The world is a temporary repository holding the real gate/law.json and, under every listed path, one tracked
file; the row script is the real one (gate/runner/rows/lawdelta.py), run under this interpreter. The two defensive
texts are reached in-process with a patched parser and a patched numstat (review QA-S5).
"""

import contextlib
import io
import json
import os
import sys
from unittest import mock

from gate.runner import law
from gate.runner.rows import lawdelta
from gate.runner.tests import support

SCRIPT = os.path.join(support.REPO, "gate", "runner", "rows", "lawdelta.py")
LAW = os.path.join("gate", "law.json")
IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")
NO_BASE = "lawdelta: no merge base with main (fetch-depth 0 or a local main is needed)"
HEAD_IS_MAIN = "LAW Δ: HEAD is main — read the PR run"
PLACEHOLDER = "placeholder.txt"


class LawDelta(support.GateTest):
    def setUp(self) -> None:
        super().setUp()
        with open(os.path.join(support.REPO, LAW), encoding="utf-8") as f:
            self.law_text = f.read()
        self.manifest = law.load_manifest(self.law_text)

    def commit(self, message: str) -> None:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "-m", message)

    def law_world(self, law_text: str | None = None) -> None:
        """`main` holding the manifest and one tracked file under every listed path."""
        self.git("checkout", "-q", "-b", "main")
        self.track(LAW, law_text or self.law_text)
        for entry in self.manifest.paths:
            rel = entry.path + PLACEHOLDER if entry.path.endswith("/") else entry.path
            if rel == LAW:
                continue
            self.track(rel, "S1.1c-1\n" if entry.kind == "marker" else f"{rel}\n")
        self.commit("main")

    def on_change(self) -> None:
        self.git("checkout", "-q", "-b", "change")

    def law_row(self):
        command = (sys.executable, "-I", "-B", "-X", "utf8", SCRIPT)
        return self.row("lawdelta", command=command, expected_scope=("git", LAW), min_wants=1)

    def run_script(self) -> tuple[int, str, str]:
        """The row as a process of its own, as the runner starts it; (exit code, stdout, the row directory)."""
        self.counter += 1
        row_dir = os.path.join(self.root, f"row{self.counter}")
        os.mkdir(row_dir)
        env = dict(support.base_env(), GATE_ROW_DIR=row_dir)
        code, out, err = support.run_tool([sys.executable, "-I", "-B", "-X", "utf8", SCRIPT], self.repo, env)
        self.assertEqual(err, b"", err)
        return code, out.decode("utf-8", errors="replace"), row_dir

    def in_process(self) -> tuple[int, str]:
        self.counter += 1
        row_dir = os.path.join(self.root, f"row{self.counter}")
        os.mkdir(row_dir)
        out = io.StringIO()
        with (
            contextlib.chdir(self.repo),
            mock.patch.dict(os.environ, {"GATE_ROW_DIR": row_dir}),
            contextlib.redirect_stdout(out),
        ):
            code = lawdelta.main(["lawdelta.py"])
        return code, out.getvalue()

    def law_txt(self, row_dir: str) -> str:
        with open(os.path.join(row_dir, "law.txt"), encoding="utf-8") as f:
            return f.read()

    # Through the runner.

    def test_no_law_change_passes_with_none(self):
        self.law_world()
        self.on_change()
        self.track("notes.txt", "not a law path\n")
        self.commit("change")
        self.run_rows([self.law_row()])
        record = self.record("lint", "lawdelta")
        self.assertEqual((record["verdict"], record["reason"]), ("pass", "covered 1"))
        self.assertEqual(self.row_file("lint", "lawdelta", "law.txt"), "LAW Δ: none\n".encode())

    # The row's own process.

    def test_covered_is_printed_after_liveness_and_before_the_block(self):
        self.law_world()
        self.on_change()
        self.write("gate/run.py", "changed\n")
        self.commit("a code law file")
        code, out, row_dir = self.run_script()
        self.assertEqual(code, 0, out)
        base = self.git("merge-base", "HEAD", "main").decode().strip()[:7]
        want = ["GATE_COVERED 1", f"LAW Δ (1 file, vs {base})", "gate/run.py: 1 file +1/−1"]
        self.assertEqual(out.splitlines(), want, out)
        self.assertEqual(self.law_txt(row_dir), "\n".join(want[1:]) + "\n")

    def test_head_on_main_says_so(self):
        self.law_world()
        code, out, row_dir = self.run_script()
        self.assertEqual((code, out.splitlines()), (0, ["GATE_COVERED 1", HEAD_IS_MAIN]), out)
        self.assertEqual(self.law_txt(row_dir), HEAD_IS_MAIN + "\n")

    def test_a_manifest_fault_is_named_before_covered(self):
        doc = json.loads(self.law_text)
        doc["schema"] = 2
        self.law_world(json.dumps(doc, ensure_ascii=False) + "\n")
        code, out, _ = self.run_script()
        self.assertEqual(code, 1, out)
        self.assertEqual(out.splitlines(), ["failed: lawdelta: gate/law.json: schema 2, want 1"], out)

    def test_an_untracked_listed_path_is_named_before_covered(self):
        doc = json.loads(self.law_text)
        doc["paths"].append({"path": "gate/planted-untracked.json", "kind": "code"})
        self.law_world(json.dumps(doc, ensure_ascii=False) + "\n")
        code, out, _ = self.run_script()
        self.assertEqual(code, 1, out)
        want = "failed: lawdelta: gate/planted-untracked.json is listed in gate/law.json but git tracks nothing there"
        self.assertEqual(out.splitlines(), [want], out)

    def test_no_merge_base_is_a_failure_after_covered(self):
        self.git("checkout", "-q", "-b", "work")
        self.track(LAW, self.law_text)
        for entry in self.manifest.paths:
            rel = entry.path + PLACEHOLDER if entry.path.endswith("/") else entry.path
            if rel != LAW:
                self.track(rel, f"{rel}\n")
        self.commit("work")
        code, out, _ = self.run_script()
        self.assertEqual((code, out.splitlines()), (1, ["GATE_COVERED 1", f"failed: {NO_BASE}"]), out)

    def test_the_sixth_line_names_uncommitted_law_edits(self):
        self.law_world()
        self.on_change()
        self.track("notes.txt", "not a law path\n")
        self.commit("change")
        self.write("tools.json", "edited, not committed\n")
        code, out, row_dir = self.run_script()
        want = ["GATE_COVERED 1", "LAW Δ: none", "LAW Δ: 1 law files have uncommitted edits (not shown)"]
        self.assertEqual((code, out.splitlines()), (0, want), out)
        self.assertEqual(self.law_txt(row_dir), "\n".join(want[1:]) + "\n")

    # The two defensive texts, in-process.

    def test_a_crashing_parser_is_named(self):
        self.law_world()
        self.on_change()
        self.write("gate/current-sprint", "S4.1\n")
        self.commit("the marker moved")
        with mock.patch("gate.runner.law.delta", side_effect=RuntimeError("planted")):
            code, out = self.in_process()
        want = "failed: lawdelta: the marker parser crashed on gate/current-sprint: RuntimeError: planted"
        self.assertEqual((code, out.splitlines()), (1, ["GATE_COVERED 1", want]), out)

    def test_a_file_matching_no_entry_is_named(self):
        self.law_world()
        self.on_change()
        self.track("notes.txt", "not a law path\n")
        self.commit("change")
        with mock.patch("gate.runner.git.numstat", return_value=[("gate/zzz/unlisted.txt", 1, 0)]):
            code, out = self.in_process()
        want = "failed: lawdelta: gate/zzz/unlisted.txt changed under a listed prefix but matches no entry"
        self.assertEqual((code, out.splitlines()), (1, ["GATE_COVERED 1", want]), out)
