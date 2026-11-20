"""The report-only prototype of S1.1c-7's naming rule (plan check §0 item 3, §3; owner, 2026-11-20, decision 5).

    python3 docs/sprints/s1-1c-7-plants/nameproto.py <out.json> [<full survey findings JSON>]

Run from the repository's top level. Reads every tracked *_test.go and test_*.py outside testdata, takes each name by
kind (test file or module; Go top-level functions and methods, and types, vars and consts including grouped and
multi-name declarations; Python classes, methods, functions and module-level names), normalises it to a lowercase token
sequence and matches the rules on whole-token runs. Writes every hit to <out.json> with the rules it matched, and
prints the counts, including the decided scope's (the core rules plus "review id with digit" and "writer prefix, no
digit", over files, tests, classes and helpers; subtests excluded). With the coordinator's full findings JSON, it also
reconciles the hits with the survey's G3 findings. S1.1c-7's naming tool must find every decided-scope hit this
prototype finds on the G0 tree (plan check §3).
"""

import ast
import json
import re
import subprocess
import sys
from collections import Counter, defaultdict

# --- normalisation (PQ-S5): split on _ - . and at case changes; lowercase; join with _ ---
_SPLIT = re.compile(r"[_\-.]+")
_CASE = re.compile(r"(?<=[a-z])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])|(?<=[0-9])(?=[A-Z])")


def tokens(name: str) -> str:
    out = []
    for part in _SPLIT.split(name):
        out += [p.lower() for p in _CASE.split(part) if p]
    return "_".join(out)


# --- the word list: each rule is a regex over the joined token string, anchored at token boundaries ---
CORE = {
    "sprint id": r"s\d+_\d+[a-z]?(?:_\d+)?|s\d+c\d+(?:_\d+)?|s\d+_\d+_\d+",
    "review step": r"break_?its?\d*|fix_?rounds?\d*|fix_s\d+\S*",
    "lane or wave": r"lane_[a-z]\d?|wave_?\d+",
}
WIDE = {  # report-only: candidates for a wider rule (owner Q1)
    "review id with digit": r"(?:qb|qa|bk|pa|pq|cv|cvi|cg|ar|br)\d+[a-z]?",
    "writer prefix, no digit": r"(?:bk|qa|qb|pq|pa|cv|cvi|cn|q2|pb)",
    "design row id": r"[dfhgr]\d+[a-z]?|slot_\d+|case_\d+|c\d+",
}


def match(rule: str, tok: str) -> bool:
    return re.search(rf"(?:^|_)(?:{rule})(?:_|$)", tok) is not None


# --- extraction ---
GO_TOP = re.compile(r"^func (?:\([^)]*\) )?([A-Za-z_]\w*)", re.M)
GO_DECL = re.compile(r"^(?:type|var|const) ([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)", re.M)
GO_BLOCK = re.compile(r"^(?:type|var|const) \(\n(.*?)^\)", re.M | re.S)
GO_INNER = re.compile(r"^\t([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)", re.M)
GO_RUN = re.compile(r"\bt\.Run\(\s*\"([^\"]*)\"")


def go_names(path: str, text: str):
    base = path.rsplit("/", 1)[-1]
    yield "test file", base[: -len("_test.go")]
    for m in GO_TOP.finditer(text):
        n = m.group(1)
        kind = "test func" if re.match(r"(Test|Benchmark|Fuzz|Example)([A-Z_0-9]|$)", n) and not m.group(0).startswith("func (") else "helper"
        yield kind, n
    for m in GO_DECL.finditer(text):
        for n in re.split(r"\s*,\s*", m.group(1)):
            yield "helper", n
    for b in GO_BLOCK.finditer(text):
        for m in GO_INNER.finditer(b.group(1)):
            for n in re.split(r"\s*,\s*", m.group(1)):
                yield "helper", n
    for m in GO_RUN.finditer(text):
        yield "subtest", m.group(1)


def py_names(path: str, text: str):
    base = path.rsplit("/", 1)[-1]
    yield "test file", base[: -len(".py")]
    try:
        tree = ast.parse(text)
    except SyntaxError:
        return
    for node in tree.body:
        if isinstance(node, ast.ClassDef):
            yield "test class", node.name
            for sub in node.body:
                if isinstance(sub, (ast.FunctionDef, ast.AsyncFunctionDef)):
                    yield ("test func" if sub.name.startswith("test") else "helper"), sub.name
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            yield ("test func" if node.name.startswith("test") else "helper"), node.name
        elif isinstance(node, ast.Assign):
            for t in node.targets:
                if isinstance(t, ast.Name):
                    yield "helper", t.id


DECIDED_KINDS = ("test file", "test func", "test class", "helper")
DECIDED_WIDE = ("review id with digit", "writer prefix, no digit")


def decided(hit: dict) -> bool:
    return hit["kind"] in DECIDED_KINDS and bool(hit["core"] or [r for r in hit["wide"] if r in DECIDED_WIDE])


def main() -> None:
    out_path = sys.argv[1]
    g3 = []
    if len(sys.argv) > 2:
        g3 = [f for f in json.load(open(sys.argv[2], encoding="utf-8")) if f["group"] == "G3"]
    files = subprocess.run(["git", "ls-files", "*_test.go", "**/test_*.py", "test_*.py"], capture_output=True,
                           text=True, check=True).stdout.split()
    files = [f for f in files if "/testdata/" not in f]
    names = Counter()
    hits = []
    for path in files:
        text = open(path, encoding="utf-8").read()
        gen = go_names(path, text) if path.endswith(".go") else py_names(path, text)
        seen = set()
        for kind, name in gen:
            if (kind, name) in seen:
                continue
            seen.add((kind, name))
            names[kind] += 1
            tok = tokens(name)
            core = [r for r, rx in CORE.items() if match(rx, tok)]
            wide = [r for r, rx in WIDE.items() if match(rx, tok)]
            if core or wide:
                hits.append({"path": path, "kind": kind, "name": name, "core": core, "wide": wide})
    for hit in hits:
        hit["decided"] = decided(hit)
    by_file = defaultdict(list)
    for f in g3:
        by_file[f["path"]].append(f)
    for hit in hits:
        hit["g3"] = sorted({f["id"] for f in by_file.get(hit["path"], [])
                            if hit["name"] in (f["element"] + " " + f["finding"] + " " + f["loc"])})
    chosen = [h for h in hits if h["decided"]]
    print(f"files {len(files)} (go {sum(f.endswith('.go') for f in files)}, py {sum(f.endswith('.py') for f in files)})")
    print("names by kind:", dict(names))
    print("decided-scope hits:", len(chosen), "in", len({h["path"] for h in chosen}), "files;",
          dict(Counter(h["kind"] for h in chosen)))
    if g3:
        print(f"G3 findings {len(g3)}; quoted by a decided hit {len({i for h in chosen for i in h['g3']})};",
              f"decided hits no G3 finding quotes {sum(1 for h in chosen if not h['g3'])}")
    with open(out_path, "w", encoding="utf-8") as out:
        json.dump({"hits": hits}, out, indent=1, ensure_ascii=False)
        out.write("\n")


if __name__ == "__main__":
    main()
