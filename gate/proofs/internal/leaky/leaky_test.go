package leaky

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain is the leak step as the engine's packages run it (S3.2b's item 7, owner Q6): a goroutine that outlives the
// package's tests fails the package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestStart passes on its own; only the leak check fails the package, after it.
func TestStart(t *testing.T) {
	Start()
}
