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
	h, ok := rfc3339Head(t)
	if !ok || !dateDigits(h) || !clockDigits(h) {
		return 0, false
	}
	year, month, day := rfc3339Date(h)
	hour, minute, sec := rfc3339Clock(h)
	if !dateExists(year, month, day) || !clockValid(hour, minute, sec) {
		return 0, false
	}
	ms, i, ok := rfc3339Fraction(t)
	if !ok {
		return 0, false
	}
	ms *= fractionScale(i - len("2006-01-02T15:04:05.")) // no fraction: i = 19, scale 1, ms 0
	offset, ok := rfc3339Offset(t[i:])
	if !ok {
		return 0, false
	}
	if sec == 60 { // a leap second: the last millisecond of its minute
		sec, ms = 59, 999
	}
	secs := ((daysFromCivil(year, month, day)*24+hour)*60+minute-offset)*60 + sec
	v := secs*1000 + ms
	if !TimestampInRange(v) { // an offset carried the instant out of RFC 3339's years (break-it K2, owner)
		return 0, false
	}
	return v, true
}

// rfc3339Head checks the fixed punctuation of a date-time's first 19 bytes, "2006-01-02T15:04:05", in a t at least
// 20 bytes long (the shortest offset is one byte), and returns those bytes as an array view: constant indices into
// it need no bounds check.
func rfc3339Head(t []byte) (*[19]byte, bool) {
	if len(t) < len("2006-01-02T15:04:05Z") {
		return nil, false
	}
	h := (*[19]byte)(t)
	return h, h[4] == '-' && h[7] == '-' && (h[10] == 'T' || h[10] == 't') && h[13] == ':' && h[16] == ':'
}

// dateDigits says the head's 8 date positions are digits: a byte below '0' wraps above 9.
func dateDigits(h *[19]byte) bool {
	return max(h[0]-'0', h[1]-'0', h[2]-'0', h[3]-'0', h[5]-'0', h[6]-'0', h[8]-'0', h[9]-'0') <= 9
}

// clockDigits says the head's 6 clock positions are digits.
func clockDigits(h *[19]byte) bool {
	return max(h[11]-'0', h[12]-'0', h[14]-'0', h[15]-'0', h[17]-'0', h[18]-'0') <= 9
}

// num2 is the value of the two digits hi and lo.
func num2(hi, lo byte) int64 { return int64(hi)*10 + int64(lo) - '0'*11 }

// rfc3339Date reads the head's year, month and day; its digits are checked by dateDigits.
func rfc3339Date(h *[19]byte) (year, month, day int64) {
	return int64(h[0])*1000 + int64(h[1])*100 + int64(h[2])*10 + int64(h[3]) - '0'*1111, num2(h[5], h[6]), num2(h[8], h[9])
}

// rfc3339Clock reads the head's hour, minute and second; its digits are checked by clockDigits.
func rfc3339Clock(h *[19]byte) (hour, minute, sec int64) {
	return num2(h[11], h[12]), num2(h[14], h[15]), num2(h[17], h[18])
}

// dateExists says the month is 1 to 12 and the day is in that month of that year.
func dateExists(year, month, day int64) bool {
	return month >= 1 && month <= 12 && day >= 1 && day <= daysIn(year, month)
}

// clockValid says the hour is at most 23, the minute at most 59 and the second at most 60 (a leap second).
func clockValid(hour, minute, sec int64) bool { return hour <= 23 && minute <= 59 && sec <= 60 }

// rfc3339Fraction reads the optional "." 1*DIGIT at t[19:], keeping the first three digits, and returns them, the
// index just after the fraction (19 when there is none) and false for a "." with no digit.
func rfc3339Fraction(t []byte) (ms int64, next int, ok bool) {
	if len(t) <= 19 || t[19] != '.' {
		return 0, 19, true
	}
	i := 20
	for i < len(t) && isDigit(t[i]) {
		if i < 23 {
			ms = ms*10 + int64(t[i]-'0')
		}
		i++
	}
	return ms, i, i > 20
}

// fractionScale is what n kept fraction digits are multiplied by to make milliseconds.
func fractionScale(n int) int64 {
	switch n {
	case 1:
		return 100
	case 2:
		return 10
	}
	return 1
}

// rfc3339Offset reads the time offset z, the rest of the timestamp, in minutes east of UTC: "Z" or "z", or a
// numeric offset.
func rfc3339Offset(z []byte) (int64, bool) {
	if len(z) == 1 {
		return 0, z[0] == 'Z' || z[0] == 'z'
	}
	return rfc3339NumOffset(z)
}

// rfc3339NumOffset reads z as exactly ("+"/"-") 2DIGIT ":" 2DIGIT, the hour at most 23 and the minute at most 59.
func rfc3339NumOffset(z []byte) (int64, bool) {
	if len(z) != 6 || z[3] != ':' || max(z[1]-'0', z[2]-'0', z[4]-'0', z[5]-'0') > 9 {
		return 0, false
	}
	oh, om := num2(z[1], z[2]), num2(z[4], z[5])
	return signedOffset(z[0], oh*60+om), (z[0] == '+' || z[0] == '-') && oh <= 23 && om <= 59
}

// signedOffset is minutes east of UTC for the sign byte: '-' is west.
func signedOffset(sign byte, minutes int64) int64 {
	if sign == '-' {
		return -minutes
	}
	return minutes
}

// The UTC instants RFC 3339 can write, in Unix milliseconds: 0000-01-01T00:00:00.000Z to 9999-12-31T23:59:59.999Z.
// A timestamp is read by its UTC instant (owner, 2026-11-05, S3.2a-1 break-it K2), so every accepted one can be
// written back.
const (
	minRFC3339 int64 = -62167219200000
	maxRFC3339 int64 = 253402300799999
)

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
