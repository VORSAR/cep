"""The GATE_COVERED line (design note §4; QA DQ14): parsed from bytes, because str.splitlines and \\d lie."""

import re
from dataclasses import dataclass

PREFIX = b"GATE_COVERED"
_LINE = re.compile(rb"GATE_COVERED ([1-9][0-9]{0,8})")
_ZERO = b"GATE_COVERED 0"


@dataclass(frozen=True)
class Covered:
    """A row's own count; `problem` is empty when the count is usable, otherwise it is the verdict's reason."""

    count: int = 0
    problem: str = ""


def parse(stdout: bytes) -> Covered:
    """Find the one GATE_COVERED line. Lines end at b"\\n" only; there is no other line ending here."""
    lines = [line for line in stdout.split(b"\n") if line.startswith(PREFIX)]
    if not lines:
        return Covered(problem="GATE_COVERED missing")
    if len(lines) > 1:
        return Covered(problem="GATE_COVERED duplicated")
    if lines[0] == _ZERO:
        return Covered(problem="GATE_COVERED 0")
    match = _LINE.fullmatch(lines[0])
    if match is None:
        return Covered(problem="GATE_COVERED malformed")
    return Covered(count=int(match.group(1)))
