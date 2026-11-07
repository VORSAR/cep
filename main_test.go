package cep_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package's tests when a goroutine outlives them (design s3-3-1 §9; rules.json test_external):
// every runtime a test starts is closed, and Close stops every goroutine the engine started.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
