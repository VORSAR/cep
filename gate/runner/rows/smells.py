"""The smells row (S1.1c-5 design note §2.3, §2.4, §2.7, §3.1): golangci-lint over the tree under linux and darwin, the
config pinned whole by gate/runner/golangci.py, the size family's exclusions from tools/allowlists/smells.json.

Per platform one environment (the go rows' offline settings, GOOS and GOARCH, a named lint cache outside the
repository) serves both `go list -e -json ./...`, which gives the file map and the covered count, and
`golangci-lint run`, whose JSON report is read from the row's directory after the stale one is removed. The exit code
must agree with the report; the report's error, warnings and enabled linters are checked; a finding at a file no
package claims is red. The two runs merge by (file, line, column, linter, text); a generated.json target, a
`[fmtprint]` finding in a package-main tool and a live size exclusion drop their findings; any `unsafe` import is red;
on a full-tree run an exclusion that dropped nothing is red. At T0 only the staged files' packages are linted, with
the same verdict and the liveness check off.
"""

import os
import posixpath
import subprocess
import sys
from datetime import date

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import allowlists, golangci, jsonstream, rows
from gate.runner.rows import gomods
from gate.toolchain import manifest

PLATFORMS = (("linux", "amd64"), ("darwin", "arm64"))
STAGED = "T0"
SMELLS_LIST = "tools/allowlists/smells.json"
GENERATED_LIST = "tools/allowlists/generated.json"
CACHE = "golangci-lint-cache"
FMTPRINT = "[fmtprint]"
UNSAFE = "imports unsafe (banned everywhere, ADR-0015 §2 smells; //go:linkname needs it)"
IMPORT_LISTS = ("Imports", "TestImports", "XTestImports")
TEMP_ONLY = "a size exclusion is TEMP, with an expiry and a known issue"
Key = tuple[str, int, int, str, str]


class Platform:
    """One GOOS's go list facts and lint report."""

    def __init__(self, goos: str) -> None:
        self.goos = goos
        self.files: dict[str, golangci.GoFile] = {}
        self.ignored: set[str] = set()
        self.unsafe: set[str] = set()
        self.report: golangci.Report | None = None


def run(argv: list[str], env: dict[str, str], cwd: str, out: str, err: str) -> int:
    with open(out, "wb") as stdout, open(err, "wb") as stderr:
        return subprocess.run(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr).returncode


def first_line(path: str) -> str:
    lines = rows.read_text(path).strip().splitlines()
    return lines[0] if lines else ""


def git_list(argv: list[str], row_dir: str, name: str, root: str) -> list[str]:
    out, err = os.path.join(row_dir, f"{name}.stdout"), os.path.join(row_dir, f"{name}.stderr")
    code = run(["git", *argv], dict(os.environ), root, out, err)
    if code != 0:
        raise OSError(f"git {' '.join(argv)}: exit {code}: {first_line(err)}")
    return sorted(p for p in rows.read_text(out).split("\0") if p)


def go_list(plat: Platform, env: dict[str, str], root: str, row_dir: str) -> list[str]:
    """Step 3: the platform's file map, its build-ignored files and its packages importing unsafe."""
    out = os.path.join(row_dir, f"golist-{plat.goos}.stdout")
    err = os.path.join(row_dir, f"golist-{plat.goos}.stderr")
    code = run(["go", "list", "-e", "-json", "./..."], env, root, out, err)
    if code != 0:
        return [f"go list ({plat.goos}) exit {code}: {first_line(err)}"]
    with open(out, "rb") as f:
        try:
            listing = jsonstream.parse(f.read())
        except ValueError as e:
            return [f"go list ({plat.goos}): {e}"]
    plat.files, faults = golangci.go_files(listing, root)
    for pkg in listing:
        rel_dir = os.path.relpath(os.path.realpath(str(pkg.get("Dir", ""))), root).replace(os.sep, "/")
        for name in pkg.get("IgnoredGoFiles") or []:
            plat.ignored.add(posixpath.normpath(posixpath.join(rel_dir, str(name))))
        if any("unsafe" in (pkg.get(key) or []) for key in IMPORT_LISTS):
            plat.unsafe.add(str(pkg.get("ImportPath", "")))
    return faults


