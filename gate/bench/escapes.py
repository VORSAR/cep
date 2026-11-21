"""The heap-escape check of a split: -gcflags=-m=2 heap lines over a function's group, before against after.

    python3.15 -I -B gate/bench/escapes.py --pkg ./internal/<pkg> --before <rev> --after <rev> \\
        --function <original name> [--helper <name>]...

Each revision is extracted with `git archive` into a temporary directory outside the repo and built with
`go build -gcflags=-m=2 <pkg>` (S1.1c-6 design note §2.4 step 2, §3.2). From the package's own files, every line
saying `escapes to heap` or `moved to heap` is assigned to the function its "... in F:" line names; a line that
carries no function takes the one named by the latest "in F:" (or "for F") line printed before it at the same
position: a value method and its generated pointer wrapper share positions, and each function's summary lines
follow its own detail lines. With none, it is exit 2.
The group is the original function, its closures, and every --helper, on each side over the members that exist at
that revision; a helper the compiler inlined is counted through its callers only, since each inlined copy is
reported again in the caller. Lines are compared as texts, without the position prefix and the " in F:" suffix.
Exit 1 if the after side has a text, or more of one, that the
before side lacks. `leaking param` lines are a diagnostic, listed on each side for every group member that exists
at that revision. A new one (a text, or more of one, the after side has and the before side lacks) costs only when
it is the original function's own and the parameter it names, the first word after "leaking param:" or "leaking
param content:", is one the function's declaration at the before revision has, the receiver included. When there
is a new one, that declaration is read from the before revision's own source (the package's built .go files); one
not found there is exit 2. A new leak of a parameter the function did not have before is printed as a diagnostic,
not counted.
"""

import argparse
import collections
import dataclasses
import os
import re
import signal
import sys
import tarfile
import tempfile
from collections.abc import Sequence
from typing import NoReturn

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate import term
from gate.bench import gorun
from gate.runner import env as gate_env

BUILD_TIMEOUT_S = 900.0
GIT_TIMEOUT_S = 300.0
STDERR_TAIL = 10

_LINE = re.compile(r"^(?P<file>[^\s:][^:]*):(?P<line>[0-9]+):(?P<col>[0-9]+): (?P<msg>\S.*)$")
_CAN = re.compile(r"^(?:can inline (?P<can>\S+) with cost |cannot inline (?P<cannot>\S+): )")
_INLINING = re.compile(r"^inlining call to (?P<fn>\S+)$")
_HEAP_IN = re.compile(r"^(?P<what>.+ escapes to heap) in (?P<fn>\S+):$")
_HEAP = re.compile(r"^(?:.+ escapes to heap|moved to heap: \S+)$")
_LEAK_FOR = re.compile(r"^parameter \S+ leaks to .+ for (?P<fn>\S+) with derefs=-?[0-9]+:$")
_LEAK = re.compile(r"^leaking param(?: content)?: .+$")
_LEAKED = re.compile(r"^leaking param(?: content)?: (?P<name>\S+)")
_IDENT = re.compile(r"[^\W\d]\w*")
_FUNC = re.compile(r"^func\b", re.MULTILINE)
_SPACE = re.compile(r"\s*")
_NAMED = re.compile(r"^(?P<name>[^\W\d]\w*)\s+(?P<type>\S.*)$", re.DOTALL)
_TYPE_WORDS = frozenset({"chan", "func", "interface", "map", "struct"})  # a type that starts with a word
_TARGET = re.compile(r"^(?:\(\*?(?P<paren>\w+)\)\.|(?P<bare>\w+)\.)?(?P<name>\w+)$")


class Error(Exception):
    """Exit 2: the check cannot be judged."""


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise Error(f"usage: {message}")


@dataclasses.dataclass
class Parsed:
    functions: set[str]
    heap: list[tuple[str, str, str]]  # (function, position, normalised text)
    leaks: list[tuple[str, str, str]]
    inlined: set[str]


def _own(file: str, pkg: str) -> bool:
    return (os.path.dirname(os.path.normpath(file)) or ".") == os.path.normpath(pkg)


