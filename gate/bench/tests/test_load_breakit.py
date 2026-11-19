"""Break-it (S1.1c-6 combined build): load.py's verdict on a test binary that dies outside a test's own result."""

import os
import unittest

from gate.bench.tests import support
from gate.bench.tests.test_load import MODULE, SOURCE, Base

# TestOK passes; TestSlow outlives go test's -timeout, so the binary panics ("panic: test timed out after …") while
# TestSlow runs: the package fails, and the panic text reaches go test -json's stdout as output events.
TESTS = """\
package l

import (
\t"testing"
\t"time"
)

func TestOK(t *testing.T) {
\tif One() != 1 {
\t\tt.Fatal("One() != 1")
\t}
}

func TestSlow(t *testing.T) {
\ttime.Sleep(time.Minute)
}
"""


class DiedOutsideATest(Base):
    def setUp(self) -> None:
        super().setUp()
        self.module = os.path.join(self.tmp, "mod")
        support.write(self.module, {"go.mod": MODULE, "l.go": SOURCE, "l_test.go": TESTS})

    def test_a_binary_that_times_out_names_the_test_or_the_cause(self):
        # The verdict must say why the run is red: the test that was running (below N/N) or the panic's line. Today
        # the tail printed is stderr's, which is empty under -json, so the verdict reads "below N/N: none; go test
        # exit 1" with no cause, as it did for internal/event under -race -count=4 -cpu 1,4 and 28 hogs.
        code, out = self.load(["--hogs", "1", "--", "-count=1", "-timeout=3s", "."], os.environ, self.module)
        self.assertEqual(code, 1, out)
        self.assertTrue("TestSlow" in out.split("verdict:")[-1] or "timed out" in out, out)


if __name__ == "__main__":
    unittest.main()
