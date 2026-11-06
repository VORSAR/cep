package playback_test

// S3.2b's exit 1 (design note docs/sprints/s3-2b-1-design.md v1, §5, with the architecture review's changes): the
// same recording run live (runtime.Start, N shards, Config.Now popping the recording's arrival stamps, TickEvery 0)
// and in replay (NewPlanDriver; and runtime.Sync directly, for the counters the driver does not expose) gives, per
// rule and key, the identical output sequence, and equal stream-late, construct-late, suppressed, purged, errored
// and key-error counts. Each rule has its own derived stream with the key projected into it, so an output names
// its rule and key. No `time arrival` stream (its stamping is S3.2b-2's). Teeth: a planted body whose output
// depends on the shard that built it turns the comparison red for N > 1.

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// qaEqSrc: A (nullable key k, divisor d), B (idle after 1 min), table T; Keyed (partitioned, rate-limited, can
// divide by zero, reads T), Global (key-less, rate-limited), Double (replaced by the buffered test double).
const qaEqSrc = `language cep/0.1;

stream A (k string optional, v int, d int, ts timestamp)
  time ts watermark bounded 1 min;

stream B (k string, v int, ts timestamp)
  time ts watermark bounded 30 sec idle after 1 min;

table T (k string) primary key (k);

stream OK (k string optional, v int, ts timestamp); -- k can be null on A: optional (E-NULL-003, owner D4)
stream OG (k string, v int, ts timestamp);
stream OD (k string, n int, ts timestamp);

rule Keyed
  intent 'Keyed, rate-limited, can divide by zero, reads a table.'
  partition by k of A ttl 5 min
  from A[v / d > 0 and k not in T]
  select k, v
  output first every 2 min
  insert into OK;

rule Global
  intent 'Key-less and rate-limited.'
  from B[v > 5]
  select k, v
  output first every 1 min
  insert into OG;

rule Double
  intent 'Replaced by the buffered test double.'
  partition by k of B ttl 3 min
  from B
  select k, v as n
  insert into OD;
`

const (
	eqA, eqB, eqOK, eqOG, eqOD = 0, 1, 2, 3, 4
)

// eqState is the double's per-key state: the timers it scheduled and has not seen fire.
type eqState struct{ timers []clock.Timer }

func (s *eqState) Walk(fn func(component, key string, state any)) { fn("op", "", len(s.timers)) }
func (s *eqState) Timers(dst []clock.Timer) []clock.Timer         { return append(dst, s.timers...) }

// eqDouble is S3.2a's buffered test-double body: per released event a Completion at Ts + 20 s (Ref its Seq); on
// fire, one OD output (k, n = Ref + plant·shard·1000, ts = due).
type eqDouble struct {
	emit  func(event.Event)
	shard int64
	plant bool
}

func (b *eqDouble) NewState(*partition.Instance) partition.OpState { return &eqState{} }
func (b *eqDouble) OnEvent(in *partition.Instance, e event.Event) {
	tm := clock.Timer{Due: e.Ts + 20*ldSec, Kind: clock.Completion, Ref: e.Seq*10 + uint64(e.StreamID)}
	in.Clock.Schedule(tm.Due, tm.Kind, tm.Ref)
	st := in.Op.(*eqState)
	st.timers = append(st.timers, tm)
}
func (b *eqDouble) OnTimers(in *partition.Instance, fires []clock.Timer, _ any) {
	st := in.Op.(*eqState)
	for _, f := range fires {
		st.timers = slices.DeleteFunc(st.timers, func(x clock.Timer) bool { return x == f })
		n := int64(f.Ref)
		if b.plant {
			n += b.shard * 1000 // planted: the output depends on the shard that built the body
		}
		b.emit(event.Event{StreamID: eqOD, Ts: f.Due, Values: []event.Value{in.Key, {Kind: event.TInt, I: n}, {Kind: event.TTimestamp, T: f.Due}}})
	}
}
func (b *eqDouble) OnDiscard(*partition.Instance, []uint64) {}
func (b *eqDouble) OnPurge(*partition.Instance)             {}

