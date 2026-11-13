"""What the proof drivers share (design note §8; QA DQ1, DQ24): a world of real copies, and the real runner at T2.

Every driver:
- works on real copies in its own temporary directory, never hard links, and never writes the shared tool cache:
  a temporary cache links to the real, read-only tool directories, and copies only what it must change;
- runs the real runner, gate/run.py under the pinned python3.15 with -I -B -X utf8, at the tier each names (T2 for most;
T4 for the notbuilt expiry's) (so it can
  never recurse into `proofs`), with a new run directory;
- asserts the red verdict, its exact reason, `GATE FAILED` and `exit: 1`, and, outside the parity drivers, a parity
  PASS in the same run;
- inside a full-tree proofs row, hands each nested T2-T4 run the outer run's verified tree digests
  (`--parity-handoff`; proof-world speed-up design §3.6), and checks every passing parity names that reuse. The parity
  drivers never pass it: they hash for real.
"""

import contextlib
import faulthandler
import json
import os
import shutil
import signal
import subprocess
import tempfile
import unittest
from dataclasses import dataclass

from gate import term
from gate.toolchain import manifest

DRIVERS = os.path.dirname(os.path.abspath(__file__))
PROOFS = os.path.dirname(DRIVERS)
REPO = os.path.dirname(os.path.dirname(PROOFS))
TIER = "T2"
CEILING_S = 900  # one driver test, under the stress harness's CPU hogs
RUN_CAP_S = 600  # one nested gate run
GRACE_S = 5
NESTED_GRACE_S = 2  # under the runner's 5 s grace, so the driver's own cleanups run after it
RECOVER = "run make clean-tools tools"  # the fix every parity reason but "not installed" names (Lane B's strings)
# The scheduler's full-tree tiers (gate/runner/scheduler.py), written out so the drivers never take from the code they
# prove when a nested run reads a hand-off.
FULL_TREE_TIERS = frozenset({"T2", "T3", "T4"})
HIDDEN = (
    "GIT_DIR",
    "GIT_WORK_TREE",
    "GIT_INDEX_FILE",
    "VORSAR_CEP_WORKERS",
    "VORSAR_CEP_TOOLS_CACHE",
    "VORSAR_CEP_VULNDB",  # S1.1c-4 design §2.3: a world reads only the copy it plants
)
GIT_IDENTITY = ("-c", "user.name=gate proofs", "-c", "user.email=proofs@gate.invalid", "-c", "commit.gpgsign=false")
# git's own config, through its environment, for every git a world starts (the nested runner's too): a commit
# otherwise starts `git maintenance run --auto --detach`, which outlives it and writes into .git during cleanup.
QUIET_GIT = {
    "GIT_CONFIG_COUNT": "2",
    "GIT_CONFIG_KEY_0": "maintenance.auto",
    "GIT_CONFIG_VALUE_0": "false",
    "GIT_CONFIG_KEY_1": "gc.auto",
    "GIT_CONFIG_VALUE_1": "0",
}


def tools() -> tuple[manifest.Tool, ...]:
    return tuple(manifest.load(os.path.join(REPO, "tools.json"), manifest.platform_key()))


def tool(name: str) -> manifest.Tool:
    return next(t for t in tools() if t.name == name)


def goos_goarch() -> tuple[str, str]:
    goos, goarch = manifest.platform_key().split("-")
    return goos, goarch


def real_cache() -> str:
    """The shared tool cache, read-only to every driver."""
    return os.path.realpath(manifest.cache_root(os.environ))


def home(cache_dir: str, name: str) -> str:
    return manifest.tool_home(cache_dir, tool(name))


def pinned_python() -> str:
    python = tool("python")
    return os.path.realpath(os.path.join(home(real_cache(), "python"), python.path_dir, python.command))


def registry_row(row_id: str) -> dict:
    """A row exactly as the real gate/registry.json has it."""
    with open(os.path.join(REPO, "gate", "registry.json"), "rb") as f:
        rows = json.loads(f.read())["rows"]
    found = [row for row in rows if row["id"] == row_id]
    if len(found) != 1:
        raise AssertionError(f"gate/registry.json has {len(found)} rows with id {row_id!r}, not 1")
    return found[0]


# The rows the coverage and proofs-row proofs run, read from gate/registry.json itself, so a driver can never run
# a stale copy of a row (forwarded item ii).
UNIT_ROW = registry_row("unit")
COVERAGE_ROW = registry_row("coverage-inclusion")
PROOFS_ROW = registry_row("proofs")


