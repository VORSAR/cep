"""The plan check's jargon sweep (S1.1c-7 plan check section 7.1, K6) over the rendered doc comments of Go files.

    python3.15 -I -B jargon_sweep.py [--all-comments] <file.go>...

A rendered comment is one go doc shows: the package doc (the // block right before the package clause), the doc
block right before a top-level declaration whose name is exported (func, method, type, var, const, or a var/const/type
group), the doc block or trailing comment of an exported struct field or interface method, and the doc of an Example
function in a _test.go file (pkg.go.dev renders it). A blank line between a block and its declaration unattaches it,
as in Go. --all-comments sweeps every // line instead, for a whole-file view.

Each hit prints `<file>:<line>: <pattern id>: <matched text> | <line text>`; exit 1 on any hit, 0 on none.
The patterns are section 7.1's list and every sprint-id form section 3 gives, written for prose (S5.1, S3.3-1,
S1.1c-7, S3.2b-2; design s3-3-1; s3_3_1 and s1c7 tokens). EXCEPTIONS lists matched texts that are not jargon.
"""

import re
import sys

PATTERNS = [
    # section 3's sprint ids, in the prose forms (S5.1, S3.3-1, S1.1c-7, S3.2b-2, S3.3-1-3) ...
    ("sprint id", re.compile(r"\bS\d+(?:\.\d+[a-z]?)+(?:-\d+)*\b")),
    # ... the design-file form (s3-3-1, s3-2b-2, s1-1c-7) ...
    ("sprint id (file form)", re.compile(r"\bs\d+-\d+[a-z]?(?:-\d+)*\b")),
    # ... and section 3's token forms (s3_3_1, s1c7_2), as identifiers carry them.
    ("sprint id (token form)", re.compile(r"\bs\d+_\d+[a-z]?(?:_\d+)*\b|\bs\d+c\d+(?:_\d+)*\b")),
    ("owner", re.compile(r"\bowner\b")),
    ("design n", re.compile(r"\bdesign \d")),
    ("decision n", re.compile(r"\bdecision \d")),
    ("break-it", re.compile(r"break-it")),
    ("review", re.compile(r"\breview\b")),
    ("review id", re.compile(r"\b[A-Z]{1,2}\d+\b")),
    ("parenthesised date", re.compile(r"\(\s*[^)]*\d{4}-\d{2}-\d{2}")),
    # Beyond section 7.1's list: a design's numbered requirement ("MUST 14"), which G7a-2 names at input.go:29-30.
    ("design requirement number", re.compile(r"\bMUST \d+\b")),
]

# Matched texts that are not jargon: a standard's name that the review-id pattern could read as an id.
EXCEPTIONS = {"UTF8", "RFC3339", "ISO8601", "CEP01"}

_DECL = re.compile(r"^(?:func\s+(?:\([^)]*\)\s*)?|type\s+|var\s+|const\s+)([A-Za-z_]\w*)")
_GROUP = re.compile(r"^(?:var|const|type)\s*\($")
_FIELD = re.compile(r"^\s+([A-Za-z_]\w*)")


def exported(name: str) -> bool:
    return name[:1].isupper()


def rendered_lines(lines: list[str], test_file: bool) -> list[int]:
    """The 1-based line numbers of the comment lines go doc renders. In a _test.go file only an Example's doc renders
    (on pkg.go.dev); a Test's doc, the package line and everything else there is not public text."""
    shown = (lambda name: name.startswith("Example")) if test_file else exported
    out: list[int] = []
    i = 0
    n = len(lines)
    while i < n:
        s = lines[i].strip()
        if s.startswith("//"):
            start = i
            while i < n and lines[i].strip().startswith("//"):
                i += 1
            if i < n:
                nxt = lines[i]
                if not test_file and (nxt.startswith("package ") or _GROUP.match(nxt.strip())):
                    out.extend(range(start + 1, i + 1))
                else:
                    m = _DECL.match(nxt) or _FIELD.match(nxt)
                    if m and shown(m.group(1)):
                        out.extend(range(start + 1, i + 1))
            continue
        m = _FIELD.match(lines[i]) or _DECL.match(lines[i])
        if m and shown(m.group(1)) and "//" in lines[i]:
            out.append(i + 1)  # a trailing comment on an exported declaration or field
        i += 1
    return out


def sweep(path: str, every: bool) -> int:
    with open(path, encoding="utf-8") as f:
        lines = f.read().split("\n")
    if every:
        numbers = [i + 1 for i, text in enumerate(lines) if "//" in text]
    else:
        numbers = rendered_lines(lines, path.endswith("_test.go"))
    hits = 0
    for num in numbers:
        line = lines[num - 1]
        comment = line[line.index("//") :] if "//" in line else line
        taken: list[tuple[int, int]] = []  # spans already reported: "S3" inside "S3.2b-2" is one hit, not two
        for pid, rx in PATTERNS:
            for m in rx.finditer(comment):
                if m.group(0) in EXCEPTIONS or any(a <= m.start() and m.end() <= b for a, b in taken):
                    continue
                taken.append(m.span())
                print(f"{path}:{num}: {pid}: {m.group(0)} | {line.strip()}")
                hits += 1
    return hits


def main(argv: list[str]) -> int:
    every = "--all-comments" in argv
    paths = [a for a in argv if a != "--all-comments"]
    if not paths:
        print(__doc__.strip().split("\n")[2].strip(), file=sys.stderr)
        return 2
    total = sum(sweep(p, every) for p in paths)
    print(f"jargon sweep: {len(paths)} file(s), {total} hit(s)")
    return 1 if total else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
