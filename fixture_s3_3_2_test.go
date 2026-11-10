package cep_test

import (
	"regexp"
	"testing"
)

// fcExampleWithoutExpects is a corpus example with its expect blocks removed: for the S3.3-1 tests that deploy 09 with
// a param override changing what its blocks assert, which Deploy now refuses (owner, 2026-11-08, S3.3-2's plan check
// decision 1). They test the override's live effect, not the blocks; the blocks are the conformance runner's.
func fcExampleWithoutExpects(t *testing.T, file string) string {
	t.Helper()
	src := fcExample(t, file)
	out := regexp.MustCompile(`(?s)\n  expect .*?(\n  select )`).ReplaceAllString(src, "$1")
	if out == src {
		t.Fatalf("%s: no expect block to remove", file)
	}
	return out
}
