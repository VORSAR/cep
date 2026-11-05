package playback_test

// S3.2a-2's tests that need the playback side (design note docs/sprints/s3-2a-2-design.md v1, §1, §8):
//   - TestIngestMatchesDriver: runtime.Ingest driven by hand over a log equals playback.Driver's steps;
//   - TestLiveOrderPerKeyEqualsReplay: the live runtime equals a replay per key. It lives here, not in
//     internal/runtime, because tools/importcheck/rules.json lets playback import runtime and never the reverse
//     (runtime's test files included), and the replay side is this package's ld trace harness (gap Q1).

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// ---- TestIngestMatchesDriver ----

// igStep is what one ingest step yields, for comparison.
type igStep struct {
	kind   string
	seq    uint64
	late   bool
	tables any
	adv    []clock.WMAdvance
}

// TestIngestMatchesDriver: the shared ingest step (design §1) equals the driver's arithmetic on one log, in a fixture
// (virtual) and in a recording (non-virtual, header start, arrival stamps, ticks, stop).
//
// The reference is playback.Driver.Step over the Reader's records. The subject is runtime.NewIngest over the SAME
// table set (d.Tables(): the driver applies the table records; table.Set.Snapshot returns one pointer while nothing
// changed, so equal pointers mean "the same snapshot"), driven by hand: an event record → Event(&e, arrival) with
// arrival = Ts in a fixture and the record's Arrival in a recording; a tick → Tick(time); a stop → Drain(); a
// fixture's end → End(). Per step: Seq, late, the Tables pointer and the advances must be equal; Snapshot() equals
// the step's Tables; at the end, Walk's "seq" holders and the registry's Walk equal the driver's.
func TestIngestMatchesDriver(t *testing.T) {
	app := &playback.App{
		Streams: []playback.Stream{ldStd(t, "S0", 1*ldMin, 30*ldSec), ldStd(t, "S1", 10*ldSec, 0)},
		Tables:  []playback.Table{ldTableT()},
	}
	at := func(s float64) string { return fmt.Sprintf(`,"arrival":%q`, ldRFC(ldAt(s))) }
	fixture := ldLog(
		ldE("S0", ldAt(0), "a", "", ""),
		`{"table":"T","op":"put","row":{"x":"p"}}`,
		ldE("S1", ldAt(5), "b", "", ""),
		ldE("S0", ldAt(70), "a", "", ""),
		ldE("S0", ldAt(1), "c", "", ""), // stream-late on S0 (1 s < 70 s − 60 s)
		`{"table":"T","op":"delete","row":{"x":"p"}}`,
		ldE("S1", ldAt(80), "b", "", ""),
	)
	recording := ldLog(
		fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0)),
		ldE("S0", ldAt(0), "a", "", at(1)),
		`{"table":"T","op":"put","row":{"x":"p"}}`,
		ldE("S1", ldAt(5), "b", "", at(6)),
		fmt.Sprintf(`{"tick":%q}`, ldRFC(ldAt(60))),
		ldE("S0", ldAt(70), "a", "", at(71)),
		ldE("S0", ldAt(1), "c", "", at(72)),
		`{"stop":true}`,
	)
	for _, tc := range []struct{ name, log string }{{"fixture", fixture}, {"recording", recording}} {
		t.Run(tc.name, func(t *testing.T) {
			recs, err := ldRecords(t, app, tc.log)
			if !ldEOF(err) {
				t.Fatalf("reading the log: %v", err)
			}
			d, err := playback.NewDriver(app)
			if err != nil {
				t.Fatalf("NewDriver: %v", err)
			}
			var specs []clock.StreamSpec
			for _, s := range app.Streams {
				specs = append(specs, clock.StreamSpec{Name: s.Schema.Stream(), Delay: s.Delay, IdleAfter: s.IdleAfter, Derived: s.Derived})
			}
			g := runtime.NewIngest(specs, 0, d.Tables(), true)
			recording := false
			var want, got []igStep
			for _, rec := range recs {
				st := d.Step(rec)
				switch st.Kind {
				case playback.StepHeader:
					recording = true
					g = runtime.NewIngest(specs, rec.Time, d.Tables(), false)
				case playback.StepEvent, playback.StepStreamLate:
					want = append(want, igStep{"event", st.Event.Seq, st.Kind == playback.StepStreamLate, st.Event.Tables, slices.Clone(st.Advances)})
					e := rec.Event
					arrival := e.Ts
					if recording {
						arrival = rec.Arrival
					}
					late, adv := g.Event(&e, arrival)
					got = append(got, igStep{"event", e.Seq, late, e.Tables, slices.Clone(adv)})
					if snap := g.Snapshot(); snap != st.Tables {
						t.Errorf("line %d: Snapshot() %p, want the step's %p", rec.Line, snap, st.Tables)
					}
				case playback.StepTick:
					want = append(want, igStep{kind: "tick", adv: slices.Clone(st.Advances)})
					got = append(got, igStep{kind: "tick", adv: slices.Clone(g.Tick(rec.Time))})
				case playback.StepStop:
					want = append(want, igStep{kind: "stop", adv: slices.Clone(st.Advances)})
					got = append(got, igStep{kind: "stop", adv: slices.Clone(g.Drain())})
				}
			}
			end := d.End()
			if !recording {
				want = append(want, igStep{kind: "end", adv: slices.Clone(end.Advances)})
				got = append(got, igStep{kind: "end", adv: slices.Clone(g.End())})
			}
			if len(got) != len(want) {
				t.Fatalf("steps: %d, want %d", len(got), len(want))
			}
			for i := range want {
				w, h := want[i], got[i]
				if w.kind != h.kind || w.seq != h.seq || w.late != h.late || w.tables != h.tables || !slices.Equal(w.adv, h.adv) {
					t.Errorf("step %d: ingest %+v,\nwant driver's %+v", i, h, w)
				}
			}
			var dw, gw []string
			d.Walk(func(c, k string, s any) { dw = append(dw, fmt.Sprintf("%s/%s=%v", c, k, s)) })
			g.Walk(func(c, k string, s any) { gw = append(gw, fmt.Sprintf("%s/%s=%v", c, k, s)) })
			if !slices.Equal(dw, gw) {
				t.Errorf("Walk: ingest %v, want the driver's %v", gw, dw)
			}
			dw, gw = nil, nil
			d.Registry().Walk(func(c, k string, s any) { dw = append(dw, fmt.Sprintf("%s/%s=%+v", c, k, s)) })
			g.Registry().Walk(func(c, k string, s any) { gw = append(gw, fmt.Sprintf("%s/%s=%+v", c, k, s)) })
			if !slices.Equal(dw, gw) {
				t.Errorf("registry: ingest %v,\nwant the driver's %v", gw, dw)
			}
		})
	}
}

