"""The launcher, run for real: gate/bootstrap.py in a child process (design note §3; QA counter-examples 36, 37).

Each test copies the real bootstrap and installer into a fixture repository, so no child can reach the
shared cache or the real tools.json. Plan check case 18, cases 16 (two installs at once), C25. Runs under
the pinned 3.15.2 and the pinned 3.12.16.
"""

import ast
import os
import subprocess
import sys
import unittest

from gate.toolchain.tests import support
from gate.toolchain.tests.support import Member

FLOOR_FIX = "install Python 3.12+ (e.g. brew install python)"

# Runs the real bootstrap as __main__ in a child whose interpreter claims another version. Only the child
# is patched, so no test mutates this process's sys.
PATCHED_VERSION = """\
import runpy, sys
sys.argv = sys.argv[2:]
version = tuple(int(part) for part in sys.argv.pop(0).split("."))
sys.version_info = (*version, "final", 0)
runpy.run_path(sys.argv[0], run_name="__main__")
"""
PATCHED_MACHINE = """\
import platform, runpy, sys
platform.machine = lambda: "riscv64"
sys.argv = sys.argv[1:]
runpy.run_path(sys.argv[0], run_name="__main__")
"""


class BootstrapTest(support.ToolchainTest):
    def setUp(self) -> None:
        super().setUp()
        support.copy_gate(self.repo)
        self.bootstrap = os.path.join(self.repo, "gate", "bootstrap.py")
        self.env = {"VORSAR_CEP_TOOLS_CACHE": self.cache, "HOME": self.world, "PATH": os.defpath}
        self.logs = os.path.join(self.world, "logs")

    def child(self, *argv: str, env: dict[str, str] | None = None) -> support.Child:
        return support.run_child(
            [sys.executable, *argv], env=self.env if env is None else env, cwd=self.world, out_dir=self.logs
        )

    def run_bootstrap(self, *args: str, env: dict[str, str] | None = None) -> support.Child:
        return self.child("-I", "-B", self.bootstrap, *args, env=env)

    def under_version(self, version: str, *args: str) -> support.Child:
        return self.child("-I", "-B", "-c", PATCHED_VERSION, "x", version, self.bootstrap, *args)

    def assert_refused(self, child: support.Child, *needles: str) -> None:
        self.assertEqual(child.code, 2, child)
        self.assertEqual(child.lines[-2:], ["GATE FAILED", "exit: 2"], child)
        for needle in needles:
            self.assertIn(needle, child.stdout)

    def plant_install(self, source: str) -> None:
        with open(os.path.join(self.repo, "gate", "toolchain", "install.py"), "w", encoding="utf-8") as f:
            f.write(source)

    def marker_install(self, marker: str) -> None:
        """An install.py that leaves a marker when imported: the witness that the floor let it load."""
        self.plant_install(
            f"open({marker!r}, 'w').close()\ndef main(argv):\n    print('installer reached')\n    return 0\n"
        )

    def pin_demo(self) -> str:
        """A one-tool fixture tools.json; returns the tool's final directory."""
        archive = support.write_archive(
            os.path.join(self.world, "demo.tar.gz"), [Member("demo/bin/demo", data=b"tool\n", mode=0o755)]
        )
        support.write_tools(self.repo, [support.tool_entry(archive)])
        return os.path.join(self.cache, f"demo-1.0-{support.sha256_of(archive)[:12]}")


class Floor(BootstrapTest):
    def test_an_old_python_gets_the_fix_text_and_never_loads_the_installer(self):
        # QA counter-example 36, amended to the 3.12 floor. The 3.12.0 case below is the witness.
        marker = os.path.join(self.world, "marker")
        self.marker_install(marker)
        for version in ("3.9.6", "3.11.9"):
            with self.subTest(version=version):
                child = self.under_version(version, "install")
                self.assert_refused(child, FLOOR_FIX)
                self.assertEqual(child.lines, [FLOOR_FIX, "GATE FAILED", "exit: 2"])
                self.assertFalse(os.path.exists(marker))

    def test_the_floor_itself_passes_and_loads_the_installer(self):
        marker = os.path.join(self.world, "marker")
        self.marker_install(marker)
        child = self.under_version("3.12.0", "install")
        self.assertEqual((child.code, child.lines), (0, ["installer reached"]), child)
        self.assertTrue(os.path.exists(marker))

    def test_the_real_interpreter_passes_the_floor(self):
        child = self.run_bootstrap("bogus")
        self.assert_refused(child, "usage:")
        self.assertNotIn(FLOOR_FIX, child.stdout)

    def test_the_launcher_parses_with_python_39_grammar(self):
        # What ruff's per-file target enforces, checked here as well against the file itself.
        with open(os.path.join(support.REPO, "gate", "bootstrap.py"), encoding="utf-8") as f:
            source = f.read()
        ast.parse(source, feature_version=(3, 9))

    def test_exec_checks_the_floor_too(self):
        # The installer refuses on its own, for a caller that did not come through the launcher.
        self.pin_demo()
        script = (
            "import sys\n"
            f"sys.path.insert(0, {self.repo!r})\n"
            "from gate.toolchain import install\n"
            "sys.version_info = (3, 11, 9, 'final', 0)\n"
            "sys.exit(install.main(['exec', '--', 'x']))\n"
        )
        self.assert_refused(self.child("-I", "-B", "-c", script), FLOOR_FIX)


