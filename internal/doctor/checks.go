package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/config"
)

func checkRuntime(ctx context.Context, report *Report, runner command.Runner, name string, args []string, id string, required bool, recommendation string) bool {
	path, err := runner.LookPath(name)
	if err != nil {
		report.addCheck(Check{
			ID:             id,
			Status:         StatusFail,
			Message:        fmt.Sprintf("%s not found on PATH", name),
			Required:       required,
			Recommendation: recommendation,
		})
		return false
	}

	result := runner.Run(ctx, command.Invocation{
		Path: path,
		Args: args,
	})
	if result.Err != nil {
		report.addCheck(Check{
			ID:             id,
			Status:         StatusFail,
			Message:        trimCommandOutput(result.Stdout, result.Stderr, result.Err.Error()),
			Required:       required,
			Recommendation: recommendation,
		})
		return false
	}

	report.addCheck(Check{
		ID:       id,
		Status:   StatusOK,
		Message:  trimCommandOutput(result.Stdout, result.Stderr, path),
		Required: required,
	})
	return true
}

func checkGitRepo(ctx context.Context, report *Report, runner command.Runner) {
	gitPath, err := runner.LookPath("git")
	if err != nil {
		report.addCheck(Check{
			ID:             "repo.git",
			Status:         StatusWarn,
			Message:        "git not found on PATH",
			Required:       false,
			Recommendation: "Install git to let go-prism inspect repository context.",
		})
		return
	}
	result := runner.Run(ctx, command.Invocation{
		Path: gitPath,
		Args: []string{"rev-parse", "--is-inside-work-tree"},
		Dir:  report.WorkDir,
	})
	if result.Err != nil || strings.TrimSpace(result.Stdout) != "true" {
		report.addCheck(Check{
			ID:             "repo.git",
			Status:         StatusWarn,
			Message:        trimCommandOutput(result.Stdout, result.Stderr, "workdir is not inside a git repository"),
			Required:       false,
			Recommendation: "Run go-prism from a git checkout when base/head diff evidence is needed.",
		})
		return
	}
	report.addCheck(Check{
		ID:       "repo.git",
		Status:   StatusOK,
		Message:  "inside a git worktree",
		Required: false,
	})
}

func checkOptionalTool(ctx context.Context, report *Report, runner command.Runner, name string, enabled bool, id string, recommendation string) {
	path, err := runner.LookPath(name)
	if err == nil {
		report.addCheck(Check{
			ID:       id,
			Status:   StatusOK,
			Message:  path,
			Required: enabled,
		})
		return
	}
	if enabled {
		report.addCheck(Check{
			ID:             id,
			Status:         StatusWarn,
			Message:        fmt.Sprintf("%s not found on PATH", name),
			Required:       true,
			Recommendation: recommendation,
		})
		return
	}
	report.addCheck(Check{
		ID:       id,
		Status:   StatusOK,
		Message:  fmt.Sprintf("%s not found; not required because the check is disabled", name),
		Required: false,
	})

	_ = ctx
}

func checkDownstream(report *Report, workDir string, cfg config.Config) {
	if !cfg.Checks.Downstream.Enabled {
		report.addCheck(Check{
			ID:       "downstream.local",
			Status:   StatusOK,
			Message:  "downstream checks disabled",
			Required: false,
		})
		return
	}
	if len(cfg.Checks.Downstream.Modules) == 0 {
		report.addCheck(Check{
			ID:             "downstream.local",
			Status:         StatusWarn,
			Message:        "checks.downstream.enabled is true, but no modules are configured",
			Required:       true,
			Recommendation: "Add checks.downstream.modules entries or disable checks.downstream.",
		})
		return
	}

	for _, module := range cfg.Checks.Downstream.Modules {
		name := module.Name
		if strings.TrimSpace(name) == "" {
			name = "<unnamed>"
		}
		id := "downstream.local." + sanitizeID(name)
		path := strings.TrimSpace(module.Path)
		if path == "" {
			report.addCheck(Check{
				ID:             id,
				Status:         StatusWarn,
				Message:        fmt.Sprintf("%s has no path", name),
				Required:       true,
				Recommendation: "Set checks.downstream.modules[].path to a local Go module path.",
			})
			continue
		}

		resolved := resolveDownstreamPath(workDir, path)
		info, err := os.Stat(resolved)
		if err != nil {
			report.addCheck(Check{
				ID:             id,
				Status:         StatusWarn,
				Message:        fmt.Sprintf("%s: %v", resolved, err),
				Required:       true,
				Recommendation: "Make the configured downstream path available before running downstream checks.",
			})
			continue
		}
		if !info.IsDir() {
			report.addCheck(Check{
				ID:             id,
				Status:         StatusWarn,
				Message:        fmt.Sprintf("%s is not a directory", resolved),
				Required:       true,
				Recommendation: "Set downstream path to a local Go module directory.",
			})
			continue
		}
		if _, err := os.Stat(filepath.Join(resolved, "go.mod")); err != nil {
			report.addCheck(Check{
				ID:             id,
				Status:         StatusWarn,
				Message:        fmt.Sprintf("read downstream go.mod: %v", err),
				Required:       true,
				Recommendation: "Point downstream path at a directory containing go.mod.",
			})
			continue
		}

		commandText := strings.TrimSpace(module.Command)
		if commandText == "" {
			commandText = "go test ./..."
		}
		report.addCheck(Check{
			ID:       id,
			Status:   StatusOK,
			Message:  fmt.Sprintf("%s (%s)", resolved, commandText),
			Required: true,
		})
	}
}

func checkGitHubActions(report *Report, environ []string) {
	env := envMap(environ)
	if env["GITHUB_ACTIONS"] != "true" {
		report.addCheck(Check{
			ID:       "github.actions",
			Status:   StatusOK,
			Message:  "not running in GitHub Actions",
			Required: false,
		})
		return
	}
	if env["GITHUB_WORKSPACE"] == "" {
		report.addCheck(Check{
			ID:             "github.actions",
			Status:         StatusWarn,
			Message:        "GITHUB_ACTIONS=true but GITHUB_WORKSPACE is not set",
			Required:       false,
			Recommendation: "Run actions/checkout before go-prism in GitHub Actions.",
		})
		return
	}
	message := "GitHub Actions environment detected"
	if env["GITHUB_STEP_SUMMARY"] == "" {
		message += "; GITHUB_STEP_SUMMARY is not set"
	} else {
		message += "; step summary available"
	}
	report.addCheck(Check{
		ID:       "github.actions",
		Status:   StatusOK,
		Message:  message,
		Required: false,
	})
}

func resolveDownstreamPath(workDir string, path string) string {
	if filepath.IsAbs(path) {
		return absPath(path)
	}
	return absPath(filepath.Join(workDir, path))
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDot := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDot = false
			continue
		}
		if !lastDot {
			out.WriteByte('.')
			lastDot = true
		}
	}
	return strings.Trim(out.String(), ".")
}

func envMap(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, item := range environ {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}
