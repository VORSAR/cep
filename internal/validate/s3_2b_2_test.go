package validate

// S3.2b-2's exit 10 (design note docs/sprints/s3-2b-2-design.md v1, §1.1, §1.3, §5; owner, S3.2b-2's plan check Q3:
// "grouped partition keys have one spelling ... `(k) of S`, an attribute named twice in one group, and a
// part-count mismatch between streams are each a deploy error whose message shows the valid form"). The counter-files
// in testdata/ prove each code at its position with errors.md's fix (TestStatic); these pin the messages of design
// §1.3's "exact shape" column, the order of the checks, worked examples 3 and 4, and the valid forms. Red on the stub:
// the parser accepts only `k of S`.

import (
	"strings"
	"testing"
)

// gkPlain is a one-stream rule keyed by keys (a plain body, as example 07).
func gkPlain(keys string) string {
	return "language cep/0.1;\n" +
		"stream S (k string, j string, ts timestamp) time ts watermark bounded 1 min;\n" +
		"stream Out (k string, ts timestamp);\n" +
		"rule R intent 'x' partition by " + keys + " ttl 10 min from S select k output first every 5 min insert into Out;\n"
}

// gkTwo is a two-stream pattern rule over A and B keyed by keys; bTypes are B's two attributes' types.
func gkTwo(aAttrs, bAttrs, keys string) string {
	return "language cep/0.1;\n" +
		"stream A (" + aAttrs + ", ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"stream B (" + bAttrs + ", ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
		"stream Out (n int, ts timestamp);\n" +
		"rule R intent 'x' partition by " + keys + " ttl 1 hour\n" +
		"  from every a = A -> b = B within 5 min after match skip past last late events drop select 1 as n insert into Out;\n"
}

// gkCol is the 1-based column of the n-th (1-based) occurrence of tok on line 4 or 5 of src (the rule's partition).
func gkCol(t *testing.T, src string, line int, tok string, n int) int {
	t.Helper()
	l := strings.Split(src, "\n")[line-1]
	at := -1
	for range n {
		next := strings.Index(l[at+1:], tok)
		if next < 0 {
			t.Fatalf("no occurrence %d of %q on line %d: %s", n, tok, line, l)
		}
		at += 1 + next
	}
	return at + 1
}

// TestGroupedKeyFindings: each wrong spelling and mismatch gives exactly one finding — its code at its position with
// design §1.3's message and fix, the fix showing the valid form.
func TestGroupedKeyFindings(t *testing.T) {
	t.Parallel()
	ex4A, ex4B := "agentId string, attempt int", "agent string, n int"
	cases := []struct {
		name, src, code string
		line            int
		tok             string // the finding's token on that line
		nth             int    // its occurrence
		msg, fix        string
	}{
		{"a single key in brackets", gkPlain("(k) of S"), "E-PART-005", 4, "(k)", 1,
			"the partition key of S in rule R is one attribute in brackets: write k of S",
			"write `k of S`; brackets group two or more attributes: `(k, j) of S`."},
		{"an attribute named twice in one group", gkPlain("(k, j, k) of S"), "E-PART-006", 4, "k", 2,
			"k is named twice in the partition key of S in rule R: name each attribute once",
			"name each attribute once: `(k, j) of S`."},
		{"a stream keyed twice", gkPlain("k of S, j of S"), "E-PART-004", 4, "j of S", 1,
			"S is keyed twice in the partition of rule R: to key it on both, write (k, j) of S, and give every stream the same number of parts",
			"key each stream once; group its attributes, `(k, j) of S`, and give every stream's key the same number of parts."},
		{"part counts differ", gkTwo("u string, s string", "u string, s string", "(u, s) of A, u of B"), "E-PART-007", 5, "partition", 1,
			"the partition keys of rule R have different numbers of parts: A 2, B 1: give every stream the same number of parts",
			"give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`."},
		{"part counts differ and types differ: only the count", gkTwo("u string, s string", "u int, s int", "(u, s) of A, u of B"), "E-PART-007", 5, "partition", 1,
			"the partition keys of rule R have different numbers of parts: A 2, B 1: give every stream the same number of parts",
			"give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`."},
		{"types differ at part 2", gkTwo("user string, device string", "user string, device int", "(user, device) of A, (user, device) of B"), "E-PART-003", 5, "device", 2,
			"part 2 of the partition key: device of B is int, but device of A is string",
			"key each position with attributes of one type, or declare them with one type."},
		// Worked example 4's invalid half: B's parts written in the other order — E-PART-003 at B's first part.
		{"worked example 4, crossed types", gkTwo(ex4A, ex4B, "(agentId, attempt) of A, (n, agent) of B"), "E-PART-003", 5, "n, agent", 1,
			"part 1 of the partition key: n of B is int, but agentId of A is string",
			"key each position with attributes of one type, or declare them with one type."},
		// v2 (review F12): the part count before the bracket spelling — one finding, E-PART-007, whose fix settles both.
		{"a single key in brackets beside a two-part key", gkTwo("u string, s string", "u string, s string", "(u, s) of A, (u) of B"), "E-PART-007", 5, "partition", 1,
			"the partition keys of rule R have different numbers of parts: A 2, B 1: give every stream the same number of parts",
			"give every stream's key the same number of parts, matched by position: `(k, j) of S, (k2, j2) of T`."},
		// An unknown attribute in a group: E-NAME-001 at it, and the key is not reported again (badKey).
		{"an unknown attribute in a group", gkPlain("(k, zz) of S"), "E-NAME-001", 4, "zz", 1, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := findings(t, c.name, c.src, Options{})
			col := gkCol(t, c.src, c.line, c.tok, c.nth)
			if len(got) != 1 {
				t.Fatalf("want exactly one finding, %s at %d:%d; got %d: %v", c.code, c.line, col, len(got), got)
			}
			f := got[0]
			if f.Code != c.code || f.Line != c.line || f.Col != col {
				t.Fatalf("want %s at %d:%d; got %s", c.code, c.line, col, f)
			}
			if c.msg != "" && !strings.Contains(f.Msg, c.msg) {
				t.Errorf("message %q; want it to read %q (design §1.3)", f.Msg, c.msg)
			}
			if c.fix != "" && f.Fix != c.fix {
				t.Errorf("fix %q; want %q (design §1.3)", f.Fix, c.fix)
			}
		})
	}
}

