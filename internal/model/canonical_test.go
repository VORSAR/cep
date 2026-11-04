package model_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// Every expected byte string in this file is hand-derived from MarshalCanonical's doc comment (model.go) and
// written before the encoder exists. None of it was produced by running an encoder.

// Hand-written fragments: the canonical form of zero values, keys in bytewise order.
const (
	zeroLit     = `{"bool":false,"float":0,"int":0,"kind":"","ms":0,"param":"","str":""}`
	zeroOperand = `{"param":"","strings":[],"table":""}`
	zeroDur     = `{"ms":0,"param":""}`
	zeroLate    = `{"allow":` + zeroDur + `,"mode":""}`
	zeroExpr    = `{"args":[],"kind":"","lit":` + zeroLit + `,"name":"","namespace":"","negated":false,"op":"",` +
		`"operand":` + zeroOperand + `,"qualifier":"","table":""}`
	zeroSource  = `{"filter":` + zeroExpr + `,"stream":""}`
	zeroPattern = `{"afterMatch":"","elements":[],"lateEvents":` + zeroLate + `,"restart":"","within":` + zeroDur + `}`
	zeroWindow  = `{"cronExpr":"","cronTZ":"","delay":` + zeroDur + `,"emit":"","kind":"","lateEvents":` + zeroLate +
		`,"sortAttr":"","sortDir":"","sortN":0,"source":` + zeroSource + `}`
)

// attrJSON is the hand-written canonical form of an attr Expr `qualifier.name` (qualifier may be empty).
func attrJSON(qualifier, name string) string {
	return `{"args":[],"kind":"attr","lit":` + zeroLit + `,"name":"` + name + `","namespace":"","negated":false,"op":"",` +
		`"operand":` + zeroOperand + `,"qualifier":"` + qualifier + `","table":""}`
}

func attr(name string) model.Expr { return model.Expr{Kind: model.ExprAttr, Name: name} }

func lit(l model.Literal) model.Expr { return model.Expr{Kind: model.ExprLiteral, Lit: l} }

// paramApp is an App holding one param and nothing else: the smallest App that carries a Literal.
func paramApp(name string, typ event.Type, def model.Literal) model.App {
	return model.App{Params: []model.Param{{Name: name, Type: typ, Default: def}}}
}

// paramAppJSON is paramApp's hand-written canonical form around a hand-written default literal.
func paramAppJSON(name, typ, defJSON string) string {
	return `{"language":"","params":[{"default":` + defJSON + `,"name":"` + name + `","type":"` + typ +
		`"}],"rules":[],"streams":[],"tables":[]}`
}

// mustEncode encodes a and fails the test on an error.
func mustEncode(t *testing.T, a model.App) []byte {
	t.Helper()
	got, err := a.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical: unexpected error: %v", err)
	}
	return got
}