def _named(at: dict[str, str], pos: str, raw: str) -> str:
    """The function of the latest detail line printed at `pos`; none before this line is exit 2."""
    fn = at.get(pos)
    if fn is None:
        raise Error(f"no function named at the position of {raw!r} before it")
    return fn


def parse(text: str, pkg: str) -> Parsed:
    """The package's own -m=2 lines: its functions, its heap and leak lines by function, the helpers it inlined."""
    parsed = Parsed(set(), [], [], set())
    heap_at: dict[str, str] = {}
    leak_at: dict[str, str] = {}
    for raw in text.splitlines():
        m = _LINE.match(raw)
        if m is None or not _own(m["file"], pkg):
            continue
        pos, msg = f"{os.path.normpath(m['file'])}:{m['line']}:{m['col']}", m["msg"]
        if c := _CAN.match(msg):
            parsed.functions.add(c["can"] or c["cannot"])
        elif i := _INLINING.match(msg):
            parsed.inlined.add(i["fn"])
        elif h := _HEAP_IN.match(msg):
            heap_at[pos] = h["fn"]
            parsed.heap.append((h["fn"], pos, h["what"]))
        elif _HEAP.match(msg):
            parsed.heap.append((_named(heap_at, pos, raw), pos, msg))
        elif lf := _LEAK_FOR.match(msg):
            leak_at[pos] = lf["fn"]
        elif _LEAK.match(msg):
            parsed.leaks.append((_named(leak_at, pos, raw), pos, msg))
    return parsed


def resolve(name: str, functions: set[str], rev: str) -> str:
    """The one printed name `name` means: itself, or the one method whose last component it is."""
    if name in functions:
        return name
    matches = sorted(f for f in functions if "." in f and f.rsplit(".", 1)[1] == name)
    if len(matches) == 1:
        return matches[0]
    if matches:
        raise Error(f"{name} is ambiguous at {rev}: {', '.join(matches)}")
    raise Error(f"function {name} does not exist at {rev}")


def leaked(text: str) -> str:
    """The parameter a `leaking param` line names: the first word after its colon."""
    m = _LEAKED.match(text)
    if m is None:
        raise Error(f"not a leaking param line: {text!r}")
    return m["name"]


def _blank(src: str) -> str:
    """`src` with every comment and string, rune and raw string literal turned to spaces, its newlines kept."""
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


def _closing(text: str, at: int) -> int | None:
    """The index just past the bracket that closes the one at `at`; None if it never closes."""
    depth = 0
    for j in range(at, len(text)):
        if text[j] in "([{":
            depth += 1
        elif text[j] in ")]}":
            depth -= 1
            if depth == 0:
                return j + 1
    return None


def _skip(text: str, at: int) -> int:
    m = _SPACE.match(text, at)
    return m.end() if m else at


def _declarations(text: str) -> list[tuple[str | None, str, str]]:
    """Each top-level `func` declaration of blanked Go text: (receiver list or None, name, parameter list)."""
    found: list[tuple[str | None, str, str]] = []
    for m in _FUNC.finditer(text):
        i, receiver = _skip(text, m.end()), None
        if text.startswith("(", i):
            if (end := _closing(text, i)) is None:
                continue
            receiver, i = text[i + 1 : end - 1], _skip(text, end)
        if (ident := _IDENT.match(text, i)) is None:
            continue
        name, i = ident[0], _skip(text, ident.end())
        if text.startswith("[", i):  # type parameters
            if (end := _closing(text, i)) is None:
                continue
            i = _skip(text, end)
        if not text.startswith("(", i) or (end := _closing(text, i)) is None:
            continue
        found.append((receiver, name, text[i + 1 : end - 1]))
    return found


def _split(params: str) -> list[str]:
    """A parameter list's items: split at its top-level commas, stripped, an empty trailing item dropped."""
    items, depth, start = [], 0, 0
    for j, c in enumerate(params):
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
        elif c == "," and depth == 0:
            items.append(params[start:j].strip())
            start = j + 1
    items.append(params[start:].strip())
    return [item for item in items if item]


