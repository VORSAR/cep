"""The coverage-inclusion instrument (plan check S6 as amended by design note §5; ADR-0015 §2.1, §2.2).

    python3.15 -I -B -X utf8 gate/coverage/main.py --floor 90 [--profile P] [--events E] [--module-dir .]

--floor is required (S1.1c-2 design §4.4): the registry passes it, so the floor is a LAW Δ line, never a default.

--profile and --events default to the `unit` row's artifacts beside this row's own directory:
dirname($GATE_ROW_DIR)/unit/cover.prof and dirname($GATE_ROW_DIR)/unit/test.json (plan check S3d).

The scope is fixed: `go list` of ./internal/... . ./rule/... ./io/... in --module-dir. A pattern that matches
nothing is not an error, so a later rule/x enters scope with no flag or registry edit (QA C30); gate/, cmd/ and the
rest can never match. Each real package gets one line, PASS or FAIL, from gate/coverage/classify.py; then one
`GATE_COVERED <packages evaluated>`, which the runner compares with its own live expansion of the same patterns.

Exit 0: every package passed. Exit 1: any FAIL, including an artifact the unit row did not write and zero packages
in scope. Exit 2: a usage or internal error. Child processes stay in this row's process group, so the runner's
watchdog reaches them, and their output goes to files.
"""

import argparse
import os
import re
import signal
import subprocess
import sys
import tempfile
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate.coverage import classify, coverprofile
from gate.runner import jsonstream

SCOPE = ("./internal/...", ".", "./rule/...", "./io/...")
NEEDED = "unit"
PROFILE_NAME = "cover.prof"
EVENTS_NAME = "test.json"
# ADR-0015 §2.1's canonical header, on the first line. A bytes pattern: `.` never crosses the line's end.
GENERATED = re.compile(rb"// Code generated .* DO NOT EDIT\.")
_FLOOR = re.compile(r"100|[1-9][0-9]?")
_NOT_A_PACKAGE = ("no Go files", "matched no packages")  # the runner's `go` scope rule (design note §4)
FIX = "fix: each FAIL package needs tests that build, run and cover at least {floor}% of its statements (ADR-0015 §2.2)"


class UsageError(Exception):
    """The command line or its environment is unusable: exit 2."""


class Failure(Exception):
    """An input the instrument needs is missing or unreadable: a FAIL line, exit 1."""


@dataclass(frozen=True)
class Options:
    profile: str
    events: str
    floor: int
    module_dir: str
    scratch: str | None  # where go list's output goes: this row's directory, if there is one


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise UsageError(f"{message}\n{self.format_usage().strip()}")


def options(argv: Sequence[str], environ: Mapping[str, str]) -> Options:
    parser = _Parser(prog="gate/coverage/main.py", add_help=False, allow_abbrev=False)
    parser.add_argument("--profile")
    parser.add_argument("--events")
    parser.add_argument("--floor", required=True)
    parser.add_argument("--module-dir", dest="module_dir", default=".")
    args = parser.parse_args(list(argv))
    if not _FLOOR.fullmatch(args.floor):
        raise UsageError(f"--floor must be a decimal from 1 to 100, got {args.floor!r}")
    if not os.path.isdir(args.module_dir):
        raise UsageError(f"--module-dir {args.module_dir} is not a directory")
    row_dir = environ.get("GATE_ROW_DIR") or None
    if row_dir is None and (args.profile is None or args.events is None):
        raise UsageError("GATE_ROW_DIR is not set: pass --profile and --events, or run as a gate row")
    unit = os.path.join(os.path.dirname(os.path.abspath(row_dir)), NEEDED) if row_dir else ""
    return Options(
        profile=args.profile or os.path.join(unit, PROFILE_NAME),
        events=args.events or os.path.join(unit, EVENTS_NAME),
        floor=int(args.floor),
        module_dir=args.module_dir,
        scratch=row_dir,
    )


def _read_artifact(path: str) -> bytes | None:
    """An artifact of the unit row; None if it is missing. Any other OSError is not ours to explain: it escapes."""
    try:
        with open(path, "rb") as f:
            return f.read()
    except FileNotFoundError:
        return None


