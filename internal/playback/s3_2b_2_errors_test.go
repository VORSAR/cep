package playback_test

// S3.2b-2's exits 2 and 3 (design note docs/sprints/s3-2b-2-design.md v1, §5, §6 and §11; SEMANTICS §10; S3.2b's
// plan check Q3), live: the program's text through the real runtime, its payloads through SendPayload. Written
// against the stub, where SendPayload answers "not built", Stats.Streams holds only Late and AfterDrain, and the
// invisible-character flag is computed nowhere. (Design §11 places exit 2 in runtime; runtime's tests may not read
// program text — tools/importcheck/rules.json — so the table lives here, over the same runtime.)

import (
	"fmt"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/runtime"
)

// qeErrSrc: one source stream S read by three rules. Divide is keyed by the optional k and divides by d; Cast
// casts the text tag to an int; Plain is key-less and fires on every accepted event — the witness that an event a
// rule drops still reaches the others (SEMANTICS §10: "one rule's bug can never censor an event for another rule").
const qeErrSrc = `language cep/0.1;

stream S (k string optional, n int, d int, tag string, ts timestamp)
  time ts watermark bounded 1 min;

stream O1 (k string, q int, ts timestamp);
stream O2 (tag string, ts timestamp);
stream O3 (k string optional, ts timestamp);

rule Divide
  intent 'n over d, per key.'
  partition by k of S ttl 1 hour
  from S[n / d > 0]
  select k, n / d as q
  insert into O1;

rule Cast
  intent 'The tag read as a number.'
  from S[cast(tag, 'int') > 0]
  select tag
  insert into O2;

rule Plain
  intent 'Every accepted event.'
  from S
  select k
  insert into O3;
`

// qeS is an S payload at T0 + at; fields overrides or adds members verbatim (a member written twice keeps its
// last value, P4): qeS(0, `"d":0`) divides by zero.
func qeS(at int64, fields ...string) string {
	base := fmt.Sprintf(`{"k":"a","n":6,"d":2,"tag":"7","ts":%q`, ldRFC(ldT0+at))
	for _, f := range fields {
		base += "," + f
	}
	return base + "}"
}

