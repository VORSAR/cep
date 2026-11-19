package parser

// (*lowerer).str's operands (S1.1c-6 design Q.3), called directly. Two rows give str a token the lexer never yields,
// so they exist only as direct calls: ''' of length 3 (the RawString rule takes six characters at least, so a token
// starting ''' is always six long, and len(quoted) >= 6 is decided only here), and a string ending in a lone
// backslash (the String rule reads \ and the next character as one escape, so a backslash never precedes the closing
// quote alone).

import "testing"

func TestStrEachOperand(t *testing.T) {
	t.Parallel()
	at := Position{Filename: "t.cep", Line: 3, Column: 7}
	for _, r := range []struct {
		name, quoted, want string
		errMsg             string // "" when str records no error
	}{
		{"''' of length 3", `'''`, `'`, ""},
		{"a raw ''' string", `'''a\nb'''`, `a\nb`, ""},
		{`a raw """ string`, `"""a\nb"""`, `a\nb`, ""},
		{"six characters, not raw", `"abcd"`, `abcd`, ""},
		{"an escape after a character", `"a\n"`, "a\n", ""},
		{"a trailing lone backslash", `"a\"`, `a\`, ""},
		{`\u with four hex digits`, `"\u0041"`, "A", ""},
		{`\u with two characters`, `"\u12"`, `\u12`, `"\u12": \u needs four hex digits`},
		{`\u with four non-hex characters`, `"\uzzzz"`, `\uzzzz`, `"\uzzzz": \u needs four hex digits`},
	} {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			l := &lowerer{}
			if got := l.str(r.quoted, at); got != r.want {
				t.Errorf("str(%s) = %q; want %q", r.quoted, got, r.want)
			}
			switch {
			case r.errMsg == "" && l.err != nil:
				t.Errorf("str(%s) recorded %s %q; want no error", r.quoted, l.err.code, l.err.msg)
			case r.errMsg != "" && l.err == nil:
				t.Errorf("str(%s) recorded no error; want E-STR-002 %q", r.quoted, r.errMsg)
			case r.errMsg != "" && (l.err.code != "E-STR-002" || l.err.pos != at || l.err.msg != r.errMsg):
				t.Errorf("str(%s) recorded %s at %v %q; want E-STR-002 at %v %q", r.quoted, l.err.code, l.err.pos, l.err.msg, at, r.errMsg)
			}
		})
	}
}
