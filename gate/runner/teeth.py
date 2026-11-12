"""The teeth, pure (S1.1c-2 design §4.2, §11; ADR-0015 §3.8): what the instrument pin counts, and the pin itself.

`python_cases` reads a proof driver's source by `ast` (it never imports it). Its cases are the `test*` methods of its
classes, named `<Class>.<method>`. The strength rule: a case must reach, in live code — not after a `return` or
`raise` in its block, not under `if False:`, not in a nested def the live code never calls — a red helper (a call on
`self` to a name in RED, or `assertRow` with `ok` the literal False) or a pass on the row under test (`assertRow`
with `ok` the literal True on a literal row id other than "parity"). `assertParityPassed` alone never counts. A
call on `self` to a name in RED or VERDICT counts as that helper wherever it is defined; any other helper of the
driver's own module is followed, up to two levels (§11.3). A driver needs one red case; zero cases is a fault.

`go_cases` parses archcheck's `-cases` listing (§11's seam: `<path>: case <name>` and `<path>: fault <why>`) into
(cases, faults) per path. `pin` holds the instrument pin's six checks over the manifest, the registry rows, the
tracked files, what each proof file counts, and the targets of tools/allowlists/teeth.json; the pin test does the
I/O.
"""

import ast
import posixpath
import re
from collections.abc import Collection, Iterable, Mapping, Sequence

from gate.runner import law, registry

RED = frozenset(
    {
        "assertRed",
        "assertGateFailed",
        "assertParityStops",
        "assertRefusedAtLoad",
        "assertHookRefused",
        "assertRegenRefused",
    }
)
VERDICT = frozenset({"assertRow"})  # red or a pass on its row by its `ok` literal and its row id
PARITY_ONLY = "assertParityPassed"
LEVELS = 2
CASE_PREFIX = "test"
LISTING = re.compile(r"^(?P<path>.*?): (?P<what>case|fault) (?P<text>.*)$")

Function = ast.FunctionDef | ast.AsyncFunctionDef


# ------------------------------------------------------------------------------------------------------ python_cases


def _blocks(stmt: ast.stmt) -> list[list]:
    blocks = []
    for field in ("body", "orelse", "finalbody", "handlers", "cases"):
        value = getattr(stmt, field, None)
        if isinstance(value, list) and value:
            blocks.append(value)
    return blocks


def _calls(node: ast.AST) -> list[ast.Call]:
    """The calls in one statement's own expressions (its nested blocks and lambdas excluded)."""
    out = []
    for child in ast.iter_child_nodes(node):
        if isinstance(child, (ast.stmt, ast.excepthandler, ast.match_case, ast.Lambda)):
            continue
        stack = [child]
        while stack:
            n = stack.pop()
            if isinstance(n, ast.Lambda):
                continue
            if isinstance(n, ast.Call):
                out.append(n)
            stack.extend(ast.iter_child_nodes(n))
    return out


def _live(body: Sequence[ast.AST], nested: dict[str, Function]) -> list[ast.Call]:
    """The calls a block's live statements make, the nested defs they call by name inlined."""
    out: list[ast.Call] = []
    for stmt in body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            nested[stmt.name] = stmt
            continue
        if isinstance(stmt, ast.ClassDef):
            continue
        if isinstance(stmt, ast.If) and isinstance(stmt.test, ast.Constant) and stmt.test.value is False:
            out += _live(stmt.orelse, nested)
            continue
        out += _calls(stmt)
        for block in _blocks(stmt):
            for item in block:
                if isinstance(item, (ast.excepthandler, ast.match_case)):
                    out += _calls(item)
                    out += _live(item.body, nested)
            if not isinstance(block[0], (ast.excepthandler, ast.match_case)):
                out += _live(block, nested)
        if isinstance(stmt, (ast.Return, ast.Raise)):
            break
    expanded = list(out)
    for call in out:
        if isinstance(call.func, ast.Name) and call.func.id in nested:
            expanded += _live(nested.pop(call.func.id).body, nested)
    return expanded