// ---- TestLiveOrderPerKeyEqualsReplay ----

// lvLog is the live side's per-key record, shared by every shard's body.
type lvLog struct {
	mu  sync.Mutex
	per map[string][]string
}

func (l *lvLog) add(k, e string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.per[k] = append(l.per[k], e)
}

// lvBody is the live twin of the ld harness's ldBody: the same Sched, the same records, per (rule, shard).
type lvBody struct {
	log  *lvLog
	spec *ldRuleSpec
}

func (b *lvBody) NewState(*partition.Instance) partition.OpState { return &ldState{} }

func (b *lvBody) OnEvent(in *partition.Instance, e event.Event) {
	b.log.add(b.spec.Name+"/"+in.Key.Str, lvEv(e.StreamID, e.Seq, e.Ts))
	st := in.Op.(*ldState)
	st.n++
	if b.spec.Sched != nil {
		for _, tm := range b.spec.Sched(e, st.n) {
			in.Clock.Schedule(tm.Due, tm.Kind, tm.Ref)
			st.drop(tm.Kind, tm.Ref)
			st.timers = append(st.timers, tm)
		}
	}
}

func (b *lvBody) OnTimers(in *partition.Instance, fires []clock.Timer, _ any) {
	st := in.Op.(*ldState)
	for _, tm := range fires {
		b.log.add(b.spec.Name+"/"+in.Key.Str, lvTm(tm))
		st.drop(tm.Kind, tm.Ref)
	}
}

