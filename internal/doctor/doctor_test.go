package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/command"
)

func TestRunDefaultConfigUsesDefaults(t *testing.T) {
	dir := writeModule(t, "module example.com/project\n\ngo 1.22\n")
	runner := newFakeRunner()

	report := Run(context.Background(), Options{
		ConfigPath: ".go-prism.yml",
		WorkDir:    dir,
		Version:    "test",
		Runner:     runner,
		Environ:    []string{},
	})

	if report.Status != StatusOK {
		t.Fatalf("status = %s, want ok: %+v", report.Status, report.Checks)
	}
	if !report.Config.UsedDefaults {
		t.Fatalf("UsedDefaults = false, want true")
	}
	if report.Module != "example.com/project" {
		t.Fatalf("Module = %q", report.Module)
	}
}

func TestRunExplicitMissingConfigFails(t *testing.T) {
	dir := writeModule(t, "module example.com/project\n\ngo 1.22\n")
	report := Run(context.Background(), Options{
		ConfigPath: filepath.Join(dir, "missing.yml"),
		WorkDir:    dir,
		Version:    "test",
		Runner:     newFakeRunner(),
		Environ:    []string{},
	})

	if report.Status != StatusFail {
		t.Fatalf("status = %s, want fail", report.Status)
	}
	if report.Config.Status != "failed" {
		t.Fatalf("config status = %q, want failed", report.Config.Status)
	}
}

func TestRunAPIEnabledMissingGoreleaseWarns(t *testing.T) {
	dir := writeModule(t, "module example.com/project\n\ngo 1.22\n")
	configPath := filepath.Join(dir, ".go-prism.yml")
	if err := os.WriteFile(configPath, []byte("checks:\n  api:\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := newFakeRunner()
	delete(runner.paths, "gorelease")

	report := Run(context.Background(), Options{
		ConfigPath: configPath,
		WorkDir:    dir,
		Version:    "test",
		Runner:     runner,
		Environ:    []string{},
	})

	if report.Status != StatusWarn {
		t.Fatalf("status = %s, want warn", report.Status)
	}
	if !hasCheck(report, "tool.gorelease", StatusWarn) {
		t.Fatalf("missing warning check for gorelease: %+v", report.Checks)
	}
}

func TestRunMissingGoModWithModuleOverrideWarns(t *testing.T) {
	dir := t.TempDir()
	report := Run(context.Background(), Options{
		ConfigPath:     ".go-prism.yml",
		WorkDir:        dir,
		ModuleOverride: "example.com/override",
		Version:        "test",
		Runner:         newFakeRunner(),
		Environ:        []string{},
	})

	if report.Status != StatusWarn {
		t.Fatalf("status = %s, want warn", report.Status)
	}
	if report.Module != "example.com/override" {
		t.Fatalf("Module = %q", report.Module)
	}
}

func TestRenderJSON(t *testing.T) {
	dir := writeModule(t, "module example.com/project\n\ngo 1.22\n")
	report := Run(context.Background(), Options{
		ConfigPath: ".go-prism.yml",
		WorkDir:    dir,
		Version:    "test",
		Runner:     newFakeRunner(),
		Environ:    []string{},
	})

	out, err := Render(report, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q", decoded.SchemaVersion)
	}
}

func TestRenderText(t *testing.T) {
	dir := writeModule(t, "module example.com/project\n\ngo 1.22\n")
	report := Run(context.Background(), Options{
		ConfigPath: ".go-prism.yml",
		WorkDir:    dir,
		Version:    "test",
		Runner:     newFakeRunner(),
		Environ:    []string{},
	})

	out, err := Render(report, FormatText)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"go-prism doctor", "Overall: OK", "runtime.go", "module.gomod"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("text output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	_, err := Render(Report{}, "xml")
	if err == nil {
		t.Fatal("Render() error = nil, want error")
	}
}

func writeModule(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func hasCheck(report Report, id string, status Status) bool {
	for _, check := range report.Checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
}

type fakeRunner struct {
	paths map[string]string
}

func newFakeRunner() fakeRunner {
	return fakeRunner{
		paths: map[string]string{
			"go":          "/bin/go",
			"git":         "/bin/git",
			"gorelease":   "/bin/gorelease",
			"govulncheck": "/bin/govulncheck",
		},
	}
}

func (f fakeRunner) LookPath(name string) (string, error) {
	path, ok := f.paths[name]
	if !ok {
		return "", errors.New("not found")
	}
	return path, nil
}

func (f fakeRunner) Run(ctx context.Context, invocation command.Invocation) command.Result {
	_ = ctx
	switch filepath.Base(invocation.Path) {
	case "go":
		return command.Result{Stdout: "go version go1.22.0 test\n"}
	case "git":
		if len(invocation.Args) > 0 && invocation.Args[0] == "--version" {
			return command.Result{Stdout: "git version 2.54.0\n"}
		}
		return command.Result{Stdout: "true\n"}
	default:
		return command.Result{}
	}
}
