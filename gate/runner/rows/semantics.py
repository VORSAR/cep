"""The semantics row (S3.1-1; owner 2026-11-04): the language's reference checks run on every gate, not by hand.

docs/language/checks/test_semantics.py is the executable oracle the engine's tests cite: SEMANTICS' worked examples
and the RT regressions, run against the reference executor. It prints one `<section> · <desc> · OK` or
`· FAIL: …` line per check and exits non-zero on a failure. Each listed harness runs under the pinned CPython. A
failing check fails the row naming it; a harness that crashes fails naming its exit; and every `@check` the file
declares must report OK, so a check that never ran cannot pass. GATE_COVERED counts the harness files that passed.
"""

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

PYTHON = "python3.15"
DECLARED = re.compile(r"^@check\(", re.M)


def judge(rel: str, code: int, out: str, err: str) -> list[str]:
    """The harness's failures, one story each: its failing checks, else its crash, else a check that never ran."""
    lines = out.splitlines()
    failed = [f"{rel}: {line}" for line in lines if " · FAIL: " in line]
    if failed:
        return failed
    if code != 0:
        last = (err.strip().splitlines() or ["no output"])[-1]
        return [f"{rel}: exit {code}: {last}"]
    declared = len(DECLARED.findall(rows.read_text(rel)))
    ok = sum(1 for line in lines if line.endswith(" · OK"))
    if declared == 0 or ok != declared:
        return [f"{rel}: {ok} of its {declared} declared checks reported OK"]
    return []


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    files = [rows.show(path) for path in rows.read_list(argv[1])]
    failures: list[str] = []
    passed = 0
    for i, rel in enumerate(files):
        out, err = os.path.join(row_dir, f"{i}.stdout"), os.path.join(row_dir, f"{i}.stderr")
        code = rows.run_tool([PYTHON, "-I", "-B", "-X", "utf8", rel], out, err)
        found = judge(rel, code, rows.read_text(out), rows.read_text(err))
        failures += found
        passed += not found
    rows.covered(passed)
    return rows.finish(failures)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
