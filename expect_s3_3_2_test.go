package cep_test

// S3.3-2's expect sandbox through the public API, written from the card, SPEC §6.8, errors.md and the owner's
// decisions (2026-11-07 item 4; 2026-11-08 items 1 and 2) before the design note. Each planted program is a corpus
// file with one block changed; a refused Deploy is asserted on its code, its position (the failing block's `expect`
// keyword, or the mistake's token), the rule's name and, where there is one, the attribute's name — never the
// message's wording. Rules (testing.md): no sleep; every wait is on a state under a ceiling; each test owns its
// runtime.

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
)

// exWant is one E-EXP-001 diagnostic a refused Deploy must carry.
type exWant struct {
	block int    // the failing block, 1-based in source order
	rule  string // the owning rule's name, named in the message
	attr  string // the diverging attribute, named in the message; "" when the divergence names none
}

// exExpectPos is the line and column of the n-th (1-based) `expect` keyword of src: the first token of a line
// whose text after its indentation starts with "expect " (comments start with "--").
func exExpectPos(t testing.TB, src string, n int) (line, col int) {
	t.Helper()
	seen := 0
	for i, l := range strings.Split(src, "\n") {
		trimmed := strings.TrimLeft(l, " \t")
		if strings.HasPrefix(trimmed, "expect ") {
			seen++
			if seen == n {
				return i + 1, len(l) - len(trimmed) + 1
			}
		}
	}
	t.Fatalf("the program has %d expect blocks, not %d", seen, n)
	return 0, 0
}

// exPosOf is the line and column of the n-th (1-based) occurrence of needle in src (ASCII lines).
func exPosOf(t testing.TB, src, needle string, n int) (line, col int) {
	t.Helper()
	from := 0
	for k := 0; k < n; k++ {
		j := strings.Index(src[from:], needle)
		if j < 0 {
			t.Fatalf("%q occurs fewer than %d times", needle, n)
		}
		from += j
		if k < n-1 {
			from += len(needle)
		}
	}
	before := src[:from]
	line = strings.Count(before, "\n") + 1
	col = from - (strings.LastIndex(before, "\n") + 1) + 1
	return line, col
}

// exPlant replaces the one occurrence of old in src; more or fewer is a fault of the test.
func exPlant(t testing.TB, src, old, new string) string {
	t.Helper()
	if c := strings.Count(src, old); c != 1 {
		t.Fatalf("the planted text occurs %d times, want 1: %q", c, old)
	}
	return strings.Replace(src, old, new, 1)
}

// exPlantRe replaces the one match of the expression re in src.
func exPlantRe(t testing.TB, src, re, repl string) string {
	t.Helper()
	x := regexp.MustCompile(re)
	if c := len(x.FindAllStringIndex(src, -1)); c != 1 {
		t.Fatalf("the planted pattern matches %d times, want 1: %q", c, re)
	}
	return x.ReplaceAllString(src, repl)
}

