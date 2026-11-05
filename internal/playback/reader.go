package playback

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/vorsar/cep/internal/event"
)

// maxLine is the longest line a log may hold (design note §5.2).
const maxLine = 16 << 20

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
	{eStream, RecEvent, "event", [eCount]bool{eStream: true, eAttrs: true, eBatch: true, eTs: true, eArrival: true}},
}

var errLongLine = errors.New("line too long")

// Reader reads a log one record at a time. Its first fault stops it: Next returns the same *LoadError from then on
// (owner D6 of S3.2a-1).
type Reader struct {
	br        *bufio.Reader
	app       *App
	env       *event.Decoder
	header    *event.Decoder
	streams   map[string]int
	tables    map[string]int
	sdec      []*event.Decoder // per stream: its payload
	tput      []*event.Decoder // per table: a full row
	tdel      []*event.Decoder // per table: its key columns, in key order
	tkey      [][]int          // per table: the key columns' attribute indexes
	line      int
	records   int
	recording bool
	stopped   bool
	last      int64 // a recording's latest arrival or tick
	err       error
	envBuf    []event.Value
}

// NewReader reads r against app; an invalid App is an error.
func NewReader(r io.Reader, app *App) (*Reader, error) {
	if err := validateApp(app); err != nil {
		return nil, err
	}
	envSchema, err := event.NewSchema("log", envelopeAttrs[:], event.ArrivalTime)
	if err != nil {
		return nil, fmt.Errorf("playback: the envelope schema: %w", err)
	}
	hdrSchema, err := event.NewSchema("recording", []event.Attr{{Name: "start", Type: event.TTimestamp}}, event.ArrivalTime)
	if err != nil {
		return nil, fmt.Errorf("playback: the header schema: %w", err)
	}
	rd := &Reader{
		br: bufio.NewReaderSize(r, 64<<10), app: app, env: event.NewDecoder(envSchema), header: event.NewDecoder(hdrSchema),
		streams: map[string]int{}, tables: map[string]int{},
	}
	for i, s := range app.Streams {
		rd.streams[s.Schema.Stream()] = i
		rd.sdec = append(rd.sdec, event.NewDecoder(s.Schema))
	}
	for i, t := range app.Tables {
		rd.tables[t.Name] = i
		full, err := event.NewSchema(t.Name, t.Attrs, event.ArrivalTime)
		if err != nil {
			return nil, fmt.Errorf("playback: table %q: %w", t.Name, err)
		}
		keyAttrs := make([]event.Attr, len(t.Key))
		keyIdx := make([]int, len(t.Key))
		for j, col := range t.Key {
			keyIdx[j] = -1
			for k, a := range t.Attrs {
				if a.Name == col {
					keyAttrs[j], keyIdx[j] = event.Attr{Name: a.Name, Type: a.Type}, k
				}
			}
			if keyIdx[j] < 0 {
				return nil, fmt.Errorf("playback: table %q: key column %q is not declared", t.Name, col)
			}
		}
		keys, err := event.NewSchema(t.Name, keyAttrs, event.ArrivalTime)
		if err != nil {
			return nil, fmt.Errorf("playback: table %q's key: %w", t.Name, err)
		}
		rd.tput = append(rd.tput, event.NewDecoder(full))
		rd.tdel = append(rd.tdel, event.NewDecoder(keys))
		rd.tkey = append(rd.tkey, keyIdx)
	}
	return rd, nil
}

// validateApp checks what a log is read against: unique, non-empty stream and table names, no negative bound.
func validateApp(app *App) error {
	if app == nil {
		return errors.New("playback: no app")
	}
	seen := map[string]bool{}
	for i, s := range app.Streams {
		name := s.Schema.Stream()
		switch {
		case name == "" || s.Schema.NumAttrs() == 0:
			return fmt.Errorf("playback: stream %d has no schema", i)
		case seen[name]:
			return fmt.Errorf("playback: stream %q is declared twice", name)
		case s.Delay < 0 || s.IdleAfter < 0:
			return fmt.Errorf("playback: stream %q has a negative bound", name)
		}
		seen[name] = true
	}
	seen = map[string]bool{}
	for i, t := range app.Tables {
		switch {
		case t.Name == "":
			return fmt.Errorf("playback: table %d has no name", i)
		case seen[t.Name]:
			return fmt.Errorf("playback: table %q is declared twice", t.Name)
		}
		seen[t.Name] = true
	}
	return nil
}

// Next is the next record; io.EOF after the last; a *LoadError at the first fault, after which it returns the same.
func (r *Reader) Next() (Record, error) {
	if r.err != nil {
		return Record{}, r.err
	}
	for {
		raw, err := r.readLine()
		if errors.Is(err, io.EOF) {
			return Record{}, io.EOF
		}
		r.line++
		if errors.Is(err, errLongLine) {
			r.err = r.fault("the line is longer than %d bytes", maxLine)
			return Record{}, r.err
		}
		if err != nil {
			r.err = err
			return Record{}, err
		}
		if blank(raw) {
			continue
		}
		rec, err := r.record(raw)
		if err != nil {
			r.err = err
			return Record{}, err
		}
		r.records++
		return rec, nil
	}
}