// eqPlan builds qaEqSrc and replaces rule Double with the buffered double reading A and B by k. NewBody numbers its
// bodies in call order: the runtime builds one per shard, shard by shard.
func eqPlan(t testing.TB, plant bool) *plan.App {
	t.Helper()
	p := pbBuild(t, "qa-eq.cep", qaEqSrc)
	var mu sync.Mutex
	n := int64(0)
	p.Rules[2] = plan.Rule{Name: "Double", Inputs: []int{eqA, eqB}, KeyAttr: []int{0, 0}, KeyType: event.TString, TTL: 3 * ldMin,
		NewBody: func(emit func(event.Event)) partition.Body {
			mu.Lock()
			defer mu.Unlock()
			b := &eqDouble{emit: emit, shard: n, plant: plant}
			n++
			return b
		}}
	return p
}

// eqReaderApp is the format layer's view of qaEqSrc, written by hand so a generated log can be read before Build.
func eqReaderApp(t testing.TB) *playback.App {
	t.Helper()
	ts := ldAttr("ts", event.TTimestamp)
	return &playback.App{
		Streams: []playback.Stream{
			{Schema: ldSchema(t, "A", 3, ldOpt("k", event.TString), ldAttr("v", event.TInt), ldAttr("d", event.TInt), ts), Delay: ldMin},
			{Schema: ldSchema(t, "B", 2, ldAttr("k", event.TString), ldAttr("v", event.TInt), ts), Delay: 30 * ldSec, IdleAfter: ldMin},
		},
		Tables: []playback.Table{{Name: "T", Attrs: []event.Attr{ldAttr("k", event.TString)}, Key: []string{"k"}}},
	}
}

// eqGenerate writes a recording: a header at 10:00, 40 records (events on A and B with arrival stamps that never
// go back, event times up to 90 s behind the arrival — some stream-late —, keys a–e and, on A, a null key; a
// divisor that is sometimes 0; puts and deletes on T; ticks), then a stop.
func eqGenerate(seed uint64) string {
	rng := rand.New(rand.NewPCG(seed, 0x5eed))
	keys := []string{"a", "b", "c", "d", "e"}
	lines := []string{fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0))}
	wall := ldT0
	for range 40 {
		wall += rng.Int64N(20 * ldSec)
		switch r := rng.IntN(100); {
		case r < 55:
			attrs := fmt.Sprintf(`"v":%d,"d":%d,"ts":%q`, rng.IntN(10), rng.IntN(4), ldRFC(wall-rng.Int64N(90*ldSec)))
			if rng.IntN(10) > 0 {
				attrs += fmt.Sprintf(`,"k":%q`, keys[rng.IntN(len(keys))])
			}
			lines = append(lines, fmt.Sprintf(`{"stream":"A","arrival":%q,"attrs":{%s}}`, ldRFC(wall), attrs))
		case r < 88:
			lines = append(lines, fmt.Sprintf(`{"stream":"B","arrival":%q,"attrs":{"k":%q,"v":%d,"ts":%q}}`,
				ldRFC(wall), keys[rng.IntN(len(keys))], rng.IntN(10), ldRFC(wall-rng.Int64N(60*ldSec))))
		case r < 94:
			op := "put"
			if rng.IntN(2) == 0 {
				op = "delete"
			}
			lines = append(lines, fmt.Sprintf(`{"table":"T","op":%q,"row":{"k":%q}}`, op, keys[rng.IntN(len(keys))]))
		default:
			lines = append(lines, fmt.Sprintf(`{"tick":%q}`, ldRFC(wall)))
		}
	}
	return ldLog(append(lines, `{"stop":true}`)...)
}

// eqResult is one run's outputs per "stream/key" and its counters.
type eqResult struct {
	out        map[string][]string
	streamLate []int64
	rules      []runtime.RuleStats
}

func eqKey(e event.Event) string {
	k := e.Values[0]
	if k.Null {
		return fmt.Sprintf("s%d/<null>", e.StreamID)
	}
	return fmt.Sprintf("s%d/%s", e.StreamID, k.Str)
}

func eqRender(e event.Event) string { return fmt.Sprintf("@%s %v", ldRFC(e.Ts), e.Values[1:]) }

func (r *eqResult) add(e event.Event) {
	r.out[eqKey(e)] = append(r.out[eqKey(e)], eqRender(e))
}

