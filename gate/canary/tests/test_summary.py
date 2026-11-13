"""The canary's summary (S1.1c-4 design §4.6; plan check §6 canary (8), (12), (16); reviews DA-S8, DQ-S9, QA-S10).
gate/canary/summary.py does not exist at W0c: every test here is red by ModuleNotFoundError until lane F1 adds it.

The seam as these tests read it (the design gives the text, not the call — W0c report, ambiguity 4):
`summary.render(*, candidate, error, needs, records, pins, breaks) -> (markdown, exit_code)` — `candidate` the
Candidate or None when the candidate job wrote none; `error` candidate/error.txt's text or None; `needs` the parsed
`toJSON(needs)`; `records` maps each T4 job to its records (a job absent: its artifact is missing); `pins` and `breaks`
the sort's lines.
"""

import unittest

from gate.canary.candidate import Candidate, Upgrade

BUILD = "build 20261026 → 20261110"
TODAY = (
    Upgrade("python", "3.15.2", "3.15.2", BUILD),
    Upgrade("python-floor", "3.12.16", "3.12.16", BUILD),
    Upgrade("ruff", "0.17.3", "0.17.5"),
)
JOBS = ("lint", "test", "proofs", "determinism-macos")
RATE = "ruff: GitHub's rate limit (5000/h) is spent until 2026-11-13T20:00:00Z"


def needs(candidate: str = "success", **jobs: str) -> dict:
    out = {"candidate": {"result": candidate, "outputs": {}}}
    for job in JOBS:
        out[job] = {"result": jobs.get(job.replace("-", "_"), "success"), "outputs": {}}
    return out


def green_records() -> dict:
    return {job: [{"id": f"{job}-row", "job": job, "verdict": "pass", "reason": "ok"}] for job in JOBS}


class Summary(unittest.TestCase):
    def setUp(self) -> None:
        from gate.canary import summary  # red by ModuleNotFoundError until F1 adds it

        self.summary = summary

    def render(self, *, candidate=None, error=None, needs_=None, records=None, pins=(), breaks=()):
        return self.summary.render(
            candidate=candidate,
            error=error,
            needs=needs_ if needs_ is not None else needs(),
            records=records if records is not None else green_records(),
            pins=list(pins),
            breaks=list(breaks),
        )

    def test_upgrades_first_line_and_green(self):
        text, code = self.render(candidate=Candidate({}, TODAY))
        self.assertEqual(
            text.splitlines()[0],
            "Upgrades: python 3.15.2 (build 20261026 → 20261110), python-floor 3.12.16 (build 20261026 → 20261110), "
            "ruff 0.17.3 → 0.17.5",
        )
        self.assertEqual(code, 0)

    def test_no_upgrades_is_named_none(self):
        # Case (8): a candidate equal to the pins.
        text, code = self.render(candidate=Candidate({}, ()))
        self.assertEqual(text.splitlines()[0], "Upgrades: none")
        self.assertEqual(code, 0)
        self.assertNotIn("Suppression tables not re-measured", text)

    def test_summary_quotes_the_candidate_error(self):
        # Reviews DA-S8, DQ-S9: a failed candidate job reads as unknown with its reason, and is a break.
        text, code = self.render(error=RATE + "\n", needs_=needs("failure"), records={})
        self.assertEqual(text.splitlines()[0], f"Upgrades: unknown — the candidate job failure: {RATE}")
        self.assertIn("canary: the candidate job ended failure", text)
        self.assertEqual(code, 1)

    def test_candidate_job_failure_without_an_error_file_is_still_upgrades_unknown(self):
        text, code = self.render(needs_=needs("cancelled"), records={})
        self.assertTrue(text.splitlines()[0].startswith("Upgrades: unknown — the candidate job cancelled"), text)
        self.assertIn("canary: the candidate job ended cancelled", text)
        self.assertEqual(code, 1)

    def test_missing_records_artifact_is_a_break(self):
        records = green_records()
        del records["proofs"]
        text, code = self.render(candidate=Candidate({}, ()), needs_=needs(proofs="failure"), records=records)
        self.assertEqual(text.splitlines()[0], "Upgrades: none")
        self.assertIn("canary: no records from job proofs", text)
        self.assertEqual(code, 1)

    def test_ruff_upgrade_names_the_suppression_tables(self):
        # Case (16), review QA-S10.
        text, _ = self.render(candidate=Candidate({}, (Upgrade("ruff", "0.17.3", "0.17.5"),)))
        self.assertIn("Suppression tables not re-measured (test_suppressions.py holds them as data)", text)

    def test_golangci_upgrade_names_the_suppression_tables(self):
        text, _ = self.render(candidate=Candidate({}, (Upgrade("golangci-lint", "v2.15.0", "v2.15.1"),)))
        self.assertIn("Suppression tables not re-measured", text)

    def test_python_only_upgrade_does_not_name_the_suppression_tables(self):
        text, _ = self.render(candidate=Candidate({}, TODAY[:2]))
        self.assertNotIn("Suppression tables not re-measured", text)

    def test_sections_in_order_and_a_break_exits_1(self):
        cand = Candidate({}, TODAY[2:], ("golangci-lint v2.16.0",))
        pin = "gate.toolchain.tests.test_manifest.RealPins.test_ruff_version"
        text, code = self.render(candidate=cand, pins=[pin], breaks=["ruff: gate/x.py:3:1: E999 new rule"])
        lines = text.splitlines()
        self.assertTrue(lines[0].startswith("Upgrades: ruff 0.17.3 → 0.17.5"), lines)
        at = {
            word: next(i for i, line in enumerate(lines) if line.startswith(word))
            for word in ("New lines (not tried):", "Pins to update:", "Breaks:")
        }
        self.assertLess(at["New lines (not tried):"], at["Pins to update:"])
        self.assertLess(at["Pins to update:"], at["Breaks:"])
        self.assertIn("golangci-lint v2.16.0", lines[at["New lines (not tried):"]])
        self.assertIn(pin, text)
        self.assertIn("E999 new rule", text)
        self.assertEqual(code, 1)

    def test_pins_alone_exit_0(self):
        pin = "gate.toolchain.tests.test_manifest.RealPins.test_ruff_version"
        text, code = self.render(candidate=Candidate({}, TODAY[2:]), pins=[pin])
        self.assertIn("Pins to update:", text)
        self.assertEqual(code, 0)

    def test_first_line_is_upgrades_in_every_outcome(self):
        # Case (12).
        outcomes = [
            dict(candidate=Candidate({}, TODAY)),
            dict(candidate=Candidate({}, ())),
            dict(error=RATE, needs_=needs("failure"), records={}),
            dict(needs_=needs("skipped"), records={}),
            dict(candidate=Candidate({}, TODAY), breaks=["unit: x"]),
        ]
        for i, kwargs in enumerate(outcomes):
            with self.subTest(outcome=i):
                text, _ = self.render(**kwargs)
                self.assertTrue(text.splitlines()[0].startswith("Upgrades: "), text)


if __name__ == "__main__":
    unittest.main()
