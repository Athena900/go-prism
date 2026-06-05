package downstream

import (
	"context"
	"errors"
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

func TestCheckWithRunnerPassingRemoteCanaryCleansTemporaryClone(t *testing.T) {
	targetDir := createTargetModule(t, "ok")
	runner := &remoteFakeRunner{subdir: "nested"}

	items := CheckWithRunner(context.Background(), Options{
		WorkDir:    targetDir,
		Base:       "origin/main",
		Head:       "HEAD",
		ModulePath: "example.com/lib",
		Modules: []Module{{
			Name:   "remote-consumer",
			Repo:   "https://github.com/example/consumer.git",
			Ref:    "main",
			Subdir: "nested",
		}},
	}, runner)

	assertHasStatus(t, items, "downstream.passed.remote-consumer", evidence.StatusPass)
	if runner.cloneDir == "" {
		t.Fatal("clone dir was not recorded")
	}
	if _, err := os.Stat(runner.cloneDir); !os.IsNotExist(err) {
		t.Fatalf("temporary clone should be removed, err=%v", err)
	}
}

func TestCheckWithRunnerRemoteFailureMapping(t *testing.T) {
	tests := []struct {
		name       string
		runner     *remoteFakeRunner
		wantID     string
		wantStatus evidence.Status
	}{
		{
			name:       "clone failure",
			runner:     &remoteFakeRunner{failClone: true},
			wantID:     "downstream.remote.clone_failed.remote-consumer",
			wantStatus: evidence.StatusUnknown,
		},
		{
			name:       "fetch failure",
			runner:     &remoteFakeRunner{failFetch: true},
			wantID:     "downstream.remote.checkout_failed.remote-consumer",
			wantStatus: evidence.StatusUnknown,
		},
		{
			name:       "checkout failure",
			runner:     &remoteFakeRunner{failCheckout: true},
			wantID:     "downstream.remote.checkout_failed.remote-consumer",
			wantStatus: evidence.StatusUnknown,
		},
		{
			name:       "module missing",
			runner:     &remoteFakeRunner{missingModule: true},
			wantID:     "downstream.remote.module_missing.remote-consumer",
			wantStatus: evidence.StatusUnknown,
		},
		{
			name:       "replace failure",
			runner:     &remoteFakeRunner{failReplace: true},
			wantID:     "downstream.replace_failed.remote-consumer",
			wantStatus: evidence.StatusUnknown,
		},
		{
			name:       "command failure",
			runner:     &remoteFakeRunner{failCommand: true},
			wantID:     "downstream.command_failed.remote-consumer",
			wantStatus: evidence.StatusBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetDir := createTargetModule(t, "ok")

			items := CheckWithRunner(context.Background(), Options{
				WorkDir:    targetDir,
				Base:       "origin/main",
				Head:       "HEAD",
				ModulePath: "example.com/lib",
				Modules: []Module{{
					Name: "remote-consumer",
					Repo: "https://github.com/example/consumer.git",
					Ref:  "main",
				}},
			}, tt.runner)

			assertHasStatus(t, items, tt.wantID, tt.wantStatus)
			if tt.runner.cloneDir != "" {
				if _, err := os.Stat(tt.runner.cloneDir); !os.IsNotExist(err) {
					t.Fatalf("temporary clone should be removed, err=%v", err)
				}
			}
		})
	}
}

func TestCheckWithRunnerRemoteRejectsUnsafeRepo(t *testing.T) {
	items := CheckWithRunner(context.Background(), Options{
		WorkDir:    ".",
		ModulePath: "example.com/lib",
		Modules: []Module{{
			Name: "remote-consumer",
			Repo: "https://token@github.com/example/consumer.git",
		}},
	}, &remoteFakeRunner{})

	assertHasStatus(t, items, "downstream.remote.config_failed.remote-consumer", evidence.StatusUnknown)
	if got := items[0].Provenance.Extra["repo"]; strings.Contains(got, "token") {
		t.Fatalf("unsafe repo was not redacted: %q", got)
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

func createTargetModule(t *testing.T, value string) string {
	t.Helper()
	targetDir := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(targetDir, "go.mod"), "module example.com/lib\n\ngo 1.22.0\n")
	writeFile(t, filepath.Join(targetDir, "lib.go"), "package lib\n\nfunc Value() string { return \""+value+"\" }\n")
	return targetDir
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

type remoteFakeRunner struct {
	subdir        string
	failClone     bool
	failFetch     bool
	failCheckout  bool
	missingModule bool
	failReplace   bool
	failCommand   bool
	cloneDir      string
}

func (r *remoteFakeRunner) LookPath(name string) (string, error) {
	return "/bin/" + name, nil
}

func (r *remoteFakeRunner) Run(_ context.Context, invocation command.Invocation) command.Result {
	switch filepath.Base(invocation.Path) {
	case "git":
		return r.runGit(invocation)
	case "go":
		if r.failReplace {
			return command.Result{
				Stderr:   "replace failed token=abc123\n",
				ExitCode: 1,
				Err:      errors.New("exit status 1"),
			}
		}
		return command.Result{}
	case "sh":
		if r.failCommand {
			return command.Result{
				Stderr:   "expected ok, got bad\n",
				ExitCode: 1,
				Err:      errors.New("exit status 1"),
			}
		}
		return command.Result{Stdout: "ok\n"}
	default:
		return command.Result{}
	}
}

func (r *remoteFakeRunner) runGit(invocation command.Invocation) command.Result {
	if len(invocation.Args) > 0 && invocation.Args[0] == "clone" {
		if r.failClone {
			return command.Result{
				Stderr:   "clone failed\n",
				ExitCode: 128,
				Err:      errors.New("exit status 128"),
			}
		}
		cloneDir := invocation.Args[len(invocation.Args)-1]
		r.cloneDir = cloneDir
		moduleDir := cloneDir
		if r.subdir != "" {
			moduleDir = filepath.Join(cloneDir, r.subdir)
		}
		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			return command.Result{Err: err}
		}
		if !r.missingModule {
			if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte("module example.com/consumer\n\ngo 1.22.0\n"), 0o644); err != nil {
				return command.Result{Err: err}
			}
		}
		return command.Result{Stdout: "cloned\n"}
	}
	if len(invocation.Args) > 2 && invocation.Args[2] == "fetch" && r.failFetch {
		return command.Result{
			Stderr:   "fetch failed\n",
			ExitCode: 128,
			Err:      errors.New("exit status 128"),
		}
	}
	if len(invocation.Args) > 2 && invocation.Args[2] == "checkout" && r.failCheckout {
		return command.Result{
			Stderr:   "checkout failed\n",
			ExitCode: 128,
			Err:      errors.New("exit status 128"),
		}
	}
	return command.Result{}
}
