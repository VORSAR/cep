package playback_test

// S3.2c's exit 3, live and replayed (the sprint card in docs/sprints/sprint-plan.md), written by QA before the build,
// red against today's wrapping arithmetic. The owner's decision (docs/sprints/decisions.md, 2026-11-06):
//
//	"Integer overflow is an expression error for that rule (known issue S3.2b-1-1): as division by zero is, the
//	event is dropped for that rule only and counted; other rules see it. No rule acts on a wrong number."
//
// SEMANTICS §10: an expression error makes the event erroneous for that rule's evaluation only — dropped from that
// rule, counted (RuleStats.Errored); every other rule sees the event normally. Prefix: ov.

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/playback"
)

// ovSrc: one source stream S read by three rules. Scale projects n * 10^12 (its select overflows for |n| ≥ 9223373);
// Wrapped keeps the events whose n * 10^12 is negative (its filter overflows on the same events — today the wrapped
// product of a large positive n is negative, so the rule acts on a wrong number); Plain projects n and is the
// witness that an event the other two drop still reaches it.
const ovSrc = `language cep/0.1;

stream S (k string, n int, ts timestamp)
  time ts watermark bounded 1 min;

stream OS (k string, q int, ts timestamp);
stream OW (k string, ts timestamp);
stream OP (k string, n int, ts timestamp);

rule Scale
  intent 'n in trillionths.'
  from S
  select k, n * 1000000000000 as q
  insert into OS;

rule Wrapped
  intent 'The events whose scaled n is negative.'
  from S[n * 1000000000000 < 0]
  select k
  insert into OW;

rule Plain
  intent 'Every accepted event.'
  from S
  select k, n
  insert into OP;
`

// ovEvents: keys a and c fit; b overflows upward (9300000 * 10^12 > MaxInt64), d downward (−9300000 * 10^12 <
// MinInt64). One second apart, in order.
var ovEvents = []struct {
	k string
	n int64
}{{"a", 6}, {"b", 9300000}, {"c", 7}, {"d", -9300000}}

// TestOverflowDropsTheEventForThatRuleOnly: the four events live on one shard, recorded, then replayed through
// NewPlanDriver. Scale and Wrapped each drop b and d and count them (Errored 2), and no output of theirs carries b or
// d; Plain emits all four; live and replay give the same outputs byte for byte and the same counters.
func TestOverflowDropsTheEventForThatRuleOnly(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-overflow.cep", ovSrc)
	s := q2Stream(t, p, "S")
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	for i, ev := range ovEvents {
		live.payload(s, fmt.Sprintf(`{"k":%q,"n":%d,"ts":%q}`, ev.k, ev.n, ldRFC(ldT0+int64(i)*ldSec)))
	}
	res := live.finish()
	log := w.text()

	check := func(run string, r *q2Run) {
		t.Helper()
		want := map[string]struct {
			errored int64
			keys    []string // the keys of the rule's outputs, in order
			out     string
		}{
			"Scale":   {2, []string{"a", "c"}, "OS"},
			"Wrapped": {2, nil, "OW"},
			"Plain":   {0, []string{"a", "b", "c", "d"}, "OP"},
		}
		for _, name := range []string{"Scale", "Wrapped", "Plain"} {
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
				t.Errorf("%s: %s emitted for keys %v %v; want %v — \"No rule acts on a wrong number\" (owner, 2026-11-06), and the other rules still see the event (SEMANTICS §10)", run, name, keys, lines, wr.keys)
			}
		}
		// The two results that fit are exact.
		os := r.outs[q2Stream(t, p, "OS")]
		for i, want := range []string{`"q":6000000000000`, `"q":7000000000000`} {
			if i >= len(os) || !strings.Contains(os[i], want) {
				t.Errorf("%s: Scale's outputs %v; want output %d to carry %s", run, os, i, want)
			}
		}
	}
	check("live", res)

	replay := q2Replay(t, p, log)
	check("replay", replay)
	for _, name := range []string{"OS", "OW", "OP"} {
		o := q2Stream(t, p, name)
		if !slices.Equal(res.outs[o], replay.outs[o]) {
			t.Errorf("%s: live %v vs replay %v (MUST 10)\nrecording:\n%s", name, res.outs[o], replay.outs[o], log)
		}
	}
	rcCounters(t, res, replay, len(p.Streams), len(p.Rules))
}

// ovKey is the value of a rendered output's "k" attribute ("<absent>" if it has none).
func ovKey(line string) string {
	const name = `"k":"`
	i := strings.Index(line, name)
	if i < 0 {
		return "<absent>"
	}
	rest := line[i+len(name):]
	return rest[:strings.IndexByte(rest, '"')]
}
