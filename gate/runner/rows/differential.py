"""The grammar differential (ADR-0002 §5 as amended; S2.2a): the normative lark grammar and the engine parser agree
on every corpus file, by verdict and by parse shape (tools/grammar-diff.md).

The lark side runs in this process, under the pinned CPython with the pinned lark; the engine side is one
`go run ./tools/shape` over the same files. A file is covered when both sides judged it; any disagreement fails the
row naming the file and the first differing expression. A reject the engine reports at lowering (a grammar-valid
file the model cannot hold) is a grammar accept, compared by verdict only. shapes.txt in the row's directory holds
every pair for reading.
"""

import itertools
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.differential import shapes
from gate.runner import rows


def engine_reports(row_dir: str, files: list[str]) -> tuple[dict[str, dict], list[str]]:
    out, err = os.path.join(row_dir, "shape.stdout"), os.path.join(row_dir, "shape.stderr")
    code = rows.run_tool(["go", "run", "./tools/shape", *files], out, err)
    failures = []
    if code != 0:
        last = (rows.read_text(err).strip().splitlines() or ["no output"])[-1]
        failures.append(f"go run ./tools/shape: exit {code}: {last}")
    reports = {}
    for line in rows.read_text(out).splitlines():
        report = json.loads(line)
        reports[report["file"]] = report
    return reports, failures


def compare(rel: str, verdict: str, lark_shapes: list[str], engine: dict) -> list[str]:
    lowering = engine.get("stage") == "lowering"
    engine_accepts = engine["verdict"] == "accept" or lowering
    if verdict == "accept" and not engine_accepts:
        return [f"{rel}: lark accepts, the engine rejects with {engine.get('code', '?')}"]
    if verdict == "reject" and engine_accepts:
        return [f"{rel}: lark rejects, the engine accepts"]
    if verdict == "reject" or lowering:
        return []
    for i, (a, b) in enumerate(itertools.zip_longest(lark_shapes, engine["shapes"]), start=1):
        if a != b:
            return [f"{rel}: expression {i} differs: lark {a} / engine {b}"]
    return []


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    files = [rows.show(p) for p in rows.read_list(argv[1])]
    engine, failures = engine_reports(row_dir, files)
    grammar = shapes.Grammar(shapes.load_lark(), rows.read_text(shapes.GRAMMAR))
    covered = 0
    with open(os.path.join(row_dir, "shapes.txt"), "w", encoding="utf-8") as pairs:
        for rel in files:
            verdict, lark_shapes = grammar.judge(rows.read_text(rel))
            report = engine.get(rel)
            if report is None:
                failures.append(f"{rel}: no verdict from the engine")
                continue
            covered += 1
            failures += compare(rel, verdict, lark_shapes, report)
            stage = f" at {report['stage']}" if report.get("stage") else ""
            pairs.write(f"{rel}: lark {verdict}, engine {report['verdict']}{stage}\n")
            for a, b in itertools.zip_longest(lark_shapes, report["shapes"]):
                pairs.write(f"  lark:   {a}\n  engine: {b}\n")
    rows.covered(covered)
    return rows.finish(failures)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
