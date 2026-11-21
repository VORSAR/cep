"""The naming tool: history names in tests, report only (S1.1c-7 plan check §3; design W0, Part B).

    python3.15 -I -B gate/runner/testnames.py [--json <out>] [<path>...]

A pure matcher, names in and verdicts out. Each name is normalised to a run of lowercase tokens (split on `_`, `-`
and `.`, and at lower->Upper, Upper->Upper-lower and digit->Upper), and a rule of the word list gate/testnames.json
matches a whole run of tokens. The names are those of test files: the file's own name, Go top-level functions and
methods and the types, vars and consts of top-level declarations (grouped and multi-name included); Python classes
and their methods, top-level functions and module-level names. Subtest strings stay manual.

With no paths, `main` reads every tracked `*_test.go` and `test_*.py` outside `testdata` (from the working
directory's git index). It prints one line per hit, sorted, then a count line, and exits 0 with or without hits:
S1.1c-8 registers the row and adds the failing exit. A file or word list it cannot read, a Python file that does not
parse and a git failure exit 2, with `error: ...` as the last line.

Standard library only and no `gate` import, so it stays a leaf.
"""

import ast
import contextlib
import dataclasses
import fnmatch
import json
import os
import re
import signal
import subprocess
import sys
import tempfile
import warnings
from collections.abc import Sequence

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
WORDS = os.path.join(REPO, "gate", "testnames.json")
USAGE = "usage: testnames.py [--json <out>] [<path>...]"
GIT_CEILING_S = 60
GIT_GRACE_S = 5

_SPLIT = re.compile(r"[_\-.]+")
_CASE = re.compile(r"(?<=[a-z])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])|(?<=[0-9])(?=[A-Z])")
_IDENT = re.compile(r"[^\W\d]\w*")
_MORE = re.compile(r"\s*,\s*([^\W\d]\w*)")
_DECLS = ("type", "var", "const")
_OPEN, _CLOSE = "([{", ")]}"


class Fault(Exception):
    """Something the run cannot read, named by where it is: exit 2."""

    def __init__(self, where: str, what: str) -> None:
        super().__init__(f"{where}: {what}")
        self.where = where


@dataclasses.dataclass(frozen=True)
class Rule:
    id: str
    rx: re.Pattern[str]


@dataclasses.dataclass(frozen=True)
class Name:
    path: str
    kind: str
    name: str
    line: int


@dataclasses.dataclass(frozen=True)
class Hit:
    name: Name
    rules: tuple[str, ...]


def _anchored(pattern: str) -> str:
    """The pattern held to a whole run of tokens."""
    return rf"(?:^|_)(?:{pattern})(?:_|$)"


def _unique_keys(pairs: list[tuple[str, object]]) -> dict[str, object]:
    keys = [key for key, _ in pairs]
    repeated = sorted({key for key in keys if keys.count(key) > 1})
    if repeated:
        raise ValueError(f"a key repeats: {', '.join(repeated)}")
    return dict(pairs)


