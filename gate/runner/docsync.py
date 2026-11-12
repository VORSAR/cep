"""docsync (a), pure (S1.1c-2 design §4.1, §11.7-8; ADR-0015 §2): every Go package is listed in ARCHITECTURE §3.

`tree` reads §3's tree: the first fenced block after the `3.` heading, within §3 (a first fence under §4 is no fence
for §3), the headings read outside fences through `anchors.prose`. `listed` reads the tree's first column — the text
before the first run of two spaces, split on ", " — as directories: `x/` names `x`; a .go file names its directory
(`cep.go` names `.`); any other file names nothing (`tools/importcheck/rules.json` is the layer law, not a listing of
`tools/importcheck`, BK-L1); a path holding `*` is a placeholder and names nothing. `missing` is the packages no
line names, exactly: a package below a listed directory must be listed itself (§11.7).
"""

import posixpath
from collections.abc import Iterable

from gate.runner import anchors

SECTION = "3. "


def _level(line: str) -> int:
    return len(line.lstrip(" ")) - len(line.lstrip(" ").lstrip("#"))


def tree(md: str) -> list[str]:
    """The lines inside §3's first fence; ValueError when there is no §3 heading or §3 holds no fence."""
    lines = md.splitlines()
    prose = dict(anchors.prose(lines))
    start = level = 0
    for number, text in prose.items():
        m = anchors.HEADING.match(text)
        if m and (m.group(1) or "").startswith(SECTION):
            start, level = number, _level(text)
            break
    if not start:
        raise ValueError("no heading `3. …` outside a fence")
    for number in range(start + 1, len(lines) + 1):
        text = lines[number - 1]
        if number in prose:
            m = anchors.HEADING.match(text)
            if m and _level(text) <= level:
                break
            continue
        fence = anchors.FENCE.match(text)
        if fence is None:
            break  # inside a fence that opened before §3's heading: cannot happen, the heading would not be prose
        mark, body = fence.group(1), []
        for inner in lines[number:]:
            close = anchors.FENCE.match(inner)
            if close and close.group(1)[0] == mark[0] and len(close.group(1)) >= len(mark):
                break
            body.append(inner)
        return body
    raise ValueError("§3 holds no fenced tree")


def listed(lines: Iterable[str]) -> set[str]:
    """The directories the tree's first column names."""
    found: set[str] = set()
    for line in lines:
        column = line.split("  ", 1)[0].strip()
        for item in column.split(", "):
            item = item.strip()
            if not item or "*" in item:
                continue
            if item.endswith("/"):
                found.add(item.rstrip("/"))
                continue
            name = posixpath.basename(item)
            if name.endswith(".go"):
                found.add(posixpath.dirname(item) or ".")  # a Go file names its package's directory
            elif "." not in name.lstrip("."):
                found.add(item)  # a directory written without its slash (`.githooks`, `Makefile` alike)
            # any other file (rules.json, deps-allowlist.json, exports.golden) names no package (BK-L1)
    return found


def missing(packages: Iterable[str], names: set[str]) -> list[str]:
    """The packages the tree does not name, sorted."""
    return sorted(set(packages) - names)
