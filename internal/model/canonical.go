package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// MarshalCanonical encodes the App as its canonical JSON (ADR-0003 §2). The rules, exactly:
//
//   - Every struct is a JSON object holding every one of its fields, always, zero values included ("explicit
//     defaults"), except the source positions: a field of type Pos or []Pos is never encoded. A key is the Go field
//     name in lowerCamelCase: its first letter lowercased, or a whole leading
//     initialism (OutputFirstEvery is "outputFirstEvery", TTL is "ttl", CronTZ is "cronTZ"). Keys are sorted
//     bytewise. There is no whitespace and no trailing newline.
//   - A slice is an array in its order; a nil slice is []. A string is a JSON string escaped as encoding/json
//     does with HTML escaping off. A bool is true or false. An int64 is its decimal digits.
//   - An event.Type is its name: "invalid", "string", "int", "float", "bool", "timestamp", "duration", "object".
//   - A float64 is strconv.FormatFloat(f, 'g', -1, 64): 0.75, 1e+21, -0. NaN and ±Inf are not encodable: an
//     error names the slot.
//   - In a Literal of kind "timestamp", the key "ms" holds the time as an RFC 3339 UTC string with exactly three
//     fraction digits ("2026-01-01T10:00:00.000Z"); everywhere else "ms" is an integer.
func (a App) MarshalCanonical() ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, reflect.ValueOf(a), "app"); err != nil { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		return nil, err
	}
	return buf.Bytes(), nil
}

// timestampLayout is RFC 3339 in UTC with exactly three fraction digits.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// errNotEncodable is wrapped by MarshalCanonical's error for a value JSON cannot hold.
var errNotEncodable = errors.New("model: not encodable")

func encode(buf *bytes.Buffer, v reflect.Value, path string) error { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	if t, ok := v.Interface().(event.Type); ok { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		return encodeString(buf, t.String())
	}
	switch v.Kind() { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	case reflect.Struct: //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		return encodeStruct(buf, v, path)
	case reflect.Slice: //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		buf.WriteByte('[')
		for i := range v.Len() { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encode(buf, v.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
				return err
			}
		}
		buf.WriteByte(']')
	case reflect.String: //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		return encodeString(buf, v.String()) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	case reflect.Bool: //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		buf.WriteString(strconv.FormatBool(v.Bool())) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	case reflect.Int64: //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		buf.WriteString(strconv.FormatInt(v.Int(), 10)) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	case reflect.Float64: //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		f := v.Float() //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%w: %s is %v", errNotEncodable, path, f)
		}
		buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	default:
		return fmt.Errorf("%w: %s has kind %s", errNotEncodable, path, v.Kind()) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	}
	return nil
}

func encodeStruct(buf *bytes.Buffer, v reflect.Value, path string) error { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	type field struct {
		key   string
		value reflect.Value //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	}
	fields := make([]field, 0, v.NumField()) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	for i := range v.NumField() {            //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		f := v.Type().Field(i)                                                      //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		if f.Type == reflect.TypeFor[Pos]() || f.Type == reflect.TypeFor[[]Pos]() { //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
			continue // positions are never canonical (ADR-0003 §1 as annotated)
		}
		fields = append(fields, field{lowerCamel(f.Name), v.Field(i)}) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	}
	slices.SortFunc(fields, func(a, b field) int { return strings.Compare(a.key, b.key) })
	stamp := v.Type() == reflect.TypeFor[Literal]() && v.Interface().(Literal).Kind == LitTimestamp //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
	buf.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeString(buf, f.key); err != nil {
			return err
		}
		buf.WriteByte(':')
		var err error
		if stamp && f.key == "ms" {
			err = encodeString(buf, time.UnixMilli(f.value.Int()).UTC().Format(timestampLayout)) //nolint:forbidigo,lll // PERMANENT reason="the canonical encoder walks the IR generically, so it cannot drift from the IR" ref=docs/architecture/adr/ADR-0003-rule-model-ir-and-parity.md#decision sprint=S1.1c-5
		} else {
			err = encode(buf, f.value, path+"."+f.key)
		}
		if err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

// lowerCamel lowercases a field name's first letter, or its whole leading initialism: TTL is "ttl".
func lowerCamel(name string) string {
	n := 0
	for n < len(name) && 'A' <= name[n] && name[n] <= 'Z' {
		n++
	}
	switch {
	case n == len(name):
		return strings.ToLower(name)
	case n > 1:
		n-- // the last capital starts the next word: URLPath is "urlPath"
	}
	return strings.ToLower(name[:n]) + name[n:]
}

// encodeString writes s as encoding/json does with HTML escaping off.
func encodeString(buf *bytes.Buffer, s string) error {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode ends with a newline
	return nil
}
