"""A1-3's K2e checks (design-a1a G2-18, -23, -26, -37, -40, -43; design-a1b G2-30, -31, -34, -48), run against one
tree. Usage, with the pinned interpreter: python3.15 -I -B a1_3_checks.py <tree root>

Each check prints `<id>: <measure>: red|green`; the last line is `checks: red <n>, green <m>`. A check's commands are
the design notes' own, run from <tree root>; G2-43's reads the comment block directly above the access (its one
deviation: the design's single comment line is 157 characters, past ruff's E501, so it is wrapped onto two lines).
Exit 0 always on a completed run; the words are the result.
"""

import os
import re
import subprocess
import sys
import tempfile

ROOT = os.path.abspath(sys.argv[1])
PY = "python3.15"
GUARDED = (
    ("G2-18", "test_allowlists"),
    ("G2-23", "test_deps"),
    ("G2-26", "test_registry"),
    ("G2-30", "test_sprintid_parts"),
    ("G2-31", "test_suppressions"),
    ("G2-34", "test_verdict"),
)
G2_37 = """import unittest; from unittest import mock; from gate.runner.tests import support; from gate.runner.tests import test_breakit_s11c1 as m
with mock.patch.object(support, "run_tool", side_effect=AssertionError("a git world was built")):
    r = unittest.TextTestRunner().run(unittest.defaultTestLoader.loadTestsFromTestCase(m.DocLine))
raise SystemExit(0 if r.wasSuccessful() and r.testsRun == 2 else 1)"""
G2_40 = """
import ast, sys
bad = []
for path, names in (("gate/runner/tests/test_fix_s11c4.py", {"test_entries_of_marks_the_scan_incomplete"}),
                    ("gate/runner/tests/test_breakit2_s11c4.py", {"test_bk2_3_the_entries_read_never_hold_every_member_in_memory_at_once"})):
    tree = ast.parse(open(path, encoding="utf-8").read())
    for fn in ast.walk(tree):
        if isinstance(fn, ast.FunctionDef) and fn.name in names:
            bad += [f"{path}:{n.lineno}: .{n.attr}" for n in ast.walk(fn) if isinstance(n, ast.Attribute) and n.attr in ("cache", "zip", "members")]
print(" ".join(bad)); sys.exit(1 if bad else 0)"""
G2_43 = """
import ast, sys
path = "gate/runner/tests/test_parity_stop.py"; src = open(path, encoding="utf-8").read(); lines = src.splitlines()
def above(lineno):
    i, block = lineno - 2, []
    while i >= 0 and lines[i].lstrip().startswith("#"):
        block.append(lines[i]); i -= 1
    return block
bad = [n.lineno for n in ast.walk(ast.parse(src)) if isinstance(n, ast.Attribute) and n.attr.startswith("_") and not n.attr.startswith("__") and not (isinstance(n.value, ast.Name) and n.value.id == "self") and not any("# private seam:" in line for line in above(n.lineno))]
print(bad); sys.exit(1 if bad else 0)"""
G2_48 = """
import os, signal, sys, unittest
sys.path.insert(0, os.getcwd())
sent, real = [], os.kill
def kill(pid, sig):
    if sig == signal.SIGKILL:
        sent.append(pid)
    return real(pid, sig)
os.kill = kill
suite = unittest.TestLoader().loadTestsFromName('gate.runner.tests.test_stop_children')
result = unittest.TextTestRunner(verbosity=0).run(suite)
print('passed' if result.wasSuccessful() and result.testsRun == 3 else 'failed', 'sigkills', len(sent))
"""


def run(*argv: str) -> tuple[int, str]:
    with tempfile.TemporaryFile() as out:
        code = subprocess.run(
            argv, cwd=ROOT, stdin=subprocess.DEVNULL, stdout=out, stderr=subprocess.STDOUT, timeout=900
        ).returncode
        out.seek(0)
        return code, out.read().decode("utf-8", errors="replace")


results: list[str] = []


def judge(fid: str, measure: str, green: bool) -> None:
    word = "green" if green else "red"
    results.append(word)
    print(f"{fid}: {measure}: {word}", flush=True)


for fid, module in GUARDED:
    _, out = run(PY, "-B", "-m", f"gate.runner.tests.{module}")
    found = re.findall(r"^Ran ([0-9]+) tests? in ", out, re.M)
    direct = found[-1] if found else "none"
    _, loader = run(
        PY, "-I", "-B", "-c",
        "import sys, unittest; sys.path.insert(0, '.'); "
        "print(unittest.TestLoader().loadTestsFromName(sys.argv[1]).countTestCases())",
        f"gate.runner.tests.{module}",
    )
    loader = loader.strip().splitlines()[-1] if loader.strip() else "none"
    judge(fid, f"{module} direct {direct}, loader {loader}", direct == loader)

code, _ = run(PY, "-B", "-c", G2_37)
judge("G2-37", f"DocLine with support.run_tool refused: exit {code}", code == 0)
code, out = run(PY, "-I", "-B", "-c", G2_40)
judge("G2-40", f"private Copy attributes: [{out.strip()}] exit {code}", code == 0)
code, out = run(PY, "-I", "-B", "-c", G2_43)
judge("G2-43", f"private accesses without a seam comment: {out.strip()} exit {code}", code == 0)
code, out = run(PY, "-I", "-B", "-c", G2_48)
line = out.strip().splitlines()[-1] if out.strip() else ""
judge("G2-48", line, line == "passed sigkills 0")
print(f"checks: red {results.count('red')}, green {results.count('green')}")