func (b *lvBody) OnDiscard(*partition.Instance, []uint64) {}

func (b *lvBody) OnPurge(in *partition.Instance) {
	b.log.add(b.spec.Name+"/"+in.Key.Str, lvPurge(in.LastActivity+b.spec.TTL))
}

func lvEv(sid int, seq uint64, ts int64) string {
	return fmt.Sprintf("ev s%d#%d @%d", sid, seq, ts-ldT0)
}
func lvTm(tm clock.Timer) string {
	return fmt.Sprintf("tm k%d @%d r%d", tm.Kind, tm.Due-ldT0, tm.Ref)
}
func lvPurge(due int64) string { return fmt.Sprintf("purge @%d", due-ldT0) }

// TestLiveOrderPerKeyEqualsReplay (design v2 §7: per key, events reach the body in the order the clock releases them,
// which equals replay's for the same admitted events; SEMANTICS §2.1, §2.2; review F15).
//
// One RECORDING log: a header (start = T0), event lines with arrival stamps, no ticks, and a stop record. A recording
// replays non-virtually with the header's start (owner D3 of S3.2a-1), and its stop is the registry's Drain (max
// admitted ts + 1 ms) — the same arithmetic as a live run with no ticks ended by Drain. So every key's sequence must be
// equal, with nothing filtered. Streams A and B (Delay 5 s, no idleness; schema ts, k, x, v). Rules: R1 reads A and B
// by k, TTL 20 s, schedules Completion at ts + 3 s (ref = the key's release count since birth); R2 reads A by k, TTL
// 30 s, schedules DelayDue at ts + 7 s, ref 1 (re-scheduled on each release). Events (s): A0 k1, B1 k2, A3 k1,
// A2 k3, B4 k1, A6 k2, B5 k3, A10 k1, B9 k2, A12 k4, B13 k1, A4 k1 (stream-late: 4 < 12 − 5 on A), A40 k2, B38 k4,
// A41 k4, B42 k2; each arrival = T0 + its line's index s (monotone; it moves nothing without ticks). The stop
// finalizes every instant ≤ 42 s: k1's and k3's TTL purges (R1: 13 + 20 = 33, 5 + 20 = 25; R2: k1 10 + 30 = 40)
// fire in both; timers due after 42 s stay pending in both. The live run's Seq per stream is assigned in Send order,
// the stream-late event included, as the driver's is.
//
// Expected: for Shards 1 and 4, every "rule/key" sequence equals the replay's; stream_late counts 1 on A; the
// replay shows at least one purge (the comparison covers resets too).
func TestLiveOrderPerKeyEqualsReplay(t *testing.T) {
	app := &playback.App{Streams: []playback.Stream{ldStd(t, "A", 5*ldSec, 0), ldStd(t, "B", 5*ldSec, 0)}}
	specs := []ldRuleSpec{
		{Name: "R1", Inputs: []int{0, 1}, Key: "k", TTL: 20 * ldSec, Sched: func(e event.Event, n uint64) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 3*ldSec, Kind: clock.Completion, Ref: n}}
		}},
		{Name: "R2", Inputs: []int{0}, Key: "k", TTL: 30 * ldSec, Sched: func(e event.Event, n uint64) []clock.Timer {
			return []clock.Timer{{Due: e.Ts + 7*ldSec, Kind: clock.DelayDue, Ref: 1}}
		}},
	}
	type ev struct {
		s  string
		ts float64
		k  string
	}
	evs := []ev{{"A", 0, "k1"}, {"B", 1, "k2"}, {"A", 3, "k1"}, {"A", 2, "k3"}, {"B", 4, "k1"}, {"A", 6, "k2"},
		{"B", 5, "k3"}, {"A", 10, "k1"}, {"B", 9, "k2"}, {"A", 12, "k4"}, {"B", 13, "k1"}, {"A", 4, "k1"},
		{"A", 40, "k2"}, {"B", 38, "k4"}, {"A", 41, "k4"}, {"B", 42, "k2"}}
	lines := []string{fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0))}
	for i, e := range evs {
		lines = append(lines, ldE(e.s, ldAt(e.ts), e.k, "", fmt.Sprintf(`,"arrival":%q`, ldRFC(ldAt(float64(i))))))
	}
	lines = append(lines, `{"stop":true}`)
	log := ldLog(lines...)

	w := ldRun(t, app, log, specs...)
	if cl := ldFilter(w.log, "construct_late"); len(cl) != 0 {
		t.Fatalf("the recording is not construct-late free under replay:%s", ldEntries(cl))
	}
	if sl := ldFilter(w.log, "stream_late"); len(sl) != 1 {
		t.Fatalf("replay stream_late %d, want 1 (A4 k1)", len(sl))
	}
	if len(ldFilter(w.log, "purge")) == 0 {
		t.Fatalf("the replay purged nothing: the comparison would not cover resets")
	}
	want := map[string][]string{}
	for _, e := range w.log {
		k := e.Rule + "/" + e.Key
		switch e.K {
		case "event":
			want[k] = append(want[k], lvEv(e.Stream, e.Seq, e.Ts))
		case "timer":
			want[k] = append(want[k], lvTm(clock.Timer{Due: e.Due, Kind: e.Kind, Ref: e.Ref}))
		case "purge":
			want[k] = append(want[k], lvPurge(e.Due))
		}
	}

	recs, err := ldRecords(t, app, log)
	if !ldEOF(err) {
		t.Fatalf("reading the log: %v", err)
	}
	for _, shards := range []int{1, 4} {
		t.Run(fmt.Sprintf("shards=%d", shards), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			live := &lvLog{per: map[string][]string{}}
			var rules []runtime.Rule
			for i := range specs {
				spec := &specs[i]
				keyAttr := make([]int, len(spec.Inputs))
				for j := range keyAttr {
					keyAttr[j] = 1 // ldStd: ts, k, x, v
				}
				rules = append(rules, runtime.Rule{Name: spec.Name, Inputs: spec.Inputs, KeyAttr: keyAttr,
					KeyType: event.TString, TTL: spec.TTL,
					NewBody: func(runtime.Emit) partition.Body { return &lvBody{log: live, spec: spec} }})
			}
			var streams []clock.StreamSpec
			for _, s := range app.Streams {
				streams = append(streams, clock.StreamSpec{Name: s.Schema.Stream(), Delay: s.Delay, IdleAfter: s.IdleAfter})
			}
			var schemas []event.Schema
			for _, s := range app.Streams {
				schemas = append(schemas, s.Schema)
			}
			r, err := runtime.Start(runtime.Config{Shards: shards, Now: func() int64 { return ldT0 }},
				[]runtime.App{{Streams: streams, Schemas: schemas, Rules: rules}})
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer func() {
				if err := r.Close(ctx); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			for _, rec := range recs {
				if rec.Kind != playback.RecEvent {
					continue // the header and the stop: the live run's are Start and Drain
				}
				e := rec.Event
				e.Seq, e.Tables = 0, nil
				if err := r.Send(ctx, 0, e); err != nil {
					t.Fatalf("Send line %d: %v", rec.Line, err)
				}
			}
			if _, err := r.Drain(ctx, 0); err != nil {
				t.Fatalf("Drain: %v (state: the drain barrier completing)", err)
			}
			st, err := r.Stats(ctx)
			if err != nil {
				t.Fatalf("Stats: %v", err)
			}
			if st.StreamLate[0][0] != 1 || st.StreamLate[0][1] != 0 {
				t.Errorf("stream_late %v, want [1 0] (A4 k1)", st.StreamLate[0])
			}
			live.mu.Lock()
			got := live.per
			live.mu.Unlock()
			keys := map[string]bool{}
			for k := range want {
				keys[k] = true
			}
			for k := range got {
				keys[k] = true
			}
			var sorted []string
			for k := range keys {
				sorted = append(sorted, k)
			}
			sort.Strings(sorted)
			for _, k := range sorted {
				if !slices.Equal(got[k], want[k]) {
					t.Errorf("%s: live\n\t%s\nwant (the recording's replay)\n\t%s", k, strings.Join(got[k], "\n\t"), strings.Join(want[k], "\n\t"))
				}
			}
		})
	}
}
