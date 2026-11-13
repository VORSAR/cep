"""The deps rule (S1.1c-1 lane C; design note §5.2; owner 2026-11-11, the plan check, 1, 4 and 8; ADR-0009 §1;
ADR-0001 §1), pure: the row reads the files and runs `go`.

tools/deps-allowlist.json names every module in either tracked module's graph (go.mod, gate/proofs/go.mod), one
entry each, with its version, scope, SPDX licence field and the owner's dated approval. compare() turns the build
lists, the module graph, the entries and the tracked go.mod files into failed lines: a module or graph node with no
entry, a stale entry, a version the graph disagrees with, a licence part outside the Apache-2-compatible set, and,
for a module whose packages are built or tested (imported), a licence file that is missing, unknown or disagrees
with the field (decision 4: a fingerprint for imported modules; the field plus liveness for graph-only ones, which
have no source on disk by design). A third tracked go.mod is the one-module failure of ADR-0001 §1. A graph node that
neither build list selects is red (S1.1c-2 design §4.5: `go list -m all` selects every graph module, so a node
outside both lists is one the rule could not check). Each entry names the sprint whose approval added it and the
deciding section as `ref`, checked by allowlists.sprint_fault and ref_fault, the grammar's one implementation
(design §4.6a; owner, 2026-11-12, plan check 17).
"""

import json
from collections.abc import Callable, Collection, Sequence
from dataclasses import dataclass

from gate.runner import allowlists

ALLOWLIST = "tools/deps-allowlist.json"
GOMODS = ("go.mod", "gate/proofs/go.mod")
ALLOWED = frozenset({"Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC"})
ALLOWED_TEXT = "Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause, ISC"
LICENSE_FILES = ("LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md")
DROPPED_NODES = frozenset({"go", "toolchain"})  # go mod graph's version nodes (review: architect S7)
TOP_KEYS = frozenset({"schema", "note", "modules"})
ENTRY_KEYS = frozenset({"module", "version", "scope", "license", "approved", "why", "sprint", "ref"})
OPTIONAL_KEYS = frozenset({"replace", "built"})  # `built` is false or absent (S1.1c-4 design §2.6; decision 1)


@dataclass(frozen=True)
class Module:
    """One module of a build list (go list -m all, the main module skipped); dir "" when absent on disk."""

    path: str
    version: str
    dir: str
    imported: bool
    replace: str = ""  # the replace directive's target as go list -m -json reports it ("" when none)


@dataclass(frozen=True)
class Entry:
    module: str
    version: str
    scope: str
    license: str
    approved: str
    why: str
    replace: str = ""  # the one replace target this entry approves (owner, 2026-11-11, the fix round)
    sprint: str = ""  # the sprint whose approval added the entry (design §4.6a)
    ref: str = ""  # the deciding section, <path>#<heading-anchor>
    built: bool = True  # False: the register marks the module never built (S1.1c-4's plan check, decision 1)


def _keys(value: object, keys: frozenset[str], where: str) -> dict:
    if not isinstance(value, dict):
        raise ValueError(f"{where}: not an object")
    if set(value) != keys:
        missing, extra = sorted(keys - set(value)), sorted(set(value) - keys)
        raise ValueError(f"{where}: keys {sorted(value)}, missing {missing}, unexpected {extra}")
    return value


