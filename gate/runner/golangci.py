"""The smells row's config pin and report reader (S1.1c-5 design note §2.2, §2.4): the only reader of .golangci.toml.

`load` parses the config (a TOML error, a duplicate key included, is a ConfigError naming the file); `differences`
compares it whole and type-strictly with PINNED, the §2.1 document written here as tomllib parses it, so an edit to
the config without the same edit here is red. `read_report` reads golangci-lint's JSON report (`Issues`, and the
`Report`'s enabled linters, `Error` and `Warnings`). `go_files` maps every Go file a `go list -e -json ./...` listing
claims to its package, and `covered` accounts for every tracked Go file against the two platforms' maps (§2.4).
"""

import json
import os
import posixpath
import tomllib
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from gate.runner import pin

CONFIG = ".golangci.toml"
LINTERS = ("errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive")
SIZE_RULES = ("funlen", "gocyclo", "file-length-limit", "argument-limit")  # the size family, as smells.json names them
REVIVE_RULES = ("file-length-limit", "argument-limit")  # revive's two size rules, named by their Text's prefix
TESTDATA = "testdata"
ROOT_MODULES = ("go.mod", "gate/proofs/go.mod")  # the root module and the proofs module; any other is nested
UNCLAIMED = (
    "no package under linux or darwin claims it (_dir/, //go:build ignore, a nested module): move it or make it build"
)
FILE_LISTS = ("GoFiles", "CgoFiles", "TestGoFiles", "XTestGoFiles")

PINNED: dict[str, object] = {
    "version": "2",
    "run": {"relative-path-mode": "gomod", "tests": True, "build-tags": [], "modules-download-mode": "readonly"},
    "output": {"show-stats": False},
    "issues": {"max-issues-per-linter": 0, "max-same-issues": 0, "uniq-by-line": False, "new": False, "fix": False},
    "severity": {"default": "error"},
    "formatters": {"enable": []},
    "linters": {
        "default": "none",
        "enable": ["errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive"],
        "settings": {
            "errcheck": {
                "check-type-assertions": False,
                "check-blank": False,
                "disable-default-exclusions": True,
                "exclude-functions": [
                    "(*bytes.Buffer).Write",
                    "(*bytes.Buffer).WriteByte",
                    "(*bytes.Buffer).WriteRune",
                    "(*bytes.Buffer).WriteString",
                    "fmt.Print",
                    "fmt.Printf",
                    "fmt.Println",
                    "fmt.Fprint(*bytes.Buffer)",
                    "fmt.Fprintf(*bytes.Buffer)",
                    "fmt.Fprintln(*bytes.Buffer)",
                    "fmt.Fprint(*strings.Builder)",
                    "fmt.Fprintf(*strings.Builder)",
                    "fmt.Fprintln(*strings.Builder)",
                    "fmt.Fprint(os.Stderr)",
                    "fmt.Fprintf(os.Stderr)",
                    "fmt.Fprintln(os.Stderr)",
                    "(*io.PipeReader).CloseWithError",
                    "(*io.PipeWriter).CloseWithError",
                    "math/rand.Read",
                    "(*math/rand.Rand).Read",
                    "(*strings.Builder).Write",
                    "(*strings.Builder).WriteByte",
                    "(*strings.Builder).WriteRune",
                    "(*strings.Builder).WriteString",
                    "(hash.Hash).Write",
                ],
            },
            "forbidigo": {
                "analyze-types": True,
                "exclude-godoc-examples": False,
                "forbid": [
                    {"pattern": "^(fmt\\.Print.*|print|println)$", "msg": "[fmtprint]"},
                    {"pattern": "^panic$", "msg": "[panic]"},
                    {"pattern": "^reflect\\..*$", "msg": "[reflect]"},
                    {"pattern": "^unsafe\\..*$", "msg": "[unsafe]"},
                    {"pattern": "^os\\.(Getenv|LookupEnv|Environ)$", "msg": "[getenv]"},
                ],
            },
            "funlen": {"lines": 60, "statements": 100000, "ignore-comments": False},
            "gocritic": {},
            "gocyclo": {"min-complexity": 10},
            "lll": {"line-length": 120, "tab-width": 1},
            "nolintlint": {
                "require-explanation": True,
                "require-specific": True,
                "allow-unused": False,
                "allow-no-explanation": [],
            },
            "revive": {
                "enable-all-rules": False,
                "rules": [
                    {
                        "name": "file-length-limit",
                        "severity": "error",
                        "arguments": [{"max": 500, "skipComments": False, "skipBlankLines": False}],
                    },
                    {"name": "argument-limit", "severity": "error", "arguments": [5]},
                ],
            },
        },
        "exclusions": {
            "generated": "disable",
            "warn-unused": False,
            "presets": [],
            "paths": [],
            "paths-except": [],
            "rules": [
                {"path": "_test\\.go$", "linters": ["funlen", "gocyclo", "lll", "revive"]},
                {"path": "_test\\.go$", "linters": ["forbidigo"], "text": "\\[reflect\\]"},
                {"path": "_test\\.go$", "linters": ["forbidigo"], "text": "\\[fmtprint\\]"},
                {
                    "path-except": "^internal/(event|model|metrics|clock|pattern|window|expr|text|table|partition)/",
                    "linters": ["forbidigo"],
                    "text": "\\[reflect\\]",
                },
                {"path": "^(cmd|io)/", "linters": ["forbidigo"], "text": "\\[getenv\\]"},
            ],
        },
    },
}