// eqLive runs recs through runtime.Start with n shards. Config.Now pops the header's start, then each event's
// arrival: the router reads it once at Start and once per event (design §7.8), so any other read fails the test.
func eqLive(t testing.TB, p *plan.App, recs []playback.Record, n int) (*eqResult, error) {
	t.Helper()
	res := &eqResult{out: map[string][]string{}}
	var mu sync.Mutex
	stamps := []int64{}
	for _, rec := range recs {
		switch rec.Kind {
		case playback.RecHeader:
			stamps = append(stamps, rec.Time)
		case playback.RecEvent:
			stamps = append(stamps, rec.Arrival)
		}
	}
	extra := 0
	now := func() int64 {
		mu.Lock()
		defer mu.Unlock()
		if len(stamps) == 0 {
			extra++
			return ldT0
		}
		v := stamps[0]
		stamps = stamps[1:]
		return v
	}
	sink := func(e event.Event) error { mu.Lock(); res.add(e); mu.Unlock(); return nil }
	sinks := []runtime.SinkBinding{{Stream: eqOK, Sink: sink}, {Stream: eqOG, Sink: sink}, {Stream: eqOD, Sink: sink}}
	r, err := runtime.Start(runtime.Config{Shards: n, Now: now, TickEvery: 0}, []runtime.App{runtime.FromPlan(p, pbTables(t, p), sinks)})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer r.Close(ctx)
	for _, rec := range recs {
		switch rec.Kind {
		case playback.RecEvent:
			err = r.Send(ctx, 0, rec.Event)
		case playback.RecTable:
			op := runtime.TablePut
			if rec.Op == playback.OpDelete {
				op = runtime.TableDelete
			}
			err = r.TableWrite(ctx, 0, rec.Table, op, [][]event.Value{rec.Row})
		case playback.RecTick:
			err = r.Tick(ctx, rec.Time)
		case playback.RecStop:
			_, err = r.Drain(ctx, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w (state: the live runtime accepting the record)", rec.Line, err)
		}
	}
	st, err := r.Stats(ctx)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	if extra > 0 {
		return nil, fmt.Errorf("Config.Now read %d times beyond once at Start and once per event (design §7.8)", extra)
	}
	res.streamLate, res.rules = st.StreamLate[0], st.Rules[0]
	return res, nil
}

// eqSync replays recs through runtime.Sync directly (recording mode from the header's start): every counter.
func eqSync(t testing.TB, p *plan.App, recs []playback.Record) *eqResult {
	t.Helper()
	res := &eqResult{out: map[string][]string{}}
	var s *runtime.Sync
	for _, rec := range recs {
		switch rec.Kind {
		case playback.RecHeader:
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("runtime.Sync panicked: %v", r)
					}
				}()
				var err error
				s, err = runtime.NewSync(runtime.FromPlan(p, pbTables(t, p), nil), rec.Time, false, res.add)
				if err != nil {
					t.Fatalf("NewSync: %v", err)
				}
			}()
		case playback.RecEvent:
			e := rec.Event
			pbSyncEvent(s, &e, rec.Arrival)
		case playback.RecTable:
			op := runtime.TablePut
			if rec.Op == playback.OpDelete {
				op = runtime.TableDelete
			}
			if err := s.TableWrite(rec.Table, op, [][]event.Value{rec.Row}); err != nil {
				t.Fatalf("line %d: Sync.TableWrite: %v", rec.Line, err)
			}
		case playback.RecTick:
			s.Tick(rec.Time)
		case playback.RecStop:
			s.Drain()
		}
	}
	st := s.Stats()
	res.streamLate, res.rules = st.StreamLate[0], st.Rules[0]
	return res
}

// eqDriver replays log through NewPlanDriver: the outputs and the stream-late steps per stream.
func eqDriver(t testing.TB, p *plan.App, log string) *eqResult {
	t.Helper()
	res := &eqResult{out: map[string][]string{}, streamLate: make([]int64, len(p.Streams))}
	outs, steps := pbReplay(t, p, log)
	for _, o := range outs {
		res.add(o.Ev)
	}
	for _, s := range steps {
		if s.Kind == playback.StepStreamLate {
			res.streamLate[s.Event.StreamID]++
		}
	}
	return res
}

