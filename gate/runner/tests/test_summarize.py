"""`summarize`: CI's records to the one required check (exit line 1; design §12.5; plan check S3; QA C11-C17,
N7, N30-N33). Each test builds its own artifact tree, `<dir>/records-<job>-<attempt>/records/<job>/<id>.json` plus
`run.json`, from a small registry, and runs the real CLI: `gate/run.py summarize` under the pinned interpreter.

Contract read here: the record run_id is the string given as --run-id; --attempt names the current
`records-<job>-<attempt>` directories; the record files are the records judged.
"""

import hashlib
import os
import shutil
import sys

from gate.runner import records, summary
from gate.runner.tests import support

RUN = "4242-2"
ATTEMPT = "2"
SHA = "a" * 40
IMAGE = "ubuntu24 20261101.1"
RUN_PY = os.path.join(support.REPO, "gate", "run.py")


class Summarize(support.GateTest):
    def setUp(self) -> None:
        super().setUp()
        self.rows = [
            self.row("gofmt", tiers=("T1", "T2", "T3", "T4")),
            self.row("ruff", tiers=("T4",), blocking=False),
            self.row("fast", tiers=("T1",)),  # not at T4: never expected
            self.row("unit", job="test", tiers=("T2", "T3", "T4")),
        ]
        self.law(self.rows)
        self.artifacts = os.path.join(self.root, "artifacts")
        self.blocking = {"gofmt": True, "ruff": False, "unit": True, "parity": True}
        self.clock = 1_000_000
        # artifact directory name -> {"job", "registry_sha256", "records": {(job, id): dict or bytes}}
        self.tree: dict[str, dict] = {}
        for job, ids in (("lint", ("parity", "gofmt", "ruff")), ("test", ("parity", "unit"))):
            name = f"records-{job}-{ATTEMPT}"
            self.tree[name] = {"job": job, "registry_sha256": self.digest, "records": {}}
            for row_id in ids:
                self.tree[name]["records"][(job, row_id)] = self.rec(job, row_id)

    def law(self, rows: list) -> None:
        """The registry the artifacts' run.json files name."""
        self.registry = self.registry_file(rows)
        with open(self.registry, "rb") as f:
            self.digest = hashlib.sha256(f.read()).hexdigest()
        for art in getattr(self, "tree", {}).values():
            art["registry_sha256"] = self.digest

    def with_lawdelta_row(self) -> None:
        """S1.1c-1 design 2.6 (review QA-S13): the four fixture rows plus the lawdelta row in lint at T1-T4."""
        self.law([*self.rows, self.row("lawdelta", tiers=("T1", "T2", "T3", "T4"))])

    def rec(self, job: str, row_id: str, **over: object) -> dict:
        self.clock += 1000
        fields = {
            "run_id": RUN, "commit": SHA, "tier": "T4", "job": job, "row_id": row_id,
            "blocking": self.blocking.get(row_id, True), "verdict": "pass", "reason": "covered 1", "exit_code": 0,
            "covered": 1, "expected": 1, "started_ms": self.clock, "ended_ms": self.clock + 400, "duration_ms": 400,
            "scope_sha256": "ab",
        }  # fmt: skip  # keys packed several per line
        fields.update(over)
        return records.make(**fields)

    def set(self, job: str, row_id: str, artifact: str | None = None, **over: object) -> None:
        name = artifact or f"records-{job}-{ATTEMPT}"
        old = self.tree[f"records-{job}-{ATTEMPT}"]["records"].get((job, row_id))
        fields = {} if not isinstance(old, dict) else {"started_ms": old["started_ms"], "ended_ms": old["ended_ms"]}
        self.tree.setdefault(name, {"job": job, "registry_sha256": self.digest, "records": {}})
        self.tree[name]["records"][(job, row_id)] = self.rec(job, row_id, **{**fields, **over})

    def materialize(self) -> None:
        shutil.rmtree(self.artifacts, ignore_errors=True)
        for name, art in self.tree.items():
            base = os.path.join(self.artifacts, name)
            judged = []
            for (job, row_id), rec in art["records"].items():
                path = os.path.join(base, "records", job, f"{row_id}.json")
                os.makedirs(os.path.dirname(path), exist_ok=True)
                with open(path, "wb") as f:
                    f.write(rec if isinstance(rec, bytes) else records.json_bytes(rec))
                if isinstance(rec, dict):
                    judged.append(rec)
            run = {
                "schema": 1, "run_id": RUN, "tier": "T4", "registry_sha256": art["registry_sha256"],
                "records": judged, "wall_ms": 0, "summed_ms": 0,
            }  # fmt: skip  # keys packed several per line
            os.makedirs(base, exist_ok=True)
            with open(os.path.join(base, "run.json"), "wb") as f:
                f.write(records.json_bytes(run))

    def summarize(self) -> tuple[int, str]:
        self.materialize()
        argv = [sys.executable, "-I", "-B", "-X", "utf8", RUN_PY, "summarize", "--registry", self.registry]
        argv += ["--tier", "T4", "--artifacts", self.artifacts, "--run-id", RUN, "--commit", SHA]
        argv += ["--attempt", ATTEMPT, "--image", IMAGE]
        code, out, err = support.run_tool(argv, self.repo)
        return code, out.decode("utf-8", errors="replace") + err.decode("utf-8", errors="replace")

    def cells(self, out: str, job: str, step: str) -> list[str]:
        for line in out.split("\n"):
            cells = [c.strip() for c in line.split(" | ", 4)]
            if cells[:2] == [job, step]:
                return cells
        raise AssertionError(f"no summary line for {job}/{step} in:\n{out}")

    def tail(self, out: str) -> list[str]:
        return out.split("\n")[:-1][-2:]

    def assertRed(self, job: str, step: str, word: str) -> str:
        code, out = self.summarize()
        self.assertEqual(self.tail(out), ["GATE FAILED", "exit: 1"], out)
        self.assertEqual(code, 1, out)
        cells = self.cells(out, job, step)
        self.assertEqual(cells[2], "FAIL", out)
        self.assertIn(word, cells[4].lower(), out)
        return out

    # Green.

    def test_all_present_and_passing_is_green(self):
        code, out = self.summarize()
        self.assertEqual(self.tail(out), ["GATE PASSED", "exit: 0"], out)
        self.assertEqual(code, 0, out)
        for job, step in (
            ("lint", "parity"),
            ("lint", "gofmt"),
            ("lint", "ruff"),
            ("test", "parity"),
            ("test", "unit"),
        ):
            self.assertEqual(self.cells(out, job, step)[2], "PASS", out)
        self.assertNotIn("fast", [c.strip() for line in out.split("\n") for c in line.split(" | ")[1:2]], out)
        self.assertIn(f"\nimage: {IMAGE}\n", out)
        self.assertIn(f"\nrun: {self.artifacts}\n", out)

    def test_rows_come_in_registry_order(self):
        _, out = self.summarize()
        steps = [line.split(" | ")[1].strip() for line in out.split("\n") if line.count(" | ") >= 4]
        rows = [s for s in steps if s not in ("step", "parity")]
        self.assertEqual(rows, ["gofmt", "ruff", "unit"], out)

    def test_the_footer_formula_across_offset_clocks(self):
        """N30: wall = latest ended_ms - earliest started_ms over every record; summed = the sum of duration_ms."""
        self.set("test", "parity", started_ms=50_000_000, ended_ms=50_000_300, duration_ms=300)
        self.set("test", "unit", started_ms=50_000_400, ended_ms=50_009_000, duration_ms=8600)
        recs = [r for art in self.tree.values() for r in art["records"].values()]
        wall = max(r["ended_ms"] for r in recs) - min(r["started_ms"] for r in recs)
        summed = sum(r["duration_ms"] for r in recs)
        code, out = self.summarize()
        self.assertEqual(code, 0, out)
        self.assertEqual(out.split("\n")[-4], summary.footer(wall, summed), out)

    def test_a_non_blocking_fail_is_a_warn_and_green(self):
        self.set("lint", "ruff", verdict="fail", reason="exit 1: x", exit_code=1)
        code, out = self.summarize()
        self.assertEqual(self.cells(out, "lint", "ruff")[2], "WARN", out)
        self.assertEqual((code, self.tail(out)), (0, ["GATE PASSED", "exit: 0"]), out)

    # RED, one class each.

    def test_a_missing_record_is_red(self):
        del self.tree[f"records-lint-{ATTEMPT}"]["records"][("lint", "gofmt")]
        self.assertRed("lint", "gofmt", "missing")

    def test_a_missing_parity_record_is_red(self):
        del self.tree[f"records-test-{ATTEMPT}"]["records"][("test", "parity")]
        self.assertRed("test", "parity", "missing")

    def test_a_missing_artifact_is_red(self):
        del self.tree[f"records-test-{ATTEMPT}"]
        self.assertRed("test", "unit", "missing")

    def test_a_duplicate_is_red(self):
        self.set("lint", "gofmt", artifact=f"records-test-{ATTEMPT}")
        self.assertRed("lint", "gofmt", "duplicate")

    def test_invalid_json_is_malformed(self):
        self.tree[f"records-lint-{ATTEMPT}"]["records"][("lint", "gofmt")] = b'{"schema": 1, "id": "gofmt"'
        self.assertRed("lint", "gofmt", "malformed")

    def test_a_record_missing_a_field_is_malformed(self):
        rec = self.rec("lint", "gofmt")
        del rec["covered"]
        self.tree[f"records-lint-{ATTEMPT}"]["records"][("lint", "gofmt")] = records.json_bytes(rec)
        self.assertRed("lint", "gofmt", "malformed")

    def test_another_run_id_is_stale(self):
        self.set("lint", "gofmt", run_id="4241-1")
        self.assertRed("lint", "gofmt", "stale")

    def test_another_commit_is_stale(self):
        self.set("lint", "gofmt", commit="b" * 40)
        self.assertRed("lint", "gofmt", "stale")

    def test_a_job_only_from_an_earlier_attempt_is_stale(self):
        """N32: re-run failed jobs: the lint job's records are attempt 1's."""
        old = self.tree.pop(f"records-lint-{ATTEMPT}")
        old["records"] = {k: {**v, "run_id": "4242-1"} for k, v in old["records"].items()}
        self.tree["records-lint-1"] = old
        out = self.assertRed("lint", "gofmt", "stale")
        self.assertNotIn("missing", self.cells(out, "lint", "gofmt")[4].lower(), out)

    def test_two_attempts_judge_only_the_current(self):
        """N33, as the coordinator settled QA's ambiguity 7: attempts 1 and 2 both downloaded ("re-run all jobs").
        Only the current attempt is judged, so a complete re-run can pass; the older records are never duplicates."""
        old = self.tree[f"records-lint-{ATTEMPT}"]
        self.tree["records-lint-1"] = {
            **old,
            "records": {k: {**v, "run_id": "4242-1"} for k, v in old["records"].items()},
        }
        code, out = self.summarize()
        self.assertEqual((code, self.tail(out)), (0, ["GATE PASSED", "exit: 0"]), out)
        self.assertNotIn("duplicate", out.lower(), out)

    def test_a_wrong_tier_is_red(self):
        self.set("lint", "gofmt", tier="T2")
        self.assertRed("lint", "gofmt", "tier")

    def test_an_unknown_row_is_red(self):
        self.set("lint", "bogus")
        self.assertRed("lint", "bogus", "unknown")

    def test_a_row_not_at_t4_is_unknown(self):
        self.set("lint", "fast")
        self.assertRed("lint", "fast", "unknown")

    def test_a_pass_with_a_nonzero_exit_is_inconsistent(self):
        self.set("lint", "gofmt", exit_code=1)
        self.assertRed("lint", "gofmt", "inconsistent")

    def test_covered_not_expected_is_inconsistent(self):
        self.set("lint", "gofmt", covered=1, expected=2)
        self.assertRed("lint", "gofmt", "inconsistent")

    def test_blocking_not_the_registrys_is_inconsistent(self):
        """N7."""
        self.set("lint", "gofmt", blocking=False)
        self.assertRed("lint", "gofmt", "inconsistent")

    def test_a_record_that_ends_before_it_starts_is_inconsistent(self):
        """N30."""
        self.set("test", "unit", started_ms=9_000_000, ended_ms=8_999_000, duration_ms=1)
        self.assertRed("test", "unit", "inconsistent")

    def test_a_foreign_registry_is_red(self):
        """N31: the lint job's run.json names another law."""
        self.tree[f"records-lint-{ATTEMPT}"]["registry_sha256"] = "f" * 64
        code, out = self.summarize()
        self.assertEqual((code, self.tail(out)), (1, ["GATE FAILED", "exit: 1"]), out)
        lint = [self.cells(out, "lint", step) for step in ("parity", "gofmt", "ruff")]
        self.assertTrue(any(c[2] == "FAIL" and "registry" in c[4].lower() for c in lint), out)
        self.assertEqual(self.cells(out, "test", "unit")[2], "PASS", out)

    def test_a_skip_at_t4_is_red(self):
        self.set("lint", "gofmt", verdict="skip", reason="skip: nothing staged in scope (3 tracked)", exit_code=-1)
        self.assertRed("lint", "gofmt", "skip")

    # The LAW delta block through the record (S1.1c-1 design 2.5; plan check section 6 LAW delta (10), (13)).

    def test_a_registry_without_the_row_says_not_run(self):
        code, out = self.summarize()
        self.assertEqual(code, 0, out)
        lines = out.split("\n")
        self.assertEqual(lines[0], "LAW Δ: not run — the lawdelta row is not in this registry at T4", out)
        self.assertEqual(lines[1], "", out)

    def test_a_missing_lawdelta_record_is_failed_in_the_block_and_a_missing_row(self):
        self.with_lawdelta_row()
        code, out = self.summarize()
        lines = out.split("\n")
        self.assertEqual(lines[0], "LAW Δ: FAILED — see the lawdelta row", out)
        self.assertEqual(lines[1], "", out)
        self.assertEqual(
            self.cells(out, "lint", "lawdelta"),
            ["lint", "lawdelta", "FAIL", "0.0s", "missing: no job recorded it"],
            out,
        )
        self.assertEqual((code, self.tail(out)), (1, ["GATE FAILED", "exit: 1"]), out)

    def test_a_passing_lawdelta_record_with_no_law_is_inconsistent(self):
        self.with_lawdelta_row()
        self.set("lint", "lawdelta")
        out = self.assertRed("lint", "lawdelta", "inconsistent")
        self.assertEqual(self.cells(out, "lint", "lawdelta")[4], "inconsistent: pass with no LAW Δ block", out)
        self.assertEqual(out.split("\n")[0], "LAW Δ: FAILED — see the lawdelta row", out)

    def test_a_passing_lawdelta_record_gives_the_block_first(self):
        self.with_lawdelta_row()
        block = ["LAW Δ (1 files, vs abc1234)", "gate/current-sprint +1/−1 — marker S1.1c-1 → S4.1"]
        self.set("lint", "lawdelta", law=block)
        code, out = self.summarize()
        self.assertEqual(out.split("\n")[:4], [*block, "", "job  | step     | verdict | time | reason"], out)
        self.assertEqual((code, self.tail(out)), (0, ["GATE PASSED", "exit: 0"]), out)

    def test_a_failing_lawdelta_record_is_failed_in_the_block(self):
        self.with_lawdelta_row()
        self.set("lint", "lawdelta", verdict="fail", reason="exit 1: lawdelta: no merge base with main", exit_code=1)
        out = self.assertRed("lint", "lawdelta", "merge base")
        self.assertEqual(out.split("\n")[0], "LAW Δ: FAILED — see the lawdelta row", out)

    def test_a_record_without_law_is_malformed(self):
        rec = self.rec("lint", "gofmt")
        del rec["law"]
        self.tree[f"records-lint-{ATTEMPT}"]["records"][("lint", "gofmt")] = records.json_bytes(rec)
        self.assertRed("lint", "gofmt", "malformed")

    def test_a_schema_1_record_names_the_schema(self):
        # owner, 2026-11-11 (design 5): the record check names the schema it expected.
        rec = self.rec("lint", "gofmt")
        rec["schema"] = 1
        self.tree[f"records-lint-{ATTEMPT}"]["records"][("lint", "gofmt")] = records.json_bytes(rec)
        out = self.assertRed("lint", "gofmt", "malformed")
        self.assertEqual(self.cells(out, "lint", "gofmt")[4], "malformed: schema 1, expected 2", out)
