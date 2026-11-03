package event

import (
	"fmt"
	"math"
	"slices"
	"testing"
)

// Sources cited below:
//   SEMANTICS §2.2  - the total order (event_time, stream id, per-stream ingestion sequence)
//   ARCH §4         - ARCHITECTURE.md §4: "events are ordered by (Ts, StreamID, Seq)"
//   design §1       - s1-1a-gate-design.md §1: Compare uses cmp.Compare per key, returns exactly
//                     -1, 0 or +1, and payload fields never affect it
//   PC §6 n         - s1-1a-plan-check.md §6 case n (v1's cases 31-37 stand in v2)
//   C36, C37        - s1-1a-plan-check-qa.md counter-examples C36 (comparator mutants), C37 (reflexivity)

// key builds an event that carries only its ordering key.
func key(ts int64, streamID int, seq uint64) *Event {
	return &Event{Ts: ts, StreamID: streamID, Seq: seq}
}

// keyString renders an event's ordering key for failure messages.
func keyString(e *Event) string {
	return fmt.Sprintf("(Ts %d, StreamID %d, Seq %d)", e.Ts, e.StreamID, e.Seq)
}

// TestComparePairs checks each row in both directions: Compare(a, b) == want and
// Compare(b, a) == -want, with want exactly -1, 0 or +1.
func TestComparePairs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b *Event
		want int
	}{
		// PC §6 31; SEMANTICS §2.2: Ts dominates StreamID and Seq.
		{"case31/ts dominates", key(1, 5, 9), key(2, 0, 0), -1},
		// PC §6 32; SEMANTICS §2.2: StreamID (declaration order) breaks a Ts tie.
		{"case32/stream id breaks a ts tie", key(5, 0, 9), key(5, 1, 0), -1},
		// PC §6 33; SEMANTICS §2.2: Seq breaks the rest.
		{"case33/seq breaks the rest", key(5, 1, 3), key(5, 1, 4), -1},

		// PC §6 34: Ts at both int64 extremes; MinInt64-MaxInt64 overflows to +1.
		{"case34/ts min vs max", key(math.MinInt64, 0, 0), key(math.MaxInt64, 0, 0), -1},
		// PC §6 34: MinInt64-1 overflows to MaxInt64, a subtraction gets the sign wrong.
		{"case34/ts min vs 1", key(math.MinInt64, 0, 0), key(1, 0, 0), -1},
		// PC §6 34: MaxInt64-(-1) overflows to MinInt64 in the reverse direction.
		{"case34/ts -1 vs max", key(-1, 0, 0), key(math.MaxInt64, 0, 0), -1},
		// PC §6 34: Seq 0 vs MaxUint64; int(0-MaxUint64) is +1, and int64(MaxUint64) is -1.
		{"case34/seq 0 vs max", key(0, 0, 0), key(0, 0, math.MaxUint64), -1},
		// PC §6 34: Seq across 2^63; a signed view of Seq flips this pair.
		{"case34/seq across 2^63", key(0, 0, 1<<63-1), key(0, 0, 1<<63), -1},
		// design §1: cmp.Compare on StreamID covers int's whole range.
		{"design/stream id min vs max", key(0, math.MinInt, 0), key(0, math.MaxInt, 0), -1},

		// PC §6 35; SEMANTICS §1 (UTC ms instants): a Ts before 1970 sorts before one after it.
		{"case35/negative ts first", key(-1, 0, 0), key(1, 0, 0), -1},
		// PC §6 35: one day before the epoch vs the epoch; the larger keys elsewhere must not matter.
		{"case35/pre-epoch day", key(-86_400_000, 9, 9), key(0, 0, 0), -1},

		// design §1: exactly -1/+1, not a difference (C36's int(a.Ts-b.Ts) gives -990 here).
		{"design/exact sign on a large ts gap", key(10, 0, 0), key(1000, 0, 0), -1},
		// design §1: exactly -1/+1 on a large StreamID gap.
		{"design/exact sign on a large stream id gap", key(7, 2, 0), key(7, 40, 0), -1},
		// design §1: exactly -1/+1 on a large Seq gap.
		{"design/exact sign on a large seq gap", key(7, 2, 5), key(7, 2, 500), -1},

		// C37: equal keys held in two distinct events compare 0.
		{"c37/equal keys", key(5, 1, 3), key(5, 1, 3), 0},
		// C37: equal keys at the low extremes compare 0.
		{"c37/equal keys at minimums", key(math.MinInt64, math.MinInt, 0), key(math.MinInt64, math.MinInt, 0), 0},
		// C37: equal keys at the high extremes compare 0.
		{"c37/equal keys at maximums", key(math.MaxInt64, math.MaxInt, math.MaxUint64), key(math.MaxInt64, math.MaxInt, math.MaxUint64), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Compare(c.a, c.b); got != c.want {
				t.Errorf("Compare(%s, %s) = %d; want %d", keyString(c.a), keyString(c.b), got, c.want)
			}
			if got := Compare(c.b, c.a); got != -c.want {
				t.Errorf("Compare(%s, %s) = %d; want %d", keyString(c.b), keyString(c.a), got, -c.want)
			}
		})
	}
}

