package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// ---- callers ----

func (r *Runtime) isClosed() bool {
	select {
	case <-r.stopped:
		return true
	default:
		return false
	}
}

func (r *Runtime) appOf(app AppID) (*appState, error) {
	if r.isClosed() {
		return nil, ErrClosed
	}
	apps := r.appList()
	if app < 0 || int(app) >= len(apps) {
		return nil, fmt.Errorf("runtime: no app %d", app)
	}
	if apps[app] == nil || appPhase(apps[app].phase.Load()) == phaseRemoved {
		return nil, ErrRemoved
	}
	return apps[app], nil
}

// gate is every entry point's caller-side check (design s3-3-1 §2.5 MUST 3): the app's phase, then the drain flag.
func (r *Runtime) gate(app AppID) (*appState, error) {
	a, err := r.appOf(app)
	if err != nil {
		return nil, err
	}
	switch appPhase(a.phase.Load()) {
	case phaseInactive:
		return nil, ErrInactive
	case phaseDraining:
		return nil, ErrDrained // the phase changes before the sources are cancelled (design s3-3-1 §2.4, MUST 3)
	case phaseRemoved:
		return nil, ErrRemoved
	}
	if a.draining.Load() {
		return nil, ErrDrained
	}
	return a, nil
}

func (r *Runtime) enqueue(ctx context.Context, it item) error {
	if err := ctx.Err(); err != nil { // an expired context queues nothing: select would pick at random (QA Q15)
		return err
	}
	select {
	case r.intake <- it:
		return nil
	case <-r.stopped:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) request(ctx context.Context, it item) (reply, error) {
	it.reply = make(chan reply, 1)
	if err := r.enqueue(ctx, it); err != nil {
		return reply{}, err
	}
	return r.await(ctx, it.reply)
}

func (r *Runtime) await(ctx context.Context, ch chan reply) (reply, error) {
	select {
	case rep := <-ch:
		return rep, rep.err
	case <-r.stopped:
		return reply{}, ErrClosed
	case <-ctx.Done():
		return reply{}, ctx.Err()
	}
}

// awaitSignal waits for ch to close: ErrClosed once Close has stopped the runtime, ctx's error at its deadline.
func (r *Runtime) awaitSignal(ctx context.Context, ch <-chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-r.stopped:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// awaitMarkers waits, in order, for every marker to close, as awaitSignal waits for one.
func (r *Runtime) awaitMarkers(ctx context.Context, markers []chan struct{}) error {
	for _, m := range markers {
		if err := r.awaitSignal(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// Closed says Close has begun: the facade checks it before reading a program (§3.7: Deploy after Close is ErrClosed).
func (r *Runtime) Closed() bool { return r.isClosed() || r.inflight.closed() }

// Tick hands a wall tick to the intake, behind every event already there.
func (r *Runtime) Tick(ctx context.Context, now int64) error {
	if !event.TimestampInRange(now) {
		return fmt.Errorf(
			"runtime: Tick at %d ms: outside the years 0000-9999, which a recording cannot write (owner D1 of S3.2b-2's "+
				"design; break-it S2)", now)
	}
	if r.isClosed() {
		return ErrClosed
	}
	return r.enqueue(ctx, item{kind: iTick, now: now})
}

// TableWrite applies rows to one of app's tables in intake order (owner F14) and returns once the router has applied
// them: put rows are whole rows; delete rows are key values, in key order.
func (r *Runtime) TableWrite(ctx context.Context, app AppID, tableID int, op TableOp, rows [][]event.Value) error {
	a, err := r.gate(app)
	if err != nil {
		return err
	}
	if a.app.Tables == nil || tableID < 0 || tableID >= a.app.Tables.Len() || (op != TablePut && op != TableDelete) {
		return fmt.Errorf("runtime: app %d: no table %d, or an unknown operation", app, tableID)
	}
	tb := a.app.Tables.Table(tableID)
	if err := CheckRows(tb.Name(), tb.Attrs(), rows); err != nil {
		return fmt.Errorf("runtime: app %d table %d: %w", app, tableID, err)
	}
	_, err = r.request(ctx, item{kind: iWrite, app: app, table: tableID, op: op, rows: rows})
	return err
}

// tick is the internal ticker (Config.TickEvery): a wall tick into the intake, behind the events before it.
func (r *Runtime) tick() {
	defer r.wg.Done()
	t := time.NewTicker(r.cfg.TickEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			select {
			case r.intake <- item{kind: iTick, now: r.cfg.Now()}:
			case <-r.stopped:
				return
			}
		case <-r.stopped:
			return
		}
	}
}

// stopping says Close has begun: a loop checks it first, so nothing starts after stop (break-it C1).
func (r *Runtime) stopping() bool { return r.isClosed() }
