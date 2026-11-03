"""The proof drivers' world (gate/proofs/drivers/support.py): what it promises, it keeps.

A world must leave nothing behind. git 2.54's `commit` starts `git maintenance run --auto --detach`, a daemon that
outlives the command, escapes its process group and writes into .git while the world is being removed; so every git
a world starts runs with auto-maintenance and auto-gc off. git's own trace2 event log is the witness.
"""

import json
import os

from gate.proofs.drivers import support


class World(support.DriverTest):
    def test_a_commit_starts_no_detached_maintenance(self):
        trace = os.path.join(self.world.root, "trace2.json")
        self.world.env["GIT_TRACE2_EVENT"] = trace
        self.world.track("a.txt", "a\n")
        self.world.commit()
        with open(trace, "rb") as f:
            argvs = [e["argv"] for line in f.read().split(b"\n") if line and "argv" in (e := json.loads(line))]
        self.assertIn("commit", [word for argv in argvs for word in argv])  # the witness: the commit was traced
        self.assertEqual([argv for argv in argvs if "maintenance" in argv or "gc" in argv], [])

    def test_copies_are_real_files_never_links_to_the_repository(self):
        self.world.copy_gate()
        run_py = os.path.join(self.world.repo, "gate", "run.py")
        self.assertFalse(os.path.islink(run_py))
        self.assertNotEqual(os.stat(run_py).st_ino, os.stat(os.path.join(support.REPO, "gate", "run.py")).st_ino)

    def test_the_gate_copy_leaves_out_the_proofs_and_on_request_the_tests(self):
        self.world.copy_gate()
        gate = os.path.join(self.world.repo, "gate")
        self.assertTrue(os.path.isfile(os.path.join(gate, "coverage", "main.py")))  # the witness: a copy was made
        self.assertFalse(os.path.exists(os.path.join(gate, "proofs")))
        self.assertFalse(os.path.exists(os.path.join(gate, "runner", "tests")))
        self.assertTrue(os.path.isfile(os.path.join(self.world.repo, "tools.json")))
        self.assertTrue(os.path.isfile(os.path.join(self.world.repo, "ruff.toml")))
