// Package control is the coverage proofs' good package: fully covered, so beside each planted package it shows the
// instrument passes what it should, and the unit row has a test to run.
package control

// Sign is -1, 0 or +1, by the sign of n.
func Sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
