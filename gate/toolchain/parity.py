"""Parity: the gate's first step in every job, and its one hard stop (design note §3).

Seam stub of S1.1a-1, landed by the coordinator before wave 1. Lane B owns this file and replaces
the body; the runner (Lane A) calls `check` and records its result as the per-job parity record.
"""

from collections.abc import Mapping

from gate.toolchain.manifest import Tool


def check(tools: list[Tool], cache: str, env: Mapping[str, str], *, full_tree: bool) -> list[str]:
    """Check every tool; return one reason per failing tool, in tools.json order ([] means pass).

    Per tool, short-circuit in order: not installed → path (shutil.which(command) under env["PATH"],
    realpath-compared) → version (version_argv run in `env`, first stdout line exact) → sha256 of every
    exec_sha256 entry → (only if full_tree, i.e. T2 and above) tree_sha256. Never runs a binary that
    lies outside the cache.
    """
    raise NotImplementedError("S1.1a-1 Lane B")
