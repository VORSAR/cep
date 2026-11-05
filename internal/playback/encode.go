package playback

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/vorsar/cep/internal/event"
)

// The RFC 3339 range a canonical timestamp can be written in: 0000-01-01T00:00:00.000Z to 9999-12-31T23:59:59.999Z,
// in Unix milliseconds.
const (
	minTimestamp int64 = -62167219200000
	maxTimestamp int64 = 253402300799999
)

// timestampLayout is RFC 3339 in UTC with exactly three fraction digits (ADR-0003's form).
const timestampLayout = "2006-01-02T15:04:05.000Z"

var (
	errNonFinite = errors.New("a NaN or infinite float is not encodable")
	errTimeRange = errors.New("a timestamp outside the years 0000-9999 is not encodable in RFC 3339")
)

// AppendValue appends v's canonical JSON (design note §5.4). Error: a NaN or ±Inf float; a timestamp outside the
// years 0000–9999; a value of no type.
func AppendValue(dst []byte, v event.Value) ([]byte, error) {
	if v.Null && v.Kind != event.TInvalid {
		return append(dst, "null"...), nil
	}
	switch v.Kind {
	case event.TString:
		return appendString(dst, v.Str), nil
	case event.TInt:
		return strconv.AppendInt(dst, v.I, 10), nil
	case event.TFloat:
		if math.IsNaN(v.F) || math.IsInf(v.F, 0) {
			return dst, errNonFinite
		}
		return strconv.AppendFloat(dst, v.F, 'g', -1, 64), nil
	case event.TBool:
		return strconv.AppendBool(dst, v.B), nil
	case event.TTimestamp:
		if v.T < minTimestamp || v.T > maxTimestamp {
			return dst, errTimeRange
		}
		return appendTime(dst, v.T), nil
	case event.TDuration:
		return strconv.AppendInt(dst, v.D, 10), nil
	case event.TObject:
		text, ok := v.Obj.(string)
		if !ok {
			return dst, fmt.Errorf("an object value is not its JSON text (%T)", v.Obj)
		}
		return append(dst, text...), nil
	}
	return dst, fmt.Errorf("a value of type %v is not encodable", v.Kind)
}

// appendTime writes ms as a quoted RFC 3339 UTC timestamp with exactly three fraction digits.
func appendTime(dst []byte, ms int64) []byte {
	dst = append(dst, '"')
	dst = time.UnixMilli(ms).UTC().AppendFormat(dst, timestampLayout)
	return append(dst, '"')
}

const hexDigits = "0123456789abcdef"

// appendString writes s as a JSON string, escaped as encoding/json does with HTML escaping off (design note §5.4):
// `"` and `\`; \b \f \n \r \t short; every other byte below 0x20 as \u00XX; U+2028 and U+2029 escaped; DEL and every
// other rune raw; malformed UTF-8 as U+FFFD.
func appendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch c {
			case '"', '\\':
				dst = append(dst, '\\', c)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			dst = append(dst, s[start:i]...)
			dst = append(dst, "�"...)
		case r == ' ' || r == ' ':
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hexDigits[r&0xf])
		default:
			i += size
			continue
		}
		i += size
		start = i
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// AppendEvent appends e as one log line without a newline: {"attrs":{…},"batch":"…","stream":"…","ts":"…"}; batch
// only when set, ts only when s has no time attribute (owner D2 of S3.2a-1). Keys are sorted bytewise; every declared
// attribute appears, a null as null. It allocates nothing into a buffer with room, for up to 64 attributes.
func AppendEvent(dst []byte, s event.Schema, e event.Event) ([]byte, error) {
	n := s.NumAttrs()
	if len(e.Values) != n {
		return dst, fmt.Errorf("playback: stream %q: an event with %d values for %d attributes", s.Stream(), len(e.Values), n)
	}
	var stack [64]int
	order := stack[:0]
	if n > len(stack) {
		order = make([]int, 0, n)
	}
	for i := range n { // insertion sort by name: attribute lists are short
		order = append(order, i)
		for j := len(order) - 1; j > 0 && s.Attr(order[j]).Name < s.Attr(order[j-1]).Name; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	dst = append(dst, `{"attrs":{`...)
	for k, i := range order {
		a := s.Attr(i)
		v := e.Values[i]
		if v.Kind != a.Type {
			return dst, fmt.Errorf("playback: stream %q: attribute %q holds a %v value; it is declared %v", s.Stream(), a.Name, v.Kind, a.Type)
		}
		if k > 0 {
			dst = append(dst, ',')
		}
		dst = appendString(dst, a.Name)
		dst = append(dst, ':')
		var err error
		if dst, err = AppendValue(dst, v); err != nil {
			return dst, fmt.Errorf("playback: stream %q: attribute %q: %w", s.Stream(), a.Name, err)
		}
	}
	dst = append(dst, '}')
	if e.BatchID != "" {
		dst = append(dst, `,"batch":`...)
		dst = appendString(dst, e.BatchID)
	}
	dst = append(dst, `,"stream":`...)
	dst = appendString(dst, s.Stream())
	if s.TimeAttr() == event.ArrivalTime {
		if e.Ts < minTimestamp || e.Ts > maxTimestamp {
			return dst, fmt.Errorf("playback: stream %q: ts: %w", s.Stream(), errTimeRange)
		}
		dst = append(dst, `,"ts":`...)
		dst = appendTime(dst, e.Ts)
	}
	return append(dst, '}'), nil
}
