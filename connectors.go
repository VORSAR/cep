package cep

import (
	"fmt"

	cepio "github.com/vorsar/cep/io"
)

// ConnectorFactory builds the connector a program's annotation names. keys are the annotation's items other than
// `type`, each value as the rule language writes it (a string without its quotes; an int, float or bool as written;
// a duration as "<ms> ms"; a timestamp as its unix milliseconds). An unknown type returns an error wrapping
// ErrConnectorNotBuilt.
type ConnectorFactory interface {
	Source(typ string, keys map[string]string) (cepio.Source, error)
	Sink(typ string, keys map[string]string) (cepio.Sink, error)
}

// defaultFactory refuses every annotation-bound type: this release builds none; a Go binding (WithSource, WithSink)
// replaces the annotation.
type defaultFactory struct{}

func notBuilt(kind, typ string) error {
	if typ == "inmemory" {
		return fmt.Errorf("%w: @%s(type = %q) has no annotation form: bind it in Go with With%s", ErrConnectorNotBuilt, kind,
			typ, title(kind))
	}
	return fmt.Errorf("%w: @%s(type = %q) is not available in this release; bind a Go %s with With%s",
		ErrConnectorNotBuilt, kind, typ, kind, title(kind))
}

func title(kind string) string {
	if kind == "" {
		return ""
	}
	return string(kind[0]-'a'+'A') + kind[1:]
}

func (defaultFactory) Source(typ string, keys map[string]string) (cepio.Source, error) {
	return nil, notBuilt("source", typ)
}

func (defaultFactory) Sink(typ string, keys map[string]string) (cepio.Sink, error) {
	return nil, notBuilt("sink", typ)
}
