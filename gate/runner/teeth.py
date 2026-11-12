"""The teeth, pure (S1.1c-2 design §4.2, §11; ADR-0015 §3.8): what the instrument pin counts, and the pin itself.

`python_cases` reads a proof driver's source by `ast` (it never imports it). Its cases are the `test*` methods of its
classes whose bases reach unittest.TestCase or a support driver base, named `<Class>.<method>`. The strength rule: a
case must reach, in live code, a red helper (a call on `self` to a name in RED, or `assertRow` with `ok` the literal
False) or a pass on a named row other than parity (`assertRow` with `ok` the literal True on a literal row id other
than "parity"). Not live (tightened at S1.1c-2's fix round, BK-B3): a branch a literal test never takes (`if 0:`,
`while False:`), a loop over an empty literal, a statement after one that ends the block on every path, a nested def
the live code never calls; not counted: a call under a `with` that is not a plain resource (`contextlib.suppress`,
`assertRaises`) or under a `try` whose handler is bare or catches AssertionError, Exception or BaseException.
`assertParityPassed` alone never counts. A call on `self` to a name in RED or VERDICT counts as that helper; a
driver may not define or assign such a name, nor reshape what unittest loads (a module-level `load_tests`, `del`,
`setattr`, `__test__`, or any statement but a docstring, an import, an assignment to names, a def and a class:
BK-B4). Any other helper of the driver's own module is followed, up to two levels (§11.3), its parameters bound to
the strings the call passes. A driver needs one red case; zero cases is a fault. `red_rows` names the rows the red
cases land on (BK-S7).

`go_cases` parses archcheck's `-cases` listing (§11's seam: `<path>: case <name>` and `<path>: fault <why>`) into
(cases, faults) per path. `pin` holds the instrument pin's seven checks over the manifest, the registry rows, the
tracked files, what each proof file counts, the targets of tools/allowlists/teeth.json, and the rows each Python
proof's red cases land on (`red_rows`; check 7, BK-S7); `unnamed_drivers` names the drivers no instrument names
(CA-S1). The pin test does the I/O.
"""

import ast
import posixpath
import re
from collections.abc import Collection, Iterable, Iterator, Mapping, Sequence

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
RESERVED = RED | VERDICT | {PARITY_ONLY}  # a driver may not define or assign these names (BK-B3)
ROW_PARAM = "row_id"  # the support helpers' row parameter; a driver helper's parameter of this name carries the row
ROW_CONST = "ROW"  # a module constant naming the row a row-less red helper lands on (BK-S7)
# A `with` whose every context is one of these lets a red call count; any other context may swallow its failure.
PLAIN_CONTEXTS = frozenset({"open", "self.subTest", "tempfile.TemporaryDirectory"})
SWALLOWS = frozenset({"AssertionError", "Exception", "BaseException"})  # a handler catching one swallows a red call
TESTCASE_BASES = frozenset({"unittest.TestCase", "support.DriverTest", "support.ParityDriver"})
MODULE_STATEMENTS = (ast.Import, ast.ImportFrom, ast.Assign, ast.AnnAssign, ast.ClassDef, ast.FunctionDef)
LOADER_HOOKS = frozenset({"setattr", "delattr", "exec", "eval", "compile", "__import__", "globals", "locals", "vars"})
LOADER_NAMES = frozenset({"load_tests", "__test__"})


# ------------------------------------------------------------------------------------------------------ python_cases


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


def _constant(node: ast.expr) -> tuple[bool, object]:
    """(True, its value) for an expression that is a literal (`0`, `""`, `None`, `()`, `[]`, `{}` …), else (False,
    None): the strength rule judges an `if`, a `while` and a loop's iterable by it (BK-B3)."""
    try:
        return True, ast.literal_eval(node)
    except ValueError, TypeError, SyntaxError, MemoryError, RecursionError:
        return False, None


def _empty(node: ast.expr) -> bool:
    known, value = _constant(node)
    try:
        return known and len(value) == 0  # type: ignore[arg-type]
    except TypeError:
        return False


