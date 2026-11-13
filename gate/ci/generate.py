"""The GitHub Actions workflow, generated from the registry (gate design §12.6; plan check P7; owner 2026-11-02).

    gate/run.py generate-ci        (make ci-workflow)

One job per registry `job` at T4, each running that job's rows with the same runner as `make strict-gate`; then one
`strict-gate` job that always runs and judges every job's records against the registry (gate/runner/summarize.py).
That job is the one required check. The `ci-fresh` row fails when the tracked file differs from this output, so the
workflow can never drift from the registry. Only the four actions the owner approved are used, each pinned by full
commit SHA (owner, 2026-11-03).

Beside the gate, the weekly canary (S1.1c-4 design §4.7): the same T4 jobs run on a candidate tools.json of each pin's
newest patch release, then a summary that sorts the reds into pins to update and breaks. It is a report, never a
required check, and restores caches without saving them (S1.1c-4's plan check, confirms (f), (g)).
"""

import os
from collections.abc import Sequence

from gate.runner import registry
from gate.runner.registry import Row

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
REGISTRY = "gate/registry.json"
WORKFLOW = ".github/workflows/gate.yml"
TIER = "T4"
# Each tier with rows of its own gets a workflow: the gate on pull requests and main, nightly on a schedule
# (owner, 2026-11-04: rows marked nightly never run on a pull request).
WORKFLOWS = {"T4": WORKFLOW, "nightly": ".github/workflows/nightly.yml"}
# The weekly canary, rendered beside the gate's workflow from the same T4 jobs (S1.1c-4 design §4.7).
CANARY = ".github/workflows/canary.yml"
CANARY_CRON = "0 6 * * 1"  # Mondays, 06:00 UTC
# The candidate job's directory and artifact: feeds.json, tools.json, upgrades.json, or error.txt (design §4.7).
CANDIDATE = "candidate"

# name: (repository, full commit SHA, the release it is). Licences: all MIT (checked 2026-11-04).
ACTIONS = {
    "checkout": ("actions/checkout", "3d3c42e5aac5ba805825da76410c181273ba90b1", "v7.0.1"),
    "cache": ("actions/cache", "55cc8345863c7cc4c66a329aec7e433d2d1c52a9", "v6.1.0"),
    "upload": ("actions/upload-artifact", "cf430e030ddbb5b0abf93d22962f4752f3646cd9", "v7.0.2"),
    "download": ("actions/download-artifact", "9000827ccba6bdab643e8b6fd33ac0654aef8333", "v8.0.2"),
}
RUN_ID = "${{ github.run_id }}-${{ github.run_attempt }}"
# A job's runner when it is not ubuntu-latest: the determinism proof's second OS (S3.2b's item 8; owner Q5: macOS).
RUNNERS = {"determinism-macos": "macos-latest"}
# The tool cache stays where a developer has it, outside the checkout: inside it, the project's go.mod would sit
# above every tool and change what go does (the first CI run, 2026-11-04).
TOOLS_CACHE = "~/.cache/vorsar/cep/tools"


def _uses(name: str, sub: str = "") -> str:
    """An approved action at its pinned SHA; `sub` names an action inside its repository (actions/cache/restore)."""
    repo, sha, version = ACTIONS[name]
    return f"uses: {repo}{sub}@{sha}  # {version}"


def _cache(restore_only: bool) -> str:
    """The canary restores caches and never saves one (confirm (g)): actions/cache/restore at the pinned SHA."""
    return _uses("cache", "/restore" if restore_only else "")


