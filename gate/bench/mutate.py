"""One mutation, its one designated test, and a tracked file never written (S1.1c-6 design note §2.3, §3.3; S1.1c-7
design note W0, Part A as revised).

    python3.15 -I -B gate/bench/mutate.py (--file <path> --find <text> --replace <text> | --patch <diff file>) \\
        (--test <TopLevelTestName> [--subtest <path>] [--pkg <dir>] [--race] [--cpu <list>] | --py <unittest id>) \\
        [--timeout 120s] [--repeat N --load]
    python3.15 -I -B gate/bench/mutate.py --batch <mutations.json> [--timeout 120s] [--race] [--cpu <list>]
    python3.15 -I -B gate/bench/mutate.py --expect <entries.json> [--timeout 120s] [--race] [--cpu <list>]

Run from the repo's top level. A mutation is a find text replaced once in one file (it must occur exactly once), or
a unified diff (`git diff`'s output included; a batch entry's "patch" holds the diff's text): each hunk is placed by
its old lines occurring exactly once in the progressively patched file, its start lines ignored and its line counts
checked; a new, deleted or renamed file, a quoted path and the no-newline marker are refused. Every file it names
must be tracked and equal to HEAD.

The designated test is --test, a top-level Go Test run in the first file's package or in --pkg's (a guard test of a
package's contract may live in a package that imports it), or --py, a dotted unittest id. --subtest (batch key
"subtest") names one subtest of --test: split at "/", each level rewritten as go test names it (each Go space to
"_"), QuoteMeta'd and anchored, so F3 never runs F30; a probe run first, each level also allowing go test's #NN
suffix for a repeated name, must count exactly one run, and each run at the subtest's depth is printed as `ran:`. A
Go mutant is written outside the tree and built through `go test -overlay`. A --py mutant is committed, hooks off,
under a throwaway identity, in a detached worktree of HEAD outside the repo (one per run, reset between entries), so
a test calling git there sees a clean tree; the id runs as `python3.15 -I -B -X dev -W error
<worktree>/gate/runner/pyshim.py <worktree> <id> <report>` from the worktree, with --timeout as its wall clock. The
worktree is removed when the run ends, a stop included; one of this run's worktrees still listed or on disk after
that is exit 2.

A batch entry holds the single-mode flags as keys (file, find, replace, patch, test, subtest, py, pkg), with note,
an optional id, and timeout: a Go duration or positive seconds, replacing --timeout for that entry. An --expect
entry also needs id (unique), old_rev and new_rev (full commit ids of this repository) and expect ({"old":
<outcome>, "new": <outcome>}); old_test, new_test, old_subtest, new_subtest, old_py and new_py name a side's own
test, and "control": true with no mutation reads the real code. Each entry runs at each revision in the run's one
worktree per revision, reset between entries, the plant committed there (Go included, no overlay); the mutation must
occur exactly once in each revision's blob. It prints `entry <id> <side> <rev>: <outcome> (expected <word>)` for
each side, then `expect: <n> entries, <k> as expected; differing: <ids or none>`, and exits 0 only when nothing
differs.

Refused (exit 2) before any go or Python test command: a file not tracked or differing from HEAD, a find text or a
hunk occurring other than exactly once, a malformed diff, a --pkg that is not a tracked directory, a test name that
is not a top-level Test identifier, a --subtest without --test, with an empty level or with a rune go test would
escape, a --py id that is not dotted identifiers, a mutation or a test given both ways or neither, --subtest,
--pkg, --race or --cpu with --py, a malformed batch (every entry is checked before the first build).

Go outcomes, judged in order: timeout (go test's own -timeout backstop), red (the line `--- FAIL: <Test> (`), green
(exit 0 and `--- PASS: <Test> (`), invalid (anything else). A subtest is judged by its own lines: invalid with no
`=== RUN` line for it, or a `--- SKIP:` one; red on its own `--- FAIL:` line (a parent's FAIL alone is invalid);
green on exit 0, its own `--- PASS:` line and no FAIL line at all; invalid otherwise, as when the probe counts other
than one run. Python outcomes: timeout (the wall clock), red (exit 1, the shim's only problem `failed: 1 failures, 0
errors`, and `Ran 1 test in`), green (exit 0 and `passed 1 tests`), invalid (anything else: an import error, a skip,
zero tests, a failure beside a leak). Only red counts (owner, 2026-11-16, G1 decision 4). Single mode exits 0 on
red, 1 otherwise; batch mode 0 when every entry is red; --repeat N --load runs N times under CPU hogs and exits 0
when k >= 1 of N. Exit 2 on a refusal or an internal error; the last line says which.
"""

import argparse
import dataclasses
import difflib
import json
import os
import re
import shutil
import signal
import sys
import tempfile
from collections.abc import Mapping, Sequence
from typing import NoReturn

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate import term
from gate.bench import gorun
from gate.runner import rows, stress

