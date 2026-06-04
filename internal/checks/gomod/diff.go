package gomod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Athena900/go-prism/internal/evidence"
)

const maxDiffDetails = 20

// CheckDiff compares base/head go.mod content and emits deterministic change evidence.
func CheckDiff(ctx context.Context, opts Options) []evidence.Item {
	select {
	case <-ctx.Done():
		return []evidence.Item{diffTimeoutEvidence(opts, ctx.Err())}
	default:
	}

	baseRef := strings.TrimSpace(opts.Base)
	headRef := strings.TrimSpace(opts.Head)
	if baseRef == "" || headRef == "" {
		return []evidence.Item{{
			ID:             "gomod.diff.skipped",
			Title:          "go.mod diff skipped",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod diff",
			Summary:        "go.mod diff was skipped because base or head ref is empty.",
			Recommendation: "Pass both --base and --head when PR evidence should include module file changes.",
			Provenance:     provenance(opts, "skip go.mod diff"),
		}}
	}

	baseData, baseSource, err := readGoModAtRef(ctx, opts, baseRef)
	if err != nil {
		if ctx.Err() != nil {
			return []evidence.Item{diffTimeoutEvidence(opts, ctx.Err())}
		}
		return []evidence.Item{diffReadFailedEvidence(opts, "base", baseRef, err)}
	}

	headData, headSource, err := readHeadGoMod(ctx, opts, headRef)
	if err != nil {
		if ctx.Err() != nil {
			return []evidence.Item{diffTimeoutEvidence(opts, ctx.Err())}
		}
		return []evidence.Item{diffReadFailedEvidence(opts, "head", headRef, err)}
	}

	baseSnapshot, err := parseSnapshot(baseSource, baseData)
	if err != nil {
		return []evidence.Item{diffParseFailedEvidence(opts, "base", baseSource, err)}
	}
	headSnapshot, err := parseSnapshot(headSource, headData)
	if err != nil {
		return []evidence.Item{diffParseFailedEvidence(opts, "head", headSource, err)}
	}

	return diffSnapshots(opts, baseSnapshot, headSnapshot)
}

