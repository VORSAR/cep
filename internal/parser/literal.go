package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"

	"github.com/vorsar/cep/internal/model"
)

// literalGrammar parses one literal on its own, as a param default is written.
var literalGrammar = participle.MustBuild[SignedLiteral](
	participle.Lexer(definition{Definition: rules}),
	participle.UseLookahead(2),
)

// ParseLiteral parses one literal as a param default is written (SPEC §2.4: a signed number, a string, a duration,
// a timestamp, true or false) with the parser's own lowering, so a deployment override and a default never differ
// (design s3-3-1 §3.3). The error, if any, is a *Error named "literal".
func ParseLiteral(text string) (model.Literal, error) {
	const name = "literal"
	code, _, err := lexAll(name, text)
	if err != nil {
		e := lexError(name, text, err)
		if e.Code == "E-SYN-001" {
			// a backtick or any other character the lexer refuses: still one literal's text, never a name's form
			return model.Literal{}, mk(name, "E-SYN-001", Position{Line: e.Line, Column: e.Col}, notALiteral(text))
		}
		return model.Literal{}, e
	}
	if len(code) == 0 {
		return model.Literal{}, mk(name, "E-SYN-001", Position{Line: 1, Column: 1}, "the text holds no literal")
	}
	peek, err := lexer.Upgrade(&sliceLexer{toks: code})
	if err != nil {
		return model.Literal{}, lexError(name, text, err)
	}
	tree, err := literalGrammar.ParseFromLexer(peek)
	if err != nil {
		// a file's error mapping would speak of the version pragma; an override is one literal (break-it S5)
		return model.Literal{}, mk(name, "E-SYN-001", code[0].Pos, notALiteral(text))
	}
	l := &lowerer{}
	lit := l.signedLiteral(tree)
	if l.err != nil {
		return model.Literal{}, mk(name, l.err.code, l.err.pos, l.err.msg)
	}
	return lit, nil
}

// notALiteral is an override's message when its text is not one literal (break-it S5 of S3.3-1).
func notALiteral(text string) string {
	return fmt.Sprintf(
		"%q is not a literal: write a signed number, a string in quotes, a duration, a timestamp '…', true or false", text)
}

// ---- literals ----

// signedLiteral: a param default's minus sign is part of its number, so the smallest int is writable (SPEC rev 6 §2.4).
func (l *lowerer) signedLiteral(s *SignedLiteral) model.Literal {
	if s.Neg != "" {
		return l.number("-"+s.Neg, s.Pos)
	}
	return l.literal(s.Literal)
}

// minIntDigits says text is an integer literal that fits int64 only with a minus sign before it: the smallest int's
// digits (SPEC rev 6 §2.4; owner, 2026-11-07). Unary minus over any other number keeps its form.
func minIntDigits(text string) bool {
	if text == "" || strings.ContainsAny(text, ".eE") {
		return false
	}
	_, err := strconv.ParseInt(text, 10, 64)
	n, errNeg := strconv.ParseInt("-"+text, 10, 64)
	return err != nil && errNeg == nil && n == math.MinInt64
}

func (l *lowerer) literal(lit *Literal) model.Literal {
	switch {
	case lit.String != "":
		return model.Literal{Kind: model.LitString, Str: l.str(lit.String, lit.Pos)}
	case lit.Raw != "":
		return model.Literal{Kind: model.LitString, Str: lit.Raw[3 : len(lit.Raw)-3]}
	case lit.Duration != nil:
		return model.Literal{Kind: model.LitDuration, Ms: l.duration(lit.Duration)}
	case lit.Number != "":
		return l.number(lit.Number, lit.Pos)
	case lit.True:
		return model.Literal{Kind: model.LitBool, Bool: true}
	case lit.False:
		return model.Literal{Kind: model.LitBool, Bool: false}
	default:
		return model.Literal{Kind: model.LitTimestamp, Ms: l.timestamp(lit.Timestamp, lit.Pos)}
	}
}

