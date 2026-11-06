package playback_test

import (
	"fmt"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
)

// TestOnArrivalKeysPurgeAtDrainAndEnd: owner D1 of S3.2b-1, the half the combined review found untested (C1): an
// on-arrival rule's clock takes a drain's and a fixture end's advances, so its due keys purge there too. qaD1Src: S
// (bounded 60 min), T (ascending), PerKey on S keyed by k, TTL 10 min. S{a}@10:00 leaves S's admission at 09:00, so
// a's TTL (due 10:10) never falls due on S alone.
//   - Fixture: End raises everything to +∞: a purges (Purged 1).
//   - Recording: T@10:20, then the stop: the drain level is 10:20 + 1 ms, past a's due: a purges (Purged 1).
func TestOnArrivalKeysPurgeAtDrainAndEnd(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "qa-d1.cep", qaD1Src)
	for _, c := range []struct{ name, log string }{
		{"fixture end", ldLog(d1Line("S", 0, "a"))},
		{"recording stop", ldLog(
			fmt.Sprintf(`{"recording":{"start":%q}}`, ldRFC(ldT0)),
			fmt.Sprintf(`{"stream":"S","attrs":{"k":"a","ts":%q},"arrival":%q}`, ldRFC(ldT0), ldRFC(ldT0)),
			fmt.Sprintf(`{"stream":"T","attrs":{"k":"x","ts":%q},"arrival":%q}`, ldRFC(ldT0+20*ldMin), ldRFC(ldT0+ldSec)),
			`{"stop":true}`)},
	} {
		d := pbDriver(t, p)
		pbRun(t, d, p, c.log)
		if rs := d.Stats().Rules[0][0]; rs.Purged != 1 {
			t.Errorf("%s: PerKey's stats %+v; want Purged 1 (the drain's or the end's advance reaches on-arrival keys)", c.name, rs)
		}
	}
}

// pbDriver is a NewPlanDriver for p whose outputs are dropped.
func pbDriver(t testing.TB, p *plan.App) *playback.Driver {
	t.Helper()
	d, err := playback.NewPlanDriver(p, func(event.Event) {})
	if err != nil {
		t.Fatalf("NewPlanDriver: %v", err)
	}
	return d
}

// pbRun steps log through d, then its End, as playback.Replay does.
func pbRun(t testing.TB, d *playback.Driver, p *plan.App, log string) {
	t.Helper()
	if err := playback.Replay(ldReader(t, pbApp(p), log), d, func(*playback.Step) error { return nil }); err != nil {
		t.Fatalf("Replay: %v", err)
	}
}