def load_rules(path: str) -> tuple[Rule, ...]:
    """The word list at `path`: keys exactly schema, note and rules; schema 1; each rule exactly id and pattern, its id
    unique and its pattern compiling, alone and held to whole tokens. Anything else is a ValueError naming the file."""
    with open(path, "rb") as f:
        raw = f.read()
    try:
        data = json.loads(raw.decode("utf-8"), object_pairs_hook=_unique_keys)
    except ValueError as e:
        raise ValueError(f"{path}: not a word list: {e}") from e
    if not isinstance(data, dict) or set(data) != {"schema", "note", "rules"}:
        raise ValueError(f"{path}: the keys must be exactly schema, note and rules")
    if data["schema"] != 1 or isinstance(data["schema"], bool):
        raise ValueError(f"{path}: schema must be 1, not {data['schema']!r}")
    if not isinstance(data["note"], str):
        raise ValueError(f"{path}: note must be a string")
    if not isinstance(data["rules"], list) or not data["rules"]:
        raise ValueError(f"{path}: rules must be a list of at least one rule")
    rules: list[Rule] = []
    for i, rule in enumerate(data["rules"]):
        if not isinstance(rule, dict) or set(rule) != {"id", "pattern"}:
            raise ValueError(f"{path}: rule {i}: the keys must be exactly id and pattern")
        rule_id, pattern = rule["id"], rule["pattern"]
        if not isinstance(rule_id, str) or not rule_id or not isinstance(pattern, str) or not pattern:
            raise ValueError(f"{path}: rule {i}: id and pattern must be non-empty strings")
        if any(r.id == rule_id for r in rules):
            raise ValueError(f"{path}: rule {i}: the id {rule_id!r} repeats")
        try:
            rx = re.compile(pattern)
            re.compile(_anchored(pattern))
        except re.error as e:
            raise ValueError(f"{path}: rule {rule_id!r}: the pattern does not compile: {e}") from e
        rules.append(Rule(rule_id, rx))
    return tuple(rules)


def tokens(name: str) -> str:
    """`name` as its lowercase tokens joined with `_`."""
    return "_".join(p.lower() for part in _SPLIT.split(name) for p in _CASE.split(part) if p)


def match(rules: Sequence[Rule], name: str) -> tuple[str, ...]:
    """The sorted ids of the rules that match a whole run of `name`'s tokens."""
    tok = tokens(name)
    return tuple(sorted(r.id for r in rules if re.search(_anchored(r.rx.pattern), tok)))


def in_scope(path: str) -> bool:
    """A test file the rule reads: a `*_test.go` or a `test_*.py`, with no `testdata` directory on its path."""
    parts = path.split("/")
    if "testdata" in parts[:-1]:
        return False
    return parts[-1].endswith("_test.go") or fnmatch.fnmatchcase(parts[-1], "test_*.py")


def _blank(src: str) -> str:
    """`src` with every comment and string, rune and raw string literal turned to spaces, its newlines kept.

    The algorithm of gate/bench/escapes.py's `_blank`, copied so this module imports nothing from the gate."""
    out: list[str] = []
    i, n = 0, len(src)
    while i < n:
        c = src[i]
        if src.startswith("//", i):
            j = src.find("\n", i)
            j = n if j < 0 else j
        elif src.startswith("/*", i):
            j = src.find("*/", i + 2)
            j = n if j < 0 else j + 2
        elif c == "`":
            j = src.find("`", i + 1)
            j = n if j < 0 else j + 1
        elif c in "\"'":
            j = i + 1
            while j < n and src[j] not in (c, "\n"):
                j += 2 if src[j] == "\\" else 1
            j = min(j + 1, n)
        else:
            out.append(c)
            i += 1
            continue
        out.append("".join(ch if ch == "\n" else " " for ch in src[i:j]))
        i = j
    return "".join(out)


def _closing(text: str) -> int:
    """The index of the `)` closing the `(` that `text` starts with; -1 when it does not close within `text`."""
    depth = 0
    for i, c in enumerate(text):
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                return i
    return -1


def _go_func(path: str, number: int, rest: str) -> list[Name]:
    """The function or method a column-0 `func ` line declares; `rest` is the text after `func `."""
    rest = rest.lstrip()
    kind = "func"
    if rest.startswith("("):
        end = _closing(rest)
        if end < 0:
            return []  # a receiver running past its line names no method on this line
        rest, kind = rest[end + 1 :].lstrip(), "method"
    m = _IDENT.match(rest)
    return [Name(path, kind, m[0], number)] if m and m[0] != "_" else []


def _go_spec(path: str, number: int, keyword: str, rest: str) -> list[Name]:
    """The names one spec declares: a type's first identifier, a var's or const's identifier list `a, b, c`."""
    rest = rest.lstrip()
    m = _IDENT.match(rest)
    if m is None:
        return []
    found, pos = [m[0]], m.end()
    while keyword != "type" and (more := _MORE.match(rest, pos)) is not None:
        found.append(more[1])
        pos = more.end()
    return [Name(path, keyword, name, number) for name in found if name != "_"]


