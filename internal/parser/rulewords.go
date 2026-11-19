package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
)

func isNameChar(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// ruleWords is err's message in rule words (decision 2 of "S1.1c-3's review and break-it"): participle's
// unexpected token, kept exactly as written (a stream may be named like a type), then what comes next — the first
// item participle expected, in words (firstExpected): `unexpected token "x"; expected a duration`. Any other error's
// message gets each whole unquoted production name replaced by its words, its quoted runs kept.
func ruleWords(err error) string {
	if ute, ok := errors.AsType[*participle.UnexpectedTokenError](err); ok {
		// Message is exactly the token found, then " (expected <notation>)" when participle knows what it expected.
		found := fmt.Sprintf("unexpected token %q", ute.Unexpected)
		expect, ok := strings.CutPrefix(strings.TrimPrefix(ute.Message(), found), " (expected ")
		if !ok {
			return found
		}
		return found + "; expected " + firstExpected(strings.TrimSuffix(expect, ")"))
	}
	if perr, ok := errors.AsType[participle.Error](err); ok {
		return wordsFor(perr.Message())
	}
	return wordsFor(err.Error())
}

// tokenWords are the words for a token type participle names in angle brackets.
var tokenWords = map[string]string{
	"<ident>": "a name", "<backticked>": "a name", "<string>": "a string", "<rawstring>": "a string",
	"<number>": "a number", "<paramref>": "${param}", "<pragma>": "the language version, cep/0.1",
}

// firstExpected is the first item of expect, participle's grammar notation for what it expected, in words: a
// literal without its quotes, a production by its productionWords entry, a token type by tokenWords, a group or an
// alternation by its own first item. No grammar operator, quote or quantifier is kept.
func firstExpected(expect string) string {
	s := strings.TrimLeft(expect, " ~")
	switch {
	case s == "":
		return expect
	case s[0] == '"':
		if lit, err := strconv.QuotedPrefix(s); err == nil {
			if text, err := strconv.Unquote(lit); err == nil {
				return text
			}
		}
	case s[0] == '(':
		return firstExpected(s[1:])
	case s[0] == '<':
		if name, _, ok := strings.Cut(s, ">"); ok {
			if words, ok := tokenWords[name+">"]; ok {
				return words
			}
			return name[1:]
		}
	case isNameChar(s[0]):
		j := 0
		for j < len(s) && isNameChar(s[j]) {
			j++
		}
		if words, ok := productionWords[s[:j]]; ok {
			return words
		}
		return s[:j]
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimRight(fields[0], "?*+!)|")
}

// wordsFor replaces each whole unquoted production name in s by its rule words; a double-quoted run is copied as
// it is. A name is read whole (the longest run of name characters), so the longest name always wins.
func wordsFor(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j+1, len(s))
			b.WriteString(s[i:j])
			i = j
		case isNameChar(c):
			j := i
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			if words, ok := productionWords[s[i:j]]; ok {
				b.WriteString(words)
			} else {
				b.WriteString(s[i:j])
			}
			i = j
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