// exNames reports whether msg names word as a whole word.
func exNames(msg, word string) bool {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_]|$)`).MatchString(msg)
}

// exDeploy deploys src into a fresh runtime of its own.
func exDeploy(t testing.TB, src string, opts ...cep.DeployOption) (*cep.AppHandle, error) {
	t.Helper()
	ctx := fcCeiling(t)
	r := fcNew(t, cep.Config{})
	return r.Deploy(ctx, "expect", src, opts...)
}

// exDeploys asserts src deploys.
func exDeploys(t testing.TB, src string, opts ...cep.DeployOption) {
	t.Helper()
	if _, err := exDeploy(t, src, opts...); err != nil {
		t.Fatalf("Deploy: %v; want it deployed (every block passes)", err)
	}
}

// exRefused asserts err is a *cep.DeployError of exactly the wanted E-EXP-001 diagnostics, in source order, each
// at its block's `expect` keyword, naming the rule and the attribute.
func exRefused(t testing.TB, src string, h *cep.AppHandle, err error, wants ...exWant) {
	t.Helper()
	var de *cep.DeployError
	if !errors.As(err, &de) {
		t.Fatalf("Deploy: handle %v, error %v; want a *cep.DeployError carrying E-EXP-001", h != nil, err)
	}
	if h != nil {
		t.Errorf("a refused Deploy returned a handle")
	}
	if len(de.Diagnostics) != len(wants) {
		t.Fatalf("%d diagnostics, want %d (one per failing block):\n%v", len(de.Diagnostics), len(wants), de)
	}
	for i, w := range wants {
		d := de.Diagnostics[i]
		line, col := exExpectPos(t, src, w.block)
		if d.Code != "E-EXP-001" || d.Line != line || d.Col != col {
			t.Errorf("diagnostic %d: %s at %d:%d; want E-EXP-001 at %d:%d (block %d's expect)", i, d.Code, d.Line, d.Col, line, col, w.block)
		}
		if !exNames(d.Message, w.rule) {
			t.Errorf("diagnostic %d's message %q does not name the rule %s", i, d.Message, w.rule)
		}
		if w.attr != "" && !exNames(d.Message, w.attr) {
			t.Errorf("diagnostic %d's message %q does not name the attribute %s", i, d.Message, w.attr)
		}
	}
}

// exRefusedDeploy deploys src and asserts the refusal.
func exRefusedDeploy(t testing.TB, src string, wants []exWant, opts ...cep.DeployOption) {
	t.Helper()
	h, err := exDeploy(t, src, opts...)
	exRefused(t, src, h, err, wants...)
}

const (
	ex07 = "07-consent-gate.cep"
	ex09 = "09-params-and-quoting.cep"
	ex10 = "10-grouped-keys.cep"
)

// TestExpectCorpusDeploys (guard; exit 1's material): the three corpus files that build today deploy with every
// block run — 07's six, 09's four and 10's five.
func TestExpectCorpusDeploys(t *testing.T) {
	t.Parallel()
	for _, f := range []string{ex07, ex09, ex10} {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			exDeploys(t, fcExample(t, f))
		})
	}
}

// TestExpectPlantedWrongExpects (exit 2): each planted wrong expect — a one-line change to one corpus block —
// fails Deploy with E-EXP-001 at that block, naming the rule and, for a wrong value or timestamp, the attribute.
func TestExpectPlantedWrongExpects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		file  string
		plant func(t *testing.T, src string) string
		want  exWant
	}{
		// 09 block 1: math:round(0.9) is 1.0; the block now expects 2.0.
		{"wrong value", ex09, func(t *testing.T, s string) string {
			return exPlant(t, s, "scaled: 1.0, ts: timestamp '2026-01-01T00:00:01Z'} ]", "scaled: 2.0, ts: timestamp '2026-01-01T00:00:01Z'} ]")
		}, exWant{1, "HighRisk", "scaled"}},
		// 10 block 1: the grant's scope is now 'read', so the alert the then-list expects never comes.
		{"missing output", ex10, func(t *testing.T, s string) string {
			return exPlant(t, s, "expect given Grants [ {tenantId: 't1', userId: 'alice', scope: 'admin', ts: timestamp '2026-01-01T10:00:00Z'} ]",
				"expect given Grants [ {tenantId: 't1', userId: 'alice', scope: 'read', ts: timestamp '2026-01-01T10:00:00Z'} ]")
		}, exWant{1, "AdminScopeGrant", ""}},
		// 07 block 5: the then-list drops t2's alert, which still fires: one output more than listed.
		{"extra output", ex07, func(t *testing.T, s string) string {
			return exPlantRe(t, s, `,\s*\{tenantId: 't2', appId: 'rogue', ts: timestamp '2026-01-01T10:02:00Z'\} \]`, " ]")
		}, exWant{5, "UnknownAppConsent", ""}},
		// 07 block 5: t2's alert (10:02) listed before t1's (10:00).
		{"wrong order", ex07, func(t *testing.T, s string) string {
			return exPlantRe(t, s,
				`\{tenantId: 't1', appId: 'rogue', ts: timestamp '2026-01-01T10:00:00Z'\},(\s*)\{tenantId: 't2', appId: 'rogue', ts: timestamp '2026-01-01T10:02:00Z'\} \]`,
				"{tenantId: 't2', appId: 'rogue', ts: timestamp '2026-01-01T10:02:00Z'},${1}{tenantId: 't1', appId: 'rogue', ts: timestamp '2026-01-01T10:00:00Z'} ]")
		}, exWant{5, "UnknownAppConsent", ""}},
		// 09 block 3: a risk of 0.76 crosses the 0.75 threshold under `then RiskAlerts [ ]`.
		{"an output under an empty then", ex09, func(t *testing.T, s string) string {
			return exPlant(t, s, "risk: 0.75", "risk: 0.76")
		}, exWant{3, "HighRisk", ""}},
		// 10 block 1: the alert is at 10:00:00; the block now expects 10:00:01.
		{"wrong timestamp", ex10, func(t *testing.T, s string) string {
			return exPlant(t, s, "then GrantAlerts [ {tenantId: 't1', userId: 'alice', scope: 'admin', ts: timestamp '2026-01-01T10:00:00Z'} ]",
				"then GrantAlerts [ {tenantId: 't1', userId: 'alice', scope: 'admin', ts: timestamp '2026-01-01T10:00:01Z'} ]")
		}, exWant{1, "AdminScopeGrant", "ts"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := c.plant(t, fcExample(t, c.file))
			exRefusedDeploy(t, src, []exWant{c.want})
		})
	}
}

// TestExpectEveryFailingBlockInSourceOrder: two failing blocks of 10 (planted in the order 4, then 2) give two
// E-EXP-001 diagnostics, block 2's first; the three passing blocks give none.
func TestExpectEveryFailingBlockInSourceOrder(t *testing.T) {
	t.Parallel()
	src := fcExample(t, ex10)
	// block 4: a non-null userId fires under `then GrantAlerts [ ]`
	src = exPlant(t, src, "{tenantId: 't1', userId: null, scope: 'admin', ts: timestamp '2026-01-01T10:00:00Z'} ]",
		"{tenantId: 't1', userId: 'bob', scope: 'admin', ts: timestamp '2026-01-01T10:00:00Z'} ]")
	// block 2: an admin scope fires under `then GrantAlerts [ ]`
	src = exPlant(t, src, "scope: 'read'", "scope: 'admin'")
	exRefusedDeploy(t, src, []exWant{{2, "AdminScopeGrant", ""}, {4, "AdminScopeGrant", ""}})
}

// exSandboxSrc: a partitioned, rate-limited rule over a table (one alert per key every 5 min, keys not in T).
// Its blocks are appended by each test; %s is the blocks.
const exSandboxSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

table T (k string) primary key (k);

stream O (k string, ts timestamp);

rule R
  intent 'Keys not in T, at most one alert per key every 5 min.'
  partition by k of S ttl 10 min
  from S[k not in T]
%s  select k
  output first every 5 min
  insert into O;
`

