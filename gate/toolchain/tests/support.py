"""Shared fixtures of the toolchain tests. Runs under the pinned 3.15.2 and the pinned 3.12.16.

Each test owns a temporary world; fixture archives are built at test time and served as file:// URLs.
Expected values come from the fixtures' literal bytes through hashlib, never from the code under test.
"""

import contextlib
import dataclasses
import faulthandler
import hashlib
import io
import json
import os
import pathlib
import shutil
import signal
import stat
import subprocess
import sys
import tarfile
import tempfile
import unittest

from gate.toolchain import manifest

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
CEILING_S = 120
CHILD_CAP_S = 60


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_of(path: str) -> str:
    with open(path, "rb") as f:
        return sha256(f.read())


def restore_write(root: str) -> None:
    """Give a test's read-only tree back to the temporary directory's cleanup."""
    os.chmod(root, 0o700)
    for dirpath, dirnames, filenames in os.walk(root):
        for name in dirnames + filenames:
            path = os.path.join(dirpath, name)
            if not os.path.islink(path):
                os.chmod(path, os.lstat(path).st_mode | 0o700)


def snapshot(root: str) -> dict[str, tuple]:
    """Every entry under root, by relative path: its kind, its permission bits and what it holds."""
    entries: dict[str, tuple] = {}
    for dirpath, dirnames, filenames in os.walk(root):
        for name in dirnames + filenames:
            path = os.path.join(dirpath, name)
            rel = os.path.relpath(path, root)
            st = os.lstat(path)
            if stat.S_ISLNK(st.st_mode):
                entries[rel] = ("l", os.readlink(path))
            elif stat.S_ISDIR(st.st_mode):
                entries[rel] = ("d", stat.S_IMODE(st.st_mode))
            else:
                entries[rel] = ("f", stat.S_IMODE(st.st_mode), sha256_of(path))
    return dict(sorted(entries.items()))


class ToolchainTest(unittest.TestCase):
    """A test with its own world: self.world (realpath-resolved, removed afterwards) and a ceiling."""

    def setUp(self) -> None:
        faulthandler.dump_traceback_later(CEILING_S, exit=True, file=sys.__stderr__)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory(prefix="gate-toolchain-")
        self.addCleanup(tmp.cleanup)
        self.addCleanup(restore_write, tmp.name)
        self.world = os.path.realpath(tmp.name)
        self.cache = os.path.join(self.world, "cache")
        self.repo = os.path.join(self.world, "repo")
        os.makedirs(self.repo)


@dataclasses.dataclass(frozen=True)
class Member:
    """One tar member of a fixture archive; kind is f, d, l (symlink), h (hard link), fifo, chr or blk."""

    name: str
    kind: str = "f"
    data: bytes = b"content\n"
    link: str = ""
    mode: int = 0o644


_TAR_TYPES = {
    "f": tarfile.REGTYPE,
    "d": tarfile.DIRTYPE,
    "l": tarfile.SYMTYPE,
    "h": tarfile.LNKTYPE,
    "fifo": tarfile.FIFOTYPE,
    "chr": tarfile.CHRTYPE,
    "blk": tarfile.BLKTYPE,
}


def write_archive(path: str, members: list[Member]) -> str:
    """Write a .tar.gz holding exactly these headers, however hostile, in this order."""
    with tarfile.open(path, "w:gz", format=tarfile.PAX_FORMAT) as tf:
        for m in members:
            info = tarfile.TarInfo(m.name)
            info.type = _TAR_TYPES[m.kind]
            info.mode = m.mode
            info.linkname = m.link
            if m.kind == "f":
                info.size = len(m.data)
                tf.addfile(info, io.BytesIO(m.data))
            else:
                tf.addfile(info)
    return path


def tool_entry(
    archive: str,
    *,
    name: str = "demo",
    version: str = "1.0",
    strip: str = "demo/",
    command: str = "demo",
    path_dir: str = "bin",
    exec_sha256: dict[str, str] | None = None,
    pinned: str | None = None,
    url: str | None = None,
) -> dict:
    """One tools.json entry for the current platform, pinning the archive as written unless told otherwise."""
    return {
        "command": command,
        "name": name,
        "path_dir": path_dir,
        "platforms": {
            manifest.platform_key(): {
                "archive_sha256": pinned or sha256_of(archive),
                "exec_sha256": exec_sha256 or {f"{path_dir}/{command}": "0" * 64},
                "strip": strip,
                "tree_sha256": "0" * 64,
                "url": url or pathlib.Path(archive).as_uri(),
            }
        },
        "version": version,
        "version_argv": [f"{path_dir}/{command}", "--version"],
        "version_line": f"{name} {version}",
    }


def write_tools(repo: str, tools: list[dict]) -> str:
    path = os.path.join(repo, "tools.json")
    with open(path, "w", encoding="utf-8") as f:
        json.dump({"schema": 1, "tools": tools}, f, sort_keys=True, indent=2)
    return path


def copy_gate(repo: str) -> None:
    """Copy the real bootstrap and installer into a fixture repository (real copies, never links)."""
    for rel in (
        "gate/__init__.py",
        "gate/bootstrap.py",
        "gate/toolchain/__init__.py",
        "gate/toolchain/install.py",
        "gate/toolchain/manifest.py",
    ):
        target = os.path.join(repo, rel)
        os.makedirs(os.path.dirname(target), exist_ok=True)
        shutil.copyfile(os.path.join(REPO, rel), target)


@dataclasses.dataclass(frozen=True)
class Child:
    code: int
    stdout: str
    stderr: str

    @property
    def lines(self) -> list[str]:
        return self.stdout.splitlines()


def kill_group(pgid: int) -> None:
    with contextlib.suppress(ProcessLookupError):
        os.killpg(pgid, signal.SIGKILL)


def run_child(argv: list[str], *, env: dict[str, str], cwd: str, out_dir: str) -> Child:
    """Run one child in its own session with its output in files; a hung child fails the test by name."""
    os.makedirs(out_dir, exist_ok=True)
    fd, out_path = tempfile.mkstemp(prefix="stdout.", dir=out_dir)
    os.close(fd)
    err_path = out_path.replace("stdout.", "stderr.")
    with open(out_path, "wb") as out, open(err_path, "wb") as err:
        proc = subprocess.Popen(
            argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True
        )
    try:
        code = proc.wait(timeout=CHILD_CAP_S)
    except subprocess.TimeoutExpired:
        kill_group(proc.pid)
        proc.wait()
        raise AssertionError(f"child did not exit within {CHILD_CAP_S}s: {argv}") from None
    finally:
        kill_group(proc.pid)  # also when the shim's TERM stops this test (review BA1)
    with open(out_path, "rb") as f:
        stdout = f.read().decode("utf-8", errors="replace")
    with open(err_path, "rb") as f:
        stderr = f.read().decode("utf-8", errors="replace")
    return Child(code, stdout, stderr)