def _plain(stmt: ast.With | ast.AsyncWith) -> bool:
    """Whether every context of the `with` is a plain resource, which never swallows the failure of its body."""
    for item in stmt.items:
        expr = item.context_expr
        if not isinstance(expr, ast.Call) or ast.unparse(expr.func) not in PLAIN_CONTEXTS:
            return False
    return True


def _swallows(handler: ast.ExceptHandler) -> bool:
    """Whether the handler catches a red helper's AssertionError: bare, or naming it or one of its bases."""
    if handler.type is None:
        return True
    names = handler.type.elts if isinstance(handler.type, ast.Tuple) else [handler.type]
    for name in names:
        text = name.attr if isinstance(name, ast.Attribute) else name.id if isinstance(name, ast.Name) else ""
        if text in SWALLOWS:
            return True
    return False


def _block_ends(block: Sequence[ast.stmt]) -> bool:
    return any(_ends(stmt) for stmt in block)


def _ends(stmt: ast.stmt) -> bool:
    """Whether control never reaches the statement after this one (a return, raise, break or continue, on every
    path the literals leave open)."""
    if isinstance(stmt, (ast.Return, ast.Raise, ast.Break, ast.Continue)):
        return True
    if isinstance(stmt, ast.If):
        known, value = _constant(stmt.test)
        if known:
            return _block_ends(stmt.body if value else stmt.orelse)
        return _block_ends(stmt.body) and _block_ends(stmt.orelse)
    if isinstance(stmt, (ast.With, ast.AsyncWith)):
        return _plain(stmt) and _block_ends(stmt.body)
    if isinstance(stmt, (ast.Try, ast.TryStar)):
        if _block_ends(stmt.finalbody):
            return True
        return (_block_ends(stmt.body) or _block_ends(stmt.orelse)) and all(_block_ends(h.body) for h in stmt.handlers)
    return False


def _live(body: Sequence[ast.stmt], nested: dict[str, Function]) -> list[ast.Call]:
    """The calls a block's live statements make whose failure reaches the case, the nested defs they call by name
    inlined. Not live: a branch a literal test never takes, a loop over an empty literal, a statement after one that
    always ends the block; not counted: a body under a `with` that is not a plain resource, or under a `try` one of
    whose handlers swallows an AssertionError (BK-B3)."""
    out: list[ast.Call] = []
    for stmt in body:
        if isinstance(stmt, (ast.FunctionDef, ast.AsyncFunctionDef)):
            nested[stmt.name] = stmt
            continue
        if isinstance(stmt, ast.ClassDef):
            continue
        out += _calls(stmt)
        if isinstance(stmt, (ast.If, ast.While)):
            known, value = _constant(stmt.test)
            if not known or value:
                out += _live(stmt.body, nested)
            if isinstance(stmt, ast.While) or not known or not value:
                out += _live(stmt.orelse, nested)
        elif isinstance(stmt, (ast.For, ast.AsyncFor)):
            if not _empty(stmt.iter):
                out += _live(stmt.body, nested)
            out += _live(stmt.orelse, nested)
        elif isinstance(stmt, (ast.With, ast.AsyncWith)):
            if _plain(stmt):
                out += _live(stmt.body, nested)
        elif isinstance(stmt, (ast.Try, ast.TryStar)):
            if not any(_swallows(h) for h in stmt.handlers):
                out += _live(stmt.body, nested)
            for handler in stmt.handlers:
                out += _calls(handler)
                out += _live(handler.body, nested)
            out += _live(stmt.orelse, nested)
            out += _live(stmt.finalbody, nested)
        elif isinstance(stmt, ast.Match):
            for case in stmt.cases:
                out += _calls(case)
                out += _live(case.body, nested)
        if _ends(stmt):
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


def _argument(call: ast.Call, position: int, keyword: str) -> ast.expr | None:
    if len(call.args) > position and not any(isinstance(a, ast.Starred) for a in call.args[: position + 1]):
        return call.args[position]
    return next((kw.value for kw in call.keywords if kw.arg == keyword), None)


