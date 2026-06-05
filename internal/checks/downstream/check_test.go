package downstream

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

func TestCheckWithRunnerNoModulesIsUnknown(t *testing.T) {
	items := CheckWithRunner(context.Background(), Options{
		WorkDir:    ".",
		ModulePath: "example.com/lib",
	}, command.LocalRunner{})

	assertHasStatus(t, items, "downstream.no_modules", evidence.StatusUnknown)
}

func TestCheckWithRunnerMissingModulePathIsUnknown(t *testing.T) {
	items := CheckWithRunner(context.Background(), Options{
		WorkDir: ".",
		Modules: []Module{{Name: "consumer", Path: "."}},
	}, command.LocalRunner{})

	assertHasStatus(t, items, "downstream.module_path_missing", evidence.StatusUnknown)
}

func TestCheckWithRunnerPassingLocalCanaryRestoresGoMod(t *testing.T) {
	targetDir, downstreamDir := createCanaryModules(t, "ok")
	originalGoMod := readFile(t, filepath.Join(downstreamDir, "go.mod"))

	items := CheckWithRunner(context.Background(), Options{
		WorkDir:    targetDir,
		Base:       "origin/main",
		Head:       "HEAD",
		ModulePath: "example.com/lib",
		Modules: []Module{{
			Name: "consumer",
			Path: downstreamDir,
		}},
	}, command.LocalRunner{})

	assertHasStatus(t, items, "downstream.passed.consumer", evidence.StatusPass)
	if got := readFile(t, filepath.Join(downstreamDir, "go.mod")); got != originalGoMod {
		t.Fatalf("go.mod was not restored:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(downstreamDir, "go.sum")); !os.IsNotExist(err) {
		t.Fatalf("go.sum should not remain after restore, err=%v", err)
	}
}

func TestCheckWithRunnerFailingLocalCanaryBlocksAndRestoresGoMod(t *testing.T) {
	targetDir, downstreamDir := createCanaryModules(t, "bad")
	originalGoMod := readFile(t, filepath.Join(downstreamDir, "go.mod"))

	items := CheckWithRunner(context.Background(), Options{
		WorkDir:    targetDir,
		Base:       "origin/main",
		Head:       "HEAD",
		ModulePath: "example.com/lib",
		Modules: []Module{{
			Name:    "consumer",
			Path:    downstreamDir,
			Command: "go test ./...",
		}},
	}, command.LocalRunner{})

	assertHasStatus(t, items, "downstream.command_failed.consumer", evidence.StatusBlock)
	if !strings.Contains(strings.Join(items[0].Details, "\n"), "expected ok") {
		t.Fatalf("failure details missing test output: %#v", items[0].Details)
	}
	if got := readFile(t, filepath.Join(downstreamDir, "go.mod")); got != originalGoMod {
		t.Fatalf("go.mod was not restored:\n%s", got)
	}
}

func createCanaryModules(t *testing.T, value string) (string, string) {
	t.Helper()
	root := t.TempDir()
	targetDir := filepath.Join(root, "target")
	downstreamDir := filepath.Join(root, "consumer")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(downstreamDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(targetDir, "go.mod"), "module example.com/lib\n\ngo 1.22.0\n")
	writeFile(t, filepath.Join(targetDir, "lib.go"), "package lib\n\nfunc Value() string { return \""+value+"\" }\n")
	writeFile(t, filepath.Join(downstreamDir, "go.mod"), "module example.com/consumer\n\ngo 1.22.0\n\nrequire example.com/lib v0.0.0\n")
	writeFile(t, filepath.Join(downstreamDir, "consumer_test.go"), `package consumer

import (
	"testing"

	"example.com/lib"
)

func TestValue(t *testing.T) {
	if got := lib.Value(); got != "ok" {
		t.Fatalf("expected ok, got %s", got)
	}
}
`)

	return targetDir, downstreamDir
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertHasStatus(t *testing.T, items []evidence.Item, id string, status evidence.Status) {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			if item.Status != status {
				t.Fatalf("%s status = %q, want %q", id, item.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing evidence item %q in %#v", id, items)
}