func exSandbox(blocks ...string) string {
	return strings.Replace(exSandboxSrc, "%s", strings.Join(blocks, ""), 1)
}

// TestExpectFreshSandboxPerBlock (rule 1): each block starts with no rule state and empty tables.
//   - the rate limit: block 1 alerts for key a at 10:00; block 2's alert for a at 10:01 passes only if block 1's
//     anchor is gone (10:01 <= 10:00 + 5 min would suppress it);
//   - the table: block 1 seeds b into T; block 2's b passes T's `not in` only if T is empty again.
//
// Each pair: the block as it must read deploys; the block reading as if state leaked is refused at block 2.
func TestExpectFreshSandboxPerBlock(t *testing.T) {
	t.Parallel()
	const anchor = "  expect given S [ {k: 'a', ts: timestamp '2026-01-01T10:00:00Z'} ]\n" +
		"         then O [ {k: 'a', ts: timestamp '2026-01-01T10:00:00Z'} ]\n"
	const seeded = "  expect given table T [ {k: 'b'} ]\n" +
		"         given S [ {k: 'b', ts: timestamp '2026-01-01T10:00:00Z'} ]\n" +
		"         then O [ ]\n"
	cases := []struct {
		name         string
		first        string
		fresh, stale string // block 2 as a fresh sandbox reads it, and as a leaking one would
	}{
		{"rate-limit anchor", anchor,
			"  expect given S [ {k: 'a', ts: timestamp '2026-01-01T10:01:00Z'} ]\n         then O [ {k: 'a', ts: timestamp '2026-01-01T10:01:00Z'} ]\n",
			"  expect given S [ {k: 'a', ts: timestamp '2026-01-01T10:01:00Z'} ]\n         then O [ ]\n"},
		{"table rows", seeded,
			"  expect given S [ {k: 'b', ts: timestamp '2026-01-01T10:00:00Z'} ]\n         then O [ {k: 'b', ts: timestamp '2026-01-01T10:00:00Z'} ]\n",
			"  expect given S [ {k: 'b', ts: timestamp '2026-01-01T10:00:00Z'} ]\n         then O [ ]\n"},
	}
	for _, c := range cases {
		t.Run(c.name+"/fresh deploys", func(t *testing.T) {
			t.Parallel()
			exDeploys(t, exSandbox(c.first, c.fresh))
		})
		t.Run(c.name+"/stale refused", func(t *testing.T) {
			t.Parallel()
			src := exSandbox(c.first, c.stale)
			exRefusedDeploy(t, src, []exWant{{2, "R", ""}})
		})
	}
}

