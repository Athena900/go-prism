package downstream

import (
	"context"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

// Options configures local downstream canary checks.
type Options struct {
	WorkDir    string
	Base       string
	Head       string
	ModulePath string
	Modules    []Module
}

// Module describes one local downstream consumer module.
type Module struct {
	Name    string
	Path    string
	Command string
}

// Check runs local downstream canaries from explicit configuration.
func Check(ctx context.Context, opts Options) []evidence.Item {
	return CheckWithRunner(ctx, opts, command.LocalRunner{})
}

// CheckWithRunner is exported for focused tests.
func CheckWithRunner(ctx context.Context, opts Options, tools command.Runner) []evidence.Item {
	select {
	case <-ctx.Done():
		return []evidence.Item{timeoutEvidence(opts, ctx.Err())}
	default:
	}

	if len(opts.Modules) == 0 {
		return []evidence.Item{noModulesEvidence(opts)}
	}
	if opts.ModulePath == "" {
		return []evidence.Item{modulePathMissingEvidence(opts)}
	}

	items := make([]evidence.Item, 0, len(opts.Modules))
	for _, module := range opts.Modules {
		select {
		case <-ctx.Done():
			items = append(items, timeoutEvidence(opts, ctx.Err()))
			return items
		default:
		}
		items = append(items, runModule(ctx, opts, module, tools))
	}
	return items
}
