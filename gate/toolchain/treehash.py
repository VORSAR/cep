"""One tool's tree hash, in a process of its own (S1.1c-3 design §3.6; owner, S1.1c-3's plan check, decision 10).

    python3.15 -I -B gate/toolchain/treehash.py <root>

Prints `manifest.tree_sha256(root)` and exits 0; an OSError prints its text on stderr and exits 3. Parity starts it
in the run's parity group with a cap of its own (`parity.tree_digest`), so a hash stuck on a slow or broken disk can
be stopped and killed, which a hash in the runner's own process could not (BA10).
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from gate.toolchain import manifest

UNREADABLE = 3  # parity.TREE_UNREADABLE: the one failed exit parity reads as a damaged tree


def main(argv: list[str]) -> int:
    try:
        digest = manifest.tree_sha256(argv[1])
    except OSError as e:
        print(e, file=sys.stderr, flush=True)
        return UNREADABLE
    print(digest, flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
