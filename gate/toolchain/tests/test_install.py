"""The installer (design note §3 steps 1-6, exec, clean-tools; plan check §6 cases 14, 16, 17; C24, C27).

QA design counter-examples 15, 16, 17, 37. Runs under the pinned 3.15.2 and the pinned 3.12.16, in-process
with the two seams a test cannot reach otherwise injected: the download and the rename.
"""

import errno
import hashlib
import http.client
import io
import os
import stat
import unittest

from gate.toolchain import install, manifest
from gate.toolchain.tests import support
from gate.toolchain.tests.support import Member

UNREACHABLE = "file:///no/such/archive.tar.gz"


class InstallTest(support.ToolchainTest):
    def setUp(self) -> None:
        super().setUp()
        self.archive = os.path.join(self.world, "demo.tar.gz")
        self.environ = {"VORSAR_CEP_TOOLS_CACHE": self.cache, "HOME": self.world}

    def run_main(self, *argv: str, **seams) -> tuple[int, list[str]]:
        out = io.StringIO()
        code = install.main(list(argv), repo=self.repo, environ=self.environ, out=out, **seams)
        return code, out.getvalue().splitlines()

    def pin(self, members: list[Member], **fields) -> str:
        """Write the archive and a one-tool tools.json; return the tool's final directory."""
        support.write_archive(self.archive, members)
        entry = support.tool_entry(self.archive, **fields)
        support.write_tools(self.repo, [entry])
        digest = entry["platforms"][manifest.platform_key()]["archive_sha256"]
        return os.path.join(self.cache, f"{entry['name']}-{entry['version']}-{digest[:12]}")

    def assert_refused(self, code: int, lines: list[str], *needles: str) -> None:
        self.assertEqual(code, 2, lines)
        self.assertEqual(lines[-2:], ["GATE FAILED", "exit: 2"])
        text = "\n".join(lines[:-2])
        for needle in needles:
            self.assertIn(needle, text)


