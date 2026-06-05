package api

import (
	"bytes"
	"context"
	"os/exec"
)

type commandRunner struct{}

func (commandRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (commandRunner) Run(ctx context.Context, invocation ToolInvocation) ToolResult {
	cmd := exec.CommandContext(ctx, invocation.Path, invocation.Args...)
	cmd.Dir = defaultWorkDir(invocation.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := ToolResult{
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