def handoff() -> str | None:
    """The outer run's parity hand-off, `<GATE_RUN_DIR>/parity/handoff.json`, when this driver runs inside a full-tree
    gate row; None for a driver run on its own (design §3.6, QB-L2)."""
    run_dir = os.environ.get("GATE_RUN_DIR", "")
    if os.environ.get("GATE_TIER") in FULL_TREE_TIERS and run_dir:
        return os.path.join(run_dir, "parity", "handoff.json")
    return None


def _given(wanted: bool | str, tier: str) -> str | None:
    """The hand-off World.run passes: an explicit path as given; the outer run's at a full-tree tier when wanted."""
    if isinstance(wanted, str):
        return wanted
    return handoff() if wanted and tier in FULL_TREE_TIERS else None


def handoff_run_id(path: str) -> str:
    """The run id of the parity hand-off at `path`, which must exist and list every tool of tools(); otherwise an
    AssertionError naming the path and what is missing (design §3.6, CA-S7, QB-S2a)."""
    try:
        with open(path, "rb") as f:
            doc = json.loads(f.read())
    except OSError as e:
        raise AssertionError(f"the parity hand-off {path}: {e.strerror}") from None
    except ValueError as e:
        raise AssertionError(f"the parity hand-off {path} is not JSON: {e}") from None
    if not (isinstance(doc, dict) and isinstance(doc.get("run_id"), str) and isinstance(doc.get("tools"), list)):
        raise AssertionError(f"the parity hand-off {path} has no run_id or no tools list")
    listed = {entry.get("name") for entry in doc["tools"] if isinstance(entry, dict)}
    missing = [t.name for t in tools() if t.name not in listed]
    if missing:
        raise AssertionError(f"the parity hand-off {path} does not list {', '.join(missing)}")
    return doc["run_id"]


def _stop_group(pgid: int, signum: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pgid, signum)


def run_process(argv: list[str], *, cwd: str, env: dict[str, str], out: str, err: str, cap_s: float) -> int:
    """Run to completion in its own group, output to files; then kill whatever of the group is left.

    Stopped while waiting (the shim's TERM, or Ctrl-C), it passes a TERM on first: a nested gate then stops its own
    rows, each in a session of its own, which a KILL of its group would leave running (review BA1). It waits
    NESTED_GRACE_S for that, inside the runner's own grace, so this driver's cleanups still run before the KILL.
    """
    proc = None
    try:
        with open(out, "wb") as o, open(err, "wb") as e:
            proc = term.popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=o, stderr=e)
        return proc.wait(timeout=cap_s)
    except subprocess.TimeoutExpired:
        _stop_group(proc.pid, signal.SIGTERM)
        with contextlib.suppress(subprocess.TimeoutExpired):
            proc.wait(timeout=GRACE_S)
        _stop_group(proc.pid, signal.SIGKILL)
        proc.wait()
        raise AssertionError(f"timed out after {cap_s}s waiting for {' '.join(argv[:6])} to finish") from None
    except BaseException:
        if proc is not None:
            _stop_group(proc.pid, signal.SIGTERM)
            with contextlib.suppress(subprocess.TimeoutExpired):
                proc.wait(timeout=NESTED_GRACE_S)
        raise
    finally:
        if proc is not None:
            _stop_group(proc.pid, signal.SIGKILL)


def read_text(path: str) -> str:
    with open(path, "rb") as f:
        return f.read().decode("utf-8", errors="replace")


