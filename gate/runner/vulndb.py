"""The vulnerability scan's rule over a copy of the Go vulnerability database (S1.1c-4 plan check §2 S1; design
note §2.2, §2.4; owner 2026-11-13, S1.1c-4's plan check, decisions 1 and 2). Pure: it reads only what it is given.

The matcher follows x/vuln v1.9.0's internal/semver/affects.go (an OSV SEMVER range, `introduced "0"` minus
infinity, events sorted before the walk) and its earliestValidFix for the fixed version; versions order as semver
2.0 (x/mod's semver.Compare), a leading `v` and `+build` metadata dropped. A version it cannot order is a named
fault, never the lowest version; a bad entry never stops the walk (review DA-S1).
"""

import json
import re
from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass
from datetime import datetime

CANONICAL_URL = "https://vuln.go.dev/vulndb.zip"
COPY_MAX_DAYS = 7  # decision 2: the copy fetched at most this many UTC calendar days ago
SOURCE_MAX_DAYS = 30  # decision 2: the database's own `modified` at most this many UTC calendar days old
FETCH_CAP = 64 << 20  # the zip, about 21x today's 2.97 MB (design §2.3)
MEMBER_CAP = 4 << 20  # one member's decompressed bytes, about 7x the largest today (design §2.4)

DB_JSON = "index/db.json"
MODULES_JSON = "index/modules.json"
VULNS_JSON = "index/vulns.json"
INDEX = frozenset({DB_JSON, MODULES_JSON, VULNS_JSON})
ENTRY_NAME = re.compile(r"ID/(GO-[0-9]{4}-[0-9]{4,})\.json", re.ASCII)
STAMP_KEYS = frozenset({"fetched", "sha256", "url"})
GO_PSEUDO = ("stdlib", "toolchain")  # the database's names for the standard library and the go command
INTRODUCED, FIXED = "introduced", "fixed"

_NUM = r"(?:0|[1-9][0-9]*)"
_IDS = r"[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*"
_SEMVER = re.compile(rf"({_NUM})\.({_NUM})\.({_NUM})(?:-({_IDS}))?(?:\+{_IDS})?", re.ASCII)
_GO_TAG = re.compile(rf"go({_NUM})\.({_NUM})(?:\.({_NUM})|(rc|beta)([1-9][0-9]*))?", re.ASCII)
_RFC3339 = re.compile(
    r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?(?:Z|[+-][0-9]{2}:[0-9]{2})", re.ASCII
)


class InvalidVersion(ValueError):
    """A version the matcher cannot order: a named red, never read as the lowest version (plan check §2 S1)."""


class UnknownEvent(ValueError):
    """A range event other than `introduced` and `fixed`: a named red (design §2.2)."""


class MemberTooLarge(ValueError):
    """A zip member whose decompressed bytes pass MEMBER_CAP; raised by the row's `read` (review DA-S10)."""


@dataclass(frozen=True)
class Finding:
    id: str  # the entry's id, e.g. GO-2026-6860
    module: str  # a module path, "stdlib" or "toolchain"
    version: str  # the version read, as the build list or GOVERSION gave it
    fixed: str  # the first fixed version after `version`, "" when none
    built: bool  # False when the register marks the module never built: named in the reason, never red (decision 1)


def _parse(text: str) -> tuple:
    """A semver 2.0 version as a key that orders as x/mod's semver.Compare; a leading `v` and `+build` dropped."""
    if not isinstance(text, str):
        raise InvalidVersion(repr(text))
    match = _SEMVER.fullmatch(text.removeprefix("v"))
    if match is None:
        raise InvalidVersion(text)
    major, minor, patch, pre = match.groups()
    core = (int(major), int(minor), int(patch))
    if pre is None:
        return (*core, 1, ())
    keys = []
    for ident in pre.split("."):
        if ident.isdigit():
            if len(ident) > 1 and ident.startswith("0"):
                raise InvalidVersion(text)
            keys.append((0, int(ident), ""))
        else:
            keys.append((1, 0, ident))
    return (*core, 0, tuple(keys))


