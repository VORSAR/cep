"""Parity proof 4 of 4, the executables' hashes (design note §8; QA DQ1, DQ24; design-QA counter-example 9).

Planted: a tool cache whose go directory holds real copies (never hard links) of the ten executables tools.json
hashes, with one byte flipped in pkg/tool/<os>_<arch>/compile; everything else links to the real tree. The copies of
bin/go, bin/gofmt, asm and cgo, which parity hashes first, are byte-identical, so the one reason names compile.
Afterwards the real cache re-hashes unchanged.

Beside it, design-QA counter-example 8: a tools.json whose go entry hashes nothing would make this check verify
nothing, so the real runner refuses it at load, exit 2, and parity never runs, let alone passes.

And the hand-off (proof-world speed-up design §4, test 20; QB-S3, CA-L6): a cache holding a real, writable copy of
lark and links to the other five. Run A, given no hand-off, hashes all six and writes its own; run B, given A's, takes
all six; then one .py file of the copy outside the lark package (never imported by the version probe, never
content-hashed on its own) is rewritten with the same size and its mtime restored, and run C, given A's hand-off,
hashes lark for real and names it. No run here is given the outer run's hand-off: A none, B and C A's own.
"""

import json
import os
import shutil
import stat

from gate import wait
from gate.proofs.drivers import support
from gate.toolchain import manifest

JOB = "lint"  # the ruff row's job: a cheap row for parity to stop, or to run beside it


def writable(root: str) -> None:
    """Give the owner write on every directory and file under root (a copy keeps the installer's seal), never
    following a symlink, so the plant later is one content write and the world's cleanup can remove it."""
    for top, dirs, files in os.walk(root):
        for path in [top, *(os.path.join(top, name) for name in dirs + files)]:
            mode = os.lstat(path).st_mode
            if not stat.S_ISLNK(mode):
                os.chmod(path, stat.S_IMODE(mode) | (0o700 if stat.S_ISDIR(mode) else 0o200))


def outside_the_package(home: str, lark: manifest.Tool) -> str:
    """The bytewise-first non-empty .py file of the tree outside the importable package, and outside exec_sha256."""
    found = []
    for top, _, files in os.walk(home):
        for name in files:
            rel = os.path.relpath(os.path.join(top, name), home)
            info = os.lstat(os.path.join(home, rel))
            outside = rel.split(os.sep)[0] != lark.module and rel not in lark.exec_sha256
            if name.endswith(".py") and outside and stat.S_ISREG(info.st_mode) and info.st_size > 0:
                found.append(rel)
    if not found:
        raise AssertionError(f"{home} holds no .py file outside {lark.module}/: pick another plant")
    return sorted(found, key=os.fsencode)[0]


def changed_now(probe: str) -> int:
    """Write the probe and give its change time: the clock tick a write lands on now."""
    with open(probe, "wb") as f:
        f.write(b"tick\n")
    return os.stat(probe).st_ctime_ns


def rewrite_same_size(path: str) -> None:
    """Flip one byte in the middle: new content, the same size, the same mode."""
    with open(path, "r+b") as f:
        f.seek(os.path.getsize(path) // 2)
        byte = f.read(1)
        f.seek(-1, os.SEEK_CUR)
        f.write(bytes([byte[0] ^ 0xFF]))


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

        run = self.world.run(cache_dir=cache, handoff=False)

        self.assertParityStops(run, f"go: sha256 of {compile_rel} differs: {support.RECOVER}", planted="go")
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

        run = self.world.run(handoff=False)

        lines = run.stdout.split("\n")[:-1]
        self.assertEqual((run.code, lines[-2:]), (2, ["GATE FAILED", "exit: 2"]), run.stdout)
        self.assertIn(f"(go) {platform}: field 'exec_sha256' is empty: parity would verify nothing", lines[0])
        self.assertFalse(os.path.exists(os.path.join(run.run_dir, "records")), "parity ran")
        # The whole line, exactly: the world's tools.json, the go entry's index, the platform (S1.1c-2 W0).
        reason = f"{path}: tools[{index}] (go) {platform}: field 'exec_sha256' is empty: parity would verify nothing"
        self.assertRefusedAtLoad(run, reason)

    def test_a_tree_changed_after_its_hand_off_is_hashed_and_named(self):
        lark = support.tool("lark")
        n = len(support.tools())
        self.world.copy_gate()
        self.world.commit()
        cache = self.world.link_cache(*(t.name for t in support.tools() if t.name != "lark"))
        real, copy = support.home(support.real_cache(), "lark"), support.home(cache, "lark")
        shutil.copytree(real, copy, symlinks=True)
        writable(copy)
        rel = outside_the_package(copy, lark)
        planted = os.path.join(copy, rel)
        self.assertNotEqual(os.stat(planted).st_ino, os.stat(os.path.join(real, rel)).st_ino, "not a real copy")
        registry = self.world.registry(support.registry_row("ruff"))

        first = self.world.run(registry=registry, cache_dir=cache, handoff=False)

        self.assertRow(first, JOB, "parity", True, f"covered {n}", label="*")
        for t in support.tools():
            lines = first.tree_out(t.name)
            self.assertIsNotNone(lines, f"run A never hashed {t.name}")
            self.assertEqual(lines[2:3], [t.tree_sha256], f"run A's {t.name} tree child: {lines}")
        self.assertEqual(first.tree_out("lark")[:1], [copy])
        handoff = os.path.join(first.run_dir, "parity", "handoff.json")
        a_id = support.handoff_run_id(handoff)
        self.assertEqual(a_id, json.loads(support.read_text(os.path.join(first.run_dir, "run.json")))["run_id"])

        second = self.world.run(registry=registry, cache_dir=cache, handoff=handoff)

        self.assertRow(second, JOB, "parity", True, f"covered {n}; {n} trees taken from run {a_id}", label="*")
        before = os.stat(planted)
        probe = os.path.join(self.world.root, "ctime.probe")
        wait.until(
            f"a write now to land on a later change-time tick than {planted}'s",
            lambda: changed_now(probe) > before.st_ctime_ns,
        )
        rewrite_same_size(planted)
        os.utime(planted, ns=(before.st_atime_ns, before.st_mtime_ns))
        after = os.stat(planted)
        self.assertEqual(
            (after.st_size, after.st_mtime_ns, after.st_mode), (before.st_size, before.st_mtime_ns, before.st_mode)
        )
        self.assertGreater(
            after.st_ctime_ns, before.st_ctime_ns, "the plant moved no change time: the details cannot see it"
        )

        third = self.world.run(registry=registry, cache_dir=cache, handoff=handoff)

        self.assertGateFailed(third)
        self.assertRow(third, JOB, "parity", False, f"lark: tree differs: {support.RECOVER}", label="*")
        self.assertEqual(third.started(), ["parity"])
        parity_dir = os.path.join(third.run_dir, "parity")
        hashed = sorted(name for name in os.listdir(parity_dir) if name.startswith("tree-") and name.endswith(".out"))
        self.assertEqual(hashed, ["tree-lark.out"], "only the changed tree is hashed; the other five are taken")
        lines = third.tree_out("lark")
        self.assertEqual(lines[:1], [copy])
        self.assertNotEqual(lines[2:3], [lark.tree_sha256], lines)
