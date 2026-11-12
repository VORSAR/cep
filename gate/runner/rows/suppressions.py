"""The suppressions row (S1.1c-2 design §2.4): every exemption in the one grammar (DEVELOPMENT.md's suppression
policy; ADR-0015 §3.10).

It reads the context once — gate/sprints.json, gate/current-sprint, gate/known-issues.json's ids, the run's date,
GATE_TIER, and the tracked Markdown a ref may name — then loads and checks every tools/allowlists/*.json, and reads
each listed file: a .go file's //nolint directives, a gate/**/*.py or .githooks/* file's # fmt: skip markers.
ruff.toml's per-file waivers and tools/allowlists/ruff.json must name the same (file, code) pairs both ways; a
ruff.json file or a teeth.json row id that no longer exists is a stale target. GATE_COVERED is the number of listed
files processed, the ones without exemptions to read included; the side inputs are never counted (DA-S9). No
network; git only for `ls-files`.
"""

import json
import os
import sys
import tomllib
from datetime import UTC, datetime

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))

from gate.runner import allowlists, anchors, rows, sprintid, suppressions

LISTS = "tools/allowlists/"
RUFF_TOML = "ruff.toml"
RUFF_WAIVERS = ("per-file-ignores", "extend-per-file-ignores")
RUFF_GLOBAL = ("ignore", "extend-ignore")
REGISTRY = "gate/registry.json"
KNOWN_ISSUES = "gate/known-issues.json"
SIDE_FAULTS = (OSError, ValueError, KeyError, TypeError)


def tracked_files(row_dir: str) -> list[str]:
    out, err = os.path.join(row_dir, "ls-files.stdout"), os.path.join(row_dir, "ls-files.stderr")
    code = rows.run_tool(["git", "ls-files", "-z"], out, err)
    if code != 0:
        raise OSError(f"git ls-files: exit {code}: {rows.read_text(err).strip()}")
    return sorted(p for p in rows.read_text(out).split("\0") if p)


def context(tracked: set[str]) -> allowlists.Ctx:
    """The run's Ctx; a ValueError or OSError names the side input that cannot be read."""
    order = sprintid.load_order(rows.read_text(sprintid.SPRINTS))
    current = sprintid.read_marker(rows.read_text(sprintid.MARKER))
    order.index(sprintid.canonical(current))  # the marker's row is in the order, or a ValueError naming it
    issues = frozenset(str(e["id"]) for e in json.loads(rows.read_text(KNOWN_ISSUES))["entries"])

    def read(path: str) -> str | None:
        if path not in tracked or not path.endswith(".md"):
            return None  # an untracked local doc never satisfies a ref (DA-L5)
        try:
            return rows.read_text(path)
        except OSError:
            return None

    return allowlists.Ctx(
        today=datetime.now(UTC).date(),
        tier=os.environ["GATE_TIER"],
        order=order,
        current=current,
        issues=issues,
        resolve=lambda ref: anchors.resolves(ref, read),
    )


def load_lists(tracked: list[str]) -> tuple[dict[str, allowlists.AllowList], list[str]]:
    lists: dict[str, allowlists.AllowList] = {}
    faults: list[str] = []
    for path in tracked:
        name = path.removeprefix(LISTS).removesuffix(".json")
        if not path.startswith(LISTS) or not path.endswith(".json") or "/" in name:
            continue
        try:
            lists[name] = allowlists.load(rows.read_text(path), name)
        except (OSError, ValueError) as e:
            faults.append(str(e))
    return lists, faults


def _pair(target: str) -> tuple[str, str]:
    file, _, code = target.partition(": ")
    return file, code