# Rows that compare against the merge base with main: their job checks out the full history (a shallow checkout has
# no main, so apidiff would pass with nothing compared; S3.3-1; lawdelta fails outright, S1.1c-1). Every other job
# keeps the one-commit checkout.
NEEDS_HISTORY = frozenset({"apidiff", "lawdelta"})
# Rows that read the module cache offline: their job downloads every module of both tracked go.mod files at setup
# (owner, 2026-11-11, S1.1c-1's design, decision 1), so a cold cache is never a verdict.
# proofs: the deps driver fingerprints the real modules from the cache; vulns: the scan loads both modules' builds.
NEEDS_MODULES = frozenset({"deps", "proofs", "vulns"})
MODULE_DOWNLOAD = "      - run: go mod download all"
# `go mod download all` writes go.sum lines for modules no build needs; no other form measured both leaves go.sum
# unchanged and lets every offline go read run, so the download is followed by restoring both files (owner,
# 2026-11-13, during S1.1c-4's build, decision 2: form D). The deps row's T4 go.sum line proves nothing is left.
GO_SUM_RESTORE = "      - run: git checkout -- go.sum gate/proofs/go.sum"
MODULE_STEPS = [MODULE_DOWNLOAD, MODULE_DOWNLOAD, "        working-directory: gate/proofs", GO_SUM_RESTORE]
# Rows that read the vulnerability database's local copy: their job fetches it at setup (S1.1c-4 design §3.3). The
# step never fails the job: with no fresh copy the vulns row itself is red, naming why (a failed fetch is a verdict
# of the row, never of setup).
NEEDS_VULNDB = frozenset({"vulns"})
VULNDB_FETCH = ["      - run: make vulndb", "        continue-on-error: true"]
# The module cache's key: both tracked modules, so a change to gate/proofs' requirements is a new key too.
MODULE_FILES = "hashFiles('go.mod', 'go.sum', 'gate/proofs/go.mod', 'gate/proofs/go.sum')"


def _candidate_download(continue_on_error: bool = False) -> list[str]:
    return [
        f"      - {_uses('download')}",
        *(["        continue-on-error: true"] if continue_on_error else []),
        "        with:",
        f"          name: {CANDIDATE}",
        f"          path: {CANDIDATE}",
    ]


def _setup_steps(
    history: bool = False,
    modules: bool = False,
    vulndb: bool = False,
    *,
    restore_only: bool = False,
    swap: bool = False,
) -> list[str]:
    """`swap`: the canary's T4 job puts the candidate tools.json in place before anything reads tools.json."""
    return [
        f"      - {_uses('checkout')}",
        "        with:",
        "          persist-credentials: false",
        *(["          fetch-depth: 0"] if history else []),
        *([*_candidate_download(), f"      - run: cp {CANDIDATE}/tools.json tools.json"] if swap else []),
        f"      - {_cache(restore_only)}",
        "        with:",
        f"          path: {TOOLS_CACHE}",
        "          key: tools-${{ runner.os }}-${{ hashFiles('tools.json') }}",
        f"      - {_cache(restore_only)}",
        "        with:",
        "          path: |",
        "            ~/.cache/go-build",
        "            ~/go/pkg/mod",
        f"          key: go-${{{{ runner.os }}}}-${{{{ {MODULE_FILES} }}}}-${{{{ github.sha }}}}",
        f"          restore-keys: go-${{{{ runner.os }}}}-${{{{ {MODULE_FILES} }}}}-",
        "      - run: make setup tools",
        *(MODULE_STEPS if modules else []),
        *(VULNDB_FETCH if vulndb else []),
    ]


def _job(
    job: str,
    timeout_min: int,
    tier: str,
    history: bool = False,
    modules: bool = False,
    vulndb: bool = False,
    canary: bool = False,
) -> list[str]:
    run_dir = f".gate/ci/{job}"
    return [
        f"  {job}:",
        f"    runs-on: {RUNNERS.get(job, 'ubuntu-latest')}",
        f"    timeout-minutes: {timeout_min}",
        *([f"    needs: {CANDIDATE}"] if canary else []),
        "    steps:",
        *_setup_steps(history, modules, vulndb, restore_only=canary, swap=canary),
        f"      - run: python3 -I -B gate/bootstrap.py exec -- --tier {tier} --job {job} "
        f"--run-id {RUN_ID} --commit ${{{{ github.sha }}}} --run-dir {run_dir}",
        f"      - {_uses('upload')}",
        "        if: always()",
        "        with:",
        f"          name: records-{job}-${{{{ github.run_attempt }}}}",
        f"          path: {run_dir}/",
        "          if-no-files-found: ignore",
    ]


