"""The doclinks row (owner, 2026-11-06): every relative link and #anchor in the tracked Markdown resolves.

For each listed file, every inline link or image `[text](target)` and every reference definition `[label]: target`
outside code spans and fenced blocks is read; a target with a scheme (https:, mailto:) is not ours to check. The file
part must name a file or directory git tracks, resolved from the linking file's directory; a template under
docs/sprints/templates/ resolves from docs/sprints/, where its copies land, and its <…> placeholders are skipped.
A #fragment into a Markdown file must be an anchor GitHub makes there: a heading's slug (lower case, punctuation
dropped, spaces as hyphens, a repeated heading numbered -1, -2, …) or an HTML id or name. Resolving against what git
tracks, not the disk, makes a local run and CI agree. A broken link fails the row naming file:line, the target and
why; GATE_COVERED counts the files read.
"""

import os
import re
import sys
import unicodedata
import urllib.parse

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

TEMPLATES = "docs/sprints/templates/"
FENCE = re.compile(r"^ {0,3}(`{3,}|~{3,})")
HEADING = re.compile(r"^ {0,3}#{1,6}(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$")
CODE_SPAN = re.compile(r"(`+).+?\1")
INLINE = re.compile(
    r"\[(?:[^\[\]\\]|\\.|\[[^\]]*\])*\]\(\s*(<[^>]*>|[^)\s]+)(?:\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\s*\)"
)
REFERENCE = re.compile(r"^ {0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)")
HTML_ANCHOR = re.compile(r"<[A-Za-z][^>]*?\s(?:id|name)=\"([^\"]+)\"")
SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:|^//")


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


def prose(lines: list[str]):
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


class Checker:
    def __init__(self, tracked: list[str]) -> None:
        self.files = set(tracked)
        self.dirs = {path[:i] for path in tracked for i, ch in enumerate(path) if ch == "/"}
        self.cache: dict[str, set[str]] = {}

    def anchors_of(self, path: str) -> set[str]:
        if path not in self.cache:
            self.cache[path] = anchors(rows.read_text(path))
        return self.cache[path]

    def check(self, rel: str) -> list[str] | None:
        """One line per broken link in `rel`; None when `rel` cannot be read."""
        template = rel.startswith(TEMPLATES)
        base = "docs/sprints" if template else os.path.dirname(rel)
        try:
            text = rows.read_text(rel)
        except OSError:
            return None
        broken = []
        for number, raw in prose(text.splitlines()):
            line = CODE_SPAN.sub("", raw)  # a link written inside code is an example, not a link
            targets = [m.group(1) for m in INLINE.finditer(line)]
            ref = REFERENCE.match(line)
            if ref:
                targets.append(ref.group(1))
            for target in targets:
                why = self.why(rel, base, target.removeprefix("<").removesuffix(">"), template)
                if why:
                    broken.append(f"{rel}:{number}: {target} ({why})")
        return broken

    def why(self, rel: str, base: str, target: str, template: bool) -> str:
        """Why `target` does not resolve, or "" when it does or is not ours to check."""
        if SCHEME.match(target) or (template and "<" in target):
            return ""
        part, _, fragment = target.partition("#")
        part, fragment = urllib.parse.unquote(part), urllib.parse.unquote(fragment)
        path = os.path.normpath(os.path.join(base, part)) if part else rel
        if path.startswith("..") or (path not in self.files and path not in self.dirs) or not os.path.exists(path):
            return "missing file"  # tracked, and in the work tree: a deleted file is missing however it is linked
        if fragment and path.endswith(".md") and path in self.files and fragment not in self.anchors_of(path):
            return "missing anchor"
        return ""


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    files = sorted(rows.show(path) for path in rows.read_list(argv[1]))
    out, err = os.path.join(row_dir, "ls-files.stdout"), os.path.join(row_dir, "ls-files.stderr")
    code = rows.run_tool(["git", "ls-files", "-z"], out, err)
    if code != 0:
        rows.covered(0)
        return rows.finish([f"git ls-files: exit {code}: {rows.read_text(err).strip()}"])
    checker = Checker([p for p in rows.read_text(out).split("\0") if p])
    failures: list[str] = []
    read = 0
    for rel in files:
        broken = checker.check(rel)
        if broken is None:
            failures.append(f"{rel}: listed but not in the work tree")
            continue
        read += 1
        failures += broken
    rows.covered(read)
    return rows.finish(failures)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
