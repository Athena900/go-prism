package vuln

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

func deltaEvidence(
	ctx context.Context,
	opts Options,
	path string,
	args []string,
	headTarget govulncheckTarget,
	headReport govulncheckReport,
	headResult command.Result,
	headParseErr error,
	tools command.Runner,
) []evidence.Item {
	if strings.TrimSpace(opts.Base) == "" || strings.TrimSpace(opts.Head) == "" {
		return []evidence.Item{deltaSkippedEvidence(opts)}
	}
	if !scanUsable(headResult, headParseErr) {
		return []evidence.Item{deltaFailedEvidence(opts, "head", headTarget, headResult, headParseErr)}
	}

	baseTarget, cleanup, err := prepareGovulncheckTarget(ctx, opts, opts.Base, "base", tools)
	if cleanup != nil {
		defer cleanup(context.Background())
	}
	if err != nil {
		return []evidence.Item{targetFailedEvidence(opts, "base", opts.Base, err)}
	}

	baseResult := tools.Run(ctx, command.Invocation{
		Path: path,
		Args: args,
		Dir:  baseTarget.WorkDir,
	})
	if ctx.Err() != nil {
		return []evidence.Item{timeoutEvidence(opts, ctx.Err())}
	}

	baseReport, baseParseErr := parseGovulncheckJSON(strings.NewReader(baseResult.Stdout))
	if !scanUsable(baseResult, baseParseErr) {
		return []evidence.Item{deltaFailedEvidence(opts, "base", baseTarget, baseResult, baseParseErr)}
	}

	return compareGovulncheckDelta(opts, path, args, baseTarget, headTarget, baseReport, headReport)
}

func compareGovulncheckDelta(
	opts Options,
	path string,
	args []string,
	baseTarget govulncheckTarget,
	headTarget govulncheckTarget,
	baseReport govulncheckReport,
	headReport govulncheckReport,
) []evidence.Item {
	newKeys := findingSetDiff(headReport.Findings, baseReport.Findings)
	fixedKeys := findingSetDiff(baseReport.Findings, headReport.Findings)

	if len(newKeys) == 0 && len(fixedKeys) == 0 {
		return []evidence.Item{{
			ID:             "vuln.govulncheck.delta.no_changes",
			Title:          "No govulncheck vulnerability delta",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryVuln,
			Source:         "govulncheck",
			Summary:        "No vulnerability finding changes were detected between base and head.",
			Recommendation: "No vulnerability delta review needed.",
			Provenance:     deltaProvenance(opts, path, args, baseTarget, headTarget, baseReport, headReport),
		}}
	}

	items := make([]evidence.Item, 0, 2)
	if len(newKeys) > 0 {
		items = append(items, newFindingDeltaEvidence(opts, path, args, baseTarget, headTarget, baseReport, headReport, newKeys))
	}
	if len(fixedKeys) > 0 {
		items = append(items, evidence.Item{
			ID:             "vuln.govulncheck.delta.fixed_findings",
			Title:          "govulncheck vulnerability findings fixed",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityLow,
			Category:       evidence.CategoryVuln,
			Source:         "govulncheck",
			Summary:        fmt.Sprintf("%d vulnerability finding group(s) are present in base but absent from head.", len(fixedKeys)),
			Details:        findingDetailsForKeys(baseReport, fixedKeys, "fixed "),
			Recommendation: "Confirm the dependency or Go toolchain update is intentional and mention the fixed vulnerabilities when useful.",
			Provenance:     deltaProvenance(opts, path, args, baseTarget, headTarget, baseReport, headReport),
		})
	}

	return items
}

