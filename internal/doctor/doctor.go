package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/config"
	"golang.org/x/mod/modfile"
)

const (
	SchemaVersion = "doctor.v1"

	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Status is the normalized doctor status.
type Status string

// Options configures a doctor run.
type Options struct {
	ConfigPath     string
	WorkDir        string
	ModuleOverride string
	Base           string
	Head           string
	Format         string
	Timeout        time.Duration
	Version        string
	Runner         command.Runner
	Environ        []string
}

// Report is the machine-readable doctor result.
type Report struct {
	SchemaVersion string       `json:"schema_version"`
	Status        Status       `json:"status"`
	Version       string       `json:"version"`
	WorkDir       string       `json:"workdir"`
	Module        string       `json:"module"`
	Base          string       `json:"base,omitempty"`
	Head          string       `json:"head,omitempty"`
	Config        ConfigStatus `json:"config"`
	Checks        []Check      `json:"checks"`
	NextSteps     []string     `json:"next_steps"`
}

// ConfigStatus describes config loading behavior.
type ConfigStatus struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	UsedDefaults bool   `json:"used_defaults"`
	Message      string `json:"message,omitempty"`
}

// Check describes one setup diagnostic.
type Check struct {
	ID             string `json:"id"`
	Status         Status `json:"status"`
	Message        string `json:"message"`
	Required       bool   `json:"required"`
	Recommendation string `json:"recommendation"`
}

// Run executes read-only environment diagnostics.
func Run(ctx context.Context, opts Options) Report {
	opts = normalizeOptions(opts)

	report := Report{
		SchemaVersion: SchemaVersion,
		Status:        StatusOK,
		Version:       opts.Version,
		WorkDir:       absPath(opts.WorkDir),
		Base:          strings.TrimSpace(opts.Base),
		Head:          strings.TrimSpace(opts.Head),
		Config: ConfigStatus{
			Path: opts.ConfigPath,
		},
	}

	report.addCheck(Check{
		ID:       "doctor.context",
		Status:   StatusOK,
		Message:  fmt.Sprintf("version %s, workdir %s", report.Version, report.WorkDir),
		Required: true,
	})

	workDirOK := checkWorkDir(&report)
	checkRuntime(ctx, &report, opts.Runner, "go", []string{"version"}, "runtime.go", true, "Install Go and make sure `go` is on PATH.")
	gitOK := checkRuntime(ctx, &report, opts.Runner, "git", []string{"--version"}, "runtime.git", true, "Install git and make sure `git` is on PATH.")

	cfg, cfgOK := checkConfig(&report, opts.ConfigPath)
	moduleFromGoMod := checkGoMod(&report, opts.WorkDir, opts.ModuleOverride, workDirOK)
	if opts.ModuleOverride != "" {
		report.Module = opts.ModuleOverride
	} else if cfgOK && cfg.Module != "" {
		report.Module = cfg.Module
	} else {
		report.Module = moduleFromGoMod
	}

	if workDirOK && gitOK {
		if checkGitRepo(ctx, &report, opts.Runner) {
			checkGitHistory(ctx, &report, opts.Runner)
			checkGitRefs(ctx, &report, opts.Runner, opts.Base, opts.Head)
		}
	} else if workDirOK {
		report.addCheck(Check{
			ID:             "repo.git",
			Status:         StatusWarn,
			Message:        "skipped because git is unavailable",
			Required:       false,
			Recommendation: "Install git to let go-prism inspect repository context.",
		})
	}

	if cfgOK {
		checkOptionalTool(ctx, &report, opts.Runner, "gorelease", cfg.Checks.API.Enabled, "tool.gorelease", "Install gorelease if enabling checks.api: go install golang.org/x/exp/cmd/gorelease@latest")
		checkOptionalTool(ctx, &report, opts.Runner, "modver", cfg.Checks.API.Enabled, "tool.modver", "Install modver for supplemental API/SemVer evidence: go install github.com/bobg/modver/v2/cmd/modver@latest")
		checkOptionalTool(ctx, &report, opts.Runner, "go-apidiff", cfg.Checks.API.Enabled, "tool.go-apidiff", "Install go-apidiff for supplemental API compatibility evidence: go install github.com/joelanford/go-apidiff@latest")
		checkOptionalTool(ctx, &report, opts.Runner, "govulncheck", cfg.Checks.Vuln.Enabled, "tool.govulncheck", "Install govulncheck if enabling checks.vuln: go install golang.org/x/vuln/cmd/govulncheck@latest")
		checkDownstream(&report, opts.WorkDir, cfg)
	}

	checkGitHubActions(&report, opts.Environ)
	report.finish()
	return report
}

