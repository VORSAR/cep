package validate

import (
	"fmt"
	"strings"

	"github.com/vorsar/cep/internal/model"
)

// bounds is phase 5: partitions (SEMANTICS §7, §9; SPEC §4.4), every bound slot's duration (SPEC §2.4), TTL against
// the bounds it must cover (SEMANTICS §9, §11.3), and the window and lateness clauses.
func (c *checker) bounds() {
	for i := range c.app.Streams {
		if t := c.app.Streams[i].Time; t.IdleAfter == 0 && t.IdlePos != (model.Pos{}) {
			c.report("E-DUR-003", t.IdlePos, "`idle after` of %s is zero", c.app.Streams[i].Name)
		}
	}
	for _, r := range c.rules() {
		if c.poisoned[r] {
			continue
		}
		c.partition(r)
		c.zeroDurations(r)
		c.ttl(r)
		c.clauses(r)
	}
}

func (c *checker) partition(r *model.Rule) {
	keys := r.Partition.Keys
	if len(keys) == 0 {
		if r.Body.Kind != model.BodyPlain {
			c.report("E-PART-001", r.Body.Pos, "rule %s holds state (a %s) and has no partition", r.Name, r.Body.Kind)
		}
		return
	}
	for i := range keys {
		if c.badKey[&keys[i]] {
			return // a key already reported may have been meant for any stream the rule reads
		}
	}
	for _, name := range read(r) {
		keyed := false
		for _, k := range keys {
			keyed = keyed || k.Stream == name
		}
		if !keyed {
			c.report("E-PART-002", r.Partition.Pos, "the partition of rule %s keys no attribute of %s, which the rule reads", r.Name, name)
		}
	}
	keyed := map[string]bool{} // a stream keyed twice: one key value per stream in this release (break-it B1 of S3.2b-1)
	twice := map[*model.PartitionKey]bool{}
	for i := range keys {
		k := &keys[i]
		if c.badKey[k] {
			continue
		}
		if keyed[k.Stream] {
			twice[k] = true
			c.report("E-PART-004", k.Pos, "%s is keyed twice in the partition of rule %s (by %s again): key each stream on one attribute",
				k.Stream, r.Name, k.Attr)
			continue
		}
		keyed[k.Stream] = true
	}
	var first *model.PartitionKey
	for i := range keys {
		k := &keys[i]
		if c.badKey[k] || twice[k] {
			continue
		}
		if first == nil {
			first = k
			continue
		}
		ft, kt := attrOf(c.streams[first.Stream], first.Attr).Type, attrOf(c.streams[k.Stream], k.Attr).Type
		if kt != ft {
			c.report("E-PART-003", k.Pos, "partition key %s of %s is %s, but %s of %s is %s", k.Attr, k.Stream, kt, first.Attr, first.Stream, ft)
		}
	}
}

// zeroDurations: within, for, ttl, delay and output first every take a duration > 0 (`idle after` above; `allow`
// is E-LATE-002 whatever its value).
func (c *checker) zeroDurations(r *model.Rule) {
	slots := []*model.DurationOrParam{&r.OutputFirstEvery}
	if len(r.Partition.Keys) > 0 {
		slots = append(slots, &r.Partition.TTL)
	}
	switch r.Body.Kind {
	case model.BodyPattern:
		slots = append(slots, &r.Body.Pattern.Within)
		for i := range r.Body.Pattern.Elements {
			if el := &r.Body.Pattern.Elements[i]; el.Kind == model.ElemAbsence {
				slots = append(slots, &el.For)
			}
		}
	case model.BodyWindow:
		if r.Body.Window.Kind == model.WindowDelay {
			slots = append(slots, &r.Body.Window.Delay)
		}
	}
	for _, d := range slots {
		if !c.skipDur[d] && d.Ms == 0 && present(*d) {
			c.report("E-DUR-003", d.Pos, "a zero duration in a bound slot of rule %s", r.Name)
		}
	}
}

