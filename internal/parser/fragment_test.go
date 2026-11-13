package parser

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"html"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// The rule-fragment test (S1.1c-3 design note v2 §2.3, §10 C1-C3, C5): every rule-like span and fence quoted in
// SPEC.md, SEMANTICS.md and errors.md parses at the kind its marker names, through the real grammar's production
// for it, or is listed in tools/allowlists/fragments.json as a documented-invalid example at the stage it fails at.

// fragmentKind names the production of the real grammar a quoted fragment parses at. A marker carries it:
// <!--cep:<kind>-->, <!--cep-meta:<kind>-->, <!--cep-invalid:<kind>-->, or a fence tag cep-<kind>.
type fragmentKind string

// The kinds: the research harness's twenty-eight (R-F2) and one per clause R-F3 found with no production.
const (
	kindProgram    fragmentKind = "program"
	kindTrivia     fragmentKind = "trivia"
	kindStatements fragmentKind = "statements"
	kindRule       fragmentKind = "rule"
	kindBody       fragmentKind = "body"
	kindPattern    fragmentKind = "patternbody"
	kindChain      fragmentKind = "chain"
	kindChainElem  fragmentKind = "chainelem"
	kindPosElem    fragmentKind = "poselem"
	kindStep       fragmentKind = "step"
	kindSource     fragmentKind = "source"
	kindWithin     fragmentKind = "within"
	kindAfterMatch fragmentKind = "aftermatch"
	kindLate       fragmentKind = "late"
	kindRate       fragmentKind = "rate"
	kindFor        fragmentKind = "for"
	kindPartition  fragmentKind = "partition"
	kindPartKeys   fragmentKind = "partkeys"
	kindPartKey    fragmentKind = "partkey"
	kindExpr       fragmentKind = "expr"
	kindDuration   fragmentKind = "duration"
	kindTimeClause fragmentKind = "timeclause"
	kindWindowKind fragmentKind = "windowkind"
	kindSelect     fragmentKind = "select"
	kindAnnotation fragmentKind = "annotation"
	kindExtra      fragmentKind = "extra"
	kindGivenSec   fragmentKind = "givensec"
	kindRecord     fragmentKind = "record"
	kindIdle       fragmentKind = "idle"
	kindTTL        fragmentKind = "ttl"
	kindRuleHead   fragmentKind = "rulehead"
	kindIntent     fragmentKind = "intent"
	kindFrom       fragmentKind = "from"
	kindWindow     fragmentKind = "window"
	kindPrimaryKey fragmentKind = "primarykey"
	kindKeyList    fragmentKind = "keylist"
)

// invalidExample is one entry of tools/allowlists/fragments.json as the fragment test reads it: a documented-invalid
// span, the document holding it, and the stage it must fail at ("grammar", "lowering" or "validator").
type invalidExample struct {
	File, Span, Stage string
}

var errFragmentNotBuilt = errors.New("not built yet (S1.1c-3 lane B)")

// The stages a fragment fails at, and the stages the register may list.
const (
	stageGrammar   = "grammar"
	stageLowering  = "lowering"
	stageValidator = "validator"
)

// incidentPattern is the incident's shape (design note v2 §2.3, DQ-L8; §10 C3), declared once: an arrow, then
// `not` or `and not`, word-bounded, in any case, with any white space between — ASCII, every Unicode space
// separator, the zero-width space (break-it BK-B1, BK-S7). fragment_research_test.go's incidentRe is this one.
var incidentPattern = regexp.MustCompile(`(?i)(->|→)[\s\p{Zs}\x{200B}]*(and[\s\p{Zs}\x{200B}]+)?not\b`)

