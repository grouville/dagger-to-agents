package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dagger/hello-dagger/internal/dagger"
)

// this const is also hardcoded in the goose-config.yaml file
const ENV_SNAPSHOT_DIR = "/tmp/env_snapshot"

// satisfy the LLMTestClientDriver interface
type GooseDriver struct{}

func (GooseDriver) NewTestClient(ev *EvalRunner) LLMTestClient {
	return NewGoose(ev)
}

//go:embed goose-config.yaml.tmpl
var gooseTmpl string

//go:embed dagger_system_prompt.md
var daggerSystemPrompt string

func sh(s string) []string {
	return []string{"sh", "-c", s}
}

// builds everything *except* the goose YAML
func (e *EvalRunner) baseGooseCtr() *dagger.Container {
	return dag.Container().
		From("debian").
		WithExec(sh(`apt-get update && apt-get install -y --no-install-recommends curl ca-certificates bzip2 libxcb1; rm -rf /var/{cache/apt,lib/apt/lists}/*`)).
		WithExec(sh(`curl -fsSL "https://get.docker.io/" | sh`)).
		WithExec(sh(`curl -fsSL "https://github.com/block/goose/releases/download/v1.0.20/download_cli.sh" | GOOSE_BIN_DIR=/usr/local/bin CONFIGURE=false bash`)).
		WithDirectory(ENV_SNAPSHOT_DIR, dag.Directory()).
		WithMountedDirectory("/target", e.Target).
		WithNewFile("/target/llm-history", `{"working_dir":"/target","description":"Initial greeting exchange","message_count":2,"total_tokens":687,"input_tokens":673,"output_tokens":14,"accumulated_total_tokens":1373,"accumulated_input_tokens":1346,"accumulated_output_tokens":27}`, dagger.ContainerWithNewFileOpts{Permissions: 0644}).
		WithMountedFile("/bin/dagger", e.DaggerCli).
		WithUnixSocket("/var/run/docker.sock", e.DockerSocket).
		WithSecretVariable("OPENAI_API_KEY", e.LLMKey).
		WithNewFile("/system_prompt.md", daggerSystemPrompt).
		WithEnvVariable("GOOSE_SYSTEM_PROMPT_FILE_PATH", "/system_prompt.md").
		WithWorkdir("/target")
}

// helper holds the substitution variables
type gooseVars struct {
	Model, Provider, Host, BasePath, SnapshotDir string
}

// render using os.Expand (simple, no third-party deps)
func renderConfig(tpl string, v gooseVars) string {
	return os.Expand(tpl, func(k string) string {
		switch k {
		case "GOOSE_MODEL":
			return v.Model
		case "GOOSE_PROVIDER":
			return v.Provider
		case "OPENAI_HOST":
			return v.Host
		case "OPENAI_BASE_PATH":
			return v.BasePath
		case "ENV_SNAPSHOT_DIR":
			return v.SnapshotDir
		default:
			return ""
		}
	})
}

type GooseClient struct {
	// llm *dagger.LLM
	goose *dagger.Container // state of the goose container with DaggeriDagger

	env    *TestEnv // keep track of the current environment + all the applied bindings
	prompt string   // the prompt to be used for the goose container
}

func NewGoose(ev *EvalRunner) LLMTestClient {
	// System prompt
	// TODO: add a system prompt utility to override the default -- goose supports its

	// attempts will be used to run parallel gooseCtr in parallel with this as a key
	// if ev.Attempt > 0 {
	// 	daggerLLM = daggerLLM.Attempt(ev.Attempt)
	// }

	base := ev.baseGooseCtr()

	// collect substitution values
	vars := gooseVars{
		Model:       ev.Model,
		Provider:    ev.Provider,
		Host:        ev.Host,
		BasePath:    ev.BasePath,
		SnapshotDir: ENV_SNAPSHOT_DIR,
	}

	cfg := renderConfig(gooseTmpl, vars)

	// mount the freshly-rendered YAML inside the container
	base = base.WithNewFile("/root/.config/goose/config.yaml", cfg)

	return &GooseClient{
		goose: base,
		env:   NewTestEnv(),
	}
}

func (d *GooseClient) SetPrompt(ctx context.Context, prompt string) {
	// append only prompt -- as the shell driver's behavior
	// d.prompt = d.prompt + " " + prompt // i think i'm wrong -- too weird
	d.prompt = prompt
}

// ApplyEnv applies environment modifications using the provided function.
func (d *GooseClient) SetEnv(ctx context.Context, fn EnvModifierFunc) {
	d.env = fn(d.env)
}

// Retrieves the current environment following a test run.
func (d *GooseClient) GetEnv(ctx context.Context) (*TestEnv, error) {
	path := fmt.Sprintf("%s/output.json", ENV_SNAPSHOT_DIR)

	file := d.goose.File(path)
	if file == nil {
		return nil, fmt.Errorf("snapshot output not found at %s", path)
	}

	content, err := file.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read goose outputs: %w", err)
	}

	var outs []TestBinding
	if err := json.Unmarshal([]byte(content), &outs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal goose outputs: %w", err)
	}

	if d.env == nil {
		d.env = NewTestEnv()
	}
	for _, b := range outs {
		d.env.Outputs[b.Key] = b
	}

	return d.env, nil
}

// Retrieves the current environment following a test run.
func (d *GooseClient) Run(ctx context.Context) (err error) {
	// marshall it and write it to the file at a fixed location
	data, err := json.Marshal(d.env)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	// set the env state in the goose container, at this path
	// the mcp.sh script will read it and set the env
	// upon the dagger mcp command initialization, with the --with-dir <path> flag
	ctr := d.goose.
		WithNewFile(fmt.Sprintf("%s/input.json", ENV_SNAPSHOT_DIR), string(data), dagger.ContainerWithNewFileOpts{Permissions: 0644})

	// per attempt later
	ctr = ctr.WithExec(sh(fmt.Sprintf("goose run -p llm-history -r -t %q", d.prompt)))

	d.goose, err = ctr.Sync(ctx) // update the state of the container
	return err
}

// Retrieves the current environment following a test run.
func (d *GooseClient) History(ctx context.Context) ([]string, error) {
	file := d.goose.File("/target/llm-history")
	if file == nil {
		return nil, fmt.Errorf("failed to get llm-history file")
	}

	content, err := file.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read llm-history: %w", err)
	}

	// bad heuristic -- but for now it's enough
	return strings.Split(content, "\n"), nil
}

// wip
func (d *GooseClient) Container() (ctr *dagger.Container, err error) {
	// marshall it and write it to the file at a fixed location
	data, err := json.Marshal(d.env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal env: %w", err)
	}

	// set the env state in the goose container, at this path
	// the mcp.sh script will read it and set the env
	// upon the dagger mcp command initialization, with the --with-env <path> flag
	ctr = d.goose.WithNewFile(fmt.Sprintf("%s/input.json", ENV_SNAPSHOT_DIR), string(data), dagger.ContainerWithNewFileOpts{Permissions: 0644})

	// per attempt later
	// ctr = ctr.WithExec(sh(fmt.Sprintf("goose run -p llm-history -r -t %q", d.prompt)))

	return ctr, nil
}
