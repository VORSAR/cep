"""The vulns row (S1.1c-4 plan check §2 S1; design note §2.5; owner 2026-11-13, S1.1c-4's plan check, decisions 1, 2
and 5): a known vulnerability in a module the build lists select, the standard library or the toolchain at the pinned
Go is red, naming its id, unless the dependency register marks the module never built (then named in the reason) or
tools/allowlists/vulns.json holds a dated exemption for it.

The steps: the shared loader (gomods.load; each load error a fault); the register's never-built marks; the copy
(vulndb.zip read once into memory, bounded by FETCH_CAP; its sha256 from those bytes; members read through the zip,
each bounded by MEMBER_CAP); fetch-error.txt; vulndb.check_db; vulndb.findings over the entries index/vulns.json
lists; the waivers. A `<id> <module>` waiver silences that pair's finding and faults; the `vulndb` waiver only the
copy's availability and age faults, never an integrity fault or a finding (review DA-B3). An expired waiver is
ignored (the suppressions row reds it). Staleness of `<id> <module>` waivers is judged only on a completed scan
(review DQ-S4a). The rule itself is gate/runner/vulndb.py.
"""

import hashlib
import io
import json
import os
import re
import sys
import zipfile
from dataclasses import dataclass, field
from datetime import UTC, date, datetime

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import allowlists, deps, rows, vulndb
from gate.runner.rows import gomods
from gate.toolchain import vulndb_fetch

WAIVERS = "tools/allowlists/vulns.json"
COVERED = 6  # go.mod, go.sum, gate/proofs/go.mod, gate/proofs/go.sum, the register, the waivers (design §2.5)
VULNDB = "vulndb"
PAIR = re.compile(r"(GO-[0-9]{4}-[0-9]{4,}) (\S+)", re.ASCII)
# The faults the `vulndb` waiver may silence: the copy's availability and age, never its integrity (review DA-B3).
# "no stamp.json" is not one: it is named only beside a zip, whose origin it leaves unknown (review CA-L5).
AVAILABILITY = (
    "vulns: no vulndb.zip at ",
    "vulns: the last make vulndb failed at ",
    "vulns: the copy was fetched ",
    "vulns: the database has not changed since ",
)
NOT_READ = (
    "not read: golangci-lint (Go build info, from S1.1c-5); ruff, CPython (outside the Go database); build tags"
    " beyond linux/amd64 and darwin/arm64, CGO_ENABLED=1"
)


@dataclass
class Copy:
    """What the copy gave: the zip (None when no zip could be read), its stamp, its faults and notes."""

    root: str
    faults: list[str] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)
    zip: zipfile.ZipFile | None = None
    sha256: str = ""
    stamp: object = None
    cache: dict[str, bytes | None] = field(default_factory=dict)

    def read(self, name: str) -> bytes | None:
        """A member's bytes, None when absent; MemberTooLarge past MEMBER_CAP decompressed bytes; MemberUnreadable
        when its bytes cannot be read (reviews BK-B1, CA-B1)."""
        if name not in self.cache:
            if self.zip is None:
                return None
            self.cache[name] = vulndb.read_member(self.zip, name)
        return self.cache[name]


def _json_file(path: str) -> tuple[bool, object]:
    """(present, parsed): parsed is the ValueError when the file is not JSON."""
    try:
        text = rows.read_text(path)
    except FileNotFoundError:
        return False, None
    try:
        return True, json.loads(text)
    except ValueError as e:
        return True, e


def _parsed(data: bytes | None) -> object:
    try:
        return json.loads(data.decode("utf-8")) if data is not None else None
    except ValueError:
        return None


