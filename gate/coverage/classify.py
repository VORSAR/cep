"""Each in-scope package's verdict, from `go test -json` events plus the profile (plan check S6, design note §5).

In order, the first that holds decides:

    the test build failed                       FAIL "build failed"
    no `run` event                              FAIL "no tests"
    tests ran, but none passed or failed        FAIL "no test ran"      (every test skipped)
    0 statements and >= 1 test passed           PASS "n/a (0 statements)"
    otherwise                                   covered * 100 >= floor * total, shown truncated, never rounded up

Generated files (ADR-0015 §2.1) are out of the numerator and the denominator. A package is "0 statements" only on
evidence: go reports `coverage: [no statements]` for a package it wrote no blocks for. A package with no blocks and
no such report is "missing from the profile": its test binary died before writing them, which would otherwise read
as a vacuous pass. A 0-statement package whose tests ran but none passed fails.
"""

import json
from collections import defaultdict
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import dataclass, field

from gate.coverage.coverprofile import Block

NO_STATEMENTS = "coverage: [no statements]\n"


class EventsError(ValueError):
    """The event stream cannot be read."""


@dataclass(frozen=True)
class Package:
    path: str  # the import path
    generated: frozenset[str] = frozenset()  # the names of its generated files


@dataclass(frozen=True)
class Verdict:
    package: str
    ok: bool
    detail: str

    def line(self) -> str:
        return f"{'PASS' if self.ok else 'FAIL'} {self.package} {self.detail}"


@dataclass
class _Facts:
    failed_build: bool = False
    runs: int = 0
    passed: int = 0
    failed: int = 0
    no_statements: bool = False
    blocks: list[Block] = field(default_factory=list)


def parse_events(data: bytes) -> list[dict]:
    """`go test -json`: one JSON object per line."""
    events: list[dict] = []
    for number, line in enumerate(data.decode("utf-8", errors="replace").split("\n"), start=1):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError as e:
            raise EventsError(f"line {number}: not JSON: {e}") from None
        if not isinstance(event, dict):
            raise EventsError(f"line {number}: expected a JSON object, got {type(event).__name__}")
        events.append(event)
    return events


def percent(covered: int, total: int) -> str:
    """covered/total as a percentage with two decimals, truncated: 17999/20000 is 89.99%, never 90.00%."""
    basis = covered * 10000 // total
    return f"{basis // 100}.{basis % 100:02d}%"


def _facts(events: Iterable[Mapping], blocks: Iterable[Block]) -> dict[str, _Facts]:
    facts: defaultdict[str, _Facts] = defaultdict(_Facts)
    for event in events:
        package = event.get("Package")
        if not isinstance(package, str):
            continue  # build output is keyed by ImportPath; the package's own `fail` event carries FailedBuild
        f = facts[package]
        action, in_test = event.get("Action"), bool(event.get("Test"))
        if action == "run" and in_test:
            f.runs += 1
        elif action == "pass" and in_test:
            f.passed += 1
        elif action == "fail" and in_test:
            f.failed += 1
        elif action == "fail" and event.get("FailedBuild"):
            f.failed_build = True
        elif action == "output" and not in_test and event.get("Output") == NO_STATEMENTS:
            f.no_statements = True
    for block in blocks:
        facts[block.package].blocks.append(block)
    return facts


def _judge(package: Package, f: _Facts, floor: int) -> Verdict:
    path = package.path
    if f.failed_build:
        return Verdict(path, False, "build failed")
    if not f.runs:
        return Verdict(path, False, "no tests")
    if not (f.passed or f.failed):
        return Verdict(path, False, "no test ran")
    if not f.blocks and not f.no_statements:
        return Verdict(path, False, "missing from the profile")
    counted = [b for b in f.blocks if b.name not in package.generated]
    total = sum(b.statements for b in counted)
    if total == 0:
        if f.passed:
            return Verdict(path, True, "n/a (0 statements)")
        return Verdict(path, False, "n/a (0 statements), but no test passed")
    covered = sum(b.statements for b in counted if b.count > 0)
    shown = f"{percent(covered, total)} ({covered}/{total})"
    if covered * 100 >= floor * total:
        return Verdict(path, True, f"{shown} >= {floor}%")
    return Verdict(path, False, f"{shown} < {floor}%")


def evaluate(
    packages: Sequence[Package], blocks: Sequence[Block], events: Sequence[Mapping], floor: int
) -> list[Verdict]:
    """One verdict per package, sorted by import path. Blocks and events of other packages are never judged."""
    facts = _facts(events, blocks)
    return [_judge(p, facts.get(p.path, _Facts()), floor) for p in sorted(packages, key=lambda p: p.path)]
