"""The law manifest and the semantic parsers (S1.1c-1 design §2.3; ADR-0015 §1.3).

Pure: texts in, lines out, no I/O. `load_manifest` reads gate/law.json's text strictly (a duplicate key, an extra
or missing key, a flag on a non-doc, an unsafe path are each a ManifestError naming the entry). `delta` gives one
changed law file's one-line semantic delta and never raises: a side its parser cannot read says so. `block`
renders the LAW Δ block: the data files and documents one line each, then the code and workflow entries grouped
(owner, 2026-11-11, decision 4).
"""

import difflib
import json
import re
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

KINDS = frozenset(
    {
        "registry",
        "law",
        "marker",
        "sprints",
        "ceilings",
        "code",
        "workflow",
        "tools",
        "deps",
        "edges",
        "exports",
        "baseline",
        "notbuilt",
        "doc",
    }
)
GROUPED = frozenset({"code", "workflow"})
TOP_KEYS = frozenset({"schema", "note", "paths"})
HEADING = re.compile(r"^#{1,6}\s+(.*?)\s*#*\s*$")
NUMBERED = re.compile(r"^(\d+)\. ")
NONE = "LAW Δ: none"


@dataclass(frozen=True)
class Path:
    path: str
    kind: str
    flag: str = ""


@dataclass(frozen=True)
class Manifest:
    paths: tuple[Path, ...]


class ManifestError(ValueError):
    """gate/law.json cannot be read as the manifest; the message names the entry."""


def _no_duplicates(pairs: list[tuple[str, object]]) -> dict[str, object]:
    out: dict[str, object] = {}
    for key, value in pairs:
        if key in out:
            raise ValueError(f"duplicate key {key!r}")
        out[key] = value
    return out


def loads(text: str) -> object:
    """json.loads refusing a duplicate key (ValueError, as any other JSON fault)."""
    return json.loads(text, object_pairs_hook=_no_duplicates)


def _entry(i: int, item: object, seen: set[str]) -> Path:
    def fault(what: str) -> ManifestError:
        return ManifestError(f"gate/law.json: paths[{i}]: {what}")

    if type(item) is not dict:
        raise fault("not an object")
    keys = set(item)
    if keys not in ({"path", "kind"}, {"path", "kind", "flag"}):
        raise fault(f"keys {', '.join(sorted(keys))}: want path, kind and, for a doc, flag")
    path, kind = item["path"], item["kind"]
    if type(path) is not str or not path:
        raise fault("path is not a non-empty string")
    if path.startswith("/") or ".." in path.split("/"):
        raise fault(f"path {path!r} must be relative to the repository, without ..")
    if path in seen:
        raise fault(f"path {path!r} is listed twice")
    if kind not in KINDS:
        raise fault(f"kind {kind!r} is not one of {', '.join(sorted(KINDS))}")
    if (kind == "doc") != ("flag" in keys):
        raise fault("a doc carries a flag; nothing else does")
    flag = item.get("flag", "")
    if kind == "doc" and (type(flag) is not str or not flag):
        raise fault("flag is not a non-empty string")
    seen.add(path)
    return Path(path, kind, flag)


def load_manifest(text: str) -> Manifest:
    try:
        data = loads(text)
    except ValueError as e:
        raise ManifestError(f"gate/law.json: {e}") from None
    if type(data) is not dict:
        raise ManifestError("gate/law.json: not a JSON object")
    for key in sorted(TOP_KEYS - set(data)):
        raise ManifestError(f"gate/law.json: missing key {key!r}")
    for key in sorted(set(data) - TOP_KEYS):
        raise ManifestError(f"gate/law.json: unknown key {key!r}")
    if data["schema"] != 1:
        raise ManifestError(f"gate/law.json: schema {data['schema']}, want 1")
    if type(data["note"]) is not str:
        raise ManifestError("gate/law.json: note is not a string")
    paths = data["paths"]
    if type(paths) is not list or not paths:
        raise ManifestError("gate/law.json: paths is not a non-empty list")
    seen: set[str] = set()
    return Manifest(tuple(_entry(i, item, seen) for i, item in enumerate(paths)))


