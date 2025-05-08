package main

import (
	"context"
	"dagger/hello-dagger/internal/dagger"
)

// TestEnv is a lightweight, serialisable replacement for dagger.Env
// holding only scalars and high-level objects needed by the eval framework.
type TestEnv struct {
	Inputs  map[string]TestBinding `json:"inputs"`
	Outputs map[string]TestBinding `json:"outputs"`
}

func NewTestEnv() *TestEnv {
	return &TestEnv{
		Inputs:  map[string]TestBinding{},
		Outputs: map[string]TestBinding{},
	}
}

func (e *TestEnv) WithStringInput(key, val, desc string) *TestEnv {
	clone := e.clone()
	clone.Inputs[key] = TestBinding{Key: key, Value: val, Description: desc}
	return clone
}

func (e *TestEnv) WithDirectoryInput(key string, dir *dagger.Directory, desc string) *TestEnv {
	clone := e.clone()
	clone.Inputs[key] = TestBinding{Key: key, Value: dir, Description: desc}
	return clone
}

func (e *TestEnv) WithStringOutput(key, desc string) *TestEnv {
	clone := e.clone()
	clone.Outputs[key] = TestBinding{Key: key, Description: desc}
	return clone
}

func (e *TestEnv) Output(name string) TestBinding {
	if b, ok := e.Outputs[name]; ok {
		return b
	}
	return TestBinding{}
}

// helpers
func FromDagger(de *dagger.Env) *TestEnv {
	ctx := context.Background()

	inputs, _ := de.Inputs(ctx)
	outputs, _ := de.Outputs(ctx)

	te := NewTestEnv()

	// helper to copy *any* binding we support
	copyBinding := func(b dagger.Binding, dest map[string]TestBinding) {
		name, _ := b.Name(ctx)
		desc, _ := b.Description(ctx)

		var val any

		// === value probes, in priority order ===
		if s, err := b.AsString(ctx); err == nil { // string
			val = s
		} else if dir := b.AsDirectory(); dir != nil { // directory
			val = dir
		}

		dest[name] = TestBinding{Key: name, Value: val, Description: desc}
	}

	for _, b := range inputs {
		copyBinding(b, te.Inputs)
	}
	for _, b := range outputs {
		copyBinding(b, te.Outputs)
	}

	return te
}

func (e *TestEnv) ToDagger() *dagger.Env {
	d := dag.Env()
	for _, b := range e.Inputs {
		switch v := b.Value.(type) {
		case string:
			d = d.WithStringInput(b.Key, v, b.Description)
		case *dagger.Directory:
			d = d.WithDirectoryInput(b.Key, v, b.Description)
		}
	}
	for _, b := range e.Outputs {
		d = d.WithStringOutput(b.Key, b.Description)
	}
	return d
}

func (e *TestEnv) clone() *TestEnv {
	n := NewTestEnv()
	for k, v := range e.Inputs {
		n.Inputs[k] = v
	}
	for k, v := range e.Outputs {
		n.Outputs[k] = v
	}
	return n
}
