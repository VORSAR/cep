// Package embedded has an exported struct with an embedded field, which the tool refuses.
package embedded

// Base is a plain exported struct.
type Base struct{ X int }

// Wrapper embeds Base: its promoted methods are invisible to the surface text.
type Wrapper struct {
	Base
	Y int
}
