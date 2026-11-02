"""Shared helpers of the runner's tests: each test owns its world (testing.md rule 3)."""

import dataclasses
import faulthandler
import json
import os
import platform
import stat
import subprocess
import sys
import tempfile
import threading
import unittest

from gate import wait
from gate.runner import clock, main, records, registry, scheduler

GATE = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
REPO = os.path.dirname(GATE)
ROWFIX = os.path.join(GATE, "runner", "testdata", "rowfix.py")
CEILING_S = 240
PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
RUN_ID = "20261102T000000Z-abc123"
COMMIT = "0" * 40
HIDDEN = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE")


def fix(*actions: object) -> list[str]:
    """The command of a fixture row running these actions under this interpreter."""
    return [sys.executable, "-I", "-B", ROWFIX, *map(str, actions)]


def tool_bin(name: str) -> str:
    """The pinned tool's PATH directory, read from tools.json the way the design note lays out the cache."""
    with open(os.path.join(REPO, "tools.json"), encoding="utf-8") as f:
        tools = json.load(f)["tools"]
    tool = next(t for t in tools if t["name"] == name)
    plat = PLATFORMS[(sys.platform, platform.machine())]
    cache = os.environ.get("VORSAR_CEP_TOOLS_CACHE") or os.path.expanduser("~/.cache/vorsar/cep/tools")
    home = os.path.join(cache, f"{tool['name']}-{tool['version']}-{tool['platforms'][plat]['archive_sha256'][:12]}")
    return os.path.join(home, tool["path_dir"])


def base_env() -> dict[str, str]:
    """A world-owned environment: nothing of the caller's git config or location leaks into a test."""
    env = {k: v for k, v in os.environ.items() if k not in HIDDEN}
    env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
    return env


def run_tool(argv: list[str], cwd: str, env: dict[str, str] | None = None) -> tuple[int, bytes, bytes]:
    """Run a command to completion with its output in files, never pipes; give (exit code, stdout, stderr)."""
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        code = subprocess.run(
            argv, cwd=cwd, env=base_env() if env is None else env, stdin=subprocess.DEVNULL, stdout=out, stderr=err
        ).returncode
        out.seek(0)
        err.seek(0)
        return code, out.read(), err.read()


def release(path: str) -> None:
    """Let a row held on `wait PATH` go. Opening needs a reader, so this also witnesses that the row is holding."""

    def open_write() -> tuple[int] | None:
        try:
            return (os.open(path, os.O_WRONLY | os.O_NONBLOCK),)
        except OSError:
            return None

    fd = wait.until(f"a row to hold {path}", open_write)[0]
    os.write(fd, b"g")
    os.close(fd)


class Witness:
    """A FIFO's read end: one start byte per process, then end-of-file when all are gone (design note §9).

    End-of-file alone proves nothing: a FIFO reads as end-of-file before any writer opens it. So `gone` refuses
    to run until `started` has seen every start byte.
    """

    def __init__(self, path: str) -> None:
        self.path = path
        os.mkfifo(path)
        self.fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK)
        self.bytes = 0
        self.expected = None

    def _read(self) -> bytes | None:
        try:
            return os.read(self.fd, 64)
        except BlockingIOError:
            return None

    def started(self, n: int, *, cap_s: float = wait.DEFAULT_CAP_S) -> None:
        def have_all() -> bool:
            self.bytes += len(self._read() or b"")
            return self.bytes >= n

        wait.until(f"{n} start byte(s) on {self.path}", have_all, cap_s=cap_s)
        self.expected = n

    def more(self, n: int) -> None:
        """Wait for n further bytes (a process signalling that it saw something)."""
        self.started(self.bytes + n)

    def gone(self) -> None:
        if self.expected is None:
            raise AssertionError("no positive witness: read the start bytes before trusting end-of-file")

        def at_eof() -> bool:
            data = self._read()
            self.bytes += len(data or b"")
            return data == b""

        wait.until(f"every process holding {self.path} to be gone", at_eof)

    def close(self) -> None:
        os.close(self.fd)


