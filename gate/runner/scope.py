"""expected_scope, expanded live (design note §4; ADR-0015 §3.3).

`git`: complete pathspecs, magic included; the runner adds nothing. `go`: real packages only.
`go-vet-units`: what `go vet -json` really counts. Tool output goes to files in the row's directory.
"""

import os
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from gate.runner import jsonstream
from gate.runner.proc import Group
from gate.runner.records import one_line

_NOT_A_PACKAGE = ("no Go files", "matched no packages")


class ScopeError(Exception):
    """The scope could not be expanded, or expanded to nothing."""


class Stopped(Exception):
    """The step was signalled while its scope was being expanded."""


@dataclass(frozen=True)
class Expansion:
    items: tuple[bytes, ...]  # what files.nul lists: paths, or import paths for the go kinds
    count: int  # the row's expected covered count


def _capture(argv: Sequence[str], group: Group, cwd: str, env: Mapping[str, str], row_dir: str) -> bytes:
    out, err = os.path.join(row_dir, "scope.stdout"), os.path.join(row_dir, "scope.stderr")
    name = " ".join(argv[:2])
    try:
        proc = group.spawn(argv, cwd=cwd, env=env, stdout=out, stderr=err)
    except OSError as e:
        raise ScopeError(f"cannot run {name}: {e}") from None
    if proc is None:
        raise Stopped
    code, _ = group.reap(proc)
    if code != 0:
        with open(err, "rb") as f:
            detail = f.read().decode("utf-8", errors="replace").strip()
        raise ScopeError(one_line(f"{name} exit {code}: {detail}"))
    with open(out, "rb") as f:
        return f.read()


def _git(args: Sequence[str], group: Group, cwd: str, env: Mapping[str, str], row_dir: str) -> Expansion:
    data = _capture(["git", "ls-files", "-z", "--", *args], group, cwd, env, row_dir)
    paths = tuple(sorted(p for p in data.split(b"\0") if p))
    return Expansion(paths, len(paths))


def _is_real(entry: dict) -> bool:
    path = entry.get("ImportPath", "")
    if path in (".", "..") or path.startswith(("./", "../")):
        return False
    error = entry.get("Error") or {}
    return not any(text in error.get("Err", "") for text in _NOT_A_PACKAGE)


def _packages(args: Sequence[str], group: Group, cwd: str, env: Mapping[str, str], row_dir: str) -> list[dict]:
    data = _capture(["go", "list", "-e", "-json", *args], group, cwd, env, row_dir)
    try:
        entries = jsonstream.parse(data)
    except ValueError as e:
        raise ScopeError(f"go list: unreadable output: {e}") from None
    return sorted((e for e in entries if _is_real(e)), key=lambda e: e.get("ImportPath", ""))


def _units(package: dict) -> int:
    """go1.27.3: no tests 1, in-package tests 1, external tests 2, both 2, only external tests 1 (re-check 5)."""
    plain = bool(package.get("GoFiles") or package.get("CgoFiles") or package.get("TestGoFiles"))
    return int(plain) + int(bool(package.get("XTestGoFiles")))


def expand(spec: Sequence[str], *, group: Group, cwd: str, env: Mapping[str, str], row_dir: str) -> Expansion:
    """Expand one row's expected_scope. Zero is a failure, so an empty expansion is a ScopeError."""
    kind, *args = spec
    if kind == "git":
        result = _git(args, group, cwd, env, row_dir)
    elif kind in ("go", "go-vet-units"):
        packages = _packages(args, group, cwd, env, row_dir)
        count = len(packages) if kind == "go" else sum(_units(p) for p in packages)
        result = Expansion(tuple(p["ImportPath"].encode("utf-8") for p in packages), count)
    else:
        raise ScopeError(f"unknown kind {kind!r}")
    if result.count == 0:
        raise ScopeError(f"{kind} scope expands to nothing: zero is a failure")
    return result