// TestCompareIgnoresPayload is PC §6 36: Values, BatchID and Invisibles never affect the order.
func TestCompareIgnoresPayload(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b *Event
		want int
	}{
		// PC §6 36; design §1: equal (Ts, StreamID, Seq) with every payload field different compares 0.
		{
			"case36/equal keys, different payload",
			&Event{Ts: 7, StreamID: 2, Seq: 3, Values: []Value{{Kind: TInt, I: 1}}, Invisibles: true, BatchID: "b1"},
			&Event{Ts: 7, StreamID: 2, Seq: 3, Values: nil, Invisibles: false, BatchID: "zzz"},
			0,
		},
		// PC §6 36; design §1: BatchID ordered against the key cannot reorder a Seq difference.
		{
			"case36/batch id decoy against seq",
			&Event{Ts: 7, StreamID: 2, Seq: 3, BatchID: "z"},
			&Event{Ts: 7, StreamID: 2, Seq: 4, BatchID: "a"},
			-1,
		},
		// PC §6 36; design §1: Invisibles and Values ordered against the key cannot reorder a StreamID difference.
		{
			"case36/invisibles and values decoy against stream id",
			&Event{Ts: 7, StreamID: 1, Seq: 9, Invisibles: true, Values: []Value{{Kind: TString, Str: "zz"}, {Kind: TInt, I: 2}}},
			&Event{Ts: 7, StreamID: 2, Seq: 0, Invisibles: false},
			-1,
		},
		// PC §6 36; design §1: a payload decoy cannot reorder a Ts difference.
		{
			"case36/payload decoy against ts",
			&Event{Ts: 6, StreamID: 9, Seq: 9, BatchID: "zzz", Invisibles: true},
			&Event{Ts: 7, StreamID: 0, Seq: 0, BatchID: "", Values: []Value{{Kind: TBool, B: true}}},
			-1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Compare(c.a, c.b); got != c.want {
				t.Errorf("Compare(%+v, %+v) = %d; want %d", *c.a, *c.b, got, c.want)
			}
			if got := Compare(c.b, c.a); got != -c.want {
				t.Errorf("Compare(%+v, %+v) = %d; want %d", *c.b, *c.a, got, -c.want)
			}
		})
	}
}

// sevenInOrder returns PC §6 37's seven events in the order SEMANTICS §2.2 gives them, written by
// hand. Every key is distinct, so the sorted order is unique. Each neighbouring pair is decided by
// a different part of the key, and the payload runs the other way (BatchID "g" down to "a"), so a
// comparator that reads payload, drops a key part, or subtracts cannot reproduce the order.
func sevenInOrder() []*Event {
	return []*Event{
		// the most negative Ts: first, whatever its StreamID and Seq
		{Ts: math.MinInt64, StreamID: 2, Seq: 9, BatchID: "g", Values: []Value{{Kind: TString, Str: "zzz"}}},
		// a pre-epoch Ts: before every Ts >= 0, despite the largest StreamID in the set
		{Ts: -5, StreamID: 7, Seq: 0, BatchID: "f", Invisibles: true},
		// Ts 0, StreamID 0: before every Ts 0 event of StreamID 1, despite the largest Seq
		{Ts: 0, StreamID: 0, Seq: math.MaxUint64, BatchID: "e"},
		// Ts 0, StreamID 1: Seq 0 < 2^63-1 < 2^63 decides the next three
		{Ts: 0, StreamID: 1, Seq: 0, BatchID: "d", Invisibles: true},
		{Ts: 0, StreamID: 1, Seq: 1<<63 - 1, BatchID: "c"},
		{Ts: 0, StreamID: 1, Seq: 1 << 63, BatchID: "b", Values: []Value{{Kind: TInt, I: -1}}},
		// the largest Ts: last, despite StreamID 0 and Seq 0
		{Ts: math.MaxInt64, StreamID: 0, Seq: 0, BatchID: "a"},
	}
}

// sign is the hand-written order's verdict for positions i and j.
func sign(i, j int) int {
	switch {
	case i < j:
		return -1
	case i > j:
		return 1
	default:
		return 0
	}
}

// TestCompareMatchesHandOrder checks Compare exactly on all 49 ordered pairs of PC §6 37's set.
func TestCompareMatchesHandOrder(t *testing.T) {
	t.Parallel()
	order := sevenInOrder()
	for i, a := range order {
		for j, b := range order {
			// PC §6 37; SEMANTICS §2.2: position in the hand-written order decides; design §1: exactly -1/0/+1.
			if got, want := Compare(a, b), sign(i, j); got != want {
				t.Errorf("Compare(%s, %s) = %d; want %d (positions %d, %d)", keyString(a), keyString(b), got, want, i, j)
			}
		}
	}
}

