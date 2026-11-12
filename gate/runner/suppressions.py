"""The in-code exemptions: Go's //nolint directives and Python's # fmt: skip markers (S1.1c-2 design §2.3, §2.8).

Each recogniser follows what the pinned tool honours, as measured (design §0.2 for ruff 0.17.3, §0.3 for
golangci-lint v2.15.0), never what its documentation suggests; test_suppressions.py holds both tables as data. A Go
comment golangci reads as a directive must be written in the one canonical form with the grammar's metadata; a form
golangci ignores is not an exemption and passes unseen. A Python marker is `# fmt: skip  # <reason>`; every other
waiver form ruff honours (or that waives nothing under --ignore-noqa) is red. Pure: texts in, directives and faults
out; the field rules are allowlists.fields, the only implementation.
"""

import io
import re
import tokenize
from collections.abc import Iterable, Iterator
from dataclasses import dataclass

from gate.runner import allowlists

ENABLED: frozenset[str] = frozenset()  # S1.1c-4 sets it from its golangci config (DA-L4)
# git grep -e patterns, case-sensitive as git grep is (design §10.12); every recognised form contains one.
PREFILTER = ("nolint", "fmt: skip", "fmt:skip", "noqa", "yapf:", "isort:", "fmt: off", "fmt: on")
KEYS = frozenset({"reason", "ref", "expiry", "issue", "sprint"})

HONOURED = re.compile(r"nolint(?: |:|$)")
CANONICAL = re.compile(r"nolint:([\w-]+(?:,[\w-]+)*)(?=\s|$)", re.ASCII)
KIND = re.compile(r"[A-Z]+")
KEY = re.compile(r"([a-z]+)=")
SKIPS = ("fmt: skip", "fmt:skip")
REGION = re.compile(r"(fmt: ?(off|on)|yapf: ?(disable|enable)|isort: ?\w+|ruff: ?isort: ?\w+)\b", re.ASCII)
NOQA = re.compile(r"((ruff|flake8): ?)?noqa\b", re.ASCII | re.IGNORECASE)

METADATA = 'a suppression carries its metadata: // PERMANENT reason="…" ref=… sprint=…'
RUFF_IGNORES = "ruff ignores this form; write # fmt: skip  # <reason>"
NO_REASON = f"write its reason as another comment: # fmt: skip  # <reason, {allowlists.REASON_MIN}+ characters>"
ON_STATEMENT = "write # fmt: skip  # <reason> on the statement"
WAIVES_NOTHING = "ruff runs --ignore-noqa: this comment waives nothing; remove it"


@dataclass(frozen=True)
class Directive:
    path: str
    line: int
    linters: tuple[str, ...]  # ("fmt",) for a Python marker
    kind: str  # PERMANENT, TEMP, or MARKER for a Python marker
    meta: dict[str, str]


# ------------------------------------------------------------------------------------------------------------- Go


def go_comments(source: str) -> Iterator[tuple[int, str]]:
    """(line, the text after the first "//") for each line comment; block comments, strings, raw strings and runes
    are stepped over. States: CODE, LINE, BLOCK, STRING (ends at an unescaped " or a newline), RAW, RUNE."""
    i, n, line = 0, len(source), 1
    while i < n:
        ch = source[i]
        if ch == "\n":
            line += 1
            i += 1
        elif source.startswith("//", i):  # LINE
            end = source.find("\n", i)
            end = n if end < 0 else end
            yield line, source[i + 2 : end]
            i = end
        elif source.startswith("/*", i):  # BLOCK
            end = source.find("*/", i + 2)
            end = n if end < 0 else end + 2
            line += source.count("\n", i, end)
            i = end
        elif ch == "`":  # RAW
            end = source.find("`", i + 1)
            end = n if end < 0 else end + 1
            line += source.count("\n", i, end)
            i = end
        elif ch in "\"'":  # STRING, RUNE
            i += 1
            while i < n and source[i] != ch and source[i] != "\n":
                i += 2 if source[i] == "\\" and i + 1 < n and source[i + 1] != "\n" else 1
            if i < n and source[i] == ch:
                i += 1
        else:  # CODE
            i += 1


def _linters_as_written(head: str) -> str:
    """The linters a non-canonical directive names after "nolint:", for the advice; else "<linter>" (§10.8)."""
    if not head.startswith("nolint:"):
        return "<linter>"
    written = head.removeprefix("nolint:").split(maxsplit=1)
    linters = written[0].strip(",") if written else ""
    return linters or "<linter>"


def _metadata(text: str) -> tuple[str, dict[str, str]] | str:
    """(kind, meta) from `<KIND> key=value …`, a reason double-quoted; else the fault's text."""
    m = KIND.match(text)
    if m is None or (m.end() < len(text) and text[m.end()] != " "):
        return METADATA
    kind, meta, i, n = m.group(0), {}, m.end(), len(text)
    while True:
        while i < n and text[i] == " ":
            i += 1
        if i >= n:
            return kind, meta
        m = KEY.match(text, i)
        if m is None:
            return METADATA
        key, i = m.group(1), m.end()
        if i < n and text[i] == '"':
            value, i = [], i + 1
            while i < n and text[i] != '"':
                if text[i] == "\\" and i + 1 < n:
                    i += 1
                value.append(text[i])
                i += 1
            if i >= n:
                return "unterminated quote"
            i += 1
            if i < n and text[i] != " ":
                return METADATA
            got = "".join(value)
        else:
            if key == "reason":
                return METADATA
            end = text.find(" ", i)
            end = n if end < 0 else end
            got, i = text[i:end], end
        if key not in KEYS:
            return f"unknown key {key}"
        if key in meta:
            return f"duplicate key {key}"
        meta[key] = got


