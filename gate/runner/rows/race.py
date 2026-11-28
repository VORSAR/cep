"""The race row (owner, 2026-11-05: pulled forward from S3.2b): `go test -race -json`, with cgo on for this row alone,
because the race detector needs it on Linux (ADR-0015 §3.6, residue item 9: the C compiler is unpinned).

It is judged as the unit row is: a data race fails the test it happened in, which is named; a skip, a build failure
or no test at all fails too. --repeat=N runs every test N times (go test's -count): the nightly race-stress rows.
N is a whole number from 1 to 999999999 in ASCII digits with no sign, space, underscore or leading zero (covered.py's
grammar for a count). --timeout=Nm is go test's -timeout in whole minutes, N from 1 to 999 in the same grammar and
the unit `m` alone (S1.1c-11): a race-stress row may name its own, chosen from a measured run of its package.
With --repeat and no --timeout, go test gets -timeout 25m (its 10m default spans all N runs, so 20 overran); with
neither, go's default stands. Any other value, and any other argument that starts with a dash, is a usage failure
before go runs. go's -timeout bounds one test binary; test_rows_go pins each race-stress row's go timeout, with
cmd/go's kill margin and a build-and-report headroom, inside its timeout_s. The nightly job runs race-stress beside
the two fuzz rows (GATE_FUZZTIME 10m each, as many workers as cores), so its run time depends on the fuzz budget
and the runner's cores: revisit 25m if either, or --repeat, rises.
Arguments are package patterns, by default `./...`.
"""

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows
from gate.runner.rows import unit

STRESS_TIMEOUT_MIN = 25  # go test's -timeout under --repeat with no --timeout; test_rows_go pins it like the rest
STRESS_TIMEOUT = f"{STRESS_TIMEOUT_MIN}m"
COUNT = re.compile(r"[1-9][0-9]{0,8}")  # --repeat's N: covered.py's grammar for a count
MINUTES = re.compile(r"([1-9][0-9]{0,2})m")  # --timeout's Nm: the count's grammar, at most 999 minutes


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "test.json"), os.path.join(row_dir, "test.stderr")
    repeat, minutes, patterns = 1, None, []
    stress = False
    for arg in argv[1:]:
        if arg.startswith("--repeat="):  # -count, and the stress timeout unless --timeout names one
            value = arg.removeprefix("--repeat=")
            if not COUNT.fullmatch(value):  # int() alone takes '+3', ' 3' or '1_0'; go test exits 2 at 2**63
                rows.covered(0)
                return rows.finish([f"usage: --repeat=N with N a whole number from 1 to 999999999, not {value!r}"])
            repeat, stress = int(value), True
        elif arg.startswith("--timeout="):
            value = arg.removeprefix("--timeout=")
            m = MINUTES.fullmatch(value)
            if m is None:  # 0m would turn go's timeout off; 999999999m overflows go's duration
                rows.covered(0)
                return rows.finish(
                    [f"usage: --timeout=Nm with N a whole number of minutes from 1 to 999, not {value!r}"]
                )
            minutes = int(m.group(1))
        elif arg.startswith("-"):  # no package pattern starts with a dash: `--repeat 20` or a go flag
            rows.covered(0)
            return rows.finish([f"usage: {arg!r} is not a package pattern, --repeat=N or --timeout=Nm"])
        else:
            patterns.append(arg)
    if minutes is None and stress:
        minutes = STRESS_TIMEOUT_MIN
    timeout = [] if minutes is None else ["-timeout", f"{minutes}m"]
    os.environ["CGO_ENABLED"] = "1"  # this row's own go only: every other row keeps the pinned 0
    command = ["go", "test", "-race", "-json", f"-count={repeat}", *timeout, *(patterns or ["./..."])]
    code = rows.run_tool(command, out, err)
    failures, packages = unit.judge(out, code)
    rows.covered(len(packages))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
