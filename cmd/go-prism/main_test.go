package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPRMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/project\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"pr", "--workdir", dir, "--format", "markdown"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"## Go Prism", "Decision: PASS", "Module path: `example.com/project`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"nope"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}
