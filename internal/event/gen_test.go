package event_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// gen builds the differential's payloads for one schema (§4.3): a valid payload, then at most one mutation.
type gen struct {
	r *rand.Rand
	s event.Schema
}

// entry is one member of the payload object: key is the text between the key's quotes (it may hold escapes);
// val is raw JSON text, deliberately invalid in some mutations.
type entry struct{ key, val string }

// Valid JSON values per attribute type, before mutation.
var validFor = map[event.Type][]string{
	event.TString: {`""`, `"alice"`, `"a\"b\\c\/d\b\f\n\r\t"`, `"é中"`, `"😀"`, "\"é中😀\"",
		`"text_invisibles"`, `"null"`, `"5"`, `"​"`, `"{}"`, `"\u0000"`},
	event.TInt: {`0`, `-0`, `1`, `-1`, `42`, `9223372036854775807`, `-9223372036854775808`, `9007199254740993`,
		`-9007199254740993`, `9007199254740992`},
	event.TFloat: {`0`, `-0`, `-0.0`, `0.0`, `1.5`, `-2.25e-3`, `1E2`, `1e+2`, `1e308`, `1.7976931348623157e308`,
		`-1.7976931348623157e308`, `5e-324`, `2e-324`, `1e-400`, `9007199254740993`, `123456789012345678901234567890`,
		`0.1`, `2.2250738585072014e-308`, `42`},
	event.TBool: {`true`, `false`},
	event.TTimestamp: {`"2026-11-04T10:00:00Z"`, `"2026-11-04T10:00:00.123456789+02:00"`, `"1969-12-31T23:59:59.9995Z"`,
		`"1970-01-01T00:00:00Z"`, `"2026-11-04T10:00:00.1Z"`, `"2026-11-04T15:30:00+05:30"`, `"2026-11-04T05:00:00-05:00"`,
		`"0001-01-01T00:00:00Z"`, `"9999-12-31T23:59:59.999999999Z"`, `"2024-02-29T00:00:00Z"`,
		`"2026-11-04T10:00:00Z"`, `"1900-01-01T00:00:00.5-00:00"`},
	event.TDuration: {`0`, `90000`, `-1`, `9223372036854775807`, `-9223372036854775808`, `3600000`},
	event.TObject: {`{}`, `[]`, `{"a":[1, 2]}`, `[ 1 , {"b" : null} ]`, `{"a":{"b":{"c":[true,false,null,"é"]}}}`,
		"{\"k\":\"\xff\"}", `{"a":1,"a":2}`, `[[[]]]`, `{"text_invisibles":true}`, "{\n\t\"x\" : [ ]\n}"},
}

// Syntactically valid JSON values that are wrong for each attribute type (§4.1: no coercion; out of range).
var invalidFor = map[event.Type][]string{
	event.TString: {`5`, `-1.5`, `true`, `false`, `{}`, `[]`, `{"a":"b"}`, `["x"]`},
	event.TInt: {`1.0`, `-0.0`, `1e3`, `1E+2`, `9223372036854775808`, `-9223372036854775809`,
		`123456789012345678901234567890`, `0.5`, `"5"`, `true`, `{}`, `[]`, `1e400`},
	event.TFloat:     {`1e400`, `-1e400`, `1.7976931348623159e308`, `"1.5"`, `true`, `{}`, `[1.5]`, `"NaN"`},
	event.TBool:      {`"true"`, `1`, `0`, `{}`, `[]`, `"false"`},
	event.TTimestamp: {`1762243200123`, `0`, `"2026-02-30T00:00:00Z"`, `"bad"`, `""`, `true`, `{}`, `["2026-11-04T10:00:00Z"]`, `"2026-11-04"`},
	event.TDuration:  {`1.5`, `"90s"`, `9e4`, `9223372036854775808`, `true`, `{}`, `"90000"`},
	event.TObject:    {`"x"`, `"{}"`, `1`, `-0`, `true`, `false`, `""`},
}

