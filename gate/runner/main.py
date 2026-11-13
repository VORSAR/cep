"""The gate's command line (plan check S4; design note §4).

    gate/run.py --tier T0..T4 [--job NAME] [--run-dir DIR] [--registry PATH] [--commit SHA] [--run-id ID]
    gate/run.py summarize ...      (gate/runner/summarize.py)
    gate/run.py generate-ci        (gate/ci/generate.py)
    gate/run.py fetch-vulndb       (gate/toolchain/vulndb_fetch.py: make vulndb)
    gate/run.py canary-feeds --out DIR/feeds.json
    gate/run.py canary-candidate --feeds FILE --out DIR/
    gate/run.py canary-summary --artifacts DIR --candidate DIR --attempt N [--summary FILE]
                                   (the weekly canary's steps, S1.1c-4 design §4.7: thin calls into gate/canary)

Exit 0: pass. Exit 1: the gate failed, or was interrupted. Exit 2: usage or internal error. The last two stdout
lines are always `GATE PASSED|GATE FAILED` and `exit: <n>`: main catches everything, because an uncaught exception
would exit 1, which means "gate failed".
"""

import argparse
import copy
import json
import os
import re
import signal
import sys
import tempfile
from collections.abc import Callable, Mapping
from dataclasses import dataclass

from gate.runner import clock, env, registry, scheduler, summary, workers
from gate.runner.clock import Clock, Timers
from gate.runner.proc import Group
from gate.toolchain import manifest, parity

DEFAULT_REGISTRY = os.path.join("gate", "registry.json")
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


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

    def parity(self, env: Mapping[str, str], *, full_tree: bool, spawn: scheduler.Spawn, scratch: str) -> list[str]:
        return parity.check(self._tools, self._cache, env, full_tree=full_tree, spawn=spawn, scratch=scratch)


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
    return Deps(repo=REPO, environ=os.environ, clock=clock.REAL, timers=clock.RealTimers(), toolchain=RealToolchain)


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise UsageError(f"{message}\n{self.format_usage().strip()}")


def _parse(argv: list[str]) -> argparse.Namespace:
    parser = _Parser(prog="gate/run.py", add_help=False, allow_abbrev=False)
    parser.add_argument("--tier", required=True, choices=registry.TIERS)
    parser.add_argument("--job")
    parser.add_argument("--run-dir", dest="run_dir")
    parser.add_argument("--registry")
    parser.add_argument("--commit", type=_matching(_SHA, "--commit", "40 or 64 lowercase hex digits"))
    parser.add_argument("--run-id", dest="run_id", type=_matching(_RUN_ID, "--run-id", "1-64 of [A-Za-z0-9._-]"))
    return parser.parse_args(argv)


_SHA = re.compile(r"[0-9a-f]{40}|[0-9a-f]{64}")
_RUN_ID = re.compile(r"[A-Za-z0-9._-]{1,64}")


def _matching(pattern: re.Pattern[str], option: str, want: str):
    """A value checker for one option (gate design §12.3); anything else is a usage error, exit 2."""

    def check(value: str) -> str:
        if not pattern.fullmatch(value) or value in (".", ".."):
            raise UsageError(f"{option} must be {want}, not {value!r}")
        return value

    return check


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
    default = os.path.join(deps.repo, DEFAULT_REGISTRY)
    path = args.registry or default
    loaded, registry_sha256 = registry.load_with_digest(path)
    rows = registry.select(loaded, args.tier, args.job)
    # review BA8: a run of any other law says so, so its "exit: 0" cannot pass for the repository's.
    shown = None
    if os.path.realpath(path) != os.path.realpath(default):
        shown = summary.shown(os.path.abspath(path), deps.repo)
    toolchain = deps.toolchain(deps.repo, deps.environ)
    run_id = args.run_id or deps.new_run_id(deps.clock.wall_ms())
    run_dir = _prepare_run_dir(args.run_dir, deps.repo, run_id)
    clean = env.clean_env(deps.environ, toolchain.path_dirs())
    runner = scheduler.Runner(
        rows=rows,
        tier=args.tier,
        job=args.job,
        workers=count,
        run_dir=run_dir,
        run_id=run_id,
        commit=args.commit or deps.commit(deps.repo, clean, run_dir, args.tier),
        repo=deps.repo,
        environ=deps.environ,
        toolchain=toolchain,
        registry_sha256=registry_sha256,
        registry_shown=shown,
        clock=deps.clock,
        timers=deps.timers,
        record_go_env=deps.record_go_env,
        install_signals=True,
    )
    result = runner.run()
    sys.stdout.write(result.summary)
    sys.stdout.flush()
    return result.exit_code


