"""The parity hand-off (proof-world speed-up design note §3.3-§3.5, tests 6-14; owner, 2026-11-14, the speed-up's
measurement, decision 2, and its design reviews, decisions 2-3).

A full-tree check whose every tool passed, at least one tree hashed for real, writes `<scratch>/handoff.json`: each
hashed tool's name, the realpath its tree child resolved, that tree's details and its pin. A later check given that
file takes a tool's tree without hashing it only on an exact match (the name, the realpath, this check's own pin and
a fresh details digest from one capped child); anything else gets a real hash. The spawn here runs the real child and
records every child it starts, so the tests count tree children and details children.

Expected values are written out here: the hand-off's bytes from the design note's canonical form, each tree's details
from `manifest.tree_details` (pinned field by field in test_tree_details), each pin by hand (test_parity).
"""

import dataclasses
import json
import os
import shutil
import stat
import unittest

from gate.toolchain import manifest, parity
from gate.toolchain.tests import support
from gate.toolchain.tests.test_parity import RECOVER, ParityTest, write_file

RUN_A = "20261114T000000Z-aaaaaa"
RUN_B = "20261114T000001Z-bbbbbb"
DETAILS = "--details"


def canonical(doc: object) -> bytes:
    return (json.dumps(doc, sort_keys=True, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")


def point(link: str, target: str) -> None:
    """Retarget a symlink in one step, as a reinstall or a hostile hand would."""
    os.symlink(target, link + ".tmp")
    os.replace(link + ".tmp", link)


class Children:
    """A spawn that runs the real child in a new session and records every argv it was given."""

    def __init__(self, test: unittest.TestCase, plant=None) -> None:
        self.test = test
        self.plant = plant  # argv -> the argv to run instead, or None to run it as given
        self.argvs: list[list[str]] = []

    def __call__(self, argv, **kw):
        self.argvs.append(list(argv))
        planted = self.plant(list(argv)) if self.plant is not None else None
        proc = parity.popen(list(argv) if planted is None else planted, **kw)
        self.test.addCleanup(support.kill_group, proc.pid)
        return proc

    @property
    def trees(self) -> list[str]:
        """The home each tree child was given."""
        return [argv[-1] for argv in self.argvs if DETAILS not in argv]

    @property
    def details(self) -> list[list[str]]:
        """The homes each details child was given."""
        return [argv[argv.index(DETAILS) + 1 :] for argv in self.argvs if DETAILS in argv]


class HandoffTest(ParityTest):
    def tools(self) -> list[manifest.Tool]:
        """Two tools, each with a plain data file that no exec_sha256 entry lists: only the tree hash covers it."""
        tools = []
        for name in ("ruff", "go"):  # not sorted: tools.json order is what the hand-off keeps
            tool = self.tool(name, files={"share/data": b"data\n"})
            exec_only = {rel: pin for rel, pin in tool.exec_sha256.items() if rel != "share/data"}
            tools.append(dataclasses.replace(tool, exec_sha256=exec_only))
        return tools

    def run_check(
        self,
        tools: list[manifest.Tool],
        label: str,
        *,
        run_id: str,
        handoff_in: str | None = None,
        cache: str | None = None,
        full_tree: bool = True,
        plant=None,
        spawn=None,
    ) -> tuple[parity.Outcome, Children, str]:
        scratch = os.path.join(self.world, "runs", label, "parity")
        os.makedirs(scratch)
        children = Children(self, plant)
        outcome = parity.check(
            tools,
            self.cache if cache is None else cache,
            self.env,
            full_tree=full_tree,
            spawn=children if spawn is None else spawn,
            scratch=scratch,
            run_id=run_id,
            handoff_in=handoff_in,
        )
        return outcome, children, scratch

    def entry(self, tool: manifest.Tool, home: str | None = None) -> dict:
        real = os.path.realpath(self.home(tool.name) if home is None else home)
        return {
            "details": manifest.tree_details(real),
            "home": real,
            "name": tool.name,
            "tree_sha256": tool.tree_sha256,
        }

    def written(self, scratch: str) -> str:
        """Check A's hand-off, which the later checks are given."""
        path = os.path.join(scratch, "handoff.json")
        self.assertTrue(os.path.exists(path), f"no hand-off at {path}")
        return path


class Writer(HandoffTest):
    def test_a_full_tree_pass_writes_the_hand_off(self):
        tools = self.tools()
        outcome, children, scratch = self.run_check(tools, "a", run_id=RUN_A)
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=0, taken_from=""))
        self.assertEqual(children.trees, [self.home("ruff"), self.home("go")])
        path = self.written(scratch)
        want = {"details_format": 1, "run_id": RUN_A, "schema": 1, "tools": [self.entry(tool) for tool in tools]}
        with open(path, "rb") as f:
            self.assertEqual(f.read(), canonical(want))
        self.assertEqual(stat.S_IMODE(os.lstat(path).st_mode), 0o444)
        # No .tmp is left behind beside the trees' own output.
        trees = sorted(f"tree-{tool.name}.{end}" for tool in tools for end in ("err", "out"))
        self.assertEqual(sorted(os.listdir(scratch)), ["handoff.json", *trees])

    def test_a_failed_parity_writes_no_hand_off(self):
        ruff, go = self.tools()
        write_file(os.path.join(self.home("go"), "share", "extra"), b"planted\n", 0o444)
        outcome, children, scratch = self.run_check([ruff, go], "a", run_id=RUN_A)
        want = parity.Outcome(reasons=(f"go: tree differs: {RECOVER}",), covered=1, taken=0, taken_from="")
        self.assertEqual(outcome, want)
        self.assertEqual(children.trees, [self.home("ruff"), self.home("go")])  # the witness: ruff's tree was hashed
        with open(os.path.join(scratch, "tree-ruff.out"), encoding="utf-8") as f:
            self.assertEqual(f.read().splitlines()[2], ruff.tree_sha256)
        self.assertFalse(os.path.exists(os.path.join(scratch, "handoff.json")))

    def test_a_run_that_took_every_tree_writes_no_hand_off(self):
        tools = self.tools()
        _, _, scratch_a = self.run_check(tools, "a", run_id=RUN_A)
        outcome, children, scratch_b = self.run_check(tools, "b", run_id=RUN_B, handoff_in=self.written(scratch_a))
        self.assertEqual((outcome.reasons, outcome.taken), ((), 2))  # the witness: B passed and took both
        self.assertEqual(children.trees, [])
        self.assertFalse(os.path.exists(os.path.join(scratch_b, "handoff.json")))

    def test_a_check_below_full_tree_writes_no_hand_off(self):
        outcome, children, scratch = self.run_check(self.tools(), "a", run_id=RUN_A, full_tree=False)
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=0, taken_from=""))
        self.assertEqual(children.argvs, [])
        self.assertFalse(os.path.exists(os.path.join(scratch, "handoff.json")))


