"""One step's process group (design note §4 steps 2-4; plan check P6).

A step's processes run in their own session, with stdin /dev/null and output in files, never pipes: a grandchild
holding stdout can then never hang the runner (QA Q25). The scheduler signals a Group from its own thread while the
step's worker thread spawns and reaps, so the lock orders the two: after any signal, nothing new starts.
"""

import contextlib
import os
import signal
import subprocess
import threading
from collections.abc import Mapping, Sequence


class Group:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._pgid: int | None = None
        self._signalled = False
        self.left_detail = ""  # after a positive probe: which answer fired and the group's members (S1.1c-1-1)

    def spawn(
        self, argv: Sequence[str], *, cwd: str, env: Mapping[str, str], stdout: str, stderr: str
    ) -> subprocess.Popen[bytes] | None:
        """Start a process in a new session; None if this step was already signalled. A failure raises OSError."""
        with self._lock:
            if self._signalled:
                return None
            with open(stdout, "wb") as out, open(stderr, "wb") as err:
                proc = subprocess.Popen(
                    list(argv),
                    cwd=cwd,
                    env=dict(env),
                    stdin=subprocess.DEVNULL,
                    stdout=out,
                    stderr=err,
                    start_new_session=True,
                )
            self._pgid = proc.pid
            return proc

    def reap(self, proc: subprocess.Popen[bytes]) -> tuple[int, bool]:
        """Wait for the leader, then probe its group: (returncode, whether anything was left running).

        The leader is reaped before the probe, so it never counts itself. Whatever is left is killed.
        Residue (design note §4): a zombie may count on Linux, and the pid-reuse window is accepted.
        """
        code = proc.wait()
        with self._lock:
            self._pgid = None
        left, self.left_detail = probe(proc.pid)
        if left:
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.killpg(proc.pid, signal.SIGKILL)
        return code, left

    def signal(self, signum: int) -> None:
        """Signal the live group, if any, and forbid any later spawn in this step."""
        with self._lock:
            self._signalled = True
            if self._pgid is not None:
                with contextlib.suppress(ProcessLookupError, PermissionError):
                    os.killpg(self._pgid, signum)


def probe(pgid: int) -> tuple[bool, str]:
    """Whether anything of the group `pgid` is left running, and what the probe saw (empty when nothing)."""
    try:
        os.killpg(pgid, 0)
        answer = "the group still exists"
    except ProcessLookupError:
        return False, ""
    except PermissionError:  # the group exists, and holds something this user may not signal (or one mid-exit)
        answer = "the group exists but may not be signalled (EPERM)"
    return True, f"{answer}: {members(pgid)}"  # named before the kill (known issue S1.1c-1-1)


def members(pgid: int) -> str:
    """The group's members as `ps` lists them now — "pid state command", the command's program by its base name —
    joined by "; "; a group whose members have already exited, or a `ps` that fails, says so. Only read on the
    failure path, to name what a positive probe saw (owner, 2026-11-11, known issue S1.1c-1-1)."""
    try:
        listing = subprocess.run(
            ["ps", "-A", "-o", "pgid=,pid=,stat=,command="],
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=10,
        ).stdout
    except (OSError, subprocess.SubprocessError) as e:
        return f"ps failed: {e}"
    found = []
    for line in listing.splitlines():
        parts = line.split(None, 3)
        if len(parts) == 4 and parts[0] == str(pgid):
            program, _, args = parts[3].partition(" ")
            found.append(" ".join(x for x in (parts[1], parts[2], os.path.basename(program), args) if x))
    return "; ".join(found) if found else "no member listed (gone before it could be named)"