def go_version(tag: str) -> str:
    """`go env GOVERSION` as a database version: go1.27.4 -> 1.27.4, go1.27 -> 1.27.0, go1.28rc1 -> 1.28.0-rc.1,
    go1.28beta2 -> 1.28.0-beta.2; anything else raises InvalidVersion (design §2.2)."""
    match = _GO_TAG.fullmatch(tag) if isinstance(tag, str) else None
    if match is None:
        raise InvalidVersion(tag)
    major, minor, patch, pre, number = match.groups()
    version = f"{major}.{minor}.{patch or 0}" + (f"-{pre}.{number}" if pre else "")
    _parse(version)
    return version


def _events(ranges: Sequence[Mapping[str, object]]) -> list[list[tuple[str, str]]] | None:
    """Each SEMVER range's events as (kind, version); None when no range is SEMVER. UnknownEvent for a key other
    than introduced and fixed, InvalidVersion for a version that is not a string."""
    found: list[list[tuple[str, str]]] = []
    for r in ranges:
        if not isinstance(r, Mapping) or r.get("type") != "SEMVER":
            continue
        events: list[tuple[str, str]] = []
        for event in r.get("events") or []:
            keys = sorted(event) if isinstance(event, Mapping) else []
            if len(keys) != 1 or keys[0] not in (INTRODUCED, FIXED):
                raise UnknownEvent(", ".join(k for k in keys if k not in (INTRODUCED, FIXED)) or repr(event))
            value = event[keys[0]]
            if not isinstance(value, str):
                raise InvalidVersion(repr(value))
            events.append((keys[0], value))
        found.append(events)
    return found or None


def _contains(version: tuple, events: list[tuple[str, str]]) -> bool:
    """affects.go's ContainsSemver: no events holds every version; else the sorted walk."""
    if not events:
        return True
    ordered = sorted(events, key=lambda e: (0, ()) if e == (INTRODUCED, "0") else (1, _parse(e[1])))
    held = False
    for kind, value in ordered:
        if not held and kind == INTRODUCED:
            held = value == "0" or version >= _parse(value)
        elif held and kind == FIXED:
            held = version < _parse(value)
    return held


def affected(version: str, ranges: Sequence[Mapping[str, object]]) -> bool:
    """Whether `version` lies in an `affected` block's `ranges` (OSV SEMVER events; "0" is minus infinity; no
    ranges, or no SEMVER range, means every version). Raises InvalidVersion for a version it cannot order."""
    key = _parse(version)
    semver = _events(ranges)
    if semver is None:
        return True
    return any(_contains(key, events) for events in semver)


def _fixed(version: str, blocks: Sequence[Sequence[Mapping[str, object]]]) -> str:
    """x/vuln's earliestValidFix: the smallest fixed event above `version` in the module's blocks that no block
    itself holds; "" when none."""
    key = _parse(version)
    fixes: dict[tuple, str] = {}
    for ranges in blocks:
        for events in _events(ranges) or []:
            for kind, value in events:
                if kind == FIXED and _parse(value) > key:
                    fixes.setdefault(_parse(value), value)
    for fix in sorted(fixes):
        if not any(affected(fixes[fix], ranges) for ranges in blocks):
            return fixes[fix]
    return ""


def _matched(module: str, given: str) -> str:
    """The version as the matcher reads it: a go tag converted for stdlib and toolchain, a module's as given."""
    if module in GO_PSEUDO:
        return go_version(given)
    _parse(given)
    return given