func normalizeOptions(opts Options) Options {
	if opts.ConfigPath == "" {
		opts.ConfigPath = ".go-prism.yml"
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	if opts.Format == "" {
		opts.Format = FormatText
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.Version == "" {
		opts.Version = "unknown"
	}
	if opts.Runner == nil {
		opts.Runner = command.LocalRunner{}
	}
	if opts.Environ == nil {
		opts.Environ = os.Environ()
	}
	return opts
}

func (r *Report) addCheck(check Check) {
	r.Checks = append(r.Checks, check)
	if check.Recommendation != "" && (check.Status == StatusWarn || check.Status == StatusFail) {
		r.NextSteps = append(r.NextSteps, check.Recommendation)
	}
}

func (r *Report) finish() {
	seen := map[string]bool{}
	nextSteps := make([]string, 0, len(r.NextSteps))
	for _, step := range r.NextSteps {
		if seen[step] {
			continue
		}
		seen[step] = true
		nextSteps = append(nextSteps, step)
	}
	r.NextSteps = nextSteps

	status := StatusOK
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			status = StatusFail
			break
		}
		if check.Status == StatusWarn {
			status = StatusWarn
		}
	}
	r.Status = status
}

func checkWorkDir(report *Report) bool {
	info, err := os.Stat(report.WorkDir)
	if err != nil {
		report.addCheck(Check{
			ID:             "repo.workdir",
			Status:         StatusFail,
			Message:        err.Error(),
			Required:       true,
			Recommendation: "Pass --workdir to an existing Go module directory.",
		})
		return false
	}
	if !info.IsDir() {
		report.addCheck(Check{
			ID:             "repo.workdir",
			Status:         StatusFail,
			Message:        fmt.Sprintf("%s is not a directory", report.WorkDir),
			Required:       true,
			Recommendation: "Pass --workdir to a Go module directory.",
		})
		return false
	}
	report.addCheck(Check{
		ID:       "repo.workdir",
		Status:   StatusOK,
		Message:  report.WorkDir,
		Required: true,
	})
	return true
}

func checkConfig(report *Report, path string) (config.Config, bool) {
	status := ConfigStatus{Path: path}
	if path == "" {
		status.Status = "defaults"
		status.UsedDefaults = true
		status.Message = "no config path provided; using defaults"
	} else if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) && path == ".go-prism.yml" {
			status.Status = "defaults"
			status.UsedDefaults = true
			status.Message = "default config file not found; using defaults"
		} else {
			status.Status = "failed"
			status.Message = err.Error()
		}
	} else {
		status.Status = "loaded"
		status.Message = "config loaded"
	}

	cfg, err := config.Load(path)
	if err != nil {
		report.Config = ConfigStatus{
			Path:    path,
			Status:  "failed",
			Message: err.Error(),
		}
		report.addCheck(Check{
			ID:             "config.load",
			Status:         StatusFail,
			Message:        err.Error(),
			Required:       true,
			Recommendation: "Fix the go-prism config file or pass --config to a valid YAML file.",
		})
		return config.Config{}, false
	}

	report.Config = status
	report.addCheck(Check{
		ID:       "config.load",
		Status:   StatusOK,
		Message:  status.Message,
		Required: true,
	})
	return cfg, true
}

func checkGoMod(report *Report, workDir string, moduleOverride string, workDirOK bool) string {
	if !workDirOK {
		report.addCheck(Check{
			ID:             "module.gomod",
			Status:         StatusFail,
			Message:        "skipped because workdir is not accessible",
			Required:       true,
			Recommendation: "Pass --workdir to a readable Go module directory.",
		})
		return ""
	}

	goModPath := filepath.Join(absPath(workDir), "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		if moduleOverride != "" {
			report.addCheck(Check{
				ID:             "module.gomod",
				Status:         StatusWarn,
				Message:        fmt.Sprintf("unable to read go.mod; using module override %s", moduleOverride),
				Required:       true,
				Recommendation: "Run doctor from a Go module root when go.mod evidence is needed.",
			})
			return moduleOverride
		}
		report.addCheck(Check{
			ID:             "module.gomod",
			Status:         StatusFail,
			Message:        err.Error(),
			Required:       true,
			Recommendation: "Run go-prism from a Go module root or pass --workdir to the module directory.",
		})
		return ""
	}

	file, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		report.addCheck(Check{
			ID:             "module.gomod",
			Status:         StatusFail,
			Message:        err.Error(),
			Required:       true,
			Recommendation: "Fix go.mod syntax before trusting go-prism output.",
		})
		return ""
	}
	if file.Module == nil || file.Module.Mod.Path == "" {
		if moduleOverride != "" {
			report.addCheck(Check{
				ID:             "module.gomod",
				Status:         StatusWarn,
				Message:        fmt.Sprintf("go.mod has no module directive; using module override %s", moduleOverride),
				Required:       true,
				Recommendation: "Add a module directive to go.mod before publishing release evidence.",
			})
			return moduleOverride
		}
		report.addCheck(Check{
			ID:             "module.gomod",
			Status:         StatusFail,
			Message:        "go.mod does not declare a module path",
			Required:       true,
			Recommendation: "Add a module directive to go.mod before using go-prism.",
		})
		return ""
	}

	message := "module " + file.Module.Mod.Path
	if file.Go != nil && file.Go.Version != "" {
		message += ", go " + file.Go.Version
	}
	report.addCheck(Check{
		ID:       "module.gomod",
		Status:   StatusOK,
		Message:  message,
		Required: true,
	})
	return file.Module.Mod.Path
}

func absPath(path string) string {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

func trimCommandOutput(stdout string, stderr string, fallback string) string {
	value := strings.TrimSpace(stdout)
	if value == "" {
		value = strings.TrimSpace(stderr)
	}
	if value == "" {
		return fallback
	}
	return value
}
