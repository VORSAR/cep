package playback_test

// The corpus the gate's tripwire and two-OS determinism rows run (design s3-2b-2 v2 §8, §9; review F10, F11):
// examples 07, 09 and 10 and the equivalence test's app, each fed seeded recordings — seeds 1 to 20 — written by
// cgGenerate (the examples) or eqGenerate (the equivalence app).

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// cgProgram is one corpus program: its plan and its seeded recordings.
type cgProgram struct {
	name string
	p    *plan.App
	log  func(seed uint64) string
}

const cgSeeds = 20

// cgCorpus is the corpus, in a fixed order.
func cgCorpus(t testing.TB) []cgProgram {
	t.Helper()
	var out []cgProgram
	for _, name := range []string{"07-consent-gate", "09-params-and-quoting", "10-grouped-keys"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "docs", "language", "examples", name+".cep"))
		if err != nil {
			t.Fatal(err)
		}
		p := pbBuild(t, name+".cep", string(src))
		out = append(out, cgProgram{name: name, p: p, log: func(seed uint64) string { return cgGenerate(p, seed) }})
	}
	return append(out, cgProgram{name: "qa-eq", p: eqPlan(t, false), log: eqGenerate})
}

// cgStrings is the string pool: values the corpus programs' filters test for, and one with a zero-width space.
var cgStrings = []string{"a", "b", "t1", "t2", "alice", "bob", "prod", "staging", "admin", "read", "crm", "rogue", "x\u200by"}

// cgGenerate writes a recording for p: a header at 10:00, 40 records — events on p's source streams (arrival stamps
// that never go back; event times up to 30 s behind, some stream-late; an optional attribute sometimes absent),
// puts and deletes on p's tables, ticks — then a stop.
func cgGenerate(p *plan.App, seed uint64) string {
	rng := rand.New(rand.NewPCG(seed, 0xc0ffee))
	var sources []int
	for i, s := range p.Streams {
		if !s.Derived {
			sources = append(sources, i)
		}
	}
	lines := []string{fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0))}
	wall := ldT0
	for range 40 {
		wall += rng.Int64N(10 * ldSec)
		switch r := rng.IntN(100); {
		case r < 80 || len(p.Tables) == 0 && r < 95:
			sid := sources[rng.IntN(len(sources))]
			s := p.Streams[sid].Schema
			var attrs []string
			for i := range s.NumAttrs() {
				a := s.Attr(i)
				if a.Name == event.InvisiblesAttr || (a.Optional && rng.IntN(8) == 0) {
					continue
				}
				var v string
				switch a.Type {
				case event.TString:
					v = fmt.Sprintf("%q", cgStrings[rng.IntN(len(cgStrings))])
				case event.TInt:
					v = fmt.Sprint(rng.IntN(10))
				case event.TFloat:
					v = fmt.Sprintf("%.2f", rng.Float64())
				case event.TBool:
					v = fmt.Sprint(rng.IntN(2) == 0)
				case event.TTimestamp:
					at := wall
					if i == s.TimeAttr() {
						at -= rng.Int64N(30 * ldSec)
					}
					v = fmt.Sprintf("%q", ldRFC(at))
				default:
					panic(fmt.Sprintf("cgGenerate: attribute type %v", a.Type))
				}
				attrs = append(attrs, fmt.Sprintf("%q:%s", a.Name, v))
			}
			rec := fmt.Sprintf(`{"stream":%q,"attrs":{%s}`, s.Stream(), strings.Join(attrs, ","))
			if s.TimeAttr() == event.ArrivalTime {
				rec += fmt.Sprintf(`,"ts":%q}`, ldRFC(wall))
			} else {
				rec += fmt.Sprintf(`,"arrival":%q}`, ldRFC(wall))
			}
			lines = append(lines, rec)
		case r < 95:
			tb := p.Tables[rng.IntN(len(p.Tables))]
			op := "put"
			if rng.IntN(3) == 0 {
				op = "delete"
			}
			lines = append(lines, fmt.Sprintf(`{"table":%q,"op":%q,"row":{%q:%q}}`, tb.Name, op, tb.Key[0], cgStrings[rng.IntN(len(cgStrings))]))
		default:
			lines = append(lines, fmt.Sprintf(`{"tick":%q}`, ldRFC(wall)))
		}
	}
	return ldLog(append(lines, `{"stop":true}`)...)
}

// cgLive runs recs live on two shards with a sink on every derived stream: the counters and the drain's result.
func cgLive(t testing.TB, p *plan.App, recs []playback.Record) (runtime.Stats, runtime.DrainResult) {
	t.Helper()
	var mu sync.Mutex
	var stamps []int64
	for _, rec := range recs {
		switch rec.Kind {
		case playback.RecHeader:
			stamps = append(stamps, rec.Time)
		case playback.RecEvent:
			stamps = append(stamps, rec.Arrival)
		}
	}
	now := func() int64 {
		mu.Lock()
		defer mu.Unlock()
		if len(stamps) == 0 {
			return ldT0
		}
		v := stamps[0]
		stamps = stamps[1:]
		return v
	}
	var sinks []runtime.SinkBinding
	for i, s := range p.Streams {
		if s.Derived {
			sinks = append(sinks, runtime.SinkBinding{Stream: i, Sink: func(event.Event) error { return nil }})
		}
	}
	r, err := runtime.Start(runtime.Config{Shards: 2, Now: now}, []runtime.App{runtime.FromPlan(p, pbTables(t, p), sinks)})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer r.Close(ctx)
	var drained runtime.DrainResult
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
			drained, err = r.Drain(ctx, 0)
		}
		if err != nil {
			t.Fatalf("line %d: %v", rec.Line, err)
		}
	}
	st, err := r.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return st, drained
}

