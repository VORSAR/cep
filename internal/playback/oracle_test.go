package playback_test

// The oracle fixtures (review F8; design note v2 §6, "Oracle fixtures"): testdata/log/gen_oracle.py runs
// docs/language/checks/executor.py's run_pattern_multi (playback mode, owner D1's drain) over fixed-seed corpora
// and commits what it saw; these tests replay the same logs through the router double and compare. Everything is
// compared by file line (the oracle's arrival index + 1), never by Seq: the oracle numbers only buffered events
// (review R5), the driver every event (design §5.3 step 1).

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
)

type ldOracleCompletion struct {
	At  int    `json:"at"`
	Key string `json:"key"`
	Due int64  `json:"due"`
}

type ldOracleWant struct {
	Seed    int64 `json:"seed"`
	Streams []struct {
		Name  string `json:"name"`
		Delay int64  `json:"delay"`
	} `json:"streams"`
	Events        int                  `json:"events"`
	D             int64                `json:"d"`
	StreamLate    []int                `json:"stream_late"`
	ConstructLate []int                `json:"construct_late"`
	Completions   []ldOracleCompletion `json:"completions"`
}

// ldOracle loads a fixture: its log, its want, and the App its streams declare (declaration order = stream id,
// SEMANTICS §2.2, as the oracle's delays dict).
func ldOracle(t *testing.T, name string) (string, ldOracleWant, *playback.App) {
	t.Helper()
	log, err := os.ReadFile("testdata/log/" + name + ".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/log/" + name + ".want.json")
	if err != nil {
		t.Fatal(err)
	}
	var want ldOracleWant
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	app := &playback.App{}
	for _, s := range want.Streams {
		app.Streams = append(app.Streams, ldStd(t, s.Name, s.Delay, 0))
	}
	return string(log), want, app
}

// ldAll is every stream id: the oracle's construct clock is the min over every declared stream.
func ldAll(app *playback.App) []int {
	ids := make([]int, len(app.Streams))
	for i := range ids {
		ids[i] = i
	}
	return ids
}

// ldLateLines is the stream-late and construct-late lines the harness saw.
func ldLateLines(w *ldWorld) (sl, cl []int) {
	for _, e := range ldFilter(w.log, "stream_late") {
		sl = append(sl, e.At)
	}
	for _, e := range ldFilter(w.log, "construct_late") {
		cl = append(cl, e.At)
	}
	return sl, cl
}

// ldAbsence is the Go side of `A, not X for D` with X never occurring (SEMANTICS §6.4): every released A opens a
// match whose deadline t_first + D completes it once the clock is past it (owner D1). The body schedules one
// Completion at e.Ts + D per released A; ref = the key's release count, so equal dues on a key fire in §2.2 order
// as the oracle's first_ord tie-break does. TTL 0: the oracle spec has no TTL.
func ldAbsence(d int64) ldRuleSpec {
	return ldRuleSpec{Name: "R", Key: "k", Sched: func(e event.Event, n uint64) []clock.Timer {
		if e.StreamID != 0 {
			return nil
		}
		return []clock.Timer{{Due: e.Ts + d, Kind: clock.Completion, Ref: n}}
	}}
}

func ldCompletions(w *ldWorld) []ldOracleCompletion {
	var out []ldOracleCompletion
	for _, e := range ldFilter(w.log, "timer") {
		out = append(out, ldOracleCompletion{At: e.At, Key: e.Key, Due: e.Due})
	}
	return out
}

