"""A Claude Code hook that keeps the GitNexus code graph at HEAD (DEVELOPMENT.md, "The code graph").

The owner chose this over an OS-level service (2026-11-06). It is registered in .claude/settings.json for every
prompt (UserPromptSubmit) and for every Bash call that runs git (PostToolUse), so it sees a commit, merge or
checkout made by an agent at once, and one made in the owner's own terminal at the next prompt, before the graph
is used. When the checkout's HEAD is not the commit the GitNexus registry says is indexed, it starts a follow-up of
itself in the background and returns. The follow-up runs `gitnexus analyze --force --index-only <checkout>` (a full
rebuild, the full-text index included; --index-only never rewrites CLAUDE.md, AGENTS.md or the skills) and, when that
succeeds, sends TERM to every `gitnexus mcp` server whose parent is Claude Code: a server keeps the handle it opened,
so after a re-index it answered from a stale index until restarted, and the clients start a new one on their next
call (owner, 2026-11-11, "the GitNexus hook"). A server under anything else is left alone.

It never blocks or fails Claude: whatever goes wrong (no git, no registry, no gitnexus, bad input), it does
nothing, prints nothing and exits 0. A linked worktree is not in the registry, so an agent's lane is left alone.
A pidfile in the index's storage keeps a second run from starting while one is going.
"""

import itertools
import json
import os
import re
import shutil
import signal
import subprocess
import sys
from collections.abc import Callable, Mapping

PIDFILE = "auto-reindex.pid"
FOLLOW_UP = "--reindex"  # the argument that runs this file as the detached follow-up
NO_RESTART = "GITNEXUS_REINDEX_NO_RESTART"  # "1": the follow-up re-indexes but restarts nothing (the hook's tests)
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
    """Start the follow-up detached, its output in the storage's log, and record its pid."""
    with (
        open(os.path.join(storage, LOGFILE), "ab") as log,
        open(os.devnull, "rb") as devnull,
    ):
        child = subprocess.Popen(
            [sys.executable, "-I", "-B", os.path.abspath(__file__), FOLLOW_UP, gitnexus, checkout],
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


def ps_listing() -> str:
    """Every process as `ps` lists it: "pid ppid command" per line; "" when ps cannot run."""
    try:
        done = subprocess.run(
            ["ps", "-A", "-o", "pid=,ppid=,command="],
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=10,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    return done.stdout


def _parse(listing: str) -> dict[int, tuple[int, list[str]]]:
    rows: dict[int, tuple[int, list[str]]] = {}
    for line in listing.splitlines():
        parts = line.split(None, 2)
        if len(parts) == 3 and parts[0].isdigit() and parts[1].isdigit():
            rows[int(parts[0])] = (int(parts[1]), parts[2].split())
    return rows


def _is_server(words: list[str]) -> bool:
    return any(os.path.basename(w) == "gitnexus" and nxt == "mcp" for w, nxt in itertools.pairwise(words))


def servers(listing: str) -> list[int]:
    """The pids of the `gitnexus mcp` servers in a ps listing whose parent is Claude Code (a word of its command
    names a program `claude`), in pid order."""
    rows = _parse(listing)
    chosen = []
    for pid, (ppid, words) in rows.items():
        parent = rows.get(ppid)
        if _is_server(words) and parent is not None and any(os.path.basename(w) == "claude" for w in parent[1]):
            chosen.append(pid)
    return sorted(chosen)


def restart(listing: str) -> list[int]:
    """TERM each chosen server; the pids signalled. A server already gone is not an error."""
    signalled = []
    for pid in servers(listing):
        try:
            os.kill(pid, signal.SIGTERM)
        except OSError:
            continue
        signalled.append(pid)
    return signalled


def reindex(
    gitnexus: str, checkout: str, environ: Mapping[str, str], restart_servers: Callable[[], object] | None = None
) -> int:
    """The follow-up: a forced, index-only analyze of the checkout; after a successful one, the servers restarted
    (unless NO_RESTART is "1", for the hook's tests). Its exit code is the analyze's."""
    try:
        done = subprocess.run(
            [gitnexus, "analyze", "--force", "--index-only", checkout],
            cwd=checkout,
            env=dict(environ),
            stdin=subprocess.DEVNULL,
        )
    except OSError:
        return 1
    if done.returncode == 0:
        if restart_servers is not None:
            restart_servers()
        elif environ.get(NO_RESTART) != "1":
            restart(ps_listing())
    return done.returncode


if __name__ == "__main__":
    if len(sys.argv) == 4 and sys.argv[1] == FOLLOW_UP:
        sys.exit(reindex(sys.argv[2], sys.argv[3], os.environ))
    sys.exit(main(sys.stdin.read(), os.environ))