def findings(
    modules: Mapping[str, str],
    never_built: frozenset[str],
    go_version: str,
    entries: Iterable[Mapping[str, object]],
) -> tuple[list[Finding], list[tuple[str, str, str]]]:
    """Every entry affecting a module of `modules` (path -> version as the build list gives it, a `replace`
    already applied), `stdlib` and `toolchain` at `go_version` (`go env GOVERSION`, e.g. go1.27.4); withdrawn
    entries skipped; a module in `never_built` gives findings with built=False; sorted by (module, id). It never
    stops at a bad entry: each one it cannot judge is a fault (id, module, text), collected (review DA-S1)."""
    tag = go_version
    found: list[Finding] = []
    faults: list[tuple[str, str, str]] = []
    for entry in entries:
        entry_id = entry.get("id") if isinstance(entry, Mapping) else None
        if not isinstance(entry_id, str) or not isinstance(entry.get("affected"), list):
            faults.append((str(entry_id), "", f"vulns: {entry_id}: the entry is malformed"))
            continue
        if entry.get("withdrawn"):
            continue
        blocks: dict[str, list[Sequence[Mapping[str, object]]]] = {}
        for block in entry["affected"]:
            package = block.get("package") if isinstance(block, Mapping) else None
            if not isinstance(package, Mapping) or package.get("ecosystem") != "Go":
                continue
            name = package.get("name")
            if not isinstance(name, str) or (name not in modules and name not in GO_PSEUDO):
                continue
            ranges = block.get("ranges") or []
            blocks.setdefault(name, []).append(ranges if isinstance(ranges, list) else [ranges])
        for module in sorted(blocks):
            given = tag if module in GO_PSEUDO else modules[module]
            try:
                version = _matched(module, given)
                if any(affected(version, ranges) for ranges in blocks[module]):
                    fixed = _fixed(version, blocks[module])
                    found.append(Finding(entry_id, module, given, fixed, module not in never_built))
            except UnknownEvent as e:
                faults.append((entry_id, module, f"vulns: {entry_id}: unknown range event {e}"))
            except InvalidVersion as e:
                faults.append((entry_id, module, f"vulns: {entry_id} {module}: version {e} cannot be ordered"))
    found.sort(key=lambda f: (f.module, f.id))
    faults.sort(key=lambda f: (f[1], f[0], f[2]))
    return found, faults


def _time(text: object) -> datetime | None:
    """An RFC 3339 time with its zone, or None."""
    if not isinstance(text, str) or not _RFC3339.fullmatch(text):
        return None
    try:
        return datetime.fromisoformat(text)
    except ValueError:
        return None


def _json(data: bytes | None) -> object:
    if data is None:
        return None
    try:
        return json.loads(data.decode("utf-8"))
    except ValueError:
        return None


def _stamp_faults(stamp: Mapping[str, object] | None, root: str, zip_sha256: str, now: datetime) -> list[str]:
    if stamp is None:
        return [f"vulns: no stamp.json at {root}: run make vulndb"]
    if not isinstance(stamp, Mapping):
        return ["vulns: stamp.json is malformed: not a JSON object"]
    if set(stamp) != STAMP_KEYS:
        return [f"vulns: stamp.json is malformed: keys {', '.join(sorted(stamp))}, want fetched, sha256, url"]
    for key in sorted(STAMP_KEYS):
        if not isinstance(stamp[key], str):
            return [f"vulns: stamp.json is malformed: {key} is not a string"]
    faults: list[str] = []
    if stamp["sha256"] != zip_sha256:
        faults.append("vulns: the zip's sha256 is not the stamp's: run make vulndb")
    if stamp["url"] != CANONICAL_URL:
        faults.append(f"vulns: the copy came from {stamp['url']}, not {CANONICAL_URL}")
    fetched = _time(stamp["fetched"])
    if fetched is None:
        faults.append(f"vulns: stamp.json is malformed: fetched {stamp['fetched']} is not an RFC 3339 time")
    elif fetched > now:
        faults.append(f"vulns: the copy's fetched time {stamp['fetched']} is in the future")
    elif (days := (now.date() - fetched.date()).days) > COPY_MAX_DAYS:
        faults.append(
            f"vulns: the copy was fetched {days} days ago ({fetched.date().isoformat()}), more than {COPY_MAX_DAYS}:"
            " run make vulndb"
        )
    return faults


def _index_items(value: object, fields: Mapping[str, type]) -> list[dict] | None:
    """A non-empty list of objects each holding `fields` with their types; None otherwise."""
    if not isinstance(value, list) or not value:
        return None
    for item in value:
        if not isinstance(item, dict) or any(not isinstance(item.get(k), t) for k, t in fields.items()):
            return None
    return value


def _named(entry: dict) -> set[str]:
    """The package names an entry's `affected` blocks name."""
    return {
        b["package"]["name"]
        for b in entry["affected"]
        if isinstance(b, dict) and isinstance(b.get("package"), dict) and isinstance(b["package"].get("name"), str)
    }


