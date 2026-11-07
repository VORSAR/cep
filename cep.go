// Package cep is the engine's public API (ADR-0001 §2; design s3-3-1 §3): a Runtime that deploys rule programs as
// apps, starts and stops them, feeds them rows and hands their outputs to sinks. It holds no goroutine, no channel
// and no shared mutable state: every lifecycle step runs inside the engine, and this package converts, binds and
// forwards. Stubs until the build's step 5.
package cep

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/vorsar/cep/internal/runtime"
)

// Info is the build this binary embeds.
type Info struct{ Module, Version, GoVersion string }

// BuildInfo reads runtime/debug.ReadBuildInfo; Version is "(devel)" outside a module build.
func BuildInfo() Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return Info{}
	}
	return Info{Module: bi.Main.Path, Version: bi.Main.Version, GoVersion: bi.GoVersion}
}

// The errors a caller sees. The engine's own errors are mapped onto these at every call (mapErr), so errors.Is
// works against the facade's values; archcheck's globals rule allows error sentinels only.
var (
	ErrClosed            = errors.New("cep: the runtime is closed")
	ErrInactive          = errors.New("cep: the app is deployed but not started")
	ErrStarted           = errors.New("cep: the app is already started")
	ErrStopped           = errors.New("cep: the app is stopped")
	ErrUndeployed        = errors.New("cep: the app is undeployed")
	ErrUnknownStream     = errors.New("cep: no such stream")
	ErrUnknownTable      = errors.New("cep: no such table")
	ErrNotASourceStream  = errors.New("cep: not a source stream")
	ErrNotADerivedStream = errors.New("cep: not a derived stream")
	ErrSinkMode          = errors.New("cep: the sink does not accept the stream's output mode")
	ErrConnectorNotBuilt = errors.New("cep: the connector named in the program is not built yet")
	ErrRepeatedOption    = errors.New("cep: a deploy option was given twice")
	ErrNotAKeyColumn     = errors.New("cep: a delete key may set primary-key columns only")
	ErrRowUsedAfterFill  = errors.New("cep: the RowWriter was used after its fill returned")
)

var errNotBuilt = errors.New("cep: not built yet (S3.3-1)")

// mapErr turns the engine's lifecycle errors into the facade's sentinels, keeping the engine's text as the wrap.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, runtime.ErrClosed):
		return fmt.Errorf("%w: %v", ErrClosed, err)
	case errors.Is(err, runtime.ErrInactive):
		return fmt.Errorf("%w: %v", ErrInactive, err)
	case errors.Is(err, runtime.ErrStarted):
		return fmt.Errorf("%w: %v", ErrStarted, err)
	case errors.Is(err, runtime.ErrDrained):
		return fmt.Errorf("%w: %v", ErrStopped, err)
	case errors.Is(err, runtime.ErrRemoved):
		return fmt.Errorf("%w: %v", ErrUndeployed, err)
	}
	return err
}
