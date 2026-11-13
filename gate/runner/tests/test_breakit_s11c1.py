"""QA break-it tests for S1.1c-1 (the LAW Δ block, the expiry, the deps row): each test pins a boundary the build
left open. A red test here is a finding for the owner, ranked in the QA report; it is kept red until the finding
is fixed, forwarded or rejected (ground rule 16).
"""

import contextlib
import io
import json
import os
from unittest import mock

from gate.runner import git, law
from gate.runner.rows import deps as deps_row
from gate.runner.rows import gomods, lawdelta
from gate.runner.tests import support

IDENTITY = ("-c", "user.name=t", "-c", "user.email=t@example.com")
FLAG = "SPEC changed — owner decision required"


class DocLine(support.GateTest):
    def test_a_changed_heading_names_its_own_section(self):
        """The first changed line IS a heading: the block must name that section, not the one above it."""
        old = "# T\n\n## 1. A\nx\n\n## 2. B\ny\n"
        new = old.replace("## 2. B", "## 2. B (renamed)")
        self.assertEqual(law.doc_line(old, new, FLAG), f"§2 — {FLAG}")

    def test_a_new_section_appended_names_the_new_section(self):
        """A whole new section after the last one: the block must name the section it adds, not the last one."""
        old = "# T\n\n## 1. A\nx\n"
        new = old + "\n## 2. B\ny\n"
        self.assertEqual(law.doc_line(old, new, FLAG), f"§2 — {FLAG}")


class LawDeltaRow(support.GateTest):
    """The row in-process in a temporary repository on `main` (test_lawdelta.py's shape)."""

    def commit(self, message: str) -> None:
        self.git("add", "-A")
        self.git(*IDENTITY, "commit", "-q", "-m", message)

    def manifest(self, *paths: dict) -> str:
        return json.dumps({"schema": 1, "note": "", "paths": list(paths), "instruments": []})

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

    def test_a_directory_entry_without_a_slash_is_not_live(self):
        """`gate/runner` (no slash) is a directory: git tracks files under it, so liveness passes, but entry_for
        never matches a file under it (only a path ending "/" is a prefix). On main the row passes having checked
        nothing under the directory; on a branch any change there is the "matches no entry" failure. A listed path
        that names no tracked file and is not a prefix must fail liveness."""
        self.git("checkout", "-q", "-b", "main")
        self.track("gate/law.json", self.manifest({"path": "gate/runner", "kind": "code"}))
        self.track("gate/runner/x.py", "x\n")
        self.commit("main")
        code, out = self.in_process()
        self.assertEqual(code, 1, out)
        self.assertIn("lawdelta: gate/runner is listed in gate/law.json but git tracks nothing there", out)

    def test_a_staged_rename_under_the_law_paths_is_one_edit(self):
        """The sixth line counts law files with uncommitted edits; `git status -z` reports a staged rename as two
        NUL records (new path, old path), so one rename reads as two files."""
        self.git("checkout", "-q", "-b", "main")
        self.track("docs/a.md", "a\n")
        self.commit("main")
        self.git("mv", "docs/a.md", "docs/b.md")
        self.assertEqual(git.dirty(["docs/"], cwd=self.repo), 1)


LIST_M = (
    '{"Path": "example.com/main", "Main": true, "Dir": "/w"}\n'
    '{"Path": "example.com/dep", "Version": "v1.2.0", "Replace": {"Path": "example.com/dep", "Version": "v1.0.0",'
    ' "Dir": "/cache/example.com/dep@v1.0.0"}, "Dir": "/cache/example.com/dep@v1.0.0"}\n'
)


class DepsGather(support.GateTest):
    def test_a_replace_directive_is_named(self):
        """`replace example.com/dep v1.2.0 => example.com/dep v1.0.0` in go.mod: what is built is v1.0.0, but
        `go list -m -json` reports Version v1.2.0 (the require line) with the replacement under Replace, and
        `go mod graph` shows v1.2.0 too. The row reads only Version, so the allowlist's v1.2.0 passes while
        v1.0.0 (or a local directory, or another module path) is what ships. A replace must be a named line."""

        def fake(row_dir: str, env: dict, cwd: str, name: str, argv, *, ok=(0,)) -> str:
            # gomods.run, answered by argv (design §2.7: the loader is the deps row's gather; review DA-S12).
            argv = list(argv)
            if argv[:3] == ["go", "list", "-m"]:
                return LIST_M if cwd == "." else '{"Path": "example.com/proofs", "Main": true, "Dir": "/p"}\n'
            if argv[:3] == ["go", "mod", "graph"]:
                return "example.com/main example.com/dep@v1.2.0\n" if cwd == "." else ""
            if argv[:2] == ["go", "list"] and argv[-1] == "./...":
                return (
                    '{"ImportPath": "example.com/dep", "Module": {"Path": "example.com/dep"}}\n' if cwd == "." else ""
                )
            if argv[:2] == ["go", "list"] and argv[-1] == "tool":
                return ""
            if argv == ["go", "env", "GOVERSION"]:
                return "go1.27.4\n"
            raise AssertionError(argv)

        with mock.patch.object(gomods, "run", fake):
            modules, graph, failures = deps_row.gather(self.run_dir, {})
        # Owner, 2026-11-11 (the fix round): gather carries the replacement; the allowlist decides (compare).
        self.assertEqual([f for f in failures if "replace" in f], [], f"failures {failures}")
        self.assertEqual([m.replace for m in modules if m.path == "example.com/dep"], ["example.com/dep v1.0.0"])
