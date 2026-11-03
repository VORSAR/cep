"""Break-it of S1.1a-1's combined build (QA), QB6: equivalent spellings walk past ruff.toml's two false-green bans.

ruff.toml bans os.wait, os.wait3 and os.wait4 ("never reap children behind subprocess's back: it turns a failure
into exit 0") and signal.SIG_IGN ("an ignored SIGCHLD reports a failed child as exit 0"). TID251 matches qualified
names only, so os.waitpid(-1, 0) and os.waitid(os.P_ALL, ...), which reap any child exactly as os.wait does, and
signal.Handlers.SIG_IGN, the same value as signal.SIG_IGN, pass `ruff check` under the real ruff.toml. The
witnesses: the banned spellings are flagged in the same world, with the same flags as the ruff row.
"""

import os
import shutil

from gate.runner.tests import support

PLANT = "gate/plant.py"


class QB6EquivalentSpellings(support.GateTest):
    def setUp(self):
        super().setUp()
        shutil.copy(os.path.join(support.REPO, "ruff.toml"), os.path.join(self.repo, "ruff.toml"))
        self.ruff = os.path.join(support.tool_bin("ruff"), "ruff")

    def check(self, line: str) -> tuple[int, str]:
        """ruff check over a module that imports exactly what the line uses, so TID251 is the only possible finding."""
        module = line.split(".", 1)[0]
        self.write(PLANT, f"import {module}\n\n{line}\n")
        argv = [self.ruff, "check", "--ignore-noqa", "--no-cache", "--config", "ruff.toml", "--", PLANT]
        code, out, err = support.run_tool(argv, self.repo)
        return code, (out + err).decode("utf-8", errors="replace")

    def assertBanned(self, line: str) -> None:
        code, out = self.check(line)
        self.assertEqual(code, 1, f"`{line}` passes ruff check under the real ruff.toml:\n{out}")
        self.assertIn("TID251", out)

    def test_qb6_witness_the_banned_spellings_are_flagged(self):
        for line in ("os.wait()", "os.wait4(-1, 0)", "signal.signal(signal.SIGCHLD, signal.SIG_IGN)"):
            with self.subTest(line):
                self.assertBanned(line)

    def test_qb6_waitpid_on_any_child_is_banned_like_os_wait(self):
        self.assertBanned("os.waitpid(-1, 0)")

    def test_qb6_waitid_on_any_child_is_banned_like_os_wait(self):
        self.assertBanned("os.waitid(os.P_ALL, 0, os.WEXITED)")

    def test_qb6_the_ignore_disposition_by_its_enum_name_is_banned(self):
        self.assertBanned("signal.signal(signal.SIGCHLD, signal.Handlers.SIG_IGN)")
