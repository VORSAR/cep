"""The cover profile: strict parsing, and duplicate blocks folded keeping the higher count (plan check S6)."""

from gate.coverage import coverprofile
from gate.coverage.coverprofile import Block, ProfileError
from gate.coverage.tests import support


def block(file: str, start: tuple[int, int], end: tuple[int, int], statements: int, count: int) -> Block:
    return Block(file, start[0], start[1], end[0], end[1], statements, count)


class Parse(support.CoverageTest):
    def test_every_mode_go_writes_is_read(self):
        for mode in ("set", "count", "atomic"):
            with self.subTest(mode=mode):
                blocks = coverprofile.parse(f"mode: {mode}\nex.com/m/a/a.go:3.16,3.26 1 4\n".encode())
                self.assertEqual(blocks, [block("ex.com/m/a/a.go", (3, 16), (3, 26), 1, 4)])

    def test_a_block_knows_its_package_and_file_name(self):
        (b,) = coverprofile.parse(b"mode: set\nex.com/m/internal/a/a.go:3.16,3.26 1 1\n")
        self.assertEqual((b.package, b.name), ("ex.com/m/internal/a", "a.go"))

    def test_an_empty_profile_has_no_blocks(self):
        self.assertEqual(coverprofile.parse(b"mode: set\n"), [])

    def test_the_last_newline_is_optional(self):
        self.assertEqual(len(coverprofile.parse(b"mode: set\nex.com/m/a/a.go:1.1,2.1 1 0")), 1)

    def test_duplicates_fold_keeping_the_higher_count_in_either_order(self):
        # dupes.prof, by hand: 3.1-4.1 seen 0 then 1 → 1; 5.1-6.1 seen 1 then 0 → 1; 7.1-8.1 once, 0.
        self.assertEqual(
            coverprofile.parse(support.fixture("dupes.prof")),
            [
                block("ex.com/m/internal/d/d.go", (3, 1), (4, 1), 2, 1),
                block("ex.com/m/internal/d/d.go", (5, 1), (6, 1), 3, 1),
                block("ex.com/m/internal/d/d.go", (7, 1), (8, 1), 5, 0),
            ],
        )

    def test_blocks_come_out_sorted_whatever_the_input_order(self):
        data = b"mode: set\nex.com/m/b/b.go:1.1,2.1 1 0\nex.com/m/a/a.go:9.1,9.9 1 0\nex.com/m/a/a.go:1.1,2.1 1 0\n"
        self.assertEqual(
            [(b.file, b.start_line) for b in coverprofile.parse(data)],
            [("ex.com/m/a/a.go", 1), ("ex.com/m/a/a.go", 9), ("ex.com/m/b/b.go", 1)],
        )


class Malformed(support.CoverageTest):
    """A profile the instrument cannot read is a failure, never a profile with fewer blocks."""

    CASES = {
        "empty": b"",
        "no mode line": b"ex.com/m/a/a.go:1.1,2.1 1 0\n",
        "unknown mode": b"mode: sometimes\n",
        "a second mode line": b"mode: set\nmode: set\n",
        "a blank line": b"mode: set\n\nex.com/m/a/a.go:1.1,2.1 1 0\n",
        "a missing count": b"mode: set\nex.com/m/a/a.go:1.1,2.1 1\n",
        "a negative count": b"mode: set\nex.com/m/a/a.go:1.1,2.1 1 -1\n",
        "a carriage return": b"mode: set\nex.com/m/a/a.go:1.1,2.1 1 0\r\n",
        "a non-ASCII digit": "mode: set\nex.com/m/a/a.go:1.1,2.1 1 ٣\n".encode(),
        "no package in the file name": b"mode: set\na.go:1.1,2.1 1 0\n",
        "two statement counts for one block": b"mode: set\nex.com/m/a/a.go:1.1,2.1 1 0\nex.com/m/a/a.go:1.1,2.1 2 1\n",
    }

    def test_each_is_refused_naming_its_line(self):
        for why, data in self.CASES.items():
            with self.subTest(why=why), self.assertRaises(ProfileError) as caught:
                coverprofile.parse(data)
            if why not in ("empty", "no mode line", "unknown mode"):
                self.assertRegex(str(caught.exception), r"^line [0-9]+: ", why)
