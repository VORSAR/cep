package inmemory_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package's tests when a goroutine outlives them (design s3-3-1 §9; rules.json test_external).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
