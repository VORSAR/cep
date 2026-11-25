"""Write cd2-C9.json: design-CD2 §3 G2-137's --expect entries (K2-del), old_rev the parent commit C8 (the deleted row
runs there), new_rev C9 (H3, the stronger duplicate, runs there).

usage: python3.15 -I make-c9-batch.py <old_rev> <new_rev> <out.json>
"""

import json
import sys

OLD, NEW = sys.argv[1], sys.argv[2]
for rev in (OLD, NEW):
    assert len(rev) == 40 and all(c in "0123456789abcdef" for c in rev), rev

sides = {
    "old_test": "TestOneFindingPerDefect",
    "old_subtest": "count() in a filter: no type error",
    "new_test": "TestOneFindingHidesAnother",
    "new_subtest": "H3 count() in a filter: no type finding for its comparison",
    "pkg": "internal/validate",
    "timeout": "180s",
    "old_rev": OLD,
    "new_rev": NEW,
}
entries = [
    {
        "id": "G2-137",
        "note": "G2-137 K2-del: the aggregate-placement finding reported as E-FN-002; "
        "red on the deleted row at old, red on H3 at new",
        "file": "internal/validate/exprs.go",
        "find": 'c.report("E-FN-001", n.Pos, "%s() is an aggregate',
        "replace": 'c.report("E-FN-002", n.Pos, "%s() is an aggregate',
        **sides,
        "expect": {"old": "red", "new": "red"},
    },
    {
        "id": "G2-137:control",
        "note": "G2-137 control: the real code on each side's test",
        "control": True,
        **sides,
        "expect": {"old": "green", "new": "green"},
    },
]
for e in entries:
    for k in ("find", "replace"):
        if k in e:
            assert "\\" not in e[k], (e["id"], k)
with open(sys.argv[3], "w", encoding="utf-8") as f:
    json.dump(entries, f, indent=1, ensure_ascii=True)
    f.write("\n")
print("wrote", sys.argv[3], len(entries), "entries")
