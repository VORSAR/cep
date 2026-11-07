// Package cepio holds the connector contracts (ADR-0008 §1; design s3-3-1 §5): what a source, a sink or a mapper
// implements, and the row boundary values cross. It imports nothing of the module and starts no goroutine.
package cepio

import "context"

// Type is an attribute's declared type (SPEC §2.3), as RowWriter.AttrType and RowReader.AttrType report it.
type Type uint8

// The seven schema types, and Invalid for an index outside the schema.
const (
	Invalid Type = iota
	String
	Int
	Float
	Bool
	Timestamp
	Duration
	Object
)

// Mode is what a sink accepts (SPEC §5.7): an append stream, or a changelog.
type Mode uint8

// The two output modes.
const (
	Append Mode = iota + 1
	Changelog
)

// RowWriter writes one row's typed values into engine-owned storage. It is valid only during the fill that received
// it; a Set after that fill returned panics. AttrIndex resolves a name once; AttrType says which Set to call.
type RowWriter interface {
	SetString(attr int, v string)
	SetInt(attr int, v int64)
	SetFloat(attr int, v float64)
	SetBool(attr int, v bool)
	SetTimestamp(attr int, unixMs int64)
	SetDuration(attr int, ms int64)
	SetObject(attr int, jsonText string) // an object enters as its JSON text (S3.1-2's B5)
	SetNull(attr int)
	AttrIndex(name string) int // -1 if unknown
	AttrType(attr int) Type    // Invalid outside the schema
}

// RowReader reads one output's typed values. It is valid until the Write that received it returns; a sink that keeps
// a value copies it (strings and JSON texts are immutable Go strings and may be kept).
type RowReader interface {
	String(attr int) (string, bool) // false: null, or not this type, or outside the schema
	Int(attr int) (int64, bool)
	Float(attr int) (float64, bool)
	Bool(attr int) (bool, bool)
	Timestamp(attr int) (int64, bool) // unix ms
	Duration(attr int) (int64, bool)  // ms
	Object(attr int) (string, bool)   // the JSON text
	IsNull(attr int) bool             // true outside the schema too
	AttrIndex(name string) int
	AttrType(attr int) Type
	Len() int
}

// Emitter is a source's way into one stream. Emit allocates the row, calls fill, checks it and ingests it; EndBatch
// closes batchID (every row of the batch was emitted). Both block for backpressure and return ctx's error when it ends
// first; both are safe for concurrent use. batchID "" on Emit means no batch; EndBatch("") is an error.
type Emitter interface {
	Emit(ctx context.Context, batchID string, fill func(RowWriter) error) error
	EndBatch(ctx context.Context, batchID string) error
}

// Source feeds a stream. Open runs on a goroutine the engine starts after the app's tables are loaded and returns
// when ctx is done (the engine cancels it first at Stop and waits for Open to return until Stop's deadline); its
// return value is reported in the stop report (context.Canceled excluded). Open MUST return when ctx is done, and
// MUST NOT call the app's Start, Stop, Undeploy or the runtime's Close: that deadlocks to the deadline.
type Source interface {
	Open(ctx context.Context, emit Emitter) error
}

// Sink receives one derived stream's outputs. Write is called from the engine's flusher goroutine only, one call at a
// time, in output order. A non-nil return is counted and the output is dropped: the engine never retries (owner,
// 2026-11-07); a sink that promises delivery retries itself. ctx is cancelled when the app is undeployed or the
// runtime's Close gives up at its deadline: Write MUST return then, and MUST NOT call the lifecycle.
type Sink interface {
	Accepts() Mode
	Write(ctx context.Context, e OutEvent) error
}

// Writer is where a recording goes: any io.Writer satisfies it (the facade may not import io, ADR-0015 §2.2).
type Writer interface {
	Write(p []byte) (int, error)
}

// Mapper decodes one payload into a row (the http and stdin sources' mappers, S5.1).
type Mapper interface {
	Decode(b []byte, w RowWriter) error
}

// OutEvent is one output. Row is valid until Write returns. Canonical is the output's canonical JSON (ADR-0003/0005;
// the log sink prints exactly this text), a fresh slice per output the sink may keep (owner, 2026-11-07).
type OutEvent struct {
	Stream    string
	Timestamp int64 // unix ms
	Row       RowReader
	Canonical []byte
}
