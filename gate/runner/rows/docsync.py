"""The docsync row (S1.1c-2 design §4.1; ADR-0015 §2 docsync (a)): every Go package `go list` reports is listed in
ARCHITECTURE §3's tree.

It runs `go list -e -f '{{.Dir}}' ./...` at the repository root with GOFLAGS=-mod=readonly and GOPROXY=off, reads
its exit code and ignores its stderr (offline, `go: downloading …` lands there, design §0.7), makes each directory
repository-relative, and names each package §3 does not list. GATE_COVERED is the number of packages go list
reported. The rule itself is gate/runner/docsync.py.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import docsync, rows

ARCHITECTURE = "docs/architecture/ARCHITECTURE.md"
GO_LIST = ("go", "list", "-e", "-f", "{{.Dir}}", "./...")
ENV = {"GOFLAGS": "-mod=readonly", "GOPROXY": "off"}


def packages(row_dir: str) -> list[str]:
    """go list's directories, repository-relative ("." for the root); OSError naming the exit code on a failure."""
    out, err = os.path.join(row_dir, "go-list.stdout"), os.path.join(row_dir, "go-list.stderr")
    os.environ.update(ENV)
    code = rows.run_tool(list(GO_LIST), out, err)
    if code != 0:
        raise OSError(f"go list failed (exit {code})")
    root = os.getcwd()
    return sorted({os.path.relpath(d, root).replace(os.sep, "/") for d in rows.read_text(out).splitlines() if d})


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    try:
        found = packages(row_dir)
    except OSError as e:
        rows.covered(0)
        return rows.finish([f"docsync: {e}"])
    try:
        names = docsync.listed(docsync.tree(rows.read_text(ARCHITECTURE)))
    except (OSError, ValueError) as e:
        rows.covered(0)
        return rows.finish([f"docsync: {ARCHITECTURE}: {e}"])
    rows.covered(len(found))
    return rows.finish(
        [
            f"docsync: {pkg} is a Go package and ARCHITECTURE §3 does not list it"
            for pkg in docsync.missing(found, names)
        ]
    )


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
