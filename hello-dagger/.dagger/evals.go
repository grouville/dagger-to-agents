package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	// local Dagger SDK import that your hello‑dagger module already uses

	"dagger/hello-dagger/internal/dagger"
	"dagger/hello-dagger/internal/telemetry"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
)

type EvalRunner struct {
	Model        string
	Attempt      int // >0, monotonically increasing so you can easily distinguish attempts
	SystemPrompt string
}

func NewEvalRunner() *EvalRunner {
	return &EvalRunner{
		Attempt: 1,
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

func (m *EvalRunner) llm(opts ...dagger.LLMOpts) *dagger.LLM {
	opts = append(opts, dagger.LLMOpts{
		Model: m.Model,
	})
	llm := dag.LLM(opts...)
	if m.SystemPrompt != "" {
		llm = llm.WithSystemPrompt(m.SystemPrompt)
	}
	if m.Attempt > 0 {
		llm = llm.Attempt(m.Attempt)
	}
	return llm
}

type EvalReport struct {
	Succeeded    bool
	Report       string
	ToolsDoc     string
	InputTokens  int
	OutputTokens int
}

type withLLMReportStep struct {
	prompt string
	envOpt func(*dagger.Env) *dagger.Env
	check  func(context.Context, testing.TB, *dagger.LLM)
}

func withLLMReport(
	ctx context.Context,
	llm *dagger.LLM,
	steps ...withLLMReportStep,
) (*EvalReport, error) {
	reportMD := new(strings.Builder)

	report := &EvalReport{}

	t := newT(ctx, "eval")

	stop := false
	for _, step := range steps {
		if step.envOpt != nil {
			llm = llm.WithEnv(step.envOpt(llm.Env()))
		}
		if step.prompt != "" {
			llm = llm.WithPrompt(step.prompt)
		}

		evaledLlm, evalErr := llm.Sync(ctx)
		(func() {
			// demarcate assertions from the eval

			ctx, span := Tracer().Start(ctx, "assert", telemetry.Reveal())
			defer func() {
				if t.Failed() {
					stop = true
					span.SetStatus(codes.Error, "assertions failed")
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

			// basic check: running the evals succeeded without e.g. hitting API limits
			require.NoError(t, evalErr, "LLM evaluation did not complete")

			// now that we know it didn't error, re-assign
			llm = evaledLlm

			// run eval-specific assertions
			step.check(ctx, t, llm)
		}())

		if stop {
			break
		}
	}

	fmt.Fprintln(reportMD, "### Message Log")
	fmt.Fprintln(reportMD)
	history, err := llm.History(ctx)
	if err != nil {
		fmt.Fprintln(reportMD, "Failed to get history:", err)
	} else {
		numLines := len(history)
		// Calculate the width needed for the largest line number
		width := len(fmt.Sprintf("%d", numLines))
		for i, line := range history {
			// Format with right-aligned padding, number, separator, and content
			fmt.Fprintf(reportMD, "    %*d | %s\n", width, i+1, line)
		}
	}
	report.InputTokens, err = llm.TokenUsage().InputTokens(ctx)
	if err != nil {
		fmt.Fprintln(reportMD, "Failed to get input tokens:", err)
	}
	report.OutputTokens, err = llm.TokenUsage().OutputTokens(ctx)
	if err != nil {
		fmt.Fprintln(reportMD, "Failed to get output tokens:", err)
	}
	fmt.Fprintln(reportMD)

	fmt.Fprintln(reportMD, "### Total Token Cost")
	fmt.Fprintln(reportMD)
	fmt.Fprintln(reportMD, "* Input Tokens:", report.InputTokens)
	fmt.Fprintln(reportMD, "* Output Tokens:", report.OutputTokens)
	fmt.Fprintln(reportMD)

	fmt.Fprintln(reportMD, "### Evaluation Result")
	fmt.Fprintln(reportMD)
	if t.Failed() {
		fmt.Fprintln(reportMD, t.Logs())
		fmt.Fprintln(reportMD, "FAILED")
	} else if t.Skipped() {
		fmt.Fprintln(reportMD, t.Logs())
		fmt.Fprintln(reportMD, "SKIPPED")
	} else {
		fmt.Fprintln(reportMD, "SUCCESS")
		report.Succeeded = true
	}

	report.Report = reportMD.String()

	toolsDoc, err := llm.Tools(ctx)
	if err != nil {
		fmt.Fprintln(reportMD, "Failed to get tools:", err)
	}
	report.ToolsDoc = toolsDoc

	return report, nil
}

type evalT struct {
	*testing.T
	name    string
	ctx     context.Context
	logs    *strings.Builder
	failed  bool
	skipped bool
}

var _ testing.TB = (*evalT)(nil)

func newT(ctx context.Context, name string) *evalT {
	return &evalT{
		T:    &testing.T{}, // unused, has to be here because private()
		name: name,
		ctx:  ctx,
		logs: &strings.Builder{},
	}
}

func (e *evalT) Name() string {
	return e.name
}

func (e *evalT) Helper() {}

func (e *evalT) Logs() string {
	return e.logs.String()
}

func (e *evalT) Context() context.Context {
	return e.ctx
}

func (e *evalT) Error(args ...interface{}) {
	e.Log(args...)
	e.Fail()
}

func (e *evalT) Errorf(format string, args ...interface{}) {
	e.Logf(format, args...)
	e.Fail()
}

func (e *evalT) Log(args ...interface{}) {
	fmt.Fprintln(e.logs, args...)
}

func (e *evalT) Logf(format string, args ...interface{}) {
	fmt.Fprintf(e.logs, format+"\n", args...)
}

func (e *evalT) Fatal(args ...interface{}) {
	e.Log(args...)
	e.FailNow()
}

func (e *evalT) Fatalf(format string, args ...interface{}) {
	e.Logf(format, args...)
	e.FailNow()
}

func (e *evalT) Fail() {
	e.failed = true
}

type testFailed struct{}
type testSkipped struct{}

func (e *evalT) FailNow() {
	e.failed = true
	panic(testFailed{})
}

func (e *evalT) Failed() bool {
	return e.failed
}

func (e *evalT) TempDir() string {
	// Create temporary directory for test
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("evalT-%d", time.Now().UnixNano()))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		e.Fatal(err)
	}
	return dir
}

func (e *evalT) Chdir(dir string) {
	err := os.Chdir(dir)
	if err != nil {
		e.Fatal(err)
	}
}

func (e *evalT) Cleanup(func()) {}

func (e *evalT) Setenv(key, value string) {
	err := os.Setenv(key, value)
	if err != nil {
		e.Fatal(err)
	}
}

func (e *evalT) Skip(args ...interface{}) {
	e.Log(args...)
	e.SkipNow()
}

func (e *evalT) Skipf(format string, args ...interface{}) {
	e.Logf(format, args...)
	e.SkipNow()
}

func (e *evalT) SkipNow() {
	e.skipped = true
	panic(testSkipped{})
}

func (e *evalT) Skipped() bool {
	return e.skipped
}

func (e *evalT) Deadline() (time.Time, bool) {
	deadline, ok := e.ctx.Deadline()
	return deadline, ok
}

func (e *EvalRunner) NPMAudit(
	ctx context.Context,
	// +defaultPath="/hello-dagger"
	project *dagger.Directory,
) (*EvalReport, error) {

	llm := e.llm().
		WithEnv(
			dag.Env(dagger.EnvOpts{
				Privileged: true,
			}),
		)

	return withLLMReport(
		ctx,
		llm,
		[]withLLMReportStep{
			{
				`Mount $src at /app in $node, set workdir /app, then run "npm install --audit=false" followed by "npm audit --json".  Write the JSON to /out/audit.json and expose it as $audit.`,
				func(env *dagger.Env) *dagger.Env {
					return env.
						WithDirectoryInput("src", project, "Node project to audit.").
						WithFileOutput("audit", "JSON output of npm audit.")
				},
				func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
					raw, err := llm.Env().Output("audit").AsFile().Contents(ctx)
					require.NoError(t, err)
					var parsed map[string]interface{}
					require.NoError(t, json.Unmarshal([]byte(raw), &parsed))
					_, ok := parsed["vulnerabilities"]
					require.True(t, ok, "npm audit JSON missing 'vulnerabilities'")
				},
			},
		}...,
	)
}

func (e *EvalRunner) TrivyScan(
	ctx context.Context,
	target *dagger.Directory,
) (*EvalReport, error) {

	llm := e.llm().
		WithEnv(
			dag.Env(dagger.EnvOpts{
				Privileged: true,
			}),
		)

	return withLLMReport(
		ctx,
		llm,
		[]withLLMReportStep{
			{
				`publish the hello dagger app`,
				func(env *dagger.Env) *dagger.Env {
					return env.WithStringOutput("imageRef", "Published docker image")
				},
				func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
					out, err := llm.Env().Output("imageRef").AsString(ctx)
					require.NoError(t, err)
					fmt.Fprintf(os.Stderr, "ImageRef: %s\n", out)
					require.Contains(t, out, "ttl.sh/hello-dagger-", "REF")
				},
			},
			{
				`check for its vulnerabilities`,
				func(env *dagger.Env) *dagger.Env {
					return env.WithStringOutput("trivyOutput", "Trivy scan output")
				},
				func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
					out, err := llm.Env().Output("trivyOutput").AsString(ctx)
					require.NoError(t, err)
					// fmt.Fprintf(os.Stderr, "🥶debug: %s\n", out)
					require.Contains(t, out, "Vulnerability", "VULNERABILITY")
					// t.FailNow() // bugs
				},
			},
		}...,
	)
}

