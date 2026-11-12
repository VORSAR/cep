"""Parity proof 3 of 4, the path check (design note §8; QA DQ1; design-QA counter-example 6).

Planted: a tool cache whose go directory lacks bin/go, and a marker-writing `go` first on the caller's PATH. The
runner puts the cache first on PATH, so the only go it can find is the one outside the cache: parity names it and
never runs it. The fake is shown to write its marker when it does run, so the absent marker means something.
"""

import os

from gate.proofs.drivers import support

FAKE = """#!{python} -I
with open({marker!r}, "w", encoding="utf-8") as f:
    f.write("the go outside the cache ran\\n")
print("go version go1.27.4 darwin/arm64")
"""


class ParityPath(support.ParityDriver):
    def test_a_go_outside_the_cache_is_named_and_never_run(self):
        self.world.copy_gate()
        self.world.commit()
        cache = self.world.link_cache(*(t.name for t in support.tools() if t.name != "go"))
        go_home = self.world.skeleton(cache, "go", copies=frozenset(), omit=frozenset({"bin/go"}))
        self.assertTrue(os.path.isdir(os.path.join(go_home, "bin")))
        self.assertFalse(os.path.lexists(os.path.join(go_home, "bin", "go")))
        marker = os.path.join(self.world.root, "marker")
        fake = self.world.root + "/fakebin/go"
        os.mkdir(os.path.dirname(fake))
        with open(fake, "w", encoding="utf-8") as f:
            f.write(FAKE.format(python=support.pinned_python(), marker=marker))
        os.chmod(fake, 0o755)

        run = self.world.run(cache_dir=cache, path_first=os.path.dirname(fake))

        self.assertParityStops(run, f"go: path: {os.path.realpath(fake)} is outside the cache: {support.RECOVER}")
        self.assertFalse(os.path.exists(marker), "parity ran the go outside the cache")
        out, err = os.path.join(self.world.root, "fake.stdout"), os.path.join(self.world.root, "fake.stderr")
        code = support.run_process([fake], cwd=self.world.root, env=self.world.env, out=out, err=err, cap_s=60)
        self.assertEqual(code, 0)
        self.assertTrue(os.path.exists(marker), "the fake go never writes its marker, so its absence proves nothing")