def entry_for(m: Manifest, file: str) -> Path | None:
    """The entry whose path is the file, else the longest prefix entry (a path ending "/") covering it."""
    best: Path | None = None
    for entry in m.paths:
        if entry.path == file:
            return entry
        if (
            entry.path.endswith("/")
            and file.startswith(entry.path)
            and (best is None or len(entry.path) > len(best.path))
        ):
            best = entry
    return best


def names(items: Sequence[str]) -> str:
    """Up to 6 items; more than 6 show the first 3 and an ellipsis."""
    return ", ".join(items) if len(items) <= 6 else ", ".join(items[:3]) + ", …"


def line(path: str, added: int, removed: int, one_liner: str) -> str:
    return f"{path} +{added}/−{removed}" + (f" — {one_liner}" if one_liner else "")


def block(base_short: str, data_lines: Sequence[str], grouped: Mapping[str, tuple[int, int, int]]) -> list[str]:
    """The block: the data lines (git's path order) first, then one line per grouped entry (manifest order)."""
    grouped_lines = [
        f"{entry}: {files} file{'s' if files != 1 else ''} +{a}/−{b}" for entry, (files, a, b) in grouped.items()
    ]
    if not data_lines and not grouped_lines:
        return [NONE]
    n = len(data_lines) + sum(files for files, _, _ in grouped.values())
    return [f"LAW Δ ({n} file{'s' if n != 1 else ''}, vs {base_short})", *data_lines, *grouped_lines]


def _heading_in(lines: Sequence[str], start: int, end: int) -> str:
    """The first heading within lines[start:end] (the break-it's finding 1: a new section appended at the end)."""
    for text in lines[start:end]:
        found = HEADING.match(text)
        if found:
            numbered = NUMBERED.match(found.group(1))
            return f"§{numbered.group(1)}" if numbered else found.group(1)
    return ""


def _heading_above(lines: Sequence[str], index: int) -> str:
    """The nearest heading at or before `index` (a changed line that is itself a heading names its own section —
    the break-it's finding 1): "§<n>" for a numbered one, else its text; "" when none."""
    for text in reversed(lines[: index + 1]):
        found = HEADING.match(text)
        if found:
            numbered = NUMBERED.match(found.group(1))
            return f"§{numbered.group(1)}" if numbered else found.group(1)
    return ""


def doc_line(old: str | None, new: str | None, flag: str) -> str:
    """The heading above the first changed line, the flag, and the hunk count (owner, 2026-11-11, decision 3)."""
    if old is None or new is None:
        heading = "(file added)" if old is None else "(file removed)"
        return " — ".join(x for x in (heading, flag) if x)
    old_lines, new_lines = old.splitlines(), new.splitlines()
    matcher = difflib.SequenceMatcher(None, old_lines, new_lines, autojunk=False)
    hunks = [op for op in matcher.get_opcodes() if op[0] != "equal"]
    heading = ""
    if hunks:
        tag, i1, i2, j1, j2 = hunks[0]
        lines, start, end = (old_lines, i1, i2) if tag == "delete" else (new_lines, j1, j2)
        # a heading inside the first hunk (an added or renamed section) names its own section; else the one above
        heading = _heading_in(lines, start, end) or _heading_above(lines, start)
    count = f" (first of {len(hunks)} hunks)" if len(hunks) > 1 else ""
    return " — ".join(x for x in (heading, flag) if x) + count


# ------------------------------------------------------------------------------------------------------- the parsers

PARSE_FAULTS = (ValueError, KeyError, TypeError, AttributeError)


def _s(n: int) -> str:
    return "" if n == 1 else "s"


def _show(value: object) -> str:
    """A field's value in a one-liner: lists comma-joined, booleans as JSON writes them."""
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, list):
        return ", ".join(_show(v) for v in value)
    return str(value)


