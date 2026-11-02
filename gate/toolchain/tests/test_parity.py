"""Parity (design note §3; plan check cases 13, 15; C23, C25, C26; QA design counter-examples 4-7, 9).

Every test plants one defect in a fixture cache where only that check can fail, and asserts the exact
reason list. Fake tools are small Python scripts run by this interpreter, so their bytes are hashed and
their launch is real. The last class reads the real cache, read-only.
"""

import contextlib
import dataclasses
import hashlib
import os
import select
import shutil
import signal
import subprocess
import sys
import unittest

from gate.toolchain import manifest, parity
from gate.toolchain.tests import support

ARCHIVE_DIGEST = "ab" * 32
GOOS_GOARCH = {"darwin-arm64": "darwin/arm64", "linux-amd64": "linux/amd64"}
RECOVER = "run make clean-tools tools"


def script(body: str) -> bytes:
    return f"#!{sys.executable} -IB\n{body}\n".encode()


def prints(line: str, marker: str | None = None) -> bytes:
    """A tool that prints one line; with a marker, it also leaves a file behind when it runs."""
    touch = f"open({marker!r}, 'a').close()\n" if marker else ""
    return script(f"{touch}print({line!r})")


def tree_by_hand(files: dict[str, bytes]) -> str:
    """The design note's digest, written out for a tree of read-only executable files."""
    directories = {"/".join(path.split("/")[:i]) for path in files for i in range(1, path.count("/") + 1)}
    entries = {d: f"d {d} -\n".encode() for d in directories}
    entries |= {path: f"x {path} {support.sha256(data)}\n".encode() for path, data in files.items()}
    return hashlib.sha256(b"".join(line for _, line in sorted(entries.items()))).hexdigest()


def write_file(path: str, data: bytes, mode: int = 0o555) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if os.path.exists(path):
        os.chmod(path, 0o755)
    with open(path, "wb") as f:
        f.write(data)
    os.chmod(path, mode)


class ParityTest(support.ToolchainTest):
    def setUp(self) -> None:
        super().setUp()
        self.env = {"PATH": os.defpath}

    def home(self, name: str) -> str:
        return os.path.join(self.cache, f"{name}-1.0-{ARCHIVE_DIGEST[:12]}")

    def tool(
        self,
        name: str = "go",
        *,
        source: bytes | None = None,
        version_line: str | None = None,
        files: dict[str, bytes] | None = None,
        path_dir: str = "bin",
    ) -> manifest.Tool:
        """Put a fake tool in the cache, pinned exactly as it stands, with its bin directory on PATH."""
        version_line = version_line or f"{name} 1.0"
        rel = os.path.normpath(os.path.join(path_dir, name))
        tree = {rel: prints(version_line) if source is None else source, **(files or {})}
        home = self.home(name)
        for path, data in tree.items():
            write_file(os.path.join(home, path), data)
        write_file(os.path.join(home, ".complete"), f"{ARCHIVE_DIGEST}\n".encode(), 0o444)
        self.env["PATH"] = os.pathsep.join([os.path.normpath(os.path.join(home, path_dir)), self.env["PATH"]])
        return manifest.Tool(
            name=name,
            version="1.0",
            command=name,
            path_dir=path_dir,
            version_argv=(rel, "--version"),
            version_line=version_line,
            url="file:///unused",
            archive_sha256=ARCHIVE_DIGEST,
            strip=f"{name}/",
            exec_sha256={path: support.sha256(data) for path, data in tree.items()},
            tree_sha256=tree_by_hand(tree),
        )

    def outside(self, name: str, data: bytes) -> str:
        """An executable outside the cache."""
        path = os.path.join(self.world, "elsewhere", name)
        write_file(path, data)
        return path

    def check(self, *tools: manifest.Tool, full_tree: bool = False) -> list[str]:
        return parity.check(list(tools), self.cache, self.env, full_tree=full_tree)


