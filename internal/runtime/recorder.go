package runtime

import (
	"fmt"
	"sync"

	"github.com/vorsar/cep/internal/event"
)

// recKind is a recording record's kind.
type recKind uint8

const (
	rHeader recKind = iota + 1
	rEvent
	rRejected
	rTick
	rTable
	rStop
	rMarker // a drain's flush marker: closed when every record before it has been handed to the Recorder
)

// recItem is one record on an app's recording queue (design s3-2b-2 §7). Everything in it is immutable once put: the
// Event is a copy whose Values the router and the shards only read; payload and rows are the record's own.
type recItem struct {
	kind    recKind
	ev      event.Event
	arrival int64
	payload []byte
	stream  int
	fault   *event.DecodeError
	table   int
	op      TableOp
	rows    [][]event.Value
	marker  chan struct{}
}

// recQ is one app's recording (owner D2 of S3.2b-2's design; review F2; MUST 13): a bounded queue the router puts
// records on without ever blocking, drained in order by the app's recorder goroutine, the only caller of the
// Recorder. A full queue or the Recorder's first error ends the recording for good: the app goes on.
type recQ struct {
	q   chan recItem
	rec Recorder

	ended  bool  // the router's: the recording has ended (a full queue, or it saw the goroutine fail); put nothing more
	queued int64 // the router's: records put

	mu      sync.Mutex // guards the goroutine's results, read by Stats outside the cut
	records int64
	failed  bool
	err     string
}

func newRecQ(rec Recorder, capacity int) *recQ {
	return &recQ{q: make(chan recItem, capacity), rec: rec}
}

// put is the router's non-blocking hand-over; it reports whether the record was queued. A full queue ends the
// recording, with the record it could not take named.
func (q *recQ) put(it recItem) bool {
	if q == nil || q.ended {
		return false
	}
	if q.isFailed() {
		q.ended = true
		return false
	}
	select {
	case q.q <- it:
		q.queued++
		return true
	default:
		q.ended = true
		q.fail(fmt.Sprintf("runtime: recording queue full at record %d: the recording stopped; detection goes on", q.queued+1))
		return false
	}
}

func (q *recQ) fail(msg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.failed {
		q.failed, q.err = true, msg
	}
}

func (q *recQ) isFailed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.failed
}

func (q *recQ) stats() RecordingStats {
	if q == nil {
		return RecordingStats{}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return RecordingStats{Records: q.records, Failed: q.failed, Err: q.err}
}

// run is the recorder goroutine: it waits only on its queue and its own Recorder. After a failure it calls the
// Recorder no more but still closes markers, so a drain never waits on a dead recording. At stop it writes what is
// queued, then flushes a Recorder that can.
func (q *recQ) run(r *Runtime) {
	defer r.wg.Done()
	for {
		select {
		case it := <-q.q:
			q.handle(it)
		case <-r.stopped:
			for {
				select {
				case it := <-q.q:
					q.handle(it)
				default:
					if f, ok := q.rec.(interface{ Flush() error }); ok && !q.isFailed() {
						_ = f.Flush()
					}
					return
				}
			}
		}
	}
}

func (q *recQ) handle(it recItem) {
	if it.kind == rMarker {
		close(it.marker)
		return
	}
	if q.isFailed() {
		return
	}
	var err error
	switch it.kind {
	case rHeader:
		err = q.rec.Header(it.arrival)
	case rEvent:
		err = q.rec.Event(&it.ev, it.arrival, it.payload)
	case rRejected:
		err = q.rec.Rejected(it.stream, it.arrival, it.fault)
	case rTick:
		err = q.rec.Tick(it.arrival)
	case rTable:
		err = q.rec.Table(it.table, it.op, it.rows)
	case rStop:
		err = q.rec.Stop()
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case err != nil && !q.failed:
		q.failed, q.err = true, err.Error()
	case err == nil:
		q.records++
	}
}
