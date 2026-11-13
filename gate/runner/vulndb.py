"""The vulnerability scan's rule over a copy of the Go vulnerability database (S1.1c-4 plan check §2 S1; design
note P1; owner 2026-11-13, S1.1c-4's plan check, decisions 1 and 2). Pure: it reads only what it is given.

W0 stub: the signatures are agreed and the bodies raise; lane E1 builds them against QA's tests.
"""

from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass
from datetime import datetime

CANONICAL_URL = "https://vuln.go.dev/vulndb.zip"
COPY_MAX_DAYS = 7  # decision 2: the copy fetched at most this many UTC calendar days ago
SOURCE_MAX_DAYS = 30  # decision 2: the database's own `modified` at most this many UTC calendar days old


class InvalidVersion(ValueError):
    """A version the matcher cannot order: a named red, never read as the lowest version (plan check §2 S1)."""


@dataclass(frozen=True)
class Finding:
    id: str  # the entry's id, e.g. GO-2026-6860
    module: str  # a module path, "stdlib" or "toolchain"
    version: str  # the version read, as the build list or GOVERSION gave it
    fixed: str  # the first fixed version after `version`, "" when none
    built: bool  # False when the register marks the module never built: named in the reason, never red (decision 1)


def check_db(
    read: Callable[[str], bytes | None],
    stamp: Mapping[str, object] | None,
    *,
    zip_sha256: str,
    now: datetime,
) -> list[str]:
    """Every fault of the copy and of the database, one named line each; [] when the copy is usable.

    `read(name)` returns a zip member's bytes, None when it is absent; `stamp` is the parsed stamp.json (None when
    the file is missing); `zip_sha256` is the zip's own digest; `now` is timezone-aware UTC."""
    raise NotImplementedError("vulndb.check_db: not built yet (S1.1c-4 lane E1)")


def affected(version: str, ranges: Sequence[Mapping[str, object]]) -> bool:
    """Whether `version` lies in an `affected` block's `ranges` (OSV SEMVER events; "0" is minus infinity; no
    ranges, or no SEMVER range, means every version). Raises InvalidVersion for a version it cannot order."""
    raise NotImplementedError("vulndb.affected: not built yet (S1.1c-4 lane E1)")


def findings(
    modules: Mapping[str, str],
    never_built: frozenset[str],
    go_version: str,
    entries: Iterable[Mapping[str, object]],
) -> list[Finding]:
    """Every entry affecting a module of `modules` (path -> version as the build list gives it, a `replace`
    already applied), `stdlib` and `toolchain` at `go_version` (`go env GOVERSION`, e.g. go1.27.4); withdrawn
    entries skipped; a module in `never_built` gives findings with built=False; sorted by (module, id)."""
    raise NotImplementedError("vulndb.findings: not built yet (S1.1c-4 lane E1)")
