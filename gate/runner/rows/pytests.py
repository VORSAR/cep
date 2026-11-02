"""The pytests row: every tracked test module, each in its own process under the pinned CPython (design note §5).

Before running, it looks for test files that would silently never run: a misnamed module, a stray file under a
tests directory, an untracked module, a module in a directory without __init__.py. Each one fails, naming the file.
GATE_COVERED counts the modules that passed.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

PYTHON = "python3.15"
ALLOWED_IN_TESTS = frozenset({"__init__.py", "support.py"})


def tracked_under_gate(row_dir: str) -> set[str]:
    out, err = os.path.join(row_dir, "tracked.stdout"), os.path.join(row_dir, "tracked.stderr")
    if rows.run_tool(["git", "ls-files", "-z", "--", "gate"], out, err) != 0:
        raise SystemExit(f"git ls-files failed: {rows.read_text(err)}")
    return {rows.show(path) for path in rows.read_list(out)}


def misplaced(tracked: set[str]) -> list[str]:
    """Every Python file under gate/, tracked or not, that could hold tests nobody runs."""
    found: list[str] = []
    for directory, _, names in os.walk("gate"):
        for name in names:
            path = os.path.join(directory, name)
            if not name.endswith(".py"):
                continue
            if name.endswith("_test.py"):
                found.append(f"{path}: misnamed: test modules are test_*.py")
            elif "tests" in path.split("/")[:-1] and not name.startswith("test_") and name not in ALLOWED_IN_TESTS:
                found.append(
                    f"{path}: not a test module: a tests directory holds test_*.py, __init__.py and support.py"
                )
            elif name.startswith("test_") and path not in tracked:
                found.append(f"{path}: untracked, so it never runs: git add it")
    return found


def missing_init(paths: list[str]) -> list[str]:
    """A namespace package imports without __init__.py, but such a directory breaks the layout rule."""
    found: list[str] = []
    for path in paths:
        parents = path.split("/")[:-1]
        for depth in range(1, len(parents) + 1):
            init = os.path.join(*parents[:depth], "__init__.py")
            if not os.path.exists(init):
                found.append(f"{path}: {init} is missing")
                break
    return found


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    paths = [rows.show(path) for path in rows.read_list(argv[1])]
    failures = misplaced(tracked_under_gate(row_dir)) + missing_init(paths)
    passed, problems = rows.run_modules(PYTHON, paths, row_dir)
    rows.covered(passed)
    return rows.finish(sorted(failures + problems))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
