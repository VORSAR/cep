"""The fuzz row (S2.2a; owner 2026-11-04): run each listed Go fuzz target for a while, keep what crashes.

Each listed file holds one or more `Fuzz*` targets. Each target runs for GATE_FUZZTIME (default 10m) through the
pinned go. A crash fails the row naming the file and the target; go writes the crashing input under the package's
testdata/fuzz/<Target>/, and the row copies it into crashers/ in its own directory, which CI uploads. The owner
commits a crasher as a regression test, and it blocks every run from then on (advisory aging, as decided for now).
GATE_COVERED counts the targets run.
"""

import os
import re
import shutil
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

TARGET = re.compile(r"^func (Fuzz\w*)\(", re.M)


def targets(path: str) -> list[str]:
    return TARGET.findall(rows.read_text(path))


def main() -> int:
    row_dir = rows.start()
    fuzztime = os.environ.get("GATE_FUZZTIME", "10m")
    files = [rows.show(p) for p in rows.read_list(os.path.join(row_dir, "files.nul"))]
    failures = []
    count = 0
    for rel in files:
        pkg = "./" + os.path.dirname(rel)
        for target in targets(rel):
            count += 1
            out = os.path.join(row_dir, f"{target}.stdout")
            argv = ["go", "test", "-run", "^$", "-fuzz", f"^{target}$", "-fuzztime", fuzztime, pkg]
            code = rows.run_tool(argv, out, os.path.join(row_dir, f"{target}.stderr"))
            found = os.path.join(os.path.dirname(rel), "testdata", "fuzz", target)
            if code != 0:
                if os.path.isdir(found):
                    shutil.copytree(found, os.path.join(row_dir, "crashers", target), dirs_exist_ok=True)
                failures.append(f"{rel}: {target} crashed: see crashers/")
    if not failures:
        rows.covered(count)
    return rows.finish(failures)


if __name__ == "__main__":
    sys.exit(main())
