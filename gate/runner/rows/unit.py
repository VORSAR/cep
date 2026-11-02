"""The unit row: `go test -json -count=1 -coverprofile` (design note §5; plan check S3d).

test.json and cover.prof stay in the row's directory for coverage-inclusion. GATE_COVERED counts the distinct
packages with a package-level pass, fail or skip. A run with no `run` event at all fails: `go test` exits 0 on that.
Arguments are package patterns, by default `./...` (with none, go would test only the current directory).
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import jsonstream, rows

PACKAGE_ENDS = frozenset({"pass", "fail", "skip"})


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "test.json"), os.path.join(row_dir, "test.stderr")
    profile = os.path.join(row_dir, "cover.prof")
    argv_go = ["go", "test", "-json", "-count=1", f"-coverprofile={profile}", *(argv[1:] or ["./..."])]
    code = rows.run_tool(argv_go, out, err)
    failures: list[str] = []
    with open(out, "rb") as f:
        data = f.read()
    try:
        events = jsonstream.parse(data)
    except ValueError as e:
        events = []
        failures.append(f"unreadable go test output: {e}")
    packages = {e.get("Package", "") for e in events if "Test" not in e and e.get("Action") in PACKAGE_ENDS}
    failures += [f"{e.get('Package')}: {e['Test']}" for e in events if e.get("Action") == "fail" and "Test" in e]
    failures += [f"{e.get('Package')}: package failed" for e in events if e.get("Action") == "fail" and "Test" not in e]
    if not any(e.get("Action") == "run" for e in events):
        failures.append("no test ran")
    if code != 0:
        failures.append(f"go test exit {code}")
    rows.covered(len(packages))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
