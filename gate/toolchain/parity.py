"""Parity: the gate's first step in every job, and its one hard stop (design note §3).

The runner calls `check` and records its result as the per-job parity record. Nothing here writes to the
cache, and nothing here runs a binary that resolves outside its own tool directory.
"""

import contextlib
import json
import os
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field

from gate.runner import records
from gate.toolchain import manifest
from gate.toolchain.manifest import Tool

RECOVER = "run make clean-tools tools"

# A python-package's version probe, run by the pinned interpreter: argv[1] is the package's sys.path directory,
# argv[2] its import name.
PROBE = (
    "import importlib, sys; sys.path.insert(0, sys.argv[1]); print(importlib.import_module(sys.argv[2]).__version__)"
)

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# BA10 (S1.1c-3 design §3.6; owner, S1.1c-3's plan check, decision 10): the tree hash runs in a child of its own, so a
# stop can end it and a cap bounds it. Both names are read when tree_digest is called, never bound at import (DA-S12).
TREE_CHILD: tuple[str, ...] = (sys.executable, "-I", "-B", os.path.join(REPO, "gate", "toolchain", "treehash.py"))
# Parity's measured durations in seconds (S1.1c-4 design note §3.4; owner, 2026-11-13, S1.1c-4's plan check, decision
# 3). `ci-<run>/<job>`: parity's whole duration (every tool's probe and tree hash; records/<job>/parity.json,
# started_ms to ended_ms) in the 24 jobs of runs 43415818780 and 43417233168 (PR #38) and 43416445889 and 43417289965
# (main), the trees just restored by actions/cache. `local-<round>/<n>`: gate/toolchain/treehash.py over go's tree
# alone (the slowest tool, 15,660 files; 2.51-2.64 s on a quiet machine, 2026-11-12), each a fresh process, on an
# M-series Mac (macOS, APFS SSD), 2026-11-13, ten rounds each, all with four agents' full gates loading the machine
# (load average 12-19 on 14 cores): `idle` with nothing more of ours; `during-proofs` beside a local
# `--tier T4 --job proofs` run; `load` beside fourteen `yes` processes, one per core, as that proofs run went on.
# `local-T4-proofs/parity`: that proofs run's own parity record. A slower CI figure from a later run raises
# SLOWEST_PARITY_S and the cap (decision 3; design note §3.4, after the push).
PARITY_MEASURED_S: dict[str, float] = {
    "ci-43415818780/determinism": 4.125,
    "ci-43415818780/determinism-macos": 10.357,
    "ci-43415818780/lint": 4.646,
    "ci-43415818780/proofs": 2.858,
    "ci-43415818780/race": 3.581,
    "ci-43415818780/test": 4.839,
    "ci-43416445889/determinism": 2.93,
    "ci-43416445889/determinism-macos": 5.119,
    "ci-43416445889/lint": 4.647,
    "ci-43416445889/proofs": 4.086,
    "ci-43416445889/race": 3.614,
    "ci-43416445889/test": 4.491,
    "ci-43417233168/determinism": 3.399,
    "ci-43417233168/determinism-macos": 5.672,
    "ci-43417233168/lint": 4.635,
    "ci-43417233168/proofs": 4.502,
    "ci-43417233168/race": 4.577,
    "ci-43417233168/test": 3.741,
    "ci-43417289965/determinism": 4.184,
    "ci-43417289965/determinism-macos": 8.306,
    "ci-43417289965/lint": 4.544,
    "ci-43417289965/proofs": 3.545,
    "ci-43417289965/race": 2.904,
    "ci-43417289965/test": 3.679,
    "local-T4-proofs/parity": 9.514,
    "local-idle/1": 5.74,
    "local-idle/2": 4.58,
    "local-idle/3": 5.88,
    "local-idle/4": 3.31,
    "local-idle/5": 3.86,
    "local-idle/6": 3.17,
    "local-idle/7": 2.43,
    "local-idle/8": 2.83,
    "local-idle/9": 6.29,
    "local-idle/10": 2.82,
    "local-during-proofs/1": 3.14,
    "local-during-proofs/2": 4.7,
    "local-during-proofs/3": 5.3,
    "local-during-proofs/4": 4.7,
    "local-during-proofs/5": 5.95,
    "local-during-proofs/6": 9.59,
    "local-during-proofs/7": 5.4,
    "local-during-proofs/8": 6.33,
    "local-during-proofs/9": 9.39,
    "local-during-proofs/10": 5.62,
    "local-load/1": 6.49,
    "local-load/2": 4.98,
    "local-load/3": 3.42,
    "local-load/4": 3.78,
    "local-load/5": 4.05,
    "local-load/6": 4.87,
    "local-load/7": 5.1,
    "local-load/8": 5.75,
    "local-load/9": 3.66,
    "local-load/10": 3.13,
}
SLOWEST_PARITY_S: float = max(PARITY_MEASURED_S.values())
# At least ten times the slowest measured parity (decision 3; test_parity.TreeCap guards both the ratio and that the
# slowest is read from the table): ceil(10 x 10.357 / 60) x 60 = ceil(1.726) x 60 = 120 s.
TREE_CAP_S: float = 120
TREE_GRACE_S = 5  # after the cap's KILL, how long to wait for the child to be reaped before giving up on it
TREE_UNREADABLE = 3  # treehash.UNREADABLE: the child could not read the tree (an OSError), the one damaged-tool exit
DETAILS = "--details"  # treehash.DETAILS: the child's second form, each root's realpath and details

