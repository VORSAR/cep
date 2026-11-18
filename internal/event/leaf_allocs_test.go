package event

import "testing"

// TestDecodeLeafAllocations pins each leaf's allocations per pass over its benchmark's list (S1.1c-6 design §6 E.3;
// decode_bench_test.go holds the inputs), each an equality, so a split that adds or removes one shows. Each pin
// names where its allocations come from. The numbers are the same under -race. Not parallel: testing.AllocsPerRun
// counts the whole process's allocations.
func TestDecodeLeafAllocations(t *testing.T) {
	var buf [128]byte
	n := 0
	d := NewDecoder(leafSchema(t))
	payload := []byte(leafPayload)
	dst := make([]Value, 0, 16)
	rfc := rfc3339Inputs()
	pins := []struct {
		name string
		want float64 // allocations per run
		why  string  // where each comes from
		run  func()
	}{
		{"compactAndValid over the six objects", 0, "a walk over raw, nothing built", func() {
			for _, o := range leafObjects {
				if compactAndValid(o) {
					n++
				}
			}
		}},
		{"CleanLen over the six objects", 0, "a count, nothing built", func() {
			for _, o := range leafObjects {
				n += CleanLen(o)
			}
		}},
		// The three compact objects cost one each: the stored string, string(raw) (decode.go:209). The two spaced
		// ones cost one each: the stored string, string(out) (:242); out's buffer of cap len(raw) stays on the stack
		// (:211 does not escape, and raw is at most 32 bytes). The malformed \xff costs two: its U+FFFD is two bytes
		// longer than the byte it replaces, so append grows out past len(raw) onto the heap, then the string.
		{"cleanObject over the six objects", 7, "3 compact x 1 + 2 spaced x 1 + the malformed byte's 2", func() {
			for _, o := range leafObjects {
				n += len(cleanObject(o))
			}
		}},
		{"cleanObject over the compact object alone", 1, "the stored string, string(raw)", func() {
			n += len(cleanObject(leafObjects[0]))
		}},
		{"cleanObject over the spaced object alone", 1, "the stored string, string(out); out on the stack", func() {
			n += len(cleanObject(leafObjects[1]))
		}},
		{"appendUnescaped over the six inputs into one [128]byte", 0, "every result fits the reused buffer", func() {
			for _, in := range leafEscaped {
				n += len(appendUnescaped(buf[:0], in))
			}
		}},
		{"parseRFC3339 over the 50 rfc3339Cases", 0, "arithmetic over the input, nothing built", func() {
			for _, in := range rfc {
				ms, _ := parseRFC3339(in)
				n += int(ms & 1)
			}
		}},
		{"(*scanner).value over the three values", 0, "a check, nothing built", func() {
			for _, in := range leafJSONValues {
				s := scanner{b: in}
				if s.value() == nil {
					n++
				}
			}
		}},
		// "alpha" is the stored string (decodeString's string(text), decode.go:405); the object is its stored string
		// (cleanObject's string(raw), :209) and that string boxed into Value.Obj, an any (:196). The spans fit
		// Decode's [16]span on the stack and the values fit dst.
		{"Decode of leafPayload into make([]Value, 0, 16)", 3, `"alpha", the object's string, its box in Obj`, func() {
			if _, _, err := d.Decode(payload, dst); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, p := range pins {
		if got := testing.AllocsPerRun(200, p.run); got != p.want {
			t.Errorf("%s: %v allocations per run; want %v (%s)", p.name, got, p.want, p.why)
		}
	}
	_ = n
}
