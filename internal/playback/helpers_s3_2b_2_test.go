package playback_test

// The shared harness of S3.2b-2's acceptance tests (design note docs/sprints/s3-2b-2-design.md v1, §11), written
// against the stub (eba78a9) before the build. A program's text goes parse → validate.Check → plan.Build (pbBuild),
// then runs live (runtime.Start with a fake Config.Now and a sink on every derived stream, fed through Send and
// SendPayload) or replayed (a log through NewPlanDriver). Each output is rendered in its AppendEvent form, so two
// runs compare byte for byte. Multi-part keys are written only as program text: plan.Rule's and partition's key
// fields change at the build (the brief), so no test here builds them by hand. Prefix: q2.

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// q2Run is one run's outputs and its counters.
type q2Run struct {
	outs  map[int][]string // per derived stream id: outputs in emission order, each in its AppendEvent form
	stats runtime.Stats
}

// q2Render is e's AppendEvent line under its stream's schema.
func q2Render(t testing.TB, p *plan.App, e event.Event) string {
	t.Helper()
	b, err := playback.AppendEvent(nil, p.Streams[e.StreamID].Schema, e)
	if err != nil {
		t.Errorf("AppendEvent(%v): %v", e, err)
		return fmt.Sprintf("<unencodable %v>", e)
	}
	return string(b)
}

// q2Stream is the id of p's stream named name.
func q2Stream(t testing.TB, p *plan.App, name string) int {
	t.Helper()
	for i, s := range p.Streams {
		if s.Schema.Stream() == name {
			return i
		}
	}
	t.Fatalf("no stream %q in the program", name)
	return -1
}

// q2Clock is a fake Config.Now: from start, step ms per read; it keeps every value it handed out.
type q2Clock struct {
	mu         sync.Mutex
	next, step int64
	seen       []int64
}

func q2NewClock(start, step int64) *q2Clock { return &q2Clock{next: start, step: step} }

func (c *q2Clock) now() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.next
	c.next += c.step
	c.seen = append(c.seen, v)
	return v
}

func (c *q2Clock) handedOut() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.seen)
}

// q2Live is one live runtime over p with one app.
type q2Live struct {
	t      testing.TB
	p      *plan.App
	r      *runtime.Runtime
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	res    q2Run
}

// q2Start starts p live on shards shards with now as Config.Now (no ticker) and rec as the app's recorder (nil:
// none); every derived stream gets a sink. The runtime closes at the test's end.
func q2Start(t testing.TB, p *plan.App, shards int, now func() int64, rec runtime.Recorder) *q2Live {
	t.Helper()
	l := &q2Live{t: t, p: p, res: q2Run{outs: map[int][]string{}}}
	var sinks []runtime.SinkBinding
	for i, s := range p.Streams {
		if !s.Derived {
			continue
		}
		sinks = append(sinks, runtime.SinkBinding{Stream: i, Sink: func(e event.Event) error {
			line := q2Render(t, p, e)
			l.mu.Lock()
			l.res.outs[e.StreamID] = append(l.res.outs[e.StreamID], line)
			l.mu.Unlock()
			return nil
		}})
	}
	app := runtime.FromPlan(p, pbTables(t, p), sinks)
	app.Recorder = rec
	r, err := runtime.Start(runtime.Config{Shards: shards, Now: now, TickEvery: 0}, []runtime.App{app})
	if err != nil {
		t.Fatalf("runtime.Start (shards %d, recorder %v): %v", shards, rec != nil, err)
	}
	l.r = r
	l.ctx, l.cancel = context.WithTimeout(context.Background(), 60*time.Second) // only breaks a hang
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = r.Close(cctx)
		l.cancel()
	})
	return l
}

// payload hands one JSON payload of stream to SendPayload; an error to the caller fails the test (a payload that
// fails its schema is not one: design §6).
func (l *q2Live) payload(stream int, js string) {
	l.t.Helper()
	if err := l.r.SendPayload(l.ctx, 0, stream, []byte(js)); err != nil {
		l.t.Fatalf("SendPayload(stream %d, %s): %v", stream, js, err)
	}
}