// twCeiling is one metric's entry in gate/tripwire.json (design s3-2b-2 §8; ADR-0015 §3.11 as annotated): a ceiling
// (a number, or "none" with a reason), blocking or advisory, and per-rule overrides.
type twCeiling struct {
	Ceiling  any              `json:"ceiling"`
	Blocking bool             `json:"blocking"`
	Reason   string           `json:"reason"`
	Rules    map[string]int64 `json:"rules"`
}

// twTracked are the metrics the tripwire tracks; each MUST have an entry (zero-config-is-failure).
var twTracked = []string{"outbox", "sink-depth", "drained_pending", "partial-count"}

// TestTripwire is the T4 tripwire row (S3.2b's item 7, exit 6; design s3-2b-2 §8): every tracked metric has a
// ceiling, and on a live run of the corpus no high-water mark is over a blocking ceiling; one over an advisory
// ceiling is reported. CEP_TRIPWIRE_FILE overrides the ceilings' file, for the gate's proofs.
func TestTripwire(t *testing.T) {
	path := os.Getenv("CEP_TRIPWIRE_FILE")
	if path == "" {
		path = filepath.Join("..", "..", "gate", "tripwire.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]twCeiling
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	ceilings := map[string]twCeiling{}
	for _, m := range twTracked {
		c, ok := file[m]
		switch {
		case !ok:
			t.Errorf("no ceiling configured for %s (%s)", m, path)
			continue
		case c.Ceiling == "none" && c.Reason == "":
			t.Errorf("%s: ceiling none needs a reason (%s)", m, path)
			continue
		}
		ceilings[m] = c
	}
	check := func(metric, name string, v int64) {
		c, ok := ceilings[metric]
		if !ok || c.Ceiling == "none" {
			return
		}
		limit, isNum := c.Ceiling.(float64)
		if !isNum {
			t.Errorf("%s: ceiling %v is neither a number nor none", metric, c.Ceiling)
			return
		}
		if o, ok := c.Rules[name]; ok {
			limit = float64(o)
		}
		if float64(v) <= limit {
			return
		}
		if c.Blocking {
			t.Errorf("%s/%s = %d, over its blocking ceiling %v", metric, name, v, limit)
		} else {
			t.Logf("advisory: %s/%s = %d, over its ceiling %v", metric, name, v, limit)
		}
	}
	for _, prog := range cgCorpus(t) {
		for seed := uint64(1); seed <= cgSeeds; seed++ {
			st, drained := cgLive(t, prog.p, ldRecordsOK(t, prog.p, prog.log(seed)))
			for ri, rs := range st.Rules[0] {
				check("outbox", prog.p.Rules[ri].Name, rs.OutboxHigh)
			}
			b := 0
			for _, s := range prog.p.Streams {
				if s.Derived {
					check("sink-depth", s.Schema.Stream(), int64(st.SinkHigh[0][b]))
					b++
				}
			}
			check("drained_pending", prog.name, drained.Pending)
		}
	}
}

// TestDeterminismGolden is the T4 determinism row (S3.2b's item 8, exit 7; design s3-2b-2 §9): the corpus replayed
// through the plan driver gives, byte for byte, the committed golden — on Linux in CI and on macOS, the second OS
// (owner Q5). CEP_UPDATE_GOLDEN=1 rewrites the golden; CEP_DETERMINISM_PLANT=maporder plants a nondeterminism in
// this path — 64 keys in map iteration order — for the gate's proof (review F11).
func TestDeterminismGolden(t *testing.T) {
	var b strings.Builder
	for _, prog := range cgCorpus(t) {
		for seed := uint64(1); seed <= cgSeeds; seed++ {
			var lines []string
			d, err := playback.NewPlanDriver(prog.p, func(e event.Event) {
				line, err := playback.AppendEvent(nil, prog.p.Streams[e.StreamID].Schema, e)
				if err != nil {
					t.Fatalf("%s seed %d: %v", prog.name, seed, err)
				}
				lines = append(lines, string(line))
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, rec := range ldRecordsOK(t, prog.p, prog.log(seed)) {
				d.Step(rec)
			}
			d.End()
			for _, l := range lines {
				fmt.Fprintf(&b, "%s %d %s\n", prog.name, seed, l)
			}
		}
	}
	if os.Getenv("CEP_DETERMINISM_PLANT") == "maporder" {
		keys := map[string]bool{}
		for i := range 64 {
			keys[fmt.Sprintf("k%02d", i)] = true
		}
		for k := range keys {
			fmt.Fprintf(&b, "plant %s\n", k)
		}
	}
	golden := filepath.Join("testdata", "determinism.golden")
	if os.Getenv("CEP_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (CEP_UPDATE_GOLDEN=1 writes it)", err)
	}
	if got := b.String(); got != string(want) {
		g, w := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range min(len(g), len(w)) {
			if g[i] != w[i] {
				t.Fatalf("the replayed corpus differs from %s at line %d:\n got  %s\n want %s", golden, i+1, g[i], w[i])
			}
		}
		t.Fatalf("the replayed corpus has %d lines, %s %d", len(g), golden, len(w))
	}
}
