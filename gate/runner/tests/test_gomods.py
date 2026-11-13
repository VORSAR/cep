"""The loader both scan rows read, and the register's `built` mark (S1.1c-4 design note v1.1 §2.1, §2.6, §2.7; plan
check §2 S1-S2, §6 "The register"; reviews A-B1, A-S7, A-L7, QA-S3, DA-B1, DA-S3, DA-S4, DA-L5; owner 2026-11-13,
S1.1c-4's plan check, decision 1).

`gomods.load` is driven through its one `run(row_dir, env, cwd, name, argv, *, ok=(0,))` (design §2.1), patched here
as test_breakit_s11c1.py's DepsGather patches the deps row's; the fake answers by argv, the module directory and the
platform read from the env's GOOS/GOARCH, so it fixes no file name, and records each listing it answered. `run`
itself is tested on a real child process.
"""

import json
import os
import sys
import unittest
from unittest import mock

from gate.runner import deps
from gate.runner.rows import gomods
from gate.runner.tests import support
from gate.runner.tests.test_deps import CTX, entry, listing

ROOT_LIST_M = (
    '{"Path": "example.com/main", "Main": true, "Dir": "/m"}\n'
    '{"Path": "example.com/dep", "Version": "v1.0.0", "Dir": "/mod/example.com/dep@v1.0.0"}\n'
    '{"Path": "example.com/shared", "Version": "v1.1.0", "Dir": "/mod/example.com/shared@v1.1.0"}\n'
    '{"Path": "example.com/forked", "Version": "v1.0.0", "Replace": {"Path": "example.com/fork", "Version": "v1.0.1"},'
    ' "Dir": "/mod/example.com/fork@v1.0.1"}\n'
    '{"Path": "example.com/local", "Version": "v0.0.0", "Replace": {"Path": "../local"}, "Dir": "/local"}\n'
)
PROOFS_LIST_M = (
    '{"Path": "example.com/proofs", "Main": true, "Dir": "/p"}\n'
    '{"Path": "example.com/tool1", "Version": "v1.0.0", "Dir": "/mod/example.com/tool1@v1.0.0"}\n'
    '{"Path": "example.com/tool2", "Version": "v2.0.0"}\n'
    '{"Path": "example.com/linuxonly", "Version": "v0.1.0", "Dir": "/mod/example.com/linuxonly@v0.1.0"}\n'
    '{"Path": "example.com/shared", "Version": "v1.0.0", "Dir": "/mod/example.com/shared@v1.0.0"}\n'
)
PLATFORMS = {("linux", "amd64"), ("darwin", "arm64")}


def pkg(path: str, module: str | None, **extra: object) -> str:
    obj: dict = {"ImportPath": path, **extra}
    if module is not None:
        obj["Module"] = {"Path": module}
    return json.dumps(obj) + "\n"


def fake_go(*, tool_pkgs="", proofs_pkgs=lambda goos: "", root_pkgs="", calls: list | None = None):
    """A `run` answering go list -m, go mod graph, go list -deps -test ./..., go list -deps tool and go env GOVERSION.
    `tool_pkgs` and `root_pkgs` are a text, or a function of GOOS giving one."""

    def per(goos: str, value) -> str:
        return value(goos) if callable(value) else value

    def run(row_dir, env, cwd, name, argv, *, ok=(0,)):
        argv = list(argv)
        goos, goarch = env.get("GOOS", ""), env.get("GOARCH", "")
        if argv == ["go", "env", "GOVERSION"]:
            return "go1.27.4\n"
        if argv[:3] == ["go", "list", "-m"]:
            return ROOT_LIST_M if cwd == "." else PROOFS_LIST_M
        if argv[:3] == ["go", "mod", "graph"]:
            return "example.com/main example.com/dep@v1.0.0\n" if cwd == "." else ""
        if argv[:2] == ["go", "list"] and argv[-1] in ("./...", "tool"):
            if calls is not None:
                calls.append((cwd, goos, goarch, argv[-1], "-test" in argv, "-json" in argv))
            if argv[-1] == "./...":
                return (
                    per(goos, root_pkgs or pkg("example.com/main/x", "example.com/main"))
                    if cwd == "."
                    else proofs_pkgs(goos)
                )
            if cwd == ".":
                raise AssertionError("the root module has no tool line")
            return per(goos, tool_pkgs)
        raise AssertionError(f"unexpected go call in {cwd}: {argv}")

    return run


