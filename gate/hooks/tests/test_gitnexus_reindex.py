"""The Claude Code hook that keeps the GitNexus code graph at HEAD (DEVELOPMENT.md, "The code graph").

Each test owns its world: a temporary git repository, a temporary GitNexus registry (GITNEXUS_HOME) and a fake
`gitnexus` on PATH that records how it was started. Nothing touches the real index.
"""

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
        self.assertEqual(world.started(), {"argv": ["analyze", "--index-only", world.repo], "cwd": world.repo})
        self.assertTrue(os.path.exists(world.pidfile()))

    def test_a_git_command_from_a_subdirectory_counts_for_the_checkout(self):
        world = World(self, indexed="0" * 40)
        sub = os.path.join(world.repo, "sub")
        os.mkdir(sub)
        self.assertEqual(world.run(bash("git commit -m y", sub)), 0)
        self.assertEqual(world.started()["argv"], ["analyze", "--index-only", world.repo])

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
