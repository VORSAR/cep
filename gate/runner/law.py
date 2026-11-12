"""The law manifest and the semantic parsers (S1.1c-1 design §2.3; ADR-0015 §1.3).

Pure: texts in, lines out, no I/O. `load_manifest` reads gate/law.json's text strictly (a duplicate key, an extra
or missing key, a flag on a non-doc, an unsafe path are each a ManifestError naming the entry). `delta` gives one
changed law file's one-line semantic delta and never raises: a side its parser cannot read says so; the list kinds
(`allowlist`, `known-issues`) give a summary and one sub-line per changed entry instead (S1.1c-2 design §2.5).
`block` renders the LAW Δ block: the data files and documents one line each (a list kind's sub-lines indented under
it, at most 6), then the code and workflow entries grouped (owner, 2026-11-11, decision 4). A document's line names the
first three distinct headings its changed hunks sit under, and counts the hunks that only add or remove a marker
comment (S1.1c-2 design §4.5).
"""

import difflib
import json
import re
from collections import Counter
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from gate.runner import anchors

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
        "allowlist",
        "known-issues",
        "gomod",
    }
)
LISTED = frozenset({"allowlist", "known-issues"})  # kinds whose delta is [summary, *sub-lines] (design §2.5)
SUB_LINES_MAX = 6
GROUPED = frozenset({"code", "workflow"})
TOP_KEYS = frozenset({"schema", "note", "paths", "instruments"})
INSTRUMENT_REQUIRED = ("id", "row", "lands_in", "code", "proofs")
INSTRUMENT_OPTIONAL = ("tiers", "min_wants", "expected_scope", "go_test", "exempt")
PROOF_REQUIRED = ("path", "cases")
NUMBERED = re.compile(r"^(\d+)\. ")
HEADINGS_SHOWN = 3
# A rule's marker comment (<!--cep…-->, <!--text-->); S1.1c-3's rule markers share this one constant (DA-L8).
MARKER_COMMENT = re.compile(r"<!--(?:cep[^<>]*?|text)-->")
NONE = "LAW Δ: none"


@dataclass(frozen=True)
class Path:
    path: str
    kind: str
    flag: str = ""


@dataclass(frozen=True)
class Proof:
    """A proof file and the cases the teeth count in it; `test` counts only that Go test function and its subtests."""

    path: str
    cases: int
    test: str = ""


@dataclass(frozen=True)
class Instrument:
    """What gives a row or a Go test its teeth (S1.1c-2 design §4.2): its code, its proofs and their counted cases.

    A row instrument pins its row's tiers, min_wants and expected_scope; a Go-test instrument names its go_test;
    `exempt` names a tools/allowlists/teeth.json entry in place of proofs."""

    id: str
    row: str
    lands_in: str
    code: tuple[str, ...]
    proofs: tuple[Proof, ...]
    tiers: tuple[str, ...] = ()
    min_wants: int = -1
    expected_scope: tuple[str, ...] = ()
    go_test: str = ""
    exempt: str = ""


@dataclass(frozen=True)
class Manifest:
    paths: tuple[Path, ...]
    instruments: tuple[Instrument, ...] = ()


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


def _safe(path: object) -> bool:
    return type(path) is str and bool(path) and not path.startswith("/") and ".." not in path.split("/")


def _strings(value: object) -> bool:
    return type(value) is list and all(type(v) is str and v for v in value)


def _proof(item: object, fault) -> Proof:
    if type(item) is not dict:
        raise fault("a proof is not an object")
    keys = set(item)
    if keys not in (set(PROOF_REQUIRED), {*PROOF_REQUIRED, "test"}):
        raise fault(f"a proof's keys {', '.join(sorted(keys))}: want path, cases and, for a Go test, test")
    if not _safe(item["path"]):
        raise fault(f"proof path {item['path']!r} must be a non-empty repository path, without ..")
    if type(item["cases"]) is not int or item["cases"] < 0:
        raise fault(f"proof {item['path']}: cases is not a whole number")
    if "test" in item and (type(item["test"]) is not str or not item["test"]):
        raise fault(f"proof {item['path']}: test is not a non-empty string")
    return Proof(item["path"], item["cases"], item.get("test", ""))


