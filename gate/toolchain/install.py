"""The installer: download, verify, extract and publish the pinned tools (design note §3).

Imported by gate/bootstrap.py once it has checked the floor, so this runs under any python3 >= 3.12, and
ruff holds it to that target. It never gives a gate verdict: its only exits are 0 and 2, and every refusal
prints its fix, then `GATE FAILED` and `exit: 2`.

An archive is verified by its sha256 before it is opened, and is still treated as hostile: tarfile keeps
absolute paths and `..` in member names, so every rule of §3 step 4 is checked here, member by member.
"""

import contextlib
import gzip
import http.client
import os
import posixpath
import re
import shutil
import stat
import sys
import tarfile
import tempfile
import typing
import urllib.request
import zlib
from collections.abc import Callable, Mapping, Sequence
from typing import BinaryIO, TextIO

from gate.toolchain import manifest

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
FLOOR = (3, 12)
FLOOR_FIX = "install Python 3.12+ (e.g. brew install python)"
USAGE = "usage: bootstrap.py install | exec -- <args> | clean-tools"
GATE_PYTHON = manifest.GATE_PYTHON
DOWNLOAD_TIMEOUT_S = 60  # a socket read with no data for this long is a hang, not a slow link
REINSTALL = "run make clean-tools, then run make tools"

_CHUNK = 1 << 20
_NEW_FILE = os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC
_DIRECTORY_ON_DEMAND = 0o755
_STAMP = ".complete"
_CACHE_ENTRY = re.compile(r"(?:dl|tmp)\.[0-9]+\.[a-z0-9_]+|[A-Za-z0-9][A-Za-z0-9.+_-]*-[0-9a-f]{12}")

Fetch = Callable[[str, BinaryIO], None]
Rename = Callable[[str, str], None]
Execv = Callable[[str, list[str]], object]


class InstallError(Exception):
    """A refusal: the message names what is wrong and how to fix it."""


class _Rejected(Exception):
    """An archive member that §3 step 4 refuses."""

    def __init__(self, name: str, why: str) -> None:
        super().__init__(f"archive member {name!r} refused: {why}")


def _fetch(url: str, sink: BinaryIO) -> None:
    with urllib.request.urlopen(url, timeout=DOWNLOAD_TIMEOUT_S) as response:
        shutil.copyfileobj(response, sink, _CHUNK)


def _name_problem(name: str) -> str | None:
    """Why this tar name can never be extracted, or None."""
    if name.startswith("/"):
        return "an absolute path"
    parts = name.split("/")
    if ".." in parts:
        return "a `..` component"
    if "" in parts or "." in parts:
        return "an empty or `.` component"
    if "\n" in name:
        return "a newline in the name"
    try:
        name.encode("utf-8")
    except UnicodeEncodeError:
        return "a name that is not UTF-8"
    return None


def _check_ends_whole(tf: tarfile.TarFile) -> None:
    """A tar stream ends with zero-filled end-of-archive blocks. tarfile stops without a word at a header cut short
    after the first member, treating it as the end (break-it QB5), so whatever follows the last member read must
    be those blocks and nothing else; otherwise the archive was truncated and only part of the tool came out."""
    tf.fileobj.seek(tf.offset)
    rest = tf.fileobj.read()
    if len(rest) < 2 * tarfile.BLOCKSIZE or rest.strip(b"\0"):
        raise _Rejected("<end of archive>", "the tar stream stops before its end-of-archive blocks: it is truncated")


