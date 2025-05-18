package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tests you wanna iterate on
var allEvals = []EvalFunc{
	TestTrivyScan,
	NpmAudit,
}

func TestTrivyScan(
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
					require.NotEmpty(t, out)

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
				`run the test coverage`,
				func(env *TestEnv) *TestEnv {
					return env.
						WithStringOutput("npmAuditOutput", "The final result to store the NPM audit output")
					// WithDirectoryInput("workdir", ec.runner.Target, "the current project's directory") // it is implied, in MCP
				},
				func(ctx context.Context, t testing.TB, env *TestEnv) {
					out, err := env.Output("npmAuditOutput").AsString(ctx)
					require.NoError(t, err)

					require.Contains(t, out, "HelloWorld.vue")
				},
			},
			{
				`Now improve the test coverage of TheWelcome.vue to 100%. You can copy some other examples present in the project and write the tests next to it. The test coverage should make sense and should work. Always test until the end AND continue until you have a working and passing test.`,
				func(env *TestEnv) *TestEnv {
					return env.WithStringOutput("trivyOutput", "Trivy scan output")
				},
				func(ctx context.Context, t testing.TB, env *TestEnv) {
					out, err := env.Output("trivyOutput").AsString(ctx)
					require.NoError(t, err)

					require.Contains(t, out, "Report")
				},
			},
		}...,
	)
}
