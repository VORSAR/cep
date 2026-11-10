package runtime

import "sync"

// inflightGate is the lifecycle's in-flight gate (known issue S3.3-1-2; design s3-3-2 §9.1): Add and Activate enter
// it; Close closes it and waits, bounded by its ctx, for the calls inside to leave. Named so beside Runtime.gate, the
// per-app caller-side check of design s3-3-1 §2.5.
type inflightGate struct {
	mu      sync.Mutex
	closing bool
	n       int
	idle    chan struct{} // closed when closing and n reaches 0
}

// enter admits a call unless Close has begun.
func (g *inflightGate) enter() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return false
	}
	g.n++
	return true
}

// leave ends an admitted call.
func (g *inflightGate) leave() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n--
	if g.closing && g.n == 0 && g.idle != nil {
		close(g.idle)
		g.idle = nil
	}
}

// close refuses every later call; the channel is closed once the calls inside have left.
func (g *inflightGate) close() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closing = true
	ch := make(chan struct{})
	if g.n == 0 {
		close(ch)
	} else {
		g.idle = ch
	}
	return ch
}
