package cep_test

// Design note docs/sprints/s3-3-2-design.md v2, §6: the expect stage sits before bind, so a refused deploy calls no
// connector factory; a cancelled deploy calls none either (ctx checked before bind, A9); a construct this release
// cannot run is refused with an error wrapping errors.ErrUnsupported (owner, 2026-11-09, decision 1). Prefix: ef.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
)

// efCounting counts every call and refuses each one: a deploy that reaches bind fails here, after counting.
type efCounting struct{ sources, sinks atomic.Int64 }

func (f *efCounting) Source(typ string, _ map[string]string) (cepio.Source, error) {
	f.sources.Add(1)
	return nil, errors.New("ef: the test factory builds nothing")
}
func (f *efCounting) Sink(typ string, _ map[string]string) (cepio.Sink, error) {
	f.sinks.Add(1)
	return nil, errors.New("ef: the test factory builds nothing")
}

// efAnnotated is example 07 with an @source on ConsentGrants and an @sink on ConsentAlerts.
func efAnnotated(t testing.TB) string {
	t.Helper()
	src := exPlant(t, fcExample(t, ex07), "stream ConsentGrants (", "@source(type = 'http')\nstream ConsentGrants (")
	return exPlant(t, src, "stream ConsentAlerts (", "@sink(type = 'http')\nstream ConsentAlerts (")
}

// A failing block refuses the deploy before bind: the factory is never called.
func TestRefusedDeployCallsNoFactory(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	bad := exPlant(t, efAnnotated(t), "then ConsentAlerts [ ]", "then ConsentAlerts [ {tenantId: 't1'} ]")
	f := &efCounting{}
	h, err := r.Deploy(ctx, "07", bad, cep.WithConnectors(f))
	exRefused(t, bad, h, err, exWant{2, "UnknownAppConsent", "", 1})
	if n, m := f.sources.Load(), f.sinks.Load(); n != 0 || m != 0 {
		t.Errorf("the factory built %d sources and %d sinks for a refused deploy; want none (the expect stage runs before bind)", n, m)
	}
}

// A deploy whose ctx has ended (after the checks, before bind) calls no factory and returns the ctx's error. The
// program's every block passes, so only the ctx can stop it.
func TestCancelledDeployCallsNoFactory(t *testing.T) {
	t.Parallel()
	r := fcNew(t, cep.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &efCounting{}
	h, err := r.Deploy(ctx, "07", efAnnotated(t), cep.WithConnectors(f))
	if h != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Deploy with a cancelled ctx: handle %v, %v; want no handle and context.Canceled", h != nil, err)
	}
	if n, m := f.sources.Load(), f.sinks.Load(); n != 0 || m != 0 {
		t.Errorf("the factory built %d sources and %d sinks for a cancelled deploy; want none (ctx before bind)", n, m)
	}
}

// Not built yet is public: example 01 (a pattern body) is refused with errors.ErrUnsupported; example 07 with a typo
// is a refusal of another kind.
func TestNotBuiltIsUnsupported(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	h, err := r.Deploy(ctx, "01", fcExample(t, "01-agent-kill-switch.cep"), cep.WithConnectors(&efOpen{}))
	if h != nil || !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("Deploy(01): handle %v, %v; want no handle and an error wrapping errors.ErrUnsupported", h != nil, err)
	}
	typo := exPlant(t, fcExample(t, ex07), "from ConsentGrants[", "from ConsentGrant[")
	h, err = r.Deploy(ctx, "07 with a typo", typo)
	if h != nil || err == nil || errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("Deploy(07 with a typo): handle %v, %v; want a refusal that is not errors.ErrUnsupported", h != nil, err)
	}
}

// efOpen builds an inert source and sink for any type, so 01's annotations never decide its refusal.
type efOpen struct{}

func (efOpen) Source(string, map[string]string) (cepio.Source, error) {
	return fcFuncSource(func(sctx context.Context, _ cepio.Emitter) error { <-sctx.Done(); return nil }), nil
}
func (efOpen) Sink(string, map[string]string) (cepio.Sink, error) {
	return &fcFuncSink{mode: cepio.Append, write: func(context.Context, cepio.OutEvent) error { return nil }}, nil
}
