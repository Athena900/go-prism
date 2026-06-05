package initconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Athena900/go-prism/internal/config"
	"golang.org/x/mod/modfile"
)

const (
	SchemaVersion = "init.v1"

	StatusCreated Status = "created"
	StatusPreview Status = "preview"
	StatusFailed  Status = "failed"
)

// Status is the normalized init operation status.
type Status string

// Options configures config generation.
type Options struct {
	WorkDir          string
	OutputPath       string
	ModuleOverride   string
	EnableAPI        bool
	EnableVuln       bool
	EnableDownstream bool
	Downstream       []DownstreamInput
	Force            bool
	DryRun           bool
	Format           string
}

// Result is the machine-readable init operation envelope.
type Result struct {
	SchemaVersion string   `json:"schema_version"`
	Status        Status   `json:"status"`
	Path          string   `json:"path"`
	Module        string   `json:"module"`
	EnabledChecks []string `json:"enabled_checks"`
	DryRun        bool     `json:"dry_run"`
	Overwritten   bool     `json:"overwritten"`
	NextSteps     []string `json:"next_steps"`
	YAML          string   `json:"yaml,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// Run generates a go-prism config and writes it unless dry-run is enabled.
func Run(opts Options) (Result, error) {
	opts = normalizeOptions(opts)

	workDir, err := resolveWorkDir(opts.WorkDir)
	if err != nil {
		result := failedResult(opts, "", "", err)
		return result, err
	}

	outputPath, displayPath, err := resolveOutputPath(workDir, opts.OutputPath)
	if err != nil {
		result := failedResult(opts, "", "", err)
		return result, err
	}

	modulePath, err := detectModulePath(workDir, opts.ModuleOverride)
	if err != nil {
		result := failedResult(opts, displayPath, "", err)
		return result, err
	}

	cfg := buildConfig(modulePath, opts)
	renderedYAML := RenderYAML(cfg)
	exists := fileExists(outputPath)

	result := Result{
		SchemaVersion: SchemaVersion,
		Status:        StatusCreated,
		Path:          displayPath,
		Module:        modulePath,
		EnabledChecks: enabledChecks(opts),
		DryRun:        opts.DryRun,
		Overwritten:   exists && opts.Force && !opts.DryRun,
		NextSteps:     defaultNextSteps(),
	}

	if opts.DryRun {
		result.Status = StatusPreview
		result.YAML = renderedYAML
		return result, nil
	}

	if exists && !opts.Force {
		err := newError(1, "%s already exists; pass --force to overwrite or --dry-run to preview", displayPath)
		result.Status = StatusFailed
		result.Error = err.Error()
		return result, err
	}

	if err := ensureParentDirectory(outputPath); err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		return result, err
	}
	if err := os.WriteFile(outputPath, []byte(renderedYAML), 0o644); err != nil {
		err = newError(1, "write %s: %w", displayPath, err)
		result.Status = StatusFailed
		result.Error = err.Error()
		return result, err
	}

	return result, nil
}

func normalizeOptions(opts Options) Options {
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}
	if opts.OutputPath == "" {
		opts.OutputPath = ".go-prism.yml"
	}
	if opts.Format == "" {
		opts.Format = FormatText
	}
	if len(opts.Downstream) > 0 {
		opts.EnableDownstream = true
	}
	opts.ModuleOverride = strings.TrimSpace(opts.ModuleOverride)
	return opts
}

func buildConfig(modulePath string, opts Options) config.Config {
	cfg := config.Default()
	cfg.Module = modulePath
	cfg.Checks.API.Enabled = opts.EnableAPI
	cfg.Checks.Vuln.Enabled = opts.EnableVuln
	cfg.Checks.Downstream.Enabled = opts.EnableDownstream
	cfg.Checks.Downstream.Modules = make([]config.DownstreamModuleConfig, 0, len(opts.Downstream))
	for _, downstream := range opts.Downstream {
		cfg.Checks.Downstream.Modules = append(cfg.Checks.Downstream.Modules, config.DownstreamModuleConfig{
			Name:    downstream.Name,
			Path:    downstream.Path,
			Command: defaultDownstreamCommand,
		})
	}
	cfg.Policy.FailOn["new_replace_directive"] = false
	return cfg
}

func resolveWorkDir(workDir string) (string, error) {
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return "", newError(1, "resolve workdir %q: %w", workDir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", newError(1, "read workdir %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return "", newError(1, "%s is not a directory", workDir)
	}
	return abs, nil
}

func resolveOutputPath(workDir string, outputPath string) (string, string, error) {
	if outputPath == "" {
		outputPath = ".go-prism.yml"
	}
	if filepath.IsAbs(outputPath) {
		return filepath.Clean(outputPath), filepath.Clean(outputPath), nil
	}
	return filepath.Join(workDir, outputPath), filepath.Clean(outputPath), nil
}

func detectModulePath(workDir string, moduleOverride string) (string, error) {
	if moduleOverride != "" {
		return moduleOverride, nil
	}

	goModPath := filepath.Join(workDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", newError(1, "module path could not be detected; pass --module or run from a Go module root")
		}
		return "", newError(1, "read go.mod: %w", err)
	}
	file, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "", newError(1, "parse go.mod: %w", err)
	}
	if file.Module == nil || file.Module.Mod.Path == "" {
		return "", newError(1, "module path could not be detected; pass --module or run from a Go module root")
	}
	return file.Module.Mod.Path, nil
}

func ensureParentDirectory(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newError(1, "parent directory %s does not exist", parent)
		}
		return newError(1, "read parent directory %s: %w", parent, err)
	}
	if !info.IsDir() {
		return newError(1, "parent path %s is not a directory", parent)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func enabledChecks(opts Options) []string {
	checks := []string{"gomod"}
	if opts.EnableAPI {
		checks = append(checks, "api")
	}
	if opts.EnableVuln {
		checks = append(checks, "vuln")
	}
	if opts.EnableDownstream || len(opts.Downstream) > 0 {
		checks = append(checks, "downstream")
	}
	return checks
}

func defaultNextSteps() []string {
	return []string{
		"Run go-prism doctor",
		"Run go-prism pr --base origin/main --head HEAD",
	}
}

func failedResult(opts Options, path string, modulePath string, err error) Result {
	return Result{
		SchemaVersion: SchemaVersion,
		Status:        StatusFailed,
		Path:          path,
		Module:        modulePath,
		EnabledChecks: enabledChecks(opts),
		DryRun:        opts.DryRun,
		NextSteps:     defaultNextSteps(),
		Error:         err.Error(),
	}
}

type initError struct {
	code int
	err  error
}

func newError(code int, format string, args ...any) initError {
	return initError{
		code: code,
		err:  fmt.Errorf(format, args...),
	}
}

func (e initError) Error() string {
	return e.err.Error()
}

func (e initError) Unwrap() error {
	return e.err
}

func (e initError) ExitCode() int {
	return e.code
}

// ExitCode returns the intended CLI exit code for an init error.
func ExitCode(err error) int {
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return 1
}
