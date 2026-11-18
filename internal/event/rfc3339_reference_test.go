package event

import "testing"

// parseRFC3339Reference is a frozen copy of parseRFC3339 as it stood before S1.1c-6 split it (decode.go:817-888 at
// 22af094), with every package function and constant it uses copied beside it under a Reference suffix, so no change
// to decode.go can change the reference. TestParseRFC3339MatchesItsReference holds the live function to it.
//
//	date-time = 4DIGIT "-" 2DIGIT "-" 2DIGIT ("T"/"t") 2DIGIT ":" 2DIGIT ":" 2DIGIT ["." 1*DIGIT] ("Z"/"z"/("+"/"-")
//
// 2DIGIT ":" 2DIGIT)
func parseRFC3339Reference(t []byte) (int64, bool) {
	if len(t) < len("2006-01-02T15:04:05Z") || t[4] != '-' || t[7] != '-' || (t[10] != 'T' && t[10] != 't') ||
		t[13] != ':' || t[16] != ':' {
		return 0, false
	}
	year, ok1 := digitsAtReference(t, 0, 4)
	month, ok2 := digitsAtReference(t, 5, 2)
	day, ok3 := digitsAtReference(t, 8, 2)
	hour, ok4 := digitsAtReference(t, 11, 2)
	minute, ok5 := digitsAtReference(t, 14, 2)
	sec, ok6 := digitsAtReference(t, 17, 2)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 ||
		month < 1 || month > 12 || day < 1 || day > daysInReference(year, month) || hour > 23 || minute > 59 || sec > 60 {
		return 0, false
	}
	i := 19
	var ms int64
	if i < len(t) && t[i] == '.' {
		i++
		start := i
		for i < len(t) && isDigitReference(t[i]) {
			if i-start < 3 {
				ms = ms*10 + int64(t[i]-'0')
			}
			i++
		}
		switch i - start {
		case 0:
			return 0, false
		case 1:
			ms *= 100
		case 2:
			ms *= 10
		}
	}
	if i >= len(t) {
		return 0, false
	}
	var offset int64 // minutes east of UTC
	switch t[i] {
	case 'Z', 'z':
		i++
	case '+', '-':
		if len(t)-i != 6 || t[i+3] != ':' {
			return 0, false
		}
		oh, okh := digitsAtReference(t, i+1, 2)
		om, okm := digitsAtReference(t, i+4, 2)
		if !okh || !okm || oh > 23 || om > 59 {
			return 0, false
		}
		offset = oh*60 + om
		if t[i] == '-' {
			offset = -offset
		}
		i += 6
	default:
		return 0, false
	}
	if i != len(t) {
		return 0, false
	}
	if sec == 60 { // a leap second: the last millisecond of its minute
		sec, ms = 59, 999
	}
	secs := ((daysFromCivilReference(year, month, day)*24+hour)*60+minute-offset)*60 + sec
	v := secs*1000 + ms
	if v < minRFC3339Reference || v > maxRFC3339Reference { // an offset carried the instant out of RFC 3339's years
		return 0, false
	}
	return v, true
}

// The bounds parseRFC3339Reference keeps an instant inside (decode.go:893-896 at 22af094).
const (
	minRFC3339Reference int64 = -62167219200000
	maxRFC3339Reference int64 = 253402300799999
)

func digitsAtReference(t []byte, at, n int) (int64, bool) {
	var v int64
	for k := at; k < at+n; k++ {
		if !isDigitReference(t[k]) {
			return 0, false
		}
		v = v*10 + int64(t[k]-'0')
	}
	return v, true
}

func daysInReference(year, month int64) int64 {
	switch month {
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// daysFromCivilReference is the number of days from 1970-01-01 to the proleptic Gregorian date (Hinnant).
func daysFromCivilReference(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	mp := (m + 9) % 12 // March = 0
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func isDigitReference(c byte) bool { return c >= '0' && c <= '9' }

// rfc3339ReferenceSeeds are the 20 timestamps the generator mutates: accepted and refused forms across every branch
// of parseRFC3339 (the S1.1c-6 scratch differential's seeds).
var rfc3339ReferenceSeeds = []string{"1970-01-01T00:00:00Z", "1970-01-01t00:00:00z", "1972-02-29T00:00:00Z",
	"1970-01-01T00:00:60.5Z", "1970-01-01T00:00:00.1239Z", "1970-01-01T01:00:00.5+01:00", "1969-12-31T23:00:00-01:00",
	"1970-01-01T23:59:00+23:59", "0000-01-01T00:00:00Z", "0000-01-01T00:00:00+00:01", "9999-12-31T23:59:59.999Z",
	"9999-12-31T23:59:59.999-00:01", "2000-02-29T12:34:56.7-05:30", "1900-02-29T00:00:00Z",
	"2026-11-15T10:00:00.123456789Z", "1970-01-01T00:00:00.Z", "1970-01-01T00:00:00.12", "1970-01-01T01:00:00+01:000",
	"1970-01-01T00:00:00+24:00", "1970-01-01T00:00:00-00:60"}

// rfc3339ReferenceInputs is the generator's input count over rfc3339ReferenceSeeds, worked out from its loops: per
// seed of length L, L+1 truncations, 256 substitutions and 256 insertions at each of the L positions, and 21 × 16
// two-position substitutions for each of the L(L-1)/2 position pairs; summed over the 20 seeds' lengths.
const rfc3339ReferenceInputs = 2115410

// TestParseRFC3339MatchesItsReference: parseRFC3339 returns the frozen reference's value and ok for every
// truncation of each seed, every 1-byte substitution and insertion, and two-position substitutions drawn from the
// bytes a timestamp is built of (fixed seeds, so the run is deterministic).
func TestParseRFC3339MatchesItsReference(t *testing.T) {
	t.Parallel()
	n := 0
	check := func(b []byte) {
		n++
		wantMs, wantOk := parseRFC3339Reference(b)
		if ms, ok := parseRFC3339(b); ms != wantMs || ok != wantOk {
			t.Fatalf("parseRFC3339(%q) = %d, %v; the reference gives %d, %v", b, ms, ok, wantMs, wantOk)
		}
	}
	for _, s := range rfc3339ReferenceSeeds {
		b := []byte(s)
		for l := 0; l <= len(b); l++ {
			check(b[:l:l])
		}
		for i := range b {
			for v := range 256 {
				c := append([]byte(nil), b...)
				c[i] = byte(v)
				check(c)
				check(append(append(append([]byte(nil), b[:i]...), byte(v)), b[i:]...))
			}
			for j := i + 1; j < len(b); j++ {
				for _, v := range []byte("0123456789+-.:TZtz \x00\xff") {
					for _, w := range []byte("0123456789+-.:Zz") {
						c := append([]byte(nil), b...)
						c[i], c[j] = v, w
						check(c)
					}
				}
			}
		}
	}
	if n != rfc3339ReferenceInputs {
		t.Fatalf("the generator yielded %d inputs; want %d", n, rfc3339ReferenceInputs)
	}
}