def go_names(path: str, text: str) -> list[Name]:
    """The names a Go test file declares at its top level, in source order, its own name first."""
    base = path.rsplit("/", 1)[-1]
    stem = base.removesuffix("_test.go") if base.endswith("_test.go") else base.removesuffix(".go")
    found = [Name(path, "file", stem, 1)]
    depth = 0
    group = ""  # the keyword of the open `type (`, `var (` or `const (` group
    for number, line in enumerate(_blank(text).split("\n"), start=1):
        if group and depth == 1:
            found += _go_spec(path, number, group, line)
        elif not group and depth == 0:
            if line.startswith("func "):
                found += _go_func(path, number, line[len("func ") :])
            keyword = next((k for k in _DECLS if line.startswith(k + " ")), "")
            rest = line[len(keyword) + 1 :].lstrip() if keyword else ""
            if rest.startswith("("):
                group = keyword
                found += _go_spec(path, number, keyword, rest[1:])
            elif keyword:
                found += _go_spec(path, number, keyword, rest)
        depth = max(0, depth + sum(line.count(c) for c in _OPEN) - sum(line.count(c) for c in _CLOSE))
        if depth == 0:
            group = ""
    return found


def _targets(path: str, target: ast.expr) -> list[Name]:
    """The module-level names an assignment target binds: a name, or a tuple or list of names."""
    if isinstance(target, ast.Name):
        return [Name(path, "var", target.id, target.lineno)]
    if isinstance(target, ast.Tuple | ast.List):
        return [Name(path, "var", e.id, e.lineno) for e in target.elts if isinstance(e, ast.Name)]
    return []


def py_names(path: str, text: str) -> list[Name]:
    """The names a Python test module declares at its top level, in source order, its own name first."""
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("ignore", SyntaxWarning)  # an escape warning says nothing about a name
            tree = ast.parse(text, filename=path)
    except SyntaxError as e:
        raise Fault(path, f"does not parse: {e.msg} (line {e.lineno})") from e
    found = [Name(path, "file", path.rsplit("/", 1)[-1].removesuffix(".py"), 1)]
    for node in tree.body:
        match node:
            case ast.ClassDef():
                found.append(Name(path, "class", node.name, node.lineno))
                found += [
                    Name(path, "method", sub.name, sub.lineno)
                    for sub in node.body
                    if isinstance(sub, ast.FunctionDef | ast.AsyncFunctionDef)
                ]
            case ast.FunctionDef() | ast.AsyncFunctionDef():
                found.append(Name(path, "func", node.name, node.lineno))
            case ast.Assign():
                for target in node.targets:
                    found += _targets(path, target)
            case ast.AnnAssign():
                found += _targets(path, node.target)
    return found


def names(path: str, text: str) -> list[Name]:
    """The names of a Go or Python file, by its suffix."""
    if path.endswith(".go"):
        return go_names(path, text)
    if path.endswith(".py"):
        return py_names(path, text)
    raise Fault(path, "neither a Go nor a Python file")


def judge(rules: Sequence[Rule], found: Sequence[Name]) -> list[Hit]:
    """The names at least one rule matches, each with the ids of its rules."""
    return [Hit(name, ids) for name in found if (ids := match(rules, name.name))]


def _kill_group(pid: int, sig: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pid, sig)


