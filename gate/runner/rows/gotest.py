"""A row that runs named Go tests: `go test -json -count=1 -run <pattern> <packages>` (S3.2b-2: the tripwire and
determinism rows, design s3-2b-2 §8, §9).

It is judged as the unit row is: a failing or skipped test, a build failure or no test at all fails. GATE_COVERED
counts the tests that ran and passed, so a pattern that matches nothing is red through min_wants.
Arguments: `-run <pattern>`, then package patterns.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import jsonstream, rows
from gate.runner.rows import unit


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "test.json"), os.path.join(row_dir, "test.stderr")
    args = argv[1:]
    if len(args) < 3 or args[0] != "-run":
        print("usage: gotest.py -run <pattern> <package>...", file=sys.stderr)
        return 2
    code = rows.run_tool(["go", "test", "-json", "-count=1", "-run", args[1], *args[2:]], out, err)
    failures, _ = unit.judge(out, code)
    with open(out, "rb") as f:
        try:
            events = jsonstream.parse(f.read())
        except ValueError:
            events = []
    rows.covered(len({(e.get("Package"), e["Test"]) for e in events if e.get("Action") == "pass" and "Test" in e}))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
