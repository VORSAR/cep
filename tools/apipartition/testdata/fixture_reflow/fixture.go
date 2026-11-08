// Package fixture is apipartition's test subject, formatted differently: the parameter list of Run broken over
// lines with a trailing comma, a blank line inside Handle, the doc comments reworded, bodies reshaped.
package fixture

import "context"

// Added: the same function, another comment.
func Added() {
}

// Handle, with a blank line between its fields.
type Handle struct {
	Public int

	hidden string

	Count int64
}

// Name, the same method.
func (h *Handle) Name() string {
	return h.hidden
}

func (h *Handle) secret() int {
	return h.Public
}

// Alias, the same alias.
type Alias = Handle

// Version, the same var.
var Version = "1"

// Limit, the same const.
const Limit = 10

func helper() {
}

type priv struct {
}

// Reach, the same method.
func (p *priv) Reach() int {
	return 0
}

// Open, the same function.
func Open() *priv {
	return &priv{}
}

// Map, the same generic function with a one-line body.
func Map[T any](xs []T, f func(T) T) []T { return xs }

// Run, its parameters one per line.
func Run(
	ctx context.Context,
	n int,
) (int, error) {
	return n, ctx.Err()
}
