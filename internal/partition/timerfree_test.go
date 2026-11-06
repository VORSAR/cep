package partition_test

import (
	"testing"

	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/partition"
)

// TestTimerFreeRefusesATimer: a timer reaching a timer-free body is a fault, never dropped in silence (combined
// review C9 of S3.2b-1): OnTimers panics, which partition counts as the body's panic and resets the key.
func TestTimerFreeRefusesATimer(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("TimerFree.OnTimers dropped a timer in silence; want a panic partition counts")
		}
	}()
	partition.TimerFree{}.OnTimers(nil, []clock.Timer{{Kind: clock.Completion}}, nil)
}
