"""Run one test module; everything that would pass vacuously fails (design note §5; QA DQ17).

    python -I -B -X dev -W error gate/runner/pyshim.py <root> <dotted.module> <report-file>

Fails a skip, an expected failure, an unexpected success, zero tests, an import error, and anything that reaches
sys.unraisablehook (under -W error, a leaked file or process raises in its destructor, and unittest alone reports
OK). A SIGTERM (the runner stopping the row) raises Terminated inside the running test, so its cleanups run and
end any child it started in a session of its own, and no later test starts (review BA1; break-it QB4). Standalone
and held to Python 3.12: `pyfloor` runs it under the pinned 3.12.16.

The shim opens the report file before it imports the module, writes its one final line there, closes it, and still
prints the line; the wrapper reads only the file, so a module that prints the pass line and leaves with os._exit(0)
is red (S1.1c-2 design §4.3, BA13). A module that writes the file itself, from sys.argv[3], is the residue
ADR-0015 §3.6 names: every test module is a law path, so review sees it as a LAW Δ line.
"""

import gc
import signal
import sys
import traceback
import unittest

USAGE = "usage: pyshim.py <root> <dotted.module> <report-file>"


class Terminated(BaseException):
    """The first SIGTERM, raised where the module is. Not an Exception, so a module's `except Exception` keeps it."""


class _Result(unittest.TextTestResult):
    """Remembers the run's result, so the TERM handler can stop the run between tests."""

    current: "unittest.TestResult | None" = None

    def __init__(self, *args: object, **kwargs: object) -> None:
        super().__init__(*args, **kwargs)
        _Result.current = self


class ImportFailed(Exception):
    """The module raised at import something the loader does not catch."""


def _on_term(signum: int, frame: object) -> None:
    signal.signal(signal.SIGTERM, lambda s, f: None)  # a cleanup in progress is never cut short by a second TERM
    if _Result.current is not None:
        _Result.current.stop()
    raise Terminated


def main(argv: list[str]) -> int:
    if len(argv) != 4:
        print(USAGE, file=sys.stderr, flush=True)
        return 2
    root, name, report = argv[1], argv[2], argv[3]
    with open(report, "w", encoding="utf-8") as f:  # before the import: a module ending the process leaves it empty
        try:
            line, code = run(root, name)
        except Terminated:  # outside any test: at import, or between tests
            line, code = f"pyshim: {name}: stopped by SIGTERM", 1
        except ImportFailed:
            traceback.print_exc()  # the module's own exception, as its cause
            line, code = f"pyshim: {name}: import error", 1
        f.write(line + "\n")
    print(line, flush=True)
    return code


def run(root: str, name: str) -> "tuple[str, int]":
    """The module's run: its one final line and the exit code."""
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)  # an ignored SIGCHLD survives exec, and reports exit 1 as exit 0
    signal.signal(signal.SIGTERM, _on_term)
    sys.path.insert(0, root)
    leaks: list[str] = []

    def on_unraisable(info: object) -> None:
        leaks.append(f"{info.exc_type.__name__}: {info.exc_value}")

    sys.unraisablehook = on_unraisable
    loader = unittest.TestLoader()
    try:
        suite = loader.loadTestsFromName(name)
    except SystemExit:  # re-raising would let `sys.exit(0)` at import pass as exit 0
        return f"pyshim: {name}: import error", 1
    except Exception as e:  # the loader only catches ImportError
        raise ImportFailed(name) from e
    result = unittest.TextTestRunner(stream=sys.stderr, verbosity=2, resultclass=_Result).run(suite)
    gc.collect()

    problems: list[str] = []
    if result.shouldStop:
        problems.append("stopped by SIGTERM")
    if loader.errors:
        problems.append("import error")
    elif result.testsRun == 0:
        problems.append("ran 0 tests")
    if result.failures or result.errors:
        problems.append(f"failed: {len(result.failures)} failures, {len(result.errors)} errors")
    if result.skipped:
        problems.append(f"skipped: {len(result.skipped)}")
    if result.expectedFailures:
        problems.append(f"expected failure: {len(result.expectedFailures)}")
    if result.unexpectedSuccesses:
        problems.append(f"unexpected success: {len(result.unexpectedSuccesses)}")
    if leaks:
        problems.append(f"leak: {'; '.join(sorted(leaks))}")
    if problems:
        return f"pyshim: {name}: {'; '.join(problems)}", 1
    return f"pyshim: {name}: passed {result.testsRun} tests", 0  # the wrapper requires this line, in the report file


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv))
    except Terminated:  # after the report was written: the line is printed only
        print(f"pyshim: {sys.argv[2]}: stopped by SIGTERM", flush=True)
        sys.exit(1)
