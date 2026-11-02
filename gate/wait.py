"""The one wait helper: wait for a named state, with a cap (testing.md rule 1; design note §5).

The only file allowed `time.sleep`. A timeout raises WaitTimeout naming the state, so a hung test says what it
was waiting for. Time is injected, so the helper's own tests never pass real time.
"""

import queue
import time
from collections.abc import Callable

DEFAULT_CAP_S = 120.0
DEFAULT_POLL_S = 0.005


class WaitTimeout(Exception):
    """A named state did not arrive before its cap."""


def _named(state: str) -> str:
    if not state:
        raise ValueError("a wait must name the state it waits for")
    return state


def until[T](
    state: str,
    predicate: Callable[[], T],
    *,
    cap_s: float = DEFAULT_CAP_S,
    poll_s: float = DEFAULT_POLL_S,
    now: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], None] = time.sleep,
) -> T:
    """Return predicate()'s first truthy value; raise WaitTimeout naming `state` once `cap_s` has passed."""
    _named(state)
    deadline = now() + cap_s
    while True:
        value = predicate()
        if value:
            return value
        remaining = deadline - now()
        if remaining <= 0:
            raise WaitTimeout(f"timed out after {cap_s}s waiting for {state}")
        sleep(min(poll_s, remaining))


def get[T](q: queue.SimpleQueue[T] | queue.Queue[T], state: str, *, cap_s: float = DEFAULT_CAP_S) -> T:
    """Receive one item from a queue; raise WaitTimeout naming `state` once `cap_s` has passed."""
    _named(state)
    try:
        return q.get(timeout=cap_s)
    except queue.Empty:
        raise WaitTimeout(f"timed out after {cap_s}s waiting for {state}") from None
