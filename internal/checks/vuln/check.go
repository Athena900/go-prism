package vuln

import (
	"context"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

// Options configures vulnerability evidence checks.
type Options struct {
	WorkDir string
	Base    string
	Head    string
}

// Adapter turns one vulnerability tool signal into normalized evidence.
type Adapter interface {
	Check(ctx context.Context, opts Options, tools command.Runner) []evidence.Item
}

// Check emits vulnerability evidence from the default adapter set.
func Check(ctx context.Context, opts Options) []evidence.Item {
	return CheckWithAdapters(ctx, opts, defaultAdapters(), command.LocalRunner{})
}

// CheckWithAdapters runs the supplied adapters. It is exported for focused tests.
func CheckWithAdapters(ctx context.Context, opts Options, adapters []Adapter, tools command.Runner) []evidence.Item {
	select {
	case <-ctx.Done():
		return []evidence.Item{timeoutEvidence(opts, ctx.Err())}
	default:
	}

	items := make([]evidence.Item, 0, len(adapters))
	for _, adapter := range adapters {
		select {
		case <-ctx.Done():
			items = append(items, timeoutEvidence(opts, ctx.Err()))
			return items
		default:
		}
		items = append(items, adapter.Check(ctx, opts, tools)...)
	}

	if len(items) == 0 {
		return []evidence.Item{{
			ID:             "vuln.no_adapters",
			Title:          "No vulnerability adapters configured",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryVuln,
			Source:         "go-prism",
			Summary:        "Vulnerability checking is enabled, but no adapters are configured.",
			Recommendation: "Configure at least one vulnerability adapter before trusting vulnerability evidence.",
			Provenance:     provenance(opts, "select vulnerability adapters", ""),
		}}
	}

	return items
}

func defaultAdapters() []Adapter {
	return []Adapter{
		GovulncheckAdapter{},
	}
}
