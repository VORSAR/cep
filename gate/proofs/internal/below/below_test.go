package below

import "testing"

// The case lo > hi is deliberately left untested: it is the one uncovered statement of the nine.
func TestClamp(t *testing.T) {
	cases := []struct{ v, lo, hi, want int }{{5, 0, 9, 5}, {-1, 0, 9, 0}, {10, 0, 9, 9}}
	for _, c := range cases {
		if got, ok := Clamp(c.v, c.lo, c.hi); got != c.want || !ok {
			t.Errorf("Clamp(%d, %d, %d) = %d, %v; want %d, true", c.v, c.lo, c.hi, got, ok, c.want)
		}
	}
}

func TestMid(t *testing.T) {
	if got := Mid(2, 9); got != 5 {
		t.Errorf("Mid(2, 9) = %d; want 5", got)
	}
}