# The tree hand-off (proof-world speed-up design note §3.4-§3.5; owner, 2026-11-14, the speed-up's measurement,
# decision 2, and its design reviews, decisions 2-3): a full-tree check whose every tool passed lists the trees it
# hashed in its scratch (the run's parity/), and a nested run given that file with --parity-handoff takes a tree
# without hashing it only when its name, realpath, this run's own pin and a fresh details digest all match.
HANDOFF = "handoff.json"
HANDOFF_MAX = 64 * 1024  # bytes: six tools take about 1 KiB
HANDOFF_SCHEMA = 1
DETAILS_FORMAT = 1  # manifest.tree_details's line format
_HANDOFF_KEYS = frozenset({"details_format", "run_id", "schema", "tools"})
_ENTRY_KEYS = frozenset({"details", "home", "name", "tree_sha256"})


class TreeCapExceeded(OSError):
    """The tree hash outlived its cap; the child's own process group was killed."""

    def __init__(self, cap_s: float) -> None:
        super().__init__(f"tree hash exceeded its cap of {cap_s} s")
        self.cap_s = cap_s


class TreeStopped(Exception):
    """The spawner refused to start the tree child: the run's parity group was already signalled (a stop)."""


class TreeStartFailed(OSError):
    """The spawner raised while starting the tree child (EMFILE, ENOMEM, a missing interpreter): no hash ran (BK-S5)."""


class TreeChildFailed(OSError):
    """The tree child ran and exited non-zero or died by a signal (`code` as Popen gives it, negative for a signal);
    `first_line` is the first line of its stderr, "" when it wrote none (BK-S5, CA-L4)."""

    def __init__(self, code: int, first_line: str) -> None:
        super().__init__(f"the tree child exited {code}" + (f": {first_line}" if first_line else ""))
        self.code = code
        self.first_line = first_line


class HandoffFault(Exception):
    """The hand-off given is not one parity reads: unopenable, not a regular file, too large, not canonical JSON, or
    outside its grammar. Parity fails naming it, before any tool is checked (design note §3.5, QB-S10)."""


@dataclass(frozen=True)
class Outcome:
    """A check's result (design note §3.3). The runner's parity reason is the reasons joined, or `covered <n>` and,
    when trees were taken, `; <taken> trees taken from run <taken_from>`."""

    reasons: tuple[str, ...]  # one per failing tool or a single hand-off fault; () means pass
    covered: int  # tools that passed (a hand-off fault: 0) — CA-L4
    taken: int  # trees taken from the hand-off (0 when none was given or none matched)
    taken_from: str  # the hand-off's run id when taken > 0, else ""


@dataclass(frozen=True)
class Entry:
    """One tool of a hand-off: the tree an earlier check hashed, as its child resolved and read it before the hash,
    and the run that wrote it."""

    name: str
    home: str  # the realpath the tree child resolved
    tree_sha256: str
    details: str
    run_id: str


type Spawn = Callable[..., subprocess.Popen[bytes] | None]


