package playback

import (
	"strconv"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
)

// Trace records a replay's canonical trace (design note §5.5): one canonical JSON line per entry, in the order
// recorded, keys sorted bytewise, every entry with "at" (the line of the step that produced it) and "k" (its kind).
type Trace struct {
	app *App
	buf []byte
	err error
}

// NewTrace makes an empty trace for app.
func NewTrace(app *App) *Trace { return &Trace{app: app} }

// timerKinds names each timer kind in the trace, by its clock.TimerKind value: the Go name, lowercased.
var timerKinds = [...]string{
	clock.Completion: "completion",
	clock.CronClose:  "cronclose",
	clock.DelayDue:   "delaydue",
	clock.BatchClose: "batchclose",
	clock.TTLPurge:   "ttlpurge",
	clock.Discard:    "discard",
}

func (t *Trace) streamName(sid int) string {
	if sid >= 0 && sid < len(t.app.Streams) {
		return t.app.Streams[sid].Schema.Stream()
	}
	return "#" + strconv.Itoa(sid)
}

// open starts an entry: {"at":<at>
func (t *Trace) open(at int) {
	t.buf = append(t.buf, `{"at":`...)
	t.buf = strconv.AppendInt(t.buf, int64(at), 10)
}

func (t *Trace) str(key, v string) {
	t.buf = append(t.buf, ',', '"')
	t.buf = append(t.buf, key...)
	t.buf = append(t.buf, '"', ':')
	t.buf = appendString(t.buf, v)
}

func (t *Trace) num(key string, v uint64) {
	t.buf = append(t.buf, ',', '"')
	t.buf = append(t.buf, key...)
	t.buf = append(t.buf, '"', ':')
	t.buf = strconv.AppendUint(t.buf, v, 10)
}

func (t *Trace) value(key string, v event.Value) {
	t.buf = append(t.buf, ',', '"')
	t.buf = append(t.buf, key...)
	t.buf = append(t.buf, '"', ':')
	var err error
	t.buf, err = AppendValue(t.buf, v)
	if err != nil && t.err == nil {
		t.err = err
	}
}

func (t *Trace) time(key string, ms int64) { t.value(key, event.Value{Kind: event.TTimestamp, T: ms}) }

func (t *Trace) close() { t.buf = append(t.buf, '}', '\n') }

// Step records the driver's own entries: recording, stream_late, table, batch_end, tick, stop, end.
func (t *Trace) Step(s *Step) {
	switch s.Kind {
	case StepHeader:
		t.open(s.Line)
		t.str("k", "recording")
		t.time("start", s.Time)
	case StepStreamLate:
		t.open(s.Line)
		t.str("k", "stream_late")
		t.num("seq", s.Event.Seq)
		t.str("stream", t.streamName(s.Event.StreamID))
		t.time("ts", s.Event.Ts)
	case StepTable:
		t.open(s.Line)
		t.str("k", "table")
		op := "put"
		if s.Op == OpDelete {
			op = "delete"
		}
		t.str("op", op)
		name := "#" + strconv.Itoa(s.Table)
		if s.Table >= 0 && s.Table < len(t.app.Tables) {
			name = t.app.Tables[s.Table].Name
		}
		t.str("table", name)
	case StepBatchEnd:
		t.open(s.Line)
		t.str("batch", s.Batch)
		t.str("k", "batch_end")
		t.str("stream", t.streamName(s.Stream))
	case StepTick:
		t.open(s.Line)
		t.str("k", "tick")
		t.time("time", s.Time)
	case StepStop:
		t.open(s.Line)
		t.str("k", "stop")
	case StepEnd:
		t.open(s.Line)
		t.str("k", "end")
	default:
		return // StepEvent: what an event does is recorded by its rules
	}
	t.close()
}

// eventEntry is the event and construct_late entries: at, k, key, rule, seq, stream, ts.
func (t *Trace) eventEntry(kind string, at int, rule string, key event.Value, e event.Event) {
	t.open(at)
	t.str("k", kind)
	t.value("key", key)
	t.str("rule", rule)
	t.num("seq", e.Seq)
	t.str("stream", t.streamName(e.StreamID))
	t.time("ts", e.Ts)
	t.close()
}

// Released records a released event.
func (t *Trace) Released(at int, rule string, key event.Value, e event.Event) {
	t.eventEntry("event", at, rule, key, e)
}

// ConstructLate records an event the rule's clock judged late.
func (t *Trace) ConstructLate(at int, rule string, key event.Value, e event.Event) {
	t.eventEntry("construct_late", at, rule, key, e)
}

// Fired records a fired timer: at, due, k, key, kind, ref, rule.
func (t *Trace) Fired(at int, rule string, key event.Value, tm clock.Timer) {
	t.open(at)
	t.time("due", tm.Due)
	t.str("k", "timer")
	t.value("key", key)
	kind := "kind(" + strconv.Itoa(int(tm.Kind)) + ")"
	if int(tm.Kind) < len(timerKinds) && timerKinds[tm.Kind] != "" {
		kind = timerKinds[tm.Kind]
	}
	t.str("kind", kind)
	t.num("ref", tm.Ref)
	t.str("rule", rule)
	t.close()
}

// Discarded records a discard: at, k, key, ref, rule.
func (t *Trace) Discarded(at int, rule string, key event.Value, ref uint64) {
	t.open(at)
	t.str("k", "discard")
	t.value("key", key)
	t.num("ref", ref)
	t.str("rule", rule)
	t.close()
}

// Purged records a TTL purge: at, due, k, key, rule.
func (t *Trace) Purged(at int, rule string, key event.Value, due int64) {
	t.open(at)
	t.time("due", due)
	t.str("k", "purge")
	t.value("key", key)
	t.str("rule", rule)
	t.close()
}

// Note records a body's own observation: at, k, key, name, rule, value.
func (t *Trace) Note(at int, rule string, key event.Value, name string, v event.Value) {
	t.open(at)
	t.str("k", "note")
	t.value("key", key)
	t.str("name", name)
	t.str("rule", rule)
	t.value("value", v)
	t.close()
}

// Bytes is the trace so far, or the first encoding error.
func (t *Trace) Bytes() ([]byte, error) {
	if t.err != nil {
		return nil, t.err
	}
	return t.buf, nil
}