def _on_self(call: ast.Call) -> str:
    f = call.func
    if isinstance(f, ast.Attribute) and isinstance(f.value, ast.Name) and f.value.id == "self":
        return f.attr
    return ""


class _Scope:
    def __init__(
        self, methods: Mapping[str, Function], functions: Mapping[str, Function], consts: Mapping[str, str]
    ) -> None:
        self.methods, self.functions, self.consts = methods, functions, consts

    def resolve(self, call: ast.Call) -> Function | None:
        name = _on_self(call)
        if name:
            return self.methods.get(name)
        if isinstance(call.func, ast.Name):
            return self.functions.get(call.func.id)
        return None

    def value(self, node: ast.expr | None, bound: Mapping[str, str]) -> str | None:
        """A string literal, or a name bound to one: a helper's parameter, else a module constant."""
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            return node.value
        if isinstance(node, ast.Name):
            return bound.get(node.id, self.consts.get(node.id))
        return None

    def bind(self, helper: Function, call: ast.Call, bound: Mapping[str, str]) -> dict[str, str]:
        """The helper's parameters the call passes a string to (`self` skipped for a method)."""
        params = [a.arg for a in (*helper.args.posonlyargs, *helper.args.args)]
        if _on_self(call):
            params = params[1:]
        out: dict[str, str] = {}
        for name, arg in zip(params, call.args, strict=False):
            if isinstance(arg, ast.Starred):
                break
            value = self.value(arg, bound)
            if value is not None:
                out[name] = value
        for kw in call.keywords:
            value = self.value(kw.value, bound)
            if kw.arg is not None and value is not None:
                out[kw.arg] = value
        return out

    def rowless(self, bound: Mapping[str, str]) -> set[str]:
        """The row a red helper without a row argument lands on: the helper's row_id, else the module's ROW."""
        row = bound.get(ROW_PARAM, self.consts.get(ROW_CONST))
        return {row} if row is not None else set()


def _reaches(
    fn: Function, scope: _Scope, level: int, seen: tuple[Function, ...], bound: Mapping[str, str]
) -> tuple[bool, bool, set[str]]:
    """(a red helper, a pass on the row under test, the rows red lands on) within LEVELS helper levels of fn's live
    code."""
    red = row_pass = False
    rows: set[str] = set()
    for call in _live(fn.body, {}):
        name = _on_self(call)
        if name in RED:
            red = True
            if name == "assertRed":
                row = scope.value(_argument(call, 2, ROW_PARAM), bound)
                rows |= {row} if row is not None else set()
            else:
                rows |= scope.rowless(bound)
            continue
        if name in VERDICT:
            ok, row = _literal(call, 3, "ok"), _literal(call, 2, ROW_PARAM)
            if ok is False:
                red = True
                value = scope.value(_argument(call, 2, ROW_PARAM), bound)
                rows |= {value} if value is not None else set()
            elif ok is True and isinstance(row, str) and row != "parity":
                row_pass = True
            continue
        if name == PARITY_ONLY:
            continue
        helper = scope.resolve(call)
        if helper is not None and level < LEVELS and helper not in seen:
            r, p, rs = _reaches(helper, scope, level + 1, (*seen, helper), scope.bind(helper, call, bound))
            red, row_pass = red or r, row_pass or p
            rows |= rs if r else set()
    return red, row_pass, rows


def _methods(cls: ast.ClassDef, classes: Mapping[str, ast.ClassDef]) -> dict[str, Function]:
    """The class's methods over its bases defined in the same module."""
    out: dict[str, Function] = {}
    for base in cls.bases:
        if isinstance(base, ast.Name) and base.id in classes and classes[base.id] is not cls:
            out.update(_methods(classes[base.id], classes))
    out.update({n.name: n for n in cls.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))})
    return out


