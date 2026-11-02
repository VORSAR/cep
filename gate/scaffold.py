"""The throwaway wave-1 gate of S1.1a-1 (owner, 2026-11-02; design note s1-1a-gate-design.md §7).

Not the gate: sequential, no records, no hash parity. It runs a short check list so wave-1 lanes
finish on an automated run, and it is deleted at combine-W1, when gate/run.py lands.
Run it under the pinned CPython 3.15.2 with -I -B (the Makefile does). Any crash is `exit: 2`.
"""

import contextlib
import json
import os
import platform
import shutil
import signal
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(REPO, ".gate", "scaffold")
GO_KEEP = frozenset({"GOCACHE", "GOMODCACHE", "GOPATH", "GOPROXY", "GOSUMDB", "GOPRIVATE", "GONOPROXY", "GONOSUMDB"})
PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
GOOS_GOARCH = {"darwin-arm64": "darwin/arm64", "linux-amd64": "linux/amd64"}
UNIT = (
    "import sys, unittest\n"
    "sys.path.insert(0, sys.argv[1])\n"
    "suite = unittest.defaultTestLoader.loadTestsFromName(sys.argv[2])\n"
    "r = unittest.TextTestRunner(verbosity=2).run(suite)\n"
    "bad = r.skipped or r.expectedFailures or r.unexpectedSuccesses or not r.wasSuccessful() or r.testsRun < 1\n"
    "sys.exit(1 if bad else 0)\n"
)


class Fail(Exception):
    """A check failed; the message is the summary's reason."""


def platform_key() -> str:
    key = PLATFORMS.get((sys.platform, platform.machine()))
    if key is None:
        raise Fail(f"unsupported platform {sys.platform}/{platform.machine()}: need darwin-arm64 or linux-amd64")
    return key


def tool_dirs(manifest: dict, plat: str) -> dict[str, str]:
    cache = os.environ.get("VORSAR_CEP_TOOLS_CACHE") or os.path.expanduser("~/.cache/vorsar/cep/tools")
    return {
        t["name"]: os.path.join(cache, f"{t['name']}-{t['version']}-{t['platforms'][plat]['archive_sha256'][:12]}")
        for t in manifest["tools"]
    }


def clean_env(base: dict[str, str], path_dirs: list[str]) -> dict[str, str]:
    """Design note §4: drop GO* outside the allow-list, PYTHON* and GIT_*_PATHSPECS; pin the Go settings."""
    env = {
        k: v
        for k, v in base.items()
        if not (k.startswith("GO") and k not in GO_KEEP)
        and not k.startswith("PYTHON")
        and not (k.startswith("GIT_") and k.endswith("_PATHSPECS"))
    }
    env["PATH"] = os.pathsep.join([*path_dirs, base.get("PATH", "")])
    env.update(GOTOOLCHAIN="local", GOENV="off", GOFLAGS="", GOWORK="off", CGO_ENABLED="0")
    return env


def parse_json_stream(data: bytes) -> list[dict]:
    """Parse a stream of possibly multi-line JSON objects, as `go vet -json` prints them."""
    text, decoder, objs, i = data.decode("utf-8", errors="replace"), json.JSONDecoder(), [], 0
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i == len(text):
            return objs
        obj, i = decoder.raw_decode(text, i)
        if not isinstance(obj, dict):
            raise ValueError(f"expected a JSON object, got {type(obj).__name__}")
        objs.append(obj)


def kill_group(pgid: int, sig: int) -> None:
    with contextlib.suppress(ProcessLookupError):
        os.killpg(pgid, sig)


def run(step: str, argv: list[str], env: dict[str, str], timeout_s: int) -> tuple[int, bytes]:
    """Run one command in its own process group, output to files; return (exit code, stdout)."""
    out_path, err_path = os.path.join(OUT, f"{step}.stdout"), os.path.join(OUT, f"{step}.stderr")
    with open(out_path, "wb") as out, open(err_path, "wb") as err:
        proc = subprocess.Popen(
            argv, cwd=REPO, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True
        )
        try:
            code = proc.wait(timeout=timeout_s)
        except subprocess.TimeoutExpired:
            kill_group(proc.pid, signal.SIGTERM)
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                kill_group(proc.pid, signal.SIGKILL)
                proc.wait()
            raise Fail(f"timeout after {timeout_s}s (output in {out_path})") from None
        kill_group(proc.pid, signal.SIGKILL)  # nothing a step started may outlive it
    with open(out_path, "rb") as f:
        return code, f.read()


def git_ls(env: dict[str, str], *pathspecs: str) -> list[str]:
    code, out = run("git-ls", ["git", "ls-files", "-z", "--", *pathspecs], env, 60)
    if code != 0:
        raise Fail(f"git ls-files exit {code}")
    return sorted(p for p in out.decode("utf-8").split("\0") if p)


