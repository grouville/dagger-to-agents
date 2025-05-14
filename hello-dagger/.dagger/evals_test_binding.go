package main

import (
	"context"
	"dagger/hello-dagger/internal/dagger"
	"fmt"
)

// Env and Binding are extracted partially from core/env.go/Binding type
// We need it to pass the env inputs and outputs in and out via the dagger mcp command
// The current communication is via a file and executed at the start -- end of the run
type TestBinding struct {
	Key   string `json:"key"`
	Value any    `json:"value"`

	Description string `json:"description"`
	// The expected type
	// Used when defining an output
	// ExpectedType string
}

// _ context.Context for parity with Dagger's binding API
// AsString converts the stored value to string, returning an error if it’s not.
func (b TestBinding) AsString(_ context.Context) (string, error) {
	if b.Value == nil {
		return "", fmt.Errorf("binding %q is nil", b.Key)
	}

	s, ok := b.Value.(string)
	if !ok {
		return "", fmt.Errorf("binding %q is not a string", b.Key)
	}
	return s, nil
}

// _ context.Context for parity with Dagger's binding API
func (b TestBinding) AsDirectory(_ context.Context) (*dagger.Directory, bool) {
	d, ok := b.Value.(*dagger.Directory)
	return d, ok
}
