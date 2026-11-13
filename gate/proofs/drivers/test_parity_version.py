"""Parity proof 1 of 4, the version check (design note §8; QA DQ1; design-QA counter-example 4).

Planted: a fixture tools.json whose go version_line says go1.27.2. The cache, the path and every hash are real, so
the keyed directory exists, the stamp matches, bin/go answers with the pinned version, and only the version check can
fail. By hand: "go: version: want 'go version go1.27.2 darwin/arm64', got 'go version go1.27.4 darwin/arm64': run
make clean-tools tools" on darwin-arm64.
"""

import json
import os

from gate.proofs.drivers import support

PLANTED = "go version go1.27.2 {goos}/{goarch}"


class ParityVersion(support.ParityDriver):
    def test_a_version_line_one_patch_off_stops_the_run_at_parity(self):
        self.world.copy_gate()
        path = os.path.join(self.world.repo, "tools.json")
        with open(path, "rb") as f:
            doc = json.loads(f.read())
        (go,) = [t for t in doc["tools"] if t["name"] == "go"]
        real = go["version_line"]
        self.assertNotEqual(real, PLANTED)
        go["version_line"] = PLANTED
        with open(path, "w", encoding="utf-8") as f:
            json.dump(doc, f, indent=2, sort_keys=True)
            f.write("\n")
        self.world.commit()

        run = self.world.run(handoff=False)

        goos, goarch = support.goos_goarch()
        want, got = PLANTED.format(goos=goos, goarch=goarch), real.format(goos=goos, goarch=goarch)
        self.assertParityStops(run, f"go: version: want {want!r}, got {got!r}: {support.RECOVER}", planted="go")