class Crash(BootstrapTest):
    def test_a_broken_installer_is_exit_2_not_a_traceback_and_exit_1(self):
        self.plant_install("raise RuntimeError('broken installer')\n")
        child = self.run_bootstrap("install")
        self.assert_refused(child, "RuntimeError", "broken installer")

    def test_an_installer_that_does_not_parse_is_exit_2(self):
        self.plant_install("def main(:\n")
        self.assert_refused(self.run_bootstrap("install"), "SyntaxError")


class Commands(BootstrapTest):
    def test_install_then_clean_tools_from_the_command_line(self):
        home = self.pin_demo()
        child = self.run_bootstrap("install")
        self.assertEqual((child.code, child.lines), (0, ["demo 1.0: installed"]), child)
        self.assertEqual(support.sha256_of(os.path.join(home, "bin", "demo")), support.sha256(b"tool\n"))
        child = self.run_bootstrap("clean-tools")
        self.assertEqual((child.code, child.lines), (0, [f"{self.cache}: removed"]), child)
        self.assertFalse(os.path.lexists(self.cache))

    def test_the_launcher_runs_from_any_directory(self):
        self.pin_demo()
        elsewhere = os.path.join(self.world, "elsewhere")
        os.makedirs(elsewhere)
        child = support.run_child(
            [sys.executable, "-I", "-B", self.bootstrap, "install"], env=self.env, cwd=elsewhere, out_dir=self.logs
        )
        self.assertEqual(child.code, 0, child)

    def test_an_unsupported_platform_names_the_two_supported_ones(self):
        # Plan check case 18, through the real entry.
        self.pin_demo()
        child = self.child("-I", "-B", "-c", PATCHED_MACHINE, self.bootstrap, "install")
        self.assert_refused(child, "riscv64", "darwin-arm64", "linux-amd64")

    def test_two_installs_at_once_both_succeed_and_leave_one_tree(self):
        # Plan check case 16: real processes, no order forced; the outcome holds for every interleaving.
        home = self.pin_demo()
        os.makedirs(self.logs)
        argv = [sys.executable, "-I", "-B", self.bootstrap, "install"]
        with (
            open(os.path.join(self.logs, "first"), "wb") as first,
            open(os.path.join(self.logs, "second"), "wb") as second,
        ):
            procs = [
                subprocess.Popen(
                    argv,
                    env=self.env,
                    cwd=self.world,
                    stdin=subprocess.DEVNULL,
                    stdout=log,
                    stderr=subprocess.STDOUT,
                    start_new_session=True,
                )
                for log in (first, second)
            ]
        for proc in procs:  # also when the shim's TERM stops this test (review BA1)
            self.addCleanup(proc.wait)
            self.addCleanup(support.kill_group, proc.pid)
        codes = [proc.wait(timeout=support.CHILD_CAP_S) for proc in procs]
        self.assertEqual(codes, [0, 0])
        self.assertEqual(os.listdir(self.cache), [os.path.basename(home)])
        self.assertEqual(support.sha256_of(os.path.join(home, "bin", "demo")), support.sha256(b"tool\n"))


