"""Break-it of S1.1a-1's combined build (QA), QB3: a FIFO at a hashed path hangs parity and the bootstrap.

parity._differs and install.exec open every exec_sha256 entry with a plain open() to hash it. If the entry is a
FIFO (a tampered or damaged cache), open() for reading blocks until some process opens it for writing, which
nothing ever does: parity never returns, so a real run waits for the run ceiling (about 39 minutes at T2) and then
reports the ceiling instead of the tool; `bootstrap.py exec` has no ceiling at all and hangs `make check` forever.
The witness: the same path holding a regular file of other bytes is named at once.

Each call runs in a daemon thread, and a hung one is released after its cap by opening the FIFO for writing and
closing it, so the thread ends and the test owns its world.
"""

import os
import threading

from gate import wait
from gate.toolchain.tests.test_install import Exec, InstallTest
from gate.toolchain.tests.test_parity import RECOVER, ParityTest

CAP_S = 30  # hashing a small fake tool takes milliseconds


def returns_within_cap(test, call, fifo: str) -> object:
    """call()'s result; fails the test, naming the FIFO, if it has not returned after CAP_S."""
    result: list[object] = []
    worker = threading.Thread(target=lambda: result.append(call()), name="qb3-call", daemon=True)
    worker.start()
    try:
        wait.until(f"the call to return though {fifo} is a FIFO", lambda: not worker.is_alive(), cap_s=CAP_S)
    except wait.WaitTimeout:
        fd = os.open(fifo, os.O_WRONLY | os.O_NONBLOCK)  # the blocked reader is there, so this cannot fail
        os.close(fd)
        wait.until("the released call to return", lambda: not worker.is_alive(), cap_s=CAP_S)
        test.fail(f"hung for {CAP_S}s opening the FIFO {fifo} to hash it; after release it gave {result}")
    return result[0]


def make_fifo(path: str) -> None:
    os.chmod(os.path.dirname(path), 0o755)
    os.unlink(path)
    os.mkfifo(path)


class QB3ParityFifo(ParityTest):
    def gofmt(self):
        tool = self.tool("go", files={"bin/gofmt": b"#!a fake gofmt\n"})
        return tool, os.path.join(self.home("go"), "bin", "gofmt")

    def test_qb3_witness_other_bytes_at_the_path_are_named_at_once(self):
        tool, path = self.gofmt()
        os.chmod(path, 0o755)
        with open(path, "wb") as f:
            f.write(b"#!other bytes\n")
        self.assertEqual(self.check(tool), [f"go: sha256 of bin/gofmt differs: {RECOVER}"])

    def test_qb3_parity_names_a_fifo_at_a_hashed_path_instead_of_hanging(self):
        tool, path = self.gofmt()
        make_fifo(path)
        reasons = returns_within_cap(self, lambda: self.check(tool), path)
        self.assertEqual(reasons, [f"go: sha256 of bin/gofmt differs: {RECOVER}"])


class QB3ExecFifo(InstallTest):
    INTERPRETER = Exec.INTERPRETER
    pin_python = Exec.pin_python  # borrowed helpers only: inheriting Exec would run its tests again here
    recorder = Exec.recorder

    def test_qb3_witness_other_bytes_are_refused_at_once(self):
        python = self.pin_python()
        os.chmod(python, 0o755)
        with open(python, "wb") as f:
            f.write(b"#!other bytes\n")
        calls, execv = self.recorder()
        code, lines = self.run_main("exec", "--", execv=execv)
        self.assert_refused(code, lines, "bin/python3.15")
        self.assertEqual(calls, [])

    def test_qb3_exec_refuses_a_fifo_interpreter_instead_of_hanging(self):
        python = self.pin_python()
        make_fifo(python)
        calls, execv = self.recorder()
        code, lines = returns_within_cap(self, lambda: self.run_main("exec", "--", execv=execv), python)
        self.assert_refused(code, lines, "bin/python3.15")
        self.assertEqual(calls, [])


del Exec, InstallTest, ParityTest  # bases and helpers only: unittest must not collect them from this module