class _Extraction:
    """One archive going into one directory. Nothing is ever written through a symlink: `kinds` records
    what this extraction made at each path, every parent of a write is a directory it made itself, and
    new files use O_EXCL | O_NOFOLLOW."""

    def __init__(self, strip: str, dest: str) -> None:
        self.strip = strip
        self.root = strip[:-1]
        self.dest = dest
        self.kinds: dict[str, str] = {}  # relative path -> d, f or l
        self.modes: dict[str, int] = {}  # relative path -> permission bits from the archive; symlinks have none

    def extract(self, archive: str) -> None:
        try:
            with tarfile.open(archive, "r:gz", encoding="utf-8") as tf:
                for member in tf:
                    try:
                        self._member(tf, member)
                    except FileExistsError as e:  # a case-insensitive filesystem folds two names into one
                        raise _Rejected(member.name, "it collides with an earlier member on this filesystem") from e
                _check_ends_whole(tf)
        except (EOFError, zlib.error, tarfile.ReadError, gzip.BadGzipFile) as e:  # a cut or corrupt stream (QB9)
            raise _Rejected("<archive>", f"the archive is truncated or corrupt ({type(e).__name__})") from e
        self._check_resolved_links()

    def _relative(self, name: str) -> str:
        problem = _name_problem(name)
        if problem is not None:
            raise _Rejected(name, problem)
        if name == self.root:
            return ""
        if not name.startswith(self.strip):
            raise _Rejected(name, f"it is outside {self.strip}")
        return name[len(self.strip) :]

    def _member(self, tf: tarfile.TarFile, member: tarfile.TarInfo) -> None:
        name = member.name
        rel = self._relative(name)
        if not rel:
            if not member.isdir():
                raise _Rejected(name, "the tree root is not a directory")
            return
        if rel == _STAMP:
            raise _Rejected(name, "it would forge the installer's stamp")
        self._make_parents(name, rel)
        seen = self.kinds.get(rel)
        if member.isdir() and seen == "d":
            self.modes[rel] = member.mode & 0o777  # a parent made on demand, now named by its own member
            return
        if seen is not None:
            raise _Rejected(name, "a member of this name was already extracted")
        target = os.path.join(self.dest, rel)
        if member.isdir():
            self._directory(rel, member.mode & 0o777)
        elif member.isreg():
            with os.fdopen(os.open(target, _NEW_FILE, 0o600), "wb") as out, tf.extractfile(member) as src:
                shutil.copyfileobj(src, out, _CHUNK)
            self.kinds[rel], self.modes[rel] = "f", member.mode & 0o777
        elif member.issym():
            self._check_symlink(name, rel, member.linkname)
            os.symlink(member.linkname, target)
            self.kinds[rel] = "l"
        elif member.islnk():
            source = self._hard_link_source(name, member.linkname)
            os.link(os.path.join(self.dest, source), target)
            self.kinds[rel], self.modes[rel] = "f", self.modes[source]
        else:
            raise _Rejected(name, "a device, FIFO or other special file")

    def _directory(self, rel: str, mode: int) -> None:
        os.mkdir(os.path.join(self.dest, rel), 0o700)  # the archive's mode is applied last: children come first
        self.kinds[rel], self.modes[rel] = "d", mode

    def _make_parents(self, name: str, rel: str) -> None:
        parts = rel.split("/")
        for depth in range(1, len(parts)):
            parent = "/".join(parts[:depth])
            kind = self.kinds.get(parent)
            if kind is None:
                self._directory(parent, _DIRECTORY_ON_DEMAND)
            elif kind != "d":
                raise _Rejected(name, f"its path passes through {'a symlink' if kind == 'l' else 'a file'}: {parent}")

    def _check_symlink(self, name: str, rel: str, target: str) -> None:
        """Judge the target by name alone: several Python symlinks come before what they point at."""
        if not target or "\n" in target:
            raise _Rejected(name, f"an unusable link target {target!r}")
        resolved = posixpath.normpath(posixpath.join(posixpath.dirname(rel), target))
        if target.startswith("/") or resolved == ".." or resolved.startswith("../"):
            raise _Rejected(name, f"its target escapes the tree: {target!r}")

    def _hard_link_source(self, name: str, target: str) -> str:
        problem = _name_problem(target)
        if problem is not None:
            raise _Rejected(name, f"its hard link target has {problem}: {target!r}")
        if not target.startswith(self.strip):
            raise _Rejected(name, f"its hard link target is outside {self.strip}: {target!r}")
        source = target[len(self.strip) :]
        if self.kinds.get(source) != "f":
            raise _Rejected(name, f"its hard link target is not an earlier regular file: {target!r}")
        return source

    def _check_resolved_links(self) -> None:
        """A chain of links can leave the tree though each stays inside by name, so with every member in
        place, resolve each one for real. Missing targets are fine: realpath does not require them."""
        tree = os.path.realpath(self.dest)
        for rel, kind in sorted(self.kinds.items()):
            if kind != "l":
                continue
            link = os.path.join(self.dest, rel)
            resolved = os.path.realpath(link)
            if resolved != tree and not resolved.startswith(tree + os.sep):
                raise _Rejected(self.strip + rel, f"it resolves outside the tree: {os.readlink(link)!r}")


