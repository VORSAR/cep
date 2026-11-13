"""The deps row (S1.1c-1 lane C; design note §5.3; owner 2026-11-11, the plan check, 1, 4 and 8, and the design, 1
and 7; ADR-0009 §1; ADR-0001 §1): the dependency allowlist, licence and one-module checks, offline.

It reads both tracked modules (go.mod, gate/proofs/go.mod) through the shared loader, gate/runner/rows/gomods.py
(S1.1c-4 design note §2.1, §2.7): the build lists, the module graph and the modules every package, test and `tool`
line compiles over both supported platforms; every package load error is a named line, never a module silently
graph-only. GOPROXY=off: a module missing from the cache is a named failure with its fix command, never a
download. Each entry's `sprint` and `ref` are checked against gate/sprints.json, gate/current-sprint and the
tracked Markdown (S1.1c-2 design §4.6a). GATE_COVERED is 3: the three named checks, printed once the allowlist is
read. The rule itself is gate/runner/deps.py.

Two lines come last, after every other check has reported (S1.1c-4 design note §3.1, §3.2): `go mod tidy -diff` in
each module, read as sets per file against TIDY_ALLOWED (a run that prints no diff header but an error could not
run, never clean), and at T4 `git diff --name-only HEAD` over both go.sum files (owner, 2026-11-13, the design's
decision 3: every T4, local and CI alike).
"""

import os
import re
import sys
from collections.abc import Mapping

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import allowlists, anchors, deps, rows, sprintid
from gate.runner.rows import gomods

ENV = gomods.ENV
# The root's tidy allowance (S1.1c-1's design, 7; S1.1c-4 plan check, confirm (a)): `go mod tidy` drops the two pinned
# requires of go.mod's second block and their modules' go.sum lines (x/sync's with them); nothing else may change.
TIDY_ALLOWED: Mapping[str, frozenset] = {
    "go.mod": frozenset(
        {("golang.org/x/mod", "v0.42.0"), ("golang.org/x/tools", "v0.52.1-0.20261106145633-d407941a339a")}
    ),
    "go.sum": frozenset(
        {
            "golang.org/x/mod v0.42.0 h1:ICeL3t2L30ljSPHHxDnR4fqVcBt/zR2zJAb9juavmQg=",
            "golang.org/x/mod v0.42.0/go.mod h1:u5fzIrRInkL8MaO1cbRkUHT64v303YIT5KEqpNr7jRs=",
            "golang.org/x/sync v0.24.0 h1:iqiDrGoDgyuvAKrhLQthUEzU/R378N78gXbBjxed7oA=",
            "golang.org/x/sync v0.24.0/go.mod h1:sUUOizhqBxiL6pEWpqNLUiaJn1ShEbZ6BBqskPbjZm0=",
            "golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a h1:Yqozs0FNPRoPGS4hNGlrNwRENWr5H5nVG9sN/f+5640=",
            "golang.org/x/tools v0.52.1-0.20261106145633-d407941a339a/go.mod "
            "h1:ItcP5UNaVGG+Yc4eIYmaFLNJ4soEopkx5Y2d4lcEurs=",
        }
    ),
}
TIDY = ("go", "mod", "tidy", "-diff")
DOWNLOADING = "go: downloading"
GOSUMS = ("go.sum", "gate/proofs/go.sum")
_PAIR = re.compile(r"(\S+)\s+(v\S+)(?:\s+//\s*indirect)?")
# go.mod's directive keywords: never a module path, so `retract v1.0.0` is a directive line, not a pair.
_DIRECTIVES = frozenset({"module", "go", "toolchain", "godebug", "require", "exclude", "replace", "retract", "tool"})


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


def _tidy_changes(text: str) -> list[tuple[str, str, tuple[str, str] | str, bool]]:
    """`go mod tidy -diff`'s changed lines in order: (file, sign, member, directive). A go.mod member is a require
    pair (path, version); any other changed go.mod line is a directive (its text); a go.sum member is its line."""
    changes: list[tuple[str, str, tuple[str, str] | str, bool]] = []
    file, headers = "", 0
    for line in text.splitlines():
        if line.startswith("diff current/"):
            file, headers = "", 2
        elif headers and line.startswith("--- current/"):
            file, headers = line.removeprefix("--- current/"), headers - 1
        elif headers and line.startswith("+++ tidy/"):
            headers -= 1
        elif line[:1] in ("-", "+") and file:
            sign, body = line[0], line[1:].strip()
            if file != "go.mod":
                changes.append((file, sign, body, False))
                continue
            if not body or body.startswith("//") or body in ("require (", ")"):
                continue
            body = body.removeprefix("require ").strip()
            pair = _PAIR.fullmatch(body)
            if pair and pair[1] not in _DIRECTIVES:
                changes.append((file, sign, (pair[1], pair[2]), False))
            else:
                changes.append((file, sign, body, True))
    return changes


