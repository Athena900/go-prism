package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Athena900/go-prism/internal/evidence"
)

// Render renders a report in a supported format.
func Render(r evidence.Report, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "", "markdown", "md":
		return Markdown(r), nil
	case "json":
		return JSON(r)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

// JSON renders report evidence as stable indented JSON.
func JSON(r evidence.Report) ([]byte, error) {
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// Markdown renders report evidence for PR comments and job summaries.
func Markdown(r evidence.Report) []byte {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "## Go Prism")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "Decision: %s\n", strings.ToUpper(string(r.Decision)))
	fmt.Fprintf(&buf, "Suggested release impact: %s\n", r.SuggestedReleaseImpact)
	if r.Module != "" {
		fmt.Fprintf(&buf, "Module: `%s`\n", r.Module)
	}
	if r.Base != "" || r.Head != "" {
		fmt.Fprintf(&buf, "Refs: `%s` -> `%s`\n", r.Base, r.Head)
	}
	fmt.Fprintln(&buf)

	renderMaintainerSummary(&buf, r.MaintainerSummary)

	renderSection(&buf, "Blocking", filterItems(r.Items, evidence.StatusBlock), "None.")
	renderSection(&buf, "Needs Maintainer Review", filterItems(r.Items, evidence.StatusWarn), "None.")
	renderSection(&buf, "Unknown", filterItems(r.Items, evidence.StatusUnknown), "None.")
	renderSection(&buf, "Informational", filterItems(r.Items, evidence.StatusInfo), "None.")
	renderSection(&buf, "Passing", filterItems(r.Items, evidence.StatusPass), "None.")

	fmt.Fprintln(&buf, "Generated from deterministic evidence. Maintainer summary is rule-based and advisory.")

	return buf.Bytes()
}

func renderMaintainerSummary(buf *bytes.Buffer, summary *evidence.MaintainerSummary) {
	if summary == nil {
		return
	}

	fmt.Fprintln(buf, "### Maintainer Summary")
	fmt.Fprintln(buf)
	fmt.Fprintln(buf, summary.Headline)
	fmt.Fprintln(buf)
	if len(summary.KeyFindings) > 0 {
		fmt.Fprintln(buf, "Key findings:")
		renderSummaryFindings(buf, summary.KeyFindings)
		fmt.Fprintln(buf)
	}
	if len(summary.NextActions) > 0 {
		fmt.Fprintln(buf, "Next actions:")
		renderSummaryFindings(buf, summary.NextActions)
		fmt.Fprintln(buf)
	}
}

func renderSummaryFindings(buf *bytes.Buffer, findings []evidence.SummaryFinding) {
	for _, finding := range findings {
		fmt.Fprintf(buf, "- %s\n", finding.Text)
		if len(finding.EvidenceIDs) > 0 {
			fmt.Fprintf(buf, "  Evidence: %s\n", formatEvidenceIDs(finding.EvidenceIDs))
		}
	}
}

func formatEvidenceIDs(ids []string) string {
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf("`%s`", id))
	}
	return strings.Join(quoted, ", ")
}

func filterItems(items []evidence.Item, status evidence.Status) []evidence.Item {
	filtered := make([]evidence.Item, 0)
	for _, item := range items {
		if item.Status == status {
			filtered = append(filtered, item)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})
	return filtered
}

func renderSection(buf *bytes.Buffer, title string, items []evidence.Item, empty string) {
	fmt.Fprintf(buf, "### %s\n\n", title)
	if len(items) == 0 {
		fmt.Fprintf(buf, "%s\n\n", empty)
		return
	}

	for _, item := range items {
		fmt.Fprintf(buf, "- **%s** `%s` %s\n", item.Title, item.ID, item.Summary)
		if item.Recommendation != "" {
			fmt.Fprintf(buf, "  Recommendation: %s\n", item.Recommendation)
		}
		for _, detail := range item.Details {
			fmt.Fprintf(buf, "  - %s\n", detail)
		}
	}
	fmt.Fprintln(buf)
}
