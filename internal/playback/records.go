package playback

import (
	"errors"
	"fmt"

	"github.com/vorsar/cep/internal/event"
)

// The envelope's keys, in the envelope schema's attribute order (design note §5.2).
const (
	eAttrs = iota
	eArrival
	eBatch
	eBatchEnd
	eOp
	eRecording
	eRow
	eStop
	eStream
	eTable
	eTick
	eTs
	eRejected
	eCount
)

// envelopeAttrs is the envelope schema: every key optional. A lookup table, never written.
var envelopeAttrs = [eCount]event.Attr{
	eAttrs:     {Name: "attrs", Type: event.TObject, Optional: true},
	eArrival:   {Name: "arrival", Type: event.TTimestamp, Optional: true},
	eBatch:     {Name: "batch", Type: event.TString, Optional: true},
	eBatchEnd:  {Name: "batch_end", Type: event.TString, Optional: true},
	eOp:        {Name: "op", Type: event.TString, Optional: true},
	eRecording: {Name: "recording", Type: event.TObject, Optional: true},
	eRow:       {Name: "row", Type: event.TObject, Optional: true},
	eStop:      {Name: "stop", Type: event.TBool, Optional: true},
	eStream:    {Name: "stream", Type: event.TString, Optional: true},
	eTable:     {Name: "table", Type: event.TString, Optional: true},
	eTick:      {Name: "tick", Type: event.TTimestamp, Optional: true},
	eTs:        {Name: "ts", Type: event.TTimestamp, Optional: true},
	eRejected:  {Name: "rejected", Type: event.TObject, Optional: true},
}

// recordKinds names each kind's choosing key, in the order the kind is chosen (design note §5.2), and the keys it
// allows.
var recordKinds = [...]struct {
	key     int
	kind    RecordKind
	name    string
	allowed [eCount]bool
}{
	{eRecording, RecHeader, "recording header", [eCount]bool{eRecording: true}},
	{eTable, RecTable, "table", [eCount]bool{eTable: true, eOp: true, eRow: true}},
	{eBatchEnd, RecBatchEnd, "batch end", [eCount]bool{eBatchEnd: true, eStream: true}},
	{eTick, RecTick, "tick", [eCount]bool{eTick: true}},
	{eStop, RecStop, "stop", [eCount]bool{eStop: true}},
	{eRejected, RecRejected, "rejected", [eCount]bool{eRejected: true, eStream: true, eArrival: true}},
	{eStream, RecEvent, "event", [eCount]bool{eStream: true, eAttrs: true, eBatch: true, eTs: true, eArrival: true}},
}

// record decodes one non-blank line (design note §5.2).
func (r *Reader) record(line []byte) (Record, error) {
	vals, err := r.envelope(line)
	if err != nil {
		return Record{}, err
	}
	present := func(i int) bool { return !vals[i].Null }
	kind, err := r.recordKind(vals)
	if err != nil {
		return Record{}, err
	}
	switch kind {
	case RecHeader:
		return r.headerRecord(vals)
	case RecTable:
		return r.tableRecord(vals, present)
	case RecBatchEnd:
		return r.batchEndRecord(vals, present)
	case RecTick:
		return r.tickRecord(vals)
	case RecStop:
		return r.stopRecord(vals)
	case RecRejected:
		return r.rejectedRecord(vals, present)
	}
	return r.eventRecord(vals, present)
}

// envelope is line decoded against the envelope schema; refused for a decode fault, an unknown key, or a record after
// the stop.
func (r *Reader) envelope(line []byte) ([]event.Value, error) {
	vals, info, err := r.env.Decode(line, r.envBuf[:0])
	if err != nil {
		var de *event.DecodeError
		if errors.As(err, &de) {
			return nil, r.fault("not a log record: %s", de.Msg)
		}
		return nil, r.fault("not a log record: %v", err)
	}
	r.envBuf = vals
	if info.Unknown > 0 {
		return nil, r.fault("a key the log format does not define")
	}
	if r.stopped {
		return nil, r.fault("a record after the stop record")
	}
	return vals, nil
}

// recordKind is the line's kind: the first choosing key present; refused when none is, or a key the kind does not
// allow is present.
func (r *Reader) recordKind(vals []event.Value) (RecordKind, error) {
	kind := -1
	for k := range recordKinds {
		if !vals[recordKinds[k].key].Null {
			kind = k
			break
		}
	}
	if kind < 0 {
		return 0, r.fault("a record names none of recording, table, batch_end, tick, stop, rejected or stream")
	}
	rk := &recordKinds[kind]
	for i := range eCount {
		if !vals[i].Null && !rk.allowed[i] {
			return 0, r.fault("a %s record takes no %q key", rk.name, envelopeAttrs[i].Name)
		}
	}
	return rk.kind, nil
}

