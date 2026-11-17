package runtime

// build and validateApp: one case per check and per operand of each compound condition, and the first fault named
// when two are present at each adjacent pair of checks. The fixture is cgApp, changed per case. Prefix: bv.

import (
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// bvKeyed makes cgApp's rule keyed by k (attribute 0, a string) with a TTL of 60 000 ms: a valid keyed rule.
func bvKeyed(a *App) {
	a.Rules[0].KeyAttrs, a.Rules[0].TTL = [][]int{{0}}, 60_000
}

// bvSecond appends a copy of cgApp's rule, named "second".
func bvSecond(a *App) {
	r := a.Rules[0]
	r.Name = "second"
	a.Rules = append(a.Rules, r)
}

// bvRun checks validateApp's verdict on cgApp after edits: nil when want is "", else an error holding want.
func bvRun(t *testing.T, want string, edits ...func(a *App)) {
	t.Helper()
	app := cgApp(t, func(event.Event) error { return nil })
	for _, e := range edits {
		e(&app)
	}
	err := validateApp(3, &app)
	switch {
	case want == "" && err != nil:
		t.Fatalf("validateApp: %v; want nil", err)
	case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
		t.Fatalf("validateApp: %v; want an error holding %q", err, want)
	}
}

func bvSchema(t *testing.T, name string) event.Schema {
	t.Helper()
	s, err := event.NewSchema(name, []event.Attr{{Name: "k", Type: event.TString}, {Name: "ts", Type: event.TTimestamp}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestValidateAppRefusesEachMalformedRule(t *testing.T) {
	t.Parallel()
	const keyless = "a key-less rule has TTL 0 and the string pseudo-key"
	cases := map[string]struct {
		want  string
		edits []func(a *App)
	}{
		"valid":           {"", nil},
		"valid keyed":     {"", []func(a *App){bvKeyed}},
		"schema count":    {"app 3: 1 schemas for 2 streams", []func(a *App){func(a *App) { a.Schemas = a.Schemas[:1] }}},
		"schema stream":   {`schema 0 describes stream "x", not "s"`, nil},
		"no input":        {`app 3 rule 0 ("pass"): reads no stream`, []func(a *App){func(a *App) { a.Rules[0].Inputs = nil }}},
		"no body":         {"has no body", []func(a *App){func(a *App) { a.Rules[0].NewBody = nil }}},
		"no key type":     {"a key has at least one part", []func(a *App){func(a *App) { a.Rules[0].KeyTypes = nil }}},
		"key list count":  {"2 key attribute lists for 1 inputs", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyAttrs = [][]int{{0}, {0}} }}},
		"keyed TTL 0":     {"a keyed rule needs a TTL above 0", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].TTL = 0 }}},
		"keyed TTL < 0":   {"a keyed rule needs a TTL above 0", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].TTL = -1 }}},
		"keyless TTL":     {keyless, []func(a *App){func(a *App) { a.Rules[0].TTL = 1_000 }}},
		"keyless parts":   {keyless, []func(a *App){func(a *App) { a.Rules[0].KeyTypes = []event.Type{event.TString, event.TString} }}},
		"keyless type":    {keyless, []func(a *App){func(a *App) { a.Rules[0].KeyTypes = []event.Type{event.TInt} }}},
		"key type below":  {"cannot key a partition", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyTypes = []event.Type{event.TInvalid} }}},
		"key type object": {"cannot key a partition", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyTypes = []event.Type{event.TObject} }}},
		"input below":     {"input -1 is not a source stream", []func(a *App){func(a *App) { a.Rules[0].Inputs = []int{-1} }}},
		"input past":      {"input 2 is not a source stream", []func(a *App){func(a *App) { a.Rules[0].Inputs = []int{2} }}},
		"input derived":   {"input 1 is not a source stream", []func(a *App){func(a *App) { a.Rules[0].Inputs = []int{1} }}},
		"input twice":     {"input 0 is listed twice", []func(a *App){func(a *App) { a.Rules[0].Inputs = []int{0, 0} }}},
		"key parts":       {"stream 0's key has 2 parts, the rule's 1", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyAttrs = [][]int{{0, 1}} }}},
		"key attr below":  {"key attribute -1 of stream 0 is not a string attribute", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyAttrs = [][]int{{-1}} }}},
		"key attr past":   {"key attribute 2 of stream 0 is not a string attribute", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyAttrs = [][]int{{2}} }}},
		"key attr type":   {"key attribute 1 of stream 0 is not a string attribute", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyAttrs = [][]int{{1}} }}},
		"sink below":      {"app 3 sink 0: binds no derived stream", []func(a *App){func(a *App) { a.Sinks[0].Stream = -1 }}},
		"sink past":       {"app 3 sink 0: binds no derived stream", []func(a *App){func(a *App) { a.Sinks[0].Stream = 2 }}},
		"sink source":     {"app 3 sink 0: binds no derived stream", []func(a *App){func(a *App) { a.Sinks[0].Stream = 0 }}},
		"sink nil":        {"app 3 sink 0: binds no derived stream", []func(a *App){func(a *App) { a.Sinks[0].Sink = nil }}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			edits := c.edits
			if name == "schema stream" {
				edits = []func(a *App){func(a *App) { a.Schemas[0] = bvSchema(t, "x") }}
			}
			bvRun(t, c.want, edits...)
		})
	}
}

