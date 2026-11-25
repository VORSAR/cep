"""Write cd2-C7.json: design-CD2 §3 G2-133's --expect entries, old_rev the lane base, new_rev C7.

usage: python3.15 -I make-c7-batch.py <new_rev> <out.json>
"""

import json
import sys

BASE = "cc2c2c1aa749c66ec7ac58c636d33747a4cf8838"
TEST = "TestBreakitGuardSnapshotUnderConcurrentWriters"

FIND = "\ts.last = &Snapshot{versions: versions}\n\treturn s.last\n}\n"
REPLACE = (
    "\ts.last = &Snapshot{versions: versions}\n"
    "\tif plantOld == nil {\n"
    "\t\tplantOld = s.last\n"
    "\t}\n"
    "\tif plantN++; plantN%2 == 0 {\n"
    "\t\treturn plantOld\n"
    "\t}\n"
    "\treturn s.last\n"
    "}\n"
    "\n"
    "var (\n"
    "\tplantOld *Snapshot\n"
    "\tplantN   int\n"
    ")\n"
)

NEW = sys.argv[1]
assert len(NEW) == 40 and all(c in "0123456789abcdef" for c in NEW), NEW
common = {"test": TEST, "pkg": "internal/table", "timeout": "180s", "old_rev": BASE, "new_rev": NEW}
entries = [
    {
        "id": "G2-133",
        "note": "G2-133 K2a: every second fresh Set.Snapshot returns the first snapshot ever taken",
        "file": "internal/table/table.go",
        "find": FIND,
        "replace": REPLACE,
        **common,
        "expect": {"old": "green", "new": "red"},
    },
    {
        "id": "G2-133:control",
        "note": "G2-133 control: the real code",
        "control": True,
        **common,
        "expect": {"old": "green", "new": "green"},
    },
]
for e in entries:
    for k in ("find", "replace"):
        if k in e:
            assert "\\" not in e[k], (e["id"], k)
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump(entries, f, indent=1, ensure_ascii=True)
    f.write("\n")
print("wrote", sys.argv[2], len(entries), "entries")