// assertBytes fails with both forms when got is not exactly want.
func assertBytes(t *testing.T, got []byte, want string) {
	t.Helper()
	if !bytes.Equal(got, []byte(want)) {
		t.Errorf("canonical JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}

// smallApp is SPEC-shaped: a float param, a bounded source stream with idle after and an annotation, a derived
// stream, a table, and one plain rule with a filter (compare with ${param}, a text operator, `in`), a select with an
// alias, and output first every.
//
//	param threshold float = 0.75;
//	@source(type = 'stdin')
//	stream Logins (user string, score float, agent string optional, ts timestamp)
//	    time ts watermark bounded 5s idle after 30s;
//	stream Alerts (user string, s float);
//	table Blocked (user string) primary key (user);
//	rule RiskyLogin intent 'flag a risky login & its score'
//	  from Logins[score > ${threshold} and agent contains_ci 'curl' and user in Blocked]
//	  select user, score as s output first every 1m insert into Alerts;
func smallApp() model.App {
	filter := model.Expr{Kind: model.ExprAnd, Args: []model.Expr{
		{Kind: model.ExprAnd, Args: []model.Expr{
			{Kind: model.ExprCompare, Op: ">", Args: []model.Expr{
				attr("score"),
				lit(model.Literal{Kind: model.LitParam, Param: "threshold"}),
			}},
			{Kind: model.ExprText, Op: "contains_ci", Args: []model.Expr{attr("agent")},
				Operand: model.TextOperand{Strings: []string{"curl"}}},
		}},
		{Kind: model.ExprIn, Table: "Blocked", Args: []model.Expr{attr("user")}},
	}}
	return model.App{
		Language: "cep/0.1",
		Params: []model.Param{{
			Name: "threshold", Type: event.TFloat,
			Default: model.Literal{Kind: model.LitFloat, Float: 0.75},
		}},
		Streams: []model.Stream{
			{
				Annotations: []model.Annotation{{
					Name: "source",
					Items: []model.AnnotationItem{{
						Key: "type", Value: model.Literal{Kind: model.LitString, Str: "stdin"},
					}},
				}},
				Name: "Logins",
				Attrs: []model.Attr{
					{Name: "user", Type: event.TString},
					{Name: "score", Type: event.TFloat},
					{Name: "agent", Type: event.TString, Optional: true},
					{Name: "ts", Type: event.TTimestamp},
				},
				Time: model.TimeClause{Kind: model.TimeBounded, Attr: "ts", Bound: 5000, IdleAfter: 30000},
			},
			{
				Name:  "Alerts",
				Attrs: []model.Attr{{Name: "user", Type: event.TString}, {Name: "s", Type: event.TFloat}},
				Time:  model.TimeClause{Kind: model.TimeNone},
			},
		},
		Tables: []model.Table{{
			Name:       "Blocked",
			Attrs:      []model.Attr{{Name: "user", Type: event.TString}},
			PrimaryKey: []string{"user"},
		}},
		Rules: []model.Rule{{
			Name:             "RiskyLogin",
			Intent:           "flag a risky login & its score",
			Body:             model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: "Logins", Filter: filter}},
			Select:           []model.SelectItem{{Expr: attr("user")}, {Expr: attr("score"), As: "s"}},
			OutputFirstEvery: model.DurationOrParam{Ms: 60000},
			InsertInto:       "Alerts",
		}},
	}
}

// TestMarshalCanonicalGoldenSmallApp pins the whole encoding of one realistic App against a hand-written golden.
// The golden is indented for reading; json.Compact strips only the insignificant whitespace (it keeps key order,
// number text and string escapes exactly), so the comparison is byte for byte.
func TestMarshalCanonicalGoldenSmallApp(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/small_app.golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, raw); err != nil {
		t.Fatalf("golden is not JSON: %v", err)
	}
	got := mustEncode(t, smallApp())
	assertBytes(t, got, want.String())
	if bytes.HasSuffix(got, []byte("\n")) {
		t.Errorf("canonical JSON ends in a newline")
	}
}

// TestMarshalCanonicalKeyOrder pins bytewise key order where Go field order is not alphabetical (Attr is Name, Type,
// Optional: keys name, optional, type; Table is Name, Attrs, PrimaryKey: keys attrs, name, primaryKey), key pairs
// where one is a prefix of the other ("name" < "namespace", "op" < "operand"), and every event.Type's name.
func TestMarshalCanonicalKeyOrder(t *testing.T) {
	t.Parallel()
	t.Run("attr and table", func(t *testing.T) {
		t.Parallel()
		app := model.App{Tables: []model.Table{{
			Name:       "T",
			Attrs:      []model.Attr{{Name: "a", Type: event.TInt, Optional: true}},
			PrimaryKey: []string{"a"},
		}}}
		want := `{"language":"","params":[],"rules":[],"streams":[],"tables":[` +
			`{"attrs":[{"name":"a","optional":true,"type":"int"}],"name":"T","primaryKey":["a"]}]}`
		assertBytes(t, mustEncode(t, app), want)
	})
	t.Run("time clause", func(t *testing.T) {
		t.Parallel()
		// Go order Kind, Attr, Bound, IdleAfter; bytewise attr < bound < idleAfter < kind.
		app := model.App{Streams: []model.Stream{{
			Name:  "S",
			Attrs: []model.Attr{{Name: "t", Type: event.TTimestamp}},
			Time:  model.TimeClause{Kind: model.TimeAscending, Attr: "t", IdleAfter: 1},
		}}}
		want := `{"language":"","params":[],"rules":[],"streams":[{"annotations":[],` +
			`"attrs":[{"name":"t","optional":false,"type":"timestamp"}],"name":"S",` +
			`"time":{"attr":"t","bound":0,"idleAfter":1,"kind":"ascending"}}],"tables":[]}`
		assertBytes(t, mustEncode(t, app), want)
	})
	t.Run("prefix keys and every event type name", func(t *testing.T) {
		t.Parallel()
		// A call with a namespace: "name" sorts before "namespace", "op" before "operand".
		call := model.Expr{Kind: model.ExprCall, Namespace: "str", Name: "lower", Args: []model.Expr{attr("x")}}
		app := model.App{
			Rules: []model.Rule{{Select: []model.SelectItem{{Expr: call}}}},
			Streams: []model.Stream{{Name: "S", Attrs: []model.Attr{
				{Name: "a", Type: event.TInvalid}, {Name: "b", Type: event.TString}, {Name: "c", Type: event.TInt},
				{Name: "d", Type: event.TFloat}, {Name: "e", Type: event.TBool}, {Name: "f", Type: event.TTimestamp},
				{Name: "g", Type: event.TDuration}, {Name: "h", Type: event.TObject},
			}}},
		}
		wantCall := `{"args":[` + attrJSON("", "x") + `],"kind":"call","lit":` + zeroLit +
			`,"name":"lower","namespace":"str","negated":false,"op":"","operand":` + zeroOperand +
			`,"qualifier":"","table":""}`
		wantRule := `{"body":{"kind":"","pattern":` + zeroPattern + `,"source":` + zeroSource + `,"window":` + zeroWindow +
			`},"expects":[],"insertInto":"","intent":"","name":"","outputFirstEvery":` + zeroDur +
			`,"partition":{"keys":[],"ttl":` + zeroDur + `},"select":[{"as":"","expr":` + wantCall + `}],"todos":[]}`
		wantAttrs := `[{"name":"a","optional":false,"type":"invalid"},{"name":"b","optional":false,"type":"string"},` +
			`{"name":"c","optional":false,"type":"int"},{"name":"d","optional":false,"type":"float"},` +
			`{"name":"e","optional":false,"type":"bool"},{"name":"f","optional":false,"type":"timestamp"},` +
			`{"name":"g","optional":false,"type":"duration"},{"name":"h","optional":false,"type":"object"}]`
		want := `{"language":"","params":[],"rules":[` + wantRule + `],"streams":[{"annotations":[],"attrs":` + wantAttrs +
			`,"name":"S","time":{"attr":"","bound":0,"idleAfter":0,"kind":""}}],"tables":[]}`
		assertBytes(t, mustEncode(t, app), want)
	})
	t.Run("every object's keys ascend bytewise", func(t *testing.T) {
		t.Parallel()
		assertKeysAscend(t, mustEncode(t, smallApp()))
		assertKeysAscend(t, mustEncode(t, patternApp()))
		assertKeysAscend(t, mustEncode(t, windowApp()))
	})
}

