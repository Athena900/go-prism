package evidence

import "time"

const ReportSchemaVersion = "report.v1"

// Status captures the outcome of an evidence item.
type Status string

const (
	StatusPass    Status = "pass"
	StatusInfo    Status = "info"
	StatusWarn    Status = "warn"
	StatusBlock   Status = "block"
	StatusUnknown Status = "unknown"
)

// Severity captures risk severity.
type Severity string

const (
	SeverityNone     Severity = "none"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Category groups evidence by maintainer concern.
type Category string

const (
	CategoryMeta       Category = "meta"
	CategoryGoMod      Category = "gomod"
	CategoryAPI        Category = "api"
	CategoryVuln       Category = "vulnerability"
	CategoryDownstream Category = "downstream"
)

// Provenance records where an evidence item came from.
type Provenance struct {
	Base    string            `json:"base,omitempty" yaml:"base,omitempty"`
	Head    string            `json:"head,omitempty" yaml:"head,omitempty"`
	WorkDir string            `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Command string            `json:"command,omitempty" yaml:"command,omitempty"`
	Tool    string            `json:"tool,omitempty" yaml:"tool,omitempty"`
	Extra   map[string]string `json:"extra,omitempty" yaml:"extra,omitempty"`
}

// Item is a single deterministic evidence finding.
type Item struct {
	ID             string     `json:"id" yaml:"id"`
	Title          string     `json:"title" yaml:"title"`
	Status         Status     `json:"status" yaml:"status"`
	Severity       Severity   `json:"severity" yaml:"severity"`
	Category       Category   `json:"category" yaml:"category"`
	Source         string     `json:"source" yaml:"source"`
	Summary        string     `json:"summary" yaml:"summary"`
	Details        []string   `json:"details,omitempty" yaml:"details,omitempty"`
	Recommendation string     `json:"recommendation,omitempty" yaml:"recommendation,omitempty"`
	Provenance     Provenance `json:"provenance,omitempty" yaml:"provenance,omitempty"`
}

// MaintainerSummary is a deterministic, human-readable report summary.
type MaintainerSummary struct {
	Headline    string           `json:"headline" yaml:"headline"`
	KeyFindings []SummaryFinding `json:"key_findings,omitempty" yaml:"key_findings,omitempty"`
	NextActions []SummaryFinding `json:"next_actions,omitempty" yaml:"next_actions,omitempty"`
	EvidenceIDs []string         `json:"evidence_ids,omitempty" yaml:"evidence_ids,omitempty"`
}

// SummaryFinding links summary text back to deterministic evidence items.
type SummaryFinding struct {
	Text        string   `json:"text" yaml:"text"`
	EvidenceIDs []string `json:"evidence_ids,omitempty" yaml:"evidence_ids,omitempty"`
}

// ReleaseNotesDraft is a deterministic draft for maintainer-authored release notes.
type ReleaseNotesDraft struct {
	SuggestedImpact string              `json:"suggested_impact" yaml:"suggested_impact"`
	Notes           []ReleaseNoteBullet `json:"notes,omitempty" yaml:"notes,omitempty"`
	EvidenceIDs     []string            `json:"evidence_ids,omitempty" yaml:"evidence_ids,omitempty"`
}

// ReleaseNoteBullet links draft release-note text to deterministic evidence items.
type ReleaseNoteBullet struct {
	Text        string   `json:"text" yaml:"text"`
	EvidenceIDs []string `json:"evidence_ids,omitempty" yaml:"evidence_ids,omitempty"`
}

// Report is the full output model used by renderers and CI.
type Report struct {
	SchemaVersion          string             `json:"schema_version" yaml:"schema_version"`
	Tool                   string             `json:"tool" yaml:"tool"`
	Version                string             `json:"version" yaml:"version"`
	Module                 string             `json:"module,omitempty" yaml:"module,omitempty"`
	Base                   string             `json:"base,omitempty" yaml:"base,omitempty"`
	Head                   string             `json:"head,omitempty" yaml:"head,omitempty"`
	Decision               Status             `json:"decision" yaml:"decision"`
	SuggestedReleaseImpact string             `json:"suggested_release_impact" yaml:"suggested_release_impact"`
	MaintainerSummary      *MaintainerSummary `json:"maintainer_summary,omitempty" yaml:"maintainer_summary,omitempty"`
	ReleaseNotesDraft      *ReleaseNotesDraft `json:"release_notes_draft,omitempty" yaml:"release_notes_draft,omitempty"`
	GeneratedAt            time.Time          `json:"generated_at" yaml:"generated_at"`
	Items                  []Item             `json:"items" yaml:"items"`
}

// ReportOptions describes report construction.
type ReportOptions struct {
	Tool      string
	Version   string
	Module    string
	Base      string
	Head      string
	Items     []Item
	Generated time.Time
}

// NewReport creates a report with computed top-level decision fields.
func NewReport(opts ReportOptions) Report {
	generated := opts.Generated
	if generated.IsZero() {
		generated = time.Now().UTC()
	}

	decision := Decide(opts.Items)
	releaseImpact := SuggestedReleaseImpact(opts.Items)

	return Report{
		SchemaVersion:          ReportSchemaVersion,
		Tool:                   opts.Tool,
		Version:                opts.Version,
		Module:                 opts.Module,
		Base:                   opts.Base,
		Head:                   opts.Head,
		Decision:               decision,
		SuggestedReleaseImpact: releaseImpact,
		MaintainerSummary:      NewMaintainerSummary(opts.Items, decision),
		ReleaseNotesDraft:      NewReleaseNotesDraft(opts.Items, releaseImpact),
		GeneratedAt:            generated,
		Items:                  opts.Items,
	}
}

// Decide returns the strongest report decision.
func Decide(items []Item) Status {
	hasUnknown := false
	hasWarn := false

	for _, item := range items {
		switch item.Status {
		case StatusBlock:
			return StatusBlock
		case StatusWarn:
			hasWarn = true
		case StatusUnknown:
			hasUnknown = true
		}
	}

	if hasWarn {
		return StatusWarn
	}
	if hasUnknown {
		return StatusUnknown
	}
	return StatusPass
}

// SuggestedReleaseImpact returns the strongest release impact reported by evidence.
func SuggestedReleaseImpact(items []Item) string {
	impact := "unknown"
	for _, item := range items {
		if item.Category == CategoryAPI && item.Provenance.Extra != nil {
			impact = strongerReleaseImpact(impact, item.Provenance.Extra["release_impact"])
		}
		if item.Category == CategoryAPI && item.Status == StatusBlock {
			impact = strongerReleaseImpact(impact, "major")
		}
	}
	return impact
}

func strongerReleaseImpact(current string, next string) string {
	if releaseImpactRank(next) > releaseImpactRank(current) {
		return next
	}
	return current
}

func releaseImpactRank(impact string) int {
	switch impact {
	case "major":
		return 3
	case "minor":
		return 2
	case "patch":
		return 1
	default:
		return 0
	}
}
