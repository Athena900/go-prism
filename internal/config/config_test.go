package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultWhenDefaultFileMissing(t *testing.T) {
	cfg, err := Load(".go-prism.yml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Checks.GoMod.Enabled {
		t.Fatal("default GoMod check should be enabled")
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(`module: example.com/mod
checks:
  gomod:
    enabled: true
  downstream:
    enabled: true
    modules:
      - name: consumer
        path: ../consumer
        command: go test ./...
ai:
  enabled: false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Module != "example.com/mod" {
		t.Fatalf("Module = %q, want example.com/mod", cfg.Module)
	}
	if !cfg.Checks.GoMod.Enabled {
		t.Fatal("GoMod check should be enabled")
	}
	if !cfg.Checks.Downstream.Enabled {
		t.Fatal("Downstream check should be enabled")
	}
	if len(cfg.Checks.Downstream.Modules) != 1 {
		t.Fatalf("Downstream modules len = %d, want 1", len(cfg.Checks.Downstream.Modules))
	}
	if got := cfg.Checks.Downstream.Modules[0].Command; got != "go test ./..." {
		t.Fatalf("Downstream command = %q, want go test ./...", got)
	}
}

func TestLoadRejectsDownstreamModuleWithoutPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(`checks:
  downstream:
    enabled: true
    modules:
      - name: consumer
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}
