"""tools.json loading, validation and locating (design note §3; QA DQ21, DQ23, counter-example 8).

Runs under the pinned 3.15.2 and the pinned 3.12.16.
"""

import copy
import hashlib
import json
import os
import stat
import sys
import threading
import unittest

from gate.toolchain import manifest
from gate.toolchain.tests import support

GOOD_DIGEST = "ab" * 32
PLATFORM = "darwin-arm64"


def good_tool() -> dict:
    return {
        "command": "demo",
        "name": "demo",
        "path_dir": "bin",
        "platforms": {
            PLATFORM: {
                "archive_sha256": GOOD_DIGEST,
                "exec_sha256": {"bin/demo": GOOD_DIGEST},
                "strip": "demo/",
                "tree_sha256": GOOD_DIGEST,
                "url": "https://example.invalid/demo.tar.gz",
            }
        },
        "version": "1.0",
        "version_argv": ["bin/demo", "--version"],
        "version_line": "demo 1.0",
    }


def good_doc() -> dict:
    return {"schema": 1, "tools": [good_tool()]}


class Platform(unittest.TestCase):
    def test_the_two_supported_platforms(self):
        self.assertEqual(manifest._platform_key("darwin", "arm64"), "darwin-arm64")
        self.assertEqual(manifest._platform_key("linux", "x86_64"), "linux-amd64")

    def test_an_unsupported_platform_names_the_two_supported_ones(self):
        for sys_platform, machine in (("linux", "aarch64"), ("darwin", "x86_64"), ("win32", "AMD64")):
            with self.subTest(sys_platform=sys_platform, machine=machine):
                with self.assertRaises(manifest.ManifestError) as caught:
                    manifest._platform_key(sys_platform, machine)
                self.assertIn("darwin-arm64", str(caught.exception))
                self.assertIn("linux-amd64", str(caught.exception))

    def test_this_machine_is_read_from_the_interpreter(self):
        self.assertEqual(manifest.platform_key().split("-")[0], {"darwin": "darwin", "linux": "linux"}[sys.platform])


class CacheRoot(unittest.TestCase):
    def test_the_override_wins(self):
        self.assertEqual(manifest.cache_root({"VORSAR_CEP_TOOLS_CACHE": "/x/y", "HOME": "/h"}), "/x/y")

    def test_the_default_sits_under_home(self):
        self.assertEqual(manifest.cache_root({"HOME": "/h"}), "/h/.cache/vorsar/cep/tools")

    def test_an_empty_override_is_unset(self):
        self.assertEqual(
            manifest.cache_root({"VORSAR_CEP_TOOLS_CACHE": "", "HOME": "/h"}), "/h/.cache/vorsar/cep/tools"
        )

    def test_xdg_is_never_consulted(self):
        self.assertEqual(manifest.cache_root({"XDG_CACHE_HOME": "/xdg", "HOME": "/h"}), "/h/.cache/vorsar/cep/tools")

    def test_a_relative_override_becomes_absolute(self):
        self.assertEqual(
            manifest.cache_root({"VORSAR_CEP_TOOLS_CACHE": "rel/cache", "HOME": "/h"}),
            os.path.join(os.getcwd(), "rel/cache"),
        )


