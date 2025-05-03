package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"

	"dagger/hello-dagger/internal/dagger"

	"github.com/stretchr/testify/require"
)

// GooseTrivyScan runs a Trivy scan evaluation using the Goose driver setup.
// It now accepts dependencies directly.
// func GooseTrivyScan(
// 	ctx context.Context,
// 	target *dagger.Directory,
// 	ev *EvalRunner,
// ) (*dagger.Container, error) {
// 	ctr := gooseCtr(ctx, target, ev.DaggerCli, ev.DockerSocket, ev.LLMKey)
// 	ctr = ctr.WithWorkdir("/root").
// 		WithNewFile("llm-history", `{"working_dir":"/root","description":"Initial greeting exchange","message_count":2,"total_tokens":687,"input_tokens":673,"output_tokens":14,"accumulated_total_tokens":1373,"accumulated_input_tokens":1346,"accumulated_output_tokens":27}`).
// 		// WithExec([]string{"bash"})
// 		WithExec(sh("goose run -p llm-history -r -t hi"))
// 	// out, err := ctr.Stdout(ctx)
// 	return ctr, nil
// }

func TrivyScan(
	ctx context.Context,
	ec EvalContext,
) (*EvalReport, error) {
	return withLLMReport(
		ctx,
		ec,
		[]withLLMReportStep{
			{
				`publish the hello dagger app`,
				func(env *dagger.Env) *dagger.Env {
					return env.WithStringOutput("imageRef", "Published docker image")
				},
				func(ctx context.Context, t testing.TB, env *dagger.Env) {
					out, err := env.Output("imageRef").AsString(ctx)
					require.NoError(t, err)

					fmt.Fprintf(os.Stderr, "ImageRef: %s\n", out)
					require.Contains(t, out, "ttl.sh/hello-dagger-")
				},
			},
			{
				`check for its vulnerabilities`,
				func(env *dagger.Env) *dagger.Env {
					return env.WithStringOutput("trivyOutput", "Trivy scan output")
				},
				func(ctx context.Context, t testing.TB, env *dagger.Env) {
					out, err := env.Output("trivyOutput").AsString(ctx)
					require.NoError(t, err)

					require.Contains(t, out, "Report")
				},
			},
			// {
			// 	`summarize the result and give me action items`,
			// 	func(env *dagger.Env) *dagger.Env {
			// 		return env.WithStringOutput("actionItems", "Action items list")
			// 	},
			// 	func(ctx context.Context, t testing.TB, env *dagger.Env) {
			// 		out, err := env.Output("trivyOutput").AsString(ctx)
			// 		require.NoError(t, err)
			// 		// fmt.Fprintf(os.Stderr, "🥶debug: %s\n", out)
			// 		require.Contains(t, out, "Vulnerability", "VULNERABILITY")
			// 	},
			// },
		}...,
	)
}

// NPMAudit function removed as part of refactoring.
// It can be reimplemented using the new driver pattern if needed.
func NpmAudit(
	ctx context.Context,
	ec EvalContext,
) (*EvalReport, error) {
	return withLLMReport(
		ctx,
		ec,
		[]withLLMReportStep{
			{
				`run the test coverage and save the output`,
				func(env *dagger.Env) *dagger.Env {
					return env.
						WithStringOutput("npmAuditOutput", "The final result to store the NPM audit output").
						WithDirectoryInput("workdir", ec.runner.Target, "the current project's directory")

				},
				func(ctx context.Context, t testing.TB, env *dagger.Env) {
					out, err := env.Output("npmAuditOutput").AsString(ctx)
					require.NoError(t, err)

					// fmt.Fprintf(os.Stderr, "🔥NPM Audit: |%s|\n", out)
					require.Contains(t, out, "HelloWorld.vue")
				},
			},
			// {
			// 	`check for its vulnerabilities`,
			// 	func(env *dagger.Env) *dagger.Env {
			// 		return env.WithStringOutput("trivyOutput", "Trivy scan output")
			// 	},
			// 	func(ctx context.Context, t testing.TB, env *dagger.Env) {
			// 		out, err := env.Output("trivyOutput").AsString(ctx)
			// 		require.NoError(t, err)

			// 		require.Contains(t, out, "Report")
			// 	},
			// },
			// {
			// 	`summarize the result and give me action items`,
			// 	func(env *dagger.Env) *dagger.Env {
			// 		return env.WithStringOutput("actionItems", "Action items list")
			// 	},
			// 	func(ctx context.Context, t testing.TB, env *dagger.Env) {
			// 		out, err := env.Output("trivyOutput").AsString(ctx)
			// 		require.NoError(t, err)
			// 		// fmt.Fprintf(os.Stderr, "🥶debug: %s\n", out)
			// 		require.Contains(t, out, "Vulnerability", "VULNERABILITY")
			// 	},
			// },
		}...,
	)
}
