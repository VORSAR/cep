"""The aging row (S1.1c-3 plan check §2 S2; design note P2): a known issue past its clock is red, naming its id.

W0 stub: it fails by name until lane E2 builds it. The rule itself is gate/runner/aging.py.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(0)
    return rows.finish(["aging: not built yet (S1.1c-3 lane E2)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
