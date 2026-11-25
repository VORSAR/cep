"""The shared parallel check of S1.1c-7's W3 (lane CD1's design note, G2-73 §4; also G2-106 and G2-134).

    python3.15 -I -B docs/sprints/s1-1c-7-plants/cd1_g2_73_check.py <file>:<TestName>...

For each pair, the line after `func <TestName>(` must be `t.Parallel()`, or the doc block above the function must hold
`Not parallel:`. Prints the count and the pairs that hold neither; exit 1 when any does, 0 when none.
"""

import sys

bad = []
for spec in sys.argv[1:]:
    path, name = spec.rsplit(":", 1)
    with open(path, encoding="utf-8") as f:
        src = f.read().split("\n")
    i = next(k for k, line in enumerate(src) if line.startswith(f"func {name}("))
    j, doc = i - 1, []
    while j >= 0 and src[j].startswith("//"):
        doc.append(src[j])
        j -= 1
    if src[i + 1].strip() != "t.Parallel()" and not any("Not parallel:" in d for d in doc):
        bad.append(spec)
print(len(bad), "without t.Parallel or a reason:", bad)
sys.exit(1 if bad else 0)