class HappyPath(InstallTest):
    MEMBERS = [
        Member("demo", "d", mode=0o755),  # the member equal to strip: go's and ruff's first member
        Member("demo/bin/link", "l", link="demo"),  # a symlink before its target; parents on demand
        Member("demo/bin/demo", data=b"#!tool\n", mode=0o775),  # group-writable, as the Python archives ship
        Member("demo/doc", "d", mode=0o750),
        Member("demo/doc/readme", data=b"read me\n", mode=0o664),
        Member("demo/doc/copy", "h", link="demo/doc/readme"),  # a hard link to an earlier member
        Member("demo/doc/sibling", "l", link="../bin/demo"),  # a target with `..` that stays inside
        Member("demo/lib/sub/deep.txt", data=b"deep\n"),
        Member("demo/dangling", "l", link="no/such/target"),  # a target that never exists
        Member("demo/Þfoo.go", data=b"go\n"),  # the go archive's non-ASCII names
    ]

    def test_the_tree_is_exactly_what_the_archive_says_with_write_bits_cleared(self):
        # C27: this listing is written by hand, and both pinned interpreters must produce it.
        home = self.pin(self.MEMBERS)
        code, lines = self.run_main("install")
        self.assertEqual(code, 0, lines)
        digest = support.sha256_of(self.archive)
        want = {
            ".complete": ("f", 0o444, support.sha256(f"{digest}\n".encode())),
            "bin": ("d", 0o555),
            "bin/demo": ("f", 0o555, support.sha256(b"#!tool\n")),
            "bin/link": ("l", "demo"),
            "dangling": ("l", "no/such/target"),
            "doc": ("d", 0o550),
            "doc/copy": ("f", 0o444, support.sha256(b"read me\n")),
            "doc/readme": ("f", 0o444, support.sha256(b"read me\n")),
            "doc/sibling": ("l", "../bin/demo"),
            "lib": ("d", 0o555),
            "lib/sub": ("d", 0o555),
            "lib/sub/deep.txt": ("f", 0o444, support.sha256(b"deep\n")),
            "Þfoo.go": ("f", 0o444, support.sha256(b"go\n")),
        }
        self.assertEqual(support.snapshot(home), dict(sorted(want.items())))

    def test_the_tool_directory_itself_is_read_only(self):
        home = self.pin(self.MEMBERS)
        self.run_main("install")
        self.assertEqual(stat.S_IMODE(os.stat(home).st_mode) & 0o222, 0)

    def test_the_cache_holds_only_the_tool_afterwards(self):
        # Design note §3 step 6: the download and the temporary directory are gone.
        home = self.pin(self.MEMBERS)
        self.run_main("install")
        self.assertEqual(os.listdir(self.cache), [os.path.basename(home)])

    def test_the_installed_tree_hashes_as_written_out_by_hand(self):
        home = self.pin([Member("demo/bin/demo", data=b"x", mode=0o755), Member("demo/d", "l", link="bin/demo")])
        self.run_main("install")
        lines = b"d bin -\n" + b"x bin/demo " + support.sha256(b"x").encode() + b"\n" + b"l d bin/demo\n"
        self.assertEqual(manifest.tree_sha256(home), hashlib.sha256(lines).hexdigest())

    def test_every_tool_is_reported_in_tools_json_order(self):
        for name in ("zeta", "alpha"):
            support.write_archive(os.path.join(self.world, f"{name}.tar.gz"), [Member(f"{name}/bin/{name}")])
        tools = [
            support.tool_entry(os.path.join(self.world, f"{name}.tar.gz"), name=name, strip=f"{name}/", command=name)
            for name in ("zeta", "alpha")
        ]
        support.write_tools(self.repo, tools)
        code, lines = self.run_main("install")
        self.assertEqual((code, lines), (0, ["zeta 1.0: installed", "alpha 1.0: installed"]))
        code, lines = self.run_main("install")
        self.assertEqual((code, lines), (0, ["zeta 1.0: present", "alpha 1.0: present"]))

    def test_the_download_is_named_after_the_process_and_sits_in_the_cache(self):
        self.pin(self.MEMBERS)
        seen: list[list[str]] = []

        def fetch(url, sink):
            seen.append(sorted(os.listdir(self.cache)))
            install._fetch(url, sink)

        self.run_main("install", fetch=fetch)
        self.assertEqual(len(seen), 1)
        self.assertRegex(seen[0][0], rf"^dl\.{os.getpid()}\.[a-z0-9_]+$")

    def test_the_tree_moves_by_os_rename_from_a_sibling_temporary_directory(self):
        # Design note §3 step 4: <cache>/tmp.<pid>.<rand>, a sibling of the final directory.
        home = self.pin(self.MEMBERS)
        moves: list[tuple[str, str]] = []

        def rename(src, dst):
            moves.append((src, dst))
            os.rename(src, dst)

        code, lines = self.run_main("install", rename=rename)
        self.assertEqual(code, 0, lines)
        self.assertEqual(len(moves), 1)
        src, dst = moves[0]
        self.assertEqual(dst, home)
        self.assertEqual(os.path.dirname(src), self.cache)
        self.assertRegex(os.path.basename(src), rf"^tmp\.{os.getpid()}\.[a-z0-9_]+$")


