package cep

import cepio "github.com/vorsar/cep/io"

// DeployOption configures one Deploy.
type DeployOption func(*deployOptions) error

type deployOptions struct {
	params     map[string]string
	sources    map[string]cepio.Source
	sinks      map[string][]cepio.Sink
	tableRows  map[string][]func(cepio.RowWriter) error
	recording  cepio.Writer
	connectors ConnectorFactory
}

// WithParam overrides a ${param}: literal is written as the rule language writes a default.
func WithParam(name, literal string) DeployOption {
	return func(o *deployOptions) error { return errNotBuilt }
}

// WithSource binds s to a declared source stream; one per stream; replaces the program's @source for it.
func WithSource(stream string, s cepio.Source) DeployOption {
	return func(o *deployOptions) error { return errNotBuilt }
}

// WithSink binds s to a derived stream; any number per stream, in call order.
func WithSink(stream string, s cepio.Sink) DeployOption {
	return func(o *deployOptions) error { return errNotBuilt }
}

// WithTableRows gives a table its starting rows, written at Start before any source opens.
func WithTableRows(table string, rows ...func(cepio.RowWriter) error) DeployOption {
	return func(o *deployOptions) error { return errNotBuilt }
}

// WithRecording turns the app's recording on, written to w.
func WithRecording(w cepio.Writer) DeployOption {
	return func(o *deployOptions) error { return errNotBuilt }
}

// WithConnectors binds the program's @source and @sink annotations through f.
func WithConnectors(f ConnectorFactory) DeployOption {
	return func(o *deployOptions) error { return errNotBuilt }
}
