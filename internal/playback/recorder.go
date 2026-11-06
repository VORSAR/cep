package playback

import (
	"errors"
	"fmt"
	"io"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// Recorder writes one app's recording as log lines (S3.2a-1's D3; owner Q1 and Q2 of S3.2b-2; design s3-2b-2 §7):
// the header, each event with its arrival, each rejected live event, every idle tick, table writes and the stop. One
// line per call, one Write per line, so a line is whole or absent; Stop and Flush flush a writer that has Flush. The
// runtime calls it from the app's recorder goroutine only.
type Recorder struct {
	w   io.Writer
	app *plan.App
	buf []byte
}

var _ runtime.Recorder = (*Recorder)(nil)

// errLineTooLong: a line the reader could not read back is a recording failure, never written (review F4).
var errLineTooLong = fmt.Errorf("playback: a recording line longer than %d bytes", event.MaxLogLine)

// NewRecorder is the recorder of app's recording into w.
func NewRecorder(w io.Writer, app *plan.App) *Recorder { return &Recorder{w: w, app: app} }

// stamp checks a time the recording writes: outside RFC 3339's years the line would not read back, so the recording
// fails instead (owner Q2 of S3.2b-2; break-it S2).
func stamp(ms int64) error {
	if !event.TimestampInRange(ms) {
		return errTimeRange
	}
	return nil
}

func (r *Recorder) line(b []byte) error {
	if len(b) > event.MaxLogLine {
		return errLineTooLong
	}
	b = append(b, '\n')
	_, err := r.w.Write(b)
	r.buf = b[:0]
	return err
}

// Header writes {"recording":{"start":…}}.
func (r *Recorder) Header(start int64) error {
	if err := stamp(start); err != nil {
		return err
	}
	b := append(r.buf[:0], `{"recording":{"start":`...)
	b = appendTime(b, start)
	return r.line(append(b, `}}`...))
}

// Event writes an event line with its arrival — none on a `time arrival` stream, whose ts is its arrival (S3.2a-1's
// D2). payload is SendPayload's payload as sent, cleaned (owner D3): written as the attrs, so replay decodes the same
// bytes; nil for a typed Send, whose attributes are written canonically without text_invisibles (review F3).
func (r *Recorder) Event(e *event.Event, arrival int64, payload []byte) error {
	if e.StreamID < 0 || e.StreamID >= len(r.app.Streams) {
		return fmt.Errorf("playback: recorder: stream %d of %d", e.StreamID, len(r.app.Streams))
	}
	s := r.app.Streams[e.StreamID].Schema
	if err := stamp(arrival); err != nil {
		return err
	}
	b := append(r.buf[:0], '{')
	if s.TimeAttr() != event.ArrivalTime {
		b = append(b, `"arrival":`...)
		b = appendTime(b, arrival)
		b = append(b, ',')
	}
	if payload == nil {
		ev, err := appendEvent(nil, s, *e, event.InvisiblesAttr)
		if err != nil {
			return err
		}
		return r.line(append(b, ev[1:]...))
	}
	b = append(b, `"attrs":`...)
	b = append(b, payload...)
	if e.BatchID != "" {
		b = append(b, `,"batch":`...)
		b = appendString(b, e.BatchID)
	}
	b = append(b, `,"stream":`...)
	b = appendString(b, s.Stream())
	if s.TimeAttr() == event.ArrivalTime {
		b = append(b, `,"ts":`...)
		b = appendTime(b, e.Ts)
	}
	return r.line(append(b, '}'))
}

// reasonNames are the rejected record's reasons, by event.DecodeReason.
var reasonNames = map[event.DecodeReason]string{
	event.DecodeSyntax: "syntax", event.DecodeType: "type", event.DecodeMissing: "missing", event.DecodeNull: "null",
}

// Rejected writes {"arrival":…,"rejected":{"attr":…,"reason":…},"stream":…} (owner Q1 of S3.2b-2): arrival on every
// stream, a rejected record being no event; attr absent for a syntax fault.
func (r *Recorder) Rejected(stream int, arrival int64, fault *event.DecodeError) error {
	if stream < 0 || stream >= len(r.app.Streams) {
		return fmt.Errorf("playback: recorder: stream %d of %d", stream, len(r.app.Streams))
	}
	if fault == nil {
		return errors.New("playback: recorder: a rejected event without its fault")
	}
	if err := stamp(arrival); err != nil {
		return err
	}
	reason, ok := reasonNames[fault.Reason]
	if !ok {
		return fmt.Errorf("playback: recorder: decode reason %d", fault.Reason)
	}
	b := append(r.buf[:0], `{"arrival":`...)
	b = appendTime(b, arrival)
	b = append(b, `,"rejected":{`...)
	if fault.Attr != "" && fault.Reason != event.DecodeSyntax {
		b = append(b, `"attr":`...)
		b = appendString(b, fault.Attr)
		b = append(b, ',')
	}
	b = append(b, `"reason":`...)
	b = appendString(b, reason)
	b = append(b, `},"stream":`...)
	b = appendString(b, r.app.Streams[stream].Schema.Stream())
	return r.line(append(b, '}'))
}

// Tick writes {"tick":…}.
func (r *Recorder) Tick(now int64) error {
	if err := stamp(now); err != nil {
		return err
	}
	b := append(r.buf[:0], `{"tick":`...)
	b = appendTime(b, now)
	return r.line(append(b, '}'))
}

// Table writes one table record per row: {"op":"put","row":{…},"table":…}; a delete's row holds the key columns.
func (r *Recorder) Table(table int, op runtime.TableOp, rows [][]event.Value) error {
	if table < 0 || table >= len(r.app.Tables) {
		return fmt.Errorf("playback: recorder: table %d of %d", table, len(r.app.Tables))
	}
	t := r.app.Tables[table]
	cols, opName := t.Attrs, "put"
	if op == runtime.TableDelete {
		opName, cols = "delete", nil
		for _, k := range t.Key {
			for _, a := range t.Attrs {
				if a.Name == k {
					cols = append(cols, a)
				}
			}
		}
	}
	for _, row := range rows {
		if len(row) != len(cols) {
			return fmt.Errorf("playback: recorder: table %q: a %s row of %d values for %d columns", t.Name, opName, len(row), len(cols))
		}
		b := append(r.buf[:0], `{"op":`...)
		b = appendString(b, opName)
		b = append(b, `,"row":{`...)
		for i, a := range cols {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendString(b, a.Name)
			b = append(b, ':')
			var err error
			if b, err = AppendValue(b, row[i]); err != nil {
				return fmt.Errorf("playback: recorder: table %q column %q: %w", t.Name, a.Name, err)
			}
		}
		b = append(b, `},"table":`...)
		b = appendString(b, t.Name)
		if err := r.line(append(b, '}')); err != nil {
			return err
		}
	}
	return nil
}

// Stop writes {"stop":true} and flushes.
func (r *Recorder) Stop() error {
	if err := r.line(append(r.buf[:0], `{"stop":true}`...)); err != nil {
		return err
	}
	return r.Flush()
}

// Flush flushes the writer if it has Flush: the runtime calls it at Close without a drain (design s3-2b-2 §7).
func (r *Recorder) Flush() error {
	if f, ok := r.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}
