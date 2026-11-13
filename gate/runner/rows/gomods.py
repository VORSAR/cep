"""The Go module facts the deps and vulns rows both read, offline (S1.1c-4 plan check §2 S1; reviews A-B1, A-S7):
each tracked module's build list and graph, the modules every package builds or tests with (and each `tool` line's),
taken over both supported platforms, and every package load error by name, so a module never falls silently from
the compiled set.

W0 stub: the signature is agreed and the body raises; lane E1 lifts rows/deps.py's gather into it.
"""

from collections.abc import Mapping, Sequence
from dataclasses import dataclass

MODULES = (".", "gate/proofs")
PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))  # tools.json's two platforms (manifest.PLATFORMS)


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
    raise NotImplementedError("gomods.run: not built yet (S1.1c-4 lane E1)")


def load(row_dir: str, env: Mapping[str, str]) -> Loaded:
    """Read both modules with the pinned go under `env` (GOPROXY=off, -mod=readonly), once per platform."""
    raise NotImplementedError("gomods.load: not built yet (S1.1c-4 lane E1)")
