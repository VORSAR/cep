"""BA10: a stop while parity hashes a tree leaves nothing running (S1.1c-3 design §3.6; owner, S1.1c-3's plan check,
decision 10; DA-S13, DA-S14, DQ-S11).

A harness process (written here, in the test's world) runs the real runner, `main.main` with `main.RealToolchain`,
at T2 over one fake tool in a fixture cache, with `parity.TREE_CHILD` set to a planted child that writes its pid to
a marker and then waits on an event nothing sets. The test waits for the marker, interrupts the harness with SIGINT,
waits for the harness to exit, then waits for the planted pid to be gone, each wait capped and naming its state; then
parity's record says the interrupt (verdict fail, exit code NEVER_RAN, reason "interrupted"; scheduler.py `_on_stop`)
and the run exits 1. The planted child dies only if it was started through the runner's parity group: the stop
TERMs that group (`_on_stop`). A child started outside it (a plain Popen in its own session) outlives the run, and
the pid-gone wait names it.

Red today: parity hashes in-process and never reads `parity.TREE_CHILD`, so the marker never appears; the harness
runs to its end first and the test says so.
"""

import contextlib
import hashlib
import json
import os
import signal
import subprocess
import sys
import threading

from gate import wait
from gate.runner import scheduler
from gate.runner.tests import support
from gate.toolchain import manifest, parity

ARCHIVE = "ab" * 32
HARNESS = """import json, os, sys
with open(sys.argv[1], encoding="utf-8") as f:
    cfg = json.load(f)
sys.path.insert(0, cfg["repo"])
from gate.runner import clock, main
from gate.toolchain import parity
parity.TREE_CHILD = tuple(cfg["child"])
deps = main.Deps(
    repo=cfg["world"],
    environ=os.environ,
    clock=clock.REAL,
    timers=clock.RealTimers(),
    toolchain=main.RealToolchain,
    commit=lambda repo, env, run_dir, tier: "0" * 40,
    new_run_id=lambda wall_ms: "parity-stop",
    record_go_env=False,
)
sys.exit(main.main(["--tier", "T2", "--registry", cfg["registry"], "--run-dir", cfg["run_dir"]], deps=deps))
"""
# The planted tree child: argv[1] the marker, argv[2] the tool's home (tree_digest appends it).
PLANTED = """import os, sys, threading
with open(sys.argv[1] + ".tmp", "w", encoding="utf-8") as f:
    f.write(str(os.getpid()))
os.rename(sys.argv[1] + ".tmp", sys.argv[1])
threading.Event().wait()
"""

# C6 (DA-S14): the same child, deaf to the stop's TERM.
IGNORES_TERM = "import signal\nsignal.signal(signal.SIGTERM, signal.SIG_IGN)\n" + PLANTED


def pid_gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    except PermissionError:
        return False
    return False


def kill(pid: int) -> None:
    with contextlib.suppress(ProcessLookupError):
        os.kill(pid, signal.SIGKILL)


def kill_group(pgid: int) -> None:
    with contextlib.suppress(ProcessLookupError):
        os.killpg(pgid, signal.SIGKILL)