class Skip(InstallTest):
    def test_a_second_install_with_the_url_unreachable_downloads_nothing(self):
        # QA counter-example 17; the first run's fetch is the witness that the seam sees downloads.
        home = self.pin([Member("demo/bin/demo")])
        calls: list[str] = []

        def fetch(url, sink):
            calls.append(url)
            install._fetch(url, sink)

        self.assertEqual(self.run_main("install", fetch=fetch)[0], 0)
        self.assertEqual(len(calls), 1)
        os.unlink(self.archive)
        before = support.snapshot(self.cache)
        code, lines = self.run_main("install", fetch=fetch)
        self.assertEqual((code, lines), (0, ["demo 1.0: present"]))
        self.assertEqual(len(calls), 1)
        self.assertEqual(support.snapshot(self.cache), before)
        self.assertTrue(os.path.isdir(home))

    def bad_stamps(self) -> dict:
        digest = "ab" * 32
        return {
            "no stamp": None,
            "another digest": f"{'cd' * 32}\n",
            "empty": "",
            "cut short": digest[:32],
            "no newline": digest,
            "two lines": f"{digest}\n{digest}\n",
        }

    def test_a_present_directory_with_a_missing_or_wrong_stamp_is_refused_and_kept(self):
        # QA counter-example 16 (and plan check case 14): never deleted, never downloaded over.
        digest = "ab" * 32
        calls: list[str] = []
        for name, stamp in self.bad_stamps().items():
            with self.subTest(name):
                home = self.pin([Member("demo/bin/demo")], pinned=digest)
                os.makedirs(home, exist_ok=True)
                with open(os.path.join(home, "kept"), "wb") as f:
                    f.write(b"mine\n")
                path = os.path.join(home, ".complete")
                if stamp is not None:
                    with open(path, "w", encoding="ascii") as f:
                        f.write(stamp)
                elif os.path.exists(path):
                    os.unlink(path)
                before = support.snapshot(home)
                code, lines = self.run_main("install", fetch=lambda url, sink: calls.append(url))
                self.assert_refused(code, lines, home, "make clean-tools")
                self.assertEqual(calls, [])
                self.assertEqual(support.snapshot(home), before)

    def test_a_file_where_the_directory_should_be_is_refused(self):
        home = self.pin([Member("demo/bin/demo")])
        os.makedirs(self.cache)
        with open(home, "wb") as f:
            f.write(b"not a directory\n")
        code, lines = self.run_main("install")
        self.assert_refused(code, lines, home, "make clean-tools")

    def test_the_exact_stamp_is_accepted_as_written_by_the_first_install(self):
        digest = "ab" * 32
        home = self.pin([Member("demo/bin/demo")], pinned=digest)
        os.makedirs(home)
        with open(os.path.join(home, ".complete"), "w", encoding="ascii") as f:
            f.write(f"{digest}\n")
        code, lines = self.run_main("install", fetch=lambda url, sink: self.fail("no download on a skip"))
        self.assertEqual((code, lines), (0, ["demo 1.0: present"]))


class Download(InstallTest):
    def test_an_unreachable_url_is_refused_naming_it(self):
        home = self.pin([Member("demo/bin/demo")], url=UNREACHABLE)
        code, lines = self.run_main("install")
        self.assert_refused(code, lines, UNREACHABLE, "make tools")
        self.assertEqual(os.listdir(self.cache), [])
        self.assertFalse(os.path.lexists(home))

    def test_every_way_a_download_breaks_is_refused_and_leaves_nothing(self):
        # A real server can raise any of these; none may crash past the fix text.
        self.pin([Member("demo/bin/demo")])
        for error in (OSError("reset"), ValueError("unknown url type"), http.client.IncompleteRead(b"")):
            with self.subTest(error=error):

                def fetch(url, sink, error=error):
                    sink.write(b"partial")
                    raise error

                code, lines = self.run_main("install", fetch=fetch)
                self.assert_refused(code, lines, self.archive)
                self.assertEqual(os.listdir(self.cache), [])

    def test_a_wrong_digest_is_refused_before_extraction_naming_both_digests(self):
        # Plan check case 17. The archive would extract cleanly: only the digest refuses it.
        pinned = "ab" * 32
        home = self.pin([Member("demo/bin/demo")], pinned=pinned)
        code, lines = self.run_main("install")
        self.assert_refused(code, lines, pinned, support.sha256_of(self.archive))
        self.assertFalse(os.path.lexists(home))
        self.assertEqual(os.listdir(self.cache), [])

    def test_a_hostile_archive_with_a_wrong_digest_never_reaches_the_extractor(self):
        # The digest comes first: the refusal names the digests, not the member.
        self.pin([Member("/abs")], pinned="ab" * 32)
        code, lines = self.run_main("install")
        self.assert_refused(code, lines, "ab" * 32)
        self.assertNotIn("/abs'", "\n".join(lines))


