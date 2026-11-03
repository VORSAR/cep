"""tools.json: load, validate and locate the pinned tools, and compute the digests it pins (design note §3).

Runs under Python 3.12+: the installer imports it before the pinned interpreter exists.
"""

import concurrent.futures
import errno
import hashlib
import json
import os
import platform as host
import re
import stat
import sys
import types
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from typing import Any

SUPPORTED_PLATFORMS = ("darwin-arm64", "linux-amd64")

_PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
_DIGEST = re.compile(r"[0-9a-f]{64}")
_FILE_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9.+_-]*")
_TOP_FIELDS = frozenset({"schema", "tools"})
_TOOL_FIELDS = frozenset({"command", "name", "path_dir", "platforms", "version", "version_argv", "version_line"})
_ENTRY_FIELDS = frozenset({"archive_sha256", "exec_sha256", "strip", "tree_sha256", "url"})


class ManifestError(Exception):
    """tools.json is missing, malformed, or has no entry for this platform."""


@dataclass(frozen=True)
class Tool:
    """One pinned tool, resolved for the current platform."""

    name: str
    version: str
    command: str
    path_dir: str
    version_argv: tuple[str, ...]
    version_line: str
    url: str
    archive_sha256: str
    strip: str
    exec_sha256: Mapping[str, str]
    tree_sha256: str


def _platform_key(sys_platform: str, machine: str) -> str:
    key = _PLATFORMS.get((sys_platform, machine))
    if key is None:
        supported = " and ".join(SUPPORTED_PLATFORMS)
        raise ManifestError(f"unsupported platform {sys_platform}/{machine}: the gate runs on {supported}")
    return key


def platform_key() -> str:
    """Return "darwin-arm64" or "linux-amd64"; raise ManifestError for anything else."""
    return _platform_key(sys.platform, host.machine())


def cache_root(environ: Mapping[str, str]) -> str:
    """VORSAR_CEP_TOOLS_CACHE if set, else ~/.cache/vorsar/cep/tools (XDG is not consulted)."""
    override = environ.get("VORSAR_CEP_TOOLS_CACHE")
    if override:
        return os.path.abspath(override)
    home = environ.get("HOME") or os.path.expanduser("~")
    return os.path.join(home, ".cache", "vorsar", "cep", "tools")