def popen(
    argv: Sequence[str], *, cwd: str, env: Mapping[str, str], stdout: str, stderr: str
) -> subprocess.Popen[bytes]:
    """A spawner outside the runner: a plain Popen in a new session, with Group.spawn's shape (DA-S13a)."""
    with open(stdout, "wb") as out, open(stderr, "wb") as err:
        return subprocess.Popen(
            list(argv), cwd=cwd, env=dict(env), stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True
        )


@dataclass(frozen=True)
class TreeResult:
    """What the tree child prints for one home: the realpath it resolved once, that tree's details digest (read
    before the hash) and its tree hash (proof-world speed-up design note §3.2)."""

    real: str
    details: str
    digest: str


NO_RESULT = TreeResult("", "", "")  # a child that printed fewer than three lines: it matches no pin


def tree_digest(
    home: str,
    *,
    spawn: Spawn,
    cap_s: float | None = None,
    argv: Sequence[str] | None = None,
    env: Mapping[str, str] | None = None,
    scratch: str | None = None,
    name: str = "tool",
) -> TreeResult:
    """What the tree child prints for `home`: `[*argv, home]` started through `spawn` (TREE_CHILD and TREE_CAP_S when
    not given, read now), its output in `<scratch>/tree-<name>.out` and `.err`. The three lines are read from the
    end, so a realpath holding a newline is still read whole; fewer than three is NO_RESULT, which matches no pin, as
    a garbled digest never did.

    Raises TreeStopped when `spawn` returns None, TreeStartFailed when it raises OSError, TreeCapExceeded past the cap
    (after killing the child's own process group, never through the runner's Group, which would refuse every later
    spawn), and TreeChildFailed when the child exits non-zero or by a signal. The runner's Group forgets a leader once
    anyone has reaped it, so the capped wait here leaves it no live group to signal (BK-L2).
    """
    cap = TREE_CAP_S if cap_s is None else cap_s
    command = [*(TREE_CHILD if argv is None else argv), home]
    with contextlib.ExitStack() as stack:
        if scratch is None:
            scratch = stack.enter_context(tempfile.TemporaryDirectory(prefix="gate-tree-"))
        out = os.path.join(scratch, f"tree-{name}.out")
        err = os.path.join(scratch, f"tree-{name}.err")
        text = _capped(command, spawn=spawn, env=env, out=out, err=err, cap=cap, name=name)
    lines = text.removesuffix("\n").rsplit("\n", 2)
    return TreeResult(*lines) if len(lines) == 3 else NO_RESULT


def _capped(
    command: Sequence[str], *, spawn: Spawn, env: Mapping[str, str] | None, out: str, err: str, cap: float, name: str
) -> str:
    """Start one tree child through `spawn`, wait at most `cap`, and return its stdout; raise as tree_digest says."""
    try:
        proc = spawn(command, cwd=REPO, env=os.environ if env is None else env, stdout=out, stderr=err)
    except OSError as e:
        raise TreeStartFailed(str(e)) from e
    if proc is None:
        raise TreeStopped(name)
    try:
        code = proc.wait(timeout=cap)
    except subprocess.TimeoutExpired:
        # The leader is not reaped yet, so its group id is still its own; a group left with only a zombie answers
        # EPERM on macOS, which means there is nothing left to kill.
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(proc.pid, signal.SIGKILL)
        with contextlib.suppress(subprocess.TimeoutExpired):
            proc.wait(timeout=TREE_GRACE_S)
        raise TreeCapExceeded(cap) from None
    if code != 0:
        with open(err, "rb") as f:
            first = f.read().split(b"\n", 1)[0].decode("utf-8", errors="replace").strip()
        raise TreeChildFailed(code, first)
    with open(out, "rb") as f:
        return f.read().decode("utf-8", errors="replace")


