"""The suppressions row (S1.1c-2 design §2.4): every exemption in the one grammar (DEVELOPMENT.md's suppression
policy; ADR-0015 §3.10).

It reads the context once — gate/sprints.json, gate/current-sprint, gate/known-issues.json's ids, the run's date,
GATE_TIER, and the tracked Markdown a ref may name — then loads and checks every tools/allowlists/*.json, and reads
each listed file: a .go file's //nolint directives and generated-code marker (listed by path in
tools/allowlists/generated.json, BK-S6), a gate/**/*.py or .githooks/* file's # fmt: skip markers.
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

from gate.runner import allowlists, anchors, pin, rows, sprintid, suppressions

LISTS = "tools/allowlists/"
RUFF_TOML = "ruff.toml"
RUFF_WAIVERS = ("per-file-ignores", "extend-per-file-ignores")
RUFF_GLOBAL = ("ignore", "extend-ignore")
_REAP = "never reap children behind subprocess's back (it turns a failure into exit 0)"
_IGNORE = "never ignore a signal: an ignored SIGCHLD reports a failed child as exit 0"
# ruff.toml as tomllib parses it, minus lint.per-file-ignores (S1.1c-5 design §5): pinned whole and type-strict
# through pin.differences, so a key that hides a finding (builtins, dummy-variable-rgx, a pycodestyle max-line-length,
# an exclude, a narrowed select, M-5) is named by its path. per-file-ignores is ruff.json's, checked both ways below.
# A file that needs per-file-target-version is edited here and in ruff.toml (both law paths).
RUFF_PINNED: dict[str, object] = {
    "target-version": "py315",
    "line-length": 120,
    "per-file-target-version": {
        "gate/bootstrap.py": "py39",
        "gate/toolchain/install.py": "py312",
        "gate/toolchain/manifest.py": "py312",
        "gate/run.py": "py312",
        "gate/runner/pyshim.py": "py312",
        "gate/runner/tests/test_run_guard.py": "py312",
        "gate/toolchain/tests/support.py": "py312",
        "gate/toolchain/tests/test_bootstrap.py": "py312",
        "gate/toolchain/tests/test_install.py": "py312",
        "gate/toolchain/tests/test_manifest.py": "py312",
        ".githooks/*": "py39",
        "gate/hooks/*.py": "py312",
        "gate/hooks/tests/test_launcher.py": "py312",
    },
    "lint": {
        "select": ["E", "W", "F", "I", "B", "UP", "SIM", "TID", "BLE", "S602", "S604", "S605"],
        "flake8-tidy-imports": {
            "banned-api": {
                "time.sleep": {"msg": "wait for a state through gate/wait.py, never for time (testing.md rule 1)"},
                "os.system": {"msg": "use subprocess with an argv list"},
                "os.wait": {"msg": _REAP},
                "os.wait3": {"msg": _REAP},
                "os.wait4": {"msg": _REAP},
                "os.waitpid": {"msg": _REAP},
                "os.waitid": {"msg": _REAP},
                "shutil.move": {"msg": "use os.rename: atomic, and never nests into an existing directory"},
                "signal.SIG_IGN": {"msg": _IGNORE},
                "signal.Handlers.SIG_IGN": {"msg": _IGNORE},
            }
        },
    },
}
REGISTRY = "gate/registry.json"
KNOWN_ISSUES = "gate/known-issues.json"
GENERATED_LIST = "tools/allowlists/generated.json"
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
    """ruff.toml pinned whole (S1.1c-5 design §5): the document minus lint.per-file-ignores equals RUFF_PINNED
    type-strictly, each difference named by its path (`pin.differences`); then ruff.toml's per-file waivers and
    ruff.json's targets, both ways. The deny-list texts below predate the whole pin (BK-B2, measured with ruff
    0.17.3): `extend` (another file's settings, maybe outside every law path), the deprecated top-level `ignore`,
    `extend-ignore`, `per-file-ignores` and `extend-per-file-ignores`, which ruff still honours, and a global
    lint ignore. The pin already names each; they stay as the clearer texts."""
    try:
        with open(RUFF_TOML, "rb") as f:
            top = tomllib.load(f)
    except (OSError, tomllib.TOMLDecodeError) as e:
        return [f"{RUFF_TOML}: {e}"]
    lint = top.get("lint", {})
    if type(lint) is not dict:
        return [f"{RUFF_TOML}: lint is not a table"]
    pinned = {**top, "lint": {key: value for key, value in lint.items() if key != "per-file-ignores"}}
    faults = pin.differences(pinned, RUFF_PINNED, RUFF_TOML)
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


def _marked(path: str) -> bool:
    """Whether the file carries the generated-code marker before its package clause; False if it cannot be read."""
    try:
        return bool(suppressions.go_generated(path, rows.read_text(path)))
    except OSError:
        return False


def generated(directives: list[suppressions.Directive], lists: dict[str, allowlists.AllowList]) -> list[str]:
    """A generated-code marker on a file tools/allowlists/generated.json does not list (BK-S6)."""
    listed = {e.target for e in lists["generated"].entries} if "generated" in lists else set()
    return [
        f"{d.path}:{d.line}: a generated file is listed by path in {GENERATED_LIST}"
        for d in directives
        if d.kind == "GENERATED" and d.path not in listed
    ]


def stale(lists: dict[str, allowlists.AllowList], tracked: set[str]) -> list[str]:
    """ruff.json targets naming a file git does not track; generated.json targets naming a file git does not track
    or one without the marker; teeth.json targets naming a row the registry lacks."""
    faults = []
    if "ruff" in lists:
        faults += [
            f"tools/allowlists/ruff.json: {e.target}: does not exist (a stale target)"
            for e in lists["ruff"].entries
            if _pair(e.target)[0] not in tracked
        ]
    for e in lists["generated"].entries if "generated" in lists else ():
        if e.target not in tracked:
            faults.append(f"{GENERATED_LIST}: {e.target}: does not exist (a stale target)")
        elif not _marked(e.target):
            faults.append(
                f"{GENERATED_LIST}: {e.target}: carries no generated marker before its package clause (a stale target)"
            )
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
    faults += generated(directives, lists)
    faults += file_faults
    faults += suppressions.check(directives, ctx, suppressions.ENABLED)
    return rows.finish([f"suppressions: {fault}" for fault in faults])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
