package playback

import (
	"fmt"
	"math"
	"strings"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

// diverge is §5.3's verdict and §5.4's first divergence with its position; "" and 0, 0 when the block passed.
func diverge(target event.Schema, x *plan.Expect, o []event.Event, c Counts) (string, int, int) {
	t := x.Then
	n, m := len(o), len(t)
	if c.Panics > 0 {
		return fmt.Sprintf("the rule panicked %d time(s)", c.Panics), x.Line, x.Col
	}
	match := func(i, j int) bool {
		for k, a := range t[j].Attrs {
			if !sameValue(o[i].Values[a], t[j].Values[k]) {
				return false
			}
		}
		return true
	}
	if n == m && func() bool {
		for i := range n {
			if !match(i, i) {
				return false
			}
		}
		return true
	}() {
		return "", 0, 0
	}
	reason := drops(c)
	if n == m { // another order: each then-record, in order, takes the first unused output it matches
		used, of := make([]bool, n), make([]int, m)
		all := true
		for j := range m {
			of[j] = -1
			for i := range n {
				if !used[i] && match(i, j) {
					used[i], of[j] = true, i
					break
				}
			}
			all = all && of[j] >= 0
		}
		if all {
			for j := range m {
				if of[j] != j {
					return fmt.Sprintf("the outputs are in another order: then-record %d matches output %d", j+1, of[j]+1) + reason,
						t[j].Line, t[j].Col
				}
			}
		}
	}
	// The alignment: a longest common subsequence under match; its first edit is the divergence. A matched prefix is
	// always paired (pairing a match never shortens the subsequence), so the walk starts where the prefix ends; the
	// first edit then needs only two subsequence lengths, computed in rows of the then-list's width (linear memory,
	// the break-it's Q-S1). Past alignCap comparisons the positional reading stands in for them.
	i := 0
	for i < n && i < m && match(i, i) {
		i++
	}
	thenFirst := func() (string, int, int) {
		text := fmt.Sprintf("then-record %d has no output: expected %s", i+1, thenText(target, t[i]))
		if near := nearest(target, o, i, t[i]); near != "" {
			text += "; " + near
		} else {
			text += fmt.Sprintf("; the rule emitted %d", n)
		}
		return text + reason, t[i].Line, t[i].Col
	}
	outputFirst := func() (string, int, int) {
		tail := fmt.Sprintf("; the then-list has %d", m)
		if m == 0 {
			tail = "; the then-list is empty"
		}
		return fmt.Sprintf("output %d was not expected: %s%s", i+1, outputText(target, o[i]), tail) + reason, x.ThenLine,
			x.ThenCol
	}
	switch {
	case i == m: // every then-record paired: output i is the first edit
		return outputFirst()
	case i == n: // every output paired: then-record i is
		return thenFirst()
	case int64(n-i)*int64(m-i) > alignCap:
		return thenFirst()
	}
	// The walk's choice at (i, i): then-record i first when the subsequence without it is no shorter.
	lcs := func(a, b int) int { // the LCS length of o[a:] and t[b:]
		w := m - b
		next, cur := make([]int, w+1), make([]int, w+1)
		for r := n - 1; r >= a; r-- {
			cur[w] = 0
			for k := w - 1; k >= 0; k-- {
				best := max(next[k], cur[k+1])
				if match(r, b+k) {
					best = max(best, 1+next[k+1])
				}
				cur[k] = best
			}
			next, cur = cur, next
		}
		return next[0]
	}
	if lcs(i, i+1) >= lcs(i+1, i) {
		return thenFirst()
	}
	return outputFirst()
}

// alignCap bounds the comparisons the alignment makes for one block (a block of thousands of records each way);
// past it the first divergence is read by position.
const alignCap = 4_000_000

// nearest is "the nearest output is <i>: <a> is <actual>, expected <expected>" — the first attribute, in the
// then-record's written order, whose value differs from output i's; "" when there is no output i or none differs.
func nearest(target event.Schema, o []event.Event, i int, r plan.ThenRecord) string {
	if i >= len(o) {
		return ""
	}
	for k, a := range r.Attrs {
		if got := o[i].Values[a]; !sameValue(got, r.Values[k]) {
			return fmt.Sprintf("the nearest output is %d: %s is %s, expected %s", i+1, target.Attr(a).Name, canon(got), canon(
				r.Values[k]))
		}
	}
	return ""
}

// drops is the reason a divergence ends with (§5.4 case 4): each non-zero count, in a fixed order.
func drops(c Counts) string {
	var b strings.Builder
	if c.Late > 0 {
		fmt.Fprintf(&b, " (%d given event(s) dropped as stream-late)", c.Late)
	}
	if c.Errored > 0 {
		fmt.Fprintf(&b, " (%d dropped by an expression error)", c.Errored)
	}
	if c.KeyErrors > 0 {
		fmt.Fprintf(&b, " (%d dropped by a key error)", c.KeyErrors)
	}
	return b.String()
}

// sameValue is §5.3's equality: both null, or the same kind and value; a float also by its sign (−0.0 ≠ +0.0).
func sameValue(a, b event.Value) bool {
	if a.Null || b.Null {
		return a.Null && b.Null
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case event.TString:
		return a.Str == b.Str
	case event.TInt:
		return a.I == b.I
	case event.TFloat:
		return a.F == b.F && math.Signbit(a.F) == math.Signbit(b.F)
	case event.TBool:
		return a.B == b.B
	case event.TTimestamp:
		return a.T == b.T
	case event.TDuration:
		return a.D == b.D
	case event.TObject: // both in event.CleanObject's form: the same text is the same object (owner, 2026-11-10)
		sa, oka := a.Obj.(string)
		sb, okb := b.Obj.(string)
		return oka && okb && sa == sb
	}
	return false
}

// canon is a value's canonical text; a value AppendValue refuses is printed for what it is: a non-finite float with
// %v, a timestamp outside the years 0000-9999 by its milliseconds (the combined review's R-L4).
func canon(v event.Value) string {
	b, err := AppendValue(nil, v)
	if err == nil {
		return string(b)
	}
	switch v.Kind {
	case event.TFloat:
		return fmt.Sprintf("%v", v.F)
	case event.TTimestamp:
		return fmt.Sprintf("%d ms (outside the years 0000-9999)", v.T)
	}
	return fmt.Sprintf("%v", v)
}

func thenText(target event.Schema, r plan.ThenRecord) string {
	parts := make([]string, len(r.Attrs))
	for k, a := range r.Attrs {
		parts[k] = target.Attr(a).Name + ": " + canon(r.Values[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func outputText(target event.Schema, e event.Event) string {
	parts := make([]string, len(e.Values))
	for i, v := range e.Values {
		parts[i] = target.Attr(i).Name + ": " + canon(v)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
