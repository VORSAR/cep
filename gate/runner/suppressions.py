"""The in-code exemptions: Go's //nolint directives and generated-code markers, and Python's # fmt: skip markers
(S1.1c-2 design §2.3, §2.8; BK-S6).

Each recogniser follows what the pinned tool honours, as measured (design §0.2 for ruff 0.17.3, §0.3 for
golangci-lint v2.15.0), never what its documentation suggests; test_suppressions.py holds both tables as data. A Go
comment golangci reads as a directive must be written in the one canonical form with the grammar's metadata; a form
golangci ignores is not an exemption and passes unseen. A Python marker is `# fmt: skip  # <reason>`; every other
waiver form ruff honours (or that waives nothing under --ignore-noqa) is red. Every waiver a tool honours is returned
as a Directive, valid or not, so the LAW Δ count holds it (W1-F3). Pure: texts in, directives and faults out; the
field rules are allowlists.fields, the only implementation.
"""

import io
import re
import tokenize
from collections.abc import Iterable, Iterator
from dataclasses import dataclass

from gate.runner import allowlists

ENABLED: frozenset[str] = frozenset()  # S1.1c-5 sets it from its golangci config (DA-L4)
# git grep -e patterns, case-sensitive as git grep is (design §10.12); every recognised form contains one. "fmt:"
# covers every spelling ruff reads after it (BK-B1).
PREFILTER = ("nolint", "fmt:", "noqa", "yapf:", "isort:", "Code generated")
KEYS = frozenset({"reason", "ref", "expiry", "issue", "sprint"})
SELF_JUDGED = frozenset({"MARKER", "REGION", "GENERATED"})  # judged when read (Python) or by the row (GENERATED)
# Go's generated-code rule (go/ast.IsGenerated): a comment line before the package clause, this prefix, then this
# suffix ending the line (BK-S6; owner, 2026-11-12, S1.1c-2's review and break-it 3).
GENERATED = ("// Code generated ", " DO NOT EDIT.")

HONOURED = re.compile(r"nolint(?: |:|$)")
CANONICAL = re.compile(r"nolint:([\w-]+(?:,[\w-]+)*)(?=\s|$)", re.ASCII)
KIND = re.compile(r"[A-Z]+")
KEY = re.compile(r"([a-z]+)=")
SKIPS = ("fmt: skip", "fmt:skip")
COMMANDS = ("fmt", "yapf")  # ruff reads `<tool>:` + any run of spaces, tabs or form feeds + the command (§0.2, BK-B1)
RUFF_SPACE = " \t\f"
REGION = re.compile(r"(fmt:[ \t\f]*(off|on)|yapf:[ \t\f]*(disable|enable)|isort: ?\w+|ruff: ?isort: ?\w+)\b", re.ASCII)
NOQA = re.compile(r"((ruff|flake8): ?)?noqa\b", re.ASCII | re.IGNORECASE)

METADATA = 'a suppression carries its metadata: // PERMANENT reason="…" ref=… sprint=…'
RUFF_IGNORES = "ruff ignores this form; write # fmt: skip  # <reason>"
SPELLING = "ruff honours this spelling too; write # fmt: skip  # <reason>"
UNTOKENIZABLE = "cannot be tokenized as Python"
REGION_WAIVERS = frozenset({("fmt", "off"), ("yapf", "disable")})
ISORT_WAIVER = re.compile(r"(?:ruff: ?)?isort: ?(skip|skip_file|off)", re.ASCII)
NO_REASON = f"write its reason as another comment: # fmt: skip  # <reason, {allowlists.REASON_MIN}+ characters>"
ON_STATEMENT = "write # fmt: skip  # <reason> on the statement"
WAIVES_NOTHING = "ruff runs --ignore-noqa: this comment waives nothing; remove it"


@dataclass(frozen=True)
class Directive:
    """One waiver a pinned tool honours, valid or not (W1-F3: the LAW Δ count holds every one; the reader's fault
    names what is wrong with an invalid one, and `check` adds nothing for it)."""

    path: str
    line: int
    linters: tuple[str, ...]  # ("fmt",) for a skip marker; the tool ("fmt", "yapf", "isort") for a region waiver
    kind: str  # PERMANENT or TEMP (or "" unread) for //nolint; MARKER for a skip; REGION for another Python waiver
    meta: dict[str, str]
    valid: bool = True


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
            written = _linters_as_written(head)
            faults.append(f"{where}: write //nolint:{written} (two slashes, no space)")
            directives.append(Directive(path, line, tuple(written.split(",")), "", {}, valid=False))
            continue
        linters = tuple(m.group(1).split(","))
        rest = text[m.end() :]
        read = _metadata(rest.removeprefix(" // ")) if rest.startswith(" // ") else METADATA
        if isinstance(read, str):
            faults.append(f"{where}: {read}")
            directives.append(Directive(path, line, linters, "", {}, valid=False))
            continue
        kind, meta = read
        directives.append(Directive(path, line, linters, kind, meta))
    return directives, faults


def _generator_line(text: str) -> bool:
    prefix, suffix = GENERATED
    return text.startswith(prefix) and text.removeprefix(prefix).endswith(suffix)


