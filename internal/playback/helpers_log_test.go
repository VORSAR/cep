package playback_test

// The log-side test harness for internal/playback (design note s3-2a-1 v2 §1, §5.2, §5.3; owner D1–D6): a test
// composition that drives playback.Replay through partition.Rule the way S3.2a-2's router will — a test double for
// that router, not a mirror of any logic under test. For each Step (design §1):
//   1. apply its Advances to every rule that reads the advanced stream;
//   2. offer a StepEvent's event to every rule that reads its stream, under the attribute the test names as its key;
//   3. then Advance(step.Tables) every rule and check the derivability invariant (Rule.CheckTimers).
// Everything is recorded twice: into a playback.Trace (exit 1 compares its bytes) and into a plain Go log of
// ldEntry values that the tests assert on, so these tests do not depend on the trace encoder's exact bytes (the
// encoder and trace tests are another QA agent's, prefix-free of "ld").
//
// Times: int64 ms since the Unix epoch, written in logs as RFC 3339 UTC. ldT0 is 2026-01-01T10:00:00Z.
// Owner D1: an instant is finalized only once the clock is past it — a walk releases an event when Ts < clock and
// fires a timer of any kind when due < clock; at +∞ everything. An event is construct-late iff Ts < clock.

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/table"
)

const (
	ldSec    int64 = 1000
	ldMin          = 60 * ldSec
	ldNegInf int64 = math.MinInt64
	ldPosInf int64 = math.MaxInt64
)

// ldT0 is 2026-01-01T10:00:00Z in ms (a fixed date: no wall clock is read).
var ldT0 = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()

// ldAt is ldT0 + s seconds.
func ldAt(s float64) int64 { return ldT0 + int64(s*1000) }

