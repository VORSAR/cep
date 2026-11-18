package playback_test

import (
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/playback"
)

// TestRunExpectsReportsAGivenTableRowTheTableRefuses: a given table row the sandbox's table refuses (two values for
// T's one column) is runBlock's error, named by rule, block and table, wrapping the table's own error (expect.go:
// 65-71), and RunExpects returns no results.
func TestRunExpectsReportsAGivenTableRowTheTableRefuses(t *testing.T) {
	t.Parallel()
	given := []plan.GivenTable{{Table: xpT, Rows: [][]event.Value{{xpStr("a"), xpInt(1)}}}}
	p := xpPlan(t, xpBlock(xpR, 1, given, nil))
	res, err := playback.RunExpects(p)
	const prefix = "playback: rule R, expect block 1: given table T: "
	if err == nil || !strings.HasPrefix(err.Error(), prefix) || !strings.Contains(err.Error(), "2 values for 1") ||
		res != nil {
		t.Errorf("RunExpects = %v, %v; want nil and %q with the table's refusal of 2 values for 1 column", res, err, prefix)
	}
}
