// Package leaky is the leak step's planted defect (gate/proofs/drivers/test_leak_planted.py): Start leaves a goroutine
// running after its caller has returned.
package leaky

// Start starts a goroutine that blocks forever on a channel nobody closes: a leak goleak reports.
func Start() {
	block := make(chan struct{})
	go func() { <-block }()
}