def _extract(archive: str, strip: str, dest: str) -> dict[str, int]:
    """Extract `archive` into the empty directory `dest` under §3 step 4's rules and return the modes to seal with
    (S1.1c-4 design §4.1). It never seals: the installer writes its stamp inside `dest` first (review DA-B2)."""
    extraction = _Extraction(strip, dest)
    extraction.extract(archive)
    return extraction.modes


def _seal(tree: str, modes: Mapping[str, int]) -> None:
    """Clear every write bit, deepest first. Symlinks are not in `modes`: they are never chmod-ed."""
    for rel in sorted(modes, key=lambda path: (-path.count("/"), path)):
        os.chmod(os.path.join(tree, rel), modes[rel] & ~0o222)
    os.chmod(tree, stat.S_IMODE(os.stat(tree).st_mode) & ~0o222)


def _restore_write(root: str) -> None:
    """Give the owner back every directory and file under root, never following a symlink."""
    os.chmod(root, stat.S_IMODE(os.stat(root).st_mode) | 0o700)
    for dirpath, dirnames, filenames in os.walk(root):
        for name in dirnames + filenames:
            path = os.path.join(dirpath, name)
            mode = os.lstat(path).st_mode
            if not stat.S_ISLNK(mode):
                os.chmod(path, stat.S_IMODE(mode) | (0o700 if stat.S_ISDIR(mode) else 0o200))


def _stamped(home: str, digest: str) -> bool:
    """Does this directory hold a finished install of this archive?"""
    try:
        with open(os.path.join(home, _STAMP), "rb") as f:
            return f.read(len(digest) + 2) == f"{digest}\n".encode("ascii")
    except OSError:
        return False


def _unfinished(home: str) -> InstallError:
    return InstallError(f"{home}: {_STAMP} is missing or wrong, so this is no finished install: {REINSTALL}")


def _publish(tmp: str, home: str, digest: str, rename: Rename) -> bool:
    """Move the sealed tree into place. False means another install got there first and its tree stands."""
    try:
        rename(tmp, home)
    except OSError:
        if not os.path.lexists(home):
            raise
        if not _stamped(home, digest):
            raise _unfinished(home) from None
        return False
    return True