def tree_details_of(
    homes: Sequence[str], *, spawn: Spawn, env: Mapping[str, str], scratch: str
) -> list[tuple[str, str]]:
    """Each home's realpath and details digest, in order, from one capped child (CA-L2, QB-S9):
    `[*TREE_CHILD, "--details", *homes]` started through `spawn` with TREE_CAP_S as its cap (both read now), its
    output in `<scratch>/details.out` and `.err`. Raises as tree_digest does. A child that printed other than two
    lines per home gives ("", "") for every home, which matches no entry: each is then hashed for real."""
    command = [*TREE_CHILD, DETAILS, *homes]
    out = os.path.join(scratch, "details.out")
    err = os.path.join(scratch, "details.err")
    text = _capped(command, spawn=spawn, env=env, out=out, err=err, cap=TREE_CAP_S, name="details")
    lines = text.removesuffix("\n").split("\n")
    if len(lines) != 2 * len(homes):
        return [("", "")] * len(homes)
    return [(lines[2 * i], lines[2 * i + 1]) for i in range(len(homes))]


@dataclass(frozen=True)
class Timing:
    """How long a version probe may run. Time only breaks a hang: a version line takes milliseconds.

    `wait` is the one place a probe waits, injected by tests so that a cap fires only after a witness.
    """

    cap_s: float = 60
    grace_s: float = 5
    wait: Callable[[subprocess.Popen, float], int] = subprocess.Popen.wait


@dataclass(frozen=True)
class Tree:
    """Where the tree children start and write: the spawner, the run's clean env and the scratch directory; and
    `listed`, each tree hashed for real that matched its pin, as the hand-off lists it (design note §3.4)."""

    spawn: Spawn
    env: Mapping[str, str]
    scratch: str
    listed: list[dict[str, str]] = field(default_factory=list)


def check(
    tools: list[Tool],
    cache: str,
    env: Mapping[str, str],
    *,
    full_tree: bool,
    spawn: Spawn,
    scratch: str,
    run_id: str,
    handoff_in: str | None,
) -> Outcome:
    """Check every tool; the outcome holds one reason per failing tool, in tools.json order (none means pass).

    Per tool, short-circuit in order: not installed → path (shutil.which(command) under env["PATH"],
    realpath-compared) → version (version_argv run in `env`, first stdout line exact) → sha256 of every
    exec_sha256 entry → (only if full_tree, i.e. T2 and above) tree_sha256. Never runs a binary that
    lies outside the cache. A python-package has no command: its path check is that its path_dir stays
    inside its home, and its version is `<module>.__version__` imported by the pinned interpreter.

    The tree hash runs in a child per tool started through `spawn` (the runner passes its parity group's spawn,
    so a stop ends it), its output in `scratch` (the runner's run directory's parity/). Both are required (design
    §10 C8; CA-L1): a default would let a caller run the tree child outside the parity group, where no stop reaches it.

    At full tree (proof-world speed-up design note §3.4-§3.5): `handoff_in`, when given, is read first, and a fault
    in it fails parity alone, naming it. A tool whose entry matches its name, this run's own pin and its home's
    realpath is a candidate; one capped details child reads every candidate, and each whose realpath and details
    match its entry is taken, with no tree child. Every other tool is hashed for real. When every tool passed and at
    least one tree was hashed, `<scratch>/handoff.json` lists those, naming `run_id`. Below full tree the hand-off is
    never read and none is written. Both are required too: a default would drop the run's id or a hand-off silently.
    """
    return _check(
        tools,
        cache,
        env,
        full_tree=full_tree,
        timing=Timing(),
        spawn=spawn,
        scratch=scratch,
        run_id=run_id,
        handoff_in=handoff_in,
    )


