package report

import (
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
		"### Needs Maintainer Review",
		"replace directives present",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	_, err := Render(evidence.Report{}, "xml")
	if err == nil {
		t.Fatal("Render() error = nil, want error")
	}
}