// permutations calls visit with every permutation of 0..n-1 (Heap's algorithm). visit must not
// keep p: it is reused.
func permutations(n int, visit func(p []int)) {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	var rec func(k int)
	rec = func(k int) {
		if k <= 1 {
			visit(p)
			return
		}
		for i := range k - 1 {
			rec(k - 1)
			if k%2 == 0 {
				p[i], p[k-1] = p[k-1], p[i]
			} else {
				p[0], p[k-1] = p[k-1], p[0]
			}
		}
		rec(k - 1)
	}
	rec(n)
}

// TestCompareSortsEveryPermutation is PC §6 37: each of the 7! = 5040 orderings of the set, sorted
// with Compare, gives the one hand-written order. It is exhaustive, so it is deterministic.
func TestCompareSortsEveryPermutation(t *testing.T) {
	t.Parallel()
	order := sevenInOrder()
	seen := make(map[[7]int]bool)
	failures := 0
	permutations(len(order), func(p []int) {
		var perm [7]int
		copy(perm[:], p)
		seen[perm] = true

		shuffled := make([]*Event, len(order))
		for i, j := range p {
			shuffled[i] = order[j]
		}
		slices.SortFunc(shuffled, Compare)
		for i := range order {
			if shuffled[i] != order[i] {
				failures++
				if failures == 1 {
					got := make([]string, len(shuffled))
					for k, e := range shuffled {
						got[k] = keyString(e)
					}
					t.Errorf("sorting permutation %v gave %v; want the hand-written order", perm, got)
				}
				break
			}
		}
	})
	// The generator itself must be exhaustive, or this test proves less than it claims.
	if len(seen) != 5040 {
		t.Fatalf("visited %d distinct permutations; want 7! = 5040", len(seen))
	}
	if failures > 0 {
		t.Errorf("%d of 5040 permutations sorted to the wrong order", failures)
	}
}

// TestCompareIsATotalOrder checks C37 (reflexivity), antisymmetry and transitivity over every
// ordered triple of PC §6 37's seven events plus one payload twin (an event whose key equals the
// fourth event's), so that ties take part in the triples.
func TestCompareIsATotalOrder(t *testing.T) {
	t.Parallel()
	set := sevenInOrder()
	// PC §6 36: a twin of set[3]'s key with a different payload; it compares 0 with set[3].
	twin := &Event{Ts: 0, StreamID: 1, Seq: 0, BatchID: "zz", Values: []Value{{Kind: TFloat, F: 1.5}}}
	set = append(set, twin)

	// A broken comparator breaks hundreds of triples; the first few say why, the count says how many.
	violations := 0
	report := func(format string, args ...any) {
		violations++
		if violations <= 5 {
			t.Errorf(format, args...)
		}
	}
	for _, a := range set {
		// C37: Compare(a, a) == 0, the same pointer on both sides.
		if got := Compare(a, a); got != 0 {
			report("Compare(a, a) = %d for a = %s; want 0", got, keyString(a))
		}
		for _, b := range set {
			ab, ba := Compare(a, b), Compare(b, a)
			// design §1: exactly -1, 0 or +1.
			if ab < -1 || ab > 1 {
				report("Compare(%s, %s) = %d; want -1, 0 or +1", keyString(a), keyString(b), ab)
			}
			// antisymmetry: swapping the arguments negates the result.
			if ab != -ba {
				report("Compare(%s, %s) = %d but Compare(%s, %s) = %d; want negatives",
					keyString(a), keyString(b), ab, keyString(b), keyString(a), ba)
			}
			for _, c := range set {
				bc, ac := Compare(b, c), Compare(a, c)
				// transitivity of a total preorder: a <= b <= c gives a <= c, equal only if both steps are equal.
				if ab <= 0 && bc <= 0 {
					want := -1
					if ab == 0 && bc == 0 {
						want = 0
					}
					if ac != want {
						report("Compare(a, b) = %d, Compare(b, c) = %d, Compare(a, c) = %d; want %d (a %s, b %s, c %s)",
							ab, bc, ac, want, keyString(a), keyString(b), keyString(c))
					}
				}
				// the same, mirrored: a >= b >= c gives a >= c.
				if ab >= 0 && bc >= 0 {
					want := 1
					if ab == 0 && bc == 0 {
						want = 0
					}
					if ac != want {
						report("Compare(a, b) = %d, Compare(b, c) = %d, Compare(a, c) = %d; want %d (a %s, b %s, c %s)",
							ab, bc, ac, want, keyString(a), keyString(b), keyString(c))
					}
				}
			}
		}
	}
	if violations > 5 {
		t.Errorf("%d violations in all (first 5 shown)", violations)
	}
}