// rejectedRecord is a recording's rejected live event (owner Q1 of S3.2b-2; design s3-2b-2 §7): its stream, its
// arrival (on every stream: a rejected record is no event) and its fault's reason, with the attribute unless the
// fault is syntax. Legal only in a recording.
func (r *Reader) rejectedRecord(vals []event.Value, present func(int) bool) (Record, error) {
	if !r.recording {
		return Record{}, r.fault("a rejected record in a log without a recording header")
	}
	if !present(eStream) || !present(eArrival) {
		return Record{}, r.fault("a rejected record carries stream and arrival")
	}
	sid, err := r.stream(vals)
	if err != nil {
		return Record{}, err
	}
	reason, attr, err := r.rejectedFault(vals)
	if err != nil {
		return Record{}, err
	}
	arrival := vals[eArrival].T
	if err := r.advance(arrival); err != nil {
		return Record{}, err
	}
	return Record{Line: r.line, Kind: RecRejected, Stream: sid, Arrival: arrival, Reason: reason, Attr: attr}, nil
}

// rejectedFault is the rejected record's fault: its reason and attribute, refused when the pair is inconsistent.
func (r *Reader) rejectedFault(vals []event.Value) (event.DecodeReason, string, error) {
	f, info, err := r.rejected.Decode([]byte(vals[eRejected].Obj.(string)), nil)
	if err != nil {
		return 0, "", r.fault("the rejected record: %s", decodeMsg(err))
	}
	if info.Unknown > 0 {
		return 0, "", r.fault("the rejected record has a key other than reason and attr")
	}
	reason, ok := reasonOf[f[1].Str]
	switch {
	case !ok:
		return 0, "", r.fault("the rejected record's reason %q is none of syntax, type, missing, null", f[1].Str)
	case reason == event.DecodeSyntax && !f[0].Null:
		return 0, "", r.fault("a syntax fault names no attribute")
	case reason != event.DecodeSyntax && f[0].Null:
		return 0, "", r.fault("a %s fault names its attribute", f[1].Str)
	}
	return reason, f[0].Str, nil
}

// advance moves the recording's clock to arrival; refused when arrival is earlier than the previous arrival or tick.
func (r *Reader) advance(arrival int64) error {
	if arrival < r.last {
		return r.fault("an arrival before the previous arrival or tick")
	}
	r.last = arrival
	return nil
}

// reasonOf maps a rejected record's reason to its decode reason.
var reasonOf = map[string]event.DecodeReason{
	"syntax": event.DecodeSyntax, "type": event.DecodeType, "missing": event.DecodeMissing, "null": event.DecodeNull,
}

func (r *Reader) headerRecord(vals []event.Value) (Record, error) {
	if r.records > 0 {
		return Record{}, r.fault("the recording header must be the first record")
	}
	h, info, err := r.header.Decode([]byte(vals[eRecording].Obj.(string)), nil)
	if err != nil {
		return Record{}, r.fault("the recording header: %s", decodeMsg(err))
	}
	if info.Unknown > 0 {
		return Record{}, r.fault("the recording header has a key other than start")
	}
	r.recording, r.last = true, h[0].T
	return Record{Line: r.line, Kind: RecHeader, Time: h[0].T}, nil
}

func (r *Reader) tableRecord(vals []event.Value, present func(int) bool) (Record, error) {
	name := vals[eTable].Str
	tid, ok := r.tables[name]
	switch {
	case !ok:
		return Record{}, r.fault("unknown table %q", name)
	case !present(eOp):
		return Record{}, r.fault("a table record needs op")
	case !present(eRow):
		return Record{}, r.fault("a table record needs row")
	}
	op, err := r.tableOp(name, vals)
	if err != nil {
		return Record{}, err
	}
	row, err := r.tableRow(name, tid, op, vals)
	if err != nil {
		return Record{}, err
	}
	return Record{Line: r.line, Kind: RecTable, Table: tid, Op: op, Row: row}, nil
}

// tableOp is the table record's op: put or delete, refused otherwise.
func (r *Reader) tableOp(name string, vals []event.Value) (TableOp, error) {
	var op TableOp
	switch vals[eOp].Str {
	case "put":
		op = OpPut
	case "delete":
		op = OpDelete
	default:
		return 0, r.fault("table %q: op %q is neither put nor delete", name, vals[eOp].Str)
	}
	return op, nil
}

// tableRow is the table record's row decoded for op: refused for a decode fault, a column the op does not take, or a
// put's null key column.
func (r *Reader) tableRow(name string, tid int, op TableOp, vals []event.Value) ([]event.Value, error) {
	dec := r.tput[tid]
	if op == OpDelete {
		dec = r.tdel[tid]
	}
	row, info, err := dec.Decode([]byte(vals[eRow].Obj.(string)), nil)
	if err != nil {
		return nil, r.fault("table %q: the row: %s", name, decodeMsg(err))
	}
	if info.Unknown > 0 {
		if op == OpDelete {
			return nil, r.fault("table %q: a delete row holds only the key columns", name)
		}
		return nil, r.fault("table %q: the row has a column the table does not declare", name)
	}
	if op == OpPut {
		for _, k := range r.tkey[tid] {
			if row[k].Null {
				return nil, r.fault("table %q: key column %q is null", name, r.app.Tables[tid].Attrs[k].Name)
			}
		}
	}
	return row, nil
}