def open_copy(root: str, now: datetime) -> Copy:
    """The zip read once, its stamp and fetch-error.txt, and check_db's faults; every fault one line."""
    copy = Copy(root)
    path = os.path.join(root, vulndb_fetch.ZIP)
    body: bytes | None = None
    try:
        with open(path, "rb") as f:
            body = f.read(vulndb.FETCH_CAP + 1)
    except FileNotFoundError:
        copy.faults.append(f"vulns: no vulndb.zip at {root}: run make vulndb")
    if body is not None and len(body) > vulndb.FETCH_CAP:
        copy.faults.append(f"vulns: {path} is larger than {vulndb.FETCH_CAP >> 20} MiB: run make vulndb")
    elif body is not None:
        try:
            if not zipfile.is_zipfile(io.BytesIO(body)):
                raise zipfile.BadZipFile(path)
            copy.zip = zipfile.ZipFile(io.BytesIO(body))
            copy.sha256 = hashlib.sha256(body).hexdigest()
        except zipfile.BadZipFile, OSError, ValueError, EOFError:
            copy.faults.append(f"vulns: {path} is not a zip: run make vulndb")
    present, stamp = _json_file(os.path.join(root, vulndb_fetch.STAMP))
    copy.stamp = stamp if present and not isinstance(stamp, ValueError) else ("not JSON" if present else None)
    fetched = vulndb._time(stamp.get("fetched")) if isinstance(stamp, dict) else None
    present, error = _json_file(os.path.join(root, vulndb_fetch.ERROR))
    if present:
        well_formed = isinstance(error, dict) and set(error) == {"at", "error"} and isinstance(error["error"], str)
        at = vulndb._time(error["at"]) if well_formed else None
        if at is None:
            copy.faults.append("vulns: fetch-error.txt is malformed: run make vulndb")
        elif fetched is None or at > fetched:
            if copy.zip is None:
                copy.faults.append(f"vulns: the last make vulndb failed at {error['at']}: {error['error']}")
            else:  # a usable copy: named, its age the red (decision 2; review DQ-S11)
                copy.notes.append(f"last fetch failed at {error['at']}: {error['error']}")
    if copy.zip is not None:
        names = sorted(set(copy.zip.namelist()))
        copy.faults += vulndb.check_db(copy.read, names, copy.stamp, root=root, zip_sha256=copy.sha256, now=now)
    return copy


def entries_of(copy: Copy) -> tuple[list[dict], bool]:
    """The entries index/vulns.json lists, and whether both indexes parsed and every listed entry was read."""
    try:
        index = _parsed(copy.read(vulndb.VULNS_JSON))
        modules = _parsed(copy.read(vulndb.MODULES_JSON))
    except vulndb.MemberTooLarge, vulndb.MemberUnreadable:  # named by check_db
        return [], False
    if not isinstance(index, list) or not index:
        return [], False
    entries: list[dict] = []
    complete = isinstance(modules, list) and bool(modules)
    for item in index:
        entry_id = item.get("id") if isinstance(item, dict) else None
        try:
            entry = _parsed(copy.read(f"ID/{entry_id}.json")) if isinstance(entry_id, str) else None
        except vulndb.MemberTooLarge, vulndb.MemberUnreadable:  # named by check_db; the scan incomplete
            entry = None
        if isinstance(entry, dict):
            entries.append(entry)
        else:
            complete = False
    return entries, complete


@dataclass(frozen=True)
class Waiver:
    target: str
    expiry: str
    issue: str


def waivers(lst: allowlists.AllowList, today: date) -> tuple[dict[str, Waiver], list[str]]:
    """The live waivers by target, and the faults of the refused ones; an expired one is ignored (review DQ-S4c)."""
    live: dict[str, Waiver] = {}
    faults: list[str] = []
    for e in lst.entries:
        if e.target != VULNDB and PAIR.fullmatch(e.target) is None:
            faults.append(f'vulns: {e.target}: not "<id> <module>" or "vulndb"')
            continue
        expiry = allowlists._day(e.expiry)
        if e.kind != "TEMP" or expiry is None or not e.issue:
            faults.append(f"vulns: {e.target}: a vulnerability exemption is TEMP, with an expiry and a known issue")
            continue
        if expiry >= today:
            live[e.target] = Waiver(e.target, e.expiry, e.issue)
    return live, faults


def _module_map(loaded: gomods.Loaded, notes: list[str]) -> dict[str, set[str]]:
    """path -> the versions govulncheck matches: at the replacement when it has a version (review DA-S2); a path
    built both as a replacement and at its own version gives both (review BK-B2)."""
    modules: dict[str, set[str]] = {}
    for m in loaded.modules:
        if m.replace_path and not m.replace_version:
            notes.append(f"replaced by a directory: {m.path} → {m.replace_path}")
        elif m.replace_path:
            modules.setdefault(m.replace_path, set()).add(m.replace_version)
        else:
            modules.setdefault(m.path, set()).add(m.version)
    return modules


def _day(text: object) -> str:
    moment = vulndb._time(text)
    return vulndb._utc_day(moment).isoformat() if moment is not None else "?"


