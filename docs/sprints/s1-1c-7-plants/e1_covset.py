"""C1's K2-del proof: covered-block sets from Go cover profiles (stdlib only)."""

import sys


def covered(path):
    blocks = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            key, _stmts, count = line.rsplit(" ", 2)
            blocks[key] = blocks.get(key, False) or int(count) > 0
    return {k for k, v in blocks.items() if v}


def main(d):
    p = {n: covered(f"{d}/{n}.out") for n in ("d113", "d129", "b1", "b2", "r1", "r2")}
    deleted = p["d113"] | p["d129"]
    remaining = p["r1"] & p["r2"]
    missing = sorted(deleted - remaining)
    print(f"covered(d113) {len(p['d113'])}, covered(d129) {len(p['d129'])}, union {len(deleted)}")
    print(f"covered(r1) {len(p['r1'])}, covered(r2) {len(p['r2'])}, intersection {len(remaining)}")
    print(f"subset check: {len(missing)} deleted-test blocks not in the intersection -> {'PASS' if not missing else 'FAIL'}")
    for m in missing:
        print("  missing:", m)
    sets = [p[n] for n in ("b1", "b2", "r1", "r2")]
    print("whole-package covered sets: " + ", ".join(f"{n} {len(p[n])}" for n in ("b1", "b2", "r1", "r2")))
    if all(s == sets[0] for s in sets):
        print("whole-package check: all four equal -> PASS")
    else:
        for a, b in (("b1", "b2"), ("r1", "r2"), ("b1", "r1"), ("b2", "r2")):
            for k in sorted(p[a] ^ p[b]):
                print(f"  {a} vs {b}: {'only ' + a if k in p[a] else 'only ' + b}: {k}")
        print("whole-package check: not all four equal -> FAIL")
    ns = {}
    with open(f"{d}/ns.out", encoding="utf-8") as f:
        for line in f:
            if "internal/runtime/shard.go:182." in line:
                print("ns block:", line.strip())
    return 1 if missing else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
