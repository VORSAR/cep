package validate

import (
	"fmt"
	"github.com/vorsar/cep/internal/event"
	"strings"

	"github.com/vorsar/cep/internal/model"
)

// bounds is phase 5: partitions (SEMANTICS §7, §9; SPEC §4.4), every bound slot's duration (SPEC §2.4), TTL against
// the bounds it must cover (SEMANTICS §9, §11.3), and the window and lateness clauses.
func (c *checker) bounds() {
	for i := range c.app.Streams {
		if t := c.app.Streams[i].Time; t.IdleAfter < 0 || t.IdleAfter == 0 && t.IdlePos != (model.Pos{}) {
			c.report("E-DUR-003", t.IdlePos, "`idle after` of %s is %s", c.app.Streams[i].Name, notPositive(t.IdleAfter))
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
	if c.anyBadKey(keys) {
		return // a key already reported may have been meant for any stream the rule reads
	}
	c.unkeyedReads(r)
	// The spelling and the shape, in design s3-2b-2 v2 §1.3's order: a group naming an attribute twice (E-PART-006), a
	// stream keyed twice (E-PART-004), then — over the keys left — the part count (E-PART-007, once, at `partition`)
	// before the bracket spelling (E-PART-005), so the first fix offered settles the clause (review F12); then the types,
	// position by position (E-PART-003).
	live := c.liveKeys(r)
	if len(live) == 0 || c.partCounts(r, live) {
		return
	}
	c.bracketedSingles(r, live)
	c.keyTypes(r, live)
	c.objectKeys(r, live)
}

// anyBadKey says whether names reported a key of keys, the rule's own slice (badKey holds pointers into it).
func (c *checker) anyBadKey(keys []model.PartitionKey) bool {
	for i := range keys {
		if c.badKey[&keys[i]] {
			return true
		}
	}
	return false
}

// unkeyedReads: the partition keys every stream r reads (E-PART-002).
func (c *checker) unkeyedReads(r *model.Rule) {
	keys := r.Partition.Keys
	for _, name := range read(r) {
		keyed := false
		for _, k := range keys {
			keyed = keyed || k.Stream == name
		}
		if !keyed {
			c.report("E-PART-002", r.Partition.Pos, "the partition of rule %s keys no attribute of %s, which the rule reads",
				r.Name, name)
		}
	}
}

// liveKeys is r's partition keys, in written order, less those naming an attribute twice or keying a stream again.
func (c *checker) liveKeys(r *model.Rule) []*model.PartitionKey {
	keys := r.Partition.Keys
	bad := map[*model.PartitionKey]bool{}
	c.repeatedAttrs(r, bad)
	c.repeatedStreams(r, bad)
	var live []*model.PartitionKey
	for i := range keys {
		if !bad[&keys[i]] {
			live = append(live, &keys[i])
		}
	}
	return live
}

// repeatedAttrs: a key names each attribute once (E-PART-006, at its first repeat); such a key is bad.
func (c *checker) repeatedAttrs(r *model.Rule, bad map[*model.PartitionKey]bool) {
	keys := r.Partition.Keys
	for i := range keys {
		k := &keys[i]
		seen := map[string]bool{}
		for _, a := range k.Attrs {
			if seen[a.Name] {
				c.report("E-PART-006", a.Pos, "%s is named twice in the partition key of %s in rule %s: name each attribute once",
					a.Name, k.Stream, r.Name)
				bad[k] = true
				break
			}
			seen[a.Name] = true
		}
	}
}

// repeatedStreams: a stream is keyed once (E-PART-004 at each later key); each later key is bad.
func (c *checker) repeatedStreams(r *model.Rule, bad map[*model.PartitionKey]bool) {
	keys := r.Partition.Keys
	first := map[string]*model.PartitionKey{} // a stream keyed twice: never drop a declared key (break-it B1 of S3.2b-1)
	for i := range keys {
		k := &keys[i]
		f, twice := first[k.Stream]
		if !twice {
			first[k.Stream] = k
			continue
		}
		bad[k] = true
		c.report("E-PART-004", k.Pos,
			"%s is keyed twice in the partition of rule %s: to key it on both, write (%s) of %s, and give every stream the "+
				"same number of parts",
			k.Stream, r.Name, strings.Join(append(attrNamesOf(f), attrNamesOf(k)...), ", "), k.Stream)
	}
}

// partCounts: the live keys have one number of parts (E-PART-007, once). It reports whether it found a difference.
func (c *checker) partCounts(r *model.Rule, live []*model.PartitionKey) bool {
	for _, k := range live[1:] {
		if len(k.Attrs) != len(live[0].Attrs) {
			counts := make([]string, len(live))
			for i, k := range live {
				counts[i] = fmt.Sprintf("%s %d", k.Stream, len(k.Attrs))
			}
			c.report("E-PART-007", r.Partition.Pos,
				"the partition keys of rule %s have different numbers of parts: %s: give every stream the same number of parts",
				r.Name, strings.Join(counts, ", "))
			return true
		}
	}
	return false
}

// bracketedSingles: a one-attribute key is written without brackets (E-PART-005).
func (c *checker) bracketedSingles(r *model.Rule, live []*model.PartitionKey) {
	if len(live[0].Attrs) == 1 {
		for _, k := range live {
			if k.Grouped {
				c.report("E-PART-005", k.Pos, "the partition key of %s in rule %s is one attribute in brackets: write %s of %s",
					k.Stream, r.Name, k.Attrs[0].Name, k.Stream)
			}
		}
	}
}

// keyTypes: each live key's parts have the first key's types, position by position (E-PART-003).
func (c *checker) keyTypes(r *model.Rule, live []*model.PartitionKey) {
	ref := live[0]
	for _, k := range live[1:] {
		for i, a := range k.Attrs {
			ra := ref.Attrs[i]
			ft, kt := attrOf(c.streams[ref.Stream], ra.Name).Type, attrOf(c.streams[k.Stream], a.Name).Type
			if kt == ft {
				continue
			}
			if len(k.Attrs) == 1 {
				c.report("E-PART-003", k.Pos, "partition key %s of %s is %s, but %s of %s is %s", a.Name, k.Stream, kt, ra.Name,
					ref.Stream, ft)
			} else {
				c.report("E-PART-003", a.Pos, "part %d of the partition key: %s of %s is %s, but %s of %s is %s", i+1, a.Name,
					k.Stream, kt, ra.Name, ref.Stream, ft)
			}
			break // one finding per key: its first differing part
		}
	}
}

// objectKeys is E-PART-008 (S3.2a-1's D7; SPEC rev 7 §4.4): a key part is a scalar; an object cannot key a partition.
func (c *checker) objectKeys(r *model.Rule, live []*model.PartitionKey) {
	for _, k := range live {
		for i, a := range k.Attrs {
			if attrOf(c.streams[k.Stream], a.Name).Type != event.TObject {
				continue
			}
			if len(k.Attrs) == 1 {
				c.report("E-PART-008", k.Pos, "partition key %s of %s is an object: an object cannot key a partition", a.Name,
					k.Stream)
			} else {
				c.report("E-PART-008", a.Pos,
					"part %d of the partition key: %s of %s is an object: an object cannot key a partition", i+1, a.Name, k.Stream)
			}
			break
		}
	}
}

// attrNamesOf is a key's attribute names, in written order.
func attrNamesOf(k *model.PartitionKey) []string {
	out := make([]string, len(k.Attrs))
	for i, a := range k.Attrs {
		out[i] = a.Name
	}
	return out
}

// zeroDurations: within, for, ttl, delay and output first every take a duration > 0 (`idle after` above; `allow`
// is E-LATE-002 whatever its value). Source writes no negative duration; an override or a model built in Go can
// (S3.2c break-it K1).
func (c *checker) zeroDurations(r *model.Rule) {
	for _, d := range boundSlots(r) {
		if !c.skipDur[d] && (d.Ms < 0 || d.Ms == 0 && present(*d)) {
			c.report("E-DUR-003", d.Pos, "a bound slot of rule %s is %s", r.Name, notPositive(d.Ms))
		}
	}
}

// boundSlots is every bound slot r has: output first every, the ttl of a partition, a pattern's within and each
// absence's for, a delay window's delay.
func boundSlots(r *model.Rule) []*model.DurationOrParam {
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
	return slots
}

// ttl: a partition's ttl is at least every pattern bound, window extent and rate-limit interval of its rule. Each
// bound it is below is one finding at the ttl, naming both numbers. A zero or negative ttl or bound is E-DUR-003's.
func (c *checker) ttl(r *model.Rule) {
	ttl := &r.Partition.TTL
	if len(r.Partition.Keys) == 0 || c.skipDur[ttl] || ttl.Ms <= 0 {
		return
	}
	for _, b := range ttlBounds(r) {
		if !c.skipDur[b.d] && b.d.Ms > ttl.Ms {
			c.report("E-TTL-002", ttl.Pos, "ttl %s (%d ms) of rule %s is below its %s %s (%d ms)",
				human(ttl.Ms), ttl.Ms, r.Name, b.what, human(b.d.Ms), b.d.Ms)
		}
	}
}

// ttlBound is a bound a partition's ttl must cover, named for the finding.
type ttlBound struct {
	what string
	d    *model.DurationOrParam
}

// ttlBounds is every bound of r its ttl must cover, the rate-limit interval last.
func ttlBounds(r *model.Rule) []ttlBound {
	var bs []ttlBound
	switch r.Body.Kind {
	case model.BodyPattern:
		bs = append(bs, ttlBound{"pattern bound `within`", &r.Body.Pattern.Within})
		for i := range r.Body.Pattern.Elements {
			if el := &r.Body.Pattern.Elements[i]; el.Kind == model.ElemAbsence {
				bs = append(bs, ttlBound{"absence bound `for`", &el.For})
			}
		}
	case model.BodyWindow:
		// sort and batch have no time extent; cron's is its longest bucket, checked with cron (S9.1).
		if r.Body.Window.Kind == model.WindowDelay {
			bs = append(bs, ttlBound{"window extent `delay`", &r.Body.Window.Delay})
		}
	}
	bs = append(bs, ttlBound{"rate-limit interval `output first every`", &r.OutputFirstEvery})
	return bs
}

// emitModes are the legal (window kind, emit mode) pairs (SEMANTICS §8).
var emitModes = map[model.WindowKind]string{
	model.WindowCron: "close", model.WindowSort: "change", model.WindowDelay: "due", model.WindowBatch: "batch end",
}

// clauses: the todos, the lateness policy, then a window's emit mode and source.
func (c *checker) clauses(r *model.Rule) {
	c.todos(r)
	c.lateness(r)
	c.windowClauses(r)
}

// todos: a rule holds no unresolved todo (E-TODO-001 at each).
func (c *checker) todos(r *model.Rule) {
	for _, at := range r.TodoPos {
		c.report("E-TODO-001", at, "rule %s holds an unresolved todo", r.Name)
	}
	if len(r.TodoPos) == 0 {
		for range r.Todos { // a model from the Go API: no position
			c.report("E-TODO-001", model.Pos{}, "rule %s holds an unresolved todo", r.Name)
		}
	}
}

// lateness: drop is the only late-events policy (E-LATE-002).
func (c *checker) lateness(r *model.Rule) {
	late := r.Body.Pattern.LateEvents
	if r.Body.Kind == model.BodyWindow {
		late = r.Body.Window.LateEvents
	}
	if late.Mode == "allow" {
		c.report("E-LATE-002", late.Pos, "`late events allow` is not in cep/0.1: drop is the only policy")
	}
}

// windowClauses: a window emits in its kind's mode (E-EMIT-002), and batch() reads a punctuating source (E-WIN-002).
func (c *checker) windowClauses(r *model.Rule) {
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

// notPositive names a bound's duration that is not > 0, for E-DUR-003.
func notPositive(ms int64) string {
	if ms == 0 {
		return "zero"
	}
	return fmt.Sprintf("negative (%d ms)", ms)
}
