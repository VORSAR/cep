"""The real gate.runner.main with a frozen clock and a passing stand-in toolchain, for determinism tests.

    python -s -B -P frozen_run.py <hash-file> <main's arguments>

Run without -I, so PYTHONHASHSEED applies. Writes this run's hash("abc") to <hash-file>: the witness that the seed
really changed between runs.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import clock, main
from gate.toolchain.parity import Outcome


class StandIn:
    def __init__(self, repo: str, environ: dict[str, str]) -> None:
        pass

    def path_dirs(self) -> list[str]:
        return [os.path.dirname(sys.executable)]

    def tool_count(self) -> int:
        return 5

    def parity(
        self, env: dict[str, str], *, full_tree: bool, spawn: object, scratch: str, run_id: str, handoff_in: str | None
    ) -> Outcome:
        return Outcome(reasons=(), covered=5, taken=0, taken_from="")


if __name__ == "__main__":
    with open(sys.argv[1], "w", encoding="utf-8") as f:
        f.write(str(hash("abc")))
    frozen = clock.Clock(wall_ms=lambda: 0, mono_ns=lambda: 0)
    deps = main.Deps(
        repo=os.getcwd(),
        environ=os.environ,
        clock=frozen,
        timers=clock.RealTimers(),
        toolchain=StandIn,
        commit=lambda repo, env, run_dir, tier: "0" * 40,
        new_run_id=lambda wall_ms: "19700101T000000Z-000000",
        record_go_env=False,
    )
    sys.exit(main.main(sys.argv[2:], deps=deps))