def _names(params: str) -> list[str] | None:
    """A parameter list's names in order (a grouped `a, b T` gives both); [] if unnamed; None if unreadable."""
    items = _split(params)
    named = [m if (m := _NAMED.match(item)) and m["name"] not in _TYPE_WORDS else None for item in items]
    if not any(named):
        return []  # every item is a type: the parameters have no names
    names = []
    for item, m in zip(items, named, strict=True):
        if m is not None:
            names.append(m["name"])
        elif _IDENT.fullmatch(item):
            names.append(item)  # a name grouped with the next item's type
        else:
            return None
    return names


def _unbracket(name: str) -> str:
    """`name` without its bracketed type arguments: (*List[go.shape.int]).Push is (*List).Push."""
    out, depth = [], 0
    for c in name:
        if c == "[":
            depth += 1
        elif c == "]":
            depth -= 1
        elif depth == 0:
            out.append(c)
    return "".join(out)


def declared(sources: dict[str, str], name: str, rev: str) -> list[str]:
    """The parameter names, receiver first, of the declaration of printed name `name` in `sources` (file -> Go).

    A method (*T).m, (T).m or T.m is `func (<recv> *T) m(` or `func (<recv> T) m(` (Go allows only one of the
    two per name, so the star is not compared); a function f is `func f(`. Not found, found with different
    parameters in two files, or a list that cannot be read is an error: the names are never guessed.
    """
    target = _TARGET.match(_unbracket(name))
    if target is None:
        raise Error(f"cannot read {name!r} as a function or method name")
    owner, fn = target["paren"] or target["bare"], target["name"]
    found: dict[tuple[str, ...], list[str]] = {}
    for file in sorted(sources):
        for receiver, decl, params in _declarations(_blank(sources[file])):
            if decl != fn or (receiver is None) != (owner is None):
                continue
            names: list[str] = []
            if receiver is not None:
                m = _NAMED.match(receiver.strip())
                recv_type = m["type"] if m and m["name"] not in _TYPE_WORDS else receiver
                if re.sub(r"[\s()*]", "", _unbracket(recv_type)) != owner:
                    continue
                names = [m["name"]] if m and m["name"] not in _TYPE_WORDS else []
            listed = _names(params)
            if listed is None:
                raise Error(f"cannot read the parameters of {name} in {file} at {rev}: ({' '.join(params.split())})")
            found.setdefault(tuple(names + listed), []).append(file)
    if not found:
        raise Error(f"no declaration of {name} in the package's source at {rev}")
    if len(found) > 1:
        seen = "; ".join(f"{', '.join(files)}: ({', '.join(key)})" for key, files in sorted(found.items()))
        raise Error(f"{name} is declared with different parameters at {rev}: {seen}")
    return list(next(iter(found)))


def _in(fn: str, members: set[str]) -> bool:
    return any(fn == m or fn.startswith(m + ".func") for m in members)


@dataclasses.dataclass
class Built:
    output: str  # go build's -m=2 text
    sources: dict[str, str]  # the package's built .go files (no _test.go, none starting "_" or "."), by name


def _sources(pkg_dir: str) -> dict[str, str]:
    sources = {}
    for name in sorted(os.listdir(pkg_dir)):
        if name.endswith(".go") and not name.endswith("_test.go") and not name.startswith(("_", ".")):
            with open(os.path.join(pkg_dir, name), "rb") as f:
                sources[name] = f.read().decode("utf-8", errors="replace")
    return sources


def build(rev: str, pkg: str, repo: str) -> Built:
    """The -m=2 output of `pkg` at `rev` and the package's source, from a git archive extracted outside the repo."""
    with tempfile.TemporaryDirectory(prefix="escapes-") as tmp:
        archive, tree = os.path.join(tmp, "tree.tar"), os.path.join(tmp, "tree")
        done = gorun.run(["git", "archive", "--format=tar", "-o", archive, rev], cwd=repo, timeout_s=GIT_TIMEOUT_S)
        if done.exit != 0 or done.timed_out:
            first = (done.stderr.splitlines() or ["(no stderr)"])[0]
            raise Error(f"git archive {rev} failed: {first}")
        with tarfile.open(archive) as tar:
            tar.extractall(tree, filter="data")
        env = {**gate_env.clean_env(os.environ, []), "GOPROXY": "off"}
        done = gorun.run(["go", "build", "-gcflags=-m=2", pkg], cwd=tree, env=env, timeout_s=BUILD_TIMEOUT_S)
        if done.exit != 0 or done.timed_out:
            tail = " | ".join(done.stderr.splitlines()[-STDERR_TAIL:])
            raise Error(f"go build {pkg} at {rev} failed (exit {done.exit}): {tail}")
        return Built(done.stderr, _sources(os.path.join(tree, pkg)))