class Passing(ParityTest):
    def test_an_intact_tool_passes_every_check_and_really_ran(self):
        marker = os.path.join(self.world, "ran")
        tool = self.tool(source=prints("go 1.0", marker))
        self.assertEqual(self.check(tool, full_tree=True), [])
        self.assertTrue(os.path.exists(marker))

    def test_several_intact_tools_pass(self):
        self.assertEqual(self.check(self.tool("go"), self.tool("ruff"), full_tree=True), [])

    def test_a_tool_on_the_top_of_its_directory_passes(self):
        # golangci-lint and ruff: path_dir "." and version_argv without a directory.
        self.assertEqual(self.check(self.tool("ruff", path_dir="."), full_tree=True), [])

    def test_goos_and_goarch_are_substituted(self):
        here = GOOS_GOARCH[manifest.platform_key()]
        tool = self.tool(
            source=prints(f"go version go1.27.3 {here}"), version_line="go version go1.27.3 {goos}/{goarch}"
        )
        self.assertEqual(self.check(tool), [])

    def test_only_the_first_stdout_line_counts(self):
        tool = self.tool(source=prints("go 1.0\nand a second line"), version_line="go 1.0")
        self.assertEqual(self.check(tool), [])

    def test_a_cache_reached_through_a_symlink_passes_on_either_side(self):
        # re-check S3: on macOS /tmp and /var/folders are symlinks, so realpath belongs on both sides.
        tool = self.tool()
        link = os.path.join(self.world, "cache-link")
        os.symlink(self.cache, link)
        self.assertEqual(parity.check([tool], link, self.env, full_tree=True), [])
        via_link = {"PATH": os.path.join(link, os.path.basename(self.home("go")), "bin")}
        self.assertEqual(parity.check([tool], self.cache, via_link, full_tree=True), [])

    def test_a_symlink_inside_the_tool_directory_is_inside(self):
        # bin/python3 -> python3.15 is the normal shape of the Python archives.
        tool = self.tool("demo")
        os.symlink("demo", os.path.join(self.home("demo"), "bin", "alias"))
        self.assertEqual(self.check(dataclasses.replace(tool, command="alias")), [])


class NotInstalled(ParityTest):
    def test_a_missing_directory_is_one_reason_naming_make_tools(self):
        # QA counter-example 5, C23(b): the other tool is there and passes.
        present = self.tool("ruff")
        missing = self.tool("go")
        shutil.rmtree(self.home("go"))
        self.assertEqual(self.check(present, missing), ["go: not installed: run make tools"])

    def test_a_manifest_pinning_another_archive_finds_no_directory(self):
        # Plan check case 13: the key holds the version and digest, so the old cache does not satisfy it.
        tool = self.tool()
        self.assertEqual(self.check(tool), [])
        self.assertEqual(
            self.check(dataclasses.replace(tool, archive_sha256="cd" * 32)), ["go: not installed: run make tools"]
        )

    def test_a_fake_on_path_is_never_called_when_the_cache_is_empty(self):
        # C26. The first half is the witness that the fake leaves a marker when something runs it.
        marker = os.path.join(self.world, "fake-ran")
        fake = self.outside("go", prints("go 1.0", marker))
        child = support.run_child([fake], env={}, cwd=self.world, out_dir=os.path.join(self.world, "logs"))
        self.assertEqual(child.code, 0)
        self.assertTrue(os.path.exists(marker))
        os.unlink(marker)
        tool = self.tool()
        shutil.rmtree(self.home("go"))
        self.env["PATH"] = os.path.dirname(fake)
        self.assertEqual(self.check(tool), ["go: not installed: run make tools"])
        self.assertFalse(os.path.exists(marker))


