package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Athena900/go-prism/internal/config"
	"github.com/Athena900/go-prism/internal/evidence"
)

func TestRunPRIncludesAPIEvidenceWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "go-prism.yml")
	config := []byte(`checks:
  gomod:
    enabled: false
  api:
    enabled: true
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunPR(context.Background(), PROptions{
		Base:       "origin/main",
		Head:       "HEAD",
		ConfigPath: configPath,
		WorkDir:    dir,
	})
	if err != nil {
		t.Fatalf("RunPR() error = %v", err)
	}

	if !hasCategoryStatus(report.Items, evidence.CategoryAPI, evidence.StatusUnknown) {
		t.Fatalf("API unknown evidence missing in %#v", report.Items)
	}
	if report.Decision != evidence.StatusUnknown {
		t.Fatalf("Decision = %q, want %q", report.Decision, evidence.StatusUnknown)
	}
}

func TestRunPRIncludesVulnEvidenceWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	configPath := filepath.Join(dir, "go-prism.yml")
	config := []byte(`checks:
  gomod:
    enabled: false
  vuln:
    enabled: true
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunPR(context.Background(), PROptions{
		Base:       "origin/main",
		Head:       "HEAD",
		ConfigPath: configPath,
		WorkDir:    dir,
	})
	if err != nil {
		t.Fatalf("RunPR() error = %v", err)
	}

	if !hasCategoryStatus(report.Items, evidence.CategoryVuln, evidence.StatusUnknown) {
		t.Fatalf("vulnerability unknown evidence missing in %#v", report.Items)
	}
	if report.Decision != evidence.StatusUnknown {
		t.Fatalf("Decision = %q, want %q", report.Decision, evidence.StatusUnknown)
	}
}

func TestRunPRIncludesDownstreamEvidenceWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "go-prism.yml")
	config := []byte(`module: example.com/project
checks:
  gomod:
    enabled: false
  downstream:
    enabled: true
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunPR(context.Background(), PROptions{
		Base:       "origin/main",
		Head:       "HEAD",
		ConfigPath: configPath,
		WorkDir:    dir,
	})
	if err != nil {
		t.Fatalf("RunPR() error = %v", err)
	}

	if !hasCategoryStatus(report.Items, evidence.CategoryDownstream, evidence.StatusUnknown) {
		t.Fatalf("downstream unknown evidence missing in %#v", report.Items)
	}
}

func TestDownstreamModulesMapsRemoteConfig(t *testing.T) {
	modules := downstreamModules([]config.DownstreamModuleConfig{{
		Name:    "remote-consumer",
		Repo:    "https://github.com/example/consumer.git",
		Ref:     "main",
		Subdir:  "nested",
		Command: "go test ./...",
	}})

	if len(modules) != 1 {
		t.Fatalf("modules len = %d, want 1", len(modules))
	}
	module := modules[0]
	if module.Name != "remote-consumer" {
		t.Fatalf("Name = %q", module.Name)
	}
	if module.Repo != "https://github.com/example/consumer.git" {
		t.Fatalf("Repo = %q", module.Repo)
	}
	if module.Ref != "main" {
		t.Fatalf("Ref = %q", module.Ref)
	}
	if module.Subdir != "nested" {
		t.Fatalf("Subdir = %q", module.Subdir)
	}
	if module.Command != "go test ./..." {
		t.Fatalf("Command = %q", module.Command)
	}
}

func TestRunPRUsesVersionOption(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "go-prism.yml")
	if err := os.WriteFile(configPath, []byte("checks:\n  gomod:\n    enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunPR(context.Background(), PROptions{
		Base:       "origin/main",
		Head:       "HEAD",
		ConfigPath: configPath,
		WorkDir:    dir,
		Version:    "test-version",
	})
	if err != nil {
		t.Fatalf("RunPR() error = %v", err)
	}
	if report.Version != "test-version" {
		t.Fatalf("Version = %q, want test-version", report.Version)
	}
}

func hasCategoryStatus(items []evidence.Item, category evidence.Category, status evidence.Status) bool {
	for _, item := range items {
		if item.Category == category && item.Status == status {
			return true
		}
	}
	return false
}
