package parser_test

// S3.2b-2's exit 10, the parser's half (design note docs/sprints/s3-2b-2-design.md v1, §1.2, §2.1, §2.2): a grouped
// partition key `(k, j, …) of S` lowers to its attributes in written order, each with its own position, the key's
// Pos at `(` and Grouped set; a single key keeps one attribute, Grouped false, Pos at the attribute. The grammar
// accepts a one-attribute group on purpose (the validator answers it with E-PART-005), and a backticked name inside a
// group. Red on the stub: the parser accepts only `k of S`.

import (
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

func TestGroupedPartitionKeysLower(t *testing.T) {
	t.Parallel()
	src := "language cep/0.1;\n" +
		"stream A (tenantId string, userId string, region string, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"stream B (t string, u string, r string, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"stream Out (n int, ts timestamp);\n" +
		"rule R intent 'x'\n" +
		"  partition by (tenantId, `userId`, region) of A, (t,u, r) of B ttl 1 hour\n" +
		"  from every a = A -> b = B within 5 min after match skip past last late events drop select 1 as n insert into Out;\n" +
		"rule S1 intent 'x' partition by tenantId of A ttl 1 hour from A select 1 as n insert into Out;\n" +
		"rule S2 intent 'x' partition by (tenantId) of A ttl 1 hour from A select 1 as n insert into Out;\n"
	res, err := parser.Parse("grouped.cep", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	at := func(line, col int) model.Pos { return model.Pos{Line: line, Col: col} }
	type attr struct {
		name string
		pos  model.Pos
	}
	cases := []struct {
		name      string
		key       model.PartitionKey
		attrs     []attr
		grouped   bool
		pos       model.Pos
		stream    string
		streamPos model.Pos
	}{
		// line 6: "  partition by (tenantId, `userId`, region) of A, (t,u, r) of B ttl 1 hour"
		{"a three-part group with a backticked name", res.App.Rules[0].Partition.Keys[0],
			[]attr{{"tenantId", at(6, 17)}, {"userId", at(6, 27)} /* the backtick */, {"region", at(6, 37)}}, true, at(6, 16), "A", at(6, 48)},
		{"a group written without a space after a comma", res.App.Rules[0].Partition.Keys[1],
			[]attr{{"t", at(6, 52)}, {"u", at(6, 54)}, {"r", at(6, 57)}}, true, at(6, 51), "B", at(6, 63)},
		// line 8: "rule S1 intent 'x' partition by tenantId of A ttl 1 hour …"
		{"a single key", res.App.Rules[1].Partition.Keys[0], []attr{{"tenantId", at(8, 33)}}, false, at(8, 33), "A", at(8, 45)},
		// line 9: "rule S2 intent 'x' partition by (tenantId) of A ttl 1 hour …": parsed, for E-PART-005
		{"a one-attribute group", res.App.Rules[2].Partition.Keys[0], []attr{{"tenantId", at(9, 34)}}, true, at(9, 33), "A", at(9, 47)},
	}
	for _, c := range cases {
		k := c.key
		var got []attr
		for _, a := range k.Attrs {
			got = append(got, attr{a.Name, a.Pos})
		}
		if len(got) != len(c.attrs) {
			t.Errorf("%s: attributes %v; want %v", c.name, got, c.attrs)
			continue
		}
		for i := range got {
			if got[i] != c.attrs[i] {
				t.Errorf("%s: part %d is %v; want %v", c.name, i+1, got[i], c.attrs[i])
			}
		}
		if k.Grouped != c.grouped || k.Pos != c.pos || k.Stream != c.stream || k.StreamPos != c.streamPos {
			t.Errorf("%s: Grouped %v, Pos %v, Stream %s at %v; want %v, %v, %s at %v", c.name, k.Grouped, k.Pos, k.Stream, k.StreamPos, c.grouped, c.pos, c.stream, c.streamPos)
		}
	}
}
