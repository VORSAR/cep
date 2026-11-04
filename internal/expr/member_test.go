package expr_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/expr"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/table"
)

// ---- membership (design note §2; SEMANTICS §1.1, §4; SPEC §5.6) ----

// mScope: one event (slot 0) whose attributes are h, g (string), p (int), fl (float), hn (nullable string), bo (bool),
// tm (timestamp), du (duration); and six single-key tables, one per key type, whose ids are their positions.
type mScope struct{}

var mAttrs = []struct {
	name string
	typ  event.Type
}{
	{"h", event.TString}, {"g", event.TString}, {"p", event.TInt}, {"fl", event.TFloat},
	{"hn", event.TString}, {"bo", event.TBool}, {"tm", event.TTimestamp}, {"du", event.TDuration},
}

var mTables = []struct {
	name string
	key  event.Type
}{
	{"Hosts", event.TString}, {"Ports", event.TInt}, {"Scores", event.TFloat},
	{"Flags", event.TBool}, {"Times", event.TTimestamp}, {"Spans", event.TDuration},
}

func (mScope) Attr(q, n string) (int, int, event.Type, bool) {
	if q != "" {
		return 0, 0, event.TInvalid, false
	}
	for i, a := range mAttrs {
		if a.name == n {
			return 0, i, a.typ, true
		}
	}
	return 0, 0, event.TInvalid, false
}

func (mScope) Capture(string) (int, bool) { return 0, false }

func (mScope) Table(name string) (int, event.Type, bool) {
	for id, tb := range mTables {
		if tb.name == name {
			return id, tb.key, true
		}
	}
	return 0, event.TInvalid, false
}

// pinned is a row pinned to its snapshots, as the runtime's row is (design note §2, owner D1). onValue, when set,
// runs inside every Value call: the test's way to write a table in the middle of one evaluation.
type pinned struct {
	vals    []event.Value
	tables  []*table.Version
	onValue func()
}

func (r *pinned) Value(slot, attr int) event.Value {
	if r.onValue != nil {
		r.onValue()
	}
	return r.vals[attr]
}

func (r *pinned) Invisibles() bool { return false }

func (r *pinned) Table(id int) *table.Version {
	if id < 0 || id >= len(r.tables) {
		return nil
	}
	return r.tables[id]
}

// mValues: h "db1", g "DB1", p 7, fl -0.0, hn null, bo false, tm 1000, du 1.
func mValues() []event.Value {
	return []event.Value{
		{Kind: event.TString, Str: "db1"}, {Kind: event.TString, Str: "DB1"}, {Kind: event.TInt, I: 7},
		{Kind: event.TFloat, F: math.Copysign(0, -1)}, {Kind: event.TString, Null: true}, {Kind: event.TBool, B: false},
		{Kind: event.TTimestamp, T: 1000}, {Kind: event.TDuration, D: 1},
	}
}