// readLine is the next line without its line ending; io.EOF when there is none.
func (r *Reader) readLine() ([]byte, error) {
	var long []byte
	for {
		chunk, err := r.br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			long = append(long, chunk...)
			if len(long) > maxLine+2 {
				return nil, errLongLine
			}
			continue
		}
		if long != nil {
			chunk = append(long, chunk...)
		}
		if errors.Is(err, io.EOF) && len(chunk) == 0 {
			return nil, io.EOF
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if n := len(chunk); n > 0 && chunk[n-1] == '\n' {
			chunk = chunk[:n-1]
		}
		if n := len(chunk); n > 0 && chunk[n-1] == '\r' {
			chunk = chunk[:n-1]
		}
		if len(chunk) > maxLine {
			return nil, errLongLine
		}
		return chunk, nil
	}
}

// blank says a line holds only JSON whitespace.
func blank(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			return false
		}
	}
	return true
}

func (r *Reader) fault(format string, args ...any) *LoadError {
	return &LoadError{Line: r.line, Msg: fmt.Sprintf(format, args...)}
}

// record decodes one non-blank line (design note §5.2).
func (r *Reader) record(line []byte) (Record, error) {
	vals, info, err := r.env.Decode(line, r.envBuf[:0])
	if err != nil {
		var de *event.DecodeError
		if errors.As(err, &de) {
			return Record{}, r.fault("not a log record: %s", de.Msg)
		}
		return Record{}, r.fault("not a log record: %v", err)
	}
	r.envBuf = vals
	if info.Unknown > 0 {
		return Record{}, r.fault("a key the log format does not define")
	}
	if r.stopped {
		return Record{}, r.fault("a record after the stop record")
	}
	present := func(i int) bool { return !vals[i].Null }
	kind := -1
	for k := range recordKinds {
		if present(recordKinds[k].key) {
			kind = k
			break
		}
	}
	if kind < 0 {
		return Record{}, r.fault("a record names none of recording, table, batch_end, tick, stop or stream")
	}
	rk := &recordKinds[kind]
	for i := range eCount {
		if present(i) && !rk.allowed[i] {
			return Record{}, r.fault("a %s record has no %q key", rk.name, envelopeAttrs[i].Name)
		}
	}
	switch rk.kind {
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
	}
	return r.eventRecord(vals, present)
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
	var op TableOp
	switch vals[eOp].Str {
	case "put":
		op = OpPut
	case "delete":
		op = OpDelete
	default:
		return Record{}, r.fault("table %q: op %q is neither put nor delete", name, vals[eOp].Str)
	}
	dec := r.tput[tid]
	if op == OpDelete {
		dec = r.tdel[tid]
	}
	row, info, err := dec.Decode([]byte(vals[eRow].Obj.(string)), nil)
	if err != nil {
		return Record{}, r.fault("table %q: the row: %s", name, decodeMsg(err))
	}
	if info.Unknown > 0 {
		if op == OpDelete {
			return Record{}, r.fault("table %q: a delete row holds only the key columns", name)
		}
		return Record{}, r.fault("table %q: the row has a column the table does not declare", name)
	}
	if op == OpPut {
		for _, k := range r.tkey[tid] {
			if row[k].Null {
				return Record{}, r.fault("table %q: key column %q is null", name, r.app.Tables[tid].Attrs[k].Name)
			}
		}
	}
	return Record{Line: r.line, Kind: RecTable, Table: tid, Op: op, Row: row}, nil
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
	switch {
	case !present(eAttrs):
		return Record{}, r.fault("an event needs attrs")
	case arrivalTime && !present(eTs):
		return Record{}, r.fault("stream %q is `time arrival`: its events carry ts", name)
	case !arrivalTime && present(eTs):
		return Record{}, r.fault("ts on stream %q, whose time is its attribute %q", name, st.Schema.Attr(st.Schema.TimeAttr()).Name)
	case !r.recording && present(eArrival):
		return Record{}, r.fault("arrival in a log without a recording header")
	case r.recording && arrivalTime && present(eArrival):
		return Record{}, r.fault("arrival on `time arrival` stream %q: its ts is the arrival", name)
	case r.recording && !arrivalTime && !present(eArrival):
		return Record{}, r.fault("an event in a recording carries arrival")
	case present(eBatch) && vals[eBatch].Str == "":
		return Record{}, r.fault("an empty batch id")
	}
	values, info, err := r.sdec[sid].Decode([]byte(vals[eAttrs].Obj.(string)), nil)
	if err != nil {
		var de *event.DecodeError
		errors.As(err, &de)
		return Record{}, &LoadError{Line: r.line, Msg: fmt.Sprintf("the payload fails stream %q's schema: %s", name, decodeMsg(err)), Decode: de}
	}
	ts := info.Ts
	if arrivalTime {
		ts = vals[eTs].T
	}
	var arrival int64
	if r.recording {
		arrival = ts
		if !arrivalTime {
			arrival = vals[eArrival].T
		}
		if arrival < r.last {
			return Record{}, r.fault("an arrival before the previous arrival or tick")
		}
		r.last = arrival
	}
	var batch string
	if present(eBatch) {
		batch = vals[eBatch].Str
	}
	return Record{Line: r.line, Kind: RecEvent, Arrival: arrival,
		Event: event.Event{StreamID: sid, Ts: ts, Values: values, BatchID: batch}}, nil
}

// decodeMsg is a decode error's message without the package prefix.
func decodeMsg(err error) string {
	var de *event.DecodeError
	if errors.As(err, &de) {
		return de.Msg
	}
	return err.Error()
}
