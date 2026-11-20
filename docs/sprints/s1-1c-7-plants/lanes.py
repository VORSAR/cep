"""S1.1c-7's lane assignment by finding id, and its commit projection by package and change kind (plan check §1, §4).

    python3 docs/sprints/s1-1c-7-plants/lanes.py

Reads survey-index.json beside this file (the id, group and path of each of the survey's 1,442 findings). Every one
of the sprint's 186 ledger ids (G10, G7a, G2 and G3-72) gets exactly one owner; G10-6 is fixed elsewhere (G1-1).
The commit count is one commit per package and kind, the coordinator's files counted under G0; deletion and bench-input
commits are estimated separately in the plan.
"""

import collections
import json
import os

HERE = os.path.dirname(os.path.abspath(__file__))
COORDINATOR = (
    "gate/runner/tests/test_aging.py",
    "gate/runner/tests/test_parts.py",
    "gate/runner/tests/test_sprintid.py",
    "gate/runner/tests/test_instruments_pin.py",
)
CONCURRENCY = {"G10-5", "G10-20"}  # K1c
PER_EVENT = {"G10-21"}  # K1h (R5); G7a-5 is error text only, so K1
COMMENT_ONLY = {"G7a-2", "G7a-6"}  # K6


def in_sprint(row: dict) -> bool:
    return row["group"] in ("G10", "G7a", "G2") or row["id"] == "G3-72"


def lane(row: dict) -> str:
    path = row["path"]
    if row["id"] == "G10-6":
        return "fixed elsewhere"
    if path.startswith("gate/bench/"):
        return "W0-py"
    if path.startswith(("tools/astequal/", "tools/archcheck/")):
        return "W0-go"
    if path in COORDINATOR:
        return "coordinator"
    if path.startswith("gate/runner/"):
        return "A1"
    if path.startswith("gate/"):
        return "A2"
    if path.startswith("internal/playback/"):
        return "B"
    if path.startswith("internal/runtime/"):
        return "E1"
    if os.path.dirname(path) == "" or path.startswith(("internal/partition/", "io/", "tools/")):
        return "E2"
    return "CD"


def kind(row: dict) -> str:
    if row["id"] in CONCURRENCY:
        return "K1c:" + row["id"]
    if row["id"] in PER_EVENT:
        return "K1h:" + row["id"]
    if row["id"] in COMMENT_ONLY:
        return "K6"
    if row["group"] in ("G10", "G7a") or row["id"] == "G3-72":
        return "K1"
    return "K2"


def main() -> None:
    with open(os.path.join(HERE, "survey-index.json"), encoding="utf-8") as f:
        rows = [r for r in json.load(f) if in_sprint(r)]
    lanes = collections.defaultdict(list)
    for row in rows:
        lanes[lane(row)].append(row)
    order = ["W0-py", "W0-go", "coordinator", "A1", "A2", "B", "E1", "E2", "CD", "fixed elsewhere"]
    assert set(lanes) <= set(order), set(lanes) - set(order)
    print(f"{'lane':16} G10 G7a  G2 G3 total commits")
    total = commits = 0
    for name in order:
        group = lanes[name]
        count = collections.Counter(r["group"] for r in group)
        cells = {(os.path.dirname(r["path"]) or ".", kind(r)) for r in group}
        made = 0 if name in ("fixed elsewhere", "coordinator") else len(cells)
        total += len(group)
        commits += made
        print(f"{name:16} {count['G10']:3} {count['G7a']:3} {count['G2']:3} {count['G3']:2} {len(group):5} {made:7}")
    print(f"ledger ids {total}; lane commits by package x kind {commits} (the coordinator's under G0)")


if __name__ == "__main__":
    main()
