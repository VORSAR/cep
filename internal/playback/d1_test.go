package playback_test

import (
	"bytes"
	"fmt"
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/playback"
)

// qaD1Src: S lags far behind T in a fixture (S bounded 60 min, T ascending), and one plain rule on S, keyed by k,
// TTL 10 min, `output first every 5 min`. T running ahead lifts S's contribution (the virtual floor) far above S's
// admission watermark, so S accepts events behind the rule clock (review F2 of S3.2b-1's design).
const qaD1Src = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 60 min;

stream T (k string, ts timestamp)
  time ts watermark ascending;

stream O (k string, ts timestamp);

rule PerKey
  intent 'Alert per key at most once per 5 min.'
  partition by k of S ttl 10 min
  from S
  select k
  output first every 5 min
  insert into O;
`

func d1Line(stream string, at int64, k string) string {
	return fmt.Sprintf(`{"stream":%q,"attrs":{"k":%q,"ts":%q}}`, stream, k, ldRFC(ldT0+at))
}

// TestOnArrivalPurgesOnItsStreamsAdmission: owner D1 of S3.2b-1 — a plain rule forgets a key on its own stream's
// admission watermark, never on the floors the virtual clock or idleness raise, so a purge never re-opens a
// suppression. The review's scenario (10:00 is 0): T@10:00, then S{a}@08:30 → alert; S{a}@08:32 → suppressed (inside
// 5 min). On the rule clock (the virtual floor, 09:00 here) the key was purged at once and 08:32 alerted again. Then
// T@10:20 and S{a}@08:33 → still suppressed; S{a}@10:00 → alert (S's admission 09:00 is past 08:33 + 10 min: purged,
// and 10:00 is past the interval anyway). Exactly the written rule's alerts: 08:30 and 10:00.
func TestOnArrivalPurgesOnItsStreamsAdmission(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-d1.cep", qaD1Src)
	log := ldLog(
		d1Line("T", 0, "x"),
		d1Line("S", -90*ldMin, "a"),
		d1Line("S", -88*ldMin, "a"),
		d1Line("T", 20*ldMin, "x"),
		d1Line("S", -87*ldMin, "a"),
		d1Line("S", 0, "a"),
	)
	outs, _ := pbReplay(t, p, log)
	var got []string
	for _, o := range outs {
		got = append(got, ldRFC(o.Ev.Ts))
	}
	want := []string{ldRFC(ldT0 - 90*ldMin), ldRFC(ldT0)}
	if !slices.Equal(got, want) {
		t.Errorf("alerts %v, want %v (one per 5 min per key, by the written rule)", got, want)
	}
}

// TestPlanDriverReplaysByteIdentically re-runs S3.2a's exit 1 through NewPlanDriver: the same log replayed twice
// gives byte-identical canonical output (example 07, per tenant, with a table record and suppressions).
func TestPlanDriverReplaysByteIdentically(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "07", pbExample(t, "07-consent-gate.cep"))
	put := `{"table":"RegisteredApps","op":"put","row":{"appId":"crm"}}`
	log := ldLog(put,
		pbGrant(0, "t1", "rogue", ""), pbGrant(ldMin, "t2", "", ""), pbGrant(2*ldMin, "t1", "crm", ""),
		pbGrant(3*ldMin, "t1", "x", ""), pbGrant(6*ldMin, "t2", "y", ""), pbGrant(30*ldMin, "t1", "z", ""))
	encode := func() []byte {
		outs, _ := pbReplay(t, p, log)
		var b []byte
		for _, o := range outs {
			var err error
			if b, err = playback.AppendEvent(b, p.Streams[o.Ev.StreamID].Schema, o.Ev); err != nil {
				t.Fatalf("AppendEvent: %v", err)
			}
			b = append(b, '\n')
		}
		return b
	}
	first, second := encode(), encode()
	if len(first) == 0 || !bytes.Equal(first, second) {
		t.Errorf("two replays differ or are empty:\n%s\n---\n%s", first, second)
	}
}
