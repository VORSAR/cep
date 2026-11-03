"""The `ci-fresh` row (gate design §12.6–12.7): the tracked workflow is exactly what the registry generates.

The workflow is generated, never hand-written, so the registry is its one source (plan check P7; architect A11).
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.ci import generate
from gate.runner import registry, rows


def main() -> int:
    row_dir = rows.start()
    repo = os.getcwd()
    want = generate.render(registry.load(os.path.join(repo, generate.REGISTRY)))
    with open(os.path.join(row_dir, "gate.yml"), "w", encoding="utf-8") as f:
        f.write(want)
    with open(os.path.join(repo, generate.WORKFLOW), "rb") as f:
        have = f.read().decode("utf-8", errors="replace")
    if have != want:
        return rows.finish([f"{generate.WORKFLOW} is not what {generate.REGISTRY} generates: run make ci-workflow"])
    rows.covered(1)
    return rows.finish([])


if __name__ == "__main__":
    sys.exit(main())
