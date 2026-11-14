"""The smells row's config pin and report reader (S1.1c-5 design note §2.2, §2.4): the only reader of .golangci.toml.

W0c stub: every function raises NotImplementedError until lane A writes it.
"""

from collections.abc import Mapping, Sequence
from dataclasses import dataclass

CONFIG = ".golangci.toml"
LINTERS = ("errcheck", "forbidigo", "funlen", "gocritic", "gocyclo", "lll", "nolintlint", "revive")
SIZE_RULES = ("funlen", "gocyclo", "file-length-limit", "argument-limit")  # the size family, as smells.json names them
PINNED: dict[str, object] = {}  # stub: lane A writes the §2.1 document as tomllib parses it


class ConfigError(ValueError):
    """The config cannot be read: a TOML error (a duplicate key included) or a missing key."""


class ReportError(ValueError):
    """A golangci-lint JSON report that is not JSON or has the wrong shape, naming its path."""


@dataclass(frozen=True)
class Issue:
    linter: str
    rule: str
    text: str
    file: str
    line: int
    column: int


@dataclass(frozen=True)
class Report:
    issues: tuple[Issue, ...]
    enabled: frozenset[str]
    error: str
    warnings: tuple[str, ...]


@dataclass(frozen=True)
class GoFile:
    import_path: str
    package: str  # go list's Name; an XTestGoFiles entry's is Name + "_test"


def load(text: str) -> dict[str, object]:
    raise NotImplementedError("S1.1c-5 lane A: golangci.load")


def differences(doc: Mapping[str, object]) -> list[str]:
    raise NotImplementedError("S1.1c-5 lane A: golangci.differences")


def enabled(doc: Mapping[str, object]) -> frozenset[str]:
    raise NotImplementedError("S1.1c-5 lane A: golangci.enabled")


def rule_of(issue: Mapping[str, object]) -> str:
    raise NotImplementedError("S1.1c-5 lane A: golangci.rule_of")


def read_report(path: str) -> Report:
    raise NotImplementedError("S1.1c-5 lane A: golangci.read_report")


def go_files(listing: Sequence[Mapping[str, object]], root: str) -> tuple[dict[str, GoFile], list[str]]:
    raise NotImplementedError("S1.1c-5 lane A: golangci.go_files")


def covered(
    tracked: Sequence[str],
    linux: Mapping[str, GoFile],
    darwin: Mapping[str, GoFile],
    ignored_both: frozenset[str],
    gomods: Sequence[str],
) -> tuple[int, int, list[str]]:
    raise NotImplementedError("S1.1c-5 lane A: golangci.covered")
