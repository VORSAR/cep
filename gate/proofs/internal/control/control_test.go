package control

import "testing"

func TestSign(t *testing.T) {
	for n, want := range map[int]int{-5: -1, 0: 0, 7: 1} {
		if got := Sign(n); got != want {
			t.Errorf("Sign(%d) = %d; want %d", n, got, want)
		}
	}
}
