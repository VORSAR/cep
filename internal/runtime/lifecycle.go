package runtime

import (
	"context"
	"errors"
	"fmt"
)

// The per-app lifecycle (design s3-3-1 §2): a runtime that starts empty, apps added, activated, run, drained and
// removed while it runs; the router installs and uninstalls, each shard builds and frees its own state, and the
// caller-side phase gates every entry point.

type appPhase int32

const (
	phaseInactive appPhase = iota // after Add
	phaseActive                   // after Activate
	phaseDraining                 // Drain begun (a.drained is the router's view, set by the drain item)
	phaseRemoved                  // after Remove: the slot is nil
)

// New starts the router, the shards and, with TickEvery > 0, the ticker, with no apps. Close stops them.
func New(cfg Config) (*Runtime, error) {
	r, err := build(cfg, nil)
	if err != nil {
		return nil, err
	}
	rt := newRouter(r)
	rt.deliver = rt.send // a message crosses goroutines: its views are copied
	r.wg.Add(1)
	go r.route(rt)
	for _, sh := range r.shards {
		r.wg.Add(1)
		go sh.run(r)
	}
	if r.cfg.TickEvery > 0 {
		r.wg.Add(1)
		go r.tick()
	}
	return r, nil
}

// Add registers app, inactive, and returns its id (dense; never reused). The router installs it between two intake
// items; every shard builds its own state for it and acks; on any shard's error nothing is installed, the id is
// consumed (its slot stays nil) and the error is returned. Nothing accepts the app's input until Activate.
func (r *Runtime) Add(ctx context.Context, app App) (AppID, error) {
	if r.isClosed() || !r.inflight.enter() { // Close has begun: refused before it lists the apps (S3.3-1-2)
		return 0, ErrClosed
	}
	defer r.inflight.leave() // after the router's reply: the install is complete when Close sees the gate empty
	if err := validateApp(len(r.appList()), &app); err != nil {
		return 0, err
	}
	rep, err := r.requestQueued(ctx, item{kind: iAdd, add: &app})
	if err != nil {
		return 0, err
	}
	return rep.id, nil
}

// requestQueued is request for an item whose effect must reach its caller: ctx bounds the wait for the intake only;
// once the item is queued the router's answer is awaited whatever ctx does (review S2: an Add or Remove whose ctx
// ended after queuing would otherwise leave an app its caller cannot see). The router always answers; Close
// unblocks the wait.
func (r *Runtime) requestQueued(ctx context.Context, it item) (reply, error) {
	it.reply = make(chan reply, 1)
	if err := r.enqueue(ctx, it); err != nil {
		return reply{}, err
	}
	select {
	case rep := <-it.reply:
		return rep, rep.err
	case <-r.stopped:
		return reply{}, ErrClosed
	}
}

// Activate starts the app's sink flushers and, with a Recorder, its recorder goroutine (the header, queued at Add,
// is its first record), then opens its input: Send, SendPayload, TableWrite and BatchEnd accept the app from here,
// and the engine's ticks reach it (owner, 2026-11-08: an app's clock starts at Start). The router decides, in intake
// order, whether the app is activated — a drain or a removal queued first wins, and the router never writes a field
// a caller reads (review B1; break-it B2). Errors: ErrStarted (already active), ErrDrained (after Drain), ErrRemoved
// (after or during Remove).
func (r *Runtime) Activate(ctx context.Context, app AppID) error {
	if r.isClosed() || !r.inflight.enter() { // Close has begun (S3.3-1-2)
		return ErrClosed
	}
	defer r.inflight.leave() // after the tail below: Close never sees an activation half done (design s3-3-2 §9.1)
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if err := activationRefusal(appPhase(a.phase.Load())); err != nil {
		return err
	}
	if _, err := r.requestQueued(ctx, item{kind: iActivate, app: app}); err != nil {
		return err
	}
	if r.activateHold != nil { // the package's tests only
		r.activateHold()
	}
	// The router has activated the app (a.activated, a.started): its flushers and recorder run from here, whatever
	// a Drain begun meanwhile does to the phase — the router's drain of an activated app pushes markers they consume.
	// Starting them and Close's stop exclude each other (the combined review's R-S3): once Close has given up at its
	// deadline and stopped the runtime, nothing starts and the caller is told ErrClosed.
	return r.startAppGoroutines(a)
}

// activationRefusal is Activate's answer for an app in phase p that cannot be activated, nil for an inactive one.
func activationRefusal(p appPhase) error {
	switch p {
	case phaseActive:
		return ErrStarted
	case phaseDraining:
		return ErrDrained
	case phaseRemoved:
		return ErrRemoved
	}
	return nil
}

// startAppGoroutines starts an activated app's flushers and recorder and makes it active, all under inflight.mu,
// which Close's close(r.stopped) also takes: nothing starts after the stop (S1.1c-6 design §4 §2 item 11, the one
// Lock and deferred Unlock a helper holds for its caller). Its Unlock runs before Activate's deferred inflight.leave.
func (r *Runtime) startAppGoroutines(a *appState) error {
	r.inflight.mu.Lock()
	defer r.inflight.mu.Unlock()
	if r.isClosed() {
		return ErrClosed
	}
	for _, q := range a.all {
		r.wg.Add(1)
		go q.run(r)
	}
	if rec := a.rec; rec != nil {
		r.wg.Add(1)
		go rec.run(r)
	}
	a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive))
	return nil
}

// Run starts each run on its own goroutine with the app's source context, which Drain cancels first. Only an active
// app: ErrInactive, ErrDrained, ErrRemoved otherwise. A run's return value is reported by Drain.
func (r *Runtime) Run(ctx context.Context, app AppID, runs ...func(ctx context.Context) error) error {
	a, err := r.gate(app)
	if err != nil {
		return err
	}
	a.runMu.Lock()
	defer a.runMu.Unlock()
	r.inflight.mu.Lock() // as Activate's tail: no run starts once Close has stopped the runtime (R-S3)
	defer r.inflight.mu.Unlock()
	if r.isClosed() {
		return ErrClosed
	}
	for _, run := range runs {
		rs := &runState{done: make(chan struct{})}
		a.runs = append(a.runs, rs)
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			defer close(rs.done)
			rs.err = run(a.srcCtx)
		}()
	}
	return nil
}

// BatchEnd hands a source's batch-close signal for stream to the intake, behind the events before it (owner,
// 2026-11-07, design 6). An empty batch id is an error (the recording's reader refuses one). After the drain it is
// refused with ErrDrained and counted in AfterDrain, as an event is.
func (r *Runtime) BatchEnd(ctx context.Context, app AppID, stream int, batch string) error {
	if batch == "" {
		return fmt.Errorf("runtime: app %d: an empty batch id cannot end a batch", app)
	}
	a, err := r.gate(app)
	if errors.Is(err, ErrDrained) { // refused, and counted in AfterDrain by the router, as a late event is
		if a, err2 := r.appOf(app); err2 == nil && stream >= 0 && stream < len(a.app.Streams) {
			_ = r.enqueue(ctx, item{kind: iBatchEnd, app: app, stream: stream, batch: batch})
		}
		return err
	}
	if err != nil {
		return err
	}
	if stream < 0 || stream >= len(a.app.Streams) || a.app.Streams[stream].Derived {
		return fmt.Errorf("runtime: app %d: stream %d is not a source stream", app, stream)
	}
	return r.enqueue(ctx, item{kind: iBatchEnd, app: app, stream: stream, batch: batch})
}
