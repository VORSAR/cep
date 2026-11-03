"""One step, run in a worker thread (design note §4 "One step"): scope, files.nul, the row's own process.

A step only gathers facts. The scheduler owns time and signals; `verdict.decide` owns the judgement.
"""

import hashlib
import os
from collections.abc import Mapping
from dataclasses import dataclass, field

from gate.runner import covered, env, records, scope
from gate.runner.covered import Covered
from gate.runner.proc import Group
from gate.runner.registry import FILES_TOKEN, Row

FILES_NAME = "files.nul"


@dataclass(frozen=True)
class Context:
    repo: str
    run_dir: str
    tier: str
    clean_env: Mapping[str, str]


@dataclass(frozen=True)
class Outcome:
    scope_error: str = ""
    spawn_error: str = ""
    returncode: int | None = None  # None: no process of the row's own ever ran
    left_running: bool = False
    covered: Covered = field(default_factory=Covered)
    expected: int = 0
    scope_sha256: str = ""
    first_failure: str = ""


def first_failure(stdout: bytes) -> str:
    """The row's own explanation of a red exit, for its reason (owner, 2026-11-03; review BA3).

    The first stdout line that starts `failed: ` (the wrappers' `rows.finish`) or `FAIL ` (the coverage instrument),
    without that marker, or one that starts `internal error: ` (a row's crash), whole. "" if there is none.
    """
    for line in stdout.decode("utf-8", errors="replace").splitlines():
        for marker, keep in (("failed: ", False), ("FAIL ", False), ("internal error: ", True)):
            if line.startswith(marker):
                return line if keep else line.removeprefix(marker)
    return ""


def row_dir(run_dir: str, row: Row) -> str:
    """A row's private directory. A row it needs is therefore its sibling (plan check S3d)."""
    return os.path.join(run_dir, "rows", row.job, row.id)


def execute(row: Row, group: Group, ctx: Context) -> Outcome:
    directory = row_dir(ctx.run_dir, row)
    os.makedirs(directory)
    row_env = env.row_env(ctx.clean_env, tier=ctx.tier, row=row.id, run_dir=ctx.run_dir, row_dir=directory)
    try:
        expansion = scope.expand(row.expected_scope, group=group, cwd=ctx.repo, env=row_env, row_dir=directory)
    except scope.ScopeError as e:
        return Outcome(scope_error=str(e))
    except scope.Stopped:
        return Outcome()

    files = os.path.join(directory, FILES_NAME)
    listing = b"".join(item + b"\0" for item in expansion.items)
    records.write_atomic(files, listing)
    digest = hashlib.sha256(listing).hexdigest()
    done = Outcome(expected=expansion.count, scope_sha256=digest)

    stdout = os.path.join(directory, "stdout")
    argv = [files if arg == FILES_TOKEN else arg for arg in row.command]
    try:
        proc = group.spawn(argv, cwd=ctx.repo, env=row_env, stdout=stdout, stderr=os.path.join(directory, "stderr"))
    except OSError as e:
        return Outcome(spawn_error=str(e), expected=expansion.count, scope_sha256=digest)
    if proc is None:
        return done
    returncode, left_running = group.reap(proc)
    with open(stdout, "rb") as f:
        printed = f.read()
    return Outcome(
        returncode=returncode,
        left_running=left_running,
        covered=covered.parse(printed),
        expected=expansion.count,
        scope_sha256=digest,
        first_failure=first_failure(printed),
    )