class Load(support.GateTest):
    def load(self, run) -> gomods.Loaded:
        with mock.patch.object(gomods, "run", run):
            return gomods.load(self.run_dir, {})

    def module(self, loaded: gomods.Loaded, path: str) -> gomods.Module:
        found = [m for m in loaded.modules if m.path == path]
        self.assertEqual(len(found), 1, loaded.modules)
        return found[0]

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
        self.assertEqual(built["example.com/dep"], False)
        self.assertEqual(loaded.errors, ())

    def test_a_package_built_only_for_linux_is_read_on_any_host(self):
        # Plan check §6 "The scan" (1e), review A-S7: the compiled set is the union over both platforms.
        def proofs(goos: str) -> str:
            return pkg("example.com/linuxonly/sys", "example.com/linuxonly") if goos == "linux" else ""

        loaded = self.load(fake_go(proofs_pkgs=proofs))
        self.assertEqual(self.module(loaded, "example.com/linuxonly").built, True)

    def test_each_platform_lists_both_modules_and_the_tool_pattern(self):
        # Design §2.1: per platform, -e -json -deps -test ./... in each module, and -e -json -deps tool in gate/proofs.
        calls: list = []
        self.load(fake_go(calls=calls))
        want = {(".", goos, goarch, "./...", True, True) for goos, goarch in PLATFORMS}
        want |= {("gate/proofs", goos, goarch, "./...", True, True) for goos, goarch in PLATFORMS}
        want |= {("gate/proofs", goos, goarch, "tool", False, True) for goos, goarch in PLATFORMS}
        self.assertEqual(set(calls), want)

    def test_the_build_lists_fields_versions_graph_and_go_version(self):
        # Review DA-B1: dir for deps' licence read; versions per module dir for the one-version check; DA-L6: the
        # replacement as fields; DA-L5: GOVERSION read once.
        loaded = self.load(fake_go())
        self.assertEqual(
            self.module(loaded, "example.com/dep"),
            gomods.Module("example.com/dep", "v1.0.0", "/mod/example.com/dep@v1.0.0", "", "", False),
        )
        forked = self.module(loaded, "example.com/forked")
        self.assertEqual(
            (forked.replace_path, forked.replace_version, forked.dir),
            ("example.com/fork", "v1.0.1", "/mod/example.com/fork@v1.0.1"),
        )
        local = self.module(loaded, "example.com/local")
        self.assertEqual((local.replace_path, local.replace_version), ("../local", ""))
        self.assertEqual(self.module(loaded, "example.com/tool2").dir, "")
        shared = self.module(loaded, "example.com/shared")
        self.assertEqual((shared.version, shared.dir), ("v1.1.0", "/mod/example.com/shared@v1.1.0"))  # the root's first
        self.assertEqual(loaded.versions["."]["example.com/shared"], "v1.1.0")
        self.assertEqual(loaded.versions["gate/proofs"]["example.com/shared"], "v1.0.0")
        self.assertEqual(set(loaded.versions), {".", "gate/proofs"})
        self.assertNotIn("example.com/main", loaded.versions["."])
        self.assertEqual([m.path for m in loaded.modules], sorted(m.path for m in loaded.modules))
        self.assertNotIn("example.com/main", {m.path for m in loaded.modules} | loaded.graph)
        self.assertIn("example.com/dep", loaded.graph)
        self.assertEqual(loaded.go_version, "go1.27.4")

    def test_an_error_is_named_before_no_module_once_for_both_platforms(self):
        # Review DA-S3: Error.Err's first line, before "no module"; the same text on both platforms is one line.
        broken = pkg("example.com/tool2/cmd/b", None, Error={"Err": "missing go.sum entry\nmore"})
        loaded = self.load(fake_go(tool_pkgs=broken))
        self.assertEqual(loaded.errors, ("gomods: example.com/tool2/cmd/b (gate/proofs): missing go.sum entry",))

    def test_a_package_with_no_module_and_no_error_is_named(self):
        loaded = self.load(fake_go(tool_pkgs=pkg("example.com/tool2/cmd/b", None)))
        self.assertEqual(loaded.errors, ("gomods: example.com/tool2/cmd/b (gate/proofs): no module",))

    def test_a_text_seen_on_one_platform_only_names_it(self):
        def tools(goos: str) -> str:
            return (
                pkg("example.com/tool2/cmd/b", None, Error={"Err": "build constraints exclude all Go files"})
                if goos == "linux"
                else ""
            )

        loaded = self.load(fake_go(tool_pkgs=tools))
        self.assertEqual(
            loaded.errors,
            (
                "gomods: example.com/tool2/cmd/b (gate/proofs): build constraints exclude all Go files "
                "(linux/amd64 only)",
            ),
        )

    def test_module_lookup_disabled_names_its_fix(self):
        # Measured with the pinned go1.27.4 and a module absent from the cache under GOPROXY=off (W0b's driver, 1c).
        missing = pkg("golang.org/x/exp/cmd/apidiff", None, Error={"Err": "module lookup disabled by GOPROXY=off"})
        loaded = self.load(fake_go(tool_pkgs=missing))
        self.assertEqual(
            loaded.errors,
            (
                "gomods: golang.org/x/exp/cmd/apidiff (gate/proofs): module lookup disabled by GOPROXY=off: "
                "run go mod download in gate/proofs, then re-run",
            ),
        )

    def test_deps_errors_are_read_only_for_a_path_absent_from_the_stream(self):
        # Design §2.1: a DepsErrors entry whose package is in the stream is that package's own Error, named once.
        stack = ["example.com/tool1/cmd/a", "example.com/tool1/inner"]
        present = pkg(
            "example.com/tool1/cmd/a", "example.com/tool1", DepsErrors=[{"Err": "boom", "ImportStack": stack}]
        )
        present += pkg("example.com/tool1/inner", "example.com/tool1", Error={"Err": "boom"})
        loaded = self.load(fake_go(tool_pkgs=present))
        self.assertEqual(loaded.errors, ("gomods: example.com/tool1/inner (gate/proofs): boom",))
        # A path in no package object: its DepsErrors entry is the only report, so it is named, by that path.
        gone = "example.com/gone/p"
        absent = pkg(
            "example.com/tool1/cmd/a",
            "example.com/tool1",
            DepsErrors=[
                {
                    "Err": f"cannot find module providing package {gone}",
                    "ImportStack": ["example.com/tool1/cmd/a", gone],
                }
            ],
        )
        loaded = self.load(fake_go(tool_pkgs=absent))
        self.assertEqual(loaded.errors, (f"gomods: {gone} (gate/proofs): cannot find module providing package {gone}",))

    def test_standard_packages_are_skipped_and_a_test_main_is_not(self):
        # Design §2.1: Standard first; no skip list, so a generated <pkg>.test main is read like any package (§0.3).
        root = pkg("fmt", None, Standard=True) + pkg("example.com/main/x", "example.com/main")
        root += pkg("example.com/main/x.test", None)
        loaded = self.load(fake_go(root_pkgs=root))
        self.assertEqual(loaded.errors, ("gomods: example.com/main/x.test (.): no module",))
        main_test = pkg("example.com/main/x.test", "example.com/main") + pkg("example.com/dep/d", "example.com/dep")
        self.assertEqual(self.module(self.load(fake_go(root_pkgs=main_test)), "example.com/dep").built, True)

    def test_errors_are_unique_and_sorted(self):
        def tools(goos: str) -> str:
            return (
                pkg("example.com/z/cmd", None, Error={"Err": "zeta"})
                + pkg("example.com/a/cmd", None, Error={"Err": "alpha"})
                + pkg("example.com/z/cmd", None, Error={"Err": "zeta"})
            )

        loaded = self.load(fake_go(tool_pkgs=tools))
        self.assertEqual(
            loaded.errors,
            ("gomods: example.com/a/cmd (gate/proofs): alpha", "gomods: example.com/z/cmd (gate/proofs): zeta"),
        )

    def test_a_module_built_in_one_build_list_and_graph_only_in_the_other_counts_once_as_built(self):
        # Plan check §6 "The scan" (20), review QA-L5.
        root = pkg("example.com/main/x", "example.com/main") + pkg("example.com/tool1/y", "example.com/tool1")
        loaded = self.load(fake_go(root_pkgs=root))
        self.assertEqual(self.module(loaded, "example.com/tool1").built, True)

    def test_the_tool_constant_is_gone(self):
        from gate.runner.rows import deps as deps_row

        self.assertFalse(hasattr(deps_row, "TOOL"), "design §2.7: the tool lines are read, not one constant")


