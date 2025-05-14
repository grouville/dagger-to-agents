package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

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
				func(env *TestEnv) *TestEnv {
					return env.WithStringOutput("imageRef", "Published docker image")
				},
				func(ctx context.Context, t testing.TB, env *TestEnv) {
					out, err := env.Output("imageRef").AsString(ctx)
					require.NoError(t, err)

					fmt.Fprintf(os.Stderr, "ImageRef: %s\n", out)
					require.Contains(t, out, "ttl.sh/hello-dagger-")
				},
			},
			{
				`check for its vulnerabilities`,
				func(env *TestEnv) *TestEnv {
					return env.WithStringOutput("trivyOutputString", "Trivy scan output string")
				},
				func(ctx context.Context, t testing.TB, env *TestEnv) {
					out, err := env.Output("trivyOutputString").AsString(ctx)
					require.NoError(t, err)

					require.Contains(t, out, "Report")
				},
			},
			{
				`summarize the result and give me action items`,
				func(env *TestEnv) *TestEnv {
					return env.WithStringOutput("actionItems", "Action items list")
				},
				func(ctx context.Context, t testing.TB, env *TestEnv) {
					_, err := env.Output("actionItems").AsString(ctx)
					require.NoError(t, err)
				},
			},
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
				func(env *TestEnv) *TestEnv {
					return env.
						WithStringOutput("npmAuditOutput", "The final result to store the NPM audit output").
						WithDirectoryInput("workdir", ec.runner.Target, "the current project's directory")
				},
				func(ctx context.Context, t testing.TB, env *TestEnv) {
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
