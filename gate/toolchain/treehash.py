"""One tool's tree hash, in a process of its own (S1.1c-3 design §3.6; owner, S1.1c-3's plan check, decision 10).

    python3.15 -I -B gate/toolchain/treehash.py <root>
    python3.15 -I -B gate/toolchain/treehash.py --details <root> [<root> ...]

The first form resolves `real = os.path.realpath(root)` once and prints three lines: `real`, then
`manifest.tree_details(real)`, then `manifest.tree_sha256(real)`, and exits 0. The details are read before the hash
and over the same resolved path, so a hand-off lists the tree that was hashed, never what a link named afterwards
(proof-world speed-up design note §3.2, QB-S9, CA-L1). The second form prints, for each root in order, its realpath
and its details: the reader's one capped child for every tree it would take (CA-L2). An OSError prints its text on
stderr and exits 3 in either form. Parity starts it in the run's parity group with a cap of its own
(`parity.tree_digest`, `parity.tree_details_of`), so a walk stuck on a slow or broken disk can be stopped and
killed, which a walk in the runner's own process could not (BA10).
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate.toolchain import manifest

UNREADABLE = 3  # parity.TREE_UNREADABLE: the one failed exit parity reads as a damaged tree
DETAILS = "--details"


def _lines(argv: list[str]) -> list[bytes]:
    if argv[1] == DETAILS:
        lines = []
        for root in argv[2:]:
            real = os.path.realpath(root)
            lines += [os.fsencode(real), manifest.tree_details(real).encode("ascii")]
        return lines
    real = os.path.realpath(argv[1])
    details = manifest.tree_details(real)  # before the hash: there is no second reading (CA-L1)
    return [os.fsencode(real), details.encode("ascii"), manifest.tree_sha256(real).encode("ascii")]


def main(argv: list[str]) -> int:
    try:
        lines = _lines(argv)
    except OSError as e:
        print(e, file=sys.stderr, flush=True)
        return UNREADABLE
    sys.stdout.buffer.write(b"".join(line + b"\n" for line in lines))  # a path's bytes as stored, never re-encoded
    sys.stdout.buffer.flush()
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
