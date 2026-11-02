"""The gate's command line (plan check S4; design note §4).

    gate/run.py --tier T0..T4 [--job NAME] [--run-dir DIR] [--registry PATH]

Exit 0: pass. Exit 1: the gate failed. Exit 2: usage or internal error. The last two stdout lines are always
`GATE PASSED|GATE FAILED` and `exit: <n>`: main catches everything, because an uncaught exception would exit 1,
which means "gate failed".
"""

import argparse
import os
import signal
import sys
from collections.abc import Callable, Mapping
from dataclasses import dataclass

from gate.runner import clock, env, registry, scheduler, workers
from gate.runner.clock import Clock, Timers
from gate.runner.proc import Group
from gate.toolchain import manifest, parity

DEFAULT_REGISTRY = os.path.join("gate", "registry.json")


class UsageError(Exception):
    """The command line, or the state it found, is not runnable; exit 2."""


class RealToolchain:
    """gate.toolchain, behind the seam the scheduler consumes."""

    def __init__(self, repo: str, environ: Mapping[str, str]) -> None:
        self._tools = manifest.load(os.path.join(repo, "tools.json"), manifest.platform_key())
        self._cache = manifest.cache_root(environ)

    def path_dirs(self) -> list[str]:
        return manifest.path_dirs(self._cache, self._tools)

    def tool_count(self) -> int:
        return len(self._tools)

    def parity(self, env: Mapping[str, str], *, full_tree: bool) -> list[str]:
        return parity.check(self._tools, self._cache, env, full_tree=full_tree)


def read_commit(repo: str, environ: Mapping[str, str], run_dir: str, tier: str) -> str:
    """HEAD's sha, or "index" at T0 (plan check S2)."""
    if tier == "T0":
        return "index"
    out, err = os.path.join(run_dir, "head.stdout"), os.path.join(run_dir, "head.stderr")
    group = Group()
    proc = group.spawn(("git", "rev-parse", "HEAD"), cwd=repo, env=environ, stdout=out, stderr=err)
    assert proc is not None  # a fresh group has not been signalled
    code, _ = group.reap(proc)
    if code != 0:
        raise UsageError(f"cannot read HEAD: git rev-parse exit {code}")
    with open(out, "rb") as f:
        return f.read().decode("ascii", errors="replace").strip()


@dataclass(frozen=True)
class Deps:
    """Everything main takes from the world; tests pass fakes."""

    repo: str
    environ: Mapping[str, str]
    clock: Clock
    timers: Timers
    toolchain: Callable[[str, Mapping[str, str]], scheduler.Toolchain]
    commit: Callable[[str, Mapping[str, str], str, str], str] = read_commit
    new_run_id: Callable[[int], str] = clock.run_id
    record_go_env: bool = True


def default_deps() -> Deps:
    repo = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    return Deps(repo=repo, environ=os.environ, clock=clock.REAL, timers=clock.RealTimers(), toolchain=RealToolchain)


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise UsageError(f"{message}\n{self.format_usage().strip()}")


def _parse(argv: list[str]) -> argparse.Namespace:
    parser = _Parser(prog="gate/run.py", add_help=False, allow_abbrev=False)
    parser.add_argument("--tier", required=True, choices=registry.TIERS)
    parser.add_argument("--job")
    parser.add_argument("--run-dir", dest="run_dir")
    parser.add_argument("--registry")
    return parser.parse_args(argv)


def _prepare_run_dir(given: str | None, repo: str, run_id: str) -> str:
    """A run owns its directory: a directory holding an earlier run could pass off stale records as its own."""
    run_dir = os.path.abspath(given) if given else os.path.join(repo, ".gate", "runs", run_id)
    if os.path.isdir(run_dir) and os.listdir(run_dir):
        raise UsageError(f"run directory {run_dir} is not empty")
    os.makedirs(run_dir, exist_ok=True)
    return run_dir


def _run(argv: list[str], deps: Deps) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)  # ignored, subprocess reports a dead child as exit 0: a false green
    args = _parse(argv)
    count = workers.parse(deps.environ)
    path = args.registry or os.path.join(deps.repo, DEFAULT_REGISTRY)
    rows = registry.select(registry.load(path), args.tier, args.job)
    toolchain = deps.toolchain(deps.repo, deps.environ)
    run_id = deps.new_run_id(deps.clock.wall_ms())
    run_dir = _prepare_run_dir(args.run_dir, deps.repo, run_id)
    clean = env.clean_env(deps.environ, toolchain.path_dirs())
    runner = scheduler.Runner(
        rows=rows,
        tier=args.tier,
        job=args.job,
        workers=count,
        run_dir=run_dir,
        run_id=run_id,
        commit=deps.commit(deps.repo, clean, run_dir, args.tier),
        repo=deps.repo,
        environ=deps.environ,
        toolchain=toolchain,
        clock=deps.clock,
        timers=deps.timers,
        record_go_env=deps.record_go_env,
        install_signals=True,
    )
    result = runner.run()
    sys.stdout.write(result.summary)
    sys.stdout.flush()
    return result.exit_code


def main(argv: list[str], *, deps: Deps | None = None) -> int:
    try:
        return _run(argv, deps if deps is not None else default_deps())
    except (UsageError, registry.RegistryError, workers.WorkersError, manifest.ManifestError) as e:
        print(e)
    except BaseException as e:
        print(f"internal error: {type(e).__name__}: {e}")
    print("GATE FAILED")
    print("exit: 2")
    sys.stdout.flush()
    return 2
