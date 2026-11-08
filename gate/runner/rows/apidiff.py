"""The apidiff row (design s3-3-1 §6.4; owner, 2026-11-07, S3.3's plan check decision 3 and design decision 7):
`go tool apidiff` compares each public package against the merge base with main and labels every change compatible
or incompatible. Red when a package has an incompatible change and api/exports.golden did not change against the
merge base (a signature changed without the law changing with it); every report is printed either way.

The tool takes one package per `-w`, and its exit code carries no verdict (0 on an incompatible change): the verdict
is read from its report text. With no merge base (a one-commit clone) the row passes and says so: nothing compared.
Arguments: package directories (".", "./io", …); GATE_COVERED counts the packages compared.
"""

import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

INCOMPATIBLE = "Incompatible changes:"
GOLDEN = os.path.join("api", "exports.golden")


def git(*args: str) -> tuple[int, str]:
    p = subprocess.run(["git", *args], stdin=subprocess.DEVNULL, capture_output=True, text=True)
    return p.returncode, p.stdout.strip()


def merge_base() -> str:
    for ref in ("origin/main", "main"):
        code, out = git("merge-base", "HEAD", ref)
        if code == 0 and out:
            return out
    return ""


def module_path() -> str:
    code, out = git("rev-parse", "--show-toplevel")
    with open(os.path.join(out if code == 0 else ".", "go.mod"), encoding="utf-8") as f:
        for line in f:
            if line.startswith("module "):
                return line.split()[1]
    return ""


def one_line(text: str) -> str:
    """A multi-line report on one line, its lines joined by "; ", for a failure reason."""
    return "; ".join(line.strip() for line in text.splitlines() if line.strip())


def has_package(path: str) -> bool:
    """A directory with at least one non-test Go file: a package the merge base had."""
    try:
        return any(n.endswith(".go") and not n.endswith("_test.go") for n in os.listdir(path))
    except OSError:
        return False


def import_path(module: str, pkg: str) -> str:
    rel = pkg.removeprefix("./").strip("/")
    return module if rel in ("", ".") else f"{module}/{rel}"


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    pkgs = argv[1:]
    if not pkgs:
        print("usage: apidiff.py <package dir>...", file=sys.stderr)
        return 2
    base = merge_base()
    if not base:
        print("apidiff: no merge base with main: nothing compared", flush=True)
        rows.covered(len(pkgs))
        return rows.finish([])
    module = module_path()
    # The tool binary is the current module's (the merge base may predate the `tool` line): `go tool -n` names it.
    p = subprocess.run(["go", "tool", "-n", "apidiff"], stdin=subprocess.DEVNULL, capture_output=True, text=True)
    if p.returncode != 0 or not p.stdout.strip():
        rows.covered(0)
        return rows.finish([f"go tool -n apidiff failed with exit {p.returncode}: {p.stderr.strip()}"])
    apidiff = p.stdout.split()[0]
    worktree = os.path.join(row_dir, "base")
    code, _ = git("worktree", "add", "--detach", worktree, base)
    if code != 0:
        rows.covered(0)
        return rows.finish([f"git worktree add at {base[:12]} failed with exit {code}"])
    try:
        failures: list[str] = []
        compared = 0
        _, changed = git("diff", "--name-only", base, "--", GOLDEN)
        golden_changed = bool(changed)
        for pkg in pkgs:
            ip = import_path(module, pkg)
            name = ip.replace("/", "_")
            export = os.path.join(row_dir, f"{name}.export")
            out, err = os.path.join(row_dir, f"{name}.base.out"), os.path.join(row_dir, f"{name}.base.err")
            if not has_package(os.path.join(worktree, pkg)):
                print(f"apidiff: {ip}: new package (absent at the merge base): compatible", flush=True)
                compared += 1
                continue
            with open(out, "wb") as o, open(err, "wb") as e:
                code = subprocess.run(
                    [apidiff, "-w", export, ip], cwd=worktree, stdin=subprocess.DEVNULL, stdout=o, stderr=e
                ).returncode
            if code != 0:
                failures.append(
                    f"{ip}: apidiff -w at the merge base failed with exit {code}: {rows.read_text(err).strip()}"
                )
                continue
            out = os.path.join(row_dir, f"{name}.out")
            err = os.path.join(row_dir, f"{name}.err")
            code = rows.run_tool([apidiff, export, ip], out, err)
            if code != 0:  # a package that does not build at HEAD: an empty report would read as "(no change)"
                failures.append(f"{ip}: apidiff at HEAD failed with exit {code}: {one_line(rows.read_text(err))}")
                continue
            report = rows.read_text(out)
            compared += 1
            print(f"apidiff: {ip}:\n{report.strip() or '(no change)'}", flush=True)
            if INCOMPATIBLE in report and not golden_changed:
                # the runner keeps a reason's first line only: the whole report goes on it (QA's second lane, F2)
                failures.append(f"incompatible API change in {ip} without {GOLDEN} changing: {one_line(report)}")
        rows.covered(compared)
        return rows.finish(sorted(failures))
    finally:
        git("worktree", "remove", "--force", worktree)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