def _instrument(i: int, item: object, seen: set[str]) -> Instrument:
    def fault(what: str) -> ManifestError:
        return ManifestError(f"gate/law.json: instruments[{i}]: {what}")

    if type(item) is not dict:
        raise fault("not an object")
    for key in sorted(item):
        if key not in INSTRUMENT_REQUIRED and key not in INSTRUMENT_OPTIONAL:
            raise fault(f"unknown key {key!r}")
    for key in INSTRUMENT_REQUIRED:
        if key not in item:
            raise fault(f"missing key {key!r}")
    for key in ("id", "row", "lands_in", "go_test", "exempt"):
        if key in item and (type(item[key]) is not str or not item[key]):
            raise fault(f"{key} is not a non-empty string")
    if item["id"] in seen:
        raise fault(f"id {item['id']!r} is listed twice")
    for key in ("code", "tiers", "expected_scope"):
        if key in item and not _strings(item[key]):
            raise fault(f"{key} is not a list of non-empty strings")
    for path in item["code"]:
        if not _safe(path):
            raise fault(f"code path {path!r} must be relative to the repository, without ..")
    if "min_wants" in item and (type(item["min_wants"]) is not int or item["min_wants"] < 0):
        raise fault("min_wants is not a whole number")
    if type(item["proofs"]) is not list:
        raise fault("proofs is not a list")
    if item["proofs"] and "exempt" in item:
        raise fault("proofs and exempt together")
    seen.add(item["id"])
    return Instrument(
        item["id"],
        item["row"],
        item["lands_in"],
        tuple(item["code"]),
        tuple(_proof(p, fault) for p in item["proofs"]),
        tiers=tuple(item.get("tiers", ())),
        min_wants=item.get("min_wants", -1),
        expected_scope=tuple(item.get("expected_scope", ())),
        go_test=item.get("go_test", ""),
        exempt=item.get("exempt", ""),
    )


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
    entries = tuple(_entry(i, item, seen) for i, item in enumerate(paths))
    if type(data["instruments"]) is not list:
        raise ManifestError("gate/law.json: instruments is not a list")
    ids: set[str] = set()
    return Manifest(entries, tuple(_instrument(i, item, ids) for i, item in enumerate(data["instruments"])))


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


def capped(sub_lines: Sequence[str]) -> list[str]:
    """Sub-lines indented two spaces, at most 6, then `  (and <k> more)` (DA-S11)."""
    shown = [f"  {text}" for text in sub_lines[:SUB_LINES_MAX]]
    more = len(sub_lines) - SUB_LINES_MAX
    return shown + ([f"  (and {more} more)"] if more > 0 else [])


def block(
    base_short: str, data_lines: Sequence[str | Sequence[str]], grouped: Mapping[str, tuple[int, int, int]]
) -> list[str]:
    """The block: the data lines (git's path order) first, then one line per grouped entry (manifest order).

    A data line is a file's line, or a list kind's [file line, *sub-lines], rendered with its sub-lines `capped`."""
    grouped_lines = [
        f"{entry}: {files} file{'s' if files != 1 else ''} +{a}/−{b}" for entry, (files, a, b) in grouped.items()
    ]
    if not data_lines and not grouped_lines:
        return [NONE]
    rendered: list[str] = []
    for item in data_lines:
        rendered += [item] if isinstance(item, str) else [item[0], *capped(item[1:])]
    n = len(data_lines) + sum(files for files, _, _ in grouped.values())
    return [f"LAW Δ ({n} file{'s' if n != 1 else ''}, vs {base_short})", *rendered, *grouped_lines]