def _check(
    tools: list[Tool],
    cache: str,
    env: Mapping[str, str],
    *,
    full_tree: bool,
    timing: Timing,
    spawn: Spawn,
    scratch: str,
    run_id: str,
    handoff_in: str | None,
) -> Outcome:
    entries: dict[str, Entry] = {}
    if full_tree and handoff_in is not None:
        try:
            entries = _read_handoff(handoff_in)
        except HandoffFault as e:
            return Outcome(reasons=(f"parity: tree hand-off {handoff_in}: {e}",), covered=0, taken=0, taken_from="")
    goos, goarch = manifest.platform_key().split("-")
    python = next((tool for tool in tools if tool.name == manifest.GATE_PYTHON), None)
    homes = [manifest.tool_home(cache, tool) for tool in tools]
    with tempfile.TemporaryDirectory(prefix="gate-parity-") as probes:
        firsts = [
            _first_failure(tool, home, goos, goarch, python, cache, env, probes, timing)
            for tool, home in zip(tools, homes, strict=True)
        ]
    tree = Tree(spawn=spawn, env=env, scratch=scratch)
    taken: set[int] = set()
    if full_tree:
        candidates = [
            i
            for i, (tool, home) in enumerate(zip(tools, homes, strict=True))
            if firsts[i] is None and _matches(entries.get(tool.name), tool, os.path.realpath(home))
        ]
        taken = _taken(candidates, [homes[i] for i in candidates], [entries[tools[i].name] for i in candidates], tree)
    reasons = []
    for i, (tool, home) in enumerate(zip(tools, homes, strict=True)):
        reason = firsts[i]
        if reason is None and full_tree and i not in taken:
            reason = _tree_failure(tool, home, tree)
        if reason is not None:
            reasons.append(reason)
    if not reasons and tree.listed:
        _write_handoff(scratch, run_id, tree.listed)
    taken_from = next(iter(entries.values())).run_id if taken else ""
    return Outcome(reasons=tuple(reasons), covered=len(tools) - len(reasons), taken=len(taken), taken_from=taken_from)


def _first_failure(
    tool: Tool,
    home: str,
    goos: str,
    goarch: str,
    python: Tool | None,
    cache: str,
    env: Mapping[str, str],
    probes: str,
    timing: Timing,
) -> str | None:
    """Every check before the tree hash, short-circuit in order: installed, path, version, exec_sha256."""
    if not os.path.isdir(home):
        return f"{tool.name}: not installed: run make tools"
    want = tool.version_line.replace("{goos}", goos).replace("{goarch}", goarch)
    if tool.kind == manifest.PYTHON_PACKAGE:
        first = _package_path_failure(tool, home) or _import_failure(
            tool, home, want, python, cache, env, probes, timing
        )
    else:
        first = _path_failure(tool, home, env) or _version_failure(tool, home, want, env, probes, timing)
    return first or _hash_failure(tool, home)


def _matches(entry: Entry | None, tool: Tool, real: str) -> bool:
    """A candidate to take: an entry for this tool, at this run's own pin, for the tree its home resolves to now."""
    return entry is not None and entry.tree_sha256 == tool.tree_sha256 and entry.home == real


def _taken(candidates: list[int], homes: list[str], entries: list[Entry], tree: Tree) -> set[int]:
    """The candidates whose fresh details, from one capped child, match their entries; none if that child failed."""
    if not candidates:
        return set()
    try:
        read = tree_details_of(homes, spawn=tree.spawn, env=tree.env, scratch=tree.scratch)
    except TreeStopped, OSError:  # every candidate is hashed for real, and a stop names itself there
        return set()
    return {
        i
        for i, entry, (real, details) in zip(candidates, entries, read, strict=True)
        if real == entry.home and details == entry.details
    }


def _read_handoff(path: str) -> dict[str, Entry]:
    """The hand-off's entries by tool name, read once (design note §3.5); HandoffFault names what is wrong.

    Opened without blocking and without following a link, as manifest.sha256_file does, and refused unless it is a
    regular file of at most HANDOFF_MAX bytes: a FIFO planted at the path would otherwise block parity for ever.
    """
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW | os.O_CLOEXEC)
    except OSError as e:
        raise HandoffFault(e.strerror or str(e)) from None
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise HandoffFault("not a regular file")
        data = b""
        while len(data) <= HANDOFF_MAX:
            chunk = os.read(fd, HANDOFF_MAX + 1 - len(data))
            if not chunk:
                break
            data += chunk
    except OSError as e:
        raise HandoffFault(e.strerror or str(e)) from None
    finally:
        os.close(fd)
    if len(data) > HANDOFF_MAX:
        raise HandoffFault(f"larger than {HANDOFF_MAX} bytes")
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        raise HandoffFault("not UTF-8") from None
    try:
        doc = json.loads(text)
        canonical = records.json_bytes(doc) == data
    except ValueError, RecursionError:
        raise HandoffFault("not JSON") from None
    if not canonical:
        raise HandoffFault("not canonical JSON")
    return _entries(doc)