class PathCheck(ParityTest):
    def test_a_tool_outside_the_cache_first_on_path_is_refused_and_never_run(self):
        # C23(c), plan check case 15. The same bytes run from the cache are the witness of the marker.
        marker = os.path.join(self.world, "ran")
        source = prints("go 1.0", marker)
        tool = self.tool(source=source)
        self.assertEqual(self.check(tool), [])
        self.assertTrue(os.path.exists(marker))
        os.unlink(marker)
        system_go = self.outside("go", source)
        self.env["PATH"] = os.pathsep.join([os.path.dirname(system_go), self.env["PATH"]])
        self.assertEqual(self.check(tool), [f"go: path: {system_go} is outside the cache: {RECOVER}"])
        self.assertFalse(os.path.exists(marker))

    def test_a_symlink_to_a_byte_identical_copy_outside_is_refused_and_never_run(self):
        # QA counter-example 6: every hash would match, and only the path check can see it.
        marker = os.path.join(self.world, "ran")
        source = prints("go 1.0", marker)
        tool = self.tool(source=source)
        copy = self.outside("go-copy", source)
        inside = os.path.join(self.home("go"), "bin", "go")
        os.unlink(inside)
        os.symlink(copy, inside)
        self.assertEqual(self.check(tool, full_tree=True), [f"go: path: {copy} is outside the cache: {RECOVER}"])
        self.assertFalse(os.path.exists(marker))

    def test_a_cache_directory_that_lacks_the_binary_falls_through_to_the_path_check(self):
        # Design note §8 test_parity_path's shape.
        marker = os.path.join(self.world, "ran")
        tool = self.tool()
        os.unlink(os.path.join(self.home("go"), "bin", "go"))
        system_go = self.outside("go", prints("go 1.0", marker))
        self.env["PATH"] = os.pathsep.join([os.path.join(self.home("go"), "bin"), os.path.dirname(system_go)])
        self.assertEqual(self.check(tool), [f"go: path: {system_go} is outside the cache: {RECOVER}"])
        self.assertFalse(os.path.exists(marker))

    def test_a_command_found_nowhere_is_refused(self):
        tool = self.tool()
        self.env["PATH"] = os.path.join(self.world, "no-such-directory")
        self.assertEqual(self.check(tool), [f"go: path: go is not on PATH: {RECOVER}"])

    def test_an_environment_without_path_is_refused_not_a_crash(self):
        tool = self.tool()
        del self.env["PATH"]
        self.assertEqual(self.check(tool), [f"go: path: go is not on PATH: {RECOVER}"])

    def test_another_tools_directory_is_outside_this_tools_cache_directory(self):
        # The check is the tool's own directory, not merely somewhere in the cache.
        ruff = self.tool("ruff")
        go = self.tool("go")
        impostor = os.path.join(self.home("ruff"), "bin", "go")
        write_file(impostor, prints("go 1.0"))
        self.env["PATH"] = os.path.join(self.home("ruff"), "bin")
        self.assertEqual(self.check(go, ruff), [f"go: path: {impostor} is outside the cache: {RECOVER}"])

    def test_the_version_binary_is_checked_as_well_as_the_command(self):
        # version_argv[0] may differ from command; neither may run from outside the cache.
        marker = os.path.join(self.world, "ran")
        tool = self.tool(files={"bin/probe": prints("go 1.0", marker)})
        probing = dataclasses.replace(tool, version_argv=("bin/probe", "--version"))
        self.assertEqual(self.check(probing), [])
        self.assertTrue(os.path.exists(marker))
        os.unlink(marker)
        inside = os.path.join(self.home("go"), "bin", "probe")
        copy = self.outside("probe-copy", prints("go 1.0", marker))
        os.unlink(inside)
        os.symlink(copy, inside)
        self.assertEqual(self.check(probing), [f"go: path: {copy} is outside the cache: {RECOVER}"])
        self.assertFalse(os.path.exists(marker))


