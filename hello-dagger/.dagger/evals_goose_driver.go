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

// Env and Binding are extracted partially from core/env.go/Binding type
// We need it to pass the env inputs and outputs in and out via the dagger mcp command
// The current communication is via a file and executed at the start -- end of the run
type Binding struct {
	Key   string
	Value any // will be any

	Description string
	// The expected type
	// Used when defining an output
	// ExpectedType string
}

type TestEnv struct {
	Inputs  []Binding
	Outputs []Binding
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
		WithWorkdir("/target")
}

type GooseClient struct {
	// llm *dagger.LLM
	goose *dagger.Container // state of the goose container with DaggeriDagger

	env    *dagger.Env // keep track of the current environment + all the applied bindings
	prompt string      // the prompt to be used for the goose container
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
		env:   dag.Env(),
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
func (d *GooseClient) GetEnv(ctx context.Context) *dagger.Env {
	// genMcpToolHandler
	content, err := d.goose.File("/tmp/declare/output").Contents(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to retrieve the output from (%w)", err))
		// return fmt.Errorf("failed to retrieve the output from (%w)", err)
	}

	var outputs []Binding
	err = json.Unmarshal([]byte(content), &outputs)
	if err != nil {
		panic(fmt.Errorf("failed to unserialize the output env from the `/tmp/declare/output` file: %w", err))
		// return fmt.Errorf("failed to unmarshal env: %w", err)
		// TODO: we don't really care after -- or we need to change the proto on the interface
	}

	env := dag.Env()
	for _, o := range outputs {
		// We know, for sure, that it only has String outputs
		// So we construct the env with this string output
		// And we set the value to the output value
		env = env.WithStringOutput(o.Key, o.Description)
		// bind := env.Output(o.Key)
		// bind.Value = dagql.NewString(o.Value)
	}

	return env
}

// Retrieves the current environment following a test run.
func (d *GooseClient) Run(ctx context.Context) (err error) {
	myEnv, err := convertEnv(ctx, d.env)
	if err != nil {
		return fmt.Errorf("failed to convert env to JSON: %w", err)
	}

	// marshall it and write it to the file at a fixed location
	data, err := json.Marshal(myEnv)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	// set the env state in the goose container, at this path
	// the mcp.sh script will read it and set the env
	// upon the dagger mcp command initialization, with the --with-env <path> flag
	ctr := d.goose.WithNewFile("/tmp/path_to_happiness", string(data), dagger.ContainerWithNewFileOpts{Permissions: 0644})

	// per attempt later
	ctr = ctr.WithExec(sh(fmt.Sprintf("goose run -p llm-history -r -t %q", d.prompt)))

	d.goose, err = ctr.Sync(ctx) // update the state of the container
	return err
}

func convertEnv(ctx context.Context, env *dagger.Env) (*TestEnv, error) {
	// extract the inputs and outputs from its new state
	inputs, _ := env.Inputs(ctx)
	outputs, _ := env.Outputs(ctx)

	// convert the inputs and outputs to our own Env type that the `dagger mcp --with-env` command will catch
	var myEnv TestEnv
	for _, input := range inputs {
		// // skip non String inputs // scalars
		// if input.Value.Type().NamedType != "String" {
		// 	continue
		// }

		name, err := input.Name(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get input name: %w", err)
		}

		if name == "workdir" {
			continue
		}

		val, err := input.AsString(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get input value: %w", err)
		}

		desc, err := input.Description(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get input description: %w", err)
		}

		myEnv.Inputs = append(myEnv.Inputs, Binding{
			Key:         name,
			Value:       val,
			Description: desc,
		})
	}

	for _, output := range outputs {
		name, err := output.Name(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get output name: %w", err)
		}

		val, err := output.AsString(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get output value: %w", err)
		}

		desc, err := output.Description(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get output description: %w", err)
		}

		myEnv.Outputs = append(myEnv.Outputs, Binding{
			Key:         name,
			Value:       val,
			Description: desc,
		})
	}

	return &myEnv, nil
}
