"""The hook launchers (gate design §12.4): standalone, any python3 starts them, and they find Python 3.12 or newer.

Held to Python 3.12: the pyfloor row runs this module under the pinned 3.12.16.
"""

import importlib.machinery
import importlib.util
import os
import sys
import tempfile
import unittest
from unittest import mock

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
LAUNCHER = os.path.join(REPO, ".githooks", "pre-commit")


def load(path, name):
    loader = importlib.machinery.SourceFileLoader(name, path)
    spec = importlib.util.spec_from_loader(name, loader)
    module = importlib.util.module_from_spec(spec)
    loader.exec_module(module)
    return module


class Launcher(unittest.TestCase):
    def setUp(self):
        self.launcher = load(LAUNCHER, "launcher")

    def test_the_pinned_path_is_the_guards(self):
        # BA9's rule: a copied computation is pinned equal to its original by a test.
        sys.path.insert(0, os.path.join(REPO, "gate"))
        self.addCleanup(sys.path.remove, os.path.join(REPO, "gate"))
        run = load(os.path.join(REPO, "gate", "run.py"), "gate_run")
        env = {"HOME": os.path.expanduser("~")}
        with mock.patch.dict(os.environ, env, clear=True):
            self.assertEqual(self.launcher.pinned_python(REPO, env), run.pinned_python())

    def test_the_cached_pinned_interpreter_comes_first(self):
        with tempfile.TemporaryDirectory() as d:
            fake = os.path.join(d, "python3.15")
            with open(fake, "w") as f:
                f.write("")
            os.chmod(fake, 0o755)
            self.launcher.pinned_python = lambda repo, environ: fake
            self.assertEqual(self.launcher.interpreter(REPO, {}, "/usr/bin/python3", (3, 9, 6)), fake)

    def test_without_the_pinned_one_a_new_enough_python_is_used(self):
        self.launcher.pinned_python = lambda repo, environ: "/nonexistent/python3.15"
        self.assertEqual(self.launcher.interpreter(REPO, {}, "/x/python3", (3, 12, 0)), "/x/python3")

    def test_without_either_there_is_none(self):
        # QA N36: the hook then refuses, naming make tools.
        self.launcher.pinned_python = lambda repo, environ: "/nonexistent/python3.15"
        self.assertEqual(self.launcher.interpreter(REPO, {}, "/usr/bin/python3", (3, 9, 6)), "")

    def test_each_hook_name_has_its_script(self):
        self.assertEqual(
            self.launcher.SCRIPTS,
            {"pre-commit": "pre_commit.py", "pre-merge-commit": "pre_commit.py", "pre-push": "pre_push.py"},
        )

    def test_the_three_launchers_are_one_file(self):
        names = ("pre-commit", "pre-merge-commit", "pre-push")
        targets = {os.path.realpath(os.path.join(REPO, ".githooks", n)) for n in names}
        self.assertEqual(targets, {os.path.realpath(LAUNCHER)})


if __name__ == "__main__":
    unittest.main()
