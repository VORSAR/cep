package racy

import "testing"

// TestCount runs the planted race; without -race it passes (the count is not asserted), so only the race row fails it.
func TestCount(t *testing.T) {
	if got := Count(2); got < 0 {
		t.Fatalf("Count(2) = %d", got)
	}
}
