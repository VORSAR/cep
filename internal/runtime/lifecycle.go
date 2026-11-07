package runtime

import (
	"context"
	"errors"
)

// The per-app lifecycle (design s3-3-1 §2): a runtime that starts empty, apps added, activated, run, drained and
// removed while it runs. Stubs until the build's step 2: each returns errNotBuilt so QA's tests compile and fail.

var errNotBuilt = errors.New("runtime: not built yet (S3.3-1)")

// New starts the router, the shards and, with TickEvery > 0, the ticker, with no apps. Close stops them.
func New(cfg Config) (*Runtime, error) { return nil, errNotBuilt }

// Add registers app, inactive, and returns its id (dense; never reused). The router installs it between two intake
// items; every shard builds its own state for it and acks; on any shard's error nothing is installed, the id is
// consumed (its slot stays nil) and the error is returned. Nothing accepts the app's input until Activate.
func (r *Runtime) Add(ctx context.Context, app App) (AppID, error) { return 0, errNotBuilt }

// Activate starts the app's sink flushers and, with a Recorder, its recorder goroutine (the header, queued at Add, is
// its first record), then opens its input. Errors: ErrStarted, ErrDrained, ErrRemoved.
func (r *Runtime) Activate(ctx context.Context, app AppID) error { return errNotBuilt }

// Run starts each run on its own goroutine with the app's source context, which Drain cancels first. Only an active
// app: ErrInactive, ErrDrained, ErrRemoved otherwise. A run's return value is reported by Drain.
func (r *Runtime) Run(ctx context.Context, app AppID, runs ...func(ctx context.Context) error) error {
	return errNotBuilt
}

// BatchEnd hands a source's batch-close signal for stream to the intake, behind the events before it (owner,
// 2026-11-07, design 6). An empty batch id is an error. After the drain it is refused with ErrDrained and counted in
// AfterDrain, as an event is.
func (r *Runtime) BatchEnd(ctx context.Context, app AppID, stream int, batch string) error {
	return errNotBuilt
}

// Remove unregisters a drained or inactive app: the router drops its routes, every shard frees its state, each sink
// flusher and the recorder goroutine return after their last marker, the slot is nil. Removing an active app is an
// error; removing twice is ErrRemoved.
func (r *Runtime) Remove(ctx context.Context, app AppID) error { return errNotBuilt }
