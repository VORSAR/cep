package event

import (
	"errors"
	"testing"
)

// TestDecodeAnUndeclaredTypeIsATypeFault reaches (*Decoder).value's last return (decode.go:200), the fault for an
// attribute whose type is none of the seven. The schema is a white-box literal: through the API the block is
// unreachable, since NewSchema rejects every type outside TString…TObject (event.go:120), so no Schema it returns
// holds such an attribute. The function keeps its signature; this test pins what the block reports if that
// contract ever changes.
func TestDecodeAnUndeclaredTypeIsATypeFault(t *testing.T) {
	t.Parallel()
	s := Schema{stream: "S", attrs: []Attr{{Name: "x", Type: TInvalid}}, timeAttr: ArrivalTime}
	vals, info, err := NewDecoder(s).Decode([]byte(`{"x":1}`), nil)
	want := &DecodeError{Reason: DecodeType, Attr: "x", Offset: 5,
		Msg: `attribute "x" (invalid) at offset 5: want a value of a declared type, found a number "1"`}
	got, isDecode := errors.AsType[*DecodeError](err)
	if !isDecode || *got != *want {
		t.Fatalf("Decode(%q) error = %#v; want %#v", `{"x":1}`, err, want)
	}
	if vals != nil || info != (Decoded{}) {
		t.Errorf("Decode(%q) = %v, %+v with its error; want nil values and a zero Decoded", `{"x":1}`, vals, info)
	}
}
