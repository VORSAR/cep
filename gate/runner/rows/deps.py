"""The deps row (S1.1c-1 lane C; design note §5.3; owner 2026-11-11, the plan check, 1, 4 and 8, and the design, 1
and 7; ADR-0009 §1; ADR-0001 §1): the dependency allowlist, licence and one-module checks, offline.

In each tracked module (go.mod, gate/proofs/go.mod) it reads the build list (`go list -m -json all`), the module
graph (`go mod graph`) and the modules the packages build or test with (`go list -e -deps -test ./...`); the apidiff
tool's modules count as imported through whichever module declares the tool (gate/proofs after decision 7, the
root before it). GOPROXY=off: a module missing from the cache is a named failure with its fix command, never a
download. `-e` keeps a package that does not build in the listing: a module reached only through it falls from
imported to graph-only, weaker, never red (design note §5.3). GATE_COVERED is 3: the three named checks, printed
once the allowlist is read. The rule itself is gate/runner/deps.py.
"""

import os
import subprocess
import sys
from collections.abc import Sequence

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import deps, rows

MODULES = (".", "gate/proofs")
TOOL = "golang.org/x/exp/cmd/apidiff"
MODULE_PATH = "{{if .Module}}{{.Module.Path}}{{end}}"
ENV = {"GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "CGO_ENABLED": "0"}


class Failed(Exception):
    """A tool that did not finish: its one failed line."""


def run(row_dir: str, env: dict[str, str], cwd: str, name: str, argv: Sequence[str]) -> str:
    """One tool's stdout, its output in the row directory; non-zero is Failed naming the first stderr line."""
    out, err = os.path.join(row_dir, f"{name}.stdout"), os.path.join(row_dir, f"{name}.stderr")
    with open(out, "wb") as o, open(err, "wb") as e:
        code = subprocess.run(list(argv), cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=o, stderr=e).returncode
    if code != 0:
        lines = rows.read_text(err).splitlines()
        raise Failed(f"deps: {' '.join(argv)} failed: {lines[0] if lines else f'exit {code}'}")
    return rows.read_text(out)


def read_license(directory: str) -> str | None:
    for name in deps.LICENSE_FILES:
        try:
            return rows.read_text(os.path.join(directory, name))
        except OSError:
            continue
    return None


def gather(row_dir: str, env: dict[str, str]) -> tuple[list[deps.Module], frozenset[str], list[str]]:
    """The union of the build lists marked imported, the union of the graphs, and the two-version lines."""
    builds: dict[str, dict[str, tuple[str, str]]] = {}
    mains: list[str] = []
    graph: set[str] = set()
    imported: set[str] = set()
    for mod in MODULES:
        tag = "root" if mod == "." else mod.replace("/", "_")
        build: dict[str, tuple[str, str]] = {}
        main = ""
        for obj in deps.decode_json_stream(
            run(row_dir, env, mod, f"{tag}.list-m", ["go", "list", "-m", "-json", "all"])
        ):
            if obj.get("Main"):
                main = obj["Path"]
            elif "Version" in obj:
                build[obj["Path"]] = (obj["Version"], obj.get("Dir") or "")
        builds[mod], mains = build, [*mains, main]
        graph |= deps.graph_nodes(run(row_dir, env, mod, f"{tag}.graph", ["go", "mod", "graph"]), [main])
        listing = ["go", "list", "-e", "-deps", "-test", "-f", MODULE_PATH, "./..."]
        imported |= set(run(row_dir, env, mod, f"{tag}.imported", listing).split())
    for mod in reversed(MODULES):  # the tool's modules: gate/proofs declares it after decision 7, the root before
        tag = "root" if mod == "." else mod.replace("/", "_")
        found = run(row_dir, env, mod, f"{tag}.tool", ["go", "list", "-e", "-deps", "-f", MODULE_PATH, TOOL]).split()
        if found:
            imported |= set(found)
            break
    failures = [
        f"deps: {path}: go.mod has {builds['.'][path][0]}, gate/proofs/go.mod has {builds['gate/proofs'][path][0]}"
        for path in sorted(set(builds["."]) & set(builds["gate/proofs"]))
        if builds["."][path][0] != builds["gate/proofs"][path][0]
    ]
    modules: dict[str, deps.Module] = {}
    for mod in MODULES:
        for path, (version, directory) in builds[mod].items():
            modules.setdefault(path, deps.Module(path, version, directory, path in imported))
    return [modules[path] for path in sorted(modules)], frozenset(graph - set(mains)), failures


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    env = {**os.environ, **ENV}
    try:
        modules, graph, failures = gather(row_dir, env)
        listed = run(row_dir, env, ".", "gomods", ["git", "ls-files", "-z", "--", "go.mod", ":(glob)**/go.mod"])
    except Failed as e:
        return rows.finish([str(e)])
    gomods = sorted({path for path in listed.split("\0") if path})
    try:
        entries = deps.load_allowlist(rows.read_text(deps.ALLOWLIST))
    except (OSError, ValueError) as e:
        return rows.finish([f"deps: {e}"])
    rows.covered(3)
    return rows.finish(failures + deps.compare(modules, graph, entries, gomods, read_license))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
