package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"testing"

	"dagger/hello-dagger/internal/dagger"
	"dagger/hello-dagger/internal/telemetry"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
)

type EnvModifierFunc func(*dagger.Env) *dagger.Env

type LLMTestClient interface {
	// SetPrompt applies the given prompt and returns an updated driver instance and any error.
	SetPrompt(ctx context.Context, prompt string)

	// // History retrieves the message history.
	// History(ctx context.Context) ([]string, error)
	// // InputTokens retrieves the total input tokens used.
	// InputTokens(ctx context.Context) (int, error)
	// // OutputTokens retrieves the total output tokens used.
	// OutputTokens(ctx context.Context) (int, error)
	// // ToolsDoc retrieves the documentation for the tools available to the LLM.
	// ToolsDoc(ctx context.Context) (string, error)

	// SetEnv applies environment modifications using the provided function.
	SetEnv(ctx context.Context, fn EnvModifierFunc)
	// Retrieves the current environment following a test run.
	GetEnv(ctx context.Context) *dagger.Env
	// Run the LLM client driver with the given context.
	Run(ctx context.Context) error
}

type LLMTestClientDriver interface {
	NewTestClient(ev *EvalRunner) LLMTestClient
}

// The context for LLM evaluation, including the runner and driver.
// It is used to create a new LLMTestClient for the evaluation.
type EvalContext struct {
	runner *EvalRunner
	driver LLMTestClientDriver
}

func (ec EvalContext) NewClient() LLMTestClient {
	return ec.driver.NewTestClient(ec.runner)
}

// EvalReport holds the results of an LLM evaluation.
type EvalReport struct {
	Succeeded    bool
	Report       string
	ToolsDoc     string
	InputTokens  int
	OutputTokens int
}

// withLLMReportStep defines a single step in an LLM evaluation.
type withLLMReportStep struct {
	prompt string
	envOpt func(*dagger.Env) *dagger.Env
	check  func(context.Context, testing.TB, *dagger.Env)
}

// withLLMReport executes a series of steps using an LLMDriver and generates a report.
func withLLMReport(
	ctx context.Context,
	ec EvalContext,
	steps ...withLLMReportStep,
) (*EvalReport, error) {
	reportMD := new(strings.Builder)
	report := &EvalReport{}
	t := newT(ctx, "eval")

	tc := ec.NewClient()

	stop := false
	for _, step := range steps {
		var stepErr error

		if step.envOpt != nil {
			tc.SetEnv(ctx, step.envOpt)
		}

		if !stop && step.prompt != "" {
			tc.SetPrompt(ctx, step.prompt)
		}

		stepErr = tc.Run(ctx)
		// Run checks even if SetPrompt wasn't called (e.g., initial state check)
		(func() {
			// demarcate assertions from the eval
			ctx, span := Tracer().Start(ctx, "assert: "+step.prompt, telemetry.Reveal())
			defer func() {
				if t.Failed() {
					stop = true
					span.SetStatus(codes.Error, "assertions failed")
				} else if stepErr != nil { // Check stepErr which covers both envOpt and SetPrompt errors
					stop = true // Stop if the driver step itself failed
					span.SetStatus(codes.Error, fmt.Sprintf("driver step failed: %s", stepErr.Error()))
				}
				span.End()
			}()

			// capture test panics, from assertions, skips, or otherwise
			defer func() {
				x := recover()
				switch x {
				case nil:
				case testSkipped{}, testFailed{}:
				default:
					fmt.Fprintln(reportMD, "PANIC:", x)
					reportMD.Write(debug.Stack())
					fmt.Fprintln(reportMD)
				}
			}()

			// basic check: running the driver step succeeded (envOpt and/or prompt)
			require.NoError(t, stepErr, "LLM driver step did not complete")

			// run eval-specific assertions using the potentially updated driver
			if step.check != nil {
				step.check(ctx, t, tc.GetEnv(ctx))
			}
		}())

		if stop {
			break
		}
	}

	// // Generate report using the final driver state
	// fmt.Fprintln(reportMD, "### Message Log")
	// fmt.Fprintln(reportMD)
	// history, err := currentDriver.History(ctx)
	// if err != nil {
	// 	fmt.Fprintln(reportMD, "Failed to get history:", err)
	// } else {
	// 	numLines := len(history)
	// 	width := len(fmt.Sprintf("%d", numLines)) // Calculate width for padding
	// 	for i, line := range history {
	// 		fmt.Fprintf(reportMD, "    %*d | %s\n", width, i+1, line)
	// 	}
	// }

	// report.InputTokens, err = currentDriver.InputTokens(ctx)
	// if err != nil {
	// 	fmt.Fprintln(reportMD, "Failed to get input tokens:", err)
	// }
	// report.OutputTokens, err = currentDriver.OutputTokens(ctx)
	// if err != nil {
	// 	fmt.Fprintln(reportMD, "Failed to get output tokens:", err)
	// }
	// fmt.Fprintln(reportMD)

	// fmt.Fprintln(reportMD, "### Total Token Cost")
	// fmt.Fprintln(reportMD)
	// fmt.Fprintln(reportMD, "* Input Tokens:", report.InputTokens)
	// fmt.Fprintln(reportMD, "* Output Tokens:", report.OutputTokens)
	// fmt.Fprintln(reportMD)

	// fmt.Fprintln(reportMD, "### Evaluation Result")
	// fmt.Fprintln(reportMD)
	// if t.Failed() {
	// 	fmt.Fprintln(reportMD, t.Logs())
	// 	fmt.Fprintln(reportMD, "FAILED")
	// } else if t.Skipped() {
	// 	fmt.Fprintln(reportMD, t.Logs())
	// 	fmt.Fprintln(reportMD, "SKIPPED")
	// } else {
	// 	fmt.Fprintln(reportMD, "SUCCESS")
	// 	report.Succeeded = true
	// }

	// report.Report = reportMD.String()

	// toolsDoc, err := currentDriver.ToolsDoc(ctx)
	// if err != nil {
	// 	fmt.Fprintln(reportMD, "Failed to get tools:", err)
	// }
	// report.ToolsDoc = toolsDoc

	return report, nil
}

// EvalRunner holds common configuration for an evaluation run.
type EvalRunner struct {
	Model        string
	Attempt      int // >0, monotonically increasing so you can easily distinguish attempts
	SystemPrompt string

	// How could we make this cross platform ? Embed a CLI ? Package and extract it ?
	DaggerCli *dagger.File

	// To improve -- bug fixes
	DockerSocket *dagger.Socket // TODO: we need a way to not request this from the user -- either the CLI does it
	LLMKey       *dagger.Secret // Should also disappear -- either via EnvPrivileged or with a fix to *Env

	// The target directory to be used in the evaluation (temporary until it is automatically mounted)
	Target *dagger.Directory
}

func NewEvalRunner(model string, systemPrompt string, daggerCli *dagger.File, target *dagger.Directory) *EvalRunner {
	return &EvalRunner{
		Attempt:      1,
		Model:        model,
		SystemPrompt: systemPrompt,
		DaggerCli:    daggerCli,
		Target:       target,
	}
}

func (m *EvalRunner) WithAttempt(attempt int) *EvalRunner {
	m.Attempt = attempt
	return m
}

func (m *EvalRunner) WithModel(model string) *EvalRunner {
	m.Model = model
	return m
}

func (m *EvalRunner) WithSystemPrompt(prompt string) *EvalRunner {
	m.SystemPrompt = prompt
	return m
}
