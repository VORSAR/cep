package validate

import (
	"strings"
	"testing"
)

// TestFixRoundS3_2b_1: the validator's part of S3.2b-1's fix round.
//   - B1 (break-it, owner 2026-11-06): one stream keyed twice in a partition clause is E-PART-004 — the planner
//     carries one key value per stream, so the second key was silently dropped and alerts were lost; multi-part keys
//     come as `(k, j) of S` in S3.2b-2.
//   - L1 (break-it): a rule's own partition-key attribute is never null when the rule runs (a null key never reaches
//     it, SEMANTICS §1.1, §9), so it is no E-NULL-003 even when declared optional (owner D4: only what can be null).
func TestFixRoundS3_2b_1(t *testing.T) {
	t.Parallel()
	const s3 = "stream S3 (k string optional, n string optional, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\nstream O9 (k string, ts timestamp);\n"
	pattern := func(sel string) string {
		return "rule Q intent 'x' partition by k of S3 ttl 1 hour from every a = S3 within 5 min after match skip past last late events drop select " + sel + " insert into O9;\n"
	}
	cases := []struct {
		name, rules, want, msg string
	}{
		{"a stream keyed twice", "rule P intent 'x' partition by k of S, note of S ttl 10 min from S select k insert into Out;\n", "E-PART-004", "S is keyed twice"},
		{"a stream keyed twice by one attribute", "rule P intent 'x' partition by k of S, k of S ttl 10 min from S select k insert into Out;\n", "E-PART-004", ""},
		{"two streams keyed once each", "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = S -> b = R within 5 min after match skip past last late events drop select a.k as k insert into Out;\n", "", ""},
		{"the rule's own optional key, selected", s3 + "rule P intent 'x' partition by k of S3 ttl 10 min from S3 select k insert into O9;\n" + plain, "", ""},
		{"a capture's optional key, selected", s3 + pattern("a.k as k") + plain, "", ""},
		{"another optional attribute is still refused", s3 + "rule P intent 'x' partition by k of S3 ttl 10 min from S3 select n as k insert into O9;\n" + plain, "E-NULL-003", "n can be null"},
		{"an optional key of a key-less rule is still refused", s3 + "rule P intent 'x' from S3 select k insert into O9;\n" + plain, "E-NULL-003", "k can be null"},
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