CHILD = "import sys; print('out'); print('first\\nsecond', file=sys.stderr); sys.exit(int(sys.argv[1]))"


class Run(support.GateTest):
    """design §2.1: `run` keeps <name>.stdout/.stderr, returns stdout, and raises Failed outside `ok` (DA-S4)."""

    def child(self, code: int, **kw: object) -> str:
        argv = [sys.executable, "-I", "-c", CHILD, str(code)]
        return gomods.run(self.run_dir, dict(os.environ), self.root, "child", argv, **kw)

    def test_exit_0_returns_stdout_and_keeps_both_files(self):
        self.assertEqual(self.child(0), "out\n")
        with open(os.path.join(self.run_dir, "child.stdout"), encoding="utf-8") as f:
            self.assertEqual(f.read(), "out\n")
        with open(os.path.join(self.run_dir, "child.stderr"), encoding="utf-8") as f:
            self.assertEqual(f.read(), "first\nsecond\n")

    def test_an_exit_outside_ok_raises_failed_naming_the_first_stderr_line(self):
        with self.assertRaises(gomods.Failed) as caught:
            self.child(1)
        self.assertTrue(str(caught.exception).endswith(": first"), str(caught.exception))
        self.assertIn(CHILD, str(caught.exception))

    def test_an_exit_in_ok_returns_stdout(self):
        self.assertEqual(self.child(1, ok=(0, 1)), "out\n")
        with self.assertRaises(gomods.Failed):
            self.child(2, ok=(0, 1))


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