// TestMarshalCanonicalFloats pins strconv.FormatFloat(f, 'g', -1, 64). Shortest 'g' uses e-notation when the
// decimal exponent is < -4 or >= 6 (two-digit exponent at least, with a sign), plain notation otherwise; the cases
// sit on both sides of both boundaries.
func TestMarshalCanonicalFloats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		f    float64
		want string
	}{
		{"negative zero", math.Copysign(0, -1), "-0"},
		{"positive zero", 0, "0"},
		{"0.75", 0.75, "0.75"},
		{"0.1", 0.1, "0.1"},
		{"1e21", 1e21, "1e+21"},
		{"1e-7", 1e-7, "1e-07"},
		{"123456789 as float", 123456789, "1.23456789e+08"},
		{"100000 stays plain (exponent 5)", 100000, "100000"},
		{"1e6 switches to e (exponent 6)", 1e6, "1e+06"},
		{"0.0001 stays plain (exponent -4)", 0.0001, "0.0001"},
		{"0.00001 switches to e (exponent -5)", 0.00001, "1e-05"},
		{"negative", -2.5, "-2.5"},
		{"shortest round-trip", 1.0 / 3.0, "0.3333333333333333"},
		{"largest finite", math.MaxFloat64, "1.7976931348623157e+308"},
		{"smallest subnormal", math.SmallestNonzeroFloat64, "5e-324"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			app := paramApp("p", event.TFloat, model.Literal{Kind: model.LitFloat, Float: c.f})
			wantLit := `{"bool":false,"float":` + c.want + `,"int":0,"kind":"float","ms":0,"param":"","str":""}`
			assertBytes(t, mustEncode(t, app), paramAppJSON("p", "float", wantLit))
		})
	}
}

