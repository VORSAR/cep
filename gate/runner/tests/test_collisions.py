"""QB10, pure (S1.1c-3 plan check §0.19, §2 S2 "QB10", §6 [3] "BA10 and QB10"; owner 2026-11-12, S1.1c-2's plan
check, decision 5): `rows.collisions(paths)` names every way a listing of Python files under gate/ would run one
module in another's place.

`_module_name` (`gate/runner/rows/__init__.py`) turns a path into a module name by replacing "/" with ".", so
`gate/a.b/test_x.py` and `gate/a/b/test_x.py` share the name `gate.a.b.test_x`, and the report file is keyed by it:
the dotted file never runs while its path counts as passed. A regular package (a directory with `__init__.py`)
shadows a same-named module (`gate/foo/__init__.py` beside `gate/foo.py`: `import gate.foo` finds the package); a
namespace directory (no `__init__.py`) does not, because the import system takes the module first. One line per
collision, naming the paths; [] when there is none. Every case is a path listing; none reads the real tree.

Red today: the stub raises NotImplementedError. The plan's signature is `collisions(paths, *, packages)`; the W0
stub is `collisions(paths)`, the `__init__.py` paths in the listing marking the regular packages - these cases follow
the stub.
"""

import unittest

from gate.runner import rows


class Collisions(unittest.TestCase):
    def assertNamed(self, lines: list[str], *needles: str) -> None:
        """One line holds every needle."""
        self.assertTrue(any(all(n in line for n in needles) for line in lines), (needles, lines))

    def test_a_dotted_directory_is_red_naming_it(self):
        lines = rows.collisions(["gate/runner/__init__.py", "gate/a.b/test_x.py"])
        self.assertNamed(lines, "gate/a.b")

    def test_a_dotted_directory_deeper_down_is_red_naming_it(self):
        lines = rows.collisions(["gate/runner/tests/v1.2/test_x.py"])
        self.assertNamed(lines, "gate/runner/tests/v1.2")

    def test_two_paths_with_one_module_name_are_red_naming_both(self):
        lines = rows.collisions(["gate/a/b/test_x.py", "gate/a.b/test_x.py"])
        self.assertNamed(lines, "gate/a/b/test_x.py", "gate/a.b/test_x.py")

    def test_the_order_of_the_listing_changes_nothing(self):
        one = rows.collisions(["gate/a/b/test_x.py", "gate/a.b/test_x.py"])
        two = rows.collisions(["gate/a.b/test_x.py", "gate/a/b/test_x.py"])
        self.assertTrue(one)
        self.assertEqual(sorted(one), sorted(two))

    def test_a_regular_package_beside_a_same_named_module_is_red_naming_both(self):
        lines = rows.collisions(["gate/foo.py", "gate/foo/__init__.py", "gate/foo/test_y.py"])
        self.assertNamed(lines, "gate/foo.py", "gate/foo/")

    def test_a_regular_package_alone_is_green(self):
        self.assertEqual(rows.collisions(["gate/foo/__init__.py", "gate/foo/test_y.py"]), [])

    def test_a_namespace_directory_beside_a_same_named_module_is_green(self):
        # No gate/foo/__init__.py: the import system takes gate/foo.py; gate/foo/test_y.py then fails to import,
        # which the row reports as a failed module, never as a pass.
        self.assertEqual(rows.collisions(["gate/foo.py", "gate/foo/test_y.py"]), [])

    def test_todays_listing_shape_is_green(self):
        listing = [
            "gate/__init__.py",
            "gate/run.py",
            "gate/runner/__init__.py",
            "gate/runner/aging.py",
            "gate/runner/rows/__init__.py",
            "gate/runner/rows/aging.py",
            "gate/runner/tests/__init__.py",
            "gate/runner/tests/test_aging.py",
            "gate/runner/tests/test_collisions.py",
            "gate/proofs/drivers/__init__.py",
            "gate/proofs/drivers/test_aging_planted.py",
            "gate/toolchain/manifest.py",
            "gate/toolchain/tests/test_manifest.py",
        ]
        self.assertEqual(rows.collisions(listing), [])

    def test_an_empty_listing_is_green(self):
        self.assertEqual(rows.collisions([]), [])


if __name__ == "__main__":
    unittest.main()
