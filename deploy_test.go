package cep_test

// Deploy's pipeline, options and binding (design s3-3-1 §3.2–§3.4; QA's T11, T16–T20; §9's additions).

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/vorsar/cep"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/validate"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// fcTwoDefects: an insert into a source stream (E-GRAPH-002) and an unknown attribute (E-NAME-001).
const fcTwoDefects = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, ts timestamp);

rule R
  intent 'Two defects.'
  from S[nope == 'x']
  select k
  insert into S;
`

// TestDeployReportsEveryDiagnosticAtOnce (T11): every static defect in validate's order, field by field; a parse
// error is one diagnostic with the parser's code; a lint deploys and sits on the handle.
func TestDeployReportsEveryDiagnosticAtOnce(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	res, err := parser.Parse("two", fcTwoDefects)
	if err != nil {
		t.Fatalf("the program must parse: %v", err)
	}
	_, want := validate.Check(res.App, validate.Options{})
	var nonLint []validate.Diagnostic
	for _, d := range want {
		if !d.Lint {
			nonLint = append(nonLint, d)
		}
	}
	if len(nonLint) < 2 {
		t.Fatalf("the program must carry two defects; validate found %+v", want)
	}
	_, err = r.Deploy(ctx, "two", fcTwoDefects)
	var de *cep.DeployError
	if !errors.As(err, &de) {
		t.Fatalf("Deploy: %v; want *DeployError", err)
	}
	if de.Name != "two" || len(de.Diagnostics) != len(nonLint) {
		t.Fatalf("DeployError %+v; want %d diagnostics for %q", de, len(nonLint), "two")
	}
	for i, d := range de.Diagnostics {
		w := nonLint[i]
		if d.Code != w.Code || d.Line != w.Line || d.Col != w.Col || d.Message != w.Msg || d.Fix != w.Fix || d.Lint {
			t.Errorf("diagnostic %d: %+v; want %+v", i, d, w)
		}
	}
	if !strings.Contains(de.Error(), "2 finding(s)") && !strings.Contains(de.Error(), "finding(s)") {
		t.Errorf("Error() %q; want the count", de.Error())
	}
	_, perr := parser.Parse("bad", "language cep/0.1;\nstream (")
	var pe *parser.Error
	if !errors.As(perr, &pe) {
		t.Fatalf("the parser's error is %T; want *parser.Error", perr)
	}
	_, err = r.Deploy(ctx, "bad", "language cep/0.1;\nstream (")
	if !errors.As(err, &de) || len(de.Diagnostics) != 1 || de.Diagnostics[0].Code != pe.Code || de.Diagnostics[0].Line != pe.Line {
		t.Errorf("a parse error: %v; want one diagnostic %s at %d:%d", err, pe.Code, pe.Line, pe.Col)
	}
	lint, err := os.ReadFile("internal/validate/testdata/e-null-002-not-equal-nullable.cep")
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Deploy(ctx, "lint", string(lint))
	if err != nil {
		t.Fatalf("a lint-only program: %v; want a handle", err)
	}
	if l := h.Lints(); len(l) != 1 || l[0].Code != "E-NULL-002" || !l[0].Lint {
		t.Errorf("Lints() %+v; want one E-NULL-002 with Lint true", l)
	}
	if h.Name() != "lint" {
		t.Errorf("Name() %q; want lint", h.Name())
	}
}

const fcIntParamSrc = `language cep/0.1;

param n int = 0;

stream S (k string, n int, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, ts timestamp);

rule R
  intent 'Above the parameter.'
  from S[n > ${n}]
  select k
  insert into O;