// TestMarshalCanonicalNonFinite pins that NaN, +Inf and -Inf are not encodable wherever they sit: MarshalCanonical
// returns an error and nil bytes. Each case first encodes the same App with a finite value in that slot, so the
// error is caused by the non-finite float alone.
func TestMarshalCanonicalNonFinite(t *testing.T) {
	t.Parallel()
	places := []struct {
		name string
		app  func(f float64) model.App
	}{
		{"param default", func(f float64) model.App {
			return paramApp("p", event.TFloat, model.Literal{Kind: model.LitFloat, Float: f})
		}},
		{"nested filter literal", func(f float64) model.App {
			app := smallApp()
			// Replace ${threshold} deep inside the filter: and(and(compare(score, HERE), …), …).
			app.Rules[0].Body.Source.Filter.Args[0].Args[0].Args[1] = lit(model.Literal{Kind: model.LitFloat, Float: f})
			return app
		}},
		{"select expression", func(f float64) model.App {
			app := smallApp()
			app.Rules[0].Select = append(app.Rules[0].Select, model.SelectItem{
				Expr: model.Expr{Kind: model.ExprArith, Op: "*", Args: []model.Expr{
					attr("score"), lit(model.Literal{Kind: model.LitFloat, Float: f}),
				}},
				As: "scaled",
			})
			return app
		}},
		{"expect record field", func(f float64) model.App {
			app := windowApp()
			app.Rules[0].Expects[0].Givens[0].Records[0].Fields[0].Value = model.Literal{Kind: model.LitFloat, Float: f}
			return app
		}},
		{"annotation value", func(f float64) model.App {
			app := smallApp()
			app.Streams[0].Annotations[0].Items[0].Value = model.Literal{Kind: model.LitFloat, Float: f}
			return app
		}},
	}
	bad := []struct {
		name string
		f    float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
	}
	for _, p := range places {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			if _, err := p.app(1.5).MarshalCanonical(); err != nil {
				t.Fatalf("control with a finite float in the slot: unexpected error: %v", err)
			}
			for _, b := range bad {
				got, err := p.app(b.f).MarshalCanonical()
				if err == nil {
					t.Errorf("%s: no error; got %s", b.name, got)
				}
				if got != nil {
					t.Errorf("%s: bytes returned with the error: %q", b.name, got)
				}
			}
		})
	}
}

// TestMarshalCanonicalNonFiniteErrorNamesSlot pins "an error names the slot" without fixing its format: the same
// NaN in two different slots gives two different messages.
func TestMarshalCanonicalNonFiniteErrorNamesSlot(t *testing.T) {
	t.Parallel()
	a := paramApp("p", event.TFloat, model.Literal{Kind: model.LitFloat, Float: math.NaN()})
	b := smallApp()
	b.Rules[0].Body.Source.Filter.Args[0].Args[0].Args[1] = lit(model.Literal{Kind: model.LitFloat, Float: math.NaN()})
	if _, err := smallApp().MarshalCanonical(); err != nil {
		t.Fatalf("control: unexpected error: %v", err)
	}
	_, errA := a.MarshalCanonical()
	_, errB := b.MarshalCanonical()
	if errA == nil || errB == nil {
		t.Fatalf("NaN encoded without an error: param default err=%v, filter err=%v", errA, errB)
	}
	if errA.Error() == errB.Error() {
		t.Errorf("NaN in a param default and NaN in a filter give the same error, so it names no slot: %v", errA)
	}
}

// TestMarshalCanonicalTimestamps pins a timestamp Literal's "ms" as an RFC 3339 UTC string with exactly three
// fraction digits (trailing zeros kept), before and after the epoch; any other "ms" stays an integer.
func TestMarshalCanonicalTimestamps(t *testing.T) {
	t.Parallel()
	// 2026-01-01T00:00:00Z = 20454 days after the epoch (56 years, 14 leap days) = 1767225600 s; +10 h = 1767261600 s.
	cases := []struct {
		name string
		ms   int64
		want string
	}{
		{"whole second", 1767261600000, "2026-01-01T10:00:00.000Z"},
		{"millisecond remainder", 1767261600123, "2026-01-01T10:00:00.123Z"},
		{"trailing zero kept", 1767261600120, "2026-01-01T10:00:00.120Z"},
		{"one millisecond", 1767261600001, "2026-01-01T10:00:00.001Z"},
		{"epoch", 0, "1970-01-01T00:00:00.000Z"},
		{"one ms before the epoch", -1, "1969-12-31T23:59:59.999Z"},
		{"1.5 s before the epoch", -1500, "1969-12-31T23:59:58.500Z"},
		// 1969-07-20T20:17:40Z = -165 days + 20 h 17 m 40 s = -14256000 + 73060 = -14182940 s.
		{"a 1969 instant", -14182940000, "1969-07-20T20:17:40.000Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			app := paramApp("t", event.TTimestamp, model.Literal{Kind: model.LitTimestamp, Ms: c.ms})
			wantLit := `{"bool":false,"float":0,"int":0,"kind":"timestamp","ms":"` + c.want + `","param":"","str":""}`
			assertBytes(t, mustEncode(t, app), paramAppJSON("t", "timestamp", wantLit))
		})
	}
	t.Run("duration literal ms stays an integer", func(t *testing.T) {
		t.Parallel()
		app := paramApp("d", event.TDuration, model.Literal{Kind: model.LitDuration, Ms: 1767261600000})
		wantLit := `{"bool":false,"float":0,"int":0,"kind":"duration","ms":1767261600000,"param":"","str":""}`
		assertBytes(t, mustEncode(t, app), paramAppJSON("d", "duration", wantLit))
	})
	t.Run("negative int and ms outside a timestamp", func(t *testing.T) {
		t.Parallel()
		app := paramApp("i", event.TInt, model.Literal{Kind: model.LitInt, Int: -42, Ms: -7})
		wantLit := `{"bool":false,"float":0,"int":-42,"kind":"int","ms":-7,"param":"","str":""}`
		assertBytes(t, mustEncode(t, app), paramAppJSON("i", "int", wantLit))
	})
}

