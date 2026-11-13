"""The weekly canary's candidate tools.json (S1.1c-4 plan check §2 S4; design note §4.3, §4.4; owner 2026-11-12,
S1.1c-3's plan check, decision 6, and 2026-11-13, S1.1c-4's plan check, confirm (c), and its design, G1-Q1). Pure over
recorded or live feeds: the network is feeds.py's, the archives come through the injected `archive` seam.

The line rule (G1-Q1): each pin moves only to the newest stable release on its own line (major.minor); a release on a
newer line is listed in Candidate.newer_lines and never downloaded.

Which feed a tool reads follows from its pinned url: go.dev's archive listing, a GitHub release download
(python-build-standalone's carries the CPython version and a build tag), or PyPI's file store.
"""

import copy
import json
import re
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass

from gate.toolchain import install

GO = "go"
GITHUB = "github"
PBS = "python-build-standalone"
PYPI = "pypi"

_GITHUB_DOWNLOAD = re.compile(r"https://github\.com/([^/]+/[^/]+)/releases/download/([^/]+)/")
_PBS_REPO = "astral-sh/python-build-standalone"
_STABLE = re.compile(r"(?:v|go)?([0-9]+)\.([0-9]+)(?:\.([0-9]+))?")
_CPYTHON = re.compile(r"[0-9]+\.[0-9]+\.[0-9]+")
_HEX64 = re.compile(r"[0-9a-f]{64}")
_ABSENT = object()
# python-floor's minor is the bootstrap's floor on purpose (install.FLOOR): a newer CPython minor is never its
# candidate and never listed; its patch and build-tag moves within its line are upgrades (design §4.3, W0c point 8).
NO_NEWER_LINES = frozenset({"python-floor"})


class CanaryError(Exception):
    """A feed or an archive the canary cannot use, naming the tool and the reason (a rate limit, a missing or
    wrong digest, a version older than the pin)."""


class DigestMismatch(CanaryError):
    """Raised by a real `archive` seam whose download's sha256 is not the published one; build names the tool and
    the platform (design §4.3)."""


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


@dataclass(frozen=True)
class _Offer:
    """One stable release a feed offers for a tool: its version as the tool writes it, the version's numbers, the
    build tag (python-build-standalone's; "" elsewhere) and the feed item it came from."""

    text: str
    key: tuple[int, ...]
    build: str
    item: Mapping[str, object]

    def order(self) -> tuple[tuple[int, ...], str]:
        return self.key, self.build


def source(tool: Mapping[str, object]) -> str:
    """Which feed a tool's pin reads, from its first platform's url (design §4.2)."""
    url = str(_first_entry(tool)["url"])
    if url.startswith("https://go.dev/dl/"):
        return GO
    if url.startswith("https://files.pythonhosted.org/"):
        return PYPI
    match = _GITHUB_DOWNLOAD.match(url)
    if match is None:
        raise CanaryError(f"{tool['name']}: no feed is known for {url}")
    return PBS if match.group(1) == _PBS_REPO else GITHUB


def github_repo(tool: Mapping[str, object]) -> str:
    """owner/repo of a GitHub-hosted pin."""
    match = _GITHUB_DOWNLOAD.match(str(_first_entry(tool)["url"]))
    if match is None:
        raise CanaryError(f"{tool['name']}: its url is not a GitHub release download")
    return match.group(1)


def _first_entry(tool: Mapping[str, object]) -> Mapping[str, object]:
    platforms = tool["platforms"]
    return platforms[sorted(platforms)[0]]


def _build_tag(url: str) -> str:
    match = _GITHUB_DOWNLOAD.match(url)
    return match.group(2) if match else ""


def _numbers(text: str) -> tuple[int, ...] | None:
    """A stable release's numbers (go1.27.4, v2.15.0, 0.17.5, 3.15.2), or None for a pre-release or anything else."""
    match = _STABLE.fullmatch(text)
    if match is None:
        return None
    major, minor, patch = match.groups()
    return int(major), int(minor), int(patch or 0)


def _forms(old: str, new: str) -> list[tuple[str, str]]:
    """The version's written forms, longest first: with its leading `v` or `go` and without (review DQ-S7)."""
    if old == new:
        return []
    for prefix in ("go", "v"):
        if old.startswith(prefix) and new.startswith(prefix):
            return [(old, new), (old[len(prefix) :], new[len(prefix) :])]
    return [(old, new)]