def _by(items: Sequence[dict], key: str) -> dict[str, dict]:
    return {str(item[key]): item for item in items}


def _edges(data: dict) -> set[str]:
    edges = set()
    for section, arrow in (
        ("imports", " → "),
        ("test_imports", " →test "),
        ("external", " → ext:"),
        ("test_external", " →test ext:"),
    ):
        for pkg, targets in data.get(section, {}).items():
            edges.update(f"{pkg}{arrow}{imp}" for imp in targets)
    return edges


def _fences(data: dict) -> dict[tuple[str, str], tuple[str, ...]]:
    fences = {}
    for section in ("fences", "test_fences"):
        for pkg, imports in data.get(section, {}).items():
            for imp, prefixes in imports.items():
                fences[(pkg, imp)] = tuple(prefixes)
    return fences


def _delta_edges(old: dict, new: dict) -> str:
    items = []
    added, removed = sorted(_edges(new) - _edges(old)), sorted(_edges(old) - _edges(new))
    if added:
        items.append(
            f"+{len(added)} edge{_s(len(added))} ({names(added)})" if len(added) > 1 else f"+1 edge {added[0]}"
        )
    if removed:
        items.append(
            f"−{len(removed)} edge{_s(len(removed))} ({names(removed)})"
            if len(removed) > 1
            else f"−1 edge {removed[0]}"
        )
    old_f, new_f = _fences(old), _fences(new)
    for (pkg, imp), new_prefixes in new_f.items():
        old_prefixes = old_f.get((pkg, imp), ())
        if old_prefixes != new_prefixes:
            items.append(f"fence {pkg} ← {imp}: {', '.join(old_prefixes)} → {', '.join(new_prefixes)}")
    for (pkg, imp), old_prefixes in old_f.items():
        if (pkg, imp) not in new_f:
            items.append(f"fence {pkg} ← {imp}: {', '.join(old_prefixes)} → ")
    if old.get("module") != new.get("module"):
        items.append(f"module {old.get('module')} → {new.get('module')}")
    if old.get("note") != new.get("note"):
        items.append("note changed")
    return "; ".join(items)


ROW_FIELDS = (
    "id",
    "job",
    "tiers",
    "blocking",
    "timeout_s",
    "needs",
    "scope",
    "expected_scope",
    "min_wants",
    "lands_in",
    "exclusive",
    "command",
)


def _delta_registry(old: dict, new: dict) -> str:
    items = []
    old_rows, new_rows = _by(old["rows"], "id"), _by(new["rows"], "id")
    for row_id, row in new_rows.items():
        if row_id not in old_rows:
            items.append(f"+1 row {row_id} ({_show(row['tiers'])}, {row['job']})")
    for row_id in old_rows:
        if row_id not in new_rows:
            items.append(f"−1 row {row_id}")
    for row_id, row in new_rows.items():
        before = old_rows.get(row_id)
        if before is None:
            continue
        changed = [
            f"{f} {_show(before.get(f))} → {_show(row.get(f))}" for f in ROW_FIELDS if before.get(f) != row.get(f)
        ]
        if changed:
            items.append(f"row {row_id}: {'; '.join(changed)}")
    return "; ".join(items)


def _exports(text: str) -> dict[str, str]:
    """identity ("pkg.ident") → its whole text; a line starting with whitespace or "}" continues the previous one."""
    found: dict[str, str] = {}
    current = ""
    for text_line in text.splitlines():
        if current and (not text_line or text_line[0] in " \t}"):
            found[current] += "\n" + text_line
            continue
        pkg, ident, rest = text_line.split(" ", 2)
        current = f"{pkg}.{ident}"
        found[current] = rest
    return found


def _delta_exports(old: dict[str, str], new: dict[str, str]) -> str:
    items = []
    added, removed = sorted(set(new) - set(old)), sorted(set(old) - set(new))
    changed = sorted(k for k in new if k in old and old[k] != new[k])
    if added:
        items.append(f"+{len(added)} export{_s(len(added))} ({names(added)})")
    if removed:
        items.append(f"−{len(removed)} export{_s(len(removed))} ({names(removed)})")
    if changed:
        items.append(f"{len(changed)} signature{_s(len(changed))} changed ({names(changed)})")
    return "; ".join(items)


