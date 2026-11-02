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
        try:
            os.killpg(proc.pid, 0)
        except ProcessLookupError:
            return code, False
        except PermissionError:
            pass  # the group exists, and holds something this user may not signal
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(proc.pid, signal.SIGKILL)
        return code, True

    def signal(self, signum: int) -> None:
        """Signal the live group, if any, and forbid any later spawn in this step."""
        with self._lock:
            self._signalled = True
            if self._pgid is not None:
                with contextlib.suppress(ProcessLookupError, PermissionError):
                    os.killpg(self._pgid, signum)
