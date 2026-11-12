"""The deps register's lane D additions (S1.1c-2 design §4.5 graph-only guard, §4.6a, §5 lane D), on fixtures.

`deps.compare(modules, graph, entries, gomods, read, ctx)`: a graph node selected by neither build list is red;
every entry carries `sprint` and `ref`, checked by `allowlists.sprint_fault` and `allowlists.ref_fault` (DA-S1,
DA-L6) with an injected `allowlists.Ctx` (DQ-S9), each fault `deps: <module>: <why>` (v1 §4.6a).

design ambiguity: Entry's field order; these tests build it by keyword with `sprint` and `ref` named.
design ambiguity: whether a graph-only node with no entry gets both lines; read here as both (v1: "beside its
allowlist check").
"""

import json
import unittest
from datetime import date

from gate.runner import allowlists, deps, sprintid

ORDER = sprintid.Order(tuple(sprintid.Row(id, closed) for id, closed in (("S1.1a-1", True), ("S1.1c", False))))
REF = "docs/sprints/decisions.md#owner-decisions-2026-11-11"


def resolve(ref: str) -> str:
    return "" if ref == REF else f"{ref.split('#')[0]} has no heading #{ref.split('#')[-1]}"


CTX = allowlists.Ctx(
    today=date(2026, 11, 12),
    tier="T2",
    order=ORDER,
    current=sprintid.parse("S1.1c-2"),
    issues=frozenset(),
    resolve=resolve,
)


def E(module: str, sprint: str = "S1.1c-1", ref: str = REF) -> deps.Entry:
    return deps.Entry(
        module=module,
        version="v1.0.0",
        scope="runtime",
        license="MIT",
        approved="2026-11-11",
        why="a fixture",
        sprint=sprint,
        ref=ref,
    )


def compare(modules, graph, entries):
    return deps.compare(modules, frozenset(graph), entries, list(deps.GOMODS), lambda d: None, CTX)


def entry_json(**over: str) -> dict:
    e = {
        "module": "example.com/a",
        "version": "v1.0.0",
        "scope": "runtime",
        "license": "MIT",
        "approved": "2026-11-11",
        "why": "a fixture",
        "sprint": "S1.1c-1",
        "ref": REF,
    }
    e.update(over)
    return e


class Register(unittest.TestCase):
    def test_a_good_entry_passes(self):
        self.assertEqual(
            compare([deps.Module("example.com/a", "v1.0.0", "", False)], ["example.com/a"], [E("example.com/a")]), []
        )

    def test_a_graph_node_selected_by_neither_build_list_is_red(self):
        self.assertEqual(
            compare([], ["example.com/g"], [E("example.com/g")]),
            ["deps: example.com/g is in the module graph and selected by neither module's build list"],
        )

    def test_a_sprint_after_the_marker_is_red(self):
        self.assertEqual(
            compare(
                [deps.Module("example.com/a", "v1.0.0", "", False)],
                ["example.com/a"],
                [E("example.com/a", sprint="S1.1c-9")],
            ),
            ["deps: example.com/a: sprint S1.1c-9 is after the current sprint S1.1c-2"],
        )

    def test_an_unresolved_ref_is_red(self):
        bad = "docs/sprints/decisions.md#gone"
        self.assertEqual(
            compare(
                [deps.Module("example.com/a", "v1.0.0", "", False)], ["example.com/a"], [E("example.com/a", ref=bad)]
            ),
            ["deps: example.com/a: docs/sprints/decisions.md has no heading #gone"],
        )

    def test_the_loader_reads_sprint_and_ref(self):
        text = json.dumps({"schema": 1, "note": "n", "modules": [entry_json()]})
        self.assertEqual(deps.load_allowlist(text), (E("example.com/a"),))

    def test_an_entry_missing_sprint_or_ref_is_refused(self):
        for key in ("sprint", "ref"):
            with self.subTest(key=key):
                e = entry_json()
                del e[key]
                with self.assertRaises(ValueError) as cm:
                    deps.load_allowlist(json.dumps({"schema": 1, "note": "n", "modules": [e]}))
                self.assertIn(f"missing ['{key}']", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
