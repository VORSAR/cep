package cep

import cepio "github.com/vorsar/cep/io"

// ConnectorFactory builds the connector a program's annotation names. keys are the annotation's items other than
// `type`, each value as the rule language writes it (a string without its quotes; any other literal in its canonical
// text). An unknown type returns an error wrapping ErrConnectorNotBuilt.
type ConnectorFactory interface {
	Source(typ string, keys map[string]string) (cepio.Source, error)
	Sink(typ string, keys map[string]string) (cepio.Sink, error)
}
