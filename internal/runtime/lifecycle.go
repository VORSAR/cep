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
	if r.isClosed() {
		return 0, ErrClosed
	}
	if err := validateApp(len(r.appList()), &app); err != nil {
		return 0, err
	}
	rep, err := r.request(ctx, item{kind: iAdd, add: &app})
	if err != nil {
		return 0, err
	}
	return rep.id, nil
}

// Activate starts the app's sink flushers and, with a Recorder, its recorder goroutine (the header, queued at Add,
// is its first record), then opens its input: Send, SendPayload, TableWrite and BatchEnd accept the app from here.
// Errors: ErrStarted (already active), ErrDrained (after Drain), ErrRemoved (after Remove).
func (r *Runtime) Activate(ctx context.Context, app AppID) error {
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	if !a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseActive)) {
		switch appPhase(a.phase.Load()) {
		case phaseActive:
			return ErrStarted
		case phaseDraining:
			return ErrDrained
		}
		return ErrRemoved
	}
	a.started.Store(true)
	for _, q := range a.all {
		r.wg.Add(1)
		go q.run(r)
	}
	if a.rec != nil {
		r.wg.Add(1)
		go a.rec.run(r)
	}
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
	if err != nil {
		return err
	}
	if stream < 0 || stream >= len(a.app.Streams) || a.app.Streams[stream].Derived {
		return fmt.Errorf("runtime: app %d: stream %d is not a source stream", app, stream)
	}
	return r.enqueue(ctx, item{kind: iBatchEnd, app: app, stream: stream, batch: batch})
}

// Drain runs the drain barrier for app (design §5; s3-3-1 §2.4): the app's input ends (owner F5); every output
// produced before the barrier has been through its sink when it returns. For an active app the ticks stop first
// (S3.1-1's D3), then the source context is cancelled and every Run goroutine awaited until ctx's deadline; a run
// that did not return makes Drain return the stuck count wrapped in ctx.Err() without the barrier (the input is
// refused from here; a repeated Drain runs the barrier without waiting again). A never-activated app is drained at
// once: nothing was delivered, nothing is recorded. A second Drain of a drained app returns the same result.
func (r *Runtime) Drain(ctx context.Context, app AppID) (DrainResult, error) {
	a, err := r.appOf(app)
	if err != nil {
		return DrainResult{}, err
	}
	if appPhase(a.phase.Load()) == phaseRemoved {
		return DrainResult{}, ErrRemoved
	}
	a.phase.CompareAndSwap(int32(phaseInactive), int32(phaseDraining))
	a.phase.CompareAndSwap(int32(phaseActive), int32(phaseDraining))
	var runs []error
	if a.started.Load() {
		runs, err = r.awaitRuns(ctx, a)
		if err != nil {
			return DrainResult{Runs: runs}, err
		}
	}
	set := !a.draining.Swap(true)
	it := item{kind: iDrain, app: app, reply: make(chan reply, 1)}
	if err := r.enqueue(ctx, it); err != nil {
		if set { // nothing was queued, and no earlier Drain set the flag: the app is not drained (QA Q15; break-it R1)
			a.draining.Store(false)
		}
		return DrainResult{Runs: runs}, err
	}
	rep, err := r.await(ctx, it.reply)
	if err != nil {
		return DrainResult{Runs: runs}, err
	}
	for _, m := range rep.markers {
		select {
		case <-m:
		case <-r.stopped:
			return DrainResult{Runs: runs}, ErrClosed
		case <-ctx.Done():
			return DrainResult{Runs: runs}, ctx.Err()
		}
	}
	if rep.recMarker != nil { // every alert is out: the recording is waited for only until the deadline (owner, 2026-11-06)
		select {
		case <-rep.recMarker:
		case <-r.stopped:
		case <-ctx.Done():
			a.rec.fail("runtime: the recording's stop was not written before the drain's deadline: the recording is incomplete")
		}
	}
	return DrainResult{Pending: rep.pending, Runs: runs}, nil
}

// awaitRuns quiets the ticks, cancels the source context and waits for every Run goroutine, once per app; a repeated
// Drain gets the collected returns at once.
func (r *Runtime) awaitRuns(ctx context.Context, a *appState) ([]error, error) {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	if !a.waited {
		if err := r.enqueue(ctx, item{kind: iQuiet, app: a.id}); err != nil {
			return nil, err
		}
		a.waited = true
		a.cancelSrc()
	}
	runs := make([]error, len(a.runs))
	stuck := 0
	for i, rs := range a.runs {
		select {
		case <-rs.done:
			if !errors.Is(rs.err, context.Canceled) {
				runs[i] = rs.err
			}
		case <-ctx.Done():
			stuck++
		}
	}
	if stuck > 0 {
		return runs, fmt.Errorf("runtime: app %d: %d source goroutine(s) did not return before the deadline: %w", a.id, stuck, ctx.Err())
	}
	return runs, nil
}

