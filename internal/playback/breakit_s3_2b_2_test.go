package playback_test

// S3.2b-2's QA break-it (design note docs/sprints/s3-2b-2-design.md v2): inputs where the build does the wrong
// thing, each a red test naming the defect, or a guard where a suspected break held. Prefix: bk2.

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
	"github.com/vorsar/cep/internal/runtime"
)

// bk2InvisKeySrc keys a rate-limited rule on a part that is the stream's declared text_invisibles slot. Every read of
// the name is the engine's flag (S3.2b's Q3; design §5: "a caller's Invisibles is overwritten"), but the router
// builds a key from e.Values[slot] directly, which is the caller's value on a typed Send and always false on a
// payload or a replayed line (the decoder fills the reserved slot with false; the typed line leaves it out, F3).
const bk2InvisKeySrc = `language cep/0.1;

stream S (k string, text_invisibles bool, ts timestamp)
  time ts watermark bounded 1 min;

stream O (p1 string, p2 bool, ts timestamp);

rule R
  intent 'The first event per key and flag every ten minutes.'
  partition by (k, text_invisibles) of S ttl 30 min
  from S
  select k as p1, text_invisibles as p2
  output first every 10 min
  insert into O;
`

// TestBreakitInvisiblesSlotAsKeyPartReplays: two typed Sends of key "a", the caller's text_invisibles slot true then
// false, both over clean text (the flag is false for both). Live and replay MUST agree (MUST 10), and both events
// share one instance: the key part named text_invisibles is the flag, false for both, so the second is suppressed.
func TestBreakitInvisiblesSlotAsKeyPartReplays(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-invis-key.cep", bk2InvisKeySrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	for i, slot := range []bool{true, false} {
		ts := ldT0 + int64(i)*ldSec
		e := event.Event{StreamID: 0, Ts: ts, Values: []event.Value{ldStr("a"), {Kind: event.TBool, B: slot}, {Kind: event.TTimestamp, T: ts}}}
		if err := live.r.Send(live.ctx, 0, e); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	res := live.finish()
	log := w.text()
	replay := q2Replay(t, p, log)
	o := q2Stream(t, p, "O")
	if !slices.Equal(res.outs[o], replay.outs[o]) {
		t.Errorf("live ≠ replay for a key part on the declared text_invisibles slot (MUST 10): the router keys on the caller's slot value, the recording drops it\nlive   %v\nreplay %v\nrecording:\n%s", res.outs[o], replay.outs[o], log)
	}
	if len(res.outs[o]) != 1 {
		t.Errorf("live gave %d outputs %v; want 1: the key part text_invisibles is the engine's flag (false for both), not the caller's slot", len(res.outs[o]), res.outs[o])
	}
}

// bk2FwdSrc forwards every S event, with a table the recording records.
const bk2FwdSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

table Block (k string) primary key (k);

stream O (k string, ts timestamp);

rule Fwd
  intent 'Every event not blocked.'
  from S[k not in Block]
  select k
  insert into O;