def _pairs(kind: str, old_version: str, new_version: str, old_url: str, new_url: str) -> list[tuple[str, str]]:
    """What an upgrade substitutes in url, strip and version_line: the version's forms, and for
    python-build-standalone the build tag, which sits twice in its url (design §4.3)."""
    pairs = _forms(old_version, new_version)
    if kind == PBS and _build_tag(old_url) != _build_tag(new_url):
        pairs.append((_build_tag(old_url), _build_tag(new_url)))
    return pairs


def substitute(text: str, pairs: Sequence[tuple[str, str]]) -> str:
    """Replace each old form by its new one in one pass, never inside a longer number (1.3.1 is not in 1.3.10)."""
    table = dict(pairs)
    if not table:
        return text
    alternatives = "|".join(re.escape(old) for old in sorted(table, key=len, reverse=True))
    pattern = re.compile(rf"(?<![0-9.])(?:{alternatives})(?![0-9])")
    return pattern.sub(lambda m: table[m.group(0)], text)


def _offers(tool: Mapping[str, object], kind: str, feed: object) -> list[_Offer]:
    """Every stable release the feed offers this tool, on any line (never a pre-release, a draft, an unstable go,
    a CPython rc or a yanked package)."""
    offers: list[_Offer] = []
    if kind == GO:
        for item in feed:
            numbers = _numbers(str(item.get("version", "")))
            if item.get("stable") is True and numbers is not None:
                offers.append(_Offer(item["version"], numbers, "", item))
    elif kind == GITHUB:
        for item in feed:
            numbers = _numbers(str(item.get("tag_name", "")))
            if not item.get("prerelease") and not item.get("draft") and numbers is not None:
                offers.append(_Offer(item["tag_name"], numbers, "", item))
    elif kind == PBS:
        template = _pbs_template(tool)
        for item in feed:
            if item.get("prerelease") or item.get("draft"):
                continue
            tag = str(item.get("tag_name", ""))
            for asset in item.get("assets", []):
                match = template.fullmatch(str(asset.get("browser_download_url", "")))
                if match is None or match.group("tag") != tag or not _CPYTHON.fullmatch(match.group("version")):
                    continue
                offers.append(_Offer(match.group("version"), _numbers(match.group("version")), tag, item))
    else:
        for version, files in feed.get("releases", {}).items():
            numbers = _numbers(version) if _CPYTHON.fullmatch(version) else None
            sdist = next((f for f in files if f.get("filename") == f"{tool['module']}-{version}.tar.gz"), None)
            if numbers is not None and sdist is not None and not sdist.get("yanked"):
                offers.append(_Offer(version, numbers, "", sdist))
    return offers


def _pbs_template(tool: Mapping[str, object]) -> re.Pattern[str]:
    """The pin's first platform url with its CPython version and build tag left open."""
    url = str(_first_entry(tool)["url"])
    tag, version = _build_tag(url), str(tool["version"])
    pattern = re.escape(url).replace(re.escape(version), "(?P<version>[0-9][0-9A-Za-z.]*)", 1)
    pattern = pattern.replace(re.escape(tag), "(?P<tag>[0-9]+)", 1)
    return re.compile(pattern.replace(re.escape(tag), "(?P=tag)"))


def _pin_order(tool: Mapping[str, object], kind: str) -> tuple[tuple[int, ...], str]:
    numbers = _numbers(str(tool["version"]))
    if numbers is None:
        raise CanaryError(f"{tool['name']}: the pin {tool['version']} is not a stable release")
    return numbers, _build_tag(str(_first_entry(tool)["url"])) if kind == PBS else ""


def _newer_lines(name: str, offers: Sequence[_Offer], line: tuple[int, ...]) -> list[str]:
    """The newest release of each line newer than the pin's, oldest line first."""
    newest: dict[tuple[int, ...], _Offer] = {}
    for offer in offers:
        held = newest.get(offer.key[:2])
        if offer.key[:2] > line and (held is None or offer.order() > held.order()):
            newest[offer.key[:2]] = offer
    return [f"{name} {newest[key].text}" for key in sorted(newest)]


