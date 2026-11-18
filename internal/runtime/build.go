package runtime

import (
	"fmt"
	"math"
	goruntime "runtime"
	"slices"
	"time"

	"github.com/vorsar/cep/internal/event"
)

// build validates the apps and builds their state and every shard's rules; it starts nothing (Start and NewSync
// share it, design s3-2b-1 v2 §4).
func build(cfg Config, apps []App) (*Runtime, error) {
	if cfg.Shards <= 0 {
		cfg.Shards = goruntime.GOMAXPROCS(0)
	}
	if cfg.Mailbox <= 0 {
		cfg.Mailbox = 1024
	}
	if cfg.Intake <= 0 {
		cfg.Intake = 1024
	}
	if cfg.SinkQueue <= 0 {
		cfg.SinkQueue = 1024
	}
	if cfg.Now == nil {
		cfg.Now = func() int64 { return time.Now().UnixMilli() }
	}
	for i := range apps {
		if err := validateApp(i, &apps[i]); err != nil {
			return nil, err
		}
	}
	if cfg.RecordQueue <= 0 {
		cfg.RecordQueue = 65536
	}
	r := &Runtime{
		cfg: cfg, intake: make(chan item, cfg.Intake), handoff: make(chan handoff, cfg.Shards),
		stopped: make(chan struct{}),
	}
	// Config.Now is read once per app at its install (design s3-3-1 §2.4, MUST 10; S3.2a-2's §7.8: once at Start),
	// never for the empty runtime: the stamps' floor starts unset and the first install raises it.
	start := int64(math.MinInt64)
	if len(apps) > 0 {
		start = cfg.Now()
	}
	r.start = start
	var list []*appState
	for i := range apps {
		a := r.newAppState(AppID(i), apps[i], start)
		a.installRoutes()
		a.activated = true
		a.started.Store(true) // a built app is live from the start (NewSync)
		a.phase.Store(int32(phaseActive))
		list = append(list, a)
	}
	r.apps.Store(&list)
	for sid := range cfg.Shards {
		sh := newShard(sid, cfg.Mailbox)
		for _, a := range list {
			if err := sh.install(a.id, &a.app); err != nil {
				return nil, err
			}
		}
		r.shards = append(r.shards, sh)
	}
	return r, nil
}

// validateApp checks an app against design §2 (review F9, F21; QA Q4).
func validateApp(i int, a *App) error {
	if len(a.Schemas) != len(a.Streams) {
		return fmt.Errorf("runtime: app %d: %d schemas for %d streams", i, len(a.Schemas), len(a.Streams))
	}
	for sid, sch := range a.Schemas {
		if sch.Stream() != a.Streams[sid].Name {
			return fmt.Errorf("runtime: app %d: schema %d describes stream %q, not %q", i, sid, sch.Stream(),
				a.Streams[sid].Name)
		}
	}
	for ri, rule := range a.Rules {
		bad := func(format string, args ...any) error {
			return fmt.Errorf("runtime: app %d rule %d (%q): %s", i, ri, rule.Name, fmt.Sprintf(format, args...))
		}
		switch {
		case len(rule.Inputs) == 0:
			return bad("reads no stream")
		case rule.NewBody == nil:
			return bad("has no body")
		case len(rule.KeyTypes) == 0:
			return bad("a key has at least one part (a key-less rule: one string part)")
		case rule.KeyAttrs != nil && len(rule.KeyAttrs) != len(rule.Inputs):
			return bad("%d key attribute lists for %d inputs", len(rule.KeyAttrs), len(rule.Inputs))
		case rule.KeyAttrs != nil && rule.TTL <= 0:
			return bad("a keyed rule needs a TTL above 0")
		case rule.KeyAttrs == nil && (rule.TTL != 0 || len(rule.KeyTypes) != 1 || rule.KeyTypes[0] != event.TString):
			return bad("a key-less rule has TTL 0 and the string pseudo-key")
		case slices.ContainsFunc(rule.KeyTypes, func(t event.Type) bool { return t < event.TString || t >= event.TObject }):
			return bad("key types %v: a part of these types cannot key a partition", rule.KeyTypes)
		}
		seen := map[int]bool{}
		for k, sid := range rule.Inputs {
			if sid < 0 || sid >= len(a.Streams) || a.Streams[sid].Derived {
				return bad("input %d is not a source stream", sid)
			}
			if seen[sid] {
				return bad("input %d is listed twice (break-it V2)", sid)
			}
			seen[sid] = true
			if rule.KeyAttrs == nil {
				continue
			}
			attrs := rule.KeyAttrs[k]
			if len(attrs) != len(rule.KeyTypes) {
				return bad("stream %d's key has %d parts, the rule's %d", sid, len(attrs), len(rule.KeyTypes))
			}
			for p, attr := range attrs {
				if attr < 0 || attr >= a.Schemas[sid].NumAttrs() || a.Schemas[sid].Attr(attr).Type != rule.KeyTypes[p] {
					return bad("key attribute %d of stream %d is not a %v attribute", attr, sid, rule.KeyTypes[p])
				}
			}
		}
	}
	for bi, b := range a.Sinks {
		if b.Stream < 0 || b.Stream >= len(a.Streams) || !a.Streams[b.Stream].Derived || b.Sink == nil {
			return fmt.Errorf("runtime: app %d sink %d: binds no derived stream, or has no sink", i, bi)
		}
	}
	return nil
}
