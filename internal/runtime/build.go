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
	cfg = withDefaults(cfg)
	for i := range apps {
		if err := validateApp(i, &apps[i]); err != nil {
			return nil, err
		}
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

// withDefaults gives every unset Config field its default.
func withDefaults(cfg Config) Config {
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
	if cfg.RecordQueue <= 0 {
		cfg.RecordQueue = 65536
	}
	return cfg
}

// validateApp checks an app against design §2 (review F9, F21; QA Q4).
func validateApp(i int, a *App) error {
	if err := checkSchemas(i, a); err != nil {
		return err
	}
	for ri := range a.Rules {
		if err := checkRule(i, ri, a); err != nil {
			return err
		}
	}
	return checkSinks(i, a)
}

// checkSchemas checks that the app has one schema per stream, each describing its stream.
func checkSchemas(i int, a *App) error {
	if len(a.Schemas) != len(a.Streams) {
		return fmt.Errorf("runtime: app %d: %d schemas for %d streams", i, len(a.Schemas), len(a.Streams))
	}
	for sid, sch := range a.Schemas {
		if sch.Stream() != a.Streams[sid].Name {
			return fmt.Errorf("runtime: app %d: schema %d describes stream %q, not %q", i, sid, sch.Stream(),
				a.Streams[sid].Name)
		}
	}
	return nil
}

// checkRule checks rule ri of app i: its inputs and body, its key's shape and types, then each input; the first
// fault wins.
func checkRule(i, ri int, a *App) error {
	rule := a.Rules[ri]
	bad := func(format string, args ...any) error {
		return fmt.Errorf("runtime: app %d rule %d (%q): %s", i, ri, rule.Name, fmt.Sprintf(format, args...))
	}
	switch {
	case len(rule.Inputs) == 0:
		return bad("reads no stream")
	case rule.NewBody == nil:
		return bad("has no body")
	}
	if err := checkKeyShape(rule, bad); err != nil {
		return err
	}
	if err := checkKeyTypes(rule, bad); err != nil {
		return err
	}
	return checkRuleInputs(a, rule, bad)
}

// checkKeyShape checks the rule's key has a part, one attribute list per input and, keyed, a TTL above 0.
func checkKeyShape(rule Rule, bad func(format string, args ...any) error) error {
	switch {
	case len(rule.KeyTypes) == 0:
		return bad("a key has at least one part (a key-less rule: one string part)")
	case rule.KeyAttrs != nil && len(rule.KeyAttrs) != len(rule.Inputs):
		return bad("%d key attribute lists for %d inputs", len(rule.KeyAttrs), len(rule.Inputs))
	case rule.KeyAttrs != nil && rule.TTL <= 0:
		return bad("a keyed rule needs a TTL above 0")
	}
	return nil
}

// checkKeyTypes checks a key-less rule's pseudo-key and that every key part's type can key a partition.
func checkKeyTypes(rule Rule, bad func(format string, args ...any) error) error {
	switch {
	case rule.KeyAttrs == nil && (rule.TTL != 0 || len(rule.KeyTypes) != 1 || rule.KeyTypes[0] != event.TString):
		return bad("a key-less rule has TTL 0 and the string pseudo-key")
	case slices.ContainsFunc(rule.KeyTypes, func(t event.Type) bool { return t < event.TString || t >= event.TObject }):
		return bad("key types %v: a part of these types cannot key a partition", rule.KeyTypes)
	}
	return nil
}

// checkRuleInputs checks each input is a source stream listed once and, keyed, its key attributes.
func checkRuleInputs(a *App, rule Rule, bad func(format string, args ...any) error) error {
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
		if err := checkKeyAttrs(a, sid, rule.KeyAttrs[k], rule.KeyTypes, bad); err != nil {
			return err
		}
	}
	return nil
}

// checkKeyAttrs checks input stream sid's key attributes: one per key part, each an attribute of the part's type.
func checkKeyAttrs(a *App, sid int, attrs []int, types []event.Type, bad func(format string, args ...any) error) error {
	if len(attrs) != len(types) {
		return bad("stream %d's key has %d parts, the rule's %d", sid, len(attrs), len(types))
	}
	for p, attr := range attrs {
		if attr < 0 || attr >= a.Schemas[sid].NumAttrs() || a.Schemas[sid].Attr(attr).Type != types[p] {
			return bad("key attribute %d of stream %d is not a %v attribute", attr, sid, types[p])
		}
	}
	return nil
}

// checkSinks checks every sink binds a derived stream and has a sink.
func checkSinks(i int, a *App) error {
	for bi, b := range a.Sinks {
		if b.Stream < 0 || b.Stream >= len(a.Streams) || !a.Streams[b.Stream].Derived || b.Sink == nil {
			return fmt.Errorf("runtime: app %d sink %d: binds no derived stream, or has no sink", i, bi)
		}
	}
	return nil
}
