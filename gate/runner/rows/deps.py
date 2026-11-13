"""The deps row (S1.1c-1 lane C; design note §5.3; owner 2026-11-11, the plan check, 1, 4 and 8, and the design, 1
and 7; ADR-0009 §1; ADR-0001 §1): the dependency allowlist, licence and one-module checks, offline.

It reads both tracked modules (go.mod, gate/proofs/go.mod) through the shared loader, gate/runner/rows/gomods.py
(S1.1c-4 design note §2.1, §2.7): the build lists, the module graph and the modules every package, test and `tool`
line compiles over both supported platforms; every package load error is a named line, never a module silently
graph-only. GOPROXY=off: a module missing from the cache is a named failure with its fix command, never a
download. Each entry's `sprint` and `ref` are checked against gate/sprints.json, gate/current-sprint and the
tracked Markdown (S1.1c-2 design §4.6a). GATE_COVERED is 3: the three named checks, printed once the allowlist is
read. The rule itself is gate/runner/deps.py.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import allowlists, anchors, deps, rows, sprintid
from gate.runner.rows import gomods

ENV = {"GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "CGO_ENABLED": "0"}


def read_license(directory: str) -> str | None:
    for name in deps.LICENSE_FILES:
        try:
            return rows.read_text(os.path.join(directory, name))
        except OSError:
            continue
    return None


def gather(row_dir: str, env: dict[str, str]) -> tuple[list[deps.Module], frozenset[str], list[str]]:
    """The union of the build lists marked imported (built), the union of the graphs, the two-version lines and the
    loader's error lines (design §2.7)."""
    loaded = gomods.load(row_dir, env)
    root, proofs = loaded.versions["."], loaded.versions["gate/proofs"]
    failures = [
        deps.two_versions_line(path, root[path], proofs[path])
        for path in sorted(set(root) & set(proofs))
        if root[path] != proofs[path]
    ]
    modules = [
        deps.Module(m.path, m.version, m.dir, m.built, " ".join(p for p in (m.replace_path, m.replace_version) if p))
        for m in loaded.modules
    ]
    return modules, loaded.graph, failures + list(loaded.errors)


def context(row_dir: str, env: dict[str, str]) -> allowlists.Ctx:
    """The sprint order, the marker and a resolver over tracked Markdown (an untracked doc never satisfies a ref)."""
    order = sprintid.load_order(rows.read_text(sprintid.SPRINTS))
    current = sprintid.read_marker(rows.read_text(sprintid.MARKER))
    order.index(sprintid.canonical(current))  # the marker's row is in the order, or a ValueError naming it
    listed = gomods.run(row_dir, env, ".", "markdown", ["git", "ls-files", "-z", "--", "*.md"])
    tracked = frozenset(path for path in listed.split("\0") if path)

    def read(path: str) -> str | None:
        if path not in tracked:
            return None
        try:
            return rows.read_text(path)
        except OSError:
            return None

    return allowlists.Ctx(
        today=rows.today(),
        tier=env.get("GATE_TIER", ""),
        order=order,
        current=current,
        issues=frozenset(),
        resolve=lambda ref: anchors.resolves(ref, read),
    )


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    env = {**os.environ, **ENV}
    try:
        modules, graph, failures = gather(row_dir, env)
        listed = gomods.run(row_dir, env, ".", "gomods", ["git", "ls-files", "-z", "--", "go.mod", ":(glob)**/go.mod"])
        ctx = context(row_dir, env)
    except (gomods.Failed, OSError, ValueError) as e:
        return rows.finish([f"deps: {e}"])
    tracked = sorted({path for path in listed.split("\0") if path})
    try:
        entries = deps.load_allowlist(rows.read_text(deps.ALLOWLIST))
    except (OSError, ValueError) as e:  # the `built` mark's faults already name the row (design §2.6)
        return rows.finish([str(e) if str(e).startswith("deps: ") else f"deps: {e}"])
    rows.covered(3)
    return rows.finish(failures + deps.compare(modules, graph, entries, tracked, read_license, ctx))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
