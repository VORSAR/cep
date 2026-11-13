"""The weekly canary's sort (S1.1c-4 design §4.5; review DQ-S8): each red record of the T4 jobs run on the candidate
tools.json is either "pins to update" — every failure in it is a known pin test failing on its pin, and the candidate
has no structural fault — or a break, named. Pure: the workflow's command reads the files and calls these.

pin_tests.json (lane F1 writes it): {"schema": 1, "note": "…", "entries": [{"id": "<test id or a non-test red's
name>", "row": "<registry row id>", "pattern": "<Python regex searched in the failure text>", "measured": "<date,
candidate>" | "unmeasured"}, …]}.
"""

import json
import re
from collections.abc import Collection, Iterable, Mapping, Sequence
from dataclasses import dataclass

from gate.canary.candidate import CanaryError

FILE = "pin_tests.json"
SCHEMA = 1
UNMEASURED = "unmeasured"
_TOP_KEYS = frozenset({"schema", "note", "entries"})
_ENTRY_KEYS = frozenset({"id", "row", "pattern", "measured"})
_IDENTIFIER = re.compile(r"[a-z_][a-z0-9_]*")


class PinTestsError(CanaryError):
    """pin_tests.json is malformed: the message names the file, the entry and the fault."""


@dataclass(frozen=True)
class Sorted:
    pins: tuple[str, ...]  # `<test id> (<file>)` or a non-test red's name, each once, sorted
    breaks: tuple[str, ...]  # the structural faults sorted, then `<row>: <test id>: <text>` lines sorted


def sort(
    records: Iterable[Mapping[str, object]],
    pin_tests: Mapping[str, str],
    structural: Sequence[str],
    *,
    unmeasured: Collection[str] = (),
) -> Sorted:
    """A red record is pins only when every failure in it is a test of `pin_tests` whose text its pattern finds
    (re.search) and `structural` is empty; otherwise each failure no pattern found is a break, or, when all were
    found, each failure of the record (a structural fault makes every pin a break). A red record with no failures read
    is a break by its reason. An id in `unmeasured` is named so wherever it fails."""
    patterns = {test: re.compile(pattern) for test, pattern in pin_tests.items()}
    pins: set[str] = set()
    breaks: set[str] = set()
    for record in records:
        row = str(record["id"])
        failures = [_pair(row, failure) for failure in record.get("failures") or ()]
        if not failures:
            if record["verdict"] != "pass":
                breaks.add(f"{row}: {_flat(str(record['reason']))}")
            continue
        unmatched = [(test, text) for test, text in failures if not _found(patterns, test, text)]
        if not unmatched and not structural:
            pins.update(_pin_line(test, unmeasured) for test, _ in failures)
            continue
        breaks.update(_break_line(row, test, text, unmeasured) for test, text in unmatched or failures)
    return Sorted(tuple(sorted(pins)), tuple(sorted(set(structural))) + tuple(sorted(breaks)))


def load_pin_tests(text: str) -> dict[str, str]:
    """pin_tests.json's text → {id: pattern}, sorted by id; a malformed file raises PinTestsError naming the fault."""
    return {entry["id"]: entry["pattern"] for entry in _entries(text)}


def unmeasured(text: str) -> list[str]:
    """The ids pin_tests.json marks "unmeasured" (design §4.5: reds today's feeds cannot produce), sorted."""
    return sorted(entry["id"] for entry in _entries(text) if entry["measured"] == UNMEASURED)


def _found(patterns: Mapping[str, re.Pattern[str]], test: str, text: str) -> bool:
    pattern = patterns.get(test)
    return pattern is not None and pattern.search(text) is not None


def _pair(row: str, failure: object) -> tuple[str, str]:
    match failure:
        case [str() as test, str() as text]:
            return test, text
        case _:
            raise CanaryError(f"{row}: a failure is not a [test id, text] pair: {failure!r}")


def _pin_line(test: str, unmeasured: Collection[str]) -> str:
    notes = [file] if (file := _test_file(test)) else []
    if test in unmeasured:
        notes.append("unmeasured pin")
    return f"{test} ({'; '.join(notes)})" if notes else test


def _break_line(row: str, test: str, text: str, unmeasured: Collection[str]) -> str:
    named = f"{row}: {_flat(text)}" if test == row else f"{row}: {test}: {_flat(text)}"
    return f"{named} (unmeasured pin)" if test in unmeasured else named


def _test_file(test: str) -> str | None:
    """A unittest id's module file (`a.b.test_c.Class.test_d` → `a/b/test_c.py`); None for a non-test red's name."""
    parts = test.split(".")
    module = []
    for part in parts:
        if not _IDENTIFIER.fullmatch(part):
            break
        module.append(part)
    if not module or len(module) == len(parts) or not module[-1].startswith("test"):
        return None
    return "/".join(module) + ".py"


def _flat(text: str) -> str:
    return " ".join(text.split())


def _entries(text: str) -> list[dict[str, str]]:
    try:
        doc = json.loads(text, object_pairs_hook=_no_duplicate_keys)
    except ValueError as e:
        raise PinTestsError(f"{FILE}: not JSON: {e}") from None
    if not isinstance(doc, dict):
        raise PinTestsError(f"{FILE}: not a JSON object")
    _keys(doc, _TOP_KEYS, "")
    if type(doc["schema"]) is not int or doc["schema"] != SCHEMA:
        raise PinTestsError(f"{FILE}: schema is {json.dumps(doc['schema'])}, not {SCHEMA}")
    if not isinstance(doc["note"], str):
        raise PinTestsError(f"{FILE}: note is not a string")
    if not isinstance(doc["entries"], list):
        raise PinTestsError(f"{FILE}: entries is not a list")
    seen: set[str] = set()
    out = []
    for n, entry in enumerate(doc["entries"], 1):
        if not isinstance(entry, dict):
            raise PinTestsError(f"{FILE}: entry {n} is not a JSON object")
        name = entry["id"] if isinstance(entry.get("id"), str) and entry["id"] else f"entry {n}"
        _keys(entry, _ENTRY_KEYS, f"{name}: ")
        for key in sorted(_ENTRY_KEYS):
            if not isinstance(entry[key], str) or not entry[key]:
                raise PinTestsError(f"{FILE}: {name}: {key} is not a non-empty string")
        try:
            pattern = re.compile(entry["pattern"])
        except re.error as e:
            raise PinTestsError(f"{FILE}: {name}: pattern {entry['pattern']!r} does not compile: {e}") from None
        if pattern.search("") is not None:
            raise PinTestsError(f"{FILE}: {name}: pattern {entry['pattern']!r} matches the empty text (any failure)")
        if name in seen:
            raise PinTestsError(f"{FILE}: {name}: the id appears twice")
        seen.add(name)
        out.append(entry)
    return sorted(out, key=lambda entry: entry["id"])


def _keys(obj: Mapping[str, object], want: frozenset[str], where: str) -> None:
    if missing := sorted(want - obj.keys()):
        raise PinTestsError(f"{FILE}: {where}missing key {', '.join(missing)}")
    if unknown := sorted(obj.keys() - want):
        raise PinTestsError(f"{FILE}: {where}unknown key {', '.join(unknown)}")


def _no_duplicate_keys(pairs: list[tuple[str, object]]) -> dict[str, object]:
    out: dict[str, object] = {}
    for key, value in pairs:
        if key in out:
            raise PinTestsError(f"{FILE}: the key {key!r} appears twice in one object")
        out[key] = value
    return out
