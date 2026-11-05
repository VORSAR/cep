package event_test

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// Owner D4 of S3.2a-1 (design note s3-2a-1 §5.6): an object value is stored as its JSON text in compact form. Every
// whitespace byte outside a string is removed (JSON whitespace: space, tab, LF, CR); member order and every token's
// text are kept, a string's escapes included; malformed UTF-8 inside a string becomes U+FFFD, as P8 does for a
// string attribute (one U+FFFD per byte that does not start a valid rune, as encoding/json's Unmarshal does). Each
// expected text below is derived by hand from those rules.

// objCase is one payload's object attribute and the text D4 stores.
type objCase struct {
	name string
	in   string // the object attribute's JSON text, placed as {"o":<in>}
	want string // Value.Obj after the decode
}

var objCases = []objCase{
	{
		// Pretty-printed with LF, tabs, spaces and CR between every token, nested objects and arrays.
		name: "pretty-printed",
		in:   "{\n\t\"id\" : 7 ,\n\t\"tags\" : [ \"a\" , \"b\" ] ,\r\n\t\"geo\" : {\n\t\t\"lat\" : 1.5 ,\n\t\t\"pts\" : [ [ 1 , 2 ] , [ ] , { } ]\n\t}\n}",
		want: `{"id":7,"tags":["a","b"],"geo":{"lat":1.5,"pts":[[1,2],[],{}]}}`,
	},
	{
		// Members stay in payload order (not sorted), a repeated inner key included.
		name: "member order kept",
		in:   `{ "z" : 1 , "a" : 2 , "m" : [ 3 ] , "a" : 4 }`,
		want: `{"z":1,"a":2,"m":[3],"a":4}`,
	},
	{
		// Escapes stay exactly as written: \u00e9 is not turned into é, a raw é is not escaped, \/ is not
		// simplified, and the escaped key stays escaped.
		name: "string escapes kept as written",
		in:   `{ "k" : "\u00e9 é \n \" \\ \/ \t \ud83d\ude00" , "\u0041" : "\u2028" }`,
		want: `{"k":"\u00e9 é \n \" \\ \/ \t \ud83d\ude00","\u0041":"\u2028"}`,
	},
	{
		// Whitespace inside a string, in keys and values, is string content: kept.
		name: "whitespace inside strings kept",
		in:   `{ "a b" : "  x   y  " , "c" : [ " " , "" ] }`,
		want: `{"a b":"  x   y  ","c":[" ",""]}`,
	},
	{
		// Every token's text is kept: a number is not reformatted (1.50 stays 1.50, 1E+2 stays 1E+2, -0 stays -0).
		name: "number text kept",
		in:   `[ 1.50 , 1E+2 , -0 , 0.0e-0 , true , false , null ]`,
		want: `[1.50,1E+2,-0,0.0e-0,true,false,null]`,
	},
	{
		// "\xff": one bad byte, one U+FFFD. "\xed\xa0\x80" is a UTF-8-encoded surrogate: none of its three bytes
		// starts a valid rune, so three U+FFFD. "\xe2\x82" is a truncated 3-byte sequence: two U+FFFD. In the key
		// too. Each U+FFFD is the raw three bytes EF BF BD, not an escape.
		name: "malformed UTF-8 replaced",
		in:   "{ \"k\xff\" : \"a\xffb\" , \"s\" : \"\xed\xa0\x80\" , \"t\" : [ \"\xe2\x82\" ] }",
		want: "{\"k\uFFFD\":\"a\uFFFDb\",\"s\":\"\uFFFD\uFFFD\uFFFD\",\"t\":[\"\uFFFD\uFFFD\"]}",
	},
	{
		// Valid UTF-8 beyond ASCII is kept: a 2-, 3- and 4-byte rune.
		name: "valid UTF-8 kept",
		in:   `{ "é" : "€ 😀" }`,
		want: `{"é":"€ 😀"}`,
	},
	{
		// Already compact and valid: stored unchanged (green before D4 too; it guards the change).
		name: "already compact unchanged",
		in:   `{"a":[1,{"b":null}],"c":"x y","d":"\n"}`,
		want: `{"a":[1,{"b":null}],"c":"x y","d":"\n"}`,
	},
	{
		// An array is an object attribute's top value too.
		name: "array top value",
		in:   "[ 1 ,\n [ 2 , { } ] , \"s t\" ,\t{ \"k\" : [ ] } ]",
		want: `[1,[2,{}],"s t",{"k":[]}]`,
	},
}

// TestObjectStoredCompact decodes each case through the real Decoder and compares the stored text.
func TestObjectStoredCompact(t *testing.T) {
	s := mustSchema(t, "O", event.ArrivalTime, req("o", event.TObject))
	d := event.NewDecoder(s)
	for _, c := range objCases {
		t.Run(c.name, func(t *testing.T) {
			payload := `{"o":` + c.in + `}`
			got, _, err := d.Decode([]byte(payload), nil)
			if err != nil {
				t.Fatalf("Decode(%q): %v", payload, err)
			}
			want := vObj(c.want)
			if len(got) != 1 || !sameValue(got[0], want) {
				t.Fatalf("Decode(%q):\n got  %v\n want %s", payload, got, showValue(want))
			}
		})
	}
}

// TestObjectMalformedUTF8AsString checks D4's "as P8 does for string attributes": the bytes a string attribute
// replaces are the bytes an object's string replaces. "a\xffb\xed\xa0\x80\xe2\x82" decodes, as a string attribute,
// to "a" U+FFFD "b" then five U+FFFD (P8, derived by hand as above), and inside an object to the same runes.
func TestObjectMalformedUTF8AsString(t *testing.T) {
	const bad = "a\xffb\xed\xa0\x80\xe2\x82"
	const replaced = "a\uFFFDb\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD"
	s := mustSchema(t, "SO", event.ArrivalTime, req("s", event.TString), req("o", event.TObject))
	got, _, err := event.NewDecoder(s).Decode([]byte(`{"s":"`+bad+`","o":["`+bad+`"]}`), nil)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got[0].Str != replaced {
		t.Fatalf("string attribute: got %q, want %q (P8)", got[0].Str, replaced)
	}
	if want := `["` + replaced + `"]`; !sameValue(got[1], vObj(want)) {
		t.Fatalf("object attribute: got %s, want Obj %q (D4)", showValue(got[1]), want)
	}
}
