"""The docsync row, S1.1c-2 lane D: every Go package go list reports is listed in ARCHITECTURE §3
(ADR-0015 §2 docsync (a)).

This is the G0 stub (plan check v2 §5 P9): it fails by name until lane D replaces it, never passes.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(0)
    return rows.finish(["docsync: not built yet (lane D)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
