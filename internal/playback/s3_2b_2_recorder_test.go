package playback_test

// S3.2b-2's exits 4 and 11 (design note docs/sprints/s3-2b-2-design.md v1, §4, §6, §7, §10 MUST 6, 8, 9, 10, §11):
// `time arrival` stamping in the live intake, and the recorder — a live run recorded through playback.NewRecorder
// and replayed through NewPlanDriver gives the same outputs byte for byte (AppendEvent form) and the same counters.
// Owner, S3.2b-2's plan check: Q1 "a recording keeps a rejected live event as a 'rejected' line ... replay counts it
// and skips it, so replaying a recording gives the live run's alerts and its error counts"; Q2 "when the recording
// cannot be written, detection goes on. The recording stops at the failure; the engine counts the failure". On the
// stub Start refuses any recorder, SendPayload answers "not built" and nothing stamps a live event.

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// rcArrivalSrc: R is a `time arrival` stream (its events carry no time attribute: the engine stamps them, SPEC
// §4.2); Fwd forwards every event, so each output's ts is the stamp its event got.
const rcArrivalSrc = `language cep/0.1;

stream R (k string, x string optional)
  time arrival;

stream O (k string, ts timestamp);

rule Fwd
  intent 'Every arrival, as it arrives.'
  from R
  select k
  insert into O;
`

// rcR is a typed R event whose caller set Ts (the engine must ignore and overwrite it: design §4).
func rcR(k string, ts int64) event.Event {
	return event.Event{StreamID: 0, Ts: ts, Values: []event.Value{ldStr(k), {Kind: event.TString, Null: true}}}
}

// TestArrivalStampsLiveEventsInArrivalOrder is exit 4's first half: live events on a `time arrival` stream, with a
// fake Config.Now, are stamped at ingest in arrival order (design §4: "e.Ts is set to the arrival stamp ... the
// stamp is monotone"), whatever Ts their callers set — here an hour ahead, an hour behind, and zero. All four reach
// the rule (none is late on the stream's ascending watermark), in send order, each output's ts a value Config.Now
// handed out, strictly rising.
func TestArrivalStampsLiveEventsInArrivalOrder(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-arrival.cep", rcArrivalSrc)
	clk := q2NewClock(ldT0, ldSec)
	live := q2Start(t, p, 2, clk.now, nil)
	callers := []event.Event{rcR("a", ldT0+60*ldMin), rcR("b", ldT0-60*ldMin), rcR("c", 0), rcR("d", ldT0+60*ldMin)}
	for _, e := range callers {
		if err := live.r.Send(live.ctx, 0, e); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	res := live.finish()
	handed := clk.handedOut()
	outs := res.outs[q2Stream(t, p, "O")]
	if len(outs) != len(callers) {
		t.Fatalf("%d outputs %v; want %d (every event stamped on arrival; none late) — R late %d", len(outs), outs, len(callers), q2StreamStats(t, res.stats, 0).Late)
	}
	var prev int64
	for i, line := range outs {
		if k := qeAttr(line, "k"); k != callers[i].Values[0].Str {
			t.Errorf("output %d is key %s; want %s (arrival order)", i, k, callers[i].Values[0].Str)
		}
		ts := qeAttr(line, "ts")
		stamp := int64(-1)
		for _, h := range handed {
			if ldRFC(h) == ts {
				stamp = h
			}
		}
		if stamp < 0 {
			t.Errorf("output %d at %s: not a time Config.Now handed out (%v); the caller's Ts was %s", i, ts, handed, ldRFC(callers[i].Ts))
			continue
		}
		if i > 0 && stamp <= prev {
			t.Errorf("output %d stamped %s, not after the previous %s", i, ts, ldRFC(prev))
		}
		prev = stamp
	}
}

// rcRec is a recording kept in memory; its writer can fail at its failAt-th Write (0: never) and fails the test if
// written after that.
type rcRec struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	writes int
	failAt int
	after  int // writes after the failure
}

