package playback_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package's tests when a goroutine outlives them (ADR-0015 §4; S3.2b's item 7, owner Q6): every
// runtime a test starts is closed, and Close stops every goroutine it started.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