class VersionCheck(ParityTest):
    def test_a_binary_reporting_another_version_is_the_only_reason(self):
        # QA counter-example 4, C23(a): the key, the path and every hash are right.
        tool = self.tool(source=prints("go version go1.27.2 x/y"), version_line="go version go1.27.3 x/y")
        self.assertEqual(
            self.check(tool, full_tree=True),
            [f"go: version: want 'go version go1.27.3 x/y', got 'go version go1.27.2 x/y': {RECOVER}"],
        )

    def test_a_carriage_return_is_a_different_line(self):
        tool = self.tool(source=script("import sys\nsys.stdout.buffer.write(b'go 1.0\\r\\n')"), version_line="go 1.0")
        self.assertEqual(self.check(tool), [f"go: version: want 'go 1.0', got 'go 1.0\\r': {RECOVER}"])

    def test_only_a_newline_ends_a_line(self):
        # NEL ends a line for str.splitlines; a tool printing it would pass a splitlines comparison.
        tool = self.tool(source=script("import sys\nsys.stdout.buffer.write(b'go 1.0\\xc2\\x85tail\\n')"))
        self.assertEqual(self.check(tool), [f"go: version: want 'go 1.0', got 'go 1.0\\x85tail': {RECOVER}"])

    def test_undecodable_output_is_replaced_not_a_crash(self):
        tool = self.tool(source=script("import sys\nsys.stdout.buffer.write(b'\\xff\\n')"))
        self.assertEqual(self.check(tool), [f"go: version: want 'go 1.0', got '\ufffd': {RECOVER}"])

    def test_no_output_is_a_mismatch(self):
        tool = self.tool(source=script("import sys\nsys.exit(0)"))
        self.assertEqual(self.check(tool), [f"go: version: want 'go 1.0', got '': {RECOVER}"])

    def test_a_version_on_stderr_does_not_count(self):
        tool = self.tool(source=script("import sys\nprint('go 1.0', file=sys.stderr)"))
        self.assertEqual(self.check(tool), [f"go: version: want 'go 1.0', got '': {RECOVER}"])

    def test_the_right_line_with_a_failing_exit_is_refused(self):
        tool = self.tool(source=script("import sys\nprint('go 1.0')\nsys.stdout.flush()\nsys.exit(3)"))
        self.assertEqual(self.check(tool), [f"go: version: want 'go 1.0', got 'go 1.0' (exit 3): {RECOVER}"])

    def test_a_binary_killed_by_a_signal_is_refused(self):
        tool = self.tool(source=script("import os, signal\nos.kill(os.getpid(), signal.SIGKILL)"))
        self.assertEqual(
            self.check(tool), [f"go: version: want 'go 1.0', got '' (exit -{int(signal.SIGKILL)}): {RECOVER}"]
        )

    def test_a_binary_that_cannot_start_is_a_reason_not_a_crash(self):
        tool = self.tool(source=b"\x00\x01 this is no executable format\n")
        reasons = self.check(tool)
        self.assertEqual(len(reasons), 1)
        self.assertTrue(reasons[0].startswith("go: version: could not run "), reasons)
        self.assertTrue(reasons[0].endswith(RECOVER), reasons)

    def test_the_version_runs_only_in_the_environment_it_is_given(self):
        # re-check S4 and C25: an inherited GOTOOLCHAIN makes the cached go fetch another toolchain.
        self.assertIn("HOME", os.environ)  # the witness that this process has a variable to leak
        tool = self.tool(
            source=script("import os\nprint(os.environ.get('GOTOOLCHAIN'), os.environ.get('HOME'))"),
            version_line="local None",
        )
        self.env["GOTOOLCHAIN"] = "local"
        self.assertEqual(self.check(tool), [])

    def test_the_version_runs_from_the_tool_directory(self):
        tool = self.tool(source=script("import os\nprint(os.getcwd())"), version_line=self.home("go"))
        self.assertEqual(self.check(tool), [])

    def test_stdin_is_empty(self):
        tool = self.tool(source=script("import sys\nprint(repr(sys.stdin.read()))"), version_line="''")
        self.assertEqual(self.check(tool), [])

    def test_a_hung_version_is_killed_and_named(self):
        tool = self.tool(source=script("import signal\nsignal.pause()"))
        timing = parity.Timing(cap_s=0.2, grace_s=support.CHILD_CAP_S)
        reasons = parity._check([tool], self.cache, self.env, full_tree=False, timing=timing)
        self.assertEqual(reasons, [f"go: version: no answer within 0.2s: {RECOVER}"])

    def test_a_hung_version_that_ignores_term_is_killed(self):
        # A real cap could fire before the fixture has installed its handler (a new file's first launch
        # takes about 0.4 s on macOS), and then KILL is never needed. So the cap here is a fake: it fires
        # only after the fixture reports that TERM is ignored, and the grace ends with the fixture alive.
        # The FIFO is open both ways, so the read blocks for the byte and never sees end-of-file.
        fifo = os.path.join(self.world, "ready")
        os.mkfifo(fifo)
        ready = os.open(fifo, os.O_RDWR)
        self.addCleanup(os.close, ready)
        tool = self.tool(
            source=script(
                "import os, signal\n"
                "signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
                f"os.write(os.open({fifo!r}, os.O_WRONLY), b'r')\n"
                "while True:\n"
                "    signal.pause()"
            )
        )
        waits: list[float] = []

        def wait(proc, timeout):
            if not waits:
                self.assertEqual(os.read(ready, 1), b"r")
            else:
                self.assertIsNone(proc.poll())  # the witness: TERM has gone and the fixture is still alive
            waits.append(timeout)
            raise subprocess.TimeoutExpired(proc.args, timeout)

        timing = parity.Timing(cap_s=7, grace_s=11, wait=wait)
        reasons = parity._check([tool], self.cache, self.env, full_tree=False, timing=timing)
        self.assertEqual(reasons, [f"go: version: no answer within 7s: {RECOVER}"])
        self.assertEqual(waits, [7, 11])


