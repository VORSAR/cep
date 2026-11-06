package playback_test

// S3.2b-2's exit 9 (design note docs/sprints/s3-2b-2-design.md v1, §1.1, §2.5, §2.6, §10 MUST 3 and 4, §11): multi-part
// partition keys through program text — live (runtime.Start, several shard counts) and in replay (NewPlanDriver,
// whose Stats are runtime.Sync's). Owner, S3.2b-1's review and break-it: "Each combination of key parts is its own
// key: its own rate limit, state and expiry; a null in any part drops the event for that rule, counted; parts may
// differ in type, matched position by position across streams." On the stub the parser accepts only `k of S`, so
// every test here is red at its program's parse. Only plain (one-stream) bodies build until S4.1, so worked
// examples 3 and 4 (two streams) are proven statically, in internal/validate's tests (report G1).

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
)

// mkSrc: Grants keyed by (tenantId, userId) — worked examples 1 and 2 —; Attempts keyed by three parts of three
// types (PerTriple) and by a string and an optional float (PerScore: -0.0 and +0.0 are one value, MUST 4). Each
// rule projects its key parts as p1, p2, … (a key part is never null where its rule runs: E-NULL-003's exemption
// covers every part, design §1.3).
const mkSrc = `language cep/0.1;

stream Grants (tenantId string, userId string optional, ts timestamp)
  time ts watermark bounded 1 min;

stream Attempts (agentId string, attempt int, score float optional, passed bool, ts timestamp)
  time ts watermark bounded 1 min;

stream OGrant (p1 string, p2 string, ts timestamp);
stream OAttempt (p1 string, p2 int, p3 bool, ts timestamp);
stream OScore (p1 string, p2 float, ts timestamp);

rule PerPair
  intent 'At most one alert per tenant-and-user pair per 5 minutes.'
  partition by (tenantId, userId) of Grants ttl 10 min
  from Grants
  select tenantId as p1, userId as p2
  output first every 5 min
  insert into OGrant;

rule PerTriple
  intent 'At most one alert per agent, attempt and outcome per 5 minutes.'
  partition by (agentId, attempt, passed) of Attempts ttl 10 min
  from Attempts
  select agentId as p1, attempt as p2, passed as p3
  output first every 5 min
  insert into OAttempt;

rule PerScore
  intent 'At most one alert per agent and score per 5 minutes.'
  partition by (agentId, score) of Attempts ttl 10 min
  from Attempts
  select agentId as p1, score as p2
  output first every 5 min
  insert into OScore;
`

// mkG is a Grants line at T0 + at; user "" → userId absent (null). arrival > 0 adds a recording's arrival.
func mkG(at int64, tenant, user string, arrival int64) string {
	attrs := fmt.Sprintf(`"tenantId":%q,"ts":%q`, tenant, ldRFC(ldT0+at))
	if user != "" {
		attrs += fmt.Sprintf(`,"userId":%q`, user)
	}
	return mkLine("Grants", attrs, arrival)
}

// mkA is an Attempts line at T0 + at; score is written verbatim ("" → absent: null).
func mkA(at int64, agent string, attempt int, score string, passed bool, arrival int64) string {
	attrs := fmt.Sprintf(`"agentId":%q,"attempt":%d,"passed":%v,"ts":%q`, agent, attempt, passed, ldRFC(ldT0+at))
	if score != "" {
		attrs += `,"score":` + score
	}
	return mkLine("Attempts", attrs, arrival)
}

func mkLine(stream, attrs string, arrival int64) string {
	if arrival > 0 {
		return fmt.Sprintf(`{"stream":%q,"arrival":%q,"attrs":{%s}}`, stream, ldRFC(arrival), attrs)
	}
	return fmt.Sprintf(`{"stream":%q,"attrs":{%s}}`, stream, attrs)
}

// mkLiveRecords runs a log's records live on shards shards through Send: Config.Now hands out T0, then 1 s per read
// (every event is inside its stream's 1 min bound, so none is stream-late live).
func mkLiveRecords(t *testing.T, p *plan.App, log string, shards int) *q2Run {
	t.Helper()
	live := q2Start(t, p, shards, q2NewClock(ldT0, ldSec).now, nil)
	for _, rec := range ldRecordsOK(t, pbApp(p), log) {
		switch rec.Kind {
		case playback.RecEvent:
			if err := live.r.Send(live.ctx, 0, rec.Event); err != nil {
				t.Fatalf("line %d: Send: %v", rec.Line, err)
			}
		case playback.RecTick:
			if err := live.r.Tick(live.ctx, rec.Time); err != nil {
				t.Fatalf("line %d: Tick: %v", rec.Line, err)
			}
		}
	}
	return live.finish()
}

