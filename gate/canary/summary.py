"""The weekly canary's summary (S1.1c-4 design §4.6; reviews DA-S8, DQ-S9, QA-S10): Markdown for the job summary, its
first line always `Upgrades: …`, and the exit code — 0 with no breaks, 1 with any (the canary is a report, never a
required check). Pure: the workflow's command reads the artifacts and `toJSON(needs)` and calls `render`.
"""

from collections.abc import Iterable, Mapping, Sequence

from gate.canary.candidate import Candidate, Upgrade

CANDIDATE_JOB = "candidate"
SUCCESS = "success"
# A ruff or golangci-lint upgrade moves the findings test_suppressions.py holds as data (review QA-S10).
SUPPRESSED_TOOLS = frozenset({"ruff", "golangci-lint"})
SUPPRESSION_NOTE = "Suppression tables not re-measured (test_suppressions.py holds them as data)"


def render(
    *,
    candidate: Candidate | None,
    error: str | None,
    needs: Mapping[str, Mapping[str, object]],
    records: Mapping[str, Sequence[Mapping[str, object]]],
    pins: Iterable[object],
    breaks: Iterable[object],
) -> tuple[str, int]:
    """`candidate` the Candidate, or None when the candidate job wrote none; `error` candidate/error.txt's text or
    None; `needs` the parsed `toJSON(needs)`; `records` each T4 job's records (a job absent: its artifact is missing);
    `pins` and `breaks` the sort's lines, printed by their str() in the order given."""
    result = str(needs.get(CANDIDATE_JOB, {}).get("result", "missing"))
    known = candidate if result == SUCCESS and candidate is not None and not _one_line(error or "") else None
    canary = _candidate_breaks(result, candidate, error) + _job_breaks(needs, records)
    all_breaks = canary + [str(b) for b in breaks]
    lines = [
        _upgrades(known, result, candidate, error),
        _section("New lines (not tried)", known.newer_lines if known else None),
        _section("Pins to update", [str(p) for p in pins]),
        _section("Breaks", all_breaks),
    ]
    if known and any(u.name in SUPPRESSED_TOOLS for u in known.upgrades):
        lines.append(SUPPRESSION_NOTE)
    return "\n\n".join(lines) + "\n", 1 if all_breaks else 0


def _upgrades(known: Candidate | None, result: str, candidate: Candidate | None, error: str | None) -> str:
    if known is not None:
        return "Upgrades: " + (", ".join(_upgrade(u) for u in known.upgrades) or "none")
    reason = _one_line(error or "") or ("it wrote no candidate" if result == SUCCESS else "no candidate/error.txt")
    return f"Upgrades: unknown — the candidate job {result}: {reason}"


def _upgrade(u: Upgrade) -> str:
    named = f"{u.name} {u.pinned}" if u.latest == u.pinned else f"{u.name} {u.pinned} → {u.latest}"
    return f"{named} ({u.note})" if u.note else named


def _section(title: str, items: Sequence[str] | None) -> str:
    if items is None:
        return f"{title}: unknown"
    return f"{title}: {', '.join(items) or 'none'}"


def _candidate_breaks(result: str, candidate: Candidate | None, error: str | None) -> list[str]:
    if result != SUCCESS:
        return [f"canary: the {CANDIDATE_JOB} job ended {result}"]
    if candidate is None:
        return [f"canary: the {CANDIDATE_JOB} job wrote no candidate"]
    if _one_line(error or ""):
        return [f"canary: the {CANDIDATE_JOB} job ended {SUCCESS} and wrote candidate/error.txt"]
    return []


def _job_breaks(
    needs: Mapping[str, Mapping[str, object]], records: Mapping[str, Sequence[Mapping[str, object]]]
) -> list[str]:
    """Per T4 job: no records is a break; a job that ended neither success nor failure is one; a failure is read from
    its records (a pin red fails its job) unless every record passed."""
    out = []
    for job in sorted(set(needs) - {CANDIDATE_JOB}):
        result = str(needs[job].get("result", "missing"))
        got = records.get(job) or []
        if not got:
            out.append(f"canary: no records from job {job}")
        elif result == "failure" and all(r.get("verdict") == "pass" for r in got):
            out.append(f"canary: the job {job} ended failure with every record passing")
        elif result not in (SUCCESS, "failure"):
            out.append(f"canary: the job {job} ended {result}")
    return out


def _one_line(text: str) -> str:
    return "; ".join(line.strip() for line in text.splitlines() if line.strip())