// TestExpectGivenTableSeedsBeforeAnyEvent (rule 1; SPEC §6.8 rev 2): a `given table` written after the block's
// `given S` still seeds T before S's event, so c is in T and nothing fires. Read the other way (the row loaded after
// the event) c would alert; that block is refused.
func TestExpectGivenTableSeedsBeforeAnyEvent(t *testing.T) {
	t.Parallel()
	const given = "  expect given S [ {k: 'c', ts: timestamp '2026-01-01T10:00:00Z'} ]\n" +
		"         given table T [ {k: 'c'} ]\n"
	t.Run("seeded first deploys", func(t *testing.T) {
		t.Parallel()
		exDeploys(t, exSandbox(given+"         then O [ ]\n"))
	})
	t.Run("seeded after refused", func(t *testing.T) {
		t.Parallel()
		src := exSandbox(given + "         then O [ {k: 'c', ts: timestamp '2026-01-01T10:00:00Z'} ]\n")
		exRefusedDeploy(t, src, []exWant{{1, "R", ""}})
	})
}

// TestExpectListOrderIsArrivalOrder (owner 2026-11-07, 4): within one list the written order is the arrival order,
// even where its times go backwards. 07 block 6: app-b (10:00:30) arrives first and alerts; app-a (10:00:00) is
// then rate-limited for tenant t1. Read sorted by time, app-a would alert and app-b be suppressed: that then-list
// is refused at block 6.
func TestExpectListOrderIsArrivalOrder(t *testing.T) {
	t.Parallel()
	src := exPlant(t, fcExample(t, ex07),
		"then ConsentAlerts [ {tenantId: 't1', appId: 'app-b', ts: timestamp '2026-01-01T10:00:30Z'} ]",
		"then ConsentAlerts [ {tenantId: 't1', appId: 'app-a', ts: timestamp '2026-01-01T10:00:00Z'} ]")
	exRefusedDeploy(t, src, []exWant{{6, "UnknownAppConsent", ""}})
}

// TestExpectThenChecksOnlyNamedAttributes (rule 2; guard): 07 block 1's then-record naming tenantId alone passes;
// naming a wrong tenantId is refused at block 1, naming the attribute.
func TestExpectThenChecksOnlyNamedAttributes(t *testing.T) {
	t.Parallel()
	const old = "then ConsentAlerts [ {tenantId: 't1', appId: 'rogue', ts: timestamp '2026-01-01T10:00:00Z'} ]"
	t.Run("named only deploys", func(t *testing.T) {
		t.Parallel()
		exDeploys(t, exPlant(t, fcExample(t, ex07), old, "then ConsentAlerts [ {tenantId: 't1'} ]"))
	})
	t.Run("named wrong refused", func(t *testing.T) {
		t.Parallel()
		src := exPlant(t, fcExample(t, ex07), old, "then ConsentAlerts [ {tenantId: 't2'} ]")
		exRefusedDeploy(t, src, []exWant{{1, "UnknownAppConsent", "tenantId"}})
	})
}

