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

// defaultFactory refuses every type: no annotation-bound connector is built before S5.1 (design s3-3-1 §3.4). The
// message names the sprint that builds it.
type defaultFactory struct{}

var connectorSprint = map[string]string{
	"http": "S5.1", "stdin": "S5.1", "jsonmap": "S5.1", "json": "S5.1",
	"smtp": "S5.2", "email": "S5.2", "log": "S5.2", "textmap": "S5.2", "text": "S5.2",
}

func notBuilt(kind, typ string) error {
	if typ == "inmemory" {
		return fmt.Errorf("%w: @%s(type = %q) has no annotation form: bind it in Go with With%s", ErrConnectorNotBuilt, kind, typ, title(kind))
	}
	sprint, ok := connectorSprint[typ]
	if !ok {
		sprint = "a later sprint"
	}
	return fmt.Errorf("%w: @%s(type = %q) arrives in %s; bind a Go %s with With%s for now", ErrConnectorNotBuilt, kind, typ, sprint, kind, title(kind))
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