def _texts(parsed: Parsed, members: set[str]) -> collections.Counter[str]:
    return collections.Counter(text for fn, _, text in parsed.heap if _in(fn, members))


def check(args: argparse.Namespace, repo: str) -> tuple[int, list[str]]:
    built_before = build(args.before, args.pkg, repo)
    before = parse(built_before.output, args.pkg)
    after = parse(build(args.after, args.pkg, repo).output, args.pkg)
    original_before = resolve(args.function, before.functions, args.before)
    original_after = resolve(args.function, after.functions, args.after)
    helpers = [resolve(h, after.functions, args.after) for h in args.helper]
    # An inlined helper's escapes are reported again at each call site, in its caller's "in F:" lines.
    counted = {original_after, *(h for h in helpers if h not in after.inlined)}
    counted_before = {original_before, *(h for h in helpers if h in before.functions and h not in before.inlined)}
    lines = [f"group at {args.after}: {', '.join([original_after, *helpers])}"]
    for h in helpers:
        if h in after.inlined:
            lines.append(f"helper {h} is inlined: counted through its callers")
    old, new = _texts(before, counted_before), _texts(after, counted)
    for side, counter in (("before", old), ("after", new)):
        for text in sorted(counter):
            lines.append(f"heap {side}: {text} x{counter[text]}")
    added = new - old
    for text in sorted(added):
        lines.append(f"new: {text} (before {old[text]}, after {new[text]})")
    members_before = {original_before, *(h for h in helpers if h in before.functions)}
    members_after = {original_after, *helpers}
    for side, parsed, members in (("before", before, members_before), ("after", after, members_after)):
        for fn, _, text in sorted(parsed.leaks):
            if _in(fn, members):
                lines.append(f"leaking param (diagnostic) {side}: {fn}: {text}")
    leaks_before = collections.Counter(t for fn, _, t in before.leaks if fn == original_before)
    leaks_after = collections.Counter(t for fn, _, t in after.leaks if fn == original_after)
    new_leaks = leaks_after - leaks_before
    # Design §2.4 step 2: a leak counts only for a parameter the original function already had.
    had = set(declared(built_before.sources, original_before, args.before)) if new_leaks else set()
    own_leaks = []
    for text in sorted(new_leaks):
        if leaked(text) in had:
            own_leaks.append(text)
            lines.append(f"new leak of {original_after}'s own parameter: {text}")
        else:
            lines.append(f"leak of a parameter {original_before} did not have before (diagnostic): {text}")
    lines.append(f"package heap lines: before {len(before.heap)}, after {len(after.heap)}")
    costs = sorted(added) + own_leaks
    if costs:
        lines.append(f"verdict: new heap lines in {args.function}'s group ({'; '.join(costs)})")
        return 1, lines
    lines.append(f"verdict: no new heap line in {args.function}'s group")
    return 0, lines


def _parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = _Parser(prog="escapes.py", add_help=False)
    parser.add_argument("--pkg", required=True)
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    parser.add_argument("--function", required=True)
    parser.add_argument("--helper", action="append", default=[])
    return parser.parse_args(list(argv))


def main(argv: Sequence[str]) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    term.install()
    try:
        code, lines = check(_parse_args(argv), os.getcwd())
    except Error as e:
        code, lines = 2, [f"error: {e}"]
    except BaseException as e:  # every other failure, a stop included, is exit 2 with its reason
        print(f"error: internal error: {type(e).__name__}: {e}", flush=True)
        raise SystemExit(2) from e
    for line in lines:
        print(line)
    sys.stdout.flush()
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
