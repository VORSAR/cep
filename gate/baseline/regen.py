"""gate/regen-baseline: re-records the Tier-2 baselines (qa-strategy §3; design s3-3-2 §8.3).

It re-runs Tier 1 first (the gate's `conformance` and `semantics` rows), writes a fresh run (TestBaseline with
CEP_BASELINE_WRITE), classifies every block against the stored baseline and records only when no block is refused.
Run by path from the repository root with the pinned interpreter:

    python3.15 -I -B -X utf8 gate/baseline/regen.py [--accept '<file> <file>:<rule>:<ordinal> …']

(`-I` keeps the working directory off sys.path, so `-m gate.baseline.regen` cannot import `gate`.)
"""

import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
if REPO not in sys.path:  # run by path: gate's own modules come from the repository it lives in
    sys.path.insert(0, REPO)

BASELINE = os.path.join("conformance", "baseline")
SPEC = os.path.join("docs", "language", "SPEC.md")
FORMAT = 1
TIER1_ROWS = ("parity", "conformance", "semantics")


@dataclass(frozen=True)
class Block:
    file: str  # the corpus file's name, e.g. "07-consent-gate.cep"
    rule: str
    ordinal: int  # 1-based within the rule; display only
    key: str  # §8.1's content key, with its "#2" suffix if any
    outputs: tuple[str, ...]  # each output's canonical JSON text, as the line holds it
    counts: tuple[tuple[str, int], ...]  # sorted (name, value) pairs


@dataclass(frozen=True)
class Baseline:
    file: str
    format: int
    source_sha256: str
    spec_rev: int
    blocks: tuple[Block, ...]


@dataclass(frozen=True)
class Verdict:
    file: str
    rule: str
    ordinal: int  # the new run's ordinal; the old one's for a gone block
    key: str
    cls: str  # §8.3's class name
    recorded: bool


# ---------------------------------------------------------------------------------------------------- the file format


def _object(line: str, what: str) -> dict:
    value = json.loads(line)  # a json.JSONDecodeError is a ValueError
    if not isinstance(value, dict):
        raise ValueError(f"{what} is not a JSON object: {line[:80]}")
    return value


def _raw_outputs(line: str) -> tuple[str, ...]:
    """Each element of the line's "outputs" array as it is written, so a render gives the same bytes back."""
    start = line.find('"outputs":[')
    if start < 0:
        raise ValueError(f"a block line without its outputs: {line[:80]}")
    i, out, dec = start + len('"outputs":['), [], json.JSONDecoder()
    while line[i] != "]":
        _, end = dec.raw_decode(line, i)
        out.append(line[i:end])
        i = end
        if line[i] == ",":
            i += 1
    return tuple(out)


def parse(text: str) -> Baseline:
    """One .jsonl file; ValueError on a malformed line."""
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    if not lines:
        raise ValueError("an empty baseline file")
    header = _object(lines[0], "the header")
    for key, kind in (("file", str), ("format", int), ("source_sha256", str), ("spec_rev", int)):
        if not isinstance(header.get(key), kind):
            raise ValueError(f"the header has no {key}: {lines[0][:80]}")
    blocks, keys = [], set()
    for line in lines[1:]:
        b = _object(line, "a block line")
        for key, kind in (("block", int), ("counts", dict), ("key", str), ("outputs", list), ("rule", str)):
            if not isinstance(b.get(key), kind):
                raise ValueError(f"a block line has no {key}: {line[:80]}")
        for name, value in b["counts"].items():  # an int, never a bool, a string or a fraction (Q-L1)
            if not isinstance(value, int) or isinstance(value, bool):
                raise ValueError(f"a block line's count {name} is not an integer: {line[:80]}")
        if b["key"] in keys:  # two blocks of one file never share a key: the Go side numbers equal ones (Q-S2)
            raise ValueError(f"the key {b['key']} is written twice in {header['file']}")
        keys.add(b["key"])
        counts = tuple(sorted((str(k), v) for k, v in b["counts"].items()))
        blocks.append(Block(header["file"], b["rule"], b["block"], b["key"], _raw_outputs(line), counts))
    return Baseline(header["file"], header["format"], header["source_sha256"], header["spec_rev"], tuple(blocks))


def render(b: Baseline) -> str:
    """The inverse of parse, byte for byte: keys sorted, no spaces, one newline per line."""
    head = {"file": b.file, "format": b.format, "source_sha256": b.source_sha256, "spec_rev": b.spec_rev}
    out = [json.dumps(head, sort_keys=True, separators=(",", ":"), ensure_ascii=False)]
    for x in b.blocks:
        counts = "{" + ",".join(f"{json.dumps(k)}:{v}" for k, v in x.counts) + "}"
        out.append(
            f'{{"block":{x.ordinal},"counts":{counts},"key":{json.dumps(x.key)},'
            f'"outputs":[{",".join(x.outputs)}],"rule":{json.dumps(x.rule, ensure_ascii=False)}}}'
        )
    return "\n".join(out) + "\n"