class Rejections(InstallTest):
    """QA counter-example 15 and design note §3 step 4: one fixture per rejection rule."""

    def rejected_archives(self, outside: str) -> dict[str, tuple[list[Member], str]]:
        """Name -> (the hostile members, the member the refusal must name)."""
        hard = Member("demo/h", "h", link="demo/later")
        return {
            "an absolute path": ([Member("/abs")], "/abs"),
            "an absolute path under strip": ([Member("/demo/x")], "/demo/x"),
            "a .. component": ([Member("demo/../escaped")], "demo/../escaped"),
            "a leading ..": ([Member("../escaped")], "../escaped"),
            "a .. that climbs back into strip": ([Member("demo/a/../../demo/x")], "demo/a/../../demo/x"),
            "a member outside strip": ([Member("other/x")], "other/x"),
            "a sibling sharing strip's letters": ([Member("demoX/x")], "demoX/x"),
            "a directory outside strip": ([Member("other", "d")], "other"),
            "a symlink with an absolute target": ([Member("demo/l", "l", link="/tmp")], "demo/l"),
            "a symlink escaping by ..": ([Member("demo/l", "l", link="../../x")], "demo/l"),
            "a symlink to the tree's parent": ([Member("demo/l", "l", link="..")], "demo/l"),
            "a deep symlink escaping": ([Member("demo/a/b/l", "l", link="../../../x")], "demo/a/b/l"),
            "a symlink with an empty target": ([Member("demo/l", "l", link="")], "demo/l"),
            "a symlink with a newline in its target": ([Member("demo/l", "l", link="a\nb")], "demo/l"),
            "an escaping symlink then a file through it": (
                [Member("demo/d", "l", link=outside), Member("demo/d/f")],
                "demo/d",
            ),
            "an inside symlink then a file through it": (
                [Member("demo/sub", "d"), Member("demo/d", "l", link="sub"), Member("demo/d/f")],
                "demo/d/f",
            ),
            "an inside symlink then a directory through it": (
                [Member("demo/sub", "d"), Member("demo/d", "l", link="sub"), Member("demo/d/e/f", "d")],
                "demo/d/e/f",
            ),
            "an inside symlink then a symlink through it": (
                [Member("demo/sub", "d"), Member("demo/d", "l", link="sub"), Member("demo/d/l", "l", link="x")],
                "demo/d/l",
            ),
            "an inside symlink then a hard link through it": (
                [
                    Member("demo/f"),
                    Member("demo/sub", "d"),
                    Member("demo/d", "l", link="sub"),
                    Member("demo/d/h", "h", link="demo/f"),
                ],
                "demo/d/h",
            ),
            "a symlink then a file over it": (
                [Member("demo/t"), Member("demo/l", "l", link="t"), Member("demo/l", data=b"through")],
                "demo/l",
            ),
            "a chain of inside links that resolves outside": (
                [Member("demo/a/up", "l", link=".."), Member("demo/esc", "l", link="a/up/..")],
                "demo/esc",
            ),
            "a hard link to a later member": ([hard, Member("demo/later")], "demo/h"),
            "a hard link to nothing": ([Member("demo/h", "h", link="demo/absent")], "demo/h"),
            "a hard link outside strip": ([Member("demo/h", "h", link="other/x")], "demo/h"),
            "a hard link to an absolute path": ([Member("demo/h", "h", link="/etc/hosts")], "demo/h"),
            "a hard link with a .. component": ([Member("demo/h", "h", link="demo/../../x")], "demo/h"),
            "a hard link to a symlink": (
                [Member("demo/f"), Member("demo/l", "l", link="f"), Member("demo/h", "h", link="demo/l")],
                "demo/h",
            ),
            "a hard link to a directory": (
                [Member("demo/sub", "d"), Member("demo/h", "h", link="demo/sub")],
                "demo/h",
            ),
            "a FIFO": ([Member("demo/p", "fifo")], "demo/p"),
            "a character device": ([Member("demo/c", "chr")], "demo/c"),
            "a block device": ([Member("demo/b", "blk")], "demo/b"),
            "a duplicate member": ([Member("demo/f"), Member("demo/f", data=b"again")], "demo/f"),
            "a directory over a file": ([Member("demo/f"), Member("demo/f", "d")], "demo/f"),
            "a file over a directory": ([Member("demo/d", "d"), Member("demo/d")], "demo/d"),
            "a member below a file": ([Member("demo/f"), Member("demo/f/g")], "demo/f/g"),
            "a root member that is a file": ([Member("demo")], "demo"),
            "a root member that is a symlink": ([Member("demo", "l", link="/tmp")], "demo"),
            "an empty name component": ([Member("demo//x")], "demo//x"),
            "a dot name component": ([Member("demo/./x")], "demo/./x"),
            "a newline in a name": ([Member("demo/a\nb")], "demo/a\nb"),
            "a member forging the stamp": ([Member("demo/.complete")], "demo/.complete"),
        }

    def test_each_hostile_archive_is_refused_with_nothing_written_outside(self):
        outside = os.path.join(self.world, "outside")
        os.makedirs(outside)
        for name, (members, named) in self.rejected_archives(outside).items():
            with self.subTest(name):
                # A good member first, so the extractor is at work when it meets the bad one.
                home = self.pin([Member("demo/ok"), *members])
                before = support.snapshot(self.world)
                code, lines = self.run_main("install")
                self.assert_refused(code, lines, repr(named))
                after = support.snapshot(self.world)
                after.pop("cache", None)
                before.pop("cache", None)
                self.assertEqual(after, before)
                self.assertFalse(os.path.lexists(home))

    def test_a_refused_archive_can_be_followed_by_a_good_one(self):
        # The refusal leaves the cache clean enough that the next install simply works.
        self.pin([Member("/abs")])
        self.assertEqual(self.run_main("install")[0], 2)
        home = self.pin([Member("demo/bin/demo")])
        self.assertEqual(self.run_main("install")[0], 0)
        self.assertEqual(os.listdir(self.cache), [os.path.basename(home)])


