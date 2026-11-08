package cep_test

// S3.3-1's build: the lines QA's tests-first lane left uncovered in package cep (the coverage row's 90% floor;
// design v2 §9: "the build adds a test only where QA's leave a line uncovered"). Prefix: cv.

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

const cvTypesSrc = `language cep/0.1;

stream S (b bool, d duration, o object optional, ts timestamp)
  time ts watermark bounded 1 min;

stream O (b bool, d duration, o object optional, ts timestamp);

rule Fwd
  intent 'Forward every row.'
  from S
  select b, d, o
  insert into O;
`

// TestCvBuildInfoAndErrors: BuildInfo names the module and the Go version; DeployError prints one line per finding.
func TestCvBuildInfoAndErrors(t *testing.T) {
	t.Parallel()
	if bi := cep.BuildInfo(); bi.GoVersion == "" || bi.Module == "" {
		t.Errorf("BuildInfo %+v; want the module and the Go version", bi)
	}
	e := &cep.DeployError{Name: "x", Diagnostics: []cep.Diagnostic{{Code: "E-TIME-001", Line: 2, Col: 3, Message: "m"}}}
	if s := e.Error(); !strings.Contains(s, "deploy x: 1 finding(s)") || !strings.Contains(s, "E-TIME-001 2:3 m") {
		t.Errorf("Error() %q", s)
	}
}

// TestCvOptionsRefuseNil: each option refuses a nil value as a plain error.
func TestCvOptionsRefuseNil(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	for name, opt := range map[string]cep.DeployOption{
		"source": cep.WithSource("S", nil), "sink": cep.WithSink("O", nil), "recording": cep.WithRecording(nil), "connectors": cep.WithConnectors(nil),
	} {
		if _, err := r.Deploy(ctx, "nil", cvTypesSrc, opt); err == nil || !strings.Contains(err.Error(), "nil") {
			t.Errorf("With%s(nil): %v; want an error naming nil", name, err)
		}
	}
	if _, err := r.Deploy(ctx, "twice", cvTypesSrc, cep.WithConnectors(cvFactory{}), cep.WithConnectors(cvFactory{})); !errors.Is(err, cep.ErrRepeatedOption) {
		t.Errorf("WithConnectors twice: %v; want ErrRepeatedOption", err)
	}
}

// cvFactory binds every annotation to an inmemory connector; cvRefusing refuses.
type cvFactory struct{ src *inmemory.Source }

func (f cvFactory) Source(typ string, keys map[string]string) (cepio.Source, error) {
	if f.src == nil {
		return inmemory.NewSource(0), nil
	}
	return f.src, nil
}
func (cvFactory) Sink(typ string, keys map[string]string) (cepio.Sink, error) {
	return &inmemory.Sink{}, nil
}

// TestCvConnectorFactory: the program's @source and @sink go through WithConnectors' factory (the keys as the rule
// language writes them); the default factory names the sprint of every type, and inmemory's annotation form is refused.
func TestCvConnectorFactory(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src := "language cep/0.1;\n" +
		"@source(type = 'stdin')\n" +
		"stream S (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
		"@sink(type = 'http', url = 'http://x', @map(type = 'json'))\n" +
		"stream O (k string, ts timestamp);\n" +
		"rule F intent 'Forward.' from S select k insert into O;\n"
	if _, err := r.Deploy(ctx, "ann", src); !errors.Is(err, cep.ErrConnectorNotBuilt) || !strings.Contains(err.Error(), "S5.1") {
		t.Errorf("default factory: %v; want ErrConnectorNotBuilt naming S5.1", err)
	}
	h, err := r.Deploy(ctx, "ann", src, cep.WithConnectors(cvFactory{}))
	if err != nil {
		t.Fatalf("WithConnectors: %v", err)
	}
	if err := h.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := h.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	later := strings.ReplaceAll(src, "'stdin'", "'inmemory'")
	if _, err := r.Deploy(ctx, "mem", later); !errors.Is(err, cep.ErrConnectorNotBuilt) || !strings.Contains(err.Error(), "WithSource") {
		t.Errorf("an inmemory annotation: %v; want ErrConnectorNotBuilt pointing at WithSource", err)
	}
	unknown := strings.ReplaceAll(src, "'stdin'", "'kafka'")
	var de *cep.DeployError
	if _, err := r.Deploy(ctx, "unk", unknown); !errors.As(err, &de) || de.Diagnostics[0].Code != "E-NAME-001" {
		t.Errorf("an unknown connector type: %v; want E-NAME-001 from the validator's closed table (ADR-0008 §1)", err)
	}
}

// TestCvEveryTypeCrossesTheBoundary: bool, duration and object values cross the writer and the reader; AttrType on
// both sides; EndBatch through the handler; Undeploy of a never-started app.
func TestCvEveryTypeCrossesTheBoundary(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	sink, bind := fcSink("O")
	h := fcDeploy(t, ctx, r, "types", cvTypesSrc, bind)
	fcStart(t, ctx, h)
	in := fcInput(t, h, "S")
	err := in.SendRow(ctx, "b1", func(w cepio.RowWriter) error {
		if w.AttrType(w.AttrIndex("b")) != cepio.Bool || w.AttrType(w.AttrIndex("d")) != cepio.Duration || w.AttrType(99) != cepio.Invalid {
			t.Error("RowWriter.AttrType")
		}
		w.SetBool(w.AttrIndex("b"), true)
		w.SetDuration(w.AttrIndex("d"), 1500)
		w.SetObject(w.AttrIndex("o"), `{"a":1}`)
		w.SetTimestamp(w.AttrIndex("ts"), fcAt(1))
		return nil
	})
	if err != nil {
		t.Fatalf("SendRow: %v", err)
	}
	if err := in.EndBatch(ctx, "b1"); err != nil {
		t.Fatalf("EndBatch: %v", err)
	}
	fcStop(t, ctx, h)
	recs := sink.Drain()
	if len(recs) != 1 || recs[0].Values[0] != true || recs[0].Values[1] != int64(1500) || recs[0].Values[2] != `{"a":1}` {
		t.Fatalf("records %+v; want b true, d 1500, o {\"a\":1}", recs)
	}
	st, err := h.Stats(ctx)
	if err != nil || st.Streams["S"].Batches != 1 {
		t.Errorf("Stats %+v, %v; want Batches 1", st, err)
	}
	h2 := fcDeploy(t, ctx, r, "types2", cvTypesSrc, bind)
	if err := h2.Undeploy(ctx); err != nil {
		t.Errorf("Undeploy of a never-started app: %v; want nil", err)
	}
}