func (r *Reader) stream(vals []event.Value) (int, error) {
	name := vals[eStream].Str
	sid, ok := r.streams[name]
	switch {
	case !ok:
		return 0, r.fault("unknown stream %q", name)
	case r.app.Streams[sid].Derived:
		return 0, r.fault("stream %q is a rule's output; a log holds source streams only", name)
	}
	return sid, nil
}

func (r *Reader) batchEndRecord(vals []event.Value, present func(int) bool) (Record, error) {
	if !present(eStream) {
		return Record{}, r.fault("a batch end needs stream")
	}
	sid, err := r.stream(vals)
	if err != nil {
		return Record{}, err
	}
	if vals[eBatchEnd].Str == "" {
		return Record{}, r.fault("an empty batch id")
	}
	return Record{Line: r.line, Kind: RecBatchEnd, Stream: sid, Batch: vals[eBatchEnd].Str}, nil
}

func (r *Reader) tickRecord(vals []event.Value) (Record, error) {
	if !r.recording {
		return Record{}, r.fault("a tick in a log without a recording header")
	}
	t := vals[eTick].T
	if t < r.last {
		return Record{}, r.fault("a tick before the previous arrival or tick")
	}
	r.last = t
	return Record{Line: r.line, Kind: RecTick, Time: t}, nil
}

func (r *Reader) stopRecord(vals []event.Value) (Record, error) {
	switch {
	case !r.recording:
		return Record{}, r.fault("a stop in a log without a recording header")
	case !vals[eStop].B:
		return Record{}, r.fault(`"stop" must be true`)
	}
	r.stopped = true
	return Record{Line: r.line, Kind: RecStop}, nil
}

func (r *Reader) eventRecord(vals []event.Value, present func(int) bool) (Record, error) {
	sid, err := r.stream(vals)
	if err != nil {
		return Record{}, err
	}
	st := &r.app.Streams[sid]
	name := st.Schema.Stream()
	arrivalTime := st.Schema.TimeAttr() == event.ArrivalTime
	if err := r.eventKeys(st, name, arrivalTime, vals, present); err != nil {
		return Record{}, err
	}
	values, info, err := r.eventPayload(sid, name, vals)
	if err != nil {
		return Record{}, err
	}
	ts := info.Ts
	if arrivalTime {
		ts = vals[eTs].T
	}
	arrival, err := r.eventArrival(ts, arrivalTime, vals)
	if err != nil {
		return Record{}, err
	}
	var batch string
	if present(eBatch) {
		batch = vals[eBatch].Str
	}
	return Record{Line: r.line, Kind: RecEvent, Arrival: arrival, Info: info,
		Event: event.Event{StreamID: sid, Ts: ts, Values: values, BatchID: batch}}, nil
}

// eventKeys checks an event's keys against its stream and the log: attrs, ts by the stream's time, arrival, a
// non-empty batch.
func (r *Reader) eventKeys(st *Stream, name string, arrivalTime bool, vals []event.Value,
	present func(int) bool) error {
	switch {
	case !present(eAttrs):
		return r.fault("an event needs attrs")
	case arrivalTime && !present(eTs):
		return r.fault("stream %q is `time arrival`: its events carry ts", name)
	case !arrivalTime && present(eTs):
		return r.fault("ts on stream %q, whose time is its attribute %q", name, st.Schema.Attr(
			st.Schema.TimeAttr()).Name)
	}
	if err := r.arrivalKeys(name, arrivalTime, present); err != nil {
		return err
	}
	if present(eBatch) && vals[eBatch].Str == "" {
		return r.fault("an empty batch id")
	}
	return nil
}

// arrivalKeys checks arrival against the recording header and the stream's time.
func (r *Reader) arrivalKeys(name string, arrivalTime bool, present func(int) bool) error {
	switch {
	case !r.recording && present(eArrival):
		return r.fault("arrival in a log without a recording header")
	case r.recording && arrivalTime && present(eArrival):
		return r.fault("arrival on `time arrival` stream %q: its ts is the arrival", name)
	case r.recording && !arrivalTime && !present(eArrival):
		return r.fault("an event in a recording carries arrival")
	}
	return nil
}

// eventPayload decodes the attrs against stream sid's schema; a fault is a *LoadError with its Decode cause.
func (r *Reader) eventPayload(sid int, name string, vals []event.Value) ([]event.Value, event.Decoded, error) {
	values, info, err := r.sdec[sid].Decode([]byte(vals[eAttrs].Obj.(string)), nil)
	if err != nil {
		var de *event.DecodeError
		errors.As(err, &de)
		return values, info, &LoadError{Line: r.line, Msg: fmt.Sprintf("the payload fails stream %q's schema: %s", name,
			decodeMsg(err)), Decode: de}
	}
	return values, info, nil
}

// eventArrival is the event's arrival in a recording, the clock advanced to it; 0 outside a recording.
func (r *Reader) eventArrival(ts int64, arrivalTime bool, vals []event.Value) (int64, error) {
	var arrival int64
	if r.recording {
		arrival = ts
		if !arrivalTime {
			arrival = vals[eArrival].T
		}
		if err := r.advance(arrival); err != nil {
			return 0, err
		}
	}
	return arrival, nil
}
