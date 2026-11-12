"""Parity proof 4 of 4, the executables' hashes (design note §8; QA DQ1, DQ24; design-QA counter-example 9).

Planted: a tool cache whose go directory holds real copies (never hard links) of the ten executables tools.json
hashes, with one byte flipped in pkg/tool/<os>_<arch>/compile; everything else links to the real tree. The copies of
bin/go, bin/gofmt, asm and cgo, which parity hashes first, are byte-identical, so the one reason names compile.
Afterwards the real cache re-hashes unchanged.

Beside it, design-QA counter-example 8: a tools.json whose go entry hashes nothing would make this check verify
nothing, so the real runner refuses it at load, exit 2, and parity never runs, let alone passes.
"""

import json
import os

from gate.proofs.drivers import support
from gate.toolchain import manifest


def flip_one_byte(path: str) -> None:
    os.chmod(path, 0o755)
    with open(path, "r+b") as f:
        f.seek(os.path.getsize(path) // 2)
        byte = f.read(1)
        f.seek(-1, os.SEEK_CUR)
        f.write(bytes([byte[0] ^ 0xFF]))


class ParityHash(support.ParityDriver):
    def test_one_flipped_byte_in_compile_is_named_and_the_real_cache_is_untouched(self):
        go = support.tool("go")
        goos, goarch = support.goos_goarch()
        compile_rel = f"pkg/tool/{goos}_{goarch}/compile"
        self.assertEqual(len(go.exec_sha256), 10)
        self.assertIn(compile_rel, go.exec_sha256)
        self.world.copy_gate()
        self.world.commit()
        cache = self.world.link_cache(*(t.name for t in support.tools() if t.name != "go"))
        go_home = self.world.skeleton(cache, "go", copies=frozenset(go.exec_sha256), omit=frozenset())
        for rel, want in sorted(go.exec_sha256.items()):
            self.assertFalse(os.path.islink(os.path.join(go_home, rel)), rel)
            self.assertEqual(manifest.sha256_file(os.path.join(go_home, rel)), want, rel)
        flip_one_byte(os.path.join(go_home, compile_rel))
        self.assertNotEqual(manifest.sha256_file(os.path.join(go_home, compile_rel)), go.exec_sha256[compile_rel])

        run = self.world.run(cache_dir=cache)

        self.assertParityStops(run, f"go: sha256 of {compile_rel} differs: {support.RECOVER}")
        real = support.home(support.real_cache(), "go")
        for rel, want in sorted(go.exec_sha256.items()):
            self.assertEqual(manifest.sha256_file(os.path.join(real, rel)), want, f"the real cache's {rel} changed")

    def test_an_empty_exec_map_is_refused_at_load_and_never_passes(self):
        self.world.copy_gate()
        path = os.path.join(self.world.repo, "tools.json")
        with open(path, "rb") as f:
            doc = json.loads(f.read())
        (index,) = [i for i, t in enumerate(doc["tools"]) if t["name"] == "go"]
        go = doc["tools"][index]
        platform = manifest.platform_key()
        self.assertTrue(go["platforms"][platform]["exec_sha256"])
        go["platforms"][platform]["exec_sha256"] = {}
        with open(path, "w", encoding="utf-8") as f:
            json.dump(doc, f, indent=2, sort_keys=True)
            f.write("\n")
        self.world.commit()

        run = self.world.run()

        lines = run.stdout.split("\n")[:-1]
        self.assertEqual((run.code, lines[-2:]), (2, ["GATE FAILED", "exit: 2"]), run.stdout)
        self.assertIn(f"(go) {platform}: field 'exec_sha256' is empty: parity would verify nothing", lines[0])
        self.assertFalse(os.path.exists(os.path.join(run.run_dir, "records")), "parity ran")
        # The whole line, exactly: the world's tools.json, the go entry's index, the platform (S1.1c-2 W0).
        reason = f"{path}: tools[{index}] (go) {platform}: field 'exec_sha256' is empty: parity would verify nothing"
        self.assertRefusedAtLoad(run, reason)
