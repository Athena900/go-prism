package api

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Athena900/go-prism/internal/evidence"
	"github.com/Athena900/go-prism/internal/redact"
)

const maxGoAPIDiffDetails = 12

// GoAPIDiffAdapter collects supplemental API compatibility evidence from go-apidiff.
type GoAPIDiffAdapter struct{}

// Check runs go-apidiff when available and normalizes compatible/incompatible API changes.
func (GoAPIDiffAdapter) Check(ctx context.Context, opts Options, tools ToolRunner) evidence.Item {
	select {
	case <-ctx.Done():
		return timeoutEvidence(opts, ctx.Err())
	default:
	}

	path, err := tools.LookPath("go-apidiff")
	if err != nil {
		return evidence.Item{
			ID:             "api.goapidiff.not_installed",
			Title:          "go-apidiff is not installed",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryAPI,
			Source:         "go-apidiff",
			Summary:        "Supplemental API compatibility evidence was skipped because `go-apidiff` was not found on PATH.",
			Details:        []string{fmt.Sprintf("lookup error: %v", err)},
			Recommendation: "Install `go-apidiff` with `go install github.com/joelanford/go-apidiff@latest` when supplemental API compatibility evidence is required.",
			Provenance:     provenance(opts, "detect go-apidiff", "go-apidiff"),
		}
	}

	repoRoot, err := resolveGoAPIDiffRepoRoot(ctx, opts, tools)
	if err != nil {
		return evidence.Item{
			ID:             "api.goapidiff.git_context_failed",
			Title:          "go-apidiff git context could not be resolved",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "go-apidiff",
			Summary:        fmt.Sprintf("go-prism could not resolve a git repository root for go-apidiff: %v.", err),
			Recommendation: "Run go-prism from a git checkout with enough history for base/head API comparison.",
			Provenance:     provenance(opts, "resolve repo root for go-apidiff", "git"),
		}
	}

	args := goAPIDiffArgs(repoRoot, opts)
	result := tools.Run(ctx, ToolInvocation{
		Path: path,
		Args: args,
		Dir:  repoRoot,
	})
	if ctx.Err() != nil {
		return timeoutEvidence(opts, ctx.Err())
	}

	return classifyGoAPIDiffResult(opts, path, repoRoot, args, result)
}

func resolveGoAPIDiffRepoRoot(ctx context.Context, opts Options, tools ToolRunner) (string, error) {
	gitPath, err := tools.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("detect git: %w", err)
	}
	workDir := defaultWorkDir(opts.WorkDir)
	result := tools.Run(ctx, ToolInvocation{
		Path: gitPath,
		Args: []string{"rev-parse", "--show-toplevel"},
		Dir:  workDir,
	})
	if result.Err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %s", commandFailure(result))
	}
	repoRoot := strings.TrimSpace(result.Stdout)
	if repoRoot == "" {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned empty output")
	}
	if filepath.IsAbs(repoRoot) {
		return filepath.Clean(repoRoot), nil
	}
	return filepath.Clean(filepath.Join(workDir, repoRoot)), nil
}

func goAPIDiffArgs(repoRoot string, opts Options) []string {
	return []string{"--repo-path", repoRoot, "--print-compatible", opts.Base, opts.Head}
}

func classifyGoAPIDiffResult(opts Options, path string, repoRoot string, args []string, result ToolResult) evidence.Item {
	output := combinedOutput(result)
	details := goAPIDiffDetails(result)
	provenance := goAPIDiffProvenance(opts, path, repoRoot, args, result.ExitCode, "")

	if hasGoAPIDiffIncompatibleChanges(output) {
		provenance = goAPIDiffProvenance(opts, path, repoRoot, args, result.ExitCode, "major")
		return evidence.Item{
			ID:             "api.goapidiff.incompatible_changes",
			Title:          "go-apidiff found incompatible API changes",
			Status:         evidence.StatusBlock,
			Severity:       evidence.SeverityHigh,
			Category:       evidence.CategoryAPI,
			Source:         "go-apidiff",
			Summary:        "go-apidiff reported public API changes that can break callers.",
			Details:        details,
			Recommendation: "Review the API break. A stable v1+ module usually needs a major version strategy before this can be released safely.",
			Provenance:     provenance,
		}
	}

	if hasGoAPIDiffCompatibleChanges(output) {
		provenance = goAPIDiffProvenance(opts, path, repoRoot, args, result.ExitCode, "minor")
		return evidence.Item{
			ID:             "api.goapidiff.compatible_changes",
			Title:          "go-apidiff found compatible API changes",
			Status:         evidence.StatusWarn,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "go-apidiff",
			Summary:        "go-apidiff reported backward-compatible public API additions.",
			Details:        details,
			Recommendation: "Review whether this PR should be released as a minor version and mention user-visible additions in release notes.",
			Provenance:     provenance,
		}
	}

	if result.Err != nil {
		return evidence.Item{
			ID:       "api.goapidiff.run_failed",
			Title:    "go-apidiff execution failed",
			Status:   evidence.StatusUnknown,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryAPI,
			Source:   "go-apidiff",
			Summary: fmt.Sprintf(
				"go-apidiff exited with code %d before go-prism could classify API compatibility evidence: %v.",
				result.ExitCode,
				result.Err,
			),
			Details:        details,
			Recommendation: "Run go-apidiff locally and fix module loading, git history, or base/head ref issues before trusting go-apidiff evidence.",
			Provenance:     provenance,
		}
	}

	if strings.TrimSpace(output) == "" {
		provenance = goAPIDiffProvenance(opts, path, repoRoot, args, result.ExitCode, "patch")
		return evidence.Item{
			ID:             "api.goapidiff.no_changes",
			Title:          "go-apidiff found no API changes",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryAPI,
			Source:         "go-apidiff",
			Summary:        "go-apidiff did not report compatible or incompatible public API changes.",
			Details:        details,
			Recommendation: "No API compatibility blocker was reported by go-apidiff.",
			Provenance:     provenance,
		}
	}

	return evidence.Item{
		ID:             "api.goapidiff.unclassified_output",
		Title:          "go-apidiff output was not classified",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryAPI,
		Source:         "go-apidiff",
		Summary:        "go-apidiff completed, but go-prism did not recognize enough output structure to classify the result.",
		Details:        details,
		Recommendation: "Inspect go-apidiff output and update go-prism's parser if this output shape is expected.",
		Provenance:     provenance,
	}
}

func hasGoAPIDiffIncompatibleChanges(output string) bool {
	return strings.Contains(output, "Incompatible changes:")
}

func hasGoAPIDiffCompatibleChanges(output string) bool {
	return strings.Contains(output, "Compatible changes:")
}

func goAPIDiffDetails(result ToolResult) []string {
	details := []string{
		"exit code: " + strconv.Itoa(result.ExitCode),
	}

	for _, line := range strings.Split(combinedOutput(result), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		details = append(details, redact.Sensitive(line))
		if len(details) == maxGoAPIDiffDetails {
			break
		}
	}

	return details
}

func goAPIDiffProvenance(opts Options, path string, repoRoot string, args []string, exitCode int, releaseImpact string) evidence.Provenance {
	command := "go-apidiff"
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}

	provenance := provenance(opts, command, "go-apidiff")
	provenance.Extra = map[string]string{
		"path":      path,
		"repo_root": repoRoot,
		"exit_code": strconv.Itoa(exitCode),
	}
	if len(args) > 0 {
		provenance.Extra["args"] = strings.Join(args, " ")
	}
	if releaseImpact != "" {
		provenance.Extra["release_impact"] = releaseImpact
	}
	return provenance
}