def _is_testcase(cls: ast.ClassDef, classes: Mapping[str, ast.ClassDef], seen: frozenset[str] = frozenset()) -> bool:
    """Whether the class's bases reach unittest.TestCase or a support driver base (BK-B4: unittest loads no other)."""
    for base in cls.bases:
        if ast.unparse(base) in TESTCASE_BASES:
            return True
        local = isinstance(base, ast.Name) and base.id in classes and base.id not in seen and base.id != cls.name
        if local and _is_testcase(classes[base.id], classes, seen | {cls.name}):
            return True
    return False


def _outside_functions(tree: ast.Module) -> Iterator[ast.AST]:
    """Every node that runs at import: the module's and its classes' statements, never a function's body."""
    stack: list[ast.AST] = [tree]
    while stack:
        node = stack.pop()
        yield node
        for child in ast.iter_child_nodes(node):
            if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
                stack += [*child.decorator_list] if not isinstance(child, ast.Lambda) else []
                stack += [d for d in (*child.args.defaults, *child.args.kw_defaults) if d is not None]
                continue
            stack.append(child)


def _shape_faults(tree: ast.Module, path: str) -> list[str]:
    """What may change which cases unittest loads, or redefine a verdict helper (BK-B3, BK-B4): a module-level
    statement other than a docstring, an import, an assignment to names, a def or a class; a del or a loader hook at
    import; setattr or delattr anywhere; a definition or assignment of a verdict helper's name, of load_tests or of
    __test__."""
    faults: list[str] = []

    def fault(node: ast.AST, why: str) -> None:
        faults.append(f"teeth: {path}:{getattr(node, 'lineno', 0)}: {why}")

    for i, stmt in enumerate(tree.body):
        docstring = i == 0 and isinstance(stmt, ast.Expr) and isinstance(stmt.value, ast.Constant)
        if not docstring and not isinstance(stmt, MODULE_STATEMENTS):
            fault(stmt, f"a module-level {type(stmt).__name__} may change what unittest loads")
        if isinstance(stmt, ast.Assign):
            targets = stmt.targets
        elif isinstance(stmt, ast.AnnAssign):
            targets = [stmt.target]
        else:
            targets = []
        for target in targets:
            names = target.elts if isinstance(target, ast.Tuple) else [target]
            if not all(isinstance(n, ast.Name) for n in names):
                fault(stmt, "a module-level assignment to an attribute or item may change what unittest loads")
    for node in _outside_functions(tree):
        if isinstance(node, ast.Delete):
            fault(node, "a del at import may remove a counted case")
        elif isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in LOADER_HOOKS:
            fault(node, f"{node.func.id}() at import may change what unittest loads")
    for node in ast.walk(tree):
        for name in _defined_names(node):
            if name in RESERVED:
                fault(node, f"{name} is a verdict helper's name: a driver may not define or assign it")
            elif name in LOADER_NAMES:
                fault(node, f"{name} decides what unittest loads")
    return sorted(set(faults), key=faults.index)


def _defined_names(node: ast.AST) -> list[str]:
    """The names a node defines or assigns: a def or class, a stored name or attribute, an import's binding."""
    if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
        return [node.name]
    if isinstance(node, ast.Name) and isinstance(node.ctx, (ast.Store, ast.Del)):
        return [node.id]
    if isinstance(node, ast.Attribute) and isinstance(node.ctx, (ast.Store, ast.Del)):
        return [node.attr]
    if isinstance(node, ast.alias):
        return [node.asname or node.name.split(".")[0]]
    return []


def _consts(tree: ast.Module) -> dict[str, str]:
    """The module's string constants (`ROW = "docsync"`), the names a row argument may use."""
    out: dict[str, str] = {}
    for stmt in tree.body:
        if isinstance(stmt, ast.Assign) and len(stmt.targets) == 1:
            target, value = stmt.targets[0], stmt.value
        elif isinstance(stmt, ast.AnnAssign) and stmt.value is not None:
            target, value = stmt.target, stmt.value
        else:
            continue
        if isinstance(target, ast.Name) and isinstance(value, ast.Constant) and isinstance(value.value, str):
            out[target.id] = value.value
    return out