// Values that break the document's syntax wherever they stand.
var syntaxBad = []string{`01`, `+1`, `.5`, `1.`, `-`, `1e`, `NaN`, `Infinity`, `-Infinity`, `True`, `nul`, `'a'`,
	`"abc`, `"\x"`, `"\u12"`, `[1,]`, `{"a":1,}`, `{"a"}`, `{1:2}`, `[1 2]`, `0x10`, `1_000`, ``, `-01`, `1.e3`,
	`1e+`, `"\uD83D\u"`, `tRue`, "\"a\tb\"", `"\U0041"`, `"\'"`}

// RFC 3339 edge forms (P1, owner D4): valid, invalid, and the ones time.Parse reads differently from the strict
// reader of design §4.2. The oracle (rfc3339Ms) decides.
var tsForms = []string{`"2026-11-04T10:00:00Z"`, `"2026-11-04t10:00:00Z"`, `"2026-11-04T10:00:00z"`,
	`"2026-11-04T10:00Z"`, `"2026-02-30T00:00:00Z"`, `"2026-11-04T10:00:00"`, `"2026-11-04 10:00:00Z"`,
	`"2026-11-04T10:00:00.Z"`, `"2026-11-04T10:00:00.1234567891Z"`, `"2026-11-04T10:00:00,123Z"`,
	`"2026-11-04T24:00:00Z"`, `"2026-11-04T23:59:60Z"`, `"2026-11-04T10:00:00+24:00"`, `"2026-11-04T10:00:00+0200"`,
	`"+2026-11-04T10:00:00Z"`, `"12026-11-04T10:00:00Z"`, `"2026-1-04T10:00:00Z"`, `" 2026-11-04T10:00:00Z"`,
	`"2026-11-04T10:00:00Z "`, `""`, `"2026-11-04T10:00:00-00:00"`, `"2026-11-04T10:00:00+23:59"`,
	`"2026-11-04T10:00:00+00:60"`, `"2026-13-01T00:00:00Z"`, `"2026-00-10T00:00:00Z"`, `"2026-11-00T00:00:00Z"`,
	`"2023-02-29T00:00:00Z"`, `"1900-02-29T00:00:00Z"`, `"2000-02-29T00:00:00Z"`, `"2026-11-04T10:00:00.000Z"`,
	`"2026-11-04T10:00:00.999999999-23:59"`, `"2026-11-04T1:00:00Z"`, `"0000-01-01T00:00:00Z"`,
	`"1969-12-31T23:59:59.999999999Z"`, `"1969-12-31T23:59:59.0000001Z"`, `"1960-06-15T12:34:56.7891-11:30"`,
	`"2026-11-04T10:00:00.1+02:00"`, `"2026-11-04T10:00:00.12Z"`, `"2026-11-04T10:00:00.123Z"`,
	`"2026-11-04T10:00:00.1234Z"`, `"2026-11-04T10:00:00.12345Z"`, `"2026-11-04T10:00:00.123456Z"`,
	`"2026-11-04T10:00:00.1234567Z"`, `"2026-11-04T10:00:00.12345678Z"`, `"2026-11-04T10:00:00.123456789Z"`,
	`"2026-11-04T10:00:00.5Z"`, "\"2026-11-04T10:00:00\xffZ\"", `"2026-11-04T10:00:00Z\u0000"`,
	`"2026-W45-3T10:00:00Z"`, `"20261104T100000Z"`, `1762243200123`, `1.7622432e12`,
	`"2026-11-04t10:00:00.5z"`, `"2016-12-31T23:59:60Z"`, `"2016-12-31T23:59:60.5+01:00"`, `"2016-12-31T23:59:60.999999z"`,
	`"2026-11-04T10:30:60-05:30"`, `"2026-11-04T10:00:61Z"`, `"2026-11-04T10:00:00,5Z"`, `"2026-11-04T10:0:00Z"`,
	`"2026-11-04T10:00:0Z"`, `"2026-11-4T10:00:00Z"`, `"2026-11-04T10:00:00+1:00"`, `"2026-11-04T10:00:00+25:00"`,
	`"2026-11-04T10:00:00-24:00"`, `"0000-02-29T00:00:00Z"`, `"2026-04-31T00:00:00Z"`, `"2026-11-04T10:00:00.9999999999Z"`,
	`"9999-12-31T23:59:59-23:59"`, `"0000-01-01T00:00:00+23:59"`, `"２026-11-04T10:00:00Z"`, `"2026-11-04T10:00:00Zz"`,
	`"2026-11-04T10:00:00+02:00Z"`, `"2026-11-04\u007410:00:00\u007a"`, `"2026-11-04T10:00:00.5"`}

