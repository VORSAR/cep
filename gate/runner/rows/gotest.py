"""A row that runs named Go tests: `go test -json -count=1 -run <pattern> <packages>` (S3.2b-2: the tripwire and
determinism rows, design s3-2b-2 §8, §9).

It is judged as the unit row is: a failing or skipped test, a build failure or no test at all fails. GATE_COVERED
counts the packages the run tested, as the unit row does, so it equals the row's `go` expected_scope however many
tests and sub-tests the pattern matches (S3.3-2: the conformance runner's twelve sub-tests read 13 and turned a
green row red); a pattern that matches nothing is red through the judge's "no test ran".
Arguments: `-run <pattern>`, then package patterns.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

import re

from gate.runner import jsonstream, rows
from gate.runner.rows import unit

# An anchored list of test names: ^Name$ or ^(A|B|…)$.
_NAMES = re.compile(r"^\^(?:\((\w+(?:\|\w+)*)\)|(\w+))\$$")


def missing_names(pattern: str, events: list[dict]) -> list[str]:
    """For a pattern that names tests, ^A$ or ^(A|B)$, one failure per name that never ran as a top-level test (the
    package count alone reads green when one of two names is misspelt: the S3.3-2 combined review's R-L1). Any other
    pattern checks nothing here."""
    m = _NAMES.match(pattern)
    if not m:
        return []
    names = (m.group(1) or m.group(2)).split("|")
    ran = {e.get("Test") for e in events if e.get("Action") == "run"}
    return [f"no test ran: {name}" for name in names if name not in ran]


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "test.json"), os.path.join(row_dir, "test.stderr")
    args = argv[1:]
    if len(args) < 3 or args[0] != "-run":
        print("usage: gotest.py -run <pattern> <package>...", file=sys.stderr)
        return 2
    code = rows.run_tool(["go", "test", "-json", "-count=1", "-run", args[1], *args[2:]], out, err)
    failures, packages = unit.judge(out, code)
    with open(out, "rb") as f:
        try:
            events = jsonstream.parse(f.read())
        except ValueError:
            events = []  # judge has already reported it
    failures += missing_names(args[1], events)
    rows.covered(len(packages))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
