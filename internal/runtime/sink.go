package runtime

import (
	"sync/atomic"

	"github.com/vorsar/cep/internal/event"
)

type sinkQ struct {
	q        chan sinkItem
	sink     Sink
	errors   atomic.Int64
	high     int           // the router's: the longest q has been after a send
	returned chan struct{} // closed by the flusher when it returns (design s3-3-1 §2.4)
}

type sinkItem struct {
	ev     event.Event
	marker chan struct{} // non-nil: a drain's flush marker, closed on reaching it
	last   bool          // with marker: the flusher returns after closing it (Remove; design s3-3-1 §2.4)
}

// ---- sink flushers ----

// run is a flusher's loop: it waits only on its queue and its own sink (design §3); a marker is a close.
func (q *sinkQ) run(r *Runtime) {
	defer r.wg.Done()
	defer close(q.returned)
	for {
		if r.stopping() {
			return
		}
		select {
		case it := <-q.q:
			if r.stopping() { // stop won the race: drop the rest (design §6)
				return
			}
			if it.marker != nil {
				close(it.marker)
				if it.last { // Remove's last marker: nothing follows (design s3-3-1 §2.4; the queue is never closed)
					return
				}
				continue
			}
			q.call(it.ev)
		case <-r.stopped:
			return
		}
	}
}

// call delivers one event; an error or a panic is counted (review F8).
func (q *sinkQ) call(e event.Event) {
	defer func() {
		if recover() != nil {
			q.errors.Add(1)
		}
	}()
	if err := q.sink(e); err != nil {
		q.errors.Add(1)
	}
}