def go_generated(path: str, source: str) -> list[Directive]:
    """The generated-code marker as Go reads it (go/ast.IsGenerated): a line `// Code generated <generator> DO NOT
    EDIT.` in a comment before the package clause — a line comment (its trailing carriage return stripped, as the Go
    scanner does) or a line of a block comment. One directive of kind GENERATED, or none; the row requires the file
    listed in tools/allowlists/generated.json (BK-S6)."""
    i, n, line = 0, len(source), 1
    while i < n:
        if source[i] == "\n":
            line += 1
            i += 1
        elif source[i] in " \t\r\f":
            i += 1
        elif source.startswith("//", i):
            end = source.find("\n", i)
            end = n if end < 0 else end
            if _generator_line(source[i:end].removesuffix("\r")):
                return [Directive(path, line, ("generated",), "GENERATED", {})]
            i = end
        elif source.startswith("/*", i):
            end = source.find("*/", i + 2)
            end = n if end < 0 else end + 2
            for k, text in enumerate(source[i:end].split("\n")):
                if _generator_line(text.replace("\r", "")):
                    return [Directive(path, line + k, ("generated",), "GENERATED", {})]
            line += source.count("\n", i, end)
            i = end
        else:
            return []  # the package clause (or anything else): a marker after it is no marker
    return []


# --------------------------------------------------------------------------------------------------------- Python


def _command(segment: str) -> tuple[str, str] | None:
    """(tool, command) for a segment ruff reads as a formatter command: `fmt:` or `yapf:`, any run of spaces, tabs or
    form feeds, then the rest (measured, §0.2 and BK-B1); else None."""
    for tool in COMMANDS:
        if segment.startswith(f"{tool}:"):
            return tool, segment[len(tool) + 1 :].lstrip(RUFF_SPACE)
    return None


def _segment_faults(segment: str) -> str:
    """The fault one comment segment carries by itself, or ""."""
    if NOQA.match(segment):
        return WAIVES_NOTHING
    if segment in SKIPS:
        return ""
    if _command(segment) == ("fmt", "skip"):  # a skip ruff honours, spelled another way
        return SPELLING
    lowered = _command(segment.lower())
    if lowered is not None and lowered[0] == "fmt" and lowered[1].startswith("skip"):
        return RUFF_IGNORES  # continued (ruff reformats it) or a case variant ruff ignores
    if REGION.match(segment):
        return ON_STATEMENT
    return ""


def _is_skip(segment: str) -> bool:
    """A segment ruff reads as `fmt: skip`, in any spelling it honours (BK-B1)."""
    return segment in SKIPS or _command(segment) == ("fmt", "skip")


def _regions(path: str, line: int, segments: list[str]) -> list[Directive]:
    """The comment's other waivers ruff honours (each red, each counted, W1-F3): `fmt: off`, `yapf: disable` in any
    spelling, and the isort skips (measured: `ruff check --select I --ignore-noqa` honours them)."""
    out = []
    for segment in segments:
        command = _command(segment)
        if command in REGION_WAIVERS:
            out.append(Directive(path, line, (command[0],), "REGION", {"command": command[1]}, valid=False))
        elif m := ISORT_WAIVER.fullmatch(segment):
            out.append(Directive(path, line, ("isort",), "REGION", {"command": m.group(1)}, valid=False))
    return out


def py_markers(path: str, source: str) -> tuple[list[Directive], list[str]]:
    """Every waiver ruff honours as a directive, valid or not (W1-F3), and every waiver form but a valid
    `# fmt: skip  # <reason>` (§0.2) as a fault.

    Python's tokenizer gives one COMMENT token per line, so the token is split on "#" into stripped segments: a
    segment that is a skip marks the line, and the others, noqa ones aside, are its reason."""
    markers: list[Directive] = []
    faults: list[str] = []
    try:
        tokens = [t for t in tokenize.generate_tokens(io.StringIO(source).readline) if t.type == tokenize.COMMENT]
    except (SyntaxError, tokenize.TokenError) as e:
        return [], [f"{path}: {UNTOKENIZABLE}: {e}"]
    for token in tokens:
        line = token.start[0]
        where = f"{path}:{line}"
        segments = [s.strip() for s in token.string.split("#")[1:]]
        own = [fault for fault in map(_segment_faults, segments) if fault]
        faults += [f"{where}: {fault}" for fault in own]
        markers += _regions(path, line, segments)
        skips = [s for s in segments if _is_skip(s)]
        if not skips:
            continue
        reason = "  # ".join(s for s in segments if s and not _is_skip(s) and not NOQA.match(s))
        spelled = all(s in SKIPS for s in skips)  # a skip spelled otherwise has its fault already
        short = len(reason.strip()) < allowlists.REASON_MIN
        if spelled and short:
            faults.append(f"{where}: {NO_REASON}")
        markers.append(Directive(path, line, ("fmt",), "MARKER", {"reason": reason}, valid=spelled and not short))
    return markers, faults


# ---------------------------------------------------------------------------------------------------------- check


def check(directives: Iterable[Directive], ctx: allowlists.Ctx, enabled: frozenset[str]) -> list[str]:
    """The field rules for each valid Go directive, then each of its linters not enabled; a Python waiver, and a
    directive whose reader faulted it, were judged when they were read."""
    out: list[str] = []
    for d in directives:
        if d.kind in SELF_JUDGED or not d.valid:
            continue
        where = f"{d.path}:{d.line}"
        got = d.meta.get
        faults = allowlists.fields(
            d.kind, got("reason", ""), got("ref", ""), got("expiry", ""), got("issue", ""), got("sprint", ""), ctx
        )
        out += [f"{where}: {fault}" for fault in faults]
        out += [
            f"{where}: //nolint:{linter}: {linter} is not enabled (none is until S1.1c-5)"
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
            directives, faults = go_directives(path, source)
            return [*go_generated(path, source), *directives], faults
        case "py":
            return py_markers(path, source)
        case _:
            return [], []


def count(path: str, source: str) -> int:
    """How many in-code exemptions the file holds (the LAW Δ block's `<n> in code`): every waiver the tools honour,
    valid or not (W1-F3)."""
    return len(read(path, source)[0])