def _section(text: str) -> str:
    """A heading's name in a line: "§<n>" for a top-level numbered one (`5. …`), else its text (`5.2 …` stays its
    text, §11.14)."""
    numbered = NUMBERED.match(text)
    return f"§{numbered.group(1)}" if numbered else text


def _headings(lines: Sequence[str]) -> tuple[dict[int, str], list[str]]:
    """The heading lines read outside fences (DA-L11) by index, and per line the section it sits in: the nearest
    heading at or above it ("" above the first)."""
    found = {}
    for number, text in anchors.prose(list(lines)):
        m = anchors.HEADING.match(text)
        if m and m.group(1):
            found[number - 1] = _section(m.group(1))
    heads, current = [], ""
    for i in range(len(lines)):
        current = found.get(i, current)
        heads.append(current)
    return found, heads


def _own_heading(found: Mapping[int, str], heads: Sequence[str], start: int, end: int) -> str:
    """A heading inside the hunk names its own section (an added or renamed one; the break-it's finding 1); else
    the section the hunk's first line sits in."""
    for i in range(start, end):
        if i in found:
            return found[i]
    return heads[start] if start < len(heads) else (heads[-1] if heads else "")


def _unmarked(lines: Sequence[str]) -> list[str]:
    """The lines with their marker comments removed; a line that held only markers drops out."""
    out = []
    for text in lines:
        bare = MARKER_COMMENT.sub("", text).rstrip()
        if bare or not MARKER_COMMENT.search(text):
            out.append(bare)
    return out


def _markers(old: Sequence[str], new: Sequence[str]) -> int:
    """The marker comments a marker-only hunk adds or removes."""
    a = Counter(m for text in old for m in MARKER_COMMENT.findall(text))
    b = Counter(m for text in new for m in MARKER_COMMENT.findall(text))
    return sum(((a - b) + (b - a)).values())


def doc_line(old: str | None, new: str | None, flag: str) -> str:
    """`<headings>[ (and <k> more)] — <flag>[; +<n> markers]` (S1.1c-2 design §4.5, §11.13): the first three distinct
    headings of the hunks that change text, and the marker comments that the marker-only hunks add or remove."""
    if old is None or new is None:
        heading = "(file added)" if old is None else "(file removed)"
        return " — ".join(x for x in (heading, flag) if x)
    old_lines, new_lines = old.splitlines(), new.splitlines()
    (old_found, old_heads), (new_found, new_heads) = _headings(old_lines), _headings(new_lines)
    matcher = difflib.SequenceMatcher(None, old_lines, new_lines, autojunk=False)
    names: list[str] = []
    markers = 0
    for tag, i1, i2, j1, j2 in matcher.get_opcodes():
        if tag == "equal":
            continue
        before, after = old_lines[i1:i2], new_lines[j1:j2]
        if _unmarked(before) == _unmarked(after) and _markers(before, after):
            markers += _markers(before, after)
            continue
        if tag == "delete":
            heading = _own_heading(old_found, old_heads, i1, i2)
        else:
            heading = _own_heading(new_found, new_heads, j1, j2)
        if heading and heading not in names:
            names.append(heading)
    shown = ", ".join(names[:HEADINGS_SHOWN])
    if len(names) > HEADINGS_SHOWN:
        shown += f" (and {len(names) - HEADINGS_SHOWN} more)"
    text = " — ".join(x for x in (shown, flag) if x)
    return text + (f"; +{markers} markers" if markers else "")


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


GOMOD_BLOCKS = frozenset({"require", "replace", "tool", "exclude", "retract", "godebug", "ignore"})


@dataclass
class GoMod:
    """What a go.mod says that the gomod kind names: the scalars, the requires (module → version), the replaces
    (the left side → the target) and the tools; `// indirect` and every other comment are dropped."""

    scalars: dict[str, str]
    requires: dict[str, str]
    replaces: dict[str, str]
    tools: set[str]