func (w *rcRec) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	switch {
	case w.failAt > 0 && w.writes == w.failAt:
		return 0, errors.New("rcRec: the disk is full")
	case w.failAt > 0 && w.writes > w.failAt:
		w.after++
		return 0, errors.New("rcRec: written after a failure")
	}
	return w.buf.Write(b)
}

func (w *rcRec) text() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestArrivalRecordingReplaysByteForByte is exit 4's second half: the arrival run recorded through
// playback.NewRecorder and replayed through NewPlanDriver gives the live outputs byte for byte (one shard, one sink:
// one order), a typed Send and a SendPayload alike; a `time arrival` event's line carries its stamp as ts (S3.2a-1's
// D2). The recording's Records count equals its lines.
func TestArrivalRecordingReplaysByteForByte(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-arrival.cep", rcArrivalSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	if err := live.r.Send(live.ctx, 0, rcR("a", ldT0+60*ldMin)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	live.payload(0, `{"k":"b","x":"y"}`)
	if err := live.r.Send(live.ctx, 0, rcR("c", 0)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	res := live.finish()
	log := w.text()
	replay := q2Replay(t, p, log)
	o := q2Stream(t, p, "O")
	if len(res.outs[o]) != 3 || !slices.Equal(res.outs[o], replay.outs[o]) {
		t.Errorf("live outputs %v\nreplayed %v\nwant three, equal byte for byte\nrecording:\n%s", res.outs[o], replay.outs[o], log)
	}
	if len(res.stats.Recording) != 1 {
		t.Fatalf("Stats.Recording %v; want one app's", res.stats.Recording)
	}
	lines := int64(strings.Count(log, "\n"))
	if rs := res.stats.Recording[0]; rs.Failed || rs.Records != lines || lines != 5 {
		t.Errorf("Recording %+v with %d lines; want Records 5 = the header, three events and the stop, not failed\nrecording:\n%s", rs, lines, log)
	}
}

// rcSrc: an event-time stream S read by a keyed, rate-limited rule that can divide by zero (Keyed) and a key-less
// rule that sees every accepted event and projects its flag (All), with a table the keyed rule reads. S declares
// text_invisibles (valid before and after Q3), so these tests are red on the recorder, not on the validator.
const rcSrc = `language cep/0.1;

stream S (k string optional, n int, d int, note string optional, text_invisibles bool, ts timestamp)
  time ts watermark bounded 1 min;

table Block (k string) primary key (k);

stream OK (k string, n int, ts timestamp);
stream OA (k string optional, flag bool, ts timestamp);

rule Keyed
  intent 'n over d per key, at most once a minute, unless blocked.'
  partition by k of S ttl 5 min
  from S[n / d > 0 and k not in Block]
  select k, n
  output first every 1 min
  insert into OK;

rule All
  intent 'Every accepted event, with its invisible-character flag.'
  from S
  select k, text_invisibles as flag
  insert into OA;
`

// rcS is an S payload at T0 + at with the given members appended.
func rcS(at int64, k string, n, d int, extra string) string {
	js := fmt.Sprintf(`{"n":%d,"d":%d,"ts":%q`, n, d, ldRFC(ldT0+at))
	if k != "" {
		js += fmt.Sprintf(`,"k":%q`, k)
	}
	return js + extra + "}"
}

// rcScript feeds one live run: good payloads, a rejected payload (missing n), an unknown key, a payload's own
// text_invisibles over a zero-width space, a stream-late event, a null key, a division by zero, a suppression, a
// table put and a typed Send.
func rcScript(t *testing.T, live *q2Live) {
	t.Helper()
	s := 0
	live.payload(s, rcS(0, "a", 4, 2, ""))
	live.payload(s, fmt.Sprintf(`{"k":"a","d":2,"ts":%q}`, ldRFC(ldT0+ldSec))) // rejected: n missing (Q1)
	live.payload(s, rcS(2*ldSec, "b", 4, 2, `,"extra":1`))                     // unknown key: counted, goes on
	live.payload(s, rcS(3*ldSec, "c", 4, 2, `,"note":"a`+qeZWSP+`b","text_invisibles":false`))
	live.payload(s, rcS(30*ldSec, "a", 4, 2, "")) // suppressed (a's anchor 10:00, 1 min)
	live.payload(s, rcS(5*ldMin, "d", 4, 2, ""))
	live.payload(s, rcS(ldSec, "e", 4, 2, ""))        // stream-late: 10:00:01 behind 10:05 − 1 min
	live.payload(s, rcS(5*ldMin+ldSec, "", 4, 2, "")) // a null key: Keyed's key error
	live.payload(s, rcS(5*ldMin+2*ldSec, "f", 4, 0, ""))
	if err := live.r.TableWrite(live.ctx, 0, 0, runtime.TablePut, [][]event.Value{{ldStr("g")}}); err != nil {
		t.Fatalf("TableWrite: %v", err)
	}
	live.payload(s, rcS(5*ldMin+3*ldSec, "g", 4, 2, "")) // blocked
	e := event.Event{StreamID: 0, Ts: ldT0 + 5*ldMin + 4*ldSec, Values: []event.Value{ldStr("h"), {Kind: event.TInt, I: 4},
		{Kind: event.TInt, I: 2}, {Kind: event.TString, Null: true}, {Kind: event.TBool}, {Kind: event.TTimestamp, T: ldT0 + 5*ldMin + 4*ldSec}}}
	if err := live.r.Send(live.ctx, 0, e); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// rcCounters compares two runs' StreamStats (AfterDrain excepted: live only) and RuleStats.
func rcCounters(t *testing.T, live, replay *q2Run, nStreams, nRules int) {
	t.Helper()
	for s := range nStreams {
		a, b := q2StreamStats(t, live.stats, s), q2StreamStats(t, replay.stats, s)
		a.AfterDrain, b.AfterDrain = 0, 0
		if a != b {
			t.Errorf("stream %d: live %+v vs replay %+v (MUST 10)", s, a, b)
		}
	}
	for r := range nRules {
		a, b := q2RuleStats(t, live.stats, r), q2RuleStats(t, replay.stats, r)
		if a != b {
			t.Errorf("rule %d: live %+v vs replay %+v (MUST 10)", r, a, b)
		}
	}
}

// TestRecorderReplayGivesTheLiveRun is exit 11's first half (owner Q1; design §7's property, MUST 10): a live run
// with a rejected payload, an unknown key and a payload's own text_invisibles — and a stream-late event, a null key,
// a division by zero, a suppression, a table write and a typed Send — recorded and replayed gives the same outputs
// in the same per-key order (one shard: byte for byte) and the same StreamStats (but AfterDrain) and RuleStats. The
// live counters are pinned too, so equal counters are not two zeros.
func TestRecorderReplayGivesTheLiveRun(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-recorder.cep", rcSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	rcScript(t, live)
	res := live.finish()
	log := w.text()
	if !strings.Contains(log, `"rejected"`) {
		t.Errorf("the recording has no rejected line (owner Q1):\n%s", log)
	}
	replay := q2Replay(t, p, log)
	for _, name := range []string{"OK", "OA"} {
		o := q2Stream(t, p, name)
		if !slices.Equal(res.outs[o], replay.outs[o]) {
			t.Errorf("%s: live %v\nreplayed %v\nrecording:\n%s", name, res.outs[o], replay.outs[o], log)
		}
	}
	rcCounters(t, res, replay, len(p.Streams), len(p.Rules))
	ss := q2StreamStats(t, res.stats, 0)
	if want := (runtime.StreamStats{Late: 1, Rejected: 1, Unknown: 1, FlagOverrides: 1}); ss != want {
		t.Errorf("live S %+v; want %+v", ss, want)
	}
	k := q2RuleStats(t, res.stats, q2Rule(t, p, "Keyed"))
	if k.KeyErrors != 1 || k.Errored != 1 || k.Suppressed != 1 {
		t.Errorf("live Keyed %+v; want KeyErrors 1, Errored 1, Suppressed 1", k)
	}
	if n := len(res.outs[q2Stream(t, p, "OA")]); n != 9 {
		t.Errorf("All gave %d outputs; want 9 (every accepted event: 11 sent, 1 rejected, 1 stream-late)", n)
	}
}

// TestRecorderFailureLeavesDetectionRunning is exit 11's second half (owner Q2; MUST 9): the writer fails at its
// N-th Write (one line per record, design §7); the app goes on — its outputs and counters equal a run without a
// recorder —, Stats.Recording says Failed with the error's text and Records = N − 1, and the writer is never called
// again (not even by Stop). N runs over the header, an early event, the rejected line and a late record.
func TestRecorderFailureLeavesDetectionRunning(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-recorder.cep", rcSrc)
	plain := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, nil)
	rcScript(t, plain)
	want := plain.finish()
	for _, n := range []int{1, 2, 3, 9} {
		w := &rcRec{failAt: n}
		live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
		rcScript(t, live)
		res := live.finish()
		for _, name := range []string{"OK", "OA"} {
			o := q2Stream(t, p, name)
			if !slices.Equal(res.outs[o], want.outs[o]) {
				t.Errorf("fail at %d: %s: %v; want the unrecorded run's %v (detection goes on, owner Q2)", n, name, res.outs[o], want.outs[o])
			}
		}
		rcCounters(t, res, want, len(p.Streams), len(p.Rules))
		if len(res.stats.Recording) != 1 {
			t.Fatalf("fail at %d: Stats.Recording %v; want one app's", n, res.stats.Recording)
		}
		rs := res.stats.Recording[0]
		if !rs.Failed || rs.Records != int64(n-1) || !strings.Contains(rs.Err, "disk is full") {
			t.Errorf("fail at %d: Recording %+v; want Failed, Records %d, Err naming the writer's error", n, rs, n-1)
		}
		w.mu.Lock()
		writes, after := w.writes, w.after
		w.mu.Unlock()
		if writes != n || after != 0 {
			t.Errorf("fail at %d: the writer was called %d times (%d after the failure); want exactly %d (MUST 9)", n, writes, after, n)
		}
	}
}

// rcCall is one recorder call as the double saw it.
type rcCall struct {
	kind    string
	stream  int
	arrival int64
	reason  event.DecodeReason
	payload string
}

// rcDouble is a runtime.Recorder that logs its calls and fails the failAt-th (0: never).
type rcDouble struct {
	mu     sync.Mutex
	calls  []rcCall
	failAt int
}

func (d *rcDouble) add(c rcCall) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, c)
	if d.failAt > 0 && len(d.calls) == d.failAt {
		return errors.New("rcDouble: refused")
	}
	return nil
}

