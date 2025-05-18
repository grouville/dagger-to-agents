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
	// +defaultPath="."
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
	// +defaultPath="."
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
	// +defaultPath="."
	source *dagger.Directory,
) (string, error) {
	return m.BuildEnv(source).
		WithExec([]string{"npm", "run", "test:unit", "run"}).
		Stdout(ctx)
}

// Build a ready-to-use development environment
func (m *HelloDagger) BuildEnv(
	// +defaultPath="."
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
	// +defaultPath="."
	project *dagger.Directory,

	// +optional
	llmKey *dagger.Secret,
	daggerCli *dagger.File,
	dockerSocket *dagger.Socket,

	// +optional
	models []string,
) (*EvalReport, error) {
	ev := NewEvalRunner(daggerCli, project, dockerSocket, llmKey)
	ev = ev.WithModel("gpt-4.1")
	ev = ev.WithSystemPrompt("")
	// r2, err := TrivyScan(ctx, EvalContext{
	r2, err := TestTrivyScan(ctx, EvalContext{
		runner: ev,
		// driver: DaggerShellDriver{},
		driver: GooseDriver{},
	})
	if err != nil {
		return nil, fmt.Errorf("model %s TrivyScan: %w", "gpt-4o", err)
	}

	return r2, nil
}

// func (m *HelloDagger) RunEvals(
// 	ctx context.Context,
// 	// +defaultPath="."
// 	project *dagger.Directory,

// 	// +optional
// 	llmKey *dagger.Secret,
// 	daggerCli *dagger.File,
// 	dockerSocket *dagger.Socket,

// 	// +optional
// 	models []string,
// 	drivers []string,
// ) ([]*EvalReport, error) {
// 	var reports []*EvalReport

// 	keyDriver := map[string]LLMTestClientDriver{
// 		"trivy": DaggerShellDriver{},
// 		"goose": GooseDriver{},
// 	}

// 	for _, driver := range drivers {
// 		d := keyDriver[driver]
// 		for _, eval := range allEvals {
// 			for _, model := range models {
// 				ev := NewEvalRunner(daggerCli, project, dockerSocket, llmKey)
// 				ev = ev.WithModel(model)
// 				ev = ev.WithSystemPrompt("")

// 				report, err := eval(ctx, EvalContext{
// 					driver: d,
// 					runner: ev,
// 				})
// 				if err != nil {
// 					return nil, fmt.Errorf("failed to run eval: %w", err)
// 				}
// 				reports = append(reports, report)
// 			}
// 		}
// 	}

// 	return reports, nil
// }
