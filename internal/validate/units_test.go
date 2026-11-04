package validate

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

// base is a clean preamble: two source streams with idleness, an output stream, a one-column and a composite
// table, and three params. Every case appends rules, at least one of them inserting into Out.
const base = `language cep/0.1;
param lim int = 5;
param terms string = 'a, b';
param ratio float = 2;
stream S (k string, n int, note string optional, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;
stream R (k string, m int, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;
stream Out (k string, ts timestamp);
table T (k string) primary key (k);
table Pair (k string, v string) primary key (k, v);
`

// baseLines is where an appended rule starts: line numbers in the cases below count from it.
var baseLines = strings.Count(base, "\n")

func got(t *testing.T, src string, opts Options) []string {
	t.Helper()
	var out []string
	for _, f := range findings(t, "unit.cep", src, opts) {
		out = append(out, f.Code)
	}
	return out
}

func parse(t *testing.T, src string) *model.App {
	t.Helper()
	res, err := parser.Parse("unit.cep", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return res.App
}

const plain = "rule P intent 'x' from S select k insert into Out;\n"

// TestOneFindingPerDefect: a defect that could set off several codes gives exactly one (design §5).
func TestOneFindingPerDefect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, rules, want string
	}{
		{"a clockless source read by a window: no E-IDLE-001", "stream Q (k string, ts timestamp);\n" +
			"rule W intent 'x' partition by k of Q ttl 1 hour from Q window batch() emit on batch end late events drop select k insert into Out;\n",
			"E-TIME-001"},
		{"a pattern with no partition: no E-PART-002", "rule P intent 'x' from every a = S -> b = R within 5 min after match skip past last late events drop select a.k insert into Out;\n",
			"E-PART-001"},
		{"ttl 0: no E-TTL-002", "rule P intent 'x' partition by k of S ttl 0 ms from every a = S within 5 min after match skip past last late events drop select a.k insert into Out;\n",
			"E-DUR-003"},
		{"allow 0: no E-DUR-003", "rule P intent 'x' partition by k of S ttl 1 hour from every a = S within 5 min after match skip past last late events allow 0 ms select a.k insert into Out;\n",
			"E-LATE-002"},
		{"!= over an optional attribute in an absence: no E-NULL-002", "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = S and not R[k != 'x' and n > 1] for 1 min within 5 min after match skip past last late events drop select a.k insert into Out;\n",
			"E-NAME-001"},
		{"an unknown from stream: nothing about its filter or select", "rule P intent 'x' from Nope[x > 1] select y insert into Out;\n", "E-NAME-001"},
		{"an undeclared param in a filter: one finding", "rule P intent 'x' from S[n > ${nope}] select k insert into Out;\n", "E-PARAM-001"},
		{"a bad param default: reported once, at the declaration", "param bad int = 'x';\nrule P intent 'x' from S[n > ${bad}] select k insert into Out;\n", "E-PARAM-001"},
		{"a composite-key in: no type error", "rule P intent 'x' from S[k in Pair] select k insert into Out;\n", "E-TBL-001"},
		{"count() in a filter: no type error", "rule P intent 'x' from S[count() > 1] select k insert into Out;\n", "E-FN-001"},
		{"a capture shadowing a stream reads as a capture", "rule P intent 'x' partition by k of S ttl 1 hour from every S = S within 5 min after match skip past last late events drop select S.k insert into Out;\n",
			"E-NAME-002"},
		{"a self-reading rule: E-GRAPH-004, never E-GRAPH-001", "stream D (k string, ts timestamp);\nrule P intent 'x' from D select k insert into D;\n" + plain, "E-GRAPH-004"},
		{"a cycle through clocked streams: E-GRAPH-002, never E-GRAPH-001", "rule A intent 'x' from S select k, n as m insert into R;\nrule B intent 'x' from R select k, m as n, k as note insert into S;\n" + plain,
			"E-GRAPH-002 E-GRAPH-002"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if g := strings.Join(got(t, base+c.rules, Options{}), " "); g != c.want {
				t.Fatalf("got [%s], want [%s]", g, c.want)
			}
		})
	}
	t.Run("a version this engine does not read stops the check", func(t *testing.T) {
		t.Parallel()
		src := strings.Replace(base, "cep/0.1", "cep/0.2", 1) + "rule P intent 'x' from Nope select k insert into Out;\n"
		if g := strings.Join(got(t, src, Options{}), " "); g != "E-PRAG-002" {
			t.Fatalf("got [%s]", g)
		}
	})
}

