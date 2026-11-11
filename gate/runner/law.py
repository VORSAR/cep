"""The law manifest and the semantic parsers (S1.1c-1 design §2.3; ADR-0015 §1.3).

Pure: texts in, lines out, no I/O. `load_manifest` reads gate/law.json's text strictly (a duplicate key, an extra
or missing key, a flag on a non-doc, an unsafe path are each a ManifestError naming the entry). `delta` gives one
changed law file's one-line semantic delta and never raises: a side its parser cannot read says so. `block`
renders the LAW Δ block: the data files and documents one line each, then the code and workflow entries grouped
(owner, 2026-11-11, decision 4).
"""

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
    grouped_lines = [f"{entry}: {files} files +{a}/−{b}" for entry, (files, a, b) in grouped.items()]
    if not data_lines and not grouped_lines:
        return [NONE]
    n = len(data_lines) + sum(files for files, _, _ in grouped.values())
    return [f"LAW Δ ({n} files, vs {base_short})", *data_lines, *grouped_lines]


def doc_line(old: str | None, new: str | None, flag: str) -> str:
    """The heading above the first changed line, the flag, and the hunk count (owner, 2026-11-11, decision 3)."""
    return ""


def delta(kind: str, old: str | None, new: str | None) -> str:
    """One changed file's semantic one-liner; "" for a grouped kind. Never raises."""
    return ""
