package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestRunPRJSONIncludesSchemaVersion(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"pr", "--workdir", dir, "--base", "HEAD", "--head", "HEAD", "--format", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}

	var out struct {
		SchemaVersion string `json:"schema_version"`
		Tool          string `json:"tool"`
		Version       string `json:"version"`
		Decision      string `json:"decision"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if out.SchemaVersion != "report.v1" {
		t.Fatalf("schema_version = %q, want report.v1", out.SchemaVersion)
	}
	if out.Tool != "go-prism" {
		t.Fatalf("tool = %q, want go-prism", out.Tool)
	}
	if out.Version != version {
		t.Fatalf("version = %q, want %q", out.Version, version)
	}
	if out.Decision != "pass" {
		t.Fatalf("decision = %q, want pass", out.Decision)
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

func TestRunDoctorAcceptsBaseHeadRefs(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--workdir", dir, "--base", "HEAD", "--head", "HEAD"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"repo.ref.base", "repo.ref.head"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
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

func TestRunInitWritesConfig(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--workdir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created .go-prism.yml") {
		t.Fatalf("stdout missing created message:\n%s", stdout.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, ".go-prism.yml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "module: example.com/project") {
		t.Fatalf("config missing module:\n%s", data)
	}
}

func TestRunInitDryRunWritesNoConfig(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--workdir", dir, "--dry-run"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# go-prism init dry run: .go-prism.yml") {
		t.Fatalf("stdout missing dry-run header:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".go-prism.yml")); !os.IsNotExist(err) {
		t.Fatalf("config exists after dry-run: %v", err)
	}
}

func TestRunInitJSON(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--workdir", dir, "--format", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}

	var out struct {
		SchemaVersion string   `json:"schema_version"`
		Status        string   `json:"status"`
		Module        string   `json:"module"`
		EnabledChecks []string `json:"enabled_checks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if out.SchemaVersion != "init.v1" {
		t.Fatalf("schema_version = %q", out.SchemaVersion)
	}
	if out.Status != "created" {
		t.Fatalf("status = %q", out.Status)
	}
	if out.Module != "example.com/project" {
		t.Fatalf("module = %q", out.Module)
	}
	if got := strings.Join(out.EnabledChecks, ","); got != "gomod" {
		t.Fatalf("enabled_checks = %q", got)
	}
}

func TestRunInitInvalidFormatFailsWithCode2(t *testing.T) {
	dir := writeTestModule(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--workdir", dir, "--dry-run", "--format", "xml"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}
	var coded interface{ ExitCode() int }
	if !errors.As(err, &coded) {
		t.Fatalf("run() error = %T, want coded error", err)
	}
	if coded.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", coded.ExitCode())
	}
}

func TestRunInitExistingConfigFails(t *testing.T) {
	dir := writeTestModule(t)
	configPath := filepath.Join(dir, ".go-prism.yml")
	if err := os.WriteFile(configPath, []byte("keep: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--workdir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep: true\n" {
		t.Fatalf("config overwritten:\n%s", data)
	}
}

func TestRunHelpMentionsDoctorAndInit(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "go-prism doctor [flags]") {
		t.Fatalf("help missing doctor:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "go-prism init [flags]") {
		t.Fatalf("help missing init:\n%s", stdout.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	if got, want := stdout.String(), "0.2.0\n"; got != want {
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