class Reader(HandoffTest):
    def test_a_hand_off_written_by_one_check_is_taken_by_the_next(self):
        # QB-S7: the writer and the reader together, over the same untouched trees.
        tools = self.tools()
        _, _, scratch_a = self.run_check(tools, "a", run_id=RUN_A)
        outcome, children, _ = self.run_check(tools, "b", run_id=RUN_B, handoff_in=self.written(scratch_a))
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=2, taken_from=RUN_A))
        self.assertEqual(children.trees, [])
        self.assertEqual(children.details, [[self.home("ruff"), self.home("go")]])

    def test_a_linked_cache_takes_the_same_hand_off(self):
        # The proof worlds' shape (link_cache): a cache of links to the real homes, so the realpaths are the same.
        tools = self.tools()
        _, _, scratch_a = self.run_check(tools, "a", run_id=RUN_A)
        linked = os.path.join(self.world, "linked")
        os.makedirs(linked)
        for tool in tools:
            os.symlink(self.home(tool.name), os.path.join(linked, os.path.basename(self.home(tool.name))))
        outcome, children, _ = self.run_check(
            tools, "b", run_id=RUN_B, handoff_in=self.written(scratch_a), cache=linked
        )
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=2, taken_from=RUN_A))
        self.assertEqual(children.trees, [])
        self.assertEqual(
            children.details, [[os.path.join(linked, os.path.basename(self.home(name))) for name in ("ruff", "go")]]
        )

    def test_each_mismatch_is_hashed_for_real(self):
        def mode(home: str) -> None:
            os.chmod(os.path.join(home, "share", "data"), 0o554)  # still executable: the tree hash is unchanged

        def content(home: str) -> None:
            path = os.path.join(home, "share", "data")
            st = os.lstat(path)
            os.chmod(path, 0o755)
            with open(path, "r+b") as f:
                f.write(b"DATA\n")
            os.chmod(path, stat.S_IMODE(st.st_mode))
            os.utime(path, ns=(st.st_atime_ns, st.st_mtime_ns))
            after = os.lstat(path)
            self.assertEqual(
                (after.st_mode, after.st_size, after.st_mtime_ns), (st.st_mode, st.st_size, st.st_mtime_ns)
            )

        def added(home: str) -> None:
            write_file(os.path.join(home, "share", "extra"), b"planted\n", 0o444)

        def removed(home: str) -> None:
            os.unlink(os.path.join(home, "share", "data"))

        # In place, after the tick wait: each moves the planted entry's own change time; `passes` is the real hash's
        # verdict (a mode that keeps the execute bits leaves the tree hash as pinned).
        for change, rel, passes in (
            (mode, "share/data", True),
            (content, "share/data", False),
            (added, "share", False),
            (removed, "share", False),
        ):
            with self.subTest(change=change.__name__):
                tools = self.fresh(change.__name__)
                _, _, scratch_a = self.run_check(tools, f"{change.__name__}-a", run_id=RUN_A)
                home = self.home("go")
                planted = os.path.join(home, rel)
                ctime = support.past_the_ctime_of(planted, os.path.join(self.world, "probe"))
                change(home)
                self.assertGreater(os.lstat(planted).st_ctime_ns, ctime, "the witness: the change moved its ctime")
                outcome, children, _ = self.run_check(
                    tools, f"{change.__name__}-b", run_id=RUN_B, handoff_in=self.written(scratch_a)
                )
                self.assertEqual(children.details, [[self.home("ruff"), home]])
                self.assertEqual(children.trees, [home])  # exactly that tool gets a real tree child
                want = () if passes else (f"go: tree differs: {RECOVER}",)
                self.assertEqual((outcome.reasons, outcome.taken, outcome.taken_from), (want, 1, RUN_A))
        for other in (self.realpath_differs, self.pin_differs, self.details_child_fails):
            with self.subTest(change=other.__name__):
                other()

    def realpath_differs(self) -> None:
        """go's home in B's cache links to a real copy: the same bytes, another realpath. Only ruff is a candidate."""
        tools = self.fresh("realpath")
        _, _, scratch_a = self.run_check(tools, "realpath-a", run_id=RUN_A)
        copy = os.path.join(self.world, "elsewhere", os.path.basename(self.home("go")))
        shutil.copytree(self.home("go"), copy, symlinks=True)
        linked = os.path.join(self.world, "linked")
        os.makedirs(linked)
        homes = {name: os.path.join(linked, os.path.basename(self.home(name))) for name in ("ruff", "go")}
        os.symlink(self.home("ruff"), homes["ruff"])
        os.symlink(copy, homes["go"])
        self.env["PATH"] = os.pathsep.join(os.path.join(homes[name], "bin") for name in ("ruff", "go"))
        outcome, children, _ = self.run_check(
            tools, "realpath-b", run_id=RUN_B, handoff_in=self.written(scratch_a), cache=linked
        )
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=1, taken_from=RUN_A))
        self.assertEqual(children.details, [[homes["ruff"]]])
        self.assertEqual(children.trees, [homes["go"]])

    def pin_differs(self) -> None:
        """B's own pin for go is not the entry's: go is no candidate, and its real hash names the difference."""
        ruff, go = self.fresh("pin")
        _, _, scratch_a = self.run_check([ruff, go], "pin-a", run_id=RUN_A)
        repinned = dataclasses.replace(go, tree_sha256="ee" * 32)
        outcome, children, _ = self.run_check(
            [ruff, repinned], "pin-b", run_id=RUN_B, handoff_in=self.written(scratch_a)
        )
        self.assertEqual(outcome.reasons, (f"go: tree differs: {RECOVER}",))
        self.assertEqual((outcome.covered, outcome.taken, outcome.taken_from), (1, 1, RUN_A))
        self.assertEqual(children.details, [[self.home("ruff")]])
        self.assertEqual(children.trees, [self.home("go")])

    def details_child_fails(self) -> None:
        """One capped child reads every candidate's details (CA-L2), so when it fails, each of them is hashed."""
        tools = self.fresh("details")
        _, _, scratch_a = self.run_check(tools, "details-a", run_id=RUN_A)

        def fails(argv: list[str]) -> list[str] | None:
            return [argv[0], "-I", "-B", "-c", "import sys; sys.exit(1)"] if DETAILS in argv else None

        outcome, children, _ = self.run_check(
            tools, "details-b", run_id=RUN_B, handoff_in=self.written(scratch_a), plant=fails
        )
        self.assertEqual(children.details, [[self.home("ruff"), self.home("go")]])  # the witness: it was started
        self.assertEqual(children.trees, [self.home("ruff"), self.home("go")])
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=0, taken_from=""))

    def test_a_home_retargeted_during_the_hash_is_not_listed(self):
        # QB-S9: the entry's home is the realpath the child resolved and hashed, so a later check of the restored
        # link resolves elsewhere and hashes for real.
        (go,) = [tool for tool in self.tools() if tool.name == "go"]
        home = self.home("go")
        first = os.path.join(self.world, "trees", "first")
        os.makedirs(os.path.dirname(first))
        os.rename(home, first)
        second = os.path.join(self.world, "trees", "second")
        shutil.copytree(first, second, symlinks=True)
        os.symlink(first, home)

        def retargeting(argv, **kw):
            if DETAILS in argv or argv[-1] != home:
                return parity.popen(argv, **kw)
            point(home, second)
            proc = parity.popen(argv, **kw)
            proc.wait()  # the child resolved and hashed the second tree; parity's own wait then reads its exit
            point(home, first)
            return proc

        _, _, scratch_a = self.run_check([go], "a", run_id=RUN_A, spawn=retargeting)
        path = self.written(scratch_a)
        with open(path, "rb") as f:
            listed = json.loads(f.read())["tools"]
        self.assertEqual([entry["home"] for entry in listed], [os.path.realpath(second)])
        self.assertEqual(os.path.realpath(home), os.path.realpath(first))  # the link is back
        outcome, children, _ = self.run_check([go], "b", run_id=RUN_B, handoff_in=path)
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=1, taken=0, taken_from=""))
        self.assertEqual((children.details, children.trees), ([], [home]))

    def test_below_t2_the_hand_off_is_never_read(self):
        tools = self.tools()
        missing = os.path.join(self.world, "no-such-handoff.json")
        outcome, _, _ = self.run_check(tools, "full", run_id=RUN_B, handoff_in=missing)
        self.assertEqual(outcome.covered, 0)  # the witness: at full tree, the same path is a fault
        outcome, children, _ = self.run_check(tools, "t1", run_id=RUN_B, handoff_in=missing, full_tree=False)
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=2, taken=0, taken_from=""))
        self.assertEqual(children.argvs, [])

    def fresh(self, label: str) -> list[manifest.Tool]:
        """Two tools in a cache of this subtest's own, with PATH naming only them."""
        self.cache = os.path.join(self.world, f"cache-{label}")
        self.env = {"PATH": os.defpath}
        return self.tools()


