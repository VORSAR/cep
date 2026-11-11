"""Step records and the run's files (plan check S2; design note §4): JSON in one form, written atomically."""

import json
import os
import re
import secrets
from collections.abc import Callable, Sequence

SCHEMA = 2  # 2 (S1.1c-1): every record carries `law`, the LAW Δ block's lines (the lawdelta row's; [] elsewhere)
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


def one_line(text: str, limit: int | None = REASON_MAX) -> str:
    """A reason is one line, whatever a tool's error text held, and at most `limit` characters (None: no limit)."""
    flat = _CONTROL.sub(" ", text)
    return flat if limit is None or len(flat) <= limit else flat[: limit - 1] + "…"


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
    reason_max: int | None = REASON_MAX,
    law: Sequence[str] = (),
) -> dict[str, object]:
    """One step record. `reason_max=None` keeps a reason whole: parity's lists every failing tool (review BA5).
    `law` is the LAW Δ block as the lawdelta row wrote it, one bounded line per entry (S1.1c-1 design §2.5)."""
    return {
        "schema": SCHEMA,
        "run_id": run_id,
        "commit": commit,
        "tier": tier,
        "job": job,
        "id": row_id,
        "blocking": blocking,  # review BA12 (owner, 2026-11-03): a non-blocking "fail" is a WARN
        "verdict": verdict,
        "reason": one_line(reason, reason_max),
        "exit_code": exit_code,
        "covered": covered,
        "expected": expected,
        "started_ms": started_ms,
        "ended_ms": ended_ms,
        "duration_ms": duration_ms,
        "scope_sha256": scope_sha256,
        "law": [one_line(x, None) for x in law],
    }