/// Example evals -- keeping it just for reference
// // Test manual intervention allowing the prompt to succeed.
// func (m *EvalRunner) LifeAlert(ctx context.Context) (*Report, error) {
// 	return withLLMReport(ctx,
// 		m.llm(dagger.LLMOpts{MaxAPICalls: 10}).
// 			WithEnv(dag.Env().
// 				WithDirectoryInput("dir", dag.Directory(), "A directory to write a file into.").
// 				WithFileOutput("file", "A file containing knowledge you don't have."),
// 			).
// 			WithPrompt("Ask me what to write to the file."),
// 		func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
// 			reply, err := llm.Env().Output("file").AsFile().Contents(ctx)
// 			require.NoError(t, err)
// 			require.Contains(t, strings.ToLower(reply), "potato")
// 		})
// }

// // Test basic prompting.
// func (m *EvalRunner) Basic(ctx context.Context) (*Report, error) {
// 	return withLLMReport(ctx,
// 		m.llm(dagger.LLMOpts{MaxAPICalls: 5}).
// 			WithPrompt("Hello there! Respond with 'potato' if you received this message."),
// 		func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
// 			reply, err := llm.LastReply(ctx)
// 			require.NoError(t, err)
// 			require.Contains(t, strings.ToLower(reply), "potato")
// 		})
// }