// TestFindings covers each check's other triggers, one finding each.
func TestFindings(t *testing.T) {
	t.Parallel()
	pattern := func(filter, sel string) string {
		return "rule P intent 'x' partition by k of S ttl 1 hour from every a = S" + filter +
			" within 5 min after match skip past last late events drop select " + sel + " insert into Out;\n"
	}
	cases := []struct {
		name, rules, want, msg string
	}{
		{"a filter that is not boolean", "rule P intent 'x' from S[n + 1] select k insert into Out;\n", "E-TYPE-001", "boolean"},
		{"a select item of the wrong type", "rule P intent 'x' from S select n as k insert into Out;\n", "E-TYPE-001", "Out.k is string"},
		{"an int select item fits a float attribute", "stream F (x float, ts timestamp);\nrule P intent 'x' from S select n as x insert into F;\n" + plain, "", ""},
		{"an unaliased expression binds nothing", "rule P intent 'x' from S select k, n + 1 insert into Out;\n", "E-SEL-001", "add `as <attribute>`"},
		{"an attribute the target lacks", "rule P intent 'x' from S select k, n insert into Out;\n", "E-SEL-001", "Out has no attribute n"},
		{"an attribute bound twice", "rule P intent 'x' from S select k, note as k insert into Out;\n", "E-SEL-001", "bound twice"},
		{"an unknown capture qualifier, with the qualified name suggested", pattern("", "aa.k"), "E-NAME-001", "did you mean a.k?"},
		{"a bare name in a pattern select, with the capture's name suggested", pattern("", "k"), "E-NAME-001", "did you mean a.k?"},
		{"an unknown attribute of a capture", pattern("", "a.kk"), "E-NAME-001", "did you mean a.k?"},
		{"an unknown capture in chain", "stream E (k string, e object, ts timestamp);\n" + strings.Replace(pattern("", "a.k, chain(b) as e"), "into Out", "into E", 1) + plain,
			"E-NAME-001", "unknown capture b"},
		{"an unknown function, suggested", "rule P intent 'x' from S[abss(n) > 1] select k insert into Out;\n", "E-NAME-001", "did you mean abs?"},
		{"an unknown sort attribute", "rule P intent 'x' partition by k of S ttl 1 hour from S window sort(3, nn, desc) emit on change late events drop select k insert into Out;\n", "E-NAME-001", "did you mean n?"},
		{"an unknown partition stream", "rule P intent 'x' partition by k of Q ttl 1 hour from every a = S within 5 min after match skip past last late events drop select a.k insert into Out;\n", "E-NAME-001", "unknown stream Q"},
		{"an unknown partition attribute", "rule P intent 'x' partition by kk of S ttl 1 hour from every a = S within 5 min after match skip past last late events drop select a.k insert into Out;\n", "E-NAME-001", "did you mean k?"},
		{"an unknown insert target", "rule P intent 'x' from S select k insert into Outt;\n" + plain, "E-NAME-001", "did you mean Out?"},
		{"an unknown primary key column", "table U (k string) primary key (kk);\n" + plain, "E-NAME-001", "primary key column of U"},
		{"a set operand naming a two-column table", "rule P intent 'x' from S[k has_any_ci(Pair)] select k insert into Out;\n", "E-TBL-001", "one-column table"},
		{"a set operand naming a one-column table", "rule P intent 'x' from S[k has_any_ci(T)] select k insert into Out;\n", "", ""},
		{"a capture shadowing a table", "rule P intent 'x' partition by k of S ttl 1 hour from every T = S within 5 min after match skip past last late events drop select T.k insert into Out;\n", "E-NAME-002", "the table T"},
		{"a capture shadowing a param", "rule P intent 'x' partition by k of S ttl 1 hour from every lim = S within 5 min after match skip past last late events drop select lim.k insert into Out;\n", "E-NAME-002", "the param lim"},
		{"a named backreference", "rule P intent 'x' from S[k matches_cs '''(?P<x>a)\\k<x>'''] select k insert into Out;\n", "E-REGEX-001", "named backreference"},
		{"a lookbehind", "rule P intent 'x' from S[k matches_ci '''(?<=a)b'''] select k insert into Out;\n", "E-REGEX-001", "lookbehind"},
		{"an escaped backslash is no backreference", "rule P intent 'x' from S[k matches_ci '''a\\\\1'''] select k insert into Out;\n", "", ""},
		{"a regex from a param is scanned", "param re string = '(a)\\\\1';\nrule P intent 'x' from S[k contains_match_cs ${re}] select k insert into Out;\n", "E-REGEX-001", "backreference"},
		{"in over an optional attribute", "rule P intent 'x' from S[note in T] select k insert into Out;\n", "E-NULL-002", "`in` is false"},
		{"not in over an optional attribute", "rule P intent 'x' from S[note not in T] select k insert into Out;\n", "E-NULL-002", "`not in` is false"},
		{"an optional attribute tested with is null is handled", "rule P intent 'x' from S[note is null or note != 'x'] select k insert into Out;\n", "", ""},
		{"coalesce handles an optional attribute", "rule P intent 'x' from S[coalesce(note, '-') != 'x'] select k insert into Out;\n", "", ""},
		{"a time attribute that is not declared", "stream Q (k string, ts timestamp) time tz watermark ascending idle after 1 min;\nrule P intent 'x' from Q select k insert into Out;\n", "E-NAME-001", "did you mean ts?"},
		{"an arrival stream needs no time attribute", "stream Q (k string, ts timestamp) time arrival;\nrule P intent 'x' from Q select k insert into Out;\n", "", ""},
		{"a second producer", "stream D (k string, ts timestamp);\nrule A intent 'x' from S select k insert into D;\nrule B intent 'x' from R select k insert into D;\n" + plain, "E-GRAPH-003", ""},
		{"a delay window longer than the ttl", "rule P intent 'x' partition by k of S ttl 5 min from S window delay(7 min) emit on due late events drop select k insert into Out;\n", "E-TTL-002", "window extent `delay` 7 min (420000 ms)"},
		{"a zero delay", "rule P intent 'x' partition by k of S ttl 5 min from S window delay(0 ms) emit on due late events drop select k insert into Out;\n", "E-DUR-003", ""},
		{"a zero within", "rule P intent 'x' partition by k of S ttl 5 min from every a = S within 0 ms after match skip past last late events drop select a.k insert into Out;\n", "E-DUR-003", ""},
		{"a todo per clause", "rule P intent 'x' from S todo 'a' todo 'b' select k insert into Out;\n", "E-TODO-001 E-TODO-001", ""},
		{"an unknown annotation", "@sorce(type = 'stdin') stream Q (k string, ts timestamp) time ts watermark ascending idle after 1 min;\nrule P intent 'x' from Q select k insert into Out;\n", "E-NAME-001", "did you mean @source?"},
		{"a nested annotation that is not @map", "@sink(type = 'http', url = 'u', @mop(type = 'json')) stream Q (k string, ts timestamp);\nrule P intent 'x' from S select k insert into Q;\n" + plain, "E-NAME-001", "did you mean @map?"},
		{"an unknown @map key", "@sink(type = 'http', url = 'u', @map(typ = 'json')) stream Q (k string, ts timestamp);\nrule P intent 'x' from S select k insert into Q;\n" + plain, "E-NAME-001", "did you mean type?"},
		{"an unknown @map type", "@sink(type = 'http', url = 'u', @map(type = 'jsn')) stream Q (k string, ts timestamp);\nrule P intent 'x' from S select k insert into Q;\n" + plain, "E-NAME-001", "did you mean 'json'?"},
		{"an annotation value from a param", "param hook string = 'http://x';\n@sink(type = 'http', url = ${hook}) stream Q (k string, ts timestamp);\nrule P intent 'x' from S select k insert into Q;\n" + plain, "", ""},
		{"an annotation value from an undeclared param", "@sink(type = 'http', url = ${hook}) stream Q (k string, ts timestamp);\nrule P intent 'x' from S select k insert into Q;\n" + plain, "E-PARAM-001", "${hook} is not a declared param"},
		{"a non-string param in a text operand", "rule P intent 'x' from S[k has_ci ${lim}] select k insert into Out;\n", "E-PARAM-001", "takes a string"},
		{"a string param in a text operand", "param word string = 'w';\nrule P intent 'x' from S[k has_ci ${word}] select k insert into Out;\n", "", ""},
		{"a keyword-set param", "rule P intent 'x' from S[k has_any_ci(${terms})] select k insert into Out;\n", "", ""},
		{"a duration param in a duration slot", "param w duration = 5 min;\nrule P intent 'x' partition by k of S ttl 1 hour from every a = S within ${w} after match skip past last late events drop select a.k insert into Out;\n", "", ""},
		{"an undeclared param in a duration slot", "rule P intent 'x' partition by k of S ttl ${nope} from every a = S within 5 min after match skip past last late events drop select a.k insert into Out;\n", "E-PARAM-001", "${nope}"},
		{"an int default for a float param", "rule P intent 'x' from S[n > ${ratio}] select k insert into Out;\n", "", ""},
		{"a sort window into a log sink: a changelog sink", "@sink(type = 'log') stream Q (k string, ts timestamp);\nrule P intent 'x' partition by k of S ttl 1 hour from S window sort(3, n, desc) emit on change late events drop select k insert into Q;\n" + plain, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fs := findings(t, "unit.cep", base+c.rules, Options{})
			var codes []string
			for _, f := range fs {
				codes = append(codes, f.Code)
			}
			if strings.Join(codes, " ") != c.want {
				t.Fatalf("got %v, want [%s]", fs, c.want)
			}
			if c.msg != "" && !strings.Contains(fs[0].Msg, c.msg) {
				t.Fatalf("message %q lacks %q", fs[0].Msg, c.msg)
			}
		})
	}
}

