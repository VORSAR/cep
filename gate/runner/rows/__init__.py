"""What the row wrappers share (design note §5; plan check S3).

A wrapper's children stay in the wrapper's process group, so the runner's watchdog reaches them: a new session here
would let a child outlive its row. A wrapper turns the runner's SIGTERM into Terminated (gate/term.py), so it waits
for the children the TERM also reached, which run their own cleanups, and starts no other (review BA1). Their
output goes to files in GATE_ROW_DIR, and stdin is /dev/null. Every wrapper prints exactly one GATE_COVERED line,
counted from the tool's own work and never from its input list.
"""

import concurrent.futures
import contextlib
import os
import signal
import subprocess
import sys
from collections.abc import Sequence
from datetime import UTC, date, datetime

from gate import term
from gate.runner import workers

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
SHIM = os.path.join(REPO, "gate", "runner", "pyshim.py")
SHIM_FLAGS = ("-I", "-B", "-X", "dev", "-W", "error")


def today() -> date:
    """The run's day: the UTC date, whatever the local zone (S1.1c-3 design §3.4; the inline copies move here when
    their files next change, BA9)."""
    return datetime.now(UTC).date()


def start() -> str:
    """Every wrapper's first line. An ignored SIGCHLD survives exec, and it turns a failed tool into exit 0."""
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    term.install()
    return os.environ["GATE_ROW_DIR"]


def read_list(path: str) -> list[bytes]:
    """files.nul: each entry ends in a NUL."""
    with open(path, "rb") as f:
        return [item for item in f.read().split(b"\0") if item]


def show(path: bytes) -> str:
    return path.decode("utf-8", errors="replace")


def run_tool(argv: Sequence[str | bytes], stdout: str, stderr: str) -> int:
    with open(stdout, "wb") as out, open(stderr, "wb") as err:
        return subprocess.run(list(argv), stdin=subprocess.DEVNULL, stdout=out, stderr=err).returncode


def read_text(path: str) -> str:
    with open(path, "rb") as f:
        return f.read().decode("utf-8", errors="replace")


def covered(count: int) -> None:
    print(f"GATE_COVERED {count}", flush=True)


def _module_name(path: str) -> str:
    return path.removesuffix(".py").replace("/", ".")


INIT = "__init__.py"


def _imported_as(path: str) -> str:
    """The name `import` finds this file by: a package's __init__.py names its directory."""
    directory, _, name = path.rpartition("/")
    return _module_name(directory) if name == INIT else _module_name(path)


def collisions(paths: Sequence[str]) -> list[str]:
    """QB10: every way a listing of Python files under gate/ (tracked or not) would run one module in another's place —
    two paths with one module name, a dotted directory, a regular package beside a same-named module — one line each
    naming the paths, sorted; [] when there is none (S1.1c-3 design §3.5).

    A regular package is a directory whose __init__.py is in the listing; a namespace directory beside a same-named
    module is not a collision, because the import system takes the module first.
    """
    found: set[str] = set()
    by_name: dict[str, list[str]] = {}
    for path in sorted(set(paths)):
        parts = path.split("/")[:-1]
        dotted = next((i for i, part in enumerate(parts) if "." in part), None)
        if dotted is not None:
            found.add(f"{'/'.join(parts[: dotted + 1])}: a dotted directory under gate/ can never be imported")
        by_name.setdefault(_imported_as(path), []).append(path)
    for name, named in by_name.items():
        if len(named) < 2:
            continue
        package = name.replace(".", "/")
        if sorted(named) == [f"{package}.py", f"{package}/{INIT}"]:
            found.add(f"{package}/ shadows {package}.py")
        else:
            found.add(f"{', '.join(named[:-1])} and {named[-1]} share the module name {name}")
    return sorted(found)


def _report_lines(path: str) -> list[str]:
    try:
        return read_text(path).splitlines()
    except FileNotFoundError:
        return []


def _run_module(python: str, path: str, out_dir: str) -> str:
    """Run one test module in its own process through the shim; give the problem, or "" if it passed.

    A pass needs the shim's own report as well as exit 0: a module can end the process with exit 0 before any test.
    The report is read only from the file the shim is given (`<name>.report`, removed first), never from the
    module's stdout, which the module can write to (S1.1c-2 design §4.3, BA13).
    """
    name = _module_name(path)
    out, report = os.path.join(out_dir, f"{name}.stdout"), os.path.join(out_dir, f"{name}.report")
    with contextlib.suppress(FileNotFoundError):
        os.remove(report)
    argv = [python, *SHIM_FLAGS, SHIM, os.getcwd(), name, report]
    code = run_tool(argv, out, os.path.join(out_dir, f"{name}.stderr"))
    said = [line for line in _report_lines(report) if line.startswith(f"pyshim: {name}: ")]
    if code == 0 and said and said[-1].startswith(f"pyshim: {name}: passed "):
        return ""
    return said[-1] if said else f"exit {code}, and the shim made no report"


def run_modules(python: str, paths: Sequence[str], row_dir: str) -> tuple[int, list[str]]:
    """Run each module in its own process: (modules that passed, one line per module that did not).

    A listing in which one module would run in another's place is refused before any module runs (QB10).
    """
    refused = collisions(paths)
    if refused:
        return 0, refused
    out_dir = os.path.join(row_dir, "py")
    os.makedirs(out_dir, exist_ok=True)
    pool = concurrent.futures.ThreadPoolExecutor(max_workers=workers.parse(os.environ))
    try:
        problems = list(pool.map(lambda p: _run_module(python, p, out_dir), paths))
    finally:
        pool.shutdown(wait=True, cancel_futures=True)  # stopped: the running modules finish, no other starts
    failed = [f"{path}: {problem}" for path, problem in zip(paths, problems, strict=True) if problem]
    return len(paths) - len(failed), sorted(failed)


def finish(failures: Sequence[str]) -> int:
    for line in failures:
        print(f"failed: {line}")
    sys.stdout.flush()
    return 1 if failures else 0
