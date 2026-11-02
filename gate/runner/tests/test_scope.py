"""expected_scope: the three kinds, expanded live (design note §4; QA DQ3, DQ8, B1)."""

import os

from gate.runner import proc, scope
from gate.runner.tests import support


class Base(support.GateTest):
    def expand(self, *spec, env=None):
        row_dir = os.path.join(self.root, "rowdir")
        os.makedirs(row_dir, exist_ok=True)
        return scope.expand(spec, group=proc.Group(), cwd=self.repo, env=env or support.go_env(), row_dir=row_dir)


class Git(Base):
    GO = ":(glob)**/*.go"
    TESTS = (":(glob)gate/**/test_*.py", ":(glob,exclude)gate/proofs/**")
    PY = ":(glob)gate/**/*.py"

    def setUp(self):
        super().setUp()
        for rel in ("top.go", "a/nested.go", "a/b/deep.go", "gate/test_top.py", "gate/x/test_n.py", "gate/x/mod.py",
                    "gate/proofs/drivers/test_d.py", "gate/proofs/planted.go"):  # fmt: skip
            self.track(rel, "x\n")
        self.write("untracked.go", "x\n")

    def test_glob_matches_top_level_and_nested_files(self):
        got = self.expand("git", self.GO)
        self.assertEqual(got.items, (b"a/b/deep.go", b"a/nested.go", b"gate/proofs/planted.go", b"top.go"))
        self.assertEqual(got.count, 4)

    def test_an_exclusion_is_a_separate_pathspec_and_it_holds(self):
        got = self.expand("git", *self.TESTS)
        self.assertEqual(got.items, (b"gate/test_top.py", b"gate/x/test_n.py"))

    def test_the_gate_python_pathspec(self):
        got = self.expand("git", self.PY)
        self.assertEqual(
            got.items, (b"gate/proofs/drivers/test_d.py", b"gate/test_top.py", b"gate/x/mod.py", b"gate/x/test_n.py")
        )

    def test_the_runner_adds_nothing_to_a_pathspec(self):
        # A plain pathspec's `*` crosses `/`; a runner that added :(glob) would drop a/b/deep.go.
        got = self.expand("git", "a/*.go")
        self.assertEqual(got.items, (b"a/b/deep.go", b"a/nested.go"))

    def test_odd_file_names_survive(self):
        self.track("sp ace.go", "x\n")
        self.track("nön.go", "x\n")
        got = self.expand("git", self.GO)
        self.assertIn(b"sp ace.go", got.items)
        self.assertIn("nön.go".encode(), got.items)

    def test_zero_files_is_a_scope_error(self):
        with self.assertRaises(scope.ScopeError) as caught:
            self.expand("git", "*.nothing")
        self.assertIn("zero", str(caught.exception))

    def test_a_git_failure_is_a_scope_error_on_one_line(self):
        with self.assertRaises(scope.ScopeError) as caught:
            self.expand("git", ":(badmagic)x")
        self.assertIn("exit 128", str(caught.exception))
        self.assertNotIn("\n", str(caught.exception))


class Go(Base):
    def module(self, **packages):
        self.track("go.mod", "module ex.com/m\n\ngo 1.27.0\n")
        for rel, text in packages.items():
            self.track(rel.replace("__", "/") + ".go", text)

    def test_real_packages_only(self):
        # design-QA counter-example 3: `.`, ./rule/... and ./io/... match nothing here.
        self.module(internal__event__e="package event\n")
        got = self.expand("go", "./internal/...", ".", "./rule/...", "./io/...")
        self.assertEqual((got.items, got.count), ((b"ex.com/m/internal/event",), 1))
        self.track("rule/x/r.go", "package x\n")
        self.assertEqual(self.expand("go", "./internal/...", ".", "./rule/...", "./io/...").count, 2)

    def test_a_package_with_a_build_error_is_kept(self):
        self.module(good__g="package good\n", broken__b='package broken\n\nvar x int = "s"\n')
        self.assertEqual(self.expand("go", "./...").count, 2)

    def test_a_syntax_error_is_kept(self):
        self.module(good__g="package good\n", broken__b="package broken\n\nfunc {\n")
        self.assertEqual(self.expand("go", "./...").count, 2)

    def test_an_existing_directory_with_no_go_files_adds_nothing(self):
        self.module(good__g="package good\n")
        self.track("rule/readme.txt", "x\n")
        self.assertEqual(self.expand("go", "./...", "./rule/...").count, 1)

    def test_zero_packages_is_a_scope_error(self):
        self.module()
        with self.assertRaises(scope.ScopeError) as caught:
            self.expand("go", "./rule/...")
        self.assertIn("zero", str(caught.exception))

    def test_go_list_failure_is_a_scope_error(self):
        # No go.mod at all, so `go list` itself fails, or reports only unmatched patterns.
        with self.assertRaises(scope.ScopeError):
            self.expand("go", "./...")


class VetUnits(Base):
    def test_the_formula_on_every_shape(self):
        # By hand from design note §4: notests 1, intests 1, xtests 2, both 2, onlyx 1 = 7.
        self.track("go.mod", "module ex.com/m\n\ngo 1.27.0\n")
        self.track("notests/a.go", "package notests\n")
        self.track("intests/a.go", "package intests\n")
        self.track("intests/a_test.go", "package intests\n")
        self.track("xtests/a.go", "package xtests\n")
        self.track("xtests/a_test.go", "package xtests_test\n")
        self.track("both/a.go", "package both\n")
        self.track("both/a_test.go", "package both\n")
        self.track("both/b_test.go", "package both_test\n")
        self.track("onlyx/a_test.go", "package onlyx_test\n")
        got = self.expand("go-vet-units", "./...")
        self.assertEqual(got.count, 7)
        self.assertEqual(len(got.items), 5)

    def test_an_unknown_kind_is_a_scope_error(self):
        with self.assertRaises(scope.ScopeError):
            self.expand("svn", "x")
