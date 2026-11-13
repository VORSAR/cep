"""The loader both scan rows read, and the register's `built` mark (S1.1c-4 design note §2.1, §2.6, §2.7; plan check
§2 S1-S2, §6 "The register"; reviews A-B1, A-S7, A-L7, QA-S3; owner 2026-11-13, S1.1c-4's plan check, decision 1).

`gomods.load` is driven through its one `run(row_dir, env, cwd, name, argv)` (design §2.1, as rows/deps.py:36), patched
here as test_breakit_s11c1.py's DepsGather patches the deps row's; the fake answers by argv, the module directory
and the platform read from the env's GOOS/GOARCH, so it fixes no file name.
"""

import json
import unittest
from unittest import mock

from gate.runner import deps
from gate.runner.rows import gomods
from gate.runner.tests import support
from gate.runner.tests.test_deps import CTX, entry, listing

ROOT_LIST_M = (
    '{"Path": "example.com/main", "Main": true, "Dir": "/m"}\n{"Path": "example.com/dep", "Version": "v1.0.0"}\n'
)
PROOFS_LIST_M = (
    '{"Path": "example.com/proofs", "Main": true, "Dir": "/p"}\n'
    '{"Path": "example.com/tool1", "Version": "v1.0.0"}\n{"Path": "example.com/tool2", "Version": "v2.0.0"}\n'
    '{"Path": "example.com/linuxonly", "Version": "v0.1.0"}\n'
)


def pkg(path: str, module: str | None, **extra: object) -> str:
    obj: dict = {"ImportPath": path, **extra}
    if module is not None:
        obj["Module"] = {"Path": module}
    return json.dumps(obj) + "\n"


def fake_go(*, tool_pkgs: str, proofs_pkgs=lambda goos: "", root_pkgs: str = ""):
    """A `run` answering go list -m, go mod graph, go list -deps -test ./... and go list -deps tool."""

    def run(row_dir, env, cwd, name, argv):
        argv = list(argv)
        goos = env.get("GOOS", "")
        if argv[:3] == ["go", "list", "-m"]:
            return ROOT_LIST_M if cwd == "." else PROOFS_LIST_M
        if argv[:3] == ["go", "mod", "graph"]:
            return "example.com/main example.com/dep@v1.0.0\n" if cwd == "." else ""
        if argv[:2] == ["go", "list"] and argv[-1] == "./...":
            return (root_pkgs or pkg("example.com/main/x", "example.com/main")) if cwd == "." else proofs_pkgs(goos)
        if argv[:2] == ["go", "list"] and argv[-1] == "tool":
            if cwd == ".":
                raise AssertionError("the root module has no tool line")
            return tool_pkgs
        raise AssertionError(f"unexpected go call in {cwd}: {argv}")

    return run


class Load(support.GateTest):
    def load(self, run) -> gomods.Loaded:
        with mock.patch.object(gomods, "run", run):
            return gomods.load(self.run_dir, {})

    def test_every_tool_lines_modules_are_built_not_only_apidiffs(self):
        # Plan check §6 "The register" (2): a second `tool` line whose module apidiff does not reach is built, so its
        # licence is fingerprinted; today's rows/deps.py reads one TOOL constant and would leave it graph-only.
        tools = pkg("example.com/tool1/cmd/a", "example.com/tool1") + pkg(
            "example.com/tool2/cmd/b", "example.com/tool2"
        )
        loaded = self.load(fake_go(tool_pkgs=tools))
        built = {m.path: m.built for m in loaded.modules}
        self.assertEqual(built["example.com/tool1"], True)
        self.assertEqual(built["example.com/tool2"], True)
        self.assertEqual(loaded.errors, ())

    def test_a_package_built_only_for_linux_is_read_on_any_host(self):
        # Plan check §6 "The scan" (1e), review A-S7: the compiled set is the union over both platforms.
        def proofs(goos: str) -> str:
            return pkg("example.com/linuxonly/sys", "example.com/linuxonly") if goos == "linux" else ""

        loaded = self.load(fake_go(tool_pkgs="", proofs_pkgs=proofs))
        self.assertEqual({m.path: m.built for m in loaded.modules}["example.com/linuxonly"], True)

    def test_a_package_load_error_is_named_never_dropped(self):
        # Review A-B1: -e keeps a broken package with an empty module path; the loader names it, on each platform.
        broken = pkg("example.com/tool2/cmd/b", None, Error={"Err": "missing go.sum entry\nmore"})
        loaded = self.load(fake_go(tool_pkgs=broken))
        self.assertIn("gomods: example.com/tool2/cmd/b (gate/proofs, linux/amd64): no module", loaded.errors)
        self.assertIn("gomods: example.com/tool2/cmd/b (gate/proofs, darwin/arm64): no module", loaded.errors)
        self.assertEqual(list(loaded.errors), sorted(loaded.errors))
        dep_err = pkg("example.com/tool1/cmd/a", "example.com/tool1", DepsErrors=[{"Err": "cannot find module x"}])
        loaded = self.load(fake_go(tool_pkgs=dep_err))
        self.assertIn("gomods: example.com/tool1/cmd/a (gate/proofs, linux/amd64): cannot find module x", loaded.errors)

    def test_a_module_built_in_one_build_list_and_graph_only_in_the_other_counts_once_as_built(self):
        # Plan check §6 "The scan" (20), review QA-L5.
        root = pkg("example.com/main/x", "example.com/main") + pkg("example.com/tool1/y", "example.com/tool1")
        loaded = self.load(fake_go(tool_pkgs="", root_pkgs=root))
        self.assertEqual([m for m in loaded.modules if m.path == "example.com/tool1"][0].built, True)
        self.assertEqual(len([m for m in loaded.modules if m.path == "example.com/tool1"]), 1)

    def test_the_tool_constant_is_gone(self):
        from gate.runner.rows import deps as deps_row

        self.assertFalse(hasattr(deps_row, "TOOL"), "design §2.7: the tool lines are read, not one constant")


class BuiltMark(unittest.TestCase):
    """The register's optional `"built": false` (design §2.6; decision 1)."""

    def test_built_false_is_accepted(self):
        loaded = deps.load_allowlist(listing({**entry("golang.org/x/net"), "built": False}))
        self.assertEqual([e.module for e in loaded], ["golang.org/x/net"])
        self.assertIs(loaded[0].built, False)

    def test_built_true_is_refused_as_the_default(self):
        with self.assertRaisesRegex(ValueError, r'deps: golang.org/x/net: "built": true is the default: remove it'):
            deps.load_allowlist(listing({**entry("golang.org/x/net"), "built": True}))

    def test_built_of_any_other_value_is_refused(self):
        for bad in ("false", 0, None, [], {}):
            with (
                self.subTest(repr(bad)),
                self.assertRaisesRegex(ValueError, r"deps: golang.org/x/net: built must be false"),
            ):
                deps.load_allowlist(listing({**entry("golang.org/x/net"), "built": bad}))

    def test_a_marked_module_that_a_package_compiles_is_red(self):
        entries = deps.load_allowlist(listing({**entry("golang.org/x/net"), "built": False}))
        modules = [deps.Module("golang.org/x/net", "v1.0.0", "", True)]
        failed = deps.compare(modules, frozenset(), entries, list(deps.GOMODS), lambda d: None, CTX)
        self.assertIn(
            'deps: golang.org/x/net is marked never built, but a package compiles it: remove "built": false', failed
        )
