"""Fake test binaries for compare.py: scripted `go test -bench` output, every invocation's argv logged."""

import json
import os
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
PY = sys.executable

FAKE = """\
#!{python} -I
import json, sys
with open({conf!r}, encoding="utf-8") as f:
    conf = json.load(f)
argv = sys.argv[1:]
kind = "alloc" if "-test.benchmem" in argv else "time"
with open(conf["log"], "a", encoding="utf-8") as f:
    f.write(json.dumps([conf["side"], kind, argv]) + "\\n")
with open(conf["log"], encoding="utf-8") as f:
    k = sum(1 for line in f if json.loads(line)[:2] == [conf["side"], kind]) - 1
runs = conf[kind]
run = runs[k % len(runs)]
sys.stdout.write(run["out"])
sys.stderr.write(run.get("err", ""))
sys.exit(run.get("exit", 0))
"""

HEADER = "goos: darwin\ngoarch: arm64\npkg: example.com/b\n"


def output(benches: dict[str, tuple]) -> str:
    """go test -bench's text: name -> (ns,) or (ns, bytes, allocs); names never carry a suffix at cpu 1."""
    lines = [HEADER]
    for name, values in benches.items():
        line = f"{name}   \t    1000\t{values[0]:>10} ns/op"
        if len(values) == 3:
            line += f"\t{values[1]:>8} B/op\t{values[2]:>8} allocs/op"
        lines.append(line + "\n")
    lines.append("PASS\n")
    return "".join(lines)


def fake(directory: str, side: str, log: str, *, time: list[dict], alloc: list[dict]) -> str:
    """An executable fake binary; `time` and `alloc` are its runs ({"out", "err", "exit"}), taken in turn."""
    conf = os.path.join(directory, f"{side}.json")
    with open(conf, "w", encoding="utf-8") as f:
        json.dump({"side": side, "log": log, "time": time, "alloc": alloc}, f)
    path = os.path.join(directory, f"{side}.test")
    with open(path, "w", encoding="utf-8") as f:
        f.write(FAKE.format(python=PY, conf=conf))
    os.chmod(path, 0o755)
    return path


def read_log(log: str) -> list[list]:
    with open(log, encoding="utf-8") as f:
        return [json.loads(line) for line in f]


def compare(argv: list[str], cwd: str, env: dict[str, str] | None = None) -> tuple[int, str]:
    """Run compare.py as the coordinator does; its exit code and stdout."""
    script = os.path.join(REPO, "gate", "bench", "compare.py")
    with open(os.path.join(cwd, "compare.stdout"), "w+b") as out:
        code = subprocess.run(
            [PY, "-I", "-B", script, *argv], cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=out
        ).returncode
        out.seek(0)
        return code, out.read().decode("utf-8", errors="replace")
