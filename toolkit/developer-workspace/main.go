// A generated module for DeveloperWorkspace functions
//
// This module provides tools for interacting with a workspace directory,
// including file operations and command execution, similar to the

package main

import (
	"context"
	"dagger/developer-workspace/internal/dagger"
	"fmt"
	"strings"
)

// DeveloperWorkspace provides tools for interacting with a workspace directory.
type DeveloperWorkspace struct {
	// The working directory containing the source code.
	Workdir *dagger.Directory

	// The workspace container tool in which the source code is mounted.
	container *dagger.Container
}

// DeveloperWorkspace provides tools for interacting with a workspace directory.
// Initializes the toolset with a starting working directory, inside an alpine container
func New(workdir *dagger.Directory) *DeveloperWorkspace {
	ctr := dag.Container().From("alpine:latest").
		WithMountedDirectory("/src", workdir).
		WithWorkdir("/src")

	return &DeveloperWorkspace{
		Workdir:   workdir,
		container: ctr,
	}
}

// Reads a file from the local filesystem. The path parameter must be an absolute path, not a relative path. By default, it reads up to ${MAX_LINES_TO_READ} lines starting from the beginning of the file. You can optionally specify a line offset and limit (especially handy for long files), but it's recommended to read the whole file by not providing these parameters. Any lines longer than ${MAX_LINE_LENGTH} characters will be truncated.

// Reads a file from the local filesystem. The path parameter must be an absolute path, not a relative path. If a relative path is provided, it will be treated as relative to the default workdir (/src).
func (w *DeveloperWorkspace) ReadFile(ctx context.Context, path string) (string, error) {
	// Basic path validation
	if strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid path: '%s'. Path must be relative to the workspace root and cannot contain '..'", path)
	}

	return w.Workdir.File(path).Contents(ctx)
}

// WriteFile writes content to a file within the working directory (overwriting if it exists).
// The path should be relative to the root of the Workdir. (/src).
func (w *DeveloperWorkspace) WriteFile(ctx context.Context, path string, contents string) (*DeveloperWorkspace, error) {
	// Basic path validation
	if strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid path: '%s'. Path must be relative to the workspace root and cannot contain '..'", path)
	}

	updatedWorkdir := w.Workdir.WithNewFile(path, contents)

	return &DeveloperWorkspace{
		Workdir:   updatedWorkdir,
		container: w.container.WithMountedDirectory("/src", updatedWorkdir),
	}, nil
}