func (d *rcDouble) Header(start int64) error { return d.add(rcCall{kind: "header", arrival: start}) }
func (d *rcDouble) Event(e *event.Event, arrival int64, payload []byte) error {
	return d.add(rcCall{kind: "event", stream: e.StreamID, arrival: arrival, payload: string(payload)})
}
func (d *rcDouble) Rejected(stream int, arrival int64, fault *event.DecodeError) error {
	c := rcCall{kind: "rejected", stream: stream, arrival: arrival}
	if fault != nil {
		c.reason = fault.Reason
	}
	return d.add(c)
}
func (d *rcDouble) Tick(now int64) error { return d.add(rcCall{kind: "tick", arrival: now}) }
func (d *rcDouble) Table(table int, op runtime.TableOp, rows [][]event.Value) error {
	return d.add(rcCall{kind: "table", stream: table})
}
func (d *rcDouble) Stop() error { return d.add(rcCall{kind: "stop"}) }
func (d *rcDouble) BatchEnd(stream int, batch string) error {
	return d.add(rcCall{kind: "batch_end", stream: stream})
}

// TestRecorderCallOrder: design §7's order and MUST 8, 9, through a recorder double — the header first; one call per
// intake item, in intake order, each with the arrival stamp Config.Now handed out (a rejected payload raises the
// stamp like an arrival); a typed Send's payload is nil, a SendPayload's is its bytes; the rejected payload's
// reason is "missing"; a table write after its success; the stop last, once. A double failing its 3rd call is never
// called again and the app's outputs are unchanged.
func TestRecorderCallOrder(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-recorder.cep", rcSrc)
	d := &rcDouble{}
	clk := q2NewClock(ldT0, ldSec)
	live := q2Start(t, p, 2, clk.now, d)
	good := rcS(0, "a", 4, 2, "")
	live.payload(0, good)
	live.payload(0, fmt.Sprintf(`{"k":"a","d":2,"ts":%q}`, ldRFC(ldT0+ldSec)))
	if err := live.r.TableWrite(live.ctx, 0, 0, runtime.TablePut, [][]event.Value{{ldStr("g")}}); err != nil {
		t.Fatalf("TableWrite: %v", err)
	}
	if err := live.r.Tick(live.ctx, ldT0+10*ldSec); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	e := event.Event{StreamID: 0, Ts: ldT0 + 2*ldSec, Values: []event.Value{ldStr("h"), {Kind: event.TInt, I: 4},
		{Kind: event.TInt, I: 2}, {Kind: event.TString, Null: true}, {Kind: event.TBool}, {Kind: event.TTimestamp, T: ldT0 + 2*ldSec}}}
	if err := live.r.Send(live.ctx, 0, e); err != nil {
		t.Fatalf("Send: %v", err)
	}
	live.finish()
	d.mu.Lock()
	calls := slices.Clone(d.calls)
	d.mu.Unlock()
	var kinds []string
	for _, c := range calls {
		kinds = append(kinds, c.kind)
	}
	if want := []string{"header", "event", "rejected", "table", "tick", "event", "stop"}; !slices.Equal(kinds, want) {
		t.Fatalf("recorder calls %v; want %v", kinds, want)
	}
	h := clk.handedOut()
	if len(h) < 3 || calls[0].arrival != h[0] || calls[1].arrival != h[1] || calls[2].arrival != h[2] {
		t.Errorf("header/event/rejected stamps %d, %d, %d; want Config.Now's first three %v", calls[0].arrival, calls[1].arrival, calls[2].arrival, h)
	}
	if calls[1].payload != good || calls[5].payload != "" {
		t.Errorf("payloads %q, %q; want SendPayload's bytes, then nil for the typed Send", calls[1].payload, calls[5].payload)
	}
	if calls[2].reason != event.DecodeMissing || calls[2].stream != 0 {
		t.Errorf("rejected call %+v; want stream 0, reason missing", calls[2])
	}
	if calls[4].arrival != ldT0+10*ldSec {
		t.Errorf("tick at %d; want %d", calls[4].arrival, ldT0+10*ldSec)
	}

	f := &rcDouble{failAt: 3}
	live2 := q2Start(t, p, 2, q2NewClock(ldT0, ldSec).now, f)
	for i := range 6 {
		live2.payload(0, rcS(int64(i)*2*ldMin, "a", 4, 2, ""))
	}
	res := live2.finish()
	f.mu.Lock()
	n := len(f.calls)
	f.mu.Unlock()
	if n != 3 {
		t.Errorf("a recorder that failed its 3rd call was called %d times; want 3 (MUST 9)", n)
	}
	if got := len(res.outs[q2Stream(t, p, "OK")]); got != 6 {
		t.Errorf("Keyed gave %d outputs after the recorder failed; want 6 (detection goes on)", got)
	}
	if len(res.stats.Recording) != 1 || !res.stats.Recording[0].Failed || res.stats.Recording[0].Records != 2 {
		t.Errorf("Recording %v; want Failed, Records 2", res.stats.Recording)
	}
}