class FakeTimer:
    def __init__(self, label: str, delay_s: float, fire) -> None:
        self.label = label
        self.delay_s = delay_s
        self._fire = fire
        self.cancelled = False

    def cancel(self) -> None:
        self.cancelled = True

    def fire(self) -> None:
        if not self.cancelled:
            self._fire()


class FakeTimers:
    """Timers that fire only when a test releases them, after a positive witness (plan check S3c)."""

    def __init__(self) -> None:
        self.armed: list[FakeTimer] = []

    def start(self, label: str, delay_s: float, fire) -> FakeTimer:
        timer = FakeTimer(label, delay_s, fire)
        self.armed.append(timer)
        return timer

    def expect(self, label: str) -> FakeTimer:
        """Wait until a timer with this label is armed."""
        return wait.until(f"timer {label} to be armed", lambda: next((t for t in self.armed if t.label == label), None))

    def labels(self) -> list[str]:
        return [t.label for t in self.armed]


class FakeToolchain:
    """Stands in for gate.toolchain: its PATH prefix and a scripted parity result."""

    def __init__(self, dirs: list[str] | None = None, reasons: list[str] | None = None) -> None:
        self.dirs = [os.path.dirname(sys.executable)] if dirs is None else dirs
        self.reasons = reasons or []
        self.calls: list[tuple[dict[str, str], bool]] = []
        self.hold: threading.Event | None = None

    def path_dirs(self) -> list[str]:
        return self.dirs

    def tool_count(self) -> int:
        return 5

    def parity(self, env, *, full_tree: bool) -> list[str]:
        self.calls.append((dict(env), full_tree))
        if self.hold is not None:
            self.hold.wait()
        return self.reasons


FROZEN = clock.Clock(wall_ms=lambda: 0, mono_ns=lambda: 0)


class Handle:
    """A Runner running in a background thread, so a test body can drive it line by line."""

    def __init__(self, test: unittest.TestCase, runner: scheduler.Runner) -> None:
        self.runner = runner
        self.result: scheduler.Result | None = None
        self.error: BaseException | None = None
        self.thread = threading.Thread(target=self._main, name="runner-under-test", daemon=True)
        self.thread.start()
        test.addCleanup(self.abort)

    def _main(self) -> None:
        try:
            self.result = self.runner.run()
        except BaseException as e:
            self.error = e
            raise

    def saw(self, event: str, row: str) -> dict:
        """Wait for a scheduler trace entry; the trace is the witness, so this needs no clock."""
        return wait.until(
            f"trace event {event} of {row}",
            lambda: next((e for e in list(self.runner.trace) if e["event"] == event and e["id"] == row), None),
        )

    def seq(self, event: str, row: str) -> int:
        return self.saw(event, row)["seq"]

    def events(self, event: str) -> list[str]:
        return [e["id"] for e in list(self.runner.trace) if e["event"] == event]

    def join(self) -> scheduler.Result:
        wait.until("the runner to finish", lambda: not self.thread.is_alive())
        if self.error is not None:
            raise self.error
        assert self.result is not None
        return self.result

    def abort(self) -> None:
        """After a failed test: free whatever still runs. Two signals mean KILL now."""
        if self.thread.is_alive():
            self.runner.post_signal(2)
            self.runner.post_signal(2)
            self.thread.join(wait.DEFAULT_CAP_S)