class Isolation(BootstrapTest):
    """C25, the Python half: nothing the user's environment plants changes what the launcher does."""

    def plant(self) -> tuple[str, dict[str, str]]:
        """A PYTHONPATH holding a json.py and a sitecustomize.py that each leave a marker when imported."""
        planted = os.path.join(self.world, "planted")
        os.makedirs(planted)
        marker = os.path.join(self.world, "planted-ran")
        for name in ("json.py", "sitecustomize.py"):
            with open(os.path.join(planted, name), "w", encoding="utf-8") as f:
                f.write(f"open({marker!r}, 'w').close()\n")
        return marker, {**self.env, "PYTHONPATH": planted}

    def test_the_plant_is_armed_without_dash_capital_i(self):
        # The witness: the same plant does fire when the interpreter is not isolated.
        self.pin_demo()
        marker, env = self.plant()
        self.child("-B", self.bootstrap, "install", env=env)
        self.assertTrue(os.path.exists(marker))

    def test_under_dash_capital_i_a_planted_python_path_has_no_effect(self):
        home = self.pin_demo()
        marker, env = self.plant()
        child = self.run_bootstrap("install", env=env)
        self.assertEqual((child.code, child.lines), (0, ["demo 1.0: installed"]), child)
        self.assertFalse(os.path.exists(marker))
        self.assertTrue(os.path.isdir(home))

    def test_the_launchers_own_directory_never_shadows_the_standard_library(self):
        # Without -I, Python puts gate/ first on sys.path: a gate/json.py would replace the real one.
        self.pin_demo()
        marker = os.path.join(self.world, "shadow-ran")
        with open(os.path.join(self.repo, "gate", "json.py"), "w", encoding="utf-8") as f:
            f.write(f"open({marker!r}, 'w').close()\n")
        child = self.child(self.bootstrap, "install")
        self.assertEqual(child.code, 0, child)
        self.assertFalse(os.path.exists(marker))

    def test_no_bytecode_lands_in_the_repository_even_without_dash_capital_b(self):
        self.pin_demo()
        self.assertEqual(self.child(self.bootstrap, "install").code, 0)
        found = [d for _, dirs, _ in os.walk(self.repo) for d in dirs if d == "__pycache__"]
        self.assertEqual(found, [])


class ExecForReal(BootstrapTest):
    """Design note §3 `exec`: the process really becomes the pinned interpreter (QA counter-example 37)."""

    def pin_interpreter(self) -> tuple[str, str]:
        """A cache holding a stand-in python3.15 that records how it was started and ran.

        A script with a shebang: execv really runs it, it needs no 20 MB copy, and its bytes are hashed
        like any interpreter's. Returns (its path, the path of the record it writes).
        """
        record = os.path.join(self.world, "started")
        source = (
            f"#!{sys.executable} -IB\n"
            "import sys\n"
            f"with open({record!r}, 'w', encoding='utf-8') as f:\n"
            "    f.write('\\n'.join(sys.argv))\n"
        ).encode()
        entry = support.tool_entry(
            "unused",
            name="python",
            version="3.15.2",
            command="python3.15",
            exec_sha256={"bin/python3.15": support.sha256(source)},
            pinned="ab" * 32,
            url="file:///unused",
        )
        support.write_tools(self.repo, [entry])
        directory = os.path.join(self.cache, f"python-3.15.2-{'ab' * 6}", "bin")
        os.makedirs(directory)
        path = os.path.join(directory, "python3.15")
        with open(path, "wb") as f:
            f.write(source)
        os.chmod(path, 0o555)
        return path, record

    def test_a_good_interpreter_receives_the_gate_and_every_argument(self):
        python, record = self.pin_interpreter()
        child = self.run_bootstrap("exec", "--", "--tier", "T2", "a b")
        self.assertEqual((child.code, child.lines), (0, []), child)
        with open(record, encoding="utf-8") as f:
            started = f.read().split("\n")
        run_py = os.path.join(self.repo, "gate", "run.py")
        self.assertEqual(started, [python, "-I", "-B", "-X", "utf8", run_py, "--tier", "T2", "a b"])

    def test_one_flipped_byte_stops_the_process_before_it_becomes_the_interpreter(self):
        # QA counter-example 37; the test above is the witness that the record appears when it runs.
        python, record = self.pin_interpreter()
        os.chmod(python, 0o755)
        with open(python, "r+b") as f:
            f.seek(-1, os.SEEK_END)
            byte = f.read(1)
            f.seek(-1, os.SEEK_END)
            f.write(bytes([byte[0] ^ 1]))
        self.assert_refused(self.run_bootstrap("exec", "--", "--tier", "T2"), "run make tools")
        self.assertFalse(os.path.exists(record))

    def test_a_missing_interpreter_stops_the_process(self):
        python, record = self.pin_interpreter()
        os.unlink(python)
        self.assert_refused(self.run_bootstrap("exec", "--", "--tier", "T2"), "run make tools")
        self.assertFalse(os.path.exists(record))


if __name__ == "__main__":
    unittest.main()
