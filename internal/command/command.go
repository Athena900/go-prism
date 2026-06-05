package command

import (
	"bytes"
	"context"
	"os/exec"
)

// Runner resolves and runs external commands.
type Runner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, invocation Invocation) Result
}

// Invocation describes one external command execution.
type Invocation struct {
	Path string
	Args []string
	Dir  string
}

// Result captures command output and exit status.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// LocalRunner runs tools on the local system.
type LocalRunner struct{}

func (LocalRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (LocalRunner) Run(ctx context.Context, invocation Invocation) Result {
	cmd := exec.CommandContext(ctx, invocation.Path, invocation.Args...)
	cmd.Dir = defaultWorkDir(invocation.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
		Err:      err,
	}

	if err == nil {
		return result
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}

	return result
}

func defaultWorkDir(workDir string) string {
	if workDir == "" {
		return "."
	}
	return workDir
}
