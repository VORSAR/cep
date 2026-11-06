"""The GitHub Actions workflow, generated from the registry (gate design §12.6; plan check P7; owner 2026-11-02).

    gate/run.py generate-ci        (make ci-workflow)

One job per registry `job` at T4, each running that job's rows with the same runner as `make strict-gate`; then one
`strict-gate` job that always runs and judges every job's records against the registry (gate/runner/summarize.py).
That job is the one required check. The `ci-fresh` row fails when the tracked file differs from this output, so the
workflow can never drift from the registry. Only the four actions the owner approved are used, each pinned by full
commit SHA (owner, 2026-11-03).
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


def _uses(name: str) -> str:
    repo, sha, version = ACTIONS[name]
    return f"uses: {repo}@{sha}  # {version}"


def _setup_steps() -> list[str]:
    return [
        f"      - {_uses('checkout')}",
        "        with:",
        "          persist-credentials: false",
        f"      - {_uses('cache')}",
        "        with:",
        f"          path: {TOOLS_CACHE}",
        "          key: tools-${{ runner.os }}-${{ hashFiles('tools.json') }}",
        f"      - {_uses('cache')}",
        "        with:",
        "          path: |",
        "            ~/.cache/go-build",
        "            ~/go/pkg/mod",
        "          key: go-${{ runner.os }}-${{ hashFiles('go.mod', 'go.sum') }}-${{ github.sha }}",
        "          restore-keys: go-${{ runner.os }}-${{ hashFiles('go.mod', 'go.sum') }}-",
        "      - run: make setup tools",
    ]


def _job(job: str, timeout_min: int, tier: str) -> list[str]:
    run_dir = f".gate/ci/{job}"
    return [
        f"  {job}:",
        f"    runs-on: {RUNNERS.get(job, 'ubuntu-latest')}",
        f"    timeout-minutes: {timeout_min}",
        "    steps:",
        *_setup_steps(),
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
    for job in jobs:
        # A job's cap: its rows' watchdogs summed, plus a margin for setup (ADR-0015 §1.2: + 20%, at least 60 s).
        seconds = sum(r.timeout_s for r in chosen if r.job == job)
        lines += _job(job, max(10, -(-(seconds * 6 // 5 + 60) // 60) + 10), tier)
    lines += _strict_gate(jobs, tier)
    return "\n".join(lines) + "\n"


def targets(rows: Sequence[Row]) -> list[tuple[str, str]]:
    """Every workflow the registry calls for, as (path, bytes): one per tier in WORKFLOWS that has rows."""
    out = []
    for tier, path in WORKFLOWS.items():
        if any(tier in r.tiers for r in rows):
            out.append((path, render(rows, tier)))
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
