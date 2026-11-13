"""The weekly canary's candidate tools.json (S1.1c-4 plan check §2 S4; design note P4; owner 2026-11-12, S1.1c-3's
plan check, decision 6, and 2026-11-13, S1.1c-4's plan check, confirm (c)). Pure over recorded or live feeds.

W0 stub: the signatures are agreed and the bodies raise; lane F1 builds them against QA's tests.
"""

from collections.abc import Callable, Mapping
from dataclasses import dataclass


class CanaryError(Exception):
    """A feed or an archive the canary cannot use, naming the tool and the reason (a rate limit, a missing or
    wrong digest, a version older than the pin)."""


@dataclass(frozen=True)
class Upgrade:
    name: str  # the tool's name in tools.json
    pinned: str
    latest: str  # equal to `pinned` for a build-only move
    note: str = ""  # e.g. "build 20261026 → 20261110" for python-build-standalone (design §4.3; review DQ-S7)


@dataclass(frozen=True)
class Candidate:
    tools: Mapping[str, object]  # the candidate tools.json document, valid for manifest.load
    upgrades: tuple[Upgrade, ...]  # in tools.json's order; empty when every pin is the latest
    newer_lines: tuple[str, ...] = ()  # releases on a newer line, listed and never tried, e.g. "golangci-lint v2.16.0"


def build(
    pinned: Mapping[str, object],
    feeds: Mapping[str, object],
    archive: Callable[[str, str], str],
) -> Candidate:
    """For each pin, the latest stable release on the pin's own line (never a pre-release, never a downgrade),
    its archive fetched by `archive(url, sha256)` (a local path, the digest already checked) and hashed by
    install.archive_pins. `pinned` is tools.json as parsed; `feeds` maps a tool name to its feed's parsed JSON."""
    raise NotImplementedError("candidate.build: not built yet (S1.1c-4 lane F1)")


def structural_faults(
    pinned: Mapping[str, object], candidate: Mapping[str, object], upgraded: frozenset[str]
) -> list[str]:
    """Every difference between the two tools.json documents other than version, url, the digests and
    version_line of an upgraded tool, one named line each (review QA-S9): a pin red counts as "pins to update"
    only when this is []."""
    raise NotImplementedError("candidate.structural_faults: not built yet (S1.1c-4 lane F1)")
