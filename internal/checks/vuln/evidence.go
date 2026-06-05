package vuln

import "github.com/Athena900/go-prism/internal/evidence"

func timeoutEvidence(opts Options, err error) evidence.Item {
	return evidence.Item{
		ID:             "vuln.timeout",
		Title:          "Vulnerability check timed out",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryVuln,
		Source:         "go-prism",
		Summary:        err.Error(),
		Recommendation: "Retry with a longer timeout before trusting vulnerability evidence.",
		Provenance:     provenance(opts, "check vulnerability timeout", ""),
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
