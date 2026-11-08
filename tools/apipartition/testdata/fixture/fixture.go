// Package fixture is apipartition's test subject: one declaration of every shape the tool prints or hides.
package fixture

import "context"

// Added is an exported function; the delta test's golden lacks it.
func Added() {}

// Handle is an exported type with one exported and one unexported method, and a struct with an exported and an
// unexported field.
type Handle struct {
	// Public is printed.
	Public int
	hidden string // hidden is not
	// Count is printed, aligned with Public.
	Count int64
}

// Name is Handle's exported method.
func (h *Handle) Name() string { return h.hidden }

func (h *Handle) secret() int { return h.Public }

// Alias is a type alias; its "=" is kept.
type Alias = Handle

// Version is an exported var.
var Version = "1"

// Limit is an exported const.
const Limit = 10

// helper is unexported: absent.
func helper() {}

// priv is unexported: absent, but its exported method is reachable through Open's result.
type priv struct{}

// Reach is priv's exported method.
func (p *priv) Reach() int { return 0 }

// Open returns the unexported type.
func Open() *priv { return &priv{} }

// Map is a generic function.
func Map[T any](xs []T, f func(T) T) []T {
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// Run has a context parameter and two results.
func Run(ctx context.Context, n int) (int, error) { return n, ctx.Err() }
