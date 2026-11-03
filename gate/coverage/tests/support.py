"""Shared helpers of the coverage instrument's tests: each test owns its world (testing.md rule 3)."""

import faulthandler
import json
import os
import platform
import sys
import tempfile
import unittest

from gate.proofs.drivers.support import read_text, run_process
from gate.runner import env as runner_env

COVERAGE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REPO = os.path.dirname(os.path.dirname(COVERAGE))
TESTDATA = os.path.join(COVERAGE, "testdata")
MAIN = os.path.join(COVERAGE, "main.py")
CEILING_S = 300
RUN_CAP_S = 240  # one command a test runs
PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
HIDDEN = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "VORSAR_CEP_WORKERS")


def fixture(name: str) -> bytes:
    """A committed fixture. `.out` is git-ignored, so a fixture named that way never reaches a clean clone (C33)."""
    if name.endswith(".out"):
        raise AssertionError(f"fixture {name}: .gitignore hides *.out, so a clean clone would not have it")
    with open(os.path.join(TESTDATA, name), "rb") as f:
        return f.read()


def tool_bin(name: str) -> str:
    """The pinned tool's PATH directory, read from tools.json the way the design note lays out the cache."""
    with open(os.path.join(REPO, "tools.json"), encoding="utf-8") as f:
        tools = json.load(f)["tools"]
    tool = next(t for t in tools if t["name"] == name)
    plat = PLATFORMS[(sys.platform, platform.machine())]
    cache = os.environ.get("VORSAR_CEP_TOOLS_CACHE") or os.path.expanduser("~/.cache/vorsar/cep/tools")
    home = os.path.join(cache, f"{tool['name']}-{tool['version']}-{tool['platforms'][plat]['archive_sha256'][:12]}")
    return os.path.join(home, tool["path_dir"])


def go_env(**extra: str) -> dict[str, str]:
    """The environment a row gets (the runner's own rule, design note §4), the pinned go first; no outer gate row."""
    base = {k: v for k, v in os.environ.items() if k not in HIDDEN and not k.startswith("GATE_")}
    base.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
    return {**runner_env.clean_env(base, [tool_bin("go")]), **extra}


def run(argv: list[str], cwd: str, env: dict[str, str], out_dir: str, name: str) -> tuple[int, str, str]:
    """Run a command in its own group, capped, output in files (never pipes); give (exit code, stdout, stderr)."""
    out, err = os.path.join(out_dir, f"{name}.stdout"), os.path.join(out_dir, f"{name}.stderr")
    code = run_process(argv, cwd=cwd, env=env, out=out, err=err, cap_s=RUN_CAP_S)
    return code, read_text(out), read_text(err)


class CoverageTest(unittest.TestCase):
    """A ceiling and a temporary directory per test."""

    def setUp(self) -> None:
        faulthandler.dump_traceback_later(CEILING_S, exit=True)
        self.addCleanup(faulthandler.cancel_dump_traceback_later)
        tmp = tempfile.TemporaryDirectory(prefix="gate-coverage-")
        self.addCleanup(tmp.cleanup)
        self.root = os.path.realpath(tmp.name)

    def write(self, rel: str, data: str | bytes) -> str:
        path = os.path.join(self.root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            f.write(data.encode("utf-8") if isinstance(data, str) else data)
        return path
