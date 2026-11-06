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
	graphFound := false // an E-GRAPH-002 or E-GRAPH-004: every v0.1 cycle has one, so E-GRAPH-001 stays silent
	for i := range c.app.Streams {
		s := &c.app.Streams[i]
		timed, prod := s.Time.Kind != model.TimeNone, producers[s.Name]
		switch {
		case !timed && len(prod) == 0:
			c.report("E-TIME-001", s.Pos, "no rule produces %s, so it is a source stream, and it has no time clause", s.Name)
		case timed && len(prod) > 0 && (readers[s.Name] || hasAnnotation(s, "source")):
			graphFound = true
			for _, r := range prod {
				c.report("E-GRAPH-002", r.InsertIntoPos, "rule %s inserts into %s, a source stream (it has a time clause and is read)", r.Name, s.Name)
			}
		case timed && len(prod) > 0:
			c.report("E-TIME-002", s.Time.Pos, "%s is only the output of rule %s, so it is derived and takes no time clause", s.Name, prod[0].Name)
		case !timed && len(prod) > 1:
			for _, r := range prod[1:] {
				c.report("E-GRAPH-003", r.InsertIntoPos, "rule %s is a second producer of %s, after rule %s", r.Name, s.Name, prod[0].Name)
			}
		}
		c.timeAttr(s)
		c.invisiblesDecl(s)
	}
	derived := func(name string) *model.Rule {
		if s := c.streams[name]; s != nil && s.Time.Kind == model.TimeNone && len(producers[name]) > 0 {
			return producers[name][0]
		}
		return nil
	}
	for _, r := range c.rules() {
		if c.poisoned[r] {
			continue
		}
		// One finding per rule and derived stream: at its first read in the body, else at its partition key.
		refs := []struct {
			name string
			at   model.Pos
		}{}
		for _, s := range sources(r) {
			refs = append(refs, struct {
				name string
				at   model.Pos
			}{s.Stream, s.Pos})
		}
		for _, k := range r.Partition.Keys {
			refs = append(refs, struct {
				name string
				at   model.Pos
			}{k.Stream, k.StreamPos})
		}
		var seen []string
		for _, ref := range refs {
			if p := derived(ref.name); p != nil && !slices.Contains(seen, ref.name) {
				seen = append(seen, ref.name)
				c.report("E-GRAPH-004", ref.at, "rule %s reads %s, the output of rule %s: v0.1 rules read source streams only", r.Name, ref.name, p.Name)
				graphFound = true
			}
		}
	}
	// E-GRAPH-001 cannot be reached in v0.1: a cycle passes through a stream some rule both reads and inserts into,
	// which is E-GRAPH-004 when it is derived and E-GRAPH-002 when it has a time clause. The search stays for
	// Phase 2's rule chaining, and reports only what nothing else explains.
	if cyc := cycle(c.app.Rules); len(cyc) > 0 && !graphFound {
		c.report("E-GRAPH-001", c.app.Rules[cyc[0]].InsertIntoPos, "rules %s form a cycle", strings.Join(ruleNames(c.app.Rules, cyc), " -> "))
	}
	c.idle()
}

// invisiblesDecl: a stream may declare `text_invisibles` only as a non-optional bool, the engine's flag's type; any
// other declaration is E-TYPE-001 at it (S3.2b's Q3), since every read of the name returns the flag.
func (c *checker) invisiblesDecl(s *model.Stream) {
	a := attrOf(s, event.InvisiblesAttr)
	switch {
	case a == nil:
	case a.Type != event.TBool:
		c.report("E-TYPE-001", a.Pos, "%s.%s is the engine's invisible-character flag: declare it bool, not %s", s.Name, a.Name, a.Type)
	case a.Optional:
		c.report("E-TYPE-001", a.Pos, "%s.%s is the engine's invisible-character flag, never null: declare it bool without optional", s.Name, a.Name)
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
		c.report("E-TIME-003", s.Time.AttrPos, "the time attribute %s of %s is declared %s, not timestamp", a.Name, s.Name, a.Type)
	}
}

// cycle finds a cycle in the rule graph (rule p feeds rule r when r reads p's output), as rule indexes in order,
// starting from the earliest rule on it; nil when there is none. The walk follows declaration order throughout.
func cycle(rules []model.Rule) []int {
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
		why := ""
		streams := read(r)
		switch {
		case r.Body.Kind == model.BodyWindow:
			why = "a window"
		case len(streams) > 1:
			why = "it reads " + strings.Join(streams, " and ")
		case r.Body.Kind == model.BodyPattern && slices.ContainsFunc(r.Body.Pattern.Elements, func(e model.PatternElement) bool {
			return e.Kind == model.ElemAbsence
		}):
			why = "an absence"
		default:
			continue
		}
		for _, name := range streams {
			s := c.streams[name]
			if s.Time.Kind == model.TimeNone || s.Time.IdleAfter != 0 || s.Time.IdlePos != (model.Pos{}) || reported[name] {
				continue
			}
			reported[name] = true
			c.report("E-IDLE-001", s.Time.Pos, "%s feeds rule %s, which holds timers (%s), and declares no `idle after`", name, r.Name, why)
		}
	}
}
