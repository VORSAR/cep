package parser

// bareDuration's net (S1.1c-6 design Q.3): the first number standing alone in a duration slot — after bounded,
// after idle after, inside delay( — from the given index to the statement's end, or -1.

import "testing"

func TestBareDurationFindsTheFirstUnitlessNumber(t *testing.T) {
	t.Parallel()
	for _, r := range []struct {
		name, src string
		at, want  int
	}{
		{"a number last, after bounded", "watermark bounded 5", 0, 2},
		{"a number at index 0", "5", 0, -1},
		{"a number at index 1", "bounded 5 ;", 0, 1},
		{"bounded 1", "time ts watermark bounded 1;", 0, 4},
		{"idle after 5", "idle after 5;", 0, 2},
		{"delay(5)", "window delay(5) emit", 0, 3},
		{"after 5 without idle", "match after 5;", 0, -1},
		{"idle, then another word before 5", "idle before 5;", 0, -1},
		{"( 5 without delay", "cron ( 5 )", 0, -1},
		{"delay, then another word before 5", "delay x 5;", 0, -1},
		{"5 min", "bounded 5 min;", 0, -1},
		{"a name after bounded", "bounded x;", 0, -1},
		{"a ; ends the scan", "within 5; bounded 1", 0, -1},
		{"the first of two", "bounded 1 bounded 2", 0, 1},
		{"the scan starts at at", "bounded 1 bounded 2", 2, 3},
		{"none", "select a from S", 0, -1},
	} {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			toks, _, err := lexAll("t.cep", r.src)
			if err != nil {
				t.Fatalf("lexAll(%q): %v", r.src, err)
			}
			if got := bareDuration(toks, r.at); got != r.want {
				t.Errorf("bareDuration(%q, %d) = %d; want %d", r.src, r.at, got, r.want)
			}
		})
	}
}