func (l *lowerer) number(text string, pos Position) model.Literal {
	if strings.ContainsAny(text, ".eE") {
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(f, 0) {
			l.fail("E-TYPE-001", pos, "number %s is out of range", text)
		}
		return model.Literal{Kind: model.LitFloat, Float: f}
	}
	i, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		l.fail("E-TYPE-001", pos, "integer %s is out of range", text)
	}
	return model.Literal{Kind: model.LitInt, Int: i}
}

var units = map[string]int64{"ms": 1, "sec": 1000, "min": 60_000, "hour": 3_600_000, "day": 86_400_000,
	"week": 604_800_000}

// unitRank orders the units from small to large: a duration's units are strictly descending (SPEC §2.4).
var unitRank = map[string]int{"ms": 0, "sec": 1, "min": 2, "hour": 3, "day": 4, "week": 5}

// duration is the sum of its parts in milliseconds. The grammar is lexically wider than SPEC §2.4, which wants
// integer counts in strictly descending, distinct units; the model keeps only the sum, so every part of E-DUR-002
// is decided here, at the duration's first token (S2.2b).
func (l *lowerer) duration(d *Duration) int64 {
	written := make([]string, len(d.Parts))
	for i, p := range d.Parts {
		written[i] = p.Count + " " + p.Unit
	}
	text, at := strings.Join(written, " "), d.Parts[0].Pos
	var total int64
	prev := len(unitRank)
	for _, p := range d.Parts {
		count, err := strconv.ParseInt(p.Count, 10, 64)
		if err != nil || strings.ContainsAny(p.Count, ".eE") {
			l.fail("E-DUR-002", at, "%s: %s is not a whole-number count", text, p.Count)
			return 0
		}
		if r := unitRank[p.Unit]; r >= prev {
			l.fail("E-DUR-002", at, "%s: units must be strictly descending and distinct", text)
			return 0
		} else {
			prev = r
		}
		if count > (math.MaxInt64-total)/units[p.Unit] {
			l.fail("E-DUR-002", at, "%s is too long", text)
			return 0
		}
		total += count * units[p.Unit]
	}
	return total
}

func (l *lowerer) durationOrParam(d *DurationOrParam) model.DurationOrParam {
	if d == nil {
		return model.DurationOrParam{}
	}
	if d.Param != "" {
		return model.DurationOrParam{Param: paramName(d.Param), Pos: pos(d.Pos)}
	}
	return model.DurationOrParam{Ms: l.duration(d.Duration), Pos: pos(d.Pos)}
}

func (l *lowerer) timestamp(quoted string, pos Position) int64 {
	text := l.str(quoted, pos)
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || t.Location() != time.UTC {
		l.fail("E-TYPE-001", pos, "timestamp %q is not ISO-8601 in UTC", text)
		return 0
	}
	return t.UnixMilli()
}

// str decodes a string token. A triple-quoted string is raw. A quoted one takes SPEC §2.4's escapes, exhaustively;
// any other escape is E-STR-002 at the string token (S2.2b: the model keeps decoded text, where an unknown escape
// could no longer be told from a raw backslash).
func (l *lowerer) str(quoted string, at Position) string {
	if len(quoted) >= 6 && (strings.HasPrefix(quoted, "'''") || strings.HasPrefix(quoted, `"""`)) {
		return quoted[3 : len(quoted)-3]
	}
	body := quoted[1 : len(quoted)-1]
	if !strings.Contains(body, `\`) {
		return body
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' || i+1 >= len(body) {
			b.WriteByte(c)
			continue
		}
		i++
		switch body[i] {
		case '\\', '\'', '"':
			b.WriteByte(body[i])
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'u':
			if i+4 < len(body) {
				if v, err := strconv.ParseUint(body[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 4
					continue
				}
			}
			l.fail("E-STR-002", at, "%s: \\u needs four hex digits", quoted)
			b.WriteString(`\u`)
		default:
			l.fail("E-STR-002", at, "%s: \\%c is not an escape", quoted, body[i])
			b.WriteByte('\\')
			b.WriteByte(body[i])
		}
	}
	return b.String()
}
