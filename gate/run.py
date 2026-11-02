"""The gate's entry point: refuse any interpreter but the pinned one, then run the gate.

Every verdict comes from the pinned CPython, started with -I -B -X utf8 (plan check S0, P3). This file must
therefore reach its own guard under whatever interpreter started it, so it stays parseable by Python 3.12 and
imports nothing from gate/ until the guard has passed. Under -I the script's directory is not on sys.path, so the
repository is put there by hand.
"""

import json
import os
import platform
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
FIX = "run `make tools`, then start the gate through it: bootstrap.py exec"


def pinned_python():
    """The pinned interpreter's path, as the design note lays out the cache. Read directly: nothing else is safe yet."""
    with open(os.path.join(REPO, "tools.json"), "rb") as f:
        tool = next(t for t in json.load(f)["tools"] if t["name"] == "python")
    digest = tool["platforms"][PLATFORMS[(sys.platform, platform.machine())]]["archive_sha256"]
    cache = os.environ.get("VORSAR_CEP_TOOLS_CACHE") or os.path.expanduser("~/.cache/vorsar/cep/tools")
    home = os.path.join(cache, f"{tool['name']}-{tool['version']}-{digest[:12]}")
    return os.path.join(home, tool["path_dir"], tool["command"])


def refusal():
    """Why this interpreter may not give a verdict; empty when it may."""
    try:
        want = pinned_python()
    except (OSError, ValueError, KeyError, StopIteration) as e:
        return f"refused: cannot locate the pinned interpreter: {type(e).__name__}: {e}"
    if os.path.realpath(sys.executable or "") != os.path.realpath(want):
        return f"refused: {sys.executable} is not the pinned {want}"
    if not (sys.flags.isolated and sys.dont_write_bytecode and sys.flags.utf8_mode):
        return "refused: the pinned interpreter must be started with -I -B -X utf8"
    return ""


def main():
    reason = refusal()
    if reason:
        print(f"{reason}: {FIX}")
        print("GATE FAILED")
        print("exit: 2")
        return 2
    sys.path.insert(0, REPO)
    from gate.runner import main as runner

    return runner.main(sys.argv[1:])


if __name__ == "__main__":
    try:
        code = main()
    except BaseException as e:
        print(f"internal error: {type(e).__name__}: {e}")
        print("GATE FAILED")
        print("exit: 2")
        code = 2
    sys.exit(code)
