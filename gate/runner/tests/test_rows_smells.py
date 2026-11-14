"""The smells row (S1.1c-5 design note §2.3, §2.4, §2.7, §3.1; §8 rows 1.7-1.12, 1.14, 1.15, 3.5, 3.6).

The row runs as the runner runs it: a child process of the pinned interpreter, cwd the world's repository, its
`git`, `go` and `golangci-lint` found on PATH. Here those three are one fake (FAKE, a Python script) that logs its
argv, cwd and Go environment to a JSON-lines file and answers from the world's scenario: `git ls-files` the tracked
lists, `go list -e -json ./...` the platform's packages, `golangci-lint run` the platform's report, stderr and exit.
Every expected line is §2.3's text, written by hand.

design ambiguity: §2.3 does not say how the row finds its tools; these tests assume PATH (as every row does) and
`git ls-files` for both tracked lists (§2.4). §2.3 step 6 does not give the format of "<sorted>": the tests check the
line's start and its two halves only.
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest

from gate.runner import golangci
from gate.runner.tests.test_golangci import DESIGN_TOML

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
ROW = os.path.join(REPO, "gate", "runner", "rows", "smells.py")
MODULE = "github.com/vorsar/cep"
ENABLED = [*golangci.LINTERS, "typecheck"]
CAP_S = 60

FAKE = r"""
import json, os, sys
name = os.path.basename(sys.argv[0])
scenario = json.load(open(os.environ["FAKE_SCENARIO"], encoding="utf-8"))
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as log:
    env = {k: v for k, v in os.environ.items() if k.startswith(("GO", "CGO_")) or k == "GOLANGCI_LINT_CACHE"}
    log.write(json.dumps({"tool": name, "argv": sys.argv[1:], "cwd": os.getcwd(), "env": env}) + "\n")
if name == "git":
    key = "gomods" if "go.mod" in sys.argv else "tracked"
    sys.stdout.write("".join(p + "\0" for p in scenario[key]))
    sys.exit(0)
goos = os.environ.get("GOOS", "")
if name == "go":
    sys.stdout.write("".join(json.dumps(p) for p in scenario["golist"][goos]))
    sys.exit(0)
lint = scenario["lint"][goos]
path = next(a.split("=", 1)[1] for a in sys.argv if a.startswith("--output.json.path="))
if lint.get("raw") is not None:
    open(path, "w", encoding="utf-8").write(lint["raw"])
elif lint.get("report") is not None:
    open(path, "w", encoding="utf-8").write(json.dumps(lint["report"]))
