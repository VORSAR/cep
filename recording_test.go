package cep_test

// The recording (design s3-3-1 §3.1 WithRecording; S3.2b-2's D4; QA's T13).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vorsar/cep"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/parser"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/validate"
)

func fcEvents09() [][4]any {
	return [][4]any{
		{"a1", 0.9, "prod", fcAt(1)}, {"a2", 0.8, "prod", fcAt(1800)},
		{"a1", 0.9, "prod", fcAt(5401)}, {"a2", 0.8, "prod", fcAt(5401.001)},
	}
}

// TestRecordingIsOffUnlessAsked (D4): a plain deploy records nothing; with WithRecording the header is first and the
// stop record last.
func TestRecordingIsOffUnlessAsked(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src := fcExample(t, "09-params-and-quoting.cep")
	_, bind := fcSink("RiskAlerts")
	plain := fcDeploy(t, ctx, r, "plain", src, bind)
	var buf bytes.Buffer
	rec := fcDeploy(t, ctx, r, "rec", src, bind, cep.WithRecording(&buf))
	for _, h := range []*cep.AppHandle{plain, rec} {
		fcStart(t, ctx, h)
		in := fcInput(t, h, "agent-events")
		for _, e := range fcEvents09() {
			fcSend(t, ctx, in, fc09(e[0].(string), e[1].(float64), e[2].(string), e[3].(int64)))
		}
		fcStop(t, ctx, h)
	}
	if st := fcStats(t, ctx, plain); st.Recording.Records != 0 {
		t.Errorf("plain Recording %+v; want Records 0", st.Recording)
	}
	if st := fcStats(t, ctx, rec); st.Recording.Records != 6 || st.Recording.Failed {
		t.Errorf("recorded Recording %+v; want 6 records (header, 4 events, stop), not failed", st.Recording)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 6 || !strings.Contains(lines[0], `"recording"`) || !strings.Contains(lines[5], `"stop"`) {
		t.Errorf("recording:\n%s\nwant 6 lines, the header first and the stop last", buf.String())
	}
}

// TestRecordingReplaysToTheLiveOutputs (§8's equivalence through the facade): replaying the recording through
// playback gives the live sink's outputs, canonical bytes equal, in order.
func TestRecordingReplaysToTheLiveOutputs(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	src := fcExample(t, "09-params-and-quoting.cep")
	sink, bind := fcSink("RiskAlerts")
	var buf bytes.Buffer
	h := fcDeploy(t, ctx, r, "09", src, bind, cep.WithRecording(&buf))
	fcStart(t, ctx, h)
	in := fcInput(t, h, "agent-events")
	for _, e := range fcEvents09() {
		fcSend(t, ctx, in, fc09(e[0].(string), e[1].(float64), e[2].(string), e[3].(int64)))
	}
	fcStop(t, ctx, h)
	live := sink.Drain()
	if len(live) != 2 {
		t.Fatalf("%d live outputs; want 2 (example 09's second expect)", len(live))
	}
	res, err := parser.Parse("09", src)
	if err != nil {
		t.Fatal(err)
	}
	checked, diags := validate.Check(res.App, validate.Options{})
	for _, d := range diags {
		if !d.Lint {
			t.Fatalf("validate: %+v", d)
		}
	}
	p, err := plan.Build(checked)
	if err != nil {
		t.Fatal(err)
	}
	var replayed [][]byte
	out := func(e event.Event) {
		b, err := playback.AppendEvent(nil, p.Streams[e.StreamID].Schema, e)
		if err != nil {
			t.Errorf("AppendEvent: %v", err)
		}
		replayed = append(replayed, b)
	}
	rd, err := playback.NewReader(bytes.NewReader(buf.Bytes()), p)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	d, err := playback.NewPlanDriver(p, out)
	if err != nil {
		t.Fatalf("NewPlanDriver: %v", err)
	}
	if err := playback.Replay(rd, d, func(*playback.Step) error { return nil }); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(replayed) != len(live) {
		t.Fatalf("%d replayed outputs; want %d", len(replayed), len(live))
	}
	for i := range live {
		if string(replayed[i]) != string(live[i].Canonical) {
			t.Errorf("output %d: replay %s; live %s", i, replayed[i], live[i].Canonical)
		}
	}
}