// // Test the model's eagerness to switch to prior states instead of mutating the
// // current state to undo past actions.
// func (m *EvalRunner) UndoChanges(ctx context.Context) (*Report, error) {
// 	env := dag.Env().
// 		WithDirectoryInput("dir", dag.Directory(),
// 			"A directory in which to write files.").
// 		WithDirectoryOutput("out", "The directory with the desired contents.")
// 	return withLLMReport(ctx,
// 		m.llm(dagger.LLMOpts{MaxAPICalls: 20}).
// 			WithEnv(env).
// 			WithPrompt("Create the file /a with contents 1.").
// 			Loop().
// 			WithPrompt("Create the file /b with contents 2.").
// 			Loop().
// 			WithPrompt("Nevermind - go back to before you created /b and create /c with contents 3, and return that."),
// 		func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
// 			entries, err := llm.Env().Output("out").AsDirectory().Entries(ctx)
// 			require.NoError(t, err)
// 			sort.Strings(entries)
// 			require.ElementsMatch(t, []string{"a", "c"}, entries)
// 		})
// }

// // Test the model's ability to pass objects around to one another and execute a
// // series of operations given at once.
// func (m *EvalRunner) BuildMulti(ctx context.Context) (*Report, error) {
// 	return withLLMReport(ctx,
// 		m.llm(dagger.LLMOpts{MaxAPICalls: 20}).
// 			WithEnv(
// 				dag.Env().
// 					WithDirectoryInput("repo",
// 						dag.Git("https://github.com/vito/booklit").Head().Tree(),
// 						"The Booklit repository.").
// 					WithContainerInput("ctr",
// 						dag.Container().
// 							From("golang").
// 							WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
// 							WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
// 							WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
// 							WithEnvVariable("GOCACHE", "/go/build-cache").
// 							WithEnvVariable("BUSTER", fmt.Sprintf("%d-%s", m.Attempt, time.Now())),
// 						"The Go container to use to build Booklit.").
// 					WithFileOutput("bin", "The /out/booklit binary."),
// 			).
// 			WithPrompt("Mount $repo into $ctr at /src, set it as your workdir, and build ./cmd/booklit with the CGO_ENABLED env var set to 0, writing it to /out/booklit."),
// 		buildMultiAssert)
// }

