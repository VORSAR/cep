"""The cpu row (S1.1c-11, lane C): the Go tests run at two GOMAXPROCS values, `go test -json -count=1 -cpu=1` and then
`-cpu=4`, with the race detector off, over the row's package patterns (by default `./...`).

go test's `-cpu 1,4` runs every test once at each value, but its -json output names a test the same way at both, so
its failure line could not say which value failed. Two invocations can: each is judged as the unit row is
(unit.judge: a failing or skipped test, a build failure or no test at all fails), and each of its failure lines
starts with its value, `-cpu 1: <package>: <Test>`. GATE_COVERED counts the packages both invocations tested.
Each invocation gets go test -timeout TIMEOUT: it bounds one test binary, and test_rows_go pins two of them, each
with cmd/go's kill margin, plus a build-and-report headroom, inside the row's timeout_s. Any argument that starts
with a dash is a usage failure before go runs.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows
from gate.runner.rows import unit

CPUS = (1, 4)  # one GOMAXPROCS value per invocation, in this order
TIMEOUT_MIN = 15  # go test's -timeout for each invocation; test_rows_go pins it inside the cpu row's timeout_s
TIMEOUT = f"{TIMEOUT_MIN}m"


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    patterns = argv[1:] or ["./..."]
    for arg in patterns:
        if arg.startswith("-"):  # no package pattern starts with a dash: a go flag would change what is judged
            rows.covered(0)
            return rows.finish([f"usage: {arg!r} is not a package pattern"])
    failures: list[str] = []
    tested: set[str] | None = None
    for cpu in CPUS:
        out = os.path.join(row_dir, f"test.cpu{cpu}.json")
        err = os.path.join(row_dir, f"test.cpu{cpu}.stderr")
        command = ["go", "test", "-json", "-count=1", f"-cpu={cpu}", "-timeout", TIMEOUT, *patterns]
        found, packages = unit.judge(out, rows.run_tool(command, out, err))
        failures += [f"-cpu {cpu}: {line}" for line in found]
        tested = packages if tested is None else tested & packages
    rows.covered(len(tested or ()))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