// TestExpectOmittedGivenAttributeIsNull (rule 2; SPEC §6.8): 07 block 3 with `appId: null` left out reads appId as
// null, so coalesce gives 'missing' and the block passes; with `then ConsentAlerts [ ]` that alert is refused.
func TestExpectOmittedGivenAttributeIsNull(t *testing.T) {
	t.Parallel()
	omitted := func(t *testing.T) string { return exPlant(t, fcExample(t, ex07), "appId: null, ", "") }
	t.Run("omitted deploys", func(t *testing.T) {
		t.Parallel()
		exDeploys(t, omitted(t))
	})
	t.Run("omitted alerts under an empty then", func(t *testing.T) {
		t.Parallel()
		src := exPlant(t, omitted(t), "then ConsentAlerts [ {tenantId: 't1', appId: 'missing', ts: timestamp '2026-01-01T10:00:00Z'} ]", "then ConsentAlerts [ ]")
		exRefusedDeploy(t, src, []exWant{{3, "UnknownAppConsent", ""}})
	})
}

// TestExpectRunsWithTheDeploymentsParams (owner 2026-11-08, 1; exit 9): the blocks run the plan built with the
// deployment's overrides.
//
// Refused: `lookback` at 2 hours. 09's rule is rate-limited without a partition, so block 2's four events share one
// anchor: a1 00:00:01 alerts (anchor); a2 00:30:00 suppressed; a1 01:30:01 suppressed (<= anchor + 2 h); a2
// 01:30:01.001 suppressed (<= 02:00:01) — one alert where the block expects two.
//
// Deploys (worked by hand, block by block, against 09's four blocks):
//   - threshold 0.79: block 1 (0.9 > 0.79) alerts; block 2's risks 0.9 and 0.8 still cross, the rate limit is
//     unchanged, so a1 00:00:01 and a2 01:30:01.001 alert as written; block 3's 0.75 and 0.5 stay at or under it,
//     nothing fires; block 4's staging event fails the label, the prod one at 00:10:00 alerts. Every result is the
//     default's. (Any threshold in [0.75, 0.8) would do; lookback, by contrast, has no other value: block 2 needs
//     01:30:01 suppressed and 01:30:01.001 alerting, so lookback is exactly 1 h 30 min at ms resolution.)
//   - lookback `90 min`: the default's value written another way.
//   - strict false: the rule never reads ${strict}.
func TestExpectRunsWithTheDeploymentsParams(t *testing.T) {
	t.Parallel()
	t.Run("lookback 2 hour refused", func(t *testing.T) {
		t.Parallel()
		exRefusedDeploy(t, fcExample(t, ex09), []exWant{{2, "HighRisk", ""}}, cep.WithParam("lookback", "2 hour"))
	})
	for _, p := range [][2]string{{"threshold", "0.79"}, {"lookback", "90 min"}, {"strict", "false"}} {
		t.Run(p[0]+" "+p[1]+" deploys", func(t *testing.T) {
			t.Parallel()
			exDeploys(t, fcExample(t, ex09), cep.WithParam(p[0], p[1]))
		})
	}
}

// TestExpectMistakeIsReportedAloneBeforeAnyBlockRuns (owner 2026-11-08, 2; exit 8): 07 with a misspelt field in
// block 1's given record (appID) and a block 2 that would fail: Deploy reports E-EXP-002 at the field's key and
// nothing else — no block ran, so no E-EXP-001.
func TestExpectMistakeIsReportedAloneBeforeAnyBlockRuns(t *testing.T) {
	t.Parallel()
	src := exPlant(t, fcExample(t, ex07),
		"given ConsentGrants [ {tenantId: 't1', appId: 'rogue', grantedBy: 'admin', ts: timestamp '2026-01-01T10:00:00Z'} ]",
		"given ConsentGrants [ {tenantId: 't1', appID: 'rogue', grantedBy: 'admin', ts: timestamp '2026-01-01T10:00:00Z'} ]")
	src = exPlant(t, src, "then ConsentAlerts [ ]", "then ConsentAlerts [ {tenantId: 't1'} ]") // block 2 would fail
	line, col := exPosOf(t, src, "appID", 1)
	h, err := exDeploy(t, src)
	var de *cep.DeployError
	if !errors.As(err, &de) {
		t.Fatalf("Deploy: handle %v, error %v; want a *cep.DeployError carrying E-EXP-002", h != nil, err)
	}
	if len(de.Diagnostics) != 1 || de.Diagnostics[0].Code != "E-EXP-002" || de.Diagnostics[0].Line != line || de.Diagnostics[0].Col != col {
		t.Fatalf("%v\nwant exactly one diagnostic, E-EXP-002 at %d:%d (the key appID), and no E-EXP-001", de, line, col)
	}
}