def lint(plat: Platform, env: dict[str, str], root: str, row_dir: str, targets: list[str]) -> list[str]:
    """Steps 4-6: run golangci-lint, check its exit against its report, then the report's health."""
    goos = plat.goos
    path = os.path.join(row_dir, f"smells-{goos}.json")
    try:
        os.unlink(path)
    except FileNotFoundError:
        pass
    except OSError as e:
        return [f"golangci-lint ({goos}): cannot remove {path}: {e.strerror}"]
    err = os.path.join(row_dir, f"smells-{goos}.stderr")
    argv = ["golangci-lint", "run", "--config", golangci.CONFIG, f"--output.json.path={path}", *targets]
    code = run(argv, env, root, os.path.join(row_dir, f"smells-{goos}.stdout"), err)
    if code not in (0, 1):
        return [f"golangci-lint ({goos}) exit {code}: {first_line(err)}"]
    if not os.path.exists(path):
        return [f"golangci-lint ({goos}) wrote no report"]
    try:
        rep = golangci.read_report(path)
    except golangci.ReportError as e:
        return [f"golangci-lint ({goos}): {e}"]
    faults: list[str] = []
    if (code == 0) != (not rep.issues and not rep.error):
        faults.append(f"golangci-lint ({goos}): exit {code} disagrees with {len(rep.issues)} issues")
    if rep.error:
        faults.append(f"golangci-lint ({goos}): {rep.error}")
    faults.extend(f"golangci-lint ({goos}) warns: {text}" for text in rep.warnings)
    want = frozenset(golangci.LINTERS) | {"typecheck"}
    if rep.enabled != want:
        faults.append(f"golangci-lint ({goos}) ran {', '.join(sorted(rep.enabled))} not {', '.join(sorted(want))}")
    plat.report = rep
    return faults


def size_entries(root: str, tier: str, today: date) -> tuple[dict[tuple[str, str], bool], list[str]]:
    """(package dir, rule) -> live, from smells.json (§3.1); a missing list holds no exclusion."""
    try:
        text = rows.read_text(os.path.join(root, SMELLS_LIST))
    except FileNotFoundError:
        return {}, []
    except OSError as e:
        return {}, [f"{SMELLS_LIST}: {e.strerror}"]
    try:
        lst = allowlists.load(text, "smells")
    except ValueError as e:
        return {}, [str(e)]
    entries: dict[tuple[str, str], bool] = {}
    faults: list[str] = []
    for entry in lst.entries:
        parts = entry.target.split(" ")
        if len(parts) != 2 or not all(parts) or parts[1] not in golangci.SIZE_RULES:
            faults.append(f'smells.json: {entry.target}: not "<package dir> <rule>"')
            continue
        if entry.kind != "TEMP":
            faults.append(f"smells.json: {entry.target}: {TEMP_ONLY}")
            continue
        try:
            expired = date.fromisoformat(entry.expiry) < today
        except ValueError:
            expired = False  # a malformed expiry is the grammar's fault, the suppressions row's (allowlists.check)
        entries[(parts[0], parts[1])] = not (expired and tier in allowlists.RED_EXPIRED)
    return entries, faults


def generated_targets(root: str) -> tuple[frozenset[str], list[str]]:
    try:
        lst = allowlists.load(rows.read_text(os.path.join(root, GENERATED_LIST)), "generated")
    except FileNotFoundError:
        return frozenset(), []
    except (OSError, ValueError) as e:
        return frozenset(), [str(e)]
    return frozenset(e.target for e in lst.entries), []


def show(issue: golangci.Issue) -> str:
    rule = "" if issue.rule == issue.linter else f"/{issue.rule}"
    return f"{issue.file}:{issue.line}:{issue.column}: {issue.linter}{rule}: {issue.text}"


def key(issue: golangci.Issue) -> Key:
    return (issue.file, issue.line, issue.column, issue.linter, issue.text)


def targets_of(files: list[str]) -> list[str]:
    """T0 (§2.7): each staged file's package once, `./<dir>` (`.` for the root), never `/...`; testdata adds none."""
    dirs = {posixpath.dirname(f) for f in files if not golangci.is_testdata(f)}
    return sorted("." if d == "" else f"./{d}" for d in dirs)


def dropped_by(issue: golangci.Issue, files: dict[str, golangci.GoFile], generated: frozenset[str]) -> bool:
    """Step 8 (a) and (b): a generated.json target; a [fmtprint] finding in a package-main tool."""
    if issue.file in generated:
        return True
    gofile = files.get(issue.file)
    return (
        issue.linter == "forbidigo"
        and FMTPRINT in issue.text
        and issue.file.startswith("tools/")
        and gofile is not None
        and gofile.package == "main"
    )


