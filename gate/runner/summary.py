"""The summary (design note §4): rows in registry order, byte-exact under an injected clock."""

from collections.abc import Sequence
from dataclasses import dataclass

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


def render(lines: Sequence[Line], *, wall_ms: int, summed_ms: int, exit_code: int) -> str:
    """The whole summary. The last column is not padded, so no line carries trailing spaces."""
    cells = [HEADER, *((x.job, x.step, x.verdict, tenths(x.ms), x.reason) for x in lines)]
    widths = [max(len(row[i]) for row in cells) for i in range(len(HEADER) - 1)]
    table = [" | ".join([*map(str.ljust, row[:-1], widths, strict=True), row[-1]]).rstrip() for row in cells]
    return "\n".join(
        [
            *table,
            footer(wall_ms, summed_ms),
            "GATE PASSED" if exit_code == 0 else "GATE FAILED",
            f"exit: {exit_code}",
            "",
        ]
    )
