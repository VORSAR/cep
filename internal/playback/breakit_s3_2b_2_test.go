package playback_test

// S3.2b-2's QA break-it (design note docs/sprints/s3-2b-2-design.md v2): inputs where the build does the wrong
// thing, each a red test naming the defect, or a guard where a suspected break held. Prefix: bk2.

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/playback"
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