// mkWant is one expected output: its key projections and its time.
func mkWant(at int64, parts ...string) string {
	return fmt.Sprintf("%v@%s", parts, ldRFC(ldT0+at))
}

// mkGot renders a run's outputs on stream name as mkWant does, per key part names p1..pn, in emission order of
// each key; keys are sorted so several shards compare.
func mkGot(t *testing.T, p *plan.App, res *q2Run, name string, n int) []string {
	t.Helper()
	var got []string
	for _, line := range res.outs[q2Stream(t, p, name)] {
		var parts []string
		for i := 1; i <= n; i++ {
			parts = append(parts, qeAttr(line, fmt.Sprintf("p%d", i)))
		}
		got = append(got, fmt.Sprintf("%v@%s", parts, qeAttr(line, "ts")))
	}
	slices.Sort(got)
	return got
}

// mkCase is one log with its hand-derived outputs per output stream and counters per rule.
type mkCase struct {
	name     string
	log      string
	want     map[string][]string // per output stream, sorted
	parts    map[string]int      // per output stream, its key's arity
	keyErrs  map[string]int64    // per rule
	suppress map[string]int64    // per rule
}

// mkCases are worked examples 1 and 2 (design §1.1) and the per-part identity rules, each derived by hand from
// SEMANTICS §8 (`first every 5 min`: an output at t ≤ anchor + 5 min is suppressed, per key) and §9 as rev 5 states
// it (a key is the tuple of its parts, equal part by part).
func mkCases() []mkCase {
	return []mkCase{
		{
			// Worked example 1: 10:00 (t1, alice) and 10:01 (t1, bob) are two keys → two outputs; 10:02 (t1, alice) is
			// inside alice's 5 min → suppressed. 10:03 (t2, alice) is a third key → output (a last-part-only key would
			// merge it with (t1, alice) and suppress it). 10:06 (t1, alice): past 10:00 + 5 min → output.
			// Worked example 2: 10:04 (t1, null) → PerPair's KeyErrors +1, no output, no instance.
			name: "worked examples 1 and 2",
			log: ldLog(mkG(0, "t1", "alice", 0), mkG(ldMin, "t1", "bob", 0), mkG(2*ldMin, "t1", "alice", 0),
				mkG(3*ldMin, "t2", "alice", 0), mkG(4*ldMin, "t1", "", 0), mkG(6*ldMin, "t1", "alice", 0)),
			want: map[string][]string{"OGrant": sorted(mkWant(0, "t1", "alice"), mkWant(ldMin, "t1", "bob"),
				mkWant(3*ldMin, "t2", "alice"), mkWant(6*ldMin, "t1", "alice"))},
			parts:    map[string]int{"OGrant": 2},
			keyErrs:  map[string]int64{"PerPair": 1},
			suppress: map[string]int64{"PerPair": 1},
		},
		{
			// No cross-part interaction: ("a", "bc") and ("ab", "c") are different keys → two outputs.
			name:     "parts do not run together",
			log:      ldLog(mkG(0, "a", "bc", 0), mkG(ldSec, "ab", "c", 0)),
			want:     map[string][]string{"OGrant": sorted(mkWant(0, "a", "bc"), mkWant(ldSec, "ab", "c"))},
			parts:    map[string]int{"OGrant": 2},
			keyErrs:  map[string]int64{},
			suppress: map[string]int64{},
		},
		{
			// Three parts of three types: (ag, 1, true) and (ag, 1, false) differ in part 3 only → two outputs;
			// (ag, 1, true) at 10:02 → suppressed; (ag, 2, true) at 10:03 differs in part 2 → output.
			// PerScore on the same events: score absent on all four → a null part each → KeyErrors 4, no output.
			name: "three parts, each its own type",
			log: ldLog(mkA(0, "ag", 1, "", true, 0), mkA(ldMin, "ag", 1, "", false, 0),
				mkA(2*ldMin, "ag", 1, "", true, 0), mkA(3*ldMin, "ag", 2, "", true, 0)),
			want: map[string][]string{"OAttempt": sorted(mkWant(0, "ag", "1", "true"), mkWant(ldMin, "ag", "1", "false"),
				mkWant(3*ldMin, "ag", "2", "true")), "OScore": nil},
			parts:    map[string]int{"OAttempt": 3, "OScore": 2},
			keyErrs:  map[string]int64{"PerScore": 4, "PerTriple": 0},
			suppress: map[string]int64{"PerTriple": 1},
		},
		{
			// A float part: -0.0 and 0.0 are one value (MUST 4) → (ag, -0.0) at 10:00 emits, (ag, 0.0) at 10:01 is the
			// same key → suppressed; (ag, 1.5) at 10:02 → output. PerTriple: (ag, 1, true) ×3 → one output, 2
			// suppressed. The stored zero part is +0.0, so the first output's p2 renders as 0 (AppendValue's form).
			name: "a float part, -0.0 equal to +0.0",
			log: ldLog(mkA(0, "ag", 1, "-0.0", true, 0), mkA(ldMin, "ag", 1, "0.0", true, 0),
				mkA(2*ldMin, "ag", 1, "1.5", true, 0)),
			want: map[string][]string{"OScore": sorted(mkWant(0, "ag", "-0"), mkWant(2*ldMin, "ag", "1.5")),
				"OAttempt": sorted(mkWant(0, "ag", "1", "true"))},
			parts:    map[string]int{"OScore": 2, "OAttempt": 3},
			keyErrs:  map[string]int64{},
			suppress: map[string]int64{"PerScore": 1, "PerTriple": 2},
		},
	}
}

