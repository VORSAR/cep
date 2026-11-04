package event

// InvisiblesAttr is the engine-owned attribute name (SPEC §4.2, rev 3): a payload value under it is ignored.
const InvisiblesAttr = "text_invisibles"

// Decoder decodes one JSON object per call into one stream's values (ADR-0010 §3; design note s3-1-2 §4). It is
// immutable after NewDecoder and safe for concurrent use.
type Decoder struct {
	schema Schema
}

// NewDecoder builds the decoder for s.
func NewDecoder(s Schema) *Decoder {
	panic("event.NewDecoder: not built yet (S3.1-2 stub)")
}

// Decoded is what a successful decode reports besides the values.
type Decoded struct {
	Ts       int64 // the event time from the schema's time attribute; 0 for a `time arrival` stream
	Unknown  int   // payload keys the schema does not declare, each occurrence: skipped
	Reserved bool  // the payload carried InvisiblesAttr: ignored
}

// Decode appends the schema's NumAttrs values to dst[:0], in attribute order, and returns them.
func (d *Decoder) Decode(b []byte, dst []Value) ([]Value, Decoded, error) {
	panic("event.Decoder.Decode: not built yet (S3.1-2 stub)")
}

// DecodeReason classifies a rejected payload.
type DecodeReason uint8

// The reasons a payload is rejected (design note s3-1-2 §4).
const (
	DecodeSyntax  DecodeReason = iota + 1 // not one JSON object, trailing data, nesting deeper than 10000
	DecodeType                            // a value of the wrong JSON type or out of range for its attribute
	DecodeMissing                         // a non-optional attribute, or the time attribute, absent
	DecodeNull                            // a non-optional attribute, or the time attribute, null
)

// DecodeError is a rejected payload: an event-level error (SEMANTICS §10).
type DecodeError struct {
	Reason DecodeReason
	Attr   string // the attribute's name; "" for DecodeSyntax
	Offset int    // the byte offset in the payload where the fault starts
	Msg    string
}

func (e *DecodeError) Error() string { return "event: decode: " + e.Msg }
