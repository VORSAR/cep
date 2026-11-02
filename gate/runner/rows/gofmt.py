"""The gofmt row: `gofmt -l` over files.nul in batches of at most 256 (design note §5).

GATE_COVERED counts the files in the batches gofmt completed.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

BATCH = 256


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    files = rows.read_list(argv[1])
    done = 0
    failures: list[str] = []
    for index, start in enumerate(range(0, len(files), BATCH)):
        batch = files[start : start + BATCH]
        out = os.path.join(row_dir, f"gofmt.{index}.stdout")
        err = os.path.join(row_dir, f"gofmt.{index}.stderr")
        code = rows.run_tool(["gofmt", "-l", "--", *batch], out, err)
        if code == 0:
            done += len(batch)
            failures += [
                f"{name} is not formatted: fix it with gofmt -w {name}"
                for name in rows.read_text(out).split("\n")
                if name
            ]
        else:
            failures.append(f"gofmt exit {code}: {' '.join(rows.read_text(err).split())}")
    rows.covered(done)
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