var intExtremes = []string{`9223372036854775807`, `-9223372036854775808`, `9223372036854775808`,
	`-9223372036854775809`, `9007199254740993`, `-9007199254740993`, `-0`, `1.0`, `1e3`, `0e0`, `-0e-0`, `00`,
	`18446744073709551615`, `99999999999999999999`, `1E18`, `-1`, `0.0`}

var floatExtremes = []string{`1e400`, `-1e400`, `1e-400`, `-1e-400`, `5e-324`, `4.9e-324`, `2.4703282292062327e-324`,
	`2.4703282292062328e-324`, `1.7976931348623157e308`, `1.7976931348623158e308`, `1.7976931348623159e308`, `-0`,
	`-0.0`, `0e-999`, "1" + strings.Repeat("0", 400), "0." + strings.Repeat("0", 400) + "1",
	`123456789012345678901234567890e-10`, strings.Repeat("9", 800), `-0e0`, `0.30000000000000004`,
	`9007199254740993`, `9007199254740995`, `1e23`, `8.98846567431158e307`}

var escapedStrings = []string{`"\ud83d"`, `"\ude00x"`, `"\ud83dA"`, `"\ud83d\ud83d"`, `"😀"`,
	`"\u0000"`, `"\u001F"`, `"￿"`, `"￾"`, `"𐀀"`, `"􏿿"`, `"a\\u0041"`, `"\""`,
	`"\\"`, `"\"`, `"\ud83d\\ude00"`, `"\udc00\ud800"`, `"\u00"`, `"\uZZZZ"`, `"\ud83d\u12"`, `"\/\/"`}

var badUTF8 = []string{"\xff", "\xfe", "\x80", "\xc0\xaf", "\xe2\x82", "\xed\xa0\x80", "\xf4\x90\x80\x80",
	"\xf0\x9f\x98", "\xc3", "\xef\xbf\xbf", "\xef\xbb\xbf", "\xf8\x88\x80\x80\x80", "\xe0\x80\xaf"}

var unknownKeys = []string{"extra", "x", "", "V", "S", "user ", `user2`, `é`, "nested", "a.b",
	"text_invisible", "TS", `\ud83d`, "k\xff"}

var invisiblesKeys = []string{event.InvisiblesAttr, `text_invisibles`, `text_invisibles`}

var nestedValues = []string{`{"a":{"b":[1,2,{"c":null}]}}`, deep(50), deepObj(50), `[{"x":[{"y":[]}]}]`,
	"{\"\xff\":\"\xfe\"}", `{"a":"\ud83d"}`, `[1e400, 01e0]`}

var trailing = []string{" ", "\n", "\t\r\n ", "x", "{}", ",", "\x00", "]", "}", "null", " {\"a\":1}", "\f", "\xef\xbb\xbf",
	"/", "//c", "\v", "\xc2\xa0"}

var leading = []string{" ", "\n\t", "\r\n", "\xef\xbb\xbf", "\f", "\v", "\xc2\xa0", "x", "[", "\x00", " "}

var topLevel = []string{"", " ", "null", " null ", "[]", `[{}]`, `"s"`, `5`, `-0`, `true`, `false`, "{", "}", "{}",
	" {} ", `{"":1}`, `{"a":1}`, "[", `"`, "nul", "\xef\xbb\xbf{}", "{}{}", `{"a"}`, `{,}`, `{"a":1}}`, "\n"}

var flipBytes = []byte{'"', '\\', '{', '}', '[', ']', ',', ':', ' ', 'n', '0', '-', '.', 'e', 0x00, 0x1f, 0x7f, 0x80,
	0xff, 'u', 't', 'Z'}

var validWS = []string{"", "", "", "", "", " ", "\n", "\t", "\r\n", "  "}