// TestHarnessPorts ports the semantics harness's static checks (docs/language/checks/test_semantics.py), each
// assertion as written there.
func TestHarnessPorts(t *testing.T) {
	t.Parallel()
	ttl := func(ttl, within, rate string) string {
		r := "rule P intent 'x' partition by k of S ttl " + ttl + " from every a = S within " + within +
			" after match skip past last late events drop select a.k"
		if rate != "" {
			r += " output first every " + rate
		}
		return base + r + " insert into Out;\n"
	}
	t.Run("1.1 · RT-14: an optional attribute compared in an absence; offenders only; both fix forms", func(t *testing.T) {
		t.Parallel()
		src := "language cep/0.1;\n" +
			"stream A (agentId string, sessionId string optional, riskScore float optional, action string, ts timestamp)\n" +
			"  time ts watermark bounded 1 min idle after 1 min;\n" +
			"stream B (agentId string, sessionId string optional, riskScore float optional, action string, ts timestamp)\n" +
			"  time ts watermark bounded 1 min idle after 1 min;\n" +
			"stream Out (k string, ts timestamp);\n" +
			"rule P intent 'x' partition by agentId of A, agentId of B ttl 1 hour\n" +
			"  from every a = A and not B[sessionId == a.sessionId and action == 'approve'] for 5 min\n" +
			"  within 10 min after match skip past last late events drop select a.agentId as k insert into Out;\n"
		fs := findings(t, "rt14.cep", src, Options{})
		if len(fs) != 1 || fs[0].Code != "E-NULL-001" {
			t.Fatalf("got %v", fs)
		}
		msg := fs[0].Msg
		if !strings.Contains(msg, "sessionId") || strings.Contains(msg, "riskScore") {
			t.Fatalf("offenders only: %q", msg)
		}
		if !strings.Contains(msg, "coalesce") || !strings.Contains(msg, "is null") {
			t.Fatalf("both fix forms: %q", msg)
		}
		clean := strings.Replace(src, "sessionId == a.sessionId and ", "", 1)
		if fs := findings(t, "rt14.cep", clean, Options{}); len(fs) != 0 {
			t.Fatalf("a comparison over required attributes only: %v", fs)
		}
	})
	t.Run("9: TTL 3 min below the pattern bound 5 min is rejected, both numbers named", func(t *testing.T) {
		t.Parallel()
		fs := findings(t, "ttl.cep", ttl("3 min", "5 min", ""), Options{})
		if len(fs) != 1 || fs[0].Code != "E-TTL-002" || !strings.Contains(fs[0].Msg, "180000") || !strings.Contains(fs[0].Msg, "300000") {
			t.Fatalf("got %v", fs)
		}
		if fs := findings(t, "ttl.cep", ttl("5 min", "5 min", ""), Options{}); len(fs) != 0 {
			t.Fatalf("ttl equal to the bound: %v", fs)
		}
	})
	t.Run("9/11.3 · RT-07: TTL 5 min below the rate-limit interval 10 min, the 2 min bound unnamed", func(t *testing.T) {
		t.Parallel()
		fs := findings(t, "rt07.cep", ttl("5 min", "2 min", "10 min"), Options{})
		if len(fs) != 1 || fs[0].Code != "E-TTL-002" {
			t.Fatalf("got %v", fs)
		}
		msg := fs[0].Msg
		if !strings.Contains(msg, "300000") || !strings.Contains(msg, "600000") || !strings.Contains(msg, "rate-limit interval") ||
			strings.Contains(msg, "120000") {
			t.Fatalf("message %q", msg)
		}
		if fs := findings(t, "rt07.cep", ttl("10 min", "2 min", "10 min"), Options{}); len(fs) != 0 {
			t.Fatalf("ttl covering both: %v", fs)
		}
	})
}

