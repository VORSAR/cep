"""The Go module facts the deps and vulns rows both read, offline (S1.1c-4 plan check §2 S1; reviews A-B1, A-S7):
each tracked module's build list and graph, the modules every package builds or tests with (and each `tool` line's),
taken over both supported platforms, and every package load error by name, so a module never falls silently from
the compiled set.

W0 stub: the signature is agreed and the body raises; lane E1 lifts rows/deps.py's gather into it.
"""

from collections.abc import Mapping
from dataclasses import dataclass

MODULES = (".", "gate/proofs")
PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))  # tools.json's two platforms (manifest.PLATFORMS)


@dataclass(frozen=True)
class Module:
    path: str
    version: str  # the version the build list selects
    replace: str  # the replacement as "path version" or "path" (a directory), "" when none
    built: bool  # a package of either module, a test, or a tool line's package compiles it on either platform


@dataclass(frozen=True)
class Loaded:
    modules: tuple[Module, ...]  # every module either build list selects, the main modules aside, sorted by path
    graph: frozenset[str]  # every module path `go mod graph` names, the main modules aside
    errors: tuple[str, ...]  # one named line per package load error or package with no module path


def load(row_dir: str, env: Mapping[str, str]) -> Loaded:
    """Read both modules with the pinned go under `env` (GOPROXY=off, -mod=readonly), once per platform."""
    raise NotImplementedError("gomods.load: not built yet (S1.1c-4 lane E1)")