// TestMarshalCanonicalStrings pins "escaped as encoding/json does with HTML escaping off": < > & stay literal;
// quote, backslash and the short control escapes; other controls as \u00xx; U+2028 and U+2029 always escaped.
func TestMarshalCanonicalStrings(t *testing.T) {
	t.Parallel()
	in := "a\"b\\c\nd\re\tf\bg\fh\x01i<j>k&l m né"
	want := `a\"b\\c\nd\re\tf\bg\fh\u0001i<j>k&l\u2028m\u2029né`
	app := model.App{Language: in}
	assertBytes(t, mustEncode(t, app), `{"language":"`+want+`","params":[],"rules":[],"streams":[],"tables":[]}`)
}

// TestMarshalCanonicalDeterminism pins byte stability: the same App twice, equal Apps built in different code
// orders, and nil versus empty slices all encode to the same bytes.
func TestMarshalCanonicalDeterminism(t *testing.T) {
	t.Parallel()
	t.Run("same app twice", func(t *testing.T) {
		t.Parallel()
		for _, app := range []model.App{smallApp(), patternApp(), windowApp()} {
			first := mustEncode(t, app)
			second := mustEncode(t, app)
			if !bytes.Equal(first, second) {
				t.Errorf("two encodings differ\n 1st: %s\n 2nd: %s", first, second)
			}
		}
	})
	t.Run("equal apps built in different orders", func(t *testing.T) {
		t.Parallel()
		// Built field by field, bottom up, in reverse declaration order, with appends.
		var b model.App
		var r model.Rule
		r.InsertInto = "Alerts"
		r.OutputFirstEvery.Ms = 60000
		r.Select = append(r.Select, model.SelectItem{Expr: attr("user")})
		r.Select = append(r.Select, model.SelectItem{As: "s", Expr: attr("score")})
		r.Body.Source.Filter = smallApp().Rules[0].Body.Source.Filter
		r.Body.Source.Stream = "Logins"
		r.Body.Kind = model.BodyPlain
		r.Intent = "flag a risky login & its score"
		r.Name = "RiskyLogin"
		b.Rules = append(b.Rules, r)
		var tbl model.Table
		tbl.PrimaryKey = append(tbl.PrimaryKey, "user")
		tbl.Attrs = append(tbl.Attrs, model.Attr{Type: event.TString, Name: "user"})
		tbl.Name = "Blocked"
		b.Tables = append(b.Tables, tbl)
		b.Streams = append(b.Streams, model.Stream{}, model.Stream{})
		b.Streams[1].Time.Kind = model.TimeNone
		b.Streams[1].Attrs = []model.Attr{{Type: event.TString, Name: "user"}, {Type: event.TFloat, Name: "s"}}
		b.Streams[1].Name = "Alerts"
		b.Streams[0].Time = model.TimeClause{IdleAfter: 30000, Bound: 5000, Attr: "ts", Kind: model.TimeBounded}
		b.Streams[0].Attrs = []model.Attr{
			{Optional: false, Type: event.TString, Name: "user"},
			{Type: event.TFloat, Name: "score"},
			{Optional: true, Type: event.TString, Name: "agent"},
			{Type: event.TTimestamp, Name: "ts"},
		}
		b.Streams[0].Name = "Logins"
		b.Streams[0].Annotations = []model.Annotation{{
			Items: []model.AnnotationItem{{Value: model.Literal{Str: "stdin", Kind: model.LitString}, Key: "type"}},
			Name:  "source",
		}}
		b.Params = []model.Param{{Default: model.Literal{Float: 0.75, Kind: model.LitFloat}, Type: event.TFloat, Name: "threshold"}}
		b.Language = "cep/0.1"
		if !reflect.DeepEqual(b, smallApp()) {
			t.Fatalf("test setup: the two apps are not equal")
		}
		assertBytes(t, mustEncode(t, b), string(mustEncode(t, smallApp())))
	})
	t.Run("nil and empty slices encode the same", func(t *testing.T) {
		t.Parallel()
		nilApp := model.App{Rules: []model.Rule{{Select: []model.SelectItem{{Expr: attr("x")}}}}}
		emptyApp := model.App{
			Params:  []model.Param{},
			Streams: []model.Stream{},
			Tables:  []model.Table{},
			Rules: []model.Rule{{
				Partition: model.Partition{Keys: []model.PartitionKey{}},
				Body: model.Body{
					Source:  model.Source{Filter: model.Expr{Args: []model.Expr{}, Operand: model.TextOperand{Strings: []string{}}}},
					Pattern: model.Pattern{Elements: []model.PatternElement{}},
					Window: model.Window{Source: model.Source{Filter: model.Expr{
						Args: []model.Expr{}, Operand: model.TextOperand{Strings: []string{}},
					}}},
				},
				Expects: []model.Expect{},
				Todos:   []string{},
				Select: []model.SelectItem{{Expr: model.Expr{
					Kind: model.ExprAttr, Name: "x", Args: []model.Expr{}, Operand: model.TextOperand{Strings: []string{}},
				}}},
			}},
		}
		wantRule := `{"body":{"kind":"","pattern":` + zeroPattern + `,"source":` + zeroSource + `,"window":` + zeroWindow +
			`},"expects":[],"insertInto":"","intent":"","name":"","outputFirstEvery":` + zeroDur +
			`,"partition":{"keys":[],"ttl":` + zeroDur + `},"select":[{"as":"","expr":` + attrJSON("", "x") +
			`}],"todos":[]}`
		want := `{"language":"","params":[],"rules":[` + wantRule + `],"streams":[],"tables":[]}`
		assertBytes(t, mustEncode(t, nilApp), want)
		assertBytes(t, mustEncode(t, emptyApp), want)
	})
	t.Run("zero app", func(t *testing.T) {
		t.Parallel()
		assertBytes(t, mustEncode(t, model.App{}), `{"language":"","params":[],"rules":[],"streams":[],"tables":[]}`)
	})
}

