"""The exemption grammar's lists and its one set of field validators (S1.1c-2 design §2.2, §2.8).

A list is `tools/allowlists/<rule>.json`: `{"schema": 1, "note": str, "rule": <rule>, "entries": [...]}`, each entry
`target`, `kind`, `reason`, `sprint` and, by kind, `ref` (PERMANENT) or `expiry` and `issue` (TEMP). `load` reads the
shape; `fields` holds the field rules, and it is their only implementation: the suppressions row applies it to the
lists and to every in-code directive, and the deps register calls `sprint_fault` and `ref_fault` (DA-S1). Pure: the
caller builds `Ctx` (the day, the tier, the sprint order, the marker, the known issues, a ref resolver), so a test
injects all of it and never reads the clock or the law files (DQ-S9).
"""

import json
import re
from collections.abc import Callable, Iterable
from dataclasses import dataclass
from datetime import date

from gate.runner import anchors, sprintid

# "fragments" from S1.1c-3 (the fragment test's register); "vulns" from S1.1c-4 (the vulns row's dated exemptions;
# owner 2026-11-13, S1.1c-4's design, after the reviews: the rule and file are named vulns, not govulncheck).
RULES = frozenset({"purity", "clock", "teeth", "ruff", "testhygiene", "generated", "fragments", "vulns"})
KINDS = ("PERMANENT", "TEMP")
TOP_KEYS = ("schema", "note", "rule", "entries")
REQUIRED = ("target", "kind", "reason", "sprint")
OPTIONAL = ("ref", "expiry", "issue")
REASON_MIN = 12
AHEAD_MAX = 90
COUNTDOWN = 14
RED_EXPIRED = frozenset({"T4", "nightly"})
ISO_DAY = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}")


@dataclass(frozen=True)
class Ctx:
    today: date
    tier: str
    order: sprintid.Order
    current: sprintid.ID
    issues: frozenset[str]
    resolve: Callable[[str], str]


@dataclass(frozen=True)
class Entry:
    target: str
    kind: str
    reason: str
    sprint: str
    ref: str = ""
    expiry: str = ""
    issue: str = ""


@dataclass(frozen=True)
class AllowList:
    name: str
    rule: str
    entries: tuple[Entry, ...]


class _Duplicate(Exception):
    def __init__(self, key: str) -> None:
        super().__init__(key)
        self.key = key


def _pairs(pairs: list[tuple[str, object]]) -> dict[str, object]:
    out: dict[str, object] = {}
    for key, value in pairs:
        if key in out:
            raise _Duplicate(key)
        out[key] = value
    return out


def _show(value: object) -> str:
    return value if isinstance(value, str) else json.dumps(value, ensure_ascii=False, sort_keys=True)


def _entry(where: str, item: object, seen: set[str]) -> Entry:
    if type(item) is not dict:
        raise ValueError(f"{where}: not an object")
    for key in sorted(item):
        if key not in REQUIRED and key not in OPTIONAL:
            raise ValueError(f"{where}: unknown key {key}")
    for key in REQUIRED:
        if key not in item:
            raise ValueError(f"{where}: missing {key}")
    for key in (*REQUIRED, *OPTIONAL):
        if key in item and type(item[key]) is not str:
            raise ValueError(f"{where}: {key} is not a string")
    target = item["target"]
    if target in seen:
        raise ValueError(f"{where}: target {target} is listed twice")
    seen.add(target)
    return Entry(
        target,
        item["kind"],
        item["reason"],
        item["sprint"],
        ref=item.get("ref", ""),
        expiry=item.get("expiry", ""),
        issue=item.get("issue", ""),
    )


def load(text: str, name: str) -> AllowList:
    """The list's shape, strictly; a ValueError names the file and the fault (§2.8). The fields are `check`'s."""
    where = f"tools/allowlists/{name}.json"
    try:
        data = json.loads(text, object_pairs_hook=_pairs)
    except _Duplicate as e:
        raise ValueError(f"{where}: duplicate key {e.key}") from None
    except ValueError as e:
        raise ValueError(f"{where}: not JSON: {e}") from None
    if type(data) is not dict:
        raise ValueError(f"{where}: not a JSON object")
    if set(data) != set(TOP_KEYS):
        raise ValueError(f"{where}: keys {', '.join(sorted(data))}, want {', '.join(TOP_KEYS)}")
    if type(data["schema"]) is not int or data["schema"] != 1:
        raise ValueError(f"{where}: schema {_show(data['schema'])}, want 1")
    if type(data["note"]) is not str:
        raise ValueError(f"{where}: note is not a string")
    rule = data["rule"]
    if rule not in RULES:
        raise ValueError(f"{where}: rule {_show(rule)} is not one of {', '.join(sorted(RULES))}")
    if rule != name:
        raise ValueError(f"{where}: rule {rule} but the file is {name}.json")
    if type(data["entries"]) is not list:
        raise ValueError(f"{where}: entries is not a list")
    seen: set[str] = set()
    entries = tuple(_entry(f"{where}: entries[{i}]", item, seen) for i, item in enumerate(data["entries"]))
    return AllowList(name, rule, entries)