def _delta_deps(old: dict, new: dict) -> str:
    items = []
    old_m, new_m = _by(old["modules"], "module"), _by(new["modules"], "module")
    for module, entry in new_m.items():
        if module not in old_m:
            items.append(f"+1 module {module} {entry['version']} ({entry['license']})")
    for module in old_m:
        if module not in new_m:
            items.append(f"−1 module {module}")
    for module, entry in new_m.items():
        before = old_m.get(module)
        if before is None:
            continue
        if before["version"] != entry["version"]:
            items.append(f"{module}: {before['version']} → {entry['version']}")
        if before["license"] != entry["license"]:
            items.append(f"{module}: license {before['license']} → {entry['license']}")
    if old.get("note") != new.get("note"):
        items.append("note changed")
    return "; ".join(items)


def _delta_ceilings(old: dict, new: dict) -> str:
    items = []
    for metric, after in new.items():
        before = old.get(metric)
        if before is None:
            continue
        if before.get("ceiling") != after.get("ceiling"):
            items.append(f"{metric}: ceiling {_show(before.get('ceiling'))} → {_show(after.get('ceiling'))}")
        if before.get("blocking") != after.get("blocking"):
            items.append(f"{metric}: blocking {_show(before.get('blocking'))} → {_show(after.get('blocking'))}")
        old_rules, new_rules = before.get("rules", {}), after.get("rules", {})
        for rule in sorted(set(old_rules) | set(new_rules)):
            if old_rules.get(rule) != new_rules.get(rule):
                items.append(
                    f"{metric}[{rule}]: {_show(old_rules.get(rule, 'none'))} → {_show(new_rules.get(rule, 'none'))}"
                )
        if before.get("reason") != after.get("reason"):
            items.append(f"{metric}: reason changed")
    items += [f"+metric {metric}" for metric in new if metric not in old]
    items += [f"−metric {metric}" for metric in old if metric not in new]
    return "; ".join(items)


def _block_name(b: object) -> str:
    return f"rule {b.rule}, block {b.ordinal}"


def _delta_baseline(old: object, new: object) -> str:
    items = []
    old_b, new_b = {b.key: b for b in old.blocks}, {b.key: b for b in new.blocks}
    redone = [
        _block_name(b)
        for key, b in new_b.items()
        if key in old_b and (old_b[key].outputs, old_b[key].counts) != (b.outputs, b.counts)
    ]
    added = [_block_name(b) for key, b in new_b.items() if key not in old_b]
    removed = [_block_name(b) for key, b in old_b.items() if key not in new_b]
    if redone:
        items.append(f"{len(redone)} block{_s(len(redone))} re-recorded ({names(redone)})")
    if added:
        items.append(f"+{len(added)} block{_s(len(added))} ({names(added)})")
    if removed:
        items.append(f"−{len(removed)} block{_s(len(removed))} ({names(removed)})")
    if old.source_sha256 != new.source_sha256:
        items.append("source changed")
    if old.spec_rev != new.spec_rev:
        items.append(f"spec_rev {old.spec_rev} → {new.spec_rev}")
    if old.format != new.format:
        items.append(f"format {old.format} → {new.format}")
    return "; ".join(items)


def _delta_notbuilt(old: dict, new: dict) -> str:
    items = []
    old_e, new_e = _by(old["entries"], "file"), _by(new["entries"], "file")
    for file, entry in new_e.items():
        if file not in old_e:
            items.append(f"+1 entry {file} ({entry['sprint']})")
    for file in old_e:
        if file not in new_e:
            items.append(f"−1 entry {file}")
    for file, entry in new_e.items():
        before = old_e.get(file)
        if before is None:
            continue
        if before["sprint"] != entry["sprint"]:
            items.append(f"{file}: {before['sprint']} → {entry['sprint']}")
        if before["construct"] != entry["construct"]:
            items.append(f"{file}: construct changed")
    return "; ".join(items)


