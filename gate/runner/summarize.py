"""CI's one required check (gate design §12.5; plan check S3): judge every CI job's records against the registry.

    gate/run.py summarize --registry gate/registry.json --tier T4 --artifacts DIR --run-id ID --commit SHA
                          --attempt N --image TEXT

The expected rows come from the registry, never from what was uploaded: every row at the tier, keyed by job, plus a
`parity` record per job. A row with no acceptable record is a named RED row, so a skipped, cancelled or missing job
can never read as a pass. Each artifact is `<DIR>/records-<job>-<attempt>/` holding `records/<job>/*.json` and that
job's `run.json`. The output is the runner's summary format, plus an `image:` line; exit 0 only when every expected
row passed (or failed without blocking, shown as WARN).
"""

import argparse
import json
import os
import re
from collections.abc import Sequence
from dataclasses import dataclass

from gate.runner import records, registry, summary

ARTIFACT = re.compile(r"records-([a-z][a-z0-9-]*)-([0-9]+)")
FIELDS = {
    "schema": int,
    "run_id": str,
    "commit": str,
    "tier": str,
    "job": str,
    "id": str,
    "blocking": bool,
    "verdict": str,
    "reason": str,
    "exit_code": int,
    "covered": int,
    "expected": int,
    "started_ms": int,
    "ended_ms": int,
    "duration_ms": int,
    "scope_sha256": str,
    "law": list,
}
LAWDELTA = "lawdelta"
MALFORMED = "malformed: unreadable or not the record schema"
LAW_FAILED = "LAW Δ: FAILED — see the lawdelta row"


class UsageError(Exception):
    """The command line is not runnable; exit 2."""


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise UsageError(f"{message}\n{self.format_usage().strip()}")


@dataclass
class _Found:
    """What one artifact directory held."""

    job: str
    attempt: int
    registry_sha256: str
    records: list[dict]
    malformed: dict[tuple[str, str], str]  # (job folder, file stem) -> the reason


def _parse(argv: Sequence[str]) -> argparse.Namespace:
    parser = _Parser(prog="gate/run.py summarize", add_help=False, allow_abbrev=False)
    for name in ("--registry", "--tier", "--artifacts", "--run-id", "--commit", "--image"):
        parser.add_argument(name, required=True)
    parser.add_argument("--attempt", required=True, type=int)
    return parser.parse_args(list(argv))


def _valid(record: object) -> bool:
    if type(record) is not dict or record.keys() != FIELDS.keys():
        return False
    if not all(type(record[name]) is kind for name, kind in FIELDS.items()):
        return False
    return record["schema"] == records.SCHEMA and all(type(x) is str for x in record["law"])


def _malformed(record: object) -> str:
    """Why a record is not one: the schema it carries when that is the fault (owner, 2026-11-11, design 5)."""
    if type(record) is dict and type(record.get("schema")) is int and record["schema"] != records.SCHEMA:
        return f"malformed: schema {record['schema']}, expected {records.SCHEMA}"
    return MALFORMED


def _read_json(path: str) -> object:
    with open(path, "rb") as f:
        return json.loads(f.read().decode("utf-8"))


def _artifact(path: str, job: str, attempt: int) -> _Found:
    found = _Found(job, attempt, "", [], {})
    try:
        run = _read_json(os.path.join(path, "run.json"))
        found.registry_sha256 = run.get("registry_sha256", "") if type(run) is dict else ""
    except OSError, ValueError:
        found.registry_sha256 = ""
    # Every job's folder is read, not only this artifact's own, so a record filed in another job's artifact is
    # still seen, and a second record for one row is a duplicate rather than invisible.
    top = os.path.join(path, "records")
    for folder in sorted(os.listdir(top)) if os.path.isdir(top) else []:
        directory = os.path.join(top, folder)
        for name in sorted(os.listdir(directory)) if os.path.isdir(directory) else []:
            try:
                record = _read_json(os.path.join(directory, name))
            except OSError, ValueError:
                record = None
            if _valid(record) and record["job"] == folder and name == f"{record['id']}.json":
                found.records.append(record)
            else:
                found.malformed[(folder, name.removesuffix(".json"))] = _malformed(record)
    return found


def _artifacts(root: str) -> list[_Found]:
    """Every artifact directory, in name order. A download that failed leaves none, and every row reads missing."""
    found = []
    for name in sorted(os.listdir(root)) if os.path.isdir(root) else []:
        match = ARTIFACT.fullmatch(name)
        if match and os.path.isdir(os.path.join(root, name)):
            found.append(_artifact(os.path.join(root, name), match.group(1), int(match.group(2))))
    return found


def _inconsistent(record: dict, blocking: bool) -> str:
    """A record that contradicts itself or the registry (plan check S3; QA C15, N7, N30); "" when it holds."""
    if record["verdict"] == "pass" and record["exit_code"] != 0:
        return f"inconsistent: pass with exit_code {record['exit_code']}"
    if record["verdict"] == "pass" and record["covered"] != record["expected"]:
        return f"inconsistent: pass with covered {record['covered']} ≠ expected {record['expected']}"
    if record["blocking"] != blocking:
        return f"inconsistent: blocking {str(record['blocking']).lower()} but the registry says {str(blocking).lower()}"
    if record["ended_ms"] < record["started_ms"]:
        return "inconsistent: ended before it started"
    if record["id"] == LAWDELTA and record["verdict"] == "pass" and not record["law"]:
        return "inconsistent: pass with no LAW Δ block"
    return ""


