package parser

// parseError's operand nets (S1.1c-6 design Q.3): one whole-program row per operand of each compound condition
// (parse.go:129, :135, :148, :172, :177, :179, :182, :184, :188, :191, :193, :197, :199, :201, :203, :205, :207,
// :219), the row on which that operand alone decides the answer; and the first-match order across the groups lane Q
// cuts the switch into (start, arrow, name, operator, missing part, bound, then the clause loop and F3), one input
// with two faults per boundary the groups do not keep apart by the refused token alone. The programs edit
// parseerror_contract_test.go's pecPattern, pecWindow and pecIncident; a row's E-SYN-001 fallback message is
// participle's expected notation in rule words (ruleWords).

import "testing"

const peoFollowed = "followed is not a construct of this language; the canonical form is `->`"

var peoOperandRows = []pecRow{
	// :129 the refused token found by line and column; :135 tok's range at index -1 (at == 0).
	{":129 at the start", pecEdit(pecPattern, "language cep/0.1;\n", ""),
		"E-PRAG-001", 1, 1, "the file does not start with the version pragma"},
	{":129 a column a token on line 1 shares, a line another token starts", pecEdit(pecPattern, "  within 5 min", "  within 5"),
		"E-DUR-001", 8, 10, "5 has no unit"},
	// :135 tok's range past the end: participle stops at the end of input.
	{":135 the end of input", pecEdit(pecPattern, "  insert into T;\n", "  insert into T\n"),
		"E-SYN-001", 13, 16, `unexpected token "<EOF>"; expected ;`},
	// :148 the filter's error: an end of input becomes its `]` (F7); another token stays.
	{":148 a filter that ends early", pecEdit(pecPattern, "a == 'x'", "a == "),
		"E-SYN-001", 7, 26, `unexpected token "]"; expected a name`},
	{":148/:191 a filter refused inside", pecEdit(pecPattern, "a == 'x'", "a has 'x'"),
		"E-TXT-001", 7, 23, "has has no case mode"},
	// :172 a reserved word.
	{":172 reserved", pecEdit(pecPattern, "  select e1.a\n", "  group by a\n  select e1.a\n"),
		"E-RES-001", 11, 3, "group is reserved for a later version"},
	{":172/:184 a keyword where a name goes", pecEdit(pecPattern, "rule R\n", "rule select\n"),
		"E-KW-001", 4, 6, "select is a reserved word, so it cannot be a name"},
	// :177 -> and not.
	{":177 the incident", pecIncident,
		"E-PAT-005", 7, 31, "an absence follows its step directly: drop the -> before and not"},
	{":177 and not after a synonym", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "followed and not S[a == 'y'] for 5 min"),
		"E-SYN-001", 7, 31, peoFollowed},
	{":177 -> then a keyword then not", pecEdit(pecPattern, "-> e2 = S[a == 'y']\n  within 5 min", "->\n  within not 5 min"),
		"E-PAT-005", 8, 3, "a step must follow ->"},
	{":177/:179 -> and, no not", pecEdit(pecIncident, "-> and not B[x == 1] for 5 min", "-> and B[x == 1]"),
		"E-PAT-005", 7, 31, "a step must follow ->"},
	// :179 -> not.
	{":179 -> not", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "-> not S[a == 'y'] for 5 min"),
		"E-PAT-005", 7, 34, "an absence is written and not, straight after its step: write and not instead of -> not"},
	{":179 not after a synonym", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "followed not S[a == 'y'] for 5 min"),
		"E-SYN-001", 7, 31, peoFollowed},
	// :182 -> with nothing that begins a step after it.
	{":182 a trailing -> before within", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "->"),
		"E-PAT-005", 8, 3, "a step must follow ->"},
	{":182/:184 a mixed-case keyword before a keyword", pecEdit(pecPattern, "  from every", "  FROM every"),
		"E-KW-002", 7, 3, "keywords are lowercase: write from"},
	{":182 a -> refused before a step", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "and not S[a == 'y'] -> e2 = S[a == 'y']"),
		"E-PAT-004", 7, 51, "the absence has no bound"},
	// :184 a keyword where a name goes.
	{":184/:205 a number where a name goes", pecEdit(pecPattern, "rule R\n", "rule 5\n"),
		"E-SYN-001", 4, 6, `unexpected token "5"; expected a name`},
	{":184 a mixed-case keyword where a name goes", pecEdit(pecPattern, "rule R\n", "rule SELECT\n"),
		"E-KW-001", 4, 6, "SELECT is a reserved word, so it cannot be a name"},
	// :188 a synonym; :191 a text operator without its case mode.
	{":188 a synonym", pecEdit(pecPattern, "-> e2", "followed by e2"),
		"E-SYN-001", 7, 31, peoFollowed},
	{":191 a name that is neither", pecEdit(pecPattern, "stream S (a string, ts timestamp) time ts watermark bounded 1 min;", "stream S (a string ts timestamp) time ts watermark bounded 1;"),
		"E-SYN-001", 2, 20, `unexpected token "ts"; expected )`},
	// :193 = after from <name>.
	{":193 the restart marker", pecEdit(pecPattern, "from every e1", "from e1"),
		"E-PAT-003", 7, 11, "the pattern has no restart marker"},
	{":193 = after from every", pecEdit(pecPattern, "every e1 = S", "every = S"),
		"E-SYN-002", 7, 14, "= is not an operator here"},
	{":193 = in a filter", pecEdit(pecPattern, "a == 'x'", "a = 'x'"),
		"E-SYN-002", 7, 23, "= is not an operator here"},
	{":193 a name after from <name>", pecEdit(pecPattern, "every e1 = S[a == 'x']", "e1 S[a == 'x']"),
		"E-SYN-001", 7, 11, `unexpected token "S"; expected select`},
	// :197 ( after in.
	{":197 a list after in", pecEdit(pecPattern, "a == 'x'", "a in ('x', 'y')"),
		"E-SYN-003", 7, 26, "in takes a table name, not a list"},
	{":197 a number after in", pecEdit(pecPattern, "a == 'x'", "a in 5"),
		"E-SYN-001", 7, 26, `unexpected token "5"; expected a name`},
	{":197 ( after (", pecEdit(pecPattern, "stream T (a string", "stream T ((a string"),
		"E-SYN-001", 3, 11, `unexpected token "("; expected an attribute, <attr> <type>`},
	// :199 * after select.
	{":199 select *", pecEdit(pecPattern, "  select e1.a\n", "  select *\n"),
		"E-SYN-004", 11, 10, "select * is not allowed"},
	{":199 ) after select", pecEdit(pecPattern, "  select e1.a\n", "  select )\n"),
		"E-SYN-001", 11, 10, `unexpected token ")"; expected a name`},
	{":199 * after ==", pecEdit(pecPattern, "a == 'x'", "a == *"),
		"E-SYN-001", 7, 26, `unexpected token "*"; expected a name`},
	// :201 and where a positive step goes.
	{":201 no positive step", pecEdit(pecPattern, "every e1 = S[a == 'x'] -> e2 = S[a == 'y']", "every and not S[a == 'y'] for 5 min"),
		"E-PAT-005", 7, 14, "the pattern has no positive step"},
	{":201 a number where a step goes", pecEdit(pecPattern, "every e1 = S[a == 'x'] -> e2 = S[a == 'y']", "every 5"),
		"E-SYN-001", 7, 14, `unexpected token "5"; expected a step`},
	{":201 and after late events", pecEdit(pecPattern, "late events drop", "late events and"),
		"E-SYN-001", 10, 15, `unexpected token "and"; expected drop or allow <duration>`},
	// :203 ) after ( <string>.
	{":203 cron without its timezone", pecEdit(pecWindow, "cron('0 0 * * *', 'UTC')", "cron('0 0 * * *')"),
		"E-WIN-001", 7, 43, "cron needs its timezone"},
	{":203 a second string after ( <string>", pecEdit(pecWindow, "cron('0 0 * * *', 'UTC')", "cron('0 0 * * *' 'UTC')"),
		"E-SYN-001", 7, 44, `unexpected token "'UTC'"; expected ,`},
	{":203 ) after a string, no (", pecEdit(pecPattern, "  select e1.a\n", "  select 'x')\n"),
		"E-SYN-001", 11, 13, `unexpected token ")"; expected insert`},
	// :205 a number where a duration goes.
	{":205 for 5", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "and not S[a == 'y'] for 5"),
		"E-DUR-001", 7, 55, "5 has no unit"},
	{":205 a name where a duration goes", pecEdit(pecPattern, "  within 5 min", "  within min"),
		"E-SYN-001", 8, 10, `unexpected token "min"; expected a duration or ${param}`},
	// :207 the absence's bound.
	{":207 no bound", pecEdit(pecPattern, "-> e2 = S[a == 'y']", "and not S[a == 'y']"),
		"E-PAT-004", 8, 3, "the absence has no bound"},
	{":207 a missing clause", pecEdit(pecPattern, "  intent 'x'\n", ""),
		"E-INT-001", 5, 3, "the intent clause is missing"},
	// :219 F3: a bare number found, and a unit after it gets the parse past it.
	{":219 bounded 1", pecEdit(pecPattern, "bounded 1 min", "bounded 1"),
		"E-DUR-001", 2, 61, "1 has no unit"},
	{":219 an abandoned clause with no bare number", pecEdit(pecWindow, "cron('0 0 * * *', 'UTC') emit on close", "delay(x) emit on due"),
		"E-SYN-001", 7, 20, `unexpected token "window"; expected select`},
	{":219 a bare number a unit does not mend", pecEdit(pecPattern, "time ts watermark bounded 1 min;", "time ts watermarkk bounded 1;"),
		"E-SYN-001", 2, 35, `unexpected token "time"; expected ;`},
}