// Remove unregisters a drained or inactive app: the router drops its routes, every shard frees its state, each sink
// flusher and the recorder goroutine return after their last marker (the queues are never closed), the slot is nil.
// Removing an active app is an error; removing twice is ErrRemoved.
func (r *Runtime) Remove(ctx context.Context, app AppID) error {
	a, err := r.appOf(app)
	if err != nil {
		return err
	}
	switch appPhase(a.phase.Load()) {
	case phaseActive:
		return fmt.Errorf("runtime: app %d is active: drain it first", app)
	case phaseRemoved:
		return ErrRemoved
	}
	rep, err := r.request(ctx, item{kind: iRemove, app: app})
	if err != nil {
		return err
	}
	a.phase.Store(int32(phaseRemoved))
	a.cancelSrc()
	defer a.removed.Do(func() {
		if a.onRemoved != nil {
			a.onRemoved()
		}
	})
	for _, m := range rep.markers {
		select {
		case <-m:
		case <-r.stopped:
			return ErrClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if rep.recMarker != nil {
		select {
		case <-rep.recMarker:
		case <-r.stopped:
			return ErrClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Close drains every active app and removes every app in reverse id order, then stops the router, the shards and
// the ticker. Past ctx's deadline it signals every goroutine and returns ctx.Err(); a Write or a run that never
// returns keeps its goroutine. A second Close returns nil.
func (r *Runtime) Close(ctx context.Context) error {
	var first error
	r.closeOnce.Do(func() {
		apps := r.appList()
		for id := len(apps) - 1; id >= 0 && first == nil; id-- {
			a := apps[id]
			if a == nil {
				continue
			}
			if appPhase(a.phase.Load()) == phaseActive {
				if _, err := r.Drain(ctx, AppID(id)); err != nil && ctx.Err() != nil {
					first = err
					break
				}
			}
			if err := r.Remove(ctx, AppID(id)); err != nil && ctx.Err() != nil {
				first = err
			}
		}
		close(r.stopped)
		for _, a := range r.appList() {
			if a != nil {
				a.cancelSrc()
				a.removed.Do(func() {
					if a.onRemoved != nil {
						a.onRemoved()
					}
				})
			}
		}
	})
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return first
	case <-ctx.Done():
		if first != nil {
			return first
		}
		return ctx.Err()
	}
}

// ---- the router's side ----

// app is the router's view of a slot: nil for a removed app.
func (rt *router) app(id AppID) *appState {
	apps := rt.r.appList()
	if int(id) < len(apps) {
		return apps[id]
	}
	return nil
}

// add installs an app (design s3-3-1 §2.4): the state built with the router's stamp, published without routes, then
// mAdd to every shard under a token; take completes it.
func (rt *router) add(it item) {
	r := rt.r
	start := rt.now(r.cfg.Now())
	apps := r.appList()
	a := r.newAppState(AppID(len(apps)), *it.add, start)
	next := append(append([]*appState(nil), apps...), a)
	r.apps.Store(&next)
	rt.token++
	rt.installs[rt.token] = &installWait{app: a, reply: it.reply}
	for sid := range r.shards {
		rt.push(send{shard: sid, msg: message{kind: mAdd, app: a.id, token: rt.token, add: &a.app}})
	}
}

// remove unregisters an app: the drain must be complete; the routes are dropped at once, mRemove goes to every shard
// under a token, and take pushes the last markers once every shard acked.
func (rt *router) remove(it item) {
	r := rt.r
	a := rt.app(it.app)
	if a == nil {
		it.reply <- reply{err: ErrRemoved}
		return
	}
	if a.started.Load() && !(a.drained && a.drainRes != nil) {
		it.reply <- reply{err: fmt.Errorf("runtime: app %d: the drain is not complete: drain it again", it.app)}
		return
	}
	for sid := range a.byStream {
		a.byStream[sid] = nil
	}
	rt.token++
	rt.installs[rt.token] = &installWait{app: a, reply: it.reply, remove: true}
	for sid := range r.shards {
		rt.push(send{shard: sid, msg: message{kind: mRemove, app: a.id, token: rt.token}})
	}
}

// installAck is one shard's ack of an mAdd or mRemove; the last one completes the item.
func (rt *router) installAck(w *installWait, it outItem) {
	r := rt.r
	w.acks++
	if it.err != nil && w.err == nil {
		w.err = it.err
	}
	if w.acks < len(r.shards) {
		return
	}
	delete(rt.installs, it.token)
	a := w.app
	switch {
	case w.remove:
		res := reply{}
		if a.started.Load() {
			for _, q := range a.all {
				m := make(chan struct{})
				res.markers = append(res.markers, m)
				rt.push(send{shard: -1, sink: q, si: sinkItem{marker: m, last: true}})
			}
			if a.rec != nil {
				close(a.rec.stop)
				res.recMarker = a.rec.returned
			}
		}
		next := append([]*appState(nil), r.appList()...)
		next[a.id] = nil
		r.apps.Store(&next)
		if w.rollback {
			res.err = w.err
		}
		w.reply <- res
	case w.err != nil: // a shard failed: undo the others, keep the id
		a.phase.Store(int32(phaseRemoved))
		rt.token++
		rt.installs[rt.token] = &installWait{app: a, reply: w.reply, remove: true, rollback: true, err: w.err}
		for sid := range r.shards {
			rt.push(send{shard: sid, msg: message{kind: mRemove, app: a.id, token: rt.token}})
		}
	default:
		a.installRoutes() // the app is reachable from here
		w.reply <- reply{id: a.id}
	}
}