WALL_EXTRA_S = 120.0
LOOKUP_TIMEOUT_S = 60.0
TAIL = 20
OUTCOMES = ("red", "green", "timeout", "invalid")
PYTHON = "python3.15"  # the pytests row's interpreter (gate/runner/rows/pytests.py PYTHON)
TRIPLE = frozenset({"file", "find", "replace"})
MUTATION_KEYS = TRIPLE | {"patch"}
TARGET_KEYS = frozenset({"test", "subtest", "py"})
PLAIN_KEYS = MUTATION_KEYS | TARGET_KEYS | {"id", "note", "pkg", "timeout"}
SIDE_KEYS = frozenset(f"{side}_{key}" for side in ("old", "new") for key in TARGET_KEYS)
EXPECT_KEYS = PLAIN_KEYS | SIDE_KEYS | {"old_rev", "new_rev", "expect", "control"}
EXPECT_REQUIRED = ("id", "old_rev", "new_rev", "expect")
FULL_REV = re.compile(r"[0-9a-f]{40}")
THROWAWAY = {
    "GIT_AUTHOR_NAME": "mutate.py",
    "GIT_AUTHOR_EMAIL": "mutate@invalid",
    "GIT_COMMITTER_NAME": "mutate.py",
    "GIT_COMMITTER_EMAIL": "mutate@invalid",
}
NO_HOOKS = ("-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false")
GO_META = frozenset("\\.+*?()|[]{}^$")  # regexp.QuoteMeta's set
# unicode.IsSpace, which go test's name rewriting uses (testing/match.go isSpace): each becomes "_".
GO_SPACES = frozenset(
    map(
        chr,
        (
            0x09,
            0x0A,
            0x0B,
            0x0C,
            0x0D,
            0x20,
            0x85,
            0xA0,
            0x1680,
            *range(0x2000, 0x200B),
            0x2028,
            0x2029,
            0x202F,
            0x205F,
            0x3000,
        ),
    )
)
_DURATION = re.compile(r"(?:[0-9]+(?:\.[0-9]*)?(?:ns|us|µs|ms|s|m|h))+")
_UNIT_S = {"ns": 1e-9, "us": 1e-6, "µs": 1e-6, "ms": 1e-3, "s": 1.0, "m": 60.0, "h": 3600.0}
_HUNK = re.compile(r"^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@")
_SKIP = ("diff --git ", "index ", "old mode ", "new mode ")
_UNSUPPORTED = (
    "new file mode ",
    "deleted file mode ",
    "rename from ",
    "rename to ",
    "similarity index ",
    "copy from ",
    "copy to ",
    "Binary files ",
    "GIT binary patch",
    "\\ No newline",
)


class Refused(Exception):
    """Exit 2 before any go or Python test command."""


class Internal(Exception):
    """Exit 2: a git step of the run failed, or one of the run's worktrees outlived it."""


class Helped(Exception):
    """--help printed the usage: exit 0."""


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise Refused(f"usage: {message}")

    def exit(self, status: int = 0, message: str | None = None) -> NoReturn:
        """Reached only from --help, after the usage is printed (error is overridden above)."""
        raise Helped


@dataclasses.dataclass(frozen=True)
class Target:
    test: str = ""  # a Go top-level Test ("" in the Python mode)
    levels: tuple[str, ...] = ()  # the Go subtest path, each level rewritten as go test names it
    py: str = ""  # a unittest id ("" in the Go mode)

    @property
    def full(self) -> str:
        return "/".join((self.test, *self.levels))


@dataclasses.dataclass(frozen=True)
class Change:
    file: str
    original: str
    mutant: str


@dataclasses.dataclass(frozen=True)
class Entry:
    id: str
    changes: tuple[Change, ...]  # the plant at HEAD (plain and single mode) or at new_rev (--expect); () for a control
    old: Target  # plain and single mode: old == new
    new: Target
    pkg: str  # the Go package directory; "" for Python
    note: str
    timeout: str  # a Go duration: the entry's "timeout", else the run's --timeout
    old_rev: str = ""
    new_rev: str = ""
    expect: tuple[str, str] = ("", "")
    control: bool = False
    old_changes: tuple[Change, ...] = ()  # --expect: the plant applied to old_rev's blobs

    @property
    def target(self) -> Target:
        """Plain and single mode's one target."""
        return self.new


def seconds(duration: str) -> float:
    """A Go duration (`120s`, `2m30s`, `500ms`) in seconds."""
    if not _DURATION.fullmatch(duration):
        raise Refused(f"--timeout {duration!r} is not a Go duration")
    total = sum(float(n) * _UNIT_S[u] for n, u in re.findall(r"([0-9.]+)(ns|us|µs|ms|s|m|h)", duration))
    if total <= 0:
        raise Refused(f"--timeout {duration!r} is not positive")
    return total


def _git(*argv: str, env: Mapping[str, str] | None = None) -> gorun.Result:
    merged = None if env is None else {**os.environ, **env}
    return gorun.run(["git", *argv], cwd=os.getcwd(), env=merged, timeout_s=LOOKUP_TIMEOUT_S)


def _git_ok(*argv: str, env: Mapping[str, str] | None = None) -> gorun.Result:
    result = _git(*argv, env=env)
    if result.exit != 0:
        first = (result.stderr.strip().splitlines() or [""])[0]
        raise Internal(f"git {' '.join(argv)}: exit {result.exit}: {first}")
    return result


def _occurrences(text: str, find: str) -> int:
    """Overlapping occurrences: `aa` occurs twice in `aaa`."""
    count, at = 0, text.find(find)
    while at != -1:
        count += 1
        at = text.find(find, at + 1)
    return count


def go_rewrite(level: str) -> str:
    """One subtest level as go test names it: each Go space becomes "_"."""
    return "".join("_" if c in GO_SPACES else c for c in level)


def quote_meta(text: str) -> str:
    """regexp.QuoteMeta."""
    return "".join("\\" + c if c in GO_META else c for c in text)


