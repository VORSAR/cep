"""Heading anchors, as GitHub makes them, and a ref to one (S1.1c-2 design §2.1, §2.8).

`slug`, `prose` and `anchors` moved here verbatim from rows/doclinks.py, which imports them. A ref is
`<repository path>#<heading anchor>` (an allowlist entry's `ref`, a known issue's `decision`): `split_ref` reads its
form, `resolves` says whether a tracked Markdown file has the heading. Pure: `read` is injected and returns a tracked
Markdown file's text or None, so an untracked local doc never satisfies a ref (DA-L5).
"""

import re
import unicodedata
from collections.abc import Callable, Iterator

FENCE = re.compile(r"^ {0,3}(`{3,}|~{3,})")
HEADING = re.compile(r"^ {0,3}#{1,6}(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$")
HTML_ANCHOR = re.compile(r"<[A-Za-z][^>]*?\s(?:id|name)=\"([^\"]+)\"")


def slug(heading: str) -> str:
    """GitHub's anchor for a heading's text."""
    text = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", heading)  # a link or an image keeps its text
    text = re.sub(r"<[^>]*>", "", text)  # inline HTML
    text = re.sub(r"(?<!\w)_([^_]+)_(?!\w)", r"\1", text)  # _emphasis_; an intraword underscore stays
    out = []
    for ch in text.strip().lower():
        if ch == " ":
            out.append("-")
        elif ch in "-_" or unicodedata.category(ch)[0] in "LNM":
            out.append(ch)
    return "".join(out)


def prose(lines: list[str]) -> Iterator[tuple[int, str]]:
    """(line number, line) outside fenced blocks. A heading's code span is part of its anchor, so spans stay."""
    fence = ""
    for number, line in enumerate(lines, 1):
        m = FENCE.match(line)
        if m and (not fence or (m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence))):
            fence = "" if fence else m.group(1)
            continue
        if not fence:
            yield number, line


def anchors(text: str) -> set[str]:
    found: set[str] = set()
    seen: dict[str, int] = {}
    for _, line in prose(text.splitlines()):
        m = HEADING.match(line)
        if m and m.group(1):
            base = slug(m.group(1))
            n = seen.get(base, 0)
            seen[base] = n + 1
            found.add(base if n == 0 else f"{base}-{n}")
        found.update(HTML_ANCHOR.findall(line))
    return found


def split_ref(ref: str) -> tuple[str, str]:
    """(path, anchor); the anchor is "" when the ref has no "#" or nothing after it (design §10.1)."""
    if not ref:
        raise ValueError("ref is empty")
    if ref.startswith("/"):
        raise ValueError("ref must be a repository path, not absolute")
    if ref.count("#") > 1:
        raise ValueError("ref has more than one #")
    path, _, anchor = ref.partition("#")
    if ".." in path.split("/"):
        raise ValueError("ref must not climb out with ..")
    return path, anchor


def resolves(ref: str, read: Callable[[str], str | None]) -> str:
    """ "" when the ref names a heading of a tracked Markdown file; else why not. Never raises (design §10.3)."""
    try:
        path, anchor = split_ref(ref)
    except ValueError as e:
        return str(e)
    if not anchor:
        return f"ref has no anchor: write {path}#<heading-anchor>"
    text = read(path) if path.endswith(".md") else None
    if text is None:
        return f"{path} is not a tracked Markdown file"
    if anchor not in anchors(text):
        return f"{path} has no heading #{anchor}"
    return ""
