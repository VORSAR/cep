"""The suppressions row, S1.1c-2 lane A: every //nolint and every tools/allowlists/*.json entry in the one
grammar (DEVELOPMENT.md's suppression policy; ADR-0015 §3.10).

This is the G0 stub (plan check v2 §5 P9): it fails by name until lane A replaces it, never passes.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(0)
    return rows.finish(["suppressions: not built yet (lane A)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