def tracked() -> list[str]:
    """Every tracked `*_test.go` and `*.py` under the working directory, as git names them, sorted."""
    argv = ["git", "ls-files", "-z", "--", "*_test.go", "*.py"]
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        proc = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True)
        try:
            code = proc.wait(timeout=GIT_CEILING_S)
        except subprocess.TimeoutExpired as e:
            _kill_group(proc.pid, signal.SIGTERM)
            try:
                proc.wait(timeout=GIT_GRACE_S)
            except subprocess.TimeoutExpired:
                _kill_group(proc.pid, signal.SIGKILL)
                proc.wait()
            raise Fault("git ls-files", f"no answer within {GIT_CEILING_S} s") from e
        finally:
            _kill_group(proc.pid, signal.SIGKILL)  # anything the leader left behind in its session
        out.seek(0)
        err.seek(0)
        if code != 0:
            raise Fault("git ls-files", f"exit {code}: {err.read().decode('utf-8', errors='replace').strip()}")
        listing = out.read()
    return sorted(p.decode("utf-8", errors="replace") for p in listing.split(b"\0") if p)


def _read(path: str) -> str:
    try:
        with open(path, "rb") as f:
            return f.read().decode("utf-8")
    except OSError as e:
        raise Fault(path, f"unreadable: {e.strerror or e}") from e
    except UnicodeDecodeError as e:
        raise Fault(path, f"not UTF-8: {e}") from e


def _write_json(path: str, hits: Sequence[Hit]) -> None:
    """The hits as a JSON list, written to a temporary name beside `path` and renamed into place."""
    rows = [
        {"path": h.name.path, "line": h.name.line, "kind": h.name.kind, "name": h.name.name, "rules": list(h.rules)}
        for h in hits
    ]
    text = json.dumps(rows, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n"
    fd, tmp = tempfile.mkstemp(prefix=".testnames-", suffix=".json", dir=os.path.dirname(os.path.abspath(path)))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            f.write(text)
        os.rename(tmp, path)
    except BaseException:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def _parse_args(argv: Sequence[str]) -> tuple[str | None, list[str]] | None:
    """(the --json path or None, the paths); None for --help."""
    out_json: str | None = None
    paths: list[str] = []
    args = list(argv)
    while args:
        arg = args.pop(0)
        if arg in ("-h", "--help"):
            return None
        if arg == "--json":
            if not args or out_json is not None:
                raise ValueError(f"--json takes one output path, once; {USAGE}")
            out_json = args.pop(0)
        elif arg == "--":
            paths += args
            break
        elif arg.startswith("-"):
            raise ValueError(f"unknown option {arg}; {USAGE}")
        else:
            paths.append(arg)
    return out_json, paths


def report(paths: Sequence[str], rules: Sequence[Rule]) -> tuple[list[str], list[Hit]]:
    """The report's lines and its hits, both sorted, for these files."""
    found: list[Name] = []
    for path in paths:
        found += names(path, _read(path))
    hits = sorted(judge(rules, found), key=lambda h: (h.name.path, h.name.line, h.name.kind, h.name.name))
    lines = [f"{h.name.path}:{h.name.line}: {h.name.kind} {h.name.name}: {', '.join(h.rules)}" for h in hits]
    files_hit = len({h.name.path for h in hits})
    lines.append(
        f"testnames: {len(paths)} files, {len(found)} names; {len(hits)} hits in {files_hit} files (report only)"
    )
    return lines, hits


def _emit(lines: Sequence[str]) -> None:
    sys.stdout.buffer.write("".join(line + "\n" for line in lines).encode("utf-8", errors="replace"))
    sys.stdout.buffer.flush()


def main(argv: Sequence[str]) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    try:
        parsed = _parse_args(argv)
        if parsed is None:
            _emit([USAGE])
            return 0
        out_json, given = parsed
        rules = load_rules(WORDS)
        paths = sorted(set(given)) if given else [p for p in tracked() if in_scope(p)]
        lines, hits = report(paths, rules)
        if out_json is not None:
            _write_json(out_json, hits)
    except (Fault, OSError, ValueError) as e:
        _emit([f"error: {e}"])
        return 2
    except BaseException as e:  # every other failure, a stop included, is exit 2 with its reason
        _emit([f"error: internal error: {type(e).__name__}: {e}"])
        raise SystemExit(2) from e
    _emit(lines)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
