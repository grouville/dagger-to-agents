package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"dagger/hello-dagger/internal/dagger"
)

type HelloDagger struct{}

// Publish the application container after building and testing it on-the-fly
func (m *HelloDagger) Publish(
	ctx context.Context,
	// +defaultPath="/hello-dagger"
	source *dagger.Directory,
) (string, error) {
	_, err := m.Test(ctx, source)
	if err != nil {
		return "", err
	}
	return m.Build(source).
		Publish(ctx, fmt.Sprintf("ttl.sh/hello-dagger-%.0f", math.Floor(rand.Float64()*10000000))) //#nosec
}

// Build the application container
func (m *HelloDagger) Build(
	// +defaultPath="/hello-dagger"
	source *dagger.Directory,
) *dagger.Container {
	build := m.BuildEnv(source).
		WithExec([]string{"npm", "run", "build"}).
		Directory("./dist")
	return dag.Container().From("nginx:1.25-alpine").
		WithDirectory("/usr/share/nginx/html", build).
		WithExposedPort(80)
}

// Return the result of running unit tests
func (m *HelloDagger) Test(
	ctx context.Context,
	// +defaultPath="/hello-dagger"
	source *dagger.Directory,
) (string, error) {
	return m.BuildEnv(source).
		WithExec([]string{"npm", "run", "test:unit", "run"}).
		Stdout(ctx)
}

// Build a ready-to-use development environment
func (m *HelloDagger) BuildEnv(
	// +defaultPath="/hello-dagger"
	source *dagger.Directory,
) *dagger.Container {
	nodeCache := dag.CacheVolume("node")
	return dag.Container().
		From("node:21-slim").
		WithDirectory("/src", source).
		WithMountedCache("/root/.npm", nodeCache).
		WithWorkdir("/src").
		WithExec([]string{"npm", "install"})
}

func (m *HelloDagger) RunEvals(
	ctx context.Context,
	// +defaultPath="/hello-dagger"
	project *dagger.Directory,

	// +optional
	llmKey *dagger.Secret,
	daggerCli *dagger.File,
	dockerSocket *dagger.Socket,

	// +optional
	models []string,
) (*EvalReport, error) {
	// var reports []*EvalReport

	// default to all available models
	// workaround as //+ default does not work with slices
	// if models == nil {
	// 	models = []string{
	// 		"gpt-4o",
	// 		// "gpt-4.1",
	// 	}
	// }

	// for _, model := range models {
	// 	// one evaluator struct per model

	// // Eval #1
	// r1, err := ev.NPMAudit(ctx, project)
	// if err != nil {
	// 	return nil, fmt.Errorf("model %s NPMAudit: %w", model, err)
	// }
	// reports = append(reports, r1)

	// Eval #2
	// r2, err := GooseTrivyScan(ctx, project, driver)
	ev := NewEvalRunner("gpt-4o", "", daggerCli, project)
	r2, err := NpmAudit(ctx, EvalContext{
		// r2, err := TrivyScan(ctx, EvalContext{
		runner: ev,
		driver: DaggerShellDriver{},
	})
	if err != nil {
		return nil, fmt.Errorf("model %s TrivyScan: %w", "gpt-4o", err)
	}
	// 	reports = append(reports, r2)
	// }

	return r2, nil
}
