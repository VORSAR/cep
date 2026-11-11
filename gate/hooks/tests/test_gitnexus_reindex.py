"""The Claude Code hook that keeps the GitNexus code graph at HEAD (DEVELOPMENT.md, "The code graph").

Each test owns its world: a temporary git repository, a temporary GitNexus registry (GITNEXUS_HOME) and a fake
`gitnexus` on PATH that records how it was started. Nothing touches the real index.
"""

import contextlib
import json
import os
import subprocess
import sys
import tempfile
import unittest
import warnings

from gate import wait
from gate.hooks import gitnexus_reindex

FAKE_GITNEXUS = """#!{python} -I
import json, os, sys
with open(os.environ["FAKE_GITNEXUS_OUT"] + ".tmp", "w") as f:
    json.dump({{"argv": sys.argv[1:], "cwd": os.getcwd()}}, f)
os.rename(os.environ["FAKE_GITNEXUS_OUT"] + ".tmp", os.environ["FAKE_GITNEXUS_OUT"])
sys.exit(int(os.environ.get("FAKE_GITNEXUS_EXIT", "0")))
"""


def _git(cwd: str, *args: str) -> str:
    done = subprocess.run(
        ["git", "-c", "user.name=t", "-c", "user.email=t@t", "-c", "core.hooksPath=/dev/null", *args],
        cwd=cwd,
        stdin=subprocess.DEVNULL,
        capture_output=True,
        text=True,
        check=True,
    )
    return done.stdout.strip()


class World:
    """A repository with one commit, a registry that indexes it at `indexed`, and a fake gitnexus on PATH."""

    def __init__(self, test: unittest.TestCase, indexed: str | None = "HEAD", with_gitnexus: bool = True):
        tmp = tempfile.TemporaryDirectory()
        test.addCleanup(tmp.cleanup)
        self.root = os.path.realpath(tmp.name)
        self.repo = os.path.join(self.root, "repo")
        os.mkdir(self.repo)
        _git(self.repo, "init", "-q", "-b", "main")
        with open(os.path.join(self.repo, "a.txt"), "w") as f:
            f.write("a\n")
        _git(self.repo, "add", "a.txt")
        _git(self.repo, "commit", "-q", "-m", "one")
        self.head = _git(self.repo, "rev-parse", "HEAD")
        self.home = os.path.join(self.root, "gitnexus-home")
        self.storage = os.path.join(self.home, "storage", "repo-0123456789ab")
        os.makedirs(self.storage)
        entries = []
        if indexed is not None:
            last = self.head if indexed == "HEAD" else indexed
            entries.append({"name": "repo", "path": self.repo, "storagePath": self.storage, "lastCommit": last})
        with open(os.path.join(self.home, "registry.json"), "w") as f:
            json.dump(entries, f)
        self.bin = os.path.join(self.root, "bin")
        os.mkdir(self.bin)
        if with_gitnexus:
            fake = os.path.join(self.bin, "gitnexus")
            with open(fake, "w") as f:
                f.write(FAKE_GITNEXUS.format(python=sys.executable))
            os.chmod(fake, 0o755)
        self.out = os.path.join(self.root, "gitnexus-called.json")
        self.environ = {
            "PATH": os.pathsep.join([self.bin, os.path.dirname(sys.executable), "/usr/bin", "/bin"]),
            "HOME": self.root,
            "GITNEXUS_HOME": self.home,
            "FAKE_GITNEXUS_OUT": self.out,
            # The follow-up would restart this machine's real servers; its restart is tested on planted ones.
            gitnexus_reindex.NO_RESTART: "1",
        }

    def pidfile(self) -> str:
        return os.path.join(self.storage, gitnexus_reindex.PIDFILE)

    def run(self, event: dict | str) -> int:
        text = event if isinstance(event, str) else json.dumps(event)
        with warnings.catch_warnings():
            # The analyze run outlives the hook on purpose, and CPython warns when a running Popen is dropped.
            # Outside a test the default filters already ignore ResourceWarning.
            warnings.simplefilter("ignore", ResourceWarning)
            return gitnexus_reindex.main(text, self.environ)

    def prompt(self, cwd: str | None = None) -> dict:
        return {"hook_event_name": "UserPromptSubmit", "cwd": cwd or self.repo, "prompt": "hello"}

    def started(self) -> dict:
        def called():
            if not os.path.exists(self.out):
                return None
            with open(self.out) as f:
                return json.load(f)

        return wait.until("the fake gitnexus to record its start", called, cap_s=30)


def bash(command: str, cwd: str) -> dict:
    return {"hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_input": {"command": command}, "cwd": cwd}


