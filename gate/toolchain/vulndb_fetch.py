"""`make vulndb`: fetch a copy of the Go vulnerability database for the vulns row (S1.1c-4 design note §2.3; owner
2026-11-13, S1.1c-4's plan check, decision 2). The network edge, beside the installer; it never gives a gate verdict.

One GET of vulndb.CANONICAL_URL with the installer's read timeout, streamed into a temporary file in the copy's
directory and stopped past vulndb.FETCH_CAP; validated (a zip whose index/db.json is {"modified": ...}) before it
replaces anything; then vulndb.zip and stamp.json each by os.replace, and fetch-error.txt removed. On any failure the
earlier zip and stamp stay exactly as they were and fetch-error.txt names the failure, for the row to read.
"""

import contextlib
import hashlib
import http.client
import json
import os
import tempfile
import zipfile
import zlib
from collections.abc import Callable, Mapping, Sequence
from datetime import UTC, datetime
from typing import BinaryIO

from gate.runner import vulndb

Get = Callable[[str, BinaryIO], None]  # install._fetch's shape: stream the URL's body into the sink
ZIP = "vulndb.zip"
STAMP = "stamp.json"
ERROR = "fetch-error.txt"
OVERRIDE = "VORSAR_CEP_VULNDB"
_FAILURES = (OSError, ValueError, EOFError, zipfile.BadZipFile, zlib.error, http.client.HTTPException)


class FetchError(Exception):
    """A fetch that installed nothing: its first line is written to fetch-error.txt."""


class _Capped:
    """A sink that refuses the write that would take it past FETCH_CAP bytes."""

    def __init__(self, sink: BinaryIO) -> None:
        self.sink = sink
        self.size = 0

    def write(self, data: bytes) -> int:
        self.size += len(data)
        if self.size > vulndb.FETCH_CAP:
            raise FetchError(f"the body is larger than {vulndb.FETCH_CAP >> 20} MiB")
        return self.sink.write(data)


def copy_root(environ: Mapping[str, str]) -> str:
    """VORSAR_CEP_VULNDB if set (absolute), else $HOME/.cache/vorsar/cep/vulndb (manifest.cache_root's shape)."""
    override = environ.get(OVERRIDE)
    if override:
        return os.path.abspath(override)
    home = environ.get("HOME") or os.path.expanduser("~")
    return os.path.join(home, ".cache", "vorsar", "cep", "vulndb")


def _utc(moment: datetime) -> str:
    return moment.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")


def _write_json(root: str, name: str, value: object) -> None:
    """`value` as JSON at root/name, by a temporary file in the same directory and os.replace."""
    fd, tmp = tempfile.mkstemp(dir=root, prefix=f".{name}.", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            f.write(json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n")
        os.replace(tmp, os.path.join(root, name))
    except BaseException:
        with contextlib.suppress(OSError):
            os.unlink(tmp)
        raise


def _validated(path: str) -> str:
    """index/db.json's `modified` from the downloaded zip; FetchError when the body is not the database."""
    if not zipfile.is_zipfile(path):
        raise FetchError("the body is not a zip")
    with zipfile.ZipFile(path) as z:
        try:
            with z.open(vulndb.DB_JSON) as member:
                data = member.read(vulndb.MEMBER_CAP + 1)
        except KeyError:
            data = b""
    try:
        parsed = json.loads(data.decode("utf-8")) if data else None
    except ValueError:
        parsed = None
    if not isinstance(parsed, dict) or not isinstance(parsed.get("modified"), str):
        raise FetchError(f"{vulndb.DB_JSON} is missing or malformed")
    return parsed["modified"]


def _first_line(error: BaseException) -> str:
    lines = str(error).strip().splitlines()
    return lines[0] if lines else type(error).__name__


def fetch(root: str, *, get: Get, now: Callable[[], datetime]) -> str:
    """Fetch, validate and install the copy under `root`; return the line to print. Raises FetchError after writing
    fetch-error.txt, leaving any earlier zip and stamp exactly as they were (design §2.3)."""
    os.makedirs(root, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=root, prefix=f".{ZIP}.", suffix=".tmp")
    try:
        try:
            with os.fdopen(fd, "wb") as f:
                get(vulndb.CANONICAL_URL, _Capped(f))
        except _FAILURES as e:
            raise FetchError(f"{vulndb.CANONICAL_URL}: {_first_line(e)}") from e
        modified = _validated(tmp)
        with open(tmp, "rb") as f:
            body = f.read()
        digest = hashlib.sha256(body).hexdigest()
        os.replace(tmp, os.path.join(root, ZIP))
    except (FetchError, *_FAILURES) as e:
        with contextlib.suppress(OSError):
            os.unlink(tmp)
        error = _first_line(e)
        _write_json(root, ERROR, {"at": _utc(now()), "error": error})
        raise FetchError(error) from e
    _write_json(root, STAMP, {"fetched": _utc(now()), "sha256": digest, "url": vulndb.CANONICAL_URL})
    with contextlib.suppress(FileNotFoundError):
        os.unlink(os.path.join(root, ERROR))
    return f"vulndb: fetched {len(body)} bytes, sha256 {digest[:12]}, modified {modified}"


def main(argv: Sequence[str], environ: Mapping[str, str]) -> int:
    """0 after a fetch, 2 after a FetchError (its text printed as `vulndb: <error>`)."""
    from gate.toolchain import install  # the installer's GET and its read timeout (DOWNLOAD_TIMEOUT_S)

    if argv:
        print(f"vulndb: usage: fetch-vulndb takes no arguments, got {' '.join(argv)}", flush=True)
        return 2
    try:
        line = fetch(copy_root(environ), get=install._fetch, now=lambda: datetime.now(UTC))
    except FetchError as e:
        print(f"vulndb: {e}", flush=True)
        return 2
    print(line, flush=True)
    return 0
