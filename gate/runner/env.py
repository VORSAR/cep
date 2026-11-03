"""The environment every row and tool runs in (design note §4 step 1; plan check S3a)."""

import os
from collections.abc import Iterable, Mapping

GO_ALLOWED = frozenset({"GOCACHE", "GOMODCACHE", "GOPATH", "GOPROXY", "GOSUMDB", "GOPRIVATE", "GONOPROXY", "GONOSUMDB"})
PINNED = {"GOTOOLCHAIN": "local", "GOENV": "off", "GOFLAGS": "", "GOWORK": "off", "CGO_ENABLED": "0"}


def _dropped(name: str) -> bool:
    if name.startswith("GO"):
        return name not in GO_ALLOWED
    # RUFF_*: a caller's RUFF_OUTPUT_FORMAT turned a clean tree red, RUFF_OUTPUT_FILE wrote outside the run (BA2).
    return name.startswith(("PYTHON", "RUFF_")) or (name.startswith("GIT_") and name.endswith("_PATHSPECS"))


def clean_env(base: Mapping[str, str], path_dirs: Iterable[str]) -> dict[str, str]:
    """The cleaned environment: the caller's, minus what could change a verdict, plus the pinned settings.

    The caller's PATH follows the cache bins only when it is non-empty: an empty entry means the current directory.
    """
    env = {name: value for name, value in base.items() if not _dropped(name)}
    inherited = base.get("PATH", "")
    env["PATH"] = os.pathsep.join([*path_dirs, *([inherited] if inherited else [])])
    env.update(PINNED)
    return env


def row_env(clean: Mapping[str, str], *, tier: str, row: str, run_dir: str, row_dir: str) -> dict[str, str]:
    return {**clean, "GATE_TIER": tier, "GATE_ROW": row, "GATE_RUN_DIR": run_dir, "GATE_ROW_DIR": row_dir}