// The tiny cases, by hand, before the corpora (the generator asserts the oracle gives the same).
// Streams A and B, d = 0; seconds after 10:00 (design §5.3; SEMANTICS §2.1; owner D1).
//
// Lateness: L1 A@0 → the clock min(0, B's virtual floor 0) = 0, buffered. L2 B@5 → the clock 5. L3 A@-2: A's
// admission 0 > -2 → stream-late. L4 A@2: A's admission 0 ≤ 2 (admitted by A); the clock 5 > 2 → construct-late.
//
// Absence, D = 5 s: L1 A@0 k=a. L2 B@5 k=b → the clock 5: A@0 released, its Completion due 5 not below 5. L3 B@6 →
// the clock 6: B@5 released, then the Completion at 5 fires (events before timers at one instant) → (line 3, a,
// 10:00:05). The end (line 4) releases B@6 and fires nothing more.
func TestOracleTinyCasesByHand(t *testing.T) {
	t.Parallel()
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "A", 0, 0), ldStd(t, "B", 0, 0)}}
	w := ldRun(t, app, ldLog(ldE("A", ldAt(0), "a", "", ""), ldE("B", ldAt(5), "b", "", ""), ldE("A", ldAt(-2), "a", "", ""),
		ldE("A", ldAt(2), "c", "", "")), ldRuleSpec{Name: "R", Inputs: []int{0, 1}, Key: "k"})
	if sl, cl := ldLateLines(w); !slices.Equal(sl, []int{3}) || !slices.Equal(cl, []int{4}) {
		t.Errorf("lateness: stream-late %v construct-late %v, want [3] and [4]%s", sl, cl, ldEntries(w.log))
	}
	spec := ldAbsence(5 * ldSec)
	spec.Inputs = []int{0, 1}
	w = ldRun(t, app, ldLog(ldE("A", ldAt(0), "a", "", ""), ldE("B", ldAt(5), "b", "", ""), ldE("B", ldAt(6), "b", "", "")), spec)
	if got, want := ldCompletions(w), []ldOracleCompletion{{At: 3, Key: "a", Due: ldAt(5)}}; !slices.Equal(got, want) {
		t.Errorf("completions %v, want %v%s", got, want, ldEntries(w.log))
	}
}

// Fixture (1): testdata/log/oracle1 — seed 20261105, 300 events on A (d 2 s), B (d 0), C (d 5 s), keys a–d. The
// oracle's spec opens no partials, so only the clock is observed; the Go body schedules nothing. Stream-late and
// construct-late lines must match the oracle's.
func TestOracleLatenessMatches(t *testing.T) {
	t.Parallel()
	log, want, app := ldOracle(t, "oracle1")
	w := ldRun(t, app, log, ldRuleSpec{Name: "R", Inputs: ldAll(app), Key: "k"})
	sl, cl := ldLateLines(w)
	if !slices.Equal(sl, want.StreamLate) {
		t.Errorf("stream-late lines\n got %v\nwant %v (the oracle, seed %d)", sl, want.StreamLate, want.Seed)
	}
	if !slices.Equal(cl, want.ConstructLate) {
		t.Errorf("construct-late lines\n got %v\nwant %v (the oracle, seed %d)", cl, want.ConstructLate, want.Seed)
	}
	if len(want.StreamLate) == 0 || len(want.ConstructLate) == 0 || want.Events != len(w.steps)-1 {
		t.Errorf("the fixture is degenerate: %d stream-late, %d construct-late, %d events for %d steps",
			len(want.StreamLate), len(want.ConstructLate), want.Events, len(w.steps))
	}
}

// Fixture (2): testdata/log/oracle2 — seed 20261106, 200 events on A (d 1 s) and B (d 3 s, filler), keys a–c, the
// oracle's `A, not X for 5 s`. Every completion must match the oracle's in order, as (the line of the walk that
// emitted it, key, due = match_end = t_first + D), the end being line 201; and the lateness lines too.
func TestOracleAbsenceCompletionsMatch(t *testing.T) {
	t.Parallel()
	log, want, app := ldOracle(t, "oracle2")
	spec := ldAbsence(want.D)
	spec.Inputs = ldAll(app)
	w := ldRun(t, app, log, spec)
	if got := ldCompletions(w); !slices.Equal(got, want.Completions) {
		t.Errorf("completions (%d) differ from the oracle's (%d), seed %d:\n got %v\nwant %v", len(got), len(want.Completions), want.Seed, got, want.Completions)
	}
	sl, cl := ldLateLines(w)
	if !slices.Equal(sl, want.StreamLate) || !slices.Equal(cl, want.ConstructLate) {
		t.Errorf("lateness: stream-late %v construct-late %v, want %v and %v", sl, cl, want.StreamLate, want.ConstructLate)
	}
	if len(want.Completions) == 0 || want.D <= 0 {
		t.Errorf("the fixture is degenerate: %d completions, D %d", len(want.Completions), want.D)
	}
}
