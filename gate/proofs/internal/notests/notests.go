// Package notests is a planted defect for the coverage-inclusion proof: it has code, and no _test.go file at all.
package notests

// Double is twice n.
func Double(n int) int {
	return 2 * n
}