# The weekly canary's commands (S1.1c-4 design §4.7). Each is a thin call into gate/canary; a CanaryError is written
# to <dir>/error.txt, which the candidate job uploads whatever happened and the summary names (review DQ-S9), and
# exits 2. The summary exits as gate/canary/summary.py says: 0 with no breaks, 1 with any.
ERROR_TXT = "error.txt"
PIN_TESTS = os.path.join("gate", "canary", "pin_tests.json")
_ATTEMPT = re.compile(r"[0-9]{1,9}")
# unittest's report of one failing test: `FAIL: <name> (<id>)`, a subtest's parameters after it (S1.1c-4 design §4.5).
_FAILURE = re.compile(r"(?:FAIL|ERROR): \S+ \(([^()\s]+)\)(?: \(.*\))?")
_RULE = "=" * 70
_THIN_RULE = "-" * 70
_EXCEPTION = re.compile(r"[A-Za-z_][\w.]*: .*|[A-Za-z_][\w.]*")


def _canary_args(prog: str, argv: list[str], *required: str, optional: tuple[str, ...] = ()) -> argparse.Namespace:
    parser = _Parser(prog=f"gate/run.py {prog}", add_help=False, allow_abbrev=False)
    for option in required:
        parser.add_argument(option, required=True)
    for option in optional:
        parser.add_argument(option)
    return parser.parse_args(argv)


def _write_atomic(path: str, text: str) -> None:
    fd, tmp = tempfile.mkstemp(prefix=f".{os.path.basename(path)}.", dir=os.path.dirname(path))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            f.write(text)
        os.rename(tmp, path)
    except BaseException:
        os.unlink(tmp)
        raise


