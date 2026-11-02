"""tools.json: load, validate and locate the pinned tools (design note §3).

Seam stub of S1.1a-1, landed by the coordinator before wave 1. Lane B owns this file and replaces
every body; the runner (Lane A) codes against these names and signatures. Runs under Python 3.12+.
"""

from collections.abc import Mapping
from dataclasses import dataclass


class ManifestError(Exception):
    """tools.json is missing, malformed, or has no entry for this platform."""


@dataclass(frozen=True)
class Tool:
    """One pinned tool, resolved for the current platform."""

    name: str
    version: str
    command: str
    path_dir: str
    version_argv: tuple[str, ...]
    version_line: str
    url: str
    archive_sha256: str
    strip: str
    exec_sha256: Mapping[str, str]
    tree_sha256: str


def platform_key() -> str:
    """Return "darwin-arm64" or "linux-amd64"; raise ManifestError for anything else."""
    raise NotImplementedError("S1.1a-1 Lane B")


def cache_root(environ: Mapping[str, str]) -> str:
    """VORSAR_CEP_TOOLS_CACHE if set, else ~/.cache/vorsar/cep/tools (XDG is not consulted)."""
    raise NotImplementedError("S1.1a-1 Lane B")


def load(path: str, platform: str) -> list[Tool]:
    """Load and validate tools.json for one platform, in file order (design note §3, load validation)."""
    raise NotImplementedError("S1.1a-1 Lane B")


def tool_home(cache: str, tool: Tool) -> str:
    """<cache>/<name>-<version>-<archive_sha256[:12]>."""
    raise NotImplementedError("S1.1a-1 Lane B")


def path_dirs(cache: str, tools: list[Tool]) -> list[str]:
    """The PATH prefix: each tool's home joined with its path_dir, in tools.json order."""
    raise NotImplementedError("S1.1a-1 Lane B")
