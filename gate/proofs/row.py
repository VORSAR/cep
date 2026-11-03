"""The proofs row: every planted-defect driver, each in its own process (design note §5, §8; ADR-0015 §3.4).

    python3.15 -I -B -X utf8 gate/proofs/row.py {files}

The drivers run as the pytests row runs its modules: under the pinned 3.15.2 with -I -B -X dev -W error, through
gate/runner/pyshim.py, which fails a skip, an expected failure, an unexpected success, zero tests, an import error
and a leak. GATE_COVERED counts the drivers that passed; the row's min_wants pins how many there must be, so a
deleted driver is red even though the live scope shrinks with it (QA DQ4, C38).
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate.runner import rows

PYTHON = "python3.15"


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    passed, problems = rows.run_modules(PYTHON, [rows.show(p) for p in rows.read_list(argv[1])], row_dir)
    rows.covered(passed)
    return rows.finish(problems)


if __name__ == "__main__":
    try:
        code = main(sys.argv)
    except BaseException as e:
        # Exit 1 means "a driver failed"; a crash must never pass for it. SystemExit carries the 2 out.
        print(f"internal error: {type(e).__name__}: {e}", "GATE FAILED", "exit: 2", sep="\n", flush=True)
        raise SystemExit(2) from e
    sys.exit(code)