// eqDiff names the first difference between two runs ("" when equal): per stream/key sequences (equal sequences
// for every key make the multisets equal), then the counters when both runs have them.
func eqDiff(a, b *eqResult, counters bool) string {
	keys := map[string]bool{}
	for k := range a.out {
		keys[k] = true
	}
	for k := range b.out {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		if !slices.Equal(a.out[k], b.out[k]) {
			return fmt.Sprintf("%s: %v vs %v", k, a.out[k], b.out[k])
		}
	}
	if !slices.Equal(a.streamLate, b.streamLate) {
		return fmt.Sprintf("stream-late %v vs %v", a.streamLate, b.streamLate)
	}
	if counters {
		for i := range a.rules {
			x, y := a.rules[i], b.rules[i]
			if x.ConstructLate != y.ConstructLate || x.Purged != y.Purged || x.Suppressed != y.Suppressed || x.Errored != y.Errored || x.KeyErrors != y.KeyErrors {
				return fmt.Sprintf("rule %d: %+v vs %+v", i, x, y)
			}
		}
	}
	return ""
}

// eqCompare runs one seed live with n shards and in both replays; it returns the first difference, or "".
func eqCompare(t testing.TB, seed uint64, n int, plant bool) (diff, log string) {
	t.Helper()
	log = eqGenerate(seed)
	recs := ldRecordsOK(t, eqReaderApp(t), log)
	live, err := eqLive(t, eqPlan(t, plant), recs, n)
	if err != nil {
		t.Fatalf("seed %d, %d shards: live: %v\nrecording:\n%s", seed, n, err, log)
	}
	if d := eqDiff(live, eqSync(t, eqPlan(t, plant), recs), true); d != "" {
		return "live vs Sync: " + d, log
	}
	if d := eqDiff(live, eqDriver(t, eqPlan(t, plant), log), false); d != "" {
		return "live vs NewPlanDriver: " + d, log
	}
	return "", log
}

// TestLiveEqualsReplay is exit 1: 200 seeds × N ∈ {1, 2, 4, 7}. Over all seeds the recordings must exercise what
// the comparison is about: every rule emits, the rate limits suppress, the division errors, a key is null, an
// event is stream-late, a key purges.
func TestLiveEqualsReplay(t *testing.T) {
	t.Parallel()
	var total runtime.RuleStats
	var late int64
	outs := map[int]int{}
	for seed := range uint64(200) {
		for _, n := range []int{1, 2, 4, 7} {
			if d, log := eqCompare(t, seed, n, false); d != "" {
				t.Fatalf("seed %d, %d shards: %s\nrecording:\n%s", seed, n, d, log)
			}
		}
		res := eqSync(t, eqPlan(t, false), ldRecordsOK(t, eqReaderApp(t), eqGenerate(seed)))
		for _, rs := range res.rules {
			total.Suppressed += rs.Suppressed
			total.Errored += rs.Errored
			total.KeyErrors += rs.KeyErrors
			total.Purged += rs.Purged
		}
		for _, x := range res.streamLate {
			late += x
		}
		for k, v := range res.out {
			outs[int(k[1]-'0')] += len(v)
		}
	}
	if total.Suppressed == 0 || total.Errored == 0 || total.KeyErrors == 0 || total.Purged == 0 || late == 0 || outs[eqOK] == 0 || outs[eqOG] == 0 || outs[eqOD] == 0 {
		t.Errorf("the generator does not exercise the comparison: totals %+v, stream-late %d, outputs per stream %v", total, late, outs)
	}
}

// TestLiveEqualsReplayTeeth: the planted shard-dependent body makes the comparison fail for every N > 1 within
// the first 20 seeds, and never for N = 1 (one shard: the replay's).
func TestLiveEqualsReplayTeeth(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 4, 7} {
		caught := false
		for seed := range uint64(20) {
			if d, _ := eqCompare(t, seed, n, true); d != "" {
				if n == 1 {
					t.Errorf("seed %d, 1 shard: the plant differs on one shard: %s", seed, d)
				}
				caught = true
				break
			}
		}
		if n > 1 && !caught {
			t.Errorf("%d shards: the planted shard-dependent body was not caught in 20 seeds", n)
		}
	}
}
