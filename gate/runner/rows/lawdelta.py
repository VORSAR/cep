"""The lawdelta row, S1.1c-1 lane A: the LAW Δ block (ADR-0015 §1.3).

This is the G0 stub (plan check v2 §5 P5): it fails by name until lane A replaces it, never passes.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(1)
    return rows.finish(["lawdelta: not built yet (lane A)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
