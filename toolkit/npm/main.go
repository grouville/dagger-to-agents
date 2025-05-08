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

func (m *Npm) installDependencies(ctx context.Context, source *dagger.Directory) (*dagger.Container, error) {
	// Install vitest and coverage dependencies
	return m.Ctr.
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"npm", "install", "--save-dev", "vitest", "@vitest/coverage-v8"})
}

// Coverage runs the Vitest coverage command on the provided source directory and returns its stdout
func (m *Npm) Coverage(ctx context.Context, source *dagger.Directory) (string, error) {
	// Ensure dependencies are installed
	ctr, err := m.installDependencies(ctx, source)
	if err != nil {
		return "", err
	}

	// Run the coverage command
	return ctr.
		WithExec([]string{"npx", "vitest", "run", "--coverage"}).
		Stdout(ctx)
}
