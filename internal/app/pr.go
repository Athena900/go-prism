package app

import (
	"context"
	"time"

	"github.com/Athena900/go-prism/internal/checks/api"
	"github.com/Athena900/go-prism/internal/checks/downstream"
	"github.com/Athena900/go-prism/internal/checks/gomod"
	"github.com/Athena900/go-prism/internal/checks/vuln"
	"github.com/Athena900/go-prism/internal/config"
	"github.com/Athena900/go-prism/internal/evidence"
)

// PROptions describes a local pull-request style evidence run.
type PROptions struct {
	Base           string
	Head           string
	ConfigPath     string
	Format         string
	OutputPath     string
	WorkDir        string
	ModuleOverride string
	Timeout        time.Duration
}

// RunPR collects deterministic evidence for a pull-request style report.
func RunPR(ctx context.Context, opts PROptions) (evidence.Report, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return evidence.Report{}, err
	}

	modulePath := cfg.Module
	if opts.ModuleOverride != "" {
		modulePath = opts.ModuleOverride
	}

	items := []evidence.Item{
		{
			ID:             "pr.context",
			Title:          "PR context captured",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryMeta,
			Source:         "go-prism",
			Summary:        "go-prism captured base/head refs and configuration for this evidence run.",
			Recommendation: "Use this context when comparing report output across CI runs.",
			Provenance: evidence.Provenance{
				Base:    opts.Base,
				Head:    opts.Head,
				WorkDir: opts.WorkDir,
				Command: "go-prism pr",
			},
		},
	}

	if cfg.Checks.GoMod.Enabled {
		goModOpts := gomod.Options{
			WorkDir: opts.WorkDir,
			Base:    opts.Base,
			Head:    opts.Head,
		}
		items = append(items, gomod.CheckCurrent(ctx, goModOpts)...)
		items = append(items, gomod.CheckDiff(ctx, goModOpts)...)
	} else {
		items = append(items, evidence.Item{
			ID:             "gomod.disabled",
			Title:          "go.mod check disabled",
			Status:         evidence.StatusInfo,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryGoMod,
			Source:         "config",
			Summary:        "The go.mod checker is disabled by configuration.",
			Recommendation: "Enable checks.gomod when release evidence should include module metadata.",
		})
	}

	if cfg.Checks.API.Enabled {
		items = append(items, api.Check(ctx, api.Options{
			WorkDir: opts.WorkDir,
			Base:    opts.Base,
			Head:    opts.Head,
		})...)
	}

	if cfg.Checks.Vuln.Enabled {
		items = append(items, vuln.Check(ctx, vuln.Options{
			WorkDir: opts.WorkDir,
			Base:    opts.Base,
			Head:    opts.Head,
		})...)
	}

	if cfg.Checks.Downstream.Enabled {
		items = append(items, downstream.Check(ctx, downstream.Options{
			WorkDir:    opts.WorkDir,
			Base:       opts.Base,
			Head:       opts.Head,
			ModulePath: modulePath,
			Modules:    downstreamModules(cfg.Checks.Downstream.Modules),
		})...)
	}

	report := evidence.NewReport(evidence.ReportOptions{
		Tool:      "go-prism",
		Version:   "0.1.0-dev",
		Module:    modulePath,
		Base:      opts.Base,
		Head:      opts.Head,
		Items:     items,
		Generated: time.Now().UTC(),
	})

	return report, nil
}

func downstreamModules(modules []config.DownstreamModuleConfig) []downstream.Module {
	out := make([]downstream.Module, 0, len(modules))
	for _, module := range modules {
		out = append(out, downstream.Module{
			Name:    module.Name,
			Path:    module.Path,
			Command: module.Command,
		})
	}
	return out
}