class ConfigError(ValueError):
    """The config cannot be read: a TOML error (a duplicate key included) or a missing key."""


class ReportError(ValueError):
    """A golangci-lint JSON report that is not JSON or has the wrong shape, naming its path."""


@dataclass(frozen=True)
class Issue:
    linter: str
    rule: str
    text: str
    file: str
    line: int
    column: int


@dataclass(frozen=True)
class Report:
    issues: tuple[Issue, ...]
    enabled: frozenset[str]
    error: str
    warnings: tuple[str, ...]


@dataclass(frozen=True)
class GoFile:
    import_path: str
    package: str  # go list's Name; an XTestGoFiles entry's is Name + "_test"


def load(text: str) -> dict[str, object]:
    try:
        return tomllib.loads(text)
    except tomllib.TOMLDecodeError as e:
        raise ConfigError(f"{CONFIG}: {e}") from None


def differences(doc: Mapping[str, object]) -> list[str]:
    return pin.differences(doc, PINNED, CONFIG)


def enabled(doc: Mapping[str, object]) -> frozenset[str]:
    linters = doc.get("linters")
    names = linters.get("enable") if isinstance(linters, Mapping) else None
    if not isinstance(names, list) or not all(isinstance(n, str) for n in names):
        raise ConfigError(f"{CONFIG}: linters.enable is absent or not a list of names")
    return frozenset(names)


def rule_of(issue: Mapping[str, object]) -> str:
    linter = str(issue.get("FromLinter", ""))
    if linter != "revive":
        return linter
    prefix, sep, _ = str(issue.get("Text", "")).partition(": ")
    return prefix if sep and prefix in REVIVE_RULES else "revive"


def _field(obj: object, key: str, kind: type, where: str) -> object:
    if not isinstance(obj, dict):
        raise ValueError(f"{where}: not an object")
    value = obj.get(key)
    if type(value) is not kind:
        raise ValueError(f"{where}.{key}: not a {kind.__name__}")
    return value


def _issue(item: object, where: str) -> Issue:
    linter = _field(item, "FromLinter", str, where)
    text = _field(item, "Text", str, where)
    pos = _field(item, "Pos", dict, where)
    return Issue(
        linter=linter,
        rule=rule_of(item),
        text=text,
        file=_field(pos, "Filename", str, f"{where}.Pos"),
        line=_field(pos, "Line", int, f"{where}.Pos"),
        column=_field(pos, "Column", int, f"{where}.Pos"),
    )


