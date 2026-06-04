package app

import (
	"context"
	"time"

	"github.com/Athena900/go-prism/internal/checks/gomod"
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
