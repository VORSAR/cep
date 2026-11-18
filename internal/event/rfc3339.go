package event

// TimestampInRange says ms is an instant RFC 3339 can write: the years 0000 to 9999, in UTC (S3.2a-1's K2).
func TimestampInRange(ms int64) bool { return ms >= minRFC3339 && ms <= maxRFC3339 }

// parseRFC3339 reads a timestamp strictly by RFC 3339 §5.6 (owner D4; design note §4.2) and returns milliseconds
// since 1970-01-01T00:00:00Z. Fraction digits past the third are dropped (rounding down, since the fraction is
// added); a leap second reads as that minute's :59.999.
//
//	date-time = 4DIGIT "-" 2DIGIT "-" 2DIGIT ("T"/"t") 2DIGIT ":" 2DIGIT ":" 2DIGIT ["." 1*DIGIT] ("Z"/"z"/("+"/"-")
//
// 2DIGIT ":" 2DIGIT)
func parseRFC3339(t []byte) (int64, bool) {
	if len(t) < len("2006-01-02T15:04:05Z") || t[4] != '-' || t[7] != '-' || (t[10] != 'T' && t[10] != 't') ||
		t[13] != ':' || t[16] != ':' {
		return 0, false
	}
	year, ok1 := digitsAt(t, 0, 4)
	month, ok2 := digitsAt(t, 5, 2)
	day, ok3 := digitsAt(t, 8, 2)
	hour, ok4 := digitsAt(t, 11, 2)
	minute, ok5 := digitsAt(t, 14, 2)
	sec, ok6 := digitsAt(t, 17, 2)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 ||
		month < 1 || month > 12 || day < 1 || day > daysIn(year, month) || hour > 23 || minute > 59 || sec > 60 {
		return 0, false
	}
	i := 19
	var ms int64
	if i < len(t) && t[i] == '.' {
		i++
		start := i
		for i < len(t) && isDigit(t[i]) {
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
		oh, okh := digitsAt(t, i+1, 2)
		om, okm := digitsAt(t, i+4, 2)
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
	secs := ((daysFromCivil(year, month, day)*24+hour)*60+minute-offset)*60 + sec
	v := secs*1000 + ms
	if v < minRFC3339 || v > maxRFC3339 { // an offset carried the instant out of RFC 3339's years (break-it K2, owner)
		return 0, false
	}
	return v, true
}

// The UTC instants RFC 3339 can write, in Unix milliseconds: 0000-01-01T00:00:00.000Z to 9999-12-31T23:59:59.999Z.
// A timestamp is read by its UTC instant (owner, 2026-11-05, S3.2a-1 break-it K2), so every accepted one can be
// written back.
const (
	minRFC3339 int64 = -62167219200000
	maxRFC3339 int64 = 253402300799999
)

func digitsAt(t []byte, at, n int) (int64, bool) {
	var v int64
	for k := at; k < at+n; k++ {
		if !isDigit(t[k]) {
			return 0, false
		}
		v = v*10 + int64(t[k]-'0')
	}
	return v, true
}

func daysIn(year, month int64) int64 {
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

// daysFromCivil is the number of days from 1970-01-01 to the proleptic Gregorian date (H. Hinnant's algorithm).
func daysFromCivil(y, m, d int64) int64 {
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
