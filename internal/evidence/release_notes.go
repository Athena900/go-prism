package evidence

import (
	"sort"
	"strings"
)

const maxReleaseNoteBullets = 5

// NewReleaseNotesDraft creates a deterministic draft for maintainer-authored release notes.
func NewReleaseNotesDraft(items []Item, suggestedImpact string) *ReleaseNotesDraft {
	notes := releaseNoteBullets(items)
	if len(notes) == 0 {
		return nil
	}

	draft := &ReleaseNotesDraft{
		SuggestedImpact: normalizeReleaseImpact(suggestedImpact),
		Notes:           firstReleaseNoteBullets(notes, maxReleaseNoteBullets),
	}
	draft.EvidenceIDs = collectReleaseNoteEvidenceIDs(draft.Notes)
	return draft
}

func releaseNoteBullets(items []Item) []ReleaseNoteBullet {
	candidates := make([]releaseNoteCandidate, 0, len(items))
	for _, item := range items {
		candidate, ok := releaseNoteCandidateForItem(item)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		return candidates[i].bullet.EvidenceIDs[0] < candidates[j].bullet.EvidenceIDs[0]
	})

	notes := make([]ReleaseNoteBullet, 0, len(candidates))
	for _, candidate := range candidates {
		notes = append(notes, candidate.bullet)
	}
	return notes
}

type releaseNoteCandidate struct {
	rank   int
	bullet ReleaseNoteBullet
}

func releaseNoteCandidateForItem(item Item) (releaseNoteCandidate, bool) {
	id := strings.TrimSpace(item.ID)
	if id == "" || item.Category == CategoryMeta {
		return releaseNoteCandidate{}, false
	}

	switch item.Category {
	case CategoryAPI:
		return apiReleaseNoteCandidate(item)
	case CategoryVuln:
		return vulnerabilityReleaseNoteCandidate(item)
	case CategoryDownstream:
		return downstreamReleaseNoteCandidate(item)
	case CategoryGoMod:
		return goModReleaseNoteCandidate(item)
	default:
		return releaseNoteCandidate{}, false
	}
}

func apiReleaseNoteCandidate(item Item) (releaseNoteCandidate, bool) {
	releaseImpact := item.Provenance.Extra["release_impact"]
	switch {
	case item.Status == StatusBlock || releaseImpact == "major":
		return newReleaseNoteCandidate(10, item.ID, "Potential breaking public API impact was detected; include a breaking-change or migration note before release."), true
	case item.Status == StatusWarn || releaseImpact == "minor":
		return newReleaseNoteCandidate(20, item.ID, "Public API changes were detected; consider documenting the added or changed API surface."), true
	case releaseImpact == "patch":
		return newReleaseNoteCandidate(30, item.ID, "API evidence indicates this change is compatible with a patch-level release impact."), true
	default:
		return releaseNoteCandidate{}, false
	}
}

func vulnerabilityReleaseNoteCandidate(item Item) (releaseNoteCandidate, bool) {
	switch {
	case strings.Contains(item.ID, ".fixed_"):
		return newReleaseNoteCandidate(40, item.ID, "Vulnerability findings were fixed; consider mentioning the dependency or toolchain update when useful."), true
	case strings.Contains(item.ID, ".new_") || item.Status == StatusBlock || item.Status == StatusWarn:
		return newReleaseNoteCandidate(45, item.ID, "New vulnerability findings need security review before release notes are finalized."), true
	default:
		return releaseNoteCandidate{}, false
	}
}

func downstreamReleaseNoteCandidate(item Item) (releaseNoteCandidate, bool) {
	switch item.Status {
	case StatusPass:
		return newReleaseNoteCandidate(55, item.ID, "Configured downstream canaries passed against this change."), true
	case StatusBlock:
		return newReleaseNoteCandidate(50, item.ID, "Downstream compatibility needs review before release notes are finalized."), true
	default:
		return releaseNoteCandidate{}, false
	}
}

func goModReleaseNoteCandidate(item Item) (releaseNoteCandidate, bool) {
	switch item.ID {
	case "gomod.diff.module_changed":
		return newReleaseNoteCandidate(60, item.ID, "The module path changed; release notes should explain the module identity change."), true
	case "gomod.diff.go_directive_changed":
		return newReleaseNoteCandidate(61, item.ID, "The supported Go version changed; release notes should mention the new Go version requirement if it affects users."), true
	case "gomod.diff.toolchain_changed":
		return newReleaseNoteCandidate(62, item.ID, "The Go toolchain directive changed; consider documenting contributor or CI impact."), true
	case "gomod.diff.direct_requirements_changed":
		return newReleaseNoteCandidate(63, item.ID, "Direct dependencies changed; consider mentioning user-visible dependency or compatibility impact."), true
	case "gomod.diff.replace_changed":
		return newReleaseNoteCandidate(64, item.ID, "Replace directives changed; verify whether this should be explained before publishing a module release."), true
	case "gomod.diff.retract_changed":
		return newReleaseNoteCandidate(65, item.ID, "Retract directives changed; release notes should explain added or removed version retractions."), true
	default:
		return releaseNoteCandidate{}, false
	}
}

func newReleaseNoteCandidate(rank int, evidenceID string, text string) releaseNoteCandidate {
	return releaseNoteCandidate{
		rank: rank,
		bullet: ReleaseNoteBullet{
			Text:        text,
			EvidenceIDs: []string{evidenceID},
		},
	}
}

func firstReleaseNoteBullets(notes []ReleaseNoteBullet, max int) []ReleaseNoteBullet {
	if len(notes) <= max {
		return notes
	}
	return notes[:max]
}

func collectReleaseNoteEvidenceIDs(notes []ReleaseNoteBullet) []string {
	seen := map[string]bool{}
	ids := make([]string, 0)
	for _, note := range notes {
		for _, id := range note.EvidenceIDs {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func normalizeReleaseImpact(impact string) string {
	switch strings.TrimSpace(impact) {
	case "major", "minor", "patch":
		return strings.TrimSpace(impact)
	default:
		return "unknown"
	}
}