`

// TestWithParamIsTheLanguagesLiteral (T16): an override is parsed as the language parses a default; a non-literal
// is E-PARAM-001 at 0:0; the smallest int parses; an out-of-range float does not.
func TestWithParamIsTheLanguagesLiteral(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("RiskAlerts")
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), bind,
		cep.WithParam("threshold", "0.5"), cep.WithParam("label", "'prod'"))
	fcStart(t, ctx, h)
	fcSend(t, ctx, fcInput(t, h, "agent-events"), fc09("a1", 0.6, "prod", fcAt(1)))
	fcStop(t, ctx, h)
	if n := len(sink.Drain()); n != 1 {
		t.Errorf("%d outputs with threshold 0.5 and risk 0.6; want 1", n)
	}
	var de *cep.DeployError
	_, err := r.Deploy(ctx, "09", fcExample(t, "09-params-and-quoting.cep"), bind, cep.WithParam("threshold", "abc"))
	if !errors.As(err, &de) || len(de.Diagnostics) != 1 || de.Diagnostics[0].Code != "E-PARAM-001" || de.Diagnostics[0].Line != 0 || de.Diagnostics[0].Col != 0 {
		t.Errorf("WithParam(threshold, abc): %v; want one E-PARAM-001 at 0:0", err)
	}
	if _, err := r.Deploy(ctx, "09", fcExample(t, "09-params-and-quoting.cep"), bind, cep.WithParam("threshold", "1e400")); !errors.As(err, &de) {
		t.Errorf("WithParam(threshold, 1e400): %v; want a DeployError (the parser's lowering, not strconv)", err)
	}
	if _, err := r.Deploy(ctx, "n", fcIntParamSrc, cep.WithSink("O", &inmemory.Sink{}), cep.WithParam("n", "-9223372036854775808")); err != nil {
		t.Errorf("WithParam(n, the smallest int): %v; want a handle", err)
	}
}

// TestStatsAreKeyedByName (T17): Stats' maps are keyed by the program's names.
func TestStatsAreKeyedByName(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("RiskAlerts")
	h := fcDeploy(t, ctx, r, "09", fcExample(t, "09-params-and-quoting.cep"), bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "agent-events")
	fcSend(t, ctx, in, fc09("a1", 0.9, "prod", fcAt(1)))
	fcSend(t, ctx, in, fc09("a1", 0.9, "prod", fcAt(2)))
	fcStop(t, ctx, h)
	st := fcStats(t, ctx, h)
	if st.Rules["HighRisk"].Suppressed != 1 {
		t.Errorf("Rules[HighRisk] %+v; want Suppressed 1", st.Rules["HighRisk"])
	}
	if _, ok := st.Streams["agent-events"]; !ok {
		t.Errorf("Streams %v; want a row for agent-events", st.Streams)
	}
	if _, ok := st.Streams["0"]; ok {
		t.Error("Streams has a row keyed \"0\"; want names only")
	}
	if s := st.Sinks["RiskAlerts"]; len(s) != 1 {
		t.Errorf("Sinks[RiskAlerts] %v; want one binding", s)
	}
}

// TestSourceAndInputOnOneStream (T18): a bound source and Input feed one stream.
func TestSourceAndInputOnOneStream(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	src := inmemory.NewSource(0)
	h := fcDeploy(t, ctx, r, "fwd", fcFwdSrc, bind, cep.WithSource("S", src))
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	for i := 1; i <= 3; i++ {
		if err := src.Push(ctx, "", fcRow("src", int64(i), fcAt(float64(i)))); err != nil {
			t.Fatalf("Push: %v", err)
		}
		fcSend(t, ctx, in, fcRow("in", int64(i), fcAt(float64(i))))
	}
	src.Close()
	fcStop(t, ctx, h)
	if n := len(sink.Drain()); n != 6 {
		t.Errorf("%d outputs; want 6", n)
	}
}

// TestWithSinkRefusesAChangelogOnlySink (T19): a sink answering Changelog on an append stream is ErrSinkMode.
func TestWithSinkRefusesAChangelogOnlySink(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	cl := &fcFuncSink{mode: cepio.Changelog, write: func(context.Context, cepio.OutEvent) error { return nil }}
	_, err := r.Deploy(ctx, "09", fcExample(t, "09-params-and-quoting.cep"), cep.WithSink("RiskAlerts", cl))
	if !errors.Is(err, cep.ErrSinkMode) || !strings.Contains(err.Error(), "RiskAlerts") {
		t.Errorf("a Changelog sink on RiskAlerts: %v; want ErrSinkMode naming the stream", err)
	}
}

// TestUnknownNames (T19): unknown stream and table names, and the wrong stream kind, each its sentinel.
func TestUnknownNames(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	_, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "tbl", fcTableSrc, bind)
	if _, err := h.Input("nope"); !errors.Is(err, cep.ErrUnknownStream) {
		t.Errorf("Input(nope): %v; want ErrUnknownStream", err)
	}
	if _, err := h.Input("O"); !errors.Is(err, cep.ErrNotASourceStream) {
		t.Errorf("Input(O): %v; want ErrNotASourceStream", err)
	}
	if _, err := h.Table("nope"); !errors.Is(err, cep.ErrUnknownTable) {
		t.Errorf("Table(nope): %v; want ErrUnknownTable", err)
	}
	if _, err := r.Deploy(ctx, "tbl", fcTableSrc, cep.WithSink("S", &inmemory.Sink{})); !errors.Is(err, cep.ErrNotADerivedStream) {
		t.Errorf("WithSink on a source stream: %v; want ErrNotADerivedStream", err)
	}
	if _, err := r.Deploy(ctx, "tbl", fcTableSrc, cep.WithSource("O", inmemory.NewSource(0))); !errors.Is(err, cep.ErrNotASourceStream) {
		t.Errorf("WithSource on a derived stream: %v; want ErrNotASourceStream", err)
	}
	if _, err := r.Deploy(ctx, "tbl", fcTableSrc, cep.WithSink("nope", &inmemory.Sink{})); !errors.Is(err, cep.ErrUnknownStream) {
		t.Errorf("WithSink on an unknown stream: %v; want ErrUnknownStream", err)
	}
	if _, err := r.Deploy(ctx, "tbl", fcTableSrc, cep.WithTableRows("nope")); !errors.Is(err, cep.ErrUnknownTable) {
		t.Errorf("WithTableRows on an unknown table: %v; want ErrUnknownTable", err)
	}
}

// TestUnbuiltConnectorAnnotation (T20, as §9 amends it; owner's design 5): example 01's @sink(type = 'http') is
// refused naming http and S5.1 without a Go sink, and deploys with WithSink on that stream.
func TestUnbuiltConnectorAnnotation(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src := fcExample(t, "01-agent-kill-switch.cep")
	_, err := r.Deploy(ctx, "01", src)
	if !errors.Is(err, cep.ErrConnectorNotBuilt) || !strings.Contains(err.Error(), "http") || !strings.Contains(err.Error(), "S5.1") {
		t.Errorf("example 01 without a Go sink: %v; want ErrConnectorNotBuilt naming http and S5.1", err)
	}
	if _, err := r.Deploy(ctx, "01", src, cep.WithSink("AgentKillSwitch", &inmemory.Sink{})); err != nil {
		t.Errorf("example 01 with WithSink(AgentKillSwitch): %v; want a handle (owner, design 5)", err)
	}
}

// TestRepeatedOptionIsAnError (§9, T20's addition): WithSource twice on one stream, WithParam twice on one name,
// WithRecording twice → ErrRepeatedOption, a plain error naming the option.
func TestRepeatedOptionIsAnError(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src09 := fcExample(t, "09-params-and-quoting.cep")
	_, bind := fcSink("RiskAlerts")
	var de *cep.DeployError
	for name, opts := range map[string][]cep.DeployOption{
		"WithSource":    {cep.WithSource("agent-events", inmemory.NewSource(0)), cep.WithSource("agent-events", inmemory.NewSource(0))},
		"WithParam":     {cep.WithParam("threshold", "0.5"), cep.WithParam("threshold", "0.6")},
		"WithRecording": {cep.WithRecording(&strings.Builder{}), cep.WithRecording(&strings.Builder{})},
	} {
		_, err := r.Deploy(ctx, "09", src09, append(opts, bind)...)
		if !errors.Is(err, cep.ErrRepeatedOption) || !strings.Contains(err.Error(), name) || errors.As(err, &de) {
			t.Errorf("%s twice: %v; want ErrRepeatedOption naming %s, not a DeployError", name, err, name)
		}
	}
	if _, err := r.Deploy(ctx, "09", src09, bind, cep.WithSink("RiskAlerts", &inmemory.Sink{})); err != nil {
		t.Errorf("WithSink twice: %v; want allowed", err)
	}
}

// TestWithTableRowsAreVisibleToTheFirstEvent (§9 new; owner's design 2): the starting rows are written at Start
// before any source opens, so a source emitting at once sees them.
func TestWithTableRowsAreVisibleToTheFirstEvent(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
		for i, k := range []string{"a", "b"} {
			if err := emit.Emit(sctx, "", fcKey(k, fcAt(float64(i+1)))); err != nil {
				return err
			}
		}
		<-sctx.Done()
		return nil
	})
	row := func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("k"), "a")
		w.SetInt(w.AttrIndex("v"), 1)
		return nil
	}
	h := fcDeploy(t, ctx, r, "tbl", fcTableSrc, bind, cep.WithSource("S", src), cep.WithTableRows("T", row))
	fcStart(t, ctx, h)
	fcStop(t, ctx, h)
	recs := sink.Drain()
	if len(recs) != 1 || recs[0].Values[0] != "b" {
		t.Errorf("outputs %+v; want exactly b (a was in T before the first event)", recs)
	}
	bad := func(w cepio.RowWriter) error { w.SetInt(w.AttrIndex("v"), 1); return nil }
	if _, err := r.Deploy(ctx, "tbl", fcTableSrc, bind, cep.WithTableRows("T", bad)); err == nil || !strings.Contains(err.Error(), "k") {
		t.Errorf("WithTableRows with the key unset: %v; want a deploy error naming k", err)
	}
}
