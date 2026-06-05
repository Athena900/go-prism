package api

import (
	"context"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

// Options configures API and SemVer evidence checks.
type Options struct {
	WorkDir string
	Base    string
	Head    string
}

// Adapter turns one API/SemVer tool signal into normalized evidence.
type Adapter interface {
	Check(ctx context.Context, opts Options, tools ToolRunner) evidence.Item
}

type ToolRunner = command.Runner

type ToolInvocation = command.Invocation

type ToolResult = command.Result

// Check emits API/SemVer evidence from the default adapter set.
func Check(ctx context.Context, opts Options) []evidence.Item {
	return CheckWithAdapters(ctx, opts, defaultAdapters(), command.LocalRunner{})
}

// CheckWithAdapters runs the supplied adapters. It is kept exported for focused
// tests and future config-driven adapter selection.
func CheckWithAdapters(ctx context.Context, opts Options, adapters []Adapter, tools ToolRunner) []evidence.Item {
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
		items = append(items, adapter.Check(ctx, opts, tools))
	}

	if len(items) == 0 {
		return []evidence.Item{{
			ID:             "api.no_adapters",
			Title:          "No API adapters configured",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryAPI,
			Source:         "go-prism",
			Summary:        "API/SemVer checking is enabled, but no adapters are configured.",
			Recommendation: "Configure at least one API adapter before trusting release-impact evidence.",
			Provenance:     provenance(opts, "select API adapters", ""),
		}}
	}

	return items
}

func defaultAdapters() []Adapter {
	return []Adapter{
		GoreleaseAdapter{},
	}
}
