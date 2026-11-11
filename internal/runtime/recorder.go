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
	rMarker   // a drain's flush marker: closed when every record before it has been handed to the Recorder
	rBatchEnd // a source's batch-close signal (design s3-3-1 §2.4)
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
	batch   string // rBatchEnd
}

// recQ is one app's recording (owner D2 of S3.2b-2's design; review F2; MUST 13): a bounded queue the router puts
// records on without ever blocking, drained in order by the app's recorder goroutine, the only caller of the
// Recorder. A full queue or the Recorder's first error or panic ends the recording for good: the app goes on. After a
// full queue the goroutine still writes every record already queued, then flushes, so the file holds whole lines
// (review C3), and a drain waits for that through settled, so Stats after a drain counts them.
type recQ struct {
	q        chan recItem
	rec      Recorder
	overflow chan int64    // the router's one signal that the queue filled: the records queued before it
	stop     chan struct{} // closed by Remove's router step: write what is queued, flush, return (design s3-3-1 §2.4)
	returned chan struct{} // closed by the goroutine when it returns
	settled  chan struct{} // closed by the goroutine once an ended recording's output is final: the records queued before a full queue written and flushed, or the Recorder's failure

	ended  bool  // the router's: the recording has ended (a full queue, or it saw the goroutine fail); put nothing more
	queued int64 // the router's: records put

	broken    bool // the goroutine's: the Recorder failed or panicked; it is never called again
	isSettled bool // the goroutine's: settled is closed

	mu      sync.Mutex // guards the results, read by Stats outside the cut
	records int64
	failed  bool // the recording has ended: a full queue (set by the router at once) or the Recorder's failure
	err     string
	cutOff  bool // a drain's deadline cut the recording off while it still had writes to make (cut, once)
}

func newRecQ(rec Recorder, capacity int) *recQ {
	return &recQ{q: make(chan recItem, capacity), rec: rec, overflow: make(chan int64, 1), stop: make(chan struct{}), returned: make(chan struct{}), settled: make(chan struct{})}
}

// drainStop is the router's drain step: it queues the recording's stop and a marker closed once the stop is written,
// and returns the marker for Drain to wait on. A recording that has ended (a full queue, the Recorder's failure) takes
// neither; Drain then waits on settled, so a stopped recording's queued records are written and flushed before Drain
// returns and Stats counts every one of them (owner D2 of S3.2b-2's design; review C3). Nil: no recording.
func (q *recQ) drainStop() chan struct{} {
	if q == nil {
		return nil
	}
	if q.put(recItem{kind: rStop}) {
		if m := make(chan struct{}); q.put(recItem{kind: rMarker, marker: m}) {
			return m
		}
	}
	return q.settled
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

// cut is a drain's deadline passing while the recording still has writes to make — a live one its stop, an ended one
// the records queued before its end (owner, 2026-11-06, decision 1; 2026-11-11): the recording is marked incomplete,
// once. An ended recording keeps its cause, the cut appended; Records stays what was written at the cut. fail is the
// router's and the Recorder's, and keeps its first-wins meaning.
func (q *recQ) cut() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cutOff {
		return
	}
	q.cutOff = true
	if !q.failed {
		q.failed, q.err = true, "runtime: the recording's stop was not written before the drain's deadline: the recording is incomplete"
		return
	}
	q.err += "; its queued records were not written before the drain's deadline: the recording is incomplete"
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
	defer close(q.returned)
	defer q.settle()
	for {
		select {
		case it := <-q.q:
			q.handle(it)
		case <-q.stop: // Remove: every record queued before it is in q (the router put them first): write, flush, return
			for {
				select {
				case it := <-q.q:
					q.handle(it)
				default:
					q.flush()
					return
				}
			}
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
			q.settle() // the output is final: nothing is put after a full queue; a drain waiting for it goes on
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
	if err != nil {
		q.broken = true
		q.settle() // the output is final: the Recorder is never called again, nothing is flushed
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case err != nil:
		if !q.failed {
			q.failed, q.err = true, err.Error()
		}
	default:
		q.records++
	}
}

// settle closes settled once: the goroutine's, after an ended recording's last write and flush, or at its return.
func (q *recQ) settle() {
	if !q.isSettled {
		q.isSettled = true
		close(q.settled)
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
	case rBatchEnd:
		err = q.rec.BatchEnd(it.stream, it.batch)
	}
	return err
}