func TestParseErrorEachOperand(t *testing.T) {
	t.Parallel()
	for _, r := range peoOperandRows {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			pecCheck(t, r)
		})
	}
}

// TestParseErrorFirstMatchAcrossGroups: an input where two groups' cases both hold answers with the earlier one.
// The arrow, operator and missing-part groups need the refused token to be `->`, `=`/`(`/`*` and `and`/`)`, and the
// name group a Keyword, MixedKeyword or Ident, so those four never both hold; the rows pair each group with the
// start group before it (the refused token at index 0, or a reserved word where a name goes) and the name group
// with the clause loop after the bound group.
func TestParseErrorFirstMatchAcrossGroups(t *testing.T) {
	t.Parallel()
	const prag = "the file does not start with the version pragma"
	for _, r := range []pecRow{
		{"start/start: a block comment at the start", "/* x\n", "E-CMT-001", 1, 1, "the block comment never closes"},
		{"start/arrow: -> not at the start", "-> not x;\n", "E-PRAG-001", 1, 1, prag},
		{"start/name: a mixed-case keyword at the start", "FROM x;\n", "E-PRAG-001", 1, 1, prag},
		{"start/name: a reserved word where a name goes", pecEdit(pecPattern, "rule R\n", "rule group\n"),
			"E-RES-001", 4, 6, "group is reserved for a later version"},
		{"start/operator: = at the start", "= x;\n", "E-PRAG-001", 1, 1, prag},
		{"start/missing part: and at the start", "and x;\n", "E-PRAG-001", 1, 1, prag},
		{"start/bound: a number at the start", "5 x;\n", "E-PRAG-001", 1, 1, prag},
		{"name/clause loop: a synonym where within goes", pecEdit(pecPattern, "  within 5 min", "  timeout 5 min"),
			"E-SYN-001", 8, 3, "timeout is not a construct of this language; the canonical form is `within <duration>`"},
	} {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			pecCheck(t, r)
		})
	}
}