class WhenToLook(unittest.TestCase):
    def test_every_prompt_is_a_moment_to_look(self):
        self.assertTrue(gitnexus_reindex.wants_check({"hook_event_name": "UserPromptSubmit"}))

    def test_a_bash_call_that_runs_git_is_a_moment_to_look(self):
        for command in ("git commit -m x", "git pull --ff-only", "cd x && git checkout main", "git -C . merge b"):
            self.assertTrue(gitnexus_reindex.wants_check(bash(command, "/r")), command)

    def test_a_bash_call_without_git_is_not(self):
        for command in ("ls", "make check", "gitnexus analyze", "echo digit"):
            self.assertFalse(gitnexus_reindex.wants_check(bash(command, "/r")), command)

    def test_other_tools_and_events_are_not(self):
        self.assertFalse(gitnexus_reindex.wants_check({"hook_event_name": "PostToolUse", "tool_name": "Grep"}))
        self.assertFalse(gitnexus_reindex.wants_check({"hook_event_name": "Stop"}))
        self.assertFalse(gitnexus_reindex.wants_check({}))


class Reindex(unittest.TestCase):
    def test_an_index_behind_head_starts_an_index_only_analyze_of_the_checkout(self):
        world = World(self, indexed="0" * 40)
        self.assertEqual(world.run(world.prompt()), 0)
        self.assertEqual(
            world.started(), {"argv": ["analyze", "--force", "--index-only", world.repo], "cwd": world.repo}
        )
        self.assertTrue(os.path.exists(world.pidfile()))

    def test_a_git_command_from_a_subdirectory_counts_for_the_checkout(self):
        world = World(self, indexed="0" * 40)
        sub = os.path.join(world.repo, "sub")
        os.mkdir(sub)
        self.assertEqual(world.run(bash("git commit -m y", sub)), 0)
        self.assertEqual(world.started()["argv"], ["analyze", "--force", "--index-only", world.repo])

    def test_a_stale_pidfile_does_not_block_a_new_run(self):
        world = World(self, indexed="0" * 40)
        dead = subprocess.Popen([sys.executable, "-I", "-c", ""], stdin=subprocess.DEVNULL)
        dead.communicate()
        with open(world.pidfile(), "w") as f:
            f.write(str(dead.pid))
        self.assertEqual(world.run(world.prompt()), 0)
        self.assertEqual(world.started()["argv"][0], "analyze")


class LeaveAlone(unittest.TestCase):
    """A run is started (and its pidfile written) inside main, so after main returns no pidfile means no run."""

    def assert_nothing_started(self, world: World, before: str | None = None):
        if before is None:
            self.assertFalse(os.path.exists(world.pidfile()))
        else:
            with open(world.pidfile()) as f:
                self.assertEqual(f.read(), before)
        self.assertFalse(os.path.exists(world.out))

    def test_an_index_at_head_is_left_alone(self):
        world = World(self)
        self.assertEqual(world.run(world.prompt()), 0)
        self.assert_nothing_started(world)

    def test_a_run_already_going_is_not_doubled(self):
        world = World(self, indexed="0" * 40)
        with open(world.pidfile(), "w") as f:
            f.write(str(os.getpid()))
        self.assertEqual(world.run(world.prompt()), 0)
        self.assert_nothing_started(world, before=str(os.getpid()))

    def test_a_linked_worktree_is_left_alone(self):
        world = World(self, indexed="0" * 40)
        tree = os.path.join(world.root, "tree")
        _git(world.repo, "worktree", "add", "-q", "-b", "lane", tree)
        self.assertEqual(world.run(world.prompt(cwd=tree)), 0)
        self.assert_nothing_started(world)

    def test_a_repository_gitnexus_does_not_index_is_left_alone(self):
        world = World(self, indexed=None)
        self.assertEqual(world.run(world.prompt()), 0)
        self.assertFalse(os.path.exists(world.out))

    def test_a_bash_call_without_git_never_starts_a_run(self):
        world = World(self, indexed="0" * 40)
        self.assertEqual(world.run(bash("ls", world.repo)), 0)
        self.assert_nothing_started(world)

    def test_no_gitnexus_on_path_means_nothing_happens(self):
        world = World(self, indexed="0" * 40, with_gitnexus=False)
        self.assertEqual(world.run(world.prompt()), 0)
        self.assert_nothing_started(world)

    def test_a_directory_outside_git_means_nothing_happens(self):
        world = World(self, indexed="0" * 40)
        self.assertEqual(world.run(world.prompt(cwd=world.root)), 0)
        self.assert_nothing_started(world)

    def test_unreadable_input_or_registry_means_nothing_happens(self):
        world = World(self, indexed="0" * 40)
        self.assertEqual(world.run("not json"), 0)
        self.assertEqual(world.run("[1, 2]"), 0)
        with open(os.path.join(world.home, "registry.json"), "w") as f:
            f.write("{broken")
        self.assertEqual(world.run(world.prompt()), 0)
        self.assert_nothing_started(world)


