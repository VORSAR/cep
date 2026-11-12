"""The testhygiene row, S1.1c-2 lane C: no wall-clock sleep or timer in a test outside bench/ and gate/proofs/
(testing.md rule 1); replaced by archcheck's -tests mode.

This is the G0 stub (plan check v2 §5 P9): it fails by name until lane C replaces it, never passes.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(0)
    return rows.finish(["testhygiene: not built yet (lane C)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
