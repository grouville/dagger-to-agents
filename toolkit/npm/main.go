package main

import (
	"context"

	"npm/internal/dagger"
)

type Npm struct {
	Ctr    *dagger.Container
}

// NPM related tools that include:
// - Coverage
// One instance can be reused many times.
func New(
	// +optional
	ctr *dagger.Container,
) *Npm {
	if ctr == nil {
		ctr = dag.Container().From("node")
	}
	return &Npm{ctr}
}

// Coverage runs the Vitest coverage command on the provided source directory and returns its stdout
func (m *Npm) Coverage(ctx context.Context, source *dagger.Directory) (string, error) {
	ctr := m.Ctr.
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"npm", "install", "--save-dev", "vitest", "@vitest/coverage-v8"})

	// Run the coverage command
	return ctr.
		WithExec([]string{"npx", "vitest", "run", "--coverage"}).
		Stdout(ctx)
}
