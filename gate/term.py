"""SIGTERM as an exception, so a stopped process cleans up before it exits (review BA1; break-it QB4).

The runner stops a row with SIGTERM to its process group, then SIGKILL after a grace. TERM's default action ends a
Python process at once: no `finally` and no test cleanup runs, so a child it started in a session of its own (a
nested gate, an in-test runner's rows, a CPU hog) outlives it, parented to init. Every gate program that starts
such children calls `install()` first: the first TERM then raises Terminated in the main thread, wherever it is, and
the program's own `finally` blocks and cleanups run within the grace. A later TERM changes nothing, so a cleanup is
never cut short by a second one. gate/runner/pyshim.py, standalone by design, carries its own copy of this rule.
"""

import signal
from collections.abc import Callable


class Terminated(BaseException):
    """Raised once, by the first SIGTERM. A BaseException, so an `except Exception` cannot swallow the stop."""


def _ignore(signum: int, frame: object) -> None:
    """The first TERM is already being acted on."""


def install(on_term: Callable[[], None] = lambda: None) -> None:
    """From now on the first SIGTERM calls `on_term`, then raises Terminated in the main thread."""

    def handler(signum: int, frame: object) -> None:
        signal.signal(signal.SIGTERM, _ignore)
        on_term()
        raise Terminated("stopped by SIGTERM")

    signal.signal(signal.SIGTERM, handler)
