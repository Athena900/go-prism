package gomod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Athena900/go-prism/internal/evidence"
	"golang.org/x/mod/modfile"
)

// Options configures the current go.mod check.
type Options struct {
	WorkDir string
	Base    string
	Head    string
}

// CheckCurrent inspects the current go.mod state and emits deterministic evidence.
func CheckCurrent(ctx context.Context, opts Options) []evidence.Item {
	select {
	case <-ctx.Done():
		return []evidence.Item{timeoutEvidence(opts, ctx.Err())}
	default:
	}

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}
	goModPath := filepath.Join(workDir, "go.mod")

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return []evidence.Item{{
			ID:             "gomod.read_failed",
			Title:          "Unable to read go.mod",
			Status:         evidence.StatusBlock,
			Severity:       evidence.SeverityHigh,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod",
			Summary:        err.Error(),
			Recommendation: "Run go-prism from a Go module root or pass --workdir to the module directory.",
			Provenance:     provenance(opts, "read go.mod"),
		}}
	}

	file, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return []evidence.Item{{
			ID:             "gomod.parse_failed",
			Title:          "go.mod parse failed",
			Status:         evidence.StatusBlock,
			Severity:       evidence.SeverityHigh,
			Category:       evidence.CategoryGoMod,
			Source:         "golang.org/x/mod/modfile",
			Summary:        err.Error(),
			Recommendation: "Fix go.mod syntax before trusting release evidence.",
			Provenance:     provenance(opts, "modfile.Parse"),
		}}
	}

	items := []evidence.Item{moduleEvidence(opts, file)}
	items = append(items, goDirectiveEvidence(opts, file))
	items = append(items, replaceEvidence(opts, file)...)
	items = append(items, retractEvidence(opts, file)...)
	items = append(items, majorPathEvidence(opts, file)...)

	return items
}

func moduleEvidence(opts Options, file *modfile.File) evidence.Item {
	if file.Module == nil || file.Module.Mod.Path == "" {
		return evidence.Item{
			ID:             "gomod.module_missing",
			Title:          "Module path missing",
			Status:         evidence.StatusBlock,
			Severity:       evidence.SeverityHigh,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod",
			Summary:        "go.mod does not declare a module path.",
			Recommendation: "Add a module directive before publishing release evidence.",
			Provenance:     provenance(opts, "parse module directive"),
		}
	}

	return evidence.Item{
		ID:             "gomod.module_path",
		Title:          "Module path detected",
		Status:         evidence.StatusInfo,
		Severity:       evidence.SeverityNone,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod",
		Summary:        fmt.Sprintf("Module path: `%s`.", file.Module.Mod.Path),
		Recommendation: "Confirm this matches the repository and intended public import path.",
		Provenance:     provenance(opts, "parse module directive"),
	}
}

func goDirectiveEvidence(opts Options, file *modfile.File) evidence.Item {
	if file.Go == nil || file.Go.Version == "" {
		return evidence.Item{
			ID:             "gomod.go_directive_missing",
			Title:          "Go directive missing",
			Status:         evidence.StatusWarn,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod",
			Summary:        "go.mod does not declare a Go language version.",
			Recommendation: "Declare a go directive so downstream users know the supported language floor.",
			Provenance:     provenance(opts, "parse go directive"),
		}
	}

	return evidence.Item{
		ID:             "gomod.go_directive",
		Title:          "Go directive detected",
		Status:         evidence.StatusInfo,
		Severity:       evidence.SeverityNone,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod",
		Summary:        fmt.Sprintf("Go directive: `%s`.", file.Go.Version),
		Recommendation: "Review Go version increases carefully in release PRs because they can affect downstream users.",
		Provenance:     provenance(opts, "parse go directive"),
	}
}

func replaceEvidence(opts Options, file *modfile.File) []evidence.Item {
	if len(file.Replace) == 0 {
		return []evidence.Item{{
			ID:             "gomod.replace_none",
			Title:          "No replace directives",
			Status:         evidence.StatusPass,
			Severity:       evidence.SeverityNone,
			Category:       evidence.CategoryGoMod,
			Source:         "go.mod",
			Summary:        "go.mod does not contain replace directives.",
			Recommendation: "No replace directive review needed.",
			Provenance:     provenance(opts, "parse replace directives"),
		}}
	}

	details := make([]string, 0, len(file.Replace))
	for _, replacement := range file.Replace {
		oldPath := replacement.Old.Path
		newPath := replacement.New.Path
		if replacement.New.Version != "" {
			newPath += "@" + replacement.New.Version
		}
		details = append(details, fmt.Sprintf("%s => %s", oldPath, newPath))
	}

	return []evidence.Item{{
		ID:             "gomod.replace_present",
		Title:          "replace directives present",
		Status:         evidence.StatusWarn,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod",
		Summary:        fmt.Sprintf("go.mod contains %d replace directive(s).", len(file.Replace)),
		Details:        details,
		Recommendation: "Confirm replace directives are intentional and safe for public module consumers.",
		Provenance:     provenance(opts, "parse replace directives"),
	}}
}

func retractEvidence(opts Options, file *modfile.File) []evidence.Item {
	if len(file.Retract) == 0 {
		return nil
	}

	details := make([]string, 0, len(file.Retract))
	for _, retract := range file.Retract {
		if retract.Low == retract.High {
			details = append(details, retract.Low)
			continue
		}
		details = append(details, fmt.Sprintf("[%s, %s]", retract.Low, retract.High))
	}

	return []evidence.Item{{
		ID:             "gomod.retract_present",
		Title:          "retract directives present",
		Status:         evidence.StatusInfo,
		Severity:       evidence.SeverityLow,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod",
		Summary:        fmt.Sprintf("go.mod contains %d retract directive(s).", len(file.Retract)),
		Details:        details,
		Recommendation: "Confirm release notes explain why versions were retracted.",
		Provenance:     provenance(opts, "parse retract directives"),
	}}
}

func majorPathEvidence(opts Options, file *modfile.File) []evidence.Item {
	if file.Module == nil {
		return nil
	}

	modulePath := file.Module.Mod.Path
	lastSlash := strings.LastIndex(modulePath, "/")
	if lastSlash == -1 {
		return nil
	}

	suffix := modulePath[lastSlash+1:]
	if len(suffix) < 2 || suffix[0] != 'v' {
		return nil
	}

	major, err := strconv.Atoi(suffix[1:])
	if err != nil || major < 2 {
		return nil
	}

	return []evidence.Item{{
		ID:             "gomod.major_path_suffix",
		Title:          "Major module path suffix detected",
		Status:         evidence.StatusInfo,
		Severity:       evidence.SeverityNone,
		Category:       evidence.CategoryGoMod,
		Source:         "go.mod",
		Summary:        fmt.Sprintf("Module path uses major suffix `/%s`.", suffix),
		Recommendation: "Confirm tags use the same major version line when publishing releases.",
		Provenance:     provenance(opts, "inspect module path"),
	}}
}

func timeoutEvidence(opts Options, err error) evidence.Item {
	return evidence.Item{
		ID:             "gomod.timeout",
		Title:          "go.mod check timed out",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryGoMod,
		Source:         "go-prism",
		Summary:        err.Error(),
		Recommendation: "Retry with a longer timeout before trusting this report.",
		Provenance:     provenance(opts, "check timeout"),
	}
}

func provenance(opts Options, command string) evidence.Provenance {
	return evidence.Provenance{
		Base:    opts.Base,
		Head:    opts.Head,
		WorkDir: opts.WorkDir,
		Command: command,
		Tool:    "go.mod",
	}
}
