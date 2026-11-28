"""Every Go row's package arguments in the real registry name packages that exist in the repository (PR 1 review,
WA-1): a row naming a missing directory would hand go a pattern that matches nothing.

`X/...` needs the directory X; `.` needs a *_test.go at the repository root; any other `./path` needs the directory
and a *_test.go in it.
"""

import os
import unittest

from gate.runner import registry
from gate.runner.tests import support

GO_ROW_MODULES = (
    "gate/runner/rows/cpu.py",
    "gate/runner/rows/gotest.py",
    "gate/runner/rows/race.py",
    "gate/runner/rows/unit.py",
)


def packages(command: tuple[str, ...]) -> tuple[str, list[str]] | None:
    """(module, package arguments) of a command that runs a Go row module, else None. The package arguments are the
    non-flag arguments after the module; gotest.py's `-run <pattern>` pair is skipped."""
    hits = [i for i, arg in enumerate(command) if arg in GO_ROW_MODULES]
    if not hits:
        return None
    module = command[hits[0]]
    rest = list(command[hits[0] + 1 :])
    found = []
    i = 0
    while i < len(rest):
        if module == "gate/runner/rows/gotest.py" and rest[i] == "-run":
            i += 2
            continue
        if not rest[i].startswith("-"):
            found.append(rest[i])
        i += 1
    return module, found


def has_test_go(directory: str) -> bool:
    return os.path.isdir(directory) and any(n.endswith("_test.go") for n in sorted(os.listdir(directory)))


class GoRowPackages(unittest.TestCase):
    def test_every_go_rows_packages_resolve_in_the_repository(self):
        rows = sorted(registry.load(os.path.join(support.GATE, "registry.json")), key=lambda r: r.id)
        seen_modules = set()
        checked = 0
        for row in rows:
            parsed = packages(tuple(row.command))
            if parsed is None:
                continue
            module, pkgs = parsed
            seen_modules.add(module)
            # Positive witness: every Go row names at least one package to check.
            self.assertTrue(pkgs, f"{row.id}: no package arguments after {module}")
            for pkg in pkgs:
                with self.subTest(row=row.id, package=pkg):
                    checked += 1
                    if pkg.endswith("/..."):
                        path = os.path.join(support.REPO, pkg[: -len("/...")])
                        self.assertTrue(os.path.isdir(path), f"{row.id}: {pkg}: no directory {path}")
                    elif pkg == ".":
                        self.assertTrue(has_test_go(support.REPO), f"{row.id}: {pkg}: no *_test.go at the root")
                    else:
                        self.assertTrue(pkg.startswith("./"), f"{row.id}: {pkg}: not a ./path")
                        path = os.path.join(support.REPO, pkg[2:])
                        self.assertTrue(os.path.isdir(path), f"{row.id}: {pkg}: no directory {path}")
                        self.assertTrue(has_test_go(path), f"{row.id}: {pkg}: no *_test.go in {path}")
        # Positive witness: the check had its chance on every Go row module the brief names.
        self.assertEqual(sorted(seen_modules), sorted(GO_ROW_MODULES))
        self.assertGreater(checked, 0)


if __name__ == "__main__":
    unittest.main()