// TestGroupedKeysCheckClean: the valid forms raise nothing, lints included — two- and three-part keys; worked
// example 3 (request/response with crossed names: §1.5 adds no rule against it); worked example 4's valid half
// (names differ, types match by position); E-NULL-003's exemption on every part (an optional key part selected into
// a non-optional attribute, design §1.3); and a read of an undeclared text_invisibles (S3.2b's plan check Q3:
// "text_invisibles can be read on every stream").
func TestGroupedKeysCheckClean(t *testing.T) {
	t.Parallel()
	src := `language cep/0.1;

stream Grants (tenantId string, userId string optional, region string, note string, ts timestamp)
  time ts watermark bounded 1 min;

stream Requests (src string, dst string, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;

stream Responses (src string, dst string, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;

stream A (agentId string, attempt int, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;

stream B (agent string, n int, ts timestamp)
  time ts watermark bounded 1 min idle after 1 min;

stream PerPairOut (tenantId string, userId string, ts timestamp);
stream PerTripleOut (tenantId string, ts timestamp);
stream Paired (src string, ts timestamp);
stream Agents (agentId string, ts timestamp);
stream Hidden (tenantId string, ts timestamp);

rule PerPair
  intent 'At most one alert per tenant-and-user pair per 5 minutes.'
  partition by (tenantId, userId) of Grants ttl 10 min
  from Grants
  select tenantId, userId
  output first every 5 min
  insert into PerPairOut;

rule PerTriple
  intent 'At most one alert per tenant, user and region per 5 minutes.'
  partition by (tenantId, ` + "`userId`" + `, region) of Grants ttl 10 min
  from Grants
  select tenantId
  output first every 5 min
  insert into PerTripleOut;

rule Pairing
  intent 'A request, then its response.'
  partition by (src, dst) of Requests, (dst, src) of Responses ttl 1 hour
  from every q = Requests -> r = Responses
  within 5 min
  after match skip past last
  late events drop
  select q.src
  insert into Paired;

rule ByPosition
  intent 'An attempt, then a matching one on B.'
  partition by (agentId, attempt) of A, (agent, n) of B ttl 1 hour
  from every a = A -> b = B
  within 5 min
  after match skip past last
  late events drop
  select a.agentId
  insert into Agents;

rule Invisible
  intent 'Grants whose text carries invisible characters.'
  from Grants[text_invisibles]
  select tenantId
  insert into Hidden;
`
	if got := findings(t, "grouped-clean.cep", src, Options{}); len(got) != 0 {
		t.Fatalf("a valid program raised %v", got)
	}
}