func sorted(s ...string) []string { slices.Sort(s); return s }

// mkCheck compares one run with a case's hand-derived outputs and counters.
func mkCheck(t *testing.T, p *plan.App, c mkCase, res *q2Run, how string) {
	t.Helper()
	for stream, want := range c.want {
		if got := mkGot(t, p, res, stream, c.parts[stream]); !slices.Equal(got, want) {
			t.Errorf("%s, %s: %s outputs %v; want %v", c.name, how, stream, got, want)
		}
	}
	for _, rule := range []string{"PerPair", "PerTriple", "PerScore"} {
		rs := q2RuleStats(t, res.stats, q2Rule(t, p, rule))
		if rs.KeyErrors != c.keyErrs[rule] {
			t.Errorf("%s, %s: %s's KeyErrors %d; want %d", c.name, how, rule, rs.KeyErrors, c.keyErrs[rule])
		}
		if rs.Suppressed != c.suppress[rule] {
			t.Errorf("%s, %s: %s's Suppressed %d; want %d", c.name, how, rule, rs.Suppressed, c.suppress[rule])
		}
	}
}

// TestMultiPartKeysWorkedExamples is exit 9's first three clauses: design §1.1's worked examples 1 and 2 and the
// identity rules of §1.1 and MUST 3, 4 — in replay (a fixture through NewPlanDriver) and live on 1, 2, 4 and 7
// shards — each against outputs derived by hand.
func TestMultiPartKeysWorkedExamples(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-multipart.cep", mkSrc)
	for _, c := range mkCases() {
		mkCheck(t, p, c, q2Replay(t, p, c.log), "replay")
		for _, n := range []int{1, 2, 4, 7} {
			mkCheck(t, p, c, mkLiveRecords(t, p, c.log, n), fmt.Sprintf("live, %d shards", n))
		}
	}
}

// mkGenerate writes a recording over mkSrc: a header at 10:00, 60 events on Grants and Attempts with arrival stamps
// that never go back and event times up to 90 s behind them (some stream-late), few distinct values per part so keys
// collide in one part and differ in another, null userId and score parts, a -0.0 score, ticks, then a stop.
func mkGenerate(seed uint64) string {
	rng := rand.New(rand.NewPCG(seed, 0x6b657973))
	tenants, users, agents := []string{"t1", "t2"}, []string{"alice", "bob", ""}, []string{"a", "ab", "b"}
	scores := []string{"", "-0.0", "0.0", "1.5"}
	lines := []string{q2Header(ldT0)}
	wall := ldT0
	for range 60 {
		wall += rng.Int64N(40 * ldSec)
		at := wall - ldT0 - rng.Int64N(90*ldSec)
		switch r := rng.IntN(100); {
		case r < 50:
			lines = append(lines, mkG(at, tenants[rng.IntN(2)], users[rng.IntN(3)], wall))
		case r < 94:
			lines = append(lines, mkA(at, agents[rng.IntN(3)], 1+rng.IntN(2), scores[rng.IntN(4)], rng.IntN(2) == 0, wall))
		default:
			lines = append(lines, fmt.Sprintf(`{"tick":%q}`, ldRFC(wall)))
		}
	}
	return ldLog(append(lines, `{"stop":true}`)...)
}