def target(test: str | None, subtest: str | None, py: str | None) -> Target:
    """The designated test: exactly one of a top-level Go Test (with a subtest path, split at "/") and a dotted
    unittest id. A level holding a rune that is neither a Go space nor printable is refused: go test would escape it
    with strconv.QuoteRune, which is not emulated."""
    if test is not None and py is not None:
        raise Refused("the designated test is test or py: not both")
    if py is not None:
        if subtest is not None:
            raise Refused("subtest names a Go subtest: not with py")
        parts = py.split(".")
        if len(parts) < 2 or not all(part.isidentifier() for part in parts):
            raise Refused(f"py {py!r} is not a dotted unittest id")
        return Target(py=py)
    if test is None:
        raise Refused("the designated test is test or py: neither is given")
    if not test.isidentifier() or not test.startswith("Test"):
        raise Refused(f"--test {test!r} is not a top-level Test identifier")
    if subtest is None:
        return Target(test=test)
    levels = subtest.split("/")
    if "" in levels:
        raise Refused(f"--subtest {subtest!r} has an empty level")
    for level in levels:
        if any(c not in GO_SPACES and not c.isprintable() for c in level):
            raise Refused(f"--subtest {subtest!r} holds a rune go test would escape")
    return Target(test, tuple(go_rewrite(level) for level in levels))


def pattern(t: Target, probe: bool) -> str:
    """go test's -run for the target: each level QuoteMeta'd and anchored; the probe also lets each level carry the
    #NN suffix go test gives a repeated name."""
    parts = [f"^{t.test}$"]
    for level in t.levels:
        parts.append(f"^{quote_meta(level)}{'(#[0-9]+)?' if probe else ''}$")
    return "/".join(parts)


def runs(text: str, t: Target) -> list[str]:
    """The probe's count: `=== RUN` names equal to the designated path, each level allowing a `#NN` suffix."""
    rx = r"^=== RUN\s+(" + re.escape(t.test)
    rx += "".join("/" + re.escape(level) + r"(?:#[0-9]+)?" for level in t.levels) + r")$"
    return re.findall(rx, text, re.MULTILINE)


def ran(text: str, t: Target) -> list[str]:
    """Every `=== RUN` name under the designated parent at the designated depth, whatever its last level: a run that
    selected a sibling (F30 beside F3) shows it here."""
    parent = re.escape(t.test) + "".join("/" + re.escape(level) + r"(?:#[0-9]+)?" for level in t.levels[:-1])
    return re.findall(r"^=== RUN\s+(" + parent + r"/[^/\n]+)$", text, re.MULTILINE)


def parse_patch(text: str) -> list[tuple[str, list[tuple[str, str]]]]:
    """A unified diff (git diff's output included) as [(path, [(old block, new block), ...])]. Hunk start lines are
    ignored: each hunk is placed by its old block occurring exactly once; its counts must match its body."""
    files: list[tuple[str, list[tuple[str, str]]]] = []
    lines = text.splitlines(keepends=True)
    i = 0
    while i < len(lines):
        line = lines[i]
        if line.startswith(_SKIP):
            i += 1
        elif line.startswith(_UNSUPPORTED):
            raise Refused(f"--patch: {line.strip()!r} is not supported: a plant edits existing files in place")
        elif line.startswith("--- "):
            if i + 1 >= len(lines) or not lines[i + 1].startswith("+++ "):
                raise Refused("--patch: a --- line without its +++ line")
            files.append((_patch_path(line[4:], lines[i + 1][4:]), []))
            i += 2
        elif m := _HUNK.match(line):
            if not files:
                raise Refused("--patch: a hunk before any file header")
            old, new, i = _hunk(lines, i + 1, int(m[1] or 1), int(m[2] or 1))
            files[-1][1].append((old, new))
        elif line.strip():
            raise Refused(f"--patch: unexpected line {line.rstrip()!r}")
        else:
            i += 1
    if not files or any(not hunks for _, hunks in files):
        raise Refused("--patch: no file with a hunk")
    return files


def _patch_path(old: str, new: str) -> str:
    old, new = old.rstrip("\n").split("\t")[0], new.rstrip("\n").split("\t")[0]
    if old.startswith('"') or new.startswith('"'):
        raise Refused("--patch: a quoted path is not supported")
    if "/dev/null" in (old, new):
        raise Refused("--patch: a plant edits existing files in place")
    if old.startswith("a/") and new.startswith("b/"):
        old, new = old[2:], new[2:]
    if old != new:
        raise Refused(f"--patch: --- {old} and +++ {new} name different files")
    if not old or os.path.isabs(old) or ".." in old.split("/"):
        raise Refused(f"--patch: {old!r} is not a repo-relative path")
    return old


def _hunk(lines: list[str], i: int, old_n: int, new_n: int) -> tuple[str, str, int]:
    old: list[str] = []
    new: list[str] = []
    while len(old) < old_n or len(new) < new_n:
        if i >= len(lines):
            raise Refused("--patch: a hunk shorter than its header")
        line = lines[i]
        tag, body = (" ", line) if line == "\n" else (line[0], line[1:])  # an editor may strip a blank context line
        if tag == " ":
            old.append(body)
            new.append(body)
        elif tag == "-":
            old.append(body)
        elif tag == "+":
            new.append(body)
        else:
            raise Refused(f"--patch: a hunk line starting {tag!r}")
        i += 1
    if len(old) != old_n or len(new) != new_n:
        raise Refused("--patch: a hunk's lines do not match its header")
    if not old:
        raise Refused("--patch: a hunk with no old line cannot be placed")
    return "".join(old), "".join(new), i