// TestMarshalCanonicalNoPositions pins that the canonical JSON carries no source position (ADR-0003 §1 as annotated
// at S2.2b): positions live only in fields of type model.Pos or []model.Pos named "…Pos"; no other field reachable
// from App is position-like; no key in the encoded JSON of three full Apps is position-like; and an App whose every
// position is set encodes byte for byte as the same App with none.
func TestMarshalCanonicalNoPositions(t *testing.T) {
	t.Parallel()
	positional := func(name string) bool {
		n := strings.ToLower(name)
		switch n {
		case "pos", "position", "line", "col", "column", "offset", "span", "start", "end", "file", "loc", "location":
			return true
		}
		return strings.HasSuffix(n, "pos") || strings.HasSuffix(n, "line") || strings.HasSuffix(n, "col") ||
			strings.HasSuffix(n, "offset")
	}
	posType, posSlice := reflect.TypeFor[model.Pos](), reflect.TypeFor[[]model.Pos]()
	seen := map[reflect.Type]bool{}
	positions := 0
	var walk func(reflect.Type)
	walk = func(t2 reflect.Type) {
		switch t2.Kind() {
		case reflect.Slice, reflect.Array, reflect.Pointer:
			walk(t2.Elem())
		case reflect.Struct:
			if seen[t2] || t2 == posType {
				return
			}
			seen[t2] = true
			for i := range t2.NumField() {
				f := t2.Field(i)
				isPos := f.Type == posType || f.Type == posSlice
				switch {
				case isPos && !strings.HasSuffix(f.Name, "Pos"):
					t.Errorf("%s.%s holds a position but is not named …Pos", t2.Name(), f.Name)
				case isPos:
					positions++
				case positional(f.Name):
					t.Errorf("%s.%s is a position-like field of type %s, not model.Pos", t2.Name(), f.Name, f.Type)
				}
				walk(f.Type)
			}
		}
	}
	walk(reflect.TypeFor[model.App]())
	if positions == 0 {
		t.Fatal("no model.Pos field is reachable from App: the walk is not looking at the model")
	}
	for _, app := range []model.App{smallApp(), patternApp(), windowApp()} {
		plain := mustEncode(t, app)
		for _, k := range allKeys(t, plain) {
			if positional(k) {
				t.Errorf("encoded JSON has a position-like key %q", k)
			}
		}
		stamped := app
		if n := stampPositions(reflect.ValueOf(&stamped).Elem()); n == 0 {
			t.Fatal("stamping set no position: the comparison below would prove nothing")
		}
		if reflect.DeepEqual(stamped, app) {
			t.Fatal("the stamped App equals the plain one: stamping changed nothing")
		}
		assertBytes(t, mustEncode(t, stamped), string(plain))
	}
}