class Load(support.ToolchainTest):
    def write(self, doc) -> str:
        path = os.path.join(self.world, "tools.json")
        with open(path, "w", encoding="utf-8") as f:
            json.dump(doc, f)
        return path

    def load_doc(self, doc):
        return manifest.load(self.write(doc), PLATFORM)

    def assert_refused(self, doc, *needles: str) -> None:
        with self.assertRaises(manifest.ManifestError) as caught:
            self.load_doc(doc)
        for needle in needles:
            self.assertIn(needle, str(caught.exception))

    def test_the_committed_tools_json_loads_in_file_order_for_both_platforms(self):
        # The table is design note §3's, typed by hand.
        want = [
            ("python", "3.15.2", "python3.15", "bin", "python-3.15.2"),
            ("python-floor", "3.12.16", "python3.12", "bin", "python-floor-3.12.16"),
            ("go", "go1.27.3", "go", "bin", "go-go1.27.3"),
            ("golangci-lint", "v2.15.0", "golangci-lint", ".", "golangci-lint-v2.15.0"),
            ("ruff", "0.17.3", "ruff", ".", "ruff-0.17.3"),
        ]
        darwin_archives = ("285a6b924713", "fbf4afef53c2", "dd3908cf2095", "d1e409481a8c", "118f7121aa83")
        linux_archives = ("08d4ae9024a7", "6a79e922e500", "65efbc80fdc1", "9b7fe6a6bf63", "2b0890f5a559")
        for platform, prefixes in (("darwin-arm64", darwin_archives), ("linux-amd64", linux_archives)):
            with self.subTest(platform=platform):
                tools = manifest.load(os.path.join(support.REPO, "tools.json"), platform)
                got = [(t.name, t.version, t.command, t.path_dir) for t in tools]
                self.assertEqual(got, [w[:4] for w in want])
                self.assertEqual(
                    [manifest.tool_home("/c", t) for t in tools],
                    [f"/c/{w[4]}-{p}" for w, p in zip(want, prefixes, strict=True)],
                )
                self.assertEqual(len(tools[2].exec_sha256), 10)

    def test_the_fields_arrive_on_the_tool(self):
        tool = self.load_doc(good_doc())[0]
        self.assertEqual(
            (tool.name, tool.version, tool.command, tool.path_dir, tool.version_argv, tool.version_line),
            ("demo", "1.0", "demo", "bin", ("bin/demo", "--version"), "demo 1.0"),
        )
        self.assertEqual(
            (tool.url, tool.archive_sha256, tool.strip, tool.tree_sha256),
            ("https://example.invalid/demo.tar.gz", GOOD_DIGEST, "demo/", GOOD_DIGEST),
        )
        self.assertEqual(dict(tool.exec_sha256), {"bin/demo": GOOD_DIGEST})

    def test_a_loaded_exec_map_cannot_be_changed(self):
        tool = self.load_doc(good_doc())[0]
        with self.assertRaises(TypeError):
            tool.exec_sha256["bin/demo"] = "0" * 64

    def test_a_missing_file_is_refused_naming_it(self):
        path = os.path.join(self.world, "absent.json")
        with self.assertRaises(manifest.ManifestError) as caught:
            manifest.load(path, PLATFORM)
        self.assertIn(path, str(caught.exception))

    def test_a_file_that_is_not_json_or_not_utf8_is_refused(self):
        path = os.path.join(self.world, "tools.json")
        for data in (b"{", b"", b'{"schema": 1, "tools": [\xff]}'):
            with self.subTest(data=data):
                with open(path, "wb") as f:
                    f.write(data)
                with self.assertRaises(manifest.ManifestError):
                    manifest.load(path, PLATFORM)

    def test_a_duplicate_key_is_refused_at_every_level(self):
        tool = json.dumps(good_tool())
        entry = json.dumps(good_tool()["platforms"][PLATFORM])
        texts = {
            "top": '{"schema": 1, "schema": 1, "tools": [' + tool + "]}",
            "tool": '{"schema": 1, "tools": [{"name": "x", ' + tool[1:] + "]}",
            "entry": '{"schema": 1, "tools": [' + tool.replace(entry, '{"url": "u", ' + entry[1:]) + "]}",
        }
        path = os.path.join(self.world, "tools.json")
        for level, text in texts.items():
            with self.subTest(level=level):
                with open(path, "w", encoding="utf-8") as f:
                    f.write(text)
                with self.assertRaises(manifest.ManifestError) as caught:
                    manifest.load(path, PLATFORM)
                self.assertIn("duplicate", str(caught.exception))

    def test_the_top_level_shape_is_checked(self):
        for name, doc in {
            "a list": [],
            "no schema": {"tools": [good_tool()]},
            "no tools": {"schema": 1},
            "schema 2": {"schema": 2, "tools": [good_tool()]},
            "schema a string": {"schema": "1", "tools": [good_tool()]},
            "schema true": {"schema": True, "tools": [good_tool()]},
            "schema a float": {"schema": 1.0, "tools": [good_tool()]},
            "tools empty": {"schema": 1, "tools": []},
            "tools a map": {"schema": 1, "tools": {}},
            "a tool not a map": {"schema": 1, "tools": ["demo"]},
        }.items():
            with self.subTest(name):
                self.assert_refused(doc)

    def test_every_tool_field_must_be_present(self):
        for field in good_tool():
            with self.subTest(field=field):
                doc = good_doc()
                del doc["tools"][0][field]
                self.assert_refused(doc, field)

    def test_every_platform_field_must_be_present(self):
        for field in good_tool()["platforms"][PLATFORM]:
            with self.subTest(field=field):
                doc = good_doc()
                del doc["tools"][0]["platforms"][PLATFORM][field]
                self.assert_refused(doc, field)

    def test_unknown_fields_are_refused_at_every_level(self):
        top = good_doc()
        top["extra"] = 1
        self.assert_refused(top, "extra")
        tool = good_doc()
        tool["tools"][0]["extra"] = 1
        self.assert_refused(tool, "extra")
        entry = good_doc()
        entry["tools"][0]["platforms"][PLATFORM]["extra"] = 1
        self.assert_refused(entry, "extra")

    def test_wrongly_typed_or_unsafe_tool_fields_are_refused(self):
        cases = [
            ("command", 7),
            ("command", ""),
            ("command", "bin/demo"),
            ("name", None),
            ("name", ""),
            ("name", "../demo"),
            ("name", "a/b"),
            ("path_dir", ""),
            ("path_dir", "/bin"),
            ("path_dir", "../bin"),
            ("path_dir", 1),
            ("platforms", []),
            ("platforms", {}),
            ("version", 1.0),
            ("version", ""),
            ("version", "1/../2"),
            ("version_argv", "bin/demo"),
            ("version_argv", []),
            ("version_argv", ["bin/demo", 1]),
            ("version_argv", [""]),
            ("version_argv", ["/bin/demo"]),
            ("version_argv", ["../demo"]),
            ("version_line", ""),
            ("version_line", ["demo 1.0"]),
        ]
        for field, value in cases:
            with self.subTest(field=field, value=value):
                doc = good_doc()
                doc["tools"][0][field] = value
                self.assert_refused(doc, field)

    def test_wrongly_typed_or_unsafe_platform_fields_are_refused(self):
        cases = [
            ("archive_sha256", 1),
            ("archive_sha256", "abc"),
            ("archive_sha256", "AB" * 32),
            ("archive_sha256", GOOD_DIGEST + "\n"),
            ("tree_sha256", ""),
            ("tree_sha256", "zz" * 32),
            ("strip", ""),
            ("strip", "demo"),
            ("strip", "/demo/"),
            ("strip", "../demo/"),
            ("strip", 1),
            ("url", ""),
            ("url", 1),
            ("exec_sha256", []),
            ("exec_sha256", {"bin/demo": "abc"}),
            ("exec_sha256", {"bin/demo": 1}),
            ("exec_sha256", {"/bin/demo": GOOD_DIGEST}),
            ("exec_sha256", {"../demo": GOOD_DIGEST}),
            ("exec_sha256", {"": GOOD_DIGEST}),
        ]
        for field, value in cases:
            with self.subTest(field=field, value=value):
                doc = good_doc()
                doc["tools"][0]["platforms"][PLATFORM][field] = value
                self.assert_refused(doc, field)

    def test_an_empty_exec_map_is_refused(self):
        # QA counter-example 8: parity would verify nothing and pass.
        doc = good_doc()
        doc["tools"][0]["platforms"][PLATFORM]["exec_sha256"] = {}
        self.assert_refused(doc, "exec_sha256", "empty")

    def test_the_current_platform_entry_must_exist(self):
        doc = good_doc()
        doc["tools"][0]["platforms"]["linux-amd64"] = doc["tools"][0]["platforms"].pop(PLATFORM)
        self.assert_refused(doc, PLATFORM, "demo")

    def test_an_unknown_platform_key_is_refused(self):
        doc = good_doc()
        doc["tools"][0]["platforms"]["darwin_arm64"] = copy.deepcopy(doc["tools"][0]["platforms"][PLATFORM])
        self.assert_refused(doc, "darwin_arm64")

    def test_every_platform_entry_is_validated_not_only_the_current_one(self):
        doc = good_doc()
        other = copy.deepcopy(doc["tools"][0]["platforms"][PLATFORM])
        other["exec_sha256"] = {}
        doc["tools"][0]["platforms"]["linux-amd64"] = other
        self.assert_refused(doc, "linux-amd64", "exec_sha256")

    def test_two_tools_with_one_name_are_refused(self):
        doc = good_doc()
        doc["tools"].append(good_tool())
        self.assert_refused(doc, "demo", "twice")


