"""Both go.mod toolchain lines name tools.json's Go (owner, 2026-11-13, during S1.1c-4's build, decision 3).

No other check compares them, so a Go bump in tools.json could leave either line stale with nothing red. The pin is
read with the manifest's own loader; the failure names the file and both versions. A scratch copy whose go.mod says
another toolchain is the witness that the check reads the line it names.
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
    def copy_repo(self) -> str:
        for rel in ("tools.json", *GOMODS):
            dest = os.path.join(self.repo, rel)
            os.makedirs(os.path.dirname(dest), exist_ok=True)
            shutil.copyfile(os.path.join(REPO, rel), dest)
        return self.repo

    def test_both_toolchain_lines_name_tools_jsons_go(self):
        self.assertEqual(stale_lines(REPO), [])

    def test_a_stale_toolchain_line_is_named_with_both_versions(self):
        repo = self.copy_repo()
        self.assertEqual(stale_lines(repo), [], "the witness: an unedited copy agrees")
        path = os.path.join(repo, "go.mod")
        with open(path, encoding="utf-8") as f:
            text = f.read()
        self.assertIn("\ntoolchain go1.27.4\n", text)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text.replace("\ntoolchain go1.27.4\n", "\ntoolchain go1.27.3\n"))
        self.assertEqual(
            stale_lines(repo),
            ["go.mod says toolchain go1.27.3, tools.json pins go1.27.4: update the toolchain line with the bump"],
        )

    def test_a_missing_toolchain_line_is_named(self):
        repo = self.copy_repo()
        path = os.path.join(repo, "gate/proofs/go.mod")
        with open(path, encoding="utf-8") as f:
            text = f.read()
        with open(path, "w", encoding="utf-8") as f:
            f.write(text.replace("\ntoolchain go1.27.4\n", "\n"))
        self.assertEqual(
            stale_lines(repo),
            [
                "gate/proofs/go.mod has 0 toolchain lines, tools.json pins go1.27.4: "
                "update the toolchain line with the bump"
            ],
        )