@dataclass(frozen=True)
class Run:
    """One finished nested gate run; `handoff` says whether it was given a parity hand-off, `handoff_run_id` the run
    that hand-off came from."""

    code: int
    stdout: str
    run_dir: str
    handoff: bool = False
    handoff_run_id: str = ""

    def parity_reason(self) -> str:
        """What this run's parity says on a pass: every tool covered, and, given a hand-off, every tree taken from it
        (design §3.6)."""
        n = len(tools())
        return f"covered {n}; {n} trees taken from run {self.handoff_run_id}" if self.handoff else f"covered {n}"

    def tree_out(self, name: str) -> list[str] | None:
        """The lines of the tool's tree child output, `<run_dir>/parity/tree-<name>.out` (its realpath, its details
        digest, its tree digest), or None when no tree child ran for it."""
        path = os.path.join(self.run_dir, "parity", f"tree-{name}.out")
        if not os.path.exists(path):
            return None
        return read_text(path).split("\n")[:-1]

    def tail(self) -> list[str]:
        return self.stdout.split("\n")[:-1][-2:]

    def _json(self, *parts: str) -> object:
        with open(os.path.join(self.run_dir, *parts), "rb") as f:
            return json.loads(f.read())

    def record(self, job: str, row_id: str) -> dict:
        return self._json("records", job, f"{row_id}.json")

    def records(self) -> list[dict]:
        return self._json("run.json")["records"]

    def trace(self) -> list[tuple[str, str]]:
        """The scheduler's trace, as (event, row id) in order."""
        return [(entry["event"], entry["id"]) for entry in self._json("trace.json")]

    def started(self) -> list[str]:
        return [row_id for event, row_id in self.trace() if event == "start"]

    def row_stdout(self, job: str, row_id: str) -> str:
        return read_text(os.path.join(self.run_dir, "rows", job, row_id, "stdout"))

    def row_file(self, job: str, row_id: str, *parts: str) -> str:
        return read_text(os.path.join(self.run_dir, "rows", job, row_id, *parts))

    def summary(self, job: str, row_id: str) -> list[str]:
        """The summary's cells for one row."""
        for line in self.stdout.split("\n"):
            cells = [cell.strip() for cell in line.split(" | ", 4)]  # the reason is last and may hold " | "
            if cells[:2] == [job, row_id]:
                return cells
        raise AssertionError(f"no summary line for {job}/{row_id} in:\n{self.stdout}")


def _check_parity_reason(run: Run) -> None:
    """Every passing parity record of a recorded run says what Run.parity_reason says (design §3.6, QB-S2b): with a
    hand-off, every tree taken from it, so a reuse that vanished is red, not just slow."""
    if not os.path.exists(os.path.join(run.run_dir, "run.json")):
        return
    want = run.parity_reason()
    for record in run.records():
        if record["id"] == "parity" and record["verdict"] == "pass" and record["reason"] != want:
            raise AssertionError(f"parity passed with reason {record['reason']!r}, not {want!r}:\n{run.stdout}")


