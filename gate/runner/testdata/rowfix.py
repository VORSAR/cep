"""A fixture row: runs the actions on its command line, in order (design note §9).

Stands in for a real row in the runner's tests. Standalone (run with -I), so it imports nothing from gate/.

    start PATH     open a FIFO for writing, write one start byte and hold it open for life
    wait PATH      open a FIFO for reading, read one byte, close it (blocks until released)
    openw PATH     open a FIFO for writing and hold it (a rendezvous, no byte)
    openr PATH     open a FIFO for reading and hold it
    print TEXT     write TEXT and a newline to stdout
    printhex HEX   write raw bytes to stdout
    flood N        write N bytes of invalid UTF-8 to stdout
    status N       the exit status to use when the actions run out
    term PATH      hold PATH open for writing with SIGTERM blocked, write a start byte, then forever: wait for
                   SIGTERM and write one byte (TERM is not fatal). sigwait on a blocked signal loses none; a
                   Python-level handler with pause() would, when TERM lands just before the pause() call.
    killself N     send this process signal N
    spawn N ...    start a child in this process group running the next N tokens as actions, and carry on
    needfile PATH  exit 1 naming the file, if it is missing
    envfile NAME PATH   write the value of NAME to PATH
    stdinfile PATH      write to PATH what stdin reads
    block          wait for a signal, forever
"""

import os
import signal
import subprocess
import sys


def _write(fd: int, data: bytes) -> None:
    while data:
        data = data[os.write(fd, data) :]


def run(actions: list[str]) -> int:
    held = []
    status = 0
    while actions:
        action, *actions = actions
        if action == "start":
            path, *actions = actions
            fd = os.open(path, os.O_WRONLY)
            held.append(fd)
            _write(fd, b"s")
        elif action == "wait":
            path, *actions = actions
            with open(path, "rb") as f:
                f.read(1)
        elif action == "openw":
            path, *actions = actions
            held.append(os.open(path, os.O_WRONLY))
        elif action == "openr":
            path, *actions = actions
            held.append(os.open(path, os.O_RDONLY))
        elif action == "print":
            text, *actions = actions
            _write(1, text.encode() + b"\n")
        elif action == "printhex":
            text, *actions = actions
            _write(1, bytes.fromhex(text))
        elif action == "flood":
            size, *actions = actions
            for _ in range(int(size) // 65536 + 1):
                _write(1, b"\xff" * 65535 + b"\n")
        elif action == "status":
            value, *actions = actions
            status = int(value)
        elif action == "term":
            path, *actions = actions
            fd = os.open(path, os.O_WRONLY)
            signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM})
            _write(fd, b"s")
            while True:
                signal.sigwait({signal.SIGTERM})
                _write(fd, b"T")
        elif action == "killself":
            value, *actions = actions
            os.kill(os.getpid(), int(value))
        elif action == "spawn":
            # A grandchild left running on purpose: this file is what the "left running" verdict catches.
            count, *actions = actions
            child, actions = actions[: int(count)], actions[int(count) :]
            subprocess.Popen([sys.executable, "-I", "-B", os.path.abspath(__file__), *child])
        elif action == "needfile":
            path, *actions = actions
            if not os.path.exists(path):
                print(f"missing artifact: {path}", flush=True)
                return 1
        elif action == "envfile":
            name, path, *actions = actions
            with open(path, "w", encoding="utf-8") as f:
                f.write(os.environ.get(name, "<unset>"))
        elif action == "stdinfile":
            path, *actions = actions
            with open(path, "wb") as f:
                f.write(os.read(0, 16) + b"|eof")
        elif action == "block":
            while True:
                signal.pause()
        else:
            raise SystemExit(f"rowfix: unknown action {action!r}")
    return status


if __name__ == "__main__":
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    sys.exit(run(sys.argv[1:]))
