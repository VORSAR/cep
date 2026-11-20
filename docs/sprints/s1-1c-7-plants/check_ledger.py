"""S1.1c-7's ledger checker (plan check §7.5), version 1: the id set and the outcome words.

    python3 docs/sprints/s1-1c-7-plants/check_ledger.py

Run from the repository's top level. Checks:
- survey-index.json's ids equal the committed survey list's (docs/reviews/s1-1c-7-survey-findings.md);
- the sprint's ledger ids are exactly the 186 of G10, G7a, G2 and G3-72;
- the record's ledger table (docs/sprints/s1-1c-7-record.md, rows "| <id> | <outcome> | ...") holds each id at most
  once, only ledger ids, and only the plan's outcome words, with a reason for "rejected" and a target for "merged",
  "fixed in part" and "carried in full to S1.1c-8" or "... to S1.1c-10" (the design notes' carry outcomes).
It prints how many ids have a row so far. Later versions add §7.5's commit, proof-row and plant-path checks.
Exit 0: consistent. Exit 1: a fault, each named. Exit 2: a file is missing or unreadable.
"""

import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SURVEY = "docs/reviews/s1-1c-7-survey-findings.md"
RECORD = "docs/sprints/s1-1c-7-record.md"
CARRIED = ("carried in full to S1.1c-8", "carried in full to S1.1c-10")  # a rename only; a later non-simple change only
OUTCOMES = ("fixed", "fixed in part", *CARRIED, "rejected", "merged", "fixed elsewhere")
NEEDS_DETAIL = ("rejected", "merged", "fixed in part", *CARRIED)
ID = r"G\w+-\d+"  # any survey id, so a row naming a non-ledger id is caught


def key(i: str) -> tuple[str, int]:
    group, n = i.split("-")
    return group, int(n)


def main() -> int:
    try:
        with open(os.path.join(HERE, "survey-index.json"), encoding="utf-8") as f:
            index = json.load(f)
        survey = open(SURVEY, encoding="utf-8").read()
        record = open(RECORD, encoding="utf-8").read()
    except OSError as e:
        print(f"check_ledger: {e}")
        return 2
    faults = []
    listed = sorted(re.findall(r"^\| (G\w+-\d+) \|", survey, re.M), key=key)
    if [r["id"] for r in index] != listed:
        faults.append("survey-index.json's ids differ from the committed survey list's")
    ledger = {r["id"] for r in index if r["group"] in ("G10", "G7a", "G2") or r["id"] == "G3-72"}
    if len(ledger) != 186:
        faults.append(f"the ledger has {len(ledger)} ids, not 186")
    seen = {}
    for line in record.splitlines():
        if not re.match(rf"^\| {ID} \|", line):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) != 5:
            faults.append(f"{cells[0]}: {len(cells)} cells, not the five of | Id | Outcome | Commit | Proof | Reason or target |")
            continue
        fid, outcome, detail = cells[0], cells[1], cells[4]
        if fid in seen:
            faults.append(f"{fid}: listed twice")
        seen[fid] = outcome
        if fid not in ledger:
            faults.append(f"{fid}: not one of the sprint's ledger ids")
        if outcome not in OUTCOMES:
            faults.append(f"{fid}: outcome {outcome!r} is not one of {OUTCOMES}")
        if outcome in NEEDS_DETAIL and not detail:
            faults.append(f"{fid}: {outcome} without its reason or target")
    for fault in faults:
        print(f"check_ledger: {fault}")
    print(f"check_ledger: {len(seen)} of {len(ledger)} ledger ids have a row; {len(faults)} faults")
    return 1 if faults else 0


if __name__ == "__main__":
    sys.exit(main())
