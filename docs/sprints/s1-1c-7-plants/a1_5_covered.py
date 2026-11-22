"""G2-49's K2-del proof (design-a1b G2-49 section 4; DQ-S3): the gate production lines the test executes, by stdlib
trace, at two trees.

    python3.15 -I -B a1_5_covered.py <old tree> <new tree>

Runs the design's trace command twice in each tree (the test id below, from the tree as the working directory) and
prints each run's `passed <n> <sha256>` line, then two checks:
- equal: all four line sets are identical (the design's form);
- subset (DQ-S3): the old test's covered lines (the union of its two runs, the deleted assertion included) are a
  subset of the intersection of the remaining test's two runs.
"""

import hashlib
import json
import os
import subprocess
import sys
import tempfile

TEST = "gate.runner.tests.test_suppressions.Enabled.test_enabled_equals_the_config_both_ways"
TRACE = """
import json, os, sys, trace, unittest
root = os.getcwd(); sys.path.insert(0, root)
suite = unittest.TestLoader().loadTestsFromName(sys.argv[1])
tracer = trace.Trace(count=1, trace=0)
result = tracer.runfunc(unittest.TextTestRunner(verbosity=0).run, suite)
gate, tests = os.path.join(root, 'gate') + os.sep, os.path.join(root, 'gate', 'runner', 'tests') + os.sep
lines = sorted({(os.path.relpath(f, root), n) for (f, n) in tracer.results().counts if f.startswith(gate) and not f.startswith(tests)})
with open(sys.argv[2], 'w', encoding='utf-8') as f:
    json.dump({'passed': result.wasSuccessful() and result.testsRun == 1, 'lines': lines}, f)
"""


def covered(tree: str) -> tuple[bool, frozenset[tuple[str, int]]]:
    with tempfile.TemporaryDirectory() as tmp:
        out = os.path.join(tmp, "lines.json")
        with open(os.path.join(tmp, "stdout"), "wb") as log:
            subprocess.run(["python3.15", "-I", "-B", "-c", TRACE, TEST, out], cwd=tree, stdin=subprocess.DEVNULL,
                           stdout=log, stderr=subprocess.STDOUT, timeout=600, check=True)  # fmt: skip  # one call
        with open(out, encoding="utf-8") as f:
            data = json.load(f)
    return data["passed"], frozenset((path, n) for path, n in data["lines"])


def show(name: str, passed: bool, lines: frozenset[tuple[str, int]]) -> None:
    digest = hashlib.sha256(repr(sorted(lines)).encode()).hexdigest()
    print(f"{name}: {'passed' if passed else 'failed'} {len(lines)} {digest}")


runs = {}
for side, tree in (("old", sys.argv[1]), ("new", sys.argv[2])):
    for i in (1, 2):
        runs[f"{side}{i}"] = covered(os.path.abspath(tree))
        show(f"{side}{i}", *runs[f"{side}{i}"])
sets = [lines for _, lines in runs.values()]
every_passed = all(passed for passed, _ in runs.values())
equal = all(s == sets[0] for s in sets)
old_union = runs["old1"][1] | runs["old2"][1]
new_meet = runs["new1"][1] & runs["new2"][1]
missing = sorted(old_union - new_meet)
print(f"every run passed: {every_passed}")
print(f"equal: {equal}")
print(f"subset: {not missing} ({len(old_union)} old lines, {len(new_meet)} in both new runs, missing {missing})")