class Locate(unittest.TestCase):
    def tool(self, path_dir: str) -> manifest.Tool:
        return manifest.Tool(
            name="ruff",
            version="0.17.3",
            command="ruff",
            path_dir=path_dir,
            version_argv=("ruff", "--version"),
            version_line="ruff 0.17.3",
            url="u",
            archive_sha256="118f7121aa838785672bf612487f755a89108bb5f44e0f3df96c5967a72614f9",
            strip="ruff/",
            exec_sha256={},
            tree_sha256="",
        )

    def test_the_home_is_name_version_and_twelve_digest_characters(self):
        self.assertEqual(manifest.tool_home("/c", self.tool(".")), "/c/ruff-0.17.3-118f7121aa83")

    def test_path_dirs_keep_tools_json_order_and_normalise_a_dot(self):
        self.assertEqual(
            manifest.path_dirs("/c", [self.tool("bin"), self.tool(".")]),
            ["/c/ruff-0.17.3-118f7121aa83/bin", "/c/ruff-0.17.3-118f7121aa83"],
        )


class Digests(support.ToolchainTest):
    def test_sha256_file_matches_the_published_vector_for_abc(self):
        path = os.path.join(self.world, "abc")
        with open(path, "wb") as f:
            f.write(b"abc")
        self.assertEqual(manifest.sha256_file(path), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")

    def test_tree_digest_follows_the_design_note_line_by_line(self):
        tree = os.path.join(self.world, "tree")
        os.makedirs(os.path.join(tree, "a"))
        os.makedirs(os.path.join(tree, "sub"))
        for rel, data, mode in (
            (".complete", b"not hashed\n", 0o444),
            ("a-b", b"dash\n", 0o444),
            ("a.txt", b"dot\n", 0o444),
            ("a/b", b"nested\n", 0o555),
            ("sub/.complete", b"hashed\n", 0o444),
        ):
            path = os.path.join(tree, rel)
            with open(path, "wb") as f:
                f.write(data)
            os.chmod(path, mode)
        os.symlink("a/b", os.path.join(tree, "link"))
        os.symlink("no such target", os.path.join(tree, "dangling"))
        # Written out by hand from design note §3: `<kind> <path> <payload>\n`, sorted bytewise by the
        # whole path, so "a-b" (0x2d) and "a.txt" (0x2e) sort before "a/b" (0x2f), and `.complete` at the
        # root alone is left out.
        lines = [
            b"d a -\n",
            b"f a-b " + support.sha256(b"dash\n").encode() + b"\n",
            b"f a.txt " + support.sha256(b"dot\n").encode() + b"\n",
            b"x a/b " + support.sha256(b"nested\n").encode() + b"\n",
            b"l dangling no such target\n",
            b"l link a/b\n",
            b"d sub -\n",
            b"f sub/.complete " + support.sha256(b"hashed\n").encode() + b"\n",
        ]
        self.assertEqual(manifest.tree_sha256(tree), hashlib.sha256(b"".join(lines)).hexdigest())

    def test_a_non_ascii_name_hashes_as_its_utf8_bytes(self):
        tree = os.path.join(self.world, "tree")
        os.makedirs(tree)
        with open(os.path.join(tree, "Þfoo.go"), "wb") as f:
            f.write(b"go\n")
        line = "f Þfoo.go ".encode() + support.sha256(b"go\n").encode() + b"\n"
        self.assertEqual(manifest.tree_sha256(tree), hashlib.sha256(line).hexdigest())

    def test_an_execute_bit_for_anyone_makes_the_kind_x(self):
        tree = os.path.join(self.world, "tree")
        os.makedirs(tree)
        path = os.path.join(tree, "f")
        with open(path, "wb") as f:
            f.write(b"x")
        os.chmod(path, 0o401)
        line = b"x f " + support.sha256(b"x").encode() + b"\n"
        self.assertEqual(manifest.tree_sha256(tree), hashlib.sha256(line).hexdigest())

    def test_a_symlink_to_a_directory_is_not_followed(self):
        tree = os.path.join(self.world, "tree")
        os.makedirs(os.path.join(tree, "d"))
        with open(os.path.join(tree, "d", "f"), "wb") as f:
            f.write(b"x")
        os.symlink("d", os.path.join(tree, "e"))
        lines = b"d d -\n" + b"f d/f " + support.sha256(b"x").encode() + b"\n" + b"l e d\n"
        self.assertEqual(manifest.tree_sha256(tree), hashlib.sha256(lines).hexdigest())


def serial_tree_sha256(root: str) -> str:
    """Design note §3's digest, written straight from its text, one file at a time: the oracle.

    It walks in the same order as the serial code the parallel one replaces (a stack of directories, each
    listed by scandir), so for a broken tree it raises the same first error that code raised.
    """
    lines = []
    pending = [(os.fsencode(root), b"")]
    while pending:
        directory, prefix = pending.pop()
        with os.scandir(directory) as listing:
            entries = list(listing)
        for entry in entries:
            rel = prefix + entry.name
            if rel == b".complete":
                continue
            mode = entry.stat(follow_symlinks=False).st_mode
            if stat.S_ISLNK(mode):
                kind, payload = b"l", os.readlink(entry.path)
            elif stat.S_ISDIR(mode):
                kind, payload = b"d", b"-"
                pending.append((entry.path, rel + b"/"))
            else:
                with open(entry.path, "rb") as f:
                    payload = hashlib.sha256(f.read()).hexdigest().encode()
                kind = b"x" if mode & 0o111 else b"f"
            lines.append((rel, kind + b" " + rel + b" " + payload + b"\n"))
    return hashlib.sha256(b"".join(line for _, line in sorted(lines))).hexdigest()


class ParallelTree(support.ToolchainTest):
    """The whole-tree digest hashes across threads (owner, 2026-11-02) and must equal the serial definition."""

    DIRECTORIES = 12
    FILES_PER_DIRECTORY = 40

    def build(self) -> tuple[str, dict[bytes, bytes]]:
        """A tree whose walk order is far from bytewise order. Returns it and its lines, keyed by path."""
        tree = os.path.join(self.world, "tree")
        lines: dict[bytes, bytes] = {}
        # "a-b" (0x2d) and "a.b" (0x2e) sort before "a/..." (0x2f): a sort by components would get them wrong.
        names = [f"d{i:02}" for i in range(self.DIRECTORIES)] + ["a"]
        for directory in names:
            os.makedirs(os.path.join(tree, directory))
            lines[directory.encode()] = f"d {directory} -\n".encode()
            for i in range(self.FILES_PER_DIRECTORY):
                rel = f"{directory}/f{i:03}"
                data = f"{rel}\n".encode() * (1 + 977 * (i % 7))  # 0 B to 57 KiB: some hashes take longer
                path = os.path.join(tree, rel)
                with open(path, "wb") as f:
                    f.write(data)
                mode = 0o555 if i % 5 == 0 else 0o444
                os.chmod(path, mode)
                kind = "x" if mode & 0o111 else "f"
                lines[rel.encode()] = f"{kind} {rel} {support.sha256(data)}\n".encode()
        for rel, data in (("a-b", b"dash\n"), ("a.b", b"dot\n"), ("empty", b"")):
            with open(os.path.join(tree, rel), "wb") as f:
                f.write(data)
            lines[rel.encode()] = f"f {rel} {support.sha256(data)}\n".encode()
        os.symlink("a/f001", os.path.join(tree, "link"))
        lines[b"link"] = b"l link a/f001\n"
        return tree, lines

    def by_hand(self, lines: dict[bytes, bytes]) -> str:
        return hashlib.sha256(b"".join(lines[rel] for rel in sorted(lines))).hexdigest()

    def test_every_worker_count_gives_the_digest_of_the_definition(self):
        tree, lines = self.build()
        want = self.by_hand(lines)
        self.assertEqual(serial_tree_sha256(tree), want)  # the oracle agrees with the hand-built lines
        self.assertEqual(manifest.tree_sha256(tree), want)
        for workers in (1, 2, 8, 64):
            with self.subTest(workers=workers):
                self.assertEqual(manifest.tree_sha256(tree, workers=workers), want)

    def test_hashes_finishing_out_of_order_never_reach_the_digest(self):
        # The first file to start is held until every other file is hashed, so completion order is never
        # walk order. The record of finishes is the witness that it really was held.
        tree, lines = self.build()
        files = sum(1 for line in lines.values() if line[:1] in b"fx")
        cond = threading.Condition()
        started: list[bytes] = []
        finished: list[bytes] = []

        def hash_file(path: bytes) -> str:
            with cond:
                held = not started
                started.append(path)
                if held:
                    self.assertTrue(cond.wait_for(lambda: len(finished) == files - 1, timeout=support.CHILD_CAP_S))
            digest = manifest.sha256_file(path)
            with cond:
                finished.append(path)
                cond.notify_all()
            return digest

        self.assertEqual(manifest._tree_sha256(tree, 8, hash_file), self.by_hand(lines))
        self.assertEqual(finished[-1], started[0])
        self.assertNotEqual(started[0], started[-1])

    def make_unreadable(self, tree: str, *rels: str) -> None:
        for rel in rels:
            os.chmod(os.path.join(tree, rel), 0o000)

    def assert_same_failure(self, tree: str, **kwargs) -> OSError:
        """The parallel digest raises exactly what the serial one raises, and returns nothing."""
        with self.assertRaises(OSError) as serial:
            serial_tree_sha256(tree)
        with self.assertRaises(OSError) as parallel:
            if kwargs:
                manifest._tree_sha256(tree, 8, **kwargs)
            else:
                manifest.tree_sha256(tree, workers=8)
        got, want = parallel.exception, serial.exception
        self.assertEqual((type(got), got.errno, got.filename), (type(want), want.errno, want.filename))
        return got

    def test_an_unreadable_file_raises_what_serial_raises(self):
        tree, _ = self.build()
        self.make_unreadable(tree, "d05/f017")
        error = self.assert_same_failure(tree)
        self.assertIsInstance(error, PermissionError)
        self.assertEqual(os.fsdecode(error.filename), os.path.join(tree, "d05/f017"))

    def test_the_first_failure_in_walk_order_wins_whatever_fails_first(self):
        # Two unreadable files: the one serial names is held until the other has failed.
        tree, _ = self.build()
        self.make_unreadable(tree, "d03/f011", "d09/f023")
        with self.assertRaises(OSError) as serial:
            serial_tree_sha256(tree)
        named = serial.exception.filename
        other = os.fsencode(os.path.join(tree, "d09/f023" if os.fsdecode(named).endswith("d03/f011") else "d03/f011"))
        cond = threading.Condition()
        failed: list[bytes] = []

        def hash_file(path: bytes) -> str:
            if path == named:
                with cond:
                    self.assertTrue(cond.wait_for(lambda: other in failed, timeout=support.CHILD_CAP_S))
            try:
                return manifest.sha256_file(path)
            except OSError:
                with cond:
                    failed.append(path)
                    cond.notify_all()
                raise

        self.assert_same_failure(tree, hash_file=hash_file)
        self.assertEqual(failed[0], other)  # the witness: the other file really failed first

    def test_a_file_failing_before_an_unreadable_directory_is_the_error(self):
        # Serial hashes a file as it meets it, and lists a directory only later: the file's error comes first.
        tree, _ = self.build()
        self.make_unreadable(tree, "a-b", "d07")
        error = self.assert_same_failure(tree)
        self.assertEqual(os.fsdecode(error.filename), os.path.join(tree, "a-b"))

    def test_an_unreadable_directory_alone_raises_what_serial_raises(self):
        tree, _ = self.build()
        self.make_unreadable(tree, "d07")
        self.assert_same_failure(tree)


if __name__ == "__main__":
    unittest.main()