if __name__ == "__main__":
    unittest.main()


# --------------------------------------------------------------------------------------------- the restart (2026-11-11)
# Owner, 2026-11-11 ("the GitNexus hook"): every re-index left an open `gitnexus mcp` server on a stale handle; after a
# forced re-index the hook's follow-up restarts the servers Claude Code runs, which restart them on their next call.

CLAUDE = "/Users/u/.vscode/extensions/anthropic.claude-code-2.1.319-darwin-arm64/resources/native-binary/claude"
LISTING = f"""\
  100     1 {CLAUDE} --output-format stream-json
  200   100 node /Users/u/.nvm/versions/node/v26.13.0/bin/gitnexus mcp
  210   100 node /Users/u/.nvm/versions/node/v26.13.0/bin/gitnexus analyze --index-only /r
  300    50 node /usr/local/bin/gitnexus mcp
   50     1 -zsh
  400   100 /usr/bin/python3 -I -B /x/gitnexus_reindex.py
  500   100 node /opt/bin/gitnexus mcp --http
"""


class Servers(unittest.TestCase):
    def test_only_a_gitnexus_mcp_whose_parent_is_claude_code_is_chosen(self):
        # 200 and 500 run under Claude Code; 210 is an analyze, 300 runs under a shell, 400 is not gitnexus.
        self.assertEqual(gitnexus_reindex.servers(LISTING), [200, 500])

    def test_an_empty_or_garbled_listing_chooses_nothing(self):
        self.assertEqual(gitnexus_reindex.servers(""), [])
        self.assertEqual(gitnexus_reindex.servers("not a listing\n  x y z\n"), [])


class Restart(unittest.TestCase):
    """A planted tree: a process named `claude` and its `gitnexus mcp` child. Only the planted processes are ever
    listed to the restart, so no server of this machine is touched."""

    def plant(self) -> tuple[int, int]:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        pidfile = os.path.join(tmp.name, "child.pid")
        claude = os.path.join(tmp.name, "claude")
        with open(claude, "w") as f:
            f.write(
                f"#!{sys.executable} -I\n"
                "import subprocess, sys, time\n"
                f"child = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(120)', 'gitnexus', 'mcp'])\n"
                f"open({pidfile!r} + '.tmp', 'w').write(str(child.pid))\n"
                "import os; os.rename(" + repr(pidfile) + " + '.tmp', " + repr(pidfile) + ")\n"
                "time.sleep(120)\n"
            )
        os.chmod(claude, 0o755)
        parent = subprocess.Popen([claude], stdin=subprocess.DEVNULL, start_new_session=True)
        self.addCleanup(lambda: (parent.kill(), parent.wait()))

        def child_pid():
            try:
                with open(pidfile) as f:
                    return int(f.read())
            except (OSError, ValueError):
                return None

        child = wait.until("the planted server to start", child_pid, cap_s=30)
        self.addCleanup(lambda: _kill_quietly(child))
        return parent.pid, child

    def planted_listing(self, *pids: int) -> str:
        listing = gitnexus_reindex.ps_listing()
        return "\n".join(line for line in listing.splitlines() if line.split() and line.split()[0] in map(str, pids))

    def test_the_planted_server_is_terminated_and_its_claude_parent_lives(self):
        parent, child = self.plant()
        self.assertEqual(gitnexus_reindex.restart(self.planted_listing(parent, child)), [child])
        wait.until("the planted server to end", lambda: not _alive(child) or None, cap_s=30)
        self.assertTrue(_alive(parent))


class FollowUp(unittest.TestCase):
    def follow_up(self, exit_code: int) -> list[str]:
        world = World(self, indexed="0" * 40)
        calls: list[str] = []
        environ = {**world.environ, "FAKE_GITNEXUS_EXIT": str(exit_code)}
        code = gitnexus_reindex.reindex(
            os.path.join(world.bin, "gitnexus"), world.repo, environ, restart_servers=lambda: calls.append("restart")
        )
        self.assertEqual(world.started()["argv"], ["analyze", "--force", "--index-only", world.repo])
        self.assertEqual(code, exit_code)
        return calls

    def test_a_successful_reindex_restarts_the_servers(self):
        self.assertEqual(self.follow_up(0), ["restart"])

    def test_a_failed_reindex_restarts_nothing(self):
        self.assertEqual(self.follow_up(1), [])


def _alive(pid: int) -> bool:
    """Whether the process exists and is not a zombie (a killed child of the planted parent stays one until reaped)."""
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    state = subprocess.run(["ps", "-o", "stat=", "-p", str(pid)], capture_output=True, text=True).stdout.strip()
    return bool(state) and not state.startswith("Z")


def _kill_quietly(pid: int) -> None:
    with contextlib.suppress(OSError):
        os.kill(pid, 9)
