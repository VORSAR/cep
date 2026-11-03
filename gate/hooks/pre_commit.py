"""The pre-commit hook (gate design §12.1; owner 2026-11-04: in place, with a guard). Also run for pre-merge-commit.

The gate checks the staged files where they are, so a staged file that also has unstaged changes would be checked
as it is on disk, not as it is committed. The guard refuses that commit first, naming each such file. Then T0 runs
in the work tree; git has already set GIT_INDEX_FILE for `commit -a` and `commit <path>`, and the runner keeps it.
"""

import os
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def _paths(*args: str) -> list[bytes]:
    out = subprocess.run(["git", *args], cwd=REPO, stdin=subprocess.DEVNULL, capture_output=True, check=True).stdout
    return [p for p in out.split(b"\0") if p]


def mixed(staged: list[bytes], unstaged: list[bytes]) -> list[str]:
    """The staged paths that also have unstaged changes, sorted."""
    return sorted(p.decode("utf-8", errors="replace") for p in set(staged) & set(unstaged))


def refusal(paths: list[str]) -> list[str]:
    return [f"{p} has unstaged changes: stage them or stash them, so the check sees what you commit" for p in paths]


def main() -> int:
    staged = _paths("diff", "--cached", "--name-only", "-z", "--diff-filter=ACMR")
    lines = refusal(mixed(staged, _paths("diff", "--name-only", "-z")))
    if lines:
        print("\n".join([*lines, "GATE FAILED", "exit: 1"]), flush=True)
        return 1
    bootstrap = os.path.join(REPO, "gate", "bootstrap.py")
    os.execv(sys.executable, [sys.executable, "-I", "-B", bootstrap, "exec", "--", "--tier", "T0"])
    return 1


if __name__ == "__main__":
    sys.exit(main())
