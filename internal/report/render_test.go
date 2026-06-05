package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestMarkdown(t *testing.T) {
	r := evidence.NewReport(evidence.ReportOptions{
		Tool:      "go-prism",
		Version:   "test",
		Module:    "github.com/example/project",
		Base:      "origin/main",
		Head:      "HEAD",
		Generated: time.Unix(0, 0).UTC(),
		Items: []evidence.Item{
			{
				ID:             "gomod.replace_present",
				Title:          "replace directives present",
				Status:         evidence.StatusWarn,
				Severity:       evidence.SeverityMedium,
				Category:       evidence.CategoryGoMod,
				Source:         "go.mod",
				Summary:        "go.mod contains replace directives.",
				Recommendation: "Review them.",
			},
		},
	})

	out := string(Markdown(r))
	for _, want := range []string{
		"## Go Prism",
		"Decision: WARN",
		"Module: `github.com/example/project`",
		"### Maintainer Summary",
		"Maintainer review is needed before release.",
		"Evidence: `gomod.replace_present`",
		"### Needs Maintainer Review",
		"replace directives present",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown output missing %q:\n%s", want, out)
		}
	}
}

func TestJSONIncludesSchemaVersion(t *testing.T) {
	r := evidence.NewReport(evidence.ReportOptions{
		Tool:      "go-prism",
		Version:   "test",
		Generated: time.Unix(0, 0).UTC(),
	})

	out, err := JSON(r)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var decoded struct {
		SchemaVersion     string                      `json:"schema_version"`
		Tool              string                      `json:"tool"`
		Version           string                      `json:"version"`
		MaintainerSummary *evidence.MaintainerSummary `json:"maintainer_summary,omitempty"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if decoded.SchemaVersion != evidence.ReportSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", decoded.SchemaVersion, evidence.ReportSchemaVersion)
	}
	if decoded.Tool != "go-prism" {
		t.Fatalf("tool = %q, want go-prism", decoded.Tool)
	}
	if decoded.Version != "test" {
		t.Fatalf("version = %q, want test", decoded.Version)
	}
	if decoded.MaintainerSummary != nil {
		t.Fatalf("maintainer_summary = %#v, want nil for empty report", decoded.MaintainerSummary)
	}
}

func TestJSONIncludesMaintainerSummaryWhenEvidenceExists(t *testing.T) {
	r := evidence.NewReport(evidence.ReportOptions{
		Tool:      "go-prism",
		Version:   "test",
		Generated: time.Unix(0, 0).UTC(),
		Items: []evidence.Item{{
			ID:       "api.breaking",
			Title:    "API incompatibility found",
			Status:   evidence.StatusBlock,
			Severity: evidence.SeverityHigh,
			Category: evidence.CategoryAPI,
			Summary:  "exported API changed incompatibly.",
		}},
	})

	out, err := JSON(r)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var decoded struct {
		MaintainerSummary *evidence.MaintainerSummary `json:"maintainer_summary,omitempty"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if decoded.MaintainerSummary == nil {
		t.Fatal("maintainer_summary = nil, want summary")
	}
	if got := decoded.MaintainerSummary.KeyFindings[0].EvidenceIDs[0]; got != "api.breaking" {
		t.Fatalf("summary evidence = %q, want api.breaking", got)
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	_, err := Render(evidence.Report{}, "xml")
	if err == nil {
		t.Fatal("Render() error = nil, want error")
	}
}
