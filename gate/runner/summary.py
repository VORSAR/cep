"""The summary (design note §4): rows in registry order, byte-exact under an injected clock."""

import os
from collections.abc import Sequence
from dataclasses import dataclass

from gate.runner.records import one_line

HEADER = ("job", "step", "verdict", "time", "reason")


@dataclass(frozen=True)
class Line:
    job: str
    step: str
    verdict: str  # PASS, FAIL or WARN
    ms: int
    reason: str


def tenths(ms: int) -> str:
    """Milliseconds as tenths of a second, truncated, never rounded up."""
    return f"{ms // 1000}.{ms % 1000 // 100}s"


def footer(wall_ms: int, summed_ms: int) -> str:
    saved = (summed_ms - wall_ms) * 100 // summed_ms if summed_ms else 0
    return f"wall {tenths(wall_ms)} · summed {tenths(summed_ms)} · saved {saved}%"


def shown(path: str, repo: str) -> str:
    """An absolute path as the summary shows it: relative to the repository when inside it, else whole."""
    rel = os.path.relpath(path, repo)
    return path if rel == os.pardir or rel.startswith(os.pardir + os.sep) else rel


def render(
    lines: Sequence[Line],
    *,
    wall_ms: int,
    summed_ms: int,
    exit_code: int,
    run_dir: str,
    registry: str | None = None,
    image: str | None = None,
) -> str:
    """The whole summary. Each reason is one bounded line (review BA5); the last column is not padded, so no line
    carries trailing spaces. `registry` is named only when it is not the repository's own (review BA8). `run_dir`,
    where every row's own output is, is always named (owner, 2026-11-03; review BA3). `image` is CI's runner image,
    which the owner asked every CI summary to record (sprint-plan §5, 2026-11-02)."""
    cells = [HEADER, *((x.job, x.step, x.verdict, tenths(x.ms), one_line(x.reason)) for x in lines)]
    widths = [max(len(row[i]) for row in cells) for i in range(len(HEADER) - 1)]
    table = [" | ".join([*map(str.ljust, row[:-1], widths, strict=True), row[-1]]).rstrip() for row in cells]
    return "\n".join(
        [
            *table,
            *([] if image is None else [f"image: {image}"]),
            *([] if registry is None else [f"registry: {registry}"]),
            f"run: {run_dir}",
            footer(wall_ms, summed_ms),
            "GATE PASSED" if exit_code == 0 else "GATE FAILED",
            f"exit: {exit_code}",
            "",
        ]
    )