VALID = {
    "details_format": 1,
    "run_id": RUN_A,
    "schema": 1,
    "tools": [{"details": "ab" * 32, "home": "/nowhere/go", "name": "go", "tree_sha256": "cd" * 32}],
}


def tool_with(**fields) -> dict:
    return {**VALID, "tools": [{**VALID["tools"][0], **fields}]}


class Faults(HandoffTest):
    """Each §3.5 fault fails parity before any tool is checked, naming the path and the fault (QB-S10)."""

    def setUp(self) -> None:
        super().setUp()
        self.runs = 0

    def given(self, label: str, data: bytes) -> str:
        path = os.path.join(self.world, f"{label}.json")
        with open(path, "wb") as f:
            f.write(data)
        return path

    def assertFault(self, path: str, fault: str) -> None:
        marker = os.path.join(self.world, "probed")
        tool = self.tool("go", source=b"#!/bin/sh\n: > " + marker.encode() + b"\necho go 1.0\n")
        self.runs += 1
        outcome, children, scratch = self.run_check([tool], f"fault-{self.runs}", run_id=RUN_B, handoff_in=path)
        self.assertEqual(outcome.reasons, (f"parity: tree hand-off {path}: {fault}",))
        self.assertEqual((outcome.covered, outcome.taken, outcome.taken_from), (0, 0, ""))
        self.assertEqual(children.argvs, [])
        self.assertFalse(os.path.exists(marker), "a tool was checked before the hand-off was read")
        self.assertFalse(os.path.exists(os.path.join(scratch, "handoff.json")))

    def test_a_valid_hand_off_names_no_fault(self):
        # The witness that each fault below is the one planted: the same document, untouched, is read, and its
        # entry (a home that is not this tool's) is hashed for real.
        tool = self.tool("go")
        outcome, children, _ = self.run_check(
            [tool], "valid", run_id=RUN_B, handoff_in=self.given("v", canonical(VALID))
        )
        self.assertEqual(outcome, parity.Outcome(reasons=(), covered=1, taken=0, taken_from=""))
        self.assertEqual(children.trees, [self.home("go")])

    def test_each_hand_off_fault_fails_parity_naming_it(self):
        top, entry, hex64 = (
            "keys must be details_format, run_id, schema, tools",
            "tools[0] keys must be details, home, name, tree_sha256",
            "must be 64 lowercase hex digits",
        )
        good = canonical(VALID)
        twice = good.replace(b'{"details_format":1,', b'{"details_format":1,"details_format":1,')
        cases = {
            "not UTF-8": (b"\xff\xfe" + good[2:], "not UTF-8"),
            "not JSON": (b"{\n", "not JSON"),
            "not canonical": (json.dumps(VALID, sort_keys=True).encode() + b"\n", "not canonical JSON"),
            "no trailing newline": (good[:-1], "not canonical JSON"),
            "a duplicate key": (twice, "not canonical JSON"),
            "over 64 KiB": (canonical(tool_with(home="/" + "x" * 70000)), "larger than 65536 bytes"),
            "not an object": (canonical([VALID]), top),
            "a missing key": (canonical({k: v for k, v in VALID.items() if k != "schema"}), top),
            "an extra key": (canonical({**VALID, "more": 1}), top),
            "schema 2": (canonical({**VALID, "schema": 2}), "schema must be 1, not 2"),
            "schema true": (canonical({**VALID, "schema": True}), "schema must be 1, not True"),
            "details_format 2": (canonical({**VALID, "details_format": 2}), "details_format must be 1, not 2"),
            "a run id outside the grammar": (canonical({**VALID, "run_id": "a/b"}), "run_id 'a/b' is not a run id"),
            "a run id of dots": (canonical({**VALID, "run_id": ".."}), "run_id '..' is not a run id"),
            "tools not a list": (canonical({**VALID, "tools": {}}), "tools must be a list"),
            "a tool not an object": (canonical({**VALID, "tools": ["go"]}), entry),
            "a tool's extra key": (canonical(tool_with(more=1)), entry),
            "a digest not 64 hex": (canonical(tool_with(tree_sha256="CD" * 32)), f"tools[0] tree_sha256 {hex64}"),
            "details not 64 hex": (canonical(tool_with(details="ab" * 31)), f"tools[0] details {hex64}"),
            "a relative home": (canonical(tool_with(home="cache/go")), "tools[0] home must be an absolute path"),
            "a home not a string": (canonical(tool_with(home=7)), "tools[0] home must be an absolute path"),
            "a name not a string": (canonical(tool_with(name=7)), "tools[0] name must be a string"),
            "a duplicate name": (
                canonical({**VALID, "tools": VALID["tools"] * 2}),
                "tools[1] name 'go' is listed twice",
            ),
        }
        for label, (data, fault) in cases.items():
            with self.subTest(fault=label):
                self.assertFault(self.given(label.replace(" ", "-"), data), fault)

    def test_an_unopenable_hand_off_names_its_strerror(self):
        target = self.given("target", canonical(VALID))
        link = os.path.join(self.world, "link.json")
        os.symlink(target, link)
        directory = os.path.join(self.world, "a-directory")
        os.makedirs(directory)
        for label, path, fault in (
            ("missing", os.path.join(self.world, "absent.json"), "No such file or directory"),
            ("a symlink", link, "Too many levels of symbolic links"),
            ("a directory", directory, "not a regular file"),
        ):
            with self.subTest(fault=label):
                self.assertFault(path, fault)

    def test_a_fifo_hand_off_is_refused_without_blocking(self):
        # QB-S10: a plain open() of a FIFO with no writer would block for ever; the test's ceiling would catch it.
        fifo = os.path.join(self.world, "handoff.fifo")
        os.mkfifo(fifo)
        self.assertFault(fifo, "not a regular file")


if __name__ == "__main__":
    unittest.main()
