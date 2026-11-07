package cep

import "context"

// AppHandle is one deployed app.
type AppHandle struct{ _ struct{} }

// StopReport is what Stop found.
type StopReport struct {
	DrainedPending int64            // body timers left unfired by the barrier (drained_pending)
	SourceErrors   map[string]error // per bound source stream: Open's non-nil return, context.Canceled excluded
}

// Name is the name Deploy was given.
func (a *AppHandle) Name() string { return "" }

// Lints are the findings that did not fail the deploy.
func (a *AppHandle) Lints() []Diagnostic { return nil }

// Start activates the app: its sinks, its tables' starting rows, then its sources last.
func (a *AppHandle) Start(ctx context.Context) error { return errNotBuilt }

// Stop quiets the app's ticks, closes its sources, drains it and reports.
func (a *AppHandle) Stop(ctx context.Context) (StopReport, error) { return StopReport{}, errNotBuilt }

// Undeploy stops the app if it is started and removes it.
func (a *AppHandle) Undeploy(ctx context.Context) error { return errNotBuilt }

// Input is the programmatic feed for a source stream.
func (a *AppHandle) Input(stream string) (*InputHandler, error) { return nil, errNotBuilt }

// Table is the write API for a declared table.
func (a *AppHandle) Table(name string) (*Table, error) { return nil, errNotBuilt }

// Stats is a consistent cut of the app's counters.
func (a *AppHandle) Stats(ctx context.Context) (Stats, error) { return Stats{}, errNotBuilt }