class NothingOutlivesTheProbe(ParityTest):
    def test_a_grandchild_left_running_is_killed_with_its_group(self):
        # The witness is a FIFO the grandchild holds open: its pid arrives first, then end-of-file proves
        # that every writer is gone. Parity returns only after the leader has exited.
        fifo = os.path.join(self.world, "held")
        os.mkfifo(fifo)
        reader = os.open(fifo, os.O_RDONLY | os.O_NONBLOCK)
        self.addCleanup(os.close, reader)
        grandchild = (
            "import os, signal\n"
            f"held = os.open({fifo!r}, os.O_WRONLY)\n"
            "os.write(held, b'%d\\n' % os.getpid())\n"
            "print('ready', flush=True)\n"
            "signal.pause()\n"
        )
        leader = script(
            "import subprocess, sys\n"
            f"child = subprocess.Popen([sys.executable, '-IBc', {grandchild!r}], stdout=subprocess.PIPE)\n"
            "child.stdout.readline()\n"
            "print('go 1.0')\n"
        )
        self.assertEqual(self.check(self.tool(source=leader, version_line="go 1.0")), [])
        os.set_blocking(reader, True)
        pid = int(os.read(reader, 32).strip())
        self.addCleanup(self.kill_quietly, pid)
        readable, _, _ = select.select([reader], [], [], support.CHILD_CAP_S)
        self.assertEqual(readable, [reader], "the grandchild outlived the version probe")
        self.assertEqual(os.read(reader, 1), b"")

    def kill_quietly(self, pid: int) -> None:
        with contextlib.suppress(ProcessLookupError):
            os.kill(pid, signal.SIGKILL)