def check_parity(manifest: dict, dirs: dict[str, str], plat: str, env: dict[str, str]) -> str:
    for t in manifest["tools"]:
        home = os.path.realpath(dirs[t["name"]])
        if not os.path.isdir(home):
            raise Fail(f"{t['name']}: not installed (expected {home})")
        found = shutil.which(t["command"], path=env["PATH"])
        if found is None or not os.path.realpath(found).startswith(home + os.sep):
            raise Fail(f"{t['name']}: path: {found} is outside the cache")
        code, out = run(
            f"version-{t['name']}", [os.path.join(home, t["version_argv"][0]), *t["version_argv"][1:]], env, 60
        )
        want = t["version_line"].replace("{goos}/{goarch}", GOOS_GOARCH[plat])
        got = out.decode("utf-8", errors="replace").splitlines()[:1]
        if code != 0 or got != [want]:
            raise Fail(f"{t['name']}: version: want {want!r}, got {got!r} (exit {code})")
    return f"{len(manifest['tools'])} tools"


def check_gofmt(env: dict[str, str]) -> str:
    files = git_ls(env, ":(glob)**/*.go")
    if not files:
        raise Fail("no tracked Go files (zero is a failure)")
    code, out = run("gofmt", ["gofmt", "-l", *files], env, 120)
    if code != 0 or out.strip():
        raise Fail(f"gofmt exit {code}; unformatted: {out.decode().split()} (fix: gofmt -w <file>)")
    return f"{len(files)} files"


def check_vet(env: dict[str, str]) -> str:
    code, out = run("vet", ["go", "vet", "-json", "./..."], env, 300)
    if code != 0:
        raise Fail(f"go vet exit {code} (see .gate/scaffold/vet.stderr)")
    objs = parse_json_stream(out)
    findings = [o for o in objs if o]
    if findings:
        raise Fail(f"go vet findings: {json.dumps(findings, sort_keys=True)[:300]}")
    return f"{len(objs)} units"


def check_build(env: dict[str, str]) -> str:
    code, _ = run("build", ["go", "build", "./..."], env, 300)
    if code != 0:
        raise Fail(f"go build exit {code} (see .gate/scaffold/build.stderr)")
    return "ok"


def check_pytests(python: str, env: dict[str, str]) -> str:
    modules = git_ls(env, ":(glob)gate/**/test_*.py", ":(glob,exclude)gate/proofs/**")
    if not modules:
        raise Fail("no tracked gate test modules (zero is a failure)")
    failed = []
    for path in modules:
        dotted = path.removesuffix(".py").replace("/", ".")
        code, _ = run(
            f"py-{dotted}", [python, "-I", "-B", "-X", "dev", "-W", "error", "-c", UNIT, REPO, dotted], env, 300
        )
        if code != 0:
            failed.append(path)
    if failed:
        raise Fail(f"failing, skipping or empty test modules: {failed} (see .gate/scaffold/py-*.stderr)")
    return f"{len(modules)} modules"


def check_ruff(env: dict[str, str]) -> str:
    files = git_ls(env, ":(glob)gate/**/*.py")
    if not files:
        raise Fail("no tracked gate Python files (zero is a failure)")
    fmt, _ = run("ruff-format", ["ruff", "format", "--check", "--no-cache", "--config", "ruff.toml", *files], env, 120)
    lint, _ = run(
        "ruff-check", ["ruff", "check", "--ignore-noqa", "--no-cache", "--config", "ruff.toml", *files], env, 120
    )
    if fmt != 0 or lint != 0:
        raise Fail(f"ruff format exit {fmt}, ruff check exit {lint} (see .gate/scaffold/ruff-*.stdout)")
    return f"{len(files)} files"


def main() -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    os.makedirs(OUT, exist_ok=True)
    plat = platform_key()
    with open(os.path.join(REPO, "tools.json"), encoding="utf-8") as f:
        manifest = json.load(f)
    dirs = tool_dirs(manifest, plat)
    paths = [os.path.join(dirs[t["name"]], t["path_dir"]) for t in manifest["tools"]]
    env = clean_env(dict(os.environ), [os.path.normpath(p) for p in paths])
    python = os.path.join(dirs["python"], "bin", "python3.15")
    steps = [
        ("parity-lite", lambda: check_parity(manifest, dirs, plat, env)),
        ("gofmt", lambda: check_gofmt(env)),
        ("vet", lambda: check_vet(env)),
        ("build", lambda: check_build(env)),
        ("pytests", lambda: check_pytests(python, env)),
        ("ruff", lambda: check_ruff(env)),
    ]
    rows, failed = [], False
    for name, step in steps:
        try:
            rows.append((name, "PASS", step()))
        except Fail as e:
            rows.append((name, "FAIL", str(e)))
            failed = True
    print("SCAFFOLD: the throwaway wave-1 check list (not the gate; deleted at combine-W1)")
    for name, verdict, reason in rows:
        print(f"{name:<12} | {verdict:<4} | {reason}")
    print("GATE FAILED" if failed else "GATE PASSED")
    print(f"exit: {1 if failed else 0}")
    return 1 if failed else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        print(f"scaffold crashed: {type(e).__name__}: {e}")
        print("GATE FAILED")
        print("exit: 2")
        sys.exit(2)