// stampPositions sets every model.Pos reachable from v to a non-zero position and counts them. Slices are copied
// first, so the App v was copied from keeps its own positions.
func stampPositions(v reflect.Value) int {
	n := 0
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == reflect.TypeFor[model.Pos]() {
			v.Set(reflect.ValueOf(model.Pos{Line: 7, Col: 9}))
			return 1
		}
		for i := range v.NumField() {
			n += stampPositions(v.Field(i))
		}
	case reflect.Slice:
		if v.Len() == 0 {
			return 0
		}
		cp := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(cp, v)
		v.Set(cp)
		for i := range v.Len() {
			n += stampPositions(v.Index(i))
		}
	}
	return n
}

// patternApp holds one pattern rule:
//
//	rule Brute partition by user of Fails, user of Logins ttl 1h
//	  from every f = Fails -> and not Logins[user == f.user] for 5m
//	  within ${win} after match skip past last late events drop
//	  select f.user insert into Out;
func patternApp() model.App {
	return model.App{Rules: []model.Rule{{
		Name: "Brute",
		Partition: model.Partition{
			Keys: []model.PartitionKey{{Attr: "user", Stream: "Fails"}, {Attr: "user", Stream: "Logins"}},
			TTL:  model.DurationOrParam{Ms: 3600000},
		},
		Body: model.Body{Kind: model.BodyPattern, Pattern: model.Pattern{
			Restart: "every",
			Elements: []model.PatternElement{
				{Kind: model.ElemStep, Steps: []model.Step{{Name: "f", Source: model.Source{Stream: "Fails"}}}},
				{
					Kind: model.ElemAbsence,
					Absence: model.Source{Stream: "Logins", Filter: model.Expr{
						Kind: model.ExprCompare, Op: "==",
						Args: []model.Expr{attr("user"), {Kind: model.ExprAttr, Qualifier: "f", Name: "user"}},
					}},
					For: model.DurationOrParam{Ms: 300000},
				},
			},
			Within:     model.DurationOrParam{Param: "win"},
			AfterMatch: "skip past last",
			LateEvents: model.LateEvents{Mode: "drop"},
		}},
		Select:     []model.SelectItem{{Expr: model.Expr{Kind: model.ExprAttr, Qualifier: "f", Name: "user"}}},
		InsertInto: "Out",
	}}}
}

// windowApp holds one window rule with an expect block and a todo:
//
//	rule Top from Logins window sort(10, score desc) emit on close late events drop
//	  expect { given Logins [{user: 'u1', ts: 2026-01-01T10:00:00Z}] then Top [{agent: null}] }
//	  todo 'tune N' select user insert into Top;
func windowApp() model.App {
	return model.App{Rules: []model.Rule{{
		Name: "Top",
		Body: model.Body{Kind: model.BodyWindow, Window: model.Window{
			Source:     model.Source{Stream: "Logins"},
			Kind:       model.WindowSort,
			SortN:      10,
			SortAttr:   "score",
			SortDir:    "desc",
			Emit:       "close",
			LateEvents: model.LateEvents{Mode: "drop"},
		}},
		Expects: []model.Expect{{
			Givens: []model.Given{{Name: "Logins", Records: []model.Record{{Fields: []model.Field{
				{Name: "user", Value: model.Literal{Kind: model.LitString, Str: "u1"}},
				{Name: "ts", Value: model.Literal{Kind: model.LitTimestamp, Ms: 1767261600000}},
			}}}}},
			Then: model.Then{Name: "Top", Records: []model.Record{{Fields: []model.Field{
				{Name: "agent", Value: model.Literal{Kind: model.LitNull}},
			}}}},
		}},
		Todos:      []string{"tune N"},
		Select:     []model.SelectItem{{Expr: attr("user")}},
		InsertInto: "Top",
	}}}
}

// TestMarshalCanonicalPatternRule pins a pattern rule's body: its Kind's fields carry the pattern, and the plain
// source and the window are present as zero values.
func TestMarshalCanonicalPatternRule(t *testing.T) {
	t.Parallel()
	rule := firstRule(t, mustEncode(t, patternApp()))
	body := object(t, rule["body"])
	wantCompare := `{"args":[` + attrJSON("", "user") + `,` + attrJSON("f", "user") + `],"kind":"compare","lit":` + zeroLit +
		`,"name":"","namespace":"","negated":false,"op":"==","operand":` + zeroOperand + `,"qualifier":"","table":""}`
	wantPattern := `{"afterMatch":"skip past last","elements":[` +
		`{"absence":` + zeroSource + `,"for":` + zeroDur + `,"kind":"step","steps":[{"name":"f","source":{"filter":` +
		zeroExpr + `,"stream":"Fails"}}]},` +
		`{"absence":{"filter":` + wantCompare + `,"stream":"Logins"},"for":{"ms":300000,"param":""},"kind":"absence",` +
		`"steps":[]}],` +
		`"lateEvents":{"allow":` + zeroDur + `,"mode":"drop"},"restart":"every","within":{"ms":0,"param":"win"}}`
	assertBytes(t, body["kind"], `"pattern"`)
	assertBytes(t, body["pattern"], wantPattern)
	assertBytes(t, body["source"], zeroSource)
	assertBytes(t, body["window"], zeroWindow)
	assertBytes(t, rule["partition"],
		`{"keys":[{"attr":"user","stream":"Fails"},{"attr":"user","stream":"Logins"}],"ttl":{"ms":3600000,"param":""}}`)
	assertBytes(t, rule["select"], `[{"as":"","expr":`+attrJSON("f", "user")+`}]`)
	assertBytes(t, rule["expects"], `[]`)
	assertBytes(t, rule["todos"], `[]`)
	assertBytes(t, rule["outputFirstEvery"], zeroDur)
}