# ------------------------------------------------------------------------------------------------------ the classifier


def _ts(output: str) -> tuple:
    """An output's times: its top-level ts and its attrs' ts, whichever it carries."""
    o = json.loads(output)
    return (o.get("ts"), (o.get("attrs") or {}).get("ts"))


def _accepted(b: Block, accept: frozenset[str]) -> bool:
    return b.file in accept or f"{b.file}:{b.rule}:{b.ordinal}" in accept


def _compare(old: Block, new: Block, stored_rev: int, spec_rev: int, accept: frozenset[str]) -> tuple[str, bool] | None:
    """The first matching row of §8.3's table for a key in both runs; None when equal."""
    if old.outputs == new.outputs and old.counts == new.counts:
        return None
    if len(old.outputs) != len(new.outputs):
        return "count-change", False
    if old.outputs != new.outputs and sorted(old.outputs) == sorted(new.outputs):
        return "ordering-change", False
    if any(_ts(a) != _ts(b) for a, b in zip(old.outputs, new.outputs, strict=True)):
        return "timestamp-drift", False
    if old.counts != new.counts:
        return "drop-change", False
    if spec_rev > stored_rev and _accepted(new, accept):
        return "spec-rev", True
    return "value-change", False


def classify(
    old: dict[str, Baseline],
    new: dict[str, Baseline],
    spec_rev: int,
    accept: frozenset[str],
    sources: dict[str, str] | None = None,
) -> list[Verdict]:
    """Every block of every file in either run, keyed (file, key); verdicts sorted by (file, ordinal, key). sources is
    each corpus file's SHA-256 as it stands: a file only in the stored run whose corpus file is unchanged is
    block-vanished, not block-removed (the break-it's Q-S3)."""
    out: list[Verdict] = []

    def verdict(b: Block, cls: str, recorded: bool) -> None:
        out.append(Verdict(b.file, b.rule, b.ordinal, b.key, cls, recorded))

    for name in sorted(set(old) | set(new)):
        o, n = old.get(name), new.get(name)
        if (o and o.format != FORMAT) or (n and n.format != FORMAT):
            for b in (n or o).blocks:
                verdict(b, "format-change", False)
            continue
        if o is None:
            for b in n.blocks:
                verdict(b, "additive-new-block", True)
            continue
        if n is None:
            vanished = sources is not None and sources.get(name) == o.source_sha256
            for b in o.blocks:  # its corpus file is gone or changed, unless its bytes still hash as stored
                verdict(b, "block-vanished" if vanished else "block-removed", not vanished)
            continue
        olds, news = {b.key: b for b in o.blocks}, {b.key: b for b in n.blocks}
        for b in n.blocks:
            if b.key not in olds:
                verdict(b, "additive-new-block", True)
                continue
            found = _compare(olds[b.key], b, o.spec_rev, spec_rev, accept)
            if found:
                verdict(b, *found)
        for b in o.blocks:
            if b.key not in news:
                changed = o.source_sha256 != n.source_sha256
                verdict(b, "block-removed" if changed else "block-vanished", changed)
    return sorted(out, key=lambda v: (v.file, v.ordinal, v.key))


# ------------------------------------------------------------------------------------------------------------ the tool


def run_tier1(repo: str) -> int:
    """Step 1: the gate on a registry of the Tier-1 rows; its exit code."""
    with open(os.path.join(repo, "gate", "registry.json"), encoding="utf-8") as f:
        registry = json.load(f)
    registry["rows"] = [r for r in registry["rows"] if r["id"] in TIER1_ROWS]
    missing = sorted(set(TIER1_ROWS) - {"parity"} - {r["id"] for r in registry["rows"]})
    if missing:  # a Tier-1 row renamed or dropped is not a green Tier 1 (the break-it's Q-L4)
        print(f"Tier 1 is incomplete: the registry has no row {', '.join(missing)}")
        return 1
    with tempfile.TemporaryDirectory(prefix="regen-tier1-") as tmp:
        path = os.path.join(tmp, "registry.json")
        with open(path, "w", encoding="utf-8") as f:
            json.dump(registry, f)
        cmd = [sys.executable, "-I", "-B", "-X", "utf8", os.path.join(repo, "gate", "run.py")]
        cmd += ["--tier", "T2", "--registry", path, "--run-dir", os.path.join(tmp, "run")]
        return subprocess.run(cmd, cwd=repo, stdin=subprocess.DEVNULL, check=False).returncode


def _toolchain(repo: str) -> tuple[str, list]:
    """The tool cache and the pinned tools, as the gate's runner reads them."""
    from gate.toolchain import manifest

    return manifest.cache_root(os.environ), manifest.load(os.path.join(repo, "tools.json"), manifest.platform_key())


