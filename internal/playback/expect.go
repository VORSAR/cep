package playback

import (
	"errors"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

// Counts are a block's drops, read from the sandbox's Stats after the end (SEMANTICS §10; design s3-3-2 §5.1).
type Counts struct {
	Late      int64 // given events dropped as stream-late
	Errored   int64 // events an expression error dropped
	KeyErrors int64 // events a key error dropped
	Panics    int64 // the rule's recovered panics
}

// BlockResult is one expect block's run.
type BlockResult struct {
	Rule       string
	Block      int           // 1-based within the rule
	Outputs    []event.Event // the rule's outputs, in emission order
	Counts     Counts
	Divergence string // "" when the block passed; otherwise design s3-3-2 §5.4's text
	Line, Col  int    // where E-EXP-001 points (§5.4); 0, 0 when the block passed
}

// RunExpects runs every block of p, in p.Expects' order, each in a fresh sandbox: the owning rule alone, its own
// empty tables, no sinks, no recorder, no connectors. It runs on the caller's goroutine and is deterministic. The
// error is an engine fault, never a block that failed.
func RunExpects(p *plan.App) ([]BlockResult, error) {
	return nil, errors.New("playback: RunExpects: not built (S3.3-2 step 5)")
}
