package event_test

// Break-it K2 of S3.2a-1 (owner, 2026-11-05: refuse at decode): a timestamp is read by its UTC instant, which must lie
// in RFC 3339's range, 0000-01-01T00:00:00.000Z to 9999-12-31T23:59:59.999Z, so every value the decoder accepts can
// be written back. The reader checked the year before applying the offset, so an offset could carry an accepted
// timestamp outside that range.

import (
	"errors"
	"testing"

	"github.com/vorsar/cep/internal/event"
)

func TestTimestampUTCInstantInRFC3339Range(t *testing.T) {
	t.Parallel()
	s, err := event.NewSchema("S", []event.Attr{{Name: "ts", Type: event.TTimestamp}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	d := event.NewDecoder(s)
	cases := []struct {
		ts   string
		ok   bool
		want int64 // Unix ms, when ok
	}{
		// The two ends, in UTC: accepted. Year 0 starts at -62167219200 s; 9999 ends at 253402300799.999 s.
		{"0000-01-01T00:00:00Z", true, -62167219200000},
		{"9999-12-31T23:59:59.999Z", true, 253402300799999},
		// An offset that lands exactly on an end: 00:01+00:01 is 00:00 UTC; 23:58:59.999-00:01 is 23:59:59.999 UTC.
		{"0000-01-01T00:01:00+00:01", true, -62167219200000},
		{"9999-12-31T23:58:59.999-00:01", true, 253402300799999},
		// One minute past either end once the offset is applied: refused.
		{"0000-01-01T00:00:00+00:01", false, 0},     // 31 Dec of year -1, 23:59 UTC
		{"9999-12-31T23:59:59.999-00:01", false, 0}, // 1 Jan 10000, 00:00:59.999 UTC
		{"0000-01-01T05:00:00+23:59", false, 0},     // year -1 by almost a day
		{"9999-12-31T20:00:00-23:59", false, 0},     // year 10000 by almost a day
	}
	for _, c := range cases {
		vals, _, err := d.Decode([]byte(`{"ts":"`+c.ts+`"}`), nil)
		if c.ok {
			if err != nil {
				t.Errorf("%s: refused (%v), want %d", c.ts, err, c.want)
			} else if vals[0].T != c.want {
				t.Errorf("%s: %d, want %d", c.ts, vals[0].T, c.want)
			}
			continue
		}
		var de *event.DecodeError
		if !errors.As(err, &de) || de.Reason != event.DecodeType || de.Attr != "ts" {
			t.Errorf("%s: (%v, %v), want a DecodeType error on ts: its UTC instant is outside RFC 3339's years", c.ts, vals, err)
		}
	}
}
