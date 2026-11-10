package parser_test

// Design note docs/sprints/s3-3-2-design.md v2, §2: positions on expect blocks, in the model — Expect.Pos (`expect`),
// Given.Pos and NamePos, Then.Pos and NamePos, Record.Pos (`{`), Field.Pos (its name) and ValuePos (the value's first
// token: `-`, the literal, or `null`). Example 07's first block, and a hand-written block with a null and a negative
// number. Red until the lowering fills them (zero today).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

type epPos = model.Pos

func epExpect(t *testing.T, name, src string) model.Expect {
	t.Helper()
	res, err := parser.Parse(name, src)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	if len(res.App.Rules) == 0 || len(res.App.Rules[0].Expects) == 0 {
		t.Fatalf("%s: no expect block in rule 1", name)
	}
	return res.App.Rules[0].Expects[0]
}

func epCheck(t *testing.T, what string, got, want epPos) {
	t.Helper()
	if got != want {
		t.Errorf("%s at %d:%d; want %d:%d", what, got.Line, got.Col, want.Line, want.Col)
	}
}

func TestExpectPositions(t *testing.T) {
	t.Run("07 block 1", func(t *testing.T) {
		src, err := os.ReadFile(filepath.Join("../../docs/language/examples", "07-consent-gate.cep"))
		if err != nil {
			t.Fatal(err)
		}
		e := epExpect(t, "07-consent-gate.cep", string(src))
		if len(e.Givens) != 2 || len(e.Givens[0].Records) != 1 || len(e.Givens[1].Records) != 1 || len(e.Then.Records) != 1 {
			t.Fatalf("07 block 1: %+v; want two givens and a then, one record each", e)
		}
		epCheck(t, "expect", e.Pos, epPos{Line: 17, Col: 3})
		g0, g1 := e.Givens[0], e.Givens[1]
		epCheck(t, "given (table)", g0.Pos, epPos{Line: 17, Col: 10})
		epCheck(t, "RegisteredApps", g0.NamePos, epPos{Line: 17, Col: 22})
		epCheck(t, "the table row's {", g0.Records[0].Pos, epPos{Line: 17, Col: 39})
		epCheck(t, "appId", g0.Records[0].Fields[0].Pos, epPos{Line: 17, Col: 40})
		epCheck(t, "'crm'", g0.Records[0].Fields[0].ValuePos, epPos{Line: 17, Col: 47})
		epCheck(t, "given", g1.Pos, epPos{Line: 18, Col: 10})
		epCheck(t, "ConsentGrants", g1.NamePos, epPos{Line: 18, Col: 16})
		epCheck(t, "the given record's {", g1.Records[0].Pos, epPos{Line: 18, Col: 32})
		epCheck(t, "tenantId", g1.Records[0].Fields[0].Pos, epPos{Line: 18, Col: 33})
		epCheck(t, "'t1'", g1.Records[0].Fields[0].ValuePos, epPos{Line: 18, Col: 43})
		epCheck(t, "ts", g1.Records[0].Fields[3].Pos, epPos{Line: 18, Col: 85})
		epCheck(t, "timestamp '…'", g1.Records[0].Fields[3].ValuePos, epPos{Line: 18, Col: 89})
		epCheck(t, "then", e.Then.Pos, epPos{Line: 19, Col: 10})
		epCheck(t, "ConsentAlerts", e.Then.NamePos, epPos{Line: 19, Col: 15})
		epCheck(t, "the then-record's {", e.Then.Records[0].Pos, epPos{Line: 19, Col: 31})
		epCheck(t, "tenantId (then)", e.Then.Records[0].Fields[0].Pos, epPos{Line: 19, Col: 32})
	})
	t.Run("a null and a negative number", func(t *testing.T) {
		const src = `language cep/0.1;

stream S (k string optional, n int, ts timestamp)
  time ts watermark bounded 1 min;

stream Out (k string optional, n int, ts timestamp);

rule R
  intent 'Forward every event.'
  from S
  expect given S [ {k: null, n: -3, ts: timestamp '2026-01-01T10:00:00Z'} ]
         then Out [ {n: -3} ]
  select k, n
  insert into Out;
`
		e := epExpect(t, "hand.cep", src)
		if len(e.Givens) != 1 || len(e.Givens[0].Records) != 1 || len(e.Givens[0].Records[0].Fields) != 3 || len(e.Then.Records) != 1 {
			t.Fatalf("the block: %+v; want one given of one record of three fields, a then of one record", e)
		}
		f := e.Givens[0].Records[0].Fields
		epCheck(t, "expect", e.Pos, epPos{Line: 11, Col: 3})
		epCheck(t, "given", e.Givens[0].Pos, epPos{Line: 11, Col: 10})
		epCheck(t, "S", e.Givens[0].NamePos, epPos{Line: 11, Col: 16})
		epCheck(t, "{", e.Givens[0].Records[0].Pos, epPos{Line: 11, Col: 20})
		epCheck(t, "k", f[0].Pos, epPos{Line: 11, Col: 21})
		epCheck(t, "null", f[0].ValuePos, epPos{Line: 11, Col: 24})
		epCheck(t, "n", f[1].Pos, epPos{Line: 11, Col: 30})
		epCheck(t, "-3 (its -)", f[1].ValuePos, epPos{Line: 11, Col: 33})
		epCheck(t, "ts", f[2].Pos, epPos{Line: 11, Col: 37})
		epCheck(t, "timestamp", f[2].ValuePos, epPos{Line: 11, Col: 41})
		epCheck(t, "then", e.Then.Pos, epPos{Line: 12, Col: 10})
		epCheck(t, "Out", e.Then.NamePos, epPos{Line: 12, Col: 15})
		epCheck(t, "then {", e.Then.Records[0].Pos, epPos{Line: 12, Col: 21})
		epCheck(t, "n (then)", e.Then.Records[0].Fields[0].Pos, epPos{Line: 12, Col: 22})
		epCheck(t, "-3 (then, its -)", e.Then.Records[0].Fields[0].ValuePos, epPos{Line: 12, Col: 25})
	})
}
