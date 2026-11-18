package playback

import (
	"testing"
)

// hpRepeat is an endless log: prefix once, then line and its newline forever, so Next always has the same record.
type hpRepeat struct {
	data []byte
	off  int
	loop int
}

func newHPRepeat(prefix, line string) *hpRepeat {
	return &hpRepeat{data: []byte(prefix + line + "\n"), loop: len(prefix)}
}

func (r *hpRepeat) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		c := copy(p[n:], r.data[r.off:])
		n += c
		r.off += c
		if r.off == len(r.data) {
			r.off = r.loop
		}
	}
	return n, nil
}

// hpLine is one of the replay path's fixed records, read against cnReaderApp: S(k string, ts timestamp) timed by ts,
// A(k string) of `time arrival`, and the table T(k string, v int) keyed by k.
type hpLine struct {
	name      string
	recording bool // read after the recording header
	kind      RecordKind
	line      string
}

// hpLines are the eight kinds of P.3's table, each a line Next can read again and again: a repeated arrival or tick
// is not earlier than the previous one.
func hpLines() []hpLine {
	return []hpLine{
		{"fixture_event", false, RecEvent, `{"attrs":{"k":"ab","ts":"` + cnT0 + `"},"stream":"S"}`},
		{"arrival_event", false, RecEvent, `{"attrs":{"k":"ab"},"stream":"A","ts":"` + cnT0 + `"}`},
		{"recording_event", true, RecEvent, `{"arrival":"` + cnT0 + `","attrs":{"k":"ab","ts":"` + cnT0 + `"},"stream":"S"}`},
		{"table_put", false, RecTable, `{"op":"put","row":{"k":"a","v":1},"table":"T"}`},
		{"table_delete", false, RecTable, `{"op":"delete","row":{"k":"a"},"table":"T"}`},
		{"batch_end", false, RecBatchEnd, `{"batch_end":"b","stream":"S"}`},
		{"tick", true, RecTick, `{"tick":"` + cnT0 + `"}`},
		{"rejected", true, RecRejected, `{"arrival":"` + cnT0 + `","rejected":{"attr":"k","reason":"type"},"stream":"S"}`},
	}
}

// hpReader is a Reader over c's line repeated forever, past the recording header for a recording kind.
func hpReader(t testing.TB, c hpLine) *Reader {
	t.Helper()
	prefix := ""
	if c.recording {
		prefix = cnHeader + "\n"
	}
	rd, err := NewReader(newHPRepeat(prefix, c.line), cnReaderApp(t))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	if c.recording {
		if rec, err := rd.Next(); err != nil || rec.Kind != RecHeader {
			t.Fatalf("the header: %+v, %v", rec, err)
		}
	}
	return rd
}

func BenchmarkReplayLine(b *testing.B) {
	for _, c := range hpLines() {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			rd := hpReader(b, c)
			for b.Loop() {
				if _, err := rd.Next(); err != nil {
					b.Fatalf("Next: %v", err)
				}
			}
		})
	}
}

func BenchmarkReadLine(b *testing.B) {
	b.ReportAllocs()
	rd := hpReader(b, hpLines()[2])
	for b.Loop() {
		if _, err := rd.readLine(); err != nil {
			b.Fatalf("readLine: %v", err)
		}
	}
}