// The mutation classes (§4.3), each applied at most once to a valid payload.
var classes = []string{"valid", "valid", "typeswap", "null", "absent", "repeat-valid-then-invalid",
	"repeat-invalid-then-valid", "repeat-valid-valid", "repeat-syntax", "unknown", "invisibles", "escapes", "utf8",
	"control", "int-extreme", "float-extreme", "rfc3339", "nesting", "trailing", "leading", "toplevel", "truncate",
	"byte-flip", "byte-delete", "byte-insert", "key-escape", "key-case", "syntax-value"}

func (g *gen) pick(xs []string) string { return xs[g.r.IntN(len(xs))] }
func (g *gen) chance(p float64) bool   { return g.r.Float64() < p }

// value is a valid JSON value for type t: from validFor, or random.
func (g *gen) value(t event.Type) string {
	if g.chance(0.6) {
		return g.pick(validFor[t])
	}
	switch t {
	case event.TString:
		var sb strings.Builder
		sb.WriteByte('"')
		for range g.r.IntN(8) {
			sb.WriteString(g.pick([]string{"a", "Z", "0", " ", "é", "中", "😀", `\n`, `\"`, `\\`, `é`,
				`😀`, `​`, "\x7f", `\/`}))
		}
		sb.WriteByte('"')
		return sb.String()
	case event.TInt, event.TDuration:
		if g.chance(0.5) {
			return strconv.FormatInt(int64(g.r.Uint64()), 10)
		}
		return strconv.FormatInt(g.r.Int64N(2000000)-1000000, 10)
	case event.TFloat:
		for {
			f := math.Float64frombits(g.r.Uint64())
			if g.chance(0.5) {
				f = g.r.NormFloat64() * 1e6
			}
			if !math.IsNaN(f) && !math.IsInf(f, 0) {
				return strconv.FormatFloat(f, g.pick([]string{"g", "e", "f"})[0], -1, 64)
			}
		}
	case event.TBool:
		return g.pick(validFor[t])
	case event.TTimestamp:
		// Any instant in years 0001-9999, any whole-minute offset within +-23:59, 0-9 fraction digits.
		lo, hi := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC).Unix()
		tm := time.Unix(lo+g.r.Int64N(hi-lo), g.r.Int64N(1e9))
		zone := time.FixedZone("", (g.r.IntN(2*1439+1)-1439)*60)
		layout := g.pick([]string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05.000Z07:00", "2006-01-02T15:04:05.000000Z07:00",
			"2006-01-02T15:04:05.000000000Z07:00"})
		s := tm.In(zone).Format(layout)
		// The strict reader's own forms (owner D4): lowercase t and z, and a leap second in any minute.
		if g.chance(0.15) {
			s = s[:10] + "t" + s[11:]
		}
		if strings.HasSuffix(s, "Z") && g.chance(0.3) {
			s = s[:len(s)-1] + "z"
		}
		if g.chance(0.05) {
			s = s[:17] + "60" + s[19:]
		}
		return `"` + s + `"`
	case event.TObject:
		return g.pick(append(slices.Clone(validFor[t]), nestedValues[:4]...))
	}
	panic(fmt.Sprintf("gen: no values for %v", t))
}

func (g *gen) anyValue() string {
	switch g.r.IntN(4) {
	case 0:
		return g.pick(nestedValues[:4])
	case 1:
		return "null"
	default:
		return g.value(event.Type(1 + g.r.IntN(7)))
	}
}

// base is a valid payload's members, in a random order. An optional attribute is sometimes absent or null; the
// time attribute and the required ones are always present and valid; a declared text_invisibles is left out.
func (g *gen) base() []entry {
	var es []entry
	for i := range g.s.NumAttrs() {
		a := g.s.Attr(i)
		if a.Name == event.InvisiblesAttr {
			continue
		}
		if a.Optional && i != g.s.TimeAttr() {
			switch {
			case g.chance(0.15):
				continue
			case g.chance(0.12):
				es = append(es, entry{a.Name, "null"})
				continue
			}
		}
		es = append(es, entry{a.Name, g.value(a.Type)})
	}
	g.r.Shuffle(len(es), func(i, j int) { es[i], es[j] = es[j], es[i] })
	return es
}