// ldRFC renders ms as RFC 3339 UTC with milliseconds.
func ldRFC(ms int64) string { return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z") }

func ldStr(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }

func ldAttr(name string, ty event.Type) event.Attr { return event.Attr{Name: name, Type: ty} }

func ldOpt(name string, ty event.Type) event.Attr {
	return event.Attr{Name: name, Type: ty, Optional: true}
}

func ldSchema(t testing.TB, name string, timeAttr int, attrs ...event.Attr) event.Schema {
	t.Helper()
	s, err := event.NewSchema(name, attrs, timeAttr)
	if err != nil {
		t.Fatalf("NewSchema(%s): %v", name, err)
	}
	return s
}

// ldStd is an event-time stream: ts (its time attribute), k (the key), x (optional string), v (optional int).
func ldStd(t testing.TB, name string, delay, idle int64) playback.Stream {
	t.Helper()
	return playback.Stream{Schema: ldSchema(t, name, 0, ldAttr("ts", event.TTimestamp), ldAttr("k", event.TString),
		ldOpt("x", event.TString), ldOpt("v", event.TInt)), Delay: delay, IdleAfter: idle}
}

// ldArrival is a `time arrival` stream: k (the key), x (optional string); its lines carry "ts" (owner D2).
func ldArrival(t testing.TB, name string, delay, idle int64) playback.Stream {
	t.Helper()
	return playback.Stream{Schema: ldSchema(t, name, event.ArrivalTime, ldAttr("k", event.TString),
		ldOpt("x", event.TString)), Delay: delay, IdleAfter: idle}
}

// ldTableT is table T(x string, n optional int) keyed by x.
func ldTableT() playback.Table {
	return playback.Table{Name: "T", Attrs: []event.Attr{ldAttr("x", event.TString), ldOpt("n", event.TInt)}, Key: []string{"x"}}
}

// ldE is an event line on an event-time stream: {"stream":S,"attrs":{"ts":…,"k":…[,"x":…]}}; extra is appended
// to the envelope verbatim (e.g. `,"batch":"b1"`).
func ldE(stream string, ts int64, k, x, extra string) string {
	attrs := fmt.Sprintf(`"ts":%q,"k":%q`, ldRFC(ts), k)
	if x != "" {
		attrs += fmt.Sprintf(`,"x":%q`, x)
	}
	return fmt.Sprintf(`{"stream":%q,"attrs":{%s}%s}`, stream, attrs, extra)
}

// ldEA is an event line on a `time arrival` stream: "ts" carries its time (owner D2).
func ldEA(stream string, ts int64, k, x, extra string) string {
	attrs := fmt.Sprintf(`"k":%q`, k)
	if x != "" {
		attrs += fmt.Sprintf(`,"x":%q`, x)
	}
	return fmt.Sprintf(`{"stream":%q,"ts":%q,"attrs":{%s}%s}`, stream, ldRFC(ts), attrs, extra)
}

// ldLog joins lines with "\n" and a final newline.
func ldLog(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

// ldReader is NewReader over log, failing the test on error.
func ldReader(t testing.TB, app *playback.App, log string) *playback.Reader {
	t.Helper()
	r, err := playback.NewReader(strings.NewReader(log), app)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	return r
}

// ldRecords reads every record up to the first error, returning them and that error (io.EOF at a clean end).
func ldRecords(t testing.TB, app *playback.App, log string) ([]playback.Record, error) {
	t.Helper()
	r := ldReader(t, app, log)
	var recs []playback.Record
	for i := 0; i < 1_000_000; i++ {
		rec, err := r.Next()
		if err != nil {
			return recs, err
		}
		recs = append(recs, rec)
	}
	t.Fatal("Reader.Next never ended")
	return nil, nil
}

// ldLoadErr asserts err is a *playback.LoadError and returns it.
func ldLoadErr(t testing.TB, err error) *playback.LoadError {
	t.Helper()
	var le *playback.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error %v (%T), want a *playback.LoadError (owner D6)", err, err)
	}
	return le
}

// ldEntry is one thing the harness saw, in order. K is a trace kind of design §5.5: "event" (released),
// "construct_late", "timer", "discard", "purge", "note", or "stream_late" (a driver step).
type ldEntry struct {
	At     int
	K      string
	Rule   string
	Key    string
	Stream int
	Seq    uint64
	Ts     int64
	Kind   clock.TimerKind
	Due    int64
	Ref    uint64
	Name   string
	Val    string
}

func (e ldEntry) String() string {
	switch e.K {
	case "event", "construct_late", "stream_late":
		return fmt.Sprintf("@%d %s %s/%s s%d#%d ts=%s", e.At, e.K, e.Rule, e.Key, e.Stream, e.Seq, ldRFC(e.Ts))
	case "timer":
		return fmt.Sprintf("@%d timer %s/%s kind=%d due=%s ref=%d", e.At, e.Rule, e.Key, e.Kind, ldRFC(e.Due), e.Ref)
	case "discard":
		return fmt.Sprintf("@%d discard %s/%s ref=%d", e.At, e.Rule, e.Key, e.Ref)
	case "purge":
		return fmt.Sprintf("@%d purge %s/%s due=%s", e.At, e.Rule, e.Key, ldRFC(e.Due))
	case "note":
		return fmt.Sprintf("@%d note %s/%s %s=%s", e.At, e.Rule, e.Key, e.Name, e.Val)
	}
	return fmt.Sprintf("@%d %s", e.At, e.K)
}

// ldFilter keeps the entries of kind k.
func ldFilter(log []ldEntry, k string) []ldEntry {
	var out []ldEntry
	for _, e := range log {
		if e.K == k {
			out = append(out, e)
		}
	}
	return out
}

// ldRuleSpec is one rule of a test composition.
type ldRuleSpec struct {
	Name   string
	Inputs []int
	Key    string // the key attribute's name in every input stream's schema (string-typed)
	TTL    int64
	// Sched names the timers to schedule for a released event; n counts the key's releases since its birth, from 1.
	Sched func(e event.Event, n uint64) []clock.Timer
	// OnEvent and OnTimers are extra body behaviour (notes), after the harness has recorded the callback.
	OnEvent  func(w *ldWorld, rule string, in *partition.Instance, e event.Event)
	OnTimers func(w *ldWorld, rule string, in *partition.Instance, fires []clock.Timer, tables any)
}

// ldState is a key's body state: it implies exactly the timers it scheduled and has not seen fire (ADR-0006 §2(c)).
type ldState struct {
	timers []clock.Timer
	n      uint64 // releases since birth
	memo   string // free for the spec's hooks
}

func (s *ldState) Walk(fn func(component, key string, state any)) { fn("op", "", len(s.timers)) }

func (s *ldState) Timers(dst []clock.Timer) []clock.Timer { return append(dst, s.timers...) }

func (s *ldState) drop(kind clock.TimerKind, ref uint64) {
	s.timers = slices.DeleteFunc(s.timers, func(x clock.Timer) bool { return x.Kind == kind && x.Ref == ref })
}

// ldBody is the recording partition.Body.
type ldBody struct {
	w    *ldWorld
	spec *ldRuleSpec
}

func (b *ldBody) NewState(*partition.Instance) partition.OpState { return &ldState{} }

func (b *ldBody) OnEvent(in *partition.Instance, e event.Event) {
	w := b.w
	w.tr.Released(w.at, b.spec.Name, in.Key, e)
	w.log = append(w.log, ldEntry{At: w.at, K: "event", Rule: b.spec.Name, Key: in.Key.Str, Stream: e.StreamID, Seq: e.Seq, Ts: e.Ts})
	st := in.Op.(*ldState)
	st.n++
	if b.spec.Sched != nil {
		for _, tm := range b.spec.Sched(e, st.n) {
			in.Clock.Schedule(tm.Due, tm.Kind, tm.Ref)
			st.drop(tm.Kind, tm.Ref)
			st.timers = append(st.timers, tm)
		}
	}
	if b.spec.OnEvent != nil {
		b.spec.OnEvent(w, b.spec.Name, in, e)
	}
}

func (b *ldBody) OnTimers(in *partition.Instance, fires []clock.Timer, tables any) {
	w := b.w
	st := in.Op.(*ldState)
	for _, tm := range fires {
		w.tr.Fired(w.at, b.spec.Name, in.Key, tm)
		w.log = append(w.log, ldEntry{At: w.at, K: "timer", Rule: b.spec.Name, Key: in.Key.Str, Kind: tm.Kind, Due: tm.Due, Ref: tm.Ref})
		st.drop(tm.Kind, tm.Ref)
	}
	if b.spec.OnTimers != nil {
		b.spec.OnTimers(w, b.spec.Name, in, fires, tables)
	}
}

func (b *ldBody) OnDiscard(in *partition.Instance, refs []uint64) {
	w := b.w
	st := in.Op.(*ldState)
	for _, ref := range refs {
		w.tr.Discarded(w.at, b.spec.Name, in.Key, ref)
		w.log = append(w.log, ldEntry{At: w.at, K: "discard", Rule: b.spec.Name, Key: in.Key.Str, Ref: ref})
		st.drop(clock.Discard, ref)
	}
}

func (b *ldBody) OnPurge(in *partition.Instance) {
	w := b.w
	due := in.LastActivity + b.spec.TTL // design §3: OnPurge sees LastActivity unchanged; due = LastActivity + TTL
	if in.LastActivity > ldPosInf-b.spec.TTL {
		due = ldPosInf
	}
	w.tr.Purged(w.at, b.spec.Name, in.Key, due)
	w.log = append(w.log, ldEntry{At: w.at, K: "purge", Rule: b.spec.Name, Key: in.Key.Str, Due: due})
}

// ldRule is one rule in the world.
type ldRule struct {
	spec    *ldRuleSpec
	r       *partition.Rule
	keyAttr []int // per stream id: the key attribute's index, -1 when the rule does not read the stream
}

// ldWorld is one replay's composition: reader, driver, trace and rules. Each test owns its world.
type ldWorld struct {
	t     *testing.T
	app   *playback.App
	d     *playback.Driver
	tr    *playback.Trace
	rules []*ldRule
	at    int
	log   []ldEntry
	steps []playback.Step // copies: Advances cloned (they are valid only until the next call)
}

func ldNewWorld(t *testing.T, app *playback.App, specs ...ldRuleSpec) *ldWorld {
	t.Helper()
	d, err := playback.NewDriver(app)
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}
	w := &ldWorld{t: t, app: app, d: d, tr: playback.NewTrace(app)}
	for i := range specs {
		spec := &specs[i]
		r, err := partition.NewRule(partition.Config{Inputs: spec.Inputs, KeyType: event.TString, TTL: spec.TTL, Body: &ldBody{w: w, spec: spec}})
		if err != nil {
			t.Fatalf("NewRule(%s): %v", spec.Name, err)
		}
		lr := &ldRule{spec: spec, r: r, keyAttr: make([]int, len(app.Streams))}
		for sid := range lr.keyAttr {
			lr.keyAttr[sid] = -1
		}
		for _, sid := range spec.Inputs {
			s := app.Streams[sid].Schema
			for i := 0; i < s.NumAttrs(); i++ {
				if s.Attr(i).Name == spec.Key {
					lr.keyAttr[sid] = i
				}
			}
			if lr.keyAttr[sid] < 0 {
				t.Fatalf("rule %s: stream %s has no key attribute %q", spec.Name, s.Stream(), spec.Key)
			}
		}
		w.rules = append(w.rules, lr)
	}
	return w
}