class GateTest(unittest.TestCase):
    """Gives each test a ceiling and its own world: a temporary directory and a small git repository."""

    def setUp(self) -> None:
        faulthandler.dump_traceback_later(CEILING_S, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory(prefix="gate-")
        self.addCleanup(tmp.cleanup)
        self.root = os.path.realpath(tmp.name)
        self.repo = os.path.join(self.root, "repo")
        os.mkdir(self.repo)
        self.git("init", "-q")
        self.track("one.txt", "1\n")
        self.run_dir = os.path.join(self.root, "run")
        os.mkdir(self.run_dir)
        self.timers = FakeTimers()
        self.toolchain = FakeToolchain()
        self.counter = 0

    def git(self, *args: str) -> bytes:
        code, out, err = run_tool(["git", *args], self.repo)
        self.assertEqual(code, 0, err.decode(errors="replace"))
        return out

    def write(self, rel: str, text: str, mode: int | None = None) -> str:
        path = os.path.join(self.repo, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        if mode is not None:
            os.chmod(path, mode)
        return path

    def track(self, rel: str, text: str) -> str:
        path = self.write(rel, text)
        self.git("add", "--", rel)
        return path

    def fifo(self, name: str) -> str:
        path = os.path.join(self.root, name)
        os.mkfifo(path)
        return path

    def witness(self, name: str) -> Witness:
        w = Witness(os.path.join(self.root, name))
        self.addCleanup(w.close)
        return w

    def script(self, name: str, directory: str, text: str) -> str:
        """An executable generated at test time, with this interpreter on its shebang line."""
        os.makedirs(directory, exist_ok=True)
        path = os.path.join(directory, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(f"#!{sys.executable} -I\n{text}")
        os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR)
        return path

    def row(self, row_id: str, *actions: object, **over: object) -> registry.Row:
        """A row that covers one.txt; with no actions it passes (covered 1 = expected 1)."""
        fields = {
            "id": row_id,
            "job": "lint",
            "tiers": ("T2",),
            "blocking": True,
            "timeout_s": 60,
            "needs": (),
            "scope": ("tools",),
            "expected_scope": ("git", "one.txt"),
            "min_wants": 0,
            "lands_in": "S1.1a-1",
            "exclusive": False,
            "command": tuple(fix(*(actions or ("print", "GATE_COVERED 1")))),
        }
        fields.update(over)
        return registry.Row(**fields)

    def good(self, row_id: str, *before: object, **over: object) -> registry.Row:
        """A row running `before`, then printing a correct GATE_COVERED."""
        return self.row(row_id, *before, "print", "GATE_COVERED 1", **over)

    def runner(self, rows: list[registry.Row], **over: object) -> scheduler.Runner:
        env = base_env()
        args = {
            "rows": rows,
            "tier": "T2",
            "job": None,
            "workers": 4,
            "run_dir": self.run_dir,
            "run_id": RUN_ID,
            "commit": COMMIT,
            "repo": self.repo,
            "environ": env,
            "toolchain": self.toolchain,
            "clock": FROZEN,
            "timers": self.timers,
            "record_go_env": False,
        }
        args.update(over)
        return scheduler.Runner(**args)

    def start(self, rows: list[registry.Row], **over: object) -> Handle:
        return Handle(self, self.runner(rows, **over))

    def run_rows(self, rows: list[registry.Row], **over: object) -> scheduler.Result:
        return self.start(rows, **over).join()

    def registry_file(self, rows: list[registry.Row]) -> str:
        """Write rows as a registry document; the JSON's shape is what `registry.load` reads."""
        path = os.path.join(self.root, "registry.json")
        with open(path, "wb") as f:
            f.write(records.json_bytes({"schema": 1, "rows": [dataclasses.asdict(r) for r in rows]}))
        return path

    def deps(self, **over: object) -> main.Deps:
        args = {
            "repo": self.repo,
            "environ": base_env(),
            "clock": FROZEN,
            "timers": self.timers,
            "toolchain": lambda repo, environ: self.toolchain,
            "commit": lambda repo, env, run_dir, tier: COMMIT,
            "new_run_id": lambda wall_ms: RUN_ID,
            "record_go_env": False,
        }
        args.update(over)
        return main.Deps(**args)

    def record(self, job: str, row_id: str) -> dict:
        with open(os.path.join(self.run_dir, "records", job, f"{row_id}.json"), "rb") as f:
            return json.loads(f.read())

    def row_file(self, job: str, row_id: str, name: str) -> bytes:
        with open(os.path.join(self.run_dir, "rows", job, row_id, name), "rb") as f:
            return f.read()

    def reason(self, row_id: str, job: str = "lint") -> str:
        return self.record(job, row_id)["reason"]

    def assertPinnedJson(self, path: str) -> None:
        """The file is whole and in the pinned JSON form."""
        with open(path, "rb") as f:
            data = f.read()
        self.assertEqual(records.json_bytes(json.loads(data)), data, path)


def go_env() -> dict[str, str]:
    """The test environment with the pinned go first on PATH."""
    env = base_env()
    env["PATH"] = os.pathsep.join([tool_bin("go"), env.get("PATH", "")])
    return env