class World:
    """A temporary git repository holding real copies of the gate, and the places a run writes to."""

    def __init__(self, root: str) -> None:
        self.root = root
        self.repo = os.path.join(root, "repo")
        os.mkdir(self.repo)
        self.runs = 0
        env = {k: v for k, v in os.environ.items() if k not in HIDDEN and not k.startswith(("GATE_", "PYTHON"))}
        self.env = {**env, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1", **QUIET_GIT}
        self.git("init", "-q")

    def git(self, *args: str) -> str:
        out, err = os.path.join(self.root, "git.stdout"), os.path.join(self.root, "git.stderr")
        code = run_process(["git", *args], cwd=self.repo, env=self.env, out=out, err=err, cap_s=RUN_CAP_S)
        if code != 0:
            raise AssertionError(f"git {' '.join(args)}: exit {code}: {read_text(err)}")
        return read_text(out)

    def copy(self, rel: str, dest: str | None = None) -> str:
        """A real copy of one file of the repository the drivers live in."""
        target = os.path.join(self.repo, dest or rel)
        os.makedirs(os.path.dirname(target), exist_ok=True)
        shutil.copy(os.path.join(REPO, rel), target)
        return target

    def tracked(self, prefix: str) -> list[str]:
        """The files the drivers' own repository tracks under `prefix`; never none."""
        out, err = os.path.join(self.root, "ls.stdout"), os.path.join(self.root, "ls.stderr")
        code = run_process(["git", "ls-files", "-z", "--", prefix], cwd=REPO, env=self.env, out=out, err=err, cap_s=60)
        if code != 0:
            raise AssertionError(f"git ls-files in {REPO}: exit {code}: {read_text(err)}")
        found = sorted(p for p in read_text(out).split("\0") if p)
        if not found:
            raise AssertionError(f"git ls-files in {REPO} lists nothing under {prefix}")
        return found

    def copy_gate(self, *, tests: bool = False) -> None:
        """The tracked gate/ files, tools.json and ruff.toml. gate/proofs never; tests/ and testdata/ on request."""
        for rel in self.tracked("gate"):
            parts = rel.split("/")
            if parts[1] == "proofs" or (not tests and {"tests", "testdata"} & set(parts[:-1])):
                continue
            self.copy(rel)
        self.copy("tools.json")
        self.copy("ruff.toml")

    def copy_proofs_module(self, *packages: str) -> None:
        """gate/proofs's go.mod and the named planted packages, at this repository's root: the coverage rows then
        run exactly as registered, and their real default scope (./internal/...) sees the plants (QA Q16)."""
        self.copy("gate/proofs/go.mod", "go.mod")
        if self.tracked("gate/proofs/go.sum"):  # goleak's sums, for the leak proof (S3.2b-2)
            self.copy("gate/proofs/go.sum", "go.sum")
        for package in packages:
            prefix = f"gate/proofs/internal/{package}/"
            for rel in self.tracked(prefix):
                self.copy(rel, "internal/" + rel.removeprefix("gate/proofs/internal/"))

    def write(self, rel: str, text: str) -> str:
        path = os.path.join(self.repo, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def track(self, rel: str, text: str) -> str:
        path = self.write(rel, text)
        self.git("add", "--", rel)
        return path

    def commit(self) -> None:
        """Everything so far, as HEAD: the runner reads HEAD at T1 and above."""
        self.git("add", "-A")
        self.git(*GIT_IDENTITY, "commit", "-q", "-m", "the world")

    def registry(self, *rows: dict) -> str:
        path = os.path.join(self.root, "registry.json")
        with open(path, "w", encoding="utf-8") as f:
            json.dump({"schema": 1, "rows": list(rows)}, f, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
            f.write("\n")
        return path

    def link_cache(self, *names: str) -> str:
        """A tool cache in this world whose named tools are links to the real, read-only tool directories."""
        cache_dir = os.path.join(self.root, "cache")
        os.makedirs(cache_dir, exist_ok=True)
        for name in names:
            os.symlink(home(real_cache(), name), home(cache_dir, name))
        return cache_dir

    def skeleton(self, cache_dir: str, name: str, *, copies: frozenset[str], omit: frozenset[str]) -> str:
        """A real tool directory mirroring the real one: each path in `copies` is a real copy (asserted not to share
        an inode with the original), each path in `omit` is absent, each directory on the way to either is a real
        directory, and everything else is a link into the real, read-only tree."""
        real, dest = home(real_cache(), name), home(cache_dir, name)
        needed = {"/".join(p.split("/")[:i]) for p in copies | omit for i in range(1, p.count("/") + 1)}

        def fill(rel_dir: str) -> None:
            os.mkdir(os.path.join(dest, rel_dir))
            for entry in sorted(os.listdir(os.path.join(real, rel_dir))):
                rel = f"{rel_dir}/{entry}" if rel_dir else entry
                source, target = os.path.join(real, rel), os.path.join(dest, rel)
                if rel in omit:
                    continue
                if rel in copies:
                    shutil.copy(source, target)
                    if os.stat(target).st_ino == os.stat(source).st_ino:
                        raise AssertionError(f"{target} shares an inode with the real {source}")
                elif rel in needed:
                    fill(rel)
                else:
                    os.symlink(source, target)

        fill("")
        return dest

    def run(
        self,
        *,
        registry: str | None = None,
        cache_dir: str | None = None,
        path_first: str = "",
        tier: str = TIER,
        handoff: bool | str = True,
    ) -> Run:
        """The real runner, at T2 unless a test of a T4 row says otherwise, in a new run directory.

        `handoff` (design §3.6): True gives a full-tree run the outer run's hand-off when this driver runs inside a
        full-tree gate row (`handoff()`); False gives none (the parity proofs hash for real); a path is passed as
        given, at any tier. A run given a hand-off must take every tree from it. Whenever parity passes, its reason is
        checked here against `Run.parity_reason`.
        """
        given = _given(handoff, tier)
        run_id = handoff_run_id(given) if given is not None else ""
        self.runs += 1
        run_dir = os.path.join(self.root, f"run{self.runs}")
        argv = [pinned_python(), "-I", "-B", "-X", "utf8", os.path.join(self.repo, "gate", "run.py")]
        argv += ["--tier", tier, "--run-dir", run_dir, *(["--registry", registry] if registry else [])]
        argv += ["--parity-handoff", given] if given is not None else []
        env = {**self.env, "VORSAR_CEP_TOOLS_CACHE": cache_dir or real_cache()}
        if path_first:
            env["PATH"] = os.pathsep.join([path_first, env.get("PATH", "")])
        out = os.path.join(self.root, f"run{self.runs}.stdout")
        err = os.path.join(self.root, f"run{self.runs}.stderr")
        code = run_process(argv, cwd=self.repo, env=env, out=out, err=err, cap_s=RUN_CAP_S)
        run = Run(code=code, stdout=read_text(out), run_dir=run_dir, handoff=given is not None, handoff_run_id=run_id)
        _check_parity_reason(run)
        return run


class DriverTest(unittest.TestCase):
    """A ceiling and a world per test."""

    def setUp(self) -> None:
        faulthandler.dump_traceback_later(CEILING_S, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory(prefix="gate-proof-")
        self.addCleanup(tmp.cleanup)
        self.world = World(os.path.realpath(tmp.name))

    def assertGateFailed(self, run: Run) -> None:
        self.assertEqual(run.tail(), ["GATE FAILED", "exit: 1"], run.stdout)
        self.assertEqual(run.code, 1, run.stdout)

    def assertRow(self, run: Run, job: str, row_id: str, ok: bool, reason: str, *, label: str = "") -> None:
        """The record, and the summary's line (whose job column says `*` for parity in an all-jobs run)."""
        record = run.record(job, row_id)
        self.assertEqual((record["verdict"], record["reason"]), ("pass" if ok else "fail", reason), run.stdout)
        cells = run.summary(label or job, row_id)
        self.assertEqual((cells[2], cells[4]), ("PASS" if ok else "FAIL", reason), run.stdout)

    def assertParityPassed(self, run: Run, job: str) -> None:
        """The control every non-parity driver carries: the pinned tools were all there and all right, in the form
        the run's hand-off calls for (Run.parity_reason; World.run checks it on every run too)."""
        self.assertRow(run, job, "parity", True, run.parity_reason(), label="*")

    def assertRed(self, run: Run, job: str, row_id: str, reason: str) -> None:
        self.assertGateFailed(run)
        self.assertRow(run, job, row_id, False, reason)

    def assertRefusedAtLoad(self, run: Run, reason: str) -> None:
        """The runner refused before any row ran (S1.1c-2 design §4.2): exit 2, `reason` as the first line of its
        output, `GATE FAILED` and `exit: 2` as its last two, and no record of any row."""
        lines = run.stdout.split("\n")[:-1]
        self.assertEqual((run.code, lines[-2:]), (2, ["GATE FAILED", "exit: 2"]), run.stdout)
        self.assertEqual(lines[0], reason, run.stdout)
        self.assertFalse(os.path.exists(os.path.join(run.run_dir, "records")), f"a row ran:\n{run.stdout}")
        self.assertFalse(os.path.exists(os.path.join(run.run_dir, "run.json")), f"a run was recorded:\n{run.stdout}")

    def assertHookRefused(self, result: tuple[int, str], reason: str) -> None:
        """A git hook refused the commit or the push (S1.1c-2 design §4.2): git's exit 1, the hook's `GATE FAILED` and
        `exit: 1` as the output's last two lines, and `reason` either as one whole line (a hook's own refusal, before
        any row) or as the reason of exactly one FAIL row of the hook's gate run, with no other row failing."""
        code, out = result
        lines = out.split("\n")[:-1]
        self.assertEqual(code, 1, out)
        self.assertEqual(lines[-2:], ["GATE FAILED", "exit: 1"], out)
        if reason in lines:
            return
        rows = [cells for cells in ([c.strip() for c in line.split(" | ", 4)] for line in lines) if len(cells) == 5]
        failed = [cells[4] for cells in rows if cells[2] == "FAIL"]
        self.assertEqual(failed, [reason], out)

    def assertRegenRefused(self, result: tuple[int, str], reason: str) -> None:
        """gate/baseline/regen.py refused (S1.1c-2 design §4.2): exit 1, `reason` as its last `refused: ` line, and no
        line saying it wrote anything (`recorded <n> block(s)`, `no change…`)."""
        code, out = result
        lines = out.split("\n")
        self.assertEqual(code, 1, out)
        refused = [line for line in lines if line.startswith("refused: ")]
        self.assertTrue(refused, out)
        self.assertEqual(refused[-1], reason, out)
        self.assertEqual([line for line in lines if line.startswith(("recorded ", "no change"))], [], out)


class ParityDriver(DriverTest):
    """A parity proof: one planted defect, exactly one reason, and nothing but parity ever starts."""

    def assertParityStops(self, run: Run, reason: str, *, planted: str) -> None:
        """Parity alone ran and failed with `reason`; the witness that it hashed for real (design §3.6, CA-B1, QB-B1):
        every tool but the `planted` one has its tree child's output, whose digest line is that tool's pin."""
        self.assertFalse(run.handoff, "a parity proof hashes for real: it is never given a hand-off")
        self.assertIn(planted, [t.name for t in tools()])
        self.assertGateFailed(run)
        records = run.records()
        parity = [r for r in records if r["id"] == "parity"]
        others = [r for r in records if r["id"] != "parity"]
        self.assertTrue(
            parity and others, "a registry with rows besides parity is the witness that parity stopped them"
        )
        self.assertEqual({(r["verdict"], r["reason"]) for r in parity}, {("fail", reason)})
        self.assertEqual(
            {(r["id"], r["reason"]) for r in others}, {(r["id"], "not run: parity failed") for r in others}
        )
        self.assertEqual(run.started(), ["parity"])
        cells = run.summary("*", "parity")
        self.assertEqual((cells[2], cells[4]), ("FAIL", reason), run.stdout)
        for t in tools():
            if t.name != planted:
                lines = run.tree_out(t.name)
                self.assertIsNotNone(lines, f"{t.name}'s tree was never hashed: no parity/tree-{t.name}.out")
                self.assertEqual(lines[2:3], [t.tree_sha256], f"{t.name}'s tree child output: {lines}")


def copy_hooks(world: World) -> list[str]:
    """The tracked .githooks/ launchers, gate/hooks/ (no tests) and the Makefile, byte for byte with their modes, so
    the hook proofs run the real hooks (QA Q2). Also what the registry's T0 rows run: go.mod and tools/archcheck with
    its rules (S1.1b added archcheck at T0; without them its row fails in the world), and tools/allowlists/, which
    archcheck reads from S1.1c-2 on (design §3, DQ-S5). Gives the copied paths."""
    copied = []
    t0 = [
        *world.tracked("tools/archcheck"),
        "tools/importcheck/rules.json",
        "go.mod",
        *world.tracked("tools/allowlists"),
    ]
    for rel in [*world.tracked(".githooks"), *world.tracked("gate/hooks"), "Makefile", ".gitignore", *t0]:
        if "tests" in rel.split("/")[:-1] or rel.endswith("_test.go"):
            continue
        copied.append(world.copy(rel))
        with open(os.path.join(REPO, rel), "rb") as a, open(copied[-1], "rb") as b:
            if a.read() != b.read():
                raise AssertionError(f"{rel} is not a byte-for-byte copy")
    make_purity_targets(world)
    return copied


def make_purity_targets(world: World) -> None:
    """An empty directory for each package tools/allowlists/purity.json names, so archcheck's stale-target check sees
    the lists as live in a world that holds only some packages (S1.1c-2 lane C's open item 1; design §3). Stale reads
    the file system, so an empty directory is enough, and it adds no file to any row's scope."""
    with open(os.path.join(world.repo, "tools", "allowlists", "purity.json"), encoding="utf-8") as f:
        targets = [entry["target"] for entry in json.load(f)["entries"]]
    for target in targets:
        os.makedirs(os.path.join(world.repo, target.split(": ", 1)[0]), exist_ok=True)


def isolated_git_env(world: World) -> dict[str, str]:
    """The world's environment with no inherited GIT_* variable, its own HOME, no global or system git config, and
    the real tool cache named explicitly (HOME no longer leads to it)."""
    home_dir = os.path.join(world.root, "home")
    os.makedirs(home_dir, exist_ok=True)
    env = {k: v for k, v in world.env.items() if not k.startswith("GIT_")}
    env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1", HOME=home_dir, **QUIET_GIT)
    env["VORSAR_CEP_TOOLS_CACHE"] = real_cache()
    return env


def hooks_on(world: World) -> None:
    """Turn the world's hooks on through its real `make setup`; plain git config only when there is no make."""
    if shutil.which("make", path=world.env.get("PATH")) is None:
        world.git("config", "core.hooksPath", ".githooks")
        return
    out, err = os.path.join(world.root, "make.stdout"), os.path.join(world.root, "make.stderr")
    code = run_process(["make", "setup"], cwd=world.repo, env=world.env, out=out, err=err, cap_s=RUN_CAP_S)
    if code != 0:
        raise AssertionError(f"make setup: exit {code}: {read_text(out)}{read_text(err)}")
