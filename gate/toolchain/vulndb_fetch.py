"""`make vulndb`: fetch a copy of the Go vulnerability database for the vulns row (S1.1c-4 design note §2.3; owner
2026-11-13, S1.1c-4's plan check, decision 2). The network edge, beside the installer; it never gives a gate verdict.

W0 stub: the signatures are agreed and the bodies raise; lane E1 builds them against QA's tests (review DQ-S5).
"""

from collections.abc import Callable, Mapping, Sequence
from datetime import datetime
from typing import BinaryIO

Get = Callable[[str, BinaryIO], None]  # install._fetch's shape: stream the URL's body into the sink


def copy_root(environ: Mapping[str, str]) -> str:
    """VORSAR_CEP_VULNDB if set (absolute), else $HOME/.cache/vorsar/cep/vulndb (manifest.cache_root's shape)."""
    raise NotImplementedError("vulndb_fetch.copy_root: not built yet (S1.1c-4 lane E1)")


def fetch(root: str, *, get: Get, now: Callable[[], datetime]) -> str:
    """Fetch, validate and install the copy under `root`; return the line to print. Raises FetchError after writing
    fetch-error.txt, leaving any earlier zip and stamp exactly as they were (design §2.3)."""
    raise NotImplementedError("vulndb_fetch.fetch: not built yet (S1.1c-4 lane E1)")


class FetchError(Exception):
    """A fetch that installed nothing: its first line is written to fetch-error.txt."""


def main(argv: Sequence[str], environ: Mapping[str, str]) -> int:
    """0 after a fetch, 2 after a FetchError (its text printed as `vulndb: <error>`)."""
    raise NotImplementedError("vulndb_fetch.main: not built yet (S1.1c-4 lane E1)")