// attr picks a declared attribute (not text_invisibles) for which keep holds, or returns false.
func (g *gen) attr(keep func(event.Attr) bool) (event.Attr, bool) {
	var cands []event.Attr
	for i := range g.s.NumAttrs() {
		if a := g.s.Attr(i); a.Name != event.InvisiblesAttr && keep(a) {
			cands = append(cands, a)
		}
	}
	if len(cands) == 0 {
		return event.Attr{}, false
	}
	return cands[g.r.IntN(len(cands))], true
}

func anyAttr(event.Attr) bool { return true }

func ofType(ts ...event.Type) func(event.Attr) bool {
	return func(a event.Attr) bool { return slices.Contains(ts, a.Type) }
}

// set gives attribute name the value val: its existing member is replaced, or one is inserted at random.
func (g *gen) set(es []entry, name, val string) []entry {
	for i := range es {
		if es[i].key == name {
			es[i].val = val
			return es
		}
	}
	return g.insert(es, entry{name, val})
}

func (g *gen) insert(es []entry, e entry) []entry {
	return slices.Insert(es, g.r.IntN(len(es)+1), e)
}

func without(es []entry, name string) []entry {
	return slices.DeleteFunc(es, func(e entry) bool { return e.key == name })
}

// twice puts first, then later, under name, at random positions in that order.
func (g *gen) twice(es []entry, name, first, later string) []entry {
	es = without(es, name)
	p := g.r.IntN(len(es) + 1)
	es = slices.Insert(es, p, entry{name, first})
	q := p + 1 + g.r.IntN(len(es)-p)
	return slices.Insert(es, q, entry{name, later})
}

func (g *gen) render(es []entry) []byte {
	var sb strings.Builder
	sb.WriteString(g.pick(validWS))
	sb.WriteByte('{')
	for i, e := range es {
		if i > 0 {
			sb.WriteString(g.pick(validWS))
			sb.WriteByte(',')
		}
		sb.WriteString(g.pick(validWS))
		sb.WriteString(`"` + e.key + `"`)
		sb.WriteString(g.pick(validWS))
		sb.WriteByte(':')
		sb.WriteString(g.pick(validWS))
		sb.WriteString(e.val)
	}
	sb.WriteString(g.pick(validWS))
	sb.WriteByte('}')
	sb.WriteString(g.pick(validWS))
	return []byte(sb.String())
}

// escapeKey writes one character of an ASCII key as a \u escape, in either hex case.
func (g *gen) escapeKey(k string) string {
	if k == "" {
		return k
	}
	i := g.r.IntN(len(k))
	format := g.pick([]string{`\u%04x`, `\u%04X`})
	return k[:i] + fmt.Sprintf(format, k[i]) + k[i+1:]
}

