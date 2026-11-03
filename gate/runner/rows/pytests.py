"""The pytests row: every tracked test module, each in its own process under the pinned CPython (design note §5).

Before running, it looks for test files that would silently never run: a misnamed module, a stray file under a
tests directory, an untracked module, a module in a directory without __init__.py, and a tracked module that no
row runs (break-it QB1). Each one fails, naming the file. GATE_COVERED counts the modules that passed.

Only two rows run test modules: this one, over its own scope, and the proofs row, over the modules directly in
gate/proofs/drivers/ (gate/registry.json). A tracked test module in neither is a test nobody runs.
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


PROOF_DRIVERS = ("gate", "proofs", "drivers")


def run_by_proofs_row(path: str) -> bool:
    """The proofs row runs the test modules directly in gate/proofs/drivers/, and nothing deeper."""
    parts = path.split("/")
    return len(parts) == len(PROOF_DRIVERS) + 1 and tuple(parts[:-1]) == PROOF_DRIVERS


def misplaced(tracked: set[str], runs: set[str]) -> list[str]:
    """Every Python file under gate/, tracked or not, that could hold tests nobody runs.

    `runs` is this row's own scope: the modules it is about to run.
    """
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
            elif name.startswith("test_") and path not in runs and not run_by_proofs_row(path):
                found.append(
                    f"{path}: no row runs it: move it into the pytests scope or directly into gate/proofs/drivers/"
                )
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


def row_scope(row_dir: str, paths: list[str]) -> set[str]:
    """The row's whole scope as the runner expanded it ($GATE_ROW_DIR/files.nul), not the list handed to this
    command: a command narrowed by mistake must still be caught by its count, not mistaken for a stray module."""
    scope = os.path.join(row_dir, "files.nul")
    return {rows.show(path) for path in rows.read_list(scope)} if os.path.exists(scope) else set(paths)


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    paths = [rows.show(path) for path in rows.read_list(argv[1])]
    failures = misplaced(tracked_under_gate(row_dir), row_scope(row_dir, paths)) + missing_init(paths)
    passed, problems = rows.run_modules(PYTHON, paths, row_dir)
    rows.covered(passed)
    return rows.finish(sorted(failures + problems))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