def _json_text(doc: object) -> str:
    return json.dumps(doc, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n"


def _read(path: str) -> str:
    with open(path, "rb") as f:
        return f.read().decode("utf-8", errors="replace")


def _load_json(path: str, what: str) -> object:
    from gate.canary.candidate import CanaryError

    try:
        return json.loads(_read(path))
    except (OSError, ValueError) as e:
        raise CanaryError(f"{what}: {path}: {e}") from e


def _ended(code: int, *lines: str) -> int:
    for line in lines:
        print(line)
    print("GATE PASSED" if code == 0 else "GATE FAILED")
    print(f"exit: {code}")
    sys.stdout.flush()
    return code


def _canary_error(directory: str, error: Exception) -> int:
    """The step's CanaryError, kept as <directory>/error.txt for the summary's first line, and exit 2."""
    os.makedirs(directory, exist_ok=True)
    _write_atomic(os.path.join(directory, ERROR_TXT), f"{error}\n")
    return _ended(2, str(error))


def canary_feeds(argv: list[str], *, repo: str, environ: Mapping[str, str], get: Callable) -> int:
    """`canary-feeds --out DIR/feeds.json`: every pin's feed, fetched with the read-only token (review A-S5)."""
    from gate.canary import feeds
    from gate.canary.candidate import CanaryError

    out = os.path.abspath(_canary_args("canary-feeds", argv, "--out").out)
    try:
        token = environ.get("GITHUB_TOKEN", "")
        if not token:
            raise CanaryError("canary-feeds: GITHUB_TOKEN is not set: GitHub's release feeds are read with it")
        fetched = feeds.fetch(_load_json(os.path.join(repo, "tools.json"), "tools.json"), token=token, get=get)
    except CanaryError as e:
        return _canary_error(os.path.dirname(out), e)
    os.makedirs(os.path.dirname(out), exist_ok=True)
    _write_atomic(out, _json_text(fetched))
    return _ended(0, f"wrote {out}")


def _version_line(tool: Mapping, archive: str, strip: str, platform: str, scratch: str) -> str:
    """The upgraded executable's own answer to its version_argv, run from a tree extracted once more under the
    installer's rules (S1.1c-4 design §4.3, §4.7); go's `{goos}/{goarch}` template is kept as tools.json holds it."""
    from gate.canary.candidate import CanaryError
    from gate.runner.proc import Group
    from gate.toolchain import install

    name, argv = str(tool["name"]), [str(a) for a in tool["version_argv"]]
    where = f"{name} {platform}"
    tree = tempfile.mkdtemp(prefix=f"{name}.", dir=scratch)
    try:
        try:
            install._seal(tree, install._extract(archive, strip, tree))
        except (install._Rejected, OSError) as e:
            raise CanaryError(f"{where}: {archive}: {e}") from e
        out, err = os.path.join(scratch, f"{name}.version.stdout"), os.path.join(scratch, f"{name}.version.stderr")
        env = {"PATH": os.defpath, "HOME": scratch, "GOTOOLCHAIN": "local", "LC_ALL": "C"}
        group = Group()
        try:
            proc = group.spawn([os.path.join(tree, argv[0]), *argv[1:]], cwd=scratch, env=env, stdout=out, stderr=err)
        except OSError as e:
            raise CanaryError(f"{where}: could not run {argv[0]}: {e.strerror}") from e
        assert proc is not None  # a fresh group has not been signalled
        code, _ = group.reap(proc)
        lines = _read(out).splitlines()
        if code != 0 or not lines:
            said = next((line for line in _read(err).splitlines() if line.strip()), "no output")
            raise CanaryError(f"{where}: {' '.join(argv)} exit {code}: {said}")
    finally:
        install._restore_write(tree)
    goos, goarch = platform.split("-", 1)
    if "{goos}/{goarch}" in str(tool["version_line"]):
        return lines[0].replace(f"{goos}/{goarch}", "{goos}/{goarch}")
    return lines[0]


def canary_candidate(
    argv: list[str], *, repo: str, archive: Callable[[str, str], str] | None = None, version_line=_version_line
) -> int:
    """`canary-candidate --feeds FILE --out DIR/`: the candidate tools.json (each upgraded executable's version_line
    from its own answer on this platform, Linux on the workflow) and upgrades.json; no token here (review DA-S7)."""
    from gate.canary import candidate, feeds
    from gate.canary.candidate import CanaryError

    args = _canary_args("canary-candidate", argv, "--feeds", "--out")
    out = os.path.abspath(args.out)
    platform = manifest.platform_key()
    with tempfile.TemporaryDirectory(prefix="canary-") as scratch:
        try:
            pinned = _load_json(os.path.join(repo, "tools.json"), "tools.json")
            fetch = archive or feeds.downloader(scratch)
            paths: dict[str, str] = {}

            def kept(url: str, sha256: str) -> str:
                paths[url] = fetch(url, sha256)
                return paths[url]

            built = candidate.build(pinned, _load_json(args.feeds, "feeds"), kept)
            tools = copy.deepcopy(dict(built.tools))
            upgraded = {u.name for u in built.upgrades}
            for tool in tools["tools"]:
                if tool["name"] not in upgraded or "version_argv" not in tool:
                    continue  # a package's version is its version_line (review DQ-S7)
                entry = tool["platforms"].get(platform)
                if entry is None:
                    raise CanaryError(f"{tool['name']}: no {platform} archive to run its version_argv")
                tool["version_line"] = version_line(tool, paths[entry["url"]], entry["strip"], platform, scratch)
            text = json.dumps(tools, indent=2, sort_keys=True, ensure_ascii=False) + "\n"  # tools.json's own form
            check = os.path.join(scratch, "tools.json")
            _write_atomic(check, text)
            for key in sorted(tools["tools"][0]["platforms"]) if tools["tools"] else ():
                manifest.load(check, key)
        except manifest.ManifestError as e:
            return _canary_error(out, CanaryError(f"canary: the candidate tools.json does not load: {e}"))
        except CanaryError as e:
            return _canary_error(out, e)
    os.makedirs(out, exist_ok=True)
    _write_atomic(os.path.join(out, "tools.json"), text)
    upgrades = [
        {"latest": u.latest, "name": u.name, "note": u.note, "pinned": u.pinned} for u in built.upgrades
    ]  # tools.json's order, as the summary names them
    _write_atomic(os.path.join(out, "upgrades.json"), _json_text({"newer_lines": list(built.newer_lines),
                                                                  "upgrades": upgrades}))  # fmt: skip
    return _ended(0, f"wrote {os.path.join(out, 'tools.json')}", f"wrote {os.path.join(out, 'upgrades.json')}")


def _failure_blocks(text: str) -> list[list[str]]:
    """unittest's FAIL and ERROR blocks in one module's stderr: [test id, the exception's line], the subtest's
    parameters dropped from the id (pin_tests.json's ids carry none)."""
    lines = text.splitlines()
    found = []
    for i, line in enumerate(lines):
        header = _FAILURE.fullmatch(lines[i + 1]) if line == _RULE and i + 1 < len(lines) else None
        if header is None:
            continue
        body = []
        for later in lines[i + 3 :]:
            if later in (_RULE, _THIN_RULE):
                break
            body.append(later)
        said = [b for b in body if b and not b[0].isspace() and b != "Traceback (most recent call last):"]
        exception = next((b for b in said if _EXCEPTION.fullmatch(b)), said[0] if said else "")
        found.append([header.group(1), exception])
    return found


def _failures(row_dir: str, row: str) -> list[list[str]]:
    """A record's failures for the sort: per `failed:` line of the row's stdout, the failing tests its module's
    stderr reports, or the line itself named by the row (a module that failed with no test failing, or a non-Python
    row's line)."""
    try:
        stdout = _read(os.path.join(row_dir, "stdout"))
    except FileNotFoundError:
        return []
    out: list[list[str]] = []
    for line in stdout.splitlines():
        if not line.startswith("failed: "):
            continue
        text = line.removeprefix("failed: ")
        path = text.split(": ", 1)[0]
        blocks: list[list[str]] = []
        if path.endswith(".py"):
            stderr = os.path.join(row_dir, "py", path.removesuffix(".py").replace("/", ".") + ".stderr")
            if os.path.isfile(stderr):
                blocks = _failure_blocks(_read(stderr))
        out += blocks or [[row, text]]
    return out


def _job_records(artifacts: str, job: str, attempt: str) -> list[dict]:
    """One T4 job's records from its artifact, each with its failures; [] when the artifact is missing."""
    from gate.canary.candidate import CanaryError

    base = os.path.join(artifacts, f"records-{job}-{attempt}")
    directory = os.path.join(base, "records", job)
    if not os.path.isdir(directory):
        return []
    records = []
    for name in sorted(os.listdir(directory)):
        if not name.endswith(".json"):
            continue
        record = _load_json(os.path.join(directory, name), f"job {job}'s record")
        if not isinstance(record, dict) or not isinstance(record.get("id"), str):
            raise CanaryError(f"job {job}'s record {name} is not a record")
        failing = record.get("verdict") != "pass"
        record["failures"] = _failures(os.path.join(base, "rows", job, record["id"]), record["id"]) if failing else []
        records.append(record)
    return records


def canary_summary(argv: list[str], *, repo: str, environ: Mapping[str, str]) -> int:
    """`canary-summary`: the candidate's files, every T4 job's records and `toJSON(needs)` (env NEEDS) through the
    sort and the summary; the Markdown printed and appended to --summary ($GITHUB_STEP_SUMMARY)."""
    from gate.canary import candidate, sort, summary
    from gate.canary.candidate import CanaryError

    args = _canary_args("canary-summary", argv, "--artifacts", "--candidate", "--attempt", optional=("--summary",))
    if not _ATTEMPT.fullmatch(args.attempt):
        raise UsageError(f"--attempt must be a run attempt number, not {args.attempt!r}")
    try:
        needs = json.loads(environ.get("NEEDS", ""))
        if not isinstance(needs, dict):
            raise ValueError("not an object")
    except ValueError as e:
        return _ended(2, f"canary-summary: NEEDS is not toJSON(needs): {e}")
    try:
        error = _read(os.path.join(args.candidate, ERROR_TXT)) if os.path.isfile(
            os.path.join(args.candidate, ERROR_TXT)) else None  # fmt: skip
        built = None
        tools_path, upgrades_path = (os.path.join(args.candidate, f) for f in ("tools.json", "upgrades.json"))
        if os.path.isfile(tools_path) and os.path.isfile(upgrades_path):
            listed = _load_json(upgrades_path, "upgrades.json")
            built = candidate.Candidate(
                _load_json(tools_path, "the candidate tools.json"),
                tuple(candidate.Upgrade(u["name"], u["pinned"], u["latest"], u["note"]) for u in listed["upgrades"]),
                tuple(listed["newer_lines"]),
            )
        structural = []
        if built is not None:
            pinned = _load_json(os.path.join(repo, "tools.json"), "tools.json")
            structural = candidate.structural_faults(pinned, built.tools, frozenset(u.name for u in built.upgrades))
        records = {job: _job_records(args.artifacts, job, args.attempt) for job in sorted(needs)}
        records = {job: got for job, got in records.items() if got and job != summary.CANDIDATE_JOB}
        text = _read(os.path.join(repo, PIN_TESTS))
        found = sort.sort(
            [r for job in sorted(records) for r in records[job]],
            sort.load_pin_tests(text),
            structural,
            unmeasured=sort.unmeasured(text),
        )
        markdown, code = summary.render(
            candidate=built, error=error, needs=needs, records=records, pins=found.pins, breaks=found.breaks
        )
    except (CanaryError, KeyError, TypeError) as e:
        return _ended(2, f"canary-summary: {type(e).__name__}: {e}")
    if args.summary:
        with open(args.summary, "a", encoding="utf-8") as f:  # GitHub's file: appended to, never replaced
            f.write(markdown)
    sys.stdout.write(markdown)
    return _ended(code)


def main(argv: list[str], *, deps: Deps | None = None) -> int:
    code = 2
    try:
        if argv[:1] == ["summarize"]:
            from gate.runner import summarize

            return summarize.main(argv[1:])
        if argv[:1] == ["generate-ci"]:
            from gate.ci import generate

            return generate.main(argv[1:])
        if argv[:1] == ["fetch-vulndb"]:
            from gate.toolchain import vulndb_fetch

            return vulndb_fetch.main(argv[1:], os.environ)
        if argv[:1] == ["canary-feeds"]:
            from gate.canary import feeds

            return canary_feeds(argv[1:], repo=REPO, environ=os.environ, get=feeds.http_get)
        if argv[:1] == ["canary-candidate"]:
            return canary_candidate(argv[1:], repo=REPO)
        if argv[:1] == ["canary-summary"]:
            return canary_summary(argv[1:], repo=REPO, environ=os.environ)
        return _run(argv, deps if deps is not None else default_deps())
    except (UsageError, registry.RegistryError, workers.WorkersError, manifest.ManifestError) as e:
        print(e)
    except KeyboardInterrupt:
        # break-it QB8: a Ctrl-C outside the scheduler's own handlers is the user's interrupt, never a gate bug.
        print(scheduler.INTERRUPTED)
        code = 1
    except BaseException as e:
        print(f"internal error: {type(e).__name__}: {e}")
    print("GATE FAILED")
    print(f"exit: {code}")
    sys.stdout.flush()
    return code
