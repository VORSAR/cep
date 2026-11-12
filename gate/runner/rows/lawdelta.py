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
failure (DA-S3). Every waiver the tools honour counts, valid or not, and one whose metadata changed in place is `~`,
never `+` (W1-F3); a file or list that cannot be read at a revision is said on the line (CA-L1).
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


def in_code(rev: str) -> tuple[list[suppressions.Directive], list[str]]:
    """The waivers at the revision (§2.3), valid or not (W1-F3), the files found by the PREFILTER patterns; and the
    files that could not be read there, said on the line (CA-L1)."""
    found: list[suppressions.Directive] = []
    unreadable: list[str] = []
    for path in sorted(git.grep_files(rev, suppressions.PREFILTER, IN_CODE)):
        if not suppressions.scoped(path):
            continue
        text = git.show(rev, path)
        if text is None:
            unreadable.append(path)
            continue
        directives, faults = suppressions.read(path, text)
        if any(f.startswith(f"{path}: {suppressions.UNTOKENIZABLE}") for f in faults):
            unreadable.append(path)
        found += directives
    return found, unreadable


def listed(rev: str) -> tuple[list[tuple[str, dict]], list[str]]:
    """(list path, entry) for every entry of tools/allowlists/*.json at the revision; and the lists that cannot be
    read there, said on the line (CA-L1; the suppressions row names the fault)."""
    entries: list[tuple[str, dict]] = []
    unreadable: list[str] = []
    for path in sorted(git.ls_tree(rev, LISTS)):
        if not path.endswith(".json") or "/" in path.removeprefix(LISTS):
            continue
        try:
            data = json.loads(git.show(rev, path) or "")
        except ValueError:
            data = None
        items = data.get("entries") if type(data) is dict else None
        if type(items) is not list:
            unreadable.append(path)
            continue
        entries += [(path, e) for e in items if type(e) is dict]
    return entries, unreadable


def _identity(d: suppressions.Directive) -> tuple:
    return d.path, d.linters, d.kind, tuple(sorted(d.meta.items()))


def _named(sign: str, d: suppressions.Directive) -> str:
    if d.kind == "MARKER":
        return f"{sign}{d.path}:{d.line} # fmt: skip"
    if d.kind == "REGION":
        return f"{sign}{d.path}:{d.line} # {d.linters[0]}: {d.meta.get('command', '')}"
    fields = [f"{k}={d.meta[k]}" for k in ("ref", "expiry", "issue") if d.meta.get(k)]
    words = [f"{sign}{d.path}:{d.line}", f"//nolint:{','.join(d.linters)}", d.kind, d.meta.get("sprint", "")]
    return " ".join(w for w in [*words, *fields] if w)


META_ORDER = ("ref", "expiry", "issue", "sprint")


def _what_changed(old: suppressions.Directive, new: suppressions.Directive) -> str:
    """What changed in place, as the list kinds say it: `<key> <old> → <new>` each, then `reason changed`."""
    keys = [*META_ORDER, *sorted((set(old.meta) | set(new.meta)) - {*META_ORDER, "reason"})]
    said = [f"{k} {old.meta.get(k, '')} → {new.meta.get(k, '')}" for k in keys if old.meta.get(k) != new.meta.get(k)]
    if old.meta.get("reason") != new.meta.get("reason"):
        said.append("reason changed")
    return "; ".join(said)


def changed(base: list[suppressions.Directive], head: list[suppressions.Directive]) -> list[str]:
    """One sub-line per in-code exemption added, changed in place, then removed; a moved one is none of these. An
    added and a removed one at the same (path, linters, kind) is one whose metadata changed: `~`, never `+` (W1-F3)."""
    before = collections.Counter(map(_identity, base))
    after = collections.Counter(map(_identity, head))
    extra = {"+": after - before, "−": before - after}
    sides: dict[str, list[suppressions.Directive]] = {"+": [], "−": []}
    for sign, side in (("+", head), ("−", base)):
        for d in sorted(side, key=lambda d: (d.path, d.line)):
            if extra[sign][_identity(d)] > 0:
                extra[sign][_identity(d)] -= 1
                sides[sign].append(d)
    added, edited = [], []
    for d in sides["+"]:
        old = next((o for o in sides["−"] if (o.path, o.linters, o.kind) == (d.path, d.linters, d.kind)), None)
        if old is None:
            added.append(_named("+", d))
            continue
        sides["−"].remove(old)
        edited.append(f"{_named('~', d)}: {_what_changed(old, d)}")
    return [*added, *edited, *(_named("−", d) for d in sides["−"])]


def _unreadable(files: set[str], lists: set[str]) -> str:
    """` (<n> file(s) unreadable, <m> list(s) unreadable)` when either revision had one; else "" (CA-L1)."""
    parts = [
        f"{len(found)} {noun}{'' if len(found) == 1 else 's'} unreadable"
        for noun, found in (("file", files), ("list", lists))
        if found
    ]
    return f" ({', '.join(parts)})" if parts else ""


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
    """The section: `exemptions: <n> in code, <m> listed (<±d> this diff)`, then any file or list unreadable at
    either revision (CA-L1), the named changes, the countdown."""
    (head_code, bad_files), (head_lists, bad_lists) = in_code(head), listed(head)
    if base == head:
        base_code, base_lists = head_code, head_lists
    else:
        (base_code, base_bad_files), (base_lists, base_bad_lists) = in_code(base), listed(base)
        bad_files, bad_lists = [*bad_files, *base_bad_files], [*bad_lists, *base_bad_lists]
    d = len(head_code) + len(head_lists) - len(base_code) - len(base_lists)
    sign = "+" if d >= 0 else "−"
    said = _unreadable(set(bad_files), set(bad_lists))
    lines = [f"exemptions: {len(head_code)} in code, {len(head_lists)} listed ({sign}{abs(d)} this diff){said}"]
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
