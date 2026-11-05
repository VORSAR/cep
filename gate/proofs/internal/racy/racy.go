// Package racy is the race row's planted defect (gate/proofs/drivers/test_race_planted.py): two goroutines write one
// counter with no synchronisation.
package racy

import "sync"

// Count adds 1 from each of n goroutines, unsynchronised: a data race the race detector reports.
func Count(n int) int {
	total := 0
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total++
		}()
	}
	wg.Wait()
	return total
}
