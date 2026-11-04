package parser_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/parser"
)

// The [static] codes the lowering raises (S2.2b): facts about a token's text that the model cannot hold. Each
// trigger gives its code at the token named, with the catalogue's fix text.
func TestLoweringCodes(t *testing.T) {
	t.Parallel()
	program := func(window, filter, pre, post string) string {
		return "language cep/0.1;\n" +
			"stream S (k string, n int, ts timestamp) time ts watermark bounded 1 min idle after 1 min;\n" +
			"stream Out (k string, ts timestamp);\n" +
			"rule R intent 'x' partition by k of S ttl 1 hour from S" + filter + window + pre + " select k" + post + " insert into Out;\n"
	}
	sort := func(n string) string {
		return program(" window sort("+n+", n, desc) emit on change late events drop", "", "", "")
	}
	text := func(v string) string { return program("", "[k has_cs "+v+"]", "", "") }
	rate := func(d string) string { return program("", "", "", " output first every "+d) }
	cases := []struct {
		name, src, code, token string // the finding is on line 4, at token's first rune
	}{
		{"a fractional count, though whole in ms", rate("1.5 min"), "E-DUR-002", "1.5 min"},
		{"an exponent count", rate("1e3 ms"), "E-DUR-002", "1e3 ms"},
		{"units not descending", rate("30 sec 1 min"), "E-DUR-002", "30 sec"},
		{"a repeated unit", rate("5 min 5 min"), "E-DUR-002", "5 min 5 min"},
		{"a duration too long for the model", rate("99999999999999 week"), "E-DUR-002", "99999999999999"},
		{"a fractional sort size", sort("1.5"), "E-WIN-004", "1.5"},
		{"a zero sort size", sort("0"), "E-WIN-004", "0,"},
		{"a sort size past int64", sort("99999999999999999999"), "E-WIN-004", "99999999999999999999"},
		{"an unknown escape", text(`'C:\q'`), "E-STR-002", `'C:\q'`},
		{`a short \u escape`, text(`'\u12'`), "E-STR-002", `'\u12'`},
		{"an unknown escape in a set item", program("", `[k has_any_cs('a', 'b\q')]`, "", ""), "E-STR-002", `'b\q'`},
		{"an unknown escape in the intent", strings.Replace(program("", "", "", ""), "intent 'x'", `intent 'a\q'`, 1), "E-STR-002", `'a\q'`},
		{"an unknown escape in a todo", program("", "", ` todo '\q'`, ""), "E-STR-002", `'\q'`},
		{"an unknown escape in a cron string", program(` window cron('0 * * * *', 'UTC\q') emit on close late events drop`, "", "", ""), "E-STR-002", `'UTC\q'`},
		{"an integer out of range", program("", "[n > 99999999999999999999]", "", ""), "E-TYPE-001", "99999999999999999999"},
		{"a timestamp that is not UTC", program("", "[ts > timestamp '2026-01-01T00:00:00+01:00']", "", ""), "E-TYPE-001", "timestamp '"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			line := strings.Split(c.src, "\n")[3]
			i := strings.Index(line, c.token)
			if i < 0 {
				t.Fatalf("test setup: %q is not on line 4", c.token)
			}
			col := len([]rune(line[:i])) + 1
			_, err := parser.Parse("lowering.cep", c.src)
			var perr *parser.Error
			if !errors.As(err, &perr) {
				t.Fatalf("want %s at 4:%d, got %v", c.code, col, err)
			}
			if perr.Code != c.code || perr.Line != 4 || perr.Col != col || perr.Fix == "" {
				t.Fatalf("want %s at 4:%d with a fix, got %v", c.code, col, err)
			}
		})
	}
	for _, ok := range []string{text(`'\u0041\t\\\'\"'`), text(`'''C:\q'''`), sort("3"), rate("1 hour 30 min")} {
		if _, err := parser.Parse("lowering.cep", ok); err != nil {
			t.Errorf("a valid program was refused: %v", err)
		}
	}
}

// TestPositionsReachTheModel: the lowering copies each node's first token into the model (S2.2b, ADR-0003 §1 as
// annotated); a few anchors spot-checked.
func TestPositionsReachTheModel(t *testing.T) {
	t.Parallel()
	src := "language cep/0.1;\n" +
		"@sink(type = 'log') stream Out (k string, ts timestamp);\n" +
		"stream S (k string, n int, ts timestamp) time ts watermark bounded 1 min idle after 2 min;\n" +
		"table T (k string) primary key (k);\n" +
		"rule R intent 'x' partition by k of S ttl 1 hour\n" +
		"  from S[k in T and n > 1]\n" +
		"  todo 'later'\n" +
		"  select k\n" +
		"  insert into Out;\n"
	res, err := parser.Parse("positions.cep", src)
	if err != nil {
		t.Fatal(err)
	}
	app := res.App
	r := app.Rules[0]
	at := func(line, col int) model.Pos { return model.Pos{Line: line, Col: col} }
	checks := []struct {
		name      string
		got, want model.Pos
	}{
		{"the pragma id", app.LanguagePos, at(1, 10)},
		{"a stream declaration starts at its annotation", app.Streams[0].Pos, at(2, 1)},
		{"an annotation item's value", app.Streams[0].Annotations[0].Items[0].ValuePos, at(2, 14)},
		{"`time`", app.Streams[1].Time.Pos, at(3, 42)},
		{"the time attribute", app.Streams[1].Time.AttrPos, at(3, 47)},
		{"`idle after`'s duration", app.Streams[1].Time.IdlePos, at(3, 85)},
		{"`rule`", r.Pos, at(5, 1)},
		{"`partition`", r.Partition.Pos, at(5, 19)},
		{"a key's stream", r.Partition.Keys[0].StreamPos, at(5, 37)},
		{"the ttl", r.Partition.TTL.Pos, at(5, 43)},
		{"`from`", r.Body.Pos, at(6, 3)},
		{"the source stream", r.Body.Source.Pos, at(6, 8)},
		{"an `and` at its first operand", r.Body.Source.Filter.Pos, at(6, 10)},
		{"the table after `in`", r.Body.Source.Filter.Args[0].TablePos, at(6, 15)},
		{"`todo`", r.TodoPos[0], at(7, 3)},
		{"`select`", r.SelectPos, at(8, 3)},
		{"the insert target", r.InsertIntoPos, at(9, 15)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: at %d:%d, want %d:%d", c.name, c.got.Line, c.got.Col, c.want.Line, c.want.Col)
		}
	}
}
