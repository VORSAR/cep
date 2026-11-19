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
	if n == m && inOrder(o, t) {
		return "", 0, 0
	}
	reason := drops(c)
	if j, i := firstMisplaced(o, t); j >= 0 {
		return fmt.Sprintf("the outputs are in another order: then-record %d matches output %d", j+1, i+1) + reason,
			t[j].Line, t[j].Col
	}
	// The alignment: a longest common subsequence under match; its first edit is the divergence. A matched prefix is
	// always paired (pairing a match never shortens the subsequence), so the walk starts where the prefix ends; the
	// first edit then needs only two subsequence lengths, computed in rows of the then-list's width (linear memory,
	// the break-it's Q-S1). Past alignCap comparisons the positional reading stands in for them.
	i := matchedPrefix(o, t)
	if thenRecordFirst(o, t, i) {
		return missingThen(target, o, t, i, reason)
	}
	return unexpectedOutput(target, x, o, i, reason)
}

// matches reports whether output i holds every value then-record j names, under §5.3's equality.
func matches(o []event.Event, t []plan.ThenRecord, i, j int) bool {
	for k, a := range t[j].Attrs {
		if !sameValue(o[i].Values[a], t[j].Values[k]) {
			return false
		}
	}
	return true
}

// inOrder reports whether every output i matches then-record i; the caller has len(o) == len(t).
func inOrder(o []event.Event, t []plan.ThenRecord) bool {
	n := len(o)
	for i := range n {
		if !matches(o, t, i, i) {
			return false
		}
	}
	return true
}

// firstMisplaced is the outputs in another order: the first then-record whose match is not its own output, and that
// output; -1, -1 unless the lists are of one length and every then-record has an output of its own.
func firstMisplaced(o []event.Event, t []plan.ThenRecord) (thenIdx, outIdx int) {
	n, m := len(o), len(t)
	if n == m { // another order: each then-record, in order, takes the first unused output it matches
		used, of := make([]bool, n), make([]int, m)
		all := true
		for j := range m {
			of[j] = -1
			for i := range n {
				if !used[i] && matches(o, t, i, j) {
					used[i], of[j] = true, i
					break
				}
			}
			all = all && of[j] >= 0
		}
		if all {
			for j := range m {
				if of[j] != j {
					return j, of[j]
				}
			}
		}
	}
	return -1, -1
}

// matchedPrefix is the length of the prefix in which each output i matches then-record i.
func matchedPrefix(o []event.Event, t []plan.ThenRecord) int {
	n, m := len(o), len(t)
	i := 0
	for i < n && i < m && matches(o, t, i, i) {
		i++
	}
	return i
}

// thenRecordFirst reports whether the alignment's first edit after the matched prefix i is then-record i (it has no
// output) rather than output i (it was not expected).
func thenRecordFirst(o []event.Event, t []plan.ThenRecord, i int) bool {
	n, m := len(o), len(t)
	switch {
	case i == m: // every then-record paired: output i is the first edit
		return false
	case i == n: // every output paired: then-record i is
		return true
	case int64(n-i)*int64(m-i) > alignCap:
		return true
	}
	// The walk's choice at (i, i): then-record i first when the subsequence without it is no shorter.
	return lcsLength(o, t, i, i+1) >= lcsLength(o, t, i+1, i)
}

// lcsLength is the LCS length of o[a:] and t[b:] under matches, computed in rows of the then-list's width.
func lcsLength(o []event.Event, t []plan.ThenRecord, a, b int) int {
	n, m := len(o), len(t)
	w := m - b
	next, cur := make([]int, w+1), make([]int, w+1)
	for r := n - 1; r >= a; r-- {
		cur[w] = 0
		for k := w - 1; k >= 0; k-- {
			best := max(next[k], cur[k+1])
			if matches(o, t, r, b+k) {
				best = max(best, 1+next[k+1])
			}
			cur[k] = best
		}
		next, cur = cur, next
	}
	return next[0]
}

// missingThen is the divergence at then-record i, which has no output: its text with the nearest output, and the
// then-record's position.
func missingThen(target event.Schema, o []event.Event, t []plan.ThenRecord, i int, reason string) (string, int, int) {
	n := len(o)
	text := fmt.Sprintf("then-record %d has no output: expected %s", i+1, thenText(target, t[i]))
	if near := nearest(target, o, i, t[i]); near != "" {
		text += "; " + near
	} else {
		text += fmt.Sprintf("; the rule emitted %d", n)
	}
	return text + reason, t[i].Line, t[i].Col
}

// unexpectedOutput is the divergence at output i, which was not expected: its text, and the then-list's position.
func unexpectedOutput(target event.Schema, x *plan.Expect, o []event.Event, i int, reason string) (string, int, int) {
	m := len(x.Then)
	tail := fmt.Sprintf("; the then-list has %d", m)
	if m == 0 {
		tail = "; the then-list is empty"
	}
	return fmt.Sprintf("output %d was not expected: %s%s", i+1, outputText(target, o[i]), tail) + reason, x.ThenLine,
		x.ThenCol
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
	return samePayload(a, b)
}

// samePayload is the equality of two non-null values of one kind: the same value; a float also by its sign.
func samePayload(a, b event.Value) bool {
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
	case event.TObject:
		return sameObject(a, b)
	}
	return false
}

// sameObject is the equality of two objects, both in event.CleanObject's form: the same text is the same object
// (owner, 2026-11-10).
func sameObject(a, b event.Value) bool {
	sa, oka := a.Obj.(string)
	sb, okb := b.Obj.(string)
	return oka && okb && sa == sb
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
