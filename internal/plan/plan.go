// Package plan turns a checked model (validate.Check's result) into an app's executable description (ARCHITECTURE §5
// step 1): the streams, the tables, and per rule its routing and its operator chain. It is pure: no goroutines, no
// I/O, no clock reads. Design: docs/sprints/s3-2b-1-design.md.
package plan

import (
	"errors"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
	"github.com/vorsar/cep/internal/partition"
)

// App is one app's executable description.
type App struct {
	Streams []Stream // declaration order: the index is the stream id (SEMANTICS §2.2)
	Tables  []Table  // declaration order: the index is the table id
	Rules   []Rule   // declaration order
}

// Stream is one declared stream.
type Stream struct {
	Schema    event.Schema // TimeAttr() == event.ArrivalTime for `time arrival`
	Delay     int64        // d, ms: the bound; 0 for ascending and arrival
	IdleAfter int64        // ms; 0: none
	Derived   bool         // no time clause, produced by a rule
}

// Table is one declared table.
type Table struct {
	Name  string
	Attrs []event.Attr
	Key   []string
}

// Rule is one rule: its routing (the runtime's routing table) and its body.
type Rule struct {
	Name      string
	Inputs    []int      // the stream ids it reads
	KeyAttr   []int      // per input, the key attribute's index; nil: key-less (the pseudo-key "")
	KeyType   event.Type // TString for a key-less rule
	TTL       int64      // ms; 0 for a key-less rule
	OnArrival bool       // a plain body: sees each event as its stream accepts it (owner Q1 of S3.2b)
	NewBody   func(emit func(event.Event)) partition.Body
}

// Stats are one rule's emission counters on one shard. Every body NewBody returns implements Counted.
type Stats struct {
	Suppressed uint64 // outputs the rate limit suppressed (suppressed_outputs, SEMANTICS §8)
	Errored    uint64 // events an expression error dropped for this rule (SEMANTICS §10)
}

// Counted is a body that reports its Stats.
type Counted interface{ Stats() Stats }

// ErrNotBuilt is a body kind this release cannot run yet: patterns (S4.1) and windows (S8).
var ErrNotBuilt = errors.New("plan: this body kind is not built yet")

// Build describes app, which MUST be validate.Check's result (no findings but lints).
func Build(app *model.App) (*App, error) {
	panic("plan: Build is a stub (S3.2b-1)")
}
