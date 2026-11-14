"""The smells row (S1.1c-5 design note §2.3): golangci-lint over the tree under linux and darwin, the config pinned
whole by gate/runner/golangci.py, the size family's exclusions from tools/allowlists/smells.json.

W0c stub: reports no coverage and fails until lane A writes it.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))


def main(argv: list[str]) -> int:
    rows.covered(0)
    print("failed: smells: S1.1c-5 lane A: the smells row is not written yet")
    sys.stdout.flush()
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
