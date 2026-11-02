"""gate/run.py's guard (design note §3; plan check S0, C9; design-QA counter-example 36).

Runs under the pinned 3.15.2 in `pytests` and under the pinned 3.12.16 in `pyfloor`, so it holds to Python 3.12 and
imports nothing from gate/. Both interpreters are named explicitly, whichever one runs this file.
"""

import faulthandler
import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
import unittest

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
RUN = os.path.join(REPO, "gate", "run.py")
PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
FLAGS = ("-I", "-B", "-X", "utf8")
CEILING_S = 240


def pinned(name: str) -> str:
    with open(os.path.join(REPO, "tools.json"), encoding="utf-8") as f:
        tool = next(t for t in json.load(f)["tools"] if t["name"] == name)
    plat = PLATFORMS[(sys.platform, platform.machine())]
    cache = os.environ.get("VORSAR_CEP_TOOLS_CACHE") or os.path.expanduser("~/.cache/vorsar/cep/tools")
    home = os.path.join(cache, f"{tool['name']}-{tool['version']}-{tool['platforms'][plat]['archive_sha256'][:12]}")
    return os.path.join(home, tool["path_dir"], tool["command"])


def run(argv: list[str], *, env: dict[str, str] | None = None, cwd: str = REPO) -> tuple[int, str]:
    """Run with output in a file, never a pipe; the pinned tools are read-only, so no pyc can land there."""
    with tempfile.TemporaryFile() as out:
        code = subprocess.run(
            argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=subprocess.STDOUT
        ).returncode
        out.seek(0)
        return code, out.read().decode("utf-8", errors="replace")


class Guard(unittest.TestCase):
    def setUp(self) -> None:
        faulthandler.dump_traceback_later(CEILING_S, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)

    def assertRefused(self, code: int, output: str) -> None:
        self.assertEqual(code, 2, output)
        self.assertEqual(output.splitlines()[-2:], ["GATE FAILED", "exit: 2"], output)
        self.assertIn("make tools", output)
        self.assertNotIn("usage:", output)

    def test_another_interpreter_is_refused(self) -> None:
        self.assertRefused(*run([pinned("python-floor"), *FLAGS, RUN, "--tier", "T2"]))

    def test_the_pinned_interpreter_reaches_the_runner(self) -> None:
        # The positive control for the refusals above: the same command line gets past the guard.
        code, output = run([pinned("python"), *FLAGS, RUN, "--no-such-flag"])
        self.assertEqual(code, 2, output)
        self.assertIn("usage: gate/run.py", output)
        self.assertNotIn("make tools", output)
        self.assertEqual(output.splitlines()[-2:], ["GATE FAILED", "exit: 2"])

    def test_a_cache_override_moves_the_pinned_path(self) -> None:
        with tempfile.TemporaryDirectory() as cache:
            env = dict(os.environ, VORSAR_CEP_TOOLS_CACHE=cache)
            self.assertRefused(*run([pinned("python"), *FLAGS, RUN, "--tier", "T2"], env=env))

    def test_each_interpreter_flag_is_required(self) -> None:
        for flags in (("-B", "-X", "utf8"), ("-I", "-X", "utf8")):
            with self.subTest(flags):
                self.assertRefused(*run([pinned("python"), *flags, RUN, "--tier", "T2"]))


class Copy(unittest.TestCase):
    """A copy of run.py in a world the test owns: its REPO is the copy's, so nothing touches the real gate."""

    def setUp(self) -> None:
        faulthandler.dump_traceback_later(CEILING_S, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = tmp.name
        os.makedirs(os.path.join(self.repo, "gate", "runner"))
        shutil.copy(RUN, os.path.join(self.repo, "gate", "run.py"))
        shutil.copy(os.path.join(REPO, "tools.json"), self.repo)
        for rel in ("gate/__init__.py", "gate/runner/__init__.py"):
            with open(os.path.join(self.repo, rel), "w", encoding="utf-8"):
                pass

    def stub(self, text: str) -> None:
        with open(os.path.join(self.repo, "gate", "runner", "main.py"), "w", encoding="utf-8") as f:
            f.write(text)

    def run_copy(self) -> tuple[int, str]:
        # From an unrelated directory: sys.path is run.py's own doing, never the current directory's.
        return run(
            [pinned("python"), *FLAGS, os.path.join(self.repo, "gate", "run.py"), "a", "b"], cwd=tempfile.gettempdir()
        )

    def test_the_runner_gets_the_arguments_and_decides_the_exit_code(self) -> None:
        self.stub("def main(argv):\n    print(argv)\n    print('GATE PASSED')\n    print('exit: 0')\n    return 0\n")
        code, output = self.run_copy()
        self.assertEqual(code, 0, output)
        self.assertIn("['a', 'b']", output)

    def test_the_runner_failing_is_exit_1(self) -> None:
        self.stub("def main(argv):\n    print('GATE FAILED')\n    print('exit: 1')\n    return 1\n")
        self.assertEqual(self.run_copy()[0], 1)

    def test_a_runner_that_cannot_be_imported_is_exit_2_not_1(self) -> None:
        self.stub("def main(argv:\n")
        code, output = self.run_copy()
        self.assertEqual(code, 2, output)
        self.assertEqual(output.splitlines()[-2:], ["GATE FAILED", "exit: 2"])

    def test_an_exception_from_the_runner_is_exit_2(self) -> None:
        self.stub("def main(argv):\n    raise RuntimeError('planted')\n")
        code, output = self.run_copy()
        self.assertEqual(code, 2, output)
        self.assertIn("planted", output)
        self.assertEqual(output.splitlines()[-2:], ["GATE FAILED", "exit: 2"])

    def test_a_missing_manifest_is_refused(self) -> None:
        os.unlink(os.path.join(self.repo, "tools.json"))
        code, output = self.run_copy()
        self.assertEqual(code, 2, output)
        self.assertEqual(output.splitlines()[-2:], ["GATE FAILED", "exit: 2"])
        self.assertIn("make tools", output)

    def test_no_bytecode_lands_in_the_repository(self) -> None:
        self.stub("def main(argv):\n    return 0\n")
        self.run_copy()
        names = [n for _, dirs, _ in os.walk(self.repo) for n in dirs]
        self.assertNotIn("__pycache__", names)


if __name__ == "__main__":
    unittest.main()