`

// TestBreakitMalformedUTF8PayloadStaysRecordable: a payload well under SendPayload's length limit whose string holds
// 6 MiB of malformed UTF-8 bytes. CleanObject replaces each byte by U+FFFD (three bytes), so the recorded line is
// about 18 MiB, past the reader's 16 MiB limit: the recorder refuses the line and the recording ends. Design §6
// (review F4, MUST 14): "every payload the intake accepts is one the reader can read back" — the payload MUST be
// rejected (syntax) or recorded so that it replays; it may not end the recording.
func TestBreakitMalformedUTF8PayloadStaysRecordable(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-fwd.cep", bk2FwdSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	bad := strings.Repeat("\xff", 6<<20)
	payload := fmt.Sprintf(`{"k":"%s","ts":%q}`, bad, ldRFC(ldT0))
	live.payload(0, payload)
	live.payload(0, fmt.Sprintf(`{"k":"b","ts":%q}`, ldRFC(ldT0+ldSec)))
	res := live.finish()
	rs := res.stats.Recording[0]
	ss := q2StreamStats(t, res.stats, 0)
	if rs.Failed {
		t.Fatalf("a %d-byte payload SendPayload accepted (Rejected %d) ended the recording: %q; MUST 14 / review F4: an accepted payload is one the reader can read back", len(payload), ss.Rejected, rs.Err)
	}
	replay := q2Replay(t, p, w.text())
	o := q2Stream(t, p, "O")
	if len(res.outs[o]) != len(replay.outs[o]) {
		t.Errorf("live %d outputs, replay %d (MUST 10)", len(res.outs[o]), len(replay.outs[o]))
	}
}

// TestBreakitTableWriteBeyondTheLineIsRefused: TableWrite accepts a row whose string is longer than a recording line
// holds; the recorder refuses the table line and the recording ends. Send refuses such an event (lineBound; design
// §16.7), so the Go boundary (owner D1, MUST 14: "Send and TableWrite MUST refuse every value a recording cannot copy
// exactly") is uneven: TableWrite MUST refuse the row, or the recording MUST hold it.
func TestBreakitTableWriteBeyondTheLineIsRefused(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-fwd.cep", bk2FwdSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	big := strings.Repeat("x", event.MaxLogLine)
	err := live.r.TableWrite(live.ctx, 0, 0, runtime.TablePut, [][]event.Value{{ldStr(big)}})
	live.payload(0, fmt.Sprintf(`{"k":"b","ts":%q}`, ldRFC(ldT0)))
	res := live.finish()
	if rs := res.stats.Recording[0]; err == nil && rs.Failed {
		t.Errorf("TableWrite accepted a %d-byte string (err nil) that the recording cannot hold: Recording %+v (owner D1, MUST 14)", len(big), rs)
	}
}

// TestBreakitRecordsCountsEveryLine (QA G5; design §7: "Records int64: the records written — every line, the header
// and the stop included"): a table write of two rows is two table lines (one per row, rev 2's table records), and a
// write of no rows is none; Records counts one per Recorder call instead, so it disagrees with the recording. (Three
// rows and an empty write: two rows would cancel the empty write's extra count.)
func TestBreakitRecordsCountsEveryLine(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-fwd.cep", bk2FwdSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	if err := live.r.TableWrite(live.ctx, 0, 0, runtime.TablePut, [][]event.Value{{ldStr("x")}, {ldStr("y")}, {ldStr("z")}}); err != nil {
		t.Fatalf("TableWrite: %v", err)
	}
	if err := live.r.TableWrite(live.ctx, 0, 0, runtime.TableDelete, nil); err != nil {
		t.Fatalf("TableWrite (no rows): %v", err)
	}
	live.payload(0, fmt.Sprintf(`{"k":"b","ts":%q}`, ldRFC(ldT0)))
	res := live.finish()
	log := w.text()
	lines := int64(strings.Count(log, "\n"))
	if rs := res.stats.Recording[0]; rs.Records != lines {
		t.Errorf("Recording.Records %d, but the recording holds %d lines (QA G5: every line):\n%s", rs.Records, lines, log)
	}
}

// TestBreakitSendBatchIDBeyondTheLine: Send's line bound (design §16.7: "Send also refuses an event whose recording
// line could exceed the log-line limit") leaves out e.BatchID, which the typed line writes as "batch": a typed Send
// with a BatchID longer than a line is accepted and ends the recording (owner D1, MUST 14).
func TestBreakitSendBatchIDBeyondTheLine(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-fwd.cep", bk2FwdSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	e := event.Event{StreamID: 0, Ts: ldT0, BatchID: strings.Repeat("b", event.MaxLogLine), Values: []event.Value{ldStr("a"), {Kind: event.TTimestamp, T: ldT0}}}
	err := live.r.Send(live.ctx, 0, e)
	res := live.finish()
	if rs := res.stats.Recording[0]; err == nil && rs.Failed {
		t.Errorf("Send accepted an event (err nil) whose BatchID the recording cannot hold: Recording %+v (owner D1, MUST 14)", rs)
	}
}

// bk2ObjSrc: S carries an optional object; Fwd forwards it, so a replayed object is compared byte for byte.
const bk2ObjSrc = `language cep/0.1;

stream S (k string, o object optional, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, o object optional, ts timestamp);

rule Fwd
  intent 'Every event with its object.'
  from S
  select k, o
  insert into O;
