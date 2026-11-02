"""The registry: one row per check, loaded strictly (ADR-0015 §1.1; design note §4).

Every rule here is enforced before anything runs; a violation is exit 2, naming the row and the rule.
Python's json module accepts duplicate keys, NaN, and a bool where an int belongs, so each is refused by hand.
"""

import json
import re
from collections.abc import Sequence
from dataclasses import dataclass

SCHEMA = 1
TIERS = ("T0", "T1", "T2", "T3", "T4")
SCOPE_CLASSES = frozenset({"prod", "test", "bench", "tools", "proofs", "generated"})
SCOPE_KINDS = frozenset({"git", "go", "go-vet-units"})
PARITY_ID = "parity"
FILES_TOKEN = "{files}"
TIMEOUT_MIN_S = 1
TIMEOUT_MAX_S = 3600
_NAME = re.compile(r"[a-z][a-z0-9-]*")
_FIELDS = frozenset(
    {
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
    }
)


class RegistryError(Exception):
    """The registry is unreadable or breaks a load rule; the run stops with exit 2."""


@dataclass(frozen=True)
class Row:
    id: str
    job: str
    tiers: tuple[str, ...]
    blocking: bool
    timeout_s: int
    needs: tuple[str, ...]
    scope: tuple[str, ...]
    expected_scope: tuple[str, ...]
    min_wants: int
    lands_in: str
    exclusive: bool
    command: tuple[str, ...]


def _refuse_duplicates(pairs: list[tuple[str, object]]) -> dict[str, object]:
    out: dict[str, object] = {}
    for key, value in pairs:
        if key in out:
            raise RegistryError(f"duplicate key {key!r}")
        out[key] = value
    return out


def _refuse_constant(name: str) -> None:
    raise RegistryError(f"{name} is not JSON")


def _str_list(value: object, where: str, field: str) -> list[str]:
    if not isinstance(value, list) or not all(type(v) is str for v in value):
        raise RegistryError(f"{where}: {field} must be a list of strings")
    return value


def _no_duplicates(items: list[str], where: str, field: str) -> None:
    if len(set(items)) != len(items):
        raise RegistryError(f"{where}: {field} has a duplicate")


def _name(value: object, where: str, field: str) -> str:
    if type(value) is not str or not _NAME.fullmatch(value):
        raise RegistryError(f"{where}: {field} must match ^[a-z][a-z0-9-]*$, got {value!r}")
    return value


def _flag(value: object, where: str, field: str) -> bool:
    if type(value) is not bool:
        raise RegistryError(f"{where}: {field} must be true or false, got {value!r}")
    return value


def _tiers(value: object, where: str) -> list[str]:
    tiers = _str_list(value, where, "tiers")
    if not tiers:
        raise RegistryError(f"{where}: tiers must not be empty")
    _no_duplicates(tiers, where, "tiers")
    for tier in tiers:
        if tier not in TIERS:
            raise RegistryError(f"{where}: tiers holds unknown tier {tier!r}")
    return tiers


def _scope(value: object, where: str) -> list[str]:
    scope = _str_list(value, where, "scope")
    _no_duplicates(scope, where, "scope")
    for cls in scope:
        if cls not in SCOPE_CLASSES:
            raise RegistryError(f"{where}: scope holds unknown class {cls!r}")
    return scope


def _expected_scope(value: object, where: str) -> list[str]:
    scope = _str_list(value, where, "expected_scope")
    if len(scope) < 2:
        raise RegistryError(f"{where}: expected_scope must be a kind and at least one argument")
    if scope[0] not in SCOPE_KINDS:
        raise RegistryError(f"{where}: expected_scope has unknown kind {scope[0]!r}")
    return scope


def _command(value: object, where: str) -> list[str]:
    command = _str_list(value, where, "command")
    if not command:
        raise RegistryError(f"{where}: command must not be empty")
    for arg in command:
        if FILES_TOKEN in arg and arg != FILES_TOKEN:
            raise RegistryError(f"{where}: command must pass {FILES_TOKEN} as a whole argument, got {arg!r}")
    return command


