"""Write cd2-C8.json: design-CD2 §3's --expect entries for G2-135, G2-136, G2-138 and G2-139, old_rev the lane
base, new_rev C8.

usage: python3.15 -I make-c8-batch.py <new_rev> <out.json>
"""

import json
import sys

BASE = "cc2c2c1aa749c66ec7ac58c636d33747a4cf8838"
PKG = "internal/validate"

G2_139_PATCH = (
    "--- a/internal/validate/nulls.go\n"
    "+++ b/internal/validate/nulls.go\n"
    "@@ -67,3 +67,3 @@\n"
    " \tdefault:\n"
    '-\t\tc.report("E-NULL-002", n.Pos, "%s is optional, and `%s` is false when it is null; '
    'wrap it in coalesce(...) "+\n'
    '+\t\tc.report("E-NULL-001", n.Pos, "%s is optional, and `%s` is false when it is null; '
    'wrap it in coalesce(...) "+\n'
    ' \t\t\t"or test `is null` explicitly", strings.Join(offenders, ", "), opName(n))\n'
    "--- a/internal/validate/validate.go\n"
    "+++ b/internal/validate/validate.go\n"
    "@@ -132,3 +132,3 @@\n"
    " \t\tCode: code, Line: at.Line, Col: at.Col, Msg: fmt.Sprintf(format, args...),\n"
    '-\t\tFix: catalog[code].Fix, Lint: code == "E-NULL-002",\n'
    '+\t\tFix: catalog[code].Fix, Lint: code == "E-NULL-001" || code == "E-NULL-002",\n'
    " \t})\n"
)

NEW = sys.argv[1]
assert len(NEW) == 40 and all(c in "0123456789abcdef" for c in NEW), NEW


def entry(id_, note, test, subtest=None, control=False, new="red", **plant):
    e = {"id": id_, "note": note}
    if control:
        e["control"] = True
    else:
        e.update(plant)
    e["test"] = test
    if subtest is not None:
        e["subtest"] = subtest
    e.update({"pkg": PKG, "timeout": "180s", "old_rev": BASE, "new_rev": NEW, "expect": {"old": "green", "new": new}})
    return e


T135 = "TestBreakitANegativeDurationOverrideInABoundSlotIsRefused"
T_ONE = "TestOneFindingPerDefect"
S136 = "!= over an optional attribute in an absence: no E-NULL-002"
S138 = "a bad param default: reported once, at the declaration"
T139 = "TestALintDoesNotRefuse"
entries = [
    entry(
        "G2-135",
        "G2-135 K2a: the negative bound reported as E-DUR-002, not E-DUR-003",
        T135,
        file="internal/validate/bounds.go",
        find='c.report("E-DUR-003", d.Pos, "a bound slot of rule %s is %s"',
        replace='c.report("E-DUR-002", d.Pos, "a bound slot of rule %s is %s"',
    ),
    entry("G2-135:control", "G2-135 control: the real code", T135, control=True, new="green"),
    entry(
        "G2-136",
        "G2-136 K2b: the absence branch reports E-NULL-002 in place of E-NULL-001",
        T_ONE,
        subtest=S136,
        file="internal/validate/nulls.go",
        find='c.report("E-NULL-001", n.Pos, "comparison over the optional attribute',
        replace='c.report("E-NULL-002", n.Pos, "comparison over the optional attribute',
    ),
    entry("G2-136:control", "G2-136 control: the real code", T_ONE, subtest=S136, control=True, new="green"),
    entry(
        "G2-138",
        "G2-138 K2a: a bad param default reported at no position",
        T_ONE,
        subtest=S138,
        file="internal/validate/resolve.go",
        find='c.report("E-PARAM-001", p.Pos, "param %s is declared',
        replace='c.report("E-PARAM-001", model.Pos{}, "param %s is declared',
    ),
    entry("G2-138:control", "G2-138 control: the real code", T_ONE, subtest=S138, control=True, new="green"),
    entry(
        "G2-139", "G2-139 K2a: the lint's code becomes E-NULL-001 and the lint rule covers it", T139, patch=G2_139_PATCH
    ),
    entry("G2-139:control", "G2-139 control: the real code", T139, control=True, new="green"),
]
for e in entries:
    for k in ("find", "replace", "patch"):
        if k in e:
            assert "\\" not in e[k], (e["id"], k)
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump(entries, f, indent=1, ensure_ascii=True)
    f.write("\n")
print("wrote", sys.argv[2], len(entries), "entries")