// entityAt is an HTML entity at the head of a text: named, decimal or hexadecimal.
var entityAt = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});`)

// rendered is s as a reader sees it, for the incident's shape (break-it BK-B1): emphasis markers (`*`, `_`, `~~`)
// dropped, HTML entities decoded, every Unicode space (and the zero-width space) a plain space. at[i] is the byte
// of s that gave byte i of the result.
func rendered(s string) (string, []int) {
	b := make([]byte, 0, len(s))
	at := make([]int, 0, len(s))
	put := func(text string, from int) {
		for _, r := range text {
			if unicode.Is(unicode.Zs, r) || r == '\u200B' {
				r = ' '
			}
			for range utf8.RuneLen(r) {
				at = append(at, from)
			}
			b = utf8.AppendRune(b, r)
		}
	}
	for i := 0; i < len(s); {
		if e := entityAt.FindString(s[i:]); e != "" {
			put(html.UnescapeString(e), i)
			i += len(e)
			continue
		}
		switch {
		case s[i] == '*' || s[i] == '_':
			i++
		case strings.HasPrefix(s[i:], "~~"):
			i += 2
		default:
			_, size := utf8.DecodeRuneInString(s[i:])
			put(s[i:i+size], i)
			i += size
		}
	}
	return string(b), at
}

// incidentIn finds the incident's shape in text as rendered: the byte of text it starts at and the shape as read.
func incidentIn(text string) (int, string, bool) {
	r, at := rendered(text)
	m := incidentPattern.FindStringIndex(r)
	if m == nil {
		return 0, "", false
	}
	return at[m[0]], r[m[0]:m[1]], true
}

// fragError is a fragment's failure: the kind it was parsed at, the stage, and the parser's or lowerer's own
// position and message (never parseError's catalogue rewording).
type fragError struct {
	kind  fragmentKind
	stage string
	pos   Position
	msg   string
}

func (e *fragError) Error() string {
	return fmt.Sprintf("%s %d:%d: %s", e.kind, e.pos.Line, e.pos.Column, e.msg)
}

// fragmentOptions are Parse's lexer and lookahead, so a fragment is read exactly as a whole file is.
func fragmentOptions() []participle.Option {
	return []participle.Option{participle.Lexer(definition{Definition: rules}), participle.UseLookahead(2)}
}

// fragRunner parses a fragment's code tokens at one production and lowers it where the lowerer takes that sub-tree.
type fragRunner func(code []Token) *fragError

// entryOf builds the production T's parser once; low, when not nil, lowers the tree.
func entryOf[T any](low func(*lowerer, *T)) func() fragRunner {
	return sync.OnceValue(func() fragRunner {
		g := participle.MustBuild[T](fragmentOptions()...)
		return func(code []Token) *fragError {
			peek, err := lexer.Upgrade(&sliceLexer{toks: code})
			if err != nil {
				return grammarFault(err)
			}
			tree, err := g.ParseFromLexer(peek)
			if err != nil {
				return grammarFault(err)
			}
			if low == nil {
				return nil
			}
			l := &lowerer{}
			low(l, tree)
			if l.err != nil {
				return &fragError{stage: stageLowering, pos: l.err.pos, msg: l.err.code + " " + l.err.msg}
			}
			return nil
		}
	})
}

func grammarFault(err error) *fragError {
	if pe, ok := errors.AsType[participle.Error](err); ok {
		return &fragError{stage: stageGrammar, pos: pe.Position(), msg: pe.Message()}
	}
	return &fragError{stage: stageGrammar, pos: Position{Line: 1, Column: 1}, msg: err.Error()}
}

// fragStatements is one or more statements without the pragma: the statements kind.
type fragStatements struct {
	Statements []*Statement `parser:"@@+"`
}

// fragEntries maps every kind but trivia to its production; trivia is read by the lexer alone.
var fragEntries = map[fragmentKind]func() fragRunner{
	kindProgram: entryOf(func(l *lowerer, p *Program) {
		if _, err := lower(p); err != nil {
			l.err = err
		}
	}),
	kindStatements: entryOf[fragStatements](nil),
	kindRule:       entryOf(func(l *lowerer, r *RuleDecl) { l.rule(r) }),
	kindBody:       entryOf(func(l *lowerer, b *Body) { l.body(b) }),
	kindPattern:    entryOf[PatternBody](nil),
	kindChain:      entryOf[PatternChain](nil),
	kindChainElem:  entryOf[ChainElem](nil),
	kindPosElem:    entryOf[PosElem](nil),
	kindStep:       entryOf[Step](nil),
	kindSource:     entryOf[Source](nil),
	kindWithin:     entryOf[WithinClause](nil),
	kindAfterMatch: entryOf[AfterMatchClause](nil),
	kindLate:       entryOf[LateEventsClause](nil),
	kindRate:       entryOf[RateClause](nil),
	kindFor:        entryOf[ForBound](nil),
	kindPartition:  entryOf[Partition](nil),
	kindPartKeys:   entryOf[PartitionKeys](nil),
	kindPartKey:    entryOf[PartKey](nil),
	kindExpr:       entryOf(func(l *lowerer, e *Expr) { l.expr(e) }),
	kindDuration:   entryOf(func(l *lowerer, d *Duration) { l.duration(d) }),
	kindTimeClause: entryOf[TimeClause](nil),
	kindWindowKind: entryOf[WindowKind](nil),
	kindSelect:     entryOf[SelectClause](nil),
	kindAnnotation: entryOf[Annotation](nil),
	kindExtra:      entryOf[Extra](nil),
	kindGivenSec:   entryOf[GivenSec](nil),
	kindRecord:     entryOf[Record](nil),
	kindIdle:       entryOf[IdleClause](nil),
	kindTTL:        entryOf[TTLClause](nil),
	kindRuleHead:   entryOf[RuleHead](nil),
	kindIntent:     entryOf[IntentClause](nil),
	kindFrom:       entryOf[PatternHead](nil),
	kindWindow:     entryOf[WindowClause](nil),
	kindPrimaryKey: entryOf[PrimaryKeyClause](nil),
	kindKeyList:    entryOf[KeyList](nil),
}

func knownKind(k fragmentKind) bool {
	_, ok := fragEntries[k]
	return ok || k == kindTrivia
}

// parseFragment parses src at kind through the real grammar's production for it — trailing tokens refused, then
// lowered where the lowerer takes that sub-tree — and gives participle's own message and position; nil when it parses.
func parseFragment(kind fragmentKind, src string) error {
	if !knownKind(kind) {
		return fmt.Errorf("unknown kind %s", kind)
	}
	code, trivia, err := lexAll("", src)
	if err != nil {
		e := grammarFault(err)
		e.kind = kind
		return e
	}
	if kind == kindTrivia {
		if len(code) != 0 {
			return &fragError{kind: kind, stage: stageGrammar, pos: code[0].Pos,
				msg: fmt.Sprintf("unexpected token %q (expected a comment)", code[0].Value)}
		}
		if len(trivia) == 0 {
			return &fragError{kind: kind, stage: stageGrammar, pos: Position{Line: 1, Column: 1}, msg: "no comment"}
		}
		return nil
	}
	if e := fragEntries[kind]()(code); e != nil {
		e.kind = kind
		return e
	}
	return nil
}

func stageOf(err error) string {
	if e, ok := errors.AsType[*fragError](err); ok {
		return e.stage
	}
	return stageGrammar
}

// ---- the placeholders ----

// anglePattern is a placeholder written in angle brackets, as R-F3 read it.
const anglePattern = `<[A-Za-z][A-Za-z0-9_ '-]*>`

// angleNames is the closed table of angle placeholders and their stand-ins (design note v2 §2.3).
var angleNames = map[string]string{
	"<duration>": "5 min", "<Name>": "Nm", "<name>": "nm", "<Stream>": "Src", "<ThatStream>": "Src2",
	"<Table>": "Tbl", "<type>": "string", "<literal>": "1", "<expr>": "x == 1", "<pred>": "x == 1",
	"<cond>": "x == 1", "<attr>": "a", "<cols>": "a", "<key>": "k", "<count>": "10", "<n>": "10",
	"<unit>": "min", "<op>": "==", "<step>": "a = A", "<dir>": "asc", "<tz>": "'UTC'", "<cron>": "'0 9 * * *'",
	"<time-clause>": "time arrival", "<timestamp>": "ts", "<int>": "n",
}

// bareForms is the closed table of the bare placeholders, in the order they bind, after the angle names. A bare
// word binds only as a whole token: no name character on either side.
var bareForms = []struct {
	re   *regexp.Regexp
	with string
	lead bool // only at the head of the span
}{
	{regexp.MustCompile(`^\s*,\s*`), "", true},      // a clause shown with its leading comma (E-PART-002's pinned fix)
	{regexp.MustCompile(`\$\{…\}`), "$${p}", false}, // $$: a literal $ in a replacement
	{regexp.MustCompile(`\[\s*…\s*\]`), "[x == 1]", false},
	{regexp.MustCompile(`^\s*…(\s*(->|and\b))`), "every a0 = A0$1", true},
	{regexp.MustCompile(`,\s*…`), "", false},
	{regexp.MustCompile(`\s…\s+(ttl\b)`), " $1", false}, // a list continued with a bare … before ttl (E-PART-001's pinned fix)
	{regexp.MustCompile(`=\s*…`), "= A1", false},
	{regexp.MustCompile(`…`), " A1 ", false},
	{regexp.MustCompile(`(^|[^A-Za-z0-9_])D([^A-Za-z0-9_]|$)`), "${1}5 min$2", false},
	{regexp.MustCompile(`(^|[^A-Za-z0-9_])pred([^A-Za-z0-9_]|$)`), "${1}x == 1$2", false},
	{regexp.MustCompile(`(^|[^A-Za-z0-9_])filter([^A-Za-z0-9_]|$)`), "${1}x == 1$2", false},
	{regexp.MustCompile(`(^|[^A-Za-z0-9_])Stream([^A-Za-z0-9_]|$)`), "${1}Src$2", false},
}

var (
	// bindSegments finds what binds apart from the code around it: a string (bound by its content), a ${…}
	// reference, an angle name (with a glued … after it).
	bindSegments = regexp.MustCompile(`'''[\s\S]*?'''|"""[\s\S]*?"""|'(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"|\$\{[^}]*\}|` +
		anglePattern + `…?`)
	spaceRuns = regexp.MustCompile(`[ \t]{2,}`)
)

// bindPlaceholders replaces each placeholder in src (D, pred, filter, …, ${…}, Stream, […], and every <angle> name
// in the closed table) with its stand-in; an unknown <angle> name is an error naming it. Angle names bind first,
// then the bare forms, each as a whole token; a string literal whose whole content is <…> binds without a table
// check; nothing else inside a string or a ${name} is touched.
func bindPlaceholders(src string) (string, error) {
	type seg struct {
		text string
		code bool
	}
	var segs []seg
	last := 0
	for _, m := range bindSegments.FindAllStringIndex(src, -1) {
		segs = append(segs, seg{src[last:m[0]], true})
		tok := src[m[0]:m[1]]
		switch {
		case tok[0] == '<':
			name := strings.TrimSuffix(tok, "…") // a glued <step>… binds as one
			with, ok := angleNames[name]
			if !ok {
				return "", fmt.Errorf("unknown placeholder %s", name)
			}
			segs = append(segs, seg{" " + with + " ", false})
		case tok == "${…}":
			segs = append(segs, seg{tok, true})
		case tok[0] == '$':
			segs = append(segs, seg{tok, false})
		default:
			segs = append(segs, seg{stringStandIn(tok), false})
		}
		last = m[1]
	}
	segs = append(segs, seg{src[last:], true})
	var b strings.Builder
	for i, s := range segs {
		t := s.text
		if s.code {
			for _, f := range bareForms {
				switch {
				case f.lead && i != 0:
				case f.lead: // once: a second leading comma is a mistake, not notation (break-it BK-L4)
					t = f.re.ReplaceAllString(t, f.with)
				default:
					for next := f.re.ReplaceAllString(t, f.with); next != t; next = f.re.ReplaceAllString(t, f.with) {
						t = next
					}
				}
			}
		}
		b.WriteString(t)
	}
	return strings.TrimSpace(spaceRuns.ReplaceAllString(b.String(), " ")), nil
}

// stringStandIn binds a string literal whose whole content is <…>: <ISO-8601> to a timestamp, any other to 'x'.
func stringStandIn(lit string) string {
	q := lit[:1]
	if strings.HasPrefix(lit, "'''") || strings.HasPrefix(lit, `"""`) {
		q = lit[:3]
	}
	body := lit[len(q) : len(lit)-len(q)]
	if !strings.HasPrefix(body, "<") || !strings.HasSuffix(body, ">") || strings.Count(body, ">") != 1 {
		return lit
	}
	if body == "<ISO-8601>" {
		return "'2026-01-01T00:00:00Z'"
	}
	return "'x'"
}

// ---- the detector ----

// fragTok is one token of a span as the detector counts it: a lexer token, an angle placeholder, or a notation
// token (a character no lexer rule accepts).
type fragTok struct {
	typ      lexer.TokenType
	val      string
	angle    bool
	notation bool
}

var (
	tokArrow = rules.Symbols()["Arrow"]
	tokSpace = rules.Symbols()["Whitespace"]
	tokCmt   = rules.Symbols()["Comment"]
	tokIdent = rules.Symbols()["Ident"]
	angleAt  = regexp.MustCompile(`^` + anglePattern)
)

// fragTokens counts s's tokens by lexer.go's rules; an angle placeholder is one token, and a character no rule
// accepts is one notation token (the span need not lex).
func fragTokens(s string) []fragTok {
	var out []fragTok
	for s != "" {
		if a := angleAt.FindString(s); a != "" {
			out = append(out, fragTok{val: a, angle: true})
			s = s[len(a):]
			continue
		}
		lx, err := rules.LexString("", s)
		if err != nil {
			return out
		}
		rest := ""
		for {
			t, err := lx.Next()
			if err != nil {
				at := 0
				if le, ok := errors.AsType[*lexer.Error](err); ok {
					at = le.Pos.Offset
				}
				_, size := utf8.DecodeRuneInString(s[at:])
				out = append(out, fragTok{val: s[at : at+size], notation: true})
				rest = s[at+size:]
				break
			}
			if t.EOF() {
				break
			}
			if t.Type == tokSpace || t.Type == tokCmt {
				continue
			}
			if t.Value == "<" && angleAt.MatchString(s[t.Pos.Offset:]) {
				rest = s[t.Pos.Offset:]
				break
			}
			out = append(out, fragTok{typ: classify(t, tokIdent).Type, val: t.Value})
		}
		s = rest
	}
	return out
}

// spanTraits is the detector's reading of one span.
type spanTraits struct {
	ruleLike bool  // ≥ 2 tokens, a keyword, not path-like
	exempt   bool  // rule-like, keyword-only, no ->: a mention
	notation bool  // a notation token after binding; an angle name is never one (break-it BK-B2)
	unbound  error // the binding's error: an angle name outside the table
}

// keywordTok reports whether t is a reserved word in any case: a keyword written in capitals is still one to a
// reader (break-it BK-S7), and the language refuses it (E-KW-002).
func keywordTok(t fragTok) bool { return t.typ == tokKeyword || t.typ == tokMixedKeyword }

// minMixedKeywords is how many reserved words in another case make a span rule-like on their own: one capitalised
// word is an acronym, a catalogue code's segment or a placeholder (`TTL ≥ D`, `E-TIME-001`, `name = Stream[filter]`
// in the documents); rule text in capitals holds several (`FROM S[x == 1] SELECT a INSERT INTO Out`).
const minMixedKeywords = 2

var (
	pathLike = regexp.MustCompile(`^[\w.\-]+(/[\w.\-*]+)+/?$`)
	fileLike = regexp.MustCompile(`^[\w\-]+(\.[\w\-]+)*\.(go|md|py|json|jsonl|cep|lark|ebnf|txt|yaml|yml|toml)$`)
)

// detect reads a span (design note v2 §2.3): rule-like when it holds ≥ 2 tokens, one a keyword (or
// minMixedKeywords in another case: BK-S7), and is not path-like; an exempt mention when its bound tokens are
// keywords, in any case, only and it holds no ->; text-able when a notation token survives binding. A placeholder
// that does not bind is unbound, never notation (BK-B2).
func detect(span string) spanTraits {
	raw := fragTokens(span)
	kw, mixed := 0, 0
	for _, t := range raw {
		switch t.typ {
		case tokKeyword:
			kw++
		case tokMixedKeyword:
			mixed++
		}
	}
	trimmed := strings.TrimSpace(span)
	path := !strings.ContainsAny(trimmed, " \t\n") && (pathLike.MatchString(trimmed) || fileLike.MatchString(trimmed))
	tr := spanTraits{ruleLike: len(raw) >= 2 && (kw >= 1 || mixed >= minMixedKeywords) && !path}
	toks := raw
	if bound, err := bindPlaceholders(span); err == nil {
		toks = fragTokens(bound)
	} else {
		tr.unbound = err
	}
	kwOnly := len(toks) > 0
	for _, t := range toks {
		if t.notation {
			tr.notation = true
		}
		if !keywordTok(t) && t.typ != tokArrow {
			kwOnly = false
		}
	}
	tr.exempt = tr.ruleLike && kwOnly && !strings.Contains(span, "->")
	return tr
}

// ---- the Markdown reader ----

// fault is one finding before formatting: its line (0 for the file), its precedence (§10 C2: the incident 0, a
// parse or stage fault 1, a marker fault 2) and its text.
type fault struct {
	line, prec int
	text       string
}

type addFault func(line, prec int, format string, args ...any)

// codeSpan is one inline code span: its content, its first line, its bytes (backticks and marker included) and
// the marker written straight after it.
type codeSpan struct {
	text       string
	line       int
	start, end int
	marker     string
	hasMarker  bool
}

var (
	fenceRun   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	markerAt   = regexp.MustCompile(`^<!--([\s\S]*?)-->`)
	commentAny = regexp.MustCompile(`<!--[\s\S]*?-->`)
	markerForm = regexp.MustCompile(`^(cep|cep-meta|cep-invalid):([a-z]+)$`)
	htmlOpen   = regexp.MustCompile(`^ {0,3}</?[A-Za-z][A-Za-z0-9-]*(\s|>|/>|$)`)
	// the lines after which CommonMark may start an indented code block, besides a blank line and a closed fence
	// (break-it BK-S2): an ATX heading, a thematic break, a setext underline
	headingLine = regexp.MustCompile(`^ {0,3}#{1,6}([ \t]|$)`)
	breakLine   = regexp.MustCompile(`^ {0,3}((-[ \t]*){3,}|(\*[ \t]*){3,}|(_[ \t]*){3,}|=+[ \t]*|-+[ \t]*)$`)
	// the prose lines that begin a text of their own (break-it BK-B1): a list item; and the one-line texts, a table
	// row and a heading
	listItem    = regexp.MustCompile(`^[ \t]*([-+*]|[0-9]{1,9}[.)])([ \t]|$)`)
	oneLineText = regexp.MustCompile(`^[ \t]*(\||#{1,6}([ \t]|$))`)
)

// openFence reads a fence's opening line (CommonMark 4.5): its run and its tag, the info string's first word. A
// backtick fence's info string holds no backtick, or the line is not a fence (break-it BK-S3).
func openFence(l string) (mark, tag string, ok bool) {
	m := fenceRun.FindStringSubmatch(l)
	if m == nil || m[1][0] == '`' && strings.Contains(m[2], "`") {
		return "", "", false
	}
	if f := strings.Fields(m[2]); len(f) > 0 {
		tag, _, _ = strings.Cut(f[0], "`")
	}
	return m[1], tag, true
}

// closesFence reports whether l closes a fence opened by mark: a run of its character, at least as long, alone.
func closesFence(l, mark string) bool {
	m := fenceRun.FindStringSubmatch(l)
	return m != nil && strings.HasPrefix(m[1], mark) && strings.TrimSpace(m[2]) == ""
}

// checkMarkdown judges one Markdown document: every fence and every rule-like inline span, its marker, its parse,
// and the invalid examples listed for this file. One fault per line, each naming file:line; none when it is clean.
func checkMarkdown(file, markdown string, invalid []invalidExample) []string {
	var faults []fault
	add := func(line, prec int, format string, args ...any) {
		faults = append(faults, fault{line, prec, fmt.Sprintf(format, args...)})
	}
	listed := map[string][]string{} // a span's stages, one per entry: each entry covers one occurrence (BK-S6)
	for _, e := range invalid {
		if e.File != file {
			continue
		}
		switch e.Stage {
		case stageGrammar, stageLowering, stageValidator:
			listed[e.Span] = append(listed[e.Span], e.Stage)
		default:
			add(0, 1, "unknown stage %s (%s)", e.Stage, e.Span)
		}
	}
	prose, lineAt, html := readBlocks(markdown, add)
	found := map[string]int{}
	text := []byte(prose)
	for _, s := range readSpans(prose, lineAt) {
		blankOut(text, s.start, s.end-s.start)
		if html[s.line] {
			add(s.line, 2, "HTML block: a code span inside it is not read; write the span outside the block")
			continue
		}
		judgeSpan(s, listed, found, add)
	}
	// a comment not written straight after a span is a malformed marker; then the incident's shape on every prose
	// text — a paragraph, a list item, a table row, a heading, its lines joined — with its code spans removed
	// (§10 C1; break-it BK-B1)
	for _, m := range commentAny.FindAllIndex(text, -1) {
		add(lineAt[m[0]], 2, "malformed marker %s: write it straight after its span", text[m[0]:m[1]])
		blankOut(text, m[0], m[1]-m[0])
	}
	for _, u := range proseTexts(string(text)) {
		checkIncident(u.text, lineAt[u.start], add)
	}
	for _, span := range slices.Sorted(maps.Keys(listed)) {
		switch n, want := found[span], len(listed[span]); {
		case n == 0:
			add(0, 2, "listed invalid example not found: %s", span)
		case n < want:
			add(0, 2, "listed invalid example not found: %s (listed %d times, found %d)", span, want, n)
		}
	}
	return formatFaults(file, faults)
}

// readBlocks reads the block structure: it judges each fence, refuses each indented code block and each HTML block
// by name, and gives the prose (fences and indented blocks blanked, blockquote markers spaced out, byte offsets
// kept), each byte's line, and the lines of an HTML block. An indented block starts where CommonMark starts one:
// after a blank line, a heading, a thematic break or a closed fence, never inside a paragraph (BK-S2); an HTML
// block where one may start, on its first line (BK-S1); a fence still open at the end of the file is a fault
// (BK-S3).
func readBlocks(markdown string, add addFault) (string, []int, map[int]bool) {
	masked := []byte(markdown)
	lineAt := make([]int, 0, len(markdown)+1)
	html := map[int]bool{}
	lines := strings.SplitAfter(markdown, "\n")
	inFence, mark, tag, at := false, "", "", 0
	var body []string
	mayStart, inIndent, inHTML, offset := true, false, false, 0
	for n, raw := range lines {
		ln := n + 1
		for range len(raw) {
			lineAt = append(lineAt, ln)
		}
		l := strings.TrimRight(raw, "\n")
		blank := strings.TrimSpace(l) == ""
		fMark, fTag, opens := openFence(l)
		switch {
		case inFence:
			if closesFence(l, mark) {
				judgeFence(tag, at, body, add)
				inFence, mayStart = false, true
			} else {
				body = append(body, l)
			}
			blankOut(masked, offset, len(l))
		case blank:
			inHTML, inIndent, mayStart = false, false, true
		case !inHTML && (inIndent || mayStart) && (strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t")):
			if !inIndent {
				add(ln, 2, "indented code block: fence it with a tag")
			}
			inIndent = true
			checkIncident(l, ln, add)
			blankOut(masked, offset, len(l))
		case opens:
			inFence, mark, tag, at, body, inHTML, inIndent = true, fMark, fTag, ln, nil, false, false
			blankOut(masked, offset, len(l))
		default:
			if !inHTML && mayStart && htmlOpen.MatchString(l) {
				inHTML = true
				add(ln, 2, "HTML block: write rule text in a tagged fence")
			}
			if inHTML {
				html[ln] = true
			}
			inIndent = false
			mayStart = !inHTML && (headingLine.MatchString(l) || breakLine.MatchString(l))
			blankOut(masked, offset, blockquote(l))
		}
		offset += len(raw)
	}
	if inFence {
		add(at, 2, "fence never closes: close it, or it hides every line after it")
		judgeFence(tag, at, body, add)
	}
	lineAt = append(lineAt, len(lines)+1)
	return string(masked), lineAt, html
}

// proseText is one text a reader reads as one, and its first byte in the prose.
type proseText struct {
	start int
	text  string
}

// proseTexts splits the prose into its texts (break-it BK-B1): a paragraph or a list item, its lines joined; a
// table row; a heading. A blank line ends each.
func proseTexts(prose string) []proseText {
	var out []proseText
	start, off := -1, 0
	flush := func(end int) {
		if start >= 0 {
			out = append(out, proseText{start, prose[start:end]})
			start = -1
		}
	}
	for line := range strings.SplitAfterSeq(prose, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			flush(off)
		case listItem.MatchString(line) || oneLineText.MatchString(line):
			flush(off)
			start = off
		case start < 0:
			start = off
		}
		off += len(line)
		if oneLineText.MatchString(line) {
			flush(off)
		}
	}
	flush(len(prose))
	return out
}

func blankOut(b []byte, from, n int) {
	for i := from; i < from+n && i < len(b); i++ {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
}

// blockquote is the length of a line's blockquote markers (`>` and one optional space, repeated).
func blockquote(line string) int {
	i := 0
	for {
		j := i
		for j < len(line) && j-i < 3 && line[j] == ' ' {
			j++
		}
		if j >= len(line) || line[j] != '>' {
			return i
		}
		j++
		if j < len(line) && line[j] == ' ' {
			j++
		}
		i = j
	}
}

// readSpans finds the inline code spans of the prose (CommonMark: a run of n backticks closes at the next run of
// exactly n in the same paragraph; a line break is a space; one space each side is stripped when both are there).
func readSpans(prose string, lineAt []int) []codeSpan {
	var out []codeSpan
	for i := 0; i < len(prose); {
		switch {
		case prose[i] == '\\' && i+1 < len(prose) && prose[i+1] == '`':
			i += 2
			continue
		case prose[i] != '`':
			i++
			continue
		}
		j := i
		for j < len(prose) && prose[j] == '`' {
			j++
		}
		closeAt := closingRun(prose, j, j-i)
		if closeAt < 0 {
			i = j
			continue
		}
		text := strings.ReplaceAll(prose[j:closeAt], "\n", " ")
		if len(text) >= 2 && text[0] == ' ' && text[len(text)-1] == ' ' && strings.TrimSpace(text) != "" {
			text = text[1 : len(text)-1]
		}
		s := codeSpan{text: text, line: lineAt[i], start: i, end: closeAt + j - i}
		if m := markerAt.FindStringSubmatch(prose[s.end:]); m != nil {
			s.marker, s.hasMarker = m[1], true
			s.end += len(m[0])
		}
		out = append(out, s)
		i = s.end
	}
	return out
}

// closingRun is the start of the next run of exactly n backticks from j in the same paragraph, or -1.
func closingRun(prose string, j, n int) int {
	for k := j; k < len(prose); {
		if prose[k] == '\n' {
			rest := prose[k+1:]
			if e := strings.IndexByte(rest, '\n'); e >= 0 {
				rest = rest[:e]
			}
			if strings.TrimSpace(rest) == "" {
				return -1 // a blank line ends the paragraph
			}
		}
		if prose[k] != '`' {
			k++
			continue
		}
		e := k
		for e < len(prose) && prose[e] == '`' {
			e++
		}
		if e-k == n {
			return k
		}
		k = e
	}
	return -1
}

// checkIncident gives the incident's fault for each place text, which starts at line ln, holds its shape as
// rendered (incidentIn), at the line the shape starts on.
func checkIncident(text string, ln int, add addFault) {
	for off := 0; off < len(text); {
		at, m, ok := incidentIn(text[off:])
		if !ok {
			return
		}
		add(ln+strings.Count(text[:off+at], "\n"), 0, "-> before and/not: %s", strings.Join(strings.Fields(m), " "))
		_, size := utf8.DecodeRuneInString(text[off+at:])
		off += at + size
	}
}

// parsesAt is the first kind, in name order, src parses at; "" when it fails at every kind.
func parsesAt(src string) fragmentKind {
	for _, k := range append(slices.Sorted(maps.Keys(fragEntries)), kindTrivia) {
		if parseFragment(k, src) == nil {
			return k
		}
	}
	return ""
}

// judgeSpan judges one inline span: the incident's shape, its marker, its parse and its listing. A listed span's
// k-th cep-invalid occurrence takes the span's k-th entry; one more occurrence than entries is not listed (BK-S6).
func judgeSpan(s codeSpan, listed map[string][]string, found map[string]int, add addFault) {
	tr := detect(s.text)
	quoted := "`" + s.text + "`"
	_, _, incident := incidentIn(s.text)
	if !s.hasMarker || s.marker == "text" {
		if incident {
			add(s.line, 0, "-> before and/not: %s", quoted)
		}
		if !tr.ruleLike || tr.exempt {
			return
		}
		if tr.unbound != nil { // bound and judged on the bound tokens, as the marked forms are (BK-B2)
			add(s.line, 2, "%v: %s", tr.unbound, quoted)
		}
		switch {
		case !s.hasMarker:
			add(s.line, 2, "unmarked rule-like span: mark its kind: %s", quoted)
		case !tr.notation:
			add(s.line, 2, "text marker on grammar tokens: mark its kind: %s", quoted)
		}
		return
	}
	m := markerForm.FindStringSubmatch(s.marker)
	if m == nil {
		add(s.line, 2, "malformed marker <!--%s-->: %s", s.marker, quoted)
		return
	}
	form, kind := m[1], fragmentKind(m[2])
	if !knownKind(kind) {
		add(s.line, 2, "unknown kind %s: %s", kind, quoted)
		return
	}
	src := s.text
	if form != "cep" {
		bound, err := bindPlaceholders(s.text)
		if err != nil {
			add(s.line, 2, "%v: %s", err, quoted)
			return
		}
		src = bound
	}
	err := parseFragment(kind, src)
	if form != "cep-invalid" {
		switch {
		case incident && err != nil:
			add(s.line, 0, "-> before and/not: %v", err)
		case incident:
			add(s.line, 0, "-> before and/not: %s", quoted)
		case err != nil:
			add(s.line, 1, "%v", err)
		}
		return
	}
	stages := listed[s.text]
	found[s.text]++
	n := found[s.text]
	if n > len(stages) {
		if incident {
			add(s.line, 0, "-> before and/not: %s", quoted)
		}
		if err == nil {
			add(s.line, 1, "documented as invalid, but parses: %s", quoted)
		}
		if len(stages) == 0 {
			add(s.line, 2, "not a listed invalid example: %s", quoted)
		} else {
			add(s.line, 2, "not a listed invalid example: %s is listed %d times and this is occurrence %d: list each", quoted, len(stages), n)
		}
		return
	}
	switch stage := stages[n-1]; {
	case err == nil && stage != stageValidator:
		add(s.line, 1, "documented as invalid, but parses: %s", quoted)
	case err != nil && stageOf(err) != stage:
		add(s.line, 1, "fails at %s, listed at %s: %s (%v)", stageOf(err), stage, quoted, err)
	case stage == stageValidator:
		// §10 C1: a listed span stands, the incident's shape included
	default:
		if k := parsesAt(src); k != "" { // invalid at every kind, not only its marker's (BK-L3)
			add(s.line, 1, "documented as invalid, but parses at %s: %s", k, quoted)
		}
	}
}

// judgeFence judges one fence by its tag (DA-B1): cep is a program, cep-<kind> a kind, text line by line under the
// inline text rule, go not read; any other tag is refused. The incident's shape is checked on the whole body.
func judgeFence(tag string, at int, body []string, add addFault) {
	checkIncident(strings.Join(body, "\n"), at+1, add) // the body as one text (BK-B1)
	switch {
	case tag == "":
		add(at, 2, "untagged fence: tag it cep, cep-<kind>, text or go")
	case tag == "go":
	case tag == "text":
		for i, l := range body {
			tr := detect(l)
			if !tr.ruleLike || tr.exempt {
				continue
			}
			if tr.unbound != nil { // bound and judged on the bound tokens (BK-B2)
				add(at+1+i, 2, "%v: `%s`", tr.unbound, strings.TrimSpace(l))
			}
			if _, _, incident := incidentIn(l); !tr.notation && !incident {
				add(at+1+i, 2, "text marker on grammar tokens: mark its kind: `%s`", strings.TrimSpace(l))
			}
		}
	case tag == "cep" || strings.HasPrefix(tag, "cep-"):
		kind := kindProgram
		if k, ok := strings.CutPrefix(tag, "cep-"); ok {
			kind = fragmentKind(k)
		}
		if !knownKind(kind) {
			add(at, 2, "unknown kind %s: fence tag %s", kind, tag)
			return
		}
		if err := parseFragment(kind, strings.Join(body, "\n")); err != nil {
			line := at + 1
			if e, ok := errors.AsType[*fragError](err); ok && e.pos.Line > 0 {
				line = at + e.pos.Line
			}
			add(line, 1, "%v", err)
		}
	default:
		add(at, 2, "unknown fence tag %s", tag)
	}
}

// formatFaults gives one fault per line — file:line: and its findings in precedence order (§10 C2) — and each
// file-level finding as file: and its text (§10 C5).
func formatFaults(file string, faults []fault) []string {
	slices.SortStableFunc(faults, func(a, b fault) int {
		if a.line != b.line {
			return a.line - b.line
		}
		return a.prec - b.prec
	})
	var out []string
	for i := 0; i < len(faults); {
		f := faults[i]
		if f.line == 0 {
			out = append(out, file+": "+f.text)
			i++
			continue
		}
		var texts []string
		j := i
		for ; j < len(faults) && faults[j].line == f.line; j++ {
			if !slices.Contains(texts, faults[j].text) {
				texts = append(texts, faults[j].text)
			}
		}
		out = append(out, fmt.Sprintf("%s:%d: %s", file, f.line, strings.Join(texts, "; ")))
		i = j
	}
	return out
}

// ---- the register ----

// registerPath is the register, from this package's directory.
const registerPath = "../../tools/allowlists/fragments.json"

// fragmentRegister is tools/allowlists/fragments.json as the fragment test reads it: its rule, and each entry's
// target and reason.
type fragmentRegister struct {
	Rule    string `json:"rule"`
	Entries []struct {
		Target string `json:"target"`
		Reason string `json:"reason"`
	} `json:"entries"`
}

// registerEntry is one register entry read: the example, and its reason as written.
type registerEntry struct {
	invalidExample
	Reason string
}

// readRegister reads the register at path: each entry's target is `<file>: <span>`; its reason begins with the
// stage token (grammar:, lowering:, validator: <code> <pkg>.<Test>/<case>).
func readRegister(path string) ([]registerEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reg fragmentRegister
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if reg.Rule != "fragments" {
		return nil, fmt.Errorf("%s: rule is %q, not fragments", path, reg.Rule)
	}
	out := make([]registerEntry, 0, len(reg.Entries))
	for _, e := range reg.Entries {
		file, span, ok := strings.Cut(e.Target, ": ")
		if !ok {
			return nil, fmt.Errorf("%s: target %q is not <file>: <span>", path, e.Target)
		}
		stage, _, _ := strings.Cut(strings.TrimSpace(e.Reason), ":")
		if f := strings.Fields(stage); len(f) > 0 {
			stage = f[0]
		}
		out = append(out, registerEntry{invalidExample{File: file, Span: span, Stage: stage}, e.Reason})
	}
	return out, nil
}

// FragmentValidatorCase is one validator-stage register entry as fragment_validator_test.go (package parser_test,
// which may import the validator) re-runs it: the span, its kind from its marker in its document, the span bound,
// and the code its reason lists (`validator: <code> <pkg>.<Test>/<case>`).
type FragmentValidatorCase struct {
	File, Span, Kind, Bound, Code string
}

// FragmentValidatorCases reads the register at registerFile and gives its validator entries; root is the
// repository root the entries' files are read from. A missing register is an error naming it.
func FragmentValidatorCases(root, registerFile string) ([]FragmentValidatorCase, error) {
	entries, err := readRegister(registerFile)
	if err != nil {
		return nil, err
	}
	var out []FragmentValidatorCase
	taken := map[invalidExample]int{} // the occurrences earlier entries for the same span took (BK-S6)
	for _, e := range entries {
		if e.Stage != stageValidator {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(e.Reason), stageValidator+":"))
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: %s: reason %q is not validator: <code> <pkg>.<Test>/<case>", registerFile, e.Span, e.Reason)
		}
		doc, err := os.ReadFile(root + "/" + e.File)
		if err != nil {
			return nil, err
		}
		after, ok := string(doc), true
		for range taken[e.invalidExample] + 1 { // this entry's occurrence: its kind is read there
			if _, after, ok = strings.Cut(after, "`"+e.Span+"`<!--cep-invalid:"); !ok {
				break
			}
		}
		taken[e.invalidExample]++
		kind, _, closed := strings.Cut(after, "-->")
		if !ok || !closed {
			return nil, fmt.Errorf("%s: %s: no <!--cep-invalid:<kind>--> marker after the span", e.File, e.Span)
		}
		bound, err := bindPlaceholders(e.Span)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", e.File, e.Span, err)
		}
		out = append(out, FragmentValidatorCase{File: e.File, Span: e.Span, Kind: kind, Bound: bound, Code: fields[0]})
	}
	return out, nil
}

