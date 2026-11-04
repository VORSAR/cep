"""The `ci-fresh` row (gate design §12.6–12.7): every tracked workflow is exactly what the registry generates.

The workflows are generated, never hand-written, so the registry is their one source (plan check P7; architect
A11). The row's scope lists the tracked workflow files; each must be a current target of the generator.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.ci import generate
from gate.runner import registry, rows


def main() -> int:
    row_dir = rows.start()
    repo = os.getcwd()
    tracked = [rows.show(p) for p in rows.read_list(os.path.join(row_dir, "files.nul"))]
    wanted = dict(generate.targets(registry.load(os.path.join(repo, generate.REGISTRY))))
    failures = []
    for rel in tracked:
        with open(os.path.join(row_dir, os.path.basename(rel)), "w", encoding="utf-8") as f:
            f.write(wanted.get(rel, ""))
        if rel not in wanted:
            failures.append(f"{rel} is tracked but {generate.REGISTRY} calls for no such workflow: git rm it")
            continue
        with open(os.path.join(repo, rel), "rb") as f:
            have = f.read().decode("utf-8", errors="replace")
        if have != wanted[rel]:
            failures.append(f"{rel} is not what {generate.REGISTRY} generates: run make ci-workflow")
    for rel in wanted:
        if rel not in tracked:
            failures.append(f"{rel} is not tracked: run make ci-workflow and git add it")
    if not failures:
        rows.covered(len(tracked))
    return rows.finish(failures)


if __name__ == "__main__":
    sys.exit(main())
