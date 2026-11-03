"""Step records and the run's files (plan check S2; design note §4): JSON in one form, written atomically."""

import json
import os
import re
import secrets
from collections.abc import Callable

SCHEMA = 1
REASON_MAX = 300
_CONTROL = re.compile(r"[\x00-\x1f\x7f-\x9f  ]")


def json_bytes(obj: object) -> bytes:
    return (json.dumps(obj, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")


def write_atomic(path: str, data: bytes, *, rename: Callable[[str, str], None] = os.rename) -> None:
    """Write a temporary name in the same directory, then rename: a reader sees the old file or the whole new one."""
    tmp = f"{path}.{os.getpid()}.{secrets.token_hex(4)}.tmp"
    try:
        with open(tmp, "xb") as f:
            f.write(data)
        rename(tmp, path)
    except BaseException:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def one_line(text: str) -> str:
    """A reason is one line of bounded length, whatever a tool's error text held."""
    flat = _CONTROL.sub(" ", text)
    return flat if len(flat) <= REASON_MAX else flat[: REASON_MAX - 1] + "…"


def make(
    *,
    run_id: str,
    commit: str,
    tier: str,
    job: str,
    row_id: str,
    blocking: bool,
    verdict: str,
    reason: str,
    exit_code: int,
    covered: int,
    expected: int,
    started_ms: int,
    ended_ms: int,
    duration_ms: int,
    scope_sha256: str,
) -> dict[str, object]:
    return {
        "schema": SCHEMA,
        "run_id": run_id,
        "commit": commit,
        "tier": tier,
        "job": job,
        "id": row_id,
        "blocking": blocking,  # review BA12 (owner, 2026-11-03): a non-blocking "fail" is a WARN
        "verdict": verdict,
        "reason": one_line(reason),
        "exit_code": exit_code,
        "covered": covered,
        "expected": expected,
        "started_ms": started_ms,
        "ended_ms": ended_ms,
        "duration_ms": duration_ms,
        "scope_sha256": scope_sha256,
    }
