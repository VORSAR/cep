"""The summary's job rules where design §4.6 and QA's test_summary are silent (lane F1b's choices, each in its report):
a T4 job's `failure` is read from its records (a pin red fails the job, and pins alone exit 0); a T4 job that failed
with every record passing, or ended cancelled or skipped, is a break; a successful candidate job that wrote no
candidate is `Upgrades: unknown` and a break; sections with nothing read `none`. Expected text written by hand."""

import unittest

from gate.canary.candidate import Candidate, Upgrade

JOBS = ("lint", "test", "proofs", "determinism-macos")
PIN = "gate.toolchain.tests.test_manifest.RealPins.test_ruff_version (gate/toolchain/tests/test_manifest.py)"
RUFF = Candidate({}, (Upgrade("ruff", "0.17.3", "0.17.5"),))


def needs(candidate: str = "success", **jobs: str) -> dict:
    out = {"candidate": {"result": candidate, "outputs": {}}}
    for job in JOBS:
        out[job] = {"result": jobs.get(job.replace("-", "_"), "success"), "outputs": {}}
    return out


def records(**verdicts: str) -> dict:
    return {
        job: [{"id": f"{job}-row", "job": job, "verdict": verdicts.get(job, "pass"), "reason": "ok"}] for job in JOBS
    }


class SummaryChoices(unittest.TestCase):
    def setUp(self) -> None:
        from gate.canary import summary

        self.summary = summary

    def render(self, *, candidate=RUFF, error=None, needs_=None, records_=None, pins=(), breaks=()):
        return self.summary.render(
            candidate=candidate,
            error=error,
            needs=needs_ if needs_ is not None else needs(),
            records=records_ if records_ is not None else records(),
            pins=list(pins),
            breaks=list(breaks),
        )

    def test_the_whole_text_of_a_green_run(self):
        got, code = self.render(candidate=Candidate({}, (Upgrade("python", "3.15.2", "3.15.2", "build 1 → 2"),)))
        self.assertEqual(
            got,
            "Upgrades: python 3.15.2 (build 1 → 2)\n\nNew lines (not tried): none\n\nPins to update: none\n\n"
            "Breaks: none\n",
        )
        self.assertEqual(code, 0)

    def test_a_job_failed_by_its_pin_records_alone_exits_0(self):
        got, code = self.render(needs_=needs(test="failure"), records_=records(test="fail"), pins=[PIN])
        self.assertIn(f"Pins to update: {PIN}", got)
        self.assertIn("Breaks: none", got)
        self.assertEqual(code, 0)

    def test_a_job_failed_with_every_record_passing_is_a_break(self):
        got, code = self.render(needs_=needs(lint="failure"))
        self.assertIn("canary: the job lint ended failure with every record passing", got)
        self.assertEqual(code, 1)

    def test_a_cancelled_job_with_records_is_a_break(self):
        got, code = self.render(needs_=needs(determinism_macos="cancelled"))
        self.assertIn("canary: the job determinism-macos ended cancelled", got)
        self.assertEqual(code, 1)

    def test_an_empty_records_artifact_is_no_records(self):
        recs = records()
        recs["proofs"] = []
        got, code = self.render(records_=recs)
        self.assertIn("canary: no records from job proofs", got)
        self.assertEqual(code, 1)

    def test_a_successful_candidate_job_with_no_candidate_is_unknown_and_a_break(self):
        got, code = self.render(candidate=None)
        self.assertEqual(got.splitlines()[0], "Upgrades: unknown — the candidate job success: it wrote no candidate")
        self.assertIn("canary: the candidate job wrote no candidate", got)
        self.assertEqual(code, 1)

    def test_a_successful_candidate_job_with_an_error_file_is_unknown_and_a_break(self):
        got, code = self.render(error="ruff: x\n")
        self.assertEqual(got.splitlines()[0], "Upgrades: unknown — the candidate job success: ruff: x")
        self.assertIn("canary: the candidate job ended success and wrote candidate/error.txt", got)
        self.assertNotIn("Suppression tables", got)
        self.assertEqual(code, 1)

    def test_a_missing_error_file_is_named(self):
        got, _ = self.render(candidate=None, needs_=needs("cancelled"), records_={})
        self.assertEqual(got.splitlines()[0], "Upgrades: unknown — the candidate job cancelled: no candidate/error.txt")

    def test_a_multi_line_error_is_one_line(self):
        got, _ = self.render(candidate=None, error="ruff: a\n\nlark: b\n", needs_=needs("failure"), records_={})
        self.assertEqual(got.splitlines()[0], "Upgrades: unknown — the candidate job failure: ruff: a; lark: b")

    def test_breaks_are_listed_sorted_after_the_jobs(self):
        got, _ = self.render(needs_=needs(lint="cancelled"), breaks=["vet: b", "ruff: a"])
        line = next(x for x in got.splitlines() if x.startswith("Breaks: "))
        self.assertEqual(line, "Breaks: canary: the job lint ended cancelled, vet: b, ruff: a")

    def test_the_suppression_note_is_the_last_line(self):
        got, _ = self.render()
        self.assertEqual(
            got.splitlines()[-1], "Suppression tables not re-measured (test_suppressions.py holds them as data)"
        )


if __name__ == "__main__":
    unittest.main()