def apply(path: str, original: str, hunks: Sequence[tuple[str, str]]) -> str:
    """The hunks applied in order on the progressively mutated text, each occurring exactly once."""
    text = original
    for old, new in hunks:
        count = _occurrences(text, old)
        if count != 1:
            raise Refused(f"a hunk of {path} occurs {count} times, not exactly once")
        text = text.replace(old, new, 1)
    return text


def base_text(path: str, rev: str | None = None) -> str:
    """A repo-relative file's text: at HEAD, refused unless it is tracked and equal to HEAD; or at `rev`, refused
    unless it exists there."""
    if rev is not None:
        if _git("cat-file", "-e", f"{rev}:{path}").exit != 0:
            raise Refused(f"{path} does not exist at {rev[:12]}")
        return _git_ok("show", f"{rev}:{path}").stdout
    if _git("ls-files", "--error-unmatch", "--", path).exit != 0:
        raise Refused(f"{path} is not tracked")
    if _git("diff", "--quiet", "HEAD", "--", path).exit != 0:
        raise Refused(f"{path} differs from HEAD")
    with open(path, "rb") as f:
        return f.read().decode("utf-8", errors="surrogateescape")


def changes_of(spec: Mapping[str, object], rev: str | None = None) -> tuple[Change, ...]:
    """The mutation's changed files, on HEAD's file or `rev`'s blob: one from file, find and replace, or one per path
    of the patch, the hunks of a path named in several ---/+++ pairs merged in order."""
    if "patch" in spec:
        hunks: dict[str, list[tuple[str, str]]] = {}
        for path, found in parse_patch(str(spec["patch"])):
            hunks.setdefault(path, []).extend(found)
        changes = []
        for path, found in hunks.items():
            original = base_text(path, rev)
            changes.append(Change(path, original, apply(path, original, found)))
        return tuple(changes)
    file, find = str(spec["file"]), str(spec["find"])
    if not file or os.path.isabs(file) or ".." in file.split("/"):
        raise Refused(f"--file {file!r} is not a repo-relative path")
    if not find:
        raise Refused("--find is empty")
    original = base_text(file, rev)
    count = _occurrences(original, find)
    if count != 1:
        raise Refused(f"--find occurs {count} times in {file}, not exactly once")
    return (Change(file, original, original.replace(find, str(spec["replace"]), 1)),)


def _entry_timeout(value: object, default: str) -> str:
    """An entry's "timeout": a Go duration string or positive seconds; the run's --timeout when absent."""
    if value is None:
        return default
    if isinstance(value, str):
        seconds(value)
        return value
    if isinstance(value, int | float) and not isinstance(value, bool) and value > 0:
        return f"{value}s"
    raise Refused(f"timeout {value!r} is not a Go duration or positive seconds")


def _side(spec: Mapping[str, object], side: str) -> Target:
    """One side's designated test: each of test, subtest and py from `<side>_<key>` if present, else `<key>`."""
    keys = ("test", "subtest", "py")
    found = [spec.get(f"{side}_{key}", spec.get(key)) for key in keys]
    return target(*(None if value is None else str(value) for value in found))


def _rev(spec: Mapping[str, object], key: str) -> str:
    """A full commit id naming a commit of this repository."""
    rev = str(spec[key])
    if not FULL_REV.fullmatch(rev):
        raise Refused(f"{key} {rev!r} is not a full commit id")
    if _git("rev-parse", "--verify", "--quiet", f"{rev}^{{commit}}").stdout.strip() != rev:
        raise Refused(f"{key} {rev} names no commit")
    return rev


def entry(spec: Mapping[str, object], timeout: str, expect: bool = False) -> Entry:
    """Every refusal of one mutation and its designated tests, before any go or Python test command: one mutation
    form (none for a control), one target per side, all Go or all Python, pkg with Go only; with --expect, an id,
    two revisions that exist and the outcome expected at each, the mutation exact-once at both."""
    keys = set(spec)
    control = spec.get("control") is True
    if control:
        if keys & MUTATION_KEYS:
            raise Refused(f"a control entry has no mutation: {sorted(keys & MUTATION_KEYS)}")
    elif "patch" in keys and keys & TRIPLE:
        raise Refused("the mutation is file, find and replace, or patch: not both")
    elif "patch" not in keys and TRIPLE - keys:
        raise Refused(f"the mutation is file, find and replace, or patch: missing {', '.join(sorted(TRIPLE - keys))}")
    old, new = _side(spec, "old"), _side(spec, "new")
    if bool(old.py) != bool(new.py):
        raise Refused("an entry is all Go or all Python")
    pkg = None if spec.get("pkg") is None else str(spec["pkg"])
    if new.py and pkg is not None:
        raise Refused("pkg names a Go package: not with py")
    if pkg is not None:
        if not pkg or os.path.isabs(pkg) or ".." in pkg.split("/"):
            raise Refused(f"--pkg {pkg!r} is not a repo-relative directory")
        if not os.path.isdir(pkg) or not _git("ls-files", "--", pkg).stdout.strip():
            raise Refused(f"--pkg {pkg!r} is not a directory holding tracked files")
    extra: dict[str, object] = {}
    if expect:
        missing = [key for key in EXPECT_REQUIRED if key not in keys]
        if missing:
            raise Refused(f"an --expect entry needs id, old_rev, new_rev and expect: missing {', '.join(missing)}")
        old_rev, new_rev = _rev(spec, "old_rev"), _rev(spec, "new_rev")
        words = spec["expect"]
        assert isinstance(words, dict)  # read_batch checked its shape
        changes = () if control else changes_of(spec, new_rev)
        extra = {
            "old_rev": old_rev,
            "new_rev": new_rev,
            "expect": (words["old"], words["new"]),
            "control": control,
            "old_changes": () if control else changes_of(spec, old_rev),
        }
    else:
        changes = changes_of(spec)
    if new.py:
        pkg = ""
    elif pkg is None:
        if not changes:
            raise Refused("a Go control entry names its pkg")
        pkg = os.path.dirname(changes[0].file) or "."
    note, ident = str(spec.get("note", "")), str(spec.get("id", ""))
    return Entry(ident, changes, old, new, pkg, note, timeout, **extra)


