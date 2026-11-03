"""The cover profile `go test -coverprofile` writes: parsed strictly, duplicate blocks folded (plan check S6).

    mode: set|count|atomic
    <import path>/<file>.go:<line>.<col>,<line>.<col> <statements> <count>

A block that appears twice keeps the higher count, so a block is covered if any test binary covered it; the
same block with two different statement counts is a broken profile, as it is to `go tool cover`. Whatever this
module cannot read is an error naming the line, never a profile with fewer blocks.
"""

import re
from dataclasses import dataclass

MODES = frozenset({b"set", b"count", b"atomic"})
# A bytes pattern, so [0-9] and \d stay ASCII. The file is everything before the last ':' that fits.
_BLOCK = re.compile(rb"(.+/[^/]+):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+) ([0-9]+) ([0-9]+)")


class ProfileError(ValueError):
    """The profile cannot be read."""


@dataclass(frozen=True, order=True)
class Block:
    file: str  # <import path>/<file name>
    start_line: int
    start_col: int
    end_line: int
    end_col: int
    statements: int
    count: int

    @property
    def package(self) -> str:
        return self.file.rpartition("/")[0]

    @property
    def name(self) -> str:
        return self.file.rpartition("/")[2]


def parse(data: bytes) -> list[Block]:
    """Every block once, sorted; a duplicate keeps the higher count."""
    lines = data.split(b"\n")
    if lines[-1] == b"":
        lines.pop()
    if not lines or not lines[0].startswith(b"mode: ") or lines[0][len(b"mode: ") :] not in MODES:
        raise ProfileError("the first line is not `mode: set`, `mode: count` or `mode: atomic`")
    blocks: dict[tuple[str, int, int, int, int], Block] = {}
    for number, line in enumerate(lines[1:], start=2):
        match = _BLOCK.fullmatch(line)
        if match is None:
            raise ProfileError(f"line {number}: not a profile block: {line[:120].decode('utf-8', errors='replace')}")
        file = match.group(1).decode("utf-8", errors="replace")
        sl, sc, el, ec, statements, count = (int(group) for group in match.groups()[1:])
        block = Block(file, sl, sc, el, ec, statements, count)
        key = (file, sl, sc, el, ec)
        earlier = blocks.get(key)
        if earlier is not None and earlier.statements != statements:
            raise ProfileError(
                f"line {number}: {file}:{sl}.{sc},{el}.{ec} has {statements} statements, "
                f"but an earlier line gives it {earlier.statements}"
            )
        if earlier is None or count > earlier.count:
            blocks[key] = block
    return sorted(blocks.values())
