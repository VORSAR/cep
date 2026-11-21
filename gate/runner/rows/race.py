"""The race row (owner, 2026-11-05: pulled forward from S3.2b): `go test -race -json`, with cgo on for this row alone,
because the race detector needs it on Linux (ADR-0015 §3.6, residue item 9: the C compiler is unpinned).

It is judged as the unit row is: a data race fails the test it happened in, which is named; a skip, a build failure
or no test at all fails too. --repeat=N runs every test N times (go test's -count): the nightly race-stress row.
With --repeat, go test gets -timeout 25m: its 10m default spans all N runs, so 20 overran; 25m fits the row's 1800 s.
The nightly job runs race-stress beside the two fuzz rows (GATE_FUZZTIME 10m each, as many workers as cores), so its
run time depends on the fuzz budget and the runner's cores: revisit 25m if either, or --repeat, rises.
Arguments are package patterns, by default `./...`.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows
from gate.runner.rows import unit

STRESS_TIMEOUT_MIN = 25  # go test's -timeout under --repeat; test_rows_go pins it inside race-stress's timeout_s
STRESS_TIMEOUT = f"{STRESS_TIMEOUT_MIN}m"


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "test.json"), os.path.join(row_dir, "test.stderr")
    repeat, patterns = 1, []
    for arg in argv[1:]:
        if arg.startswith("--repeat="):
            value = arg.removeprefix("--repeat=")
            try:
                repeat = int(value)
            except ValueError:
                rows.covered(0)
                return rows.finish([f"usage: --repeat=N with N a whole number, not {value!r}"])
        else:
            patterns.append(arg)
    os.environ["CGO_ENABLED"] = "1"  # this row's own go only: every other row keeps the pinned 0
    timeout = ["-timeout", STRESS_TIMEOUT] if any(a.startswith("--repeat=") for a in argv[1:]) else []
    command = ["go", "test", "-race", "-json", f"-count={repeat}", *timeout, *(patterns or ["./..."])]
    code = rows.run_tool(command, out, err)
    failures, packages = unit.judge(out, code)
    rows.covered(len(packages))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