def _refuse_duplicates(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ManifestError(f"duplicate key {key!r}")
        result[key] = value
    return result


def _fields(value: Any, want: frozenset[str], where: str) -> dict[str, Any]:
    """Every field present, no field unknown."""
    if type(value) is not dict:
        raise ManifestError(f"{where}: expected an object")
    missing = sorted(want - value.keys())
    if missing:
        raise ManifestError(f"{where}: field {missing[0]!r} is missing")
    unknown = sorted(value.keys() - want)
    if unknown:
        raise ManifestError(f"{where}: unknown field {unknown[0]!r}")
    return value


def _text(value: Any, field: str, where: str) -> str:
    if type(value) is not str or not value:
        raise ManifestError(f"{where}: field {field!r} must be a non-empty string")
    return value


def _file_name(value: Any, field: str, where: str) -> str:
    if not _FILE_NAME.fullmatch(_text(value, field, where)):
        raise ManifestError(f"{where}: field {field!r} must be one safe path component, not {value!r}")
    return value


def _digest(value: Any, field: str, where: str) -> str:
    if type(value) is not str or not _DIGEST.fullmatch(value):
        raise ManifestError(f"{where}: field {field!r} must be 64 lowercase hex digits, not {value!r}")
    return value


def _relative_path(value: Any, field: str, where: str) -> str:
    """A path that stays inside the tool's directory: relative, with no empty or `..` component."""
    parts = _text(value, field, where).split("/")
    if "" in parts or ".." in parts:
        raise ManifestError(f"{where}: field {field!r} must stay inside the tool directory, not {value!r}")
    return value


def _strip_prefix(value: Any, where: str) -> str:
    text = _text(value, "strip", where)
    if not text.endswith("/"):
        raise ManifestError(f"{where}: field 'strip' must end with '/', not {value!r}")
    _relative_path(text[:-1], "strip", where)
    return text


def _argv(value: Any, where: str) -> tuple[str, ...]:
    if type(value) is not list or not value:
        raise ManifestError(f"{where}: field 'version_argv' must be a non-empty list")
    _relative_path(value[0], "version_argv", where)
    return tuple(_text(arg, "version_argv", where) for arg in value)


def _exec_map(value: Any, where: str) -> Mapping[str, str]:
    if type(value) is not dict:
        raise ManifestError(f"{where}: field 'exec_sha256' must be an object")
    if not value:
        raise ManifestError(f"{where}: field 'exec_sha256' is empty: parity would verify nothing")
    checked = {_relative_path(k, "exec_sha256", where): _digest(v, "exec_sha256", where) for k, v in value.items()}
    return types.MappingProxyType(dict(sorted(checked.items())))


def _entry(value: Any, where: str) -> dict[str, Any]:
    fields = _fields(value, _ENTRY_FIELDS, where)
    return {
        "url": _text(fields["url"], "url", where),
        "archive_sha256": _digest(fields["archive_sha256"], "archive_sha256", where),
        "strip": _strip_prefix(fields["strip"], where),
        "exec_sha256": _exec_map(fields["exec_sha256"], where),
        "tree_sha256": _digest(fields["tree_sha256"], "tree_sha256", where),
    }


def _tool(value: Any, platform: str, where: str) -> Tool:
    fields = _fields(value, _TOOL_FIELDS, where)
    name = _file_name(fields["name"], "name", where)
    where = f"{where} ({name})"
    platforms = fields["platforms"]
    if type(platforms) is not dict or not platforms:
        raise ManifestError(f"{where}: field 'platforms' must be a non-empty object")
    unknown = sorted(platforms.keys() - set(SUPPORTED_PLATFORMS))
    if unknown:
        raise ManifestError(f"{where}: unknown platform {unknown[0]!r}")
    entries = {key: _entry(entry, f"{where} {key}") for key, entry in sorted(platforms.items())}
    if platform not in entries:
        raise ManifestError(f"{where}: no entry for platform {platform}")
    return Tool(
        name=name,
        version=_file_name(fields["version"], "version", where),
        command=_file_name(fields["command"], "command", where),
        path_dir=_relative_path(fields["path_dir"], "path_dir", where),
        version_argv=_argv(fields["version_argv"], where),
        version_line=_text(fields["version_line"], "version_line", where),
        **entries[platform],
    )


def load(path: str, platform: str) -> list[Tool]:
    """Load and validate tools.json for one platform, in file order (design note §3, load validation)."""
    try:
        with open(path, "rb") as f:
            doc = json.loads(f.read().decode("utf-8"), object_pairs_hook=_refuse_duplicates)
    except (OSError, ValueError, ManifestError) as e:
        raise ManifestError(f"{path}: {e}") from e
    top = _fields(doc, _TOP_FIELDS, path)
    if type(top["schema"]) is not int or top["schema"] != 1:
        raise ManifestError(f"{path}: field 'schema' must be the integer 1, not {top['schema']!r}")
    if type(top["tools"]) is not list or not top["tools"]:
        raise ManifestError(f"{path}: field 'tools' must be a non-empty list")
    tools = [_tool(value, platform, f"{path}: tools[{i}]") for i, value in enumerate(top["tools"])]
    names = [tool.name for tool in tools]
    twice = sorted({name for name in names if names.count(name) > 1})
    if twice:
        raise ManifestError(f"{path}: tool {twice[0]!r} is listed twice")
    return tools


def tool_home(cache: str, tool: Tool) -> str:
    """<cache>/<name>-<version>-<archive_sha256[:12]>."""
    return os.path.join(cache, f"{tool.name}-{tool.version}-{tool.archive_sha256[:12]}")


def path_dirs(cache: str, tools: list[Tool]) -> list[str]:
    """The PATH prefix: each tool's home joined with its path_dir, in tools.json order."""
    return [os.path.normpath(os.path.join(tool_home(cache, tool), tool.path_dir)) for tool in tools]


def sha256_file(path: str | bytes) -> str:
    """The hex sha256 of one regular file's content.

    Opened without blocking and without following a link, then refused unless it is a regular file: a plain open()
    of a FIFO planted at a hashed path would block for ever, hanging parity and `bootstrap.py exec` (break-it QB3).
    """
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise OSError(errno.EINVAL, "not a regular file", os.fsdecode(path))
        with os.fdopen(fd, "rb") as f:
            fd = -1  # the file object owns it now
            return hashlib.file_digest(f, "sha256").hexdigest()
    finally:
        if fd != -1:
            os.close(fd)


def _default_workers() -> int:
    """os.process_cpu_count() where it exists (3.13+); 3.12, the installer's floor, has only os.cpu_count()."""
    count = getattr(os, "process_cpu_count", os.cpu_count)()
    return count or 1


def tree_sha256(root: str, *, workers: int | None = None) -> str:
    """The digest tools.json pins for an installed tree (design note §3, whole-tree digests).

    The sha256 of the lines `<kind> <path> <payload>\\n` for every entry below root except the root's
    `.complete`, sorted bytewise by the whole path. Kind is d, f, x (a file with any execute bit) or l;
    the payload is `-`, the content's sha256 or the link target. Nothing is followed. Paths stay bytes, so
    every name hashes as stored. File contents are hashed on `workers` threads (owner, 2026-11-02); the
    digest and any error are those of the serial walk.
    """
    return _tree_sha256(root, _default_workers() if workers is None else workers, sha256_file)


def _tree_sha256(root: str, workers: int, hash_file: Callable[[bytes], str]) -> str:
    """Walk serially, hash in parallel, then assemble the lines in walk order and sort them.

    The serial code hashed each file as the walk met it, so its first error was whichever came first in
    walk order. Here the walk stops at its first error and only the files met before it are hashed; the
    results come back in walk order, so a hash error, being earlier, still wins over the walk's error.
    """
    lines: list[tuple[bytes, bytes, bytes | None]] = []  # path, kind, payload (None: a file to hash)
    files: list[bytes] = []
    walk_error: OSError | None = None
    pending = [(os.fsencode(root), b"")]
    try:
        while pending:
            directory, prefix = pending.pop()
            with os.scandir(directory) as listing:
                entries = list(listing)
            for entry in entries:
                rel = prefix + entry.name
                if rel == b".complete":
                    continue
                mode = entry.stat(follow_symlinks=False).st_mode
                if stat.S_ISLNK(mode):
                    lines.append((rel, b"l", os.readlink(entry.path)))
                elif stat.S_ISDIR(mode):
                    lines.append((rel, b"d", b"-"))
                    pending.append((entry.path, rel + b"/"))
                elif stat.S_ISREG(mode):
                    lines.append((rel, b"x" if mode & 0o111 else b"f", None))
                    files.append(entry.path)
                else:
                    lines.append((rel, b"?", b"-"))  # no archive holds one, so the digest cannot match
    except OSError as e:
        walk_error = e
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
        digests = iter(list(pool.map(hash_file, files)))  # in walk order; the first error in it is raised
    if walk_error is not None:
        raise walk_error
    # Each digest goes to its file while both are still in walk order; only then are the lines sorted.
    filled = [
        (rel, kind, next(digests).encode("ascii") if payload is None else payload) for rel, kind, payload in lines
    ]
    digest = hashlib.sha256()
    for rel, kind, payload in sorted(filled, key=lambda line: line[0]):
        digest.update(b"%b %b %b\n" % (kind, rel, payload))
    return digest.hexdigest()
