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
    """How many uncommitted edits — staged, unstaged or untracked — `git status --porcelain -z` reports under the
    paths; a rename or copy is one edit (its second record, the source path, is skipped — the break-it's finding 4;
    owner, 2026-11-11)."""
    _, out = _raw("status", "--porcelain", "-z", "--", *paths, cwd=cwd)
    count, skip = 0, False
    for record in _nul_split(out):
        if skip:
            skip = False
            continue
        count += 1
        status = record[:2]
        status = status.decode("ascii", errors="replace") if isinstance(status, bytes) else status
        skip = "R" in status or "C" in status
    return count


class GitError(Exception):
    """A git command exited outside its contract; the message names the command and its exit code (DA-S3)."""


def grep_files(rev: str, patterns: Sequence[str], pathspecs: Sequence[str], cwd: str | None = None) -> list[str]:
    """The files at the revision under the pathspecs holding any of the fixed-string patterns, without the `<rev>:`
    prefix git writes; exit 1 (no match) is []; any other exit is a GitError."""
    args = ["grep", "-l", "-z", "-F", *(arg for p in patterns for arg in ("-e", p)), rev, "--", *pathspecs]
    code, out = _raw(*args, cwd=cwd)
    if code == 1:
        return []
    if code != 0:
        raise GitError(f"git grep failed (exit {code})")
    return [path.removeprefix(f"{rev}:") for path in _nul_split(out)]


def ls_tree(rev: str, prefix: str, cwd: str | None = None) -> list[str]:
    """The files at the revision under the prefix (none is []); a bad revision is a GitError."""
    code, out = _raw("ls-tree", "-r", "-z", "--name-only", rev, "--", prefix, cwd=cwd)
    if code != 0:
        raise GitError(f"git ls-tree failed (exit {code})")
    return _nul_split(out)