def _digest(name: str, platform: str, kind: str, offer: _Offer, url: str) -> object:
    """The feed's published sha256 for this url, or None."""
    if kind == GO:
        file_name = url.rsplit("/", 1)[-1]
        found = next((f for f in offer.item.get("files", []) if f.get("filename") == file_name), None)
        if found is None:
            raise CanaryError(f"{name} {platform}: {offer.text} lists no file {file_name}")
        return found.get("sha256")
    found = next((a for a in offer.item.get("assets", []) if a.get("browser_download_url") == url), None)
    if found is None:
        raise CanaryError(f"{name} {platform}: release {offer.item.get('tag_name')} has no asset {url}")
    digest = found.get("digest")
    return digest.removeprefix("sha256:") if isinstance(digest, str) and digest.startswith("sha256:") else None


def _platform_entry(
    name: str,
    platform: str,
    pinned_entry: Mapping[str, object],
    kind: str,
    offer: _Offer,
    pairs: Sequence[tuple[str, str]],
    archive: Callable[[str, str], str],
) -> dict[str, object]:
    """One platform's pins for the offer: url and digest from the feed, the hashes from the archive (design §4.3)."""
    if kind == PYPI:  # a hash path no substitution can make: url and digest from the feed by file name (DQ-S7)
        url, digest = str(offer.item.get("url")), (offer.item.get("digests") or {}).get("sha256")
    else:
        url = substitute(str(pinned_entry["url"]), pairs)
        digest = _digest(name, platform, kind, offer, url)
    if not isinstance(digest, str) or not _HEX64.fullmatch(digest):
        raise CanaryError(f"{name} {platform}: no published digest")
    strip = substitute(str(pinned_entry["strip"]), pairs)
    try:
        path = archive(url, digest)
    except DigestMismatch as e:
        raise CanaryError(f"{name} {platform}: the download's sha256 is not the published one") from e
    except OSError as e:
        raise CanaryError(f"{name} {platform}: the download failed: {url}: {e}") from e
    try:
        pins = install.archive_pins(path, strip, sorted(pinned_entry["exec_sha256"]))
    except (install._Rejected, OSError) as e:
        raise CanaryError(f"{name} {platform}: {url}: {e}") from e
    return {
        **pinned_entry,
        "archive_sha256": digest,
        "exec_sha256": dict(pins.exec_sha256),
        "strip": strip,
        "tree_sha256": pins.tree_sha256,
        "url": url,
    }


def _upgraded(tool: Mapping[str, object], kind: str, offer: _Offer, archive: Callable[[str, str], str]) -> dict:
    pairs = _forms(str(tool["version"]), offer.text)
    pin_tag = _build_tag(str(_first_entry(tool)["url"]))
    if kind == PBS and pin_tag != offer.build:
        pairs.append((pin_tag, offer.build))
    entry = copy.deepcopy(dict(tool))
    entry["version"] = offer.text
    # A python package has no version_argv: its version is its version_line (review DQ-S7). For an executable this is
    # provisional: the workflow's build step replaces it by running version_argv on Linux (design §4.7).
    entry["version_line"] = offer.text if kind == PYPI else substitute(str(tool["version_line"]), pairs)
    entry["platforms"] = {
        platform: _platform_entry(str(tool["name"]), platform, tool["platforms"][platform], kind, offer, pairs, archive)
        for platform in sorted(tool["platforms"])
    }
    return entry


def build(
    pinned: Mapping[str, object],
    feeds: Mapping[str, object],
    archive: Callable[[str, str], str],
) -> Candidate:
    """For each pin, the latest stable release on the pin's own line (never a pre-release, never a downgrade),
    its archive fetched by `archive(url, sha256)` (a local path, the digest already checked) and hashed by
    install.archive_pins. `pinned` is tools.json as parsed; `feeds` maps a tool name to its feed's parsed JSON."""
    tools: list[object] = []
    upgrades: list[Upgrade] = []
    newer: list[str] = []
    for tool in pinned["tools"]:
        name = str(tool["name"])
        if name not in feeds:
            raise CanaryError(f"{name}: no feed was fetched")
        kind = source(tool)
        pin = _pin_order(tool, kind)
        offers = _offers(tool, kind, feeds[name])
        if name not in NO_NEWER_LINES:
            newer += _newer_lines(name, offers, pin[0][:2])
        on_line = [o for o in offers if o.key[:2] == pin[0][:2]]
        if not on_line:
            raise CanaryError(f"{name}: no stable release on its line {'.'.join(map(str, pin[0][:2]))}")
        latest = max(on_line, key=_Offer.order)
        if latest.order() < pin:
            built = f" (build {latest.build})" if kind == PBS else ""
            pinned_build = f" (build {pin[1]})" if kind == PBS else ""
            raise CanaryError(
                f"{name}: the newest on its line, {latest.text}{built}, is older than the pin "
                f"{tool['version']}{pinned_build}"
            )
        if latest.order() == pin:
            tools.append(copy.deepcopy(tool))
            continue
        tools.append(_upgraded(tool, kind, latest, archive))
        note = f"build {pin[1]} → {latest.build}" if kind == PBS else ""
        upgrades.append(Upgrade(name, str(tool["version"]), latest.text, note))
    return Candidate({**copy.deepcopy(dict(pinned)), "tools": tools}, tuple(upgrades), tuple(newer))