def _literal(call: ast.Call, position: int, keyword: str) -> object:
    if len(call.args) > position and isinstance(call.args[position], ast.Constant):
        return call.args[position].value
    for kw in call.keywords:
        if kw.arg == keyword and isinstance(kw.value, ast.Constant):
            return kw.value.value
    return None


def _on_self(call: ast.Call) -> str:
    f = call.func
    if isinstance(f, ast.Attribute) and isinstance(f.value, ast.Name) and f.value.id == "self":
        return f.attr
    return ""


class _Scope:
    def __init__(self, methods: Mapping[str, Function], functions: Mapping[str, Function]) -> None:
        self.methods, self.functions = methods, functions

    def resolve(self, call: ast.Call) -> Function | None:
        name = _on_self(call)
        if name:
            return self.methods.get(name)
        if isinstance(call.func, ast.Name):
            return self.functions.get(call.func.id)
        return None


def _reaches(fn: Function, scope: _Scope, level: int, seen: tuple[Function, ...]) -> tuple[bool, bool]:
    """(a red helper, a pass on the row under test) within LEVELS helper levels of fn's live code."""
    red = row_pass = False
    for call in _live(fn.body, {}):
        name = _on_self(call)
        if name in RED:
            red = True
            continue
        if name in VERDICT:
            ok, row = _literal(call, 3, "ok"), _literal(call, 2, "row_id")
            if ok is False:
                red = True
            elif ok is True and isinstance(row, str) and row != "parity":
                row_pass = True
            continue
        if name == PARITY_ONLY:
            continue
        helper = scope.resolve(call)
        if helper is not None and level < LEVELS and helper not in seen:
            r, p = _reaches(helper, scope, level + 1, (*seen, helper))
            red, row_pass = red or r, row_pass or p
    return red, row_pass


def _methods(cls: ast.ClassDef, classes: Mapping[str, ast.ClassDef]) -> dict[str, Function]:
    """The class's methods over its bases defined in the same module."""
    out: dict[str, Function] = {}
    for base in cls.bases:
        if isinstance(base, ast.Name) and base.id in classes and classes[base.id] is not cls:
            out.update(_methods(classes[base.id], classes))
    out.update({n.name: n for n in cls.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))})
    return out


def python_cases(source: str, path: str = "<source>") -> tuple[list[str], list[str]]:
    """A driver's cases (`<Class>.<method>`) and its faults by the strength rule."""
    tree = ast.parse(source)
    classes = {n.name: n for n in tree.body if isinstance(n, ast.ClassDef)}
    functions = {n.name: n for n in tree.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))}
    cases: list[str] = []
    faults: list[str] = []
    red_cases = 0
    for cls in classes.values():
        methods = _methods(cls, classes)
        scope = _Scope(methods, functions)
        for node in cls.body:
            if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) or not node.name.startswith(CASE_PREFIX):
                continue
            name = f"{cls.name}.{node.name}"
            cases.append(name)
            red, row_pass = _reaches(node, scope, 0, (node,))
            if not (red or row_pass):
                faults.append(f"teeth: {path}::{name} asserts no gate verdict")
            red_cases += red
    if not cases:
        faults.append(f"teeth: {path} counts 0 cases")
    elif not red_cases:
        faults.append(f"teeth: {path} plants nothing red")
    return cases, faults


# ---------------------------------------------------------------------------------------------------------- go_cases


def go_cases(listing: str) -> dict[str, tuple[list[str], list[str]]]:
    """archcheck -cases' listing grouped by path, in its order: path → (cases, faults); ValueError on a stray line."""
    out: dict[str, tuple[list[str], list[str]]] = {}
    for line in listing.splitlines():
        if not line:
            continue
        m = LISTING.match(line)
        if m is None:
            raise ValueError(f"archcheck -cases: an unreadable line {line!r}")
        cases, faults = out.setdefault(m["path"], ([], []))
        (cases if m["what"] == "case" else faults).append(m["text"])
    return out


