package main

import (
	"context"

	"npm/internal/dagger"
)

type Npm struct {
	Ctr    *dagger.Container
	Source *dagger.Directory
}

// NPM related tools that work with any given source directory. Tools include:
// - InstallDependencies
// - Coverage
func New(
	// +optional
	ctr *dagger.Container,
	source *dagger.Directory,
) *Npm {
	if ctr == nil {
		ctr = dag.Container().From("node")
	}
	return &Npm{ctr, source}
}

// InstallDependencies installs the necessary dependencies for running tests and coverage
func (m *Npm) InstallDependencies(ctx context.Context) error {
	// Install vitest and coverage dependencies
	_, err := m.Ctr.
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec([]string{"npm", "install", "--save-dev", "vitest", "@vitest/coverage-v8"}).
		Stdout(ctx)
	return err
}

// Coverage runs the Vitest coverage command and returns its stdout
func (m *Npm) Coverage(ctx context.Context) (string, error) {
	// Ensure dependencies are installed
	if err := m.InstallDependencies(ctx); err != nil {
		return "", err
	}

	// Run the coverage command
	return m.Ctr.
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec([]string{"npx", "vitest", "run", "--coverage"}).
		Stdout(ctx)
}
