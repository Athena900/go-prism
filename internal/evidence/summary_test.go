package evidence

import "testing"

func TestNewMaintainerSummaryOmittedForEmptyReport(t *testing.T) {
	if got := NewMaintainerSummary(nil, StatusPass); got != nil {
		t.Fatalf("NewMaintainerSummary() = %#v, want nil", got)
	}
}

func TestNewMaintainerSummaryUsesDecisionHeadline(t *testing.T) {
	tests := []struct {
		name     string
		decision Status
		want     string
	}{
		{
			name:     "block",
			decision: StatusBlock,
			want:     "Release review is blocked by deterministic evidence.",
		},
		{
			name:     "warn",
			decision: StatusWarn,
			want:     "Maintainer review is needed before release.",
		},
		{
			name:     "unknown",
			decision: StatusUnknown,
			want:     "Some checks could not complete and need review.",
		},
		{
			name:     "pass",
			decision: StatusPass,
			want:     "No blockers or warnings were found in deterministic evidence.",
		},
	}

	items := []Item{{
		ID:       "gomod.ok",
		Title:    "go.mod parsed",
		Status:   StatusPass,
		Severity: SeverityNone,
		Category: CategoryGoMod,
		Summary:  "current go.mod can be parsed.",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMaintainerSummary(items, tt.decision)
			if got.Headline != tt.want {
				t.Fatalf("Headline = %q, want %q", got.Headline, tt.want)
			}
		})
	}
}

func TestNewMaintainerSummaryPrioritizesRiskEvidence(t *testing.T) {
	items := []Item{
		{
			ID:       "vuln.clean",
			Title:    "no new vulnerabilities",
			Status:   StatusPass,
			Severity: SeverityNone,
			Category: CategoryVuln,
			Summary:  "no vulnerability delta was found.",
		},
		{
			ID:       "api.breaking",
			Title:    "API incompatibility found",
			Status:   StatusBlock,
			Severity: SeverityHigh,
			Category: CategoryAPI,
			Summary:  "exported API changed incompatibly.",
		},
		{
			ID:       "downstream.failed",
			Title:    "downstream canary failed",
			Status:   StatusWarn,
			Severity: SeverityMedium,
			Category: CategoryDownstream,
			Summary:  "consumer tests failed.",
		},
		{
			ID:       "api.unknown",
			Title:    "go-apidiff unavailable",
			Status:   StatusUnknown,
			Severity: SeverityMedium,
			Category: CategoryAPI,
			Summary:  "tool was not found.",
		},
	}

	summary := NewMaintainerSummary(items, StatusBlock)
	if len(summary.KeyFindings) != 3 {
		t.Fatalf("KeyFindings len = %d, want 3", len(summary.KeyFindings))
	}
	wantIDs := []string{"api.breaking", "downstream.failed", "api.unknown"}
	for i, want := range wantIDs {
		got := summary.KeyFindings[i].EvidenceIDs[0]
		if got != want {
			t.Fatalf("KeyFindings[%d] evidence = %q, want %q", i, got, want)
		}
	}
}

func TestNewMaintainerSummarySkipsMetaWhenNonMetaEvidenceExists(t *testing.T) {
	items := []Item{
		{
			ID:       "pr.context",
			Title:    "PR context captured",
			Status:   StatusInfo,
			Severity: SeverityNone,
			Category: CategoryMeta,
			Summary:  "base and head refs were captured.",
		},
		{
			ID:       "gomod.ok",
			Title:    "go.mod parsed",
			Status:   StatusPass,
			Severity: SeverityNone,
			Category: CategoryGoMod,
			Summary:  "current go.mod can be parsed.",
		},
	}

	summary := NewMaintainerSummary(items, StatusPass)
	if len(summary.KeyFindings) != 1 {
		t.Fatalf("KeyFindings len = %d, want 1", len(summary.KeyFindings))
	}
	if got := summary.KeyFindings[0].EvidenceIDs[0]; got != "gomod.ok" {
		t.Fatalf("KeyFindings[0] evidence = %q, want gomod.ok", got)
	}
}

func TestNewMaintainerSummaryActionsUseRecommendationsAndDefaults(t *testing.T) {
	items := []Item{
		{
			ID:             "api.breaking",
			Title:          "API incompatibility found",
			Status:         StatusBlock,
			Severity:       SeverityHigh,
			Category:       CategoryAPI,
			Summary:        "exported API changed incompatibly.",
			Recommendation: "Publish this change as a major release.",
		},
		{
			ID:       "api.unknown",
			Title:    "go-apidiff unavailable",
			Status:   StatusUnknown,
			Severity: SeverityMedium,
			Category: CategoryAPI,
			Summary:  "tool was not found.",
		},
	}

	summary := NewMaintainerSummary(items, StatusBlock)
	if len(summary.NextActions) != 2 {
		t.Fatalf("NextActions len = %d, want 2", len(summary.NextActions))
	}
	if got := summary.NextActions[0].Text; got != "Publish this change as a major release." {
		t.Fatalf("NextActions[0] = %q", got)
	}
	if got := summary.NextActions[1].Text; got != "Re-run or inspect this check before relying on the report." {
		t.Fatalf("NextActions[1] = %q", got)
	}
}

func TestNewMaintainerSummaryCollectsStableEvidenceIDs(t *testing.T) {
	items := []Item{
		{
			ID:       "downstream.failed",
			Title:    "downstream canary failed",
			Status:   StatusWarn,
			Severity: SeverityMedium,
			Category: CategoryDownstream,
			Summary:  "consumer tests failed.",
		},
		{
			ID:       "api.breaking",
			Title:    "API incompatibility found",
			Status:   StatusBlock,
			Severity: SeverityHigh,
			Category: CategoryAPI,
			Summary:  "exported API changed incompatibly.",
		},
	}

	summary := NewMaintainerSummary(items, StatusBlock)
	want := []string{"api.breaking", "downstream.failed"}
	if len(summary.EvidenceIDs) != len(want) {
		t.Fatalf("EvidenceIDs len = %d, want %d", len(summary.EvidenceIDs), len(want))
	}
	for i, id := range want {
		if summary.EvidenceIDs[i] != id {
			t.Fatalf("EvidenceIDs[%d] = %q, want %q", i, summary.EvidenceIDs[i], id)
		}
	}
}
