"""Output does not depend on hash seeds (plan check C7; design note §4), proven across real processes."""

import os
import sys

from gate.runner.tests import support

FROZEN_RUN = os.path.join(support.GATE, "runner", "testdata", "frozen_run.py")


class HashSeeds(support.GateTest):
    def test_seeds_0_to_7_print_identical_bytes(self):
        rows = [
            self.good("zeta", job="zulu"),
            self.good("alpha", "status", 1, job="alpha"),
            self.good("mid", "status", 2, job="mike", blocking=False),
            self.row("beta", job="alpha", needs=("alpha",)),
        ]
        registry = self.registry_file(rows)
        outputs, hashes, files = set(), set(), set()
        for seed in range(8):
            run_dir = os.path.join(self.root, f"seed-{seed}")
            probe = os.path.join(self.root, f"hash-{seed}")
            env = dict(support.base_env(), PYTHONHASHSEED=str(seed))
            argv = [sys.executable, "-s", "-B", "-P", "-X", "utf8", FROZEN_RUN, probe,
                    "--tier", "T2", "--registry", registry, "--run-dir", run_dir]  # fmt: skip
            code, out, err = support.run_tool(argv, self.repo, env)
            self.assertEqual((code, err), (1, b""))
            outputs.add(out)
            with open(probe, encoding="utf-8") as f:
                hashes.add(f.read())
            with open(os.path.join(run_dir, "run.json"), "rb") as f:
                files.add(f.read())
        self.assertGreater(len(hashes), 1)  # the witness: the seed really changed the process's hashing
        self.assertEqual(len(outputs), 1)
        self.assertEqual(len(files), 1)
