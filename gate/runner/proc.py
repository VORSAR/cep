"""One step's process group (design note §4 steps 2-4; plan check P6).

A step's processes run in their own session, with stdin /dev/null and output in files, never pipes: a grandchild
holding stdout can then never hang the runner (QA Q25). The scheduler signals a Group from its own thread while the
step's worker thread spawns and reaps, so the lock orders the two: after any signal, nothing new starts.
"""

import contextlib
import dataclasses
import os
import signal
import subprocess
import threading
from collections.abc import Callable, Mapping, Sequence

# Group.spawn's shape: argv, then cwd, env, stdout and stderr by keyword; None once the group was signalled.
type Spawn = Callable[..., subprocess.Popen[bytes] | None]


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

        The leader is reaped before the probe, so it never counts itself; a member that has exited but awaits its
        parent (a zombie) does not count either. Whatever live is left is killed. Residue (design note §4): the
        pid-reuse window is accepted.
        """
        code = proc.wait()
        with self._lock:
            self._pgid = None
        left, self.left_detail = probe(proc.pid)
        if left:
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.killpg(proc.pid, signal.SIGKILL)
        return code, left

    def kill_left(self) -> tuple[bool, str]:
        """After a stop that did not wait for the group's leader: probe the last group started, naming what the
        TERM left (S1.1c-1-1), then KILL it and forbid any later spawn (S1.1c-3 design §10 C6)."""
        with self._lock:
            self._signalled = True
            pgid = self._pgid
        if pgid is None:
            return False, ""
        left, self.left_detail = probe(pgid)
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(pgid, signal.SIGKILL)
        return left, self.left_detail

    def signal(self, signum: int) -> None:
        """Signal the live group, if any, and forbid any later spawn in this step."""
        with self._lock:
            self._signalled = True
            if self._pgid is not None:
                with contextlib.suppress(ProcessLookupError, PermissionError):
                    os.killpg(self._pgid, signum)


def probe(pgid: int) -> tuple[bool, str]:
    """Whether anything of the group `pgid` is left running, and what the probe saw ("" when nothing was).

    A group with any live member is left running, and named. A group whose members are all zombies, or that has
    none by the time `ps` looks, is finished: macOS answers EPERM for a group whose only members are zombies, Linux
    answers that it exists (owner, 2026-11-12, known issue S1.1c-1-1). A `ps` that fails counts as left running.
    """
    try:
        os.killpg(pgid, 0)
        answer = "the group still exists"
    except ProcessLookupError:
        return False, ""
    except PermissionError:  # the group exists, and holds something this user may not signal (or only zombies)
        answer = "the group exists but may not be signalled (EPERM)"
    try:
        found = members(pgid)
    except PsFailed as e:
        return True, f"{answer}: ps failed: {e}"
    if all(m.zombie for m in found):
        return False, ""
    return True, f"{answer}: {'; '.join(map(str, found))}"  # named before the kill


class PsFailed(Exception):
    """`ps` could not list the processes: it did not start, timed out or exited non-zero."""


@dataclasses.dataclass(frozen=True, slots=True)
class Member:
    """One process of a group as `ps` lists it: its state's first letter Z means exited, awaiting its parent."""

    pid: int
    state: str
    program: str  # the command's program, by its base name
    args: str

    @property
    def zombie(self) -> bool:
        return self.state.startswith("Z")

    def __str__(self) -> str:
        return " ".join(x for x in (str(self.pid), self.state, self.program, self.args) if x)


def members(pgid: int) -> list[Member]:
    """The group's members as `ps` lists them now, in its order; raises PsFailed when `ps` gives no listing."""
    try:
        done = subprocess.run(
            ["ps", "-A", "-o", "pgid=,pid=,stat=,command="],
            stdin=subprocess.DEVNULL,
            capture_output=True,
            timeout=10,
        )
    except (OSError, subprocess.SubprocessError) as e:
        raise PsFailed(e) from e
    if done.returncode != 0:
        detail = done.stderr.decode("utf-8", errors="replace").strip()
        raise PsFailed(f"exit {done.returncode}" + (f": {detail}" if detail else ""))
    found = []
    for line in done.stdout.decode("utf-8", errors="replace").splitlines():
        parts = line.split(None, 3)
        if len(parts) == 4 and parts[0] == str(pgid):
            if not parts[1].isdigit():
                raise PsFailed(f"no pid on the line {line!r}")
            program, _, args = parts[3].partition(" ")
            found.append(Member(int(parts[1]), parts[2], os.path.basename(program), args))
    return found
