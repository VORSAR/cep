package playback

import (
	"errors"
	"io"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/runtime"
)

// Recorder writes one app's recording as log lines (S3.2a-1's D3; owner Q1 and Q2 of S3.2b-2; design s3-2b-2 §7):
// the header, each event with its arrival, each rejected live event, every idle tick, table writes and the stop. One
// line per call, one Write per line; Stop flushes a writer that has Flush. Stub: not built yet; every call fails.
type Recorder struct{}

var _ runtime.Recorder = (*Recorder)(nil)

// errRecorderNotBuilt is the stub's answer to every call.
var errRecorderNotBuilt = errors.New("playback: S3.2b-2: the recorder is not built yet")

// NewRecorder is the recorder of app's recording into w.
func NewRecorder(w io.Writer, app *plan.App) *Recorder { return &Recorder{} }

// Header writes {"recording":{"start":…}}.
func (r *Recorder) Header(start int64) error { return errRecorderNotBuilt }

// Event writes an event line with its arrival; payload is SendPayload's bytes (written cleaned as attrs), nil for Send.
func (r *Recorder) Event(e *event.Event, arrival int64, payload []byte) error {
	return errRecorderNotBuilt
}

// Rejected writes a rejected line (owner Q1 of S3.2b-2).
func (r *Recorder) Rejected(stream int, arrival int64, fault *event.DecodeError) error {
	return errRecorderNotBuilt
}

// Tick writes {"tick":…}.
func (r *Recorder) Tick(now int64) error { return errRecorderNotBuilt }

// Table writes a table write's records.
func (r *Recorder) Table(table int, op runtime.TableOp, rows [][]event.Value) error {
	return errRecorderNotBuilt
}

// Stop writes {"stop":true} and flushes.
func (r *Recorder) Stop() error { return errRecorderNotBuilt }