class HashCheck(ParityTest):
    def flip(self, path: str) -> None:
        with open(path, "rb") as f:
            data = bytearray(f.read())
        data[-1] ^= 1
        write_file(path, bytes(data))

    def test_a_flipped_byte_in_a_listed_file_is_named(self):
        # Plan check "a tampered cached executable"; the first check is the witness on intact bytes.
        tool = self.tool(files={"pkg/tool/compile": b"compiler bytes\n"})
        self.assertEqual(self.check(tool), [])
        self.flip(os.path.join(self.home("go"), "pkg/tool/compile"))
        self.assertEqual(self.check(tool), [f"go: sha256 of pkg/tool/compile differs: {RECOVER}"])

    def test_a_flipped_byte_in_the_binary_that_still_reports_its_version_is_named(self):
        tool = self.tool(source=prints("go 1.0") + b"# a trailing comment\n", version_line="go 1.0")
        self.flip(os.path.join(self.home("go"), "bin", "go"))
        self.assertEqual(self.check(tool), [f"go: sha256 of bin/go differs: {RECOVER}"])

    def test_a_missing_listed_file_differs(self):
        tool = self.tool(files={"pkg/tool/compile": b"compiler bytes\n"})
        os.unlink(os.path.join(self.home("go"), "pkg/tool/compile"))
        self.assertEqual(self.check(tool), [f"go: sha256 of pkg/tool/compile differs: {RECOVER}"])

    def test_every_listed_file_is_hashed(self):
        files = {f"pkg/tool/{name}": name.encode() for name in ("asm", "compile", "vet")}
        tool = self.tool(files=files)
        for name in ("asm", "compile", "vet"):
            with self.subTest(name):
                path = os.path.join(self.home("go"), "pkg/tool", name)
                self.flip(path)
                self.assertEqual(self.check(tool), [f"go: sha256 of pkg/tool/{name} differs: {RECOVER}"])
                self.flip(path)

    def test_a_listed_file_replaced_by_a_link_to_an_identical_copy_differs(self):
        # go runs pkg/tool/compile without parity's path check: its hash must not pass through a link.
        data = b"compiler bytes\n"
        tool = self.tool(files={"pkg/tool/compile": data})
        copy = self.outside("compile", data)
        inside = os.path.join(self.home("go"), "pkg/tool/compile")
        os.unlink(inside)
        os.symlink(copy, inside)
        self.assertEqual(self.check(tool), [f"go: sha256 of pkg/tool/compile differs: {RECOVER}"])

    def test_a_listed_file_reached_through_a_linked_directory_differs(self):
        data = b"compiler bytes\n"
        tool = self.tool(files={"pkg/tool/compile": data})
        elsewhere = os.path.dirname(self.outside("compile", data))
        inside = os.path.join(self.home("go"), "pkg/tool")
        shutil.rmtree(inside)
        os.symlink(elsewhere, inside)
        self.assertEqual(self.check(tool), [f"go: sha256 of pkg/tool/compile differs: {RECOVER}"])


class TreeCheck(ParityTest):
    TREE_REASON = f"go: tree differs: {RECOVER}"

    def assert_only_full_tree_sees_it(self, tool: manifest.Tool) -> None:
        self.assertEqual(self.check(tool, full_tree=False), [])
        self.assertEqual(self.check(tool, full_tree=True), [self.TREE_REASON])

    def test_an_added_file_is_seen_at_full_tree_only(self):
        tool = self.tool()
        self.assertEqual(self.check(tool, full_tree=True), [])
        write_file(os.path.join(self.home("go"), "src/planted.go"), b"package x\n", 0o444)
        self.assert_only_full_tree_sees_it(tool)

    def test_a_changed_unlisted_file_is_seen_at_full_tree_only(self):
        tool = self.tool(files={"src/a.go": b"package a\n"})
        narrowed = dataclasses.replace(tool, exec_sha256={"bin/go": tool.exec_sha256["bin/go"]})
        self.assertEqual(self.check(narrowed, full_tree=True), [])
        write_file(os.path.join(self.home("go"), "src/a.go"), b"package b\n")
        self.assert_only_full_tree_sees_it(narrowed)

    def test_a_removed_execute_bit_is_seen(self):
        tool = self.tool(files={"lib/data": b"data\n"})
        self.assertEqual(self.check(tool, full_tree=True), [])
        os.chmod(os.path.join(self.home("go"), "lib/data"), 0o444)
        self.assert_only_full_tree_sees_it(tool)

    def test_the_stamp_is_outside_the_digest(self):
        tool = self.tool()
        write_file(os.path.join(self.home("go"), ".complete"), b"anything\n", 0o444)
        self.assertEqual(self.check(tool, full_tree=True), [])

    def test_an_unreadable_file_differs_rather_than_crashes(self):
        tool = self.tool()
        path = os.path.join(self.home("go"), "src/secret")
        write_file(path, b"x", 0o000)
        self.assertEqual(self.check(tool, full_tree=True), [self.TREE_REASON])


