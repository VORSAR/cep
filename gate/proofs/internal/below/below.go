// Package below is a planted defect for the coverage-inclusion proof: nine statements, eight of them covered by
// its test, so it sits just under the 90% floor (88.88%, truncated).
package below

// Clamp limits v to the range [lo, hi]; it reports lo > hi as an error.
func Clamp(v, lo, hi int) (int, bool) {
	if lo > hi {
		return 0, false
	}
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v, true
}

// Mid is the midpoint of [lo, hi], rounded towards lo.
func Mid(lo, hi int) int {
	half := (hi - lo) / 2
	return lo + half
}
