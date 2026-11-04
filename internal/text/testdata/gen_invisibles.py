"""Writes invisibles.txt: the code points SEMANTICS §5.1 stage 1 flags, as the reference executor decides them.

The oracle is docs/language/checks/executor.py's own _strip_invisibles, imported, never re-implemented: a code point
is listed when _strip_invisibles(chr(cp)) flags it. The fixture records the Unicode version of the interpreter's
unicodedata, which internal/text's cross-check asserts equals text.UnicodeTables.

Run from anywhere with the pinned interpreter (CPython 3.15.2, Unicode 17.0.0):

    python3.15 -I internal/text/testdata/gen_invisibles.py
"""

import pathlib
import sys
import unicodedata

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(ROOT / "docs" / "language" / "checks"))

from executor import _strip_invisibles  # noqa: E402  (the oracle's directory joins sys.path first)


def ranges():
    """The flagged code points over 0..0x10FFFF, as inclusive (first, last) runs."""
    out = []
    start = None
    for cp in range(0x110000):
        _, flagged = _strip_invisibles(chr(cp))
        if flagged and start is None:
            start = cp
        elif not flagged and start is not None:
            out.append((start, cp - 1))
            start = None
    if start is not None:
        out.append((start, 0x10FFFF))
    return out


def main():
    runs = ranges()
    total = sum(last - first + 1 for first, last in runs)
    lines = [
        "# The invisible code points of SEMANTICS §5.1 stage 1 (Cf, Co, Cn, and Cc other than TAB, LF and CR), as",
        "# docs/language/checks/executor.py's _strip_invisibles flags each one alone. Generated; do not edit.",
        "# Regenerate from the module root with the pinned interpreter:",
        "#     python3.15 -I internal/text/testdata/gen_invisibles.py",
        "# Format: one 'unicode <version>' line (the interpreter's unicodedata.unidata_version), then one",
        "# 'XXXX..YYYY' line per inclusive run of flagged code points, in hex, ascending.",
        f"# {total} code points in {len(runs)} runs.",
        f"unicode {unicodedata.unidata_version}",
    ]
    lines += [f"{first:04X}..{last:04X}" for first, last in runs]
    (HERE / "invisibles.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
