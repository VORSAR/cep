package validate

import (
	"strings"
	"testing"
)

// TestOutputAttributes: what an output stream's attributes may receive (owner D3 and D4 of S3.2b-1, design v2 §1).
// D3: a derived stream's `ts` holds the output timestamp, so it is declared `timestamp` (E-TYPE-001). D4: a select
// item that can be null is never bound to a non-optional attribute (E-NULL-003); "can be null" follows SEMANTICS §1.1
// and each function's own null rule in the registry.
func TestOutputAttributes(t *testing.T) {
	t.Parallel()
	const s2 = "stream S2 (k string, m int optional, b bool optional, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n"
	const o2 = "stream O2 (note string, ts timestamp);\n"
	const om = "stream OM (m int, ts timestamp);\n"
	const ob = "stream OB (b bool, ts timestamp);\n"
	pattern := func(sel, into string) string {
		return "rule Q intent 'x' partition by k of S ttl 1 hour from every a = S within 5 min after match skip past last late events drop select " +
			sel + " insert into " + into + ";\n"
	}
	cases := []struct {
		name, rules, want, msg string
	}{
		// D4: can be null, into a non-optional attribute.
		{"an optional attribute", o2 + "rule P intent 'x' from S select note insert into O2;\n" + plain, "E-NULL-003", "note can be null"},
		{"arithmetic over an optional int", s2 + om + "rule P intent 'x' from S2 select m + 1 as m insert into OM;\n" + plain, "E-NULL-003", "OM.m is not optional"},
		{"a negated optional int", s2 + om + "rule P intent 'x' from S2 select -m as m insert into OM;\n" + plain, "E-NULL-003", ""},
		{"maximum with an optional argument", s2 + om + "rule P intent 'x' from S2 select maximum(m, 1) as m insert into OM;\n" + plain, "E-NULL-003", ""},
		{"abs of an optional int", s2 + om + "rule P intent 'x' from S2 select abs(m) as m insert into OM;\n" + plain, "E-NULL-003", ""},
		{"ifThenElse with a nullable branch", o2 + "rule P intent 'x' from S select ifThenElse(n > 0, note, 'x') as note insert into O2;\n" + plain, "E-NULL-003", ""},
		{"coalesce of two nullables", o2 + "rule P intent 'x' from S select coalesce(note, note) as note insert into O2;\n" + plain, "E-NULL-003", ""},
		{"a pattern capture's optional attribute", o2 + pattern("a.note as note", "O2") + plain, "E-NULL-003", "a.note can be null"},
		{"an optional bool passed through", s2 + ob + "rule P intent 'x' from S2 select b insert into OB;\n" + plain, "E-NULL-003", ""},
		// D4: never null, or into an optional attribute.
		{"coalesce with a non-null fallback", o2 + "rule P intent 'x' from S select coalesce(note, 'none') as note insert into O2;\n" + plain, "", ""},
		{"into an optional attribute", "stream O3 (note string optional, ts timestamp);\nrule P intent 'x' from S select note insert into O3;\n" + plain, "", ""},
		{"a literal", o2 + "rule P intent 'x' from S select 'x' as note insert into O2;\n" + plain, "", ""},
		{"a parameter", o2 + "param word string = 'w';\nrule P intent 'x' from S select ${word} as note insert into O2;\n" + plain, "", ""},
		{"ifThenElse with non-null branches over a nullable test", o2 + "rule P intent 'x' from S select ifThenElse(note is null, 'a', 'b') as note insert into O2;\n" + plain, "", ""},
		{"a comparison over an optional attribute", ob + "rule P intent 'x' from S select note == 'x' as b insert into OB;\n" + plain, "", ""},
		{"not over an optional bool", s2 + ob + "rule P intent 'x' from S2 select not b as b insert into OB;\n" + plain, "", ""},
		{"and over an optional bool", s2 + ob + "rule P intent 'x' from S2 select b and k == 'x' as b insert into OB;\n" + plain, "", ""},
		{"is null over an optional attribute", ob + "rule P intent 'x' from S select note is null as b insert into OB;\n" + plain, "", ""},
		{"membership over an optional attribute", ob + "rule P intent 'x' from S select coalesce(note, '') in T as b insert into OB;\n" + plain, "", ""},
		{"a non-optional attribute", plain, "", ""},
		// D3: a derived stream's ts.
		{"a derived ts declared a string", "stream O6 (k string, ts string);\nrule P intent 'x' from S select k insert into O6;\n" + plain, "E-TYPE-001", "O6.ts is filled with the output timestamp"},
		{"a derived ts declared an int", "stream O6 (k string, ts int);\nrule P intent 'x' from S select k insert into O6;\n" + plain, "E-TYPE-001", "declare it timestamp"},
		{"a derived stream with no ts", "stream O7 (k string);\nrule P intent 'x' from S select k insert into O7;\n" + plain, "", ""},
		{"a source's ts of another type", "stream Q (k string, ts string, at timestamp) time at watermark ascending idle after 1 min;\nrule P intent 'x' from Q select k insert into Out;\n", "", ""},
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
