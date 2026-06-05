package downstream

import (
	"context"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

// Options configures downstream canary checks.
type Options struct {
	WorkDir    string
	Base       string
	Head       string
	ModulePath string
	Modules    []Module
}

// Module describes one downstream consumer module.
type Module struct {
	Name    string
	Path    string
	Repo    string
	Ref     string
	Subdir  string
	Command string
}

// Check runs downstream canaries from explicit configuration.
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
		if module.isRemote() {
			items = append(items, runRemoteModule(ctx, opts, module, tools))
			continue
		}
		items = append(items, runModule(ctx, opts, module, tools))
	}
	return items
}

func (m Module) isRemote() bool {
	return m.Repo != ""
}
