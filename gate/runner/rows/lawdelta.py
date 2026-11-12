"""The lawdelta row (ADR-0015 §1.3 as annotated at S1.1c-1; design §2.4): the LAW Δ block.

It reads gate/law.json, checks that git tracks something under every listed path, diffs the law paths between
the merge base with main and HEAD, and writes the block to law.txt in its row directory, where the scheduler
carries it into the record and the summary prints it first. A delta of any size is PASS: the row is red only when
it cannot read — a manifest fault, an untracked listed path, no merge base (owner, 2026-11-10, decision 9: never
the silent pass of the apidiff row), a parser crash. On main's own run the block says so (owner, 2026-11-11,
plan check 3). Uncommitted law edits are named on a last line (owner, 2026-11-11, design 2).

Between them, the exemption section (S1.1c-2 design §2.6): one line counting the in-code exemptions and the list
entries at HEAD and their change since the merge base; each in-code exemption added or removed, named (owner,
2026-11-12, design 1), matched across revisions by what it is, not its line; then the countdown of every TEMP
exemption due within 14 days or past. `today()` is the one date source. A git command outside its contract is a
failure (DA-S3).
"""

import collections
import json
import os
import sys
from datetime import UTC, date, datetime

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import allowlists, git, law, records, rows, suppressions

LAW = os.path.join("gate", "law.json")
NO_BASE = "lawdelta: no merge base with main (fetch-depth 0 or a local main is needed)"
HEAD_IS_MAIN = "LAW Δ: HEAD is main — read the PR run"
# A parser crash is a named failure, never a pass; `law.delta` itself never raises, so this is the defensive net.
PARSER_CRASHES = (ArithmeticError, AssertionError, AttributeError, LookupError, RuntimeError, TypeError, ValueError)
LISTS = "tools/allowlists/"
IN_CODE = (":(glob)**/*.go", ":(glob)gate/**/*.py", ":(glob).githooks/*")  # suppressions.scoped, as pathspecs


def today() -> date:
    """The run's date, the section's one clock (patched in tests, DQ-S9)."""
    return datetime.now(UTC).date()


def in_code(rev: str) -> list[suppressions.Directive]:
    """The directives and markers at the revision (§2.3), the files found by the PREFILTER patterns."""
    found: list[suppressions.Directive] = []
    for path in sorted(git.grep_files(rev, suppressions.PREFILTER, IN_CODE)):
        text = git.show(rev, path)
        if text is not None and suppressions.scoped(path):
            found += suppressions.read(path, text)[0]
    return found


def listed(rev: str) -> list[tuple[str, dict]]:
    """(list path, entry) for every entry of tools/allowlists/*.json at the revision; a list that cannot be read
    holds none here (the suppressions row names it)."""
    entries: list[tuple[str, dict]] = []
    for path in sorted(git.ls_tree(rev, LISTS)):
        if not path.endswith(".json") or "/" in path.removeprefix(LISTS):
            continue
        try:
            data = json.loads(git.show(rev, path) or "")
        except ValueError:
            continue
        items = data.get("entries") if type(data) is dict else None
        entries += [(path, e) for e in items or [] if type(e) is dict]
    return entries


def _identity(d: suppressions.Directive) -> tuple:
    return d.path, d.linters, d.kind, tuple(sorted(d.meta.items()))


def _named(sign: str, d: suppressions.Directive) -> str:
    if d.kind == "MARKER":
        return f"{sign}{d.path}:{d.line} # fmt: skip"
    fields = [f"{k}={d.meta[k]}" for k in ("ref", "expiry", "issue") if d.meta.get(k)]
    words = [f"{sign}{d.path}:{d.line}", f"//nolint:{','.join(d.linters)}", d.kind, d.meta.get("sprint", "")]
    return " ".join([*words, *fields])


def changed(base: list[suppressions.Directive], head: list[suppressions.Directive]) -> list[str]:
    """One sub-line per in-code exemption added, then removed; a moved one is neither."""
    before = collections.Counter(map(_identity, base))
    after = collections.Counter(map(_identity, head))
    added, removed = after - before, before - after
    out = []
    for sign, side, extra in (("+", head, added), ("−", base, removed)):
        for d in sorted(side, key=lambda d: (d.path, d.line)):
            if extra[_identity(d)] > 0:
                extra[_identity(d)] -= 1
                out.append(_named(sign, d))
    return out


def countdown(head_code: list[suppressions.Directive], head_lists: list[tuple[str, dict]], day: date) -> list[str]:
    """Every TEMP list entry and TEMP directive at HEAD due within 14 days, or past (DA-S2, design §10.17, §10.19)."""
    items = [
        (f"{path}: {e.get('target', '')}", str(e.get("expiry", "")))
        for path, e in head_lists
        if e.get("kind") == "TEMP"
    ]
    items += [
        (f"{d.path}:{d.line}: {','.join(d.linters)}", d.meta.get("expiry", ""))
        for d in sorted(head_code, key=lambda d: (d.path, d.line))
        if d.kind == "TEMP"
    ]
    expiry = dict(items)
    return [
        f"  exemption {label} expires {expiry[label]} (in {k} days)"
        if k >= 0
        else f"  exemption {label} expired {expiry[label]} (red at T4 and nightly)"
        for label, k in allowlists.expiring(items, day)
    ]


def exemptions(base: str, head: str) -> list[str]:
    """The section: `exemptions: <n> in code, <m> listed (<±d> this diff)`, the named changes, the countdown."""
    head_code, head_lists = in_code(head), listed(head)
    base_code, base_lists = (head_code, head_lists) if base == head else (in_code(base), listed(base))
    d = len(head_code) + len(head_lists) - len(base_code) - len(base_lists)
    lines = [
        f"exemptions: {len(head_code)} in code, {len(head_lists)} listed ({'+' if d >= 0 else '−'}{abs(d)} this diff)"
    ]
    lines += law.capped(changed(base_code, head_code))
    return lines + countdown(head_code, head_lists, today())


def _block(manifest: law.Manifest, base: str, head: str) -> tuple[list[str], str]:
    """(the block's lines, "") or ([], the failure text)."""
    paths = [e.path for e in manifest.paths]
    data: list[str | list[str]] = []
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
        if isinstance(one, list):  # a list kind: [summary, *sub-lines] (design §2.5)
            data.append([law.line(file, added, removed, one[0]), *one[1:]])
        else:
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
    try:
        lines += exemptions(base, head)
    except git.GitError as e:
        return rows.finish([f"lawdelta: {e}"])
    dirty = git.dirty([e.path for e in manifest.paths])
    if dirty:
        lines.append(f"LAW Δ: {dirty} law files have uncommitted edits (not shown)")
    records.write_atomic(os.path.join(row_dir, "law.txt"), ("\n".join(lines) + "\n").encode("utf-8"))
    for text in lines:
        print(text)
    return rows.finish([])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
