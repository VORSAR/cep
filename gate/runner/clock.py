"""The two injected clocks and the timer source (design note §4; plan check S3c)."""

import secrets
import threading
import time
from collections.abc import Callable
from dataclasses import dataclass
from typing import Protocol

NS_PER_MS = 1_000_000


@dataclass(frozen=True)
class Clock:
    wall_ms: Callable[[], int]  # UTC epoch ms: run_id, started_ms, ended_ms
    mono_ns: Callable[[], int]  # durations and the footer


REAL = Clock(wall_ms=lambda: time.time_ns() // NS_PER_MS, mono_ns=time.monotonic_ns)


class Timer(Protocol):
    def cancel(self) -> None: ...


class Timers(Protocol):
    def start(self, label: str, delay_s: float, fire: Callable[[], None]) -> Timer: ...


class RealTimers:
    def start(self, label: str, delay_s: float, fire: Callable[[], None]) -> Timer:
        timer = threading.Timer(delay_s, fire)
        timer.name = f"gate-timer-{label}"
        timer.daemon = True
        timer.start()
        return timer


def run_id(wall_ms: int, token: str | None = None) -> str:
    """YYYYMMDDTHHMMSSZ-<6 hex>, UTC."""
    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime(wall_ms // 1000))
    return f"{stamp}-{secrets.token_hex(3) if token is None else token}"