// // BuildMulti is like BuildMulti but without explicitly referencing the relevant
// // objects, leaving the LLM to figure it out.
// func (m *EvalRunner) BuildMultiNoVar(ctx context.Context) (*Report, error) {
// 	return withLLMReport(ctx,
// 		m.llm(dagger.LLMOpts{MaxAPICalls: 20}).
// 			WithEnv(
// 				dag.Env().
// 					WithDirectoryInput("notRepo", dag.Directory(), "Bait - ignore this.").
// 					WithDirectoryInput("repo",
// 						dag.Git("https://github.com/vito/booklit").Head().Tree(),
// 						"The Booklit repository.").
// 					WithContainerInput("notCtr", dag.Container(), "Bait - ignore this.").
// 					WithContainerInput("ctr",
// 						dag.Container().
// 							From("golang").
// 							WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
// 							WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
// 							WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
// 							WithEnvVariable("GOCACHE", "/go/build-cache").
// 							WithEnvVariable("BUSTER", fmt.Sprintf("%d-%s", m.Attempt, time.Now())),
// 						"The Go container to use to build Booklit.").
// 					WithFileOutput("bin", "The /out/booklit binary."),
// 			).
// 			WithPrompt("Mount my repo into the container, set it as your workdir, and build ./cmd/booklit with the CGO_ENABLED env var set to 0, writing it to /out/booklit."),
// 		buildMultiAssert)
// }

// // Extracted for reuse between BuildMulti tests
// func buildMultiAssert(ctx context.Context, t testing.TB, llm *dagger.LLM) {
// 	f, err := llm.Env().Output("bin").AsFile().Sync(ctx)
// 	require.NoError(t, err)

// 	history, err := llm.History(ctx)
// 	require.NoError(t, err)
// 	if !strings.Contains(strings.Join(history, "\n"), "Container_with_env_variable") {
// 		t.Error("should have used Container_with_env_variable - use the right tool for the job!")
// 	}

// 	ctr := dag.Container().
// 		From("alpine").
// 		WithFile("/bin/booklit", f).
// 		WithExec([]string{"chmod", "+x", "/bin/booklit"}).
// 		WithExec([]string{"/bin/booklit", "--version"})
// 	out, err := ctr.Stdout(ctx)
// 	require.NoError(t, err, "command failed - did you forget CGO_ENABLED=0?")

// 	out = strings.TrimSpace(out)
// 	require.Equal(t, "0.0.0-dev", out)
// }

// // Test that the LLM is able to access the content of variables without the user
// // having to expand them in the prompt.
// //
// // SUCCESS RATE (ballpark):
// // - claude-3-7-sonnet-latest: 100%
// // - gpt-4o: 100%
// // - gemini-2.0-flash: 0%
// func (m *EvalRunner) ReadImplicitVars(ctx context.Context) (*Report, error) {
// 	// use some fun formatting here to make sure it doesn't get lost in
// 	// the shuffle
// 	//
// 	// NOTE: an earlier iteration included a trailing line break, but... honestly
// 	// just don't do that. when it gets that weird, pass in a file instead. it's a
// 	// similar issue you might run into with passing it around in a shell, which
// 	// these vars already draw parallels to (and may even be sourced from).
// 	weirdText := "I'm a strawberry!"
// 	return withLLMReport(ctx,
// 		m.llm(dagger.LLMOpts{MaxAPICalls: 20}).
// 			WithEnv(dag.Env().
// 				WithStringInput("myContent", weirdText,
// 					"The content to write.").
// 				WithStringInput("desiredName", "/weird.txt",
// 					"The name of the file to write to.").
// 				WithDirectoryInput("dest", dag.Directory(),
// 					"The directory in which to write the file.").
// 				WithDirectoryOutput("out", "The directory containing the written file.")).
// 			WithPrompt("I gave you some content, a directory, and a filename. Write the content to the specified file in the directory."),
// 		func(ctx context.Context, t testing.TB, llm *dagger.LLM) {
// 			content, err := llm.Env().
// 				Output("out").
// 				AsDirectory().
// 				File("weird.txt").
// 				Contents(ctx)
// 			require.NoError(t, err)
// 			require.Equal(t, weirdText, content)
// 		})
// }
