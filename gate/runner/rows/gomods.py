"""The Go module facts the deps and vulns rows both read, offline (S1.1c-4 plan check §2 S1; design note §2.1;
reviews A-B1, A-S7, DA-B1, DA-S3, DA-S4, DA-L5): each tracked module's build list and graph, the modules every
package builds or tests with (and each `tool` line's), taken over both supported platforms, and every package load
error by name, so a module never falls silently from the compiled set.

Each `go list -e -json` stream is decoded object by object: a standard package is skipped; a package's `Error` is a
named line before its missing `Module`; otherwise its module joins the compiled set. `DepsErrors` are read too, and
errors are keyed by their text (a dependency's error that its own package object reports is one line). No skip list:
the generated `<pkg>.test` mains carry a Module (design §0.3).
"""

import os
import subprocess
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from gate.runner import deps, rows

MODULES = (".", "gate/proofs")
PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))  # tools.json's two platforms (manifest.PLATFORMS)
LOOKUP_DISABLED = "module lookup disabled"
ENV = {"GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "CGO_ENABLED": "0"}  # both rows' go


@dataclass(frozen=True)
class Module:
    path: str
    version: str  # the version the build list selects (the root's when both select it)
    dir: str  # its source directory in the module cache, "" when absent on disk (deps' licence read; review DA-B1)
    replace_path: str  # the replacement's path, "" when none (review DA-L6)
    replace_version: str  # the replacement's version, "" for a directory replacement or none
    built: bool  # a package of either module, a test, or a tool line's package compiles it on either platform


@dataclass(frozen=True)
class Loaded:
    modules: tuple[Module, ...]  # every module either build list selects, the main modules aside, sorted by path
    versions: Mapping[str, Mapping[str, str]]  # module dir ("." or "gate/proofs") -> path -> version (one-version)
    graph: frozenset[str]  # every module path `go mod graph` names, the main modules aside
    go_version: str  # `go env GOVERSION` of the pinned go, e.g. go1.27.4 (review DA-L5)
    errors: tuple[str, ...]  # one named line per package load error, de-duplicated across the platforms


class Failed(Exception):
    """A go command that could not run: its argv and the first line of its stderr."""


def run(
    row_dir: str, env: Mapping[str, str], cwd: str, name: str, argv: Sequence[str], *, ok: Sequence[int] = (0,)
) -> str:
    """Run one command in `cwd`, keeping <name>.stdout and <name>.stderr in `row_dir`, and return its stdout; an
    exit code outside `ok` raises Failed (review DA-S4: `go mod tidy -diff` exits 1 for "differs", its diff on
    stdout, so the tidy line passes ok=(0, 1) and reads the text)."""
    out, err = os.path.join(row_dir, f"{name}.stdout"), os.path.join(row_dir, f"{name}.stderr")
    with open(out, "wb") as o, open(err, "wb") as e:
        code = subprocess.run(
            list(argv), cwd=cwd, env=dict(env), stdin=subprocess.DEVNULL, stdout=o, stderr=e
        ).returncode
    if code not in ok:
        lines = rows.read_text(err).splitlines()
        raise Failed(f"{' '.join(argv)}: {lines[0] if lines else f'exit {code}'}")
    return rows.read_text(out)


def _tag(mod: str) -> str:
    return "root" if mod == "." else mod.replace("/", "_")


class _Errors:
    """Load errors keyed by text: the first package reporting a text names it; the platforms that saw it counted."""

    def __init__(self) -> None:
        self.first: dict[tuple[str, str], tuple[str, str, str]] = {}  # key -> (path, module dir, text)
        self.seen: dict[tuple[str, str], set[tuple[str, str]]] = {}

    def add(self, key: tuple[str, str], path: str, mod: str, text: str, platform: tuple[str, str]) -> None:
        self.first.setdefault(key, (path, mod, text))
        self.seen.setdefault(key, set()).add(platform)

    def lines(self) -> tuple[str, ...]:
        out: set[str] = set()
        for key, (path, mod, text) in self.first.items():
            line = f"gomods: {path} ({mod}): {text}"
            if len(self.seen[key]) == 1:
                (goos, goarch) = next(iter(self.seen[key]))
                line += f" ({goos}/{goarch} only)"
            if LOOKUP_DISABLED in text:
                line += f": run go mod download in {mod}, then re-run"
            out.add(line)
        return tuple(sorted(out))


def _first_line(text: str) -> str:
    lines = text.strip().splitlines()
    return lines[0] if lines else ""


def _read_stream(
    text: str, mod: str, platform: tuple[str, str], compiled: set[str], errors: _Errors, deps_errors: list
) -> None:
    """One `go list -e -json -deps` stream: the compiled modules, each package's own error, its DepsErrors kept."""
    for obj in deps.decode_json_stream(text):
        if obj.get("Standard"):
            continue
        path = obj.get("ImportPath", "")
        err = _first_line((obj.get("Error") or {}).get("Err", ""))
        module = (obj.get("Module") or {}).get("Path", "")
        if err:
            errors.add(("err", err), path, mod, err, platform)
        elif not module:
            errors.add(("no module", path), path, mod, "no module", platform)
        else:
            compiled.add(module)
        for item in obj.get("DepsErrors") or []:
            text_ = _first_line(item.get("Err", ""))
            stack = item.get("ImportStack") or []
            if text_:
                deps_errors.append((("err", text_), stack[-1] if stack else path, mod, text_, platform))


def load(row_dir: str, env: Mapping[str, str]) -> Loaded:
    """Read both modules with the pinned go under `env` (GOPROXY=off, -mod=readonly), once per platform."""
    versions: dict[str, dict[str, str]] = {}
    fields: dict[str, tuple[str, str, str, str]] = {}  # path -> (version, dir, replace path, replace version)
    mains: set[str] = set()
    graph: set[str] = set()
    for mod in MODULES:
        build: dict[str, str] = {}
        main = ""
        listing = run(row_dir, env, mod, f"{_tag(mod)}.list-m", ["go", "list", "-m", "-json", "all"])
        for obj in deps.decode_json_stream(listing):
            if obj.get("Main"):
                main = obj["Path"]
            elif "Version" in obj:
                rep = obj.get("Replace") or {}
                build[obj["Path"]] = obj["Version"]
                fields.setdefault(
                    obj["Path"], (obj["Version"], obj.get("Dir") or "", rep.get("Path", ""), rep.get("Version", ""))
                )
        versions[mod] = build
        mains.add(main)
        graph |= deps.graph_nodes(run(row_dir, env, mod, f"{_tag(mod)}.graph", ["go", "mod", "graph"]), [main])
    compiled: set[str] = set()
    errors = _Errors()
    deps_errors: list = []
    for goos, goarch in PLATFORMS:
        penv = {**env, "GOOS": goos, "GOARCH": goarch}
        for mod in MODULES:
            listings = [("packages", ["go", "list", "-e", "-json", "-deps", "-test", "./..."])]
            if mod == "gate/proofs":
                listings.append(("tool", ["go", "list", "-e", "-json", "-deps", "tool"]))
            for kind, argv in listings:
                text = run(row_dir, penv, mod, f"{_tag(mod)}.{kind}.{goos}_{goarch}", argv)
                _read_stream(text, mod, (goos, goarch), compiled, errors, deps_errors)
    for key, path, mod, text, platform in deps_errors:  # after every package's own error: the text is the key
        errors.add(key, path, mod, text, platform)
    go_version = run(row_dir, env, ".", "goversion", ["go", "env", "GOVERSION"]).strip()
    modules = tuple(
        Module(path, version, directory, rpath, rversion, path in compiled)
        for path, (version, directory, rpath, rversion) in sorted(fields.items())
    )
    return Loaded(modules, versions, frozenset(graph - mains), go_version, errors.lines())
