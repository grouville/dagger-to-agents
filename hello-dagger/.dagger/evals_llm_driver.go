package main

import (
	"context"
	"fmt"

	"dagger/hello-dagger/internal/dagger"
)

type DaggerShellDriver struct{}

func (DaggerShellDriver) NewTestClient(ev *EvalRunner) LLMTestClient {
	return NewDaggerShell(ev)
}

// Returns an instance of a dagger shell client
func NewDaggerShell(ev *EvalRunner) LLMTestClient {
	opts := dagger.LLMOpts{
		Model: ev.Model,
	}
	daggerLLM := dag.LLM(opts)

	if ev.SystemPrompt != "" {
		daggerLLM = daggerLLM.WithSystemPrompt(ev.SystemPrompt)
	}
	if ev.Attempt > 0 {
		daggerLLM = daggerLLM.Attempt(ev.Attempt)
	}

	daggerLLM = daggerLLM.WithEnv(
		dag.Env(dagger.EnvOpts{
			Privileged: true,
		}),
	)

	return &DaggerShellClient{llm: daggerLLM}
}

type DaggerShellClient struct {
	llm *dagger.LLM
}

func (d *DaggerShellClient) History(ctx context.Context) ([]string, error) {
	return d.llm.History(ctx)
}

func (d *DaggerShellClient) InputTokens(ctx context.Context) (int, error) {
	return d.llm.TokenUsage().InputTokens(ctx)
}

func (d *DaggerShellClient) OutputTokens(ctx context.Context) (int, error) {
	return d.llm.TokenUsage().OutputTokens(ctx)
}

func (d *DaggerShellClient) ToolsDoc(ctx context.Context) (string, error) {
	return d.llm.Tools(ctx)
}

func (d *DaggerShellClient) SetPrompt(ctx context.Context, prompt string) {
	d.llm = d.llm.WithPrompt(prompt)
}

// var MY_ENV *dagger.Env

func (d *DaggerShellClient) GetEnv(ctx context.Context) *TestEnv {
	return FromDagger(d.llm.Env())
}

func (d *DaggerShellClient) SetEnv(ctx context.Context, modFn EnvModifierFunc) {
	fmt.Errorf("modFn: %v", modFn)
	after := modFn(FromDagger(d.llm.Env()))
	d.llm = d.llm.WithEnv(after.ToDagger())
}

func (d *DaggerShellClient) Run(ctx context.Context) (err error) {
	_, err = d.llm.Sync(ctx)
	return err
}