def load_allowlist(text: str) -> tuple[Entry, ...]:
    """{"schema": 1, "note": str, "modules": [{the eight keys, non-empty strings, and optionally `replace` (a
    non-empty string) and `built` (false only)}, ...]}: non-empty, modules unique."""
    try:
        data = json.loads(text)
    except ValueError as e:
        raise ValueError(f"{ALLOWLIST}: {e}") from None
    data = _keys(data, TOP_KEYS, ALLOWLIST)
    if data["schema"] != 1:
        raise ValueError(f"{ALLOWLIST}: schema {data['schema']!r}, not 1")
    if not isinstance(data["note"], str):
        raise ValueError(f"{ALLOWLIST}: note is not a string")
    if not isinstance(data["modules"], list):
        raise ValueError(f"{ALLOWLIST}: modules is not a list")
    if not data["modules"]:
        raise ValueError(f"{ALLOWLIST}: modules is empty")
    entries: list[Entry] = []
    for i, item in enumerate(data["modules"]):
        where = f"{ALLOWLIST}: modules[{i}]"
        optional = OPTIONAL_KEYS & set(item) if isinstance(item, dict) else frozenset()
        item = _keys(item, ENTRY_KEYS | optional, where)
        if "replace" in optional and (not isinstance(item["replace"], str) or not item["replace"]):
            fault = "replace is empty" if item["replace"] == "" else "replace is not a string"
            raise ValueError(f"{where}: {fault}")
        if "built" in optional and item["built"] is not False:
            if item["built"] is True:
                raise ValueError(f'deps: {item["module"]}: "built": true is the default: remove it')
            raise ValueError(f"deps: {item['module']}: built must be false")
        for key in sorted(ENTRY_KEYS):
            if not isinstance(item[key], str) or not item[key]:
                raise ValueError(f"{where}: {key} is not a non-empty string")
        if any(entry.module == item["module"] for entry in entries):
            raise ValueError(f"{where}: duplicate module {item['module']}")
        entries.append(
            Entry(
                item["module"],
                item["version"],
                item["scope"],
                item["license"],
                item["approved"],
                item["why"],
                item.get("replace", ""),
                sprint=item["sprint"],
                ref=item["ref"],
                built=item.get("built", True),
            )
        )
    return tuple(entries)


def spdx_parts(field: str) -> frozenset[str]:
    """The parts of a licence field joined by " AND ", each stripped and non-empty."""
    parts = [part.strip() for part in field.split(" AND ")]
    if not all(parts):
        raise ValueError(f"license field {field!r}: empty part")
    return frozenset(parts)


NEGATIVES = (
    "gnu general public license",
    "gnu lesser general public license",
    "gnu affero general public license",
    "mozilla public license",
)


def two_versions_line(path: str, root: str, proofs: str) -> str:
    """A module selected at two versions by the two tracked modules, with the cure (review L4)."""
    return (
        f"deps: {path}: go.mod has {root}, gate/proofs/go.mod has {proofs}; one version across both modules:"
        " restore the pins in go.mod (see its comment)"
    )


def classify_license(text: str) -> str | None:
    """The SPDX tag a licence text's opening fingerprint names, case-insensitively, first match; None otherwise."""
    lower = text.lower()
    for copyleft in NEGATIVES:  # a GPL text that cites the Apache License is not Apache-2.0 (the break-it's probe)
        if copyleft in lower:
            return None
    if "apache license" in lower and "version 2.0" in lower:
        return "Apache-2.0"
    if "permission is hereby granted, free of charge" in lower:
        return "MIT"
    if "redistribution and use in source and binary forms" in lower:
        return "BSD-3-Clause" if "endorse or promote" in lower else "BSD-2-Clause"
    if "permission to use, copy, modify, and/or distribute" in lower:
        return "ISC"
    return None


def decode_json_stream(text: str) -> list[dict]:
    """go list -m -json's output: objects back to back, decoded one by one (review: architect S7)."""
    decoder = json.JSONDecoder()
    objects: list[dict] = []
    at = 0
    while True:
        while at < len(text) and text[at].isspace():
            at += 1
        if at >= len(text):
            return objects
        obj, at = decoder.raw_decode(text, at)
        objects.append(obj)


def graph_nodes(text: str, mains: Collection[str]) -> frozenset[str]:
    """go mod graph's nodes as module paths: both ends of every edge, the "@..." suffix cut, go and toolchain
    (DROPPED_NODES) and the main modules removed."""
    nodes: set[str] = set()
    for line in text.splitlines():
        for node in line.split():
            nodes.add(node.split("@", 1)[0])
    return frozenset(nodes - DROPPED_NODES - set(mains))