def _row(raw: object, index: int) -> Row:
    if not isinstance(raw, dict):
        raise RegistryError(f"row {index}: must be an object")
    where = f"row {index} ({raw.get('id')!r})"
    missing = sorted(_FIELDS - raw.keys())
    if missing:
        raise RegistryError(f"{where}: missing field(s) {', '.join(missing)}")
    unknown = sorted(raw.keys() - _FIELDS)
    if unknown:
        raise RegistryError(f"{where}: unknown field(s) {', '.join(unknown)}")
    row_id = _name(raw["id"], where, "id")
    if row_id == PARITY_ID:
        raise RegistryError(f"{where}: id {PARITY_ID!r} is the runner's built-in first step")
    timeout_s = raw["timeout_s"]
    if type(timeout_s) is not int or not TIMEOUT_MIN_S <= timeout_s <= TIMEOUT_MAX_S:
        raise RegistryError(
            f"{where}: timeout_s must be an integer {TIMEOUT_MIN_S}..{TIMEOUT_MAX_S}, got {timeout_s!r}"
        )
    min_wants = raw["min_wants"]
    if type(min_wants) is not int or min_wants < 0:
        raise RegistryError(f"{where}: min_wants must be an integer >= 0, got {min_wants!r}")
    lands_in = raw["lands_in"]
    if type(lands_in) is not str or not lands_in:
        raise RegistryError(f"{where}: lands_in must be a non-empty string")
    needs = _str_list(raw["needs"], where, "needs")
    _no_duplicates(needs, where, "needs")
    return Row(
        id=row_id,
        job=_name(raw["job"], where, "job"),
        tiers=tuple(_tiers(raw["tiers"], where)),
        blocking=_flag(raw["blocking"], where, "blocking"),
        timeout_s=timeout_s,
        needs=tuple(needs),
        scope=tuple(_scope(raw["scope"], where)),
        expected_scope=tuple(_expected_scope(raw["expected_scope"], where)),
        min_wants=min_wants,
        lands_in=lands_in,
        exclusive=_flag(raw["exclusive"], where, "exclusive"),
        command=tuple(_command(raw["command"], where)),
    )


def _check_graph(rows: list[Row]) -> None:
    """`needs` names earlier rows of the same job, whose tiers include this row's; so the order is topological."""
    earlier: dict[str, Row] = {}
    for row in rows:
        if row.id in earlier:
            raise RegistryError(f"duplicate id {row.id!r}")
        for need in row.needs:
            found = earlier.get(need)
            if found is None:
                raise RegistryError(f"row {row.id!r}: needs {need!r}, which is not an earlier row")
            if found.job != row.job:
                raise RegistryError(f"row {row.id!r}: needs {need!r}, which is not in the same job")
            uncovered = sorted(set(row.tiers) - set(found.tiers))
            if uncovered:
                raise RegistryError(f"row {row.id!r}: needs {need!r}, which does not run at {', '.join(uncovered)}")
        earlier[row.id] = row


def loads(text: str) -> list[Row]:
    """Parse and validate registry JSON: {"schema": 1, "rows": [...]}, the rows in topological order."""
    try:
        doc = json.loads(text, object_pairs_hook=_refuse_duplicates, parse_constant=_refuse_constant)
    except json.JSONDecodeError as e:
        raise RegistryError(f"not JSON: {e}") from None
    if not isinstance(doc, dict) or doc.keys() != {"schema", "rows"}:
        raise RegistryError('the top level must be {"schema": 1, "rows": [...]}')
    if type(doc["schema"]) is not int or doc["schema"] != SCHEMA:
        raise RegistryError(f"schema must be {SCHEMA}, got {doc['schema']!r}")
    if not isinstance(doc["rows"], list):
        raise RegistryError("rows must be a list")
    rows = [_row(raw, i) for i, raw in enumerate(doc["rows"])]
    _check_graph(rows)
    return rows


def load(path: str) -> list[Row]:
    try:
        with open(path, "rb") as f:
            text = f.read().decode("utf-8")
    except (OSError, UnicodeDecodeError) as e:
        raise RegistryError(f"cannot read {path}: {e}") from None
    return loads(text)


def select(rows: Sequence[Row], tier: str, job: str | None) -> list[Row]:
    """The rows of this run, in registry order. Selecting nothing is refused: zero is a failure."""
    if job is not None and not any(r.job == job for r in rows):
        raise RegistryError(f"no row has job {job!r}")
    chosen = [r for r in rows if tier in r.tiers and (job is None or r.job == job)]
    if not chosen:
        scope = tier if job is None else f"{tier}, job {job}"
        raise RegistryError(f"{scope} selects zero rows: zero is a failure")
    return chosen
