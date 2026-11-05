"""The race row (owner, 2026-11-05: pulled forward from S3.2b): `go test -race -json`, with cgo on for this row alone,
because the race detector needs it on Linux (ADR-0015 §3.6, residue item 9: the C compiler is unpinned).

It is judged as the unit row is: a data race fails the test it happened in, which is named; a skip, a build failure
or no test at all fails too. --repeat=N runs every test N times (go test's -count): the nightly race-stress row.
Arguments are package patterns, by default `./...`.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows
from gate.runner.rows import unit


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "test.json"), os.path.join(row_dir, "test.stderr")
    repeat, patterns = 1, []
    for arg in argv[1:]:
        if arg.startswith("--repeat="):
            repeat = int(arg.removeprefix("--repeat="))
        else:
            patterns.append(arg)
    os.environ["CGO_ENABLED"] = "1"  # this row's own go only: every other row keeps the pinned 0
    code = rows.run_tool(["go", "test", "-race", "-json", f"-count={repeat}", *(patterns or ["./..."])], out, err)
    failures, packages = unit.judge(out, code)
    rows.covered(len(packages))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
