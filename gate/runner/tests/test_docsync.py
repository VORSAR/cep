"""docsync (a) (S1.1c-2 design §4.1, §0.7, §5 lane D): every Go package `go list` reports is listed in ARCHITECTURE
§3's tree.

`gate/runner/docsync.py` is pure: `tree(md)` is the first fence after the `## 3.` heading (read through
`anchors.prose`), `listed(lines)` the first column's directories (text before two spaces, split on ", "; `x/` -> `x`;
a .go file names its directory, `cep.go` -> `.`, any other file nothing (BK-L1); a placeholder path such as `io/*`
is ignored), `missing(packages, listed)` the packages not listed, sorted. The row runs `go list -e -f '{{.Dir}}'
./...` with `GOFLAGS=-mod=readonly` and `GOPROXY=off`, reads its exit code and ignores its stderr (offline, `go:
downloading ...` lands there, §0.7).

design ambiguity: whether a package under a listed directory (internal/runtime/sub under internal/runtime/) is listed;
read here as not (an exact directory, the §3 tree names packages one per line).
design ambiguity: a `## 3.` heading whose section holds no fence, the first fence sitting under `## 4.`; read here as
the ValueError ("the first fence after the heading" taken within §3).
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest

from gate.runner import docsync

ROWS = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "rows")

TREE = """\
cep.go, runtime.go, event.go, errors.go   package cep        — public embeddable API
rule/                                     package rule       — public generics rule-builder API
io/http, io/stdin, io/inmemory   one package per built-in connector
io/*                                      a placeholder, never a package
internal/parser/                          lexer, grammar, AST
tools/importcheck/rules.json              the layer law
tools/allowlists/, tools/deps-allowlist.json  the one allowlist grammar
.githooks/, Makefile, tools.json          the gate
"""

DOC = f"""\
# Architecture

## 2. Layers

```
internal/earlier/                         a fence before §3 is not §3's tree
```

## 3. Repository and package layout (normative — ADR-0001)

Module path: prose naming `internal/prose/` is not the tree.

```
{TREE}```

*(Annotated: `tools/archcheck/` is built.)*

```
internal/second/                          a second fence is not the tree
```

## 4. The canonical event
"""


class Tree(unittest.TestCase):
    def test_the_first_fence_after_the_heading(self):
        self.assertEqual(docsync.tree(DOC), TREE.splitlines())

    def test_no_section_3_is_a_value_error(self):
        with self.assertRaises(ValueError):
            docsync.tree(DOC.replace("## 3. Repository", "## 7. Repository"))

    def test_a_section_3_without_a_fence_is_a_value_error(self):
        doc = "# A\n\n## 3. Layout\n\nprose only\n\n## 4. Next\n\n```\ninternal/x/  late\n```\n"
        with self.assertRaises(ValueError):
            docsync.tree(doc)

    def test_a_heading_inside_a_fence_is_not_the_section(self):
        doc = "# A\n\n```\n## 3. a heading in code\n```\n\n```\ninternal/x/  x\n```\n"
        with self.assertRaises(ValueError):
            docsync.tree(doc)


class Listed(unittest.TestCase):
    def test_the_real_first_column_shapes(self):
        self.assertEqual(
            docsync.listed(TREE.splitlines()),
            {
                ".",
                "rule",
                "io/http",
                "io/stdin",
                "io/inmemory",
                "internal/parser",
                "tools/allowlists",
                ".githooks",
                "Makefile",
            },
        )

    def test_a_placeholder_is_ignored(self):
        self.assertEqual(docsync.listed(["io/*                    connectors"]), set())

    def test_only_a_go_file_names_its_directory(self):
        # BK-L1 (S1.1c-2's fix round): a data file (rules.json, deps-allowlist.json, tools.json) names no package.
        lines = [
            "cep.go                         the API",
            "internal/x/x.go                a file of internal/x",
            "tools/importcheck/rules.json   the layer law",
            "tools/deps-allowlist.json      the register",
            "api/exports.golden             the export law",
        ]
        self.assertEqual(docsync.listed(lines), {".", "internal/x"})


class Missing(unittest.TestCase):
    LISTED = frozenset({".", "rule", "internal/parser", "io/http"})

    def test_every_package_listed(self):
        self.assertEqual(docsync.missing([".", "rule", "internal/parser", "io/http"], set(self.LISTED)), [])

    def test_a_missing_package_is_named_sorted(self):
        got = docsync.missing(["tools/shape", ".", "tools/archcheck", "rule"], set(self.LISTED))
        self.assertEqual(got, ["tools/archcheck", "tools/shape"])

    def test_a_package_named_only_in_prose_is_missing(self):
        listed = docsync.listed(docsync.tree(DOC))
        self.assertEqual(docsync.missing(["internal/prose", "internal/parser"], listed), ["internal/prose"])

    def test_a_package_under_a_listed_directory_is_missing(self):
        self.assertEqual(docsync.missing(["internal/parser/sub"], set(self.LISTED)), ["internal/parser/sub"])


FAKE_GO = """\
import json, os, sys
with open(os.path.join(os.path.dirname(os.path.abspath(sys.argv[0])), "argv.json"), "w") as f:
    json.dump({"argv": sys.argv[1:], "GOFLAGS": os.environ.get("GOFLAGS"), "GOPROXY": os.environ.get("GOPROXY")}, f)
sys.stderr.write("go: downloading example.com/x v1.0.0\\ngo: example.com/x: offline\\n")
root = os.getcwd()
for rel in PACKAGES:
    print(os.path.join(root, rel) if rel != "." else root)
sys.exit(CODE)
"""


class Row(unittest.TestCase):
    """The real row, run as the runner runs it: at the repository root, GATE_ROW_DIR set, a fake `go` on PATH."""

    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="gate-docsync-")
        self.addCleanup(tmp.cleanup)
        self.root = os.path.realpath(tmp.name)
        self.repo = os.path.join(self.root, "repo")
        self.bin = os.path.join(self.root, "bin")
        self.row_dir = os.path.join(self.root, "row")
        for d in (self.repo, self.bin, self.row_dir, os.path.join(self.repo, "docs", "architecture")):
            os.makedirs(d, exist_ok=True)
        with open(os.path.join(self.repo, "docs", "architecture", "ARCHITECTURE.md"), "w", encoding="utf-8") as f:
            f.write(DOC)

    def run_row(self, packages: list[str], code: int = 0) -> tuple[int, list[str]]:
        go = os.path.join(self.bin, "go")
        with open(go, "w", encoding="utf-8") as f:
            f.write(f"#!{sys.executable} -I\nPACKAGES = {packages!r}\nCODE = {code}\n" + FAKE_GO)
        os.chmod(go, 0o755)
        env = {
            "PATH": os.pathsep.join([self.bin, os.path.dirname(sys.executable), "/usr/bin", "/bin"]),
            "GATE_ROW_DIR": self.row_dir,
            "HOME": self.root,
        }
        proc = subprocess.run(
            [sys.executable, "-I", "-B", "-X", "utf8", os.path.join(ROWS, "docsync.py")],
            cwd=self.repo,
            env=env,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        return proc.returncode, proc.stdout.splitlines()

    def test_listed_packages_pass_with_go_list_stderr_ignored(self):
        self.assertEqual(self.run_row([".", "rule", "internal/parser", "io/http"]), (0, ["GATE_COVERED 4"]))

    def test_go_list_is_run_offline_and_read_only(self):
        self.run_row(["."])
        with open(os.path.join(self.bin, "argv.json"), encoding="utf-8") as f:
            seen = json.load(f)
        self.assertEqual(seen["argv"], ["list", "-e", "-f", "{{.Dir}}", "./..."])
        self.assertEqual((seen["GOFLAGS"], seen["GOPROXY"]), ("-mod=readonly", "off"))

    def test_a_package_the_tree_does_not_list_is_named(self):
        code, out = self.run_row([".", "internal/parser", "tools/archcheck"])
        self.assertEqual(code, 1)
        self.assertIn("failed: docsync: tools/archcheck is a Go package and ARCHITECTURE §3 does not list it", out)
        self.assertIn("GATE_COVERED 3", out)

    def test_go_list_failing_is_red(self):
        code, out = self.run_row(["."], code=1)
        self.assertEqual(code, 1, out)


if __name__ == "__main__":
    unittest.main()
