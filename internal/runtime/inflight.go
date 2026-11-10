package runtime

import "sync"

// inflightGate is the lifecycle's in-flight gate (known issue S3.3-1-2; design s3-3-2 §9.1): Add and Activate enter it;
// Close closes it and waits, bounded by its ctx, for the calls inside to leave.
type inflightGate struct {
	mu      sync.Mutex
	closing bool
	n       int
	idle    chan struct{} // closed when closing and n reaches 0
}

// enter admits a call unless Close has begun.
func (g *inflightGate) enter() bool { panic("runtime: inflightGate.enter: not built (S3.3-2 step 7)") }

// leave ends an admitted call.
func (g *inflightGate) leave() { panic("runtime: inflightGate.leave: not built (S3.3-2 step 7)") }

// close refuses every later call; the channel is closed once the calls inside have left.
func (g *inflightGate) close() <-chan struct{} {
	panic("runtime: inflightGate.close: not built (S3.3-2 step 7)")
}