def go_directives(path: str, source: str) -> tuple[list[Directive], list[str]]:
    """The //nolint directives golangci would honour (§0.3): each canonical one with its metadata read, each other
    one a fault. A comment golangci ignores (//NOLINT, a tab, text before it, a block comment) is neither."""
    directives: list[Directive] = []
    faults: list[str] = []
    for line, text in go_comments(source):
        head = text.lstrip("/ ")
        if not HONOURED.match(head):
            continue
        where = f"{path}:{line}"
        m = CANONICAL.match(text)
        if m is None:
            faults.append(f"{where}: write //nolint:{_linters_as_written(head)} (two slashes, no space)")
            continue
        rest = text[m.end() :]
        if not rest.startswith(" // "):
            faults.append(f"{where}: {METADATA}")
            continue
        read = _metadata(rest.removeprefix(" // "))
        if isinstance(read, str):
            faults.append(f"{where}: {read}")
            continue
        kind, meta = read
        directives.append(Directive(path, line, tuple(m.group(1).split(",")), kind, meta))
    return directives, faults


# --------------------------------------------------------------------------------------------------------- Python


def _segment_faults(segment: str) -> str:
    """The fault one comment segment carries by itself, or ""."""
    if NOQA.match(segment):
        return WAIVES_NOTHING
    if segment in SKIPS:
        return ""
    if segment.lower().startswith(SKIPS):  # continued (ruff reformats it) or a case variant ruff ignores
        return RUFF_IGNORES
    if REGION.match(segment):
        return ON_STATEMENT
    return ""


def py_markers(path: str, source: str) -> tuple[list[Directive], list[str]]:
    """The `# fmt: skip  # <reason>` markers (§0.2) as directives, and every other waiver form as a fault.

    Python's tokenizer gives one COMMENT token per line, so the token is split on "#" into stripped segments: a
    segment that is exactly a skip marks the line, and the others, noqa ones aside, are its reason."""
    markers: list[Directive] = []
    faults: list[str] = []
    try:
        tokens = [t for t in tokenize.generate_tokens(io.StringIO(source).readline) if t.type == tokenize.COMMENT]
    except (SyntaxError, tokenize.TokenError) as e:
        return [], [f"{path}: cannot be tokenized as Python: {e}"]
    for token in tokens:
        where = f"{path}:{token.start[0]}"
        segments = [s.strip() for s in token.string.split("#")[1:]]
        own = [fault for fault in map(_segment_faults, segments) if fault]
        faults += [f"{where}: {fault}" for fault in own]
        if not any(s in SKIPS for s in segments):
            continue
        reason = "  # ".join(s for s in segments if s and s not in SKIPS and not NOQA.match(s))
        if len(reason.strip()) < allowlists.REASON_MIN:
            faults.append(f"{where}: {NO_REASON}")
            continue
        markers.append(Directive(path, token.start[0], ("fmt",), "MARKER", {"reason": reason}))
    return markers, faults


# ---------------------------------------------------------------------------------------------------------- check


def check(directives: Iterable[Directive], ctx: allowlists.Ctx, enabled: frozenset[str]) -> list[str]:
    """The field rules for each Go directive, then each of its linters not enabled; a marker's reason was checked
    when it was read."""
    out: list[str] = []
    for d in directives:
        if d.kind == "MARKER":
            continue
        where = f"{d.path}:{d.line}"
        got = d.meta.get
        faults = allowlists.fields(
            d.kind, got("reason", ""), got("ref", ""), got("expiry", ""), got("issue", ""), got("sprint", ""), ctx
        )
        out += [f"{where}: {fault}" for fault in faults]
        out += [
            f"{where}: //nolint:{linter}: {linter} is not enabled (none is until S1.1c-4)"
            for linter in d.linters
            if linter not in enabled
        ]
    return out


def scoped(path: str) -> str:
    """ "go" or "py" for a file whose exemptions are read (tracked `*.go`; `gate/**/*.py` and `.githooks/*`, DA-L1),
    else ""."""
    if path.endswith(".go"):
        return "go"
    if (path.startswith("gate/") and path.endswith(".py")) or (path.startswith(".githooks/") and "/" not in path[10:]):
        return "py"
    return ""


def read(path: str, source: str) -> tuple[list[Directive], list[str]]:
    """The file's directives or markers, and its faults, by `scoped`; ([], []) for a file out of scope."""
    match scoped(path):
        case "go":
            return go_directives(path, source)
        case "py":
            return py_markers(path, source)
        case _:
            return [], []


def count(path: str, source: str) -> int:
    """How many in-code exemptions the file holds (the LAW Δ block's `<n> in code`); a fault is not one."""
    return len(read(path, source)[0])
