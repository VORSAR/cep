"""`make vulndb`: fetch a copy of the Go vulnerability database for the vulns row (S1.1c-4 design note §2.3; owner
2026-11-13, S1.1c-4's plan check, decision 2). The network edge, beside the installer; it never gives a gate verdict.

One GET of vulndb.CANONICAL_URL with the installer's read timeout, every redirect hop kept on https (owner
2026-11-13, S1.1c-4's review and break-it, decision 3), streamed into a temporary file in the copy's directory and
stopped past vulndb.FETCH_CAP; validated before it replaces anything: every member read (ZipFile.testzip) and the
row's own format check, vulndb.check_format (reviews CA-B1, CA-S2); then vulndb.zip and stamp.json each by
os.replace, the earlier zip kept as a hard link until the stamp is written and put back if it is not (reviews BK-S2,
CA-L8), and fetch-error.txt removed. On any failure the earlier zip and stamp stay exactly as they were, no temporary
file is left, and fetch-error.txt names the failure, for the row to read; an interrupt restores them too.
"""

import contextlib
import hashlib
import http.client
import json
import os
import shutil
import tempfile
import urllib.parse
import urllib.request
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


class Redirected(Exception):
    """A redirect hop off https: the database carries no digest, so a downgraded hop could serve a doctored copy
    (owner 2026-11-13, S1.1c-4's review and break-it, decision 3)."""

    def __init__(self, url: str, to: str) -> None:
        super().__init__(f"{url} redirected to a non-https URL {to}: refused")


class _HttpsOnly(urllib.request.HTTPRedirectHandler):
    """urllib's redirect handler, refusing any hop whose target is not https."""

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        if urllib.parse.urlsplit(newurl).scheme != "https":
            with contextlib.suppress(Exception):
                fp.close()
            raise Redirected(req.full_url, newurl)
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def _opener() -> urllib.request.OpenerDirector:
    return urllib.request.build_opener(_HttpsOnly)


def _get(url: str, sink: BinaryIO) -> None:
    """install._fetch's GET (its read timeout), every redirect hop on https."""
    from gate.toolchain import install  # the installer's read timeout and chunk (DOWNLOAD_TIMEOUT_S)

    with _opener().open(url, timeout=install.DOWNLOAD_TIMEOUT_S) as response:
        shutil.copyfileobj(response, sink, install._CHUNK)


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
    """index/db.json's `modified` from the downloaded zip; FetchError when the body is not the database: not a zip,
    a member that cannot be read, or a fault of the row's own format check (reviews CA-B1, CA-S2)."""
    if not zipfile.is_zipfile(path):
        raise FetchError("the body is not a zip")
    with zipfile.ZipFile(path) as z:
        names = sorted(set(z.namelist()))
        faults = vulndb.check_format(lambda name: vulndb.read_member(z, name), names)
        if faults:
            more = f" (and {len(faults) - 1} more)" if len(faults) > 1 else ""
            raise FetchError(f"the body is not the database: {faults[0].removeprefix('vulns: ')}{more}")
        try:
            bad = z.testzip()
        except vulndb._UNREADABLE as e:
            raise FetchError(f"the body is not the database: a member cannot be read: {_first_line(e)}") from e
        if bad is not None:
            raise FetchError(f"the body is not the database: member {bad} cannot be read: bad CRC-32")
        data = vulndb.read_member(z, vulndb.DB_JSON) or b""
    return json.loads(data.decode("utf-8"))["modified"]


def _first_line(error: BaseException) -> str:
    lines = str(error).strip().splitlines()
    return lines[0] if lines else type(error).__name__


def _kept(zip_path: str) -> str | None:
    """A hard link to the earlier zip beside it, None when there is none: put back if the stamp is not written."""
    if not os.path.exists(zip_path):
        return None
    kept = os.path.join(os.path.dirname(zip_path), f".{ZIP}.{os.urandom(8).hex()}.kept")
    os.link(zip_path, kept)
    return kept


def fetch(root: str, *, get: Get, now: Callable[[], datetime]) -> str:
    """Fetch, validate and install the copy under `root`; return the line to print. Raises FetchError after writing
    fetch-error.txt, leaving any earlier zip and stamp exactly as they were, a failed stamp write included (design
    §2.3; reviews BK-S2, CA-L8); an interrupt (BaseException) restores them too and propagates."""
    os.makedirs(root, exist_ok=True)
    zip_path = os.path.join(root, ZIP)
    kept: str | None = None
    replaced = committed = False
    fd, tmp = tempfile.mkstemp(dir=root, prefix=f".{ZIP}.", suffix=".tmp")
    try:
        try:
            with os.fdopen(fd, "wb") as f:
                get(vulndb.CANONICAL_URL, _Capped(f))
        except Redirected as e:
            raise FetchError(str(e)) from e
        except _FAILURES as e:
            raise FetchError(f"{vulndb.CANONICAL_URL}: {_first_line(e)}") from e
        modified = _validated(tmp)
        with open(tmp, "rb") as f:
            body = f.read()
        digest = hashlib.sha256(body).hexdigest()
        kept = _kept(zip_path)
        os.replace(tmp, zip_path)
        replaced = True
        _write_json(root, STAMP, {"fetched": _utc(now()), "sha256": digest, "url": vulndb.CANONICAL_URL})
        committed = True
    except BaseException as e:
        with contextlib.suppress(OSError):
            os.unlink(tmp)
        if replaced:  # the stamp was not written: the earlier zip back, or none as before
            if kept is not None:
                os.replace(kept, zip_path)
                kept = None
            else:
                os.unlink(zip_path)
        if not isinstance(e, (FetchError, *_FAILURES)):
            raise
        error = _first_line(e)
        _write_json(root, ERROR, {"at": _utc(now()), "error": error})
        raise FetchError(error) from e
    finally:
        if kept is not None:
            with contextlib.suppress(OSError):
                os.unlink(kept)
    if committed:
        with contextlib.suppress(FileNotFoundError):
            os.unlink(os.path.join(root, ERROR))
    return f"vulndb: fetched {len(body)} bytes, sha256 {digest[:12]}, modified {modified}"


def main(argv: Sequence[str], environ: Mapping[str, str]) -> int:
    """0 after a fetch, 2 after a FetchError (its text printed as `vulndb: <error>`)."""
    if argv:
        print(f"vulndb: usage: fetch-vulndb takes no arguments, got {' '.join(argv)}", flush=True)
        return 2
    try:
        line = fetch(copy_root(environ), get=_get, now=lambda: datetime.now(UTC))
    except FetchError as e:
        print(f"vulndb: {e}", flush=True)
        return 2
    print(line, flush=True)
    return 0