def _entries(doc: object) -> dict[str, Entry]:
    if type(doc) is not dict or doc.keys() != _HANDOFF_KEYS:
        raise HandoffFault(f"keys must be {', '.join(sorted(_HANDOFF_KEYS))}")
    for key, want in (("schema", HANDOFF_SCHEMA), ("details_format", DETAILS_FORMAT)):
        if type(doc[key]) is not int or doc[key] != want:
            raise HandoffFault(f"{key} must be {want}, not {doc[key]!r}")
    run_id = doc["run_id"]
    if type(run_id) is not str or not records.RUN_ID.fullmatch(run_id) or run_id in (".", ".."):
        raise HandoffFault(f"run_id {run_id!r} is not a run id")
    if type(doc["tools"]) is not list:
        raise HandoffFault("tools must be a list")
    entries: dict[str, Entry] = {}
    for i, item in enumerate(doc["tools"]):
        where = f"tools[{i}]"
        if type(item) is not dict or item.keys() != _ENTRY_KEYS:
            raise HandoffFault(f"{where} keys must be {', '.join(sorted(_ENTRY_KEYS))}")
        name, home = item["name"], item["home"]
        if type(name) is not str:
            raise HandoffFault(f"{where} name must be a string")
        if name in entries:
            raise HandoffFault(f"{where} name {name!r} is listed twice")
        if type(home) is not str or not os.path.isabs(home):
            raise HandoffFault(f"{where} home must be an absolute path")
        for key in ("tree_sha256", "details"):
            if type(item[key]) is not str or not manifest._DIGEST.fullmatch(item[key]):
                raise HandoffFault(f"{where} {key} must be 64 lowercase hex digits")
        entries[name] = Entry(name, home, item["tree_sha256"], item["details"], run_id)
    return entries


def _write_handoff(scratch: str, run_id: str, listed: Sequence[Mapping[str, str]]) -> None:
    """`<scratch>/handoff.json`, canonical, read-only before it takes its name (QB-L3): a reader sees it whole or
    not at all. Exclusive creation: nothing planted at the temporary name is followed or reused."""
    doc = {"details_format": DETAILS_FORMAT, "run_id": run_id, "schema": HANDOFF_SCHEMA, "tools": list(listed)}
    path = os.path.join(scratch, HANDOFF)
    tmp = f"{path}.tmp"
    with open(tmp, "xb") as f:
        f.write(records.json_bytes(doc))
    os.chmod(tmp, 0o444)
    os.rename(tmp, path)


def _inside(resolved: str, home: str) -> bool:
    """Is this realpath inside the tool's directory? Both sides are resolved: on macOS, /tmp is a symlink."""
    return resolved.startswith(os.path.realpath(home) + os.sep)


def _path_failure(tool: Tool, home: str, env: Mapping[str, str]) -> str | None:
    found = shutil.which(tool.command, path=env.get("PATH", ""))
    if found is None:
        return f"{tool.name}: path: {tool.command} is not on PATH: {RECOVER}"
    for candidate in (found, os.path.join(home, tool.version_argv[0])):
        resolved = os.path.realpath(candidate)
        if not _inside(resolved, home):
            return f"{tool.name}: path: {resolved} is outside the cache: {RECOVER}"
    return None


def _signal_group(pgid: int, sig: signal.Signals) -> None:
    with contextlib.suppress(ProcessLookupError):
        os.killpg(pgid, sig)


def _run(argv: list[str], cwd: str, env: Mapping[str, str], scratch: str, timing: Timing) -> tuple[int | None, bytes]:
    """Run one command in its own process group, output to files; return (exit code or None if hung, stdout)."""
    stdout = os.path.join(scratch, "stdout")
    with open(stdout, "wb") as out, open(os.path.join(scratch, "stderr"), "wb") as err:
        proc = subprocess.Popen(
            argv, cwd=cwd, env=dict(env), stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True
        )
    code: int | None = None
    try:
        code = timing.wait(proc, timing.cap_s)
    except subprocess.TimeoutExpired:
        _signal_group(proc.pid, signal.SIGTERM)
        with contextlib.suppress(subprocess.TimeoutExpired):
            timing.wait(proc, timing.grace_s)
    _signal_group(proc.pid, signal.SIGKILL)  # nothing a probe started may outlive it
    proc.wait()
    with open(stdout, "rb") as f:
        return code, f.read()