def _day(text: str) -> date | None:
    if not ISO_DAY.fullmatch(text):
        return None
    try:
        return date.fromisoformat(text)
    except ValueError:
        return None


def sprint_fault(sprint: str, ctx: Ctx) -> str:
    """ "" when the sprint is a row (or a part of one) of the order, not after the current sprint; else why."""
    try:
        ctx.order.index(sprint)
    except ValueError:
        return f"sprint {sprint} is not a row of gate/sprints.json"
    if ctx.order.passed(sprintid.canonical(ctx.current), sprint):  # the part order, shared with aging (decision 8)
        return f"sprint {sprint} is after the current sprint {sprintid.canonical(ctx.current)}"
    return ""


# Where a PERMANENT exemption's ref may point (owner, 2026-11-12, S1.1c-2's review and break-it 2): a recorded decision.
DECISION_DIR = "docs/architecture/adr/"
DECISIONS = "docs/sprints/decisions.md"


def ref_fault(ref: str, ctx: Ctx) -> str:
    """ "" when the ref names a recorded decision — a heading of an ADR or of docs/sprints/decisions.md (CA-S3) —
    that resolves; else why not: the path rule's text, or the resolver's why, verbatim (design §10.6)."""
    try:
        path, _ = anchors.split_ref(ref)
    except ValueError:
        return ctx.resolve(ref)  # the resolver names the form's fault
    if not (path.startswith(DECISION_DIR) or path == DECISIONS):
        return f"ref {path} is not a recorded decision: cite {DECISION_DIR}… or {DECISIONS}#…"
    return ctx.resolve(ref)


def _expiry_faults(expiry: str, ctx: Ctx) -> list[str]:
    day = _day(expiry)
    if day is None:
        return ["expiry must be YYYY-MM-DD"]
    k = (day - ctx.today).days
    if k > AHEAD_MAX:
        return [f"expiry {expiry} is more than {AHEAD_MAX} days ahead"]
    if k < 0 and ctx.tier in RED_EXPIRED:
        return [f"expired {expiry}"]
    return []


def fields(kind: str, reason: str, ref: str, expiry: str, issue: str, sprint: str, ctx: Ctx) -> list[str]:
    """Every field rule's fault for one exemption, in field order; [] when it is valid."""
    faults: list[str] = []
    if kind not in KINDS:
        faults.append("kind must be PERMANENT or TEMP")
    if len(reason.strip()) < REASON_MIN:
        faults.append(f"reason must be at least {REASON_MIN} characters")
    if kind == "PERMANENT":
        if not ref:
            faults.append("PERMANENT needs ref=<path>#<anchor>")
        else:
            why = ref_fault(ref, ctx)
            if why:
                faults.append(why)
        if expiry or issue:
            faults.append("PERMANENT takes no expiry or issue")
    elif kind == "TEMP":
        if not expiry or not issue:
            faults.append("TEMP needs expiry=YYYY-MM-DD and issue=<known issue id>")
        if ref:
            faults.append("TEMP takes no ref")
        if expiry:
            faults += _expiry_faults(expiry, ctx)
        if issue and issue not in ctx.issues:
            faults.append(f"issue {issue} is not in gate/known-issues.json")
    why = sprint_fault(sprint, ctx)
    if why:
        faults.append(why)
    return faults


def check(lst: AllowList, ctx: Ctx) -> list[str]:
    """`fields` for every entry, each fault prefixed with the file, the entry's index and its target."""
    out = []
    for i, e in enumerate(lst.entries):
        prefix = f"tools/allowlists/{lst.name}.json: entries[{i}] ({e.target}): "
        out += [prefix + fault for fault in fields(e.kind, e.reason, e.ref, e.expiry, e.issue, e.sprint, ctx)]
    return out


def expiring(items: Iterable[tuple[str, str]], today: date) -> list[tuple[str, int]]:
    """(label, k) for every (label, expiry) with k = (expiry - today).days ≤ 14, every past one included (§10.19); an
    expiry that is not a date is the field rules' to name, not the countdown's."""
    out = []
    for label, expiry in items:
        day = _day(expiry)
        if day is not None and (k := (day - today).days) <= COUNTDOWN:
            out.append((label, k))
    return out
