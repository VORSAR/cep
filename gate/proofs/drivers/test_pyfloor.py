"""The pyfloor proof (design note §8; QA DQ7; re-check L3): code the 3.12 floor cannot run is red under 3.12.16.

Planted, one run each, into the copied gate/toolchain/install.py: an `except ValueError, TypeError:` (3.14 syntax),
and a call to `copy.replace` (a 3.13 API) on a path its tests run. The fixture registry holds only parity and
pyfloor: ruff would flag the plant too. As the witness that only the interpreter differs, the same planted tests pass
under the pinned 3.15.2.
"""

import os

from gate.proofs.drivers import support
from gate.runner import env as runner_env
from gate.toolchain import manifest

INSTALL = "gate/toolchain/install.py"
MODULE = "gate.toolchain.tests.test_install"
SYNTAX = "\n\ndef _floor_plant() -> None:\n    try:\n        pass\n    except ValueError, TypeError:\n        pass\n"
IMPORTS = ("import contextlib\n", "import contextlib\nimport copy\nimport types\n")
API = (
    '    if name.startswith("/"):\n        return "an absolute path"\n',
    "    name = copy.replace(types.SimpleNamespace(name=name), name=name).name\n"
    '    if name.startswith("/"):\n        return "an absolute path"\n',
)


class Pyfloor(support.DriverTest):
    def setUp(self):
        super().setUp()
        self.world.copy_gate(tests=True)
        self.world.commit()

    def plant(self, *edits: tuple[str, str], append: str = "") -> None:
        path = os.path.join(self.world.repo, INSTALL)
        with open(path, encoding="utf-8") as f:
            text = f.read()
        for old, new in edits:
            self.assertEqual(text.count(old), 1, f"the plant's anchor moved in {INSTALL}: update this driver")
            text = text.replace(old, new)
        self.world.track(INSTALL, text + append)

    def assertRedUnder312Only(self, why: str) -> None:
        run = self.world.run(registry=self.world.registry(support.registry_row("pyfloor")))
        self.assertParityPassed(run, "lint")
        failed = [line for line in run.row_stdout("lint", "pyfloor").split("\n") if line.startswith("failed: ")]
        prefix = f"failed: gate/toolchain/tests/test_install.py: pyshim: {MODULE}: "
        self.assertTrue([line for line in failed if line.startswith(prefix)], failed)
        self.assertRed(run, "lint", "pyfloor", f"exit 1: {failed[0].removeprefix('failed: ')}")  # the first one
        self.assertIn(why, run.row_file("lint", "pyfloor", "py", f"{MODULE}.stderr"))
        # The witness: the same planted tree passes under the pinned 3.15.2, so the floor alone made it red.
        cache = support.real_cache()
        env = runner_env.clean_env(self.world.env, manifest.path_dirs(cache, list(support.tools())))
        shim = os.path.join(self.world.repo, "gate", "runner", "pyshim.py")
        report = os.path.join(self.world.root, "w.report")
        argv = [support.pinned_python(), "-I", "-B", "-X", "dev", "-W", "error", shim, self.world.repo, MODULE, report]
        out_path, err_path = os.path.join(self.world.root, "w.stdout"), os.path.join(self.world.root, "w.stderr")
        code = support.run_process(argv, cwd=self.world.repo, env=env, out=out_path, err=err_path, cap_s=600)
        self.assertEqual(code, 0, support.read_text(err_path)[-2000:])
        self.assertIn(f"pyshim: {MODULE}: passed ", support.read_text(report))

    def test_3_14_syntax_is_red(self):
        self.plant(append=SYNTAX)
        self.assertRedUnder312Only("SyntaxError: multiple exception types must be parenthesized")

    def test_a_3_13_api_on_a_tested_path_is_red(self):
        self.plant(IMPORTS, API)
        self.assertRedUnder312Only("AttributeError: module 'copy' has no attribute 'replace'")