// ---- the real documents ----

// fragmentDocs are the three documents whose quoted rule fragments the test judges (design note v2 §2.3).
var fragmentDocs = []string{"docs/language/SPEC.md", "docs/language/SEMANTICS.md", "docs/language/errors.md"}

// docCensus is one document's count: its rule-like spans (exempt mentions included) and its markers by form, a
// fence counted by its tag.
type docCensus struct {
	ruleLike int
	markers  map[string]int
}

func censusOf(md string) docCensus {
	c := docCensus{markers: map[string]int{}}
	prose, lineAt, _ := readBlocks(md, func(int, int, string, ...any) {})
	for _, s := range readSpans(prose, lineAt) {
		if detect(s.text).ruleLike {
			c.ruleLike++
		}
		if s.hasMarker {
			form, _, _ := strings.Cut(s.marker, ":")
			c.markers[form]++
		}
	}
	for l := range strings.Lines(md) {
		if _, tag, ok := openFence(strings.TrimRight(l, "\n")); ok && tag != "" {
			c.markers["fence "+tag]++
		}
	}
	return c
}

// TestEveryQuotedRuleFragmentParses: every rule-like span and fence in SPEC.md, SEMANTICS.md and errors.md is
// marked and parses at its kind, or is listed in tools/allowlists/fragments.json at the stage it fails at; each
// document's rule-like span count and markers by form are pinned, and move with each document commit.
func TestEveryQuotedRuleFragmentParses(t *testing.T) {
	t.Parallel()
	var invalid []invalidExample
	entries, err := readRegister(filepath.FromSlash(registerPath))
	if err != nil {
		t.Errorf("the register tools/allowlists/fragments.json: %v", err)
	}
	for _, e := range entries {
		invalid = append(invalid, e.invalidExample)
	}
	pins := map[string]docCensus{
		// The corrections and markers commits (S1.1c-3) moved these, as the corpus pin moves (design §2.3).
		"docs/language/SPEC.md": {ruleLike: 84, markers: map[string]int{
			"cep": 15, "cep-meta": 22, "cep-invalid": 4, "text": 13,
			"fence go": 1, "fence cep": 2, "fence cep-body": 1, "fence cep-patternbody": 1, "fence cep-rule": 1,
			"fence cep-trivia": 1, "fence text": 2,
		}},
		"docs/language/SEMANTICS.md": {ruleLike: 62, markers: map[string]int{
			"cep": 16, "cep-meta": 17, "cep-invalid": 3, "text": 15,
		}},
		"docs/language/errors.md": {ruleLike: 63, markers: map[string]int{
			"cep": 19, "cep-meta": 16, "cep-invalid": 9, "text": 1,
		}},
	}
	for _, file := range fragmentDocs {
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		md := string(data)
		got, want := censusOf(md), pins[file]
		if got.ruleLike != want.ruleLike {
			t.Errorf("%s: %d rule-like spans; pinned %d", file, got.ruleLike, want.ruleLike)
		}
		for _, form := range sortedKeys(got.markers, want.markers) {
			if got.markers[form] != want.markers[form] {
				t.Errorf("%s: %d markers %s; pinned %d", file, got.markers[form], form, want.markers[form])
			}
		}
		if got.ruleLike == 0 {
			t.Errorf("%s: no fragments", file)
		}
		for _, f := range checkMarkdown(file, md, invalid) {
			t.Error(f)
		}
	}
}

func sortedKeys(a, b map[string]int) []string {
	var out []string
	for _, m := range []map[string]int{a, b} {
		for k := range m {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	slices.Sort(out)
	return out
}