def _gomod(text: str) -> GoMod:
    mod = GoMod({}, {}, {}, set())
    block = ""
    for raw in text.splitlines():
        words = raw.split("//", 1)[0].split()
        if not words:
            continue
        if block:
            if words == [")"]:
                block = ""
                continue
            verb, args = block, words
        elif len(words) == 2 and words[0] in GOMOD_BLOCKS and words[1] == "(":
            block = words[0]
            continue
        else:
            verb, args = words[0], words[1:]
        if verb in ("module", "go", "toolchain"):
            if len(args) != 1:
                raise ValueError(f"{verb}: {' '.join(args)}")
            mod.scalars[verb] = args[0]
        elif verb == "require":
            if len(args) != 2:
                raise ValueError(f"require: {' '.join(args)}")
            mod.requires[args[0]] = args[1]
        elif verb == "replace":
            if "=>" not in args:
                raise ValueError(f"replace: {' '.join(args)}")
            at = args.index("=>")
            mod.replaces[" ".join(args[:at])] = " ".join(args[at + 1 :])
        elif verb == "tool":
            if len(args) != 1:
                raise ValueError(f"tool: {' '.join(args)}")
            mod.tools.add(args[0])
    if block:
        raise ValueError(f"an unclosed {block} block")
    return mod


def _delta_gomod(old: GoMod, new: GoMod) -> str:
    items = []
    for key in ("module", "go", "toolchain"):
        a, b = old.scalars.get(key, "none"), new.scalars.get(key, "none")
        if a != b:
            items.append(f"{key} {a} → {b}")
    items += [f"+require {m} {new.requires[m]}" for m in sorted(set(new.requires) - set(old.requires))]
    items += [
        f"{m} {old.requires[m]} → {new.requires[m]}"
        for m in sorted(set(new.requires) & set(old.requires))
        if old.requires[m] != new.requires[m]
    ]
    items += [f"−require {m}" for m in sorted(set(old.requires) - set(new.requires))]
    items += [f"+replace {m} => {new.replaces[m]}" for m in sorted(set(new.replaces) - set(old.replaces))]
    items += [
        f"replace {m} => {old.replaces[m]} → {new.replaces[m]}"
        for m in sorted(set(new.replaces) & set(old.replaces))
        if old.replaces[m] != new.replaces[m]
    ]
    items += [f"−replace {m}" for m in sorted(set(old.replaces) - set(new.replaces))]
    items += [f"+tool {t}" for t in sorted(new.tools - old.tools)]
    items += [f"−tool {t}" for t in sorted(old.tools - new.tools)]
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
    old_i, new_i = _by(old.get("instruments", []), "id"), _by(new.get("instruments", []), "id")
    items += [f"+instrument {id}" for id in new_i if id not in old_i]
    items += [f"−instrument {id}" for id in old_i if id not in new_i]
    for id, after in new_i.items():
        before = old_i.get(id)
        if before is None:
            continue
        changed = [
            f"{f} {_instrument_field(before, f)} → {_instrument_field(after, f)}"
            for f in INSTRUMENT_FIELDS
            if _instrument_field(before, f) != _instrument_field(after, f)
        ]
        if changed:
            items.append(f"instrument {id}: {'; '.join(changed)}")
    return "; ".join(items)


INSTRUMENT_FIELDS = ("row", "lands_in", "code", "proofs", "tiers", "min_wants", "expected_scope", "go_test", "exempt")
INSTRUMENT_DEFAULTS: dict[str, object] = {"min_wants": -1, "go_test": "", "exempt": ""}


def _instrument_field(entry: dict, field: str) -> str:
    """A field as a LAW Δ line shows it: a list comma-joined (§11.16), a proof as <path>[ <test>]: <cases>."""
    value = entry.get(field, INSTRUMENT_DEFAULTS.get(field, []))
    if field == "proofs":
        return ", ".join(f"{p['path']}{' ' + p['test'] if p.get('test') else ''}: {p['cases']}" for p in value)
    return _show(value)


