package evidence

import "testing"

func TestNewReportSetsSchemaVersion(t *testing.T) {
	report := NewReport(ReportOptions{
		Tool:    "go-prism",
		Version: "test",
	})

	if report.SchemaVersion != ReportSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", report.SchemaVersion, ReportSchemaVersion)
	}
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name  string
		items []Item
		want  Status
	}{
		{name: "empty is pass", want: StatusPass},
		{
			name:  "warn beats unknown",
			items: []Item{{Status: StatusUnknown}, {Status: StatusWarn}},
			want:  StatusWarn,
		},
		{
			name:  "block wins",
			items: []Item{{Status: StatusWarn}, {Status: StatusBlock}},
			want:  StatusBlock,
		},
		{
			name:  "unknown when no warn or block",
			items: []Item{{Status: StatusPass}, {Status: StatusUnknown}},
			want:  StatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(tt.items); got != tt.want {
				t.Fatalf("Decide() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSuggestedReleaseImpact(t *testing.T) {
	items := []Item{
		{
			Status:   StatusPass,
			Category: CategoryAPI,
			Provenance: Provenance{
				Extra: map[string]string{"release_impact": "patch"},
			},
		},
		{
			Status:   StatusWarn,
			Category: CategoryAPI,
			Provenance: Provenance{
				Extra: map[string]string{"release_impact": "minor"},
			},
		},
	}

	if got := SuggestedReleaseImpact(items); got != "minor" {
		t.Fatalf("SuggestedReleaseImpact() = %q, want minor", got)
	}
}

func TestSuggestedReleaseImpactAPIBlockFallsBackToMajor(t *testing.T) {
	items := []Item{
		{
			Status:   StatusBlock,
			Category: CategoryAPI,
		},
	}

	if got := SuggestedReleaseImpact(items); got != "major" {
		t.Fatalf("SuggestedReleaseImpact() = %q, want major", got)
	}
}

func TestSuggestedReleaseImpactUsesModverProvenance(t *testing.T) {
	items := []Item{
		{
			Status:   StatusWarn,
			Category: CategoryAPI,
			Source:   "modver",
			Provenance: Provenance{
				Extra: map[string]string{"release_impact": "minor"},
			},
		},
	}

	if got := SuggestedReleaseImpact(items); got != "minor" {
		t.Fatalf("SuggestedReleaseImpact() = %q, want minor", got)
	}
}
