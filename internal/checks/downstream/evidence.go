package downstream

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
	"github.com/Athena900/go-prism/internal/redact"
)

const maxDetails = 20

var idUnsafePattern = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func noModulesEvidence(opts Options) evidence.Item {
	return evidence.Item{
		ID:             "downstream.no_modules",
		Title:          "No downstream canaries configured",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "config",
		Summary:        "Downstream checking is enabled, but no downstream modules are configured.",
		Recommendation: "Add at least one checks.downstream.modules entry before trusting downstream compatibility evidence.",
		Provenance:     provenance(opts, "select downstream modules"),
	}
}

func modulePathMissingEvidence(opts Options) evidence.Item {
	return evidence.Item{
		ID:             "downstream.module_path_missing",
		Title:          "Module path missing for downstream canaries",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "config",
		Summary:        "Downstream checking requires the target module path so go-prism can add a temporary replace directive.",
		Recommendation: "Set module in .go-prism.yml or pass --module.",
		Provenance:     provenance(opts, "resolve module path for downstream canaries"),
	}
}

func configFailedEvidence(opts Options, module Module, err error) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.config_failed", module.Name),
		Title:          "Downstream canary configuration failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "downstream",
		Summary:        fmt.Sprintf("Downstream canary `%s` could not be prepared: %v.", displayName(module.Name), err),
		Recommendation: "Confirm the downstream path is local, contains go.mod, and has the required local tools installed.",
		Provenance:     moduleProvenance(opts, module, moduleTarget{}, "prepare downstream canary"),
	}
}

func remoteConfigFailedEvidence(opts Options, module Module, err error) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.remote.config_failed", module.Name),
		Title:          "Remote downstream canary configuration failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "downstream",
		Summary:        fmt.Sprintf("Remote downstream canary `%s` could not be prepared: %v.", displayName(module.Name), err),
		Recommendation: "Confirm the remote repo is a public HTTPS Git URL and the configured subdir is a Go module.",
		Provenance:     moduleProvenance(opts, module, moduleTarget{}, "prepare remote downstream canary"),
	}
}

func remoteCloneFailedEvidence(opts Options, module Module, target moduleTarget, result command.Result) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.remote.clone_failed", module.Name),
		Title:          "Remote downstream clone failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "git clone",
		Summary:        fmt.Sprintf("go-prism could not clone remote downstream canary `%s`.", displayName(module.Name)),
		Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxDetails),
		Recommendation: "Confirm the remote repo URL is public, reachable, and trusted before using it as a downstream canary.",
		Provenance:     moduleProvenance(opts, module, target, "git clone"),
	}
}

func remoteCheckoutFailedEvidence(opts Options, module Module, target moduleTarget, result command.Result) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.remote.checkout_failed", module.Name),
		Title:          "Remote downstream checkout failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "git checkout",
		Summary:        fmt.Sprintf("go-prism could not check out the configured ref for remote downstream canary `%s`.", displayName(module.Name)),
		Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxDetails),
		Recommendation: "Confirm the configured remote downstream ref exists and is fetchable.",
		Provenance:     moduleProvenance(opts, module, target, "git checkout remote downstream ref"),
	}
}

func remoteModuleMissingEvidence(opts Options, module Module, target moduleTarget, err error) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.remote.module_missing", module.Name),
		Title:          "Remote downstream module was not found",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "downstream",
		Summary:        fmt.Sprintf("Remote downstream canary `%s` does not have a readable go.mod at the configured subdir: %v.", displayName(module.Name), err),
		Recommendation: "Set checks.downstream.modules[].subdir to the Go module directory inside the cloned repository.",
		Provenance:     moduleProvenance(opts, module, target, "resolve remote downstream module"),
	}
}

func remoteCleanupFailedEvidence(opts Options, module Module, target moduleTarget, err error) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.remote.cleanup_failed", module.Name),
		Title:          "Remote downstream cleanup failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "go-prism",
		Summary:        fmt.Sprintf("go-prism could not remove the temporary clone for remote downstream canary `%s`: %v.", displayName(module.Name), err),
		Recommendation: "Inspect and remove the temporary downstream clone before rerunning canaries.",
		Provenance:     moduleProvenance(opts, module, target, "cleanup remote downstream clone"),
	}
}

func replaceFailedEvidence(opts Options, module Module, target moduleTarget, result command.Result) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.replace_failed", module.Name),
		Title:          "Downstream temporary replace failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "go mod edit",
		Summary:        fmt.Sprintf("go-prism could not add a temporary replace directive for downstream canary `%s`.", target.Name),
		Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxDetails),
		Recommendation: "Run go mod edit manually in the downstream module and confirm the module path is correct.",
		Provenance:     moduleProvenance(opts, module, target, "go mod edit -replace"),
	}
}