class Race(InstallTest):
    """C24 and design note §3 step 5, made deterministic: the rival finishes during our download."""

    def rival(self) -> int:
        return install.main(["install"], repo=self.repo, environ=self.environ, out=io.StringIO())

    def rival_first(self):
        def fetch(url, sink):
            self.assertEqual(self.rival(), 0)
            install._fetch(url, sink)

        return fetch

    def test_the_read_only_loser_keeps_the_winners_tree_and_removes_its_own(self):
        home = self.pin([Member("demo/bin/demo", data=b"tool\n"), Member("demo/bin/l", "l", link="demo")])
        code, lines = self.run_main("install", fetch=self.rival_first())
        self.assertEqual((code, lines), (0, ["demo 1.0: present"]))
        self.assertEqual(os.listdir(self.cache), [os.path.basename(home)])  # no tmp.*, no dl.*
        self.assertEqual(sorted(os.listdir(home)), [".complete", "bin"])  # nothing nested inside
        self.assertEqual(sorted(os.listdir(os.path.join(home, "bin"))), ["demo", "l"])

    def test_every_errno_a_lost_race_can_raise_counts_as_lost(self):
        # macOS raises EACCES for a read-only loser, Linux ENOTEMPTY or EEXIST.
        for number in (errno.EACCES, errno.ENOTEMPTY, errno.EEXIST, errno.EPERM):
            with self.subTest(errno=errno.errorcode[number]):
                home = self.pin([Member("demo/bin/demo")])

                def rename(src, dst, number=number):
                    self.assertEqual(self.rival(), 0)
                    raise OSError(number, os.strerror(number), src)

                code, lines = self.run_main("install", rename=rename)
                self.assertEqual((code, lines), (0, ["demo 1.0: present"]))
                self.assertEqual(os.listdir(self.cache), [os.path.basename(home)])
                self.assertEqual(self.run_main("clean-tools")[0], 0)

    def test_a_rename_failure_with_no_final_directory_is_a_failure(self):
        home = self.pin([Member("demo/bin/demo")])

        def rename(src, dst):
            raise OSError(errno.EIO, "input/output error", src)

        code, lines = self.run_main("install", rename=rename)
        self.assert_refused(code, lines, "input/output error")
        self.assertFalse(os.path.lexists(home))
        self.assertEqual(os.listdir(self.cache), [])  # our temporary directory went too

    def test_a_winner_with_a_bad_stamp_fails_the_loser_and_is_kept(self):
        home = self.pin([Member("demo/bin/demo")])

        def fetch(url, sink):
            install._fetch(url, sink)
            os.makedirs(home)
            with open(os.path.join(home, "half"), "wb") as f:
                f.write(b"half a tree\n")

        code, lines = self.run_main("install", fetch=fetch)
        self.assert_refused(code, lines, home, "make clean-tools")
        self.assertEqual(os.listdir(self.cache), [os.path.basename(home)])
        self.assertEqual(os.listdir(home), ["half"])

    def test_an_interrupted_install_leaves_no_final_directory_and_is_recovered(self):
        # Plan check case 16: a killed install leaves only its tmp.* and dl.*, which are not touched,
        # never mistaken for a tool, and cleared by make clean-tools.
        home = self.pin([Member("demo/bin/demo")])
        os.makedirs(os.path.join(self.cache, "tmp.1.abc123", "half"))
        with open(os.path.join(self.cache, "dl.1.def456"), "wb") as f:
            f.write(b"partial")
        code, lines = self.run_main("install")
        self.assertEqual((code, lines), (0, ["demo 1.0: installed"]))
        self.assertEqual(
            sorted(os.listdir(self.cache)), sorted([os.path.basename(home), "dl.1.def456", "tmp.1.abc123"])
        )
        self.assertEqual(self.run_main("clean-tools")[0], 0)
        self.assertFalse(os.path.lexists(self.cache))

    def test_ctrl_c_during_a_download_cleans_up_and_keeps_the_exit_contract(self):
        home = self.pin([Member("demo/bin/demo")])

        def fetch(url, sink):
            raise KeyboardInterrupt

        code, lines = self.run_main("install", fetch=fetch)
        self.assert_refused(code, lines, "interrupted", "make tools")
        self.assertFalse(os.path.lexists(home))
        self.assertEqual(os.listdir(self.cache), [])


