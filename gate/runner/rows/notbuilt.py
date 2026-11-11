"""The notbuilt row (S1.1c-1 lane D; design note §3.4; ADR-0005 §4 as annotated; owner 2026-11-09, 6; 2026-11-11, 6).

The not-built expiry, at T4 by the registry's tier: gate/current-sprint names the sprint under way; gate/sprints.json
gives the plan table's printed order; conformance/notbuilt.json lists every corpus file that cannot deploy yet with
the row that turns it green. An entry whose row the marker has reached is a failed line naming the file, its
construct and the marker; a missing or malformed marker, a marker naming no row, a malformed list or an entry naming
no row or a part is a FAIL by its own text, never a pass. GATE_COVERED is 1: the one list read. No engine runs.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import notbuilt, rows, sprintid

MARKER = "gate/current-sprint"
SPRINTS = "gate/sprints.json"
NOTBUILT = "conformance/notbuilt.json"


def read(path: str) -> str:
    with open(path, "rb") as f:
        return f.read().decode("utf-8", errors="replace")


def check() -> tuple[bool, list[str]]:
    """(the list was read, the failed lines): one fault stops the row before the next read."""
    try:
        text = read(MARKER)
    except OSError:
        return False, [f"{MARKER}: missing"]
    try:
        current = sprintid.read_marker(text)
        order = sprintid.load_order(read(SPRINTS))
        order.index(sprintid.canonical(current))
        entries = notbuilt.load_entries(read(NOTBUILT))
    except ValueError as e:
        return False, [str(e)]
    try:
        return True, notbuilt.expired(entries, order, current)
    except ValueError as e:
        return True, [str(e)]


def main(argv: list[str]) -> int:
    rows.start()
    read_list, failures = check()
    if read_list:
        rows.covered(1)
    return rows.finish([f"notbuilt: {line}" for line in failures])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