def _law_block_ci(
    rows: Sequence[registry.Row], lines: Sequence[summary.Line], used: Sequence[dict], tier: str
) -> list[str]:
    """The block CI's aggregate opens with: the lawdelta row's record, judged (S1.1c-1 design §2.5)."""
    job = next((r.job for r in rows if r.id == LAWDELTA), None)  # never a literal job name (review: architect L4)
    if job is None:
        return [f"LAW Δ: not run — the lawdelta row is not in this registry at {tier}"]
    line = next((x for x in lines if (x.job, x.step) == (job, LAWDELTA)), None)
    if line is None or line.verdict != "PASS":
        return [LAW_FAILED]
    record = next(r for r in used if (r["job"], r["id"]) == (job, LAWDELTA))
    return list(record["law"])


def _judge(record: dict, args: argparse.Namespace, blocking: bool, sha: str, found_sha: str) -> tuple[str, str]:
    """(verdict word, reason) for the one current record of an expected row."""
    if record["run_id"] != args.run_id or record["commit"] != args.commit:
        return "FAIL", f"stale: from run {record['run_id']} at {record['commit'][:12]}"
    if found_sha != sha:
        return "FAIL", "ran another registry: its run.json registry_sha256 differs"
    if record["tier"] != args.tier:
        return "FAIL", f"wrong tier: recorded at {record['tier']}"
    if record["verdict"] not in ("pass", "fail"):
        return "FAIL", f"{record['verdict']} at {args.tier}: only T0 may skip"
    problem = _inconsistent(record, blocking)
    if problem:
        return "FAIL", problem
    if record["verdict"] == "pass":
        return "PASS", record["reason"]
    return ("FAIL" if blocking else "WARN"), record["reason"]


def summarize(args: argparse.Namespace) -> tuple[str, int, dict]:
    loaded, sha = registry.load_with_digest(args.registry)
    rows = registry.select(loaded, args.tier, None)
    jobs = list(dict.fromkeys(r.job for r in rows))
    expected = [(job, registry.PARITY_ID, True) for job in jobs] + [(r.job, r.id, r.blocking) for r in rows]
    found = _artifacts(args.artifacts)
    current = [f for f in found if f.attempt == args.attempt]
    older = [f for f in found if f.attempt != args.attempt]

    lines: list[summary.Line] = []
    judged: list[dict] = []
    used: list[dict] = []
    for job, row_id, blocking in expected:
        mine = [(f, r) for f in current for r in f.records if (r["job"], r["id"]) == (job, row_id)]
        broken = [f.malformed[(job, row_id)] for f in current if (job, row_id) in f.malformed]
        if broken:
            word, reason, ms = "FAIL", broken[0], 0
        elif len(mine) > 1:
            word, reason, ms = "FAIL", f"duplicate: {len(mine)} records", 0
        elif not mine:
            before = sorted({f.attempt for f in older for r in f.records if (r["job"], r["id"]) == (job, row_id)})
            word, ms = "FAIL", 0
            reason = f"stale: only attempt {before[-1]} ran it" if before else "missing: no job recorded it"
        else:
            artifact, record = mine[0]
            word, reason = _judge(record, args, blocking, sha, artifact.registry_sha256)
            ms = record["duration_ms"]
            if not reason.startswith(("stale", "inconsistent: ended")):
                used.append(record)
        lines.append(summary.Line(job, row_id, word, ms, reason))
        judged.append({"job": job, "id": row_id, "verdict": word, "reason": records.one_line(reason)})

    names = {(job, row_id) for job, row_id, _ in expected}
    for f in current:
        for record in f.records:
            if (record["job"], record["id"]) not in names:
                lines.append(summary.Line(record["job"], record["id"], "FAIL", 0, "unknown row: not in the registry"))
                judged.append({"job": record["job"], "id": record["id"], "verdict": "FAIL", "reason": "unknown row"})

    wall_ms = max(r["ended_ms"] for r in used) - min(r["started_ms"] for r in used) if used else 0
    summed_ms = sum(r["duration_ms"] for r in used)
    exit_code = 1 if any(line.verdict == "FAIL" for line in lines) else 0
    text = summary.render(
        lines,
        law=_law_block_ci(rows, lines, used, args.tier),
        wall_ms=wall_ms,
        summed_ms=summed_ms,
        exit_code=exit_code,
        run_dir=args.artifacts,
        image=args.image,
    )
    run = {
        "schema": records.SCHEMA,
        "run_id": args.run_id,
        "commit": args.commit,
        "tier": args.tier,
        "attempt": args.attempt,
        "registry_sha256": sha,
        "rows": judged,
        "wall_ms": wall_ms,
        "summed_ms": summed_ms,
    }
    return text, exit_code, run


def main(argv: Sequence[str]) -> int:
    try:
        args = _parse(argv)
        text, code, run = summarize(args)
    except (UsageError, registry.RegistryError) as e:
        print(e)
        print("GATE FAILED")
        print("exit: 2")
        return 2
    out = os.path.join(args.artifacts, "strict-gate")
    os.makedirs(out, exist_ok=True)
    records.write_atomic(os.path.join(out, "run.json"), records.json_bytes(run))
    records.write_atomic(os.path.join(out, "summary.md"), text.encode("utf-8"))
    print(text, end="", flush=True)
    return code