class CleanTools(InstallTest):
    def test_a_read_only_tree_with_symlinks_is_removed(self):
        self.pin([Member("demo/bin/demo"), Member("demo/bin/l", "l", link="demo"), Member("demo/d", "l", link="gone")])
        self.assertEqual(self.run_main("install")[0], 0)
        code, lines = self.run_main("clean-tools")
        self.assertEqual((code, lines), (0, [f"{self.cache}: removed"]))
        self.assertFalse(os.path.lexists(self.cache))

    def test_write_is_restored_without_following_symlinks(self):
        # The links lead out of the tree: chmod must change neither their targets nor anything else.
        home = self.pin([Member("demo/bin/demo")])
        self.assertEqual(self.run_main("install")[0], 0)
        outside_file = os.path.join(self.world, "outside.txt")
        outside_dir = os.path.join(self.world, "outside-dir")
        with open(outside_file, "wb") as f:
            f.write(b"mine\n")
        os.makedirs(outside_dir)
        os.chmod(outside_file, 0o444)
        os.chmod(outside_dir, 0o555)
        os.chmod(home, 0o700)
        os.symlink(outside_file, os.path.join(home, "to-file"))
        os.symlink(outside_dir, os.path.join(home, "to-dir"))
        os.chmod(home, 0o500)
        self.assertEqual(self.run_main("clean-tools")[0], 0)
        self.assertFalse(os.path.lexists(self.cache))
        self.assertEqual(stat.S_IMODE(os.stat(outside_file).st_mode), 0o444)
        self.assertEqual(stat.S_IMODE(os.stat(outside_dir).st_mode), 0o555)

    def test_no_cache_is_nothing_to_do(self):
        code, lines = self.run_main("clean-tools")
        self.assertEqual((code, lines), (0, [f"{self.cache}: nothing to remove"]))

    def test_a_cache_the_installer_did_not_write_is_refused_and_kept(self):
        # VORSAR_CEP_TOOLS_CACHE=$HOME must not delete a home directory.
        os.makedirs(self.cache)
        with open(os.path.join(self.cache, "thesis.doc"), "wb") as f:
            f.write(b"years of work\n")
        code, lines = self.run_main("clean-tools")
        self.assert_refused(code, lines, "thesis.doc", "VORSAR_CEP_TOOLS_CACHE")
        self.assertEqual(os.listdir(self.cache), ["thesis.doc"])

    def test_a_cache_that_is_a_symlink_is_refused_and_its_target_kept(self):
        target = os.path.join(self.world, "elsewhere")
        os.makedirs(target)
        os.chmod(target, 0o555)
        os.symlink(target, self.cache)
        code, lines = self.run_main("clean-tools")
        self.assert_refused(code, lines, self.cache)
        self.assertTrue(os.path.isdir(target))
        self.assertEqual(stat.S_IMODE(os.stat(target).st_mode), 0o555)


