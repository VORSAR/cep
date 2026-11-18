package playback

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/vorsar/cep/internal/event"
)

// maxLine is the longest line a log may hold (design note §5.2): event.MaxLogLine, which live intake honours too.
const maxLine = event.MaxLogLine

var errLongLine = errors.New("line too long")

// Reader reads a log one record at a time. Its first fault stops it: Next returns the same *LoadError from then on
// (owner D6 of S3.2a-1).
type Reader struct {
	br        *bufio.Reader
	app       *App
	env       *event.Decoder
	header    *event.Decoder
	rejected  *event.Decoder
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
	hdrSchema, err := event.NewSchema("recording", []event.Attr{{Name: "start", Type: event.TTimestamp}},
		event.ArrivalTime)
	if err != nil {
		return nil, fmt.Errorf("playback: the header schema: %w", err)
	}
	rejSchema, err := event.NewSchema("rejected", []event.Attr{{Name: "attr", Type: event.TString, Optional: true}, {
		Name: "reason", Type: event.TString}}, event.ArrivalTime)
	if err != nil {
		return nil, fmt.Errorf("playback: the rejected record's schema: %w", err)
	}
	rd := &Reader{
		br: bufio.NewReaderSize(r, 64<<10), app: app, env: event.NewDecoder(envSchema), header: event.NewDecoder(hdrSchema),
		rejected: event.NewDecoder(rejSchema),
		streams:  map[string]int{}, tables: map[string]int{},
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

// decodeMsg is a decode error's message without the package prefix.
func decodeMsg(err error) string {
	var de *event.DecodeError
	if errors.As(err, &de) {
		return de.Msg
	}
	return err.Error()
}