# --------------------------------------------------------------------------------------------------------------- pin


def _go_count(cases: Iterable[str], test: str) -> int:
    return sum(1 for c in cases if c == test or c.startswith(test + "/"))


def _law_fault(manifest: law.Manifest, tracked: Collection[str], path: str) -> str:
    if path not in tracked:
        return f"{path} is not tracked"
    if path.endswith("_test.go"):
        if not any(entry.path == path for entry in manifest.paths):
            return f"{path} is not an exact gate/law.json entry"
    elif law.entry_for(manifest, path) is None:
        return f"{path} is not a law path"
    return ""


ROW_FIELDS = ("tiers", "lands_in", "min_wants", "expected_scope")


def pin(
    manifest: law.Manifest,
    rows: Sequence[registry.Row],
    tracked: Collection[str],
    counted: Mapping[str, tuple[Sequence[str], Sequence[str]]],
    teeth_targets: Collection[str],
) -> list[str]:
    """The instrument pin's faults, each naming the row or instrument and the field; [] when the teeth hold."""
    faults: list[str] = []
    by_row = {row.id: row for row in rows}
    row_instruments: dict[str, list[law.Instrument]] = {}
    for inst in manifest.instruments:
        if not inst.go_test:
            row_instruments.setdefault(inst.row, []).append(inst)
    # 1. registry <-> row instruments, both ways
    for row_id in sorted(set(by_row) - set(row_instruments)):
        faults.append(f"teeth: row {row_id} has no instrument in gate/law.json")
    for row_id, insts in sorted(row_instruments.items()):
        if len(insts) > 1:
            faults.append(f"teeth: row {row_id} has {len(insts)} instruments: {', '.join(i.id for i in insts)}")
    for inst in manifest.instruments:
        where = f"teeth: instrument {inst.id}"
        row = by_row.get(inst.row)
        if row is None:
            faults.append(f"{where}: row {inst.row} is not in gate/registry.json")
        elif not inst.go_test:
            # 2. the row pinned: blocking, tiers, lands_in, min_wants, expected_scope
            if not row.blocking:
                faults.append(f"{where}: row {row.id} is not blocking")
            for field in ROW_FIELDS:
                want, got = getattr(inst, field), getattr(row, field)
                if want != got:
                    faults.append(f"{where}: {field} {_show(want)}, the registry row has {_show(got)}")
        # 3. a go_test is listed by -cases in its package
        if inst.go_test:
            package, _, test = inst.go_test.rpartition(".")
            listed = any(
                posixpath.dirname(p.path) == package and test in counted.get(p.path, ((), ()))[0] for p in inst.proofs
            )
            if not listed:
                faults.append(f"{where}: go_test {inst.go_test} is not listed by archcheck -cases")
        # 4. code and proofs are tracked law paths
        for path in (*inst.code, *(p.path for p in inst.proofs)):
            why = _law_fault(manifest, tracked, path)
            if why:
                faults.append(f"{where}: {why}")
        # 5. cases counted
        for proof in inst.proofs:
            if proof.path not in counted:
                faults.append(f"{where}: proof {proof.path} was not counted")
                continue
            cases, proof_faults = counted[proof.path]
            faults += [f"{where}: {fault}" for fault in proof_faults]
            n = _go_count(cases, proof.test) if proof.test else len(cases)
            if n != proof.cases:
                faults.append(f"{where}: proof {proof.path} lists {proof.cases} cases, the teeth count {n}")
        # 6. exempt names a teeth.json entry
        if inst.exempt and inst.exempt not in teeth_targets:
            faults.append(f"{where}: exempt {inst.exempt} is not a target of tools/allowlists/teeth.json")
        if not inst.proofs and not inst.exempt:
            faults.append(f"{where}: no proofs and no exempt")
    return faults


def _show(value: object) -> str:
    return ", ".join(value) if isinstance(value, tuple) else str(value)