def _is_real(entry: dict) -> bool:
    path = entry.get("ImportPath", "")
    if path in (".", "..") or path.startswith(("./", "../")):
        return False
    error = entry.get("Error") or {}
    return not any(text in error.get("Err", "") for text in _NOT_A_PACKAGE)


def _generated(directory: str, names: Sequence[str]) -> frozenset[str]:
    found = set()
    for name in names:
        with open(os.path.join(directory, name), "rb") as f:
            first = f.readline().removesuffix(b"\n")
        if GENERATED.fullmatch(first):
            found.add(name)
    return frozenset(found)


def packages(module_dir: str, scratch: str) -> list[classify.Package]:
    """The real packages the scope patterns match, each with its generated files."""
    out, err = os.path.join(scratch, "golist.stdout"), os.path.join(scratch, "golist.stderr")
    with open(out, "wb") as o, open(err, "wb") as e:
        code = subprocess.run(
            ["go", "list", "-e", "-json", *SCOPE], cwd=module_dir, stdin=subprocess.DEVNULL, stdout=o, stderr=e
        ).returncode
    if code != 0:
        with open(err, "rb") as e:
            detail = " ".join(e.read().decode("utf-8", errors="replace").split())
        raise Failure(f"go list exit {code}: {detail}")
    with open(out, "rb") as o:
        try:
            entries = jsonstream.parse(o.read())
        except ValueError as e:
            raise Failure(f"go list: unreadable output: {e}") from None
    found = []
    for entry in entries:
        if _is_real(entry):
            names = [*(entry.get("GoFiles") or ()), *(entry.get("CgoFiles") or ())]
            found.append(classify.Package(entry["ImportPath"], _generated(entry.get("Dir", ""), names)))
    return sorted(found, key=lambda p: p.path)


def evaluate(opts: Options, scratch: str) -> tuple[list[str], int, bool]:
    """(the lines to print, packages evaluated, whether every one passed)."""
    profile, events = _read_artifact(opts.profile), _read_artifact(opts.events)
    missing = [p for p, data in ((opts.profile, profile), (opts.events, events)) if data is None]
    if missing:
        raise Failure(*(f"{path} is missing: the {NEEDED} row did not write it" for path in missing))
    assert profile is not None and events is not None
    try:
        blocks = coverprofile.parse(profile)
    except coverprofile.ProfileError as e:
        raise Failure(f"{opts.profile}: {e}") from None
    try:
        stream = classify.parse_events(events)
    except classify.EventsError as e:
        raise Failure(f"{opts.events}: {e}") from None
    if not stream:
        raise Failure(f"{opts.events} holds no events: the {NEEDED} row did not finish")
    found = packages(opts.module_dir, scratch)
    if not found:
        raise Failure("no package in scope: zero is a failure")
    verdicts = classify.evaluate(found, blocks, stream, opts.floor)
    ok = all(v.ok for v in verdicts)
    lines = [v.line() for v in verdicts] + ([] if ok else [FIX.format(floor=opts.floor)])
    return lines, len(verdicts), ok


def _say(*lines: str) -> None:
    for line in lines:
        print(line)
    sys.stdout.flush()


def main(argv: Sequence[str], environ: Mapping[str, str] = os.environ) -> int:
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)  # an ignored SIGCHLD survives exec and turns a failed go into 0
    try:
        opts = options(argv, environ)
    except UsageError as e:
        _say(str(e), "GATE FAILED", "exit: 2")
        return 2
    with tempfile.TemporaryDirectory(prefix="gate-coverage-") as tmp:
        try:
            lines, evaluated, ok = evaluate(opts, opts.scratch or tmp)
        except Failure as e:
            _say(*(f"FAIL {reason}" for reason in e.args), "GATE_COVERED 0")
            return 1
    _say(*lines, f"GATE_COVERED {evaluated}")
    return 0 if ok else 1


if __name__ == "__main__":
    try:
        code = main(sys.argv[1:])
    except BaseException as e:
        # Exit 1 means "coverage failed"; a crash must never pass for it. SystemExit carries the 2 out.
        _say(f"internal error: {type(e).__name__}: {e}", "GATE FAILED", "exit: 2")
        raise SystemExit(2) from e
    sys.exit(code)
