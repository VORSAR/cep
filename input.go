package cep

import (
	"context"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
	cepio "github.com/vorsar/cep/io"
)

// InputHandler feeds one source stream. It holds only ids and the schema, so it is safe for concurrent use.
type InputHandler struct {
	rt     *runtime.Runtime
	app    runtime.AppID
	stream int
	sch    event.Schema
	names  []string
}

// SendRow allocates the row, calls fill, checks it (design s3-3-1 §4.3) and hands it to the engine, which keeps it
// (MUST 14). It blocks for backpressure and respects ctx. batchID names the source batch the row belongs to; "" for
// none.
func (h *InputHandler) SendRow(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error {
	w := newStreamRow(h.sch, h.names)
	if err := w.run(fill); err != nil {
		return err
	}
	e := event.Event{StreamID: h.stream, Values: w.values, BatchID: batchID}
	if ta := h.sch.TimeAttr(); ta != event.ArrivalTime {
		e.Ts = w.values[ta].T
	}
	return mapErr(h.rt.Send(ctx, h.app, e))
}

// EndBatch is the batch-close signal (design s3-3-1 §5.3): recorded and counted; batchID must not be empty.
func (h *InputHandler) EndBatch(ctx context.Context, batchID string) error {
	return mapErr(h.rt.BatchEnd(ctx, h.app, h.stream, batchID))
}

// emitter is a source's Emitter over an InputHandler.
type emitter struct{ h *InputHandler }

func (e *emitter) Emit(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error {
	return e.h.SendRow(ctx, batchID, fill)
}

func (e *emitter) EndBatch(ctx context.Context, batchID string) error {
	return e.h.EndBatch(ctx, batchID)
}

// names is a schema's attribute names, for AttrIndex (built once per handle).
func names(sch event.Schema) []string {
	out := make([]string, sch.NumAttrs())
	for i := range out {
		out[i] = sch.Attr(i).Name
	}
	return out
}
