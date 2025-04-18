// NPM related tools that works with any given source directory

package main

import (
	"context"

	"npm/internal/dagger"
)

type Npm struct {
	Ctr    *dagger.Container
	Source *dagger.Directory
}

// NPM related tools that works with any given source directory
func New(
	// +optional
	ctr *dagger.Container,
	// +defaultPath="/"
	source *dagger.Directory,
) *Npm {
	if ctr == nil {
		ctr = dag.Container().From("node")
	}
	return &Npm{ctr, source}
}

// Coverage runs the Vitest coverage command and returns its stdout
func (m *Npm) Coverage(ctx context.Context) (string, error) {
	// TODO: add npm cache ?
	return m.Ctr.WithMountedDirectory("/src", m.Source).WithWorkdir("/src").WithExec([]string{"npx", "vitest", "run", "--coverage"}).Stdout(ctx)
}
