"""The deps rule (S1.1c-1 lane C; design note §5.2; owner 2026-11-11, the plan check, 1, 4 and 8; ADR-0009 §1;
ADR-0001 §1), pure: the row reads the files and runs `go`.

tools/deps-allowlist.json names every module in either tracked module's graph (go.mod, gate/proofs/go.mod), one
entry each, with its version, scope, SPDX licence field and the owner's dated approval. compare() turns the build
lists, the module graph, the entries and the tracked go.mod files into failed lines: a module or graph node with no
entry, a stale entry, a version the graph disagrees with, a licence part outside the Apache-2-compatible set, and,
for a module whose packages are built or tested (imported), a licence file that is missing, unknown or disagrees
with the field (decision 4: a fingerprint for imported modules; the field plus liveness for graph-only ones, which
have no source on disk by design). A third tracked go.mod is the one-module failure of ADR-0001 §1.
"""

import json
from collections.abc import Callable, Collection, Sequence
from dataclasses import dataclass

ALLOWLIST = "tools/deps-allowlist.json"
GOMODS = ("go.mod", "gate/proofs/go.mod")
ALLOWED = frozenset({"Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC"})
ALLOWED_TEXT = "Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause, ISC"
LICENSE_FILES = ("LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md")
DROPPED_NODES = frozenset({"go", "toolchain"})  # go mod graph's version nodes (review: architect S7)
TOP_KEYS = frozenset({"schema", "note", "modules"})
ENTRY_KEYS = frozenset({"module", "version", "scope", "license", "approved", "why"})


@dataclass(frozen=True)
class Module:
    """One module of a build list (go list -m all, the main module skipped); dir "" when absent on disk."""

    path: str
    version: str
    dir: str
    imported: bool


@dataclass(frozen=True)
class Entry:
    module: str
    version: str
    scope: str
    license: str
    approved: str
    why: str


def _keys(value: object, keys: frozenset[str], where: str) -> dict:
    if not isinstance(value, dict):
        raise ValueError(f"{where}: not an object")
    if set(value) != keys:
        missing, extra = sorted(keys - set(value)), sorted(set(value) - keys)
        raise ValueError(f"{where}: keys {sorted(value)}, missing {missing}, unexpected {extra}")
    return value


def load_allowlist(text: str) -> tuple[Entry, ...]:
    """{"schema": 1, "note": str, "modules": [{the six keys, non-empty strings}, ...]}: non-empty, modules unique."""
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
        item = _keys(item, ENTRY_KEYS, where)
        for key in sorted(ENTRY_KEYS):
            if not isinstance(item[key], str) or not item[key]:
                raise ValueError(f"{where}: {key} is not a non-empty string")
        if any(entry.module == item["module"] for entry in entries):
            raise ValueError(f"{where}: duplicate module {item['module']}")
        entries.append(
            Entry(item["module"], item["version"], item["scope"], item["license"], item["approved"], item["why"])
        )
    return tuple(entries)


def spdx_parts(field: str) -> frozenset[str]:
    """The parts of a licence field joined by " AND ", each stripped and non-empty."""
    parts = [part.strip() for part in field.split(" AND ")]
    if not all(parts):
        raise ValueError(f"license field {field!r}: empty part")
    return frozenset(parts)


def classify_license(text: str) -> str | None:
    """The SPDX tag a licence text's opening fingerprint names, case-insensitively, first match; None otherwise."""
    lower = text.lower()
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
) -> list[str]:
    """The failed lines, one each, in the design's order (§5.2), sorted by module within a kind.

    read(dir): the first LICENSE_FILES text found under dir, or None.
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
    for module in sorted(by_module):
        if module not in by_path and module not in graph:
            failures.append(f"deps: stale entry {module}: not in the module graph of {' or '.join(GOMODS)}")
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
    return failures
