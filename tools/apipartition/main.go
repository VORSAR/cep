// Command apipartition prints a package's exported surface as law text and diffs it against a golden file
// (ADR-0015 §3's apipartition row; design s3-3-1 §6.1). Stub until the build's step 6.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "usage: go run ./tools/apipartition [--write] <golden> <import path>...  (not built yet: S3.3-1)")
	os.Exit(2)
}