def _report(doc: object) -> Report:
    if not isinstance(doc, dict):
        raise ValueError("not a JSON object")
    items = doc.get("Issues")
    if items is None:
        items = []
    if not isinstance(items, list):
        raise ValueError("Issues: not a list")
    rep = _field(doc, "Report", dict, "")
    linters = rep.get("Linters", [])
    warnings = rep.get("Warnings") or []
    error = rep.get("Error", "")
    if not isinstance(linters, list) or not isinstance(warnings, list) or type(error) is not str:
        raise ValueError("Report: Linters, Warnings or Error of the wrong type")
    on = frozenset(
        _field(x, "Name", str, "Report.Linters[]") for x in linters if isinstance(x, dict) and x.get("Enabled") is True
    )
    return Report(
        issues=tuple(_issue(item, f"Issues[{i}]") for i, item in enumerate(items)),
        enabled=on,
        error=error,
        warnings=tuple(_field(w, "Text", str, "Report.Warnings[]") for w in warnings),
    )


def read_report(path: str) -> Report:
    try:
        with open(path, "rb") as f:
            doc = json.loads(f.read().decode("utf-8", errors="replace"))
        return _report(doc)
    except ValueError as e:
        raise ReportError(f"{path}: unreadable report: {e}") from None


def go_files(listing: Sequence[Mapping[str, object]], root: str) -> tuple[dict[str, GoFile], list[str]]:
    """file -> GoFile for every file a package of the listing claims, paths relative to realpath(root)."""
    base = os.path.realpath(root)
    files: dict[str, GoFile] = {}
    faults: set[str] = set()
    for pkg in listing:
        import_path, name = str(pkg.get("ImportPath", "")), str(pkg.get("Name", ""))
        err = pkg.get("Error")
        if isinstance(err, Mapping):
            faults.add(f"{import_path}: {str(err.get('Err', '')).splitlines()[0] if err.get('Err') else 'error'}")
        rel_dir = os.path.relpath(os.path.realpath(str(pkg.get("Dir", ""))), base)
        if rel_dir == os.pardir or rel_dir.startswith(os.pardir + os.sep):
            faults.add(f"{import_path}: its directory {pkg.get('Dir')} is outside the root")
            continue
        for key in FILE_LISTS:
            package = f"{name}_test" if key == "XTestGoFiles" else name
            for file in pkg.get(key) or []:
                path = posixpath.normpath(posixpath.join(rel_dir.replace(os.sep, "/"), str(file)))
                if path in files:
                    faults.add(f"{path}: claimed by {files[path].import_path} and {import_path}")
                    continue
                files[path] = GoFile(import_path, package)
    return files, sorted(faults)


def is_testdata(path: str) -> bool:
    return TESTDATA in path.split("/")[:-1]


def covered(
    tracked: Sequence[str],
    linux: Mapping[str, GoFile],
    darwin: Mapping[str, GoFile],
    ignored_both: frozenset[str],
    gomods: Sequence[str],
) -> tuple[int, int, list[str]]:
    """(covered, testdata_count, faults): every tracked Go file accounted for, by §2.4's rules."""
    count = data = 0
    faults: list[str] = []
    for path in sorted(set(tracked)):
        if is_testdata(path):
            count, data = count + 1, data + 1
        elif path in ignored_both:
            faults.append(f"{path}: excluded by build constraints under linux and darwin")
        elif path in linux or path in darwin:
            count += 1
        else:
            faults.append(f"{path}: {UNCLAIMED}")
    faults.extend(f"{m}: a nested module is never linted" for m in sorted(set(gomods)) if m not in ROOT_MODULES)
    return count, data, sorted(faults)
