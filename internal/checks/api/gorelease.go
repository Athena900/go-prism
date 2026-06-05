package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Athena900/go-prism/internal/evidence"
	"golang.org/x/mod/semver"
)

const maxGoreleaseDetails = 18

var suggestedVersionPattern = regexp.MustCompile(`(?m)^Suggested version: (v[^\s]+)`)
var baseVersionPattern = regexp.MustCompile(`(?m)^(?:Base version|Inferred base version): (?:[^@\s]+@)?(v[^\s]+)`)

type redactionPattern struct {
	pattern     *regexp.Regexp
	replacement string
}

var sensitiveValuePatterns = []redactionPattern{
	{
		pattern:     regexp.MustCompile(`(?i)(token|secret|password|passwd|api[_-]?key)=\S+`),
		replacement: "$1=[REDACTED]",
	},
	{
		pattern:     regexp.MustCompile(`(?i)Bearer\s+\S+`),
		replacement: "Bearer [REDACTED]",
	},
}

// GoreleaseAdapter collects API/SemVer evidence from gorelease.
type GoreleaseAdapter struct{}

// Check runs gorelease when available and normalizes its report into evidence.
func (GoreleaseAdapter) Check(ctx context.Context, opts Options, tools ToolRunner) evidence.Item {
	select {
	case <-ctx.Done():
		return timeoutEvidence(opts, ctx.Err())
	default:
	}

	path, err := tools.LookPath("gorelease")
	if err != nil {
		return evidence.Item{
			ID:       "api.gorelease.not_installed",
			Title:    "gorelease is not installed",
			Status:   evidence.StatusUnknown,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryAPI,
			Source:   "gorelease",
			Summary:  "API/SemVer release-impact evidence could not be collected because `gorelease` was not found on PATH.",
			Details: []string{
				fmt.Sprintf("lookup error: %v", err),
			},
			Recommendation: "Install `gorelease` with `go install golang.org/x/exp/cmd/gorelease@latest`, or keep checks.api disabled until API evidence is required.",
			Provenance:     provenance(opts, "detect gorelease", "gorelease"),
		}
	}

	args := goreleaseArgs(opts)
	result := tools.Run(ctx, ToolInvocation{
		Path: path,
		Args: args,
		Dir:  opts.WorkDir,
	})
	if ctx.Err() != nil {
		return timeoutEvidence(opts, ctx.Err())
	}

	return classifyGoreleaseReport(opts, path, args, result)
}

func goreleaseArgs(opts Options) []string {
	base := strings.TrimSpace(opts.Base)
	if !isGoreleaseBase(base) {
		return nil
	}
	return []string{"-base=" + base}
}

func isGoreleaseBase(base string) bool {
	if base == "" {
		return false
	}
	if base == "none" || base == "latest" {
		return true
	}
	if semver.IsValid(base) {
		return true
	}

	at := strings.LastIndex(base, "@")
	if at < 0 || at == len(base)-1 {
		return false
	}
	version := base[at+1:]
	return version == "latest" || semver.IsValid(version)
}

func classifyGoreleaseReport(opts Options, path string, args []string, result ToolResult) evidence.Item {
	output := combinedOutput(result)
	details := goreleaseDetails(output)
	provenance := goreleaseProvenance(opts, path, args, "")

	if hasIncompatibleChanges(output) {
		provenance = goreleaseProvenance(opts, path, args, "major")
		return evidence.Item{
			ID:             "api.gorelease.incompatible_changes",
			Title:          "gorelease detected incompatible API changes",
			Status:         evidence.StatusBlock,
			Severity:       evidence.SeverityHigh,
			Category:       evidence.CategoryAPI,
			Source:         "gorelease",
			Summary:        "gorelease reported backward-incompatible public API changes.",
			Details:        details,
			Recommendation: "Review the API break. A stable v1+ module usually needs a new major version path before this can be released safely.",
			Provenance:     provenance,
		}
	}

	if hasCompatibleChanges(output) {
		provenance = goreleaseProvenance(opts, path, args, "minor")
		return evidence.Item{
			ID:             "api.gorelease.compatible_changes",
			Title:          "gorelease detected compatible API changes",
			Status:         evidence.StatusWarn,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "gorelease",
			Summary:        "gorelease reported backward-compatible public API additions or dependency changes.",
			Details:        details,
			Recommendation: "Review whether this PR should be released as a minor version and mention user-visible additions in release notes.",
			Provenance:     provenance,
		}
	}

	if result.Err != nil {
		return evidence.Item{
			ID:       "api.gorelease.run_failed",
			Title:    "gorelease execution failed",
			Status:   evidence.StatusUnknown,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryAPI,
			Source:   "gorelease",
			Summary: fmt.Sprintf(
				"gorelease exited with code %d before go-prism could classify API/SemVer evidence: %v.",
				result.ExitCode,
				result.Err,
			),
			Details:        details,
			Recommendation: "Run gorelease locally and fix module loading, version, or repository issues before trusting API evidence.",
			Provenance:     provenance,
		}
	}

	if hasSuccessfulSummary(output) {
		provenance = goreleaseProvenance(opts, path, args, releaseImpactFromGoreleaseSummary(output))
		return evidence.Item{
			ID:             "api.gorelease.no_incompatible_changes",
			Title:          "gorelease found no incompatible API changes",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryAPI,
			Source:         "gorelease",
			Summary:        "gorelease completed without reporting incompatible or compatible public API changes.",
			Details:        details,
			Recommendation: "No API/SemVer blocker was reported by gorelease.",
			Provenance:     provenance,
		}
	}

	return evidence.Item{
		ID:             "api.gorelease.unclassified_output",
		Title:          "gorelease output was not classified",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryAPI,
		Source:         "gorelease",
		Summary:        "gorelease completed, but go-prism did not recognize enough report structure to classify the result.",
		Details:        details,
		Recommendation: "Inspect gorelease output and update go-prism's parser if this output shape is expected.",
		Provenance:     provenance,
	}
}

