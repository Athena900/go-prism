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

func TestLoadAcceptsRemoteDownstreamModule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(`checks:
  downstream:
    enabled: true
    modules:
      - name: remote-consumer
        repo: https://github.com/example/consumer.git
        ref: main
        subdir: nested/module
        command: go test ./...
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	module := cfg.Checks.Downstream.Modules[0]
	if module.Repo != "https://github.com/example/consumer.git" {
		t.Fatalf("Repo = %q", module.Repo)
	}
	if module.Ref != "main" {
		t.Fatalf("Ref = %q", module.Ref)
	}
	if module.Subdir != "nested/module" {
		t.Fatalf("Subdir = %q", module.Subdir)
	}
}

func TestLoadRejectsDownstreamModuleWithoutPathOrRepo(t *testing.T) {
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

func TestLoadRejectsDownstreamModuleWithPathAndRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(`checks:
  downstream:
    enabled: true
    modules:
      - name: consumer
        path: ../consumer
        repo: https://github.com/example/consumer.git
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestLoadRejectsUnsafeRemoteDownstreamConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "credential bearing URL",
			body: "repo: https://token@github.com/example/consumer.git\n",
		},
		{
			name: "non HTTPS URL",
			body: "repo: ssh://github.com/example/consumer.git\n",
		},
		{
			name: "query string",
			body: "repo: https://github.com/example/consumer.git?token=abc\n",
		},
		{
			name: "absolute subdir",
			body: "repo: https://github.com/example/consumer.git\n        subdir: /tmp/module\n",
		},
		{
			name: "escaping subdir",
			body: "repo: https://github.com/example/consumer.git\n        subdir: ../module\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yml")
			if err := os.WriteFile(path, []byte(`checks:
  downstream:
    enabled: true
    modules:
      - name: consumer
        `+tt.body), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(path)
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}
