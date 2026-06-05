package initconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/config"
	"gopkg.in/yaml.v3"
)

func TestRunDefaultCreatesConfig(t *testing.T) {
	dir := writeModule(t, "example.com/project")

	result, err := Run(Options{WorkDir: dir})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusCreated {
		t.Fatalf("Status = %q, want %q", result.Status, StatusCreated)
	}
	if result.Module != "example.com/project" {
		t.Fatalf("Module = %q, want example.com/project", result.Module)
	}

	data := readConfig(t, dir)
	for _, want := range []string{
		"module: example.com/project",
		"  gomod:\n    enabled: true",
		"  api:\n    enabled: false",
		"  vuln:\n    enabled: false",
		"    modules: []",
		"    gomod_parse_error: true",
		"    new_replace_directive: false",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("config missing %q:\n%s", want, data)
		}
	}
}

func TestRunModuleOverrideWins(t *testing.T) {
	dir := writeModule(t, "example.com/project")

	result, err := Run(Options{WorkDir: dir, ModuleOverride: "example.com/override", DryRun: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Module != "example.com/override" {
		t.Fatalf("Module = %q, want override", result.Module)
	}
	if !strings.Contains(result.YAML, "module: example.com/override") {
		t.Fatalf("YAML missing override:\n%s", result.YAML)
	}
}

func TestRunMissingGoModWithoutModuleOverrideFails(t *testing.T) {
	dir := t.TempDir()

	result, err := Run(Options{WorkDir: dir})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if ExitCode(err) != 1 {
		t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
	}
	if result.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}

func TestRunExistingOutputWithoutForceFails(t *testing.T) {
	dir := writeModule(t, "example.com/project")
	path := filepath.Join(dir, ".go-prism.yml")
	if err := os.WriteFile(path, []byte("keep: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(Options{WorkDir: dir})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if result.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
	if got := readFile(t, path); got != "keep: true\n" {
		t.Fatalf("config was overwritten:\n%s", got)
	}
}

func TestRunExistingOutputWithForceOverwrites(t *testing.T) {
	dir := writeModule(t, "example.com/project")
	path := filepath.Join(dir, ".go-prism.yml")
	if err := os.WriteFile(path, []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(Options{WorkDir: dir, Force: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Overwritten {
		t.Fatal("Overwritten = false, want true")
	}
	if !strings.Contains(readFile(t, path), "module: example.com/project") {
		t.Fatalf("config was not overwritten:\n%s", readFile(t, path))
	}
}

func TestRunDryRunWritesNoFile(t *testing.T) {
	dir := writeModule(t, "example.com/project")

	result, err := Run(Options{WorkDir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPreview {
		t.Fatalf("Status = %q, want preview", result.Status)
	}
	if result.YAML == "" {
		t.Fatal("YAML is empty")
	}
	if _, err := os.Stat(filepath.Join(dir, ".go-prism.yml")); !os.IsNotExist(err) {
		t.Fatalf("config file exists after dry-run: %v", err)
	}
}

func TestRunEnablesOptionalChecks(t *testing.T) {
	dir := writeModule(t, "example.com/project")

	result, err := Run(Options{WorkDir: dir, EnableAPI: true, EnableVuln: true, DryRun: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := strings.Join(result.EnabledChecks, ","); got != "gomod,api,vuln" {
		t.Fatalf("EnabledChecks = %q", got)
	}
	if !strings.Contains(result.YAML, "  api:\n    enabled: true") {
		t.Fatalf("YAML missing enabled api:\n%s", result.YAML)
	}
	if !strings.Contains(result.YAML, "  vuln:\n    enabled: true") {
		t.Fatalf("YAML missing enabled vuln:\n%s", result.YAML)
	}
}

func TestRunDownstreamFlagEnablesDownstream(t *testing.T) {
	dir := writeModule(t, "example.com/project")

	result, err := Run(Options{
		WorkDir: dir,
		DryRun:  true,
		Downstream: []DownstreamInput{
			{Name: "consumer", Path: "../consumer"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := strings.Join(result.EnabledChecks, ","); got != "gomod,downstream" {
		t.Fatalf("EnabledChecks = %q", got)
	}
	for _, want := range []string{
		"  downstream:\n    enabled: true",
		"      - name: consumer",
		"        path: ../consumer",
		"        command: go test ./...",
	} {
		if !strings.Contains(result.YAML, want) {
			t.Fatalf("YAML missing %q:\n%s", want, result.YAML)
		}
	}
}

func TestParseDownstreamRejectsInvalidValue(t *testing.T) {
	_, err := ParseDownstream("consumer")
	if err == nil {
		t.Fatal("ParseDownstream() error = nil, want error")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2", ExitCode(err))
	}
}

func TestRenderJSONIncludesSchemaVersion(t *testing.T) {
	dir := writeModule(t, "example.com/project")
	result, err := Run(Options{WorkDir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := Render(result, FormatJSON)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var out Result
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	if out.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", out.SchemaVersion, SchemaVersion)
	}
}

func TestRenderYAMLCanLoadAsConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Module = "example.com/project"
	cfg.Checks.Downstream.Enabled = true
	cfg.Checks.Downstream.Modules = []config.DownstreamModuleConfig{
		{Name: "consumer", Path: "../consumer", Command: "go test ./..."},
	}
	cfg.Policy.FailOn["new_replace_directive"] = false

	var loaded config.Config
	if err := yaml.Unmarshal([]byte(RenderYAML(cfg)), &loaded); err != nil {
		t.Fatalf("generated YAML is invalid: %v", err)
	}
	if loaded.Module != cfg.Module {
		t.Fatalf("Module = %q, want %q", loaded.Module, cfg.Module)
	}
	if len(loaded.Checks.Downstream.Modules) != 1 {
		t.Fatalf("Downstream modules len = %d, want 1", len(loaded.Checks.Downstream.Modules))
	}
}

func writeModule(t *testing.T, modulePath string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readConfig(t *testing.T, dir string) string {
	t.Helper()
	return readFile(t, filepath.Join(dir, ".go-prism.yml"))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