def _install_tool(tool: manifest.Tool, cache: str, home: str, fetch: Fetch, rename: Rename) -> bool:
    """Steps 2-6 for one tool. True if this call published the tree, False if another install did."""
    digest = tool.archive_sha256
    fd, download = tempfile.mkstemp(prefix=f"dl.{os.getpid()}.", dir=cache)
    try:
        with os.fdopen(fd, "wb") as sink:
            try:
                fetch(tool.url, sink)
            except (OSError, ValueError, http.client.HTTPException) as e:
                raise InstallError(
                    f"{tool.name}: download failed: {tool.url}: {e}: check the network, then run make tools"
                ) from e
        got = manifest.sha256_file(download)
        if got != digest:
            raise InstallError(
                f"{tool.name}: {tool.url}: archive sha256 {got} differs from the pinned {digest}: "
                "run make tools to download it again; never edit the pin to pass"
            )
        tmp = tempfile.mkdtemp(prefix=f"tmp.{os.getpid()}.", dir=cache)
        try:
            try:
                modes = _extract(download, tool.strip, tmp)
            except _Rejected as e:
                raise InstallError(
                    f"{tool.name}: {tool.url}: {e}: this archive cannot be installed: check its pin in tools.json"
                ) from e
            with os.fdopen(os.open(os.path.join(tmp, _STAMP), _NEW_FILE, 0o600), "w", encoding="ascii") as stamp:
                stamp.write(f"{digest}\n")
            _seal(tmp, {**modes, _STAMP: 0o444})
            return _publish(tmp, home, digest, rename)
        finally:
            if os.path.lexists(tmp):
                _restore_write(tmp)
                shutil.rmtree(tmp)
    finally:
        with contextlib.suppress(FileNotFoundError):
            os.unlink(download)


class ArchivePins(typing.NamedTuple):
    """What tools.json pins from an archive's tree (S1.1c-4 plan check §2 S4): the sealed tree's hash and each
    listed executable's hash, by its path relative to the tree."""

    tree_sha256: str
    exec_sha256: Mapping[str, str]


def archive_pins(archive: str, strip: str, executables: Sequence[str]) -> ArchivePins:
    """The pins the installer would publish for `archive` (already checked against its sha256): extracted with the
    installer's own hostile-archive rules, sealed, then hashed (S1.1c-4 plan check §2 S4; review A-L1: one helper
    shared with _install_tool, so the canary's hashes cannot drift from the installer's).

    The tree is extracted into a directory of its own, sealed without the installer's stamp (tree_sha256 skips the
    root's .complete by name, so both hash the same tree), hashed, and removed whatever happens (review DQ-L5). A
    refused member raises _Rejected, naming it; a listed executable that is not a regular file raises OSError."""
    tmp = tempfile.mkdtemp(prefix="archive-pins.")
    try:
        _seal(tmp, _extract(archive, strip, tmp))
        return ArchivePins(
            manifest.tree_sha256(tmp),
            {rel: manifest.sha256_file(os.path.join(tmp, rel)) for rel in sorted(executables)},
        )
    finally:
        _restore_write(tmp)
        shutil.rmtree(tmp)


def _load(tools_json: str, platform: str) -> list[manifest.Tool]:
    try:
        return manifest.load(tools_json, platform)
    except manifest.ManifestError as e:
        raise InstallError(f"{e}: fix tools.json") from e


def install(
    tools_json: str, cache: str, platform: str, out: TextIO, *, fetch: Fetch = _fetch, rename: Rename = os.rename
) -> int:
    """Install every pinned tool for this platform that is not installed yet (design note §3 steps 1-6)."""
    tools = _load(tools_json, platform)
    os.makedirs(cache, exist_ok=True)
    for tool in tools:
        home = manifest.tool_home(cache, tool)
        if os.path.lexists(home):
            if not _stamped(home, tool.archive_sha256):
                raise _unfinished(home)
            state = "present"
        else:
            state = "installed" if _install_tool(tool, cache, home, fetch, rename) else "present"
        print(f"{tool.name} {tool.version}: {state}", file=out, flush=True)
    return 0


