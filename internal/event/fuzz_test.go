package event_test

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

// FuzzDecode is the decoder's fuzz target (design note s3-1-2 §4.3; its nightly row is fuzz-decoder, P7). Any
// bytes, over every schema the decoder's tests use: the decoder never panics, a rejection is a well-formed
// *DecodeError, and it agrees with §4.1's oracle as the differential does. The seeds are the hand-written cases
// and the generator's fixed edge forms.
func FuzzDecode(f *testing.F) {
	for _, c := range handCases {
		f.Add([]byte(c.in))
	}
	for _, list := range [][]string{topLevel, syntaxBad, tsForms} {
		for _, v := range list {
			f.Add([]byte(v))
			f.Add([]byte(`{"ts":` + v + `,"user":"u","score":1}`))
		}
	}
	schemas := testSchemas(f)
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	slices.Sort(names)
	decoders := make([]*event.Decoder, len(names))
	for i, name := range names {
		decoders[i] = newDecoderSafe(f, schemas[name])
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		p := oracleParse(b)
		for i, name := range names {
			s := schemas[name]
			got := decodeSafe(decoders[i], b, nil)
			if msg := disagreement(s, b, oracle(s, p), got); msg != "" {
				t.Fatalf("schema %s: Decode(%s): %s", name, clip(b), msg)
			}
		}
	})
}
