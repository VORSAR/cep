"""The heap-escape check of a split: -gcflags=-m=2 heap lines over a function's group, before against after.

    python3.15 -I -B gate/bench/escapes.py --pkg ./internal/<pkg> --before <rev> --after <rev> \\
        --function <original name> [--helper <name>]...

Each revision is extracted with `git archive` into a temporary directory outside the repo and built with
`go build -gcflags=-m=2 <pkg>` (S1.1c-6 design note §2.4 step 2, §3.2). From the package's own files, every line
saying `escapes to heap` or `moved to heap` is assigned to the function its "... in F:" line names; a line that
carries no function takes the one named by the latest "in F:" (or "for F") line printed before it at the same
position: a value method and its generated pointer wrapper share positions, and each function's summary lines
follow its own detail lines. With none, it is exit 2.
The group is the original function, its closures, and every --helper; a helper the compiler inlined is counted
through its callers only, since each inlined copy is reported again in the caller. Lines are compared as texts,
without the position prefix and the " in F:" suffix. Exit 1 if the after side has a text, or more of one, that the
before side lacks. `leaking param` lines are a diagnostic, counted only for the original function itself.
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


def _in(fn: str, members: set[str]) -> bool:
    return any(fn == m or fn.startswith(m + ".func") for m in members)


def build(rev: str, pkg: str, repo: str) -> str:
    """The -m=2 output of `pkg` at `rev`, from a git archive extracted outside the repo."""
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
        return done.stderr


def _texts(parsed: Parsed, members: set[str]) -> collections.Counter[str]:
    return collections.Counter(text for fn, _, text in parsed.heap if _in(fn, members))


def check(args: argparse.Namespace, repo: str) -> tuple[int, list[str]]:
    before = parse(build(args.before, args.pkg, repo), args.pkg)
    after = parse(build(args.after, args.pkg, repo), args.pkg)
    original_before = resolve(args.function, before.functions, args.before)
    original_after = resolve(args.function, after.functions, args.after)
    helpers = [resolve(h, after.functions, args.after) for h in args.helper]
    # An inlined helper's escapes are reported again at each call site, in its caller's "in F:" lines.
    counted = {original_after, *(h for h in helpers if h not in after.inlined)}
    lines = [f"group at {args.after}: {', '.join([original_after, *helpers])}"]
    for h in helpers:
        if h in after.inlined:
            lines.append(f"helper {h} is inlined: counted through its callers")
    old, new = _texts(before, {original_before}), _texts(after, counted)
    for side, counter in (("before", old), ("after", new)):
        for text in sorted(counter):
            lines.append(f"heap {side}: {text} x{counter[text]}")
    added = new - old
    for text in sorted(added):
        lines.append(f"new: {text} (before {old[text]}, after {new[text]})")
    members_after = {original_after, *helpers}
    for side, parsed, members in (("before", before, {original_before}), ("after", after, members_after)):
        for fn, _, text in sorted(parsed.leaks):
            if _in(fn, members):
                lines.append(f"leaking param (diagnostic) {side}: {fn}: {text}")
    leaks_before = collections.Counter(t for fn, _, t in before.leaks if fn == original_before)
    leaks_after = collections.Counter(t for fn, _, t in after.leaks if fn == original_after)
    new_leaks = leaks_after - leaks_before
    for text in sorted(new_leaks):
        lines.append(f"new leak of {original_after}'s own parameter: {text}")
    lines.append(f"package heap lines: before {len(before.heap)}, after {len(after.heap)}")
    costs = sorted(added) + sorted(new_leaks)
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