def exec(repo: str, cache: str, platform: str, args: Sequence[str], out: TextIO, *, execv: Execv = os.execv) -> int:
    """Verify the pinned interpreter, then become it, running gate/run.py (design note §3).

    The absolute path is also argv[0], so that run.py's guard sees the real executable on Linux too.
    """
    if sys.version_info < FLOOR:
        raise InstallError(FLOOR_FIX)
    python = next((t for t in _load(os.path.join(repo, "tools.json"), platform) if t.name == GATE_PYTHON), None)
    if python is None:
        raise InstallError(f"tools.json has no tool named {GATE_PYTHON!r}: fix tools.json")
    home = os.path.abspath(manifest.tool_home(cache, python))
    rel = os.path.normpath(os.path.join(python.path_dir, python.command))
    if rel not in python.exec_sha256:
        raise InstallError(f"{python.name}: {rel} is not listed in exec_sha256: fix tools.json")
    interpreter = os.path.join(home, rel)
    # Containment first: sha256_file refuses links, so a link leading out must still be named for what it is.
    if not os.path.realpath(interpreter).startswith(os.path.realpath(home) + os.sep):
        raise InstallError(f"{python.name}: {interpreter} is outside the cache: {REINSTALL}")
    for path, want in sorted(python.exec_sha256.items()):
        try:
            got = manifest.sha256_file(os.path.join(home, path))
        except OSError as e:
            raise InstallError(f"{python.name}: cannot read {path} in {home}: {e.strerror}: run make tools") from e
        if got != want:
            raise InstallError(f"{python.name}: sha256 of {path} differs from tools.json: {REINSTALL}")
    out.flush()
    try:
        execv(interpreter, [interpreter, "-I", "-B", "-X", "utf8", os.path.join(repo, "gate", "run.py"), *args])
    except OSError as e:
        raise InstallError(f"{python.name}: cannot start {interpreter}: {e}: {REINSTALL}") from e
    return 0  # reached only with an injected execv


def clean_tools(cache: str, out: TextIO) -> int:
    """Remove the cache, restoring owner write first: the installed trees are read-only (design note §3)."""
    if not os.path.lexists(cache):
        print(f"{cache}: nothing to remove", file=out, flush=True)
        return 0
    if os.path.islink(cache):
        raise InstallError(f"{cache} is a symlink: check VORSAR_CEP_TOOLS_CACHE, then remove what you want by hand")
    foreign = sorted(name for name in os.listdir(cache) if not _CACHE_ENTRY.fullmatch(name))
    if foreign:
        raise InstallError(
            f"{cache} holds {foreign[0]!r}, which no install wrote: "
            "check VORSAR_CEP_TOOLS_CACHE, then remove what you want by hand"
        )
    _restore_write(cache)
    shutil.rmtree(cache)
    print(f"{cache}: removed", file=out, flush=True)
    return 0


def _refuse(out: TextIO, reason: str) -> int:
    print(reason, file=out)
    print("GATE FAILED", file=out)
    print("exit: 2", file=out, flush=True)
    return 2


def _dispatch(
    argv: Sequence[str], repo: str, environ: Mapping[str, str], out: TextIO, fetch: Fetch, rename: Rename, execv: Execv
) -> int:
    cache = manifest.cache_root(environ)
    if list(argv) == ["clean-tools"]:
        return clean_tools(cache, out)
    if list(argv) == ["install"]:
        return install(
            os.path.join(repo, "tools.json"), cache, manifest.platform_key(), out, fetch=fetch, rename=rename
        )
    if list(argv[:2]) == ["exec", "--"]:
        return exec(repo, cache, manifest.platform_key(), argv[2:], out, execv=execv)
    raise InstallError(USAGE)


def main(
    argv: Sequence[str],
    *,
    repo: str = REPO,
    environ: Mapping[str, str] = os.environ,
    out: TextIO | None = None,
    fetch: Fetch = _fetch,
    rename: Rename = os.rename,
    execv: Execv = os.execv,
) -> int:
    """Dispatch bootstrap.py's command line. Every failure ends in `exit: 2`, never in a traceback."""
    stream = sys.stdout if out is None else out
    try:
        return _dispatch(argv, repo, environ, stream, fetch, rename, execv)
    except (InstallError, manifest.ManifestError) as e:
        return _refuse(stream, str(e))
    except KeyboardInterrupt:
        return _refuse(stream, "interrupted: run make tools")
    except Exception as e:
        return _refuse(stream, f"internal error: {type(e).__name__}: {e}")