// mkLiveRecording runs a recording live as eqLive does: Config.Now pops the header's start, then each event's
// arrival; ticks and the stop go through Tick and Drain.
func mkLiveRecording(t *testing.T, p *plan.App, log string, shards int) *q2Run {
	t.Helper()
	recs := ldRecordsOK(t, pbApp(p), log)
	var stamps []int64
	for _, rec := range recs {
		switch rec.Kind {
		case playback.RecHeader:
			stamps = append(stamps, rec.Time)
		case playback.RecEvent:
			stamps = append(stamps, rec.Arrival)
		}
	}
	clk := &mkPop{stamps: stamps}
	live := q2Start(t, p, shards, clk.now, nil)
	for _, rec := range recs {
		var err error
		switch rec.Kind {
		case playback.RecEvent:
			err = live.r.Send(live.ctx, 0, rec.Event)
		case playback.RecTick:
			err = live.r.Tick(live.ctx, rec.Time)
		}
		if err != nil {
			t.Fatalf("line %d: %v", rec.Line, err)
		}
	}
	return live.finish()
}

// mkPop is a Config.Now handing out a recording's stamps in order (then the last one again).
type mkPop struct {
	q2Clock
	stamps []int64
}

func (c *mkPop) now() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.stamps) == 0 {
		return c.next
	}
	c.next, c.stamps = c.stamps[0], c.stamps[1:]
	return c.next
}

// TestMultiPartKeysLiveEqualsReplay extends S3.2b's exit 1 to two- and three-part keys (exit 9's fourth clause;
// design §11): 50 seeds × 1, 2, 4, 7 shards, the same recording live and replayed gives identical per-key output on
// every rule and equal KeyErrors, Suppressed, ConstructLate, Purged and Errored, and equal stream-late counts. Over
// all seeds the recordings must exercise a null part, a suppression and a stream-late event.
func TestMultiPartKeysLiveEqualsReplay(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-multipart.cep", mkSrc)
	var keyErrs, suppressed, late int64
	for seed := range uint64(50) {
		log := mkGenerate(seed)
		replay := q2Replay(t, p, log)
		for _, n := range []int{1, 2, 4, 7} {
			live := mkLiveRecording(t, p, log, n)
			for stream, arity := range map[string]int{"OGrant": 2, "OAttempt": 3, "OScore": 2} {
				s := q2Stream(t, p, stream)
				a, b := &q2Run{outs: map[int][]string{s: live.outs[s]}}, &q2Run{outs: map[int][]string{s: replay.outs[s]}}
				if d := q2Diff(t, p, a, b, arity); d != "" {
					t.Fatalf("seed %d, %d shards, live vs replay: %s\nrecording:\n%s", seed, n, d, log)
				}
			}
			for r := range p.Rules {
				x, y := q2RuleStats(t, live.stats, r), q2RuleStats(t, replay.stats, r)
				if x.KeyErrors != y.KeyErrors || x.Suppressed != y.Suppressed || x.ConstructLate != y.ConstructLate || x.Purged != y.Purged || x.Errored != y.Errored {
					t.Fatalf("seed %d, %d shards: rule %s: live %+v vs replay %+v\nrecording:\n%s", seed, n, p.Rules[r].Name, x, y, log)
				}
			}
			for s := range p.Streams {
				if x, y := q2StreamStats(t, live.stats, s).Late, q2StreamStats(t, replay.stats, s).Late; x != y {
					t.Fatalf("seed %d, %d shards: stream %d late: live %d vs replay %d", seed, n, s, x, y)
				}
			}
		}
		for r := range p.Rules {
			rs := q2RuleStats(t, replay.stats, r)
			keyErrs += rs.KeyErrors
			suppressed += rs.Suppressed
		}
		for s := range p.Streams {
			late += q2StreamStats(t, replay.stats, s).Late
		}
	}
	if keyErrs == 0 || suppressed == 0 || late == 0 {
		t.Errorf("the generator does not exercise the comparison: key errors %d, suppressed %d, stream-late %d", keyErrs, suppressed, late)
	}
}