def judge(
    plats: list[Platform], generated: frozenset[str], entries: dict[tuple[str, str], bool]
) -> tuple[list[golangci.Issue], list[str], dict[tuple[str, str], int]]:
    """Steps 7-8: the file map check, the merge and the drops; (kept issues, faults, dropped per entry)."""
    faults: set[str] = set()
    merged: dict[Key, golangci.Issue] = {}
    pinned: set[Key] = set()  # never dropped: a typecheck issue, or one at a file no package claims
    for plat in plats:
        for issue in plat.report.issues if plat.report else ():
            merged.setdefault(key(issue), issue)
            if issue.linter == "typecheck":
                pinned.add(key(issue))
            if issue.file not in plat.files:
                pinned.add(key(issue))
                faults.add(
                    f"{issue.file}:{issue.line}: {issue.linter}: reported at a file no package claims under"
                    f" {plat.goos} (a //line directive?): {issue.text}"
                )
    files = {f: g for plat in reversed(plats) for f, g in plat.files.items()}
    dropped = dict.fromkeys(entries, 0)
    kept: list[golangci.Issue] = []
    for k in sorted(merged):
        issue = merged[k]
        if k not in pinned:
            if dropped_by(issue, files, generated):
                continue
            entry = (posixpath.dirname(issue.file) or ".", issue.rule)
            if issue.rule in golangci.SIZE_RULES and entries.get(entry):
                dropped[entry] += 1
                continue
        kept.append(issue)
    return kept, sorted(faults), dropped


def config_faults(root: str) -> list[str]:
    """Step 1: the config pinned whole; any fault here stops the run before any lint."""
    try:
        return golangci.differences(golangci.load(rows.read_text(os.path.join(root, golangci.CONFIG))))
    except OSError as e:
        return [f"{golangci.CONFIG}: {e.strerror}"]
    except golangci.ConfigError as e:
        return [str(e)]


def summary(dropped: dict[tuple[str, str], int], data: int) -> str:
    used = sorted((e, n) for e, n in dropped.items() if n)
    detail = f" ({', '.join(f'{pkg} {rule} {n}' for (pkg, rule), n in used)})" if used else ""
    return (
        f"smells: {sum(n for _, n in used)} issues dropped by {len(used)} exclusions{detail};"
        f" {data} testdata files counted as data; linux and darwin"
    )


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    tier = os.environ["GATE_TIER"]
    root = os.path.realpath(os.getcwd())
    staged = sorted(rows.show(p) for p in rows.read_list(argv[1])) if tier == STAGED else []

    faults = config_faults(root)
    if faults:
        rows.covered(0)
        return rows.finish([f"smells: {line}" for line in faults])
    try:
        if tier == STAGED:
            tracked = staged
        else:
            tracked = git_list(["ls-files", "-z", "--", "*.go", ":(exclude)gate/proofs/**"], row_dir, "ls-go", root)
        mods = git_list(["ls-files", "-z", "--", "go.mod", ":(glob)**/go.mod"], row_dir, "ls-gomod", root)
    except OSError as e:
        rows.covered(0)
        return rows.finish([f"smells: {e}"])

    cache = os.path.join(manifest.cache_root(os.environ), CACHE)
    os.makedirs(cache, exist_ok=True)
    targets = targets_of(staged) if tier == STAGED else ["./..."]
    plats = [Platform(goos) for goos, _ in PLATFORMS]
    run_faults: list[str] = []  # steps 1-6: any one and the run proves nothing about an exclusion's liveness
    for plat, (goos, goarch) in zip(plats, PLATFORMS, strict=True):
        env = {**os.environ, **gomods.ENV, "GOOS": goos, "GOARCH": goarch, "GOLANGCI_LINT_CACHE": cache}
        run_faults += go_list(plat, env, root, row_dir)
        if targets:
            run_faults += lint(plat, env, root, row_dir, targets)

    ignored_both = frozenset(plats[0].ignored & plats[1].ignored)
    count, data, cover_faults = golangci.covered(tracked, plats[0].files, plats[1].files, ignored_both, mods)
    entries, list_faults = size_entries(root, tier, rows.today())
    generated, gen_faults = generated_targets(root)
    faults = sorted(set(run_faults)) + cover_faults + list_faults + gen_faults

    kept, map_faults, dropped = judge(plats, generated, entries)
    faults += map_faults + [show(issue) for issue in kept]  # kept is in (file, line, column, …) order
    lines: list[str] = []
    if tier == STAGED:
        lines.append("smells: exclusions not judged at T0 (staged packages only)")
    elif not run_faults:
        faults += [
            f"{SMELLS_LIST}: {pkg} {rule} waives nothing: the package is clean for {rule}; remove it"
            for (pkg, rule), n in sorted(dropped.items())
            if n == 0 and entries[(pkg, rule)]
        ]
    faults += sorted(f"{path}: {UNSAFE}" for path in plats[0].unsafe | plats[1].unsafe)
    lines.append(summary(dropped, data))
    rows.covered(count)
    for line in lines:
        print(line)
    return rows.finish([f"smells: {fault}" for fault in faults])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