class Ordering(ParityTest):
    def test_each_tool_gets_only_its_first_failing_check(self):
        # Wrong version and a flipped file: the version is checked first, so only it is reported.
        tool = self.tool(source=prints("go 2.0"), version_line="go 1.0", files={"pkg/compile": b"c\n"})
        write_file(os.path.join(self.home("go"), "pkg/compile"), b"d\n")
        self.assertEqual(self.check(tool, full_tree=True), [f"go: version: want 'go 1.0', got 'go 2.0': {RECOVER}"])

    def test_every_failing_tool_is_listed_in_the_order_given(self):
        # QA counter-example 7: go's version off and ruff missing, reported together.
        go = self.tool("go", source=prints("go 0.9"), version_line="go 1.0")
        ruff = self.tool("ruff")
        python = self.tool("python")
        shutil.rmtree(self.home("ruff"))
        go_reason = f"go: version: want 'go 1.0', got 'go 0.9': {RECOVER}"
        ruff_reason = "ruff: not installed: run make tools"
        self.assertEqual(self.check(python, go, ruff), [go_reason, ruff_reason])
        self.assertEqual(self.check(ruff, python, go), [ruff_reason, go_reason])

    def test_no_tools_is_no_reasons(self):
        self.assertEqual(self.check(), [])


class RealCache(support.ToolchainTest):
    """The pinned tools themselves, read-only: this cache is shared, and these tests never write to it."""

    def setUp(self) -> None:
        super().setUp()
        self.real_cache = manifest.cache_root(os.environ)
        self.tools = manifest.load(os.path.join(support.REPO, "tools.json"), manifest.platform_key())
        self.env = {
            "PATH": os.pathsep.join(manifest.path_dirs(self.real_cache, self.tools)),
            "GOTOOLCHAIN": "local",
            "GOENV": "off",
            "GOFLAGS": "",
            "GOWORK": "off",
            "CGO_ENABLED": "0",
            "HOME": self.world,
        }

    def test_the_installed_tools_pass_every_check_and_their_whole_trees_match(self):
        self.assertEqual(parity.check(self.tools, self.real_cache, self.env, full_tree=True), [])

    def test_a_flip_in_a_real_copy_is_named_and_the_real_cache_is_unchanged(self):
        # QA counter-example 9 and DQ24: a real copy, never a hard link, and the original re-hashed after.
        go = next(tool for tool in self.tools if tool.name == "go")
        vet = next(rel for rel in go.exec_sha256 if rel.endswith("/vet"))
        real_home = manifest.tool_home(self.real_cache, go)
        cache = os.path.join(self.world, "copied-cache")
        home = manifest.tool_home(cache, go)
        for rel in ("bin/go", vet):
            target = os.path.join(home, rel)
            os.makedirs(os.path.dirname(target), exist_ok=True)
            shutil.copyfile(os.path.join(real_home, rel), target)
            os.chmod(target, 0o755)
            self.assertNotEqual(os.stat(target).st_ino, os.stat(os.path.join(real_home, rel)).st_ino)
        with open(os.path.join(home, vet), "r+b") as f:
            f.seek(100)
            byte = f.read(1)
            f.seek(100)
            f.write(bytes([byte[0] ^ 1]))
        narrowed = dataclasses.replace(go, exec_sha256={rel: go.exec_sha256[rel] for rel in ("bin/go", vet)})
        env = {**self.env, "PATH": os.path.join(home, "bin")}
        self.assertEqual(
            parity.check([narrowed], cache, env, full_tree=False), [f"go: sha256 of {vet} differs: {RECOVER}"]
        )
        self.assertEqual(support.sha256_of(os.path.join(real_home, vet)), go.exec_sha256[vet])


if __name__ == "__main__":
    unittest.main()
