"""The covered statements of gate/canary's production code (tests excluded), by stdlib trace (S1.1c-7 A2 C4).

    python3.15 -I -B covset.py <root> <out.json> suite
        the design note's form: unittest.main(module=None, argv=["x", "discover", "-s", "gate/canary/tests",
        "-t", "."], exit=False) run through Trace(count=1, trace=0).runfunc, from <root>
    python3.15 -I -B covset.py <root> <out.json> test <unittest id>
        one test's own set: loaded (imported) before tracing starts, then run through runfunc, so module-level
        statements that run at import are not counted as the test's

Writes {"ran", "ok", "failures", "errors", "skipped", "lines": [[file, line], ...]} sorted, then a newline.
"""

import json
import os
import sys
import trace
import unittest


def covered(root: str, counts: dict) -> list[list[object]]:
    prod = os.path.join(root, "gate", "canary") + os.sep
    tests = os.path.join(prod, "tests") + os.sep
    lines = set()
    for (filename, line), _ in counts.items():
        path = os.path.realpath(filename if os.path.isabs(filename) else os.path.join(root, filename))
        if path.startswith(prod) and not path.startswith(tests):
            lines.add((os.path.relpath(path, root), line))
    return [list(item) for item in sorted(lines)]


def main(argv: list[str]) -> int:
    root, out, mode = os.path.realpath(argv[1]), argv[2], argv[3]
    os.chdir(root)
    sys.path.insert(0, root)
    tracer = trace.Trace(count=1, trace=0)
    if mode == "suite":
        holder = {}

        def run() -> None:
            holder["program"] = unittest.main(
                module=None, argv=["x", "discover", "-s", "gate/canary/tests", "-t", "."], exit=False
            )

        tracer.runfunc(run)
        result = holder["program"].result
    elif mode == "test":
        suite = unittest.defaultTestLoader.loadTestsFromName(argv[4])
        result = tracer.runfunc(unittest.TextTestRunner(stream=sys.stderr).run, suite)
    else:
        raise SystemExit(f"unknown mode {mode}")
    record = {
        "errors": len(result.errors),
        "failures": len(result.failures),
        "lines": covered(root, tracer.results().counts),
        "ok": result.wasSuccessful(),
        "ran": result.testsRun,
        "skipped": len(result.skipped),
    }
    tmp = out + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        f.write(json.dumps(record, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n")
    os.rename(tmp, out)
    return 0 if record["ok"] else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
