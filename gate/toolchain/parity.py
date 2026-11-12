"""Parity: the gate's first step in every job, and its one hard stop (design note §3).

The runner calls `check` and records its result as the per-job parity record. Nothing here writes to the
cache, and nothing here runs a binary that resolves outside its own tool directory.
"""

import contextlib
import os
import shutil
import signal
import subprocess
import sys
import tempfile
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass

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
# Measured 2026-11-12 on an M-series Mac (macOS, APFS SSD), the cache in the page cache, three rounds per tool, each in
# a fresh process: go 2.51-2.64 s (15,660 files), python 0.14-0.15 s (1,771), python-floor 0.13-0.14 s (1,666),
# golangci-lint, ruff and lark 0.02-0.03 s. A cold page cache on a CI runner's network disk is assumed ten to fifty
# times slower; the cap sits about a hundred times above go's warm hash. Refine from CI: parity's duration at T2+
# on each OS's cold runner (records/<job>/parity.json, started_ms to ended_ms), over several runs.
TREE_CAP_S: float = 300
TREE_GRACE_S = 5  # after the cap's KILL, how long to wait for the child to be reaped before giving up on it


class TreeCapExceeded(OSError):
    """The tree hash outlived its cap; the child's own process group was killed."""

    def __init__(self, cap_s: float) -> None:
        super().__init__(f"tree hash exceeded its cap of {cap_s} s")
        self.cap_s = cap_s


class TreeStopped(Exception):
    """The spawner refused to start the tree child: the run's parity group was already signalled (a stop)."""


type Spawn = Callable[..., subprocess.Popen[bytes] | None]


def popen(
    argv: Sequence[str], *, cwd: str, env: Mapping[str, str], stdout: str, stderr: str
) -> subprocess.Popen[bytes]:
    """A spawner outside the runner: a plain Popen in a new session, with Group.spawn's shape (DA-S13a)."""
    with open(stdout, "wb") as out, open(stderr, "wb") as err:
        return subprocess.Popen(
            list(argv), cwd=cwd, env=dict(env), stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True
        )


def tree_digest(
    home: str,
    *,
    spawn: Spawn,
    cap_s: float | None = None,
    argv: Sequence[str] | None = None,
    env: Mapping[str, str] | None = None,
    scratch: str | None = None,
    name: str = "tool",
) -> str:
    """The digest the tree child prints for `home`: `[*argv, home]` started through `spawn` (TREE_CHILD and
    TREE_CAP_S when not given, read now), its output in `<scratch>/tree-<name>.out` and `.err`.

    Raises TreeStopped when `spawn` returns None, TreeCapExceeded past the cap (after killing the child's own process
    group, never through the runner's Group, which would refuse every later spawn), and OSError when the child fails.
    """
    cap = TREE_CAP_S if cap_s is None else cap_s
    command = [*(TREE_CHILD if argv is None else argv), home]
    with contextlib.ExitStack() as stack:
        if scratch is None:
            scratch = stack.enter_context(tempfile.TemporaryDirectory(prefix="gate-tree-"))
        out = os.path.join(scratch, f"tree-{name}.out")
        err = os.path.join(scratch, f"tree-{name}.err")
        proc = spawn(command, cwd=REPO, env=os.environ if env is None else env, stdout=out, stderr=err)
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
        with open(out, "rb") as f:
            digest = f.read().decode("utf-8", errors="replace").strip()
        if code != 0:
            raise OSError(f"the tree child exited {code}")
        return digest


@dataclass(frozen=True)
class Timing:
    """How long a version probe may run. Time only breaks a hang: a version line takes milliseconds.

    `wait` is the one place a probe waits, injected by tests so that a cap fires only after a witness.
    """

    cap_s: float = 60
    grace_s: float = 5
    wait: Callable[[subprocess.Popen, float], int] = subprocess.Popen.wait


def check(
    tools: list[Tool],
    cache: str,
    env: Mapping[str, str],
    *,
    full_tree: bool,
    spawn: Spawn = popen,
    scratch: str | None = None,
) -> list[str]:
    """Check every tool; return one reason per failing tool, in tools.json order ([] means pass).

    Per tool, short-circuit in order: not installed → path (shutil.which(command) under env["PATH"],
    realpath-compared) → version (version_argv run in `env`, first stdout line exact) → sha256 of every
    exec_sha256 entry → (only if full_tree, i.e. T2 and above) tree_sha256. Never runs a binary that
    lies outside the cache. A python-package has no command: its path check is that its path_dir stays
    inside its home, and its version is `<module>.__version__` imported by the pinned interpreter.

    The tree hash runs in a child per tool started through `spawn` (the runner passes its parity group's spawn,
    so a stop ends it), its output in `scratch` (the runner's run directory's parity/; a temporary one if None).
    """
    return _check(tools, cache, env, full_tree=full_tree, timing=Timing(), spawn=spawn, scratch=scratch)


def _check(
    tools: list[Tool],
    cache: str,
    env: Mapping[str, str],
    *,
    full_tree: bool,
    timing: Timing,
    spawn: Spawn = popen,
    scratch: str | None = None,
) -> list[str]:
    goos, goarch = manifest.platform_key().split("-")
    python = next((tool for tool in tools if tool.name == manifest.GATE_PYTHON), None)
    reasons = []
    with tempfile.TemporaryDirectory(prefix="gate-parity-") as probes:
        tree = Tree(spawn=spawn, env=env, scratch=probes if scratch is None else scratch)
        for tool in tools:
            home = manifest.tool_home(cache, tool)
            if not os.path.isdir(home):
                reasons.append(f"{tool.name}: not installed: run make tools")
                continue
            want = tool.version_line.replace("{goos}", goos).replace("{goarch}", goarch)
            if tool.kind == manifest.PYTHON_PACKAGE:
                first = _package_path_failure(tool, home) or _import_failure(
                    tool, home, want, python, cache, env, probes, timing
                )
            else:
                first = _path_failure(tool, home, env) or _version_failure(tool, home, want, env, probes, timing)
            reason = first or _hash_failure(tool, home) or (_tree_failure(tool, home, tree) if full_tree else None)
            if reason is not None:
                reasons.append(reason)
    return reasons


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


@dataclass(frozen=True)
class Tree:
    """Where the tree children start and write: the spawner, the run's clean env and the scratch directory."""

    spawn: Spawn
    env: Mapping[str, str]
    scratch: str


def _tree_failure(tool: Tool, home: str, tree: Tree) -> str | None:
    try:
        digest = tree_digest(home, spawn=tree.spawn, env=tree.env, scratch=tree.scratch, name=tool.name)
    except TreeStopped:
        return f"{tool.name}: stopped before the tree hash"
    except TreeCapExceeded as e:  # a slow disk is not a damaged tool: clean-tools would not help (DA-S13e)
        return f"{tool.name}: tree hash exceeded its cap of {e.cap_s} s"
    except OSError:
        digest = ""
    return None if digest == tool.tree_sha256 else f"{tool.name}: tree differs: {RECOVER}"