def _downloads(jobs: Sequence[str]) -> list[str]:
    """One download per job, by name, into its own records-<job>-<attempt>/ directory: summarize's layout whatever the
    job count. A pattern download extracts a single match straight into its path (the nightly's first run, 2026-11-05).
    A job that uploaded nothing leaves no directory, and its rows read missing."""
    steps = []
    for job in jobs:
        name = f"records-{job}-${{{{ github.run_attempt }}}}"
        steps += [
            f"      - {_uses('download')}",
            "        continue-on-error: true",
            "        with:",
            f"          name: {name}",
            f"          path: .gate/artifacts/{name}",
        ]
    return steps


def _strict_gate(jobs: Sequence[str], tier: str) -> list[str]:
    return [
        "  strict-gate:",
        "    runs-on: ubuntu-latest",
        "    timeout-minutes: 15",
        f"    needs: [{', '.join(jobs)}]",
        "    if: always()",
        "    steps:",
        *_setup_steps(),
        *_downloads(jobs),
        "      - if: always()",
        f"        run: python3 -I -B gate/bootstrap.py exec -- summarize --registry {REGISTRY} --tier {tier} "
        f"--artifacts .gate/artifacts --run-id {RUN_ID} --commit ${{{{ github.sha }}}} "
        '--attempt ${{ github.run_attempt }} --image "$ImageOS $ImageVersion"',
    ]


def render(rows: Sequence[Row], tier: str = TIER) -> str:
    """The workflow's exact bytes for the registry rows at one tier."""
    chosen = registry.select(list(rows), tier, None)
    jobs = list(dict.fromkeys(r.job for r in chosen))
    nightly = tier == "nightly"
    lines = [
        f"# Generated from {REGISTRY} by make ci-workflow. Do not edit: the ci-fresh row checks it.",
        f"name: {'nightly' if nightly else 'gate'}",
        "on:",
        *(
            ["  schedule:", "    - cron: '0 3 * * *'", "  workflow_dispatch:"]
            if nightly
            else ["  pull_request:", "  push:", "    branches: [main]"]
        ),
        "permissions:",
        "  contents: read",
        "concurrency:",
        *(
            ["  group: nightly", "  cancel-in-progress: false"]
            if nightly
            else ["  group: gate-${{ github.ref }}", "  cancel-in-progress: ${{ github.event_name == 'pull_request' }}"]
        ),
        "env:",
        "  CGO_ENABLED: '0'",
        "jobs:",
    ]
    lines += _jobs(chosen, tier)
    lines += _strict_gate(jobs, tier)
    return "\n".join(lines) + "\n"


