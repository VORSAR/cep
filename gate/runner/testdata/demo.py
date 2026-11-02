"""`make runner-check`: the real runner and the real row wrappers, over this repository, before combine-W1.

gate/run.py cannot run yet on a lane branch: gate/toolchain is Lane B's, and its stubs raise. So this calls the real
main with a stand-in for that one seam. The stand-in does the scaffold's parity-lite: each tool resolves inside the
cache, and its version line matches. It is not parity: nothing is hashed. It exists only for this demonstration and
goes away with the demo at combine-W1, when `gate/run.py` carries the real thing.
"""

import json
import os
import platform
import shutil
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import clock, main
from gate.runner.proc import Group

PLATFORMS = {("darwin", "arm64"): "darwin-arm64", ("linux", "x86_64"): "linux-amd64"}
GOOS_GOARCH = {"darwin-arm64": "darwin/arm64", "linux-amd64": "linux/amd64"}


class StandIn:
    def __init__(self, repo: str, environ: dict[str, str]) -> None:
        with open(os.path.join(repo, "tools.json"), encoding="utf-8") as f:
            self._tools = json.load(f)["tools"]
        self._repo = repo
        self._plat = PLATFORMS[(sys.platform, platform.machine())]
        cache = environ.get("VORSAR_CEP_TOOLS_CACHE") or os.path.expanduser("~/.cache/vorsar/cep/tools")
        self._homes = [
            os.path.join(cache, f"{t['name']}-{t['version']}-{t['platforms'][self._plat]['archive_sha256'][:12]}")
            for t in self._tools
        ]

    def path_dirs(self) -> list[str]:
        return [os.path.join(h, t["path_dir"]) for h, t in zip(self._homes, self._tools, strict=True)]

    def tool_count(self) -> int:
        return len(self._tools)

    def parity(self, env: dict[str, str], *, full_tree: bool) -> list[str]:
        pairs = zip(self._tools, self._homes, strict=True)
        return [reason for tool, home in pairs if (reason := self._check(tool, home, env))]

    def _check(self, tool: dict, home: str, env: dict[str, str]) -> str:
        found = shutil.which(tool["command"], path=env["PATH"])
        if found is None or not os.path.realpath(found).startswith(os.path.realpath(home) + os.sep):
            return f"{tool['name']}: path: {found} is outside the cache"
        with tempfile.TemporaryDirectory() as d:
            out = os.path.join(d, "out")
            group = Group()
            proc = group.spawn([os.path.join(home, tool["version_argv"][0]), *tool["version_argv"][1:]], cwd=self._repo,
                               env=env, stdout=out, stderr=os.path.join(d, "err"))  # fmt: skip
            assert proc is not None
            group.reap(proc)
            with open(out, "rb") as f:
                got = f.read().decode("utf-8", errors="replace").split("\n")[0]
        want = tool["version_line"].replace("{goos}/{goarch}", GOOS_GOARCH[self._plat])
        return "" if got == want else f"{tool['name']}: version: want {want}, got {got}"


if __name__ == "__main__":
    deps = main.default_deps()
    deps = main.Deps(repo=deps.repo, environ=deps.environ, clock=clock.REAL, timers=deps.timers, toolchain=StandIn)
    sys.exit(main.main(sys.argv[1:], deps=deps))