`

// TestBreakitDepthLimitsRoundTrip (guard; design §6, review F4, owner D1): the deepest payload SendPayload accepts
// (9,999 levels, here inside an undeclared member and inside the object attribute) and the deepest object value Send
// accepts are recorded and replayed to the live run's outputs and counters; one level more is refused by Send.
func TestBreakitDepthLimitsRoundTrip(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-obj.cep", bk2ObjSrc)
	w := &rcRec{}
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, playback.NewRecorder(w, p))
	nest := func(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }
	live.payload(0, fmt.Sprintf(`{"k":"u","x":%s,"ts":%q}`, nest(event.MaxDepth-2), ldRFC(ldT0)))
	live.payload(0, fmt.Sprintf(`{"k":"o","o":%s,"ts":%q}`, nest(event.MaxDepth-2), ldRFC(ldT0+ldSec)))
	typed := func(n int, at int64) event.Event {
		return event.Event{StreamID: 0, Ts: at, Values: []event.Value{ldStr("t"), {Kind: event.TObject, Obj: nest(n)}, {Kind: event.TTimestamp, T: at}}}
	}
	if err := live.r.Send(live.ctx, 0, typed(event.MaxDepth-2, ldT0+2*ldSec)); err != nil {
		t.Errorf("Send of an object %d deep (its line %d deep): %v; want accepted", event.MaxDepth-2, event.MaxDepth, err)
	}
	if err := live.r.Send(live.ctx, 0, typed(event.MaxDepth-1, ldT0+3*ldSec)); err == nil {
		t.Errorf("Send of an object %d deep (its line %d deep): accepted; want refused", event.MaxDepth-1, event.MaxDepth+1)
	}
	res := live.finish()
	if rs := res.stats.Recording[0]; rs.Failed {
		t.Fatalf("the recording failed: %s", rs.Err)
	}
	replay := q2Replay(t, p, w.text())
	o := q2Stream(t, p, "O")
	if len(res.outs[o]) != 3 || !slices.Equal(res.outs[o], replay.outs[o]) {
		t.Errorf("live %d outputs, replay %d; want 3, equal byte for byte", len(res.outs[o]), len(replay.outs[o]))
	}
	rcCounters(t, res, replay, len(p.Streams), len(p.Rules))
	if ss := q2StreamStats(t, res.stats, 0); ss.Unknown != 1 || ss.Rejected != 0 {
		t.Errorf("live S %+v; want Unknown 1, Rejected 0", ss)
	}
}

// bk2KeyedSrc: a two-part keyed, rate-limited rule and a key-less forwarder on one stream, so per-key order and
// suppression both depend on intake order.
const bk2KeyedSrc = `language cep/0.1;

stream S (a string, b int, note string optional, ts timestamp)
  time ts watermark bounded 1 hour;

stream O (p1 string, p2 int, ts timestamp);
stream F (p1 string, p2 int, note string optional, ts timestamp);

rule Keyed
  intent 'The first event per (a, b) every ten seconds.'
  partition by (a, b) of S ttl 1 hour
  from S
  select a as p1, b as p2
  output first every 10 sec
  insert into O;

rule All
  intent 'Every accepted event.'
  from S
  select a as p1, b as p2, note
  insert into F;
`

// TestBreakitConcurrentPayloadsReplay (guard; MUST 10, design §6 "event.Decoder is safe for concurrent use"): eight
// goroutines SendPayload at once on four shards — good payloads, rejected ones, unknown keys, payload
// text_invisibles, invisible characters — and the recording, which is the intake's order, replays to the live run's
// per-key outputs and exact counters. Run with -race.
func TestBreakitConcurrentPayloadsReplay(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "bk2-keyed.cep", bk2KeyedSrc)
	w := &rcRec{}
	live := q2Start(t, p, 4, q2NewClock(ldT0, 7).now, playback.NewRecorder(w, p))
	const workers, each = 8, 150
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for g := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				ts := ldRFC(ldT0 + int64(i)*ldSec)
				var js string
				switch i % 5 {
				case 0:
					js = fmt.Sprintf(`{"a":"g%d","b":%d,"ts":%q}`, g%3, i%4, ts)
				case 1:
					js = fmt.Sprintf(`{"a":"g%d","ts":%q}`, g%3, ts) // rejected: b missing
				case 2:
					js = fmt.Sprintf(`{"a":"g%d","b":%d,"zz":[1,{"q":2}],"ts":%q}`, g%3, i%4, ts)
				case 3:
					js = fmt.Sprintf(`{"a":"g%d","b":%d,"note":"x%sy","text_invisibles":true,"ts":%q}`, g%3, i%4, qeZWSP, ts)
				default:
					js = fmt.Sprintf(`{ "a" : "g%d" , "b" : %d , "ts" : %q }`, g%3, i%4, ts)
				}
				buf := []byte(js)
				if err := live.r.SendPayload(live.ctx, 0, 0, buf); err != nil {
					errs <- err
					return
				}
				for j := range buf { // the caller may reuse payload once SendPayload returns (design §6)
					buf[j] = 'X'
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("SendPayload: %v", err)
	}
	res := live.finish()
	if rs := res.stats.Recording[0]; rs.Failed {
		t.Fatalf("the recording failed: %s", rs.Err)
	}
	replay := q2Replay(t, p, w.text())
	if d := q2Diff(t, p, res, replay, 2); d != "" {
		t.Errorf("live ≠ replay per key (MUST 10): %s", d)
	}
	rcCounters(t, res, replay, len(p.Streams), len(p.Rules))
	want := runtime.StreamStats{Rejected: workers * each / 5, Unknown: workers * each / 5, FlagOverrides: workers * each / 5}
	if ss := q2StreamStats(t, res.stats, 0); ss != want {
		t.Errorf("live S %+v; want %+v", ss, want)
	}
}
