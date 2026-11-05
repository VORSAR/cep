package playback

import (
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/plan"
)

// FromPlan is the format layer's view of p: its streams and tables (design s3-2b-1 §4).
func FromPlan(p *plan.App) *App { panic("playback: FromPlan is a stub (S3.2b-1)") }

// NewPlanDriver makes a driver that replays through the runtime with p's rules on one shard (runtime.Sync); out
// receives every output in emission order, during the step that emits it.
func NewPlanDriver(p *plan.App, out func(e event.Event)) (*Driver, error) {
	panic("playback: NewPlanDriver is a stub (S3.2b-1)")
}