// Two faults at each adjacent pair of checks: the first in validateApp's order is the one named.
func TestValidateAppNamesTheFirstFault(t *testing.T) {
	t.Parallel()
	noInputs := func(a *App) { a.Rules[0].Inputs = nil }
	noBody := func(a *App) { a.Rules[0].NewBody = nil }
	badSink := func(a *App) { a.Sinks[0].Sink = nil }
	cases := map[string]struct {
		want  string
		edits []func(a *App)
	}{
		"schema count, bad sink":     {"1 schemas for 2 streams", []func(a *App){func(a *App) { a.Schemas = a.Schemas[:1] }, badSink}},
		"schema stream, rule":        {`describes stream "x"`, []func(a *App){noInputs}},
		"no inputs, no body":         {"reads no stream", []func(a *App){noInputs, noBody}},
		"no body, no key type":       {"has no body", []func(a *App){noBody, func(a *App) { a.Rules[0].KeyTypes = nil }}},
		"key list count, TTL 0":      {"2 key attribute lists", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyAttrs, a.Rules[0].TTL = [][]int{{0}, {0}}, 0 }}},
		"TTL 0, key type":            {"needs a TTL above 0", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].TTL, a.Rules[0].KeyTypes = 0, []event.Type{event.TObject} }}},
		"keyless TTL, key type":      {"a key-less rule has TTL 0", []func(a *App){func(a *App) { a.Rules[0].TTL, a.Rules[0].KeyTypes = 1_000, []event.Type{event.TObject} }}},
		"key type, bad input":        {"cannot key a partition", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].KeyTypes, a.Rules[0].Inputs = []event.Type{event.TObject}, []int{1} }}},
		"bad input, duplicate":       {"input 5 is not a source stream", []func(a *App){func(a *App) { a.Rules[0].Inputs = []int{5, 5} }}},
		"bad input, key attrs":       {"input 1 is not a source stream", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].Inputs, a.Rules[0].KeyAttrs = []int{1}, [][]int{{9}} }}},
		"key parts, later bad input": {"stream 0's key has 2 parts", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].Inputs, a.Rules[0].KeyAttrs = []int{0, 1}, [][]int{{0, 0}, {0}} }}},
		"key attr, later bad input":  {"key attribute 1 of stream 0", []func(a *App){bvKeyed, func(a *App) { a.Rules[0].Inputs, a.Rules[0].KeyAttrs = []int{0, 1}, [][]int{{1}, {0}} }}},
		"rule 0, rule 1":             {`rule 0 ("pass"): has no body`, []func(a *App){bvSecond, noBody, func(a *App) { a.Rules[1].Inputs = nil }}},
		"rule 1 alone":               {`rule 1 ("second"): reads no stream`, []func(a *App){bvSecond, func(a *App) { a.Rules[1].Inputs = nil }}},
		"last rule, bad sink":        {"reads no stream", []func(a *App){noInputs, badSink}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			edits := c.edits
			if name == "schema stream, rule" {
				edits = append([]func(a *App){func(a *App) { a.Schemas[0] = bvSchema(t, "x") }}, edits...)
			}
			bvRun(t, c.want, edits...)
		})
	}
}

func TestBuildRefusesAnInvalidApp(t *testing.T) {
	t.Parallel()
	good := cgApp(t, func(event.Event) error { return nil })
	bad := cgApp(t, func(event.Event) error { return nil })
	bad.Rules[0].Inputs = nil
	r, err := build(Config{Shards: 2}, []App{good, bad})
	if r != nil || err == nil || !strings.Contains(err.Error(), `app 1 rule 0 ("pass"): reads no stream`) {
		t.Fatalf("build with app 1 invalid: %v, %v; want no runtime and app 1's fault", r, err)
	}
}

// A rule a shard cannot build (its NewBody gives no body, which partition.NewRule refuses) is build's error; no
// runtime is returned.
func TestBuildReturnsARuleBuildError(t *testing.T) {
	t.Parallel()
	app := cgApp(t, func(event.Event) error { return nil })
	app.Rules[0].NewBody = func(func(event.Event)) partition.Body { return nil }
	r, err := build(Config{Shards: 2}, []App{app})
	if r != nil || err == nil || !strings.Contains(err.Error(), `app 0 rule 0 ("pass"): partition:`) {
		t.Fatalf("build with a panicking NewBody: %v, %v; want no runtime and the rule's build error", r, err)
	}
}
