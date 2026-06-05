package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPRMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/project\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init")
	runGit(t, dir, "add", "go.mod")
	runGit(t, dir, "-c", "user.name=go-prism", "-c", "user.email=go-prism@example.com", "commit", "-m", "base")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"pr", "--workdir", dir, "--base", "HEAD", "--head", "HEAD", "--format", "markdown"}, &stdout, &stderr)
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

func TestRunDoctorText(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--workdir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"go-prism doctor", "Overall: OK", "runtime.go", "module.gomod", "repo.git"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunDoctorJSON(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--workdir", dir, "--format", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}

	var out struct {
		SchemaVersion string `json:"schema_version"`
		Status        string `json:"status"`
		Module        string `json:"module"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if out.SchemaVersion != "doctor.v1" {
		t.Fatalf("schema_version = %q", out.SchemaVersion)
	}
	if out.Status != "ok" {
		t.Fatalf("status = %q", out.Status)
	}
	if out.Module != "example.com/project" {
		t.Fatalf("module = %q", out.Module)
	}
}

func TestRunDoctorExplicitMissingConfigFails(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--workdir", dir, "--config", filepath.Join(dir, "missing.yml"), "--format", "json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}
	if !strings.Contains(stdout.String(), `"status": "fail"`) {
		t.Fatalf("stdout missing failing status:\n%s", stdout.String())
	}
}

func TestRunHelpMentionsDoctor(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "go-prism doctor [flags]") {
		t.Fatalf("help missing doctor:\n%s", stdout.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	if got, want := stdout.String(), "0.1.0\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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

func writeTestModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/project\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init")
	runGit(t, dir, "add", "go.mod")
	runGit(t, dir, "-c", "user.name=go-prism", "-c", "user.email=go-prism@example.com", "commit", "-m", "base")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.Clean(dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
