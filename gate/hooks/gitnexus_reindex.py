"""A Claude Code hook that keeps the GitNexus code graph at HEAD (DEVELOPMENT.md, "The code graph").

The owner chose this over an OS-level service (2026-11-06). It is registered in .claude/settings.json for every
prompt (UserPromptSubmit) and for every Bash call that runs git (PostToolUse), so it sees a commit, merge or
checkout made by an agent at once, and one made in the owner's own terminal at the next prompt, before the graph
is used. When the checkout's HEAD is not the commit the GitNexus registry says is indexed, it starts
`gitnexus analyze --index-only <checkout>` in the background and returns: --index-only never rewrites CLAUDE.md,
AGENTS.md or the skills.

It never blocks or fails Claude: whatever goes wrong (no git, no registry, no gitnexus, bad input), it does
nothing, prints nothing and exits 0. A linked worktree is not in the registry, so an agent's lane is left alone.
A pidfile in the index's storage keeps a second run from starting while one is going.
"""

import json
import os
import re
import shutil
import subprocess
import sys
from collections.abc import Mapping

PIDFILE = "auto-reindex.pid"
LOGFILE = "auto-reindex.log"
GIT_WORD = re.compile(r"(?<![\w.-])git(?![\w.-])")


def wants_check(event: Mapping) -> bool:
    """Every prompt, and every Bash call that runs git, is a moment to compare the index with HEAD."""
    name = event.get("hook_event_name")
    if name == "UserPromptSubmit":
        return True
    if name == "PostToolUse" and event.get("tool_name") == "Bash":
        tool_input = event.get("tool_input")
        command = tool_input.get("command", "") if isinstance(tool_input, Mapping) else ""
        return isinstance(command, str) and GIT_WORD.search(command) is not None
    return False


def _git(cwd: str, *args: str) -> str:
    """git's stdout, stripped; "" when git fails or is missing."""
    try:
        done = subprocess.run(
            ["git", *args], cwd=cwd, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=5
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    return done.stdout.strip() if done.returncode == 0 else ""


def registry_entry(environ: Mapping[str, str], checkout: str) -> Mapping | None:
    """The registry row for this checkout, read where GitNexus reads it ($GITNEXUS_HOME, else ~/.gitnexus)."""
    home = environ.get("GITNEXUS_HOME") or os.path.join(environ.get("HOME", "~"), ".gitnexus")
    try:
        with open(os.path.join(os.path.expanduser(home), "registry.json"), encoding="utf-8") as f:
            rows = json.load(f)
    except (OSError, ValueError):
        return None
    target = os.path.realpath(checkout)
    for row in rows if isinstance(rows, list) else []:
        if isinstance(row, dict) and isinstance(row.get("path"), str) and os.path.realpath(row["path"]) == target:
            return row
    return None


def running(pidfile: str) -> bool:
    """Whether the pid recorded by an earlier start is still alive."""
    try:
        with open(pidfile, encoding="utf-8") as f:
            pid = int(f.read().strip())
        os.kill(pid, 0)
    except (OSError, ValueError):
        return False
    return True


def start(gitnexus: str, checkout: str, storage: str, environ: Mapping[str, str]) -> None:
    """Start the index-only analyze detached, its output in the storage's log, and record its pid."""
    with (
        open(os.path.join(storage, LOGFILE), "ab") as log,
        open(os.devnull, "rb") as devnull,
    ):
        child = subprocess.Popen(
            [gitnexus, "analyze", "--index-only", checkout],
            cwd=checkout,
            env=dict(environ),
            stdin=devnull,
            stdout=log,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
    pidfile = os.path.join(storage, PIDFILE)
    with open(pidfile + ".tmp", "w", encoding="utf-8") as f:
        f.write(str(child.pid))
    os.rename(pidfile + ".tmp", pidfile)


def main(text: str, environ: Mapping[str, str]) -> int:
    try:
        event = json.loads(text or "{}")
    except ValueError:
        return 0
    if not isinstance(event, dict) or not wants_check(event):
        return 0
    cwd = event.get("cwd")
    if not isinstance(cwd, str) or not os.path.isdir(cwd):
        return 0
    checkout, head = _git(cwd, "rev-parse", "--show-toplevel"), _git(cwd, "rev-parse", "HEAD")
    if not checkout or not head:
        return 0
    entry = registry_entry(environ, checkout)
    if entry is None or entry.get("lastCommit") == head:
        return 0
    storage = entry.get("storagePath")
    if not isinstance(storage, str) or not os.path.isdir(storage) or running(os.path.join(storage, PIDFILE)):
        return 0
    gitnexus = shutil.which("gitnexus", path=environ.get("PATH"))
    if gitnexus is None:
        return 0
    try:
        start(gitnexus, os.path.realpath(checkout), storage, environ)
    except OSError:
        return 0
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.stdin.read(), os.environ))
