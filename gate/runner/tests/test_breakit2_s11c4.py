"""S1.1c-4 second break-it (QA): the vulns row's copy under a hostile zip. Each test is red on the code it was written
against, for the reason its comment states; never the network."""

import io
import json
import tracemalloc
import unittest
import zipfile

from gate.runner import vulndb
from gate.runner.rows import vulns

ENTRIES = 24  # 24 members of about MEMBER_CAP each: 96 MiB decompressed from a zip of a few hundred KiB


def bomb() -> bytes:
    """A zip in the database's layout whose every listed entry decompresses to just under MEMBER_CAP (whitespace
    around a well-formed entry), so no single member is refused."""
    ids = [f"GO-2099-{n:04d}" for n in range(ENTRIES)]
    body = io.BytesIO()
    with zipfile.ZipFile(body, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        z.writestr(vulndb.DB_JSON, json.dumps({"modified": "2026-11-12T00:00:00Z"}))
        z.writestr(vulndb.MODULES_JSON, json.dumps([{"path": "example.com/x", "vulns": [{"id": i} for i in ids]}]))
        z.writestr(vulndb.VULNS_JSON, json.dumps([{"id": i, "modified": "2026-11-12T00:00:00Z"} for i in ids]))
        for i in ids:
            entry = json.dumps({"id": i, "modified": "2026-11-12T00:00:00Z", "affected": []})
            z.writestr(f"ID/{i}.json", entry + " " * (vulndb.MEMBER_CAP - len(entry) - 1))
    return body.getvalue()


class BreakIt2(unittest.TestCase):
    def test_bk2_3_the_entries_read_never_hold_every_member_in_memory_at_once(self):
        # Red today: rows/vulns.py Copy.read caches every member's decompressed bytes for the whole scan (Copy.cache),
        # and nothing bounds their sum: MEMBER_CAP bounds one member, FETCH_CAP the compressed zip. Deflate packs
        # 4 MiB of whitespace into about 4 KiB, so a 64 MiB zip can list some 16,000 such entries, tens of GiB held at
        # once, and the row dies by the kernel's OOM kill with no named line. Here 24 entries: the peak is the sum
        # (about 96 MiB), not one member.
        data = bomb()
        self.assertLess(len(data), 1 << 20)
        copy = vulns.Copy("/nonexistent")
        copy.zip = zipfile.ZipFile(io.BytesIO(data))
        tracemalloc.start()
        try:
            entries, complete = vulns.entries_of(copy)
            _, peak = tracemalloc.get_traced_memory()
        finally:
            tracemalloc.stop()
        self.assertEqual((len(entries), complete), (ENTRIES, True))
        self.assertLess(
            peak,
            4 * vulndb.MEMBER_CAP,
            f"{peak >> 20} MiB held at once for {ENTRIES} members of 4 MiB from a {len(data) >> 10} KiB zip",
        )


if __name__ == "__main__":
    unittest.main()
