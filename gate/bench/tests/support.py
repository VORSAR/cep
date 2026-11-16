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


GIT_ENV = {
    "GIT_CONFIG_GLOBAL": os.devnull,
    "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "t",
    "GIT_AUTHOR_EMAIL": "t@example.com",
    "GIT_COMMITTER_NAME": "t",
    "GIT_COMMITTER_EMAIL": "t@example.com",
}


def git(repo: str, env: dict[str, str], *argv: str) -> str:
    done = subprocess.run(["git", *argv], cwd=repo, env=env, check=True, capture_output=True)
    return done.stdout.decode().strip()


def write(directory: str, files: dict[str, str]) -> None:
    for name, text in files.items():
        path = os.path.join(directory, name)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)


def fake_go(directory: str, body: str) -> str:
    """A directory holding an executable `go` (Python under this interpreter) running `body`; put it first on PATH."""
    bin_dir = os.path.join(directory, "fakebin")
    os.makedirs(bin_dir, exist_ok=True)
    path = os.path.join(bin_dir, "go")
    with open(path, "w", encoding="utf-8") as f:
        f.write(f"#!{PY} -I\n{body}")
    os.chmod(path, 0o755)
    return bin_dir


def tool(name: str, argv: list[str], cwd: str, env: dict[str, str], out_path: str) -> tuple[int, str]:
    """Run gate/bench/<name>.py as a lane does; its exit code and combined output."""
    script = os.path.join(REPO, "gate", "bench", f"{name}.py")
    with open(out_path, "w+b") as out:
        code = subprocess.run(
            [PY, "-I", "-B", script, *argv], cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=out
        ).returncode
        out.seek(0)
        return code, out.read().decode("utf-8", errors="replace")


def pids_line(out: str, prefix: str) -> list[int]:
    """The pids a tool printed on its `<prefix> ... pids a b c` line."""
    for line in out.splitlines():
        if line.startswith(prefix) and "pids " in line:
            return [int(p) for p in line.split("pids ", 1)[1].split()]
    raise AssertionError(f"no {prefix!r} line with pids in:\n{out}")


def alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True
