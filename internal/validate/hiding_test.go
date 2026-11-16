package validate

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
)

// posOf is where the nth occurrence of tok in src starts, plus off bytes, as line:col (both from 1).
func posOf(t *testing.T, src, tok string, nth, off int) string {
	t.Helper()
	i := -1
	for range nth {
		j := strings.Index(src[i+1:], tok)
		if j < 0 {
			t.Fatalf("occurrence %d of %q is not in the source", nth, tok)
		}
		i += 1 + j
	}
	i += off
	line := 1 + strings.Count(src[:i], "\n")
	return fmt.Sprintf("%d:%d", line, i-strings.LastIndex(src[:i], "\n"))
}

// codesAt is every finding as code@line:col, in the order check returns them (sorted by position, code, message).
func codesAt(diags []Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, fmt.Sprintf("%s@%d:%d", d.Code, d.Line, d.Col))
	}
	return out
}

// TestOneFindingHidesAnother: in each program one finding suppresses, or stands before, a second fault's (design s1-1c-6
// §5 §0, §3.3). check sorts every finding, so each row is the exact sorted list of code@line:col.
func TestOneFindingHidesAnother(t *testing.T) {
	t.Parallel()
	type at struct {
		code, tok string
		nth, off  int
	}
	pattern := func(part, sel string) string {
		return "rule P intent 'x' partition by " + part + " ttl 1 hour from every a = S -> b = R" + patTail + sel +
			" insert into Out;\n"
	}
	override := Options{Overrides: map[string]model.Literal{"p": {Kind: model.LitString, Str: "x"}}}
	cases := []struct {
		name, rules string
		opts        Options
		want        []at
	}{
		{name: "H1 an unknown partition attribute: no E-PART-002 for the stream it may have meant",
			rules: pattern("nope of S", "a.k"), want: []at{{"E-NAME-001", "nope", 1, 0}}},
		{name: "H2 part counts differ: no type finding (the design's spelling)",
			rules: pattern("(k, n) of S, (k) of R", "a.k"), want: []at{{"E-PART-007", "partition", 1, 0}}},
		{name: "H2 part counts differ: no bracket finding for the single key in brackets",
			rules: pattern("(k) of S, (k, m) of R", "a.k"), want: []at{{"E-PART-007", "partition", 1, 0}}},
		{name: "H3 count() in a filter: no type finding for its comparison",
			rules: "rule P intent 'x' from S[count() > 'a'] select k insert into Out;\n",
			want:  []at{{"E-FN-001", "count", 1, 0}}},
		{name: "H5 a text operator against a two-column table: its siblings are not checked",
			rules: "rule P intent 'x' from S[k has_any_ci(Pair) or count() > 1] select k insert into Out;\n",
			want:  []at{{"E-TBL-001", "(Pair)", 1, 1}}},
		{name: "H6 chain() outside a pattern select: no finding for its argument",
			rules: "rule P intent 'x' from S select chain(k) as k insert into Out;\n",
			want:  []at{{"E-FN-001", "chain", 1, 0}}},
		{name: "H7 a bare name a capture has: one finding, the hint",
			rules: "rule P intent 'x' partition by k of S ttl 1 hour from every a = S" + patTail + "k insert into Out;\n",
			want:  []at{{"E-NAME-001", "k insert", 1, 0}}},
		{name: "H8 an item that does not type: no unbound-attribute finding",
			rules: "rule P intent 'x' from S select k + 1 as k insert into Out;\n",
			want:  []at{{"E-TYPE-001", "k + 1", 1, 0}}},
		{name: "H9 an attribute bound twice: no type finding for the second item",
			rules: "rule P intent 'x' from S select k, n as k insert into Out;\n",
			want:  []at{{"E-SEL-001", "n as k", 1, 0}}},
		{name: "H10 an undeclared param in a select item: no placement finding",
			rules: "rule P intent 'x' from S select count() + ${nope} as k insert into Out;\n",
			want:  []at{{"E-PARAM-001", "${nope}", 1, 0}}},
		{name: "H11 a written source and a derived read: both found",
			rules: "stream Q (k string, ts timestamp)" + clocked + "stream D (k string, ts timestamp);\n" +
				"rule A intent 'x' from S select k insert into Q;\nrule B intent 'x' from Q select k insert into D;\n" +
				"rule C intent 'x' from D select k insert into Out;\n",
			want: []at{{"E-GRAPH-002", "into Q;", 1, 5}, {"E-GRAPH-004", "from D", 1, 5}}},
		{name: "H12 an unknown target: its expect block is not checked",
			rules: "rule R intent 'x' from S expect given S [ {k: 'a', n: 1, ts: " + given + "} ] then Nope [ {k: 'a'} ]" +
				" select k insert into Nope;\n" + plain,
			want: []at{{"E-NAME-001", "Nope;", 1, 0}}},
		{name: "H13 an unknown sort attribute: the rule's filter is not checked",
			rules: "rule P intent 'x' partition by k of S ttl 1 hour from S[n + 1] window sort(3, nn, desc) emit on change" +
				" late events drop select k insert into Out;\n",
			want: []at{{"E-NAME-001", "nn,", 1, 0}}},
		{name: "H14 a bad override: reported at the param, its slot not typed with the default",
			rules: "param p int = 5;\nrule P intent 'x' from S[k == ${p}] select k insert into Out;\n", opts: override,
			want: []at{{"E-PARAM-001", "param p", 1, 0}}},
		{name: "H15 an undeclared param in a text operand: one finding",
			rules: "rule P intent 'x' from S[k has_ci ${nope}] select k insert into Out;\n",
			want:  []at{{"E-PARAM-001", "${nope}", 1, 0}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := base + c.rules
			want := make([]string, 0, len(c.want))
			for _, w := range c.want {
				want = append(want, w.code+"@"+posOf(t, src, w.tok, w.nth, w.off))
			}
			_, diags := Check(parse(t, src), c.opts)
			if got := codesAt(diags); strings.Join(got, " ") != strings.Join(want, " ") {
				t.Fatalf("got %v, want %v\n%v", got, want, diags)
			}
		})
	}
	// H17: a model built in Go may leave a window on a plain body; nothing reads it (its emit, its sort attribute, its
	// kind against a sink, its aggregation).
	goBuilt := []struct {
		name string
		w    model.Window
	}{
		{"H17 a sort window emitting on due, over an unknown sort attribute", model.Window{Kind: model.WindowSort, Emit: "due",
			Source: model.Source{Stream: "S"}, SortAttr: "nope"}},
		{"H17 a batch window", model.Window{Kind: model.WindowBatch, Emit: "batch end", Source: model.Source{Stream: "S"}}},
	}
	for _, g := range goBuilt {
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()
			app := parse(t, base+"@sink(type = 'http', url = 'u') stream H (k string, ts timestamp);\n"+
				"rule W intent 'x' from S select k insert into H;\n"+plain)
			app.Rules[0].Body.Window = g.w
			if _, diags := Check(app, Options{}); len(diags) != 0 {
				t.Fatalf("got %v, want none", diags)
			}
		})
	}
}
