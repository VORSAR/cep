"""Run one test module; everything that would pass vacuously fails (design note §5; QA DQ17).

    python -I -B -X dev -W error gate/runner/pyshim.py <root> <dotted.module>

Fails a skip, an expected failure, an unexpected success, zero tests, an import error, and anything that reaches
sys.unraisablehook (under -W error, a leaked file or process raises in its destructor, and unittest alone reports
OK). Standalone and held to Python 3.12: `pyfloor` runs it under the pinned 3.12.16.
"""

import gc
import signal
import sys
import unittest


def main(argv: list[str]) -> int:
    root, name = argv[1], argv[2]
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)  # an ignored SIGCHLD survives exec, and reports exit 1 as exit 0
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
    result = unittest.TextTestRunner(stream=sys.stderr, verbosity=2).run(suite)
    gc.collect()

    problems: list[str] = []
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
    sys.exit(main(sys.argv))