// TestOverrides: an override replaces a param's value in the returned copy, never in the input; a wrong one is
// E-PARAM-001 at the declaration, an override of no param at 0:0.
func TestOverrides(t *testing.T) {
	t.Parallel()
	src := base + "rule P intent 'x' from S[n > ${lim}] select k insert into Out;\n"
	app := parse(t, src)
	before, err := app.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	out, diags := Check(app, Options{Overrides: map[string]model.Literal{"lim": {Kind: model.LitInt, Int: 9}}})
	if len(diags) != 0 || out == nil {
		t.Fatalf("a good override: %v", diags)
	}
	if lit := out.Rules[0].Body.Source.Filter.Args[1].Lit; lit.Kind != model.LitInt || lit.Int != 9 {
		t.Fatalf("the copy's slot holds %+v, not the override", lit)
	}
	after, _ := app.MarshalCanonical()
	if string(before) != string(after) {
		t.Fatal("Check wrote the App it was given")
	}
	_, diags = Check(app, Options{Overrides: map[string]model.Literal{
		"lim": {Kind: model.LitString, Str: "x"}, "nope": {Kind: model.LitInt, Int: 1},
	}})
	var got []string
	for _, d := range diags {
		got = append(got, fmt.Sprintf("%s %d:%d", d.Code, d.Line, d.Col))
	}
	if strings.Join(got, ", ") != "E-PARAM-001 0:0, E-PARAM-001 2:1" {
		t.Fatalf("got %v", diags)
	}
}

