"""Parity: the gate's first step in every job, and its one hard stop (design note §3).

The runner calls `check` and records its result as the per-job parity record. Nothing here writes to the
cache, and nothing here runs a binary that resolves outside its own tool directory.
"""

import contextlib
import os
import shutil
import signal
import subprocess
import tempfile
from collections.abc import Mapping

from gate.toolchain import manifest
from gate.toolchain.manifest import Tool

VERSION_CAP_S = 60  # breaks a hang only: a version line takes milliseconds
GRACE_S = 5
RECOVER = "run make clean-tools tools"


def check(tools: list[Tool], cache: str, env: Mapping[str, str], *, full_tree: bool) -> list[str]:
    """Check every tool; return one reason per failing tool, in tools.json order ([] means pass).

    Per tool, short-circuit in order: not installed → path (shutil.which(command) under env["PATH"],
    realpath-compared) → version (version_argv run in `env`, first stdout line exact) → sha256 of every
    exec_sha256 entry → (only if full_tree, i.e. T2 and above) tree_sha256. Never runs a binary that
    lies outside the cache.
    """
    return _check(tools, cache, env, full_tree=full_tree, version_cap_s=VERSION_CAP_S, grace_s=GRACE_S)


def _check(
    tools: list[Tool], cache: str, env: Mapping[str, str], *, full_tree: bool, version_cap_s: float, grace_s: float
) -> list[str]:
    goos, goarch = manifest.platform_key().split("-")
    reasons = []
    with tempfile.TemporaryDirectory(prefix="gate-parity-") as scratch:
        for tool in tools:
            home = manifest.tool_home(cache, tool)
            if not os.path.isdir(home):
                reasons.append(f"{tool.name}: not installed: run make tools")
                continue
            want = tool.version_line.replace("{goos}", goos).replace("{goarch}", goarch)
            reason = (
                _path_failure(tool, home, env)
                or _version_failure(tool, home, want, env, scratch, version_cap_s, grace_s)
                or _hash_failure(tool, home)
                or (_tree_failure(tool, home) if full_tree else None)
            )
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


def _run(
    argv: list[str], cwd: str, env: Mapping[str, str], scratch: str, cap_s: float, grace_s: float
) -> tuple[int | None, bytes]:
    """Run one command in its own process group, output to files; return (exit code or None if hung, stdout)."""
    stdout = os.path.join(scratch, "stdout")
    with open(stdout, "wb") as out, open(os.path.join(scratch, "stderr"), "wb") as err:
        proc = subprocess.Popen(
            argv, cwd=cwd, env=dict(env), stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True
        )
    code: int | None = None
    try:
        code = proc.wait(timeout=cap_s)
    except subprocess.TimeoutExpired:
        _signal_group(proc.pid, signal.SIGTERM)
        with contextlib.suppress(subprocess.TimeoutExpired):
            proc.wait(timeout=grace_s)
    _signal_group(proc.pid, signal.SIGKILL)  # nothing a probe started may outlive it
    proc.wait()
    with open(stdout, "rb") as f:
        return code, f.read()


def _version_failure(
    tool: Tool, home: str, want: str, env: Mapping[str, str], scratch: str, cap_s: float, grace_s: float
) -> str | None:
    argv = [os.path.join(home, tool.version_argv[0]), *tool.version_argv[1:]]
    try:
        code, stdout = _run(argv, home, env, scratch, cap_s, grace_s)
    except OSError as e:
        return f"{tool.name}: version: could not run {argv[0]}: {e.strerror}: {RECOVER}"
    if code is None:
        return f"{tool.name}: version: no answer within {cap_s}s: {RECOVER}"
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


def _tree_failure(tool: Tool, home: str) -> str | None:
    try:
        same = manifest.tree_sha256(home) == tool.tree_sha256
    except OSError:
        same = False
    return None if same else f"{tool.name}: tree differs: {RECOVER}"
