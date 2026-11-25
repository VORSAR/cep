package cep

import (
	"fmt"

	cepio "github.com/vorsar/cep/io"
)

// DeployOption configures one Deploy. A repeated WithParam for one name, WithSource for one stream, WithRecording
// or WithConnectors is an error wrapping ErrRepeatedOption; WithSink and WithTableRows may repeat.
type DeployOption func(*deployOptions) error

type deployOptions struct {
	params     map[string]string
	sources    map[string]cepio.Source
	sinks      map[string][]cepio.Sink
	sinkOrder  []string // the streams WithSink named, first call first
	tableRows  map[string][]func(cepio.RowWriter) error
	recording  cepio.Writer
	recorded   bool
	connectors ConnectorFactory
}

func newDeployOptions() *deployOptions {
	return &deployOptions{
		params: map[string]string{}, sources: map[string]cepio.Source{}, sinks: map[string][]cepio.Sink{},
		tableRows: map[string][]func(cepio.RowWriter) error{},
	}
}

// WithParam overrides a ${param}: literal is written as the rule language writes a default (SPEC §2.4).
func WithParam(name, literal string) DeployOption {
	return func(o *deployOptions) error {
		if _, dup := o.params[name]; dup {
			return fmt.Errorf("%w: WithParam(%q)", ErrRepeatedOption, name)
		}
		o.params[name] = literal
		return nil
	}
}

// WithSource binds s to a declared source stream; one per stream; it replaces the program's @source for it.
func WithSource(stream string, s cepio.Source) DeployOption {
	return func(o *deployOptions) error {
		if _, dup := o.sources[stream]; dup {
			return fmt.Errorf("%w: WithSource(%q)", ErrRepeatedOption, stream)
		}
		if s == nil {
			return fmt.Errorf("cep: WithSource(%q): a nil source", stream)
		}
		o.sources[stream] = s
		return nil
	}
}

// WithSink binds s to a derived stream; any number per stream, in call order; it replaces the program's @sink for it.
func WithSink(stream string, s cepio.Sink) DeployOption {
	return func(o *deployOptions) error {
		if s == nil {
			return fmt.Errorf("cep: WithSink(%q): a nil sink", stream)
		}
		if _, seen := o.sinks[stream]; !seen {
			o.sinkOrder = append(o.sinkOrder, stream)
		}
		o.sinks[stream] = append(o.sinks[stream], s)
		return nil
	}
}

// WithTableRows gives a table its starting rows, written at Start after the recording's header and before any
// source opens. The fills run at Deploy.
func WithTableRows(table string, rows ...func(cepio.RowWriter) error) DeployOption {
	return func(o *deployOptions) error {
		o.tableRows[table] = append(o.tableRows[table], rows...)
		return nil
	}
}

// WithRecording turns the app's recording on, written to w. Recording is off unless this option is given.
func WithRecording(w cepio.Writer) DeployOption {
	return func(o *deployOptions) error {
		if o.recorded {
			return fmt.Errorf("%w: WithRecording", ErrRepeatedOption)
		}
		if w == nil {
			return fmt.Errorf("cep: WithRecording: a nil writer")
		}
		o.recording, o.recorded = w, true
		return nil
	}
}

// WithConnectors binds the program's @source and @sink annotations through f.
func WithConnectors(f ConnectorFactory) DeployOption {
	return func(o *deployOptions) error {
		if o.connectors != nil {
			return fmt.Errorf("%w: WithConnectors", ErrRepeatedOption)
		}
		if f == nil {
			return fmt.Errorf("cep: WithConnectors: a nil factory")
		}
		o.connectors = f
		return nil
	}
}
