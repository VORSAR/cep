package validate

import (
	"slices"

	"github.com/vorsar/cep/internal/model"
)

// nearest is the candidate closest to name by edit distance, within max(1, len/3) edits; ties go to the earlier
// candidate (callers pass declaration order). "" when none is close enough.
func nearest(name string, candidates []string) string {
	limit := max(1, len([]rune(name))/3)
	best, bestD := "", limit+1
	for _, c := range candidates {
		if c == name {
			continue
		}
		if d := distance(name, c); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// distance is the Levenshtein distance between a and b, in runes.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// names is phase 2: every name a rule or a declaration uses resolves, and captures are unique and unshadowing.
func (c *checker) names() {
	for i := range c.app.Streams {
		c.annotations(&c.app.Streams[i])
	}
	for i := range c.app.Tables {
		t := &c.app.Tables[i]
		for _, k := range t.PrimaryKey {
			if !slices.Contains(attrNames(t.Attrs), k) {
				c.unknown(t.Pos, "primary key column of "+t.Name, k, attrNames(t.Attrs))
			}
		}
	}
	for _, r := range c.rules() {
		for _, s := range sources(r) {
			if c.streams[s.Stream] == nil {
				c.unknown(s.Pos, "stream", s.Stream, streamNames(c.app.Streams))
				c.poisoned[r] = true
			}
		}
		for i := range r.Partition.Keys {
			k := &r.Partition.Keys[i]
			switch s := c.streams[k.Stream]; {
			case s == nil:
				c.unknown(k.StreamPos, "stream", k.Stream, streamNames(c.app.Streams))
				c.badKey[k] = true
			default:
				for _, a := range k.Attrs {
					if attrOf(s, a.Name) == nil {
						c.unknown(a.Pos, "attribute of "+s.Name, a.Name, attrNames(s.Attrs))
						c.badKey[k] = true
					}
				}
			}
		}
		if c.streams[r.InsertInto] == nil {
			c.unknown(r.InsertIntoPos, "stream", r.InsertInto, streamNames(c.app.Streams))
			c.noTarget[r] = true
		}
		if w := r.Body.Window; r.Body.Kind == model.BodyWindow && w.Kind == model.WindowSort {
			if s := c.streams[w.Source.Stream]; s != nil && attrOf(s, w.SortAttr) == nil {
				c.unknown(w.SortAttrPos, "attribute of "+s.Name, w.SortAttr, attrNames(s.Attrs))
				c.poisoned[r] = true
			}
		}
		c.captures(r)
	}
}

// captures: unique within the rule (E-PAT-006) and never a declared stream, table or param name (E-NAME-002).
func (c *checker) captures(r *model.Rule) {
	if r.Body.Kind != model.BodyPattern {
		return
	}
	seen := map[string]bool{}
	for _, el := range r.Body.Pattern.Elements {
		for _, st := range el.Steps {
			switch {
			case seen[st.Name]:
				c.report("E-PAT-006", st.Pos, "capture %s is named twice in rule %s", st.Name, r.Name)
			case c.streams[st.Name] != nil:
				c.report("E-NAME-002", st.Pos, "capture %s shadows the stream %s", st.Name, st.Name)
			case c.tables[st.Name] != nil:
				c.report("E-NAME-002", st.Pos, "capture %s shadows the table %s", st.Name, st.Name)
			case c.params[st.Name] != nil:
				c.report("E-NAME-002", st.Pos, "capture %s shadows the param %s", st.Name, st.Name)
			}
			seen[st.Name] = true
		}
	}
}
