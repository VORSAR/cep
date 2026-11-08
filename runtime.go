package cep

import (
	"context"

	"github.com/vorsar/cep/internal/runtime"
)

// Runtime is a running engine. New starts it; Deploy adds apps; Close stops every app in reverse deploy order.
type Runtime struct{ rt *runtime.Runtime }

// New starts a runtime with no apps.
func New(cfg Config) (*Runtime, error) {
	rt, err := runtime.New(cfg.engine())
	if err != nil {
		return nil, mapErr(err)
	}
	return &Runtime{rt: rt}, nil
}

// Close stops every app in reverse deploy order, then the engine. Past ctx's deadline it returns ctx's error with
// any goroutine that did not return still running. A second Close waits, until its own ctx's deadline, for the
// goroutines the first left running: nil once they have all returned, ctx's error otherwise.
func (r *Runtime) Close(ctx context.Context) error { return mapErr(r.rt.Close(ctx)) }