// step is the router double (design §1).
func (w *ldWorld) step(s *playback.Step) error {
	w.at = s.Line
	cp := *s
	cp.Advances = slices.Clone(s.Advances)
	w.steps = append(w.steps, cp)
	w.tr.Step(s)
	if s.Kind == playback.StepStreamLate {
		w.log = append(w.log, ldEntry{At: s.Line, K: "stream_late", Stream: s.Event.StreamID, Seq: s.Event.Seq, Ts: s.Event.Ts})
	}
	// 1. Apply every advance to every rule reading its stream.
	for _, a := range s.Advances {
		for _, r := range w.rules {
			if r.keyAttr[a.StreamID] >= 0 {
				r.r.Apply(a)
			}
		}
	}
	// 2. Offer the event to every rule reading its stream, under its key.
	if s.Kind == playback.StepEvent {
		for _, r := range w.rules {
			ka := r.keyAttr[s.Event.StreamID]
			if ka < 0 {
				continue
			}
			key := s.Event.Values[ka]
			if r.r.Offer(key, s.Event) {
				w.tr.ConstructLate(s.Line, r.spec.Name, key, s.Event)
				w.log = append(w.log, ldEntry{At: s.Line, K: "construct_late", Rule: r.spec.Name, Key: key.Str,
					Stream: s.Event.StreamID, Seq: s.Event.Seq, Ts: s.Event.Ts})
			}
		}
	}
	// 3. Advance every rule with this step's snapshot (owner D5), then the derivability invariant.
	for _, r := range w.rules {
		r.r.Advance(s.Tables)
		if err := r.r.CheckTimers(); err != nil {
			w.t.Errorf("line %d, rule %s: %v", s.Line, r.spec.Name, err)
		}
	}
	return nil
}