func mBuild(t *testing.T, node model.Expr) expr.Expr {
	t.Helper()
	e, err := expr.Build(node, mScope{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return e
}

func mTable(t *testing.T, name string, key event.Type, keys ...event.Value) *table.Table {
	t.Helper()
	tb, err := table.New(name, []event.Attr{{Name: "k", Type: key}}, []string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	rows := make([][]event.Value, len(keys))
	for i, k := range keys {
		rows[i] = []event.Value{k}
	}
	if err := tb.Upsert(rows); err != nil {
		t.Fatal(err)
	}
	return tb
}

// mSnapshots builds the six tables: Hosts {"db1", "abc"}, Ports {7, 443}, Scores {0.0, 1.5}, Flags {true},
// Times {1000}, Spans {60000}; it returns their current snapshots by id.
func mSnapshots(t *testing.T) []*table.Version {
	t.Helper()
	str := func(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }
	return []*table.Version{
		mTable(t, "Hosts", event.TString, str("db1"), str("abc")).Snapshot(),
		mTable(t, "Ports", event.TInt, event.Value{Kind: event.TInt, I: 7}, event.Value{Kind: event.TInt, I: 443}).Snapshot(),
		mTable(t, "Scores", event.TFloat, event.Value{Kind: event.TFloat, F: 0}, event.Value{Kind: event.TFloat, F: 1.5}).Snapshot(),
		mTable(t, "Flags", event.TBool, event.Value{Kind: event.TBool, B: true}).Snapshot(),
		mTable(t, "Times", event.TTimestamp, event.Value{Kind: event.TTimestamp, T: 1000}).Snapshot(),
		mTable(t, "Spans", event.TDuration, event.Value{Kind: event.TDuration, D: 60_000}).Snapshot(),
	}
}

func wantBool(t *testing.T, e expr.Expr, r expr.Row, want bool, what string) {
	t.Helper()
	v, err := e.Eval(r)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if v.Kind != event.TBool || v.Null || v.B != want {
		t.Fatalf("%s = %+v, want %v", what, v, want)
	}
}

// TestMembership: `in` is Has on the row's snapshot of the table, per key type; `not in` is its negation; a null
// operand is false for both.
func TestMembership(t *testing.T) {
	r := &pinned{vals: mValues(), tables: mSnapshots(t)}
	cases := []struct {
		name string
		node model.Expr
		want bool
	}{
		{"a string hit", in(a("h"), "Hosts", false), true},
		{"a string hit, negated", in(a("h"), "Hosts", true), false},
		{"strings are byte-exact: DB1 misses", in(a("g"), "Hosts", false), false},
		{"a string miss, negated", in(a("g"), "Hosts", true), true},
		{"a literal operand", in(lit("abc"), "Hosts", false), true},
		{"an int hit", in(a("p"), "Ports", false), true},
		{"an int miss", in(lit(8080), "Ports", false), false},
		{"-0.0 finds the 0.0 key", in(a("fl"), "Scores", false), true},
		{"a float miss, negated", in(lit(2.5), "Scores", true), true},
		{"a bool miss", in(a("bo"), "Flags", false), false},
		{"a bool miss, negated", in(a("bo"), "Flags", true), true},
		{"a timestamp hit", in(a("tm"), "Times", false), true},
		{"a duration miss", in(a("du"), "Spans", false), false},
		{"null: in is false", in(a("hn"), "Hosts", false), false},
		{"null: not in is false", in(a("hn"), "Hosts", true), false},
		{"not over in is pure negation", not(in(a("h"), "Hosts", false)), false},
		{"membership inside and", and(in(a("h"), "Hosts", false), in(a("p"), "Ports", true)), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantBool(t, mBuild(t, c.node), r, c.want, c.name)
		})
	}
}

// TestMembershipNilSnapshot: a row with no snapshot for the table is a wiring fault and a RuntimeError, never an
// empty table (design note §2, review F17).
func TestMembershipNilSnapshot(t *testing.T) {
	snaps := mSnapshots(t)
	for name, r := range map[string]*pinned{
		"no snapshots at all":          {vals: mValues()},
		"a nil snapshot for Hosts":     {vals: mValues(), tables: append([]*table.Version{nil}, snaps[1:]...)},
		"snapshots for other ids only": {vals: mValues(), tables: []*table.Version{}},
	} {
		for _, negated := range []bool{false, true} {
			v, err := mBuild(t, in(a("h"), "Hosts", negated)).Eval(r)
			var re *expr.RuntimeError
			if !errors.As(err, &re) {
				t.Errorf("%s, negated %v: want a RuntimeError, got %+v, %v", name, negated, v, err)
			}
		}
	}
	// The other table's snapshot is still read.
	r := &pinned{vals: mValues(), tables: append([]*table.Version{nil}, snaps[1:]...)}
	wantBool(t, mBuild(t, in(a("p"), "Ports", false)), r, true, "p in Ports beside a nil Hosts")
}

// TestMembershipOperandError: the operand's runtime error comes out as is (design note §2).
func TestMembershipOperandError(t *testing.T) {
	r := &pinned{vals: mValues(), tables: mSnapshots(t)}
	bad := call("cast", lit("x"), lit("int")) // cast: "x" is not an int
	_, want := mBuild(t, bad).Eval(r)
	for _, negated := range []bool{false, true} {
		_, err := mBuild(t, in(bad, "Ports", negated)).Eval(r)
		var re *expr.RuntimeError
		if !errors.As(err, &re) || !strings.HasPrefix(re.Error(), "cast: ") || err.Error() != want.Error() {
			t.Fatalf("negated %v: want the cast's RuntimeError %v, got %v", negated, want, err)
		}
	}
}

// TestMembershipSnapshot is exit line 4's proof (design note §2; SEMANTICS §4 as annotated by owner D1): a row
// pinned to Version v1 keeps v1's answers for `in` and `not in` while the table moves on, including when the
// write happens inside the row's own Value call in the middle of one evaluation; a row pinned to the new
// Snapshot() reads the new state.
func TestMembershipSnapshot(t *testing.T) {
	str := func(s string) event.Value { return event.Value{Kind: event.TString, Str: s} }
	hosts := mTable(t, "Hosts", event.TString, str("a"))
	v1 := hosts.Snapshot()
	snaps := mSnapshots(t)
	snaps[0] = v1

	vals := mValues()
	vals[0] = str("b") // h: absent from v1, added below
	vals[1] = str("a") // g: present in v1, deleted below
	r1 := &pinned{vals: vals, tables: snaps}

	hIn, hNotIn := mBuild(t, in(a("h"), "Hosts", false)), mBuild(t, in(a("h"), "Hosts", true))
	gIn, gNotIn := mBuild(t, in(a("g"), "Hosts", false)), mBuild(t, in(a("g"), "Hosts", true))
	v1Answers := func(r expr.Row, when string) {
		t.Helper()
		wantBool(t, hIn, r, false, when+": h in Hosts")
		wantBool(t, hNotIn, r, true, when+": h not in Hosts")
		wantBool(t, gIn, r, true, when+": g in Hosts")
		wantBool(t, gNotIn, r, false, when+": g not in Hosts")
	}
	v1Answers(r1, "v1, before any write")

	// v2: an Upsert published between two evaluations.
	if err := hosts.Upsert([][]event.Value{{str("b")}}); err != nil {
		t.Fatal(err)
	}
	if hosts.Snapshot() == v1 {
		t.Fatal("the Upsert published nothing")
	}
	v1Answers(r1, "the row pinned to v1, after v2 was published")

	// v3, v4: written from inside the row's own Value call, in the middle of one evaluation that reads the
	// table twice. Each read of an attribute writes; every answer of the evaluation is still v1's.
	writes := 0
	r1.onValue = func() {
		writes++
		var err error
		if writes == 1 {
			err = hosts.Delete([][]event.Value{{str("a")}})
		} else {
			err = hosts.Upsert([][]event.Value{{str("b")}, {str("c")}})
		}
		if err != nil {
			t.Errorf("the write inside Value: %v", err)
		}
	}
	// (h not in Hosts) and (g in Hosts): true under v1; false under any later version.
	both := mBuild(t, and(in(a("h"), "Hosts", true), in(a("g"), "Hosts", false)))
	wantBool(t, both, r1, true, "(h not in Hosts) and (g in Hosts), written mid-evaluation")
	// (h in Hosts) or (g not in Hosts): false under v1; true under any later version.
	either := mBuild(t, or(in(a("h"), "Hosts", false), in(a("g"), "Hosts", true)))
	wantBool(t, either, r1, false, "(h in Hosts) or (g not in Hosts), written mid-evaluation")
	if writes < 4 {
		t.Fatalf("the evaluations read %d attributes, want 4: the mid-evaluation writes did not all happen", writes)
	}
	r1.onValue = nil
	v1Answers(r1, "the row pinned to v1, after the mid-evaluation writes")
	if v1.Len() != 1 || !v1.Has(str("a")) || v1.Has(str("b")) {
		t.Fatalf("v1 changed: Len %d", v1.Len())
	}

	// The next evaluation, pinned to the table's new snapshot, sees the writes: h in, g out.
	now := hosts.Snapshot()
	if now == v1 || !now.Has(str("b")) || now.Has(str("a")) {
		t.Fatal("the table's current snapshot does not hold the writes")
	}
	snaps2 := append([]*table.Version{}, snaps...)
	snaps2[0] = now
	r2 := &pinned{vals: vals, tables: snaps2}
	wantBool(t, hIn, r2, true, "the new snapshot: h in Hosts")
	wantBool(t, hNotIn, r2, false, "the new snapshot: h not in Hosts")
	wantBool(t, gIn, r2, false, "the new snapshot: g in Hosts")
	wantBool(t, gNotIn, r2, true, "the new snapshot: g not in Hosts")
	wantBool(t, both, r2, false, "the new snapshot: (h not in Hosts) and (g in Hosts)")
	wantBool(t, either, r2, true, "the new snapshot: (h in Hosts) or (g not in Hosts)")
}