class ParityStop(support.GateTest):
    def fake_tool(self, cache: str) -> dict:
        """One executable tool, intact in the cache: parity passes it up to the tree hash."""
        home = os.path.join(cache, f"demo-1.0-{ARCHIVE[:12]}")
        os.makedirs(os.path.join(home, "bin"))
        demo = os.path.join(home, "bin", "demo")
        with open(demo, "w", encoding="utf-8") as f:
            f.write(f"#!{sys.executable} -IB\nprint('demo 1.0')\n")
        os.chmod(demo, 0o555)
        with open(demo, "rb") as f:
            exec_sha256 = {"bin/demo": hashlib.sha256(f.read()).hexdigest()}
        return {
            "command": "demo",
            "name": "demo",
            "path_dir": "bin",
            "platforms": {
                manifest.platform_key(): {
                    "archive_sha256": ARCHIVE,
                    "exec_sha256": exec_sha256,
                    "strip": "demo/",
                    "tree_sha256": manifest.tree_sha256(home),  # fixture set-up: today's in-process hash passes
                    "url": "file:///unused",
                }
            },
            "version": "1.0",
            "version_argv": ["bin/demo", "--version"],
            "version_line": "demo 1.0",
        }

    def interrupt_during_the_tree_hash(self, planted_body: str) -> tuple[int, str]:
        """Run the harness until the planted tree child holds the hash, SIGINT it, wait for it to exit and check the
        records; return the planted child's pid and the run directory."""
        cache = os.path.join(self.root, "cache")
        with open(os.path.join(self.repo, "tools.json"), "w", encoding="utf-8") as f:
            json.dump({"schema": 1, "tools": [self.fake_tool(cache)]}, f)
        marker = os.path.join(self.root, "tree-child.pid")
        planted = os.path.join(self.root, "planted.py")
        with open(planted, "w", encoding="utf-8") as f:
            f.write(planted_body)
        harness = os.path.join(self.root, "harness.py")
        with open(harness, "w", encoding="utf-8") as f:
            f.write(HARNESS)
        run_dir = os.path.join(self.root, "harness-run")
        cfg = os.path.join(self.root, "harness.json")
        with open(cfg, "w", encoding="utf-8") as f:
            json.dump(
                {
                    "repo": support.REPO,
                    "world": self.repo,
                    "child": [sys.executable, "-I", "-B", planted, marker],
                    "registry": self.registry_file([self.row("x")]),
                    "run_dir": run_dir,
                },
                f,
            )
        env = support.base_env()
        env["VORSAR_CEP_TOOLS_CACHE"] = cache
        output = os.path.join(self.root, "harness.out")
        with open(output, "wb") as out:
            proc = subprocess.Popen(
                [sys.executable, "-I", "-B", harness, cfg],
                cwd=self.repo,
                env=env,
                stdin=subprocess.DEVNULL,
                stdout=out,
                stderr=subprocess.STDOUT,
                start_new_session=True,
            )
        self.addCleanup(proc.wait)
        self.addCleanup(kill_group, proc.pid)

        def lines() -> list[str]:
            with open(output, "rb") as f:
                return f.read().decode("utf-8", errors="replace").splitlines()

        wait.until(
            "the planted tree child to write its pid, or the harness to exit first",
            lambda: os.path.exists(marker) or proc.poll() is not None,
        )
        if not os.path.exists(marker):
            tail = lines()[-4:]
            self.fail(
                f"the harness exited {proc.returncode} before the planted tree child started: parity never spawned "
                f"parity.TREE_CHILD; its last lines: {tail}"
            )
        with open(marker, encoding="utf-8") as f:
            pid = int(f.read())
        self.addCleanup(kill, pid)

        proc.send_signal(signal.SIGINT)
        wait.until("the harness to exit after SIGINT", lambda: proc.poll() is not None)

        self.assertEqual(proc.returncode, 1)
        self.assertEqual(lines()[-2:], ["GATE FAILED", "exit: 1"])
        with open(os.path.join(run_dir, "records", "lint", "parity.json"), "rb") as f:
            parity = json.loads(f.read())
        self.assertEqual(
            (parity["verdict"], parity["exit_code"], parity["reason"]),
            ("fail", scheduler.NEVER_RAN, scheduler.INTERRUPTED),
        )
        with open(os.path.join(run_dir, "records", "lint", "x.json"), "rb") as f:
            self.assertEqual(json.loads(f.read())["reason"], scheduler.INTERRUPTED)
        return pid, run_dir

    def test_a_stop_during_the_tree_hash_kills_the_tree_child_and_records_the_interrupt(self):
        pid, _ = self.interrupt_during_the_tree_hash(PLANTED)
        wait.until(f"the planted tree child {pid} to be gone after the harness exited", lambda: pid_gone(pid))

    def test_a_tree_child_deaf_to_term_is_named_left_running_and_killed(self):
        # C6 (DA-S14): the stop ends the run's loop at once, so the parity group is probed and KILLed in run's
        # finally; the S1.1c-1-1 probe names what the TERM left, in the parity scratch's left_running.txt.
        pid, run_dir = self.interrupt_during_the_tree_hash(IGNORES_TERM)
        left = os.path.join(run_dir, "parity", "left_running.txt")
        self.assertTrue(os.path.exists(left), "the parity group's probe named nothing left running")
        with open(left, encoding="utf-8") as f:
            named = f.read()
        self.assertIn(str(pid), named)
        wait.until(f"the TERM-deaf tree child {pid} to be gone after the harness exited", lambda: pid_gone(pid))


class HashesThenBreaksTheLoop(support.FakeToolchain):
    """A parity that starts the planted tree child through the run's parity group (`parity.tree_digest` with the
    spawn the runner gives), and once the child has written its pid, puts on the runner's queue an event its loop
    does not know: `_handle` raises, which ends `run` early with no stop (CA-L3)."""

    def __init__(self, argv: tuple[str, ...], marker: str) -> None:
        super().__init__()
        self.argv = argv
        self.marker = marker
        self.runner: scheduler.Runner | None = None
        self.outcome = ""
        self.done = threading.Event()

    def parity(self, env, *, full_tree: bool, spawn, scratch: str) -> list[str]:
        def spawn_then_break(argv, **kw):
            proc = spawn(argv, **kw)
            wait.until(f"the planted tree child to write {self.marker}", lambda: os.path.exists(self.marker))
            assert self.runner is not None
            self.runner._q.put_nowait(("not an event",))
            return proc

        try:
            home = os.path.dirname(self.marker)
            parity.tree_digest(home, spawn=spawn_then_break, argv=self.argv, env=env, scratch=scratch)
            self.outcome = "hashed"
        except (OSError, parity.TreeStopped) as e:  # the runner is gone: this thread only notes how it ended
            self.outcome = f"{type(e).__name__}: {e}"
        finally:
            self.done.set()
        return []


class ParityEarlyEnd(support.GateTest):
    def test_an_exception_that_ends_run_during_the_tree_hash_kills_the_tree_child(self):
        # CA-L3: the parity group's cleanup (kill_left and its probe) runs on any early end of run while a tree child
        # may be alive, not only after a stop: here a bug's exception ends the loop, and no stop ever came.
        marker = os.path.join(self.root, "tree-child.pid")
        planted = os.path.join(self.root, "planted.py")
        with open(planted, "w", encoding="utf-8") as f:
            f.write(PLANTED)
        toolchain = HashesThenBreaksTheLoop((sys.executable, "-I", "-B", planted, marker), marker)
        runner = self.runner([self.row("x")], toolchain=toolchain)
        toolchain.runner = runner
        with self.assertRaises(AssertionError) as caught:
            runner.run()
        self.assertIn("unknown event", str(caught.exception))  # the witness: the loop ended by the exception
        with open(marker, encoding="utf-8") as f:
            pid = int(f.read())
        self.addCleanup(kill, pid)
        wait.until(f"the planted tree child {pid} to be gone after run raised", lambda: pid_gone(pid))
        wait.until("the parity thread's tree hash to return", toolchain.done.is_set)
        self.assertEqual(toolchain.outcome, "TreeChildFailed: the tree child exited -9")
        left = os.path.join(self.run_dir, "parity", "left_running.txt")
        with open(left, encoding="utf-8") as f:
            self.assertIn(str(pid), f.read())