def scan(row_dir: str, env: dict[str, str], *, repo: str, copy_root: str, now: datetime) -> tuple[list[str], list[str]]:
    """The row's faults and its reason lines: `repo` holds the register and the waivers, `copy_root` the copy."""
    faults: list[str] = []
    notes: list[str] = []
    lines: list[str] = []
    complete = True
    loaded: gomods.Loaded | None = None
    try:
        loaded = gomods.load(row_dir, env)
    except (gomods.Failed, OSError, ValueError) as e:
        complete = False
        faults.append(f"vulns: {e}")
    if loaded is not None:
        faults += list(loaded.errors)
        if not loaded.modules:
            faults.append("vulns: no module read")
            complete = False
    never_built: frozenset[str] = frozenset()
    try:
        register = deps.load_allowlist(rows.read_text(os.path.join(repo, deps.ALLOWLIST)))
        never_built = frozenset(e.module for e in register if not e.built)
    except (OSError, ValueError) as e:
        faults.append(f"vulns: {e}")
    live: dict[str, Waiver] = {}
    try:
        live, refused = waivers(allowlists.load(rows.read_text(os.path.join(repo, WAIVERS)), "vulns"), now.date())
        faults += refused
    except (OSError, ValueError) as e:
        faults.append(f"vulns: {e}")
    used: set[str] = set()

    copy = open_copy(copy_root, now)
    notes += copy.notes
    silenced_copy: list[str] = []
    for fault in copy.faults:
        if VULNDB in live and fault.startswith(AVAILABILITY):
            silenced_copy.append(fault)
            used.add(VULNDB)
        else:
            faults.append(fault)

    found: list[vulndb.Finding] = []
    scan_faults: list[tuple[str, str, str]] = []
    entries: list[dict] = []
    if copy.zip is None:
        complete = False
    else:
        entries, read_all = entries_of(copy)
        complete = complete and read_all
        if loaded is not None:
            found, scan_faults = vulndb.findings(_module_map(loaded, notes), never_built, loaded.go_version, entries)

    unbuilt: list[str] = []
    for f in found:
        pair = f"{f.id} {f.module}"
        if not f.built:  # decision 1: named, never red
            unbuilt.append(f"{pair} {f.version} fixed {f.fixed or 'no fix yet'}")
        elif pair in live:
            used.add(pair)
        else:
            faults.append(f"vulns: {pair} {f.version}: {f'fixed in {f.fixed}' if f.fixed else 'no fix yet'}")
    for entry_id, module, text in scan_faults:
        pair = f"{entry_id} {module}"
        if pair in live:
            used.add(pair)
        else:
            faults.append(text)
    for target in sorted(set(live) - used):
        if target == VULNDB or complete:
            faults.append(f"vulns: {target} matches nothing: remove it")

    if copy.zip is None and silenced_copy and not faults:
        w = live[VULNDB]
        lines.append(f"vulns: not scanned: {'; '.join(silenced_copy)}; exempted by vulndb until {w.expiry} ({w.issue})")
    if copy.zip is not None and loaded is not None:
        stamp = copy.stamp if isinstance(copy.stamp, dict) else {}
        db = _parsed(copy.cache.get(vulndb.DB_JSON))
        lines.append(
            f"vulns: vulndb {_day(db.get('modified') if isinstance(db, dict) else None)} (fetched"
            f" {_day(stamp.get('fetched'))}, {copy.root}), {len(entries)} entries; {len(loaded.modules)} modules"
            f" ({sum(1 for m in loaded.modules if m.built)} built); stdlib and toolchain at {loaded.go_version}"
        )
    if unbuilt:
        lines.append(f"vulns: never built: {', '.join(unbuilt)}")
    if used:
        silenced = ", ".join(f"{t} until {live[t].expiry} ({live[t].issue})" for t in sorted(used))
        lines.append(f"vulns: silenced: {silenced}")
    lines += [f"vulns: {note}" for note in sorted(set(notes))]
    lines.append(f"vulns: {NOT_READ}")
    return faults, lines


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    env = {**os.environ, **gomods.ENV}
    faults, lines = scan(row_dir, env, repo=".", copy_root=vulndb_fetch.copy_root(os.environ), now=datetime.now(UTC))
    rows.covered(COVERED)
    for line in lines:
        print(line)
    sys.stdout.flush()
    return rows.finish(faults)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
