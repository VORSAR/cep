package runtime

// The defensive spots kept unreachable (owner, G1 2026-11-16, decision 3), each guarded by a test of the contract that
// makes it unreachable: if the contract changes, the test fails before the guard matters.
//   - SendPayload's !errors.As conversion (boundary.go): every error event.Decoder.Decode returns is an
//     *event.DecodeError.
//   - the a == nil slot checks (appOf, the router's ingest and take, Close's loops): no code stores a nil slot in the
//     app list, and the router's app returns nil only for an id beyond the list.
// Prefix: dg.

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// dgMembers is a valid payload's members, in order, for a schema with one attribute of each kind; ts is the time.
var dgMembers = [][2]string{
	{"k", `"a"`}, {"i", `1`}, {"d", `2`}, {"f", `1.5`}, {"b", `true`}, {"o", `{"x":[1]}`},
	{"ts", `"2026-01-01T10:00:00.000Z"`},
}

// dgPayload is the valid payload with member name's value replaced by value ("" drops the member).
func dgPayload(name, value string) string {
	var parts []string
	for _, m := range dgMembers {
		v := m[1]
		if m[0] == name {
			if value == "" {
				continue
			}
			v = value
		}
		parts = append(parts, `"`+m[0]+`":`+v)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func TestEveryDecoderErrorIsADecodeError(t *testing.T) {
	t.Parallel()
	attrs := []event.Attr{
		{Name: "k", Type: event.TString}, {Name: "i", Type: event.TInt}, {Name: "d", Type: event.TDuration},
		{Name: "f", Type: event.TFloat}, {Name: "b", Type: event.TBool}, {Name: "o", Type: event.TObject},
		{Name: "ts", Type: event.TTimestamp},
	}
	s, err := event.NewSchema("s", attrs, 6)
	if err != nil {
		t.Fatal(err)
	}
	dec := event.NewDecoder(s)
	if _, _, err := dec.Decode([]byte(dgPayload("", "")), nil); err != nil {
		t.Fatalf("the valid payload: %v; want nil (the witness that each row below fails on its own fault)", err)
	}
	deep := `{"x":` + strings.Repeat("[", event.MaxDepth) + strings.Repeat("]", event.MaxDepth) + `}`
	rows := map[string]string{
		"empty":                 ``,
		"not an object":         `[]`,
		"data after":            `{} x`,
		"member name":           `{1:2}`,
		"colon":                 `{"k" 1}`,
		"unterminated object":   `{"k":"a"`,
		"comma or brace":        `{"k":"a" "i":1}`,
		"end of payload":        `{"k":`,
		"no value":              `{"k":@}`,
		"too deep":              deep,
		"unterminated array":    `{"x":[1`,
		"comma or bracket":      `{"x":[1 2]}`,
		"unterminated string":   `{"k":"abc`,
		"unterminated escape":   `{"k":"\`,
		"unterminated \\u":      `{"k":"\u12`,
		"hex":                   `{"k":"\u12G4"}`,
		"invalid escape":        `{"k":"\q"}`,
		"control byte":          "{\"k\":\"a\x01\"}",
		"number sign only":      `{"x":-}`,
		"number fraction":       `{"x":1.}`,
		"number exponent":       `{"x":1e}`,
		"unterminated number":   `{"x":-`,
		"literal":               `{"x":tru}`,
		"nested member name":    `{"x":{1}}`,
		"nested colon":          `{"x":{"a" 1}}`,
		"nested unterminated":   `{"x":{"a":1`,
		"nested comma or brace": `{"x":{"a":1 "b":2}}`,
		"missing":               dgPayload("k", ""),
		"missing time":          dgPayload("ts", ""),
		"null":                  dgPayload("k", "null"),
		"null time":             dgPayload("ts", "null"),
		"string type":           dgPayload("k", "1"),
		"int type":              dgPayload("i", `"x"`),
		"int fraction":          dgPayload("i", "1.5"),
		"int range":             dgPayload("i", "99999999999999999999"),
		"duration type":         dgPayload("d", `"x"`),
		"float type":            dgPayload("f", `"x"`),
		"float range":           dgPayload("f", "1e999"),
		"bool type":             dgPayload("b", "1"),
		"timestamp type":        dgPayload("ts", "5"),
		"timestamp text":        dgPayload("ts", `"yesterday"`),
		"object type":           dgPayload("o", "1"),
	}
	for name, p := range rows {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, err := event.NewDecoder(s).Decode([]byte(p), nil) // each row its own decoder
			var de *event.DecodeError
			if err == nil || !errors.As(err, &de) {
				t.Fatalf("Decode(%.60q): %v (%T); want an *event.DecodeError", p, err, err)
			}
		})
	}
}

// dgNoNilSlot fails when the app list holds a nil slot.
func dgNoNilSlot(t *testing.T, r *Runtime, step string) {
	t.Helper()
	for id, a := range r.appList() {
		if a == nil {
			t.Fatalf("after %s: slot %d is nil; want every slot an app or its tombstone", step, id)
		}
	}
}

// No lifecycle step stores a nil slot: Add (and its rollback), Activate, Drain, Remove and Close each leave every slot
// an app or a removed app's tombstone.
func TestNoLifecycleStepStoresANilSlot(t *testing.T) {
	t.Parallel()
	ctx := cgCtx(t)
	r, err := New(Config{Shards: 2, Mailbox: 4, Intake: 4, SinkQueue: 4, Now: func() int64 { return cgT0 }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(cgCtx(t)) })
	var ids []AppID
	for i := range 3 {
		id, err := r.Add(ctx, cgApp(t, func(event.Event) error { return nil }))
		if err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
		ids = append(ids, id)
		dgNoNilSlot(t, r, "Add")
	}
	broken := cgApp(t, func(event.Event) error { return nil })
	broken.Rules[0].NewBody = func(func(event.Event)) partition.Body { return nil }
	if _, err := r.Add(ctx, broken); err == nil {
		t.Fatal("Add of an app no shard can build: nil; want its error (the rollback)")
	}
	dgNoNilSlot(t, r, "a rolled-back Add")
	for _, id := range ids[:2] {
		if err := r.Activate(ctx, id); err != nil {
			t.Fatalf("Activate %d: %v", id, err)
		}
		dgNoNilSlot(t, r, "Activate")
	}
	if err := r.Send(ctx, ids[0], cgEv(1)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if _, err := r.Drain(ctx, ids[0]); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	dgNoNilSlot(t, r, "Drain")
	for _, id := range []AppID{ids[0], ids[2]} { // a drained app, an inactive one
		if err := r.Remove(ctx, id); err != nil {
			t.Fatalf("Remove %d: %v", id, err)
		}
		dgNoNilSlot(t, r, "Remove")
	}
	if len(r.appList()) != 4 {
		t.Fatalf("%d slots; want 4 (three adds and the rolled-back one's tombstone; ids are never reused)", len(r.appList()))
	}
	if err := r.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dgNoNilSlot(t, r, "Close")
}

// The router's app is nil only for an id beyond the list; the last id is an app (the witness).
func TestRouterAppIsNilOnlyBeyondTheList(t *testing.T) {
	t.Parallel()
	w := igBuild(t, 1, 1, nil)
	n := AppID(len(w.r.appList()))
	if w.rt.app(n-1) == nil || w.rt.app(n) != nil {
		t.Fatalf("the router's app(%d), app(%d): %p, %p; want the app, then nil", n-1, n, w.rt.app(n-1), w.rt.app(n))
	}
}