// TestExpectThenTypoIsAMistake (owner 2026-11-08, 2; S3.2a-1's break-it forward): a misspelt attribute in a
// then-record (09 block 1's `scaled` as `scaeld`) would read as unasserted; it is E-EXP-002 at the key.
func TestExpectThenTypoIsAMistake(t *testing.T) {
	t.Parallel()
	src := exPlant(t, fcExample(t, ex09), "scaled: 1.0, ts: timestamp '2026-01-01T00:00:01Z'} ]", "scaeld: 1.0, ts: timestamp '2026-01-01T00:00:01Z'} ]")
	line, col := exPosOf(t, src, "scaeld", 1)
	h, err := exDeploy(t, src)
	var de *cep.DeployError
	if !errors.As(err, &de) {
		t.Fatalf("Deploy: handle %v, error %v; want a *cep.DeployError carrying E-EXP-002", h != nil, err)
	}
	if len(de.Diagnostics) != 1 || de.Diagnostics[0].Code != "E-EXP-002" || de.Diagnostics[0].Line != line || de.Diagnostics[0].Col != col {
		t.Fatalf("%v\nwant exactly one diagnostic, E-EXP-002 at %d:%d (the key scaeld)", de, line, col)
	}
}

// exGrant fills 07's ConsentGrants.
func exGrant(tenant, app string, ts int64) func(cepio.RowWriter) error {
	return func(w cepio.RowWriter) error {
		w.SetString(w.AttrIndex("tenantId"), tenant)
		w.SetString(w.AttrIndex("appId"), app)
		w.SetString(w.AttrIndex("grantedBy"), "admin")
		w.SetTimestamp(w.AttrIndex("ts"), ts)
		return nil
	}
}

// TestExpectSandboxTouchesNothingLive (rule 6).
func TestExpectSandboxTouchesNothingLive(t *testing.T) {
	t.Parallel()
	// guard: a Go-bound source is never opened and a Go-bound sink never written during Deploy.
	t.Run("bound source and sink untouched", func(t *testing.T) {
		t.Parallel()
		ctx := fcCeiling(t)
		r := fcNew(t, cep.Config{})
		var opens, writes atomic.Int64
		src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
			opens.Add(1)
			<-sctx.Done()
			return nil
		})
		sink := &fcFuncSink{mode: cepio.Append, write: func(context.Context, cepio.OutEvent) error {
			writes.Add(1)
			return nil
		}}
		fcDeploy(t, ctx, r, "07", fcExample(t, ex07), cep.WithSource("ConsentGrants", src), cep.WithSink("ConsentAlerts", sink))
		if opens.Load() != 0 || writes.Load() != 0 {
			t.Errorf("during Deploy: %d source opens, %d sink writes; want none (the blocks' alerts stay in the sandbox)", opens.Load(), writes.Load())
		}
	})
	// guard: the deployment's recording holds nothing of the blocks: empty after Deploy; header and stop only
	// after a Start and a Stop with no events.
	t.Run("recording holds nothing of the blocks", func(t *testing.T) {
		t.Parallel()
		ctx := fcCeiling(t)
		r := fcNew(t, cep.Config{})
		var buf bytes.Buffer
		_, bind := fcSink("ConsentAlerts")
		h := fcDeploy(t, ctx, r, "07", fcExample(t, ex07), bind, cep.WithRecording(&buf))
		if buf.Len() != 0 {
			t.Errorf("after Deploy the recording holds %q; want nothing", buf.String())
		}
		fcStart(t, ctx, h)
		fcStop(t, ctx, h)
		if st := fcStats(t, ctx, h); st.Recording.Records != 2 || st.Recording.Failed {
			t.Errorf("Recording %+v; want 2 records (header, stop), not failed:\n%s", st.Recording, buf.String())
		}
		if strings.Contains(buf.String(), "rogue") || strings.Contains(buf.String(), "crm") {
			t.Errorf("the recording carries a block's value:\n%s", buf.String())
		}
	})
	// guard: 07's blocks seed RegisteredApps with crm in their sandboxes; the live table stays empty, so a live
	// grant for crm alerts.
	t.Run("live table empty", func(t *testing.T) {
		t.Parallel()
		ctx := fcCeiling(t)
		r := fcNew(t, cep.Config{})
		sink, bind := fcSink("ConsentAlerts")
		h := fcDeploy(t, ctx, r, "07", fcExample(t, ex07), bind)
		fcStart(t, ctx, h)
		fcSend(t, ctx, fcInput(t, h, "ConsentGrants"), exGrant("t1", "crm", fcAt(1)))
		fcStop(t, ctx, h)
		recs := sink.Drain()
		if len(recs) != 1 || !strings.Contains(string(recs[0].Canonical), `"crm"`) {
			t.Errorf("live outputs %v; want one alert for crm (a block's `given table` row is not in the deployed app)", recs)
		}
	})
	// a Deploy refused with E-EXP-001 returns no handle, and its bound source and sink are never used.
	t.Run("refused deploy registers no app", func(t *testing.T) {
		t.Parallel()
		ctx := fcCeiling(t)
		r := fcNew(t, cep.Config{})
		var opens, writes atomic.Int64
		src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
			opens.Add(1)
			<-sctx.Done()
			return nil
		})
		sink := &fcFuncSink{mode: cepio.Append, write: func(context.Context, cepio.OutEvent) error {
			writes.Add(1)
			return nil
		}}
		bad := exPlant(t, fcExample(t, ex07), "then ConsentAlerts [ ]", "then ConsentAlerts [ {tenantId: 't1'} ]")
		h, err := r.Deploy(ctx, "07", bad, cep.WithSource("ConsentGrants", src), cep.WithSink("ConsentAlerts", sink))
		exRefused(t, bad, h, err, exWant{2, "UnknownAppConsent", ""})
		if err := r.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
		if opens.Load() != 0 || writes.Load() != 0 {
			t.Errorf("%d source opens, %d sink writes; want none", opens.Load(), writes.Load())
		}
	})
}