def _value_types(spec: Mapping[str, object]) -> None:
    """expect: exactly an old and a new outcome word; control: the literal true; timeout: checked by
    _entry_timeout; every other value a string."""
    for key, value in spec.items():
        if key == "expect":
            if (
                not isinstance(value, dict)
                or set(value) != {"old", "new"}
                or not all(word in OUTCOMES for word in value.values())
            ):
                raise Refused(f'expect {value!r} is not {{"old": <outcome>, "new": <outcome>}}')
        elif key == "control":
            if value is not True:
                raise Refused(f"control {value!r} is not true")
        elif key != "timeout" and not isinstance(value, str):
            raise Refused(f"{key}: every value but expect, control and timeout must be a string")


def read_batch(path: str, expect: bool, default_timeout: str) -> list[Entry]:
    """Every entry of a plain batch or an --expect batch, each checked before the first build or worktree."""
    try:
        with open(path, "rb") as f:
            specs = json.loads(f.read().decode("utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as e:
        raise Refused(f"batch {path}: {e}") from None
    if not isinstance(specs, list) or not specs:
        raise Refused(f"batch {path}: not a non-empty list of entries")
    entries: list[Entry] = []
    for index, spec in enumerate(specs, 1):
        if not isinstance(spec, dict):
            raise Refused(f"batch entry {index}: not an object")
        unknown = sorted(set(spec) - EXPECT_KEYS)
        if unknown:
            raise Refused(f"batch entry {index}: unknown keys {unknown}")
        if not expect and set(spec) - PLAIN_KEYS:
            raise Refused(f"batch entry {index}: an --expect key in a plain batch: {sorted(set(spec) - PLAIN_KEYS)}")
        try:
            _value_types(spec)
            entries.append(entry(spec, _entry_timeout(spec.get("timeout"), default_timeout), expect))
        except Refused as e:
            raise Refused(f"batch entry {index}: {e}") from None
        if expect and entries[-1].id in {e.id for e in entries[:-1]}:
            raise Refused(f"batch entry {index}: id {entries[-1].id!r} is not unique")
    return entries


def read_patch(path: str) -> str:
    try:
        with open(path, "rb") as f:
            return f.read().decode("utf-8")
    except (OSError, UnicodeDecodeError) as e:
        raise Refused(f"--patch {path}: {e}") from None


def single_spec(args: argparse.Namespace) -> dict[str, str]:
    """Single mode's flags as a batch entry's keys."""
    keys = ("file", "find", "replace", "test", "subtest", "py", "pkg")
    spec = {key: getattr(args, key) for key in keys if getattr(args, key) is not None}
    if args.patch is not None:
        spec["patch"] = read_patch(args.patch)
    return spec


def diff(c: Change) -> list[str]:
    lines = difflib.unified_diff(
        c.original.splitlines(keepends=True), c.mutant.splitlines(keepends=True), f"a/{c.file}", f"b/{c.file}"
    )
    return [line.rstrip("\n") for line in lines]


def judge_go(result: gorun.Result, t: Target) -> str:
    """A subtest is judged by its own lines only: a parent FAIL without the subtest's own FAIL line is invalid."""
    text = f"{result.stdout}\n{result.stderr}"
    if "panic: test timed out after" in text:
        return "timeout"
    if not t.levels:
        if re.search(rf"^--- FAIL: {re.escape(t.test)} \(", text, re.MULTILINE):
            return "red"
        if (
            result.exit == 0
            and not result.timed_out
            and re.search(rf"^--- PASS: {re.escape(t.test)} \(", text, re.MULTILINE)
        ):
            return "green"
        return "invalid"
    name = re.escape(t.full)
    if not re.search(rf"^=== RUN\s+{name}$", text, re.MULTILINE):
        return "invalid"
    if re.search(rf"^\s*--- SKIP: {name} \(", text, re.MULTILINE):
        return "invalid"
    if re.search(rf"^\s*--- FAIL: {name} \(", text, re.MULTILINE):
        return "red"
    if (
        result.exit == 0
        and not result.timed_out
        and re.search(rf"^\s*--- PASS: {name} \(", text, re.MULTILINE)
        and not re.search(r"^\s*--- FAIL: ", text, re.MULTILINE)
    ):
        return "green"
    return "invalid"


def judge_py(result: gorun.Result, said: str, t: Target) -> str:
    """Red only when the shim's sole problem is the one failure; a failure beside a leak or a skip is invalid."""
    if result.timed_out:
        return "timeout"
    head = f"pyshim: {t.py}: "
    if (
        result.exit == 1
        and said == head + "failed: 1 failures, 0 errors"
        and re.search(r"^Ran 1 test in ", result.stderr, re.MULTILINE)
    ):
        return "red"
    if result.exit == 0 and said == head + "passed 1 tests":
        return "green"
    return "invalid"


def tail(result: gorun.Result) -> list[str]:
    lines = []
    for name, text in (("stderr", result.stderr), ("stdout", result.stdout)):
        kept = text.splitlines()[-TAIL:]
        if kept:
            lines += [f"{name} tail:", *(f"  {line}" for line in kept)]
    if result.timed_out:
        lines.append("killed by the wall clock")
    return lines


class Worktrees:
    """One detached worktree per revision for one run, reset between entries, so the Go build cache (keyed by the
    worktree's path) is reused; every one removed when the run ends, a stop included."""

    def __init__(self) -> None:
        self._trees: dict[str, tuple[str, str]] = {}  # rev -> (tmp, path)

    def __enter__(self) -> Worktrees:
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()

    def at(self, rev: str, changes: tuple[Change, ...]) -> str:
        """The worktree at `rev`, clean, with `changes` committed under the throwaway identity."""
        if rev in self._trees:
            path = self._trees[rev][1]
            _git_ok("-C", path, *NO_HOOKS, "checkout", "-q", "--force", "--detach", rev)
            _git_ok("-C", path, "clean", "-q", "-ffdx")
        else:
            tmp = tempfile.mkdtemp(prefix="mutate-wt-")  # outside the repo: pytests' misplaced() walks gate/
            path = os.path.join(tmp, "wt")
            self._trees[rev] = (tmp, path)  # recorded first, so close() removes a half-made one
            _git_ok(*NO_HOOKS, "worktree", "add", "--detach", path, rev)
        for change in changes:
            with open(os.path.join(path, change.file), "wb") as f:
                f.write(change.mutant.encode("utf-8", errors="surrogateescape"))
        if changes:
            _git_ok("-C", path, *NO_HOOKS, "add", "--", *(c.file for c in changes))
            _git_ok("-C", path, *NO_HOOKS, "commit", "-q", "-m", "mutate.py plant", env=THROWAWAY)
        return path

    def close(self) -> None:
        """Remove each worktree; then this run's own paths must be gone from `git worktree list` and its temporary
        directories from disk. Other worktrees (sibling lanes') are never compared."""
        for tmp, path in self._trees.values():
            if _git("worktree", "remove", "--force", path).exit != 0:
                shutil.rmtree(path, ignore_errors=True)
                _git("worktree", "prune")
            shutil.rmtree(tmp, ignore_errors=True)
        porcelain = _git("worktree", "list", "--porcelain").stdout.splitlines()
        listed = {
            os.path.realpath(line.removeprefix("worktree ")) for line in porcelain if line.startswith("worktree ")
        }
        left = sorted(
            path for tmp, path in self._trees.values() if os.path.realpath(path) in listed or os.path.exists(tmp)
        )
        self._trees.clear()
        if left:
            raise Internal(f"a worktree was left: {', '.join(left)}")


def _overlay(e: Entry, scratch: str, top: str) -> str:
    """The mutants outside the tree, one overlay entry per changed file."""
    replace = {}
    for change in e.changes:
        mutant = os.path.join(scratch, "mutant", change.file)
        os.makedirs(os.path.dirname(mutant), exist_ok=True)
        with open(mutant, "wb") as f:
            f.write(change.mutant.encode("utf-8", errors="surrogateescape"))
        replace[os.path.join(top, change.file)] = mutant
    overlay = os.path.join(scratch, "overlay.json")
    with open(overlay, "w", encoding="utf-8") as f:
        json.dump({"Replace": replace}, f, sort_keys=True, ensure_ascii=False)
        f.write("\n")
    return overlay


def run_go(
    e: Entry, side: Target, args: argparse.Namespace, scratch: str, cwd: str | None = None
) -> tuple[str, gorun.Result, list[str]]:
    """One Go run of `side`: through the overlay in the tracked tree (cwd None), or in a worktree holding the plant;
    a designated subtest's probe must count exactly one run first; the `ran:` lines name every run at its depth."""
    if side.levels:
        probe = _go(e, args, scratch, cwd, pattern(side, probe=True))
        n = len(runs(f"{probe.stdout}\n{probe.stderr}", side))
        if n != 1:
            return "invalid", probe, [f"probe: {n} subtests run as {side.full}"]
    result = _go(e, args, scratch, cwd, pattern(side, probe=False))
    names = ran(f"{result.stdout}\n{result.stderr}", side) if side.levels else []
    return judge_go(result, side), result, [f"ran: {name}" for name in names]


def _go(e: Entry, args: argparse.Namespace, scratch: str, cwd: str | None, run: str) -> gorun.Result:
    """go test -run `run` with the entry's -timeout: in the tracked tree with the mutants through -overlay (the
    tracked files only ever read), or in `cwd`, a worktree holding the committed plant."""
    top = cwd or os.getcwd()
    argv = ["go", "test"]
    if cwd is None:
        argv.append(f"-overlay={_overlay(e, scratch, top)}")
    argv += ["-count=1", "-v", "-run", run, "-timeout", e.timeout]
    if args.race:
        argv.append("-race")
    if args.cpu:
        argv += ["-cpu", args.cpu]
    argv.append("./" + e.pkg)
    # PWD names the real directory, so go's working directory matches the overlay's key. Never -trimpath: frame
    # tables and line-anchored tests match file paths in stack frames.
    env = {**os.environ, "PWD": top}
    return gorun.run(argv, cwd=top, env=env, timeout_s=seconds(e.timeout) + WALL_EXTRA_S)


def _said(report: str, py: str) -> str:
    """The report file's last line from the shim about `py`, or ""."""
    try:
        with open(report, "rb") as f:
            lines = f.read().decode("utf-8", errors="replace").splitlines()
    except FileNotFoundError:
        return ""
    head = f"pyshim: {py}: "
    return next((line for line in reversed(lines) if line.startswith(head)), "")


def run_py(
    e: Entry, side: Target, scratch: str, trees: Worktrees, rev: str, changes: tuple[Change, ...]
) -> tuple[str, gorun.Result, str]:
    """The id under the pytests row's shim and flags, the copy's own shim, in the worktree at `rev` holding the
    plant, with the entry's timeout as its wall clock."""
    wt = trees.at(rev, changes)
    with tempfile.TemporaryDirectory(dir=scratch) as own:  # never in the worktree: git status would see it (#13a)
        report = os.path.join(own, "report")
        argv = [PYTHON, *rows.SHIM_FLAGS, os.path.join(wt, "gate", "runner", "pyshim.py"), wt, side.py, report]
        result = gorun.run(argv, cwd=wt, env={**os.environ, "PWD": wt}, timeout_s=seconds(e.timeout))
        said = _said(report, side.py)
    return judge_py(result, said, side), result, said


def run_one(
    e: Entry, args: argparse.Namespace, scratch: str, trees: Worktrees, head: str
) -> tuple[str, gorun.Result, list[str]]:
    """One run of the entry's designated test against its mutation: the outcome, the run, and its notes (a Go
    subtest's probe or `ran:` lines; an invalid Python run's shim line)."""
    return run_side(e, e.target, args, scratch, trees, head, e.changes, None)


def run_side(
    e: Entry,
    side: Target,
    args: argparse.Namespace,
    scratch: str,
    trees: Worktrees,
    rev: str,
    changes: tuple[Change, ...],
    cwd: str | None,
) -> tuple[str, gorun.Result, list[str]]:
    """One side: Python in the worktree at `rev`; Go through the overlay (cwd None) or in the worktree `cwd`."""
    if side.py:
        outcome, result, said = run_py(e, side, scratch, trees, rev, changes)
        return outcome, result, [f"report: {said or '(no pyshim line)'}"] if outcome == "invalid" else []
    return run_go(e, side, args, scratch, cwd)


def details(outcome: str, result: gorun.Result, notes: list[str]) -> list[str]:
    """What a run shows before its outcome: the tails of an invalid one, then its notes."""
    return [*(tail(result) if outcome == "invalid" else []), *notes]


def record(python: bool) -> list[str]:
    head = _git("rev-parse", "HEAD")
    version = gorun.run(["go", "version"], cwd=os.getcwd(), timeout_s=LOOKUP_TIMEOUT_S)
    lines = [f"head: {head.stdout.strip()}", f"go: {version.stdout.strip() or version.stderr.strip()}"]
    if python:
        interpreter = gorun.run([PYTHON, "--version"], cwd=os.getcwd(), timeout_s=LOOKUP_TIMEOUT_S)
        lines.append(f"python: {interpreter.stdout.strip() or interpreter.stderr.strip()}")
    return lines


def header(e: Entry, head: str = "") -> list[str]:
    """The entry's designated test (and, given the head, single mode's copy line), then each file's diff."""
    if e.target.py:
        named = [f"py: {e.target.py}", *([f"copy: detached worktree at {head}"] if head else [])]
    else:
        named = [f"test: {e.target.test}", f"package: ./{e.pkg}"]
        if e.target.levels:
            named.append(f"subtest: {e.target.full}")
    return [*named, f"timeout: {e.timeout}", *(line for change in e.changes for line in diff(change))]


def expect_header(e: Entry) -> list[str]:
    """An --expect entry's designated tests at each revision, its timeout, and its plant at the new revision."""
    named = [
        f"{side}: {t.py or t.full} at {rev[:12]}"
        for side, t, rev in (("old", e.old, e.old_rev), ("new", e.new, e.new_rev))
    ]
    package = [] if e.new.py else [f"package: ./{e.pkg}"]
    plant = ["control: the real code"] if e.control else [line for change in e.changes for line in diff(change)]
    return [f"entry {e.id}", f"note: {e.note}", *named, *package, f"timeout: {e.timeout}", *plant]


def _parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = _Parser(prog="gate/bench/mutate.py")
    parser.add_argument("--file")
    parser.add_argument("--find")
    parser.add_argument("--replace")
    parser.add_argument("--patch")
    parser.add_argument("--test")
    parser.add_argument("--subtest")
    parser.add_argument("--py")
    parser.add_argument("--pkg")
    parser.add_argument("--batch")
    parser.add_argument("--expect")
    parser.add_argument("--timeout", default="120s")
    parser.add_argument("--race", action="store_true")
    parser.add_argument("--cpu")
    parser.add_argument("--repeat", type=int)
    parser.add_argument("--load", action="store_true")
    args = parser.parse_args(argv)
    singles = (args.file, args.find, args.replace, args.patch, args.test, args.subtest, args.py, args.pkg, args.repeat)
    if args.batch is not None and args.expect is not None:
        raise Refused("--batch and --expect exclude each other")
    if (args.batch, args.expect) != (None, None) and (any(value is not None for value in singles) or args.load):
        raise Refused(
            "--batch and --expect take no --file, --find, --replace, --patch, --test, --subtest, --py, --pkg, --repeat"
            " or --load"
        )
    if (args.repeat is None) != (not args.load):
        raise Refused("--repeat N and --load go together")
    if args.repeat is not None and args.repeat < 1:
        raise Refused("--repeat must be at least 1")
    seconds(args.timeout)
    return args


def _print(lines: Sequence[str]) -> None:
    for line in lines:
        print(line)
    sys.stdout.flush()


def single(e: Entry, args: argparse.Namespace, scratch: str, trees: Worktrees, head: str) -> tuple[int, list[str]]:
    _print([*header(e, head), *record(bool(e.target.py))])
    outcome, result, notes = run_one(e, args, scratch, trees, head)
    return (0 if outcome == "red" else 1), [*details(outcome, result, notes), f"outcome: {outcome}"]


def repeat(e: Entry, args: argparse.Namespace, scratch: str, trees: Worktrees, head: str) -> tuple[int, list[str]]:
    _print([*header(e, head), *record(bool(e.target.py))])
    caught = 0
    with stress.hogs(stress.hog_count(), sys.executable) as procs:
        _print([f"load: {len(procs)} hogs, pids {' '.join(str(p) for p in sorted(p.pid for p in procs))}"])
        for n in range(1, args.repeat + 1):
            outcome, _, _ = run_one(e, args, scratch, trees, head)
            caught += outcome == "red"
            _print([f"run {n}/{args.repeat}: {outcome}"])
    return (0 if caught else 1), [f"caught {caught}/{args.repeat}"]


def batch(
    entries: list[Entry], args: argparse.Namespace, scratch: str, trees: Worktrees, head: str
) -> tuple[int, list[str]]:
    _print(record(any(e.target.py for e in entries)))
    counts = dict.fromkeys(OUTCOMES, 0)
    for index, e in enumerate(entries, 1):
        _print([f"entry {index}", f"note: {e.note}", *header(e)])
        with tempfile.TemporaryDirectory(dir=scratch) as own:
            outcome, result, notes = run_one(e, args, own, trees, head)
        counts[outcome] += 1
        _print([*details(outcome, result, notes), f"entry {index}: {outcome}"])
    summary = ", ".join(f"{name} {counts[name]}" for name in OUTCOMES)
    return (0 if counts["red"] == len(entries) else 1), [f"counts: {summary}"]


def expect(entries: list[Entry], args: argparse.Namespace, scratch: str, trees: Worktrees) -> tuple[int, list[str]]:
    """Each entry at its old and its new revision, in the run's one worktree per revision (reset between entries);
    exit 0 only when every outcome is the one expected."""
    _print(record(any(e.new.py for e in entries)))
    outcomes: dict[str, tuple[str, ...]] = {}
    for e in entries:
        _print(expect_header(e))
        got = []
        sides = (
            ("old", e.old_rev, e.old, e.expect[0], e.old_changes),
            ("new", e.new_rev, e.new, e.expect[1], e.changes),
        )
        for name, rev, side, want, changes in sides:
            with tempfile.TemporaryDirectory(dir=scratch) as own:
                wt = None if side.py else trees.at(rev, changes)
                outcome, result, notes = run_side(e, side, args, own, trees, rev, changes, wt)
            got.append(outcome)
            _print([*details(outcome, result, notes), f"entry {e.id} {name} {rev[:12]}: {outcome} (expected {want})"])
        outcomes[e.id] = tuple(got)
    differing = [e.id for e in entries if outcomes[e.id] != e.expect]
    kept = len(entries) - len(differing)
    summary = f"expect: {len(entries)} entries, {kept} as expected; differing: {', '.join(differing) or 'none'}"
    return (1 if differing else 0), [summary]


def mutate(args: argparse.Namespace) -> tuple[int, list[str]]:
    if args.expect is not None:
        entries = read_batch(args.expect, True, args.timeout)
    elif args.batch is not None:
        entries = read_batch(args.batch, False, args.timeout)
    else:
        entries = [entry(single_spec(args), args.timeout)]
    python = any(e.old.py or e.new.py for e in entries)
    if python and (args.race or args.cpu):
        raise Refused("--race and --cpu apply to go test: not with py")
    head = _git_ok("rev-parse", "--verify", "HEAD^{commit}").stdout.strip() if python else ""
    with tempfile.TemporaryDirectory(prefix="mutate-") as scratch, Worktrees() as trees:
        if args.expect is not None:
            return expect(entries, args, scratch, trees)
        if args.batch is not None:
            return batch(entries, args, scratch, trees, head)
        if args.repeat is not None:
            return repeat(entries[0], args, scratch, trees, head)
        return single(entries[0], args, scratch, trees, head)


def main(argv: Sequence[str]) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    term.install()
    try:
        code, lines = mutate(_parse_args(argv))
    except Helped:
        return 0
    except Refused as e:
        code, lines = 2, [f"error: refused: {e}"]
    except BaseException as e:  # every other failure, a stop included, is exit 2 with its reason
        print(f"error: internal error: {type(e).__name__}: {e}", flush=True)
        raise SystemExit(2) from e
    _print(lines)
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