def _package_path_failure(tool: Tool, home: str) -> str | None:
    resolved = os.path.realpath(os.path.join(home, tool.path_dir))
    if resolved != os.path.realpath(home) and not _inside(resolved, home):
        return f"{tool.name}: path: {resolved} is outside the cache: {RECOVER}"
    return None


def _import_failure(
    tool: Tool,
    home: str,
    want: str,
    python: Tool | None,
    cache: str,
    env: Mapping[str, str],
    scratch: str,
    timing: Timing,
) -> str | None:
    if python is None:
        gate_python = manifest.GATE_PYTHON
        return f"{tool.name}: version: tools.json has no tool named {gate_python!r} to import it: fix tools.json"
    python_home = manifest.tool_home(cache, python)
    interpreter = os.path.join(python_home, python.path_dir, python.command)
    if not _inside(os.path.realpath(interpreter), python_home):
        return f"{tool.name}: version: {interpreter} is outside the cache: {RECOVER}"
    path = os.path.normpath(os.path.join(home, tool.path_dir))
    argv = [interpreter, "-I", "-B", "-X", "utf8", "-c", PROBE, path, tool.module]
    return _probe(tool, argv, home, want, env, scratch, timing)


def _version_failure(
    tool: Tool, home: str, want: str, env: Mapping[str, str], scratch: str, timing: Timing
) -> str | None:
    argv = [os.path.join(home, tool.version_argv[0]), *tool.version_argv[1:]]
    return _probe(tool, argv, home, want, env, scratch, timing)


def _probe(
    tool: Tool, argv: list[str], home: str, want: str, env: Mapping[str, str], scratch: str, timing: Timing
) -> str | None:
    try:
        code, stdout = _run(argv, home, env, scratch, timing)
    except OSError as e:
        return f"{tool.name}: version: could not run {argv[0]}: {e.strerror}: {RECOVER}"
    if code is None:
        return f"{tool.name}: version: no answer within {timing.cap_s}s: {RECOVER}"
    got = stdout.split(b"\n", 1)[0].decode("utf-8", errors="replace")
    if code == 0 and got == want:
        return None
    status = "" if code == 0 else f" (exit {code})"
    return f"{tool.name}: version: want {want!r}, got {got!r}{status}: {RECOVER}"


def _differs(home: str, rel: str, want: str) -> bool:
    """A listed file must be a real file in the tool's directory: a link could lead the tools outside it."""
    path = os.path.join(home, rel)
    try:
        if os.path.islink(path) or not _inside(os.path.realpath(path), home):
            return True
        return manifest.sha256_file(path) != want
    except OSError:
        return True


def _hash_failure(tool: Tool, home: str) -> str | None:
    for rel, want in sorted(tool.exec_sha256.items()):
        if _differs(home, rel, want):
            return f"{tool.name}: sha256 of {rel} differs: {RECOVER}"
    return None


def _tree_failure(tool: Tool, home: str, tree: Tree) -> str | None:
    try:
        result = tree_digest(home, spawn=tree.spawn, env=tree.env, scratch=tree.scratch, name=tool.name)
    except TreeStopped:
        return f"{tool.name}: stopped before the tree hash"
    except TreeCapExceeded as e:  # a slow disk is not a damaged tool: clean-tools would not help (DA-S13e)
        return f"{tool.name}: tree hash exceeded its cap of {e.cap_s} s"
    except TreeStartFailed as e:  # no hash ran: clean-tools would not help either (BK-S5)
        return f"{tool.name}: tree hash could not start: {e}"
    except TreeChildFailed as e:  # only an unreadable tree is a damaged tool; a crash or a signal is not (CA-L4)
        if e.code != TREE_UNREADABLE:
            line = f": {e.first_line}" if e.first_line else ""
            return f"{tool.name}: tree hash failed (exit {e.code}){line}"
        result = NO_RESULT
    except OSError:
        result = NO_RESULT
    if result.digest != tool.tree_sha256:
        return f"{tool.name}: tree differs: {RECOVER}"
    # The home is the realpath the child resolved and hashed, never what the link names now (QB-S9).
    tree.listed.append(
        {"details": result.details, "home": result.real, "name": tool.name, "tree_sha256": result.digest}
    )
    return None
