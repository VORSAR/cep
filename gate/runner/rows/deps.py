"""The deps row, S1.1c-1 lane C: the dependency allowlist, licence and one-module checks (ADR-0009 §1, ADR-0001 §1).

This is the G0 stub (plan check v2 §5 P5): it fails by name until lane C replaces it, never passes.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(3)
    return rows.finish(["deps: not built yet (lane C)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
