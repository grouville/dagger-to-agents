package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"dagger/hello-dagger/internal/dagger"
)

// satisfy the LLMTestClientDriver interface
type GooseDriver struct{}

func (GooseDriver) NewTestClient(ev *EvalRunner) LLMTestClient {
	return NewGoose(ev)
}

//go:embed goose-config.yaml
var gooseConfig string

//go:embed mcp.sh
var mcpSh string

func sh(s string) []string {
	return []string{"sh", "-c", s}
}

func (e *EvalRunner) gooseCtr(ctx context.Context, target *dagger.Directory) *dagger.Container {
	// utiliser ça pour faire un docker-in-docker
	// dagger -c 'container | from debian | with-exec sh,-c,"apt update && apt-get install -y --no-install-recommends curl ca-certificates && curl -fsSL https://get.docker.io/ | sh" | with-mounted-file /bin/dagger $(host | file /home/guillaume/dagger/bin/dagger) | with-unix-socket /var/run/docker.sock $(host | unix-socket /var/run/docker.sock)  | terminal'
	return dag.Container().
		From("debian").
		WithExec(sh(`apt-get update && apt-get install -y --no-install-recommends curl ca-certificates bzip2 libxcb1; rm -rf /var/{cache/apt,lib/apt/lists}/*`)).
		WithExec(sh(`curl -fsSL "https://get.docker.io/" | sh`)).
		WithExec(sh(`curl -fsSL "https://github.com/block/goose/releases/download/v1.0.20/download_cli.sh" | GOOSE_BIN_DIR=/usr/local/bin CONFIGURE=false bash`)).
		WithNewFile("/root/.config/goose/config.yaml", gooseConfig).
		WithNewFile("/tmp/mcp.sh", mcpSh, dagger.ContainerWithNewFileOpts{Permissions: 755}).
		WithMountedDirectory("/target", target).
		WithMountedFile("/bin/dagger", e.DaggerCli).
		WithUnixSocket("/var/run/docker.sock", e.DockerSocket).
		WithSecretVariable("OPENAI_API_KEY", e.LLMKey).
		WithWorkdir("/target").
		WithNewFile("/target/llm-history", `{"working_dir":"/target","description":"Initial greeting exchange","message_count":2,"total_tokens":687,"input_tokens":673,"output_tokens":14,"accumulated_total_tokens":1373,"accumulated_input_tokens":1346,"accumulated_output_tokens":27}`, dagger.ContainerWithNewFileOpts{Permissions: 0644})
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

	baseCtr := ev.gooseCtr(context.Background(), ev.Target)

	return &GooseClient{
		goose: baseCtr,
		env:   NewTestEnv(),
	}
}

func (d *GooseClient) SetPrompt(ctx context.Context, prompt string) {
	// append only prompt -- as the shell driver's behavior
	d.prompt = d.prompt + " " + prompt
}

// ApplyEnv applies environment modifications using the provided function.
func (d *GooseClient) SetEnv(ctx context.Context, fn EnvModifierFunc) {
	d.env = fn(d.env)
}

// Retrieves the current environment following a test run.
func (d *GooseClient) GetEnv(ctx context.Context) *TestEnv {
	// 1. Read the JSON file generated inside the container.
	content, err := d.goose.File("/tmp/declare/output").Contents(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to read goose outputs: %w", err))
	}

	// 2. Unmarshal into a slice of TestBinding (same shape as we exported).
	var outs []TestBinding
	if err := json.Unmarshal([]byte(content), &outs); err != nil {
		panic(fmt.Errorf("failed to unmarshal goose outputs: %w", err))
	}

	// 3. Merge results into the client’s current environment.
	if d.env == nil {
		d.env = NewTestEnv()
	}
	for _, b := range outs {
		// Note: this export method from the engine only works with string values.
		d.env.Outputs[b.Key] = b
	}

	return d.env
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
	// upon the dagger mcp command initialization, with the --with-env <path> flag
	ctr := d.goose.
		WithNewFile("/tmp/path_to_happiness", string(data), dagger.ContainerWithNewFileOpts{Permissions: 0644})

	// per attempt later
	ctr = ctr.WithExec(sh(fmt.Sprintf("goose run -p llm-history -r -t %q", d.prompt)))

	d.goose, err = ctr.Sync(ctx) // update the state of the container
	return err
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
	ctr = d.goose.WithNewFile("/tmp/path_to_happiness", string(data), dagger.ContainerWithNewFileOpts{Permissions: 0644})

	// per attempt later
	// ctr = ctr.WithExec(sh(fmt.Sprintf("goose run -p llm-history -r -t %q", d.prompt)))

	return ctr, nil
}