def _member(member: tuple[str, str] | str) -> str:
    return member if isinstance(member, str) else " ".join(member)


def tidy_faults(mod: str, stdout: str, stderr: str, code: int) -> list[str]:
    """The tidy line for one module (design §3.1): could not run, would change, or none. Keyed on the exit code
    (review CA-L3): no diff header and exit 0 is clean whatever stderr says; no diff header and any other exit is
    "could not run" (review DQ-B1), quoting the last stderr line that is not a download."""
    if not any(line.startswith("diff current/") for line in stdout.splitlines()):
        if code == 0:
            return []
        said = [line for line in stderr.splitlines() if line.strip() and not line.startswith(DOWNLOADING)]
        if said:
            why = said[-1]
        elif stdout.strip():
            why = f"no diff header on stdout: {stdout.strip().splitlines()[0]}"
        else:
            why = f"exit {code} and no diff header"
        return [f"deps: go mod tidy could not run offline in {mod}: {why}: run go mod download in {mod}"]
    allowed = TIDY_ALLOWED if mod == "." else {}
    differing: list[str] = []
    removed: dict[str, set] = {}
    for file, sign, member, directive in _tidy_changes(stdout):
        if directive or sign == "+" or member not in allowed.get(file, frozenset()):
            differing.append(f"{file}: {sign}{_member(member)}")
        else:
            removed.setdefault(file, set()).add(member)
    for file in sorted(allowed):
        kept = allowed[file] - removed.get(file, set())
        differing += [f"{file}: {_member(m)} is in TIDY_ALLOWED but tidy keeps it" for m in sorted(kept)]
    return [f"deps: go mod tidy would change {mod}: {differing[0]}"] if differing else []


def tidy_lines(row_dir: str, env: Mapping[str, str]) -> list[str]:
    """`go mod tidy -diff` in each module, offline; exit 1 is "differs" (review DA-S4), any other failure a line."""
    found: list[str] = []
    for mod in gomods.MODULES:
        name = "root.tidy" if mod == "." else f"{mod.replace('/', '_')}.tidy"
        try:
            code, stdout = gomods.run_status(row_dir, env, mod, name, list(TIDY), ok=(0, 1))
            stderr = rows.read_text(os.path.join(row_dir, f"{name}.stderr"))
        except (gomods.Failed, OSError) as e:
            found.append(f"deps: go mod tidy could not run in {mod}: {e}")
            continue
        found += tidy_faults(mod, stdout, stderr, code)
    return found


def gosum_lines(row_dir: str, env: Mapping[str, str], cwd: str = ".") -> list[str]:
    """At T4 only, each tracked go.sum that differs from HEAD (design §3.2; review DA-S5): a step wrote it."""
    if env.get("GATE_TIER", "") != "T4":
        return []
    try:
        listed = gomods.run(row_dir, env, cwd, "gosum", ["git", "diff", "--name-only", "HEAD", "--", *GOSUMS])
    except (gomods.Failed, OSError) as e:
        return [f"deps: the T4 go.sum check could not run: {e}"]
    return [
        f"deps: {name} differs from HEAD at T4: commit it, or fix the step that wrote it"
        for name in sorted(set(listed.splitlines()))
        if name
    ]


def checks(row_dir: str, env: dict[str, str]) -> list[str]:
    """The allowlist, licence and one-module checks (GATE_COVERED 3 once the allowlist is read)."""
    try:
        modules, graph, failures = gather(row_dir, env)
        listed = gomods.run(row_dir, env, ".", "gomods", ["git", "ls-files", "-z", "--", "go.mod", ":(glob)**/go.mod"])
        ctx = context(row_dir, env)
    except (gomods.Failed, OSError, ValueError) as e:
        return [f"deps: {e}"]
    tracked = sorted({path for path in listed.split("\0") if path})
    try:
        entries = deps.load_allowlist(rows.read_text(deps.ALLOWLIST))
    except (OSError, ValueError) as e:  # the `built` mark's faults already name the row (design §2.6)
        return [str(e) if str(e).startswith("deps: ") else f"deps: {e}"]
    rows.covered(3)
    return failures + deps.compare(modules, graph, entries, tracked, read_license, ctx)


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    env = {**os.environ, **ENV}
    # The tidy and go.sum lines last, after every other check has reported; none hides another (review A-S9).
    return rows.finish(checks(row_dir, env) + tidy_lines(row_dir, env) + gosum_lines(row_dir, env))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