// replay runs log through the world: playback.Replay with the router double.
func (w *ldWorld) replay(log string) error {
	w.t.Helper()
	r, err := playback.NewReader(strings.NewReader(log), w.app)
	if err != nil {
		w.t.Fatalf("NewReader: %v", err)
	}
	return playback.Replay(r, w.d, w.step)
}

// ldRun builds a world, replays log and fails the test on a replay error.
func ldRun(t *testing.T, app *playback.App, log string, specs ...ldRuleSpec) *ldWorld {
	t.Helper()
	w := ldNewWorld(t, app, specs...)
	if err := w.replay(log); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	return w
}

// note records a body observation into the trace and the log.
func (w *ldWorld) note(rule string, in *partition.Instance, name string, v event.Value) {
	w.tr.Note(w.at, rule, in.Key, name, v)
	w.log = append(w.log, ldEntry{At: w.at, K: "note", Rule: rule, Key: in.Key.Str, Name: name, Val: ldVal(v)})
}

// ldVal renders a value for the plain log.
func ldVal(v event.Value) string {
	if v.Null {
		return "null"
	}
	switch v.Kind {
	case event.TString:
		return strconv.Quote(v.Str)
	case event.TInt:
		return strconv.FormatInt(v.I, 10)
	case event.TBool:
		return strconv.FormatBool(v.B)
	}
	return fmt.Sprintf("%+v", v)
}

// ldHasX reads `x in T` (table 0, keyed by x) through a snapshot carried as any: an event's Tables or a timer's
// tables argument (design §2).
func ldHasX(t testing.TB, tables any, x string) bool {
	t.Helper()
	snap, ok := tables.(*table.Snapshot)
	if !ok || snap == nil {
		t.Fatalf("tables %T(%v), want a non-nil *table.Snapshot (design §2: set on every event and every step)", tables, tables)
	}
	return snap.Table(0).Has(ldStr(x))
}

// ldEntries renders a log for a failure message.
func ldEntries(log []ldEntry) string {
	var b strings.Builder
	for _, e := range log {
		b.WriteString("\n\t")
		b.WriteString(e.String())
	}
	return b.String()
}

// ldEOF reports whether err is a clean end.
func ldEOF(err error) bool { return errors.Is(err, io.EOF) }