// TestErrorSemanticsTable is exit 2's proof: one row per line of design §6's error-semantics table, each in its own
// runtime, through SendPayload (and Send for the last line). Each row names exactly the counters it moves —
// StreamStats of S and Divide's, Cast's and Plain's RuleStats — and how many outputs each rule gives; every other
// counter stays zero.
func TestErrorSemanticsTable(t *testing.T) {
	t.Parallel()
	type ruleWant struct{ KeyErrors, Errored int64 }
	rows := []struct {
		name     string
		payloads []string
		send     *event.Event // a typed Send after the payloads; nil: none
		sendErr  bool         // the Send must be refused (an error to the caller)
		stream   struct{ Late, Rejected, Unknown, FlagOverrides int64 }
		rules    [3]ruleWant // Divide, Cast, Plain
		outs     [3]int      // O1 (Divide), O2 (Cast), O3 (Plain)
	}{
		// Line 1 of the table: rejected payloads — StreamStats.Rejected, dropped for every rule.
		{name: "not JSON", payloads: []string{`this is not json`},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "not one JSON object", payloads: []string{`[1,2,3]`},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "a value of the wrong type", payloads: []string{qeS(0, `"n":"six"`)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "a non-optional attribute missing", payloads: []string{fmt.Sprintf(`{"k":"a","d":2,"tag":"7","ts":%q}`, ldRFC(ldT0))},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "a non-optional attribute null", payloads: []string{qeS(0, `"n":null`)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "the time attribute missing", payloads: []string{`{"k":"a","n":6,"d":2,"tag":"7"}`},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "the time attribute null", payloads: []string{qeS(0, `"ts":null`)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}},
		{name: "a rejected payload between two good ones", payloads: []string{qeS(0), `{"k":`, qeS(ldSec)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Rejected: 1}, outs: [3]int{2, 2, 2}},
		// Line 2: an undeclared key — counted per occurrence, the event goes on.
		{name: "keys the schema does not declare", payloads: []string{qeS(0, `"extra":1`, `"more":"x"`), qeS(ldSec, `"extra":true`)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Unknown: 3}, outs: [3]int{2, 2, 2}},
		// Line 3: a payload's own text_invisibles — counted, whatever its value; the event goes on.
		{name: "a payload's own text_invisibles", payloads: []string{qeS(0, `"text_invisibles":false`), qeS(ldSec, `"text_invisibles":true`)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{FlagOverrides: 2}, outs: [3]int{2, 2, 2}},
		// Line 4: stream-late — StreamStats.Late, dropped for every rule (10:10, then 10:00 behind the 1 min bound).
		{name: "stream-late", payloads: []string{qeS(10 * ldMin), qeS(0)},
			stream: struct{ Late, Rejected, Unknown, FlagOverrides int64 }{Late: 1}, outs: [3]int{1, 1, 1}},
		// Line 5: a null key part — Divide's KeyErrors, dropped for Divide only.
		{name: "a null key", payloads: []string{qeS(0, `"k":null`)},
			rules: [3]ruleWant{{KeyErrors: 1}, {}, {}}, outs: [3]int{0, 1, 1}},
		{name: "an absent optional key", payloads: []string{fmt.Sprintf(`{"n":6,"d":2,"tag":"7","ts":%q}`, ldRFC(ldT0))},
			rules: [3]ruleWant{{KeyErrors: 1}, {}, {}}, outs: [3]int{0, 1, 1}},
		// Line 6: expression errors — the rule's Errored, dropped for that rule only.
		{name: "division by zero", payloads: []string{qeS(0, `"d":0`)},
			rules: [3]ruleWant{{Errored: 1}, {}, {}}, outs: [3]int{0, 1, 1}},
		{name: "a failed cast", payloads: []string{qeS(0, `"tag":"seven"`)},
			rules: [3]ruleWant{{}, {Errored: 1}, {}}, outs: [3]int{1, 0, 1}},
		// Line 7: a malformed typed Send — an error to the caller, nothing counted, nothing emitted.
		{name: "a typed Send with too few values", send: &event.Event{StreamID: 0, Ts: ldT0, Values: []event.Value{ldStr("a")}}, sendErr: true},
		{name: "a typed Send with a null non-optional value", sendErr: true, send: &event.Event{StreamID: 0, Ts: ldT0, Values: []event.Value{
			ldStr("a"), {Kind: event.TInt, Null: true}, {Kind: event.TInt, I: 2}, ldStr("7"), {Kind: event.TTimestamp, T: ldT0}}}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			p := pbBuild(t, "qa-errors.cep", qeErrSrc)
			s := q2Stream(t, p, "S")
			live := q2Start(t, p, 2, q2NewClock(ldT0, ldSec).now, nil)
			for _, js := range row.payloads {
				live.payload(s, js)
			}
			if row.send != nil {
				err := live.r.Send(live.ctx, 0, *row.send)
				if row.sendErr != (err != nil) {
					t.Fatalf("Send(%v): error %v; want an error: %v (S3.1-2's D2)", row.send, err, row.sendErr)
				}
			}
			res := live.finish()
			ss := q2StreamStats(t, res.stats, s)
			got := struct{ Late, Rejected, Unknown, FlagOverrides int64 }{ss.Late, ss.Rejected, ss.Unknown, ss.FlagOverrides}
			if got != row.stream {
				t.Errorf("S's StreamStats %+v; want %+v", ss, row.stream)
			}
			for i, name := range []string{"Divide", "Cast", "Plain"} {
				rs := q2RuleStats(t, res.stats, q2Rule(t, p, name))
				if (ruleWant{rs.KeyErrors, rs.Errored}) != row.rules[i] {
					t.Errorf("%s's RuleStats %+v; want KeyErrors %d, Errored %d", name, rs, row.rules[i].KeyErrors, row.rules[i].Errored)
				}
			}
			for i, name := range []string{"O1", "O2", "O3"} {
				if n := len(res.outs[q2Stream(t, p, name)]); n != row.outs[i] {
					t.Errorf("%s: %d outputs %v; want %d", name, n, res.outs[q2Stream(t, p, name)], row.outs[i])
				}
			}
		})
	}
}

// TestSendPayloadRefusesAWrongTarget: design §6 — a wrong app, a derived stream or an unknown stream id is an error
// to the caller, as Send's is; a well-formed payload on a source stream is not.
func TestSendPayloadRefusesAWrongTarget(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-errors.cep", qeErrSrc)
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, nil)
	if err := live.r.SendPayload(live.ctx, 0, q2Stream(t, p, "S"), []byte(qeS(0))); err != nil {
		t.Fatalf("SendPayload on S: %v; want nil", err)
	}
	for _, c := range []struct {
		name   string
		app    int
		stream int
	}{
		{"a wrong app", 1, q2Stream(t, p, "S")},
		{"a derived stream", 0, q2Stream(t, p, "O1")},
		{"an unknown stream id", 0, len(p.Streams)},
		{"a negative stream id", 0, -1},
	} {
		if err := live.r.SendPayload(live.ctx, runtime.AppID(c.app), c.stream, []byte(qeS(0))); err == nil {
			t.Errorf("%s: SendPayload returned nil; want an error to the caller", c.name)
		}
	}
	res := live.finish()
	if n := len(res.outs[q2Stream(t, p, "O3")]); n != 1 {
		t.Errorf("Plain gave %d outputs; want 1 (the one good payload)", n)
	}
}

// qeFlagSrc: S does not declare text_invisibles, SD declares it as a non-optional bool (S3.2b's plan check Q3: "can
// be read on every stream"); each rule projects the flag, so every output carries the value the rule read.
const qeFlagSrc = `language cep/0.1;

stream S (k string, note string, ts timestamp)
  time ts watermark bounded 1 min;

stream SD (k string, note string, text_invisibles bool, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, flag bool, ts timestamp);
stream OD (k string, flag bool, ts timestamp);

rule Flag
  intent 'Each event with its invisible-character flag.'
  from S
  select k, text_invisibles as flag
  insert into O;

rule FlagDeclared
  intent 'Each event with its declared invisible-character flag.'
  from SD
  select k, text_invisibles as flag
  insert into OD;
`

// qeFlagSrcDeclaredOnly is qeFlagSrc's declared half alone: it checks clean today, so its test's red line is the
// flag, not the validator.
const qeFlagSrcDeclaredOnly = `language cep/0.1;

stream SD (k string, note string, text_invisibles bool, ts timestamp)
  time ts watermark bounded 1 min;

stream OD (k string, flag bool, ts timestamp);

rule FlagDeclared
  intent 'Each event with its declared invisible-character flag.'
  from SD
  select k, text_invisibles as flag
  insert into OD;
`

// zwsp is U+200B ZERO WIDTH SPACE, a format character (SEMANTICS §5.1 stage 1's class): its payload form is the
// JSON escape, so the source holds no invisible byte.
const qeZWSP = `\u200b`

// qeFlagCase is one event and the flag its rule must read.
type qeFlagCase struct {
	k, payload string // payload "" → a typed Send
	want       bool
}

// qeFlagCases are exit 3's events on stream name, each with a distinct key, 1 s apart.
func qeFlagCases(name string) []qeFlagCase {
	pl := func(at int64, k, note, extra string) string {
		return fmt.Sprintf(`{"k":%q,"note":"%s","ts":%q%s}`, k, note, ldRFC(ldT0+at), extra)
	}
	return []qeFlagCase{
		{"clean", pl(0, "clean", "plain text", ""), false},
		{"computed", pl(ldSec, "computed", "a"+qeZWSP+"b", ""), true},                                           // no text_invisibles: computed
		{"overridden-false", pl(2*ldSec, "overridden-false", "a"+qeZWSP+"b", `,"text_invisibles":false`), true}, // exit 3: false over a ZWSP → true
		{"overridden-true", pl(3*ldSec, "overridden-true", "plain", `,"text_invisibles":true`), false},          // a payload's true over clean text → false
		{"typed", "", false}, // a typed Send with Invisibles: true over clean text → false
	}
}

// qeFlagRun sends name's cases (the typed one with Invisibles set and, on SD, the declared slot true) and returns
// the flag each key's output carries, and the stream's counters.
func qeFlagRun(t *testing.T, src, name, out string) (map[string]string, int64) {
	t.Helper()
	p := pbBuild(t, "qa-flag.cep", src)
	s := q2Stream(t, p, name)
	live := q2Start(t, p, 1, q2NewClock(ldT0, ldSec).now, nil)
	sch := p.Streams[s].Schema
	for _, c := range qeFlagCases(name) {
		if c.payload != "" {
			live.payload(s, c.payload)
			continue
		}
		vals := make([]event.Value, sch.NumAttrs())
		for i := range vals {
			a := sch.Attr(i)
			switch {
			case a.Name == "k":
				vals[i] = ldStr(c.k)
			case a.Name == "note":
				vals[i] = ldStr("plain")
			case a.Name == event.InvisiblesAttr:
				vals[i] = event.Value{Kind: event.TBool, B: true} // a caller's value in the declared slot: overwritten
			case a.Type == event.TTimestamp:
				vals[i] = event.Value{Kind: event.TTimestamp, T: ldT0 + 4*ldSec}
			}
		}
		if err := live.r.Send(live.ctx, 0, event.Event{StreamID: s, Ts: ldT0 + 4*ldSec, Values: vals, Invisibles: true}); err != nil {
			t.Fatalf("typed Send: %v", err)
		}
	}
	res := live.finish()
	flags := map[string]string{}
	for _, line := range res.outs[q2Stream(t, p, out)] {
		flags[qeAttr(line, "k")] = qeAttr(line, "flag")
	}
	return flags, q2StreamStats(t, res.stats, s).FlagOverrides
}

// qeAttr is the raw JSON of attribute name in an AppendEvent line (a string value keeps its quotes stripped).
func qeAttr(line, name string) string {
	key := fmt.Sprintf(`"%s":`, name)
	for i := 0; i+len(key) <= len(line); i++ {
		if line[i:i+len(key)] != key {
			continue
		}
		rest := line[i+len(key):]
		if rest[0] == '"' {
			for j := 1; j < len(rest); j++ {
				if rest[j] == '"' {
					return rest[1:j]
				}
			}
		}
		for j := 0; j < len(rest); j++ {
			if rest[j] == ',' || rest[j] == '}' {
				return rest[:j]
			}
		}
	}
	return "<absent>"
}

// qeCheckFlags asserts exit 3's flags and the override count (2: the two payloads carrying text_invisibles; the
// typed Send counts nothing — design §5).
func qeCheckFlags(t *testing.T, flags map[string]string, overrides int64) {
	t.Helper()
	for _, c := range qeFlagCases("") {
		if want := fmt.Sprint(c.want); flags[c.k] != want {
			t.Errorf("key %s: the rule read text_invisibles = %s; want %s (the engine's flag, computed in the ingest step)", c.k, flags[c.k], want)
		}
	}
	if overrides != 2 {
		t.Errorf("FlagOverrides %d; want 2 (each payload carrying text_invisibles, whatever its value)", overrides)
	}
}

// TestIngestFlagDeclaredStream is exit 3 on a stream declaring text_invisibles as bool (S3.2b's plan check Q3):
// design §5 — computed once in the ingest step, a payload's own value ignored and counted, a caller's Invisibles
// and declared slot overwritten.
func TestIngestFlagDeclaredStream(t *testing.T) {
	t.Parallel()
	flags, overrides := qeFlagRun(t, qeFlagSrcDeclaredOnly, "SD", "OD")
	qeCheckFlags(t, flags, overrides)
}

// TestIngestFlagUndeclaredStream is exit 3 on a stream that does not declare text_invisibles: the name resolves on
// every source stream (S3.2b's plan check Q3: "text_invisibles can be read on every stream"; design §5's expression
// node reads Row.Invisibles()), and both rules read the same engine flag.
func TestIngestFlagUndeclaredStream(t *testing.T) {
	t.Parallel()
	flags, overrides := qeFlagRun(t, qeFlagSrc, "S", "O")
	qeCheckFlags(t, flags, overrides)
	flagsD, overridesD := qeFlagRun(t, qeFlagSrc, "SD", "OD")
	qeCheckFlags(t, flagsD, overridesD)
}
