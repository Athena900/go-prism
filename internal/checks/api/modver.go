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

const maxModverDetails = 8

// ModverAdapter collects supplemental API/SemVer evidence from modver.
type ModverAdapter struct{}

// Check runs modver when available and normalizes its required bump result.
func (ModverAdapter) Check(ctx context.Context, opts Options, tools ToolRunner) evidence.Item {
	select {
	case <-ctx.Done():
		return timeoutEvidence(opts, ctx.Err())
	default:
	}

	path, err := tools.LookPath("modver")
	if err != nil {
		return evidence.Item{
			ID:             "api.modver.not_installed",
			Title:          "modver is not installed",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        "Supplemental API/SemVer evidence was skipped because `modver` was not found on PATH.",
			Details:        []string{fmt.Sprintf("lookup error: %v", err)},
			Recommendation: "Install `modver` with `go install github.com/bobg/modver/v2/cmd/modver@latest` when supplemental API/SemVer evidence is required.",
			Provenance:     provenance(opts, "detect modver", "modver"),
		}
	}

	gitDir, err := resolveModverGitDir(ctx, opts, tools)
	if err != nil {
		return evidence.Item{
			ID:             "api.modver.git_context_failed",
			Title:          "modver git context could not be resolved",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        fmt.Sprintf("go-prism could not resolve a git directory for modver: %v.", err),
			Recommendation: "Run go-prism from a git checkout with enough history for base/head API comparison.",
			Provenance:     provenance(opts, "resolve git dir for modver", "git"),
		}
	}

	args := modverArgs(gitDir, opts)
	result := tools.Run(ctx, ToolInvocation{
		Path: path,
		Args: args,
		Dir:  defaultWorkDir(opts.WorkDir),
	})
	if ctx.Err() != nil {
		return timeoutEvidence(opts, ctx.Err())
	}

	return classifyModverResult(opts, path, gitDir, args, result)
}

func resolveModverGitDir(ctx context.Context, opts Options, tools ToolRunner) (string, error) {
	gitPath, err := tools.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("detect git: %w", err)
	}
	workDir := defaultWorkDir(opts.WorkDir)
	result := tools.Run(ctx, ToolInvocation{
		Path: gitPath,
		Args: []string{"rev-parse", "--git-dir"},
		Dir:  workDir,
	})
	if result.Err != nil {
		return "", fmt.Errorf("git rev-parse --git-dir: %s", commandFailure(result))
	}
	gitDir := strings.TrimSpace(result.Stdout)
	if gitDir == "" {
		return "", fmt.Errorf("git rev-parse --git-dir returned empty output")
	}
	if filepath.IsAbs(gitDir) {
		return filepath.Clean(gitDir), nil
	}
	return filepath.Clean(filepath.Join(workDir, gitDir)), nil
}

func modverArgs(gitDir string, opts Options) []string {
	return []string{"-q", "-git", gitDir, opts.Base, opts.Head}
}

func classifyModverResult(opts Options, path string, gitDir string, args []string, result ToolResult) evidence.Item {
	provenance := modverProvenance(opts, path, gitDir, args, result.ExitCode, "")
	details := modverDetails(result, modverBumpName(result.ExitCode))

	if result.Err != nil && result.ExitCode == 0 {
		return evidence.Item{
			ID:             "api.modver.run_failed",
			Title:          "modver execution failed",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        fmt.Sprintf("modver failed before returning a classified exit code: %v.", result.Err),
			Details:        details,
			Recommendation: "Run modver locally and fix module loading, git history, or base/head ref issues before trusting modver evidence.",
			Provenance:     provenance,
		}
	}

	switch result.ExitCode {
	case 0:
		return evidence.Item{
			ID:             "api.modver.none_required",
			Title:          "modver found no required version bump",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        "modver did not report public API changes requiring a version bump.",
			Details:        details,
			Recommendation: "No API/SemVer blocker was reported by modver.",
			Provenance:     provenance,
		}
	case 1:
		provenance = modverProvenance(opts, path, gitDir, args, result.ExitCode, "patch")
		return evidence.Item{
			ID:             "api.modver.patch_required",
			Title:          "modver requires at most a patch release",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        "modver did not report API changes requiring a minor or major version bump.",
			Details:        details,
			Recommendation: "No API/SemVer blocker was reported by modver.",
			Provenance:     provenance,
		}
	case 2:
		provenance = modverProvenance(opts, path, gitDir, args, result.ExitCode, "minor")
		return evidence.Item{
			ID:             "api.modver.minor_required",
			Title:          "modver requires a minor version bump",
			Status:         evidence.StatusWarn,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        "modver reported backward-compatible public API additions.",
			Details:        details,
			Recommendation: "Review whether this PR should be released as a minor version and mention user-visible additions in release notes.",
			Provenance:     provenance,
		}
	case 3:
		provenance = modverProvenance(opts, path, gitDir, args, result.ExitCode, "major")
		return evidence.Item{
			ID:             "api.modver.major_required",
			Title:          "modver requires a major version bump",
			Status:         evidence.StatusBlock,
			Severity:       evidence.SeverityHigh,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        "modver reported backward-incompatible public API changes.",
			Details:        details,
			Recommendation: "Review the API break. A stable v1+ module usually needs a new major version path before this can be released safely.",
			Provenance:     provenance,
		}
	case 4:
		return evidence.Item{
			ID:       "api.modver.run_failed",
			Title:    "modver execution failed",
			Status:   evidence.StatusUnknown,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryAPI,
			Source:   "modver",
			Summary: fmt.Sprintf(
				"modver exited with code %d before go-prism could classify API/SemVer evidence: %v.",
				result.ExitCode,
				result.Err,
			),
			Details:        details,
			Recommendation: "Run modver locally and fix module loading, git history, or base/head ref issues before trusting modver evidence.",
			Provenance:     provenance,
		}
	default:
		return evidence.Item{
			ID:             "api.modver.unclassified_output",
			Title:          "modver output was not classified",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "modver",
			Summary:        fmt.Sprintf("modver exited with unexpected code %d.", result.ExitCode),
			Details:        details,
			Recommendation: "Inspect modver output and update go-prism's parser if this output shape is expected.",
			Provenance:     provenance,
		}
	}
}

func modverBumpName(exitCode int) string {
	switch exitCode {
	case 0:
		return "None"
	case 1:
		return "Patchlevel"
	case 2:
		return "Minor"
	case 3:
		return "Major"
	default:
		return "Unknown"
	}
}

func modverDetails(result ToolResult, bump string) []string {
	details := []string{
		"modver required bump: " + bump,
		"exit code: " + strconv.Itoa(result.ExitCode),
	}

	for _, line := range strings.Split(combinedOutput(result), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		details = append(details, redact.Sensitive(line))
		if len(details) == maxModverDetails {
			break
		}
	}

	return details
}

func modverProvenance(opts Options, path string, gitDir string, args []string, exitCode int, releaseImpact string) evidence.Provenance {
	command := "modver"
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}

	provenance := provenance(opts, command, "modver")
	provenance.Extra = map[string]string{
		"path":      path,
		"git_dir":   gitDir,
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

func commandFailure(result ToolResult) string {
	output := strings.TrimSpace(combinedOutput(result))
	if output != "" {
		return redact.Sensitive(output)
	}
	if result.Err != nil {
		return result.Err.Error()
	}
	return fmt.Sprintf("exit code %d", result.ExitCode)
}
