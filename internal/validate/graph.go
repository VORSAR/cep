package validate

import (
	"slices"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// graph is phase 4: the rule–stream graph and the time clauses (SPEC §3.3, §4.2; owner A2, 2026-11-04). A stream
// is classified two ways. By its time clause: a stream with one is a source and never derived (E-GRAPH-002,
// E-GRAPH-004, E-IDLE-001). By its producers: a stream no rule inserts into is a source, one with no time clause
// and a producer is derived (E-TIME-001, E-TIME-002, E-GRAPH-003).
func (c *checker) graph() {
	producers, readers := c.producersAndReaders()
	found := c.streamClasses(producers, readers)
	derived := c.derivedReads(producers)
	c.unexplainedCycle(found || derived)
	c.idle()
}

// producersAndReaders: the rules inserting into each stream (a rule with no target produces nothing), and every
// stream a rule reads, in its body or as a partition key.
func (c *checker) producersAndReaders() (map[string][]*model.Rule, map[string]bool) {
	producers := map[string][]*model.Rule{}
	readers := map[string]bool{}
	for _, r := range c.rules() {
		if !c.noTarget[r] {
			producers[r.InsertInto] = append(producers[r.InsertInto], r)
		}
		for _, s := range sources(r) {
			readers[s.Stream] = true
		}
		for _, k := range r.Partition.Keys {
			readers[k.Stream] = true
		}
	}
	return producers, readers
}

// streamClasses classifies every stream, then checks its time attribute and its text_invisibles declaration. It
// reports whether any stream was an E-GRAPH-002; every stream is checked whatever an earlier one found.
func (c *checker) streamClasses(producers map[string][]*model.Rule, readers map[string]bool) bool {
	found := false
	for i := range c.app.Streams {
		s := &c.app.Streams[i]
		if c.classify(s, producers[s.Name], readers[s.Name]) {
			found = true
		}
		c.timeAttr(s)
		c.invisiblesDecl(s)
	}
	return found
}

// classify checks s by its time clause and its producers prod; read says a rule reads s. It reports whether s is
// an E-GRAPH-002.
func (c *checker) classify(s *model.Stream, prod []*model.Rule, read bool) bool {
	if s.Time.Kind == model.TimeNone {
		c.untimedStream(s, prod)
		return false
	}
	return c.timedStream(s, prod, read)
}

// untimedStream: a stream with no time clause needs a producer (else it is a source, E-TIME-001), and only one
// (E-GRAPH-003 at each later one).
func (c *checker) untimedStream(s *model.Stream, prod []*model.Rule) {
	switch {
	case len(prod) == 0:
		c.report("E-TIME-001", s.Pos, "no rule produces %s, so it is a source stream, and it has no time clause", s.Name)
	case len(prod) > 1:
		for _, r := range prod[1:] {
			c.report("E-GRAPH-003", r.InsertIntoPos, "rule %s is a second producer of %s, after rule %s", r.Name, s.Name,
				prod[0].Name)
		}
	}
}

// timedStream: a stream with a time clause and a producer is a source a rule writes when it is read or a @source
// (E-GRAPH-002 at each producer, and true), else derived with a time clause it does not take (E-TIME-002).
func (c *checker) timedStream(s *model.Stream, prod []*model.Rule, read bool) bool {
	if len(prod) == 0 {
		return false
	}
	if read || hasAnnotation(s, "source") {
		for _, r := range prod {
			c.report("E-GRAPH-002", r.InsertIntoPos,
				"rule %s inserts into %s, a source stream (it has a time clause and is read)", r.Name, s.Name)
		}
		return true
	}
	c.report("E-TIME-002", s.Time.Pos, "%s is only the output of rule %s, so it is derived and takes no time clause",
		s.Name, prod[0].Name)
	return false
}

// streamRef is a stream a rule reads, at the place it names it.
type streamRef struct {
	name string
	at   model.Pos
}

// streamRefs: every stream r reads, its body's sources first, then its partition keys'.
func streamRefs(r *model.Rule) []streamRef {
	refs := []streamRef{}
	for _, s := range sources(r) {
		refs = append(refs, streamRef{s.Stream, s.Pos})
	}
	for _, k := range r.Partition.Keys {
		refs = append(refs, streamRef{k.Stream, k.StreamPos})
	}
	return refs
}

// producerOf is the rule a derived stream (no time clause, a producer) is the output of; nil for any other name.
func (c *checker) producerOf(name string, producers map[string][]*model.Rule) *model.Rule {
	if s := c.streams[name]; s != nil && s.Time.Kind == model.TimeNone && len(producers[name]) > 0 {
		return producers[name][0]
	}
	return nil
}

// derivedReads: a rule reads source streams only (E-GRAPH-004). It reports whether it found one; every rule is
// checked whatever an earlier one found, a poisoned one too (producerOf ignores an unknown stream).
func (c *checker) derivedReads(producers map[string][]*model.Rule) bool {
	found := false
	for _, r := range c.rules() {
		// One finding per rule and derived stream: at its first read in the body, else at its partition key.
		var seen []string
		for _, ref := range streamRefs(r) {
			if p := c.producerOf(ref.name, producers); p != nil && !slices.Contains(seen, ref.name) {
				seen = append(seen, ref.name)
				c.report("E-GRAPH-004", ref.at, "rule %s reads %s, the output of rule %s: v0.1 rules read source streams only",
					r.Name, ref.name, p.Name)
				found = true
			}
		}
	}
	return found
}

// unexplainedCycle reports a cycle in the rule graph unless graphFound (an E-GRAPH-002 or E-GRAPH-004 explains it)
// or a rule on it has a name finding.
func (c *checker) unexplainedCycle(graphFound bool) {
	// E-GRAPH-001 reports a cycle nothing else explained. It is unreachable in v0.1 (SPEC §3.3): a cycle over declared
	// streams passes through a rule reading a derived stream, which derivedReads reports as E-GRAPH-004, and a cycle
	// through an undeclared stream passes through a rule with a name finding. It stays for Phase 2's rule chaining.
	if cyc := cycle(c.app.Rules); len(cyc) > 0 && !graphFound && !c.namedFault(cyc) {
		c.report("E-GRAPH-001", c.app.Rules[cyc[0]].InsertIntoPos, "rules %s form a cycle", strings.Join(ruleNames(
			c.app.Rules, cyc), " -> "))
	}
}

// namedFault says whether a rule of cyc (indexes into the rules) has a name finding already reported: an unknown
// source stream (poisoned) or an unknown target (noTarget) (owner, 2026-11-16, S1.1c-6 lane V, decision 3).
func (c *checker) namedFault(cyc []int) bool {
	return slices.ContainsFunc(cyc, func(i int) bool {
		r := &c.app.Rules[i]
		return c.poisoned[r] || c.noTarget[r]
	})
}

// invisiblesDecl: a stream may declare `text_invisibles` only as a non-optional bool, the engine's flag's type; any
// other declaration is E-TYPE-001 at it (S3.2b's Q3), since every read of the name returns the flag.
func (c *checker) invisiblesDecl(s *model.Stream) {
	a := attrOf(s, event.InvisiblesAttr)
	switch {
	case a == nil:
	case a.Type != event.TBool:
		c.report("E-TYPE-001", a.Pos, "%s.%s is the engine's invisible-character flag: declare it bool, not %s", s.Name,
			a.Name, a.Type)
	case a.Optional:
		c.report("E-TYPE-001", a.Pos,
			"%s.%s is the engine's invisible-character flag, never null: declare it bool without optional", s.Name, a.Name)
	}
}

func (c *checker) timeAttr(s *model.Stream) {
	if s.Time.Kind != model.TimeBounded && s.Time.Kind != model.TimeAscending {
		return
	}
	switch a := attrOf(s, s.Time.Attr); {
	case a == nil:
		c.unknown(s.Time.AttrPos, "attribute of "+s.Name, s.Time.Attr, attrNames(s.Attrs))
	case a.Type != event.TTimestamp:
		c.report("E-TIME-003", s.Time.AttrPos, "the time attribute %s of %s is declared %s, not timestamp", a.Name, s.Name,
			a.Type)
	case a.Optional: // owner, 2026-11-09, design 7: live ingest refuses an event without a time, whatever optional says
		c.report("E-TIME-003", s.Time.AttrPos, "the time attribute %s of %s is declared optional: every event needs a time",
			a.Name, s.Name)
	}
}

// cycle finds a cycle in the rule graph (rule p feeds rule r when r reads p's output), as rule indexes in order,
// starting from the earliest rule on it; nil when there is none. The walk follows declaration order throughout.
func cycle(rules []model.Rule) []int {
	feeds := feedEdges(rules)
	const (
		unseen = iota
		open
		done
	)
	state := make([]int, len(rules))
	var path []int
	var visit func(int) []int
	visit = func(i int) []int {
		state[i] = open
		path = append(path, i)
		for _, j := range feeds[i] {
			if state[j] == open {
				return slices.Clone(path[slices.Index(path, j):])
			}
			if state[j] == unseen {
				if found := visit(j); found != nil {
					return found
				}
			}
		}
		state[i] = done
		path = path[:len(path)-1]
		return nil
	}
	for i := range rules {
		if state[i] == unseen {
			if found := visit(i); found != nil {
				return found
			}
		}
	}
	return nil
}

// feedEdges: for each rule p, the rules that read its output, each once, in declaration order.
func feedEdges(rules []model.Rule) [][]int {
	feeds := make([][]int, len(rules))
	for p := range rules {
		for r := range rules {
			for _, s := range read(&rules[r]) {
				if s == rules[p].InsertInto && !slices.Contains(feeds[p], r) {
					feeds[p] = append(feeds[p], r)
				}
			}
		}
	}
	return feeds
}

func ruleNames(rules []model.Rule, idx []int) []string {
	out := make([]string, len(idx)+1)
	for i, j := range idx {
		out[i] = rules[j].Name
	}
	out[len(idx)] = rules[idx[0]].Name
	return out
}

// idle: a stream with a time clause that feeds a timer-holding rule — a pattern with an absence, any window rule,
// any rule reading two streams or more — declares `idle after` (E-IDLE-001, at its `time`, once per stream).
func (c *checker) idle() {
	reported := map[string]bool{}
	for _, r := range c.rules() {
		if c.poisoned[r] {
			continue
		}
		streams := read(r)
		why := timerReason(r, streams)
		if why == "" {
			continue
		}
		for _, name := range streams {
			s := c.streams[name]
			if s.Time.Kind == model.TimeNone || s.Time.IdleAfter != 0 || s.Time.IdlePos != (model.Pos{}) || reported[name] {
				continue
			}
			reported[name] = true
			c.report("E-IDLE-001", s.Time.Pos, "%s feeds rule %s, which holds timers (%s), and declares no `idle after`", name,
				r.Name, why)
		}
	}
}

// timerReason says why rule r, reading streams, holds timers; "" when it holds none.
func timerReason(r *model.Rule, streams []string) string {
	switch {
	case r.Body.Kind == model.BodyWindow:
		return "a window"
	case len(streams) > 1:
		return "it reads " + strings.Join(streams, " and ")
	case r.Body.Kind == model.BodyPattern && slices.ContainsFunc(r.Body.Pattern.Elements, func(
		e model.PatternElement) bool {
		return e.Kind == model.ElemAbsence
	}):
		return "an absence"
	}
	return ""
}
