"""Exit line 1 (plan check §7; architect A7, QA Q3): CI shows the same rows as a local strict-gate.

A real local T4 run's own records, laid out as CI's artifacts, must pass `summarize` with exactly the run's own
`(job, id)` set, parity included once per job. A row missing from either side would make the sets differ.
"""

import contextlib
import hashlib
import io
import json
import os
import shutil

from gate.runner import summarize
from gate.runner.tests import support


class LocalRecordsAsCi(support.GateTest):
    def test_a_local_t4_runs_records_pass_summarize_with_the_same_rows(self):
        rows = [self.row("a", tiers=("T4",)), self.row("b", job="test", tiers=("T4",))]
        reg = self.registry_file(rows)
        with open(reg, "rb") as f:
            digest = hashlib.sha256(f.read()).hexdigest()
        self.assertEqual(self.run_rows(rows, tier="T4", registry_sha256=digest).exit_code, 0)
        with open(os.path.join(self.run_dir, "run.json"), "rb") as f:
            local = json.loads(f.read())
        artifacts = os.path.join(self.root, "artifacts")
        for job in ("lint", "test"):
            dest = os.path.join(artifacts, f"records-{job}-1")
            shutil.copytree(os.path.join(self.run_dir, "records", job), os.path.join(dest, "records", job))
            shutil.copy(os.path.join(self.run_dir, "run.json"), os.path.join(dest, "run.json"))
        argv = ["--registry", reg, "--tier", "T4", "--artifacts", artifacts, "--run-id", support.RUN_ID,
                "--commit", support.COMMIT, "--attempt", "1", "--image", "local"]  # fmt: skip
        with contextlib.redirect_stdout(io.StringIO()) as out:
            code = summarize.main(argv)
        self.assertEqual(code, 0, out.getvalue())
        with open(os.path.join(artifacts, "strict-gate", "run.json"), "rb") as f:
            judged = json.loads(f.read())
        self.assertEqual({(r["job"], r["id"]) for r in judged["rows"]}, {(r["job"], r["id"]) for r in local["records"]})
        self.assertEqual({r["verdict"] for r in judged["rows"]}, {"PASS"})