def ruff_pin(lists: dict[str, allowlists.AllowList]) -> list[str]:
    """ruff.toml's per-file waivers and ruff.json's targets, both ways; a global ignore is red. The pin reads
    [lint]'s per-file tables only, so every other place ruff takes a waiver from is red (BK-B2, measured with the
    pinned ruff 0.17.3): `extend` (another file's settings, maybe outside every law path), and the deprecated
    top-level `ignore`, `extend-ignore`, `per-file-ignores` and `extend-per-file-ignores`, which ruff still honours.
    Refused rather than read: one table to pin both ways, and nothing ruff itself warns about."""
    try:
        with open(RUFF_TOML, "rb") as f:
            top = tomllib.load(f)
    except (OSError, tomllib.TOMLDecodeError) as e:
        return [f"{RUFF_TOML}: {e}"]
    lint = top.get("lint", {})
    faults = []
    if "extend" in top:
        faults.append("ruff.toml sets extend: the ruff pin reads ruff.toml alone; write every setting there")
    faults += [
        f"ruff.toml sets top-level {key}: write per-file waivers under [lint] (ruff deprecates it)"
        for key in (*RUFF_GLOBAL, *RUFF_WAIVERS)
        if top.get(key)  # an empty one waives nothing, as for lint.ignore
    ]
    faults += [f"ruff.toml sets lint.{key}: write per-file waivers" for key in RUFF_GLOBAL if lint.get(key)]
    waived = {(file, code) for key in RUFF_WAIVERS for file, codes in lint.get(key, {}).items() for code in codes}
    listed = {_pair(e.target) for e in lists["ruff"].entries} if "ruff" in lists else set()
    for file, code in sorted(waived - listed):
        faults.append(f"ruff.toml waives {code} for {file} without an entry in tools/allowlists/ruff.json")
    for file, code in sorted(listed - waived):
        faults.append(f"tools/allowlists/ruff.json lists {file}: {code}, which ruff.toml does not waive")
    return faults


def stale(lists: dict[str, allowlists.AllowList], tracked: set[str]) -> list[str]:
    """ruff.json targets naming a file git does not track; teeth.json targets naming a row the registry lacks."""
    faults = []
    if "ruff" in lists:
        faults += [
            f"tools/allowlists/ruff.json: {e.target}: does not exist (a stale target)"
            for e in lists["ruff"].entries
            if _pair(e.target)[0] not in tracked
        ]
    if "teeth" in lists and lists["teeth"].entries:
        try:
            ids = {str(row["id"]) for row in json.loads(rows.read_text(REGISTRY))["rows"]}
        except SIDE_FAULTS as e:
            return [*faults, f"{REGISTRY}: {e}"]
        faults += [
            f"tools/allowlists/teeth.json: {e.target}: does not exist (a stale target)"
            for e in lists["teeth"].entries
            if e.target not in ids
        ]
    return faults


def in_code(files: list[str]) -> tuple[list[suppressions.Directive], list[str], int]:
    """(directives and markers, faults, files processed) over the listed files."""
    directives: list[suppressions.Directive] = []
    faults: list[str] = []
    processed = 0
    for path in files:
        processed += 1
        if not suppressions.scoped(path):
            continue  # a side input (a list, ruff.toml): processed, its exemptions read below
        try:
            source = rows.read_text(path)
        except OSError:
            faults.append(f"{path}: listed but not in the work tree")
            continue
        found, wrong = suppressions.read(path, source)
        directives += found
        faults += wrong
    return directives, faults, processed


def main(argv: list[str]) -> int:
    row_dir = rows.start()
    files = sorted(rows.show(path) for path in rows.read_list(argv[1]))
    directives, file_faults, processed = in_code(files)
    rows.covered(processed)
    try:
        tracked = tracked_files(row_dir)
        ctx = context(set(tracked))
    except SIDE_FAULTS as e:
        return rows.finish([f"suppressions: {e}"])
    lists, faults = load_lists(tracked)
    for name in sorted(lists):
        faults += allowlists.check(lists[name], ctx)
    faults += ruff_pin(lists)
    faults += stale(lists, set(tracked))
    faults += file_faults
    faults += suppressions.check(directives, ctx, suppressions.ENABLED)
    return rows.finish([f"suppressions: {fault}" for fault in faults])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
