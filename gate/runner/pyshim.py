"""Run one test module; everything that would pass vacuously fails (design note §5; QA DQ17).

    python -I -B -X dev -W error gate/runner/pyshim.py <root> <dotted.module>

Fails a skip, an expected failure, an unexpected success, zero tests, an import error, and anything that reaches
sys.unraisablehook (under -W error, a leaked file or process raises in its destructor, and unittest alone reports
OK). A SIGTERM (the runner stopping the row) raises Terminated inside the running test, so its cleanups run and
end any child it started in a session of its own, and no later test starts (review BA1; break-it QB4). Standalone
and held to Python 3.12: `pyfloor` runs it under the pinned 3.12.16.
"""

import gc
import signal
import sys
import unittest


class Terminated(BaseException):
    """The first SIGTERM, raised where the module is. Not an Exception, so a module's `except Exception` keeps it."""


class _Result(unittest.TextTestResult):
    """Remembers the run's result, so the TERM handler can stop the run between tests."""

    current: "unittest.TestResult | None" = None

    def __init__(self, *args: object, **kwargs: object) -> None:
        super().__init__(*args, **kwargs)
        _Result.current = self


def _on_term(signum: int, frame: object) -> None:
    signal.signal(signal.SIGTERM, lambda s, f: None)  # a cleanup in progress is never cut short by a second TERM
    if _Result.current is not None:
        _Result.current.stop()
    raise Terminated


def main(argv: list[str]) -> int:
    root, name = argv[1], argv[2]
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
        print(f"pyshim: {name}: import error", flush=True)
        return 1
    except Exception:  # the loader only catches ImportError
        print(f"pyshim: {name}: import error", flush=True)
        raise
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
        print(f"pyshim: {name}: {'; '.join(problems)}", flush=True)
        return 1
    print(f"pyshim: {name}: passed {result.testsRun} tests", flush=True)  # the wrapper requires this line
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv))
    except Terminated:  # outside any test: at import, or between tests
        print(f"pyshim: {sys.argv[2]}: stopped by SIGTERM", flush=True)
        sys.exit(1)
