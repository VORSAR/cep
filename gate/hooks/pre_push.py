"""The pre-push hook (gate design §12.4; owner 2026-11-04: in place).

Git gives one line per pushed ref on stdin: `<local ref> <local sha> <remote ref> <remote sha>`. A sha that is HEAD,
with no tracked changes in the work tree, is checked in place at T1; a failure refuses the push. Anything else is
left to CI, the net, and said so. Deletions push nothing and are skipped.
"""

import os
import subprocess
import sys
from collections.abc import Callable, Iterable

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def plan(lines: Iterable[str], *, head: str, clean: bool, peel: Callable[[str], str]) -> list[tuple[str, str, str]]:
    """(ref, commit, "check" or "ci") per distinct pushed commit, in the order git gave them."""
    seen: set[str] = set()
    out = []
    for line in lines:
        fields = line.split()
        if len(fields) != 4 or set(fields[1]) == {"0"}:
            continue
        ref, sha = fields[0], peel(fields[1])
        if sha in seen:
            continue
        seen.add(sha)
        out.append((ref, sha, "check" if sha == head and clean else "ci"))
    return out


def _git(*args: str) -> str:
    done = subprocess.run(["git", *args], cwd=REPO, stdin=subprocess.DEVNULL, capture_output=True, check=True)
    return done.stdout.decode("utf-8", errors="replace").strip()


def main() -> int:
    head = _git("rev-parse", "HEAD")
    clean = _git("status", "--porcelain", "--untracked-files=no") == ""
    bootstrap = os.path.join(REPO, "gate", "bootstrap.py")
    failed = []
    for ref, sha, what in plan(sys.stdin, head=head, clean=clean, peel=lambda s: _git("rev-parse", f"{s}^{{commit}}")):
        if what == "ci":
            print(f"{ref} ({sha[:12]}) is not checked locally: CI checks it", flush=True)
            continue
        argv = [sys.executable, "-I", "-B", bootstrap, "exec", "--", "--tier", "T1", "--commit", sha]
        if subprocess.run(argv, cwd=REPO, stdin=subprocess.DEVNULL).returncode != 0:
            failed.append(f"{ref} ({sha[:12]})")
    if failed:
        print(f"push refused: the T1 gate failed for {', '.join(failed)}", flush=True)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