def _delta_tools(old: dict, new: dict) -> str:
    items = []
    old_t, new_t = _by(old["tools"], "name"), _by(new["tools"], "name")
    for name, tool in new_t.items():
        before = old_t.get(name)
        if before is None:
            items.append(f"+tool {name}")
        elif before.get("version") != tool.get("version"):
            items.append(f"{name}: {before.get('version')} → {tool.get('version')}")
        elif before != tool:
            items.append(f"{name}: hashes changed")
    items += [f"−tool {name}" for name in old_t if name not in new_t]
    return "; ".join(items)


def _delta_marker(old: str, new: str) -> str:
    a, b = old.strip(), new.strip()
    return f"marker {a} → {b}" if a != b else ""


def _delta_sprints(old: dict, new: dict) -> str:
    items = []
    old_rows, new_rows = _by(old["rows"], "id"), _by(new["rows"], "id")
    for row_id, row in new_rows.items():
        before = old_rows.get(row_id)
        if before is not None and before["closed"] != row["closed"]:
            items.append(f"{row_id} closed" if row["closed"] else f"{row_id} reopened")
    for i, row_id in enumerate(new_rows):
        if row_id not in old_rows:
            items.append(f"+1 row {row_id} at {i}")
    items += [f"−1 row {row_id}" for row_id in old_rows if row_id not in new_rows]
    common_old = [r for r in old_rows if r in new_rows]
    common_new = [r for r in new_rows if r in old_rows]
    if common_old != common_new:
        items.append("order changed")
    return "; ".join(items)


def _delta_law(old: dict, new: dict) -> str:
    items = []
    old_p, new_p = _by(old["paths"], "path"), _by(new["paths"], "path")
    for path, entry in new_p.items():
        before = old_p.get(path)
        if before is None:
            items.append(f"+1 path {path} ({entry['kind']})")
        elif before["kind"] != entry["kind"]:
            items.append(f"{path}: kind {before['kind']} → {entry['kind']}")
        elif before.get("flag") != entry.get("flag"):
            items.append(f"{path}: flag changed")
    items += [f"−1 path {path}" for path in old_p if path not in new_p]
    return "; ".join(items)


def _baseline(text: str) -> object:
    from gate.baseline import regen  # the file's own reader (its content key); imported here to keep law.py light

    return regen.parse(text)


PARSERS: dict[str, tuple] = {
    # kind: (how a side's text is read, the comparison of two read sides)
    "edges": (loads, _delta_edges),
    "registry": (loads, _delta_registry),
    "exports": (_exports, _delta_exports),
    "deps": (loads, _delta_deps),
    "ceilings": (loads, _delta_ceilings),
    "baseline": (_baseline, _delta_baseline),
    "notbuilt": (loads, _delta_notbuilt),
    "tools": (loads, _delta_tools),
    "marker": (str, _delta_marker),
    "sprints": (loads, _delta_sprints),
    "law": (loads, _delta_law),
}


def delta(kind: str, old: str | None, new: str | None) -> str:
    """One changed file's semantic one-liner; "" for a grouped kind. Never raises: a side that cannot be read says
    "unparseable (old)" or "unparseable (new)"; a side that is absent reads "file added" / "file removed"."""
    if kind not in PARSERS:
        return ""
    read, compare = PARSERS[kind]
    if old is None or new is None:
        if kind == "baseline" and old is None:
            try:
                return f"file added: {len(read(new).blocks)} blocks"
            except PARSE_FAULTS:
                return "unparseable (new)"
        return "file added" if old is None else "file removed"
    try:
        before = read(old)
    except PARSE_FAULTS:
        return "unparseable (old)"
    try:
        after = read(new)
    except PARSE_FAULTS:
        return "unparseable (new)"
    try:
        return compare(before, after)
    except PARSE_FAULTS:
        return "unparseable (new)"