sys.stderr.write(lint.get("stderr", ""))
sys.exit(lint["exit"])
"""


def package(rel_dir: str, *files: str, name: str = "", **extra: object) -> dict:
    """One `go list -e -json` object; Dir is filled in by the world."""
    pkg = {
        "Dir": rel_dir,
        "ImportPath": MODULE if rel_dir == "." else f"{MODULE}/{rel_dir}",
        "Name": name or ("cep" if rel_dir == "." else rel_dir.rsplit("/", 1)[-1]),
        "GoFiles": [f for f in files if not f.endswith("_test.go")],
        "TestGoFiles": [f for f in files if f.endswith("_test.go")],
    }
    pkg.update(extra)
    return pkg


def issue(file: str, line: int, linter: str, text: str, column: int = 2) -> dict:
    return {"FromLinter": linter, "Text": text, "Pos": {"Filename": file, "Line": line, "Column": column}}


def report(*issues: dict, enabled: list[str] | None = None, error: str = "", warnings: list[str] = ()) -> dict:
    rep: dict = {"Linters": [{"Name": n, "Enabled": True} for n in (ENABLED if enabled is None else enabled)]}
    if error:
        rep["Error"] = error
    if warnings:
        rep["Warnings"] = [{"Tag": "runner/source_code", "Text": w} for w in warnings]
    return {"Issues": list(issues) or None, "Report": rep}


def clean(*issues: dict, **kw) -> dict:
    return {"exit": 1 if issues or kw.get("error") else 0, "report": report(*issues, **kw)}


PANIC = 'use of `panic` forbidden because "[panic]"'
FMTPRINT = 'use of `fmt.Println` forbidden because "[fmtprint]"'
GOCYCLO = "cyclomatic complexity 11 of func `F` is high (> 10)"
FUNLEN = "Function 'F' is too long (61 > 60)"


def temp(target: str, kind: str = "TEMP", expiry: str = "2026-12-14") -> dict:
    entry = {"target": target, "kind": kind, "reason": "size family not yet cleaned", "sprint": "S1.1c-5"}
    if kind == "TEMP":
        entry |= {"expiry": expiry, "issue": "S1.1c-5-1"}
    else:
        entry["ref"] = "docs/sprints/decisions.md"
    return entry


WAIVES_NOTHING = (
    "tools/allowlists/smells.json: internal/p gocyclo waives nothing: the package is clean for gocyclo; remove it"
)
UNCLAIMED = (
    "no package under linux or darwin claims it (_dir/, //go:build ignore, a nested module): move it or make it build"
)


class World(unittest.TestCase):
    """A repository, the fake tools, a scenario; `run` executes the row once and returns (exit, lines, calls)."""

    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = os.path.realpath(tmp.name)
        self.repo = os.path.join(self.root, "repo")
        self.bin = os.path.join(self.root, "bin")
        self.row_dir = os.path.join(self.root, "row")
        self.cache = os.path.join(self.root, "toolcache")
        for d in (self.repo, self.bin, self.row_dir, os.path.join(self.repo, "tools", "allowlists")):
            os.makedirs(d, exist_ok=True)
        with open(os.path.join(self.repo, golangci.CONFIG), "w", encoding="utf-8") as f:
            f.write(DESIGN_TOML)  # the design's §2.1 document, never the committed file under test
        for tool in ("git", "go", "golangci-lint"):
            path = os.path.join(self.bin, tool)
            with open(path, "w", encoding="utf-8") as f:
                f.write(f"#!{sys.executable} -IB\n{FAKE}")
            os.chmod(path, 0o755)
        self.packages = {"linux": [package("internal/p", "p.go")], "darwin": [package("internal/p", "p.go")]}
        self.tracked = ["internal/p/p.go"]
        self.gomods = ["go.mod"]
        self.lint = {"linux": clean(), "darwin": clean()}
        self.smells: list[dict] = []
        self.generated: list[dict] = []

    def write_json(self, rel: str, doc: object) -> str:
        path = os.path.join(self.root, rel) if not rel.startswith("tools/") else os.path.join(self.repo, rel)
        with open(path, "w", encoding="utf-8") as f:
            json.dump(doc, f)
        return path

    def run_row(self, tier: str = "T1", files: list[str] | None = None, cwd: str | None = None):
        golist = {
            goos: [p | {"Dir": os.path.join(self.repo, p["Dir"]) if p["Dir"] != "." else self.repo} for p in pkgs]
            for goos, pkgs in self.packages.items()
        }
        scenario = {"tracked": self.tracked, "gomods": self.gomods, "golist": golist, "lint": self.lint}
        for rule, entries in (("smells", self.smells), ("generated", self.generated)):
            self.write_json(
                f"tools/allowlists/{rule}.json", {"schema": 1, "note": "x", "rule": rule, "entries": entries}
            )
        log = os.path.join(self.root, "calls.jsonl")
        if os.path.exists(log):
            os.unlink(log)
        files_path = os.path.join(self.root, "files.nul")
        with open(files_path, "wb") as f:
            f.write(b"".join(p.encode() + b"\0" for p in (self.tracked if files is None else files)))
        env = {
            "PATH": self.bin + os.pathsep + "/usr/bin:/bin",
            "HOME": self.root,
            "GATE_ROW_DIR": self.row_dir,
            "GATE_TIER": tier,
            "GOWORK": "off",
            "VORSAR_CEP_TOOLS_CACHE": self.cache,
            "FAKE_SCENARIO": self.write_json("scenario.json", scenario),
            "FAKE_LOG": log,
        }
        out_path = os.path.join(self.root, "row.stdout")
        with open(out_path, "wb") as out:
            code = subprocess.run(
                [sys.executable, "-I", "-B", "-X", "utf8", ROW, files_path],
                cwd=cwd or self.repo,
                env=env,
                stdin=subprocess.DEVNULL,
                stdout=out,
                stderr=subprocess.STDOUT,
                timeout=CAP_S,
                check=False,
            ).returncode
        with open(out_path, "rb") as f:
            lines = f.read().decode("utf-8", errors="replace").splitlines()
        calls = []
        if os.path.exists(log):
            with open(log, encoding="utf-8") as f:
                calls = [json.loads(line) for line in f]
        return code, lines, calls

    def faults(self, lines: list[str]) -> list[str]:
        return [line.removeprefix("failed: smells: ") for line in lines if line.startswith("failed: smells: ")]

    def assert_fault(self, expected: str, **kw) -> list[str]:
        code, lines, _ = self.run_row(**kw)
        self.assertEqual(code, 1, lines)
        self.assertIn(expected, self.faults(lines), lines)
        return lines

    def assert_green(self, **kw) -> list[str]:
        code, lines, _ = self.run_row(**kw)
        self.assertEqual((code, self.faults(lines)), (0, []), lines)
        return lines


class Exit(World):
    def test_exit_0_with_an_issue_is_red(self):
        self.assert_green()  # witness: the clean world is green
        self.lint["linux"] = {"exit": 0, "report": report(issue("internal/p/p.go", 3, "forbidigo", PANIC))}
        self.assert_fault("golangci-lint (linux): exit 0 disagrees with 1 issues")

    def test_exit_1_with_nothing_is_red(self):
        self.lint["darwin"] = {"exit": 1, "report": report()}
        self.assert_fault("golangci-lint (darwin): exit 1 disagrees with 0 issues")

    def test_exit_3_is_red_with_stderr(self):
        self.lint["linux"] = {"exit": 3, "report": None, "stderr": "Error: can't load config: x\nsecond line\n"}
        self.assert_fault("golangci-lint (linux) exit 3: Error: can't load config: x")

    def test_no_report_file_is_red(self):
        self.lint["linux"] = {"exit": 1, "report": None}
        self.assert_fault("golangci-lint (linux) wrote no report")

    def test_a_report_left_by_an_earlier_run_is_never_read(self):
        stale = report(issue("internal/p/p.go", 9, "forbidigo", "STALE ISSUE"))
        for goos in ("linux", "darwin"):
            with open(os.path.join(self.row_dir, f"smells-{goos}.json"), "w", encoding="utf-8") as f:
                json.dump(stale, f)
        self.lint["linux"] = {"exit": 0, "report": None}
        lines = self.assert_fault("golangci-lint (linux) wrote no report")
        self.assertFalse([line for line in lines if "STALE ISSUE" in line], lines)

    def test_a_truncated_report_is_a_named_fault(self):
        self.lint["linux"] = {"exit": 1, "raw": '{"Issues": [{"FromLinter": "forbidigo"'}
        _, lines, _ = self.run_row()
        prefix = f"golangci-lint (linux): {self.row_dir}/smells-linux.json: unreadable report: "
        self.assertTrue([f for f in self.faults(lines) if f.startswith(prefix)], lines)


class Health(World):
    def test_report_error_is_red(self):
        self.lint["linux"] = clean(error='setup revive: cannot configure rule: "argument-limit"')
        self.assert_fault('golangci-lint (linux): setup revive: cannot configure rule: "argument-limit"')

    def test_a_warning_is_red(self):
        self.lint["darwin"] = clean(warnings=["Failed to get line 3 for file x.go"])
        self.assert_fault("golangci-lint (darwin) warns: Failed to get line 3 for file x.go")

    def assert_ran_short(self, enabled: list[str]) -> None:
        self.lint["linux"] = clean(enabled=enabled)
        _, lines, _ = self.run_row()
        ran = [f for f in self.faults(lines) if f.startswith("golangci-lint (linux) ran ")]
        self.assertEqual(len(ran), 1, lines)
        self.assertIn(" not ", ran[0])

    def test_enabled_short_by_one_is_red(self):
        self.assert_ran_short([n for n in ENABLED if n != "lll"])

    def test_enabled_without_typecheck_is_red(self):
        self.assert_ran_short(list(golangci.LINTERS))

    def test_a_typecheck_issue_in_an_excluded_package_is_red(self):
        self.smells = [temp("internal/p gocyclo")]
        self.lint["linux"] = clean(issue("internal/p/p.go", 3, "gocyclo", GOCYCLO))
        self.lint["darwin"] = clean(issue("internal/p/p.go", 3, "gocyclo", GOCYCLO))
        self.assert_green()  # witness: the exclusion drops its gocyclo finding
        self.lint["linux"] = clean(issue("internal/p/p.go", 4, "typecheck", "undefined: x", column=5))
        self.assert_fault("internal/p/p.go:4:5: typecheck: undefined: x")


class Merge(World):
    def test_a_finding_on_both_goos_counts_once(self):
        found = issue("internal/p/p.go", 3, "forbidigo", PANIC)
        self.lint = {"linux": clean(found), "darwin": clean(found)}
        _, lines, _ = self.run_row()
        self.assertEqual(self.faults(lines).count(f"internal/p/p.go:3:2: forbidigo: {PANIC}"), 1, lines)

    def test_a_linux_only_finding_is_kept(self):
        self.lint["linux"] = clean(issue("internal/p/p.go", 3, "forbidigo", PANIC))
        self.assert_fault(f"internal/p/p.go:3:2: forbidigo: {PANIC}")


class Drops(World):
    def with_package(self, pkg: dict, file: str) -> None:
        self.packages = {goos: [*pkgs, pkg] for goos, pkgs in self.packages.items()}
        self.tracked = sorted([*self.tracked, file])
        found = issue(file, 3, "forbidigo", FMTPRINT)
        self.lint = {"linux": clean(found), "darwin": clean(found)}

    def test_fmtprint_in_a_main_tool_is_dropped(self):
        self.with_package(package("tools/t", "main.go", name="main"), "tools/t/main.go")
        self.assert_green()

    def test_fmtprint_in_a_non_main_tools_package_is_red(self):
        self.with_package(package("tools/t", "t.go", name="t"), "tools/t/t.go")
        self.assert_fault(f"tools/t/t.go:3:2: forbidigo: {FMTPRINT}")

    def test_fmtprint_outside_tools_in_package_main_is_red(self):
        self.with_package(package("cmd/c", "main.go", name="main"), "cmd/c/main.go")
        self.assert_fault(f"cmd/c/main.go:3:2: forbidigo: {FMTPRINT}")

    def test_fmtprint_in_a_main_tools_xtest_is_red(self):
        pkg = package("tools/t", "main.go", name="main", XTestGoFiles=["x_test.go"])
        self.with_package(pkg, "tools/t/x_test.go")
        self.tracked = sorted([*self.tracked, "tools/t/main.go"])
        self.assert_fault(f"tools/t/x_test.go:3:2: forbidigo: {FMTPRINT}")

    def test_a_generated_json_target_is_dropped(self):
        found = issue("internal/p/p.go", 3, "lll", "The line is 130 characters long, which exceeds the maximum")
        self.lint = {"linux": clean(found), "darwin": clean(found)}
        self.assert_fault("internal/p/p.go:3:2: lll: The line is 130 characters long, which exceeds the maximum")
        self.generated = [
            {
                "target": "internal/p/p.go",
                "kind": "PERMANENT",
                "reason": "a generated file, listed",
                "sprint": "S1.1c-2",
            }
            | {"ref": "docs/sprints/decisions.md"}
        ]
        self.assert_green()

    def test_an_issue_at_a_file_no_package_claims_is_red(self):
        found = issue("tools/t/main.go", 3, "forbidigo", FMTPRINT)
        self.lint = {"linux": clean(found), "darwin": clean()}
        self.assert_fault(
            "tools/t/main.go:3: forbidigo: reported at a file no package claims under linux (a //line directive?): "
            + FMTPRINT
        )


class Env(World):
    def calls(self, tier: str = "T1", cwd: str | None = None) -> list[dict]:
        _, _, calls = self.run_row(tier=tier, cwd=cwd)
        return calls

    def test_go_list_and_golangci_share_one_env_per_platform(self):
        calls = self.calls()
        for goos, goarch in (("linux", "amd64"), ("darwin", "arm64")):
            with self.subTest(goos=goos):
                mine = [c for c in calls if c["tool"] in ("go", "golangci-lint") and c["env"].get("GOOS") == goos]
                self.assertEqual(sorted(c["tool"] for c in mine), ["go", "golangci-lint"], calls)
                self.assertEqual(mine[0]["env"], mine[1]["env"])
                want = {
                    "GOOS": goos,
                    "GOARCH": goarch,
                    "CGO_ENABLED": "0",
                    "GOFLAGS": "-mod=readonly",
                    "GOPROXY": "off",
                    "GOTOOLCHAIN": "local",
                    "GOWORK": "off",
                }
                self.assertEqual({k: mine[0]["env"].get(k) for k in want}, want)

    def test_the_lint_cache_is_named(self):
        lints = [c for c in self.calls() if c["tool"] == "golangci-lint"]
        self.assertEqual(len(lints), 2)
        for call in lints:
            cache = call["env"].get("GOLANGCI_LINT_CACHE")
            self.assertEqual(cache, os.path.join(self.cache, "golangci-lint-cache"))
            self.assertFalse(cache.startswith(self.repo + os.sep))
        self.assertTrue(os.path.isdir(os.path.join(self.cache, "golangci-lint-cache")))

    def test_the_root_is_realpathed(self):
        link = os.path.join(self.root, "link")
        os.symlink(self.repo, link)
        calls = [c for c in self.calls(cwd=link) if c["tool"] in ("go", "golangci-lint")]
        self.assertEqual(len(calls), 4, calls)
        self.assertEqual({c["cwd"] for c in calls}, {self.repo})


class Unsafe(World):
    def test_imports_test_imports_and_xtest_imports_are_each_red(self):
        pkgs = [
            package("internal/a", "a.go", Imports=["unsafe"]),
            package("internal/b", "b.go", TestImports=["unsafe"]),
            package("internal/c", "c.go", XTestImports=["unsafe"]),
            package("internal/d", "d.go", Imports=["unsafe/x", "os"]),
        ]
        self.packages = {"linux": pkgs, "darwin": pkgs}
        self.tracked = ["internal/a/a.go", "internal/b/b.go", "internal/c/c.go", "internal/d/d.go"]
        _, lines, _ = self.run_row()
        want = [
            f"{MODULE}/internal/{x}: imports unsafe (banned everywhere, ADR-0015 §2 smells; //go:linkname needs it)"
            for x in "abc"
        ]
        self.assertEqual([f for f in self.faults(lines) if "imports unsafe" in f], want, lines)


class Exclusions(World):
    def plant(self, *found: dict) -> None:
        self.lint = {"linux": clean(*found), "darwin": clean(*found)}

    def setUp(self) -> None:
        super().setUp()
        pkgs = [package(".", "main.go"), package("internal/p", "p.go"), package("internal/p/sub", "s.go")]
        pkgs.append(package("internal/q", "q.go"))
        self.packages = {"linux": pkgs, "darwin": pkgs}
        self.tracked = ["internal/p/p.go", "internal/p/sub/s.go", "internal/q/q.go", "main.go"]

    def test_an_entry_drops_only_its_package_and_rule(self):
        self.smells = [temp("internal/p gocyclo")]
        self.plant(
            issue("internal/p/p.go", 3, "gocyclo", GOCYCLO),
            issue("internal/p/p.go", 9, "funlen", FUNLEN),
            issue("internal/q/q.go", 3, "gocyclo", GOCYCLO),
        )
        _, lines, _ = self.run_row()
        faults = self.faults(lines)
        self.assertNotIn(f"internal/p/p.go:3:2: gocyclo: {GOCYCLO}", faults)
        self.assertIn(f"internal/p/p.go:9:2: funlen: {FUNLEN}", faults)
        self.assertIn(f"internal/q/q.go:3:2: gocyclo: {GOCYCLO}", faults)

    def test_a_subpackage_is_not_dropped(self):
        self.smells = [temp("internal/p gocyclo")]
        self.plant(issue("internal/p/p.go", 3, "gocyclo", GOCYCLO), issue("internal/p/sub/s.go", 3, "gocyclo", GOCYCLO))
        self.assert_fault(f"internal/p/sub/s.go:3:2: gocyclo: {GOCYCLO}")

    def test_a_root_package_entry_drops_a_root_finding(self):
        self.smells = [temp(". funlen")]
        self.plant(issue("main.go", 3, "funlen", FUNLEN))
        self.assert_green()

    def test_an_expired_entry_drops_nothing_at_t4(self):
        self.smells = [temp("internal/p gocyclo", expiry="2026-01-01")]
        self.plant(issue("internal/p/p.go", 3, "gocyclo", GOCYCLO))
        self.assert_green(tier="T3")  # witness: below T4 an expired entry still drops
        self.assert_fault(f"internal/p/p.go:3:2: gocyclo: {GOCYCLO}", tier="T4")

    def test_an_entry_that_drops_nothing_is_red(self):
        self.smells = [temp("internal/p gocyclo")]
        self.plant(issue("internal/p/p.go", 3, "gocyclo", GOCYCLO))
        self.assert_green()  # witness: the entry drops its finding
        self.plant()
        self.assert_fault(WAIVES_NOTHING)

    def test_waives_nothing_is_not_judged_after_a_fault(self):
        self.smells = [temp("internal/p gocyclo")]
        self.lint["darwin"] = {"exit": 3, "report": None, "stderr": "Error: x\n"}
        _, lines, _ = self.run_row()
        self.assertIn("golangci-lint (darwin) exit 3: Error: x", self.faults(lines))
        self.assertFalse([f for f in self.faults(lines) if "waives nothing" in f], lines)

    def test_waives_nothing_is_not_judged_on_a_staged_run(self):
        self.smells = [temp("internal/p gocyclo")]
        self.assert_fault(WAIVES_NOTHING)  # witness: at T1 the unrelated entry is judged
        lines = self.assert_green(tier="T0", files=["internal/q/q.go"])
        self.assertIn("smells: exclusions not judged at T0 (staged packages only)", lines)

    def test_a_bad_target_form_is_red(self):
        for target in ("internal/p", "internal/p lll", "internal/p gocyclo extra", "internal/p  gocyclo"):
            with self.subTest(target=target):
                self.smells = [temp(target)]
                self.assert_fault(f'smells.json: {target}: not "<package dir> <rule>"')

    def test_a_permanent_size_entry_is_red(self):
        self.smells = [temp("internal/p gocyclo", kind="PERMANENT")]
        self.assert_fault("smells.json: internal/p gocyclo: a size exclusion is TEMP, with an expiry and a known issue")

    def test_an_unlinted_target_is_a_bad_form(self):
        self.smells = [temp("internal/p unlinted")]
        self.assert_fault('smells.json: internal/p unlinted: not "<package dir> <rule>"')


class Covered(unittest.TestCase):
    """§2.4: golangci.go_files and golangci.covered, pure."""

    ROOT = "/w/repo"

    def files(self, *pkgs: dict) -> tuple[dict, list[str]]:
        listing = [p | {"Dir": os.path.join(self.ROOT, p["Dir"])} for p in pkgs]
        return golangci.go_files(listing, self.ROOT)

    def test_an_unclaimed_file_is_red(self):
        linux, _ = self.files(package("a", "a.go"))
        got = golangci.covered(["a/a.go", "_x/x.go"], linux, linux, frozenset(), ["go.mod"])
        self.assertEqual(
            got[2],
            [f"_x/x.go: {UNCLAIMED}"],
        )

    def test_ignored_under_both_goos_is_red(self):
        linux, _ = self.files(package("a", "a.go"))
        got = golangci.covered(["a/a.go", "a/i.go"], linux, linux, frozenset({"a/i.go"}), ["go.mod"])
        self.assertIn("a/i.go: excluded by build constraints under linux and darwin", got[2])

    def test_a_nested_go_mod_is_red(self):
        linux, _ = self.files(package("a", "a.go"))
        mods = ["gate/proofs/go.mod", "go.mod", "x/go.mod"]
        self.assertEqual(
            golangci.covered(["a/a.go"], linux, linux, frozenset(), mods)[2],
            ["x/go.mod: a nested module is never linted"],
        )

    def test_a_linux_only_file_counts_once(self):
        linux, _ = self.files(package("a", "a.go", "a_linux.go"))
        darwin, _ = self.files(package("a", "a.go", "a_darwin.go"))
        got = golangci.covered(["a/a.go", "a/a_darwin.go", "a/a_linux.go"], linux, darwin, frozenset(), ["go.mod"])
        self.assertEqual(got, (3, 0, []))

    def test_a_testdata_file_is_counted_as_data(self):
        linux, _ = self.files(package("a", "a.go"))
        got = golangci.covered(["a/a.go", "a/testdata/x/x.go"], linux, linux, frozenset(), ["go.mod"])
        self.assertEqual(got, (2, 1, []))

    def test_a_testdata_segment_must_be_whole(self):
        linux, _ = self.files(package("a", "a.go"))
        got = golangci.covered(["a/a.go", "mytestdata/x.go", "a/testdata.go"], linux, linux, frozenset(), ["go.mod"])
        self.assertEqual(got[1], 0)
        self.assertEqual(len(got[2]), 2, got)

    def test_a_file_claimed_twice_is_red(self):
        _, faults = self.files(package("a", "a.go"), package("a", "a.go", name="b"))
        self.assertTrue([f for f in faults if "a/a.go" in f], faults)

    def test_a_package_error_is_red(self):
        broken = package("a", "a.go", Error={"Err": "a.go:1:1: expected 'package', found x\nmore"})
        _, faults = self.files(broken)
        self.assertIn(f"{MODULE}/a: a.go:1:1: expected 'package', found x", faults)

    def test_covered_equals_tracked_when_all_accounted(self):
        linux, faults = self.files(
            package(".", "main.go"), package("a", "a.go", "a_test.go", XTestGoFiles=["x_test.go"])
        )
        self.assertEqual(faults, [])
        self.assertEqual(linux["a/x_test.go"], golangci.GoFile(f"{MODULE}/a", "a_test"))
        self.assertEqual(linux["main.go"], golangci.GoFile(MODULE, "cep"))
        tracked = ["a/a.go", "a/a_test.go", "a/testdata/t.go", "a/x_test.go", "main.go"]
        self.assertEqual(
            golangci.covered(tracked, linux, linux, frozenset(), ["gate/proofs/go.mod", "go.mod"]), (5, 1, [])
        )


class Staged(World):
    def lint_targets(self, calls: list[dict]) -> list[list[str]]:
        return [
            [a for a in c["argv"] if not a.startswith("-") and a not in ("run",)]
            for c in calls
            if c["tool"] == "golangci-lint"
        ]

    def test_each_staged_files_package_is_linted_once(self):
        pkgs = [package(".", "main.go"), package("internal/p", "a.go", "b.go")]
        self.packages = {"linux": pkgs, "darwin": pkgs}
        self.tracked = ["internal/p/a.go", "internal/p/b.go", "internal/p/testdata/x.go", "main.go"]
        _, _, calls = self.run_row(
            tier="T0", files=["internal/p/a.go", "internal/p/b.go", "internal/p/testdata/x.go", "main.go"]
        )
        targets = self.lint_targets(calls)
        self.assertEqual(len(targets), 2, calls)
        for argv in targets:
            self.assertEqual(argv[-2:], [".", "./internal/p"])
            self.assertFalse([a for a in argv if a.endswith("/...") or a == "./internal/p/testdata"], argv)

    def test_t0_gives_t1s_verdict_on_the_staged_packages(self):
        found = issue("internal/p/p.go", 3, "forbidigo", PANIC)
        self.lint = {"linux": clean(found), "darwin": clean(found)}
        _, t1, _ = self.run_row(tier="T1")
        _, t0, _ = self.run_row(tier="T0", files=["internal/p/p.go"])
        self.assertIn(f"internal/p/p.go:3:2: forbidigo: {PANIC}", self.faults(t1))
        self.assertEqual(self.faults(t0), self.faults(t1))

    def test_no_new_or_whole_files_flag_at_t2_and_up(self):
        banned = ("--new", "--whole-files")
        for tier in ("T0", "T1", "T2", "T3", "T4"):
            with self.subTest(tier=tier):
                _, _, calls = self.run_row(tier=tier)
                lints = [c["argv"] for c in calls if c["tool"] == "golangci-lint"]
                self.assertEqual(len(lints), 2, calls)
                self.assertFalse([a for argv in lints for a in argv if a.startswith(banned)], lints)
        with open(os.path.join(REPO, ".github", "workflows", "gate.yml"), encoding="utf-8") as f:
            workflow = f.read()
        self.assertFalse([b for b in banned if b in workflow])


class Lock(World):
    """Owner, during S1.1c-5's W1: the row runs golangci-lint with --allow-parallel-runners."""

    def test_the_lint_argv_is_pinned(self):
        _, _, calls = self.run_row()
        lints = sorted((c["env"]["GOOS"], c["argv"]) for c in calls if c["tool"] == "golangci-lint")
        want = [
            (
                goos,
                [
                    "run",
                    "--allow-parallel-runners",
                    "--config",
                    ".golangci.toml",
                    f"--output.json.path={self.row_dir}/smells-{goos}.json",
                    "./...",
                ],
            )
            for goos in ("darwin", "linux")
        ]
        self.assertEqual(lints, want)


