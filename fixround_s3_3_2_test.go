package cep_test

// The fix round of S3.3-2 (the combined review's R-L2): once Close has begun, Deploy is refused at once with
// ErrClosed — no parse-to-bind pipeline, no connector factory called. Prefix: fr.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
)

func TestDeployDuringCloseCallsNoFactory(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r, err := cep.New(cep.Config{Now: fcNow})
	if err != nil {
		t.Fatalf("cep.New: %v", err)
	}
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	entered := make(chan struct{}, 1)
	sink := &fcFuncSink{mode: cepio.Append, write: func(wctx context.Context, e cepio.OutEvent) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			return nil
		case <-wctx.Done():
			return wctx.Err()
		}
	}}
	cancelled := make(chan struct{})
	src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
		if err := emit.Emit(sctx, "", fc09("a1", 0.9, "prod", fcAt(1))); err != nil {
			return err
		}
		<-sctx.Done()
		close(cancelled)
		return nil
	})
	h := fcDeploy(t, ctx, r, "held", fcExample(t, ex09), cep.WithSink("RiskAlerts", sink), cep.WithSource("agent-events", src))
	fcStart(t, ctx, h)
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the sink to hold the alert: %v", ctx.Err())
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close(ctx) }()
	select {
	case <-cancelled: // Close is in its walk, draining held
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for Close to cancel held's source: %v", ctx.Err())
	}
	f := &efCounting{}
	if _, err := r.Deploy(ctx, "late", efAnnotated(t), cep.WithConnectors(f)); !errors.Is(err, cep.ErrClosed) {
		t.Errorf("Deploy during Close: %v; want ErrClosed", err)
	}
	if n := f.sources.Load() + f.sinks.Load(); n != 0 {
		t.Errorf("Deploy during Close called the connector factory %d time(s); want none", n)
	}
	free()
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for Close: %v", ctx.Err())
	}
}

// The owner, 2026-11-10 (S3.3-2's break-it decision 1): an object field in a block takes a string holding its JSON
// text; the then-record compares the object.
func TestObjectFieldFlowsThroughABlock(t *testing.T) {
	t.Parallel()
	prog := func(then string) string {
		return "language cep/0.1;\n\n" +
			"stream S (k string, doc object, ts timestamp)\n  time ts watermark bounded 1 min;\n\n" +
			"stream O (k string, doc object, ts timestamp);\n\n" +
			"rule R\n  intent 'Pass every key with its document.'\n  from S\n" +
			"  expect given S [ {k: 'a', doc: '{ \"tool\": \"rm\", \"args\": [1, 2] }', ts: timestamp '2026-01-01T10:00:00Z'} ]\n" +
			"         then O [ {k: 'a', doc: '" + then + "'} ]\n" +
			"  select k, doc\n  insert into O;\n"
	}
	if _, err := exDeploy(t, prog(`{"tool":"rm","args":[1,2]}`)); err != nil {
		t.Errorf("the same object, written compact in the then-record: %v; want it deployed", err)
	}
	_, err := exDeploy(t, prog(`{"tool":"ls"}`))
	var de *cep.DeployError
	if !errors.As(err, &de) || len(de.Diagnostics) != 1 || de.Diagnostics[0].Code != "E-EXP-001" || !strings.Contains(de.Diagnostics[0].Message, "doc is ") {
		t.Errorf("another object in the then-record: %v; want one E-EXP-001 naming doc", err)
	}
}
