package cep

import "context"

// Deploy parses, validates and plans src, binds its connectors and registers the app, inactive. Every parse or static
// defect is returned at once as a *DeployError; lints are on the handle. Expect blocks are not run in S3.3-1.
func (r *Runtime) Deploy(ctx context.Context, name, src string, opts ...DeployOption) (*AppHandle, error) {
	return nil, errNotBuilt
}