def _jobs(chosen: Sequence[Row], tier: str, canary: bool = False) -> list[str]:
    lines: list[str] = []
    for job in dict.fromkeys(r.job for r in chosen):
        mine = [r for r in chosen if r.job == job]
        # A job's cap: its rows' watchdogs summed, plus a margin for setup (ADR-0015 §1.2: + 20%, at least 60 s).
        seconds = sum(r.timeout_s for r in mine)
        history = any(r.id in NEEDS_HISTORY for r in mine)
        modules = any(r.id in NEEDS_MODULES for r in mine)
        vulndb = any(r.id in NEEDS_VULNDB for r in mine)
        timeout_min = max(10, -(-(seconds * 6 // 5 + 60) // 60) + 10)
        lines += _job(job, timeout_min, tier, history, modules, vulndb, canary)
    return lines


def _exec(command: str) -> str:
    return f"python3 -I -B gate/bootstrap.py exec -- {command}"


def _candidate_job() -> list[str]:
    """Reads the feeds with the read-only token, then builds the candidate with none (reviews A-S5, DA-S7), on Linux:
    python-build-standalone's Linux archives hold names that differ only by case (lane F1). Its directory uploads
    whatever happened, error.txt included (review DQ-S9)."""
    return [
        f"  {CANDIDATE}:",
        "    runs-on: ubuntu-latest",
        "    timeout-minutes: 30",
        "    steps:",
        *_setup_steps(restore_only=True),
        "      - name: feeds",
        "        env:",
        "          GITHUB_TOKEN: ${{ github.token }}",
        f"        run: {_exec(f'canary-feeds --out {CANDIDATE}/feeds.json')}",
        "      - name: build",
        f"        run: {_exec(f'canary-candidate --feeds {CANDIDATE}/feeds.json --out {CANDIDATE}/')}",
        f"      - {_uses('upload')}",
        "        if: always()",
        "        with:",
        f"          name: {CANDIDATE}",
        f"          path: {CANDIDATE}/",
        "          if-no-files-found: ignore",
        "          overwrite: true",
    ]


def _canary_summary(jobs: Sequence[str]) -> list[str]:
    """Always runs, after every job, and reads each job's result from toJSON(needs) (review DA-S8)."""
    command = (
        f"canary-summary --artifacts .gate/artifacts --{CANDIDATE} {CANDIDATE} --attempt ${{{{ github.run_attempt }}}} "
        '--summary "$GITHUB_STEP_SUMMARY"'
    )
    return [
        "  summary:",
        "    runs-on: ubuntu-latest",
        "    timeout-minutes: 15",
        f"    needs: [{', '.join([CANDIDATE, *jobs])}]",
        "    if: always()",
        "    steps:",
        *_setup_steps(restore_only=True),
        *_downloads(jobs),
        *_candidate_download(continue_on_error=True),
        "      - if: always()",
        "        env:",
        "          NEEDS: ${{ toJSON(needs) }}",
        f"        run: {_exec(command)}",
    ]


def render_canary(rows: Sequence[Row]) -> str:
    """The weekly canary's exact bytes (S1.1c-4 design §4.7): the candidate job, the gate's T4 jobs each on the
    candidate tools.json, and the summary. The T4 jobs are the gate's exactly, on the same runners (owner,
    2026-11-13, S1.1c-4's design, decision 2)."""
    chosen = registry.select(list(rows), TIER, None)
    jobs = list(dict.fromkeys(r.job for r in chosen))
    lines = [
        f"# Generated from {REGISTRY} by make ci-workflow. Do not edit: the ci-fresh row checks it.",
        "name: canary",
        "on:",
        "  schedule:",
        f"    - cron: '{CANARY_CRON}'",
        "  workflow_dispatch:",
        "  pull_request:",
        "    paths: ['gate/canary/**']",
        "permissions:",
        "  contents: read",
        "concurrency:",
        "  group: canary-${{ github.ref }}",
        "  cancel-in-progress: false",
        "env:",
        "  CGO_ENABLED: '0'",
        "jobs:",
        *_candidate_job(),
        *_jobs(chosen, TIER, canary=True),
        *_canary_summary(jobs),
    ]
    return "\n".join(lines) + "\n"


def targets(rows: Sequence[Row]) -> list[tuple[str, str]]:
    """Every workflow the registry calls for, as (path, bytes): one per tier in WORKFLOWS that has rows, and the
    canary after the gate's."""
    out = []
    for tier, path in WORKFLOWS.items():
        if any(tier in r.tiers for r in rows):
            out.append((path, render(rows, tier)))
            if tier == TIER:
                out.append((CANARY, render_canary(rows)))
    return out


def main(argv: Sequence[str]) -> int:
    if argv:
        print(f"generate-ci takes no arguments, got {' '.join(argv)}")
        print("GATE FAILED")
        print("exit: 2")
        return 2
    for rel, text in targets(registry.load(os.path.join(REPO, REGISTRY))):
        path = os.path.join(REPO, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        print(f"wrote {rel}")
    print("GATE PASSED")
    print("exit: 0")
    return 0