// TestMarshalCanonicalWindowRule pins a window rule's body, plus an expect block (a timestamp in a record field, a
// null literal) and a todo.
func TestMarshalCanonicalWindowRule(t *testing.T) {
	t.Parallel()
	rule := firstRule(t, mustEncode(t, windowApp()))
	body := object(t, rule["body"])
	wantWindow := `{"cronExpr":"","cronTZ":"","delay":` + zeroDur + `,"emit":"close","kind":"sort",` +
		`"lateEvents":{"allow":` + zeroDur + `,"mode":"drop"},"sortAttr":"score","sortDir":"desc","sortN":10,` +
		`"source":{"filter":` + zeroExpr + `,"stream":"Logins"}}`
	assertBytes(t, body["kind"], `"window"`)
	assertBytes(t, body["window"], wantWindow)
	assertBytes(t, body["pattern"], zeroPattern)
	assertBytes(t, body["source"], zeroSource)
	wantExpects := `[{"givens":[{"name":"Logins","records":[{"fields":[` +
		`{"name":"user","value":{"bool":false,"float":0,"int":0,"kind":"string","ms":0,"param":"","str":"u1"}},` +
		`{"name":"ts","value":{"bool":false,"float":0,"int":0,"kind":"timestamp","ms":"2026-01-01T10:00:00.000Z",` +
		`"param":"","str":""}}]}],"table":false}],` +
		`"then":{"name":"Top","records":[{"fields":[` +
		`{"name":"agent","value":{"bool":false,"float":0,"int":0,"kind":"null","ms":0,"param":"","str":""}}]}]}}]`
	assertBytes(t, rule["expects"], wantExpects)
	assertBytes(t, rule["todos"], `["tune N"]`)
	assertBytes(t, rule["partition"], `{"keys":[],"ttl":`+zeroDur+`}`)
}

// firstRule decodes the encoding's rules[0] into its raw members (json.RawMessage keeps each member's bytes).
func firstRule(t *testing.T, enc []byte) map[string]json.RawMessage {
	t.Helper()
	top := object(t, enc)
	var rules []json.RawMessage
	if err := json.Unmarshal(top["rules"], &rules); err != nil || len(rules) == 0 {
		t.Fatalf("rules: %v (%d rules) in %s", err, len(rules), enc)
	}
	return object(t, rules[0])
}

// object decodes one JSON object into its raw members.
func object(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %v: %s", err, raw)
	}
	return m
}

// allKeys returns every object key in enc, at every depth.
func allKeys(t *testing.T, enc []byte) []string {
	t.Helper()
	var keys []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				keys = append(keys, k)
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	var v any
	if err := json.Unmarshal(enc, &v); err != nil {
		t.Fatalf("not JSON: %v: %s", err, enc)
	}
	walk(v)
	return keys
}

// assertKeysAscend walks enc's token stream and fails on any object whose keys are not strictly ascending bytewise.
func assertKeysAscend(t *testing.T, enc []byte) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(enc))
	type frame struct {
		object  bool
		wantKey bool
		last    string
		first   bool
	}
	var stack []frame
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].wantKey {
			if k, ok := tok.(string); ok {
				top := &stack[n-1]
				if !top.first && k <= top.last {
					t.Errorf("key %q follows %q: not bytewise ascending", k, top.last)
				}
				top.last, top.first, top.wantKey = k, false, false
				continue
			}
		}
		switch tok {
		case json.Delim('{'):
			stack = append(stack, frame{object: true, wantKey: true, first: true})
			continue
		case json.Delim('['):
			stack = append(stack, frame{})
			continue
		case json.Delim('}'), json.Delim(']'):
			stack = stack[:len(stack)-1]
		}
		// A value (scalar or a closed container) completes a member of the enclosing object.
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].wantKey = true
		}
	}
}
