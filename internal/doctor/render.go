package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

// Render serializes a doctor report.
func Render(report Report, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", FormatText:
		return renderText(report), nil
	case FormatJSON:
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(out, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported doctor format %q", format)
	}
}

func renderText(report Report) []byte {
	var out bytes.Buffer
	fmt.Fprintln(&out, "go-prism doctor")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "Overall: %s\n", strings.ToUpper(string(report.Status)))
	fmt.Fprintf(&out, "Version: %s\n", report.Version)
	fmt.Fprintf(&out, "Workdir: %s\n", report.WorkDir)
	if report.Module != "" {
		fmt.Fprintf(&out, "Module: %s\n", report.Module)
	} else {
		fmt.Fprintln(&out, "Module: <unknown>")
	}
	fmt.Fprintf(&out, "Config: %s (%s)\n", configDisplayPath(report.Config.Path), report.Config.Status)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "Checks:")
	for _, check := range report.Checks {
		fmt.Fprintf(&out, "  %-4s  %-24s %s\n", strings.ToUpper(string(check.Status)), check.ID, check.Message)
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "Next steps:")
		for _, step := range report.NextSteps {
			fmt.Fprintf(&out, "  - %s\n", step)
		}
	}
	return out.Bytes()
}

func configDisplayPath(path string) string {
	if path == "" {
		return "<defaults>"
	}
	return path
}