// TestALintDoesNotRefuse: a program whose only finding is E-NULL-002 deploys.
func TestALintDoesNotRefuse(t *testing.T) {
	t.Parallel()
	out, diags := Check(parse(t, base+"rule P intent 'x' from S[note != 'x'] select k insert into Out;\n"), Options{})
	if len(diags) != 1 || !diags[0].Lint || out == nil {
		t.Fatalf("got %v, app %v", diags, out != nil)
	}
}

// TestBatchOverANonPunctuatingSource proves E-WIN-002's check, unreachable with the built-in connectors (owner,
// 2026-11-04): over a planted table whose `pull` source marks no batches.
func TestBatchOverANonPunctuatingSource(t *testing.T) {
	t.Parallel()
	planted := builtin
	planted.Sources = append([]connector{{Name: "pull"}}, builtin.Sources...)
	src := base + "@source(type = 'pull') stream Q (k string, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"rule P intent 'x' partition by k of Q ttl 1 hour from Q window batch() emit on batch end late events drop select k insert into Out;\n"
	_, diags := check(parse(t, src), Options{}, planted)
	if len(diags) != 1 || diags[0].Code != "E-WIN-002" {
		t.Fatalf("got %v", diags)
	}
	if _, diags := Check(parse(t, strings.Replace(src, "'pull'", "'stdin'", 1)), Options{}); len(diags) != 0 {
		t.Fatalf("a built-in source: %v", diags)
	}
}

