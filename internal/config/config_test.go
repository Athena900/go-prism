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
	if err := os.WriteFile(path, []byte("module: example.com/mod\nchecks:\n  gomod:\n    enabled: true\nai:\n  enabled: false\n"), 0o644); err != nil {
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
}
