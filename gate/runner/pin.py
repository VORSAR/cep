"""A whole-document pin, type-strict (S1.1c-5 design §2.2, §5; DQ-S1, DQ-L1): the one diff walker behind the
smells row's `.golangci.toml` pin (gate/runner/golangci.py) and the ruff pin (gate/runner/rows/suppressions.py).

Python's `==` is type-blind (`True == 1`, `5 == 5.0`, M-11), and a tool reads a float where it wanted an int as a
setup failure that hides a whole linter (M-1); so two values are equal here only when their types are identical at
every level. Dicts compare by key set and per key, lists by length and per index (order counts), and the walk
descends into the first level that differs, so a change deep inside a list of tables names that element's leaf.
"""

from collections.abc import Mapping

_QUOTED = (".", "[", " ")


def _key(key: str) -> str:
    """One path segment: the key, double-quoted when it holds ".", "[" or a space."""
    return f'"{key}"' if any(c in key for c in _QUOTED) else key


def _join(path: str, key: str) -> str:
    return f"{path}.{_key(key)}" if path else _key(key)


def _walk(got: object, want: object, path: str, out: list[str]) -> None:
    if type(got) is not type(want):
        out.append(f"{path}: {got!r} ≠ pinned {want!r}")
        return
    if isinstance(want, dict) and isinstance(got, dict):
        for key in sorted(set(got) | set(want)):
            sub = _join(path, key)
            if key not in want:
                out.append(f"{sub}: not pinned (remove it)")
            elif key not in got:
                out.append(f"{sub}: missing (pinned {want[key]!r})")
            else:
                _walk(got[key], want[key], sub, out)
        return
    if isinstance(want, list) and isinstance(got, list):
        for i in range(max(len(got), len(want))):
            sub = f"{path}[{i}]"
            if i >= len(want):
                out.append(f"{sub}: not pinned (remove it)")
            elif i >= len(got):
                out.append(f"{sub}: missing (pinned {want[i]!r})")
            else:
                _walk(got[i], want[i], sub, out)
        return
    if got != want:
        out.append(f"{path}: {got!r} ≠ pinned {want!r}")


def differences(doc: Mapping[str, object], pinned: Mapping[str, object], name: str) -> list[str]:
    """[] when doc equals pinned type-strictly; else one line per path, sorted:
    "<name>: <path>: <got!r> ≠ pinned <want!r>", "<name>: <path>: not pinned (remove it)",
    "<name>: <path>: missing (pinned <want!r>)". Type-strict and recursive: type(a) is type(b) at every level, so
    bool ≠ int ≠ float; dicts by key set and per key; lists by length and per index, order counting. <path> joins
    keys with "." and indexes lists ("a.b[1].c[0]"), quoting a key that holds ".", "[" or a space ('"a.b"')."""
    out: list[str] = []
    _walk(dict(doc), dict(pinned), "", out)
    return sorted(f"{name}: {line}" for line in out)