// TestCycle: the cycle search behind E-GRAPH-001 (unreachable in v0.1; kept for Phase 2's rule chaining).
func TestCycle(t *testing.T) {
	t.Parallel()
	rule := func(name, from, into string) model.Rule {
		return model.Rule{Name: name, Body: model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: from}}, InsertInto: into}
	}
	cases := []struct {
		name  string
		rules []model.Rule
		want  []int
	}{
		{"none", []model.Rule{rule("A", "S", "X"), rule("B", "X", "Y")}, nil},
		{"a rule reading its own output", []model.Rule{rule("A", "X", "X")}, []int{0}},
		{"two rules", []model.Rule{rule("A", "S", "X"), rule("B", "Y", "Z"), rule("C", "X", "Y"), rule("D", "Z", "S")}, []int{0, 2, 1, 3}},
	}
	for _, c := range cases {
		if got := cycle(c.rules); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := strings.Join(ruleNames(cases[2].rules, cases[2].want), " -> "); got != "A -> C -> B -> D -> A" {
		t.Errorf("names: %s", got)
	}
	// Through Check with no E-GRAPH-002 or -004 to explain it, a cycle is reported (no v0.1 program builds one):
	// streams with no time clause that nothing marks as produced cannot occur, so build the App by hand.
	c := &checker{app: &model.App{Rules: []model.Rule{rule("A", "X", "X")}}, conn: builtin, streams: map[string]*model.Stream{},
		tables: map[string]*model.Table{}, params: map[string]*model.Param{}, noTarget: map[*model.Rule]bool{},
		poisoned: map[*model.Rule]bool{}}
	c.app.Rules[0].Body.Source.Stream = "X"
	c.poisoned[&c.app.Rules[0]] = true // so no E-GRAPH-004 runs for it
	c.graph()
	if len(c.diags) != 1 || c.diags[0].Code != "E-GRAPH-001" || !strings.Contains(c.diags[0].Msg, "A -> A") {
		t.Fatalf("got %v", c.diags)
	}
}

// TestNearest: within max(1, len/3) edits, ties to the earlier candidate, nothing past the cutoff.
func TestNearest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		cands []string
		want  string
	}{
		{"colr", []string{"color", "colour"}, "color"},
		{"ab", []string{"ax", "ay"}, "ax"},
		{"total", []string{"count", "sum"}, ""},
		{"x", []string{"x", "y"}, "y"},
		{"ünïcode", []string{"unicode", "ünïcodé"}, "ünïcodé"},
	}
	for _, c := range cases {
		if got := nearest(c.name, c.cands); got != c.want {
			t.Errorf("nearest(%q, %v) = %q, want %q", c.name, c.cands, got, c.want)
		}
	}
}

// TestAGoBuiltModel: with no positions, findings sit at 0:0 and still come in one order (code, then message).
func TestAGoBuiltModel(t *testing.T) {
	t.Parallel()
	app := parse(t, base+plain)
	zero := func(v reflect.Value) {}
	_ = zero
	clearPositions(reflect.ValueOf(app).Elem())
	app.Rules[0].Todos = []string{"one", "two"}
	app.Rules[0].InsertInto = "Nope"
	_, diags := Check(app, Options{})
	var got []string
	for _, d := range diags {
		got = append(got, fmt.Sprintf("%s %d:%d %s", d.Code, d.Line, d.Col, d.Msg))
	}
	want := []string{
		"E-NAME-001 0:0 unknown stream Nope",
		"E-TIME-001 0:0 no rule produces Out, so it is a source stream, and it has no time clause",
		"E-TODO-001 0:0 rule P holds an unresolved todo",
		"E-TODO-001 0:0 rule P holds an unresolved todo",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func clearPositions(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == reflect.TypeFor[model.Pos]() {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		for i := range v.NumField() {
			clearPositions(v.Field(i))
		}
	case reflect.Slice:
		if v.Type() == reflect.TypeFor[[]model.Pos]() {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		for i := range v.Len() {
			clearPositions(v.Index(i))
		}
	}
}

// TestHuman writes durations as the language does.
func TestHuman(t *testing.T) {
	t.Parallel()
	for ms, want := range map[int64]string{0: "0 ms", 250: "250 ms", 90_000: "1 min 30 sec", 694_800_000: "1 week 1 day 1 hour"} {
		if got := human(ms); got != want {
			t.Errorf("human(%d) = %q, want %q", ms, got, want)
		}
	}
}