func newFindingDeltaEvidence(
	opts Options,
	path string,
	args []string,
	baseTarget govulncheckTarget,
	headTarget govulncheckTarget,
	baseReport govulncheckReport,
	headReport govulncheckReport,
	newKeys []string,
) evidence.Item {
	status := evidence.StatusWarn
	severity := evidence.SeverityMedium
	id := "vuln.govulncheck.delta.new_findings"
	title := "New govulncheck vulnerability findings"
	recommendation := "Review whether the new vulnerable modules or packages are imported, reachable, or need dependency updates before merge."
	if hasSymbolFinding(headReport, newKeys) {
		status = evidence.StatusBlock
		severity = evidence.SeverityHigh
		id = "vuln.govulncheck.delta.new_reachable_findings"
		title = "New reachable govulncheck vulnerability findings"
		recommendation = "Update affected dependencies or the Go toolchain before merge, or document why the reachable finding is not exploitable."
	}

	return evidence.Item{
		ID:             id,
		Title:          title,
		Status:         status,
		Severity:       severity,
		Category:       evidence.CategoryVuln,
		Source:         "govulncheck",
		Summary:        fmt.Sprintf("%d vulnerability finding group(s) are present in head but absent from base.", len(newKeys)),
		Details:        findingDetailsForKeys(headReport, newKeys, "new "),
		Recommendation: recommendation,
		Provenance:     deltaProvenance(opts, path, args, baseTarget, headTarget, baseReport, headReport),
	}
}

func deltaSkippedEvidence(opts Options) evidence.Item {
	return evidence.Item{
		ID:             "vuln.govulncheck.delta.skipped",
		Title:          "govulncheck vulnerability delta skipped",
		Status:         evidence.StatusInfo,
		Severity:       evidence.SeverityNone,
		Category:       evidence.CategoryVuln,
		Source:         "govulncheck",
		Summary:        "Vulnerability delta was skipped because base or head ref is empty.",
		Recommendation: "Pass both --base and --head when PR evidence should include vulnerability changes.",
		Provenance:     provenance(opts, "skip govulncheck delta", "govulncheck"),
	}
}

func deltaFailedEvidence(opts Options, label string, target govulncheckTarget, result command.Result, parseErr error) evidence.Item {
	reason := "govulncheck execution failed"
	if parseErr != nil {
		reason = "govulncheck JSON parse failed: " + parseErr.Error()
	}
	return evidence.Item{
		ID:             "vuln.govulncheck.delta.failed",
		Title:          "govulncheck vulnerability delta failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryVuln,
		Source:         "govulncheck",
		Summary:        fmt.Sprintf("Vulnerability delta could not be computed because the %s scan failed: %s.", label, reason),
		Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxGovulncheckDetails),
		Recommendation: "Run govulncheck for both base and head locally before trusting vulnerability delta evidence.",
		Provenance: evidence.Provenance{
			Base:    opts.Base,
			Head:    opts.Head,
			WorkDir: target.WorkDir,
			Command: "govulncheck -format=json ./...",
			Tool:    "govulncheck",
			Extra: map[string]string{
				"failed_target": label,
				"target_ref":    target.Ref,
				"target_source": target.Source,
			},
		},
	}
}

func scanUsable(result command.Result, parseErr error) bool {
	return parseErr == nil && result.Err == nil
}

func findingSetDiff(left map[string]findingGroup, right map[string]findingGroup) []string {
	keys := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func hasSymbolFinding(report govulncheckReport, keys []string) bool {
	for _, key := range keys {
		if report.Findings[key].Level == "symbol" {
			return true
		}
	}
	return false
}

func deltaProvenance(
	opts Options,
	path string,
	args []string,
	baseTarget govulncheckTarget,
	headTarget govulncheckTarget,
	baseReport govulncheckReport,
	headReport govulncheckReport,
) evidence.Provenance {
	return evidence.Provenance{
		Base:    opts.Base,
		Head:    opts.Head,
		WorkDir: opts.WorkDir,
		Command: "compare govulncheck -format=json ./...",
		Tool:    "govulncheck",
		Extra: map[string]string{
			"args":                strings.Join(args, " "),
			"path":                path,
			"base_target":         baseTarget.WorkDir,
			"base_source":         baseTarget.Source,
			"head_target":         headTarget.WorkDir,
			"head_source":         headTarget.Source,
			"base_finding_groups": fmt.Sprint(len(baseReport.Findings)),
			"head_finding_groups": fmt.Sprint(len(headReport.Findings)),
		},
	}
}
