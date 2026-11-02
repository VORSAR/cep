"""The gate's one entry point; any python3 can start it (design note §3).

Held to Python 3.9 syntax by ruff's per-file target, so that Apple's 3.9.6 reaches the fix text instead
of a SyntaxError. Everything newer lives in gate/toolchain/install.py, imported after the floor check.
"""

import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def refuse(reason):
    print(reason)
    print("GATE FAILED")
    print("exit: 2")
    return 2


def main(argv):
    if sys.version_info < (3, 12):
        return refuse("install Python 3.12+ (e.g. brew install python)")
    sys.dont_write_bytecode = True
    sys.path[:] = [os.path.dirname(HERE)] + [p for p in sys.path if os.path.abspath(p) != HERE]
    try:
        from gate.toolchain import install

        return install.main(argv)
    except Exception as e:
        return refuse(f"bootstrap crashed: {type(e).__name__}: {e}")


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
