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
// Recorder. A full queue or the Recorder's first error or panic ends the recording for good: the app goes on. After a
// full queue the goroutine still writes every record already queued, then flushes, so the file holds whole lines
// (review C3).
type recQ struct {
	q        chan recItem
	rec      Recorder
	overflow chan int64 // the router's one signal that the queue filled: the records queued before it

	ended  bool  // the router's: the recording has ended (a full queue, or it saw the goroutine fail); put nothing more
	queued int64 // the router's: records put

	broken bool // the goroutine's: the Recorder failed or panicked; it is never called again

	mu      sync.Mutex // guards the results, read by Stats outside the cut
	records int64
	failed  bool // the recording has ended: a full queue (set by the router at once) or the Recorder's failure
	err     string
}

func newRecQ(rec Recorder, capacity int) *recQ {
	return &recQ{q: make(chan recItem, capacity), rec: rec, overflow: make(chan int64, 1)}
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
		q.fail(fmt.Sprintf("runtime: recording queue full after record %d: the recording stopped; detection goes on", q.queued))
		q.overflow <- q.queued // once: ended stops every later put; the goroutine writes what is queued
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
		case <-q.overflow: // every record queued before the overflow is in q: write them, then flush (review C3)
			for {
				select {
				case it := <-q.q:
					q.handle(it)
					continue
				default:
				}
				break
			}
			q.flush()
		case <-r.stopped:
			for {
				select {
				case it := <-q.q:
					q.handle(it)
				default:
					q.flush()
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
	if q.broken {
		return
	}
	err := q.call(it)
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case err != nil:
		q.broken = true
		if !q.failed {
			q.failed, q.err = true, err.Error()
		}
	default:
		q.records++
	}
}

// flush flushes a Recorder that can, unless it has failed: a file then ends on a whole line.
func (q *recQ) flush() {
	if f, ok := q.rec.(interface{ Flush() error }); ok && !q.broken {
		_ = f.Flush()
	}
}

// call hands one record to the Recorder; a panic is the recording's failure, never the process's (break-it B4), as a
// sink's is counted.
func (q *recQ) call(it recItem) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("runtime: the recorder panicked: %v", p)
		}
	}()
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
	return err
}
