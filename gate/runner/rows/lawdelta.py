"""The lawdelta row (ADR-0015 §1.3 as annotated at S1.1c-1; design §2.4): the LAW Δ block.

It reads gate/law.json, checks that git tracks something under every listed path, diffs the law paths between
the merge base with main and HEAD, and writes the block to law.txt in its row directory, where the scheduler
carries it into the record and the summary prints it first. A delta of any size is PASS: the row is red only when
it cannot read — a manifest fault, an untracked listed path, no merge base (owner, 2026-11-10, decision 9: never
the silent pass of the apidiff row), a parser crash. On main's own run the block says so (owner, 2026-11-11,
plan check 3). Uncommitted law edits are named on a last line (owner, 2026-11-11, design 2).
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import git, law, records, rows

LAW = os.path.join("gate", "law.json")
NO_BASE = "lawdelta: no merge base with main (fetch-depth 0 or a local main is needed)"
HEAD_IS_MAIN = "LAW Δ: HEAD is main — read the PR run"
# A parser crash is a named failure, never a pass; `law.delta` itself never raises, so this is the defensive net.
PARSER_CRASHES = (ArithmeticError, AssertionError, AttributeError, LookupError, RuntimeError, TypeError, ValueError)


def _block(manifest: law.Manifest, base: str, head: str) -> tuple[list[str], str]:
    """(the block's lines, "") or ([], the failure text)."""
    paths = [e.path for e in manifest.paths]
    data: list[str] = []
    grouped: dict[str, tuple[int, int, int]] = {}
    for file, added, removed in git.numstat(base, head, paths):
        entry = law.entry_for(manifest, file)
        if entry is None:
            return [], f"lawdelta: {file} changed under a listed prefix but matches no entry"
        if entry.kind in law.GROUPED:
            files, a, r = grouped.get(entry.path, (0, 0, 0))
            grouped[entry.path] = (files + 1, a + added, r + removed)
            continue
        old, new = git.show(base, file), git.show(head, file)
        try:
            one = law.doc_line(old, new, entry.flag) if entry.kind == "doc" else law.delta(entry.kind, old, new)
        except PARSER_CRASHES as e:
            return [], f"lawdelta: the {entry.kind} parser crashed on {file}: {type(e).__name__}: {e}"
        data.append(law.line(file, added, removed, one))
    ordered = {e.path: grouped[e.path] for e in manifest.paths if e.path in grouped}
    return law.block(base[:7], data, ordered), ""


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    try:
        manifest = law.load_manifest(rows.read_text(LAW))
    except law.ManifestError as e:
        return rows.finish([f"lawdelta: {e}"])
    except OSError as e:
        return rows.finish([f"lawdelta: gate/law.json: {e}"])
    faults: list[str] = []
    for e in manifest.paths:
        tracked = git.ls_files([e.path])
        if not tracked:
            faults.append(f"lawdelta: {e.path} is listed in gate/law.json but git tracks nothing there")
        elif not e.path.endswith("/") and tracked != [e.path]:  # a directory listed without its slash (break-it 2)
            faults.append(
                f"lawdelta: {e.path} is listed in gate/law.json but git tracks nothing there"
                f" (it is a directory: list it as {e.path}/)"
            )
    if faults:
        return rows.finish(faults)
    rows.covered(1)  # the manifest is the row's scope, counted once it is read and every path is live
    base = git.merge_base()
    if not base:
        return rows.finish([NO_BASE])
    head = git.head()
    if base == head:
        lines = [HEAD_IS_MAIN]
    else:
        lines, failure = _block(manifest, base, head)
        if failure:
            return rows.finish([failure])
    dirty = git.dirty([e.path for e in manifest.paths])
    if dirty:
        lines.append(f"LAW Δ: {dirty} law files have uncommitted edits (not shown)")
    records.write_atomic(os.path.join(row_dir, "law.txt"), ("\n".join(lines) + "\n").encode("utf-8"))
    for text in lines:
        print(text)
    return rows.finish([])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
