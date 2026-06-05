package vuln

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
	"github.com/Athena900/go-prism/internal/redact"
)

const maxGovulncheckDetails = 20

// GovulncheckAdapter collects vulnerability evidence from govulncheck.
type GovulncheckAdapter struct{}

// Check runs govulncheck when available and normalizes JSON findings into evidence.
func (GovulncheckAdapter) Check(ctx context.Context, opts Options, tools command.Runner) []evidence.Item {
	select {
	case <-ctx.Done():
		return []evidence.Item{timeoutEvidence(opts, ctx.Err())}
	default:
	}

	path, err := tools.LookPath("govulncheck")
	if err != nil {
		return []evidence.Item{{
			ID:       "vuln.govulncheck.not_installed",
			Title:    "govulncheck is not installed",
			Status:   evidence.StatusUnknown,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryVuln,
			Source:   "govulncheck",
			Summary:  "Vulnerability evidence could not be collected because `govulncheck` was not found on PATH.",
			Details: []string{
				fmt.Sprintf("lookup error: %v", err),
			},
			Recommendation: "Install `govulncheck` with `go install golang.org/x/vuln/cmd/govulncheck@latest`, or keep checks.vuln disabled until vulnerability evidence is required.",
			Provenance:     provenance(opts, "detect govulncheck", "govulncheck"),
		}}
	}

	headTarget, cleanup, err := prepareGovulncheckTarget(ctx, opts, opts.Head, "head", tools)
	if cleanup != nil {
		defer cleanup(context.Background())
	}
	if err != nil {
		return []evidence.Item{targetFailedEvidence(opts, "head", opts.Head, err)}
	}

	args := []string{"-format=json", "./..."}
	result := tools.Run(ctx, command.Invocation{
		Path: path,
		Args: args,
		Dir:  headTarget.WorkDir,
	})
	if ctx.Err() != nil {
		return []evidence.Item{timeoutEvidence(opts, ctx.Err())}
	}

	headReport, headParseErr := parseGovulncheckJSON(strings.NewReader(result.Stdout))
	items := []evidence.Item{
		classifyGovulncheckResult(targetOptions(opts, headTarget), path, args, result),
	}
	items = append(items, deltaEvidence(ctx, opts, path, args, headTarget, headReport, result, headParseErr, tools)...)
	return items
}

func classifyGovulncheckResult(opts Options, path string, args []string, result command.Result) evidence.Item {
	report, parseErr := parseGovulncheckJSON(strings.NewReader(result.Stdout))
	provenance := govulncheckProvenance(opts, path, args)

	if parseErr != nil {
		return evidence.Item{
			ID:             "vuln.govulncheck.parse_failed",
			Title:          "govulncheck JSON parse failed",
			Status:         evidence.StatusUnknown,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryVuln,
			Source:         "govulncheck",
			Summary:        fmt.Sprintf("go-prism could not parse govulncheck JSON output: %v.", parseErr),
			Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxGovulncheckDetails),
			Recommendation: "Inspect govulncheck output and update go-prism's parser if this JSON shape is expected.",
			Provenance:     provenance,
		}
	}

	if len(report.Findings) > 0 {
		details := findingDetails(report)
		if report.HasSymbolFinding {
			return evidence.Item{
				ID:             "vuln.govulncheck.reachable_findings",
				Title:          "govulncheck found reachable vulnerabilities",
				Status:         evidence.StatusBlock,
				Severity:       evidence.SeverityHigh,
				Category:       evidence.CategoryVuln,
				Source:         "govulncheck",
				Summary:        fmt.Sprintf("govulncheck reported %d vulnerability finding group(s), including reachable symbol-level findings.", len(report.Findings)),
				Details:        details,
				Recommendation: "Review the reachable vulnerabilities and update affected dependencies or Go toolchain before release.",
				Provenance:     provenance,
			}
		}

		return evidence.Item{
			ID:             "vuln.govulncheck.findings",
			Title:          "govulncheck found vulnerability findings",
			Status:         evidence.StatusWarn,
			Severity:       evidence.SeverityMedium,
			Category:       evidence.CategoryVuln,
			Source:         "govulncheck",
			Summary:        fmt.Sprintf("govulncheck reported %d vulnerability finding group(s), but no reachable symbol-level finding was detected in the JSON stream.", len(report.Findings)),
			Details:        details,
			Recommendation: "Review whether these vulnerable modules or packages are imported, reachable, or need dependency updates.",
			Provenance:     provenance,
		}
	}

	if result.Err != nil {
		return evidence.Item{
			ID:       "vuln.govulncheck.run_failed",
			Title:    "govulncheck execution failed",
			Status:   evidence.StatusUnknown,
			Severity: evidence.SeverityMedium,
			Category: evidence.CategoryVuln,
			Source:   "govulncheck",
			Summary: fmt.Sprintf(
				"govulncheck exited with code %d before reporting vulnerability findings: %v.",
				result.ExitCode,
				result.Err,
			),
			Details:        boundedLines(result.Stderr+"\n"+result.Stdout, maxGovulncheckDetails),
			Recommendation: "Run govulncheck locally and fix module loading, package pattern, or database access issues before trusting vulnerability evidence.",
			Provenance:     provenance,
		}
	}

	return evidence.Item{
		ID:             "vuln.govulncheck.no_findings",
		Title:          "govulncheck found no vulnerability findings",
		Status:         evidence.StatusPass,
		Severity:       evidence.SeverityNone,
		Category:       evidence.CategoryVuln,
		Source:         "govulncheck",
		Summary:        "govulncheck completed without reporting vulnerability findings for `./...`.",
		Details:        configDetails(report),
		Recommendation: "No vulnerability blocker was reported by govulncheck.",
		Provenance:     provenance,
	}
}

