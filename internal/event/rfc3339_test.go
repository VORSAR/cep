package event_test

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// rfc3339Ms is the oracle's timestamp step: design note s3-1-2 v2 §4.2's strict RFC 3339 reader (owner D4),
// written here independently of package time and of the decoder. It returns the instant in ms since
// 1970-01-01T00:00:00Z, or false.
//
//	date-time   = full-date ("T" / "t") full-time
//	full-date   = 4DIGIT "-" 2DIGIT "-" 2DIGIT          ; month 01-12, day by month and leap year
//	full-time   = 2DIGIT ":" 2DIGIT ":" 2DIGIT ["." 1*DIGIT] time-offset   ; hour 00-23, minute 00-59, second 00-60
//	time-offset = "Z" / "z" / ("+" / "-") 2DIGIT ":" 2DIGIT                ; hour 00-23, minute 00-59
//
// Fraction digits past the third are dropped (rounding toward -inf, the fraction being added); a leap second
// :60 reads as that minute's :59.999, whatever its fraction.
func rfc3339Ms(s string) (int64, bool) {
	num := func(i, n int) (int64, bool) {
		if i+n > len(s) {
			return 0, false
		}
		var v int64
		for _, c := range []byte(s[i : i+n]) {
			if c < '0' || c > '9' {
				return 0, false
			}
			v = v*10 + int64(c-'0')
		}
		return v, true
	}
	at := func(i int, c ...byte) bool { return i < len(s) && strings.IndexByte(string(c), s[i]) >= 0 }
	y, ok1 := num(0, 4)
	mo, ok2 := num(5, 2)
	d, ok3 := num(8, 2)
	h, ok4 := num(11, 2)
	mi, ok5 := num(14, 2)
	sec, ok6 := num(17, 2)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !at(4, '-') || !at(7, '-') || !at(10, 'T', 't') ||
		!at(13, ':') || !at(16, ':') {
		return 0, false
	}
	i := 19
	var frac int64 // ms
	if at(i, '.') {
		i++
		start := i
		for at(i, '0', '1', '2', '3', '4', '5', '6', '7', '8', '9') {
			if i-start < 3 {
				frac = frac*10 + int64(s[i]-'0')
			}
			i++
		}
		if i == start {
			return 0, false
		}
		for n := i - start; n < 3; n++ {
			frac *= 10
		}
	}
	var off int64 // seconds east of UTC
	switch rest := s[i:]; {
	case rest == "Z" || rest == "z":
	case len(rest) == 6 && (rest[0] == '+' || rest[0] == '-') && rest[3] == ':':
		oh, okh := num(i+1, 2)
		om, okm := num(i+4, 2)
		if !okh || !okm || oh > 23 || om > 59 {
			return 0, false
		}
		off = oh*3600 + om*60
		if rest[0] == '-' {
			off = -off
		}
	default:
		return 0, false
	}
	if mo < 1 || mo > 12 || d < 1 || d > daysIn(y, mo) || h > 23 || mi > 59 || sec > 60 {
		return 0, false
	}
	if sec == 60 {
		sec, frac = 59, 999
	}
	secs := daysFromCivil(y, mo, d)*86400 + h*3600 + mi*60 + sec - off
	return secs*1000 + frac, true
}

