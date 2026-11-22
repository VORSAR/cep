"""How many times one production line runs while one test runs alone, by stdlib trace (S1.1c-7 A2 C8, G2-14's K2b).

    python3.15 -I -B linecount.py <root> <file relative to root> <line> <unittest id>

The test is loaded (imported) before tracing starts, then run through Trace(count=1, trace=0).runfunc from <root>.
Prints `ran <n>, ok <bool>, failures <n>, errors <n>, skipped <n>` and `count <file>:<line> = <n>`; exits 0 when the
test passed, else 1.
"""

import os
import sys
import trace
import unittest


def main(argv: list[str]) -> int:
    root, rel, line, test_id = os.path.realpath(argv[1]), argv[2], int(argv[3]), argv[4]
    os.chdir(root)
    sys.path.insert(0, root)
    target = os.path.join(root, rel)
    suite = unittest.defaultTestLoader.loadTestsFromName(test_id)
    tracer = trace.Trace(count=1, trace=0)
    result = tracer.runfunc(unittest.TextTestRunner(stream=sys.stderr).run, suite)
    count = 0
    for (filename, number), n in tracer.results().counts.items():
        path = os.path.realpath(filename if os.path.isabs(filename) else os.path.join(root, filename))
        if path == target and number == line:
            count += n
    print(
        f"ran {result.testsRun}, ok {result.wasSuccessful()}, failures {len(result.failures)}, "
        f"errors {len(result.errors)}, skipped {len(result.skipped)}"
    )
    print(f"count {rel}:{line} = {count}")
    return 0 if result.wasSuccessful() else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