def check_db(
    read: Callable[[str], bytes | None],
    names: Sequence[str],
    stamp: Mapping[str, object] | None,
    *,
    root: str,
    zip_sha256: str,
    now: datetime,
) -> list[str]:
    """Every fault of the copy and of the database, one named line each; [] when the copy is usable.

    `read(name)` returns a zip member's bytes (None when absent; it raises MemberTooLarge past MEMBER_CAP); `names`
    is the zip's member list; `stamp` is the parsed stamp.json (None when the file is missing); `root` is the copy's
    directory, for the texts; `zip_sha256` is the zip's own digest; `now` is timezone-aware UTC."""
    faults = _stamp_faults(stamp, root, zip_sha256, now)
    fetched = _time(stamp.get("fetched")) if isinstance(stamp, Mapping) else None
    too_large: set[str] = set()

    def member(name: str) -> bytes | None:
        try:
            return read(name)
        except MemberTooLarge:
            too_large.add(name)
            faults.append(f"vulns: member {name} exceeds 4 MiB")
            return None

    entry_ids: set[str] = set()
    for name in sorted(names):
        match = ENTRY_NAME.fullmatch(name)
        if match is not None:
            entry_ids.add(match.group(1))
        elif name not in INDEX:
            faults.append(f"vulns: member {name} is not in the database's layout")

    db = _json(member(DB_JSON))
    modified = _time(db.get("modified")) if isinstance(db, dict) and set(db) == {"modified"} else None
    if modified is None:
        if DB_JSON not in too_large:
            faults.append(f"vulns: {DB_JSON} is missing or malformed")
    else:
        if fetched is not None and modified > fetched:
            faults.append(
                f"vulns: the database's modified {db['modified']} is after the copy's fetched {stamp['fetched']}"
            )
        if modified > now:
            faults.append(f"vulns: the database's modified {db['modified']} is in the future")
        elif (days := (now.date() - modified.date()).days) > SOURCE_MAX_DAYS:
            faults.append(
                f"vulns: the database has not changed since {modified.date().isoformat()} ({days} days, more than"
                f" {SOURCE_MAX_DAYS}): check vuln.go.dev, or add a dated vulndb exemption naming a known issue"
            )

    index_modules = _index_items(_json(member(MODULES_JSON)), {"path": str, "vulns": list})
    if index_modules is not None and any(
        not isinstance(v, dict) or not isinstance(v.get("id"), str) for item in index_modules for v in item["vulns"]
    ):
        index_modules = None
    if index_modules is None and MODULES_JSON not in too_large:
        faults.append(f"vulns: {MODULES_JSON} is missing, malformed or empty")
    index_vulns = _index_items(_json(member(VULNS_JSON)), {"id": str, "modified": str})
    if index_vulns is None and VULNS_JSON not in too_large:
        faults.append(f"vulns: {VULNS_JSON} is missing, malformed or empty")

    if index_vulns is not None:
        listed = {item["id"] for item in index_vulns}
        for entry_id in sorted(listed - entry_ids):
            faults.append(f"vulns: {entry_id} is listed in {VULNS_JSON} but ID/{entry_id}.json is missing")
        for entry_id in sorted(entry_ids - listed):
            faults.append(f"vulns: ID/{entry_id}.json is not listed in {VULNS_JSON}")
    if index_modules is not None:
        pairs: dict[str, set[str]] = {}
        for item in index_modules:
            for v in item["vulns"]:
                pairs.setdefault(v["id"], set()).add(item["path"])
        for entry_id in sorted(set(pairs) | entry_ids):
            name = f"ID/{entry_id}.json"
            named: set[str] = set()
            if entry_id in entry_ids:
                data = member(name)
                if data is None:
                    continue  # too large (named above) or absent from the zip
                entry = _json(data)
                if (
                    not isinstance(entry, dict)
                    or entry.get("id") != entry_id
                    or not isinstance(entry.get("affected"), list)
                ):
                    faults.append(f"vulns: {name} is malformed")
                    continue
                named = _named(entry)
            for module in sorted(named ^ pairs.get(entry_id, set())):
                faults.append(f"vulns: {MODULES_JSON} and {name} disagree on {module}")
    return faults
