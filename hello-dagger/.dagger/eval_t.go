// Package provides a minimal testing.TB implementation
// suitable for programmatic evaluations outside the `go test` framework.
//
// evalT captures logs, failures, skips, and offers TempDir/Chdir utilities.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// evalT implements testing.TB for standalone use.
type evalT struct {
	*testing.T
	name            string
	ctx             context.Context
	logs            *strings.Builder
	failed, skipped bool
}

var _ testing.TB = (*evalT)(nil)

// newT creates an evalT with context and identifier.
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

// Fatal and Fatalf record a failure and panic.
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

// TempDir and Chdir manage workspace.
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

// Cleanup is a stub.
func (e *evalT) Cleanup(func()) {}

// Setenv sets an environment variable.
func (e *evalT) Setenv(key, value string) {
	err := os.Setenv(key, value)
	if err != nil {
		e.Fatal(err)
	}
}

// Skip, Skipf, SkipNow trigger skip.
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

// Deadline reports context deadline.
func (e *evalT) Deadline() (time.Time, bool) {
	deadline, ok := e.ctx.Deadline()
	return deadline, ok
}
