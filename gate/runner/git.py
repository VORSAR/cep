"""Git as the gate's rows read it (S1.1c-1 design §2.2): one subprocess per call, stdin /dev/null, output captured.

`merge_base` has one contract: it returns "" when HEAD has no merge base with `origin/main` or `main`, and the
caller decides. `lawdelta` fails on "" (owner, 2026-11-10, decision 9); `apidiff` passes and says nothing was
compared (owner, 2026-11-07). Every function takes `cwd` so a test runs it in its own repository.
"""

import subprocess
from collections.abc import Sequence


def _raw(*args: str, cwd: str | None = None) -> tuple[int, bytes]:
    p = subprocess.run(["git", *args], cwd=cwd, stdin=subprocess.DEVNULL, capture_output=True)
    return p.returncode, p.stdout


def git(*args: str, cwd: str | None = None) -> tuple[int, str]:
    """(exit code, stdout stripped); apidiff.py's helper, moved here verbatim in effect."""
    p = subprocess.run(["git", *args], cwd=cwd, stdin=subprocess.DEVNULL, capture_output=True, text=True)
    return p.returncode, p.stdout.strip()


def merge_base(cwd: str | None = None) -> str:
    """`origin/main` then `main`: the first merge base that resolves, else ""."""
    for ref in ("origin/main", "main"):
        code, out = git("merge-base", "HEAD", ref, cwd=cwd)
        if code == 0 and out:
            return out
    return ""


def head(cwd: str | None = None) -> str:
    return git("rev-parse", "HEAD", cwd=cwd)[1]


def _nul_split(data: bytes) -> list[str]:
    return [item for item in data.decode("utf-8", errors="replace").split("\0") if item]


def ls_files(paths: Sequence[str], cwd: str | None = None) -> list[str]:
    """The tracked files under each path; a path ending "/" is a prefix."""
    _, out = _raw("ls-files", "-z", "--", *paths, cwd=cwd)
    return _nul_split(out)


def _count(text: str) -> int:
    """A numstat count; a binary file's "-" reads as 0."""
    return 0 if text == "-" else int(text)


def numstat(base: str, head: str, paths: Sequence[str], cwd: str | None = None) -> list[tuple[str, int, int]]:
    """(path, added, removed) per changed file between base and head under the paths, in git's (bytewise) order.

    `--no-renames` makes a renamed file a removal of the old path and an addition of the new, so every record is
    `added\\tremoved\\tpath\\0` (review: architect S3, QA-B1).
    """
    _, out = _raw("diff", "--numstat", "-z", "--no-renames", base, head, "--", *paths, cwd=cwd)
    changes = []
    for record in _nul_split(out):
        added, removed, path = record.split("\t", 2)
        changes.append((path, _count(added), _count(removed)))
    return changes


def show(rev: str, path: str, cwd: str | None = None) -> str | None:
    """The file's text at the revision (UTF-8, errors replaced); None when absent there."""
    code, out = _raw("show", f"{rev}:{path}", cwd=cwd)
    return out.decode("utf-8", errors="replace") if code == 0 else None


def dirty(paths: Sequence[str], cwd: str | None = None) -> int:
    """How many uncommitted edits, staged or not, `git status --porcelain` reports under the paths."""
    _, out = _raw("status", "--porcelain", "-z", "--", *paths, cwd=cwd)
    return len(_nul_split(out))
