"""The canary's network edge (S1.1c-4 design §4.2), kept thin: one GET per feed, and the archive download that checks
the published digest. Everything it returns is parsed JSON or a local path; candidate.py does the reading.

`get(url, headers) -> (status, response_headers, body)` is the seam: `response_headers` an email.message.Message
(case-blind, as http.client gives them), `body` bytes. The token goes on GitHub requests only (review A-S5).
"""

import hashlib
import json
import os
import tempfile
import time
import urllib.error
import urllib.request
from collections.abc import Callable, Mapping
from email.message import Message

from gate.canary import candidate
from gate.canary.candidate import CanaryError, DigestMismatch
from gate.toolchain import install

GO_FEED = "https://go.dev/dl/?mode=json&include=all"  # include=all: a pinned line stays listed (review DA-L4)
GITHUB_API = "https://api.github.com/"
PER_PAGE = {"astral-sh/python-build-standalone": 10}  # its releases carry hundreds of assets each
PER_PAGE_DEFAULT = 30
USER_AGENT = "complex-events-engine-canary"

Get = Callable[[str, Mapping[str, str]], tuple[int, Message, bytes]]


def feed_url(tool: Mapping[str, object]) -> str:
    """The one feed a pin reads (design §4.2)."""
    kind = candidate.source(tool)
    if kind == candidate.GO:
        return GO_FEED
    if kind == candidate.PYPI:
        return f"https://pypi.org/pypi/{tool['module']}/json"
    repo = candidate.github_repo(tool)
    return f"{GITHUB_API}repos/{repo}/releases?per_page={PER_PAGE.get(repo, PER_PAGE_DEFAULT)}"


def _spent(headers: Message) -> str | None:
    """GitHub's rate-limit text when the answer says the limit is spent, else None."""
    if headers.get("x-ratelimit-remaining") != "0":
        return None
    reset = headers.get("x-ratelimit-reset", "")
    when = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(int(reset))) if reset.isdigit() else "an unstated time"
    return f"GitHub's rate limit ({headers.get('x-ratelimit-limit', '?')}/h) is spent until {when}"


def fetch(pinned: Mapping[str, object], *, token: str, get: Get) -> dict[str, object]:
    """Each tool's feed, parsed, keyed by its name in tools.json; one request per distinct feed (python and
    python-floor share python-build-standalone's). A non-200, a spent rate limit or a body that is not JSON raises
    CanaryError naming the first tool that reads the feed."""
    by_url: dict[str, list[str]] = {}
    for tool in pinned["tools"]:
        by_url.setdefault(feed_url(tool), []).append(str(tool["name"]))
    out: dict[str, object] = {}
    for url, names in by_url.items():
        headers = {"User-Agent": USER_AGENT, "Accept": "application/json"}
        if url.startswith(GITHUB_API):
            headers |= {"Accept": "application/vnd.github+json", "Authorization": f"Bearer {token}"}
        try:
            status, answer, body = get(url, headers)
        except OSError as e:
            raise CanaryError(f"{names[0]}: {url}: {e}") from e
        if status in (403, 429) and url.startswith(GITHUB_API):
            spent = _spent(answer)
            if spent is not None:
                raise CanaryError(f"{names[0]}: {spent}")
        if status != 200:
            raise CanaryError(f"{names[0]}: {url} answered {status}")
        try:
            document = json.loads(body.decode("utf-8", errors="replace"))
        except ValueError as e:
            raise CanaryError(f"{names[0]}: {url} is not JSON: {e}") from e
        for name in names:
            out[name] = document
    return out


def http_get(url: str, headers: Mapping[str, str]) -> tuple[int, Message, bytes]:
    """The production `get`: an error status is an answer, not an exception (its headers carry the rate limit)."""
    request = urllib.request.Request(url, headers=dict(headers))
    try:
        with urllib.request.urlopen(request, timeout=install.DOWNLOAD_TIMEOUT_S) as response:
            return response.status, response.headers, response.read()
    except urllib.error.HTTPError as e:
        with e:
            return e.code, e.headers, e.read()


def downloader(directory: str, fetch_into: install.Fetch = install._fetch) -> Callable[[str, str], str]:
    """The production `archive(url, sha256)` seam: download into `directory` (the installer's fetch, its read
    timeout) and return the path once its sha256 is the published one; otherwise remove it and raise
    DigestMismatch, which candidate.build names with the tool and platform (design §4.3)."""

    def archive(url: str, sha256: str) -> str:
        fd, path = tempfile.mkstemp(prefix="archive.", suffix=".tar.gz", dir=directory)
        try:
            with os.fdopen(fd, "wb") as sink:
                fetch_into(url, sink)
            digest = hashlib.sha256()
            with open(path, "rb") as f:
                for chunk in iter(lambda: f.read(1 << 20), b""):
                    digest.update(chunk)
            if digest.hexdigest() != sha256:
                raise DigestMismatch(f"{url}: sha256 {digest.hexdigest()} is not the published {sha256}")
        except BaseException:
            os.unlink(path)
            raise
        return path

    return archive