def ignored(path: str, line: int, col: int, linter: str, first: int, last: int, expanded: bool) -> str:
    """One `GL_DEBUG=nolint_filter` line for an issue a //nolint silenced, as golangci-lint v2.15.0 writes it
    (recorded by hand from the pinned binary, pkg/result/processors/nolint_filter.go:128)."""
    ptr = "0x6e04262e0b40" if expanded else "<nil>"
    return (
        f'level=debug msg="[nolint_filter] found ignored range for issue &{{{linter} use of `panic` forbidden because'
        f' \\"[panic]\\"  [] planted.example/internal/p {path}:{line}:{col} <nil> 0 [] false  internal/p/p.go'
        f' internal/p/p.go}}: {{[{linter}] map[] {{{first} {last}}} 8 {ptr}}}"\n'
    )


class Silenced(World):
    """Coordinator, during S1.1c-5's W1 (acceptance line 2): a waiver on the wrong line is red. golangci-lint widens a
    trailing //nolint to the next line's node when that node starts at the comment's column; the row reads the
    nolint filter's own record of each silenced issue and requires the issue's line to be the directive's line."""

    def test_an_issue_silenced_from_the_line_above_is_red(self):
        path = os.path.join(self.repo, "internal", "p", "p.go")
        self.lint["linux"] = clean() | {"stderr": ignored(path, 9, 2, "forbidigo", 9, 9, False)}
        self.assert_green()  # witness: an issue silenced by the //nolint on its own line is green
        self.lint["linux"] = clean() | {"stderr": ignored(path, 5, 2, "forbidigo", 4, 5, True)}
        self.assert_fault(
            "internal/p/p.go:5:2: forbidigo: silenced by the //nolint on line 4; a //nolint waives only its own line"
        )

    def test_a_silenced_nolintlint_report_is_not_judged(self):
        path = os.path.join(self.repo, "internal", "p", "p.go")
        self.lint["linux"] = clean() | {"stderr": ignored(path, 4, 8, "nolintlint", 3, 3, False)}
        self.assert_green()


class Summary(World):
    def test_the_green_line_format_is_pinned(self):
        pkgs = [package("internal/p", "p.go"), package("internal/q", "q.go")]
        self.packages = {"linux": pkgs, "darwin": pkgs}
        self.tracked = ["internal/p/p.go", "internal/p/testdata/x.go", "internal/q/q.go"]
        self.smells = [temp("internal/q funlen"), temp("internal/p gocyclo")]
        self.lint["linux"] = clean(
            issue("internal/p/p.go", 3, "gocyclo", GOCYCLO), issue("internal/q/q.go", 3, "funlen", FUNLEN)
        )
        self.lint["darwin"] = clean(
            issue("internal/p/p.go", 7, "gocyclo", GOCYCLO), issue("internal/q/q.go", 3, "funlen", FUNLEN)
        )
        lines = self.assert_green()
        self.assertIn("GATE_COVERED 3", lines)
        self.assertIn(
            "smells: 3 issues dropped by 2 exclusions (internal/p gocyclo 2, internal/q funlen 1); "
            "1 testdata files counted as data; linux and darwin",
            lines,
        )


if __name__ == "__main__":
    unittest.main()