class Exec(InstallTest):
    INTERPRETER = b"#!the pinned interpreter, never really run in this test class\n"

    def pin_python(self, *, exec_sha256: dict[str, str] | None = None) -> str:
        """A cache holding only the gate's interpreter; returns the absolute path of the interpreter."""
        files = exec_sha256 or {"bin/python3.15": support.sha256(self.INTERPRETER)}
        entry = support.tool_entry(
            "unused",
            name="python",
            version="3.15.2",
            command="python3.15",
            exec_sha256=files,
            pinned="ab" * 32,
            url=UNREACHABLE,
        )
        support.write_tools(self.repo, [entry])
        home = os.path.join(self.cache, f"python-3.15.2-{'ab' * 6}")
        os.makedirs(os.path.join(home, "bin"))
        path = os.path.join(home, "bin", "python3.15")
        with open(path, "wb") as f:
            f.write(self.INTERPRETER)
        os.chmod(path, 0o555)
        return path

    def recorder(self) -> tuple[list, object]:
        calls: list[tuple[str, list[str]]] = []
        return calls, lambda path, argv: calls.append((path, argv))

    def test_the_interpreter_is_started_by_its_absolute_path_with_the_pinned_flags(self):
        python = self.pin_python()
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", "--tier", "T2", execv=execv)
        self.assertEqual((code, lines), (0, []))
        run_py = os.path.join(self.repo, "gate", "run.py")
        self.assertEqual(calls, [(python, [python, "-I", "-B", "-X", "utf8", run_py, "--tier", "T2"])])
        self.assertTrue(os.path.isabs(python))

    def test_a_relative_cache_still_gives_an_absolute_interpreter(self):
        python = self.pin_python()
        self.environ["VORSAR_CEP_TOOLS_CACHE"] = os.path.relpath(self.cache)
        calls, execv = self.recorder()
        self.assertEqual(self.run_main("exec", "--", execv=execv)[0], 0)
        self.assertEqual(calls[0][0], python)
        self.assertEqual(calls[0][1][0], python)

    def test_one_flipped_byte_is_refused_and_the_runner_never_starts(self):
        # QA counter-example 37; the test above is the witness that execv is called on a good file.
        python = self.pin_python()
        os.chmod(python, 0o755)
        with open(python, "r+b") as f:
            f.write(b"X")
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "run make tools", "bin/python3.15")
        self.assertEqual(calls, [])

    def test_a_missing_interpreter_is_refused_naming_make_tools(self):
        self.pin_python()
        support.restore_write(self.cache)
        os.unlink(os.path.join(self.cache, f"python-3.15.2-{'ab' * 6}", "bin", "python3.15"))
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "run make tools")
        self.assertEqual(calls, [])

    def test_an_empty_cache_is_refused_naming_make_tools(self):
        self.pin_python()
        support.restore_write(self.cache)
        install.clean_tools(self.cache, io.StringIO())
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "run make tools")
        self.assertEqual(calls, [])

    def test_every_listed_executable_is_hashed_not_only_the_interpreter(self):
        python = self.pin_python(
            exec_sha256={"bin/python3.15": support.sha256(self.INTERPRETER), "bin/zz-other": "cd" * 32}
        )
        with open(os.path.join(os.path.dirname(python), "zz-other"), "wb") as f:
            f.write(b"other\n")
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "bin/zz-other")
        self.assertEqual(calls, [])

    def test_an_interpreter_that_resolves_outside_the_cache_is_refused_though_its_bytes_match(self):
        python = self.pin_python()
        outside = os.path.join(self.world, "byte-identical")
        with open(outside, "wb") as f:
            f.write(self.INTERPRETER)
        os.unlink(python)
        os.symlink(outside, python)
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "outside the cache")
        self.assertEqual(calls, [])

    def test_an_interpreter_the_manifest_does_not_hash_is_refused(self):
        python = self.pin_python(exec_sha256={"bin/other": support.sha256(self.INTERPRETER)})
        with open(os.path.join(os.path.dirname(python), "other"), "wb") as f:
            f.write(self.INTERPRETER)
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "bin/python3.15", "exec_sha256")
        self.assertEqual(calls, [])

    def test_a_manifest_without_the_gate_python_is_refused(self):
        self.pin_python()
        support.write_tools(self.repo, [support.tool_entry("unused", pinned="ab" * 32, url=UNREACHABLE)])
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "python", "tools.json")
        self.assertEqual(calls, [])

    def test_a_failing_execv_is_refused_naming_make_tools(self):
        self.pin_python()

        def execv(path, argv):
            raise PermissionError(errno.EACCES, "permission denied", path)

        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "permission denied", "run make tools")