func restoreFailedEvidence(opts Options, module Module, target moduleTarget, err error) evidence.Item {
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.restore_failed", module.Name),
		Title:          "Downstream canary restore failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityHigh,
		Category:       evidence.CategoryDownstream,
		Source:         "go-prism",
		Summary:        fmt.Sprintf("go-prism could not restore go.mod/go.sum for downstream canary `%s`: %v.", target.Name, err),
		Recommendation: "Inspect the downstream module and restore go.mod/go.sum before rerunning downstream canaries.",
		Provenance:     moduleProvenance(opts, module, target, "restore downstream go.mod and go.sum"),
	}
}

func commandFailedEvidence(opts Options, module Module, target moduleTarget, result command.Result) evidence.Item {
	return evidence.Item{
		ID:       moduleEvidenceID("downstream.command_failed", module.Name),
		Title:    "Downstream canary failed",
		Status:   evidence.StatusBlock,
		Severity: evidence.SeverityHigh,
		Category: evidence.CategoryDownstream,
		Source:   "downstream",
		Summary: fmt.Sprintf(
			"Downstream canary `%s` failed with exit code %d.",
			target.Name,
			result.ExitCode,
		),
		Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxDetails),
		Recommendation: "Review whether this PR broke a configured downstream consumer or whether the downstream command needs adjustment.",
		Provenance:     moduleProvenance(opts, module, target, target.Command),
	}
}

func passedEvidence(opts Options, module Module, target moduleTarget, result command.Result) evidence.Item {
	details := boundedLines(result.Stderr+"\n"+result.Stdout, 5)
	if len(details) == 0 {
		details = []string{"command completed successfully"}
	}
	return evidence.Item{
		ID:             moduleEvidenceID("downstream.passed", module.Name),
		Title:          "Downstream canary passed",
		Status:         evidence.StatusPass,
		Severity:       evidence.SeverityNone,
		Category:       evidence.CategoryDownstream,
		Source:         "downstream",
		Summary:        fmt.Sprintf("Downstream canary `%s` passed.", target.Name),
		Details:        details,
		Recommendation: "No downstream compatibility blocker was reported by this canary.",
		Provenance:     moduleProvenance(opts, module, target, target.Command),
	}
}

func timeoutEvidence(opts Options, err error) evidence.Item {
	return evidence.Item{
		ID:             "downstream.timeout",
		Title:          "Downstream canary timed out",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryDownstream,
		Source:         "go-prism",
		Summary:        err.Error(),
		Recommendation: "Retry with a longer timeout before trusting downstream compatibility evidence.",
		Provenance:     provenance(opts, "check downstream timeout"),
	}
}

func provenance(opts Options, command string) evidence.Provenance {
	return evidence.Provenance{
		Base:    opts.Base,
		Head:    opts.Head,
		WorkDir: opts.WorkDir,
		Command: command,
		Tool:    "downstream",
	}
}

func moduleProvenance(opts Options, module Module, target moduleTarget, command string) evidence.Provenance {
	extra := map[string]string{
		"module":      module.Name,
		"module_path": opts.ModulePath,
	}
	if module.Path != "" {
		extra["configured_path"] = module.Path
	}
	if module.Repo != "" {
		extra["repo"] = safeRemoteRepo(module.Repo)
	}
	if module.Ref != "" {
		extra["ref"] = module.Ref
	}
	if module.Subdir != "" {
		extra["subdir"] = module.Subdir
	}
	if target.Path != "" {
		extra["downstream_path"] = target.Path
	}
	if target.HeadPath != "" {
		extra["replace_target"] = target.HeadPath
	}
	return evidence.Provenance{
		Base:    opts.Base,
		Head:    opts.Head,
		WorkDir: opts.WorkDir,
		Command: command,
		Tool:    "downstream",
		Extra:   extra,
	}
}

func safeRemoteRepo(raw string) string {
	redacted := redact.Sensitive(raw)
	parsed, err := url.Parse(redacted)
	if err != nil {
		return redacted
	}
	if parsed.User != nil {
		parsed.User = url.User("REDACTED")
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = "REDACTED"
	}
	if parsed.Fragment != "" {
		parsed.Fragment = "REDACTED"
	}
	return parsed.String()
}

func boundedLines(output string, limit int) []string {
	lines := strings.Split(output, "\n")
	details := make([]string, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		details = append(details, redact.Sensitive(line))
		if len(details) == limit {
			break
		}
	}
	return details
}

func moduleEvidenceID(prefix string, name string) string {
	clean := strings.Trim(idUnsafePattern.ReplaceAllString(strings.ToLower(name), "_"), "_")
	if clean == "" {
		clean = "unnamed"
	}
	return prefix + "." + clean
}

func displayName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "unnamed"
	}
	return name
}