type govulncheckReport struct {
	Config           govulncheckConfig
	OSV              map[string]govulncheckOSV
	Findings         map[string]findingGroup
	HasSymbolFinding bool
}

type govulncheckMessage struct {
	Config  *govulncheckConfig  `json:"config,omitempty"`
	OSV     *govulncheckOSV     `json:"osv,omitempty"`
	Finding *govulncheckFinding `json:"finding,omitempty"`
}

type govulncheckConfig struct {
	ProtocolVersion string `json:"protocol_version,omitempty"`
	ScannerName     string `json:"scanner_name,omitempty"`
	ScannerVersion  string `json:"scanner_version,omitempty"`
	DB              string `json:"db,omitempty"`
	GoVersion       string `json:"go_version,omitempty"`
	ScanLevel       string `json:"scan_level,omitempty"`
	ScanMode        string `json:"scan_mode,omitempty"`
}

type govulncheckOSV struct {
	ID               string `json:"id,omitempty"`
	Details          string `json:"details,omitempty"`
	DatabaseSpecific struct {
		URL string `json:"url,omitempty"`
	} `json:"database_specific,omitempty"`
}

type govulncheckFinding struct {
	OSV          string             `json:"osv,omitempty"`
	FixedVersion string             `json:"fixed_version,omitempty"`
	Trace        []govulncheckFrame `json:"trace,omitempty"`
}

type govulncheckFrame struct {
	Module   string `json:"module,omitempty"`
	Version  string `json:"version,omitempty"`
	Package  string `json:"package,omitempty"`
	Function string `json:"function,omitempty"`
	Receiver string `json:"receiver,omitempty"`
}

type findingGroup struct {
	OSV          string
	FixedVersion string
	Level        string
	Module       string
	Version      string
	Package      string
	Function     string
	Count        int
}

func parseGovulncheckJSON(r io.Reader) (govulncheckReport, error) {
	report := govulncheckReport{
		OSV:      make(map[string]govulncheckOSV),
		Findings: make(map[string]findingGroup),
	}

	decoder := json.NewDecoder(r)
	for {
		var msg govulncheckMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return report, nil
			}
			return report, err
		}

		if msg.Config != nil {
			report.Config = *msg.Config
		}
		if msg.OSV != nil && msg.OSV.ID != "" {
			report.OSV[msg.OSV.ID] = *msg.OSV
		}
		if msg.Finding != nil && msg.Finding.OSV != "" {
			key := findingKey(*msg.Finding)
			group := report.Findings[key]
			group = mergeFinding(group, *msg.Finding)
			report.Findings[key] = group
			if group.Level == "symbol" {
				report.HasSymbolFinding = true
			}
		}
	}
}

