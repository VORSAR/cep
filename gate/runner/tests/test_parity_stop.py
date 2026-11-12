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

from gate import wait
from gate.runner import scheduler
from gate.runner.tests import support
from gate.toolchain import manifest

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

    def test_a_stop_during_the_tree_hash_kills_the_tree_child_and_records_the_interrupt(self):
        cache = os.path.join(self.root, "cache")
        with open(os.path.join(self.repo, "tools.json"), "w", encoding="utf-8") as f:
            json.dump({"schema": 1, "tools": [self.fake_tool(cache)]}, f)
        marker = os.path.join(self.root, "tree-child.pid")
        planted = os.path.join(self.root, "planted.py")
        with open(planted, "w", encoding="utf-8") as f:
            f.write(PLANTED)
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
        wait.until(f"the planted tree child {pid} to be gone after the harness exited", lambda: pid_gone(pid))

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