def compare(
    modules: Sequence[Module],
    graph: frozenset[str],
    entries: Sequence[Entry],
    gomods: Sequence[str],
    read: Callable[[str], str | None],
    ctx: allowlists.Ctx,
) -> list[str]:
    """The failed lines, one each, in the design's order (§5.2), sorted by module within a kind.

    read(dir): the first LICENSE_FILES text found under dir, or None. ctx: the sprint order, the marker and the ref
    resolver for each entry's `sprint` and `ref`.
    """
    failures: list[str] = []
    tracked = sorted(set(gomods))
    if tracked != sorted(GOMODS):
        failures.append(
            f"deps: tracked go.mod files: {', '.join(tracked)}; ADR-0001 §1 allows two: {', '.join(GOMODS)}"
        )
    by_path = {module.path: module for module in modules}
    by_module = {entry.module: entry for entry in entries}
    for path in sorted(by_path):
        if path not in by_module:
            failures.append(f"deps: {path} {by_path[path].version} is not in {ALLOWLIST}")
    for path in sorted(graph - set(by_path)):
        if path not in by_module:
            failures.append(f"deps: {path} is in the module graph and not in {ALLOWLIST}")
    for path in sorted(graph - set(by_path)):
        failures.append(f"deps: {path} is in the module graph and selected by neither module's build list")
    for module in sorted(by_module):
        if module not in by_path and module not in graph:
            failures.append(f"deps: stale entry {module}: not in the module graph of {' or '.join(GOMODS)}")
    for path in sorted(by_path):  # a replace is red unless the entry approves exactly its target (owner, 2026-11-11)
        module, entry = by_path[path], by_module.get(path)
        if entry is None:
            continue
        if module.replace and entry.replace != module.replace:
            failures.append(f"deps: {path}: replaced in go.mod by {module.replace}; not approved in {ALLOWLIST}")
        elif entry.replace and not module.replace:
            failures.append(f"deps: {path}: the allowlist approves a replace by {entry.replace}, go.mod has none")
    for module in sorted(by_module):
        found = by_path.get(module)
        if found is not None and found.version != by_module[module].version:
            failures.append(
                f"deps: {module}: the allowlist says {by_module[module].version}, the graph has {found.version}"
            )
    parts: dict[str, frozenset[str]] = {}
    for module in sorted(by_module):
        try:
            parts[module] = spdx_parts(by_module[module].license)
        except ValueError as e:
            failures.append(f"deps: {module}: {e}")
            continue
        for part in sorted(parts[module] - ALLOWED):
            failures.append(f"deps: {module}: license {part} is not Apache-2 compatible (allowed: {ALLOWED_TEXT})")
    for path in sorted(by_path):
        module = by_path[path]
        if not module.imported or path not in by_module:
            continue
        if not module.dir:
            failures.append(f"deps: {path}: no source on disk; run: go mod download {path}")
            continue
        text = read(module.dir)
        if text is None:
            failures.append(f"deps: {path}: no LICENSE file in {module.dir}")
            continue
        classified = classify_license(text)
        if classified is None:
            failures.append(f"deps: {path}: unknown licence text in {module.dir}")
        elif path in parts and classified not in parts[path]:
            failures.append(f"deps: {path}: license field {by_module[path].license}, file reads {classified}")
    for module in sorted(by_module):  # decision 1: a never-built mark is red once a package compiles the module
        found = by_path.get(module)
        if not by_module[module].built and found is not None and found.imported:
            failures.append(f'deps: {module} is marked never built, but a package compiles it: remove "built": false')
    for module in sorted(by_module):
        entry = by_module[module]
        for why in (allowlists.sprint_fault(entry.sprint, ctx), allowlists.ref_fault(entry.ref, ctx)):
            if why:
                failures.append(f"deps: {module}: {why}")
    return failures