func mergeFinding(group findingGroup, finding govulncheckFinding) findingGroup {
	level, frame := findingLevel(finding)
	if group.OSV == "" {
		group.OSV = finding.OSV
		group.Level = level
		group.Module = frame.Module
		group.Version = frame.Version
		group.Package = frame.Package
		group.Function = frame.Symbol()
	}

	if finding.FixedVersion != "" {
		group.FixedVersion = finding.FixedVersion
	}
	if findingLevelRank(level) > findingLevelRank(group.Level) {
		group.Level = level
		group.Module = frame.Module
		group.Version = frame.Version
		group.Package = frame.Package
		group.Function = frame.Symbol()
	}
	group.Count++
	return group
}

func findingKey(finding govulncheckFinding) string {
	level, frame := findingLevel(finding)
	return strings.Join([]string{
		finding.OSV,
		level,
		frame.Module,
		frame.Package,
		frame.Symbol(),
	}, "\x00")
}

func findingLevel(finding govulncheckFinding) (string, govulncheckFrame) {
	if len(finding.Trace) == 0 {
		return "module", govulncheckFrame{}
	}

	bestLevel := "module"
	bestFrame := finding.Trace[0]
	for _, frame := range finding.Trace {
		level := frameLevel(frame)
		if findingLevelRank(level) > findingLevelRank(bestLevel) {
			bestLevel = level
			bestFrame = frame
		}
	}
	return bestLevel, bestFrame
}

func frameLevel(frame govulncheckFrame) string {
	switch {
	case frame.Function != "":
		return "symbol"
	case frame.Package != "":
		return "package"
	default:
		return "module"
	}
}

func findingLevelRank(level string) int {
	switch level {
	case "symbol":
		return 3
	case "package":
		return 2
	case "module":
		return 1
	default:
		return 0
	}
}

func (f govulncheckFrame) Symbol() string {
	if f.Function == "" {
		return ""
	}
	if f.Receiver == "" {
		return f.Function
	}
	return f.Receiver + "." + f.Function
}

func findingDetails(report govulncheckReport) []string {
	keys := make([]string, 0, len(report.Findings))
	for key := range report.Findings {
		keys = append(keys, key)
	}
	return findingDetailsForKeys(report, keys, "")
}

func findingDetailsForKeys(report govulncheckReport, keys []string, prefix string) []string {
	sort.Strings(keys)
	details := make([]string, 0, minInt(len(keys), maxGovulncheckDetails))
	for _, key := range keys {
		details = append(details, prefix+formatFindingDetail(report, report.Findings[key]))
		if len(details) == maxGovulncheckDetails {
			break
		}
	}

	if len(keys) > maxGovulncheckDetails {
		details = append(details, fmt.Sprintf("... %d more vulnerability group(s) omitted", len(keys)-maxGovulncheckDetails))
	}

	return details
}

func formatFindingDetail(report govulncheckReport, group findingGroup) string {
	detail := group.OSV + " [" + group.Level + "]"
	if group.Module != "" {
		detail += " " + group.Module
	}
	if group.Version != "" {
		detail += "@" + group.Version
	}
	if group.Package != "" {
		detail += " package=" + group.Package
	}
	if group.Function != "" {
		detail += " symbol=" + group.Function
	}
	if group.FixedVersion != "" {
		detail += " fixed=" + group.FixedVersion
	}
	if osv := report.OSV[group.OSV]; osv.DatabaseSpecific.URL != "" {
		detail += " " + osv.DatabaseSpecific.URL
	}
	if group.Count > 1 {
		detail += fmt.Sprintf(" findings=%d", group.Count)
	}
	return detail
}

func configDetails(report govulncheckReport) []string {
	details := make([]string, 0, 4)
	if report.Config.ScannerName != "" {
		details = append(details, "scanner="+report.Config.ScannerName)
	}
	if report.Config.ScannerVersion != "" {
		details = append(details, "scanner_version="+report.Config.ScannerVersion)
	}
	if report.Config.DB != "" {
		details = append(details, "db="+report.Config.DB)
	}
	if report.Config.ScanLevel != "" {
		details = append(details, "scan_level="+report.Config.ScanLevel)
	}
	return details
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

func govulncheckProvenance(opts Options, path string, args []string) evidence.Provenance {
	command := "govulncheck"
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}

	provenance := provenance(opts, command, "govulncheck")
	provenance.Extra = map[string]string{
		"path": path,
	}
	if len(args) > 0 {
		provenance.Extra["args"] = strings.Join(args, " ")
	}
	return provenance
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