func readHeadGoMod(ctx context.Context, opts Options, headRef string) ([]byte, string, error) {
	if headRef != "HEAD" {
		return readGoModAtRef(ctx, opts, headRef)
	}

	path := filepath.Join(defaultWorkDir(opts.WorkDir), "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	return data, "worktree:go.mod", nil
}

func diffSnapshots(opts Options, base moduleSnapshot, head moduleSnapshot) []evidence.Item {
	items := make([]evidence.Item, 0)

	if base.modulePath != head.modulePath {
		items = append(items, evidence.Item{
			ID:       "gomod.diff.module_changed",
			Title:    "Module path changed",
			Status:   evidence.StatusBlock,
			Severity: evidence.SeverityHigh,
			Category: evidence.CategoryGoMod,
			Source:   "go.mod diff",
			Summary: fmt.Sprintf(
				"Module path changed from `%s` to `%s`.",
				displayValue(base.modulePath),
				displayValue(head.modulePath),
			),
			Recommendation: "Confirm this is an intentional module identity change before release.",
			Provenance:     diffProvenance(opts, "compare module directive", base.source, head.source),
		})
	}

	if base.goVersion != head.goVersion {
		items = append(items, evidence.Item{
			ID:       "gomod.diff.go_directive_changed",
			Title:    "Go directive changed",
			Status:   evidence.StatusWarn,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryGoMod,
			Source:   "go.mod diff",
			Summary: fmt.Sprintf(
				"Go directive changed from `%s` to `%s`.",
				displayValue(base.goVersion),
				displayValue(head.goVersion),
			),
			Recommendation: "Review the supported Go version floor and update release notes if this affects users.",
			Provenance:     diffProvenance(opts, "compare go directive", base.source, head.source),
		})
	}

	if base.toolchain != head.toolchain {
		items = append(items, evidence.Item{
			ID:       "gomod.diff.toolchain_changed",
			Title:    "Toolchain directive changed",
			Status:   evidence.StatusWarn,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryGoMod,
			Source:   "go.mod diff",
			Summary: fmt.Sprintf(
				"Toolchain directive changed from `%s` to `%s`.",
				displayValue(base.toolchain),
				displayValue(head.toolchain),
			),
			Recommendation: "Confirm the selected toolchain is intentional for contributors and CI.",
			Provenance:     diffProvenance(opts, "compare toolchain directive", base.source, head.source),
		})
	}

	items = append(items, replacementDiffEvidence(opts, base, head)...)
	items = append(items, retractionDiffEvidence(opts, base, head)...)
	items = append(items, requirementDiffEvidence(opts, base, head)...)

	if len(items) == 0 {
		return []evidence.Item{{
			ID:             "gomod.diff.no_changes",
			Title:          "No go.mod diff",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod diff",
			Summary:        "No meaningful go.mod changes were detected between base and head.",
			Recommendation: "No go.mod diff review needed.",
			Provenance:     diffProvenance(opts, "compare go.mod snapshots", base.source, head.source),
		}}
	}

	return items
}

func replacementDiffEvidence(opts Options, base moduleSnapshot, head moduleSnapshot) []evidence.Item {
	details, risky := replacementChanges(base.replacements, head.replacements)
	if len(details) == 0 {
		return nil
	}

	status := evidence.StatusInfo
	severity := evidence.SeverityLow
	recommendation := "Confirm release notes or PR context explain why replace directives changed."
	if risky {
		status = evidence.StatusWarn
		severity = evidence.SeverityMedium
		recommendation = "Review added or changed replace directives before publishing a public module release."
	}

	return []evidence.Item{{
		ID:             "gomod.diff.replace_changed",
		Title:          "Replace directives changed",
		Status:         status,
		Severity:       severity,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod diff",
		Summary:        fmt.Sprintf("%d replace directive change(s) detected.", len(details)),
		Details:        limitDetails(details),
		Recommendation: recommendation,
		Provenance:     diffProvenance(opts, "compare replace directives", base.source, head.source),
	}}
}

func retractionDiffEvidence(opts Options, base moduleSnapshot, head moduleSnapshot) []evidence.Item {
	details := setChanges(base.retractions, head.retractions)
	if len(details) == 0 {
		return nil
	}

	return []evidence.Item{{
		ID:             "gomod.diff.retract_changed",
		Title:          "Retract directives changed",
		Status:         evidence.StatusInfo,
		Severity:       evidence.SeverityLow,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod diff",
		Summary:        fmt.Sprintf("%d retract directive change(s) detected.", len(details)),
		Details:        limitDetails(details),
		Recommendation: "Confirm release notes explain added or removed version retractions.",
		Provenance:     diffProvenance(opts, "compare retract directives", base.source, head.source),
	}}
}

func requirementDiffEvidence(opts Options, base moduleSnapshot, head moduleSnapshot) []evidence.Item {
	directDetails, indirectDetails := requirementChanges(base.requirements, head.requirements)
	items := make([]evidence.Item, 0, 2)

	if len(directDetails) > 0 {
		items = append(items, evidence.Item{
			ID:             "gomod.diff.direct_requirements_changed",
			Title:          "Direct requirements changed",
			Status:         evidence.StatusWarn,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod diff",
			Summary:        fmt.Sprintf("%d direct requirement change(s) detected.", len(directDetails)),
			Details:        limitDetails(directDetails),
			Recommendation: "Review direct dependency changelogs and compatibility notes before release.",
			Provenance:     diffProvenance(opts, "compare direct requirements", base.source, head.source),
		})
	}

	if len(indirectDetails) > 0 {
		items = append(items, evidence.Item{
			ID:             "gomod.diff.indirect_requirements_changed",
			Title:          "Indirect requirements changed",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityLow,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod diff",
			Summary:        fmt.Sprintf("%d indirect requirement change(s) detected.", len(indirectDetails)),
			Details:        limitDetails(indirectDetails),
			Recommendation: "Confirm indirect dependency updates are expected from go mod tidy or dependency updates.",
			Provenance:     diffProvenance(opts, "compare indirect requirements", base.source, head.source),
		})
	}

	return items
}

func replacementChanges(base map[string]replacementSnapshot, head map[string]replacementSnapshot) ([]string, bool) {
	details := make([]string, 0)
	risky := false

	for _, key := range unionKeys(base, head) {
		baseValue, baseOK := base[key]
		headValue, headOK := head[key]

		switch {
		case !baseOK:
			details = append(details, "added "+headValue.format())
			risky = true
		case !headOK:
			details = append(details, "removed "+baseValue.format())
		case baseValue.value() != headValue.value():
			details = append(details, fmt.Sprintf("changed %s: %s -> %s", key, baseValue.value(), headValue.value()))
			risky = true
		}
	}

	return details, risky
}

func setChanges(base map[string]string, head map[string]string) []string {
	details := make([]string, 0)

	for _, key := range unionKeys(base, head) {
		_, baseOK := base[key]
		_, headOK := head[key]

		switch {
		case !baseOK:
			details = append(details, "added "+key)
		case !headOK:
			details = append(details, "removed "+key)
		}
	}

	return details
}

func requirementChanges(base map[string]requirementSnapshot, head map[string]requirementSnapshot) ([]string, []string) {
	directDetails := make([]string, 0)
	indirectDetails := make([]string, 0)

	for _, path := range unionKeys(base, head) {
		baseReq, baseOK := base[path]
		headReq, headOK := head[path]

		switch {
		case !baseOK:
			detail := "added " + headReq.format()
			if headReq.indirect {
				indirectDetails = append(indirectDetails, detail)
			} else {
				directDetails = append(directDetails, detail)
			}
		case !headOK:
			detail := "removed " + baseReq.format()
			if baseReq.indirect {
				indirectDetails = append(indirectDetails, detail)
			} else {
				directDetails = append(directDetails, detail)
			}
		case baseReq.version != headReq.version || baseReq.indirect != headReq.indirect:
			detail := fmt.Sprintf("changed %s: %s -> %s", path, baseReq.format(), headReq.format())
			if baseReq.indirect && headReq.indirect {
				indirectDetails = append(indirectDetails, detail)
			} else {
				directDetails = append(directDetails, detail)
			}
		}
	}

	return directDetails, indirectDetails
}

func unionKeys[V any](base map[string]V, head map[string]V) []string {
	keys := make(map[string]struct{}, len(base)+len(head))
	for key := range base {
		keys[key] = struct{}{}
	}
	for key := range head {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

func limitDetails(details []string) []string {
	if len(details) <= maxDiffDetails {
		return details
	}

	limited := append([]string{}, details[:maxDiffDetails]...)
	limited = append(limited, fmt.Sprintf("... %d more change(s) omitted", len(details)-maxDiffDetails))
	return limited
}

func displayValue(value string) string {
	if value == "" {
		return "(missing)"
	}
	return value
}

func defaultWorkDir(workDir string) string {
	if workDir == "" {
		return "."
	}
	return workDir
}

func diffReadFailedEvidence(opts Options, side string, ref string, err error) evidence.Item {
	return evidence.Item{
		ID:             "gomod.diff.read_failed",
		Title:          "Unable to read go.mod diff input",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod diff",
		Summary:        fmt.Sprintf("Unable to read %s go.mod for ref `%s`: %v.", side, ref, err),
		Recommendation: "Fetch the compared refs and run go-prism from a Git checkout that contains the target module.",
		Provenance:     provenance(opts, "read go.mod diff input"),
	}
}

func diffParseFailedEvidence(opts Options, side string, source string, err error) evidence.Item {
	baseSource := ""
	headSource := ""
	if side == "base" {
		baseSource = source
	} else {
		headSource = source
	}

	return evidence.Item{
		ID:             "gomod.diff.parse_failed",
		Title:          "Unable to parse go.mod diff input",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryGoMod,
		Source:         "golang.org/x/mod/modfile",
		Summary:        fmt.Sprintf("Unable to parse %s go.mod from `%s`: %v.", side, source, err),
		Recommendation: "Fix go.mod syntax or compare against a valid module ref before trusting diff evidence.",
		Provenance:     diffProvenance(opts, "parse go.mod diff input", baseSource, headSource),
	}
}

func diffTimeoutEvidence(opts Options, err error) evidence.Item {
	return evidence.Item{
		ID:             "gomod.diff.timeout",
		Title:          "go.mod diff check timed out",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryGoMod,
		Source:         "go-prism",
		Summary:        err.Error(),
		Recommendation: "Retry with a longer timeout before trusting this report.",
		Provenance:     provenance(opts, "check go.mod diff timeout"),
	}
}

func diffProvenance(opts Options, command string, baseSource string, headSource string) evidence.Provenance {
	provenance := provenance(opts, command)
	extra := make(map[string]string)
	if baseSource != "" {
		extra["base_source"] = baseSource
	}
	if headSource != "" {
		extra["head_source"] = headSource
	}
	if len(extra) > 0 {
		provenance.Extra = extra
	}
	return provenance
}