LIST_FIELDS = ("kind", "sprint", "ref", "expiry", "issue")


def _entries(data: dict, key: str) -> dict[str, dict]:
    if type(data) is not dict or type(data.get("entries")) is not list:
        raise ValueError("entries is not a list")
    return _by(data["entries"], key)


def _listed(old: dict | None, new: dict | None, key: str, added, changed) -> list[str]:
    """[summary, *sub-lines]: added (by key), changed, removed (§10.14); the summary counts the new side's entries."""
    before = _entries(old, key) if old is not None else {}
    after = _entries(new, key) if new is not None else {}
    subs = [added(name, after[name]) for name in sorted(set(after) - set(before))]
    for name in sorted(set(after) & set(before)):
        subs += changed(name, before[name], after[name])
    subs += [f"−{name}" for name in sorted(set(before) - set(after))]
    if old is None:
        return [f"file added: {len(after)} entries", *subs]
    if new is None:
        return [f"file removed: {len(before)} entries", *subs]
    return [f"({len(after)} entries)", *subs]


def _allowlist_added(target: str, e: dict) -> str:
    fields = [f"{k}={e[k]}" for k in ("ref", "expiry", "issue") if e.get(k)]
    return " ".join([f"+{target}", str(e.get("kind", "")), str(e.get("sprint", "")), *fields])


def _allowlist_changed(target: str, old: dict, new: dict) -> list[str]:
    subs = [
        f"{target}: {k} {old.get(k, '')} → {new.get(k, '')}" for k in LIST_FIELDS if old.get(k, "") != new.get(k, "")
    ]
    return subs + ([f"{target}: reason changed"] if old.get("reason") != new.get("reason") else [])


def _issue_added(id: str, e: dict) -> str:
    return f"+{id} first seen {e.get('first_seen', '')}"


def _issue_changed(id: str, old: dict, new: dict) -> list[str]:
    subs = []
    if old.get("first_seen") != new.get("first_seen"):
        subs.append(f"{id}: first seen {old.get('first_seen', '')} → {new.get('first_seen', '')}")
    if old.get("forward") != new.get("forward"):
        subs.append(f"{id}: forward {old.get('forward', '')} → {new.get('forward', '')}")
    a, b = old.get("decision", ""), new.get("decision", "")
    if a != b:
        subs.append(f"{id}: decision {'set' if not a else 'cleared' if not b else 'changed'}")
    return subs


LIST_PARSERS: dict[str, tuple] = {
    # kind: (the entries' key, an added entry's sub-line, a changed entry's sub-lines)
    "allowlist": ("target", _allowlist_added, _allowlist_changed),
    "known-issues": ("id", _issue_added, _issue_changed),
}


def listed_delta(kind: str, old: str | None, new: str | None) -> list[str]:
    """A list kind's [summary, *sub-lines]; never raises: a side that cannot be read is ["unparseable (old|new)"]."""
    key, added, changed = LIST_PARSERS[kind]
    sides = []
    for side, text in (("old", old), ("new", new)):
        try:
            sides.append(None if text is None else loads(text))
        except PARSE_FAULTS:
            return [f"unparseable ({side})"]
    try:
        return _listed(sides[0], sides[1], key, added, changed)
    except PARSE_FAULTS:
        return ["unparseable (new)"]


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
    "gomod": (_gomod, _delta_gomod),
}


def delta(kind: str, old: str | None, new: str | None) -> str | list[str]:
    """One changed file's semantic one-liner; "" for a grouped kind; a list kind's [summary, *sub-lines]. Never
    raises: a side that cannot be read says "unparseable (old)" or "unparseable (new)"; a side that is absent reads
    "file added" / "file removed"."""
    if kind in LIST_PARSERS:
        return listed_delta(kind, old, new)
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
