package api

import "github.com/Athena900/go-prism/internal/evidence"

func timeoutEvidence(opts Options, err error) evidence.Item {
	return evidence.Item{
		ID:             "api.timeout",
		Title:          "API check timed out",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryAPI,
		Source:         "go-prism",
		Summary:        err.Error(),
		Recommendation: "Retry with a longer timeout before trusting API/SemVer release-impact evidence.",
		Provenance:     provenance(opts, "check API timeout", ""),
	}
}

func provenance(opts Options, command string, tool string) evidence.Provenance {
	return evidence.Provenance{
		Base:    opts.Base,
		Head:    opts.Head,
		WorkDir: opts.WorkDir,
		Command: command,
		Tool:    tool,
	}
}