// TestCloseRefusesADeployOnceItHasBegun (known issue S3.3-1-2): Close refuses new apps from the moment it starts,
// before its snapshot of the apps. The window is forced: app "held" has a source that waits for its cancellation
// and a sink whose Write holds the drain; once the source is cancelled, Close has taken its snapshot and is inside
// held's drain, and a Deploy then must get ErrClosed (today it is registered after the snapshot and never drained).
func TestCloseRefusesADeployOnceItHasBegun(t *testing.T) {
	t.Parallel()
	ctx := fcCeiling(t)
	r, err := cep.New(cep.Config{Now: fcNow})
	if err != nil {
		t.Fatalf("cep.New: %v", err)
	}
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	entered := make(chan struct{}, 1)
	sink := &fcFuncSink{mode: cepio.Append, write: func(wctx context.Context, e cepio.OutEvent) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			return nil
		case <-wctx.Done():
			return wctx.Err()
		}
	}}
	cancelled := make(chan struct{})
	src := fcFuncSource(func(sctx context.Context, emit cepio.Emitter) error {
		if err := emit.Emit(sctx, "", fc09("a1", 0.9, "prod", fcAt(1))); err != nil {
			return err
		}
		<-sctx.Done()
		close(cancelled)
		return nil
	})
	h := fcDeploy(t, ctx, r, "held", fcExample(t, ex09), cep.WithSink("RiskAlerts", sink), cep.WithSource("agent-events", src))
	fcStart(t, ctx, h)
	select {
	case <-entered: // the alert is in the sink's Write, held
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for the sink to hold the alert: %v", ctx.Err())
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close(ctx) }()
	select {
	case <-cancelled: // Close is draining held: its snapshot is taken
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for Close to cancel held's source: %v", ctx.Err())
	}
	late, err := r.Deploy(ctx, "late", fcExample(t, ex09))
	if !errors.Is(err, cep.ErrClosed) || late != nil {
		t.Errorf("Deploy while Close drains: handle %v, error %v; want ErrClosed and no handle (S3.3-1-2)", late != nil, err)
	}
	free()
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("ceiling hit waiting for Close: %v", ctx.Err())
	}
}