func daysIn(y, m int64) int64 {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// daysFromCivil is the proleptic Gregorian day number of y-m-d relative to 1970-01-01 (H. Hinnant's algorithm).
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	doy := (153*((m+9)%12)+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// rfc3339Disagreements documents every class of form on which the strict reference and
// time.Parse(time.RFC3339Nano, s) disagree (§4.2: checked against time.Parse where both accept, against the
// grammar where they do not). refOnly: the reference accepts and time.Parse refuses; otherwise the reverse.
var rfc3339Disagreements = []struct {
	name    string
	refOnly bool
	match   func(s string) bool
}{
	{"lowercase t or z: RFC 3339 §5.6 allows them, time.Parse refuses them", true,
		func(s string) bool { return len(s) > 10 && (s[10] == 't' || strings.HasSuffix(s, "z")) }},
	{"a leap second :60: RFC 3339 allows it, time.Parse refuses it", true,
		func(s string) bool { return len(s) > 18 && s[17:19] == "60" }},
	{"a comma before the fraction: time.Parse accepts it, RFC 3339 does not", false,
		func(s string) bool { return strings.Contains(s, ",") }},
	{"a one-digit hour: time.Parse's layout accepts it, RFC 3339 does not", false,
		func(s string) bool { return len(s) > 12 && s[12] == ':' }},
	{"an offset hour above 23 or minute above 59: time.Parse accepts it, RFC 3339 does not", false,
		func(s string) bool {
			if len(s) < 6 {
				return false
			}
			o := s[len(s)-6:]
			return (o[0] == '+' || o[0] == '-') && o[3] == ':' && (o[1:3] > "23" || o[4:6] > "59")
		}},
}

// rfc3339Forms is every timestamp text the decoder's tests use, decoded from JSON, plus extra edge forms.
func rfc3339Forms(t *testing.T) []string {
	t.Helper()
	var forms []string
	add := func(lit string) {
		var s string
		if json.Unmarshal([]byte(lit), &s) == nil {
			forms = append(forms, s)
		}
	}
	for _, lit := range tsForms {
		add(lit)
	}
	for _, lit := range validFor[event.TTimestamp] {
		add(lit)
	}
	for _, c := range handCases {
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(c.in), &m) == nil {
			for _, k := range []string{"ts", "at", "t"} {
				if raw, ok := m[k]; ok {
					add(string(raw))
				}
			}
		}
	}
	s := mustSchema(t, "ts", event.ArrivalTime, req("t", event.TTimestamp))
	g := &gen{r: rand.New(rand.NewPCG(3339, 5)), s: s}
	for range 3000 {
		add(g.value(event.TTimestamp))
	}
	return append(forms, "2026-11-04T10:00:00.5z", "2016-12-31T23:59:60.5+01:00", "2026-11-04T10:00:00+25:00",
		"2026-11-04T10:00:00+00:99", "2026-11-04T10:00:00-24:00", "2026-11-04T1:00:00z", "0000-02-29T00:00:00Z",
		"0000-02-29T00:00:00z", "2026-11-04T10:00:00.Z", "2026-11-04T10:00:00Z ", " 2026-11-04T10:00:00Z")
}

// TestRFC3339ReferenceAgainstTimeParse checks the oracle's strict reader against time.Parse(time.RFC3339Nano):
// equal milliseconds on every form both accept, and every disagreement in a documented class of
// rfc3339Disagreements, each class seen at least once (so the documentation stays live).
func TestRFC3339ReferenceAgainstTimeParse(t *testing.T) {
	t.Parallel()
	forms := rfc3339Forms(t)
	seen := make([]int, len(rfc3339Disagreements))
	both := 0
	for _, s := range forms {
		ref, refOK := rfc3339Ms(s)
		tm, err := time.Parse(time.RFC3339Nano, s)
		goOK := err == nil
		switch {
		case refOK && goOK:
			both++
			if ref != tm.UnixMilli() {
				t.Errorf("%q: the reference reads %d ms, time.Parse %d ms", s, ref, tm.UnixMilli())
			}
		case refOK != goOK:
			found := false
			for i, d := range rfc3339Disagreements {
				if d.refOnly == refOK && d.match(s) {
					seen[i]++
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%q: an undocumented disagreement: the reference accepts=%v, time.Parse accepts=%v (%v)", s, refOK, goOK, err)
			}
		}
	}
	for i, d := range rfc3339Disagreements {
		if seen[i] == 0 {
			t.Errorf("the documented disagreement %q never occurred over %d forms", d.name, len(forms))
		}
		t.Logf("%d forms: %s", seen[i], d.name)
	}
	if both < 1000 {
		t.Errorf("only %d of %d forms were accepted by both; want at least 1000", both, len(forms))
	}
}