def pinned_python(repo: str) -> str:
    """The pinned CPython's path, laid out as gate/toolchain/install.py's exec finds it."""
    from gate.toolchain import manifest

    cache, tools = _toolchain(repo)
    tool = next(t for t in tools if t.name == manifest.GATE_PYTHON)
    return os.path.join(
        os.path.abspath(manifest.tool_home(cache, tool)), os.path.normpath(os.path.join(tool.path_dir, tool.command))
    )


def fresh_run(repo: str, out_dir: str) -> int:
    """Step 2: TestBaseline writing the fresh files into out_dir, under the pinned Go; go's exit code."""
    from gate.runner import env as gate_env
    from gate.toolchain import manifest

    cache, tools = _toolchain(repo)
    env = gate_env.clean_env(os.environ, manifest.path_dirs(cache, tools))
    env["CEP_BASELINE_WRITE"] = out_dir
    cmd = ["go", "test", "-count=1", "-run", "^TestBaseline$", "./internal/playback"]
    return subprocess.run(cmd, cwd=repo, env=env, stdin=subprocess.DEVNULL, check=False).returncode


def _read_dir(path: str) -> tuple[dict[str, Baseline], dict[str, str]]:
    parsed, texts = {}, {}
    if not os.path.isdir(path):  # the first recording
        return parsed, texts
    for name in sorted(os.listdir(path)):
        if not name.endswith(".jsonl"):
            continue  # the README
        with open(os.path.join(path, name), encoding="utf-8") as f:
            text = f.read()
        b = parse(text)
        parsed[b.file], texts[name] = b, text
    return parsed, texts


def _sources(repo: str) -> dict[str, str]:
    """Each corpus file's SHA-256 by its name, as the baseline headers name it."""
    out = {}
    corpus = os.path.join(repo, "docs", "language", "examples")
    for sub in ("", "torture"):
        d = os.path.join(corpus, sub)
        if not os.path.isdir(d):
            continue
        for name in sorted(os.listdir(d)):
            if name.endswith(".cep"):
                with open(os.path.join(d, name), "rb") as f:
                    out[name] = hashlib.sha256(f.read()).hexdigest()
    return out


def _spec_rev(repo: str) -> int:
    with open(os.path.join(repo, SPEC), encoding="utf-8") as f:
        m = re.search(r"rev (\d+) \(", f.read())
    if m is None:
        raise ValueError(f"{SPEC}: no `rev N (` in its status line")
    return int(m.group(1))


def main(argv: list[str]) -> int:
    accept: frozenset[str] = frozenset()
    if argv[:1] == ["--accept"] and len(argv) == 2:
        accept = frozenset(argv[1].split())
    elif argv:
        print("usage: regen.py [--accept '<file> <file>:<rule>:<ordinal> …']")
        return 2
    repo = REPO
    if run_tier1(repo) != 0:
        print("refused: Tier 1 is red")
        return 1
    stored_dir = os.path.join(repo, BASELINE)
    with tempfile.TemporaryDirectory(prefix="regen-fresh-") as fresh_dir:
        if fresh_run(repo, fresh_dir) != 0:
            print("refused: the fresh run failed")
            return 1
        try:
            old, old_texts = _read_dir(stored_dir)
            new, new_texts = _read_dir(fresh_dir)
        except ValueError as e:
            print(f"refused: a malformed baseline: {e}")
            return 1
        spec_rev = _spec_rev(repo)
        verdicts = classify(old, new, spec_rev, accept, _sources(repo))
        if not verdicts and old_texts == new_texts:
            print("no change")
            return 0
        for v in verdicts:
            verdict = "recorded" if v.recorded else "REFUSED"
            print(f"{v.cls} {v.file} rule {v.rule} block {v.ordinal} key {v.key[:12]}: {verdict}")
            if v.cls == "value-change" and v.file in old and spec_rev > old[v.file].spec_rev:  # it can record (Q-S4)
                print(f"  to accept it as a SPEC revision's change: ACCEPT {v.file}:{v.rule}:{v.ordinal}")
        refused = [v for v in verdicts if not v.recorded]
        if refused:
            print(f"refused: {len(refused)} block(s); nothing written")
            return 1
        for name in old_texts:
            if name not in new_texts:
                os.remove(os.path.join(stored_dir, name))
        os.makedirs(stored_dir, exist_ok=True)
        for name in new_texts:
            shutil.copyfile(os.path.join(fresh_dir, name), os.path.join(stored_dir, name))
        print("no change in any block; headers re-recorded" if not verdicts else f"recorded {len(verdicts)} block(s)")
        return 0


if __name__ == "__main__":
    pinned = pinned_python(REPO)
    if os.path.realpath(sys.executable) != os.path.realpath(pinned):  # gate/run.py runs under the pinned CPython only
        os.execv(pinned, [pinned, "-I", "-B", "-X", "utf8", os.path.abspath(__file__), *sys.argv[1:]])
    sys.exit(main(sys.argv[1:]))
