"""Parity proof 4 of 4, the executables' hashes (design note §8; QA DQ1, DQ24; design-QA counter-example 9).

Planted: a tool cache whose go directory holds real copies (never hard links) of the ten executables tools.json
hashes, with one byte flipped in pkg/tool/<os>_<arch>/compile; everything else links to the real tree. The copies of
bin/go, bin/gofmt, asm and cgo, which parity hashes first, are byte-identical, so the one reason names compile.
Afterwards the real cache re-hashes unchanged.
"""

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
