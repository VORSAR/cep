"""A whole-document pin, type-strict (S1.1c-5 design §2.2, §5; DQ-S1, DQ-L1): the one diff walker behind the
smells row's `.golangci.toml` pin (gate/runner/golangci.py) and the ruff pin (gate/runner/rows/suppressions.py).

Stub (W0b): lane B writes the walker.
"""

from collections.abc import Mapping


def differences(doc: Mapping[str, object], pinned: Mapping[str, object], name: str) -> list[str]:
    """[] when doc equals pinned type-strictly; else one line per path, sorted:
    "<name>: <path>: <got!r> ≠ pinned <want!r>", "<name>: <path>: not pinned (remove it)",
    "<name>: <path>: missing (pinned <want!r>)". Type-strict and recursive: type(a) is type(b) at every level, so
    bool ≠ int ≠ float; dicts by key set and per key; lists by length and per index, order counting. <path> joins
    keys with "." and indexes lists ("a.b[1].c[0]"), quoting a key that holds ".", "[" or a space ('"a.b"')."""
    raise NotImplementedError("S1.1c-5 lane B: pin.differences")
