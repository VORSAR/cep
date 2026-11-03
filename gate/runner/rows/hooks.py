"""The `hooks` row (gate design §12.7; plan check S4): `make check` refuses until the hooks are on.

Passes when git's `core.hooksPath`, resolved against the repository, is this repository's `.githooks`, and each
launcher in the row's scope exists and is executable. GATE_COVERED counts the launchers checked.
"""

import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

OFF = "the hooks are off: run make setup"


def setting(repo: str) -> str:
    """core.hooksPath as git resolves it for this repository, or "" when it is not set."""
    done = subprocess.run(
        ["git", "config", "--get", "core.hooksPath"], cwd=repo, stdin=subprocess.DEVNULL, capture_output=True
    )
    value = done.stdout.decode("utf-8", errors="replace").strip()
    return os.path.realpath(os.path.join(repo, os.path.expanduser(value))) if value else ""


def problems(repo: str, launchers: list[str]) -> list[str]:
    if setting(repo) != os.path.realpath(os.path.join(repo, ".githooks")):
        return [OFF]
    return [f"{rel} is not executable: run git checkout -- .githooks" for rel in launchers
            if not os.access(os.path.join(repo, rel), os.X_OK)]  # fmt: skip


def main() -> int:
    row_dir = rows.start()
    launchers = [rows.show(p) for p in rows.read_list(os.path.join(row_dir, "files.nul"))]
    failures = problems(os.getcwd(), launchers)
    if not failures:
        rows.covered(len(launchers))
    return rows.finish(failures)


if __name__ == "__main__":
    sys.exit(main())
