"""Parity proof 2 of 4, not installed (design note §8; QA DQ1; design-QA counter-example 5).

Planted: a tool cache holding python, python-floor, golangci-lint, ruff and lark (links to the real, read-only
tool directories) and no go directory at all. The runner is started directly, so the failure is the runner's exit 1,
never the bootstrap's exit 2.
"""

import os

from gate.proofs.drivers import support


class ParityMissing(support.ParityDriver):
    def test_a_cache_without_go_stops_the_run_at_parity(self):
        self.world.copy_gate()
        self.world.commit()
        others = [t.name for t in support.tools() if t.name != "go"]
        self.assertEqual(others, ["python", "python-floor", "golangci-lint", "ruff", "lark"])
        cache = self.world.link_cache(*others)
        self.assertFalse(os.path.lexists(support.home(cache, "go")))

        run = self.world.run(cache_dir=cache)

        self.assertParityStops(run, "go: not installed: run make tools")
