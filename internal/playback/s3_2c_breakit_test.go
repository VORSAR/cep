package playback_test

// S3.2c break-it (QA, short, as the owner asked), live and replayed: the duration and timestamp operations of the
// owner's decision (docs/sprints/decisions.md, 2026-11-06) — "every 64-bit operation: ints, durations and timestamps
// alike" — reached from a stream's duration attribute, in a filter and in a select, with four rules on one stream.
// QA's s3_2c_overflow_test.go covers int × int; this covers timestamp + duration, unary - on a duration and duration
// × int, each overflowing on a different event, so each rule drops a different one. Prefix: ox.

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/playback"
)

const oxSrc = `language cep/0.1;

stream S (k string, n int, d duration, ts timestamp)
  time ts watermark bounded 1 min;

stream OL (k string, ts timestamp);
stream OM (k string, nd duration, ts timestamp);
stream OD (k string, dd duration, ts timestamp);
stream OP (k string, ts timestamp);

rule Later
  intent 'The events whose d moves ts forward.'
  from S[ts + d > ts]
  select k
  insert into OL;

rule Flip
  intent 'd negated.'
  from S
  select k, -d as nd
  insert into OM;

rule Twice
  intent 'd times n.'
  from S
  select k, d * n as dd
  insert into OD;

rule Plain
  intent 'Every accepted event.'
  from S
  select k
  insert into OP;
`

// oxEvents: a fits everywhere; b's ts + d and d * 2 overflow (d is the largest duration), and its -d fits; c's -d
// overflows (d is the smallest duration), its d * 1 fits, and its ts + d fits and is not later than ts.
var oxEvents = []struct {
	k    string
	n, d int64
}{{"a", 2, 1000}, {"b", 2, 9223372036854775807}, {"c", 1, -9223372036854775808}}

// TestBreakitDurationAndTimestampOverflowDropsTheEventForThatRuleOnly: Later drops b (errored 1), Flip drops c
// (errored 1), Twice drops b (errored 1), Plain emits all three; every result that fits is exact; live and replay
// agree byte for byte with the same counters.
func TestBreakitDurationAndTimestampOverflowDropsTheEventForThatRuleOnly(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-breakit-overflow.cep", oxSrc)
	s := q2Stream(t, p, "S")
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	for i, ev := range oxEvents {
		live.payload(s, fmt.Sprintf(`{"k":%q,"n":%d,"d":%d,"ts":%q}`, ev.k, ev.n, ev.d, ldRFC(ldT0+int64(i)*ldSec)))
	}
	res := live.finish()
	log := w.text()

	check := func(run string, r *q2Run) {
		t.Helper()
		want := map[string]struct {
			errored int64
			keys    []string
			out     string
			has     []string // a substring each output carries, in order
		}{
			"Later": {1, []string{"a"}, "OL", nil},
			"Flip":  {1, []string{"a", "b"}, "OM", []string{`"nd":-1000`, `"nd":-9223372036854775807`}},
			"Twice": {1, []string{"a", "c"}, "OD", []string{`"dd":2000`, `"dd":-9223372036854775808`}},
			"Plain": {0, []string{"a", "b", "c"}, "OP", nil},
		}
		for _, name := range []string{"Later", "Flip", "Twice", "Plain"} {
			wr := want[name]
			rs := q2RuleStats(t, r.stats, q2Rule(t, p, name))
			if rs.Errored != wr.errored {
				t.Errorf("%s: %s's Errored %d (RuleStats %+v); want %d — an overflow is an expression error for that rule, counted (owner, 2026-11-06)", run, name, rs.Errored, rs, wr.errored)
			}
			lines := r.outs[q2Stream(t, p, wr.out)]
			var keys []string
			for _, l := range lines {
				keys = append(keys, ovKey(l))
			}
			if !slices.Equal(keys, wr.keys) {
				t.Errorf("%s: %s emitted for keys %v %v; want %v — \"No rule acts on a wrong number\" (owner, 2026-11-06)", run, name, keys, lines, wr.keys)
			}
			for i, sub := range wr.has {
				if i >= len(lines) || !strings.Contains(lines[i], sub) {
					t.Errorf("%s: %s's outputs %v; want output %d to carry %s exactly (no result that fits changes)", run, name, lines, i, sub)
				}
			}
		}
	}
	check("live", res)
	replay := q2Replay(t, p, log)
	check("replay", replay)
	for _, name := range []string{"OL", "OM", "OD", "OP"} {
		o := q2Stream(t, p, name)
		if !slices.Equal(res.outs[o], replay.outs[o]) {
			t.Errorf("%s: live %v vs replay %v (MUST 10)\nrecording:\n%s", name, res.outs[o], replay.outs[o], log)
		}
	}
	rcCounters(t, res, replay, len(p.Streams), len(p.Rules))
}
