package api

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/Athena900/go-prism/internal/evidence"
)

// GoreleaseAdapter is the first API/SemVer adapter boundary.
//
// The current MVP only detects tool availability. Executing gorelease and
// normalizing its output will land in the next implementation step.
type GoreleaseAdapter struct{}

// Check reports whether gorelease evidence can be collected.
func (GoreleaseAdapter) Check(ctx context.Context, opts Options, tools ToolResolver) evidence.Item {
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

	return evidence.Item{
		ID:       "api.gorelease.execution_pending",
		Title:    "gorelease adapter execution pending",
		Status:   evidence.StatusUnknown,
		Severity: evidence.SeverityMedium,
		Category: evidence.CategoryAPI,
		Source:   "gorelease",
		Summary:  "`gorelease` is available, but go-prism does not execute or normalize its output yet.",
		Details: []string{
			"path: " + path,
		},
		Recommendation: "Implement the gorelease runner before treating API/SemVer evidence as complete.",
		Provenance:     provenance(opts, "detect gorelease", "gorelease"),
	}
}

type commandResolver struct{}

func (commandResolver) LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		if errors.Is(err, exec.ErrDot) {
			return "", err
		}
		return "", err
	}
	return path, nil
}
