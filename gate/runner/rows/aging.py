"""The aging row (S1.1c-3 design §3.4): a known issue past its clock is red, naming its id. The rule itself is
gate/runner/aging.py.

It reads the files its expected_scope selects (the runner's files.nul: gate/known-issues.json, strictly through
`knownissues._entries`; gate/sprints.json; gate/current-sprint; docs/sprints/decisions.md) and, through a reader of
tracked Markdown only, the section a decision names; the day is `rows.today()` (UTC). GATE_COVERED is the number of
those files it read — the count the runner expects (`verdict.py`: covered must equal expected) — so a list emptied of
entries stays green. A side input that is missing or unreadable is one named line.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import aging, knownissues, rows, sprintid

SIDE_FAULTS = (OSError, ValueError)


def tracked_markdown(row_dir: str) -> set[str]:
    """The Markdown git tracks: a decision resolves only into a tracked file (as the known-issues pin reads it)."""
    out, err = os.path.join(row_dir, "ls-files.stdout"), os.path.join(row_dir, "ls-files.stderr")
    code = rows.run_tool(["git", "ls-files", "-z", "--", "*.md"], out, err)
    if code != 0:
        raise OSError(f"git ls-files: exit {code}: {rows.read_text(err).strip()}")
    return {p for p in rows.read_text(out).split("\0") if p}


def _listed(texts: dict[str, str], path: str) -> str:
    if path not in texts:
        raise ValueError(f"{path} is not among the row's files (tracked, in its expected_scope)")
    return texts[path]


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    listed = sorted(rows.show(p) for p in rows.read_list(os.path.join(row_dir, "files.nul")))
    texts: dict[str, str] = {}
    faults = []
    for path in listed:
        try:
            texts[path] = rows.read_text(path)
        except OSError as e:
            faults.append(f"aging: {path}: listed but not readable: {e}")
    rows.covered(len(texts))
    if faults:
        return rows.finish(faults)
    try:
        data = _listed(texts, knownissues.JSON)
        try:
            entries = knownissues._entries(data)
        except ValueError as e:
            raise ValueError(f"{knownissues.JSON}: {e}") from None
        order = sprintid.load_order(_listed(texts, sprintid.SPRINTS))
        marker = sprintid.canonical(sprintid.read_marker(_listed(texts, sprintid.MARKER)))
        tracked = tracked_markdown(row_dir)
    except SIDE_FAULTS as e:
        return rows.finish([f"aging: {e}"])

    def read(path: str) -> str | None:
        if path not in tracked or not path.endswith(".md"):
            return None  # an untracked local doc never decides an entry
        try:
            return texts[path] if path in texts else rows.read_text(path)
        except OSError:
            return None

    return rows.finish(aging.aged(entries, order, marker, today=rows.today(), read=read))


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