// payload builds one payload and names its mutation class.
func (g *gen) payload() (string, []byte) {
	es := g.base()
	class := g.pick(classes)
	switch class {
	case "typeswap":
		if a, ok := g.attr(anyAttr); ok {
			es = g.set(es, a.Name, g.pick(invalidFor[a.Type]))
		}
	case "null":
		if a, ok := g.attr(anyAttr); ok {
			es = g.set(es, a.Name, "null")
		}
	case "absent":
		if a, ok := g.attr(anyAttr); ok {
			es = without(es, a.Name)
		}
	case "repeat-valid-then-invalid":
		if a, ok := g.attr(anyAttr); ok {
			es = g.twice(es, a.Name, g.value(a.Type), g.pick(append(slices.Clone(invalidFor[a.Type]), "null")))
		}
	case "repeat-invalid-then-valid":
		if a, ok := g.attr(anyAttr); ok {
			es = g.twice(es, a.Name, g.pick(append(slices.Clone(invalidFor[a.Type]), "null", nestedValues[0])), g.value(a.Type))
		}
	case "repeat-valid-valid":
		if a, ok := g.attr(anyAttr); ok {
			es = g.twice(es, a.Name, g.value(a.Type), g.value(a.Type))
		}
	case "repeat-syntax":
		if a, ok := g.attr(anyAttr); ok {
			es = g.twice(es, a.Name, g.pick(syntaxBad), g.value(a.Type))
		}
	case "unknown":
		for range 1 + g.r.IntN(3) {
			es = g.insert(es, entry{g.pick(unknownKeys), g.anyValue()})
		}
	case "invisibles":
		for range 1 + g.r.IntN(2) {
			es = g.insert(es, entry{g.pick(invisiblesKeys), g.anyValue()})
		}
	case "escapes":
		if a, ok := g.attr(ofType(event.TString)); ok && g.chance(0.8) {
			es = g.set(es, a.Name, g.pick(escapedStrings))
		} else {
			es = g.insert(es, entry{g.pick(unknownKeys), g.pick(escapedStrings)})
		}
	case "utf8":
		bad := g.pick(badUTF8)
		switch a, ok := g.attr(ofType(event.TString, event.TTimestamp, event.TObject)); {
		case ok && a.Type == event.TString:
			es = g.set(es, a.Name, `"a`+bad+`b"`)
		case ok && a.Type == event.TTimestamp:
			es = g.set(es, a.Name, `"2026-11-04T10:00:00`+bad+`Z"`)
		case ok && a.Type == event.TObject:
			es = g.set(es, a.Name, `{"k`+bad+`":"`+bad+`"}`)
		default:
			es = g.insert(es, entry{"k" + bad, `"` + bad + `"`})
		}
	case "control":
		c := g.r.IntN(32)
		val := fmt.Sprintf(`"a\u%04x"`, c)
		if g.chance(0.5) {
			val = "\"a" + string(rune(c)) + "\""
		}
		if a, ok := g.attr(ofType(event.TString)); ok {
			es = g.set(es, a.Name, val)
		} else {
			es = g.insert(es, entry{"x", val})
		}
	case "int-extreme":
		if a, ok := g.attr(ofType(event.TInt, event.TDuration)); ok {
			es = g.set(es, a.Name, g.pick(intExtremes))
		}
	case "float-extreme":
		if a, ok := g.attr(ofType(event.TFloat)); ok {
			es = g.set(es, a.Name, g.pick(floatExtremes))
		}
	case "rfc3339":
		if a, ok := g.attr(ofType(event.TTimestamp)); ok {
			es = g.set(es, a.Name, g.pick(tsForms))
		}
	case "nesting":
		// The top-level object is one level: a value nested 9999 deep is the deepest encoding/json accepts.
		n := 9998 + g.r.IntN(4)
		v := deep(n)
		if g.chance(0.4) {
			v = deepObj(n)
		}
		if a, ok := g.attr(ofType(event.TObject)); ok && g.chance(0.6) {
			es = g.set(es, a.Name, v)
		} else {
			es = g.insert(es, entry{"deep", v})
		}
	case "key-escape":
		if a, ok := g.attr(anyAttr); ok {
			for i := range es {
				if es[i].key == a.Name {
					es[i].key = g.escapeKey(a.Name)
				}
			}
		}
	case "key-case":
		if a, ok := g.attr(anyAttr); ok {
			for i := range es {
				if es[i].key == a.Name {
					es[i].key = strings.ToUpper(a.Name[:1]) + a.Name[1:]
				}
			}
		}
	case "syntax-value":
		if a, ok := g.attr(anyAttr); ok {
			es = g.set(es, a.Name, g.pick(syntaxBad))
		}
	}
	b := g.render(es)
	switch class {
	case "trailing":
		b = append(b, g.pick(trailing)...)
	case "leading":
		b = append([]byte(g.pick(leading)), b...)
	case "toplevel":
		b = []byte(g.pick(topLevel))
	case "truncate":
		b = b[:g.r.IntN(len(b))]
	case "byte-flip":
		b[g.r.IntN(len(b))] = flipBytes[g.r.IntN(len(flipBytes))]
	case "byte-delete":
		i := g.r.IntN(len(b))
		b = append(b[:i], b[i+1:]...)
	case "byte-insert":
		b = slices.Insert(b, g.r.IntN(len(b)+1), flipBytes[g.r.IntN(len(flipBytes))])
	}
	return class, b
}
