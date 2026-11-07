package cep

import (
	"context"

	cepio "github.com/vorsar/cep/io"
)

// InputHandler feeds one source stream. It is safe for concurrent use.
type InputHandler struct{ _ struct{} }

// SendRow allocates the row, calls fill, checks it and hands it to the engine, which keeps it. It blocks for
// backpressure and respects ctx. batchID names the source batch the row belongs to; "" for none.
func (h *InputHandler) SendRow(ctx context.Context, batchID string, fill func(cepio.RowWriter) error) error {
	return errNotBuilt
}

// EndBatch is the batch-close signal: recorded and counted; batchID must not be empty.
func (h *InputHandler) EndBatch(ctx context.Context, batchID string) error { return errNotBuilt }
