"""The vulns row (S1.1c-4 plan check §2 S1; design note P1; owner 2026-11-13, S1.1c-4's plan check, decisions 1 and
2): a known vulnerability in a module the build lists select, the standard library or the toolchain at the pinned Go
is red, naming its id, unless the dependency register marks the module never built or tools/allowlists/vulns.json
holds a dated exemption for it.

W0 stub: it fails by name until lane E1 builds it. The rule itself is gate/runner/vulndb.py.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows


def main(argv: list[str]) -> int:
    rows.start()
    rows.covered(0)
    return rows.finish(["vulns: not built yet (S1.1c-4 lane E1)"])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
