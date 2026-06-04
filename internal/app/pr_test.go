package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

func hasCategoryStatus(items []evidence.Item, category evidence.Category, status evidence.Status) bool {
	for _, item := range items {
		if item.Category == category && item.Status == status {
			return true
		}
	}
	return false
}