class Main(InstallTest):
    def test_a_bad_command_line_prints_the_usage(self):
        for argv in ([], ["bogus"], ["install", "extra"], ["exec"], ["exec", "x"], ["clean-tools", "x"]):
            with self.subTest(argv=argv):
                code, lines = self.run_main(*argv)
                self.assert_refused(code, lines, "usage:", "install", "exec --", "clean-tools")

    def test_a_broken_manifest_is_refused_naming_tools_json(self):
        with open(os.path.join(self.repo, "tools.json"), "wb") as f:
            f.write(b"{")
        for argv in (["install"], ["exec", "--"]):
            with self.subTest(argv=argv):
                code, lines = self.run_main(*argv)
                self.assert_refused(code, lines, "tools.json")

    def test_a_crash_is_exit_2_never_exit_1(self):
        self.pin([Member("demo/bin/demo")])

        def fetch(url, sink):
            raise RuntimeError("a bug in the installer")

        code, lines = self.run_main("install", fetch=fetch)
        self.assert_refused(code, lines, "RuntimeError", "a bug in the installer")
        self.assertEqual(os.listdir(self.cache), [])

    def test_a_refused_tool_stops_the_install(self):
        support.write_archive(self.archive, [Member("demo/bin/demo")])
        tools = [
            support.tool_entry(self.archive, name="first", url=UNREACHABLE),
            support.tool_entry(self.archive, name="second"),
        ]
        support.write_tools(self.repo, tools)
        code, lines = self.run_main("install")
        self.assert_refused(code, lines, UNREACHABLE)
        self.assertEqual(os.listdir(self.cache), [])
        self.assertNotIn("second 1.0: installed", lines)


if __name__ == "__main__":
    unittest.main()
