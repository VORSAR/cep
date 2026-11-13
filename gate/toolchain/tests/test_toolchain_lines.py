"""Both go.mod toolchain lines name tools.json's Go (owner, 2026-11-13, during S1.1c-4's build, decision 3).

No other check compares them, so a Go bump in tools.json could leave either line stale with nothing red. The pin is
read with the manifest's own loader; the failure names the file and both versions. A scratch copy whose go.mod says
another toolchain is the witness that the check reads the line it names.

No test here names a Go version: each reads the pin from SOURCE's tools.json, so a Go bump edits tools.json and the two
go.mod lines only, and under the weekly canary's candidate (tools.json's go moved, the go.mod lines not) every test
here fails only by the stale-line rule, its exception line naming each stale line in full ("tools.json pins go…"), the
text the canary's pin pattern for these tests finds (gate/canary/pin_tests.json; review CA-S1).
"""

import os
import shutil

from gate.toolchain import manifest
from gate.toolchain.tests import support

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
GOMODS = ("go.mod", "gate/proofs/go.mod")


def stale_lines(repo: str) -> list[str]:
    """One text per go.mod whose `toolchain` line is not tools.json's go version (a missing line is one too)."""
    tools = manifest.load(os.path.join(repo, "tools.json"), manifest.platform_key())
    pinned = next(tool.version for tool in tools if tool.name == "go")
    found = []
    for rel in GOMODS:
        with open(os.path.join(repo, rel), encoding="utf-8") as f:
            lines = [line.split() for line in f.read().splitlines()]
        named = [words[1] for words in lines if len(words) == 2 and words[0] == "toolchain"]
        said = f"says toolchain {named[0]}" if len(named) == 1 else f"has {len(named)} toolchain lines"
        if named != [pinned]:
            found.append(f"{rel} {said}, tools.json pins {pinned}: update the toolchain line with the bump")
    return found


class ToolchainLines(support.ToolchainTest):
    SOURCE = REPO  # the tree whose tools.json and go.mod files the tests read (a candidate's under test_fix_s11c4)
    STALE = "go1.0.0"  # a toolchain no pin can be: Go 1.0 predates go.mod

    def copy_repo(self) -> str:
        for rel in ("tools.json", *GOMODS):
            dest = os.path.join(self.repo, rel)
            os.makedirs(os.path.dirname(dest), exist_ok=True)
            shutil.copyfile(os.path.join(self.SOURCE, rel), dest)
        return self.repo

    def pinned(self, repo: str) -> str:
        tools = manifest.load(os.path.join(repo, "tools.json"), manifest.platform_key())
        return next(tool.version for tool in tools if tool.name == "go")

    def assert_agrees(self, repo: str, what: str) -> None:
        """Every toolchain line in `repo` names its tools.json's go; a failure's first line names each stale line."""
        if stale := stale_lines(repo):
            self.fail(f"{what}: {'; '.join(stale)}")

    def edit(self, repo: str, rel: str, old: str, new: str) -> None:
        path = os.path.join(repo, rel)
        with open(path, encoding="utf-8") as f:
            text = f.read()
        self.assertEqual(text.count(old), 1, f"{rel} holds {old!r} once")
        with open(path, "w", encoding="utf-8") as f:
            f.write(text.replace(old, new))

    def test_both_toolchain_lines_name_tools_jsons_go(self):
        self.assert_agrees(self.SOURCE, "the repository")

    def test_a_stale_toolchain_line_is_named_with_both_versions(self):
        repo = self.copy_repo()
        self.assert_agrees(repo, "the witness, an unedited copy")
        pinned = self.pinned(repo)
        self.assertNotEqual(pinned, self.STALE)
        self.edit(repo, "go.mod", f"\ntoolchain {pinned}\n", f"\ntoolchain {self.STALE}\n")
        self.assertEqual(
            stale_lines(repo),
            [f"go.mod says toolchain {self.STALE}, tools.json pins {pinned}: update the toolchain line with the bump"],
        )

    def test_a_missing_toolchain_line_is_named(self):
        repo = self.copy_repo()
        self.assert_agrees(repo, "the witness, an unedited copy")
        pinned = self.pinned(repo)
        self.edit(repo, "gate/proofs/go.mod", f"\ntoolchain {pinned}\n", "\n")
        self.assertEqual(
            stale_lines(repo),
            [
                f"gate/proofs/go.mod has 0 toolchain lines, tools.json pins {pinned}: "
                "update the toolchain line with the bump"
            ],
        )
