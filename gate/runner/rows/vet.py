"""The vet row: `go vet -json` (design note §5; re-check claim 5, S5).

go1.27.3 exits 0 even with diagnostics, so any non-empty object fails, naming `posn`. A compile error prints no
object and exits 1, so vet's own non-zero exit fails too. GATE_COVERED counts the objects parsed. Arguments are
package patterns, by default `./...` (with none, go would vet only the current directory).
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import jsonstream, rows


def positions(node: object) -> list[str]:
    """Every diagnostic in one object, however deep: {package: {analyzer: [{posn, message}]}}."""
    if isinstance(node, list):
        return [found for item in node for found in positions(item)]
    if not isinstance(node, dict):
        return []
    if "posn" in node:
        return [f"{node['posn']}: {node.get('message', '')}"]
    return [found for value in node.values() for found in positions(value)]


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    out, err = os.path.join(row_dir, "vet.json"), os.path.join(row_dir, "vet.stderr")
    code = rows.run_tool(["go", "vet", "-json", *(argv[1:] or ["./..."])], out, err)
    failures: list[str] = []
    with open(out, "rb") as f:
        data = f.read()
    try:
        objects = jsonstream.parse(data)
    except ValueError as e:
        objects = []
        failures.append(f"unreadable go vet output: {e}")
    for obj in objects:
        if obj:
            failures += positions(obj) or [json.dumps(obj, sort_keys=True)]
    if code != 0:
        failures.append(f"go vet exit {code}: {' '.join(rows.read_text(err).split())}")
    rows.covered(len(objects))
    return rows.finish(sorted(failures))


if __name__ == "__main__":
    sys.exit(main(sys.argv))
