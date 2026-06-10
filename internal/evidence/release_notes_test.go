package evidence

import (
	"strings"
	"testing"
)

func TestNewReleaseNotesDraftOmittedForEmptyOrMetaOnlyReport(t *testing.T) {
	if got := NewReleaseNotesDraft(nil, "minor"); got != nil {
		t.Fatalf("NewReleaseNotesDraft(nil) = %#v, want nil", got)
	}

	got := NewReleaseNotesDraft([]Item{{
		ID:       "pr.context",
		Title:    "PR context captured",
		Status:   StatusInfo,
		Severity: SeverityNone,
		Category: CategoryMeta,
		Summary:  "base and head refs were captured.",
	}}, "minor")
	if got != nil {
		t.Fatalf("NewReleaseNotesDraft(meta-only) = %#v, want nil", got)
	}
}

func TestNewReleaseNotesDraftBuildsAPIBreakingNote(t *testing.T) {
	draft := NewReleaseNotesDraft([]Item{{
		ID:       "api.goapidiff.incompatible_changes",
		Title:    "go-apidiff found incompatible API changes",
		Status:   StatusBlock,
		Severity: SeverityHigh,
		Category: CategoryAPI,
		Source:   "go-apidiff",
		Summary:  "go-apidiff reported public API changes that can break callers.",
		Provenance: Provenance{
			Extra: map[string]string{"release_impact": "major"},
		},
	}}, "major")

	if draft == nil {
		t.Fatal("NewReleaseNotesDraft() = nil, want draft")
	}
	if draft.SuggestedImpact != "major" {
		t.Fatalf("SuggestedImpact = %q, want major", draft.SuggestedImpact)
	}
	assertReleaseNote(t, draft, 0, "api.goapidiff.incompatible_changes", "breaking public API")
}

func TestNewReleaseNotesDraftBuildsAPIMinorNote(t *testing.T) {
	draft := NewReleaseNotesDraft([]Item{{
		ID:       "api.modver.minor_required",
		Title:    "modver requires a minor version bump",
		Status:   StatusWarn,
		Severity: SeverityMedium,
		Category: CategoryAPI,
		Source:   "modver",
		Summary:  "modver reported backward-compatible public API additions.",
		Provenance: Provenance{
			Extra: map[string]string{"release_impact": "minor"},
		},
	}}, "minor")

	assertReleaseNote(t, draft, 0, "api.modver.minor_required", "Public API changes")
}

func TestNewReleaseNotesDraftBuildsVulnerabilityAndDownstreamNotes(t *testing.T) {
	draft := NewReleaseNotesDraft([]Item{
		{
			ID:       "vuln.govulncheck.delta.fixed_findings",
			Title:    "govulncheck vulnerability findings fixed",
			Status:   StatusInfo,
			Severity: SeverityLow,
			Category: CategoryVuln,
			Source:   "govulncheck",
			Summary:  "1 vulnerability finding group is present in base but absent from head.",
		},
		{
			ID:       "downstream.passed.consumer",
			Title:    "Downstream canary passed",
			Status:   StatusPass,
			Severity: SeverityNone,
			Category: CategoryDownstream,
			Source:   "downstream",
			Summary:  "Downstream canary `consumer` passed.",
		},
	}, "patch")

	if draft.SuggestedImpact != "patch" {
		t.Fatalf("SuggestedImpact = %q, want patch", draft.SuggestedImpact)
	}
	assertReleaseNote(t, draft, 0, "vuln.govulncheck.delta.fixed_findings", "Vulnerability findings were fixed")
	assertReleaseNote(t, draft, 1, "downstream.passed.consumer", "downstream canaries passed")
}

func TestNewReleaseNotesDraftBuildsGoModNotes(t *testing.T) {
	draft := NewReleaseNotesDraft([]Item{{
		ID:       "gomod.diff.go_directive_changed",
		Title:    "Go directive changed",
		Status:   StatusWarn,
		Severity: SeverityMedium,
		Category: CategoryGoMod,
		Source:   "go.mod diff",
		Summary:  "Go directive changed from `1.22.0` to `1.23.0`.",
	}}, "unknown")

	assertReleaseNote(t, draft, 0, "gomod.diff.go_directive_changed", "supported Go version changed")
}

func TestNewReleaseNotesDraftOrdersAndCapsNotes(t *testing.T) {
	items := []Item{
		releaseNoteItem("gomod.diff.replace_changed", StatusWarn, CategoryGoMod),
		releaseNoteItem("downstream.command_failed.consumer", StatusBlock, CategoryDownstream),
		releaseNoteItem("vuln.govulncheck.delta.new_findings", StatusWarn, CategoryVuln),
		releaseNoteItem("api.modver.minor_required", StatusWarn, CategoryAPI),
		releaseNoteItem("gomod.diff.direct_requirements_changed", StatusWarn, CategoryGoMod),
		releaseNoteItem("downstream.passed.consumer", StatusPass, CategoryDownstream),
	}

	draft := NewReleaseNotesDraft(items, "minor")
	if len(draft.Notes) != maxReleaseNoteBullets {
		t.Fatalf("Notes len = %d, want %d", len(draft.Notes), maxReleaseNoteBullets)
	}
	wantIDs := []string{
		"api.modver.minor_required",
		"vuln.govulncheck.delta.new_findings",
		"downstream.command_failed.consumer",
		"downstream.passed.consumer",
		"gomod.diff.direct_requirements_changed",
	}
	for i, want := range wantIDs {
		if got := draft.Notes[i].EvidenceIDs[0]; got != want {
			t.Fatalf("Notes[%d] evidence = %q, want %q", i, got, want)
		}
	}
}

func TestNewReleaseNotesDraftCollectsStableEvidenceIDs(t *testing.T) {
	draft := NewReleaseNotesDraft([]Item{
		releaseNoteItem("downstream.passed.consumer", StatusPass, CategoryDownstream),
		releaseNoteItem("api.modver.minor_required", StatusWarn, CategoryAPI),
	}, "minor")

	want := []string{"api.modver.minor_required", "downstream.passed.consumer"}
	if len(draft.EvidenceIDs) != len(want) {
		t.Fatalf("EvidenceIDs len = %d, want %d", len(draft.EvidenceIDs), len(want))
	}
	for i, id := range want {
		if draft.EvidenceIDs[i] != id {
			t.Fatalf("EvidenceIDs[%d] = %q, want %q", i, draft.EvidenceIDs[i], id)
		}
	}
}

func releaseNoteItem(id string, status Status, category Category) Item {
	item := Item{
		ID:       id,
		Title:    id,
		Status:   status,
		Severity: SeverityMedium,
		Category: category,
		Source:   "test",
		Summary:  id,
	}
	if category == CategoryAPI {
		item.Provenance.Extra = map[string]string{"release_impact": "minor"}
	}
	return item
}

func assertReleaseNote(t *testing.T, draft *ReleaseNotesDraft, index int, wantID string, wantText string) {
	t.Helper()
	if draft == nil {
		t.Fatal("draft = nil, want release notes draft")
	}
	if len(draft.Notes) <= index {
		t.Fatalf("Notes len = %d, want index %d", len(draft.Notes), index)
	}
	note := draft.Notes[index]
	if len(note.EvidenceIDs) != 1 || note.EvidenceIDs[0] != wantID {
		t.Fatalf("Notes[%d].EvidenceIDs = %#v, want [%q]", index, note.EvidenceIDs, wantID)
	}
	if !strings.Contains(note.Text, wantText) {
		t.Fatalf("Notes[%d].Text = %q, want substring %q", index, note.Text, wantText)
	}
}