def _shown(value: object) -> str:
    if value is _ABSENT:
        return "(absent)"
    return value if isinstance(value, str) else json.dumps(value, sort_keys=True, ensure_ascii=False)


def _differences(path: tuple[str, ...], a: object, b: object) -> list[tuple[tuple[str, ...], object, object]]:
    """Every key path where a and b differ, recursing into objects only (a list is one value). A path is a tuple:
    an executable's key holds dots of its own (bin/python3.15)."""
    if isinstance(a, Mapping) and isinstance(b, Mapping):
        out: list[tuple[tuple[str, ...], object, object]] = []
        for key in sorted(set(a) | set(b)):
            out += _differences((*path, str(key)), a.get(key, _ABSENT), b.get(key, _ABSENT))
        return out
    return [] if a == b else [(path, a, b)]


def _changed(path: tuple[str, ...], a: object, b: object) -> str:
    return f"canary: {'.'.join(path)} changed: {_shown(a)} → {_shown(b)}"


def _allowed(
    tool: Mapping[str, object], cand: Mapping[str, object], path: tuple[str, ...], a: object, b: object
) -> bool:
    """Is this difference one an upgrade of this tool makes (design §4.4; review DA-S6)?"""
    parts = path[1:]
    if parts in (("version",), ("version_line",)):
        return True
    if len(parts) < 3 or parts[0] != "platforms" or a is _ABSENT or b is _ABSENT:
        return False
    field = parts[2]
    if field == "exec_sha256":
        return len(parts) == 4  # a value; a key added or removed is absent on one side, and named
    if len(parts) != 3:
        return False
    if field in ("archive_sha256", "tree_sha256"):
        return True
    if field not in ("url", "strip"):
        return False
    kind = source(tool)
    if kind == PYPI and field == "url":
        return True  # from the feed (review DQ-S7)
    platform = parts[1]
    pairs = _pairs(
        kind,
        str(tool["version"]),
        str(cand["version"]),
        str(tool["platforms"][platform]["url"]),
        str(cand["platforms"][platform]["url"]),
    )
    return substitute(str(a), pairs) == b


def structural_faults(
    pinned: Mapping[str, object], candidate: Mapping[str, object], upgraded: frozenset[str]
) -> list[str]:
    """Every difference between the two tools.json documents other than version, url, the digests and
    version_line of an upgraded tool, one named line each (review QA-S9): a pin red counts as "pins to update"
    only when this is []."""
    faults: list[str] = []
    for key in sorted((set(pinned) | set(candidate)) - {"tools"}):
        faults += [_changed(*d) for d in _differences((key,), pinned.get(key, _ABSENT), candidate.get(key, _ABSENT))]
    before = {str(t["name"]): t for t in pinned.get("tools", [])}
    after = {str(t["name"]): t for t in candidate.get("tools", [])}
    faults += [f"canary: {name} removed" for name in sorted(set(before) - set(after))]
    faults += [f"canary: {name} added" for name in sorted(set(after) - set(before))]
    kept_before = [n for n in before if n in after]
    kept_after = [n for n in after if n in before]
    for i, name in enumerate(kept_before):
        if kept_after[i] != name:
            faults.append(f"canary: {name} moved: position {i + 1} → {kept_after.index(name) + 1}")
    for name in kept_before:
        for path, a, b in _differences((name,), before[name], after[name]):
            if not (name in upgraded and _allowed(before[name], after[name], path, a, b)):
                faults.append(_changed(path, a, b))
    return faults