func releaseImpactFromGoreleaseSummary(output string) string {
	baseVersion := firstRegexSubmatch(baseVersionPattern, output)
	suggestedVersion := firstRegexSubmatch(suggestedVersionPattern, output)
	if baseVersion == "" || suggestedVersion == "" {
		return "unknown"
	}
	if semver.Major(baseVersion) != semver.Major(suggestedVersion) {
		return "major"
	}
	if semver.MajorMinor(baseVersion) != semver.MajorMinor(suggestedVersion) {
		return "minor"
	}
	if semver.Canonical(baseVersion) != semver.Canonical(suggestedVersion) {
		return "patch"
	}
	return "unknown"
}

func firstRegexSubmatch(pattern *regexp.Regexp, input string) string {
	match := pattern.FindStringSubmatch(input)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func hasIncompatibleChanges(output string) bool {
	return strings.Contains(output, "## incompatible changes") ||
		strings.Contains(output, "Incompatible changes were detected.") ||
		strings.Contains(output, "There are incompatible changes.")
}

func hasCompatibleChanges(output string) bool {
	return strings.Contains(output, "## compatible changes") ||
		strings.Contains(output, "There are compatible changes,")
}

func hasSuccessfulSummary(output string) bool {
	return suggestedVersionPattern.MatchString(output) ||
		strings.Contains(output, " is a valid semantic version for this release.")
}

func combinedOutput(result ToolResult) string {
	switch {
	case result.Stdout == "":
		return result.Stderr
	case result.Stderr == "":
		return result.Stdout
	default:
		return result.Stdout + "\n" + result.Stderr
	}
}

func goreleaseDetails(output string) []string {
	lines := strings.Split(output, "\n")
	details := make([]string, 0, maxGoreleaseDetails)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !isUsefulGoreleaseLine(line) {
			continue
		}
		details = append(details, redactSensitive(line))
		if len(details) == maxGoreleaseDetails {
			break
		}
	}

	if len(details) == 0 && strings.TrimSpace(output) != "" {
		details = append(details, redactSensitive(firstNonEmptyLine(output)))
	}

	return details
}

func redactSensitive(line string) string {
	redacted := line
	for _, pattern := range sensitiveValuePatterns {
		redacted = pattern.pattern.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}

func isUsefulGoreleaseLine(line string) bool {
	return strings.HasPrefix(line, "# ") ||
		strings.HasPrefix(line, "## ") ||
		strings.HasPrefix(line, "Base version:") ||
		strings.HasPrefix(line, "Inferred base version:") ||
		strings.HasPrefix(line, "Suggested version:") ||
		strings.Contains(line, "is a valid semantic version") ||
		strings.Contains(line, "is not a valid semantic version") ||
		strings.Contains(line, "Cannot suggest a release version.") ||
		strings.Contains(line, "Incompatible changes were detected.") ||
		strings.Contains(line, "There are incompatible changes.") ||
		strings.Contains(line, "There are compatible changes,") ||
		strings.Contains(line, "Errors were found")
}

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func goreleaseProvenance(opts Options, path string, args []string, releaseImpact string) evidence.Provenance {
	command := "gorelease"
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}

	provenance := provenance(opts, command, "gorelease")
	provenance.Extra = map[string]string{
		"path": path,
	}
	if len(args) > 0 {
		provenance.Extra["args"] = strings.Join(args, " ")
	}
	if releaseImpact != "" && releaseImpact != "unknown" {
		provenance.Extra["release_impact"] = releaseImpact
	}
	return provenance
}