// finish drains the app (every output through its sink), then takes the counters.
func (l *q2Live) finish() *q2Run {
	l.t.Helper()
	if _, err := l.r.Drain(l.ctx, 0); err != nil {
		l.t.Fatalf("Drain: %v", err)
	}
	st, err := l.r.Stats(l.ctx)
	if err != nil {
		l.t.Fatalf("Stats: %v", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := &q2Run{outs: map[int][]string{}, stats: st}
	for k, v := range l.res.outs {
		out.outs[k] = slices.Clone(v)
	}
	return out
}

// q2Replay replays log through NewPlanDriver (End included): its outputs, and the driver's counters.
func q2Replay(t testing.TB, p *plan.App, log string) *q2Run {
	t.Helper()
	res := &q2Run{outs: map[int][]string{}}
	d, err := playback.NewPlanDriver(p, func(e event.Event) { res.outs[e.StreamID] = append(res.outs[e.StreamID], q2Render(t, p, e)) })
	if err != nil {
		t.Fatalf("NewPlanDriver: %v", err)
	}
	r := ldReader(t, pbApp(p), log)
	for {
		rec, err := r.Next()
		if ldEOF(err) {
			break
		}
		if err != nil {
			t.Fatalf("replaying the log: %v\nlog:\n%s", err, log)
		}
		d.Step(rec)
	}
	d.End()
	res.stats = d.Stats()
	return res
}

// q2StreamStats is app 0's stream counters, guarded: the test fails instead of panicking on a short Stats.
func q2StreamStats(t testing.TB, st runtime.Stats, stream int) runtime.StreamStats {
	t.Helper()
	if len(st.Streams) == 0 || stream >= len(st.Streams[0]) {
		t.Fatalf("Stats.Streams %v has no app 0 stream %d", st.Streams, stream)
	}
	return st.Streams[0][stream]
}

// q2RuleStats is app 0's rule counters, guarded.
func q2RuleStats(t testing.TB, st runtime.Stats, rule int) runtime.RuleStats {
	t.Helper()
	if len(st.Rules) == 0 || rule >= len(st.Rules[0]) {
		t.Fatalf("Stats.Rules %v has no app 0 rule %d", st.Rules, rule)
	}
	return st.Rules[0][rule]
}

// q2Rule is the index of p's rule named name.
func q2Rule(t testing.TB, p *plan.App, name string) int {
	t.Helper()
	for i, r := range p.Rules {
		if r.Name == name {
			return i
		}
	}
	t.Fatalf("no rule %q in the program", name)
	return -1
}

// q2PerKey groups one stream's outputs by their first n attributes (the tests' programs select the key parts
// first), keeping each key's order: per-key order is what live with several shards and replay share.
func q2PerKey(t testing.TB, p *plan.App, lines []string, n int) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, l := range lines {
		// AppendEvent sorts attribute names; the key is recovered from the line by the caller's key names instead.
		out[q2KeyOf(l, n)] = append(out[q2KeyOf(l, n)], l)
	}
	return out
}

// q2KeyOf is a line's grouping key: the values of its "p1".."pn" attributes (the key projections the tests'
// programs name p1, p2, …), in order.
func q2KeyOf(line string, n int) string {
	var parts []string
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf(`"p%d":`, i)
		j := strings.Index(line, name)
		if j < 0 {
			parts = append(parts, "<absent>")
			continue
		}
		rest := line[j+len(name):]
		end := strings.IndexAny(rest, ",}")
		if strings.HasPrefix(rest, `"`) {
			end = strings.Index(rest[1:], `"`) + 2
		}
		parts = append(parts, rest[:end])
	}
	return strings.Join(parts, "|")
}

// q2Diff names the first difference between two runs' outputs, per stream and per key of n parts ("" if equal).
func q2Diff(t testing.TB, p *plan.App, a, b *q2Run, n int) string {
	t.Helper()
	streams := map[int]bool{}
	for k := range a.outs {
		streams[k] = true
	}
	for k := range b.outs {
		streams[k] = true
	}
	ids := make([]int, 0, len(streams))
	for k := range streams {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	for _, s := range ids {
		ka, kb := q2PerKey(t, p, a.outs[s], n), q2PerKey(t, p, b.outs[s], n)
		keys := map[string]bool{}
		for k := range ka {
			keys[k] = true
		}
		for k := range kb {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			if !slices.Equal(ka[k], kb[k]) {
				return fmt.Sprintf("stream %d, key %s:\n  %v\nvs\n  %v", s, k, ka[k], kb[k])
			}
		}
	}
	return ""
}

// q2Header is a recording's header line.
func q2Header(start int64) string { return fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(start)) }
