package cepio

import "testing"

// TestModeAndTypeValues (design s3-3-1 §5): the contract's constants are the values the spec writes down (the
// coverage row needs one passing test in a package with no statements).
func TestModeAndTypeValues(t *testing.T) {
	t.Parallel()
	if Append != 1 || Changelog != 2 {
		t.Errorf("Append %d, Changelog %d; want 1, 2", Append, Changelog)
	}
	if Invalid != 0 || String != 1 || Int != 2 || Float != 3 || Bool != 4 || Timestamp != 5 || Duration != 6 || Object != 7 {
		t.Errorf("Type values %d %d %d %d %d %d %d %d; want 0..7", Invalid, String, Int, Float, Bool, Timestamp, Duration, Object)
	}
}
