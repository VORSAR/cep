"""The pyfloor row: the floor modules' tests, run under the pinned 3.12.16 through the pytests shim (design note §5).

That verifies syntax and library use at the bootstrap's floor, which a parse under `feature_version` cannot.
GATE_COVERED counts the modules that passed.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

PYTHON = "python3.12"


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    passed, problems = rows.run_modules(PYTHON, [rows.show(p) for p in rows.read_list(argv[1])], row_dir)
    rows.covered(passed)
    return rows.finish(problems)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