def _judge(source: str, path: str) -> tuple[list[str], list[str], frozenset[str]]:
    tree = ast.parse(source)
    classes = {n.name: n for n in tree.body if isinstance(n, ast.ClassDef)}
    functions = {n.name: n for n in tree.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))}
    consts = _consts(tree)
    cases: list[str] = []
    faults = _shape_faults(tree, path)
    rows: set[str] = set()
    red_cases = 0
    for cls in classes.values():
        own = [n for n in cls.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))]
        tests = [n for n in own if n.name.startswith(CASE_PREFIX)]
        if not _is_testcase(cls, classes):
            if tests:
                faults.append(f"teeth: {path}::{cls.name} is not a unittest.TestCase: its cases never run")
            continue
        scope = _Scope(_methods(cls, classes), functions, consts)
        names = [n.name for n in tests]
        for node in tests:
            name = f"{cls.name}.{node.name}"
            if names.count(node.name) > 1:
                faults.append(f"teeth: {path}::{name} is defined more than once: unittest runs only the last")
            cases.append(name)
            red, row_pass, on = _reaches(node, scope, 0, (node,), {})
            if not (red or row_pass):
                faults.append(f"teeth: {path}::{name} asserts no gate verdict")
            red_cases += red
            rows |= on if red else set()
        for stmt in cls.body:
            if isinstance(stmt, (ast.Assign, ast.AnnAssign)):
                for target in stmt.targets if isinstance(stmt, ast.Assign) else [stmt.target]:
                    if isinstance(target, ast.Name) and target.id.startswith(CASE_PREFIX):
                        faults.append(f"teeth: {path}::{cls.name}.{target.id} is assigned: a case may be replaced")
    if not cases:
        faults.append(f"teeth: {path} counts 0 cases")
    elif not red_cases:
        faults.append(f"teeth: {path} plants nothing red")
    return cases, faults, frozenset(rows)


def python_cases(source: str, path: str = "<source>") -> tuple[list[str], list[str]]:
    """A driver's cases (`<Class>.<method>`) and its faults by the strength rule."""
    cases, faults, _ = _judge(source, path)
    return cases, faults


def red_rows(source: str) -> frozenset[str]:
    """The rows a driver's red cases land on (BK-S7): the literal row id passed to a red helper or to a helper of
    the driver's own as its row_id, a module constant named there, or the module's ROW for a row-less red helper."""
    return _judge(source, "<source>")[2]


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
    *,
    red_rows: Mapping[str, Collection[str]] | None = None,
) -> list[str]:
    """The instrument pin's faults, each naming the row or instrument and the field; [] when the teeth hold.

    `red_rows` maps a Python proof to the rows its red cases land on (`red_rows(source)`); a proof it does not name
    lands on none, so a row instrument whose Python proofs hold no red case on its own row is red (check 7, BK-S7)."""
    red_rows = red_rows or {}
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
        # 7. a row instrument's Python proofs hold a red case on its own row (BK-S7)
        if not inst.go_test and inst.proofs:
            py = [p.path for p in inst.proofs if p.path.endswith(".py")]
            if not any(inst.row in red_rows.get(path, ()) for path in py):
                faults.append(f"{where}: no Python proof holds a red case on row {inst.row}")
    return faults


DRIVERS = "gate/proofs/drivers"


def unnamed_drivers(manifest: law.Manifest, tracked: Collection[str]) -> list[str]:
    """The tracked drivers (`gate/proofs/drivers/test_*.py`) no instrument names as a proof, sorted (CA-S1): the
    real-tree pin holds them to the strength rule too."""
    named = {p.path for inst in manifest.instruments for p in inst.proofs}
    return sorted(
        path
        for path in tracked
        if posixpath.dirname(path) == DRIVERS
        and posixpath.basename(path).startswith("test_")
        and path.endswith(".py")
        and path not in named
    )


def _show(value: object) -> str:
    return ", ".join(value) if isinstance(value, tuple) else str(value)