// ttl: a partition's ttl is at least every pattern bound, window extent and rate-limit interval of its rule. Each
// bound it is below is one finding at the ttl, naming both numbers. A zero ttl or bound is E-DUR-003's.
func (c *checker) ttl(r *model.Rule) {
	ttl := &r.Partition.TTL
	if len(r.Partition.Keys) == 0 || c.skipDur[ttl] || ttl.Ms == 0 {
		return
	}
	type bound struct {
		what string
		d    *model.DurationOrParam
	}
	var bs []bound
	switch r.Body.Kind {
	case model.BodyPattern:
		bs = append(bs, bound{"pattern bound `within`", &r.Body.Pattern.Within})
		for i := range r.Body.Pattern.Elements {
			if el := &r.Body.Pattern.Elements[i]; el.Kind == model.ElemAbsence {
				bs = append(bs, bound{"absence bound `for`", &el.For})
			}
		}
	case model.BodyWindow:
		// sort and batch have no time extent; cron's is its longest bucket, checked with cron (S9.1).
		if r.Body.Window.Kind == model.WindowDelay {
			bs = append(bs, bound{"window extent `delay`", &r.Body.Window.Delay})
		}
	}
	bs = append(bs, bound{"rate-limit interval `output first every`", &r.OutputFirstEvery})
	for _, b := range bs {
		if !c.skipDur[b.d] && b.d.Ms > ttl.Ms {
			c.report("E-TTL-002", ttl.Pos, "ttl %s (%d ms) of rule %s is below its %s %s (%d ms)",
				human(ttl.Ms), ttl.Ms, r.Name, b.what, human(b.d.Ms), b.d.Ms)
		}
	}
}

// emitModes are the legal (window kind, emit mode) pairs (SEMANTICS §8).
var emitModes = map[model.WindowKind]string{
	model.WindowCron: "close", model.WindowSort: "change", model.WindowDelay: "due", model.WindowBatch: "batch end",
}

func (c *checker) clauses(r *model.Rule) {
	for _, at := range r.TodoPos {
		c.report("E-TODO-001", at, "rule %s holds an unresolved todo", r.Name)
	}
	if len(r.TodoPos) == 0 {
		for range r.Todos { // a model from the Go API: no position
			c.report("E-TODO-001", model.Pos{}, "rule %s holds an unresolved todo", r.Name)
		}
	}
	late := r.Body.Pattern.LateEvents
	if r.Body.Kind == model.BodyWindow {
		late = r.Body.Window.LateEvents
	}
	if late.Mode == "allow" {
		c.report("E-LATE-002", late.Pos, "`late events allow` is not in cep/0.1: drop is the only policy")
	}
	if r.Body.Kind != model.BodyWindow {
		return
	}
	w := r.Body.Window
	if want := emitModes[w.Kind]; w.Emit != want {
		c.report("E-EMIT-002", w.EmitPos, "a %s window emits on %s, not on %s", w.Kind, want, w.Emit)
	}
	if w.Kind == model.WindowBatch {
		if s := c.streams[w.Source.Stream]; s != nil {
			if src := c.connectorOf(s, "source"); src != nil && !src.Punctuates {
				c.report("E-WIN-002", w.Source.Pos, "batch() reads %s, whose source (%s) marks no batches", s.Name, src.Name)
			}
		}
	}
}

// human writes milliseconds as the language does: `1 hour 30 min`, `250 ms`.
func human(ms int64) string {
	if ms == 0 {
		return "0 ms"
	}
	units := []struct {
		name string
		ms   int64
	}{{"week", 604_800_000}, {"day", 86_400_000}, {"hour", 3_600_000}, {"min", 60_000}, {"sec", 1000}, {"ms", 1}}
	var parts []string
	for _, u := range units {
		if n := ms / u.ms; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, u.name))
			ms -= n * u.ms
		}
	}
	return strings.Join(parts, " ")
}
