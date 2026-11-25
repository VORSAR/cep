"""Write cd2-C4.json: the --expect entries of design-CD2 §3 (G2-85, G2-86, G2-88), old_rev the lane base, new_rev C4.

usage: python3.15 -I make-c4-batch.py <new_rev> <out.json>
"""

import json
import sys

BASE = "cc2c2c1aa749c66ec7ac58c636d33747a4cf8838"
PKG = "internal/plan"
TIMEOUT = "180s"

G2_85_FIND = "\ttimeAttr := event.ArrivalTime\n"
G2_85_REPLACE = (
    "\ttimeAttr := event.ArrivalTime\n"
    "\tif s.Time.Kind == model.TimeNone {\n"
    "\t\tfor i, a := range s.Attrs {\n"
    "\t\t\tif a.Type == event.TTimestamp {\n"
    "\t\t\t\ttimeAttr = i\n"
    "\t\t\t}\n"
    "\t\t}\n"
    "\t}\n"
)

G2_86_A_FIND = (
    "\tif timeIdx < 0 {\n"
    '\t\treturn GivenList{}, fmt.Errorf("stream %s has no time attribute a block can give", g.Name)\n'
    "\t}\n"
    "\tgl := GivenList{Stream: sid}\n"
)
G2_86_A_REPLACE = (
    "\tgl := GivenList{Stream: sid}\n"
    "\tfor _, rec := range g.Records {\n"
    "\t\tif _, err := values(st.Attrs, rec, timeIdx); err != nil {\n"
    "\t\t\treturn GivenList{}, err\n"
    "\t\t}\n"
    "\t}\n"
    "\tif timeIdx < 0 {\n"
    '\t\treturn GivenList{}, fmt.Errorf("stream %s has no time attribute a block can give", g.Name)\n'
    "\t}\n"
)

G2_86_B_FIND = (
    "\tfor _, f := range rec.Fields {\n"
    "\t\tidx := fieldIndex(attrs, f.Name)\n"
    "\t\tif idx < 0 {\n"
    '\t\t\treturn nil, fmt.Errorf("no attribute %s", f.Name)\n'
    "\t\t}\n"
    "\t\tv, err := value(f.Value, attrs[idx].Type)\n"
    "\t\tif err != nil {\n"
    '\t\t\treturn nil, fmt.Errorf("%s: %w", f.Name, err)\n'
    "\t\t}\n"
    "\t\tout[idx] = v\n"
    "\t}\n"
    "\tif timeIdx >= 0 && out[timeIdx].Null {\n"
    '\t\treturn nil, fmt.Errorf("the time attribute %s is null", attrs[timeIdx].Name)\n'
    "\t}\n"
    "\treturn out, nil\n"
)
G2_86_B_REPLACE = (
    "\tvar first error\n"
    "\tfor _, f := range rec.Fields {\n"
    "\t\tidx := fieldIndex(attrs, f.Name)\n"
    "\t\tif idx < 0 {\n"
    '\t\t\treturn nil, fmt.Errorf("no attribute %s", f.Name)\n'
    "\t\t}\n"
    "\t\tv, err := value(f.Value, attrs[idx].Type)\n"
    "\t\tif err != nil {\n"
    "\t\t\tif first == nil {\n"
    '\t\t\t\tfirst = fmt.Errorf("%s: %w", f.Name, err)\n'
    "\t\t\t}\n"
    "\t\t\tcontinue\n"
    "\t\t}\n"
    "\t\tout[idx] = v\n"
    "\t}\n"
    "\tif timeIdx >= 0 && out[timeIdx].Null {\n"
    '\t\treturn nil, fmt.Errorf("the time attribute %s is null", attrs[timeIdx].Name)\n'
    "\t}\n"
    "\tif first != nil {\n"
    "\t\treturn nil, first\n"
    "\t}\n"
    "\treturn out, nil\n"
)

G2_88_FIND = '\t\t\treturn nil, fmt.Errorf("plan: rule %s: %w", app.Rules[i].Name, err)\n'
G2_88_REPLACE = '\t\t\treturn p, fmt.Errorf("plan: rule %s: %w", app.Rules[i].Name, err)\n'


def entry(id_, note, test, find=None, replace=None, file=None, control=False, old="green", new="red"):
    e = {"id": id_, "note": note}
    if control:
        e["control"] = True
    else:
        e.update({"file": file, "find": find, "replace": replace})
    e.update(
        {
            "test": test,
            "pkg": PKG,
            "timeout": TIMEOUT,
            "old_rev": BASE,
            "new_rev": NEW,
            "expect": {"old": old, "new": new},
        }
    )
    return e


NEW = sys.argv[1]
assert len(NEW) == 40 and all(c in "0123456789abcdef" for c in NEW), NEW
plant85 = dict(file="internal/plan/plan.go", find=G2_85_FIND, replace=G2_85_REPLACE)
plant86a = dict(file="internal/plan/expect.go", find=G2_86_A_FIND, replace=G2_86_A_REPLACE)
plant86b = dict(file="internal/plan/expect.go", find=G2_86_B_FIND, replace=G2_86_B_REPLACE)
plant88 = dict(file="internal/plan/plan.go", find=G2_88_FIND, replace=G2_88_REPLACE)
entries = [
    entry(
        "G2-85:07",
        "G2-85 K2a: a derived stream's TimeAttr becomes its last timestamp attribute",
        "TestBuildExample07",
        **plant85,
    ),
    entry("G2-85:09", "G2-85 K2a: the same plant", "TestBuildExample09", **plant85),
    entry("G2-85:clauses", "G2-85 K2a: the same plant", "TestBuildPartitionedAndTimeClauses", **plant85),
    entry("G2-85:control", "G2-85 control: the real code", "TestBuildExample07", control=True, new="green"),
    entry(
        "G2-86:time-before-records",
        "G2-86 K2a: the records are checked before the missing time attribute",
        "TestBuildNamesABlocksFirstFault",
        **plant86a,
    ),
    entry(
        "G2-86:fields-before-null-time",
        "G2-86 K2a: a null time is reported before a field fault",
        "TestBuildNamesABlocksFirstFault",
        **plant86b,
    ),
    entry(
        "G2-86:control", "G2-86 control: the real code", "TestBuildNamesABlocksFirstFault", control=True, new="green"
    ),
    entry("G2-88", "G2-88 K2a: Build returns the partial plan with ErrNotBuilt", "TestBuildRefusesNotBuilt", **plant88),
    entry("G2-88:control", "G2-88 control: the real code", "TestBuildRefusesNotBuilt", control=True, new="green"),
]
for e in entries:
    for k in ("find", "replace"):
        if k in e:
            assert "\\" not in e[k], (e["id"], k)  # no backslash reaches a plant; tabs and newlines are real bytes
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump(entries, f, indent=1, ensure_ascii=True)
    f.write("\n")
print("wrote", sys.argv[2], len(entries), "entries")
