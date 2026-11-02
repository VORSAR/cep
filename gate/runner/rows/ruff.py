"""The ruff row: format, then lint, the gate's own Python over files.nul in batches (design note §5).

Per batch: `ruff format --check`, then `ruff check --ignore-noqa`, each with `--no-cache --config ruff.toml`, so
ruff.toml's per-file-ignores are the only waivers and nothing lands in the worktree. The explicit file list keeps
ruff from finding untracked files (re-check S6). `--` comes before the list: a top-level `-x.py` is a file.

GATE_COVERED is the sum over batches of the files in `ruff format --check`'s summary line, in ruff 0.17.3's exact
forms (re-checked). Any other summary is a failure: a count we cannot read is not a count. `ruff check`'s own
count is residue (QA DQ22). Ruff's output is echoed under a prefix, so the row's stdout names every finding but
no line of it can pass for the row's own GATE_COVERED.
"""

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import rows

BATCH = 256
FLAGS = ("--no-cache", "--config", "ruff.toml")
ECHO = "ruff> "
# Bytes patterns, so \d is ASCII only.
_BOTH = re.compile(rb"(?:(\d+) files? would be reformatted, )?(\d+) files? already formatted")
_REFORMAT = re.compile(rb"(\d+) files? would be reformatted")
_LOCATION = re.compile(rb" ?--> (.+):\d+:\d+")


def summary_count(line: bytes) -> int | None:
    """The files a summary line accounts for, or None if it is not one of ruff 0.17.3's forms."""
    match = _BOTH.fullmatch(line)
    if match is not None:
        return int(match.group(1) or b"0") + int(match.group(2))
    match = _REFORMAT.fullmatch(line)
    return None if match is None else int(match.group(1))


def _named(output: bytes) -> list[str]:
    """The files ruff's diagnostics point at."""
    return sorted({rows.show(m.group(1)) for line in output.split(b"\n") if (m := _LOCATION.fullmatch(line))})


def _run(name: str, argv: list[str | bytes], row_dir: str, index: int) -> tuple[int, bytes]:
    out = os.path.join(row_dir, f"{name}.{index}.stdout")
    code = rows.run_tool(argv, out, os.path.join(row_dir, f"{name}.{index}.stderr"))
    with open(out, "rb") as f:
        data = f.read()
    for line in data.decode("utf-8", errors="replace").split("\n"):
        if line:
            print(f"{ECHO}{line}")
    return code, data


def _failures(name: str, code: int, output: bytes) -> list[str]:
    if code == 0:
        return []
    named = _named(output)
    return [f"{path}: ruff {name} (exit {code})" for path in named] or [f"ruff {name} exit {code}, naming no file"]


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    files = rows.read_list(argv[1])
    done = 0
    failures: list[str] = []
    for index, start in enumerate(range(0, len(files), BATCH)):
        batch = files[start : start + BATCH]
        code, out = _run("format", ["ruff", "format", "--check", *FLAGS, "--", *batch], row_dir, index)
        lines = [line for line in out.split(b"\n") if line]
        count = summary_count(lines[-1]) if lines else None
        if count is None:
            failures.append(
                f"unrecognised ruff summary in batch {index}: {rows.show(lines[-1]) if lines else '(none)'}"
            )
        else:
            done += count
        failures += _failures("format", code, out)
        code, out = _run("check", ["ruff", "check", "--ignore-noqa", *FLAGS, "--", *batch], row_dir, index)
        failures += _failures("check", code, out)
    rows.covered(done)
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
